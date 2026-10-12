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
// CortexOS is a local-first desktop application with IPC confined to the
// in-process Wails runtime. If a remote HTTP/TCP transport is ever added,
// it must undergo formal security threat modeling (origin validation,
// CSRF protection, authentication, authorization, and replay defense)
// before merging. This test fails if any production Go source imports
// net/http or calls network listener primitives without explicit review.
func TestNoUnreviewedRemoteTransport(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	productionDirs := []string{
		filepath.Join(repoRoot, "cmd"),
		filepath.Join(repoRoot, "internal"),
	}

	fset := token.NewFileSet()
	var violations []string

	for _, dir := range productionDirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				// Skip hidden or testdata directories
				if strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatalf("failed to parse %s: %v", path, err)
			}

			relPath, _ := filepath.Rel(repoRoot, path)
			for _, imp := range node.Imports {
				pkg := strings.Trim(imp.Path.Value, `"`)
				if pkg == "net/http" {
					violations = append(violations, relPath+": imports net/http (requires P7-06 security review)")
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed on %s: %v", dir, err)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("P7-06 regression: unreviewed remote transport detected:\n%s\nSee docs/specs/ban131-remote-transport-guard.md", strings.Join(violations, "\n"))
	}
}

// TestNoUnreviewedNetworkListeners scans production code for raw listener calls.
func TestNoUnreviewedNetworkListeners(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	productionDirs := []string{
		filepath.Join(repoRoot, "cmd"),
		filepath.Join(repoRoot, "internal"),
	}

	var violations []string

	for _, dir := range productionDirs {
		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			src := string(data)
			relPath, _ := filepath.Rel(repoRoot, path)

			forbidden := []string{
				"net.Listen(",
				"net.ListenTCP(",
				"http.ListenAndServe(",
				"http.ListenAndServeTLS(",
			}
			for _, pat := range forbidden {
				if strings.Contains(src, pat) {
					violations = append(violations, relPath+": contains "+pat)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk failed on %s: %v", dir, err)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("P7-06 regression: raw network listener found:\n%s", strings.Join(violations, "\n"))
	}
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
