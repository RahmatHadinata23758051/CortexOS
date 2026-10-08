package knowledge

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var knowledgeIDCounter uint64

func generateKnowledgeID() KnowledgeID {
	count := atomic.AddUint64(&knowledgeIDCounter, 1)
	return KnowledgeID(fmt.Sprintf("kno-%d-%d", time.Now().UTC().UnixNano(), count))
}

// Service implements the KnowledgePort interface, governing knowledge lifecycle,
// provenance verification, and validation gates.
type Service struct {
	store Store
	mu    sync.RWMutex
}

// NewService constructs a validated knowledge lifecycle service.
func NewService(store Store) (*Service, error) {
	if store == nil {
		return nil, newError(ErrInvalidRequest, "knowledge store is required")
	}
	return &Service{store: store}, nil
}

// Capture ingests candidate knowledge into a draft, unvalidated state.
// In compliance with ADR-0005, unvalidated worker claims cannot become durable active knowledge.
func (s *Service) Capture(ctx context.Context, req CaptureRequest) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	if req.WorkspaceID == "" || req.ProjectID == "" {
		return Item{}, newError(ErrInvalidRequest, "workspace id and project id are required")
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Content) == "" {
		return Item{}, newError(ErrInvalidRequest, "title and content are required")
	}
	if !validKind(req.Kind) {
		return Item{}, newError(ErrInvalidRequest, fmt.Sprintf("unsupported knowledge kind %q", req.Kind))
	}

	now := time.Now().UTC()
	prov := req.Provenance
	if prov.CapturedAt.IsZero() {
		prov.CapturedAt = now
	}
	if prov.SourceKind == "" {
		prov.SourceKind = "capture"
	}
	if prov.Author == "" {
		prov.Author = "anonymous"
	}

	id := generateKnowledgeID()

	item := Item{
		ID:               id,
		WorkspaceID:      req.WorkspaceID,
		ProjectID:        req.ProjectID,
		Title:            strings.TrimSpace(req.Title),
		Content:          req.Content,
		Kind:             req.Kind,
		Tags:             req.Tags,
		Provenance:       prov,
		Lifecycle:        LifecycleDraft,
		ValidationStatus: StatusUnvalidated,
		ContentHash:      HashContent(req.Content),
		Version:          1,
		CreatedAt:        now,
		UpdatedAt:        now,
		ExpiresAt:        req.ExpiresAt,
		SchemaVersion:    ContractVersion,
	}

	if err := item.Validate(); err != nil {
		return Item{}, err
	}

	return s.store.Save(ctx, item)
}

// Validate approves and promotes a draft knowledge item to Active state.
// Requires authority (Inspector, Orchestra, or User).
func (s *Service) Validate(ctx context.Context, id KnowledgeID, validatedBy, reason string) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	if id == "" {
		return Item{}, newError(ErrInvalidRequest, "knowledge id is required")
	}
	if strings.TrimSpace(validatedBy) == "" {
		return Item{}, newError(ErrInvalidRequest, "validatedBy authority is required")
	}

	item, err := s.store.Get(ctx, id)
	if err != nil {
		return Item{}, err
	}

	if item.Lifecycle == LifecycleTombstone {
		return Item{}, newError(ErrConflict, "cannot validate tombstoned knowledge item")
	}
	if item.ValidationStatus == StatusRejected {
		return Item{}, newError(ErrInvalidRequest, "rejected knowledge item cannot be promoted")
	}

	now := time.Now().UTC()
	item.ValidationStatus = StatusVerified
	item.Lifecycle = LifecycleActive
	item.UpdatedAt = now
	item.Provenance.ValidatedAt = &now
	item.Provenance.ValidatedBy = validatedBy
	item.Provenance.ValidationReason = reason

	if err := item.Validate(); err != nil {
		return Item{}, err
	}

	return s.store.Save(ctx, item)
}

