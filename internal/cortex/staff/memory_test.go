package staff

import (
	"context"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

type mockMemoryContextPort struct {
	mu       sync.RWMutex
	memories map[string]MemoryContext
}

func newMockMemoryContextPort() *mockMemoryContextPort {
	return &mockMemoryContextPort{memories: make(map[string]MemoryContext)}
}

func (m *mockMemoryContextPort) Save(ctx context.Context, memory MemoryContext) (MemoryContext, error) {
	if err := ctx.Err(); err != nil {
		return MemoryContext{}, err
	}
	if err := memory.Validate(); err != nil {
		return MemoryContext{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.memories[memory.ID] = memory
	return memory, nil
}

func (m *mockMemoryContextPort) Get(ctx context.Context, id string) (MemoryContext, error) {
	if err := ctx.Err(); err != nil {
		return MemoryContext{}, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	mem, ok := m.memories[id]
	if !ok {
		return MemoryContext{}, WrapError(ErrNotFound, "memory not found", nil)
	}
	return mem, nil
}

func (m *mockMemoryContextPort) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.memories[id]; !ok {
		return WrapError(ErrNotFound, "memory not found", nil)
	}
	delete(m.memories, id)
	return nil
}

func (m *mockMemoryContextPort) List(ctx context.Context, filter MemoryFilter) ([]MemoryContext, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := filter.Validate(); err != nil {
		return nil, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	var res []MemoryContext
	now := time.Now().UTC()
	for _, mem := range m.memories {
		if filter.StaffID != nil && mem.StaffID != *filter.StaffID {
			continue
		}
		if filter.WorkspaceID != nil && mem.WorkspaceID != *filter.WorkspaceID {
			continue
		}
		if filter.Kind != nil && mem.Kind != *filter.Kind {
			continue
		}
		if mem.Priority < filter.MinPriority || (filter.MaxPriority > 0 && mem.Priority > filter.MaxPriority) {
			continue
		}
		if filter.ActiveOnly && mem.IsExpired(now) {
			continue
		}
		res = append(res, mem)
	}
	sort.Slice(res, func(i, j int) bool {
		if res[i].Kind != res[j].Kind {
			return res[i].Kind < res[j].Kind
		}
		if res[i].Priority != res[j].Priority {
			return res[i].Priority > res[j].Priority
		}
		return res[i].CreatedAt.After(res[j].CreatedAt)
	})
	return res, nil
}

func (m *mockMemoryContextPort) Select(ctx context.Context, req MemorySelectionRequest) (MemorySelectionResult, error) {
	if err := ctx.Err(); err != nil {
		return MemorySelectionResult{}, err
	}
	if req.StaffID == "" {
		return MemorySelectionResult{}, newError(ErrInvalidRequest, "staff id required")
	}
	if req.WorkspaceID == "" {
		return MemorySelectionResult{}, newError(ErrInvalidRequest, "workspace id required")
	}
	if req.MaxItems <= 0 {
		req.MaxItems = 10
	}
	if req.MaxTotalChars <= 0 {
		req.MaxTotalChars = 8192
	}
	if req.MinRelevance < 0 {
		req.MinRelevance = 0
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	var candidates []MemoryContext
	now := time.Now().UTC()
	for _, mem := range m.memories {
		if mem.StaffID != req.StaffID {
			continue
		}
		if mem.WorkspaceID != req.WorkspaceID {
			continue
		}
		if req.ProjectID != "" && mem.WorkspaceID != WorkspaceID(req.ProjectID) {
			continue
		}
		if len(req.Kinds) > 0 {
			kindMatch := false
			for _, k := range req.Kinds {
				if mem.Kind == k {
					kindMatch = true
					break
				}
			}
			if !kindMatch {
				continue
			}
		}
		if mem.Relevance < req.MinRelevance {
			continue
		}
		if mem.IsExpired(now) {
			continue
		}
		if len(req.RequiredTags) > 0 {
			tagMatch := false
			for _, reqTag := range req.RequiredTags {
				for _, memTag := range mem.Tags {
					if reqTag == memTag {
						tagMatch = true
						break
					}
				}
				if tagMatch {
					break
				}
			}
			if !tagMatch {
				continue
			}
		}
		if len(req.ExcludeTags) > 0 {
			excluded := false
			for _, exclTag := range req.ExcludeTags {
				for _, memTag := range mem.Tags {
					if exclTag == memTag {
						excluded = true
						break
					}
				}
				if excluded {
					break
				}
			}
			if excluded {
				continue
			}
		}
		candidates = append(candidates, mem)
	}

	// Deterministic sort: relevance desc, priority desc, recency desc
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Relevance != candidates[j].Relevance {
			return candidates[i].Relevance > candidates[j].Relevance
		}
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		if req.PreferRecency {
			return candidates[i].CreatedAt.After(candidates[j].CreatedAt)
		}
		return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
	})

	var selected []MemoryContext
	totalChars := 0
	for _, mem := range candidates {
		if len(selected) >= req.MaxItems {
			break
		}
		if totalChars+len(mem.Content) > req.MaxTotalChars {
			break
		}
		selected = append(selected, mem)
		totalChars += len(mem.Content)
	}

	return MemorySelectionResult{
		ContractVersion:  MemorySelectionContractVersion,
		StaffID:          req.StaffID,
		SelectedContexts: selected,
		TotalChars:       totalChars,
		SelectionReason:  "deterministic selection by relevance, priority, recency",
		SelectedAt:       time.Now().UTC(),
	}, nil
}

func (m *mockMemoryContextPort) PruneExpired(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	pruned := 0
	for id, mem := range m.memories {
		if mem.IsExpired(now) {
			delete(m.memories, id)
			pruned++
		}
	}
	return pruned, nil
}

func TestMemoryContextValidation(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(24 * time.Hour)

	valid := MemoryContext{
		ID:          "mem-1",
		StaffID:     "staff-1",
		Kind:        MemoryKindShortTerm,
		Content:     "user prefers Go for backend",
		Provenance:  Provenance{Source: "user_input", Timestamp: now},
		Relevance:   0.9,
		Priority:    10,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   &expiresAt,
		WorkspaceID: "ws-1",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid memory should pass validation: %v", err)
	}

	// Invalid: missing id
	invalid := valid
	invalid.ID = ""
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected ErrInvalidRequest for missing id, got %v", err)
	}

	// Invalid: empty content
	invalid = valid
	invalid.Content = "   "
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected ErrInvalidRequest for empty content, got %v", err)
	}

	// Invalid: relevance out of range
	invalid = valid
	invalid.Relevance = 1.5
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected ErrInvalidRequest for relevance > 1, got %v", err)
	}

	// Invalid: unsupported kind
	invalid = valid
	invalid.Kind = MemoryKind("unknown")
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected ErrInvalidRequest for unknown kind, got %v", err)
	}

	// Invalid: expires before created
	invalid = valid
	past := now.Add(-1 * time.Hour)
	invalid.ExpiresAt = &past
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected ErrInvalidRequest for expires before created, got %v", err)
	}

	// Invalid: control characters in tags
	invalid = valid
	invalid.Tags = []string{"tag\x00"}
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected ErrInvalidRequest for control chars in tag, got %v", err)
	}
}

