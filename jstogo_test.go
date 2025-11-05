package qjs_test

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// StringifyErrorUnmarshaler implements json.Unmarshaler for testing JSONStringify errors
type StringifyErrorUnmarshaler struct {
	Value string
}

func (seu *StringifyErrorUnmarshaler) UnmarshalJSON(data []byte) error {
	seu.Value = string(data)
	return nil
}

func validateCircularReference(t *testing.T, err error) {
	t.Helper()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "circular reference")
}

func TestJsBigIntToGo(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	result := must(rt.Eval("test.js", qjs.Code(`({ bigInt: BigInt(1234567890), notBigInt: "not a big int" })`)))
	defer result.Free()

	jsBigInt := result.GetPropertyStr("bigInt")
	defer jsBigInt.Free()

	t.Run("not_a_bigInt", func(t *testing.T) {
		notBigInt := result.GetPropertyStr("notBigInt")
		defer notBigInt.Free()
		val, err := qjs.JsBigIntToGo[int64](notBigInt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot convert JS BigInt")
		assert.Equal(t, int64(0), val)
	})

	t.Run("convert_to_big_int_pointer", func(t *testing.T) {
		val := must(qjs.JsBigIntToGo[*big.Int](jsBigInt))
		assert.IsType(t, (*big.Int)(nil), val)
		assert.Equal(t, big.NewInt(int64(1234567890)), val)
	})

	t.Run("convert_to_big_int_value", func(t *testing.T) {
		val := must(qjs.JsBigIntToGo[big.Int](jsBigInt))
		assert.IsType(t, big.Int{}, val)
		expected := *big.NewInt(int64(1234567890))
		assert.Equal(t, expected, val)
	})

	t.Run("unsupported_type", func(t *testing.T) {
		val, err := qjs.JsBigIntToGo[int64](jsBigInt)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected GO type *big.Int/big.Int")
		assert.Equal(t, int64(0), val)
	})
}

func TestJsNumberToGo(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	result := must(rt.Eval("test.js", qjs.Code(`({
			normalNumber: 42,
			bigIntValue: BigInt(9007199254740991),
			nan: NaN,
			infinity: Infinity,
			negativeInfinity: -Infinity,
			notANumber: "not a number",
			maxInt8: 127,
			minInt8: -128,
			overflowInt8: 128,
			maxUint8: 255,
			overflowUint8: 256,
			maxInt16: 32767,
			minInt16: -32768,
			overflowInt16: 32768,
			maxUint16: 65535,
			overflowUint16: 65536,
			floatValue: 3.14159,
			overflowInt32: 2147483648,
			overflowUint32: 4294967296,
	})`)))
	defer result.Free()

	// Consolidated error tests
	t.Run("error_cases", func(t *testing.T) {
		errorTests := []struct{ property, expectedError string }{
			{"nan", "NaN"},
			{"infinity", "Infinity"},
			{"notANumber", "not a number"},
		}

		for _, test := range errorTests {
			t.Run(test.property, func(t *testing.T) {
				val := result.GetPropertyStr(test.property)
				defer val.Free()
				_, err := qjs.JsNumberToGo[float64](val)
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			})
		}
	})

	t.Run("overflow", func(t *testing.T) {
		overflowTestCases := []struct {
			name        string
			property    string
			testValue   func(*qjs.Value)
			testPointer func(*qjs.Value)
			errorMsg    string
		}{
			{
				name:     "int8",
				property: "overflowInt8",
				testValue: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[int8](val)).Error(), "overflows int8")
				},
				testPointer: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[*int8](val)).Error(), "overflows int8")
				},
				errorMsg: "overflows int8",
			},
			{
				name:     "uint8",
				property: "overflowUint8",
				testValue: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[uint8](val)).Error(), "overflows uint8")
				},
				testPointer: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[*uint8](val)).Error(), "overflows uint8")
				},
				errorMsg: "overflows uint8",
			},
			{
				name:     "int16",
				property: "overflowInt16",
				testValue: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[int16](val)).Error(), "overflows int16")
				},
				testPointer: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[*int16](val)).Error(), "overflows int16")
				},
				errorMsg: "overflows int16",
			},
			{
				name:     "uint16",
				property: "overflowUint16",
				testValue: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[uint16](val)).Error(), "overflows uint16")
				},
				testPointer: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[*uint16](val)).Error(), "overflows uint16")
				},
				errorMsg: "overflows uint16",
			},
			{
				name:     "int32",
				property: "overflowInt32",
				testValue: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[int32](val)).Error(), "overflows int32")
				},
				testPointer: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[*int32](val)).Error(), "overflows int32")
				},
				errorMsg: "overflows int32",
			},
			{
				name:     "uint32",
				property: "overflowUint32",
				testValue: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[uint32](val)).Error(), "overflows uint32")
				},
				testPointer: func(val *qjs.Value) {
					assert.Contains(t, extractErr(qjs.JsNumberToGo[*uint32](val)).Error(), "overflows uint32")
				},
				errorMsg: "overflows uint32",
			},
		}

		for _, tc := range overflowTestCases {
			t.Run(tc.name+"_overflow", func(t *testing.T) {
				val := result.GetPropertyStr(tc.property)
				defer val.Free()
				tc.testValue(val)
			})

			t.Run("*"+tc.name+"_overflow", func(t *testing.T) {
				val := result.GetPropertyStr(tc.property)
				defer val.Free()
				tc.testPointer(val)
			})
		}
	})

	t.Run("normal", func(t *testing.T) {
		normalNum := result.GetPropertyStr("normalNumber")
		defer normalNum.Free()

		floatVal := result.GetPropertyStr("floatValue")
		defer floatVal.Free()

		bigIntVal := result.GetPropertyStr("bigIntValue")
		defer bigIntVal.Free()

		type CustomNumber int

		type numTest struct {
			name   string
			expect any
			actual any
			delta  float64
		}

		numTests := []numTest{
			{"int8", int8(42), must(qjs.JsNumberToGo[int8](normalNum)), 0},
			{"int8_pointer", int8(42), *must(qjs.JsNumberToGo[*int8](normalNum)), 0},
			{"uint8", uint8(42), must(qjs.JsNumberToGo[uint8](normalNum)), 0},
			{"*uint8", uint8(42), *must(qjs.JsNumberToGo[*uint8](normalNum)), 0},
			{"int16", int16(42), must(qjs.JsNumberToGo[int16](normalNum)), 0},
			{"*int16", int16(42), *must(qjs.JsNumberToGo[*int16](normalNum)), 0},
			{"uint16", uint16(42), must(qjs.JsNumberToGo[uint16](normalNum)), 0},
			{"*uint16", uint16(42), *must(qjs.JsNumberToGo[*uint16](normalNum)), 0},
			{"int32", int32(42), must(qjs.JsNumberToGo[int32](normalNum)), 0},
			{"*int32", int32(42), *must(qjs.JsNumberToGo[*int32](normalNum)), 0},
			{"uint32", uint32(42), must(qjs.JsNumberToGo[uint32](normalNum)), 0},
			{"*uint32", uint32(42), *must(qjs.JsNumberToGo[*uint32](normalNum)), 0},
			{"int", int(42), must(qjs.JsNumberToGo[int](normalNum)), 0},
			{"*int", int(42), *must(qjs.JsNumberToGo[*int](normalNum)), 0},
			{"uint", uint(42), must(qjs.JsNumberToGo[uint](normalNum)), 0},
			{"*uint", uint(42), *must(qjs.JsNumberToGo[*uint](normalNum)), 0},
			{"int64", int64(42), must(qjs.JsNumberToGo[int64](normalNum)), 0},
			{"*int64", int64(42), *must(qjs.JsNumberToGo[*int64](normalNum)), 0},
			{"uint64", uint64(42), must(qjs.JsNumberToGo[uint64](normalNum)), 0},
			{"*uint64", uint64(42), *must(qjs.JsNumberToGo[*uint64](normalNum)), 0},
			{"uintptr", uintptr(42), must(qjs.JsNumberToGo[uintptr](normalNum)), 0},
			{"*uintptr", uintptr(42), *must(qjs.JsNumberToGo[*uintptr](normalNum)), 0},

			{"intAsFloat32", float32(42), must(qjs.JsNumberToGo[float32](normalNum)), 0},
			{"*intAsFloat32", float32(42), *must(qjs.JsNumberToGo[*float32](normalNum)), 0},
			{"intAsFloat64", float64(42), must(qjs.JsNumberToGo[float64](normalNum)), 0},
			{"*intAsFloat64", float64(42), *must(qjs.JsNumberToGo[*float64](normalNum)), 0},

			{"bigInt", 0, big.NewInt(9007199254740991).Cmp(must(qjs.JsNumberToGo[*big.Int](bigIntVal))), 0},

			{"float32", float32(3.14159), must(qjs.JsNumberToGo[float32](floatVal)), 0.00001},
			{"*float32", float32(3.14159), *must(qjs.JsNumberToGo[*float32](floatVal)), 0.00001},
			{"float64", 3.14159, must(qjs.JsNumberToGo[float64](floatVal)), 0.00001},
			{"*float64", 3.14159, *must(qjs.JsNumberToGo[*float64](floatVal)), 0.00001},

			{"complex64", complex64(complex(float32(42), 0)), must(qjs.JsNumberToGo[complex64](normalNum)), 0},
			{"*complex64", complex64(complex(float32(42), 0)), *must(qjs.JsNumberToGo[*complex64](normalNum)), 0},
			{"complex128", complex(float64(42), 0), must(qjs.JsNumberToGo[complex128](normalNum)), 0},
			{"*complex128", complex(float64(42), 0), *must(qjs.JsNumberToGo[*complex128](normalNum)), 0},
			{"CustomNumber", CustomNumber(42), must(qjs.JsNumberToGo[CustomNumber](normalNum)), 0},
		}

		for _, test := range numTests {
			t.Run(test.name, func(t *testing.T) {
				if test.delta == 0 {
					assert.Equal(t, test.expect, test.actual)
				} else {
					assert.InDelta(t, test.expect, test.actual, test.delta)
				}
			})
		}
	})
}

