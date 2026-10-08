package staff

import (
	"context"
	"errors"
	"fmt"
)

// ErrorCode is a stable, machine-readable error classification for the Staff
// contract layer. Codes are chosen to map cleanly to Harness/Orchestra error
// codes and bridge-safe transport.
type ErrorCode string

const (
	ErrInvalidRequest     ErrorCode = "staff.invalid_request"
	ErrUnsupportedVersion ErrorCode = "staff.unsupported_version"
	ErrNotFound           ErrorCode = "staff.not_found"
	ErrConflict           ErrorCode = "staff.conflict"
	ErrPermissionDenied   ErrorCode = "staff.permission_denied"
	ErrInactiveStaff      ErrorCode = "staff.inactive"
	ErrUnavailableStaff   ErrorCode = "staff.unavailable"
	ErrInvalidAssignment  ErrorCode = "staff.invalid_assignment"
	ErrInvalidPermission  ErrorCode = "staff.invalid_permission"
	ErrCapabilityMismatch ErrorCode = "staff.capability_mismatch"
	ErrWorkspaceMismatch  ErrorCode = "staff.workspace_mismatch"
	ErrStaffBusy          ErrorCode = "staff.busy"
	ErrResourceMismatch   ErrorCode = "staff.resource_mismatch"
	ErrCanceled           ErrorCode = "staff.canceled"
	ErrInternal           ErrorCode = "staff.internal"
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

func ErrorCodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var staffErr *Error
	if errors.As(err, &staffErr) {
		return staffErr.Code
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return ErrCanceled
	}
	return ErrInternal
}

func CanceledError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return WrapError(ErrCanceled, "operation canceled", err)
	}
	return err
}

// IsNotFound returns true if the error represents a not-found condition.
func IsNotFound(err error) bool {
	return ErrorCodeOf(err) == ErrNotFound
}

// IsPermissionDenied returns true if the error represents a permission denial.
func IsPermissionDenied(err error) bool {
	return ErrorCodeOf(err) == ErrPermissionDenied
}

// IsInvalidRequest returns true if the error represents an invalid request.
func IsInvalidRequest(err error) bool {
	return ErrorCodeOf(err) == ErrInvalidRequest
}

// IsUnsupportedVersion returns true if the error represents an unsupported version.
func IsUnsupportedVersion(err error) bool {
	return ErrorCodeOf(err) == ErrUnsupportedVersion
}
