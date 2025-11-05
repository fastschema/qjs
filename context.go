package qjs

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// Context represents a QuickJS execution context with associated runtime.
type Context struct {
	context.Context

	handle  *Handle
	runtime *Runtime
	global  *Value
}

func (c *Context) Call(name string, args ...uint64) *Value {
	return c.NewValue(c.runtime.Call(name, args...))
}

// CallUnPack delegates function calls and unpacks the result.
func (c *Context) CallUnPack(name string, args ...uint64) (uint32, uint32) {
	return c.runtime.CallUnPack(name, args...)
}

// FreeHandle releases memory associated with the given handle pointer.
func (c *Context) FreeHandle(ptr uint64) {
	c.runtime.FreeHandle(ptr)
}

// FreeJsValue releases memory associated with the JavaScript value.
func (c *Context) FreeJsValue(val uint64) {
	c.runtime.FreeJsValue(val)
}

// Malloc allocates memory in the WASM runtime.
func (c *Context) Malloc(size uint64) uint64 {
	return c.runtime.Malloc(size)
}

// MemRead reads bytes from WASM memory at the given address.
func (c *Context) MemRead(addr uint32, size uint64) []byte {
	return c.runtime.mem.MustRead(addr, size)
}

// MemWrite writes bytes to WASM memory at the given address.
func (c *Context) MemWrite(addr uint32, b []byte) {
	c.runtime.mem.MustWrite(addr, b)
}

// NewProxyValue creates a new Value that represents a proxy to a Go value.
func (c *Context) NewProxyValue(v any) *Value {
	proxyID := c.runtime.registry.Register(v)

	return c.Call("QJS_NewProxyValue", c.Raw(), proxyID)
}

// NewStringHandle creates a Value from a string using runtime handle.
func (c *Context) NewStringHandle(v string) *Value {
	handle := c.runtime.NewStringHandle(v)

	return c.NewValue(handle)
}

// NewBytes creates a Value from a byte slice.
func (c *Context) NewBytes(v []byte) *Value {
	return c.NewValue(c.runtime.NewBytesHandle(v))
}

// String returns the string representation of the Context.
func (c *Context) String() string {
	return fmt.Sprintf("Context(%p)", c.handle)
}

// Raw returns the raw handle value for low-level operations.
func (c *Context) Raw() uint64 {
	return c.handle.raw
}

// Load loads a JavaScript module without evaluating it.
func (c *Context) Load(file string, flags ...EvalOptionFunc) (*Value, error) {
	return load(c, file, flags...)
}

// Eval evaluates a script within the current context.
func (c *Context) Eval(file string, flags ...EvalOptionFunc) (*Value, error) {
	return eval(c, file, flags...)
}

// Compile compiles a script into bytecode.
func (c *Context) Compile(file string, flags ...EvalOptionFunc) ([]byte, error) {
	return compile(c, file, flags...)
}

// Global returns the global object, caching it for subsequent calls.
func (c *Context) Global() *Value {
	if c.global == nil {
		c.global = c.Call("JS_GetGlobalObject", c.Raw())
	}

	return c.global
}

// SetFunc sets a function with given name in the global object.
func (c *Context) SetFunc(name string, fn Function) {
	jsFn := c.Function(fn)
	global := c.Global()
	global.SetPropertyStr(name, jsFn)
}

// SetAsyncFunc sets an async function with given name in the global object.
func (c *Context) SetAsyncFunc(name string, fn AsyncFunction) {
	jsFn := c.Function(func(this *This) (*Value, error) {
		fn(this)

		return this.context.NewUndefined(), nil
	}, true)
	global := c.Global()
	global.SetPropertyStr(name, jsFn)
}

// ParseJSON parses given JSON string and returns an object value.
func (c *Context) ParseJSON(v string) *Value {
	cStr := c.NewStringHandle(v)
	defer cStr.Free()

	return c.Call("QJS_ParseJSON", c.Raw(), cStr.Raw())
}

// NewValue creates a new Value wrapper around the given handle.
func (c *Context) NewValue(handle *Handle) *Value {
	return &Value{context: c, handle: handle}
}

// NewUninitialized creates a new uninitialized JavaScript value.
func (c *Context) NewUninitialized() *Value {
	return c.Call("JS_NewUninitialized")
}

// NewNull creates a new null JavaScript value.
func (c *Context) NewNull() *Value {
	return c.Call("JS_NewNull")
}

// NewUndefined creates a new undefined JavaScript value.
func (c *Context) NewUndefined() *Value {
	return c.Call("JS_NewUndefined")
}

