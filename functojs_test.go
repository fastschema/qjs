package qjs_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Calculator struct {
	Name string
}

func (c Calculator) Add(a, b int) int {
	return a + b
}

func (c Calculator) Multiply(a, b float64) (float64, error) {
	return a * b, nil
}

func (c Calculator) Sum(nums ...int) int {
	total := 0
	for _, num := range nums {
		total += num
	}
	return total
}

func (c Calculator) GetName() string {
	return c.Name
}

func (c *Calculator) SetName(name string) error {
	if name == "" {
		return errors.New("name cannot be empty")
	}
	c.Name = name
	return nil
}

func createAndRegisterJSFunc(t *testing.T, ctx *qjs.Context, name string, fn any) *qjs.Value {
	jsFunc := must(qjs.FuncToJS(ctx, fn))
	assert.True(t, jsFunc.IsFunction())
	ctx.Global().SetPropertyStr(name, jsFunc)
	return jsFunc
}

func TestBasicConversion(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("InvalidTypes", func(t *testing.T) {
		_, err := qjs.FuncToJS(ctx, "not a function")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected GO type function")
	})

	t.Run("FunctionTypes", func(t *testing.T) {
		// Simple function
		jsFunc := createAndRegisterJSFunc(t, ctx, "add", func(a, b int) int { return a + b })
		defer jsFunc.Free()
		result := must(ctx.Eval("test.js", qjs.Code("add(5, 3)")))
		defer result.Free()
		assert.Equal(t, int32(8), result.Int32())

		// Function with error
		jsFunc2 := createAndRegisterJSFunc(t, ctx, "divide", func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, errors.New("division by zero")
			}
			return a / b, nil
		})
		defer jsFunc2.Free()
		result2 := must(ctx.Eval("test.js", qjs.Code("divide(10, 2)")))
		defer result2.Free()
		assert.InEpsilon(t, float64(5), result2.Float64(), 0.0001)

		_, err := ctx.Eval("test.js", qjs.Code("divide(10, 0)"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "division by zero")

		// Error-only return
		jsFunc3 := createAndRegisterJSFunc(t, ctx, "errorOnly", func(shouldError bool) error {
			if shouldError {
				return errors.New("test error")
			}
			return nil
		})
		defer jsFunc3.Free()
		result3 := must(ctx.Eval("test.js", qjs.Code("errorOnly(false)")))
		defer result3.Free()
		assert.True(t, result3.IsUndefined())

		// No return value
		jsFunc4 := createAndRegisterJSFunc(t, ctx, "print", func(msg string) { fmt.Printf("Message: %s\n", msg) })
		defer jsFunc4.Free()
		result4 := must(ctx.Eval("test.js", qjs.Code(`print("hello")`)))
		defer result4.Free()
		assert.True(t, result4.IsUndefined())
	})

	t.Run("NilHandling", func(t *testing.T) {
		// Nil value
		result := must(qjs.FuncToJS(ctx, nil))
		defer result.Free()
		assert.True(t, result.IsNull())

		// Nil function
		var nilFunc func()
		jsFunc := must(qjs.FuncToJS(ctx, nilFunc))
		defer jsFunc.Free()
		assert.True(t, jsFunc.IsNull())

		// Nil function pointer
		var nilFuncPtr *func()
		jsNilPtr := must(qjs.FuncToJS(ctx, nilFuncPtr))
		defer jsNilPtr.Free()
		assert.True(t, jsNilPtr.IsNull())
	})

	t.Run("FunctionPointer", func(t *testing.T) {
		addFunc := func(a, b int) int { return a + b }
		funcPtr := &addFunc
		jsFunc := createAndRegisterJSFunc(t, ctx, "funcPtr", funcPtr)
		defer jsFunc.Free()
		result := must(ctx.Eval("test.js", qjs.Code("funcPtr(7, 8)")))
		defer result.Free()
		assert.Equal(t, int32(15), result.Int32())
	})
}

