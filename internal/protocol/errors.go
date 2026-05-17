package protocol

import (
	"errors"
)

type ErrorCode string

const (
	ErrorRuntimeNotAvailable  ErrorCode = "runtime_not_available"
	ErrorRuntimeImageMissing  ErrorCode = "runtime_image_missing"
	ErrorRuntimeTimeout       ErrorCode = "runtime_timeout"
	ErrorRuntimeBadOutput     ErrorCode = "runtime_bad_output"
	ErrorRuntimeDeniedPolicy  ErrorCode = "runtime_denied_by_policy"
	ErrorRuntimeResourceLimit ErrorCode = "runtime_resource_limit"
	ErrorManifestInvalid      ErrorCode = "manifest_invalid"
	ErrorDoorCrashed          ErrorCode = "door_crashed"
	ErrorProtocol             ErrorCode = "protocol_error"
	ErrorAuth                 ErrorCode = "auth_error"
	ErrorStorage              ErrorCode = "storage_error"
	ErrorClientIncompatible   ErrorCode = "client_incompatible"
)

type CodedError struct {
	Code    ErrorCode
	Message string
	Err     error
}

func NewCodedError(code ErrorCode, message string, err error) error {
	return &CodedError{Code: code, Message: message, Err: err}
}

func (e *CodedError) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Message != "":
		return e.Message
	case e.Err != nil:
		return e.Err.Error()
	default:
		return string(e.Code)
	}
}

func (e *CodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ErrorCodeOf(err error) ErrorCode {
	var coded *CodedError
	if errors.As(err, &coded) && coded.Code != "" {
		return coded.Code
	}
	return ErrorProtocol
}

func ErrorMessageFor(err error) ErrorMessage {
	if err == nil {
		return ErrorMessage{Type: TypeError, Code: string(ErrorProtocol), Message: "unknown error"}
	}
	return ErrorMessage{
		Type:    TypeError,
		Code:    string(ErrorCodeOf(err)),
		Message: err.Error(),
	}
}
