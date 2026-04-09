package qjs_test

import (
	"errors"
	"math/big"
	"slices"
	"testing"
	"unsafe"

	"github.com/fastschema/qjs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueBasicOperations(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("NewUndefined", func(t *testing.T) {
		val := ctx.NewUndefined()
		defer val.Free()
		assert.True(t, val.IsUndefined())
		assert.False(t, val.IsNull())
	})

	t.Run("Clone", func(t *testing.T) {
		original := ctx.NewInt32(42)
		defer original.Free()

		clone := original.Clone()
		defer clone.Free()

		assert.Equal(t, original.Int32(), clone.Int32())
		assert.Equal(t, int32(42), clone.Int32())
	})

	t.Run("Handle", func(t *testing.T) {
		val := ctx.NewInt32(42)
		defer val.Free()

		assert.NotNil(t, val.Handle())
		assert.NotEqual(t, uint64(0), val.Raw())
	})

	t.Run("Raw_nil_value", func(t *testing.T) {
		var nilValue *qjs.Value = nil
		result := nilValue.Raw()
		assert.Equal(t, uint64(0), result, "nil Value should return 0 from Raw()")
	})

	t.Run("Raw_after_free", func(t *testing.T) {
		val := ctx.NewInt32(42)
		assert.NotEqual(t, uint64(0), val.Raw())
		val.Free()
		result := val.Raw()
		assert.True(t, result == 0)
	})

	t.Run("Context", func(t *testing.T) {
		val := ctx.NewInt32(42)
		defer val.Free()
		assert.Equal(t, ctx, val.Context())
	})
}

func TestValueTypeChecks(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("IsNumber", func(t *testing.T) {
		num := ctx.NewInt32(42)
		defer num.Free()
		assert.True(t, num.IsNumber())

		str := ctx.NewString("test")
		defer str.Free()
		assert.False(t, str.IsNumber())
	})

	t.Run("IsString", func(t *testing.T) {
		str := ctx.NewString("test")
		defer str.Free()
		assert.True(t, str.IsString())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsString())
	})

	t.Run("IsBool", func(t *testing.T) {
		b := ctx.NewBool(true)
		defer b.Free()
		assert.True(t, b.IsBool())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsBool())
	})

	t.Run("IsNull", func(t *testing.T) {
		n := ctx.NewNull()
		defer n.Free()
		assert.True(t, n.IsNull())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsNull())
	})

	t.Run("IsUndefined", func(t *testing.T) {
		u := ctx.NewUndefined()
		defer u.Free()
		assert.True(t, u.IsUndefined())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsUndefined())
	})

	t.Run("IsObject", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()
		assert.True(t, obj.IsObject())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsObject())
	})

	t.Run("IsArray", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("[]")))
		defer val.Free()
		assert.True(t, val.IsArray())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsArray())
	})

	t.Run("IsFunction", func(t *testing.T) {
		val := must(ctx.Eval(
			"test.js",
			qjs.Code(`
				function test() {};
				test;
			`),
		))
		defer val.Free()
		assert.True(t, val.IsFunction())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsFunction())
	})

	t.Run("IsConstructor", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("Array")))
		defer val.Free()
		assert.True(t, val.IsConstructor())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsConstructor())
	})

	t.Run("IsPromise", func(t *testing.T) {
		val := must(ctx.Eval(
			"test.js",
			qjs.Code(`({
				data: new Promise((resolve, reject) => resolve())
			})`),
		))
		defer val.Free()
		data := val.GetPropertyStr("data")
		assert.True(t, data.IsPromise())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsPromise())
	})

	t.Run("IsError", func(t *testing.T) {
		val, err := ctx.Eval("test.js", qjs.Code("new Error('test')"))
		defer val.Free()
		require.Error(t, err)

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsError())
	})

	t.Run("IsBigInt", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("BigInt(42)")))
		defer val.Free()
		assert.True(t, val.IsBigInt())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsBigInt())
	})

	t.Run("IsMap", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("new Map()")))
		defer val.Free()
		assert.True(t, val.IsMap())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsMap())
	})

	t.Run("IsSet", func(t *testing.T) {
		set := must(ctx.Eval("test.js", qjs.Code("new Set()")))
		defer set.Free()
		assert.True(t, set.IsSet())
		assert.False(t, set.IsMap())

		mapObj := must(ctx.Eval("test.js", qjs.Code("new Map()")))
		defer mapObj.Free()
		assert.False(t, mapObj.IsSet())
		assert.True(t, mapObj.IsMap())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsSet())
	})

	t.Run("IsByteArray", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("new ArrayBuffer(10)")))
		defer val.Free()
		assert.True(t, val.IsByteArray())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsByteArray())
	})

	t.Run("IsNaN", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("NaN")))
		defer val.Free()
		assert.True(t, val.IsNaN())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsNaN())
	})

	t.Run("IsInfinity", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("Infinity")))
		defer val.Free()
		assert.True(t, val.IsInfinity())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.False(t, num.IsInfinity())
	})

	t.Run("IsGlobalInstanceOf", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("new Date()")))
		defer val.Free()
		assert.True(t, val.IsGlobalInstanceOf("Date"))
		assert.False(t, val.IsGlobalInstanceOf("Array"))

		assert.False(t, val.IsGlobalInstanceOf("NonExistentConstructor"))
		assert.False(t, val.IsGlobalInstanceOf("FakeConstructor"))
		assert.False(t, val.IsGlobalInstanceOf(""))
	})
}