func TestParameters(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("ParametersCountMismatch", func(t *testing.T) {
		createAndRegisterJSFunc(t, ctx, "manyParamFunc", func(a, b, c, d, e int) int {
			return a + b + c + d + e
		})

		result := must(ctx.Eval("test.js", qjs.Code(`manyParamFunc(1, 2, 3, 4, 5)`)))
		defer result.Free()
		assert.Equal(t, int32(15), result.Int32())

		createAndRegisterJSFunc(t, ctx, "paramFunc", func(a, b, c int) int {
			return a + b + c
		})

		result2 := must(ctx.Eval("test.js", qjs.Code(`paramFunc(10)`)))
		defer result2.Free()
		assert.Equal(t, int32(10), result2.Int32())

		createAndRegisterJSFunc(t, ctx, "paramFunc2", func(a int) int {
			return a * 2
		})

		result3 := must(ctx.Eval("test.js", qjs.Code(`paramFunc2(5, 10, 15)`)))
		defer result3.Free()
		assert.Equal(t, int32(10), result3.Int32())
	})

	t.Run("InvalidParameterTypes", func(t *testing.T) {
		goFunc, err := qjs.FuncToJS(ctx, func(x int, y ...int) int {
			return len(y) + 1
		})
		assert.NoError(t, err)
		defer goFunc.Free()
		ctx.Global().SetPropertyStr("testFunc", goFunc)
		_, err = ctx.Eval("test.js", qjs.Code(`testFunc("invalid", 2)`))
		require.Error(t, err)
	})

	t.Run("BasicParameterTypes", func(t *testing.T) {
		_ = createAndRegisterJSFunc(t, ctx, "basicTypesFunc", func(s string, i int, f float64, b bool) string {
			return fmt.Sprintf("string:%s,int:%d,float:%.1f,bool:%t", s, i, f, b)
		})

		result := must(ctx.Eval("test.js", qjs.Code(`basicTypesFunc("test", 42, 3.14, true)`)))
		defer result.Free()
		assert.Equal(t, "string:test,int:42,float:3.1,bool:true", result.String())
	})

	t.Run("ComplexParameterTypes", func(t *testing.T) {
		t.Run("MapParameters", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "processData", func(data map[string]any) (string, error) {
				if len(data) == 0 {
					return "", errors.New("empty data")
				}
				return fmt.Sprintf("processed %d items, value=%v", len(data), data["value"]), nil
			})

			result := must(ctx.Eval("test.js", qjs.Code(`processData({name: "test", value: 42})`)))
			defer result.Free()
			assert.Equal(t, "processed 2 items, value=42", result.String())

			_, err := ctx.Eval("test.js", qjs.Code(`processData({})`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "empty data")
		})

		t.Run("ArrayAndSliceParameters", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "arrayFunc", func(arr [3]int) string {
				return fmt.Sprintf("array: %v", arr)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`arrayFunc([1, 2, 3])`)))
			defer result.Free()
			assert.Contains(t, result.String(), "array:")
		})
	})

	t.Run("PointerParameters", func(t *testing.T) {
		t.Run("BasicPointerHandling", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "pointerFunc", func(ptr *int) string {
				if ptr == nil {
					return "nil"
				}
				return fmt.Sprintf("value: %d", *ptr)
			})

			result1 := must(ctx.Eval("test.js", qjs.Code(`pointerFunc(42)`)))
			defer result1.Free()
			assert.Equal(t, "value: 42", result1.String())

			result2 := must(ctx.Eval("test.js", qjs.Code(`pointerFunc(null)`)))
			defer result2.Free()
			assert.Equal(t, "nil", result2.String())

			result3 := must(ctx.Eval("test.js", qjs.Code(`pointerFunc(undefined)`)))
			defer result3.Free()
			assert.Equal(t, "nil", result3.String())
		})

		t.Run("VariousPointerTypes", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "ptrStringFunc", func(ptr *string) string {
				if ptr == nil {
					return "nil"
				}
				return fmt.Sprintf("value: %s", *ptr)
			})

			result4 := must(ctx.Eval("test.js", qjs.Code(`ptrStringFunc("hello")`)))
			defer result4.Free()
			assert.Equal(t, "value: hello", result4.String())

			_ = createAndRegisterJSFunc(t, ctx, "ptrParamFunc", func(ptr *float64) string {
				if ptr == nil {
					return "nil"
				}
				return fmt.Sprintf("value: %.2f", *ptr)
			})

			result5 := must(ctx.Eval("test.js", qjs.Code(`ptrParamFunc(3.14159)`)))
			defer result5.Free()
			assert.Equal(t, "value: 3.14", result5.String())
		})

		t.Run("PointerConversionErrors", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "ptrFunc", func(ptr *string) string {
				if ptr == nil {
					return "nil"
				}
				return *ptr
			})

			_, err := ctx.Eval("test.js", qjs.Code(`ptrFunc(new Date())`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert JS function argument")
		})
	})

	t.Run("InterfaceParameters", func(t *testing.T) {
		t.Run("BasicInterfaceHandling", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "interfaceFunc", func(value any) string {
				return fmt.Sprintf("%T: %v", value, value)
			})

			tests := []struct {
				input    string
				contains string
			}{
				{`interfaceFunc(42)`, "42"},
				{`interfaceFunc("hello")`, "hello"},
				{`interfaceFunc(true)`, "true"},
				{`interfaceFunc({a: 1, b: "test"})`, "map"},
			}

			for _, test := range tests {
				result := must(ctx.Eval("test.js", qjs.Code(test.input)))
				defer result.Free()
				assert.Contains(t, result.String(), test.contains)
			}
		})

		t.Run("InterfaceTypeDetection", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "anyFunc", func(value any) string {
				return fmt.Sprintf("%T: %v", value, value)
			})

			// Test whole number conversion to int
			result := must(ctx.Eval("test.js", qjs.Code(`anyFunc(42)`)))
			defer result.Free()
			assert.Contains(t, result.String(), "int64: 42")

			// Test fractional number conversion to float64
			result2 := must(ctx.Eval("test.js", qjs.Code(`anyFunc(3.14)`)))
			defer result2.Free()
			assert.Contains(t, result2.String(), "float64")
			assert.Contains(t, result2.String(), "3.14")
		})
	})

	t.Run("FunctionParameters", func(t *testing.T) {
		t.Run("CallbackFunctions", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "higherOrderFunc", func(callback func(int) (int, error), value int) (int, error) {
				return callback(value)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`higherOrderFunc(x => x * 2, 21)`)))
			defer result.Free()
			assert.Equal(t, int32(42), result.Int32())

			_, err := ctx.Eval("test.js", qjs.Code(`higherOrderFunc(x => { throw new Error("callback error"); }, 10)`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "callback error")
		})

		t.Run("FunctionConversionErrors", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "callbackTest", func(callback func() error) error {
				return callback()
			})

			_, err2 := ctx.Eval("test.js", qjs.Code(`callbackTest(42)`))
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert JS function argument")
		})
	})

	t.Run("VariadicParameters", func(t *testing.T) {
		t.Run("BasicVariadicFunction", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "sumFunc", func(nums ...int) int {
				total := 0
				for _, num := range nums {
					total += num
				}
				return total
			})

			result1 := must(ctx.Eval("test.js", qjs.Code("sumFunc(1, 2, 3, 4, 5)")))
			defer result1.Free()
			assert.Equal(t, int32(15), result1.Int32())

			result2 := must(ctx.Eval("test.js", qjs.Code("sumFunc()")))
			defer result2.Free()
			assert.Equal(t, int32(0), result2.Int32())

			result3 := must(ctx.Eval("test.js", qjs.Code("sumFunc(42)")))
			defer result3.Free()
			assert.Equal(t, int32(42), result3.Int32())
		})

		t.Run("MixedVariadicFunction", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "partialSumFunc", func(prefix string, nums ...int) string {
				total := 0
				for _, num := range nums {
					total += num
				}
				return fmt.Sprintf("%s: %d", prefix, total)
			})

			result1 := must(ctx.Eval("test.js", qjs.Code(`partialSumFunc("Total", 1, 2, 3)`)))
			defer result1.Free()
			assert.Equal(t, "Total: 6", result1.String())

			result2 := must(ctx.Eval("test.js", qjs.Code(`partialSumFunc("Total")`)))
			defer result2.Free()
			assert.Equal(t, "Total: 0", result2.String())
		})

		t.Run("VariadicWithAnyType", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "formatFunc", func(format string, args ...any) string {
				return fmt.Sprintf(format, args...)
			})

			result := must(ctx.Eval("test.js", qjs.Code(`formatFunc("Hello %s, you are %d years old", "John", 25)`)))
			defer result.Free()
			assert.Equal(t, "Hello John, you are 25 years old", result.String())
		})

		t.Run("VariadicConversionErrors", func(t *testing.T) {
			_ = createAndRegisterJSFunc(t, ctx, "variadicIntFunc", func(nums ...int8) string {
				total := int8(0)
				for _, num := range nums {
					total += num
				}
				return fmt.Sprintf("total: %d", total)
			})

			_, err := ctx.Eval("test.js", qjs.Code(`variadicIntFunc(1, 2, 1000)`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert JS function argument")
		})
	})
}

