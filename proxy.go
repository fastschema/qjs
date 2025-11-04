package qjs

import (
	"context"
	"encoding/binary"
	"sync"
	"sync/atomic"

	"github.com/tetratelabs/wazero/api"
)

// ProxyRegistry stores Go functions that can be called from JavaScript.
// It provides thread-safe registration and retrieval of functions with automatic ID generation.
type ProxyRegistry struct {
	nextID  uint64
	mu      sync.RWMutex
	proxies map[uint64]any
}

// NewProxyRegistry creates a new thread-safe proxy registry.
func NewProxyRegistry() *ProxyRegistry {
	return &ProxyRegistry{
		proxies: make(map[uint64]any),
	}
}

// Register adds a function to the registry and returns its unique ID.
// This method is thread-safe and can be called concurrently.
func (r *ProxyRegistry) Register(fn any) uint64 {
	if fn == nil {
		return 0
	}

	id := atomic.AddUint64(&r.nextID, 1)
	r.mu.Lock()
	r.proxies[id] = fn
	r.mu.Unlock()

	return id
}

// Get retrieves a function by its ID.
// Returns the function and true if found, nil, and false otherwise.
// This method is thread-safe and can be called concurrently.
func (r *ProxyRegistry) Get(id uint64) (any, bool) {
	if id == 0 {
		return nil, false
	}

	r.mu.RLock()
	fn, ok := r.proxies[id]
	r.mu.RUnlock()

	return fn, ok
}

// Unregister removes a function from the registry by its ID.
// Returns true if the function was found and removed, false otherwise.
// This method is thread-safe and can be called concurrently.
func (r *ProxyRegistry) Unregister(id uint64) bool {
	if id == 0 {
		return false
	}

	r.mu.Lock()

	_, exists := r.proxies[id]
	if exists {
		delete(r.proxies, id)
	}

	r.mu.Unlock()

	return exists
}

// Len returns the number of registered functions.
// This method is thread-safe.
func (r *ProxyRegistry) Len() int {
	r.mu.RLock()
	count := len(r.proxies)
	r.mu.RUnlock()

	return count
}

// Clear removes all registered functions from the registry.
// This method is thread-safe.
func (r *ProxyRegistry) Clear() {
	r.mu.Lock()
	r.proxies = make(map[uint64]any)
	r.mu.Unlock()
}

// JsFunctionProxy is the Go host function that will be imported by the WASM module.
// It corresponds to the following C declaration:
//
//	__attribute__((import_module("env"), import_name("jsFunctionProxy")))
//	extern JSValue jsFunctionProxy(JSContext *ctx, JSValueConst this, int argc, JSValueConst *argv);
//
// Parameters:
//   - ctx: 	JSContext pointer
//   - this: 	JSValueConst this (the "this" value)
//   - argc: 	int argc (number of arguments)
//   - argv: 	pointer to argv (an array of JSValueConst/JSValue, each 8 bytes - uint64)
type JsFunctionProxy = func(
	ctx context.Context,
	module api.Module,
	jsCtx uint32,
	thisVal uint64,
	argc uint32,
	argv uint32,
) (rs uint64)

// ModuleLoaderFunc is a Go function that loads JavaScript modules.
// It receives the module name and should return the module's source code as a string,
// or an error if the module cannot be loaded.
//
// Return values:
//   - (source, nil): The module source code will be compiled and loaded
//   - ("", error): The error will be thrown as a JavaScript exception
//   - ("", nil): Falls back to the default file system loader
//
// Example:
//
//	func(ctx *Context, moduleName string) (string, error) {
//	    if source, ok := myModules[moduleName]; ok {
//	        return source, nil
//	    }
//	    return "", nil  // Fall back to default loader
//	}
type ModuleLoaderFunc func(ctx *Context, moduleName string) (string, error)

// JsModuleLoaderProxy is the Go host function for module loading that will be imported by the WASM module.
// It corresponds to the following C declaration:
//
//	__attribute__((import_module("env"), import_name("jsModuleLoaderProxy")))
//	extern uint64_t jsModuleLoaderProxy(uint32_t ctx, uint32_t module_name, uint64_t callback_id);
//
// Parameters:
//   - jsCtx: JSContext pointer (as uint32)
//   - moduleNamePtr: pointer to module name string in WASM memory
//   - callbackID: ID of the registered Go callback function
//
// Returns:
//   - JSModuleDef pointer as uint64 (or 0 on error)
type JsModuleLoaderProxy = func(
	ctx context.Context,
	module api.Module,
	jsCtx uint32,
	moduleNamePtr uint32,
	callbackID uint64,
) uint64

// createFuncProxyWithRegistry creates a WASM function proxy that bridges JavaScript function calls to Go functions.
// It handles parameter extraction, error recovery, and result conversion between JS and Go.
func createFuncProxyWithRegistry(registry *ProxyRegistry) JsFunctionProxy {
	return func(
		_ context.Context,
		module api.Module,
		_ uint32,
		thisVal uint64,
		argc uint32,
		argv uint32,
	) (rs uint64) {
		goFunc, this := getProxyFuncParams(registry, module.Memory(), thisVal, argc, argv)

		defer func() {
			if r := recover(); r != nil {
				rs = handlePanicRecovery(this, r)
			}
		}()

		result, err := goFunc(this)
		if err != nil {
			return this.context.ThrowError(err).Raw()
		}

		return validateAndReturnResult(this, result)
	}
}

// handlePanicRecovery handles panics during Go function execution.
func handlePanicRecovery(this *This, r any) uint64 {
	recoveredErr := newProxyErr(0, r)
	if this.isAsync && this.promise != nil {
		errVal := this.context.NewError(recoveredErr)
		rejectErr := this.promise.Reject(errVal)

		return this.context.ThrowError(combineErrors(recoveredErr, rejectErr)).Raw()
	}

	return this.context.ThrowError(recoveredErr).Raw()
}