func TestValueIntrinsicKind(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	cases := []struct {
		name string
		code string
		want qjs.IntrinsicKind
	}{
		{name: "plain_object", code: `({ a: 1 })`, want: qjs.IntrinsicObject},
		{name: "array", code: `[1, 2, 3]`, want: qjs.IntrinsicArray},
		{name: "date", code: `new Date("2024-01-02T03:04:05Z")`, want: qjs.IntrinsicDate},
		{name: "regexp", code: `/abc/gi`, want: qjs.IntrinsicRegExp},
		{name: "map", code: `new Map([["a", 1]])`, want: qjs.IntrinsicMap},
		{name: "set", code: `new Set([1, 2])`, want: qjs.IntrinsicSet},
		{name: "array_buffer", code: `new ArrayBuffer(8)`, want: qjs.IntrinsicArrayBuffer},
		{name: "data_view", code: `new DataView(new ArrayBuffer(8))`, want: qjs.IntrinsicDataView},
		{name: "uint8_array", code: `new Uint8Array(new ArrayBuffer(8))`, want: qjs.IntrinsicUint8Array},
		{name: "big_uint64_array", code: `new BigUint64Array(new ArrayBuffer(16))`, want: qjs.IntrinsicBigUint64Array},
		{name: "spoofed_to_string_tag", code: `({ [Symbol.toStringTag]: "Map" })`, want: qjs.IntrinsicObject},
		{name: "spoofed_constructor_name", code: `({ constructor: { name: "Set" } })`, want: qjs.IntrinsicObject},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			val := must(ctx.Eval("test.js", qjs.Code(tc.code)))
			defer val.Free()
			assert.Equal(t, tc.want, val.IntrinsicKind())
		})
	}
}

func TestValueOwnPropertyDescriptor(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("data_property", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code(`({ a: 1 })`)))
		defer val.Free()

		desc, err := val.OwnPropertyDescriptor("a")
		require.NoError(t, err)
		assert.True(t, desc.Exists)
		assert.True(t, desc.Enumerable)
		assert.True(t, desc.Configurable)
		assert.True(t, desc.Writable)
		assert.True(t, desc.HasValue)
		assert.False(t, desc.IsAccessor())
	})

	t.Run("enumerable_accessor", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code(`(() => {
			const obj = {};
			Object.defineProperty(obj, "a", { enumerable: true, get() { return 1; } });
			return obj;
		})()`)))
		defer val.Free()

		desc, err := val.OwnPropertyDescriptor("a")
		require.NoError(t, err)
		assert.True(t, desc.Exists)
		assert.True(t, desc.Enumerable)
		assert.True(t, desc.HasGetter)
		assert.False(t, desc.HasValue)
		assert.True(t, desc.IsAccessor())
	})

	t.Run("non_enumerable_accessor", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code(`(() => {
			const obj = {};
			Object.defineProperty(obj, "a", { enumerable: false, get() { return 1; } });
			return obj;
		})()`)))
		defer val.Free()

		desc, err := val.OwnPropertyDescriptor("a")
		require.NoError(t, err)
		assert.True(t, desc.Exists)
		assert.False(t, desc.Enumerable)
		assert.True(t, desc.HasGetter)
		assert.True(t, desc.IsAccessor())
	})

	t.Run("null_prototype_object", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code(`(() => {
			const obj = Object.create(null);
			Object.defineProperty(obj, "a", { value: 1, enumerable: true, writable: true, configurable: true });
			return obj;
		})()`)))
		defer val.Free()

		desc, err := val.OwnPropertyDescriptor("a")
		require.NoError(t, err)
		assert.True(t, desc.Exists)
		assert.True(t, desc.Enumerable)
		assert.True(t, desc.Writable)
		assert.True(t, desc.Configurable)

		names, err := val.EnumerableOwnPropertyNames()
		require.NoError(t, err)
		assert.Equal(t, []string{"a"}, names)
	})
}