func TestReturnValues(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("BasicReturnTypes", func(t *testing.T) {
		t.Run("SingleReturn", func(t *testing.T) {
			jsFunc := createAndRegisterJSFunc(t, ctx, "nilFunc", func() any {
				return nil
			})
			defer jsFunc.Free()
			result := must(ctx.Eval("test.js", qjs.Code(`nilFunc()`)))
			defer result.Free()
			assert.True(t, result.IsNull())
		})

		t.Run("FunctionReturn", func(t *testing.T) {
			jsFunc := createAndRegisterJSFunc(t, ctx, "returningFunc", func() func(int) int {
				return func(x int) int {
					return x * 3
				}
			})
			defer jsFunc.Free()
			result := must(ctx.Eval("test.js", qjs.Code(`const func = returningFunc(); func(4)`)))
			defer result.Free()
			assert.Equal(t, int32(12), result.Int32())
		})
	})

	t.Run("SingleReturnWithNilError", func(t *testing.T) {
		jsFunc := createAndRegisterJSFunc(t, ctx, "singleReturnNilError", func() (string, error) {
			return "single", nil
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`singleReturnNilError()`)))
		defer result.Free()
		assert.Equal(t, "single", result.String())
	})

	t.Run("SingleReturnWithNonNilError", func(t *testing.T) {
		jsFunc := createAndRegisterJSFunc(t, ctx, "onlyNilError", func() error {
			return nil
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`onlyNilError()`)))
		defer result.Free()
		assert.True(t, result.IsUndefined())
	})

	t.Run("TwoReturnsWithNilError", func(t *testing.T) {
		jsFunc := createAndRegisterJSFunc(t, ctx, "twoReturnsNilError", func() (string, int, error) {
			return "success", 100, nil
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`twoReturnsNilError()`)))
		defer result.Free()
		assert.True(t, result.IsArray())

		elem0 := result.GetPropertyIndex(0)
		defer elem0.Free()
		assert.Equal(t, "success", elem0.String())

		elem1 := result.GetPropertyIndex(1)
		defer elem1.Free()
		assert.Equal(t, int32(100), elem1.Int32())
	})

	t.Run("MultipleNonErrorReturns", func(t *testing.T) {
		jsFunc := createAndRegisterJSFunc(t, ctx, "threeReturns", func() (string, int, bool) {
			return "test", 123, true
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`threeReturns()`)))
		defer result.Free()
		assert.True(t, result.IsArray())

		elem0 := result.GetPropertyIndex(0)
		defer elem0.Free()
		assert.Equal(t, "test", elem0.String())

		elem1 := result.GetPropertyIndex(1)
		defer elem1.Free()
		assert.Equal(t, int32(123), elem1.Int32())

		elem2 := result.GetPropertyIndex(2)
		defer elem2.Free()
		assert.Equal(t, true, elem2.Bool())
	})

	t.Run("FiveReturnsWithError", func(t *testing.T) {
		jsFunc := createAndRegisterJSFunc(t, ctx, "fiveReturns", func() (int, string, bool, float64, error) {
			return 1, "two", true, 4.5, nil
		})
		defer jsFunc.Free()

		result := must(ctx.Eval("test.js", qjs.Code(`fiveReturns()`)))
		defer result.Free()
		assert.True(t, result.IsArray())

		// Should return array with 4 elements (error is popped)
		arrayLen := result.GetPropertyStr("length")
		defer arrayLen.Free()
		assert.Equal(t, int32(4), arrayLen.Int32())

		elem0 := result.GetPropertyIndex(0)
		defer elem0.Free()
		assert.Equal(t, int32(1), elem0.Int32())

		elem1 := result.GetPropertyIndex(1)
		defer elem1.Free()
		assert.Equal(t, "two", elem1.String())

		elem2 := result.GetPropertyIndex(2)
		defer elem2.Free()
		assert.Equal(t, true, elem2.Bool())

		elem3 := result.GetPropertyIndex(3)
		defer elem3.Free()
		assert.Equal(t, 4.5, elem3.Float64())
	})

	t.Run("UnsupportedReturnTypes", func(t *testing.T) {
		t.Run("UnsafePointerReturnTypes", func(t *testing.T) {
			_, err4 := qjs.FuncToJS(ctx, func() *unsafe.Pointer {
				return nil
			})
			require.Error(t, err4)
			assert.Contains(t, err4.Error(), "cannot convert Go func return 'unsafe.Pointer")

			_, err5 := qjs.FuncToJS(ctx, func() **unsafe.Pointer {
				return nil
			})
			require.Error(t, err5)
			assert.Contains(t, err5.Error(), "cannot convert Go func return 'unsafe.Pointer")
		})

		t.Run("ArrayWithUnsupportedElements", func(t *testing.T) {
			_, err3 := qjs.FuncToJS(ctx, func() [2]unsafe.Pointer {
				return [2]unsafe.Pointer{}
			})
			require.Error(t, err3)
			assert.Contains(t, err3.Error(), "cannot convert Go func return 'array: unsafe.Pointer' to JS")
		})

		t.Run("StructWithUnsupportedFields", func(t *testing.T) {
			type StructWithUnsafePtr struct {
				Name string
				Ptr  unsafe.Pointer
			}
			_, err2 := qjs.FuncToJS(ctx, func() StructWithUnsafePtr {
				return StructWithUnsafePtr{Name: "test", Ptr: unsafe.Pointer(&[]byte{1}[0])}
			})
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert Go 'qjs_test.StructWithUnsafePtr.Ptr' to JS")
			assert.Contains(t, err2.Error(), "cannot convert Go func return 'unsafe.Pointer")

			// Nested validation
			type StructWithNestedUnsafePtr struct {
				Field struct {
					PtrField unsafe.Pointer
				}
			}
			_, err5 := qjs.FuncToJS(ctx, func() StructWithNestedUnsafePtr {
				return StructWithNestedUnsafePtr{}
			})
			require.Error(t, err5)
			assert.Contains(t, err5.Error(), "cannot convert Go 'qjs_test.StructWithNestedUnsafePtr.Field' to JS")
			assert.Contains(t, err5.Error(), "cannot convert Go func return 'unsafe.Pointer")
		})

		t.Run("ComplexNestedValidation", func(t *testing.T) {
			_, err := qjs.FuncToJS(ctx, func() map[string]unsafe.Pointer {
				return nil
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot convert Go func return 'map value: unsafe.Pointer' to JS")

			type StructWithUnsafeField struct {
				Field unsafe.Pointer
			}
			_, err2 := qjs.FuncToJS(ctx, func() StructWithUnsafeField {
				return StructWithUnsafeField{}
			})
			require.Error(t, err2)
			assert.Contains(t, err2.Error(), "cannot convert Go 'qjs_test.StructWithUnsafeField.Field' to JS")
			assert.Contains(t, err2.Error(), "cannot convert Go func return 'unsafe.Pointer'")
		})
	})
}

func TestMethodBinding(t *testing.T) {
	_, ctx := setupTestContext(t)

	t.Run("StructMethodBinding", func(t *testing.T) {
		t.Run("ValueReceiverMethods", func(t *testing.T) {
			calc := Calculator{Name: "MyCalculator"}

			jsAddMethod := createAndRegisterJSFunc(t, ctx, "calcAdd", calc.Add)
			defer jsAddMethod.Free()
			result := must(ctx.Eval("test.js", qjs.Code("calcAdd(10, 20)")))
			defer result.Free()
			assert.Equal(t, int32(30), result.Int32())

			jsMultiplyMethod := createAndRegisterJSFunc(t, ctx, "calcMultiply", calc.Multiply)
			defer jsMultiplyMethod.Free()
			result2 := must(ctx.Eval("test.js", qjs.Code("calcMultiply(3.5, 2.0)")))
			defer result2.Free()
			assert.Equal(t, float64(7), result2.Float64())

			jsSumMethod := createAndRegisterJSFunc(t, ctx, "calcSum", calc.Sum)
			defer jsSumMethod.Free()
			result3 := must(ctx.Eval("test.js", qjs.Code("calcSum(1, 2, 3)")))
			defer result3.Free()
			assert.Equal(t, int32(6), result3.Int32())
		})

		t.Run("PointerReceiverMethods", func(t *testing.T) {
			calc := &Calculator{Name: "TestCalc"}

			jsSetNameMethod := createAndRegisterJSFunc(t, ctx, "setName", calc.SetName)
			defer jsSetNameMethod.Free()

			result := must(ctx.Eval("test.js", qjs.Code(`setName("NewName")`)))
			defer result.Free()
			assert.True(t, result.IsUndefined() || result.IsNull())
			assert.Equal(t, "NewName", calc.Name)

			_, err := ctx.Eval("test.js", qjs.Code(`setName("")`))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "name cannot be empty")
		})
	})
}

