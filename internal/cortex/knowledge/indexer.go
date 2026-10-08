package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace"
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/workspace/retrieval"
)

// Indexer connects knowledge items to the retrieval index.
type Indexer struct {
	index     workspace.RetrievalIndex
	workspace WorkspaceID
}

// NewIndexer creates a new indexer.
func NewIndexer(index workspace.RetrievalIndex, workspaceID WorkspaceID) *Indexer {
	return &Indexer{index: index, workspace: workspaceID}
}

// IndexItem converts a knowledge item to a retrieval document and upserts it.
func (i *Indexer) IndexItem(ctx context.Context, item Item) error {
	if i.index == nil {
		return fmt.Errorf("retrieval index is required")
	}
	if err := checkContext(ctx); err != nil {
		return err
	}

	doc := retrievalDocumentFromItem(item, i.workspace)
	if err := validateRetrievalDocument(doc); err != nil {
		return err
	}

	return i.index.Upsert(ctx, doc)
}

// RemoveItem removes a knowledge item from the retrieval index.
func (i *Indexer) RemoveItem(ctx context.Context, item Item) error {
	if i.index == nil {
		return fmt.Errorf("retrieval index is required")
	}
	if err := checkContext(ctx); err != nil {
		return err
	}

	docID := retrievalDocumentID(item.ProjectID, item.Provenance.SourceURI, item.ID)
	return i.index.Remove(ctx, docID)
}