func TestValueConversions(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("String", func(t *testing.T) {
		val := ctx.NewString("hello")
		defer val.Free()
		assert.Equal(t, "hello", val.String())
	})

	t.Run("JSONStringify", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()

		objVal := ctx.NewString("world")
		obj.SetPropertyStr("hello", objVal)
		assert.JSONEq(t, `{"hello":"world"}`, must(obj.JSONStringify()))
	})

	t.Run("Bool", func(t *testing.T) {
		val := ctx.NewBool(true)
		defer val.Free()
		assert.True(t, val.Bool())

		val = ctx.NewBool(false)
		defer val.Free()
		assert.False(t, val.Bool())
	})

	t.Run("Int32", func(t *testing.T) {
		val := ctx.NewInt32(42)
		defer val.Free()
		assert.Equal(t, int32(42), val.Int32())
	})

	t.Run("Int64", func(t *testing.T) {
		val := ctx.NewInt64(1234567890123)
		defer val.Free()
		assert.Equal(t, int64(1234567890123), val.Int64())
	})

	t.Run("Uint32", func(t *testing.T) {
		val := ctx.NewUint32(42)
		defer val.Free()
		assert.Equal(t, uint32(42), val.Uint32())
	})

	t.Run("Float64", func(t *testing.T) {
		val := ctx.NewFloat64(3.14159)
		defer val.Free()
		assert.InDelta(t, 3.14159, val.Float64(), 0.0001)
	})

	t.Run("BigInt", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("BigInt('9007199254740991')")))
		defer val.Free()
		expected := new(big.Int)
		expected.SetString("9007199254740991", 10)
		assert.Equal(t, expected, val.BigInt())

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.Nil(t, num.BigInt())
	})

	t.Run("ToArray", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("[1, 2, 3]")))
		defer val.Free()
		arr := must(val.ToArray())
		assert.NotNil(t, arr)
		assert.Equal(t, int64(3), arr.Len())

		num := ctx.NewInt32(42)
		defer num.Free()
		_, err := num.ToArray()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected JS array, got Number=42")
	})

	t.Run("ToMap", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("new Map([['key', 'value']])")))
		defer val.Free()
		m := val.ToMap()
		assert.NotNil(t, m)

		num := ctx.NewInt32(42)
		defer num.Free()
		assert.Nil(t, num.ToMap())
	})

	t.Run("ToSet", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("new Set([1, 2, 3])")))
		defer val.Free()
		assert.NotNil(t, val.ToSet())

		num := ctx.NewInt32(42)
		defer num.Free()
		nilSet := num.ToSet()
		assert.Nil(t, nilSet)
	})

	t.Run("Object", func(t *testing.T) {
		val := must(ctx.Eval("test.js", qjs.Code("({ prop: 'value' })")))
		defer val.Free()
		obj := val.Object()
		defer obj.Free()
		assert.True(t, obj.IsObject())

		propVal := obj.GetPropertyStr("prop")
		defer propVal.Free()
		assert.Equal(t, "value", propVal.String())
	})

	t.Run("DateTime", func(t *testing.T) {
		// Normal date
		val := must(ctx.Eval("test.js", qjs.Code("new Date('2023-01-01T00:00:00Z')")))
		defer val.Free()

		dateTime := val.DateTime("UTC")
		assert.NotNil(t, dateTime)
		assert.Contains(t, dateTime.String(), "2023-01-01 00:00:00")

		// Non-Date object returns nil
		num := ctx.NewInt32(42)
		defer num.Free()
		assert.Nil(t, num.DateTime())

		// Unix epoch (1970-01-01 00:00:00 UTC)
		epochVal := must(ctx.Eval("test.js", qjs.Code("new Date(0)")))
		defer epochVal.Free()
		epochTime := epochVal.DateTime("UTC")
		assert.NotNil(t, epochTime, "Unix epoch (1970-01-01) should not return nil")
		assert.Equal(t, int64(0), epochTime.Unix(), "Unix epoch should have timestamp 0")
		assert.Equal(t, 1970, epochTime.Year(), "Unix epoch should be year 1970")
		assert.Equal(t, 1, int(epochTime.Month()), "Unix epoch should be January")
		assert.Equal(t, 1, epochTime.Day(), "Unix epoch should be day 1")

		// Invalid Date object (NaN)
		invalidDate := must(ctx.Eval("test.js", qjs.Code("new Date('invalid')")))
		defer invalidDate.Free()
		assert.Nil(t, invalidDate.DateTime(), "Invalid Date should return nil")

		// Different timezone
		localTime := epochVal.DateTime("America/New_York")
		assert.NotNil(t, localTime)
		// Unix epoch in EST/EDT should be different hour
		assert.NotEqual(t, epochTime.Hour(), localTime.Hour())

		// Default timezone (should use time.Local)
		defaultTz := epochVal.DateTime()
		assert.NotNil(t, defaultTz)

		// Empty timezone string (should use time.Local)
		emptyTz := epochVal.DateTime("")
		assert.NotNil(t, emptyTz)
	})
}