func TestCreateNonNilSample(t *testing.T) {
	tests := []struct {
		name              string
		input             any
		expectedType      any
		shouldBeNotNil    bool
		testFuncExecution bool
	}{
		{
			name:           "Context type",
			input:          context.Background(),
			expectedType:   context.Background(),
			shouldBeNotNil: true,
		},
		{
			name:           "Pointer type",
			input:          (*int)(nil),
			expectedType:   (*int)(nil),
			shouldBeNotNil: false, // Pointer itself can be nil but sample won't be
		},
		{
			name:           "Array type",
			input:          [3]string{},
			expectedType:   [3]string{},
			shouldBeNotNil: true,
		},
		{
			name:           "Slice type",
			input:          []string{},
			expectedType:   []string{},
			shouldBeNotNil: true,
		},
		{
			name:           "Map type",
			input:          map[string]int{},
			expectedType:   map[string]int{},
			shouldBeNotNil: true,
		},
		{
			name:           "Chan type",
			input:          (chan int)(nil),
			expectedType:   (chan int)(nil),
			shouldBeNotNil: true,
		},
		{
			name:              "Func type",
			input:             (func(int) int)(nil),
			expectedType:      (func(int) int)(nil),
			shouldBeNotNil:    true,
			testFuncExecution: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sample := qjs.CreateNonNilSample(reflect.TypeOf(tt.input))

			assert.IsType(t, tt.expectedType, sample)

			if tt.shouldBeNotNil {
				assert.NotNil(t, sample)
			}

			// Special test for function execution
			if tt.testFuncExecution {
				dummyFunc := sample.(func(int) int)
				result := dummyFunc(42)
				assert.Equal(t, 0, result) // Should return zero value
			}
		})
	}
}

