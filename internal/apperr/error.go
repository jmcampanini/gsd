// Package apperr defines the stable error codes that service and store
// boundaries return and that the CLI maps to exit statuses and JSON error
// envelopes.
package apperr

import "errors"

// Code is the stable machine-readable classification carried by an
// application error; the CLI prints it in JSON error envelopes.
type Code string

// The codes an application error can carry. NotFound and InvalidArgument
// describe the caller's input, Conflict describes stored state that blocks
// the operation, and Internal marks an unexpected failure.
const (
	NotFound        Code = "not_found"
	InvalidArgument Code = "invalid_argument"
	Conflict        Code = "conflict"
	Internal        Code = "internal"
)

// Error is a coded application error. Message states semantics only; the
// optional cause is reachable through errors.Unwrap.
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

// New builds a coded application error. A nil cause is allowed.
func New(code Code, message string, cause error) error {
	return &Error{Code: code, Message: message, cause: cause}
}

// CodeOf returns the code of the first application error in err's chain and
// reports whether one was found.
func CodeOf(err error) (Code, bool) {
	var applicationError *Error
	if !errors.As(err, &applicationError) {
		return "", false
	}

	return applicationError.Code, true
}
