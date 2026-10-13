package version_test

import (
	"os"
	"strings"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/version"
)

func TestVersion_ConsistentWithVERSIONFile(t *testing.T) {
	data, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatalf("failed to read VERSION file: %v", err)
	}
	expected := strings.TrimSpace(string(data))
	expectedBare := strings.TrimPrefix(expected, "v")

	if version.Version != expectedBare {
		t.Errorf("version.Version = %q, want %q", version.Version, expectedBare)
	}
	if version.VersionFull != expected {
		t.Errorf("version.VersionFull = %q, want %q", version.VersionFull, expected)
	}
}
