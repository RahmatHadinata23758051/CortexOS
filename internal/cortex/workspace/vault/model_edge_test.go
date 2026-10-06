package vault

import (
	"testing"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

func TestSerializeQuotesScalarsAndRoundTripsSpecialCharacters(t *testing.T) {
	t.Parallel()

	note := workspace.VaultNote{
		ID: "note-quoted", ProjectID: "project-1", RelativePath: "notes/quoted.md",
		Title: "Title: with # marker", Body: "body\n", FormatVersion: FormatVersion,
		Source: "manual", Author: "author@example.test", CreatedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
	}
	data, err := Serialize(note)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Note.Title != note.Title {
		t.Fatalf("title = %q, want %q", parsed.Note.Title, note.Title)
	}
}

func TestParseRejectsMalformedQuotedScalar(t *testing.T) {
	t.Parallel()

	data := "---\nid: note\nproject_id: project\nrelative_path: note.md\ntitle: \"unterminated\nformat_version: cortexos.vault.v1\nsource: manual\nauthor: tester\ncreated_at: 2026-10-06T12:00:00Z\nupdated_at: 2026-10-06T12:00:00Z\ncontent_hash: sha256:bad\n---\n\nbody\n"
	if _, err := Parse([]byte(data)); workspace.ErrorCodeOf(err) != workspace.ErrInvalidRequest {
		t.Fatalf("error code = %q, want %q", workspace.ErrorCodeOf(err), workspace.ErrInvalidRequest)
	}
}
