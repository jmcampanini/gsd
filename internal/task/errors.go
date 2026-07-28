package task

import "errors"

type ErrorCode string

const (
	ErrorNotFound        ErrorCode = "not_found"
	ErrorInvalidArgument ErrorCode = "invalid_argument"
	ErrorConflict        ErrorCode = "conflict"
	ErrorInternal        ErrorCode = "internal"
)

type Error struct {
	Code    ErrorCode
	Message string
	cause   error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

func NewError(code ErrorCode, message string, cause error) error {
	return &Error{Code: code, Message: message, cause: cause}
}

func ErrorCodeOf(err error) (ErrorCode, bool) {
	var applicationError *Error
	if !errors.As(err, &applicationError) {
		return "", false
	}

	return applicationError.Code, true
}
