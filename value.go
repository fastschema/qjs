package qjs

import (
	"errors"
	"fmt"
	"math/big"
	"time"
	"unsafe"
)

type (
	Function      func(ctx *This) (*Value, error)
	AsyncFunction func(ctx *This)
	JSAtom        uint32
)

type Value struct {
	handle  *Handle
	context *Context
}

type This struct {
	*Value

	context *Context
	args    []*Value
	promise *Value
	isAsync bool
}

type JSPropertyEnum struct {
	isEnumerable bool
	atom         JSAtom
}

const jsPropertyEnumSize = uint32(unsafe.Sizeof(JSPropertyEnum{}))

const (
	JSPropConfigurable uint32 = 1 << 0
	JSPropWritable     uint32 = 1 << 1
	JSPropEnumerable   uint32 = 1 << 2
	JSPropHasGet       uint32 = 1 << 11
	JSPropHasSet       uint32 = 1 << 12
	JSPropHasValue     uint32 = 1 << 13

	JSGPNStringMask  uint32 = 1 << 0
	JSGPNSymbolMask  uint32 = 1 << 1
	JSGPNPrivateMask uint32 = 1 << 2
	JSGPNSetEnum     uint32 = 1 << 5
)

// Atom represents a JavaScript atom:
// Object property names and some strings are stored as Atoms (unique strings) to save memory and allow fast comparison.
type Atom struct {
	*Value

	context *Context
}

type OwnProperty struct {
	isEnumerable bool
	atom         Atom
}

type PropertyDescriptor struct {
	Exists       bool
	Enumerable   bool
	Configurable bool
	Writable     bool
	HasValue     bool
	HasGetter    bool
	HasSetter    bool
}

func (d PropertyDescriptor) IsAccessor() bool {
	return d.HasGetter || d.HasSetter
}

type IntrinsicKind uint32

const (
	IntrinsicUnknown IntrinsicKind = iota
	IntrinsicObject
	IntrinsicArray
	IntrinsicDate
	IntrinsicRegExp
	IntrinsicMap
	IntrinsicSet
	IntrinsicArrayBuffer
	IntrinsicDataView
	IntrinsicUint8Array
	IntrinsicUint8ClampedArray
	IntrinsicInt8Array
	IntrinsicUint16Array
	IntrinsicInt16Array
	IntrinsicUint32Array
	IntrinsicInt32Array
	IntrinsicBigInt64Array
	IntrinsicBigUint64Array
	IntrinsicFloat32Array
	IntrinsicFloat64Array
	IntrinsicFloat16Array
	IntrinsicError
)

func (k IntrinsicKind) String() string {
	switch k {
	case IntrinsicObject:
		return "Object"
	case IntrinsicArray:
		return "Array"
	case IntrinsicDate:
		return "Date"
	case IntrinsicRegExp:
		return "RegExp"
	case IntrinsicMap:
		return "Map"
	case IntrinsicSet:
		return "Set"
	case IntrinsicArrayBuffer:
		return "ArrayBuffer"
	case IntrinsicDataView:
		return "DataView"
	case IntrinsicUint8Array:
		return "Uint8Array"
	case IntrinsicUint8ClampedArray:
		return "Uint8ClampedArray"
	case IntrinsicInt8Array:
		return "Int8Array"
	case IntrinsicUint16Array:
		return "Uint16Array"
	case IntrinsicInt16Array:
		return "Int16Array"
	case IntrinsicUint32Array:
		return "Uint32Array"
	case IntrinsicInt32Array:
		return "Int32Array"
	case IntrinsicBigInt64Array:
		return "BigInt64Array"
	case IntrinsicBigUint64Array:
		return "BigUint64Array"
	case IntrinsicFloat32Array:
		return "Float32Array"
	case IntrinsicFloat64Array:
		return "Float64Array"
	case IntrinsicFloat16Array:
		return "Float16Array"
	case IntrinsicError:
		return "Error"
	default:
		return "Unknown"
	}
}

func (k IntrinsicKind) IsTypedArray() bool {
	switch k {
	case IntrinsicDataView,
		IntrinsicUint8Array,
		IntrinsicUint8ClampedArray,
		IntrinsicInt8Array,
		IntrinsicUint16Array,
		IntrinsicInt16Array,
		IntrinsicUint32Array,
		IntrinsicInt32Array,
		IntrinsicBigInt64Array,
		IntrinsicBigUint64Array,
		IntrinsicFloat32Array,
		IntrinsicFloat64Array,
		IntrinsicFloat16Array:
		return true
	default:
		return false
	}
}