func TestValuePropertyOperations(t *testing.T) {
	runtime := must(qjs.New())
	defer runtime.Close()
	ctx := runtime.Context()

	t.Run("GetSetPropertyStr", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()

		val := ctx.NewString("value")
		defer val.Free()

		obj.SetPropertyStr("prop", val)

		prop := obj.GetPropertyStr("prop")
		defer prop.Free()

		assert.Equal(t, "value", prop.String())
	})

	t.Run("GetSetProperty", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()

		key := ctx.NewString("prop")
		defer key.Free()

		val := ctx.NewString("value")
		defer val.Free()

		obj.SetProperty(key, val)

		prop := obj.GetProperty(key)
		defer prop.Free()

		assert.Equal(t, "value", prop.String())
	})

	t.Run("GetSetPropertyIndex", func(t *testing.T) {
		arr := must(ctx.Eval("test.js", qjs.Code("[]")))
		defer arr.Free()

		val := ctx.NewString("value")
		defer val.Free()

		arr.SetPropertyIndex(0, val)

		prop := arr.GetPropertyIndex(0)
		defer prop.Free()

		assert.Equal(t, "value", prop.String())
	})

	t.Run("HasProperty", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()

		val := ctx.NewString("value")
		defer val.Free()

		obj.SetPropertyStr("prop", val)

		assert.True(t, obj.HasProperty("prop"))
		assert.False(t, obj.HasProperty("nonExistent"))
	})

	t.Run("HasPropertyIndex", func(t *testing.T) {
		arr := must(ctx.Eval("test.js", qjs.Code("['value']")))
		defer arr.Free()

		assert.True(t, arr.HasPropertyIndex(0))
		assert.False(t, arr.HasPropertyIndex(1))
	})

	t.Run("DeleteProperty", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()

		val := ctx.NewString("value")
		defer val.Free()

		obj.SetPropertyStr("prop", val)
		assert.True(t, obj.HasProperty("prop"))

		deleteResult := obj.DeleteProperty("prop")
		assert.True(t, deleteResult)
		assert.False(t, obj.HasProperty("prop"))
	})

	t.Run("GetOwnProperties", func(t *testing.T) {
		obj := must(ctx.Eval("test.js", qjs.Code(`({a: 1, b: 2})`)))
		defer obj.Free()

		props := obj.GetOwnProperties()
		assert.Len(t, props, 2)

		// Check if properties include 'a' and 'b'
		found := make(map[string]bool)
		for _, prop := range props {
			propName := prop.String()
			found[propName] = true
		}

		assert.True(t, found["a"])
		assert.True(t, found["b"])
	})

	t.Run("GetOwnPropertyNames", func(t *testing.T) {
		obj := must(ctx.Eval("test.js", qjs.Code(`({a: 1, b: 2})`)))
		defer obj.Free()

		names, err := obj.GetOwnPropertyNames()
		assert.NoError(t, err)
		assert.Len(t, names, 2)

		// Check if names include 'a' and 'b'
		found := make(map[string]bool)
		for _, name := range names {
			found[name] = true
		}

		assert.True(t, found["a"])
		assert.True(t, found["b"])
	})

	t.Run("ForEach", func(t *testing.T) {
		nonObject := ctx.NewInt32(42)
		defer nonObject.Free()
		nonObject.ForEach(func(key *qjs.Value, value *qjs.Value) {
			t.Fail()
		})

		obj := must(ctx.Eval("test.js", qjs.Code(`({a: 1, b: 2})`)))
		defer obj.Free()

		props := make(map[string]string)

		obj.ForEach(func(key *qjs.Value, value *qjs.Value) {
			props[key.String()] = value.String()
		})

		assert.Len(t, props, 2)
		assert.Equal(t, "1", props["a"])
		assert.Equal(t, "2", props["b"])
	})
}

