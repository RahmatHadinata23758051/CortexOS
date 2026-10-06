package retrieval

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func document(project, note, path, content string) workspace.RetrievalDocument {
	return workspace.RetrievalDocument{
		ID:           DocumentID(workspace.ProjectID(project), workspace.NoteID(note), path),
		NoteID:       workspace.NoteID(note),
		ProjectID:    workspace.ProjectID(project),
		RelativePath: path,
		SourceHash:   Hash(content),
		IndexVersion: IndexVersion,
		Content:      content,
		Attribution:  "manual/tester",
	}
}

func Hash(content string) string {
	return "sha256:" + content
}

func TestIndexGoldenOrderingAndProjectIsolation(t *testing.T) {
	t.Parallel()

	index, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, item := range []workspace.RetrievalDocument{
		document("project-1", "note-b", "b.md", "alpha beta"),
		document("project-1", "note-a", "a.md", "alpha"),
		document("project-2", "note-c", "c.md", "alpha alpha"),
	} {
		if err := index.Upsert(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	results, err := index.Query(ctx, "project-1", "ALPHA BETA", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 || results[0].NoteID != "note-b" || results[1].NoteID != "note-a" {
		t.Fatalf("results = %#v", results)
	}
	other, err := index.Query(ctx, "project-2", "alpha", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || other[0].ProjectID != "project-2" {
		t.Fatalf("other project results = %#v", other)
	}
}

func TestIndexUpsertRemoveAndBoundedLimit(t *testing.T) {
	t.Parallel()

	index, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		item := document("project", string(rune('a'+i)), string(rune('a'+i))+".md", "same token")
		if err := index.Upsert(ctx, item); err != nil {
			t.Fatal(err)
		}
	}
	results, err := index.Query(ctx, "project", "token", 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("results = %d", len(results))
	}
	if err := index.Remove(ctx, results[0].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Query(ctx, "project", "token", 0); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("limit error code = %q", workspace.ErrorCodeOf(err))
	}
}

func TestIndexCorruptStateFailsClosedAndRebuildRepairs(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	index, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index.Path(), []byte(`{"version":"invalid"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := index.Query(context.Background(), "project", "token", 10); workspace.ErrorCodeOf(err) != workspace.ErrRetrievalCorrupt {
		t.Fatalf("corrupt error code = %q", workspace.ErrorCodeOf(err))
	}
	if err := index.RebuildFrom(context.Background(), "project", []workspace.RetrievalDocument{document("project", "note", "note.md", "token")}); err != nil {
		t.Fatal(err)
	}
	results, err := index.Query(context.Background(), "project", "token", 10)
	if err != nil || len(results) != 1 {
		t.Fatalf("rebuilt results = %#v, err=%v", results, err)
	}
}

func TestIndexCancellationAndCanonicalDocumentValidation(t *testing.T) {
	t.Parallel()

	index, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := index.Upsert(ctx, document("project", "note", "note.md", "body")); workspace.ErrorCodeOf(err) != workspace.ErrCanceled {
		t.Fatalf("canceled error code = %q", workspace.ErrorCodeOf(err))
	}
	invalid := document("project", "note", "note.md", "body")
	invalid.ID = "arbitrary"
	if err := index.Upsert(context.Background(), invalid); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("invalid id error code = %q", workspace.ErrorCodeOf(err))
	}
}

func TestIndexJSONIsStable(t *testing.T) {
	t.Parallel()

	index, err := New(filepath.Join(t.TempDir(), "index"))
	if err != nil {
		t.Fatal(err)
	}
	if err := index.Upsert(context.Background(), document("project", "note", "note.md", "body")); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(index.Path())
	if err != nil {
		t.Fatal(err)
	}
	var decoded persisted
	if err := json.Unmarshal(first, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version != IndexVersion || len(decoded.Documents) != 1 {
		t.Fatalf("decoded = %#v", decoded)
	}
	if err := index.Upsert(context.Background(), decoded.Documents[0]); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(index.Path())
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("index serialization changed")
	}
}