func TestMemoryContextExpiry(t *testing.T) {
	now := time.Now().UTC()
	expiresAt := now.Add(1 * time.Hour)

	mem := MemoryContext{
		ID:          "mem-expiring",
		StaffID:     "staff-1",
		Kind:        MemoryKindShortTerm,
		Content:     "temporary note",
		Provenance:  Provenance{Source: "test", Timestamp: now},
		Relevance:   0.5,
		Priority:    5,
		CreatedAt:   now,
		UpdatedAt:   now,
		ExpiresAt:   &expiresAt,
		WorkspaceID: "ws-1",
	}

	if mem.IsExpired(now) {
		t.Errorf("memory should not be expired at creation time")
	}
	if !mem.IsExpired(now.Add(2 * time.Hour)) {
		t.Errorf("memory should be expired after expiry time")
	}
}

func TestMemoryRedaction(t *testing.T) {
	mem := MemoryContext{
		ID:          "mem-secret",
		StaffID:     "staff-1",
		Kind:        MemoryKindShortTerm,
		Content:     "my email is user@example.com and api_key = secret123 and Bearer token_xyz",
		Provenance:  Provenance{Source: "test", Timestamp: time.Now().UTC()},
		Relevance:   0.5,
		Priority:    5,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
		WorkspaceID: "ws-1",
	}

	redacted := mem.RedactedContent()
	if redacted == mem.Content {
		t.Fatal("content should be redacted")
	}
	if !strings.Contains(redacted, "[EMAIL]") {
		t.Errorf("email not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED]") {
		t.Errorf("secret not redacted: %s", redacted)
	}
	if !strings.Contains(redacted, "Bearer [REDACTED]") {
		t.Errorf("bearer token not redacted: %s", redacted)
	}
}

func TestMemorySelection(t *testing.T) {
	ctx := context.Background()
	port := newMockMemoryContextPort()
	now := time.Now().UTC()

	// Seed memories of different kinds, relevance, priority
	memories := []MemoryContext{
		{ID: "mem-1", StaffID: "staff-dev", Kind: MemoryKindShortTerm, Content: "recent decision: use interfaces", Provenance: Provenance{Source: "tool_output", Timestamp: now}, Relevance: 0.9, Priority: 10, CreatedAt: now, UpdatedAt: now, WorkspaceID: "ws-main", Tags: []string{"decision"}},
		{ID: "mem-2", StaffID: "staff-dev", Kind: MemoryKindLongTerm, Content: "team convention: error handling", Provenance: Provenance{Source: "user_input", Timestamp: now.Add(-24 * time.Hour)}, Relevance: 0.7, Priority: 20, CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now.Add(-24 * time.Hour), WorkspaceID: "ws-main", Tags: []string{"convention"}},
		{ID: "mem-3", StaffID: "staff-dev", Kind: MemoryKindEpisodic, Content: "fixed bug #42 in auth", Provenance: Provenance{Source: "tool_output", Timestamp: now.Add(-48 * time.Hour)}, Relevance: 0.5, Priority: 5, CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now.Add(-48 * time.Hour), WorkspaceID: "ws-main", Tags: []string{"bug"}},
		{ID: "mem-4", StaffID: "staff-dev", Kind: MemoryKindShortTerm, Content: "secret: api_key=abc123", Provenance: Provenance{Source: "skill_injection", Timestamp: now}, Relevance: 0.8, Priority: 15, CreatedAt: now, UpdatedAt: now, WorkspaceID: "ws-main", Tags: []string{"secret"}},
		{ID: "mem-5", StaffID: "staff-other", Kind: MemoryKindShortTerm, Content: "other staff memory", Provenance: Provenance{Source: "test", Timestamp: now}, Relevance: 0.9, Priority: 10, CreatedAt: now, UpdatedAt: now, WorkspaceID: "ws-main"},
	}

	for _, mem := range memories {
		if _, err := port.Save(ctx, mem); err != nil {
			t.Fatalf("failed to save memory %s: %v", mem.ID, err)
		}
	}

	// Select with defaults
	result, err := port.Select(ctx, MemorySelectionRequest{
		StaffID:       "staff-dev",
		WorkspaceID:   "ws-main",
		MaxItems:      3,
		MaxTotalChars: 100,
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	if len(result.SelectedContexts) == 0 {
		t.Fatal("expected at least one selected context")
	}
	if result.TotalChars > 100 {
		t.Errorf("total chars %d exceeds limit 100", result.TotalChars)
	}

	// Check deterministic ordering (relevance desc, priority desc)
	if len(result.SelectedContexts) >= 2 {
		if result.SelectedContexts[0].Relevance < result.SelectedContexts[1].Relevance {
			t.Errorf("not sorted by relevance desc: %f < %f", result.SelectedContexts[0].Relevance, result.SelectedContexts[1].Relevance)
		}
	}

	// Filter by kind
	result, err = port.Select(ctx, MemorySelectionRequest{
		StaffID:     "staff-dev",
		WorkspaceID: "ws-main",
		Kinds:       []MemoryKind{MemoryKindLongTerm},
	})
	if err != nil {
		t.Fatalf("Select by kind failed: %v", err)
	}
	for _, mem := range result.SelectedContexts {
		if mem.Kind != MemoryKindLongTerm {
			t.Errorf("expected only long_term memories, got %s", mem.Kind)
		}
	}

	// Filter by tags
	result, err = port.Select(ctx, MemorySelectionRequest{
		StaffID:      "staff-dev",
		WorkspaceID:  "ws-main",
		RequiredTags: []string{"decision"},
		ExcludeTags:  []string{"secret"},
	})
	if err != nil {
		t.Fatalf("Select by tags failed: %v", err)
	}
	for _, mem := range result.SelectedContexts {
		if !contains(mem.Tags, "decision") {
			t.Errorf("expected 'decision' tag, got %v", mem.Tags)
		}
		if contains(mem.Tags, "secret") {
			t.Errorf("should have excluded 'secret' tag")
		}
	}

	// Workspace isolation - other staff should not appear
	result, err = port.Select(ctx, MemorySelectionRequest{
		StaffID:     "staff-dev",
		WorkspaceID: "ws-main",
	})
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}
	for _, mem := range result.SelectedContexts {
		if mem.StaffID != "staff-dev" {
			t.Errorf("workspace isolation violated: got memory from %s", mem.StaffID)
		}
	}
}

func TestMemoryFilterValidation(t *testing.T) {
	filter := MemoryFilter{
		StaffID:     staffIDPtr("staff-1"),
		WorkspaceID: workspaceIDPtr("ws-1"),
		Kind:        kindPtr(MemoryKindShortTerm),
	}
	if err := filter.Validate(); err != nil {
		t.Fatalf("valid filter should pass: %v", err)
	}

	invalid := filter
	invalid.Kind = kindPtr(MemoryKind("invalid"))
	if err := invalid.Validate(); err == nil || !IsInvalidRequest(err) {
		t.Errorf("expected error for invalid kind, got %v", err)
	}
}

func TestMemoryPortList(t *testing.T) {
	ctx := context.Background()
	port := newMockMemoryContextPort()
	now := time.Now().UTC()

	expiresAt := now.Add(-1 * time.Hour)
	memExpired := MemoryContext{
		ID:          "mem-expired",
		StaffID:     "staff-1",
		Kind:        MemoryKindShortTerm,
		Content:     "expired",
		Provenance:  Provenance{Source: "test", Timestamp: now},
		Relevance:   0.5,
		Priority:    5,
		CreatedAt:   now.Add(-2 * time.Hour),
		UpdatedAt:   now.Add(-2 * time.Hour),
		ExpiresAt:   &expiresAt,
		WorkspaceID: "ws-1",
	}
	memActive := MemoryContext{
		ID:          "mem-active",
		StaffID:     "staff-1",
		Kind:        MemoryKindShortTerm,
		Content:     "active",
		Provenance:  Provenance{Source: "test", Timestamp: now},
		Relevance:   0.5,
		Priority:    5,
		CreatedAt:   now,
		UpdatedAt:   now,
		WorkspaceID: "ws-1",
	}
	_, _ = port.Save(ctx, memExpired)
	_, _ = port.Save(ctx, memActive)

	// ActiveOnly = true should exclude expired
	list, err := port.List(ctx, MemoryFilter{ActiveOnly: true})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].ID != "mem-active" {
		t.Errorf("expected only active memory, got %v", list)
	}

	// ActiveOnly = false should include expired
	list, err = port.List(ctx, MemoryFilter{ActiveOnly: false})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 memories including expired, got %d", len(list))
	}

	// PruneExpired should remove expired
	pruned, err := port.PruneExpired(ctx)
	if err != nil {
		t.Fatalf("PruneExpired failed: %v", err)
	}
	if pruned != 1 {
		t.Errorf("expected 1 pruned, got %d", pruned)
	}
	list, _ = port.List(ctx, MemoryFilter{ActiveOnly: false})
	if len(list) != 1 || list[0].ID != "mem-active" {
		t.Errorf("expected only active after prune, got %v", list)
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func staffIDPtr(s StaffID) *StaffID             { return &s }
func workspaceIDPtr(w WorkspaceID) *WorkspaceID { return &w }
func kindPtr(k MemoryKind) *MemoryKind          { return &k }