// NewDate creates a new JavaScript Date object from the given time.
func (c *Context) NewDate(t *time.Time) *Value {
	epochMs := float64(t.UnixNano()) / NanosToMillis // Nanoseconds to milliseconds

	return c.Call("JS_NewDate", c.Raw(), math.Float64bits(epochMs))
}

// NewBool creates a new JavaScript boolean value.
func (c *Context) NewBool(b bool) *Value {
	boolVal := uint64(0)
	if b {
		boolVal = 1
	}

	return c.Call("QJS_NewBool", c.Raw(), boolVal)
}

// NewUint32 creates a new JavaScript number from uint32.
func (c *Context) NewUint32(v uint32) *Value {
	return c.Call("QJS_NewUint32", c.Raw(), uint64(v))
}

// NewInt32 creates a new JavaScript number from int32.
func (c *Context) NewInt32(v int32) *Value {
	return c.Call("QJS_NewInt32", c.Raw(), uint64(v))
}

// NewInt64 creates a new JavaScript number from int64.
func (c *Context) NewInt64(v int64) *Value {
	return c.Call("QJS_NewInt64", c.Raw(), uint64(v))
}

// NewBigInt64 creates a new JavaScript BigInt from int64.
func (c *Context) NewBigInt64(v int64) *Value {
	return c.Call("QJS_NewBigInt64", c.Raw(), uint64(v))
}

// NewBigUint64 creates a new JavaScript BigInt from uint64.
func (c *Context) NewBigUint64(v uint64) *Value {
	return c.Call("QJS_NewBigUint64", c.Raw(), v)
}

// NewFloat64 creates a new JavaScript number from float64.
func (c *Context) NewFloat64(v float64) *Value {
	return c.Call("QJS_NewFloat64", c.Raw(), math.Float64bits(v))
}

// NewString creates a new JavaScript string value.
func (c *Context) NewString(v string) *Value {
	str := c.NewStringHandle(v)

	return c.Call("QJS_NewString", c.Raw(), str.Raw())
}

// NewObject creates a new empty JavaScript object.
func (c *Context) NewObject() *Value {
	return c.Call("JS_NewObject", c.Raw())
}

// NewArray creates a new empty JavaScript array.
func (c *Context) NewArray() *Array {
	val := c.Call("JS_NewArray", c.Raw())

	return NewArray(val)
}

// NewMap creates a new JavaScript Map object.
func (c *Context) NewMap() *Map {
	m := c.Global().GetPropertyStr("Map")
	defer m.Free()

	val := c.Call("JS_CallConstructor", c.Raw(), m.Raw(), 0, 0)

	return NewMap(val)
}

// NewSet creates a new JavaScript Set object.
func (c *Context) NewSet() *Set {
	s := c.Global().GetPropertyStr("Set")
	defer s.Free()

	val := c.Call("JS_CallConstructor", c.Raw(), s.Raw(), 0, 0)

	return NewSet(val)
}

// NewAtom creates a new Atom from the given string.
func (c *Context) NewAtom(v string) Atom {
	cstr := c.NewStringHandle(v)

	atomValue := c.Call("JS_NewAtom", c.Raw(), cstr.Raw())

	defer cstr.Free()

	return Atom{context: c, Value: atomValue}
}

// NewAtomIndex creates a new Atom with the given index.
func (c *Context) NewAtomIndex(index int64) Atom {
	return Atom{
		context: c,
		Value:   c.Call("QJS_NewAtomUInt32", c.Raw(), uint64(index)),
	}
}

// NewArrayBuffer creates a new JavaScript ArrayBuffer with the given binary data.
func (c *Context) NewArrayBuffer(binaryData []byte) *Value {
	ptr := c.Malloc(uint64(len(binaryData)))
	defer c.FreeHandle(ptr)

	c.MemWrite(uint32(ptr), binaryData)

	return c.Call("QJS_NewArrayBufferCopy", c.Raw(), ptr, uint64(len(binaryData)))
}

// NewError creates a new JavaScript Error object from Go error.
func (c *Context) NewError(e error) *Value {
	errString := c.NewString(e.Error())
	errVal := c.Call("JS_NewError", c.Raw())
	errVal.SetPropertyStr("message", errString)

	return errVal
}

// HasException returns true if there is a pending exception.
func (c *Context) HasException() bool {
	return c.Call("JS_HasException", c.Raw()).Bool()
}

// Exception returns and clears the current pending exception.
func (c *Context) Exception() error {
	val := c.Call("JS_GetException", c.Raw())
	defer val.Free()

	return val.Exception()
}

