package qjs

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	wsp1 "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

//go:embed qjs.wasm
var wasmBytes []byte

var (
	compiledQJSModule wazero.CompiledModule
	compilationCache  wazero.CompilationCache // Shared cache for performance
	cachedBytesHash   uint64
	compilationMutex  sync.Mutex
)

// Runtime wraps a QuickJS WebAssembly runtime with memory management.
type Runtime struct {
	wrt      wazero.Runtime
	module   api.Module
	malloc   api.Function
	free     api.Function
	mem      *Mem
	option   Option
	handle   *Handle
	context  *Context
	registry *ProxyRegistry
}

func createGlobalCompiledModule(
	ctx context.Context,
	closeOnContextDone bool,
	disableBuildCache bool,
	cacheDir string,
	quickjsWasmBytes ...[]byte,
) (err error) {
	// Protect global compilation state with mutex
	compilationMutex.Lock()
	defer compilationMutex.Unlock()

	var qjsBytes []byte
	if len(quickjsWasmBytes) > 0 && len(quickjsWasmBytes[0]) > 0 {
		qjsBytes = quickjsWasmBytes[0]
	} else {
		qjsBytes = wasmBytes
	}

	// Calculate hash of the bytes to check if we need to recompile
	currentHash := hashBytes(qjsBytes)

	// Check if we need to compile or recompile
	if compiledQJSModule == nil || cachedBytesHash != currentHash || disableBuildCache {
		// Create or reuse compilation cache (expensive, so we cache it globally)
		if compilationCache == nil {
			if cacheDir == "" {
				compilationCache = wazero.NewCompilationCache()
			} else if compilationCache, err = wazero.NewCompilationCacheWithDir(cacheDir); err != nil {
				return fmt.Errorf("failed to create compilation cache with dir %s: %w", cacheDir, err)
			}
		}

		// Create temporary RuntimeConfig for compilation (not cached)
		runtimeConfig := wazero.
			NewRuntimeConfig().
			WithCompilationCache(compilationCache).
			WithCloseOnContextDone(closeOnContextDone)
		wrt := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)

		if compiledQJSModule, err = wrt.CompileModule(ctx, qjsBytes); err != nil {
			return fmt.Errorf("failed to compile qjs module: %w", err)
		}

		cachedBytesHash = currentHash
	}

	return nil
}

// New creates a QuickJS runtime with optional configuration.
func New(options ...Option) (runtime *Runtime, err error) {
	defer func() {
		rerr := AnyToError(recover())
		if rerr != nil {
			runtime = nil
			err = fmt.Errorf("failed to create QJS runtime: %w", rerr)
		}
	}()

	proxyRegistry := NewProxyRegistry()

	option, err := getRuntimeOption(proxyRegistry, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime options: %w", err)
	}

	if err := createGlobalCompiledModule(
		option.Context,
		option.CloseOnContextDone,
		option.DisableBuildCache,
		option.CacheDir,
		option.QuickJSWasmBytes,
	); err != nil {
		return nil, fmt.Errorf("failed to create global compiled module: %w", err)
	}

	runtime = &Runtime{
		option:   option,
		context:  &Context{Context: option.Context},
		registry: proxyRegistry,
	}

	// Create per-instance RuntimeConfig with shared compilation cache
	// This allows per-instance memory limits while keeping compilation fast
	runtimeConfig := wazero.
		NewRuntimeConfig().
		WithCompilationCache(compilationCache). // Shared cache (fast)
		WithCloseOnContextDone(option.CloseOnContextDone)

	// Apply per-instance memory limit if specified
	// Convert MemoryLimit (bytes) to wazero pages (1 page = 64KB = 65536 bytes)
	// Rounds UP to ensure user gets at least requested memory.
	// For exact limits, ensure MemoryLimit is a multiple of 65536 (64KB page size).
	// Example: 268435456 bytes (256MB) → 4096 pages (exact)
	// Example: 268435457 bytes (256MB + 1 byte) → 4097 pages (~256.015625MB)
	if option.MemoryLimit > 0 {
		// Round up: (bytes + pageSize - 1) / pageSize
		memoryPages := uint32((option.MemoryLimit + 65535) / 65536)
		if memoryPages > 0 {
			runtimeConfig = runtimeConfig.WithMemoryLimitPages(memoryPages)
		}
	}

	runtime.wrt = wazero.NewRuntimeWithConfig(option.Context, runtimeConfig)

	if _, err := wsp1.Instantiate(option.Context, runtime.wrt); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	if _, err := runtime.wrt.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithFunc(option.ProxyFunction).
		Export("jsFunctionProxy").
		Instantiate(option.Context); err != nil {
		return nil, fmt.Errorf("failed to setup host module: %w", err)
	}

	// Build module config with optional WASI APIs based on security options
	moduleConfig := wazero.NewModuleConfig().
		WithStartFunctions(option.StartFunctionName).
		WithStdout(option.Stdout).
		WithStderr(option.Stderr)

	// Conditionally enable filesystem access (GitHub issue #31)
	if !option.DisableFilesystem {
		fsConfig := wazero.NewFSConfig().WithDirMount(runtime.option.CWD, "/")
		moduleConfig = moduleConfig.WithFSConfig(fsConfig)
	}

	// Conditionally enable system time APIs (GitHub issue #31)
	if !option.DisableSystemTime {
		moduleConfig = moduleConfig.
			WithSysWalltime().
			WithSysNanotime().
			WithSysNanosleep()
	}

	if runtime.module, err = runtime.wrt.InstantiateModule(
		option.Context,
		compiledQJSModule,
		moduleConfig,
	); err != nil {
		return nil, fmt.Errorf("failed to instantiate module: %w", err)
	}

	runtime.initializeRuntime()

	return runtime, nil
}