func TestValueInvokeOperations(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("Invoke", func(t *testing.T) {
		obj := must(ctx.Eval("test.js", qjs.Code(`({
				add: function(a, b) { return a + b; }
		})`)))
		defer obj.Free()

		result, err := obj.Invoke("add", 5, 3)
		defer result.Free()

		assert.NoError(t, err)
		assert.Equal(t, int32(8), result.Int32())
	})

	t.Run("InvokeError", func(t *testing.T) {
		obj := must(ctx.Eval("test.js", qjs.Code(`({
				add: function(a, b) { return a + b; }
		})`)))
		defer obj.Free()

		var a any = 5
		var b any = unsafe.Pointer(&[]byte{1, 2, 3}[0])

		_, err := obj.Invoke("add", a, b)
		assert.Error(t, err)
	})

	t.Run("InvokeJS", func(t *testing.T) {
		obj := must(ctx.Eval("test.js", qjs.Code(`({
				add: function(a, b) { return a + b; }
		})`)))
		defer obj.Free()

		a := ctx.NewInt32(5)
		defer a.Free()

		b := ctx.NewInt32(3)
		defer b.Free()

		result, err := obj.InvokeJS("add", a, b)
		defer result.Free()

		assert.NoError(t, err)
		assert.Equal(t, int32(8), result.Int32())
	})

	t.Run("InvokeJSError", func(t *testing.T) {
		num := ctx.NewInt32(42)
		defer num.Free()

		_, err := num.InvokeJS("toString")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot call function on non-object")

		obj := ctx.NewObject()
		defer obj.Free()

		_, err = obj.InvokeJS("nonExistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a function")
	})

	t.Run("New", func(t *testing.T) {
		arrayConstructor := must(ctx.Eval("test.js", qjs.Code("Array")))
		defer arrayConstructor.Free()

		length := ctx.NewInt32(3)
		defer length.Free()

		newArray := arrayConstructor.New(length)
		defer newArray.Free()

		assert.True(t, newArray.IsArray())
		assert.Equal(t, int64(3), newArray.Len())
	})

	t.Run("CallConstructor", func(t *testing.T) {
		dateConstructor := must(ctx.Eval("test.js", qjs.Code("Date")))
		defer dateConstructor.Free()

		newDate := dateConstructor.CallConstructor()
		defer newDate.Free()

		assert.True(t, newDate.IsGlobalInstanceOf("Date"))
	})

	t.Run("ConstructorError", func(t *testing.T) {
		obj := ctx.NewObject()
		defer obj.Free()

		result := obj.CallConstructor()
		defer result.Free()

		assert.True(t, result.IsError())
	})
}

func TestValueArrayBufferOperations(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("ByteLen", func(t *testing.T) {
		buffer := must(ctx.Eval("test.js", qjs.Code("new ArrayBuffer(10)")))
		defer buffer.Free()

		assert.Equal(t, int64(10), buffer.ByteLen())
	})

	t.Run("ToByteArray", func(t *testing.T) {
		code := `
			const buffer = new ArrayBuffer(4);
			const view = new Uint8Array(buffer);
			view[0] = 10;
			view[1] = 20;
			view[2] = 30;
			view[3] = 40;
			buffer;
		`

		buffer := must(ctx.Eval("test.js", qjs.Code(code)))
		defer buffer.Free()

		bytes := buffer.ToByteArray()
		assert.Equal(t, []byte{10, 20, 30, 40}, bytes)
	})

	t.Run("Len", func(t *testing.T) {
		arr := must(ctx.Eval("test.js", qjs.Code("[1, 2, 3, 4, 5]")))
		defer arr.Free()

		assert.Equal(t, int64(5), arr.Len())
	})
}

