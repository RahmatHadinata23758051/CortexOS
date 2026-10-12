package platform_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoUnreviewedRemoteTransport implements Phase 7 gate P7-06 (BAN-131).
// CortexOS is local-first: IPC stays inside the in-process Wails runtime. Any
// remote HTTP/TCP transport needs a security review (origin checks, CSRF,
// authentication, authorization, replay defense) before it can merge. This
// test fails when production Go code imports net/http or opens a raw listener.
func TestNoUnreviewedRemoteTransport(t *testing.T) {
	t.Parallel()

	violations := scanTreeForTransport(findRepoRoot(t))
	if len(violations) > 0 {
		t.Fatalf("P7-06 regression: unreviewed remote transport:\n%s\nSee docs/specs/ban131-remote-transport-guard.md", strings.Join(violations, "\n"))
	}
}

func scanTreeForTransport(root string) []string {
	fset := token.NewFileSet()
	var violations []string
	for _, productionDir := range []string{"internal", "cmd"} {
		_ = filepath.WalkDir(filepath.Join(root, productionDir), func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return walkErr
			}
			node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				violations = append(violations, path+": parse error")
				return nil
			}
			for _, imp := range node.Imports {
				if strings.Trim(imp.Path.Value, `"`) == "net/http" {
					violations = append(violations, path+": imports net/http")
				}
			}
			data, err := os.ReadFile(path)
			if err != nil {
				violations = append(violations, path+": read error")
				return nil
			}
			for _, pattern := range []string{"net.Listen(", "net.ListenTCP(", "http.ListenAndServe(", "http.ListenAndServeTLS("} {
				if strings.Contains(string(data), pattern) {
					violations = append(violations, path+": contains "+pattern)
				}
			}
			return nil
		})
	}
	return violations
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod in parents)")
		}
		dir = parent
	}
}
