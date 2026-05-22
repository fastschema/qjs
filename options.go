package qjs

import (
	"context"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
)

const (
	// JsEvalTypeGlobal evaluates code in global scope (default).
	JsEvalTypeGlobal = (0 << 0)
	// JsEvalTypeModule evaluates code as ES6 module.
	JsEvalTypeModule = (1 << 0)
	// JsEvalTypeDirect performs direct call (internal use).
	JsEvalTypeDirect = (2 << 0)
	// JsEvalTypeInDirect performs indirect call (internal use).
	JsEvalTypeInDirect = (3 << 0)
	// JsEvalTypeMask masks the eval type bits.
	JsEvalTypeMask = (3 << 0)
	// JsEvalFlagStrict forces strict mode execution.
	JsEvalFlagStrict = (1 << 3)
	// JsEvalFlagUnUsed is reserved for future use.
	JsEvalFlagUnUsed = (1 << 4)
	// JsEvalFlagCompileOnly returns a JS bytecode/module for JS_EvalFunction().
	JsEvalFlagCompileOnly = (1 << 5)
	// JsEvalFlagBackTraceBarrier prevents the stack frames before this eval in the Error() backtraces.
	JsEvalFlagBackTraceBarrier = (1 << 6)
	// JsEvalFlagAsync enables top-level await (global scope only).
	JsEvalFlagAsync = (1 << 7)
)

type Option struct {
	Context context.Context
	// Enabling this option significantly increases evaluation time
	// because every operation must check the done context, which introduces additional overhead.
	CloseOnContextDone bool
	DisableBuildCache  bool
	CacheDir           string
	MemoryLimit        int
	MaxStackSize       int
	MaxExecutionTime   int
	GCThreshold        int
	QuickJSWasmBytes   []byte
	ProxyFunction      any
	ModuleConfig       wazero.ModuleConfig
}

// EvalOption configures JavaScript evaluation behavior in QuickJS context.
type EvalOption struct {
	c           *Context
	file        string
	code        string
	bytecode    []byte
	bytecodeLen int
	flags       uint64

	// QuickJS value handles for memory management
	fileValue     *Value
	codeValue     *Value
	byteCodeValue *Value
}

// EvalOptionFunc configures evaluation behavior using functional option pattern.
type EvalOptionFunc func(*EvalOption)

// createEvalOption initializes default option with global scope and strict mode.
func createEvalOption(c *Context, file string, flags ...EvalOptionFunc) *EvalOption {
	evalOption := &EvalOption{
		c:     c,
		file:  file,
		flags: JsEvalTypeGlobal | JsEvalFlagStrict,
	}

	for _, flag := range flags {
		flag(evalOption)
	}

	return evalOption
}

// Code sets the JavaScript source code to evaluate.
func Code(code string) EvalOptionFunc {
	return func(o *EvalOption) {
		o.code = code
	}
}

// Bytecode sets precompiled JavaScript bytecode to execute.
func Bytecode(buf []byte) EvalOptionFunc {
	return func(o *EvalOption) {
		o.bytecode = buf
		o.bytecodeLen = len(buf)
	}
}

// TypeGlobal sets evaluation to run in global scope (default behavior).
func TypeGlobal() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalTypeGlobal
	}
}

// TypeModule sets evaluation to run as ES6 module.
func TypeModule() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalTypeModule
	}
}

// FlagAsync enables top-level await in global scripts.
// Returns a promise from JS_Eval(). Only valid with TypeGlobal.
func FlagAsync() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalFlagAsync
	}
}

// FlagStrict forces strict mode execution.
func FlagStrict() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalFlagStrict
	}
}

// FlagCompileOnly compiles code without execution.
// Returns bytecode object for later execution with JS_EvalFunction().
func FlagCompileOnly() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalFlagCompileOnly
	}
}

// TypeDirect sets direct call mode (internal QuickJS use).
func TypeDirect() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalTypeDirect
	}
}

// TypeIndirect sets indirect call mode (internal QuickJS use).
func TypeIndirect() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalTypeInDirect
	}
}

// TypeMask applies eval type mask (internal QuickJS use).
func TypeMask() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalTypeMask
	}
}

// FlagUnused is reserved for future QuickJS features.
func FlagUnused() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalFlagUnUsed
	}
}

// FlagBacktraceBarrier excludes stack frames before this eval from error backtraces.
func FlagBacktraceBarrier() EvalOptionFunc {
	return func(o *EvalOption) {
		o.flags |= JsEvalFlagBackTraceBarrier
	}
}

// Handle creates QuickJS evaluation option handle for WASM function calls.
func (o *EvalOption) Handle() (handle uint64) {
	codeHandle := uint64(0)
	byteCodeHandle := uint64(0)
	o.fileValue = o.c.NewStringHandle(o.file)

	if o.code != "" {
		o.codeValue = o.c.NewStringHandle(o.code)
		codeHandle = o.codeValue.Raw()
	}

	if o.bytecode != nil {
		o.byteCodeValue = o.c.NewBytes(o.bytecode)
		byteCodeHandle = o.byteCodeValue.Raw()
	}

	// Create QuickJS option struct via WASM call
	option := o.c.Call(
		"QJS_CreateEvalOption",
		codeHandle,
		byteCodeHandle,
		uint64(o.bytecodeLen),
		o.fileValue.Raw(),
		o.flags,
	)

	return option.Raw()
}

// Free releases QuickJS value handles to prevent memory leaks.
// Must be called after Handle() to clean up WASM memory.
func (o *EvalOption) Free() {
	if o.fileValue.Raw() != 0 {
		o.c.Call("JS_FreeValue", o.c.Raw(), o.fileValue.Raw())
	}

	if o.codeValue != nil && o.codeValue.Raw() != 0 {
		o.c.Call("JS_FreeValue", o.c.Raw(), o.codeValue.Raw())
	}

	if o.byteCodeValue != nil && o.byteCodeValue.Raw() != 0 {
		o.c.Call("JS_FreeValue", o.c.Raw(), o.byteCodeValue.Raw())
	}
}

func getRuntimeOption(registry *ProxyRegistry, options ...Option) (option Option, err error) {
	if len(options) == 0 {
		option = Option{}
	} else {
		option = options[0]
	}

	if option.Context == nil {
		option.Context = context.Background()
	}

	if option.ProxyFunction == nil {
		option.ProxyFunction = createFuncProxyWithRegistry(registry)
	}

	if option.ModuleConfig == nil {
		var cwd string
		if cwd, err = os.Getwd(); err != nil {
			return Option{}, fmt.Errorf("cannot get current working directory: %w", err)
		}

		fsConfig := wazero.
			NewFSConfig().
			WithDirMount(cwd, "/")

		option.ModuleConfig = wazero.NewModuleConfig().
			WithStartFunctions().
			WithSysWalltime().
			WithSysNanotime().
			WithSysNanosleep().
			WithFSConfig(fsConfig).
			WithStdout(os.Stdout).
			WithStderr(os.Stderr)
	}

	return option, nil
}