func (p OwnProperty) String() string {
	return p.atom.String()
}

func (t *This) Context() *Context {
	return t.context
}

func (t *This) Args() []*Value {
	return t.args
}

func (t *This) Promise() *Value {
	return t.promise
}

func (t *This) IsAsync() bool {
	return t.isAsync
}

func (a Atom) Free() {
	a.context.Call("JS_FreeAtom", a.context.Raw(), a.Raw())
}

func (a Atom) String() string {
	result := a.context.Call("QJS_AtomToCString", a.context.Raw(), a.Raw())
	defer result.handle.Free()

	return result.handle.String()
}

func (a Atom) ToValue() *Value {
	return a.context.Call("JS_AtomToValue", a.context.Raw(), a.Raw())
}

func (v *Value) Handle() *Handle {
	return v.handle
}

func (v *Value) Raw() uint64 {
	if v == nil || v.handle == nil {
		return 0
	}

	return v.handle.raw
}

func (v *Value) Free() {
	if v != nil && v.Raw() != 0 {
		v.context.FreeJsValue(v.handle.raw)
		v.handle.raw = 0
	}
}

func (v *Value) Context() *Context {
	return v.context
}

func (v *Value) Ctx() uint64 {
	return v.context.Raw()
}

func (v *Value) Call(name string, args ...uint64) *Value {
	return v.context.Call(name, args...)
}

func (v *Value) Clone() *Value {
	return v.Call("QJS_CloneValue", v.Ctx(), v.Raw())
}

func (v *Value) Type() string {
	// Check Symbol first, as it is a special case
	if v.IsSymbol() {
		return "Symbol"
	}

	if v.IsQJSProxyValue() {
		return "QJSProxyValue"
	}

	if v.IsNaN() {
		return "NaN"
	}

	if v.IsInfinity() {
		return "Infinity"
	}

	if v.IsBigInt() {
		return "BigInt"
	}

	if v.IsNumber() {
		return "Number"
	}

	if v.IsDate() {
		return "Date"
	}

	if v.IsBool() {
		return "Boolean"
	}

	if v.IsNull() {
		return "Null"
	}

	if v.IsUndefined() {
		return "Undefined"
	}

	if v.IsUninitialized() {
		return "Uninitialized"
	}

	if v.IsString() {
		return "String"
	}

	if v.IsError() {
		return "Error"
	}

	if v.IsArray() {
		return "Array"
	}

	if v.IsMap() {
		return "Map"
	}

	if v.IsSet() {
		return "Set"
	}

	// Check Promise before Function and Constructor.
	if v.IsPromise() {
		return "Promise"
	}

	// Check Constructor before Function.
	if v.IsConstructor() {
		name := v.GetPropertyStr("name")
		defer name.Free()

		constructorName := ""
		if name.IsString() && name.String() != "" {
			constructorName = " " + name.String()
		}

		return "Constructor" + constructorName
	}

	if v.IsFunction() {
		return "Function"
	}

	if v.IsByteArray() {
		return "ArrayBuffer"
	}

	if kind := v.IntrinsicKind(); kind.IsTypedArray() {
		return kind.String()
	}

	return "unknown"
}

func (v *Value) NewUndefined() *Value {
	return v.context.NewUndefined()
}

// GetOwnPropertyNames returns the names of the properties of the value.
func (v *Value) GetOwnPropertyNames() (_ []string, err error) {
	pList := v.getOwnProperties(JSGPNStringMask | JSGPNSymbolMask | JSGPNPrivateMask | JSGPNSetEnum)

	names := make([]string, len(pList))

	for i := range names {
		names[i] = pList[i].String()
	}

	return names, nil
}

func (v *Value) GetOwnProperties() []OwnProperty {
	return v.getOwnProperties(JSGPNStringMask | JSGPNSymbolMask | JSGPNPrivateMask | JSGPNSetEnum)
}