// Reject marks an item as rejected, preventing it from ever being promoted to Active.
func (s *Service) Reject(ctx context.Context, id KnowledgeID, validatedBy, reason string) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	if id == "" {
		return Item{}, newError(ErrInvalidRequest, "knowledge id is required")
	}

	item, err := s.store.Get(ctx, id)
	if err != nil {
		return Item{}, err
	}

	now := time.Now().UTC()
	item.ValidationStatus = StatusRejected
	item.Lifecycle = LifecycleDraft
	item.UpdatedAt = now
	item.Provenance.ValidatedAt = &now
	item.Provenance.ValidatedBy = validatedBy
	item.Provenance.ValidationReason = reason

	if err := item.Validate(); err != nil {
		return Item{}, err
	}

	return s.store.Save(ctx, item)
}

// Update amends content and metadata, resetting the item to Draft/Unvalidated.
func (s *Service) Update(ctx context.Context, req UpdateRequest) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	if req.ID == "" {
		return Item{}, newError(ErrInvalidRequest, "knowledge id is required")
	}

	item, err := s.store.Get(ctx, req.ID)
	if err != nil {
		return Item{}, err
	}

	if item.Lifecycle == LifecycleTombstone {
		return Item{}, newError(ErrConflict, "cannot update tombstoned knowledge item")
	}

	now := time.Now().UTC()
	if strings.TrimSpace(req.Title) != "" {
		item.Title = strings.TrimSpace(req.Title)
	}
	if strings.TrimSpace(req.Content) != "" {
		item.Content = req.Content
		item.ContentHash = HashContent(req.Content)
	}
	if req.Kind != "" && validKind(req.Kind) {
		item.Kind = req.Kind
	}
	if req.Tags != nil {
		item.Tags = req.Tags
	}
	if req.ExpiresAt != nil {
		item.ExpiresAt = req.ExpiresAt
	}

	// Any update invalidates prior validation status
	item.ValidationStatus = StatusUnvalidated
	item.Lifecycle = LifecycleDraft
	item.Version++
	item.UpdatedAt = now

	// Track update provenance
	if req.Provenance.SourceKind != "" {
		item.Provenance.SourceKind = req.Provenance.SourceKind
	}
	if req.Provenance.Author != "" {
		item.Provenance.Author = req.Provenance.Author
	}
	if req.Provenance.Attribution != "" {
		item.Provenance.Attribution = req.Provenance.Attribution
	}

	if err := item.Validate(); err != nil {
		return Item{}, err
	}

	return s.store.Save(ctx, item)
}

// Correct applies a verified correction with an audit trail and preserves attribution.
func (s *Service) Correct(ctx context.Context, req CorrectionRequest) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	if req.ID == "" {
		return Item{}, newError(ErrInvalidRequest, "knowledge id is required")
	}
	if strings.TrimSpace(req.Content) == "" {
		return Item{}, newError(ErrInvalidRequest, "corrected content is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return Item{}, newError(ErrInvalidRequest, "correction reason is required")
	}

	item, err := s.store.Get(ctx, req.ID)
	if err != nil {
		return Item{}, err
	}

	now := time.Now().UTC()
	item.Content = req.Content
	item.ContentHash = HashContent(req.Content)
	item.Version++
	item.UpdatedAt = now
	item.Provenance.ValidationReason = "correction: " + req.Reason
	if req.Author != "" {
		item.Provenance.Author = req.Author
	}
	if req.License != "" {
		item.Provenance.License = req.License
	}

	if err := item.Validate(); err != nil {
		return Item{}, err
	}

	return s.store.Save(ctx, item)
}

// Deprecate marks an active knowledge item as deprecated.
func (s *Service) Deprecate(ctx context.Context, id KnowledgeID) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	item, err := s.store.Get(ctx, id)
	if err != nil {
		return Item{}, err
	}
	item.Lifecycle = LifecycleDeprecated
	item.UpdatedAt = time.Now().UTC()
	if err := item.Validate(); err != nil {
		return Item{}, err
	}
	return s.store.Save(ctx, item)
}

