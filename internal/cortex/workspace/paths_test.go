package workspace

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestCanonicalRootRequiresAbsoluteNonRootPath(t *testing.T) {
	t.Parallel()

	if _, err := CanonicalRoot("relative/root"); ErrorCodeOf(err) != ErrPathDenied {
		t.Fatalf("expected path denied for relative root, got %v", err)
	}
	if _, err := CanonicalRoot(""); ErrorCodeOf(err) != ErrInvalidRequest {
		t.Fatalf("expected invalid request for empty root, got %v", err)
	}
	if _, err := CanonicalRoot(string(filepath.Separator)); ErrorCodeOf(err) != ErrPathDenied {
		t.Fatalf("expected path denied for filesystem root, got %v", err)
	}
}

func TestCanonicalRelativeRejectsEscapesAndAbsolutePaths(t *testing.T) {
	t.Parallel()

	inputs := []string{"..", "../outside", "nested/../../outside"}
	if runtime.GOOS == "windows" {
		// "C:outside" is a Windows volume-rooted path, rejected via volume check.
		inputs = append(inputs, "C:outside")
	} else {
		// "/outside" is an absolute POSIX path.
		inputs = append(inputs, string(filepath.Separator)+"outside")
	}
	for _, input := range inputs {
		if _, err := CanonicalRelative(input); ErrorCodeOf(err) != ErrPathDenied {
			t.Errorf("CanonicalRelative(%q) error code = %q, want %q", input, ErrorCodeOf(err), ErrPathDenied)
		}
	}
	if _, err := CanonicalRelative("notes/readme.md"); err != nil {
		t.Fatalf("expected safe relative path, got %v", err)
	}
}

func TestContainedPathUsesComponentAwareContainment(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "workspace")
	inside, err := ContainedPath(root, "vault/note.md")
	if err != nil {
		t.Fatalf("inside path failed: %v", err)
	}
	want := filepath.Join(root, "vault", "note.md")
	if inside != want {
		t.Fatalf("inside path = %q, want %q", inside, want)
	}

	if _, err := ContainedPath(root, "../workspace-escape/note.md"); ErrorCodeOf(err) != ErrPathDenied {
		t.Fatalf("expected outside path denial, got %v", err)
	}
}

func TestContainedPathRejectsRootPrefixCollision(t *testing.T) {
	t.Parallel()

	root := filepath.Join(t.TempDir(), "project")
	candidate := filepath.Join("..", "project-sibling", "file.txt")
	if _, err := ContainedPath(root, candidate); ErrorCodeOf(err) != ErrPathDenied {
		t.Fatalf("expected root-prefix collision to be denied, got %v", err)
	}
}
