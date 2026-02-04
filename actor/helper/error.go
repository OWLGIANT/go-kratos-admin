package helper

import (
	"errors"
	"fmt"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrDelisted       = errors.New("delisted") // 下架
)

type ApiError struct {
	NetworkError error
	HandlerError error
	// isSuccess    bool // 证伪，明确成功才是成功
}

var (
	ApiErrorNil      ApiError
	ApiErrorDelisted = ApiError{
		NetworkError: nil,
		HandlerError: ErrDelisted,
	}
	ApiErrorNotImplemented = ApiError{
		NetworkError: nil,
		HandlerError: ErrNotImplemented,
	}
)

func (e *ApiError) NotNil() bool {
	return !e.Nil()
}
func (e *ApiError) Reset() {
	e.NetworkError = nil
	e.HandlerError = nil
}
func (e *ApiError) Nil() bool {
	// return e.NetworkError == nil && e.HandlerError == nil && e.isSuccess
	return e.NetworkError == nil && e.HandlerError == nil
}

// func (e *ApiError) Success() {
// e.isSuccess = true
// }

func (e *ApiError) String() string {
	return e.Error()
}

func (e *ApiError) Error() string {
	if e.NetworkError == nil && e.HandlerError == nil {
		// if !e.isSuccess {
		// return "UnknowError_NotSuccess"
		// }
		return ""
	}
	return fmt.Sprintf("NetworkError: %v, HandlerError:%v ", e.NetworkError, e.HandlerError)
}

func ApiErrorWithHandlerError(msg string) ApiError {
	return ApiError{HandlerError: errors.New(msg)}
}
func ApiErrorWithNetworkError(msg string) ApiError {
	return ApiError{NetworkError: errors.New(msg)}
}

type ApiOmitError struct {
}

func (e *ApiOmitError) Error() string {
	return "未知Api错误"
}

type SystemError struct {
	ClientError error
	InfraError  error
}

var (
	SystemErrorNil       SystemError
	SystemErrorBtAfOrder = SystemError{
		ClientError: errors.New("BeforeTrade/AfterTrade调用顺序错乱，必须BABA交替调用"),
		InfraError:  nil,
	}
	SystemErrorPairNotFound = SystemError{
		ClientError: errors.New("pair not found"),
		InfraError:  nil,
	}
)

func (e *SystemError) NotNil() bool {
	return !e.Nil()
}
func (e *SystemError) Nil() bool {
	// return e.NetworkError == nil && e.HandlerError == nil && e.isSuccess
	return e.ClientError == nil && e.InfraError == nil
}

// func (e *SystemError) TakeIn(err SystemError) {
// if e.Nil() {
// e.ClientError = err.ClientError
// e.InfraError = err.InfraError
// return
// } else {
// e.Others = append(e.Others, err)
// }
// }

// func (e *SystemError) Success() {
// e.isSuccess = true
// }

func (e *SystemError) String() string {
	return e.Error()
}

func (e *SystemError) Error() string {
	if e.ClientError == nil && e.InfraError == nil {
		// if !e.isSuccess {
		// return "UnknowError_NotSuccess"
		// }
		return ""
	}
	// others := ""
	// for _, o := range e.Others {
	// 	others += o.Error() + ";"
	// }
	return fmt.Sprintf("ClientError: %v, ServerError:%v", e.ClientError, e.InfraError)
}

func SystemErrorWithClientError(msg string) SystemError {
	return SystemError{ClientError: errors.New(msg)}
}
func SystemErrorWithInfraError(msg string) SystemError {
	return SystemError{InfraError: errors.New(msg)}
}
