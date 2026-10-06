package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

const FormatVersion = "cortexos.vault.v1"

type Document struct {
	Note workspace.VaultNote
	Body string
}

func Parse(data []byte) (Document, error) {
	text := string(data)
	if !strings.HasPrefix(text, "---\n") {
		return Document{}, workspace.NewError(workspace.ErrInvalidRequest, "Vault note front matter opening delimiter is missing")
	}
	separator := strings.Index(text[4:], "\n---\n")
	if separator < 0 {
		return Document{}, workspace.NewError(workspace.ErrInvalidRequest, "Vault note front matter closing delimiter is missing")
	}
	separator += 4
	metadata := text[4:separator]
	body := text[separator+5:]
	if !strings.HasPrefix(body, "\n") {
		return Document{}, workspace.NewError(workspace.ErrInvalidRequest, "Vault note body separator is missing")
	}
	body = body[1:]

	values, err := parseMetadata(metadata)
	if err != nil {
		return Document{}, err
	}
	note, err := noteFromMetadata(values, body)
	if err != nil {
		return Document{}, err
	}
	return Document{Note: note, Body: body}, nil
}

func Serialize(note workspace.VaultNote) ([]byte, error) {
	if err := validateNote(note); err != nil {
		return nil, err
	}
	body := note.Body
	note.ContentHash = HashBody(body)
	if note.UpdatedAt.IsZero() {
		note.UpdatedAt = time.Now().UTC()
	}
	if note.CreatedAt.IsZero() {
		note.CreatedAt = note.UpdatedAt
	}
	var builder strings.Builder
	builder.WriteString("---\n")
	writeField(&builder, "id", string(note.ID))
	writeField(&builder, "project_id", string(note.ProjectID))
	if note.WorktreeID != "" {
		writeField(&builder, "worktree_id", string(note.WorktreeID))
	}
	writeField(&builder, "relative_path", note.RelativePath)
	writeField(&builder, "title", note.Title)
	writeField(&builder, "format_version", note.FormatVersion)
	writeField(&builder, "source", note.Source)
	writeField(&builder, "author", note.Author)
	writeField(&builder, "created_at", note.CreatedAt.UTC().Format(time.RFC3339Nano))
	writeField(&builder, "updated_at", note.UpdatedAt.UTC().Format(time.RFC3339Nano))
	writeField(&builder, "content_hash", note.ContentHash)
	builder.WriteString("---\n\n")
	builder.WriteString(body)
	return []byte(builder.String()), nil
}

func HashBody(body string) string {
	hash := sha256.Sum256([]byte(body))
	return "sha256:" + hex.EncodeToString(hash[:])
}

func validateNote(note workspace.VaultNote) error {
	if note.ID == "" || note.ProjectID == "" || note.RelativePath == "" || note.Title == "" || note.Source == "" || note.Author == "" {
		return workspace.NewError(workspace.ErrInvalidRequest, "Vault note identity, scope, title, and attribution are required")
	}
	if note.FormatVersion != FormatVersion {
		return workspace.NewError(workspace.ErrUnsupportedVersion, "Vault note format version is unsupported")
	}
	if note.CreatedAt.IsZero() || note.UpdatedAt.IsZero() || note.UpdatedAt.Before(note.CreatedAt) {
		return workspace.NewError(workspace.ErrInvalidRequest, "Vault note timestamps are invalid")
	}
	if note.ContentHash != "" && note.ContentHash != HashBody(note.Body) {
		return workspace.NewError(workspace.ErrConflict, "Vault note body hash does not match content")
	}
	return nil
}

func writeField(builder *strings.Builder, key, value string) {
	builder.WriteString(key)
	builder.WriteString(": ")
	builder.WriteString(value)
	builder.WriteByte('\n')
}

func parseMetadata(metadata string) (map[string]string, error) {
	allowed := map[string]bool{
		"id": true, "project_id": true, "worktree_id": true, "relative_path": true,
		"title": true, "format_version": true, "source": true, "author": true,
		"created_at": true, "updated_at": true, "content_hash": true,
	}
	values := make(map[string]string)
	for _, line := range strings.Split(metadata, "\n") {
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ": ")
		if !ok || !allowed[key] || values[key] != "" {
			return nil, workspace.NewError(workspace.ErrInvalidRequest, "Vault note metadata is malformed or unknown")
		}
		if strings.ContainsAny(key, "\r\n\t") || strings.ContainsAny(value, "\r\n") {
			return nil, workspace.NewError(workspace.ErrInvalidRequest, "Vault note metadata contains invalid characters")
		}
		values[key] = value
	}
	return values, nil
}

func noteFromMetadata(values map[string]string, body string) (workspace.VaultNote, error) {
	required := []string{"id", "project_id", "relative_path", "title", "format_version", "source", "author", "created_at", "updated_at", "content_hash"}
	for _, key := range required {
		if values[key] == "" {
			return workspace.VaultNote{}, workspace.NewError(workspace.ErrInvalidRequest, fmt.Sprintf("Vault note field %q is required", key))
		}
	}
	createdAt, err := time.Parse(time.RFC3339Nano, values["created_at"])
	if err != nil {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrInvalidRequest, "Vault note created_at is invalid")
	}
	updatedAt, err := time.Parse(time.RFC3339Nano, values["updated_at"])
	if err != nil || updatedAt.Before(createdAt) {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrInvalidRequest, "Vault note updated_at is invalid")
	}
	note := workspace.VaultNote{
		ID:            workspace.NoteID(values["id"]),
		ProjectID:     workspace.ProjectID(values["project_id"]),
		WorktreeID:    workspace.WorktreeID(values["worktree_id"]),
		RelativePath:  values["relative_path"],
		Title:         values["title"],
		Body:          body,
		FormatVersion: values["format_version"],
		Source:        values["source"],
		Author:        values["author"],
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		ContentHash:   values["content_hash"],
		Status:        workspace.NoteStatusActive,
	}
	if err := validateNote(note); err != nil {
		return workspace.VaultNote{}, err
	}
	if note.ContentHash != HashBody(body) {
		return workspace.VaultNote{}, workspace.NewError(workspace.ErrConflict, "Vault note body hash does not match content")
	}
	return note, nil
}