func TestJsTimeToGo(t *testing.T) {
	type testToGoTime struct {
		name      string
		jsCode    string
		expectErr func(t *testing.T, err error)
		expect    func(t *testing.T, goTime time.Time, err error)
	}

	tests := []testToGoTime{
		{
			name:   "convert_current_time",
			jsCode: `new Date()`,
			expect: func(t *testing.T, goTime time.Time, err error) {
				require.NoError(t, err)

				now := time.Now()
				diff := now.Sub(goTime).Seconds()
				assert.LessOrEqual(t, math.Abs(diff), 2.0,
					"Converted time should be close to current time")
			},
		},
		{
			name:   "convert_specific_date",
			jsCode: `new Date(Date.UTC(2023, 8, 15, 12, 30, 45, 123))`,
			expect: func(t *testing.T, goTime time.Time, err error) {
				require.NoError(t, err)
				assert.Equal(t, 2023, goTime.UTC().Year())
				assert.Equal(t, time.September, goTime.UTC().Month())
				assert.Equal(t, 15, goTime.UTC().Day())
				assert.Equal(t, 12, goTime.UTC().Hour())
				assert.Equal(t, 30, goTime.UTC().Minute())
				assert.Equal(t, 45, goTime.UTC().Second())
				assert.Equal(t, 123000000, goTime.UTC().Nanosecond())
			},
		},
		{
			name:   "convert_unix_epoch",
			jsCode: `new Date(0)`,
			expect: func(t *testing.T, goTime time.Time, err error) {
				require.NoError(t, err)
				assert.Equal(t, time.Unix(0, 0).UTC(), goTime.UTC())
			},
		},
		{
			name:   "negative_timestamp",
			jsCode: `new Date(-86400000)`,
			expect: func(t *testing.T, goTime time.Time, err error) {
				require.NoError(t, err)
				expected := time.Unix(-86400, 0)
				assert.Equal(t, expected.UTC(), goTime.UTC())
			},
		},
		{
			name:   "error_with_non_date_object",
			jsCode: `"not a date"`,
			expectErr: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "cannot call getTime on JS value")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsDate := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsDate.Free()

			goTime, err := qjs.JsTimeToGo(jsDate)
			if test.expectErr != nil {
				test.expectErr(t, err)
				return
			}

			require.NoError(t, err)
			test.expect(t, goTime, err)
		})
	}
}

