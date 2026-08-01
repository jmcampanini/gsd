package apperr

import "errors"

type Code string

const (
	NotFound        Code = "not_found"
	InvalidArgument Code = "invalid_argument"
	Conflict        Code = "conflict"
	Internal        Code = "internal"
)

type Error struct {
	Code    Code
	Message string
	cause   error
}

func (e *Error) Error() string {
	return e.Message
}

func (e *Error) Unwrap() error {
	return e.cause
}

func New(code Code, message string, cause error) error {
	return &Error{Code: code, Message: message, cause: cause}
}

func CodeOf(err error) (Code, bool) {
	var applicationError *Error
	if !errors.As(err, &applicationError) {
		return "", false
	}

	return applicationError.Code, true
}