// IndexItems bulk indexes multiple knowledge items.
func (i *Indexer) IndexItems(ctx context.Context, items []Item) error {
	for _, item := range items {
		if err := i.IndexItem(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

// RebuildProject rebuilds the retrieval index for a project from knowledge items.
func (i *Indexer) RebuildProject(ctx context.Context, projectID ProjectID, items []Item) error {
	if i.index == nil {
		return fmt.Errorf("retrieval index is required")
	}
	if err := checkContext(ctx); err != nil {
		return err
	}

	docs := make([]workspace.RetrievalDocument, 0, len(items))
	for _, item := range items {
		if item.ProjectID != projectID {
			continue
		}
		docs = append(docs, retrievalDocumentFromItem(item, i.workspace))
	}

	// Use RebuildFrom on the concrete Index type if available, otherwise upsert individually
	type rebuilder interface {
		RebuildFrom(context.Context, workspace.ProjectID, []workspace.RetrievalDocument) error
	}
	if r, ok := i.index.(rebuilder); ok {
		return r.RebuildFrom(ctx, workspace.ProjectID(projectID), docs)
	}
	// Fallback: upsert each document
	for _, doc := range docs {
		if err := i.index.Upsert(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func retrievalDocumentFromItem(item Item, workspaceID WorkspaceID) workspace.RetrievalDocument {
	// Create a deterministic document ID that includes the knowledge item ID (which is chunk-aware)
	docID := retrievalDocumentID(item.ProjectID, item.Provenance.SourceURI, item.ID)

	// Extract attribution from provenance
	attribution := item.Provenance.Attribution
	if attribution == "" {
		attribution = item.Provenance.Author + "/" + item.Provenance.SourceKind
	}

	hash := item.Provenance.SourceHash
	if hash == "" {
		hash = item.ContentHash
	}

	return workspace.RetrievalDocument{
		ID:           docID,
		NoteID:       workspace.NoteID(item.ID),
		ProjectID:    workspace.ProjectID(item.ProjectID),
		WorktreeID:   workspace.WorktreeID(workspaceID),
		RelativePath: item.Provenance.SourceURI,
		SourceHash:   hash,
		IndexVersion: retrieval.IndexVersion,
		Content:      item.Content,
		Attribution:  attribution,
		IndexedAt:    item.UpdatedAt.UTC(),
	}
}

func retrievalDocumentID(projectID ProjectID, sourceURI string, knowledgeID KnowledgeID) string {
	return retrieval.DocumentID(workspace.ProjectID(projectID), workspace.NoteID(knowledgeID), sourceURI)
}

func validateRetrievalDocument(doc workspace.RetrievalDocument) error {
	if doc.ID == "" || doc.NoteID == "" || doc.ProjectID == "" || doc.RelativePath == "" || doc.SourceHash == "" || doc.IndexVersion != retrieval.IndexVersion {
		return fmt.Errorf("retrieval document identity, source, path, and version are required")
	}
	if doc.ID != retrievalDocumentID(ProjectID(doc.ProjectID), doc.RelativePath, KnowledgeID(doc.NoteID)) {
		return fmt.Errorf("retrieval document id is not canonical")
	}
	return nil
}

// IncrementalIngest performs incremental ingestion: upsert changed, remove deleted.
func (p *Pipeline) IncrementalIngest(ctx context.Context, projectID workspace.ProjectID, workspaceID WorkspaceID, indexer *Indexer) (IngestionResult, error) {
	start := time.Now()
	res := IngestionResult{
		ProjectID: projectID,
	}

	if err := checkContext(ctx); err != nil {
		return res, err
	}

	// Discover current Vault notes
	refs, err := p.discovery.Discover(ctx, projectID)
	if err != nil {
		return res, err
	}
	res.Discovered = len(refs)

	// Build a map of current source hashes
	currentSources := make(map[string]SourceRef)
	for _, ref := range refs {
		currentSources[ref.RelativePath] = ref
	}

	// Get existing knowledge items for this project
	existingItems, err := p.knowledge.List(ctx, Filter{
		ProjectID:  projectIDPtr(string(projectID)),
		ActiveOnly: true,
	})
	if err != nil {
		return res, err
	}

	// Map existing items by source URI
	existingBySource := make(map[string][]Item)
	for _, item := range existingItems {
		existingBySource[item.Provenance.SourceURI] = append(existingBySource[item.Provenance.SourceURI], item)
	}

	// Process each current source
	for _, ref := range refs {
		if err := checkContext(ctx); err != nil {
			return res, err
		}

		// Check if content has changed. A source may have multiple historical
		// chunks; any active chunk carrying the current authoritative source hash
		// means the source is already indexed.
		existing := existingBySource[ref.RelativePath]
		changed := len(existing) == 0
		if !changed {
			changed = true
			for _, item := range existing {
				if item.Provenance.SourceHash == ref.ContentHash {
					changed = false
					break
				}
			}
		}

		if changed {
			// Remove old chunks from retrieval and tombstone old knowledge items
			for _, oldItem := range existing {
				_ = indexer.RemoveItem(ctx, oldItem)
				_ = p.knowledge.Delete(ctx, oldItem.ID)
			}

			note, err := p.vault.GetNote(ctx, ref.NoteID)
			if err != nil {
				res.Errors = append(res.Errors, IngestionError{SourceRef: ref, Operation: "get_note", Err: err})
				continue
			}
			ingested, err := p.ingestNote(ctx, note, workspaceID)
			if err != nil {
				res.Errors = append(res.Errors, IngestionError{SourceRef: ref, Operation: "ingest_note", Err: err})
				continue
			}
			if ingested {
				res.Ingested++
			} else {
				res.Skipped++
			}

			// Re-index the new knowledge items
			newItems, err := p.knowledge.List(ctx, Filter{
				ProjectID:  projectIDPtr(string(projectID)),
				ActiveOnly: true,
			})
			if err == nil {
				for _, item := range newItems {
					if item.Provenance.SourceURI == ref.RelativePath {
						_ = indexer.IndexItem(ctx, item)
					}
				}
			}
		} else {
			res.Skipped++
		}
	}

	// Remove items for deleted sources
	for sourceURI, items := range existingBySource {
		if _, exists := currentSources[sourceURI]; !exists {
			for _, item := range items {
				if err := checkContext(ctx); err != nil {
					return res, err
				}
				if err := indexer.RemoveItem(ctx, item); err != nil {
					res.Errors = append(res.Errors, IngestionError{SourceRef: SourceRef{RelativePath: sourceURI}, Operation: "remove_item", Err: err})
					continue
				}
				// Tombstone the knowledge item
				if err := p.knowledge.Delete(ctx, item.ID); err != nil {
					res.Errors = append(res.Errors, IngestionError{SourceRef: SourceRef{RelativePath: sourceURI}, Operation: "tombstone", Err: err})
				}
			}
		}
	}

	res.Duration = time.Since(start)
	return res, nil
}

// RebuildAll performs a full rebuild from Vault.
func (p *Pipeline) RebuildAll(ctx context.Context, projectID workspace.ProjectID, workspaceID WorkspaceID, indexer *Indexer) (IngestionResult, error) {
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

	// Get all existing items and tombstone them
	existingItems, err := p.knowledge.List(ctx, Filter{
		ProjectID: projectIDPtr(string(projectID)),
	})
	if err != nil {
		return res, err
	}

	for _, item := range existingItems {
		if err := checkContext(ctx); err != nil {
			return res, err
		}
		if err := p.knowledge.Delete(ctx, item.ID); err != nil {
			res.Errors = append(res.Errors, IngestionError{SourceRef: SourceRef{RelativePath: item.Provenance.SourceURI}, Operation: "tombstone", Err: err})
		}
	}

	// Re-ingest all notes
	for _, ref := range refs {
		if err := checkContext(ctx); err != nil {
			return res, err
		}
		note, err := p.vault.GetNote(ctx, ref.NoteID)
		if err != nil {
			res.Errors = append(res.Errors, IngestionError{SourceRef: ref, Operation: "get_note", Err: err})
			continue
		}
		ingested, err := p.ingestNote(ctx, note, workspaceID)
		if err != nil {
			res.Errors = append(res.Errors, IngestionError{SourceRef: ref, Operation: "ingest_note", Err: err})
			continue
		}
		if ingested {
			res.Ingested++
		} else {
			res.Skipped++
		}
	}

	// Re-index all active items using RebuildProject (which handles corruption)
	allItems, err := p.knowledge.List(ctx, Filter{
		ProjectID:  projectIDPtr(string(projectID)),
		ActiveOnly: true,
	})
	if err != nil {
		return res, err
	}
	if err := indexer.RebuildProject(ctx, ProjectID(projectID), allItems); err != nil {
		return res, err
	}

	res.Duration = time.Since(start)
	return res, nil
}
