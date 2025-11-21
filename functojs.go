package qjs

import (
	"fmt"
	"reflect"
)

// FuncToJS converts a Go function to a JavaScript function.
func FuncToJS(c *Context, v any) (_ *Value, err error) {
	if v == nil {
		return c.NewNull(), nil
	}

	defer func() {
		if err != nil {
			err = fmt.Errorf("[FuncToJS] %w", err)
		}
	}()

	rval := reflect.ValueOf(v)
	rtype := reflect.TypeOf(v)

	if rtype.Kind() == reflect.Ptr {
		if rval.IsNil() {
			return c.NewNull(), nil
		}

		rval = rval.Elem()
		rtype = rtype.Elem()
	}

	if rtype.Kind() != reflect.Func {
		return nil, newInvalidGoTypeErr("function", v)
	}

	if rval.IsNil() {
		return c.NewNull(), nil
	}

	fnType := rval.Type()
	if err := VerifyGoFunc(fnType, v); err != nil {
		return nil, err
	}

	return c.Function(func(this *This) (*Value, error) {
		goArgs, err := JsFuncArgsToGo(this.Args(), fnType)
		if err != nil {
			return nil, err
		}

		var results []reflect.Value
		if fnType.IsVariadic() {
			results = rval.CallSlice(goArgs)
		} else {
			results = rval.Call(goArgs)
		}

		return GoFuncResultToJs(c, results)
	}), nil
}

// JsFuncArgsToGo converts JS arguments to Go arguments in both variadic and non-variadic functions,
// filling missing arguments with zero values.
func JsFuncArgsToGo(jsArgs []*Value, fnType reflect.Type) ([]reflect.Value, error) {
	numGoArgs := fnType.NumIn()
	goArgs := make([]reflect.Value, 0, numGoArgs)
	numJSArgs := len(jsArgs)

	if fnType.IsVariadic() {
		fixedArgs := Min(numGoArgs-1, numJSArgs)
		for i := range fixedArgs {
			goVal, err := JsArgToGo(jsArgs[i], fnType.In(i))
			if err != nil {
				return nil, newArgConversionErr(i, err)
			}

			goArgs = append(goArgs, goVal)
		}

		// Handle variadic slice
		variadicSlice, err := CreateVariadicSlice(jsArgs[fixedArgs:], fnType.In(numGoArgs-1), fixedArgs)
		if err != nil {
			return nil, err
		}

		return append(goArgs, variadicSlice), nil
	}

	// Limit JS args to Go args count to avoid index out of bounds
	argsToProcess := Min(numJSArgs, numGoArgs)
	for i := range argsToProcess {
		goVal, err := JsArgToGo(jsArgs[i], fnType.In(i))
		if err != nil {
			return nil, newArgConversionErr(i, err)
		}

		goArgs = append(goArgs, goVal)
	}

	// Fill missing arguments with zero values
	for i := argsToProcess; i < numGoArgs; i++ {
		goArgs = append(goArgs, reflect.Zero(fnType.In(i)))
	}

	return goArgs, nil
}

// handlePointerArgument processes JS arguments for pointer types.
func handlePointerArgument(jsArg *Value, argType reflect.Type) (reflect.Value, error) {
	if jsArg.IsNull() || jsArg.IsUndefined() {
		return reflect.Zero(argType), nil
	}

	underlyingType := argType.Elem()
	zeroVal := reflect.New(underlyingType).Elem()

	goVal, err := ToGoValue(jsArg, zeroVal.Interface())
	if err != nil {
		return reflect.Value{}, newJsToGoErr(jsArg, err, "function param pointer to "+jsArg.Type())
	}

	ptrVal := reflect.New(underlyingType)
	ptrVal.Elem().Set(reflect.ValueOf(goVal))

	return ptrVal, nil
}