func (v *Value) EnumerableOwnPropertyNames() ([]string, error) {
	props := v.getOwnProperties(JSGPNStringMask | JSGPNSetEnum)
	names := make([]string, 0, len(props))
	for _, prop := range props {
		if !prop.isEnumerable {
			continue
		}
		names = append(names, prop.String())
	}
	return names, nil
}

func (v *Value) getOwnProperties(flags uint32) []OwnProperty {
	ptr, entriesCount := v.context.CallUnPack(
		"QJS_GetOwnPropertyNames",
		v.Ctx(),
		v.Raw(),
		uint64(flags),
	)
	if entriesCount == 0 {
		return []OwnProperty{}
	}

	// Block size: number of entries * sizeof(JSPropertyEnum)
	blockSize := entriesCount * jsPropertyEnumSize
	bytes := v.context.MemRead(ptr, uint64(blockSize))

	// SAFETY: This converts C memory layout to Go structs.
	// The memory comes from QJS C code and matches JSPropertyEnum layout.
	// This is safe because:
	// 1. Memory size is validated (size * jsPropertyEnumSize)
	// 2. JSPropertyEnum layout matches C struct layout
	// 3. Memory lifetime is managed by context.FreeHandle()
	entries := unsafe.Slice((*JSPropertyEnum)(unsafe.Pointer(&bytes[0])), entriesCount)

	property := make([]OwnProperty, len(entries))

	for i, entry := range entries {
		property[i].isEnumerable = entry.isEnumerable
		property[i].atom = Atom{
			context: v.context,
			Value: v.context.NewValue(NewHandle(
				v.context.runtime,
				uint64(entry.atom),
			)),
		}
	}

	v.context.FreeHandle(uint64(ptr))

	return property
}

func (v *Value) OwnPropertyDescriptor(name string) (PropertyDescriptor, error) {
	atom := v.context.NewAtom(name)
	defer atom.Free()

	raw := v.Call("QJS_GetOwnPropertyFlags", v.Ctx(), v.Raw(), atom.Raw()).Handle().Uint64()
	status := uint32(raw >> 32)
	flags := uint32(raw)

	switch status {
	case 0:
		return PropertyDescriptor{}, nil
	case 1:
		return PropertyDescriptor{
			Exists:       true,
			Enumerable:   flags&JSPropEnumerable != 0,
			Configurable: flags&JSPropConfigurable != 0,
			Writable:     flags&JSPropWritable != 0,
			HasValue:     flags&JSPropHasValue != 0,
			HasGetter:    flags&JSPropHasGet != 0,
			HasSetter:    flags&JSPropHasSet != 0,
		}, nil
	default:
		if v.context.HasException() {
			return PropertyDescriptor{}, v.context.Exception()
		}
		return PropertyDescriptor{}, errors.New("failed to inspect own property descriptor")
	}
}

func (v *Value) IntrinsicKind() IntrinsicKind {
	if v == nil || !v.IsObject() {
		return IntrinsicUnknown
	}
	return IntrinsicKind(v.Call("QJS_GetIntrinsicKind", v.Ctx(), v.Raw()).Uint32())
}

func (v *Value) GetProperty(name *Value) *Value {
	atom := v.Call("JS_ValueToAtom", v.Ctx(), name.Raw())

	return v.Call("JS_GetProperty", v.Ctx(), v.Raw(), atom.Raw())
}

func (v *Value) SetProperty(name, val *Value) {
	atom := v.Call("JS_ValueToAtom", v.Ctx(), name.Raw())
	v.Call("JS_SetProperty", v.Ctx(), v.Raw(), atom.Raw(), val.Raw())
}

// GetPropertyStr returns the value of the property with the given name.
func (v *Value) GetPropertyStr(name string) *Value {
	nameVal := v.context.NewStringHandle(name)
	defer v.context.FreeHandle(nameVal.Raw())

	return v.Call("JS_GetPropertyStr", v.Ctx(), v.Raw(), nameVal.Raw())
}

// SetPropertyStr sets the value of the property with the given name.
func (v *Value) SetPropertyStr(name string, val *Value) {
	if val != nil {
		nameVal := v.context.NewStringHandle(name)
		v.Call("JS_SetPropertyStr", v.Ctx(), v.Raw(), nameVal.Raw(), val.Raw())
	}
}

