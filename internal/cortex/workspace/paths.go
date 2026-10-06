package workspace

import (
	"errors"
	"path/filepath"
	"strings"
)

var ErrEmptyPath = NewError(ErrInvalidRequest, "path must not be empty")

// CanonicalRoot returns an absolute, cleaned root. The root does not need to exist;
// callers that access the filesystem must validate existence separately.
func CanonicalRoot(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrEmptyPath
	}
	if !filepath.IsAbs(path) {
		return "", NewError(ErrPathDenied, "root must be absolute")
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == string(filepath.Separator) {
		return "", NewError(ErrPathDenied, "root is not an approved workspace root")
	}
	return filepath.Abs(cleaned)
}

// CanonicalRelative returns a cleaned relative path and rejects paths that escape
// their root. It never resolves filesystem links; callers must perform a final
// link/reparse containment check before mutation.
func CanonicalRelative(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", ErrEmptyPath
	}
	if filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", NewError(ErrPathDenied, "relative path must not include a volume or be absolute")
	}
	cleaned := filepath.Clean(filepath.FromSlash(path))
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", NewError(ErrPathDenied, "relative path escapes its root")
	}
	return cleaned, nil
}

// ContainedPath resolves a relative path below root and verifies component-aware
// containment. It intentionally does not follow symlinks or reparse points.
func ContainedPath(root, relative string) (string, error) {
	canonicalRoot, err := CanonicalRoot(root)
	if err != nil {
		return "", err
	}
	canonicalRelative, err := CanonicalRelative(relative)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(canonicalRoot, canonicalRelative)
	cleanCandidate, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", WrapError(ErrPathDenied, "cannot canonicalize candidate path", err)
	}
	if !isContained(canonicalRoot, cleanCandidate) {
		return "", NewError(ErrPathDenied, "path is outside the approved root")
	}
	return cleanCandidate, nil
}

func isContained(root, candidate string) bool {
	root = filepath.Clean(root)
	candidate = filepath.Clean(candidate)
	if equalPath(root, candidate) {
		return true
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func equalPath(left, right string) bool {
	if filepath.VolumeName(left) != filepath.VolumeName(right) {
		return false
	}
	if filepath.Separator == '\\' {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}

func IsPathError(err error) bool {
	return ErrorCodeOf(err) == ErrPathDenied || errors.Is(err, ErrEmptyPath)
}
