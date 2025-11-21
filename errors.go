package qjs

import (
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

var (
	ErrRType                   = reflect.TypeFor[error]()
	ErrZeroRValue              = reflect.Zero(ErrRType)
	ErrCallFuncOnNonObject     = errors.New("cannot call function on non-object")
	ErrNotAnObject             = errors.New("value is not an object")
	ErrObjectNotAConstructor   = errors.New("object not a constructor")
	ErrInvalidFileName         = errors.New("file name is required")
	ErrMissingProperties       = errors.New("value has no properties")
	ErrInvalidPointer          = errors.New("null pointer dereference")
	ErrIndexOutOfRange         = errors.New("index out of range")
	ErrNoNullTerminator        = errors.New("no NUL terminator")
	ErrInvalidContext          = errors.New("invalid context")
	ErrNotANumber              = errors.New("js value is not a number")
	ErrAsyncFuncRequirePromise = errors.New("jsFunctionProxy: async function requires a promise")
	ErrEmptyStringToNumber     = errors.New("empty string cannot be converted to number")
	ErrJsFuncDeallocated       = errors.New("js function context has been deallocated")
	ErrNotByteArray            = errors.New("invalid TypedArray: buffer is not a byte array")
	ErrNotArrayBuffer          = errors.New("input is not an ArrayBuffer")
	ErrMissingBufferProperty   = errors.New("invalid TypedArray: missing buffer property")
	ErrRuntimeClosed           = errors.New("runtime is closed")
	ErrNilModule               = errors.New("WASM module is nil")
	ErrNilHandle               = errors.New("handle is nil")
	ErrChanClosed              = errors.New("channel is closed")
	ErrChanSend                = errors.New("channel send would block: buffer full or no receiver ready")
	ErrChanReceive             = errors.New("channel receive would block: buffer empty or no sender ready")
	ErrChanCloseReceiveOnly    = errors.New("cannot close receive-only channel")
)

func combineErrors(errs ...error) error {
	if len(errs) == 0 {
		return nil
	}

	var errStr strings.Builder

	for _, err := range errs {
		if err != nil {
			errStr.WriteString(err.Error())
			errStr.WriteString("\n")
		}
	}

	return errors.New(errStr.String())
}

func newMaxLengthExceededErr(request uint, maxLen int64, index int) error {
	return fmt.Errorf("length %d exceeds max %d at index %d", request, maxLen, index)
}

func newOverflowErr(value any, targetType string) error {
	return fmt.Errorf("value %v overflows %s", value, targetType)
}

func newGoToJsErr(kind string, err error, details ...string) error {
	detail := ""
	if len(details) > 0 {
		detail = " " + details[0]
	}

	if err == nil {
		return fmt.Errorf("cannot convert Go%s '%s' to JS", detail, kind)
	}

	return fmt.Errorf("cannot convert Go%s '%s' to JS: %w", detail, kind, err)
}

func newJsToGoErr(kind *Value, err error, details ...string) error {
	detail := ""
	if len(details) > 0 {
		detail = " " + details[0]
	}

	kindStr := ""

	var kindErr error

	if kind != nil {
		kindStr, kindErr = kind.JSONStringify()
		if kindErr != nil {
			kindStr = fmt.Errorf("(%w), %s", kindErr, kind.String()).Error()
		}
	}

	if kindStr == "undefined" || kindStr == "null" {
		kindStr = kind.Type()
	}

	if kindStr != "" {
		kindStr = " " + kindStr
	}

	if err == nil {
		return fmt.Errorf("cannot convert JS%s%s to Go", detail, kindStr)
	}

	return fmt.Errorf("cannot convert JS%s%s to Go: %w", detail, kindStr, err)
}

func newArgConversionErr(index int, err error) error {
	return fmt.Errorf("cannot convert JS function argument at index %d: %w", index, err)
}

func newInvalidGoTypeErr(expect string, got any) error {
	return fmt.Errorf("expected GO type %s, got %T", expect, got)
}

func newInvalidJsInputErr(kind string, input *Value) (err error) {
	var detail string
	if detail, err = input.JSONStringify(); err != nil {
		detail = fmt.Sprintf("(JSONStringify failed: %v), (.String()) %s", err, input.String())
	}

	return fmt.Errorf("expected JS %s, got %s=%s", kind, input.Type(), detail)
}

func newJsStringifyErr(kind string, err error) error {
	return fmt.Errorf("js %s: %w", kind, err)
}

func newProxyErr(id uint64, r any) error {
	if err, ok := r.(error); ok {
		return fmt.Errorf("functionProxy [%d]: %w\n%s", id, err, debug.Stack())
	}

	if str, ok := r.(string); ok {
		return fmt.Errorf("functionProxy [%d]: %s\n%s", id, str, debug.Stack())
	}

	return fmt.Errorf("functionProxy [%d]: %v\n%s", id, r, debug.Stack())
}

func newInvokeErr(input *Value, err error) error {
	return fmt.Errorf("cannot call getTime on JS value '%s', err=%w", input.String(), err)
}