// HasPropertyIndex returns true if the value has the property with the given index.
func (v *Value) HasPropertyIndex(index int64) bool {
	prop := v.context.NewAtomIndex(index)
	defer prop.Free()

	return v.Call("JS_HasProperty", v.Ctx(), v.Raw(), prop.Raw()).Bool()
}

// HasProperty returns true if the value has the property with the given name.
func (v *Value) HasProperty(name string) bool {
	prop := v.context.NewAtom(name)
	defer prop.Free()

	return v.Call("JS_HasProperty", v.Ctx(), v.Raw(), prop.Raw()).Bool()
}

// DeleteProperty deletes the property with the given name.
func (v *Value) DeleteProperty(name string) bool {
	prop := v.context.NewAtom(name)
	defer prop.Free()

	return v.Call("JS_DeleteProperty", v.Ctx(), v.Raw(), prop.Raw(), uint64(1)).Bool()
}

// Invoke call the object's method with the given name and arguments.
func (v *Value) Invoke(fname string, args ...any) (_ *Value, err error) {
	jsArgs := make([]*Value, len(args))
	for i, arg := range args {
		if jsArgs[i], err = ToJsValue(v.context, arg); err != nil {
			return nil, err
		}
		defer jsArgs[i].Free()
	}

	return v.InvokeJS(fname, jsArgs...)
}

// InvokeJS call the object's method with the given name and JS arguments.
func (v *Value) InvokeJS(fname string, args ...*Value) (*Value, error) {
	if !v.IsObject() {
		return v.NewUndefined(), ErrCallFuncOnNonObject
	}

	fn := v.GetPropertyStr(fname)
	defer fn.Free()

	if !fn.IsFunction() {
		return v.NewUndefined(), fmt.Errorf("JS property '%s' is not a function", fname)
	}

	argc, argvPtr := createJsCallArgs(v.context, args...)
	defer v.context.FreeHandle(argvPtr)

	jsCallArgs := []uint64{v.Ctx(), fn.Raw(), v.Raw(), argc, argvPtr}
	result := v.Call("QJS_Call", jsCallArgs...)

	return normalizeJsValue(v.context, result)
}

// SetPropertyIndex sets the value of the property with the given index.
func (v *Value) SetPropertyIndex(index int64, val *Value) {
	v.Call("JS_SetPropertyUint32", v.Ctx(), v.Raw(), uint64(index), val.Raw())
}

// GetPropertyIndex returns the value of the property with the given index.
func (v *Value) GetPropertyIndex(index int64) *Value {
	return v.Call("QJS_GetPropertyUint32", v.Ctx(), v.Raw(), uint64(index))
}

// Len returns the length of the array.
func (v *Value) Len() int64 {
	l := v.GetPropertyStr("length")
	defer l.Free()

	return l.Int64()
}

// ByteLen returns the length of the ArrayBuffer.
func (v *Value) ByteLen() int64 {
	return v.GetPropertyStr("byteLength").Int64()
}

// ToByteArray returns the byte array of the ArrayBuffer.
func (v *Value) ToByteArray() []byte {
	v2 := v.Clone()

	result := v2.context.Call("QJS_GetArrayBuffer", v2.context.Raw(), v2.Raw())
	defer result.Free()

	return result.handle.Bytes()
}

func (v *Value) Exception() error {
	cause := v.String()

	stack := v.GetPropertyStr("stack")
	defer stack.Free()

	if stack.IsUndefined() {
		return errors.New(cause)
	}

	stackStr := stack.String()

	return errors.New(cause + "\n" + stackStr)
}

func (v *Value) IsNumber() bool {
	return v.Call("QJS_IsNumber", v.Raw()).handle.Bool()
}

func (v *Value) IsNaN() bool {
	return v.String() == "NaN"
}

func (v *Value) IsInfinity() bool {
	return v.String() == "Infinity"
}

func (v *Value) IsBigInt() bool {
	return v.Call("QJS_IsBigInt", v.Raw()).handle.Bool()
}

func (v *Value) IsDate() bool {
	return v.Call("JS_IsDate", v.Raw()).handle.Bool()
}

func (v *Value) IsBool() bool {
	return v.Call("QJS_IsBool", v.Raw()).handle.Bool()
}

func (v *Value) IsNull() bool {
	return v.Call("QJS_IsNull", v.Raw()).handle.Bool()
}

