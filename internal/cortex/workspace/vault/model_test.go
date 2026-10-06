package vault

import (
	"strings"
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestParseAndSerializePreservesAttributedMarkdown(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	note := workspace.VaultNote{
		ID:            "note-1",
		ProjectID:     "project-1",
		RelativePath:  "decisions/storage.md",
		Title:         "Storage Decision",
		Body:          "# Storage\n\nSQLite is authoritative.\n",
		FormatVersion: FormatVersion,
		Source:        "manual",
		Author:        "tester",
		CreatedAt:     created,
		UpdatedAt:     created,
	}
	data, err := Serialize(note)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Note.ID != note.ID || parsed.Note.ProjectID != note.ProjectID || parsed.Body != note.Body || parsed.Note.ContentHash != HashBody(note.Body) {
		t.Fatalf("parsed = %#v body=%q", parsed.Note, parsed.Body)
	}
}

func TestParseRejectsUnknownAndMalformedMetadata(t *testing.T) {
	t.Parallel()

	base := "---\nid: note-1\nproject_id: project-1\nrelative_path: note.md\ntitle: Note\nformat_version: cortexos.vault.v1\nsource: manual\nauthor: tester\ncreated_at: 2026-10-06T12:00:00Z\nupdated_at: 2026-10-06T12:00:00Z\ncontent_hash: sha256:bad\n---\n\nbody\n"
	for _, fixture := range []string{
		strings.Replace(base, "content_hash: sha256:bad\n", "content_hash: sha256:bad\nunknown: value\n", 1),
		"---\nid: note-1\n---\nbody",
		"---\nid: note-1\nid: duplicate\n---\n\nbody",
	} {
		if _, err := Parse([]byte(fixture)); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest && workspace.ErrorCodeOf(err) != workspace.ErrConflict {
			t.Errorf("fixture error code = %q, err=%v", workspace.ErrorCodeOf(err), err)
		}
	}
}

func TestParseRejectsInvalidUTF8(t *testing.T) {
	t.Parallel()

	if _, err := Parse([]byte{0xff, 0xfe}); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("UTF-8 error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrInvalidRequest)
	}
}

func TestParseRejectsUnsupportedVersionAndHashMismatch(t *testing.T) {
	t.Parallel()

	data := "---\nid: note-1\nproject_id: project-1\nrelative_path: note.md\ntitle: Note\nformat_version: cortexos.vault.v0\nsource: manual\nauthor: tester\ncreated_at: 2026-10-06T12:00:00Z\nupdated_at: 2026-10-06T12:00:00Z\ncontent_hash: sha256:bad\n---\n\nbody\n"
	if _, err := Parse([]byte(data)); workspace.ErrorCodeOf(err) != workspace.ErrUnsupportedVersion {
		t.Fatalf("version error code = %q", workspace.ErrorCodeOf(err))
	}
}