func (r *Runtime) Raw() uint64 {
	return r.handle.raw
}

// FreeQJSRuntime frees the QJS runtime.
func (r *Runtime) FreeQJSRuntime() {
	defer func() {
		err := AnyToError(recover())
		if err != nil {
			panic(fmt.Errorf("failed to free QJS runtime: %w", err))
		}
	}()

	r.Call("QJS_Free", r.handle.raw)
}

// Mem returns the WebAssembly memory interface for this runtime.
func (r *Runtime) Mem() *Mem {
	return r.mem
}

// String returns a string representation of the runtime.
func (r *Runtime) String() string {
	return fmt.Sprintf("QJSRuntime: %p", r)
}

// Close cleanly shuts down the runtime and frees all associated resources.
func (r *Runtime) Close() {
	if r == nil {
		return
	}

	// Free QJS runtime handle
	if r.handle != nil {
		r.FreeQJSRuntime()
		r.handle = nil
	}

	// Close WASM module
	if r.module != nil {
		r.module.Close(r.context)
		r.module = nil
	}

	// Clear references
	if r.context != nil {
		r.context = nil
	}

	if r.registry != nil {
		r.registry.Clear()
		r.registry = nil
	}

	// Clear function references
	r.malloc = nil
	r.free = nil
	r.mem = nil
}

// Load executes a JavaScript file in the runtime's context.
func (r *Runtime) Load(file string, flags ...EvalOptionFunc) (*Value, error) {
	return r.context.Load(file, flags...)
}

// Eval executes JavaScript code in the runtime's context.
func (r *Runtime) Eval(file string, flags ...EvalOptionFunc) (*Value, error) {
	return r.context.Eval(file, flags...)
}

// Compile compiles JavaScript code to bytecode without executing it.
func (r *Runtime) Compile(file string, flags ...EvalOptionFunc) ([]byte, error) {
	return r.context.Compile(file, flags...)
}

// Context returns the JavaScript execution context for this runtime.
func (r *Runtime) Context() *Context {
	return r.context
}

// Call invokes a WebAssembly function by name with the given arguments.
func (r *Runtime) Call(name string, args ...uint64) *Handle {
	return NewHandle(r, r.call(name, args...))
}

// CallUnPack calls a WebAssembly function and unpacks the returned pointer.
func (r *Runtime) CallUnPack(name string, args ...uint64) (uint32, uint32) {
	return r.mem.UnpackPtr(r.Call(name, args...).raw)
}

// Malloc allocates memory in the WebAssembly linear memory and return a pointer to it.
func (r *Runtime) Malloc(size uint64) uint64 {
	ptrs, err := r.malloc.Call(r.context, size)
	if err != nil {
		panic(fmt.Errorf("failed to allocate memory: %w", err))
	}

	return ptrs[0]
}

// FreeHandle releases memory allocated in WebAssembly linear memory.
func (r *Runtime) FreeHandle(ptr uint64) {
	if _, err := r.free.Call(r.context, ptr); err != nil {
		panic(fmt.Errorf("failed to free memory: %w", err))
	}
}

// FreeJsValue frees a JavaScript value in the QuickJS runtime.
func (r *Runtime) FreeJsValue(val uint64) {
	r.Call("QJS_FreeValue", r.context.Raw(), val)
}

// NewBytesHandle creates a handle for byte data in WebAssembly memory.
func (r *Runtime) NewBytesHandle(b []byte) *Handle {
	if len(b) == 0 {
		return nil
	}

	ptr := r.Malloc(uint64(len(b)))
	r.mem.MustWrite(uint32(ptr), b)

	return NewHandle(r, ptr)
}