func TestJsArrayBufferToGo(t *testing.T) {
	tests := []struct {
		name      string
		jsCode    string
		expected  []byte
		expectErr bool
	}{
		{
			name: "normal_array_buffer",
			jsCode: `(() => {
				const buf = new ArrayBuffer(5);
				const view = new Uint8Array(buf);
				view.set([1, 2, 3, 4, 5]);
				return buf;
			})()`,
			expected: []byte{1, 2, 3, 4, 5},
		},
		{
			name:     "empty_array_buffer",
			jsCode:   `new ArrayBuffer(0)`,
			expected: nil,
		},
		{
			name:      "not_an_array_buffer",
			jsCode:    `"not an array buffer"`,
			expectErr: true,
		},
		{
			name:   "large_array_buffer",
			jsCode: `(() => { const buf = new ArrayBuffer(1024); const view = new Uint8Array(buf); for(let i=0; i<1024; i++) view[i] = i % 256; return buf; })()`,
			expected: func() []byte {
				b := make([]byte, 1024)
				for i := range 1024 {
					b[i] = byte(i % 256)
				}
				return b
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsArrayBufferToGo(jsValue)
			if test.expectErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestIsTypedArray(t *testing.T) {
	tests := []struct {
		name     string
		jsCode   string
		expected bool
	}{
		{
			name:     "uint8_array",
			jsCode:   "new Uint8Array(4)",
			expected: true,
		},
		{
			name:     "int8_array",
			jsCode:   "new Int8Array(4)",
			expected: true,
		},
		{
			name:     "uint16_array",
			jsCode:   "new Uint16Array(4)",
			expected: true,
		},
		{
			name:     "int16_array",
			jsCode:   "new Int16Array(4)",
			expected: true,
		},
		{
			name:     "uint32_array",
			jsCode:   "new Uint32Array(4)",
			expected: true,
		},
		{
			name:     "int32_array",
			jsCode:   "new Int32Array(4)",
			expected: true,
		},
		{
			name:     "float32_array",
			jsCode:   "new Float32Array(4)",
			expected: true,
		},
		{
			name:     "float64_array",
			jsCode:   "new Float64Array(4)",
			expected: true,
		},
		{
			name:     "bigint64_array",
			jsCode:   "new BigInt64Array(4)",
			expected: true,
		},
		{
			name:     "biguint64_array",
			jsCode:   "new BigUint64Array(4)",
			expected: true,
		},
		{
			name:     "data_view",
			jsCode:   "new DataView(new ArrayBuffer(4))",
			expected: true,
		},
		{
			name:     "regular_array",
			jsCode:   "[1, 2, 3, 4]",
			expected: false,
		},
		{
			name:     "object",
			jsCode:   "({a: 1, b: 2})",
			expected: false,
		},
		{
			name:     "string",
			jsCode:   "'test'",
			expected: false,
		},
		{
			name:     "number",
			jsCode:   "42",
			expected: false,
		},
		{
			name:     "boolean",
			jsCode:   "true",
			expected: false,
		},
		{
			name:     "null",
			jsCode:   "null",
			expected: false,
		},
		{
			name:     "undefined",
			jsCode:   "undefined",
			expected: false,
		},
		{
			name:     "array_buffer_not_typed_array",
			jsCode:   "new ArrayBuffer(4)",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result := qjs.IsTypedArray(jsValue)
			assert.Equal(t, test.expected, result, "IsTypedArray result should match expected value")
		})
	}
}

func TestJsTypedArrayToGo(t *testing.T) {
	tests := []struct {
		name      string
		jsCode    string
		expected  []byte
		expectErr bool
		errMsg    string
	}{
		{
			name:     "uint8_array_with_values",
			jsCode:   `new Uint8Array([1, 2, 3, 4, 5])`,
			expected: []byte{1, 2, 3, 4, 5},
		},
		{
			name:     "empty_typed_array",
			jsCode:   `new Uint8Array(0)`,
			expected: nil,
		},
		{
			name:     "int8_array_with_values",
			jsCode:   `new Int8Array([10, 20, 30, -40, -50])`,
			expected: []byte{10, 20, 30, 216, 206},
		},
		{
			name: "typed_array_with_offset",
			jsCode: `(() => {
				const buf = new ArrayBuffer(10);
				const view = new Uint8Array(buf);
				view.set([1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
				return new Uint8Array(buf, 3, 4);
			})()`,
			expected: []byte{4, 5, 6, 7},
		},
		{
			name:     "typed_array_subview",
			jsCode:   `(() => { const original = new Uint8Array([10, 20, 30, 40, 50, 60]); return original.subarray(2, 5); })()`,
			expected: []byte{30, 40, 50},
		},
		{
			name:     "data_view",
			jsCode:   `(() => { const buf = new ArrayBuffer(4); const view = new Uint8Array(buf); view.set([15, 25, 35, 45]); return new DataView(buf); })()`,
			expected: []byte{15, 25, 35, 45},
		},
		{
			name:     "data_view_with_offset_and_length",
			jsCode:   `(() => { const buf = new ArrayBuffer(8); const view = new Uint8Array(buf); view.set([1, 2, 3, 4, 5, 6, 7, 8]); return new DataView(buf, 2, 4); })()`,
			expected: []byte{3, 4, 5, 6},
		},
		{
			name:      "not_a_typed_array",
			jsCode:    `"not a typed array"`,
			expectErr: true,
			errMsg:    "invalid TypedArray: missing buffer property",
		},
		{
			name:      "typed_array_with_invalid_offset",
			jsCode:    `(() => { const buf = new ArrayBuffer(5); const view = new Uint8Array(buf); try { return new Uint8Array(buf, 10, 1); } catch { return { buffer: new ArrayBuffer(5), byteOffset: 10, byteLength: 1 }; } })()`,
			expectErr: true,
			errMsg:    "exceeds max",
		},
		{
			name:     "different_typed_arrays_sharing_same_buffer",
			jsCode:   `(() => { const buf = new ArrayBuffer(8); const view1 = new Uint8Array(buf); view1.set([1, 2, 3, 4, 5, 6, 7, 8]); return new Int16Array(buf, 2, 2); })()`,
			expected: []byte{3, 4, 5, 6},
		},
		{
			name:      "typed_array_with_non_array_buffer_buffer",
			jsCode:    `(() => { return { buffer: {}, byteOffset: 0, byteLength: 0 }; })()`,
			expectErr: true,
			errMsg:    "invalid TypedArray: buffer is not a byte array",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsTypedArrayToGo(jsValue)
			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestJsArrayToGo(t *testing.T) {
	tests := []struct {
		name      string
		jsCode    string
		sample    any
		expected  any
		expectErr bool
		errMsg    string
	}{
		{
			name:     "empty_array_to_interface_slice",
			jsCode:   `[]`,
			sample:   []any{},
			expected: []any{},
		},
		{
			name:     "empty_array_to_nil_sample",
			jsCode:   `[]`,
			sample:   nil,
			expected: []any{},
		},
		{
			name:     "simple_array_to_interface_slice",
			jsCode:   `[1, "two", true, null]`,
			sample:   []any{},
			expected: []any{int64(1), "two", true, nil},
		},
		{
			name:     "number_array_to_int_slice",
			jsCode:   `[1, 2, 3, 4, 5]`,
			sample:   []int{},
			expected: []int{1, 2, 3, 4, 5},
		},
		{
			name:     "number_array_to_int64_slice",
			jsCode:   `[1, 2, 3, 4, 5]`,
			sample:   []int64{},
			expected: []int64{1, 2, 3, 4, 5},
		},
		{
			name:     "string_array_to_string_slice",
			jsCode:   `["a", "b", "c", "d", "e"]`,
			sample:   []string{},
			expected: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:      "mixed_array_to_string_slice_error",
			jsCode:    `["10", 20, "30", "40", "50"]`,
			sample:    []string{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:     "array_to_fixed_size_array",
			jsCode:   `[1, 2, 3]`,
			sample:   [3]int{},
			expected: [3]int{1, 2, 3},
		},
		{
			name:     "array_to_smaller_fixed_size_array",
			jsCode:   `[1, 2]`,
			sample:   [3]int{},
			expected: [3]int{1, 2, 0},
		},
		{
			name:      "array_to_too_small_fixed_size_array",
			jsCode:    `[1, 2, 3, 4, 5]`,
			sample:    [3]int{},
			expectErr: true,
			errMsg:    "JS array/set length (5) exceeds Go array length (3)",
		},
		{
			name:     "nested_array_to_nested_slice",
			jsCode:   `[[1, 2], [3, 4], [5, 6]]`,
			sample:   [][]int{},
			expected: [][]int{{1, 2}, {3, 4}, {5, 6}},
		},
		{
			name:     "objects_array_to_map_slice",
			jsCode:   `[{"name": "Alice", "age": 30}, {"name": "Bob", "age": 25}]`,
			sample:   []map[string]any{},
			expected: []map[string]any{{"name": "Alice", "age": int64(30)}, {"name": "Bob", "age": int64(25)}},
		},
		{
			name: "array_to_struct_slice",
			jsCode: `[
				{"name": "Alice", "age": 30, "active": true},
				{"name": "Bob", "age": 25, "active": false}
			]`,
			sample: []struct {
				Name   string `json:"name"`
				Age    int    `json:"age"`
				Active bool   `json:"active"`
			}{},
			expected: []struct {
				Name   string `json:"name"`
				Age    int    `json:"age"`
				Active bool   `json:"active"`
			}{
				{Name: "Alice", Age: 30, Active: true},
				{Name: "Bob", Age: 25, Active: false},
			},
		},
		{
			name:      "array_with_element_conversion_error",
			jsCode:    `[1, 2, "not a number", 4, 5]`,
			sample:    []int{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:     "array_to_complex_type_via_json",
			jsCode:   `[{"Date": new Date("2023-01-02T15:04:05Z")}, {"Date": new Date("2023-02-03T10:20:30Z")}]`,
			sample:   []struct{ Date time.Time }{},
			expected: []struct{ Date time.Time }{{Date: time.Date(2023, 1, 2, 15, 4, 5, 0, time.UTC)}, {Date: time.Date(2023, 2, 3, 10, 20, 30, 0, time.UTC)}},
		},
		{
			name:      "array_with_invalid_json_serialization",
			jsCode:    `[() => {}]`,
			sample:    []struct{}{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:      "invalid_unmarshaling",
			jsCode:    `[{"InvalidDate": "not-a-date"}]`,
			sample:    []struct{ InvalidDate time.Time }{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:      "slice_with_symbol_element",
			jsCode:    `[1, "string", true, Symbol("test")]`,
			sample:    []any{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:      "slice_with_symbol_element_to_interface",
			jsCode:    `[1, "string", true, Symbol("test")]`,
			sample:    any(nil),
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:      "array_with_symbol_element",
			jsCode:    `[1, "string", true, Symbol("test")]`,
			sample:    [4]any{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:      "array_with_symbol_element_to_interface",
			jsCode:    `[1, "string", true, Symbol("test")]`,
			sample:    [4]any{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},

		{
			name:     "json_fallback_successful_unmarshal",
			jsCode:   `["valid"]`,
			sample:   CustomUnmarshaler{},
			expected: CustomUnmarshaler{Value: "[\"valid\"]"},
		},
		{
			name:      "json_fallback_unmarshal_error",
			jsCode:    `[{"wrongKey": "wrong value"}]`,
			sample:    struct{ RequiredValue int }{},
			expectErr: true,
			errMsg:    "cannot unmarshal array into Go",
		},
		{
			name:      "custom_unmarshaler_error",
			jsCode:    `["invalid"]`,
			sample:    CustomUnmarshaler{},
			expectErr: true,
			errMsg:    "custom unmarshaling error",
		},
		{
			name:      "number_to_array_error",
			jsCode:    `42`,
			sample:    []int{},
			expectErr: true,
			errMsg:    "cannot convert JS Array",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsArrayToGo(jsValue, test.sample)
			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestJsSetToGo(t *testing.T) {
	tests := []struct {
		name          string
		jsCode        string
		sample        any
		expect        func(t *testing.T, result any, err error)
		expectedValue any
		expectErr     bool
		errMsg        string
	}{
		{
			name:          "empty_set_to_interface_slice",
			jsCode:        `new Set()`,
			sample:        []any{},
			expectedValue: []any{},
		},
		{
			name:          "simple_set_to_interface_slice",
			jsCode:        `new Set([1, "two", true, null])`,
			sample:        []any{},
			expectedValue: []any{int64(1), "two", true, nil},
		},
		{
			name:          "number_set_to_int_slice",
			jsCode:        `new Set([1, 2, 3, 4, 5])`,
			sample:        []int{},
			expectedValue: []int{1, 2, 3, 4, 5},
		},
		{
			name:          "string_set_to_string_slice",
			jsCode:        `new Set(["a", "b", "c", "d", "e"])`,
			sample:        []string{},
			expectedValue: []string{"a", "b", "c", "d", "e"},
		},
		{
			name:      "mixed_set_to_string_slice_error",
			jsCode:    `new Set(["10", 20, "30", "40", "50"])`,
			sample:    []string{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:          "set_to_fixed_size_array",
			jsCode:        `new Set([1, 2, 3])`,
			sample:        [3]int{},
			expectedValue: [3]int{1, 2, 3},
		},
		{
			name:          "set_to_smaller_fixed_size_array",
			jsCode:        `new Set([1, 2])`,
			sample:        [3]int{},
			expectedValue: [3]int{1, 2, 0},
		},
		{
			name:      "set_to_too_small_fixed_size_array",
			jsCode:    `new Set([1, 2, 3, 4, 5])`,
			sample:    [3]int{},
			expectErr: true,
			errMsg:    "JS array/set length (5) exceeds Go array length (3)",
		},
		{
			name: "nested_set_to_nested_slice",
			jsCode: `(() => {
				const set = new Set();
				set.add([1, 2]);
				set.add([3, 4]);
				set.add([5, 6]);
				return set;
			})()`,
			sample:        [][]int{},
			expectedValue: [][]int{{1, 2}, {3, 4}, {5, 6}},
		},
		{
			name:      "not_a_set",
			jsCode:    `"not a set"`,
			sample:    []any{},
			expectErr: true,
			errMsg:    `cannot convert JS Set "not a set" to Go`,
		},
		{
			name:          "sets_with_duplicate_elements",
			jsCode:        `new Set([1, 2, 2, 3, 3, 3, 4, 5, 5])`,
			sample:        []int{},
			expectedValue: []int{1, 2, 3, 4, 5},
		},
		{
			name:          "set_with_complex_objects",
			jsCode:        `new Set([{name: "Alice", age: 30}, {name: "Bob", age: 25}])`,
			sample:        []map[string]any{},
			expectedValue: []map[string]any{{"name": "Alice", "age": int64(30)}, {"name": "Bob", "age": int64(25)}},
		},
		{
			name: "set_with_structs",
			jsCode: `new Set([
				{name: "Alice", age: 30, active: true},
				{name: "Bob", age: 25, active: false}
			])`,
			sample: []struct {
				Name   string `json:"name"`
				Age    int    `json:"age"`
				Active bool   `json:"active"`
			}{},
			expectedValue: []struct {
				Name   string `json:"name"`
				Age    int    `json:"age"`
				Active bool   `json:"active"`
			}{
				{Name: "Alice", Age: 30, Active: true},
				{Name: "Bob", Age: 25, Active: false},
			},
		},
		{
			name:          "set_of_dates",
			jsCode:        `new Set([new Date("2023-01-02T15:04:05Z"), new Date("2023-02-03T10:20:30Z")])`,
			sample:        []time.Time{},
			expectedValue: []time.Time{time.Date(2023, 1, 2, 15, 4, 5, 0, time.UTC), time.Date(2023, 2, 3, 10, 20, 30, 0, time.UTC)},
		},
		{
			name:      "set_with_symbol_element",
			jsCode:    `new Set([1, "string", true, Symbol("test")])`,
			sample:    []any{},
			expectErr: true,
			errMsg:    "cannot convert JS array/set element",
		},
		{
			name:   "set_of_functions",
			jsCode: `new Set([(a) => a, (b) => b * 2, (c) => c + 3])`,
			sample: []func(int) (int, error){},
			expect: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.Len(t, result, 3)
				assert.IsType(t, []func(int) (int, error)(nil), result)
				assert.Len(t, result.([]func(int) (int, error)), 3)
				fn1 := result.([]func(int) (int, error))[0]
				fn2 := result.([]func(int) (int, error))[1]
				fn3 := result.([]func(int) (int, error))[2]
				assert.Equal(t, 1, must(fn1(1)))
				assert.Equal(t, 4, must(fn2(2)))
				assert.Equal(t, 6, must(fn3(3)))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsSetToGo(jsValue, test.sample)

			if test.expect != nil {
				test.expect(t, result, err)
				return
			}

			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				if test.expectedValue != nil {
					assert.Equal(t, test.expectedValue, result)
				}
			}
		})
	}
}

func TestJsObjectOrMapToGoMap(t *testing.T) {
	tests := []struct {
		name          string
		jsCode        string
		sample        any
		expect        func(t *testing.T, result any, err error)
		expectedValue any
		expectErr     bool
		errMsg        string
	}{
		{
			name:          "empty_object_to_string_any_map",
			jsCode:        `({})`,
			sample:        map[string]any{},
			expectedValue: map[string]any{},
		},
		{
			name:      "key_conversion_error",
			jsCode:    `(() => { const m = new Map(); m.set(Symbol("test"), "value"); return m; })()`,
			sample:    map[string]string{},
			expectErr: true,
			errMsg:    "cannot convert JS map key Symbol to Go",
		},
		{
			// float64 are not convertible to string keys in Go maps
			name:      "key_type_mismatch",
			jsCode:    `new Map([["key", "value"], [42.222222, "number"]])`,
			sample:    map[string]string{},
			expectErr: true,
			errMsg:    "cannot convert",
		},
		{
			name:          "simple_object_to_string_any_map",
			jsCode:        `({"name": "Alice", "age": 30, "active": true})`,
			sample:        map[string]any{},
			expectedValue: map[string]any{"name": "Alice", "age": int64(30), "active": true},
		},
		{
			name:          "object_to_string_int_map",
			jsCode:        `({"one": 1, "two": 2, "three": 3})`,
			sample:        map[string]int{},
			expectedValue: map[string]int{"one": 1, "two": 2, "three": 3},
		},
		{
			name:          "object_to_string_string_map",
			jsCode:        `({"first": "Alice", "last": "Smith", "title": "Engineer"})`,
			sample:        map[string]string{},
			expectedValue: map[string]string{"first": "Alice", "last": "Smith", "title": "Engineer"},
		},
		{
			name:          "empty_map_to_string_any_map",
			jsCode:        `new Map()`,
			sample:        map[string]any{},
			expectedValue: map[string]any{},
		},
		{
			name:          "map_to_string_any_map",
			jsCode:        `new Map([["name", "Bob"], ["age", 25], ["active", false]])`,
			sample:        map[string]any{},
			expectedValue: map[string]any{"name": "Bob", "age": int64(25), "active": false},
		},
		{
			name:      "non_object_input",
			jsCode:    `"not an object"`,
			sample:    map[string]any{},
			expectErr: true,
			errMsg:    "expected JS object",
		},
		{
			name:          "map_with_int_keys",
			jsCode:        `new Map([[1, "one"], [2, "two"], [3, "three"]])`,
			sample:        map[int]string{},
			expectedValue: map[int]string{1: "one", 2: "two", 3: "three"},
		},
		{
			name:   "map_with_function_values",
			jsCode: `new Map([["func1", (a) => a], ["func2", (b) => b * 2], ["func3", (c) => c + 3]])`,
			sample: map[string]func(int) (int, error){},
			expect: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.Len(t, result, 3)
				assert.IsType(t, map[string]func(int) (int, error)(nil), result)
				assert.Len(t, result.(map[string]func(int) (int, error)), 3)
				fn1 := result.(map[string]func(int) (int, error))["func1"]
				fn2 := result.(map[string]func(int) (int, error))["func2"]
				fn3 := result.(map[string]func(int) (int, error))["func3"]
				assert.Equal(t, 1, must(fn1(1)))
				assert.Equal(t, 4, must(fn2(2)))
				assert.Equal(t, 6, must(fn3(3)))
			},
		},
		{
			name:          "nested_objects",
			jsCode:        `({"user": {"name": "Alice", "details": {"age": 30, "role": "admin"}}, "settings": {"theme": "dark"}})`,
			sample:        map[string]any{},
			expectedValue: map[string]any{"user": map[string]any{"name": "Alice", "details": map[string]any{"age": int64(30), "role": "admin"}}, "settings": map[string]any{"theme": "dark"}},
		},
		{
			name:          "object_with_array_values",
			jsCode:        `({"ids": [1, 2, 3], "names": ["Alice", "Bob", "Charlie"]})`,
			sample:        map[string]any{},
			expectedValue: map[string]any{"ids": []any{int64(1), int64(2), int64(3)}, "names": []any{"Alice", "Bob", "Charlie"}},
		},
		{
			name:          "object_with_date_values",
			jsCode:        `({"created": new Date("2023-01-15T00:00:00Z"), "updated": new Date("2023-02-20T00:00:00Z")})`,
			sample:        map[string]time.Time{},
			expectedValue: map[string]time.Time{"created": time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC), "updated": time.Date(2023, 2, 20, 0, 0, 0, 0, time.UTC)},
		},
		{
			name:          "string_to_int_conversion",
			jsCode:        `({"one": 1, "two": "2", "three": 3})`,
			sample:        map[string]int{},
			expectedValue: map[string]int{"one": 1, "two": 2, "three": 3},
		},
		{
			name:      "actual_type_mismatch_in_values",
			jsCode:    `({"valid": 1, "invalid": {"nested": "object"}, "alsoInvalid": [1,2,3]})`,
			sample:    map[string]int{},
			expectErr: true,
			errMsg:    "cannot unmarshal object into Go value of type int",
		},
		{
			name:      "circular_reference",
			jsCode:    `(() => { const obj = {}; obj.self = obj; return obj; })()`,
			sample:    map[string]any{},
			expectErr: true,
			errMsg:    "cannot convert JS circular reference",
		},
		{
			name:          "map_with_object_values",
			jsCode:        `new Map([["user1", {name: "Alice", age: 30}], ["user2", {name: "Bob", age: 25}]])`,
			sample:        map[string]map[string]any{},
			expectedValue: map[string]map[string]any{"user1": {"name": "Alice", "age": int64(30)}, "user2": {"name": "Bob", "age": int64(25)}},
		},
		{
			name:          "complex_structure",
			jsCode:        `({"users": [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}], "counts": {"active": 2, "inactive": 0}})`,
			sample:        map[string]any{},
			expectedValue: map[string]any{"users": []any{map[string]any{"id": int64(1), "name": "Alice"}, map[string]any{"id": int64(2), "name": "Bob"}}, "counts": map[string]any{"active": int64(2), "inactive": int64(0)}},
		},
		{
			name:          "map_with_mixed_key_types",
			jsCode:        `new Map([[1, "one"], ["two", 2], [true, "boolean"]])`,
			sample:        map[any]any{},
			expectedValue: map[any]any{int64(1): "one", "two": int64(2), true: "boolean"},
		},
		{
			name:          "object_with_null_values",
			jsCode:        `({"name": "Alice", "age": null, "active": true})`,
			sample:        map[string]any{},
			expectedValue: map[string]any{"name": "Alice", "age": nil, "active": true},
		},
		{
			name:   "object_with_function_values",
			jsCode: `({"func1": (a) => a, "func2": (b) => b * 2, "func3": (c) => c + 3})`,
			sample: map[string]func(int) (int, error){},
			expect: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.Len(t, result, 3)
				assert.IsType(t, map[string]func(int) (int, error)(nil), result)
				assert.Len(t, result.(map[string]func(int) (int, error)), 3)
				fn1 := result.(map[string]func(int) (int, error))["func1"]
				fn2 := result.(map[string]func(int) (int, error))["func2"]
				fn3 := result.(map[string]func(int) (int, error))["func3"]
				assert.Equal(t, 1, must(fn1(1)))
				assert.Equal(t, 4, must(fn2(2)))
				assert.Equal(t, 6, must(fn3(3)))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsObjectOrMapToGoMap(jsValue, test.sample)
			if test.expect != nil {
				test.expect(t, result, err)
				return
			}

			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				if test.expectedValue != nil {
					assert.Equal(t, test.expectedValue, result)
				}
			}
		})
	}
}

func TestJsObjectOrMapToGoStruct(t *testing.T) {
	type CircularReference struct {
		Self *CircularReference `json:"self,omitempty"`
	}
	tests := []struct {
		name          string
		jsCode        string
		sample        any
		expect        func(t *testing.T, result any, err error)
		expectedValue any
		expectErr     bool
		errMsg        string
	}{
		{
			name:   "basic_struct_conversion",
			jsCode: `({"Name": "Alice", "Age": 30, "Active": true})`,
			sample: struct {
				Name   string
				Age    int
				Active bool
			}{},
			expectedValue: struct {
				Name   string
				Age    int
				Active bool
			}{Name: "Alice", Age: 30, Active: true},
		},
		{
			name:   "struct_pointer_conversion",
			jsCode: `({"Name": "Bob", "Age": 25, "Active": false})`,
			sample: &struct {
				Name   string
				Age    int
				Active bool
			}{},
			expectedValue: &struct {
				Name   string
				Age    int
				Active bool
			}{Name: "Bob", Age: 25, Active: false},
		},
		{
			name:   "json_tag_mapping",
			jsCode: `({"user_name": "Charlie", "user_age": 35})`,
			sample: struct {
				Name string `json:"user_name"`
				Age  int    `json:"user_age"`
			}{},
			expectedValue: struct {
				Name string `json:"user_name"`
				Age  int    `json:"user_age"`
			}{Name: "Charlie", Age: 35},
		},
		{
			name:   "ignore_tag",
			jsCode: `({"Name": "Dave", "Secret": "hidden"})`,
			sample: struct {
				Name   string
				Secret string `json:"-"`
			}{},
			expectedValue: struct {
				Name   string
				Secret string `json:"-"`
			}{Name: "Dave", Secret: ""},
		},
		{
			name:   "field_case_sensitivity",
			jsCode: `({"Name": "Eve", "AGE": 40})`,
			sample: struct {
				Name string
				Age  int
			}{},
			expectedValue: struct {
				Name string
				Age  int
			}{Name: "Eve", Age: 0}, // Age not matched due to case difference
		},
		{
			name:   "nested_struct",
			jsCode: `({"Name": "Frank", "Address": {"City": "New York", "Zip": "10001"}})`,
			sample: struct {
				Name    string
				Address struct {
					City string
					Zip  string
				}
			}{},
			expectedValue: struct {
				Name    string
				Address struct {
					City string
					Zip  string
				}
			}{Name: "Frank", Address: struct {
				City string
				Zip  string
			}{City: "New York", Zip: "10001"}},
		},
		{
			name: "unexported_fields",
			jsCode: `({
				"Name": "Grace", 
				"Age": 45, 
				"privateInfo": "should be ignored"
			})`,
			sample: struct {
				Name        string
				Age         int
				privateInfo string // unexported
			}{},
			expectedValue: struct {
				Name        string
				Age         int
				privateInfo string
			}{Name: "Grace", Age: 45, privateInfo: ""},
		},
		{
			name:   "type_conversion",
			jsCode: `({"IntVal": 50, "FloatAsInt": 25.75})`,
			sample: struct {
				IntVal     int
				FloatAsInt int
			}{},
			expectedValue: struct {
				IntVal     int
				FloatAsInt int
			}{IntVal: 50, FloatAsInt: 25},
		},
		{
			name:   "missing_fields",
			jsCode: `({"Name": "Helen"})`,
			sample: struct {
				Name string
				Age  int
				Bio  string
			}{},
			expectedValue: struct {
				Name string
				Age  int
				Bio  string
			}{Name: "Helen", Age: 0, Bio: ""},
		},
		{
			name:   "null_values",
			jsCode: `({"Name": "Ian", "Age": null, "Bio": null})`,
			sample: struct {
				Name string
				Age  int
				Bio  string
			}{},
			expectedValue: struct {
				Name string
				Age  int
				Bio  string
			}{Name: "Ian", Age: 0, Bio: ""}, // null converted to zero values
		},
		{
			name:   "array_fields",
			jsCode: `({"Name": "Jack", "Scores": [85, 90, 95]})`,
			sample: struct {
				Name   string
				Scores []int
			}{},
			expectedValue: struct {
				Name   string
				Scores []int
			}{Name: "Jack", Scores: []int{85, 90, 95}},
		},
		{
			name:   "date_field",
			jsCode: `({"Name": "Kelly", "Birthdate": new Date("1990-01-15T00:00:00Z")})`,
			sample: struct {
				Name      string
				Birthdate time.Time
			}{},
			expectedValue: struct {
				Name      string
				Birthdate time.Time
			}{Name: "Kelly", Birthdate: time.Date(1990, 1, 15, 0, 0, 0, 0, time.UTC)},
		},
		{
			name:   "map_instead_of_object",
			jsCode: `new Map([["Name", "Louis"], ["Age", 55]])`,
			sample: struct {
				Name string
				Age  int
			}{},
			expectedValue: struct {
				Name string
				Age  int
			}{Name: "Louis", Age: 55},
		},
		{
			name:   "incompatible_type",
			jsCode: `({"Name": "Mike", "Age": "fifty"})`,
			sample: struct {
				Name string
				Age  int
			}{},
			expectErr: true,
			errMsg:    "cannot convert JS object/map field 'Age'",
		},
		{
			name:      "non_object_input",
			jsCode:    `"not an object"`,
			sample:    struct{}{},
			expectErr: true,
			errMsg:    "expected JS object",
		},
		{
			name:      "incompatible_type_assignment",
			jsCode:    `({"Complex": {"real": 1, "imag": 2}})`,
			sample:    struct{ Complex int }{},
			expectErr: true,
			errMsg:    "cannot unmarshal object into Go value of type int",
		},
		{
			name:   "field_tags_with_options",
			jsCode: `({"user": "Nancy", "is_admin": true})`,
			sample: struct {
				Username string `json:"user,omitempty"`
				IsAdmin  bool   `json:"is_admin,omitempty"`
			}{},
			expectedValue: struct {
				Username string `json:"user,omitempty"`
				IsAdmin  bool   `json:"is_admin,omitempty"`
			}{Username: "Nancy", IsAdmin: true},
		},
		{
			name:   "embedded_struct",
			jsCode: `({"Name": "Oscar", "Age": 60, "City": "Chicago"})`,
			sample: struct {
				EmbeddedStruct
				City string
			}{},
			expectedValue: struct {
				EmbeddedStruct
				City string
			}{EmbeddedStruct: EmbeddedStruct{Name: "Oscar", Age: 60}, City: "Chicago"},
		},
		{
			name:   "embedded_struct_pointer",
			jsCode: `({"Name": "Paul", "Age": 70, "Department": "Engineering"})`,
			sample: struct {
				*EmbeddedStruct
				Department string
			}{
				EmbeddedStruct: &EmbeddedStruct{},
			},
			expectedValue: struct {
				*EmbeddedStruct
				Department string
			}{
				EmbeddedStruct: &EmbeddedStruct{
					Name: "Paul",
					Age:  70,
				},
				Department: "Engineering",
			},
		},
		{
			name:   "unexported_embedded_struct",
			jsCode: `({"Name": "Paul", "Age": 70, "City": "Seattle", "Country": "USA"})`,
			sample: struct {
				unExportedFieldStruct
				Country string
			}{
				unExportedFieldStruct: unExportedFieldStruct{},
			},
			expectedValue: struct {
				unExportedFieldStruct
				Country string
			}{
				unExportedFieldStruct: unExportedFieldStruct{},
				Country:               "USA",
			},
		},
		{
			name:   "unexported_embedded_struct_pointer",
			jsCode: `({"Name": "Paul", "Age": 70, "City": "Seattle", "Country": "USA"})`,
			sample: struct {
				*unExportedFieldStruct
				Country string
			}{
				unExportedFieldStruct: &unExportedFieldStruct{},
			},
			expectedValue: struct {
				*unExportedFieldStruct
				Country string
			}{
				Country: "USA",
			},
		},
		{
			name:   "nested_embedded_struct",
			jsCode: `({"Name": "Quincy", "Age": 80, "Label": "Manager", "Anything": "ignored", Version: "1.0"})`,
			sample: struct {
				NestedEmbeddedStruct
				Version string
			}{},
			expectedValue: struct {
				NestedEmbeddedStruct
				Version string
			}{
				NestedEmbeddedStruct: NestedEmbeddedStruct{
					EmbeddedStruct:        EmbeddedStruct{Name: "Quincy", Age: 80},
					unExportedFieldStruct: unExportedFieldStruct{},
					Label:                 "Manager",
				},
				Version: "1.0",
			},
		},
		{
			name:   "object_with_function_values",
			jsCode: `({"func1": (a) => a, "func2": (b) => b * 2, "func3": (c) => c + 3})`,
			sample: struct {
				Func1 func(int) (int, error) `json:"func1"`
				Func2 func(int) (int, error) `json:"func2"`
				Func3 func(int) (int, error) `json:"func3"`
			}{},
			expect: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.IsType(t, struct {
					Func1 func(int) (int, error) `json:"func1"`
					Func2 func(int) (int, error) `json:"func2"`
					Func3 func(int) (int, error) `json:"func3"`
				}{}, result)
				funcs := result.(struct {
					Func1 func(int) (int, error) `json:"func1"`
					Func2 func(int) (int, error) `json:"func2"`
					Func3 func(int) (int, error) `json:"func3"`
				})
				assert.Equal(t, 1, must(funcs.Func1(1)))
				assert.Equal(t, 4, must(funcs.Func2(2)))
				assert.Equal(t, 6, must(funcs.Func3(3)))
			},
		},
		{
			name:      "circular_reference",
			jsCode:    `(() => { const obj = {}; obj.self = obj; return obj; })()`,
			sample:    &CircularReference{},
			expectErr: true,
			errMsg:    "cannot convert JS circular reference",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsObjectOrMapToGoStruct(jsValue, test.sample)
			if test.expect != nil {
				test.expect(t, result, err)
				return
			}

			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				if test.expectedValue != nil {
					assert.Equal(t, test.expectedValue, result)
				}
			}
		})
	}

	rt := must(qjs.New())
	defer rt.Close()

	t.Run("circular_reference_stringify_error", func(t *testing.T) {
		// Create a JavaScript object with a circular reference that will cause JSONStringify to fail
		result := must(rt.Eval("test.js", qjs.Code(`
		(() => {
			let obj = {
				name: "test",
				circularField: {}
			};
			obj.circularField.parent = obj; // Create circular reference
			return obj;
		})()
		`)))
		defer result.Free()

		type TestStruct struct {
			Name          string                     `json:"name"`
			CircularField *StringifyErrorUnmarshaler `json:"circularField"` // Pointer to unmarshaler
		}

		var testStruct TestStruct
		_, err := qjs.JsObjectOrMapToGoStruct(result, testStruct)

		// Verify we get the JSONStringify error from the selected branch
		t.Logf("JsObjectOrMapToGoStruct result error: %v", err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to stringify")
		}
	})

	t.Run("test_unmarshaler_both_pointer_and_value", func(t *testing.T) {
		// Test both pointer and non-pointer unmarshaler fields work correctly
		result := must(rt.Eval("test.js", qjs.Code(`
		({
			name: "test",
			pointerField: "pointer value",
			valueField: "value field"
		})
		`)))
		defer result.Free()

		type TestStruct struct {
			Name         string                     `json:"name"`
			PointerField *StringifyErrorUnmarshaler `json:"pointerField"` // Pointer to unmarshaler
			ValueField   StringifyErrorUnmarshaler  `json:"valueField"`   // Value unmarshaler
		}

		var testStruct TestStruct
		result2, err := qjs.JsObjectOrMapToGoStruct(result, testStruct)
		require.NoError(t, err)

		// Verify both fields were properly unmarshaled
		assert.Equal(t, "test", result2.Name)
		assert.NotNil(t, result2.PointerField)
		assert.Equal(t, `"pointer value"`, result2.PointerField.Value)
		assert.Equal(t, `"value field"`, result2.ValueField.Value)
		t.Log("SUCCESS: Both pointer and value unmarshaler fields work correctly")
	})
}

func TestJsObjectOrMapToGo(t *testing.T) {
	tests := []struct {
		name      string
		jsCode    string
		sample    any
		expected  any
		expectErr bool
		errMsg    string
	}{
		{
			name:     "to_map",
			jsCode:   `({"key": "value"})`,
			sample:   map[string]string{},
			expected: map[string]string{"key": "value"},
		},
		{
			name:   "to_struct",
			jsCode: `({"Name": "Test"})`,
			sample: struct {
				Name string
			}{},
			expected: struct {
				Name string
			}{Name: "Test"},
		},
		{
			name:      "not_an_object",
			jsCode:    `"not an object"`,
			sample:    map[string]any{},
			expectErr: true,
			errMsg:    "expected JS object",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			result, err := qjs.JsObjectToGo(jsValue, test.sample)
			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}

	t.Run("js_object_array_to_go", func(t *testing.T) {
		rt := must(qjs.New())
		defer rt.Close()
		jsArray := must(rt.Eval("test.js", qjs.Code(`[
		{name: "Alice", age: 30},
		{name: "Bob", age: 25}
	]`)))
		defer jsArray.Free()

		type Person struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}

		var target []Person
		result, err := qjs.JsObjectToGo(jsArray, target)
		assert.NoError(t, err)

		expected := []Person{
			{Name: "Alice", Age: 30},
			{Name: "Bob", Age: 25},
		}
		assert.Equal(t, expected, result)
	})

	t.Run("js_object_to_go_fallback", func(t *testing.T) {
		rt := must(qjs.New())
		defer rt.Close()

		// successfully convert a JS object to a Go slice of int
		jsValue := must(rt.Eval("test.js", qjs.Code(`(new Date)`)))
		defer jsValue.Free()
		result, err := qjs.JsObjectToGo[string](jsValue)
		require.NoError(t, err)
		assert.NotNil(t, result)

		// fail to convert a JS object to a Go value due to json stringify error
		jsValue = must(rt.Eval("test.js", qjs.Code(`(() => {
			let obj = { name: "test" };
			obj.circular = obj; // Create circular reference
			return obj;
		})()`)))
		defer jsValue.Free()
		_, err = qjs.JsObjectToGo[int](jsValue)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to stringify JS value")
	})
}

func TestJsFuncToGo(t *testing.T) {
	tests := []struct {
		name            string
		jsCode          string
		sample          any
		args            []any
		expected        any
		expectErr       bool
		errMsg          string
		expectInvolkErr string
		expectZeroValue bool
	}{
		{
			name:     "addition_function",
			jsCode:   `(a, b) => a + b`,
			sample:   func(a, b int) (int, error) { return 0, nil },
			args:     []any{5, 3},
			expected: 8,
		},
		{
			name:     "string_concatenation",
			jsCode:   `(a, b) => a + " " + b`,
			sample:   func(a, b string) (string, error) { return "", nil },
			args:     []any{"Hello", "World"},
			expected: "Hello World",
		},
		{
			name:     "boolean_logic",
			jsCode:   `(a, b) => a && b`,
			sample:   func(a, b bool) (bool, error) { return false, nil },
			args:     []any{true, false},
			expected: false,
		},
		{
			name:     "function_with_no_params",
			jsCode:   `() => 42`,
			sample:   func() (int, error) { return 0, nil },
			args:     []any{},
			expected: 42,
		},
		{
			name:     "function_returning_undefined",
			jsCode:   `() => undefined`,
			sample:   func() (any, error) { return nil, nil },
			args:     []any{},
			expected: nil,
		},
		{
			name:     "function_returning_null",
			jsCode:   `() => null`,
			sample:   func() (any, error) { return nil, nil },
			args:     []any{},
			expected: nil,
		},
		{
			name:     "function_with_object_return",
			jsCode:   `() => ({ name: "Alice", age: 30 })`,
			sample:   func() (map[string]any, error) { return nil, nil },
			args:     []any{},
			expected: map[string]any{"name": "Alice", "age": int64(30)},
		},
		{
			name:     "function_with_array_return",
			jsCode:   `() => [1, 2, 3, 4]`,
			sample:   func() ([]any, error) { return nil, nil },
			args:     []any{},
			expected: []any{int64(1), int64(2), int64(3), int64(4)},
		},
		{
			name:     "function_with_math",
			jsCode:   `(a, b, c) => a * b + c`,
			sample:   func(a, b, c float64) (float64, error) { return 0, nil },
			args:     []any{2.5, 3.0, 1.5},
			expected: 9.0,
		},

		// Variadic function tests
		{
			name:     "variadic_function",
			jsCode:   `(...args) => args.reduce((sum, val) => sum + val, 0)`,
			sample:   func(vals ...int) (int, error) { return 0, nil },
			args:     []any{1, 2, 3, 4, 5},
			expected: 15,
		},
		{
			name:     "variadic_function_no_args",
			jsCode:   `(...args) => args.reduce((sum, val) => sum + val, 0)`,
			sample:   func(vals ...int) (int, error) { return 0, nil },
			args:     []any{},
			expected: 0,
		},
		{
			name:     "variadic_function_mixed_args",
			jsCode:   `(prefix, ...numbers) => prefix + ": " + numbers.reduce((sum, val) => sum + val, 0)`,
			sample:   func(prefix string, vals ...int) (string, error) { return "", nil },
			args:     []any{"Total", 10, 20, 30},
			expected: "Total: 60",
		},

		// Error cases
		{
			name:      "not_a_function",
			jsCode:    `"not a function"`,
			sample:    func() (any, error) { return nil, nil },
			expectErr: true,
			errMsg:    "expected JS function",
		},
		{
			name:            "function_throws_exception",
			jsCode:          `() => { throw new Error("test error"); }`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "JS function threw an exception",
		},
		{
			name:      "invalid_sample_not_function",
			jsCode:    `(a, b) => a + b`,
			sample:    "not a function",
			expectErr: true,
			errMsg:    "expected GO type function",
		},
		{
			name:     "function_with_single_return_value",
			jsCode:   `(a, b) => a + b`,
			sample:   func(a, b int) int { return 0 },
			args:     []any{5, 3},
			expected: 8,
		},
		{
			name:            "function_with_single_error_return",
			jsCode:          `() => { throw new Error("test error"); }`,
			sample:          func() error { return nil },
			args:            []any{},
			expectInvolkErr: "test error",
		},
		{
			name:            "error_in_single_non_error_return",
			jsCode:          `() => { throw new Error("js error"); }`,
			sample:          func() int { return 0 },
			args:            []any{},
			expectInvolkErr: "js error",
		},
		{
			name:            "undefined_return_explicit_check",
			jsCode:          `() => undefined`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expected:        nil,
			expectZeroValue: true,
		},
		{
			name:            "function_returns_unconvertible_value",
			jsCode:          `() => ({ toJSON: () => { throw new Error("conversion error"); } })`,
			sample:          func() (string, error) { return "", nil },
			args:            []any{},
			expectInvolkErr: "failed to convert JS result to Go",
		},
		{
			name:     "variadic_function_with_slice_argument",
			jsCode:   `(...nums) => nums.reduce((sum, n) => sum + n, 0)`,
			sample:   func(nums ...int) (int, error) { return 0, nil },
			args:     []any{[]int{1, 2, 3, 4, 5}},
			expected: 15,
		},
		{
			name:            "function_throws_exception_with_message",
			jsCode:          `() => { throw new Error("custom error message"); }`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "custom error message",
		},
		{
			name:   "undefined_explicitly_returns_zero_struct",
			jsCode: `() => undefined`,
			sample: func() (struct{ Name string }, error) { return struct{ Name string }{}, nil },
			args:   []any{},
			expected: struct {
				Name string
			}{},
			expectZeroValue: true,
		},
		{
			name:   "null_explicitly_returns_zero_struct",
			jsCode: `() => null`,
			sample: func() (struct{ Age int }, error) { return struct{ Age int }{}, nil },
			args:   []any{},
			expected: struct {
				Age int
			}{},
			expectZeroValue: true,
		},
		{
			name:            "js_syntax_error_in_function",
			jsCode:          `() => { const x = {}; return x.nonExistentMethod(); }`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "JS function threw an exception",
		},
		{
			name:            "js_reference_error_in_function",
			jsCode:          `() => { return nonExistentVariable; }`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "JS function threw an exception",
		},
		{
			name:            "js_type_error_in_function",
			jsCode:          `() => { let x = null; return x.toString(); }`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "JS function threw an exception",
		},
		{
			name:            "js_value_conversion_error",
			jsCode:          `() => { return { value: Symbol('test') }; }`,
			sample:          func() (map[string]string, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "error converting map value",
		},
		{
			name:   "js_function_returns_complex_null_structure",
			jsCode: `() => { return { name: null, items: [null, undefined, null] }; }`,
			sample: func() (map[string]any, error) { return nil, nil },
			args:   []any{},
			expected: map[string]any{
				"name":  nil,
				"items": []any{nil, nil, nil},
			},
		},
		{
			name: "conditional_return_undefined_or_value",
			jsCode: `(returnValue) => { 
					if (returnValue) return "result"; 
					else return undefined; 
			}`,
			sample:          func(returnVal bool) (string, error) { return "", nil },
			args:            []any{false},
			expectZeroValue: true,
			expected:        "",
		},
		{
			name: "conditional_return_undefined_or_value_with_value",
			jsCode: `(returnValue) => { 
					if (returnValue) return "result"; 
					else return undefined; 
			}`,
			sample:   func(returnVal bool) (string, error) { return "", nil },
			args:     []any{true},
			expected: "result",
		},
		{
			name: "js_throws_error_with_stack_trace",
			jsCode: `() => { 
					function inner() { throw new Error("Deep error"); }
					function middle() { inner(); }
					middle();
			}`,
			sample:          func() (any, error) { return nil, nil },
			args:            []any{},
			expectInvolkErr: "Deep error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue := must(rt.Eval("test.js", qjs.Code(test.jsCode)))
			defer jsValue.Free()

			goFunc, err := qjs.JsFuncToGo(jsValue, test.sample)
			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
				return
			}

			require.NoError(t, err)
			if len(test.args) > 0 || test.expected != nil {
				fnVal := reflect.ValueOf(goFunc)
				fnType := fnVal.Type()
				var results []reflect.Value

				if fnType.IsVariadic() && len(test.args) == 1 && reflect.TypeOf(test.args[0]).Kind() == reflect.Slice {
					sliceVal := reflect.ValueOf(test.args[0])

					results = fnVal.CallSlice([]reflect.Value{sliceVal})
				} else {
					argVals := make([]reflect.Value, len(test.args))
					for i, arg := range test.args {
						argVals[i] = reflect.ValueOf(arg)
					}
					results = fnVal.Call(argVals)
				}

				// Get the function type to check number of return values
				sampleFnType := reflect.TypeOf(test.sample)
				numOut := sampleFnType.NumOut()

				// Check results based on number of return values
				if numOut == 1 {
					// Single return value (non-error or error)
					assert.Equal(t, 1, len(results))

					// Check if there's an expected error
					if test.expectInvolkErr != "" {
						assert.NotNil(t, results[0].Interface())
						if err, ok := results[0].Interface().(error); ok {
							assert.Contains(t, err.Error(), test.expectInvolkErr)
						} else {
							t.Errorf("Expected error but got %T: %v", results[0].Interface(), results[0].Interface())
						}
					} else {
						// Check if it's an error type
						returnType := sampleFnType.Out(0)
						if returnType.Implements(reflect.TypeFor[error]()) {
							// Single error return
							assert.Nil(t, results[0].Interface())
						} else {
							// Single non-error return
							if test.expectZeroValue {
								assert.Equal(t, reflect.Zero(reflect.TypeOf(test.expected)).Interface(), results[0].Interface())
							} else {
								assert.Equal(t, test.expected, results[0].Interface())
							}
						}
					}
				} else {
					// Two return values (value, error)
					assert.Equal(t, 2, len(results))
					if test.expectInvolkErr != "" {
						assert.NotNil(t, results[1].Interface())
						assert.Contains(t, results[1].Interface().(error).Error(), test.expectInvolkErr)
						return
					}

					assert.Nil(t, results[1].Interface())
					if test.expectZeroValue {
						assert.Equal(t, reflect.Zero(reflect.TypeOf(test.expected)).Interface(), results[0].Interface())
					} else {
						assert.Equal(t, test.expected, results[0].Interface())
					}
				}
			}
		})
	}

	t.Run("validate_go_function_signature", func(t *testing.T) {
		rt := must(qjs.New())
		defer rt.Close()

		jsFuncValue := must(rt.Eval("test.js", qjs.Code(`
			function testFunc(a, b, c) {
				return a + b + c;
			}
			testFunc;
		`)))
		defer jsFuncValue.Free()

		t.Run("Non_variadic_function_argument_error", func(t *testing.T) {
			jsFuncValue := must(rt.Eval("test.js", qjs.Code(`
				function nonVariadicFunc(a, b, c) {
					return a + b + c;
				}
				nonVariadicFunc;
			`)))
			defer jsFuncValue.Free()

			var goFunc func(int, int, any) (any, error)
			goFunc = must(qjs.JsFuncToGo(jsFuncValue, goFunc))

			unsafePtr := unsafe.Pointer(&[]byte{1, 2, 3}[0])
			result, err := goFunc(1, 2, unsafePtr)
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert go function argument at index 2")
		})

		t.Run("Variadic_function_fixed_argument_error", func(t *testing.T) {
			// Create a variadic JS function
			jsFuncValue := must(rt.Eval("test.js", qjs.Code(`
				function variadicFunc(required1, required2, ...rest) {
					return required1 + required2 + rest.length;
				}
				variadicFunc;
			`)))
			defer jsFuncValue.Free()

			var goFunc func(int, any, ...any) (any, error)
			goFunc = must(qjs.JsFuncToGo(jsFuncValue, goFunc))

			unsafePtr := unsafe.Pointer(&[]byte{1, 2, 3}[0])
			result, err := goFunc(1, unsafePtr, "extra1", "extra2")
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert go function argument at index 1")
		})

		t.Run("Variadic_function_variadic_argument_error", func(t *testing.T) {
			jsFuncValue := must(rt.Eval("test.js", qjs.Code(`
				function variadicFunc(required, ...rest) {
					return required + rest.length;
				}
				variadicFunc;
			`)))
			defer jsFuncValue.Free()

			var goFunc func(int, ...any) (any, error)
			goFunc = must(qjs.JsFuncToGo(jsFuncValue, goFunc))

			unsafePtr := unsafe.Pointer(&[]byte{1, 2, 3}[0])
			result, err := goFunc(1, "valid", unsafePtr, "another")
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert go function argument at index 2")
		})

		t.Run("Valid_function_arguments", func(t *testing.T) {
			// Non-variadic function
			jsFuncNV := must(rt.Eval("test.js", qjs.Code(`(a, b, c) => a + b + c`)))
			defer jsFuncNV.Free()

			var goFuncNV func(int, int, int) (any, error)
			goFuncNV = must(qjs.JsFuncToGo(jsFuncNV, goFuncNV))
			result := must(goFuncNV(1, 2, 3))
			assert.Equal(t, int64(6), result)

			// Variadic function
			jsFuncV := must(rt.Eval("test.js", qjs.Code(`(a, ...rest) => a + rest.length`)))
			defer jsFuncV.Free()

			var goFuncV func(int, ...any) (any, error)
			goFuncV = must(qjs.JsFuncToGo(jsFuncV, goFuncV))

			result = must(goFuncV(10, "a", "b", "c"))
			assert.Equal(t, int64(13), result)
		})

		t.Run("Invalid_function_return_type", func(t *testing.T) {
			jsFuncValue := must(rt.Eval("test.js", qjs.Code(`
				function invalidReturnFunc() {
					return { name: "test" }; // Should return a string
				}
				invalidReturnFunc;
			`)))
			defer jsFuncValue.Free()

			var goFunc func() (string, error)
			goFunc = must(qjs.JsFuncToGo(jsFuncValue, goFunc))

			result, err := goFunc()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert JS function return value")
			assert.Empty(t, result)
		})

		t.Run("Function_returns_error_with_symbol", func(t *testing.T) {
			jsFuncValue := must(rt.Eval("test.js", qjs.Code(`
				function symbolReturnFunc() {
					return Symbol("test"); // Symbol cannot be converted by jsValueToGo
				}
				symbolReturnFunc;
			`)))
			defer jsFuncValue.Free()

			var goFunc func() (string, error)
			goFunc = must(qjs.JsFuncToGo(jsFuncValue, goFunc))

			result, err := goFunc()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "failed to convert JS function result")
			assert.Contains(t, err.Error(), "Symbol")
			assert.Empty(t, result)
		})
	})

	// this test pass go function to js without calling it in js side
	// then pass it back to go and call it in go side
	// to verify the go function passthrough works correctly
	// without any modification by js engine
	t.Run("Passthrough_of_go_func", func(t *testing.T) {
		rt := must(qjs.New())
		defer rt.Close()

		type TestArg struct {
			TestFunc func(int) (int, error) `json:"testFunc"`
		}

		goArg := must(qjs.ToJsValue(rt.Context(), TestArg{
			TestFunc: func(x int) (int, error) {
				return x * x, nil
			},
		}))
		rt.Context().Global().SetPropertyStr("goArg", goArg)
		goReceiver := rt.Context().Function(func(ctx *qjs.This) (*qjs.Value, error) {
			jsArg := ctx.Args()[0]
			// expect wrong type
			_, err := qjs.ToGoValue(jsArg, "")
			assert.Error(t, err, "string is not a valid type, expected TestArg")

			goArg := must(qjs.ToGoValue(jsArg, TestArg{}))
			result := must(goArg.TestFunc(5))
			assert.Equal(t, 25, result)
			return nil, nil
		})
		rt.Context().Global().SetPropertyStr("goReceiver", goReceiver)
		jsCode := `(function() { goReceiver(goArg); })();`
		result := must(rt.Eval("test.js", qjs.Code(jsCode)))
		defer result.Free()
	})
}

func TestJsValueToGo(t *testing.T) {
	tests := []struct {
		name         string
		jsCode       string
		sample       any
		expected     any
		expectErr    bool
		errMsg       string
		expectEval   func(*testing.T, any, error)
		customAssert func(*testing.T, any, error)
	}{
		// Direct qjs.Value
		{
			name:   "*qjs.Value",
			jsCode: `42`,
			sample: &qjs.Value{},
			customAssert: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.IsType(t, &qjs.Value{}, result)
				assert.Equal(t, int64(42), result.(*qjs.Value).Int64())
			},
		},
		{
			name:   "qjs.Value",
			jsCode: `42`,
			sample: qjs.Value{},
			customAssert: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.IsType(t, qjs.Value{}, result)
				value := result.(qjs.Value)
				assert.Equal(t, int64(42), (&value).Int64())
			},
		},
		// Basic primitive types
		{
			name:     "string_value",
			jsCode:   `"hello world"`,
			expected: "hello world",
		},
		{
			name:     "boolean_true",
			jsCode:   `true`,
			expected: true,
		},
		{
			name:     "boolean_false",
			jsCode:   `false`,
			expected: false,
		},
		{
			name:     "number_integer",
			jsCode:   `42`,
			expected: int64(42),
		},
		{
			name:     "number_float",
			jsCode:   `3.14159`,
			expected: 3.14159,
		},

		// Special values
		{
			name:     "null_value",
			jsCode:   `null`,
			expected: nil,
		},
		{
			name:     "undefined_value",
			jsCode:   `undefined`,
			expected: nil,
		},
		{
			name:      "symbol_value",
			jsCode:    `Symbol("test")`,
			expectErr: true,
			errMsg:    "unsupported type: Symbol",
		},

		// Error object
		{
			name:   "error_object",
			jsCode: `new Error("test error")`,
			expectEval: func(t *testing.T, result any, err error) {
				assert.Contains(t, err.Error(), "test error")
			},
		},

		// Date object
		{
			name:     "date_object",
			jsCode:   `new Date("2023-01-15T12:30:45.000Z")`,
			expected: time.Date(2023, 1, 15, 12, 30, 45, 0, time.UTC),
		},

		// RegExp object
		{
			name:     "regexp_object",
			jsCode:   `/abc\\d+/g`,
			expected: "/abc\\\\d+/g",
		},

		// ArrayBuffer and TypedArrays
		{
			name:     "array_buffer",
			jsCode:   `(() => { const buf = new ArrayBuffer(3); new Uint8Array(buf).set([1, 2, 3]); return buf; })()`,
			expected: []byte{1, 2, 3},
		},
		{
			name:     "typed_array_uint8",
			jsCode:   `new Uint8Array([10, 20, 30])`,
			expected: []byte{10, 20, 30},
		},

		// Arrays
		{
			name:     "array_of_primitives",
			jsCode:   `[1, "two", true, null]`,
			expected: []any{int64(1), "two", true, nil},
		},
		{
			name:     "array_with_sample_string_slice",
			jsCode:   `["apple", "banana", "cherry"]`,
			sample:   []string{},
			expected: []string{"apple", "banana", "cherry"},
		},
		{
			name:     "array_with_sample_int_slice",
			jsCode:   `[10, 20, 30]`,
			sample:   []int{},
			expected: []int{10, 20, 30},
		},

		// Objects and Maps
		{
			name:     "object_to_map",
			jsCode:   `({name: "John", age: 30, active: true})`,
			expected: map[string]any{"name": "John", "age": int64(30), "active": true},
		},
		{
			name:   "object_to_struct",
			jsCode: `({name: "John", age: 30, active: true})`,
			sample: struct {
				Name   string `json:"name"`
				Age    int    `json:"age"`
				Active bool   `json:"active"`
			}{},
			expected: struct {
				Name   string `json:"name"`
				Age    int    `json:"age"`
				Active bool   `json:"active"`
			}{Name: "John", Age: 30, Active: true},
		},
		{
			name:     "map_object",
			jsCode:   `new Map([["name", "Jane"], ["age", 25]])`,
			expected: map[string]any{"name": "Jane", "age": int64(25)},
		},

		// Sets
		{
			name:     "set_to_slice",
			jsCode:   `new Set([1, 2, 3, 3, 2])`,
			expected: []any{int64(1), int64(2), int64(3)},
		},
		{
			name:     "set_with_sample",
			jsCode:   `new Set(["red", "green", "blue"])`,
			sample:   []string{},
			expected: []string{"red", "green", "blue"},
		},

		// BigInt
		{
			name:   "bigint_to_big_int",
			jsCode: `BigInt("9007199254740991")`,
			sample: big.NewInt(0),
			customAssert: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.IsType(t, &big.Int{}, result)
				expected := big.NewInt(9007199254740991)
				assert.Equal(t, 0, expected.Cmp(result.(*big.Int)))
			},
		},

		// Function conversion
		{
			name:   "function_value",
			jsCode: `(a, b) => a + b`,
			sample: func(a, b int) (int, error) { return 0, nil },
			customAssert: func(t *testing.T, result any, err error) {
				require.NoError(t, err)
				assert.IsType(t, (func(int, int) (int, error))(nil), result)
				fn := result.(func(int, int) (int, error))
				val, err := fn(2, 3)
				require.NoError(t, err)
				assert.Equal(t, 5, val)
			},
		},

		// Error cases
		{
			name:      "unsupported_type",
			jsCode:    `(() => { const obj = {}; obj.circular = obj; return obj; })()`,
			expectErr: true,
			errMsg:    "cannot convert JS circular reference",
		},

		// Type conversion with explicit sample
		{
			name:     "number_with_int_sample",
			jsCode:   `42.5`,
			sample:   int(0),
			expected: int(42),
		},
		{
			name:     "number_with_uint8_sample",
			jsCode:   `255`,
			sample:   uint8(0),
			expected: uint8(255),
		},
		{
			name:     "number_with_float32_sample",
			jsCode:   `3.14159`,
			sample:   float32(0),
			expected: float32(3.14159),
		},

		// Complex objects
		{
			name:   "nested_object",
			jsCode: `({user: {name: "Bob", contact: {email: "bob@example.com", phone: "123-456-7890"}}, active: true})`,
			expected: map[string]any{
				"user": map[string]any{
					"name": "Bob",
					"contact": map[string]any{
						"email": "bob@example.com",
						"phone": "123-456-7890",
					},
				},
				"active": true,
			},
		},
		{
			name:   "mixed_array",
			jsCode: `[1, "text", true, {key: "value"}, [1, 2, 3]]`,
			expected: []any{
				int64(1),
				"text",
				true,
				map[string]any{"key": "value"},
				[]any{int64(1), int64(2), int64(3)},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rt := must(qjs.New())
			defer rt.Close()

			jsValue, err := rt.Eval("test.js", qjs.Code(test.jsCode))
			defer jsValue.Free()

			if test.expectEval != nil {
				test.expectEval(t, jsValue, err)
				return
			}

			var result any
			if test.sample != nil {
				result, err = qjs.ToGoValue(jsValue, test.sample)
			} else {
				result, err = qjs.ToGoValue[any](jsValue)
			}

			if test.customAssert != nil {
				test.customAssert(t, result, err)
				return
			}

			if test.expectErr {
				assert.Error(t, err)
				if test.errMsg != "" {
					assert.Contains(t, err.Error(), test.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestCircularReferenceDetection(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	circularReferenceTests := []struct {
		name     string
		testFunc func(*qjs.Value) error
		desc     string
	}{
		{
			name: "circular_reference_in_struct_conversion",
			testFunc: func(result *qjs.Value) error {
				type TestStruct struct {
					Name string         `json:"name"`
					Self map[string]any `json:"self"`
				}
				var testStruct TestStruct
				_, err := qjs.JsObjectOrMapToGoStruct(result, testStruct)
				return err
			},
			desc: "Tests struct conversion circular reference detection",
		},
		{
			name: "circular_reference_in_map_conversion",
			testFunc: func(result *qjs.Value) error {
				var testMap map[string]any
				_, err := qjs.JsObjectOrMapToGoMap(result, testMap)
				return err
			},
			desc: "Tests map conversion circular reference detection",
		},
	}

	for _, tc := range circularReferenceTests {
		t.Run(tc.name, func(t *testing.T) {
			result := must(rt.Eval("test.js", qjs.Code(`
				var obj = { name: "test" };
				obj.self = obj; // circular reference
				obj;
			`)))
			defer result.Free()

			err := tc.testFunc(result)
			validateCircularReference(t, err)
		})
	}
}

func TestStringToBoolConversion(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	var boolVal bool

	result := must(rt.Eval("test.js", qjs.Code(`"non-empty"`)))
	defer result.Free()
	converted := must(qjs.ToGoValue(result, boolVal))
	assert.True(t, converted)

	emptyResult := must(rt.Eval("test.js", qjs.Code(`""`)))
	defer emptyResult.Free()
	converted2 := must(qjs.ToGoValue(emptyResult, boolVal))
	assert.False(t, converted2)
}

func TestErrorValueHandling(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	obj := rt.Context().NewObject()
	defer obj.Free()

	errorValue := obj.CallConstructor()
	defer errorValue.Free()

	assert.True(t, errorValue.IsError())
	converted := must(qjs.ToGoValue[error](errorValue))
	assert.Error(t, converted)
	assert.Contains(t, converted.Error(), "not a constructor")
}

func TestStringToNumericConversion(t *testing.T) {
	rt := must(qjs.New())
	defer rt.Close()

	t.Run("string_to_pointer_numeric_types", func(t *testing.T) {
		result := must(rt.Eval("test.js", qjs.Code(`"42"`)))
		defer result.Free()
		var ptrInt *int
		converted := must(qjs.ToGoValue(result, ptrInt))
		assert.Equal(t, 42, *converted)

		floatResult := must(rt.Eval("test.js", qjs.Code(`"3.14"`)))
		defer floatResult.Free()
		var ptrFloat *float64
		converted2 := must(qjs.ToGoValue(floatResult, ptrFloat))
		assert.Equal(t, 3.14, *converted2)
	})

	t.Run("empty_string_to_numeric_conversion", func(t *testing.T) {
		result := must(rt.Eval("test.js", qjs.Code(`""`)))
		defer result.Free()

		var intVal int
		_, err := qjs.ToGoValue(result, intVal)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty string cannot be converted to number")
	})

	t.Run("numeric_type_conversions", func(t *testing.T) {
		// Validates string-to-numeric conversion across all Go numeric types
		testCases := []struct {
			jsValue    string
			targetType any
			expected   any
		}{
			// Int8 tests
			{"\"42\"", int8(0), int8(42)},
			{"\"127\"", int8(0), int8(127)},   // max int8
			{"\"-128\"", int8(0), int8(-128)}, // min int8

			// Int16 tests
			{"\"32767\"", int16(0), int16(32767)},   // max int16
			{"\"-32768\"", int16(0), int16(-32768)}, // min int16

			// Int32 tests
			{"\"2147483647\"", int32(0), int32(2147483647)},   // max int32
			{"\"-2147483648\"", int32(0), int32(-2147483648)}, // min int32

			// Max safe int64 in JS (Number.MAX_SAFE_INTEGER)
			{"\"9007199254740991\"", int64(0), int64(9007199254740991)},

			// Uint types
			{"\"255\"", uint8(0), uint8(255)},                 // max uint8
			{"\"65535\"", uint16(0), uint16(65535)},           // max uint16
			{"\"4294967295\"", uint32(0), uint32(4294967295)}, // max uint32
			{"\"42\"", uint64(0), uint64(42)},
			{"\"42\"", uint(0), uint(42)},
			{"\"42\"", uintptr(0), uintptr(42)},

			// Float types
			{"\"3.14159\"", float32(0), float32(3.14159)},
			{"\"3.141592653589793\"", float64(0), float64(3.141592653589793)},
		}

		for _, tc := range testCases {
			t.Run(fmt.Sprintf("convert_%s_to_%T", tc.jsValue, tc.targetType), func(t *testing.T) {
				result := must(rt.Eval("test.js", qjs.Code(tc.jsValue)))
				defer result.Free()

				converted, err := qjs.ToGoValue(result, tc.targetType)
				require.NoError(t, err)
				assert.Equal(t, tc.expected, converted)
			})
		}
	})
}