func TestValueType(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	testCases := []struct {
		name         string
		code         string
		value        *qjs.Value
		expectedType string
		allowedTypes []string
	}{
		{"Symbol", "Symbol('test')", nil, "Symbol", nil},
		{"NaN", "NaN", nil, "NaN", nil},
		{"Infinity", "Infinity", nil, "Infinity", nil},
		{"BigInt", "123n", nil, "BigInt", nil},
		{"Number", "42", nil, "Number", nil},
		{"Date", "new Date()", nil, "Date", nil},
		{"Boolean", "true", nil, "Boolean", nil},
		{"Null", "null", nil, "Null", nil},
		{"Undefined", "undefined", nil, "Undefined", nil},
		{"String", "'hello'", nil, "String", nil},
		{"Array", "[]", nil, "Array", nil},
		{"Map", "new Map()", nil, "Map", nil},
		{"Set", "new Set()", nil, "Set", nil},
		{"Constructor", "Array", nil, "Constructor", []string{"Constructor Array"}},
		{"Function", "(() => 42)", nil, "Function", []string{"Constructor"}},
		{"ArrayBuffer", "new ArrayBuffer(8)", nil, "ArrayBuffer", nil},

		{"IsUninitialized", "", ctx.NewUninitialized(), "Uninitialized", nil},
		{"Error", "", ctx.NewError(errors.New("some error")), "Error", nil},
		{"ProxyValue", "", ctx.NewProxyValue(42), "QJSProxyValue", nil},

		{"unknown", "({})", nil, "unknown", nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var val *qjs.Value
			if tc.value != nil {
				val = tc.value
			} else {
				val = must(ctx.Eval("test.js", qjs.Code(tc.code)))
			}

			defer val.Free()

			actualType := val.Type()
			validTypes := append([]string{tc.expectedType}, tc.allowedTypes...)
			matched := slices.Contains(validTypes, actualType)

			assert.True(t, matched,
				"Code '%s' expected type in %v but got '%s'",
				tc.code, validTypes, actualType)
		})
	}

	t.Run("PromiseType", func(t *testing.T) {
		result := must(ctx.Eval("test.js", qjs.Code(`({
			promise: new Promise((resolve) => resolve(42))
		})`)))
		defer result.Free()

		promise := result.GetPropertyStr("promise")
		defer promise.Resolve()
		defer promise.Free()
		assert.Equal(t, "Promise", promise.Type())
	})
}

func TestValuePromiseOperations(t *testing.T) {
	rt, ctx := setupTestContext(t)
	defer rt.Close()

	t.Run("AwaitOnNonPromise", func(t *testing.T) {
		val := ctx.NewInt32(42)
		defer val.Free()
		_, err := val.Await()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "expected JS Promise")
	})

	t.Run("ResolveOnRegularPromise", func(t *testing.T) {
		result := must(ctx.Eval("test.js", qjs.Code(`({
			promise: new Promise((resolve) => resolve(42))
		})`)))
		defer result.Free()

		promise := result.GetPropertyStr("promise")
		assert.True(t, promise.IsPromise())
		val := must(promise.Await())
		defer val.Free()
		assert.Equal(t, int32(42), val.Int32())
	})

	t.Run("RejectOnRegularPromise", func(t *testing.T) {
		result := must(ctx.Eval("test.js", qjs.Code(`({
			promise: new Promise((resolve, reject) => reject(new Error("promise rejected")))
		})`)))
		defer result.Free()

		promise := result.GetPropertyStr("promise")
		assert.True(t, promise.IsPromise())
		_, err := promise.Await()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "promise rejected")
	})

	t.Run("AsyncGoFunctionResolveRejectError", func(t *testing.T) {
		rt, ctx := setupTestContext(t)
		defer rt.Close()
		ctx.SetAsyncFunc("testAsyncFunc", func(this *qjs.This) {
			val := this.Context().NewString("async result")
			this.Promise().Resolve(val)
		})

		// Call the async function and verify behavior
		result := must(ctx.Eval("test.js", qjs.Code(`({
			promise: testAsyncFunc()
		})`)))
		defer result.Free()

		promise := result.GetPropertyStr("promise")
		assert.True(t, promise.IsPromise())
		// remove resolve/reject from the promise
		promise.SetPropertyStr("resolve", nil)
		promise.SetPropertyStr("reject", nil)
		assert.Error(t, promise.Resolve())
		assert.Error(t, promise.Reject())
	})
}