// NewStringHandle creates a handle for string data with null termination.
func (r *Runtime) NewStringHandle(v string) *Handle {
	// Allocate len+1 for null terminator
	ptr := r.Malloc(uint64(len(v) + 1))

	// Write string data
	r.mem.MustWrite(uint32(ptr), []byte(v))
	// Add null terminator
	r.mem.MustWrite(uint32(ptr)+uint32(len(v)), []byte{0})

	return NewHandle(r, ptr)
}

// initializeRuntime sets up the runtime components after module instantiation.
func (r *Runtime) initializeRuntime() {
	r.malloc = r.module.ExportedFunction("malloc")
	r.free = r.module.ExportedFunction("free")
	r.mem = &Mem{mem: r.module.Memory()}
	r.handle = r.Call(
		"New_QJS",
		uint64(r.option.MemoryLimit),
		uint64(r.option.MaxStackSize),
		uint64(r.option.MaxExecutionTime),
		uint64(r.option.GCThreshold),
	)

	r.context.handle = r.Call("QJS_GetContext", r.handle.raw)
	r.context.runtime = r
}

func (r *Runtime) call(name string, args ...uint64) uint64 {
	fn := r.module.ExportedFunction(name)
	if fn == nil {
		panic(fmt.Errorf("WASM function %s not found", name))
	}

	results, err := fn.Call(r.context, args...)
	if err != nil {
		// Handle context cancellation/timeout gracefully via sys.ExitError
		// This is the proper way to detect when wazero's CloseOnContextDone
		// terminates execution due to context cancellation or timeout.
		// See: https://pkg.go.dev/github.com/tetratelabs/wazero/sys#ExitError
		var exitErr *sys.ExitError
		if errors.As(err, &exitErr) {
			// Context was cancelled or timed out - this is expected behavior
			// Return a clean error instead of panicking
			panic(fmt.Errorf("execution interrupted (context done): %w", err))
		}

		// For other errors, panic with full context
		stack := debug.Stack()
		panic(fmt.Errorf("failed to call %s: %w\nstack: %s", name, err, stack))
	}

	if len(results) == 0 {
		return 0
	}

	return results[0]
}

// Pool manages a collection of reusable QuickJS runtimes.
type Pool struct {
	pools      chan *Runtime
	size       int
	option     Option
	setupFuncs []func(*Runtime) error
	mu         sync.Mutex
}

// NewPool creates a new runtime pool with the specified size and configuration.
func NewPool(size int, option Option, setupFuncs ...func(*Runtime) error) *Pool {
	if size <= 0 {
		panic("pool size must be greater than 0")
	}

	p := &Pool{
		pools:      make(chan *Runtime, size),
		size:       size,
		option:     option,
		setupFuncs: setupFuncs,
	}

	return p
}

// Get returns a runtime from the pool or creates a new one if the pool is empty.
// The caller must call Put() to return the runtime when finished.
func (p *Pool) Get() (*Runtime, error) {
	if p == nil {
		return nil, errors.New("pool is nil")
	}

	// Try to get from pool first
	select {
	case rt := <-p.pools:
		return p.prepareRuntimeForUse(rt), nil
	// Pool is empty, need to create new runtime
	default:
	}

	// Double-check with lock to avoid race conditions
	p.mu.Lock()
	defer p.mu.Unlock()

	select {
	case rt := <-p.pools:
		return p.prepareRuntimeForUse(rt), nil
	default:
		return p.createNewRuntime()
	}
}

// Put returns the runtime back to the pool for reuse.
// If the pool is full, the runtime is closed to prevent resource leaks.
func (p *Pool) Put(rt *Runtime) {
	if rt == nil {
		return
	}

	// Update stack top before returning to pool
	if rt.handle != nil {
		rt.Call("QJS_UpdateStackTop", rt.handle.raw)
	}

	select {
	case p.pools <- rt:
		// Successfully returned to pool
	default:
		// Pool is full, close the runtime
		rt.Close()
	}
}

// prepareRuntimeForUse prepares a pooled runtime for use.
func (p *Pool) prepareRuntimeForUse(rt *Runtime) *Runtime {
	if rt != nil && rt.handle != nil {
		rt.Call("QJS_UpdateStackTop", rt.handle.raw)
	}

	return rt
}

// createNewRuntime creates a new runtime with setup functions applied.
func (p *Pool) createNewRuntime() (*Runtime, error) {
	rt, err := New(p.option)
	if err != nil {
		return nil, fmt.Errorf("failed to create new runtime: %w", err)
	}

	// Apply setup functions
	for i, setupFunc := range p.setupFuncs {
		err := setupFunc(rt)
		if err != nil {
			rt.Close()

			return nil, fmt.Errorf("setup function %d failed: %w", i, err)
		}
	}

	return rt, nil
}
