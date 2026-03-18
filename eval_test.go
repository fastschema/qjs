package qjs_test

import (
	"path"
	"testing"
	"testing/fstest"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
)

type evalTest struct {
	name        string
	file        string
	code        string
	options     []qjs.EvalOptionFunc
	prepare     func(*qjs.Runtime)
	useByteCode bool
	wantError   bool
	expectErr   func(*testing.T, error)
	expectValue func(*testing.T, *qjs.Value, error)
}

func testGlobalModeEvaluation(t *testing.T) {
	t.Run("Basic_Scripts", func(t *testing.T) {
		tests := []evalTest{
			{name: "no_result_statement", file: "global_no_result.js", code: "const a = 55555", expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.True(t, val.IsUndefined()) }},
			{name: "expression_result", file: "global_has_result.js", code: "55555", expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, int32(55555), val.Int32()) }},
			{name: "syntax_error", file: "syntax_error.js", code: "const a = (", expectErr: func(t *testing.T, err error) { assert.Error(t, err); assert.Contains(t, err.Error(), "SyntaxError") }},
			{name: "runtime_error", file: "throw_error.js", code: "throw new Error('test error')", expectErr: func(t *testing.T, err error) { assert.Error(t, err); assert.Contains(t, err.Error(), "test error") }},
		}
		runEvalTests(t, tests, "")
	})

	t.Run("Top_Level_Await", func(t *testing.T) {
		tests := []evalTest{
			{name: "top_level_await_error_without_flag", file: "global_top_level_await_error.js", code: "await Promise.resolve(42)", expectErr: func(t *testing.T, err error) { assert.Error(t, err); assert.Contains(t, err.Error(), "SyntaxError") }},
			{name: "top_level_await_works_with_flag", file: "global_top_level_await_works.js", code: "await Promise.resolve(42)", options: []qjs.EvalOptionFunc{qjs.FlagAsync()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, int32(42), val.Int32()) }},
		}
		runEvalTests(t, tests, "")
	})

	t.Run("From_Test_Files", func(t *testing.T) {
		globalTests := genModeGlobalTests(t)
		runModeGlobalTests(t, globalTests, false) // File mode
		runModeGlobalTests(t, globalTests, true)  // Script mode
	})
}

func testModuleModeEvaluation(t *testing.T) {
	t.Run("Basic_Modules", func(t *testing.T) {
		tests := []evalTest{
			{name: "no_result_statement", file: "module_no_result.js", code: "const a = 55555", options: []qjs.EvalOptionFunc{qjs.TypeModule()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.True(t, val.IsUndefined()) }},
			{name: "no_default_export", file: "module_no_default_export.js", code: "export const a = 55555", options: []qjs.EvalOptionFunc{qjs.TypeModule()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.True(t, val.IsUndefined()) }},
			{name: "has_default_export", file: "module_has_result.js", code: "export default 55555", options: []qjs.EvalOptionFunc{qjs.TypeModule()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, int32(55555), val.Int32()) }},
		}
		runEvalTests(t, tests, "")
	})

	t.Run("Top_Level_Await", func(t *testing.T) {
		tests := []evalTest{
			{name: "module_top_level_await", file: "module_top_level_await.js", code: `async function main() { const res = await Promise.resolve(42); return res; } export default await main();`, options: []qjs.EvalOptionFunc{qjs.TypeModule()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, int32(42), val.Int32()) }},
		}
		runEvalTests(t, tests, "")
	})

	t.Run("From_Test_Files", func(t *testing.T) {
		moduleTests := genModeModuleTests(t)
		runModeModuleTests(t, moduleTests, false) // File mode
		runModeModuleTests(t, moduleTests, true)  // Script mode
	})
}

func testLoadOperations(t *testing.T) {
	tests := []evalTest{
		{
			name:      "empty_filename_error",
			file:      "",
			wantError: true,
			expectErr: func(t *testing.T, err error) {
				assert.Equal(t, qjs.ErrInvalidFileName, err)
			},
		},
		{
			name: "load_module_code",
			file: "01_load_module_code.js",
			code: "export const moduleExported = 55555",
			expectValue: func(t *testing.T, val *qjs.Value, err error) {
				assert.Equal(t, int32(55555), val.Int32())
			},
		},
		{
			name: "load_module_file",
			file: "02_load_module_file.js",
			expectValue: func(t *testing.T, val *qjs.Value, err error) {
				assert.Equal(t, "exported from module file", val.String())
			},
		},
		{
			name:        "load_module_bytecode",
			file:        "03_load_module_bytecode.js",
			code:        "export const moduleExported = 55555",
			useByteCode: true,
			expectValue: func(t *testing.T, val *qjs.Value, err error) {
				assert.Equal(t, int32(55555), val.Int32())
			},
		},
		{
			name: "mixed_loading_scenario",
			file: "03_mixed_load.js",
			prepare: func(rt *qjs.Runtime) {
				_ = must(rt.Load("mod_a.js", qjs.Code("export const getA = () => 'A';")))
				_ = must(rt.Load("./testdata/04_load/03_mixed/mod_b"))
				_ = must(rt.Load("./testdata/04_load/03_mixed/mod_c"))
				bytesD := must(rt.Compile("mod_d.js", qjs.Code("export const getD = () => 'D';"), qjs.TypeModule()))
				_ = must(rt.Load("mod_d.js", qjs.Bytecode(bytesD)))
				bytesE := must(rt.Compile("./testdata/04_load/03_mixed/mod_e", qjs.TypeModule()))
				_ = must(rt.Load("./testdata/04_load/03_mixed/mod_e", qjs.Bytecode(bytesE)))
			},
			code: `import { getA } from 'mod_a.js'; import { getB } from './03_mixed/mod_b'; import { getC } from './03_mixed/mod_c'; import { getD } from 'mod_d.js'; import { getE } from './03_mixed/mod_e'; export const moduleExported = getA() + getB() + getC() + getD() + getE();`,
			expectValue: func(t *testing.T, val *qjs.Value, err error) {
				assert.Equal(t, "ABCDE", val.String())
			},
		},
	}

	runLoadTests(t, tests, "./testdata/04_load")
}