func (v *Value) IsUndefined() bool {
	return v.Call("QJS_IsUndefined", v.Raw()).handle.Bool()
}

// func (v *Value) IsException() bool {
// 	return v.Call("QJS_IsException", v.Raw()).handle.Bool()
// }

func (v *Value) IsUninitialized() bool {
	return v.Call("QJS_IsUninitialized", v.Raw()).handle.Bool()
}

func (v *Value) IsString() bool {
	return v.Call("QJS_IsString", v.Raw()).handle.Bool()
}

func (v *Value) IsSymbol() bool {
	return v.Call("QJS_IsSymbol", v.Raw()).handle.Bool()
}

func (v *Value) IsQJSProxyValue() bool {
	return v.IsObject() && v.IsGlobalInstanceOf("QJS_PROXY_VALUE")
}

func (v *Value) IsObject() bool {
	return v.Call("QJS_IsObject", v.Raw()).handle.Bool()
}

func (v *Value) IsArray() bool {
	return v.Call("QJS_IsArray", v.Raw()).handle.Bool()
}

func (v *Value) IsError() bool {
	return v.Call("QJS_IsError", v.Ctx(), v.Raw()).handle.Bool()
}

func (v *Value) IsFunction() bool {
	return v.Call("QJS_IsFunction", v.Ctx(), v.Raw()).handle.Bool()
}

func (v *Value) IsConstructor() bool {
	return v.Call("QJS_IsConstructor", v.Ctx(), v.Raw()).handle.Bool()
}

func (v *Value) IsPromise() bool {
	return v.Call("QJS_IsPromise", v.Ctx(), v.Raw()).handle.Bool()
}

// Resolve resolves a promise with the given arguments.
// This method is intended for use with Go function bindings (this.Promise() in async Go functions).
// It will NOT work with native JavaScript promises created via "new Promise()".
// For native JS promises, use direct function calls or Promise.withResolvers instead.
func (v *Value) Resolve(args ...*Value) error {
	if v.IsPromise() {
		result, err := v.InvokeJS("resolve", args...)
		if err != nil {
			return err
		}

		result.Free()
	}

	return nil
}

// Reject rejects a promise with the given arguments.
// This method is intended for use with Go function bindings (this.Promise() in async Go functions).
// It will NOT work with native JavaScript promises created via "new Promise()".
// For native JS promises, use direct function calls or Promise.withResolvers instead.
func (v *Value) Reject(args ...*Value) error {
	if v.IsPromise() {
		result, err := v.InvokeJS("reject", args...)
		if err != nil {
			return err
		}

		result.Free()
	}

	return nil
}

func (v *Value) Await() (*Value, error) {
	if !v.IsPromise() {
		return nil, newInvalidJsInputErr("Promise", v)
	}

	result := v.Call("js_std_await", v.Ctx(), v.Raw())

	return normalizeJsValue(v.context, result)
}

func (v *Value) IsMap() bool {
	return v.IntrinsicKind() == IntrinsicMap
}

func (v *Value) IsSet() bool {
	return v.IntrinsicKind() == IntrinsicSet
}

// IsGlobalInstanceOf checks if the value is an instance of the given global constructor.
func (v *Value) IsGlobalInstanceOf(name string) bool {
	ctor := v.context.Global().GetPropertyStr(name)
	defer ctor.Free()

	if ctor.IsUndefined() {
		return false
	}

	instanceOf := v.Call("QJS_IsInstanceOf", v.Ctx(), v.Raw(), ctor.Raw())

	return instanceOf.handle.Bool()
}

// IsByteArray return true if the value is array buffer.
func (v *Value) IsByteArray() bool {
	return v.IntrinsicKind() == IntrinsicArrayBuffer
}

func (v *Value) IsRegExp() bool {
	return v.IntrinsicKind() == IntrinsicRegExp
}

// Object returns the object value of the value.
func (v *Value) Object() *Value {
	return v.Call("JS_ToObject", v.Ctx(), v.Raw())
}