// validateAndReturnResult validates the function result and handles JavaScript exceptions.
func validateAndReturnResult(this *This, result *Value) uint64 {
	if this.context.HasException() {
		panic(this.context.Exception())
	}

	if result == nil {
		return this.context.NewUndefined().Raw()
	}

	return result.Raw()
}

// getProxyFuncParams extracts and validates parameters for proxy function calls.
// It parses WASM memory to extract function ID, context, arguments, and async information.
func getProxyFuncParams(
	registry *ProxyRegistry,
	mem api.Memory,
	thisRef uint64,
	argc uint32,
	argv uint32,
) (Function, *This) {
	args := readArgsFromWasmMem(mem, argc, argv)
	functionHandle := args[0]  // The first argument is the function ID
	jsContextHandle := args[1] // The second argument is the context ID
	isAsync := args[2]         // The third argument is the async flag

	goFunc, goContext := retrieveGoResources(registry, functionHandle, jsContextHandle)
	promise := extractPromiseIfAsync(goContext, args, isAsync)
	fnArgs := extractFunctionArguments(goContext, args)
	this := createThisContext(goContext, thisRef, fnArgs, promise, isAsync)

	return goFunc, this
}

// extractPromiseIfAsync extracts the promise handle for async functions.
func extractPromiseIfAsync(context *Context, args []uint64, isAsync uint64) *Value {
	if isAsync == 0 {
		return context.NewUndefined()
	}

	promiseHandle := args[3]

	return context.NewValue(NewHandle(context.runtime, promiseHandle))
}

// extractFunctionArguments extracts and clones function arguments from WASM memory.
func extractFunctionArguments(context *Context, args []uint64) []*Value {
	fnArgs := make([]*Value, len(args)-4)
	for i := range fnArgs {
		argHandle := args[4+i]
		arg := context.NewValue(NewHandle(context.runtime, argHandle))
		fnArgs[i] = arg.Clone()
	}

	return fnArgs
}

// createThisContext creates the This context for function execution.
func createThisContext(context *Context, thisRef uint64, args []*Value, promise *Value, isAsync uint64) *This {
	thisValue := context.NewValue(NewHandle(context.runtime, thisRef))
	this := &This{
		context: context,
		Value:   thisValue,
		args:    args,
		promise: promise,
		isAsync: isAsync != 0,
	}

	return this
}

// retrieveGoResources retrieves and validates Go function and context from the registry.
func retrieveGoResources(
	registry *ProxyRegistry,
	functionHandle uint64,
	jsContextHandle uint64,
) (Function, *Context) {
	// Retrieve and validate function
	fn, _ := registry.Get(functionHandle)
	goFunc, _ := fn.(Function)
	jsContext, _ := registry.Get(jsContextHandle)
	goContext, _ := jsContext.(*Context)

	return goFunc, goContext
}

// readArgsFromWasmMem reads the argument array from WASM memory with proper validation.
// Each argument is 8 bytes (uint64) in little-endian format.
func readArgsFromWasmMem(mem api.Memory, argc uint32, argv uint32) []uint64 {
	args := make([]uint64, argc)

	for i := range argc {
		// Calculate the offset for the i-th argument
		offset := argv + (i * 8)
		data, _ := mem.Read(offset, 8)
		// Convert the 8 bytes to a uint64 (little-endian encoding)
		args[i] = binary.LittleEndian.Uint64(data)
	}

	return args
}

// createModuleLoaderProxyWithRegistry creates a WASM module loader proxy that bridges QuickJS module loading to Go
// functions.
func createModuleLoaderProxyWithRegistry(registry *ProxyRegistry, runtime *Runtime) JsModuleLoaderProxy {
	return func(
		_ context.Context,
		module api.Module,
		_ uint32,
		moduleNamePtr uint32,
		callbackID uint64,
	) uint64 {
		// Read the module name from WASM memory
		moduleName := readStringFromWasmMem(module.Memory(), moduleNamePtr)

		// Get the Go callback function
		fn, ok := registry.Get(callbackID)
		if !ok {
			// No callback registered - return 0 (NULL)
			return 0
		}

		moduleLoader, ok := fn.(ModuleLoaderFunc)
		if !ok {
			// Wrong type - return 0 (NULL)
			return 0
		}

		// Get the context
		ctx := runtime.Context()

		// Call the Go module loader function
		defer func() {
			if r := recover(); r != nil {
				// Handle panic by setting an exception
				ctx.ThrowError(AnyToError(r))
			}
		}()

		source, err := moduleLoader(ctx, moduleName)
		if err != nil {
			// Throw error as JavaScript exception
			ctx.ThrowError(err)

			return 0
		}

		if source == "" {
			// Empty source - fall back to default file system loader
			return ctx.defaultModuleLoader(moduleName)
		}

		// Compile and load the module source code
		value, err := ctx.Load(moduleName, Code(source), TypeModule())
		if err != nil {
			// Compilation/loading failed - throw error
			ctx.ThrowError(err)

			return 0
		}

		// Return JSModuleDef pointer as uint64
		return value.Raw()
	}
}

// readStringFromWasmMem reads a null-terminated C string from WASM memory.
func readStringFromWasmMem(mem api.Memory, ptr uint32) string {
	if ptr == 0 {
		return ""
	}

	// Read bytes until we hit a null terminator
	var bytes []byte
	offset := ptr

	for {
		b, ok := mem.ReadByte(offset)
		if !ok || b == 0 {
			break
		}
		bytes = append(bytes, b)

		offset++
	}

	return string(bytes)
}
