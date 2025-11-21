package qjs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalOptions(t *testing.T) {
	t.Run("DefaultOptions", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		// Use Eval to test the default options (should be global with strict mode)
		val, err := runtime.Eval("test.js", qjs.Code("'use strict'; 'test'"))
		assert.NoError(t, err)
		assert.Equal(t, "test", val.String())
		val.Free()
	})

	t.Run("CWDOption", func(t *testing.T) {
		// Store original working directory to restore later
		originalCwd, err := os.Getwd()
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.Chdir(originalCwd)
		})

		t.Run("default_cwd_from_os_getwd", func(t *testing.T) {
			runtime, err := qjs.New()
			require.NoError(t, err)
			runtime.Close()
		})

		t.Run("explicit_cwd_provided", func(t *testing.T) {
			tempDir := t.TempDir()
			runtime, err := qjs.New(qjs.Option{CWD: tempDir})
			require.NoError(t, err)
			runtime.Close()
		})

		t.Run("deleted_working_directory", func(t *testing.T) {
			// Create a temporary directory and change to it
			tempDir := t.TempDir()
			subDir := filepath.Join(tempDir, "workdir")
			err := os.Mkdir(subDir, 0755)
			require.NoError(t, err, "Failed to create subdirectory")

			// Change to the subdirectory
			err = os.Chdir(subDir)
			require.NoError(t, err)

			// Remove the current working directory while we're in it
			err = os.RemoveAll(subDir)
			require.NoError(t, err)

			_, err = qjs.New()
			_ = os.Chdir(originalCwd)
			// Some systems handle deleted directories gracefully, so error is optional
			if err != nil {
				assert.Contains(t, err.Error(), "failed to get runtime options")
			}
		})
	})

	t.Run("CodeOption", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		val, err := runtime.Eval("test.js", qjs.Code("42"))
		assert.NoError(t, err)
		assert.Equal(t, int32(42), val.Int32())
		val.Free()
	})

	t.Run("TypeModuleOption", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		val, err := runtime.Eval("test.js",
			qjs.Code("export default 'module'"),
			qjs.TypeModule())
		assert.NoError(t, err)
		assert.Equal(t, "module", val.String())
		val.Free()
	})

	t.Run("BytecodeCompileAndEval", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		bytecode, err := runtime.Compile("test.js", qjs.Code("123 + 456"))
		assert.NoError(t, err)
		assert.NotEmpty(t, bytecode)

		val, err := runtime.Eval("test.js", qjs.Bytecode(bytecode))
		assert.NoError(t, err)
		assert.Equal(t, int32(579), val.Int32())
		val.Free()
	})

	t.Run("ModuleBytecodeCompileAndEval", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		bytecode, err := runtime.Compile(
			"test.js",
			qjs.Code("export default 'module bytecode'"),
			qjs.TypeModule())
		assert.NoError(t, err)
		assert.NotEmpty(t, bytecode)

		val, err := runtime.Eval("test.js",
			qjs.Bytecode(bytecode),
			qjs.TypeModule())
		assert.NoError(t, err)
		assert.Equal(t, "module bytecode", val.String())
		val.Free()
	})

	t.Run("FlagAsyncOption", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		// Test async flag with top-level await
		val, err := runtime.Eval("test.js",
			qjs.Code("await Promise.resolve(100)"),
			qjs.FlagAsync())
		assert.NoError(t, err)
		assert.Equal(t, int32(100), val.Int32())
		val.Free()
	})

	t.Run("FlagStrictOption", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		// In strict mode, using undeclared variables throws an error
		_, err := runtime.Eval("test.js",
			qjs.Code("undeclaredVar = 10"),
			qjs.FlagStrict())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "ReferenceError")
	})

	t.Run("FlagCompileOnlyOption", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()
		ctx := runtime.Context()

		// We need to use Go API directly because runtime.Compile
		// already applies FlagCompileOnly
		val, err := ctx.Eval("test.js",
			qjs.Code("42"),
			qjs.FlagCompileOnly())
		assert.NoError(t, err)

		// The result should be bytecode, not the executed result
		assert.False(t, val.IsNumber())
		val.Free()
	})

	t.Run("FlagBacktraceBarrierOption", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		_, err := runtime.Eval("outer.js", qjs.Code(`
            function outer() {
                throw new Error("test error");
            }
        `))
		assert.NoError(t, err)

		_, err = runtime.Eval("without-barrier.js", qjs.Code(`outer()`))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "outer.js")

		_, err = runtime.Eval("with-barrier.js",
			qjs.Code(`outer()`),
			qjs.FlagBacktraceBarrier())
		assert.Error(t, err)
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		val, err := runtime.Eval("test.js",
			qjs.Code("export default await Promise.resolve('async module')"),
			qjs.TypeModule(),
			qjs.FlagStrict())
		assert.NoError(t, err)
		assert.Equal(t, "async module", val.String())
		val.Free()
	})

	t.Run("ExplicitTypeGlobal", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		val, err := runtime.Eval("test.js",
			qjs.Code("var x = 100; x;"),
			qjs.TypeGlobal())
		assert.NoError(t, err)
		assert.Equal(t, int32(100), val.Int32())
		val.Free()
	})

	t.Run("BytecodeWithModuleAndAsync", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		bytecode, err := runtime.Compile(
			"test.js",
			qjs.Code("export default await Promise.resolve(42)"),
			qjs.TypeModule())
		assert.NoError(t, err)
		assert.NotEmpty(t, bytecode)

		val, err := runtime.Eval("test.js",
			qjs.Bytecode(bytecode),
			qjs.TypeModule())
		assert.NoError(t, err)
		assert.Equal(t, int32(42), val.Int32())
		val.Free()
	})

	t.Run("ModuleWithStrictMode", func(t *testing.T) {
		runtime := must(qjs.New())
		defer runtime.Close()

		val, err := runtime.Eval("test.js",
			qjs.Code("export default 'strict module'"),
			qjs.TypeModule(),
			qjs.FlagStrict())
		assert.NoError(t, err)
		assert.Equal(t, "strict module", val.String())
		val.Free()

		_, err = runtime.Eval("test.js",
			qjs.Code("export default (function() { with({}) { return 'test'; } })()"),
			qjs.TypeModule(),
			qjs.FlagStrict())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "SyntaxError")
	})
}

func TestInternalOptions(t *testing.T) {
	runtime := must(qjs.New())
	defer runtime.Close()

	_, err := runtime.Compile("test.js",
		qjs.Code("42"),
		qjs.TypeDirect())
	assert.NoError(t, err)

	_, err = runtime.Compile("test.js",
		qjs.Code("42"),
		qjs.TypeIndirect())
	assert.NoError(t, err)

	_, err = runtime.Compile("test.js",
		qjs.Code("42"),
		qjs.TypeMask())
	assert.NoError(t, err)

	_, err = runtime.Compile("test.js",
		qjs.Code("42"),
		qjs.FlagUnused())
	assert.NoError(t, err)
}