func runEvalTests(t *testing.T, tests []evalTest, basePath string) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			if test.prepare != nil {
				test.prepare(rt)
			}

			testFile := test.file
			if basePath != "" && testFile != "" {
				testFile = path.Join(basePath, test.file)
			}

			var options []qjs.EvalOptionFunc
			if test.code != "" {
				if test.useByteCode {
					bytecode := must(rt.Compile(testFile, qjs.Code(test.code), qjs.TypeModule()))
					options = append(options, qjs.Bytecode(bytecode))
				} else {
					options = append(options, qjs.Code(test.code))
				}
			}
			options = append(options, test.options...)

			val, err := rt.Eval(testFile, options...)
			if val != nil {
				defer val.Free()
			}

			if test.wantError {
				assert.Error(t, err)
				if test.expectErr != nil {
					test.expectErr(t, err)
				}
				return
			}

			if test.expectErr != nil {
				test.expectErr(t, err)
				return
			}

			assert.NoError(t, err)
			if test.expectValue != nil {
				test.expectValue(t, val, err)
			}
		})
	}
}

func runCompileTests(t *testing.T, tests []evalTest, basePath string) {
	t.Run("compile_error", func(t *testing.T) {
		rt := must(qjs.New())
		defer rt.Close()

		_, err := rt.Compile("compile_error.js", qjs.Code("const a = ("))
		assert.Error(t, err)
	})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			testFile := path.Join(basePath, test.file)

			// Compile phase
			var compileOptions []qjs.EvalOptionFunc
			if test.code != "" {
				compileOptions = append(compileOptions, qjs.Code(test.code))
			}
			compileOptions = append(compileOptions, test.options...)

			bytecode, err := rt.Compile(testFile, compileOptions...)
			assert.NoError(t, err)
			assert.NotEmpty(t, bytecode)

			// Execute phase
			var evalOptions []qjs.EvalOptionFunc
			evalOptions = append(evalOptions, qjs.Bytecode(bytecode))
			evalOptions = append(evalOptions, test.options...)

			val, err := rt.Eval(testFile, evalOptions...)
			defer val.Free()
			assert.NoError(t, err)
			if test.expectValue != nil {
				test.expectValue(t, val, err)
			}
		})
	}
}

func runLoadTests(t *testing.T, tests []evalTest, basePath string) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			if test.prepare != nil {
				test.prepare(rt)
			}

			testFile := test.file
			if testFile != "" {
				testFile = path.Join(basePath, test.file)
			}

			var options []qjs.EvalOptionFunc
			if test.code != "" {
				if test.useByteCode {
					bytecode := must(rt.Compile(testFile, qjs.TypeModule(), qjs.Code(test.code)))
					options = append(options, qjs.Bytecode(bytecode))
				} else {
					options = append(options, qjs.Code(test.code))
				}
			}

			loadVal, err := rt.Load(testFile, options...)
			if test.expectErr != nil {
				test.expectErr(t, err)
				return
			}

			if test.wantError {
				assert.Error(t, err)
				return
			}

			loadVal.Free()

			// Test by importing the loaded module
			val, err := rt.Eval("import_"+test.name, qjs.Code(`import { moduleExported } from '`+testFile+`'; export default moduleExported`), qjs.TypeModule())
			defer val.Free()
			assert.NoError(t, err)
			if test.expectValue != nil {
				test.expectValue(t, val, err)
			}
		})
	}
}

