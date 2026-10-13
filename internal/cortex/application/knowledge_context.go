package application

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// KnowledgeContextSchemaVersion is the stable contract version for retrieval grounding.
const KnowledgeContextSchemaVersion = "cortexos.knowledge.context.v1"

var (
	ErrKnowledgeContextInvalidRequest = errors.New("invalid knowledge context request")
	ErrKnowledgeContextBudgetExceeded = errors.New("knowledge context budget exceeded")
)

var (
	rxAbsWinPath  = regexp.MustCompile(`(?i)[a-zA-Z]:[\\/]`)
	rxAbsUnixPath = regexp.MustCompile(`(?:^|[\s"'])(\/(?:etc|var|home|Users|usr|bin|tmp|opt|private)[\/\w.-]*)`)
	rxSecretToken = regexp.MustCompile(`(?i)(api[_-]?key|bearer\s+[a-z0-9_\-\.]+|password|secret|token)[=:\s]+[^\s,;]+`)
)

// KnowledgeContextBudget defines bounded limits on retrieved context.
type KnowledgeContextBudget struct {
	// MaxItems bounds the total number of documents/items returned. Default 5, max 20.
	MaxItems int `json:"maxItems"`
	// MaxSnippetChars bounds characters per snippet. Default 400, max 2000.
	MaxSnippetChars int `json:"maxSnippetChars"`
	// MaxTotalChars bounds cumulative characters across all snippets. Default 2000, max 8000.
	MaxTotalChars int `json:"maxTotalChars"`
	// TimeoutMs bounds retrieval duration before degrading safely. Default 2000ms, max 10000ms.
	TimeoutMs int `json:"timeoutMs"`
}

// DefaultKnowledgeContextBudget returns safe, bounded production defaults.
func DefaultKnowledgeContextBudget() KnowledgeContextBudget {
	return KnowledgeContextBudget{
		MaxItems:        5,
		MaxSnippetChars: 400,
		MaxTotalChars:   2000,
		TimeoutMs:       2000,
	}
}

// WithDefaults applies safe defaults for zero or unset fields.
func (b KnowledgeContextBudget) WithDefaults() KnowledgeContextBudget {
	def := DefaultKnowledgeContextBudget()
	if b.MaxItems <= 0 {
		b.MaxItems = def.MaxItems
	}
	if b.MaxSnippetChars <= 0 {
		b.MaxSnippetChars = def.MaxSnippetChars
	}
	if b.MaxTotalChars <= 0 {
		b.MaxTotalChars = def.MaxTotalChars
	}
	if b.TimeoutMs <= 0 {
		b.TimeoutMs = def.TimeoutMs
	}
	return b
}

// Validate ensures budget values stay within hard ceilings.
func (b KnowledgeContextBudget) Validate() error {
	if b.MaxItems < 0 || b.MaxSnippetChars < 0 || b.MaxTotalChars < 0 || b.TimeoutMs < 0 {
		return fmt.Errorf("%w: budget values cannot be negative", ErrKnowledgeContextInvalidRequest)
	}
	if b.MaxItems > 20 {
		return fmt.Errorf("%w: maxItems %d exceeds ceiling of 20", ErrKnowledgeContextInvalidRequest, b.MaxItems)
	}
	if b.MaxSnippetChars > 2000 {
		return fmt.Errorf("%w: maxSnippetChars %d exceeds ceiling of 2000", ErrKnowledgeContextInvalidRequest, b.MaxSnippetChars)
	}
	if b.MaxTotalChars > 8000 {
		return fmt.Errorf("%w: maxTotalChars %d exceeds ceiling of 8000", ErrKnowledgeContextInvalidRequest, b.MaxTotalChars)
	}
	if b.TimeoutMs > 10000 {
		return fmt.Errorf("%w: timeoutMs %d exceeds ceiling of 10000", ErrKnowledgeContextInvalidRequest, b.TimeoutMs)
	}
	return nil
}

// KnowledgeContextRequest requests retrieval grounding for a prompt or query.
type KnowledgeContextRequest struct {
	ProjectID     string                 `json:"projectId"`
	Query         string                 `json:"query"`
	Budget        KnowledgeContextBudget `json:"budget"`
	SchemaVersion string                 `json:"schemaVersion"`
}

// Validate checks request validity and bounds.
func (r KnowledgeContextRequest) Validate() error {
	if r.SchemaVersion != KnowledgeContextSchemaVersion {
		return fmt.Errorf("%w: unsupported schema version %q", ErrKnowledgeContextInvalidRequest, r.SchemaVersion)
	}
	if strings.TrimSpace(r.ProjectID) == "" {
		return fmt.Errorf("%w: projectId is required", ErrKnowledgeContextInvalidRequest)
	}
	if strings.TrimSpace(r.Query) == "" {
		return fmt.Errorf("%w: query is required", ErrKnowledgeContextInvalidRequest)
	}
	if len(r.Query) > 4000 {
		return fmt.Errorf("%w: query exceeds maximum length of 4000 chars", ErrKnowledgeContextInvalidRequest)
	}
	return r.Budget.Validate()
}

// KnowledgeContextItem is one sanitized, provenance-labeled snippet of context.
type KnowledgeContextItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	SourceLabel string  `json:"sourceLabel"`
	Snippet     string  `json:"snippet"`
	Score       float64 `json:"score"`
}

// GroundedContextRef is an opaque reference to a grounded item for routing results and events.
type GroundedContextRef struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	SourceLabel string `json:"sourceLabel"`
}