// Throw throws a value as an exception.
func (c *Context) Throw(v *Value) *Value {
	return c.Call("JS_Throw", c.Raw(), v.Raw())
}

// ThrowError throws an exception with the given error.
func (c *Context) ThrowError(err error) *Value {
	return c.Throw(c.NewError(err))
}

// ThrowSyntaxError throws syntax error with given cause.
func (c *Context) ThrowSyntaxError(format string, args ...any) *Value {
	cause := fmt.Sprintf(format, args...)

	causePtr := c.NewStringHandle(cause)
	defer causePtr.Free()

	return c.Call("QJS_ThrowSyntaxError", c.Raw(), causePtr.Raw())
}

// ThrowTypeError throws type error with given cause.
func (c *Context) ThrowTypeError(format string, args ...any) *Value {
	cause := fmt.Sprintf(format, args...)

	causePtr := c.NewStringHandle(cause)
	defer causePtr.Free()

	return c.Call("QJS_ThrowTypeError", c.Raw(), causePtr.Raw())
}

// ThrowReferenceError throws reference error with given cause.
func (c *Context) ThrowReferenceError(format string, args ...any) *Value {
	cause := fmt.Sprintf(format, args...)

	causePtr := c.NewStringHandle(cause)
	defer causePtr.Free()

	return c.Call("QJS_ThrowReferenceError", c.Raw(), causePtr.Raw())
}

// ThrowRangeError throws range error with given cause.
func (c *Context) ThrowRangeError(format string, args ...any) *Value {
	cause := fmt.Sprintf(format, args...)

	causePtr := c.NewStringHandle(cause)
	defer causePtr.Free()

	return c.Call("QJS_ThrowRangeError", c.Raw(), causePtr.Raw())
}

// ThrowInternalError throws internal error with given cause.
func (c *Context) ThrowInternalError(format string, args ...any) *Value {
	cause := fmt.Sprintf(format, args...)

	causePtr := c.NewStringHandle(cause)
	defer causePtr.Free()

	return c.Call("QJS_ThrowInternalError", c.Raw(), causePtr.Raw())
}

// Function creates a JavaScript function that wraps the given Go function.
func (c *Context) Function(fn Function, isAsyncs ...bool) *Value {
	isAsync := uint64(0)
	if len(isAsyncs) > 0 && isAsyncs[0] {
		isAsync = 1
	}

	fnID := c.runtime.registry.Register(fn)
	ctxID := c.runtime.registry.Register(c)
	proxyFuncVal := c.Call("QJS_CreateFunctionProxy", c.Raw(), fnID, ctxID, isAsync)

	// Registry: Store IDs for cleanup and callback identification
	proxyFuncVal.SetPropertyStr("__fnID", c.NewInt64(int64(fnID)))
	proxyFuncVal.SetPropertyStr("__ctxID", c.NewInt64(int64(ctxID)))

	return proxyFuncVal
}

// Invoke invokes a function with given this value and arguments.
func (c *Context) Invoke(fn *Value, this *Value, args ...*Value) (*Value, error) {
	argc, argvPtr := createJsCallArgs(c, args...)
	defer c.FreeHandle(argvPtr)

	jsCallArgs := []uint64{
		c.Raw(),
		fn.Raw(),
		this.Raw(),
		argc,
		argvPtr,
	}
	result := c.Call("QJS_Call", jsCallArgs...)

	return normalizeJsValue(c, result)
}

func (c *Context) defaultModuleLoader(moduleName string) uint64 {
	moduleNameHandle := c.NewStringHandle(moduleName)
	defer moduleNameHandle.Free()

	result := c.runtime.Call("QJS_ModuleLoader", c.Raw(), moduleNameHandle.Raw(), 0)

	return result.raw
}

// createJsCallArgs marshals Go Value arguments to WASM memory for JavaScript calls.
func createJsCallArgs(c *Context, args ...*Value) (uint64, uint64) {
	var argvPtr uint64

	argc := uint64(len(args))
	if argc > 0 {
		argvBytes := make([]byte, Uint64ByteSize*argc)
		for i, v := range args {
			// WASM: Pack 64-bit JSValue handles in little-endian format
			binary.LittleEndian.PutUint64(argvBytes[i*Uint64ByteSize:], v.Raw())
		}

		// WASM: Allocate and write argument array to memory
		argvPtr = c.Malloc(uint64(len(argvBytes)))
		c.MemWrite(uint32(argvPtr), argvBytes)
	}

	return argc, argvPtr
}
