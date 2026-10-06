package workspace

import (
	"context"
	"errors"
	"fmt"
)

type ErrorCode string

const (
	ErrInvalidRequest     ErrorCode = "workspace.invalid_request"
	ErrUnsupportedVersion ErrorCode = "workspace.unsupported_version"
	ErrPathDenied         ErrorCode = "workspace.path_denied"
	ErrNotFound           ErrorCode = "workspace.not_found"
	ErrConflict           ErrorCode = "workspace.conflict"
	ErrNotGitRepository   ErrorCode = "workspace.not_git_repository"
	ErrMigrationFailed    ErrorCode = "workspace.migration_failed"
	ErrStorageUnavailable ErrorCode = "workspace.storage_unavailable"
	ErrWatcherOverflow    ErrorCode = "workspace.watcher_overflow"
	ErrRetrievalStale     ErrorCode = "workspace.retrieval_stale"
	ErrRetrievalCorrupt   ErrorCode = "workspace.retrieval_corrupt"
	ErrCanceled           ErrorCode = "workspace.canceled"
	ErrInternal           ErrorCode = "workspace.internal"
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

func NewError(code ErrorCode, message string) *Error {
	return &Error{Code: code, Message: message}
}

func WrapError(code ErrorCode, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}

func ErrorCodeOf(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var workspaceErr *Error
	if errors.As(err, &workspaceErr) {
		return workspaceErr.Code
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