// KnowledgeContextBundle contains the complete bounded retrieval grounding result.
type KnowledgeContextBundle struct {
	Items          []KnowledgeContextItem `json:"items"`
	TotalChars     int                    `json:"totalChars"`
	Truncated      bool                   `json:"truncated"`
	Degraded       bool                   `json:"degraded"`
	DegradedReason string                 `json:"degradedReason,omitempty"`
	SchemaVersion  string                 `json:"schemaVersion"`
}

// Refs returns sanitized summary references for inclusion in route results/events.
func (b KnowledgeContextBundle) Refs() []GroundedContextRef {
	if len(b.Items) == 0 {
		return nil
	}
	refs := make([]GroundedContextRef, 0, len(b.Items))
	for _, it := range b.Items {
		refs = append(refs, GroundedContextRef{
			ID:          it.ID,
			Title:       it.Title,
			SourceLabel: it.SourceLabel,
		})
	}
	return refs
}

// KnowledgeGroundingPort is the application boundary for retrieval context assembly.
// Implementations query Knowledge through KnowledgePort and enforce budget policy.
type KnowledgeGroundingPort interface {
	GroundRoutingContext(ctx context.Context, req KnowledgeContextRequest) (KnowledgeContextBundle, error)
}

// SanitizeSourceLabel ensures relative paths, handles, and secret tokens do not leak in source labels.
func SanitizeSourceLabel(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "unknown"
	}
	s = filepath.ToSlash(s)
	// Absolute paths are intentionally reduced to a label, not returned as paths.
	if len(s) >= 2 && s[1] == ':' {
		s = s[2:]
	}
	for _, prefix := range []string{"/users/", "/home/"} {
		if idx := strings.Index(strings.ToLower(s), prefix); idx >= 0 {
			after := s[idx+len(prefix):]
			parts := strings.SplitN(after, "/", 2)
			if len(parts) == 2 {
				s = parts[1]
			}
		}
	}
	for _, sys := range []string{"/etc", "/var", "/tmp", "/opt", "/usr", "/bin", "/private"} {
		if strings.HasPrefix(strings.ToLower(s), sys) {
			return "source"
		}
	}
	s = strings.TrimLeft(s, "/")
	if s == "" {
		return "source"
	}
	if len(s) > 120 {
		s = s[:120]
	}
	return s
}

// SanitizeSnippet ensures text does not contain raw secrets or process handles.
func SanitizeSnippet(raw string, maxLen int) (string, bool) {
	clean := rxSecretToken.ReplaceAllString(raw, "[REDACTED]")
	clean = rxAbsWinPath.ReplaceAllString(clean, "[PATH]")
	clean = rxAbsUnixPath.ReplaceAllString(clean, "[PATH]")
	clean = strings.TrimSpace(clean)

	if maxLen <= 0 || len(clean) <= maxLen {
		return clean, false
	}
	if maxLen <= 3 {
		return clean[:maxLen], true
	}
	return clean[:maxLen-3] + "...", true
}

// BuildGroundedContextPrompt produces deterministic, bounded context text for planning prompts.
func BuildGroundedContextPrompt(bundle KnowledgeContextBundle) string {
	if len(bundle.Items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nGrounding Context:")
	for i, it := range bundle.Items {
		fmt.Fprintf(&b, "\n[%d] %s (%s): %s", i+1, it.Title, it.SourceLabel, it.Snippet)
	}
	return b.String()
}

// FoldKnowledgeQueryResults builds a bounded, deterministic KnowledgeContextBundle from query results.
func FoldKnowledgeQueryResults(results []KnowledgeQueryResult, budget KnowledgeContextBudget) KnowledgeContextBundle {
	b := budget.WithDefaults()
	if len(results) == 0 {
		return KnowledgeContextBundle{
			Items:         []KnowledgeContextItem{},
			TotalChars:    0,
			Truncated:     false,
			Degraded:      false,
			SchemaVersion: KnowledgeContextSchemaVersion,
		}
	}

	// Deterministic sorting: highest score first, then ID
	sorted := make([]KnowledgeQueryResult, len(results))
	copy(sorted, results)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Score != sorted[j].Score {
			return sorted[i].Score > sorted[j].Score
		}
		return sorted[i].DocumentID < sorted[j].DocumentID
	})

	maxItems := b.MaxItems
	if len(sorted) < maxItems {
		maxItems = len(sorted)
	}

	items := make([]KnowledgeContextItem, 0, maxItems)
	totalChars := 0
	wasTruncated := len(sorted) > b.MaxItems

	for _, r := range sorted[:maxItems] {
		snip, snipTrunc := SanitizeSnippet(r.Snippet, b.MaxSnippetChars)
		if snipTrunc {
			wasTruncated = true
		}
		if totalChars+len(snip) > b.MaxTotalChars {
			rem := b.MaxTotalChars - totalChars
			if rem > 6 {
				snip = snip[:rem-3] + "..."
				wasTruncated = true
			} else {
				wasTruncated = true
				break
			}
		}
		totalChars += len(snip)
		items = append(items, KnowledgeContextItem{
			ID:          r.DocumentID,
			Title:       r.Title,
			SourceLabel: SanitizeSourceLabel(r.RelativePath),
			Snippet:     snip,
			Score:       r.Score,
		})
		if totalChars >= b.MaxTotalChars {
			wasTruncated = true
			break
		}
	}

	return KnowledgeContextBundle{
		Items:         items,
		TotalChars:    totalChars,
		Truncated:     wasTruncated,
		Degraded:      false,
		SchemaVersion: KnowledgeContextSchemaVersion,
	}
}
