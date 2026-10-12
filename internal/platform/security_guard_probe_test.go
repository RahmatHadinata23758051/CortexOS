package platform_test

import (
	"os"
	"path/filepath"
	"testing"
)

// TestTransportGuardDetectsViolations proves the P7-06 guard is not vacuous:
// it must flag a synthetic production file that imports net/http or opens a
// raw listener, and it must pass a compliant file.
func TestTransportGuardDetectsViolations(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module guardprobe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "internal"), 0o700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		src      string
		violated bool
	}{
		{
			name:     "http import",
			src:      "package probe\n\nimport \"net/http\"\n\nvar _ = http.StatusOK\n",
			violated: true,
		},
		{
			name:     "raw tcp listener",
			src:      "package probe\n\nimport \"net\"\n\nfunc open() { net.Listen(\"tcp\", \":8080\") }\n",
			violated: true,
		},
		{
			name:     "listen and serve",
			src:      "package probe\n\nimport \"net/http\"\n\nfunc serve() { http.ListenAndServe(\":8080\", nil) }\n",
			violated: true,
		},
		{
			name:     "compliant wails bridge",
			src:      "package probe\n\nfunc (b *Bridge) Ping() string { return \"pong\" }\n",
			violated: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "internal", "probe.go"), []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}
			got := scanTreeForTransport(dir)
			if tc.violated && len(got) == 0 {
				t.Fatalf("guard missed violation in %q", tc.name)
			}
			if !tc.violated && len(got) > 0 {
				t.Fatalf("guard flagged compliant file: %v", got)
			}
			os.Remove(filepath.Join(dir, "internal", "probe.go"))
		})
	}
}
