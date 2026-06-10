// Package errors defines typed errors for motofb.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for errors.Is checks.
var (
	ErrAuthentication   = errors.New("motofb: authentication failed")
	ErrSessionExpired   = errors.New("motofb: session expired")
	ErrNotLoggedIn      = errors.New("motofb: not logged in")
	ErrFacebookAPI      = errors.New("motofb: facebook api error")
	ErrNetwork          = errors.New("motofb: network error")
	ErrParsing          = errors.New("motofb: parsing error")
	ErrValidation       = errors.New("motofb: validation error")
	ErrConfiguration    = errors.New("motofb: configuration error")
)

// Error is the base error type with optional code and details.
type Error struct {
	Op      string
	Message string
	Code    int
	Err     error
	Details map[string]any
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	if e.Code != 0 {
		return fmt.Sprintf("%s: %s (code %d)", e.Op, e.Message, e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Op, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func New(op, message string) *Error {
	return &Error{Op: op, Message: message}
}

func Wrap(op, message string, err error) *Error {
	return &Error{Op: op, Message: message, Err: err}
}

func WithCode(op, message string, code int) *Error {
	return &Error{Op: op, Message: message, Code: code}
}