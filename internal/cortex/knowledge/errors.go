package knowledge

import (
	"context"
	"errors"
	"fmt"
	"sort"
)

// ErrorCode classifies stable failure modes.
type ErrorCode string

const (
	ErrInvalidRequest     ErrorCode = "knowledge.invalid_request"
	ErrUnsupportedVersion ErrorCode = "knowledge.unsupported_version"
	ErrNotFound           ErrorCode = "knowledge.not_found"
	ErrConflict           ErrorCode = "knowledge.conflict"
	ErrUntrustedClaim     ErrorCode = "knowledge.untrusted_claim"
	ErrCanceled           ErrorCode = "knowledge.canceled"
	ErrInternal           ErrorCode = "knowledge.internal"
)

type Error struct {
	Code    ErrorCode
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message == "" {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func newError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code ErrorCode, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func CanceledError(cause error) *Error {
	return &Error{Code: ErrCanceled, Message: "operation canceled", Cause: cause}
}

func ErrorCodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var kErr *Error
	if errors.As(err, &kErr) {
		return kErr.Code
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrCanceled
	}
	return ErrInternal
}

func IsNotFound(err error) bool {
	return ErrorCodeOf(err) == ErrNotFound
}

func IsConflict(err error) bool {
	return ErrorCodeOf(err) == ErrConflict
}

func IsInvalidRequest(err error) bool {
	return ErrorCodeOf(err) == ErrInvalidRequest
}

func IsUntrustedClaim(err error) bool {
	return ErrorCodeOf(err) == ErrUntrustedClaim
}

func IsCanceled(err error) bool {
	return ErrorCodeOf(err) == ErrCanceled
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return newError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return CanceledError(err)
	}
	return nil
}

// SortItems provides deterministic ordering for list and retrieval operations.
func SortItems(items []Item) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
}
