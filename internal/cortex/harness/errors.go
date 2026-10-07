package harness

import (
	"context"
	"errors"
	"fmt"
)

// Stable error classes exposed by Harness. Error messages never contain paths,
// command lines, environment variables, credentials, or raw provider output.
var (
	ErrPolicyDenied      = errors.New("harness: policy denied")
	ErrPolicyAsk         = errors.New("harness: policy approval required")
	ErrTimeout           = errors.New("harness: execution timed out")
	ErrCanceled          = errors.New("harness: execution canceled")
	ErrWorktreeViolation = errors.New("harness: worktree violation")
	ErrSandbox           = errors.New("harness: sandbox error")
	ErrAdapterFailure    = errors.New("harness: adapter failure")
	ErrToolNotFound      = errors.New("harness: tool not found")
	ErrDuplicateTool     = errors.New("harness: duplicate tool")
	ErrInvalidContract   = errors.New("harness: invalid contract")
)

// ErrorCode is the stable serialized code for a Harness error.
type ErrorCode string

const (
	ErrorCodePolicyDenied      ErrorCode = "harness.policy_denied"
	ErrorCodePolicyAsk         ErrorCode = "harness.policy_approval_required"
	ErrorCodeTimeout           ErrorCode = "harness.timeout"
	ErrorCodeCanceled          ErrorCode = "harness.canceled"
	ErrorCodeWorktreeViolation ErrorCode = "harness.worktree_violation"
	ErrorCodeSandbox           ErrorCode = "harness.sandbox_error"
	ErrorCodeAdapterFailure    ErrorCode = "harness.adapter_failure"
	ErrorCodeToolNotFound      ErrorCode = "harness.tool_not_found"
	ErrorCodeDuplicateTool     ErrorCode = "harness.duplicate_tool"
	ErrorCodeInvalidContract   ErrorCode = "harness.invalid_contract"
)

// CodedError is the typed, redacted error returned by the broker.
type CodedError struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Retryable bool      `json:"retryable"`
	Cause     error     `json:"-"`
}

func (e *CodedError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return string(e.Code) + ": " + e.Message
}

func (e *CodedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *CodedError) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	return errors.Is(e.Cause, target)
}

// ErrorFor maps internal errors to a stable, redacted CodedError.
func ErrorFor(err error) *CodedError {
	if err == nil {
		return nil
	}
	if coded, ok := err.(*CodedError); ok {
		return coded
	}
	code, message, retryable := ErrorCodeAdapterFailure, "tool adapter failed", false
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, ErrCanceled):
		code, message, retryable = ErrorCodeCanceled, "tool execution was canceled", true
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, ErrTimeout):
		code, message, retryable = ErrorCodeTimeout, "tool execution timed out", true
	case errors.Is(err, ErrPolicyDenied):
		code, message = ErrorCodePolicyDenied, "tool execution was denied by policy"
	case errors.Is(err, ErrPolicyAsk):
		code, message = ErrorCodePolicyAsk, "tool execution requires policy approval"
	case errors.Is(err, ErrWorktreeViolation):
		code, message = ErrorCodeWorktreeViolation, "tool attempted an invalid worktree operation"
	case errors.Is(err, ErrSandbox):
		code, message, retryable = ErrorCodeSandbox, "tool sandbox failed", true
	case errors.Is(err, ErrToolNotFound):
		code, message = ErrorCodeToolNotFound, "requested tool is unavailable"
	case errors.Is(err, ErrDuplicateTool):
		code, message = ErrorCodeDuplicateTool, "tool is already registered"
	case errors.Is(err, ErrInvalidContract):
		code, message = ErrorCodeInvalidContract, "harness contract is invalid"
	}
	return &CodedError{Code: code, Message: message, Retryable: retryable, Cause: err}
}

func wrapError(base, err error, format string, args ...any) error {
	if err == nil {
		return base
	}
	return fmt.Errorf("%w: %s", base, fmt.Sprintf(format, args...))
}