// CreateNonNilSample creates appropriate non-nil samples for types that have nil zero values.
func CreateNonNilSample(argType reflect.Type) any {
	switch argType.Kind() {
	case reflect.Interface:
		// return nil to let the conversion
		// use the default logic which can handle dynamic type inference
		return nil

	case reflect.Ptr:
		elemType := argType.Elem()
		elemZero := reflect.Zero(elemType)
		ptr := reflect.New(elemType)
		ptr.Elem().Set(elemZero)

		return ptr.Interface()

	case reflect.Array:
		return reflect.New(argType).Elem().Interface()

	case reflect.Slice:
		return reflect.MakeSlice(argType, 0, 0).Interface()

	case reflect.Map:
		return reflect.MakeMap(argType).Interface()

	case reflect.Chan:
		return reflect.MakeChan(argType, 1).Interface() // size 1 buffer to avoid blocking

	case reflect.Func:
		return createDummyFunction(argType)

	default:
		// For other types (shouldn't happen), return zero value
		return reflect.Zero(argType).Interface()
	}
}

// createDummyFunction creates a dummy function with the specified signature for type inference.
func createDummyFunction(funcType reflect.Type) any {
	fn := reflect.MakeFunc(funcType, func(_ []reflect.Value) []reflect.Value {
		results := make([]reflect.Value, funcType.NumOut())
		for i := range funcType.NumOut() {
			results[i] = reflect.Zero(funcType.Out(i))
		}

		return results
	})

	return fn.Interface()
}

// JsArgToGo converts a single JS argument to a Go value with enhanced type handling.
func JsArgToGo(jsArg *Value, argType reflect.Type) (reflect.Value, error) {
	if argType.Kind() == reflect.Ptr {
		return handlePointerArgument(jsArg, argType)
	}

	goZeroVal := CreateNonNilSample(argType)

	goVal, err := ToGoValue(jsArg, goZeroVal)
	if err != nil {
		return reflect.Value{}, newJsToGoErr(jsArg, err, "function param "+jsArg.Type())
	}

	return reflect.ValueOf(goVal), nil
}

// CreateVariadicSlice creates a reflect.Value slice for variadic arguments.
// Converts remaining JS arguments to the slice element type and returns as a slice value.
func CreateVariadicSlice(jsArgs []*Value, sliceType reflect.Type, fixedArgsCount int) (reflect.Value, error) {
	varArgType := sliceType.Elem()
	numVarArgs := len(jsArgs)
	variadicSlice := reflect.MakeSlice(sliceType, numVarArgs, numVarArgs)

	for i, jsArg := range jsArgs {
		goVal, err := JsArgToGo(jsArg, varArgType)
		if err != nil {
			return reflect.Value{}, newArgConversionErr(fixedArgsCount+i, err)
		}

		// Handle JavaScript null/undefined which result in invalid reflect.Value
		// Set zero value for both interface{}/any and concrete types
		if !goVal.IsValid() {
			variadicSlice.Index(i).Set(reflect.Zero(varArgType))
		} else {
			variadicSlice.Index(i).Set(goVal)
		}
	}

	return variadicSlice, nil
}

// GoFuncResultToJs processes Go function call results and converts them to JS values.
// If last return value is a non-nil error, it's thrown in JS context.
// The remaining return values are converted to JS value or JS array if there are multiple.
func GoFuncResultToJs(c *Context, results []reflect.Value) (*Value, error) {
	if len(results) == 0 {
		return nil, nil
	}

	// Check if last return value is an error
	lastIdx := len(results) - 1
	lastResult := results[lastIdx]

	// Last return is error
	if IsImplementError(lastResult.Type()) {
		// Last return is non-nil error -> throw in JS context
		if !lastResult.IsNil() {
			resultErr, _ := lastResult.Interface().(error)

			return nil, resultErr
		}

		// Error is nil, handle remaining return values
		remaining := results[:lastIdx]

		if len(remaining) == 0 {
			return nil, nil
		}

		// Single remaining value -> return that value
		if len(remaining) == 1 {
			return ToJsValue(c, remaining[0].Interface())
		}

		// Multiple remaining values -> return as JS array
		jsValues := make([]any, len(remaining))
		for i, result := range remaining {
			jsValues[i] = result.Interface()
		}

		return ToJsValue(c, jsValues)
	}

	// Single return value -> return that value
	if len(results) == 1 {
		return ToJsValue(c, results[0].Interface())
	}

	// Multiple return values -> return as JS array
	jsValues := make([]any, len(results))
	for i, result := range results {
		jsValues[i] = result.Interface()
	}

	return ToJsValue(c, jsValues)
}
