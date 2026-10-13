package brain

import (
	"context"
	"fmt"
	"strings"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// ContextAssembler collects and formats relevant context from knowledge and memory sources.
type ContextAssembler struct {
	knowledge application.KnowledgePort
	maxChars  int
}

// NewContextAssembler creates an assembler with the given KnowledgePort.
// maxChars bounds the total characters across all assembled context snippets.
func NewContextAssembler(k application.KnowledgePort, maxChars int) *ContextAssembler {
	if maxChars <= 0 {
		maxChars = 4000
	}
	return &ContextAssembler{
		knowledge: k,
		maxChars:  maxChars,
	}
}

// AssembleQuery fetches relevant knowledge documents for a query and formats them safely.
// Absolute filesystem paths and raw embeddings are omitted per architecture policy.
func (a *ContextAssembler) AssembleQuery(ctx context.Context, projectID, query string, limit int) ([]string, error) {
	if a == nil || a.knowledge == nil {
		return nil, nil
	}
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}

	results, err := a.knowledge.QueryKnowledge(ctx, projectID, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query knowledge: %w", err)
	}

	snippets := make([]string, 0, len(results))
	totalChars := 0

	for _, r := range results {
		snippet := strings.TrimSpace(r.Snippet)
		if snippet == "" {
			continue
		}
		// Bound individual and cumulative length
		formatted := fmt.Sprintf("[%s] %s", r.Title, snippet)
		if totalChars+len(formatted) > a.maxChars {
			remaining := a.maxChars - totalChars
			if remaining > 6 {
				formatted = formatted[:remaining-3] + "..."
				snippets = append(snippets, formatted)
			}
			break
		}
		totalChars += len(formatted)
		snippets = append(snippets, formatted)
	}

	return snippets, nil
}