// TestVariadicFunctionWithNullUndefined is a regression test for the bug where
// JavaScript null/undefined values passed to Go variadic functions caused panics.
//
// Bug: JavaScript null/undefined convert to invalid reflect.Value, which caused
// "reflect: call of reflect.Value.Set on zero Value" panic in CreateVariadicSlice.
//
// Fix: Check for invalid reflect.Value before calling Set(), use reflect.Zero() instead.
func TestVariadicFunctionWithNullUndefined(t *testing.T) {
	runtime := must(qjs.New(qjs.Option{}))
	defer runtime.Close()
	ctx := runtime.Context()

	// Create a variadic function that logs all arguments
	logFunc := func(args ...any) string {
		result := fmt.Sprintf("received %d args", len(args))
		for i, arg := range args {
			result += fmt.Sprintf(", arg[%d]=%v (nil=%v)", i, arg, arg == nil)
		}
		return result
	}

	// Register function
	logValue := must(qjs.ToJsValue(ctx, map[string]any{
		"log": logFunc,
	}))
	ctx.Global().SetPropertyStr("api", logValue)

	tests := []struct {
		name     string
		code     string
		wantErr  bool
		contains string // Expected substring in result
	}{
		{
			name:     "null_only",
			code:     `api.log(null)`,
			wantErr:  false,
			contains: "received 1 args",
		},
		{
			name:     "undefined_only",
			code:     `api.log(undefined)`,
			wantErr:  false,
			contains: "received 1 args",
		},
		{
			name:     "null_and_undefined",
			code:     `api.log(null, undefined)`,
			wantErr:  false,
			contains: "received 2 args",
		},
		{
			name:     "mixed_with_strings",
			code:     `api.log("hello", null, "world", undefined)`,
			wantErr:  false,
			contains: "received 4 args",
		},
		{
			name:     "mixed_with_numbers",
			code:     `api.log(42, null, 3.14, undefined)`,
			wantErr:  false,
			contains: "received 4 args",
		},
		{
			name:     "only_undefined_multiple",
			code:     `api.log(undefined, undefined, undefined)`,
			wantErr:  false,
			contains: "received 3 args",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ctx.Eval("test.js", qjs.Code(tt.code))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Variadic function with null/undefined should not error")
			require.NotNil(t, result, "Result should not be nil")
			defer result.Free()

			// Verify result contains expected substring
			resultStr := result.String()
			assert.Contains(t, resultStr, tt.contains, "Result should contain expected substring")

			t.Logf("Test %s result: %s", tt.name, resultStr)
		})
	}
}