// Expire transitions an item to Expired state.
func (s *Service) Expire(ctx context.Context, id KnowledgeID) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	item, err := s.store.Get(ctx, id)
	if err != nil {
		return Item{}, err
	}
	item.Lifecycle = LifecycleExpired
	item.UpdatedAt = time.Now().UTC()
	if err := item.Validate(); err != nil {
		return Item{}, err
	}
	return s.store.Save(ctx, item)
}

// Get retrieves an item by ID.
func (s *Service) Get(ctx context.Context, id KnowledgeID) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	return s.store.Get(ctx, id)
}

// List returns items matching safe filters.
func (s *Service) List(ctx context.Context, filter Filter) ([]Item, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	return s.store.List(ctx, filter)
}

// PruneExpired delegates to store for removing expired items.
func (s *Service) PruneExpired(ctx context.Context) (int, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	return s.store.PruneExpired(ctx)
}

// MemoryStore is an in-memory thread-safe implementation of Store for testing and default use.
type MemoryStore struct {
	mu    sync.RWMutex
	items map[KnowledgeID]Item
	open  bool
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		items: make(map[KnowledgeID]Item),
		open:  true,
	}
}

func (m *MemoryStore) Open(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = true
	return nil
}

func (m *MemoryStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.open = false
	return nil
}

func (m *MemoryStore) SchemaVersion(ctx context.Context) (string, error) {
	return ContractVersion, nil
}

func (m *MemoryStore) Save(ctx context.Context, item Item) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	if err := item.Validate(); err != nil {
		return Item{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.items[item.ID] = item
	return item, nil
}

func (m *MemoryStore) Get(ctx context.Context, id KnowledgeID) (Item, error) {
	if err := checkContext(ctx); err != nil {
		return Item{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.items[id]
	if !ok {
		return Item{}, WrapError(ErrNotFound, fmt.Sprintf("knowledge item %q not found", id), nil)
	}
	return item, nil
}

func (m *MemoryStore) Delete(ctx context.Context, id KnowledgeID) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	item, ok := m.items[id]
	if !ok {
		return WrapError(ErrNotFound, fmt.Sprintf("knowledge item %q not found", id), nil)
	}
	item.Lifecycle = LifecycleTombstone
	item.UpdatedAt = time.Now().UTC()
	m.items[id] = item
	return nil
}

func (m *MemoryStore) List(ctx context.Context, filter Filter) ([]Item, error) {
	if err := checkContext(ctx); err != nil {
		return nil, err
	}
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now().UTC()
	var res []Item
	for _, item := range m.items {
		if filter.WorkspaceID != nil && item.WorkspaceID != *filter.WorkspaceID {
			continue
		}
		if filter.ProjectID != nil && item.ProjectID != *filter.ProjectID {
			continue
		}
		if filter.Kind != nil && item.Kind != *filter.Kind {
			continue
		}
		if filter.Lifecycle != nil && item.Lifecycle != *filter.Lifecycle {
			continue
		}
		if filter.ValidationStatus != nil && item.ValidationStatus != *filter.ValidationStatus {
			continue
		}
		if filter.ActiveOnly && item.Lifecycle != LifecycleActive {
			continue
		}
		if filter.ActiveOnly && item.IsExpired(now) {
			continue
		}
		if filter.MinVersion > 0 && item.Version < filter.MinVersion {
			continue
		}
		res = append(res, item)
	}
	SortItems(res)
	return res, nil
}

func (m *MemoryStore) PruneExpired(ctx context.Context) (int, error) {
	if err := checkContext(ctx); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	pruned := 0
	for id, item := range m.items {
		if item.IsExpired(now) {
			item.Lifecycle = LifecycleExpired
			item.UpdatedAt = now
			m.items[id] = item
			pruned++
		}
	}
	return pruned, nil
}