// ForEach iterates over the properties of the object and calls the given function for each property.
func (v *Value) ForEach(fn func(key *Value, value *Value)) {
	if !v.IsObject() {
		return
	}

	props := v.GetOwnProperties()
	for _, prop := range props {
		key := prop.atom

		keyValue := key.ToValue()
		if keyValue.String() == "length" {
			keyValue.Free()

			continue // Skip the length property
		}

		value := v.GetProperty(keyValue)
		fn(keyValue, value)
		key.Free()

		if !value.IsFunction() {
			value.Free()
		}
	}
}

func (v *Value) Bytes() []byte {
	return v.handle.Bytes()
}

func (v *Value) String() string {
	result := v.Call("QJS_ToCString", v.Ctx(), v.Raw())
	defer result.handle.Free()

	return result.handle.String()
}

// JSONStringify returns the JSON string representation of the value.
func (v *Value) JSONStringify() (_ string, err error) {
	defer func() {
		r := AnyToError(recover())
		if r != nil {
			err = fmt.Errorf("failed to stringify JS value: %w", r)
		}
	}()

	result := v.Call("QJS_JSONStringify", v.Ctx(), v.Raw())
	defer result.handle.Free()

	return result.handle.String(), nil
}

// DateTime returns the date value of the value.
func (v *Value) DateTime(tzs ...string) *time.Time {
	var loc *time.Location
	if len(tzs) > 0 && tzs[0] != "" {
		loc = ParseTimezone(tzs[0])
	} else {
		loc = time.Local
	}

	if !v.IsDate() {
		return nil
	}

	epochValue := v.Call("QJS_ToEpochTime", v.Ctx(), v.Raw())
	defer epochValue.Free()

	epoch := epochValue.handle.Float64()

	// Check for NaN which indicates an invalid Date
	// Note: NaN != NaN is true, so this checks for NaN
	if epoch != epoch {
		return nil
	}

	epochNs := int64(epoch * 1e6)
	t := time.Unix(0, epochNs).In(loc)

	return &t
}

// Bool returns the boolean value of the value.
func (v *Value) Bool() bool {
	return v.Call("JS_ToBool", v.Ctx(), v.Raw()).handle.Bool()
}

// Int32 returns the int32 value of the value.
// in c int is 32 bit, but in go it is depends on the architecture.
func (v *Value) Int32() int32 {
	return v.Call("QJS_ToInt32", v.Ctx(), v.Raw()).handle.Int32()
}

func (v *Value) Int64() int64 {
	return v.Call("QJS_ToInt64", v.Ctx(), v.Raw()).handle.Int64()
}

// Uint32 returns the uint32 value of the value.
func (v *Value) Uint32() uint32 {
	return v.Call("QJS_ToUint32", v.Ctx(), v.Raw()).handle.Uint32()
}

// Float64 returns the float64 value of the value.
func (v *Value) Float64() float64 {
	return v.Call("QJS_ToFloat64", v.Ctx(), v.Raw()).handle.Float64()
}

// BigInt returns the big.Int value of the value.
func (v *Value) BigInt() *big.Int {
	if !v.IsBigInt() {
		return nil
	}

	val, _ := new(big.Int).SetString(v.String(), 10)

	return val
}

// ToArray returns the array value of the value. DO NOT FREE.
func (v *Value) ToArray() (*Array, error) {
	if !v.IsArray() {
		return nil, fmt.Errorf("expected JS array, got %s=%s", v.Type(), v.String())
	}

	return NewArray(v), nil
}

// ToMap returns the map value of the value. DO NOT FREE.
func (v *Value) ToMap() *Map {
	if !v.IsMap() {
		return nil
	}

	return NewMap(v)
}

// ToSet returns the set value of the value. DO NOT FREE.
func (v *Value) ToSet() *Set {
	if !v.IsSet() {
		return nil
	}

	return NewSet(v)
}

// New creates a new instance of the value as a constructor with the given arguments.
func (v *Value) New(args ...*Value) *Value {
	return v.CallConstructor(args...)
}

// CallConstructor calls the constructor with the given arguments.
func (v *Value) CallConstructor(args ...*Value) *Value {
	if !v.IsConstructor() {
		return v.context.NewError(ErrObjectNotAConstructor)
	}

	argc, argvPtr := createJsCallArgs(v.context, args...)
	defer v.context.FreeHandle(argvPtr)

	jsCallArgs := []uint64{v.Ctx(), v.Raw(), argc, argvPtr}

	return v.Call("JS_CallConstructor", jsCallArgs...)
}
