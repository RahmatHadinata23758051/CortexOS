package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
)

// Pipeline coordinates discovery, chunking, filtering, and knowledge item creation.
type Pipeline struct {
	discovery *Discovery
	vault     workspace.VaultStore
	knowledge *Service
	filter    *SecretFilter
	config    IngestionConfig
}

// NewPipeline creates a new ingestion pipeline.
func NewPipeline(vault workspace.VaultStore, knowledgeSvc *Service, config IngestionConfig) (*Pipeline, error) {
	if vault == nil {
		return nil, fmt.Errorf("vault store is required")
	}
	if knowledgeSvc == nil {
		return nil, fmt.Errorf("knowledge service is required")
	}
	return &Pipeline{
		discovery: NewDiscovery(vault),
		vault:     vault,
		knowledge: knowledgeSvc,
		filter:    NewSecretFilter(config),
		config:    config,
	}, nil
}

// IngestProject runs full ingestion for a project.
func (p *Pipeline) IngestProject(ctx context.Context, projectID workspace.ProjectID, workspaceID WorkspaceID) (IngestionResult, error) {
	start := time.Now()
	res := IngestionResult{
		ProjectID: projectID,
	}

	if err := checkContext(ctx); err != nil {
		return res, err
	}

	refs, err := p.discovery.Discover(ctx, projectID)
	if err != nil {
		return res, err
	}
	res.Discovered = len(refs)

	// Fetch existing active items for this project
	existingItems, err := p.knowledge.List(ctx, Filter{
		ProjectID:  projectIDPtr(string(projectID)),
		ActiveOnly: true,
	})
	if err != nil {
		return res, err
	}
	existingHashes := make(map[string]string)
	for _, item := range existingItems {
		if item.Provenance.SourceHash != "" {
			existingHashes[item.Provenance.SourceURI] = item.Provenance.SourceHash
		}
	}

	for _, ref := range refs {
		if err := checkContext(ctx); err != nil {
			return res, err
		}

		if existingHash, ok := existingHashes[ref.RelativePath]; ok && existingHash == ref.ContentHash {
			res.Skipped++
			continue
		}

		note, err := p.vault.GetNote(ctx, ref.NoteID)
		if err != nil {
			res.Errors = append(res.Errors, IngestionError{
				SourceRef: ref,
				Operation: "get_note",
				Err:       err,
			})
			continue
		}

		// Ingest note into knowledge items (one per chunk)
		ingested, err := p.ingestNote(ctx, note, workspaceID)
		if err != nil {
			res.Errors = append(res.Errors, IngestionError{
				SourceRef: ref,
				Operation: "ingest_note",
				Err:       err,
			})
			continue
		}

		if ingested {
			res.Ingested++
		} else {
			res.Skipped++
		}
	}

	res.Duration = time.Since(start)
	return res, nil
}

// IngestNote ingests a single note by ID.
func (p *Pipeline) IngestNote(ctx context.Context, noteID workspace.NoteID, workspaceID WorkspaceID) (bool, error) {
	if err := checkContext(ctx); err != nil {
		return false, err
	}
	note, err := p.vault.GetNote(ctx, noteID)
	if err != nil {
		return false, err
	}
	if !IsAllowedPath(note.RelativePath) {
		return false, fmt.Errorf("note path %q is not allowed for ingestion", note.RelativePath)
	}
	return p.ingestNote(ctx, note, workspaceID)
}

func (p *Pipeline) ingestNote(ctx context.Context, note workspace.VaultNote, workspaceID WorkspaceID) (bool, error) {
	// Chunk the Markdown body
	chunks := ChunkMarkdown(note.Body, p.config)
	if len(chunks) == 0 {
		return false, nil
	}

	// Filter secrets in each chunk
	now := time.Now().UTC()
	for _, chunk := range chunks {
		if err := checkContext(ctx); err != nil {
			return false, err
		}

		filteredContent := p.filter.Redact(chunk.Content)

		// Create capture request for knowledge service
		req := CaptureRequest{
			WorkspaceID: workspaceID,
			ProjectID:   ProjectID(note.ProjectID),
			Title:       fmt.Sprintf("%s (chunk %d)", note.Title, chunk.Index),
			Content:     filteredContent,
			Kind:        KindReference,
			Tags:        chunk.HeadingPath,
			Provenance: Provenance{
				SourceKind:  "vault_note",
				SourceURI:   note.RelativePath,
				SourceHash:  note.ContentHash,
				Author:      note.Author,
				Attribution: note.Source + "/" + note.Author,
				TaskID:      string(note.ID),
				CapturedAt:  now,
			},
		}

		item, err := p.knowledge.Capture(ctx, req)
		if err != nil {
			return false, err
		}
		// Vault content is authoritative workspace content. Promote the captured
		// draft through the knowledge lifecycle so only verified items are indexed.
		if _, err := p.knowledge.Validate(ctx, item.ID, "workspace-vault", "authoritative Markdown Vault source"); err != nil {
			return false, err
		}
	}

	return true, nil
}