func TestEval(t *testing.T) {
	t.Run("Invalid_Inputs", func(t *testing.T) {
		tests := []evalTest{
			{
				name:      "empty_filename",
				file:      "",
				wantError: true,
				expectErr: func(t *testing.T, err error) {
					assert.Equal(t, qjs.ErrInvalidFileName, err)
				},
			},
			{
				name:      "empty_filename",
				file:      "",
				wantError: true,
				expectErr: func(t *testing.T, err error) {
					assert.Equal(t, qjs.ErrInvalidFileName, err)
				},
			},
		}
		runEvalTests(t, tests, "")
	})

	t.Run("Invalid_Inputs", func(t *testing.T) {
		tests := []evalTest{
			{name: "empty_filename", file: "", wantError: true, expectErr: func(t *testing.T, err error) { assert.Equal(t, qjs.ErrInvalidFileName, err) }},
		}
		runEvalTests(t, tests, "")
	})

	t.Run("Load_Operations", func(t *testing.T) {
		testLoadOperations(t)
	})

	t.Run("Error_Normalization", func(t *testing.T) {
		tests := []evalTest{
			{name: "throw_error_normalization", file: "error_test.js", code: `throw new Error("ThrowError test")`, expectErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "ThrowError test")
			}},
			{name: "is_error_path_normalization", file: "error_test.js", code: `new Error("IsError test")`, expectErr: func(t *testing.T, err error) { assert.Error(t, err); assert.Contains(t, err.Error(), "IsError test") }},
		}
		runEvalTests(t, tests, "")
	})
}

// Module Loading Tests
func TestModuleLoading(t *testing.T) {
	t.Run("File_Resolution", func(t *testing.T) {
		tests := []evalTest{
			{
				name:    "file_not_found",
				file:    "not_found.js",
				options: []qjs.EvalOptionFunc{qjs.TypeModule()},
				expectErr: func(t *testing.T, err error) {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), "No such file or directory: testdata/00_loader/not_found.js")
				},
			},
			{
				name:    "real_filename",
				file:    "01_realname/file.js",
				options: []qjs.EvalOptionFunc{qjs.TypeModule()},
				expectValue: func(t *testing.T, val *qjs.Value, err error) {
					assert.Equal(t, "Hello, World!", val.String())
				},
			},
		}
		runEvalTests(t, tests, "./testdata/00_loader")
	})

	t.Run("Auto_Extensions", func(t *testing.T) {
		tests := []evalTest{
			{
				name:    "auto_index_js",
				file:    "02_autoindexjs",
				options: []qjs.EvalOptionFunc{qjs.TypeModule()},
				expectValue: func(t *testing.T, val *qjs.Value, err error) {
					assert.Equal(t, "autoindexjs", val.String())
				},
			},
			{
				name:    "auto_index_mjs",
				file:    "03_autoindexmjs",
				options: []qjs.EvalOptionFunc{qjs.TypeModule()},
				expectValue: func(t *testing.T, val *qjs.Value, err error) {
					assert.Equal(t, "autoindexmjs", val.String())
				},
			},
			{
				name:    "auto_ext_js",
				file:    "04_autoextjs/file",
				options: []qjs.EvalOptionFunc{qjs.TypeModule()},
				expectValue: func(t *testing.T, val *qjs.Value, err error) {
					assert.Equal(t, "file.js", val.String())
				},
			},
			{
				name:    "auto_ext_mjs",
				file:    "05_autoextmjs/file",
				options: []qjs.EvalOptionFunc{qjs.TypeModule()},
				expectValue: func(t *testing.T, val *qjs.Value, err error) {
					assert.Equal(t, "file.mjs", val.String())
				},
			},
		}
		runEvalTests(t, tests, "./testdata/00_loader")
	})

	t.Run("FS_Module_Imports", func(t *testing.T) {
		rt := must(qjs.New(qjs.Option{
			FS: fstest.MapFS{
				"math-utils.js": &fstest.MapFile{
					Data: []byte("export function add(a, b) { return a + b; }"),
				},
				"main.js": &fstest.MapFile{
					Data: []byte("import { add } from 'math-utils.js'; export default String(add(5, 5));"),
				},
			},
		}))
		defer rt.Close()

		val, err := rt.Eval("main.js", qjs.TypeModule())
		defer val.Free()
		assert.NoError(t, err)
		assert.Equal(t, "10", val.String())
	})
}

// Compilation Tests
func TestCompilation(t *testing.T) {
	t.Run("Bytecode_Generation", func(t *testing.T) {
		tests := []evalTest{
			{name: "compile_global_code", file: "compile_global_code.js", code: "55555", expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, int32(55555), val.Int32()) }},
			{name: "compile_global_file", file: "01_global/index.js", expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, "Hello, World!", val.String()) }},
			{name: "compile_module_code", file: "compile_module_code.js", code: "export default 55555", options: []qjs.EvalOptionFunc{qjs.TypeModule()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, int32(55555), val.Int32()) }},
			{name: "compile_module_file", file: "02_module/index.js", options: []qjs.EvalOptionFunc{qjs.TypeModule()}, expectValue: func(t *testing.T, val *qjs.Value, err error) { assert.Equal(t, "Hello from lib_a", val.String()) }},
		}

		runCompileTests(t, tests, "./testdata/03_compile")
	})
}

// Script Evaluation Tests
func TestScriptEvaluation(t *testing.T) {
	t.Run("Global_Mode", func(t *testing.T) {
		testGlobalModeEvaluation(t)
	})

	t.Run("Module_Mode", func(t *testing.T) {
		testModuleModeEvaluation(t)
	})
}
