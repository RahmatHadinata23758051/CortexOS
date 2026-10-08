package knowledge

import (
	"context"
	"fmt"
	"sort"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// Discovery discovers Vault notes for ingestion.
type Discovery struct {
	vault workspace.VaultStore
}

// NewDiscovery creates a new discovery instance.
func NewDiscovery(vault workspace.VaultStore) *Discovery {
	return &Discovery{vault: vault}
}

// Discover lists all ingestible notes for a project.
func (d *Discovery) Discover(ctx context.Context, projectID workspace.ProjectID) ([]SourceRef, error) {
	if d.vault == nil {
		return nil, fmt.Errorf("vault store is required")
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if projectID == "" {
		return nil, fmt.Errorf("project id is required")
	}

	notes, err := d.vault.ListNotes(ctx, projectID)
	if err != nil {
		return nil, err
	}

	refs := make([]SourceRef, 0, len(notes))
	for _, note := range notes {
		if err := checkContext(ctx); err != nil {
			return nil, err
		}
		// Only ingest active notes
		if note.Status != workspace.NoteStatusActive {
			continue
		}
		// Only allowed paths
		if !IsAllowedPath(note.RelativePath) {
			continue
		}
		refs = append(refs, SourceRef{
			ProjectID:    note.ProjectID,
			NoteID:       note.ID,
			RelativePath: note.RelativePath,
			ContentHash:  note.ContentHash,
			UpdatedAt:    note.UpdatedAt,
		})
	}

	sort.Slice(refs, func(i, j int) bool {
		if refs[i].RelativePath != refs[j].RelativePath {
			return refs[i].RelativePath < refs[j].RelativePath
		}
		return refs[i].NoteID < refs[j].NoteID
	})

	return refs, nil
}

// DiscoverNote fetches a single note by ID.
func (d *Discovery) DiscoverNote(ctx context.Context, noteID workspace.NoteID) (*SourceRef, error) {
	if d.vault == nil {
		return nil, fmt.Errorf("vault store is required")
	}
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if noteID == "" {
		return nil, fmt.Errorf("note id is required")
	}

	note, err := d.vault.GetNote(ctx, noteID)
	if err != nil {
		return nil, err
	}

	if note.Status != workspace.NoteStatusActive {
		return nil, fmt.Errorf("note is not active")
	}
	if !IsAllowedPath(note.RelativePath) {
		return nil, fmt.Errorf("note path not allowed for ingestion")
	}

	return &SourceRef{
		ProjectID:    note.ProjectID,
		NoteID:       note.ID,
		RelativePath: note.RelativePath,
		ContentHash:  note.ContentHash,
		UpdatedAt:    note.UpdatedAt,
	}, nil
}
