package bridge

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// KnowledgeGroundingAdapter assembles bounded routing context through the
// application KnowledgePort. It never exposes raw provider or storage details.
type KnowledgeGroundingAdapter struct {
	knowledge application.KnowledgePort
}

func NewKnowledgeGroundingAdapter(knowledge application.KnowledgePort) (application.KnowledgeGroundingPort, error) {
	if knowledge == nil {
		return nil, errors.New("knowledge port cannot be nil")
	}
	return &KnowledgeGroundingAdapter{knowledge: knowledge}, nil
}

func (a *KnowledgeGroundingAdapter) GroundRoutingContext(ctx context.Context, req application.KnowledgeContextRequest) (application.KnowledgeContextBundle, error) {
	if a == nil || a.knowledge == nil {
		return application.KnowledgeContextBundle{}, errors.New("knowledge grounding is not configured")
	}
	if err := req.Validate(); err != nil {
		return application.KnowledgeContextBundle{}, err
	}
	if err := ctx.Err(); err != nil {
		return application.KnowledgeContextBundle{}, err
	}

	budget := req.Budget.WithDefaults()
	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(budget.TimeoutMs)*time.Millisecond)
	defer cancel()

	results, err := a.knowledge.QueryKnowledge(queryCtx, req.ProjectID, strings.TrimSpace(req.Query), budget.MaxItems)
	if err != nil {
		if errors.Is(err, context.Canceled) && ctx.Err() != nil {
			return application.KnowledgeContextBundle{}, ctx.Err()
		}
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(queryCtx.Err(), context.DeadlineExceeded) {
			return degradedContext("retrieval_timeout"), nil
		}
		// Retrieval is advisory: route execution continues without untrusted context.
		return degradedContext("retrieval_unavailable"), nil
	}

	bundle := application.FoldKnowledgeQueryResults(results, budget)
	if len(bundle.Items) == 0 {
		bundle.Degraded = true
		bundle.DegradedReason = "no_matches"
	}
	return bundle, nil
}

func degradedContext(reason string) application.KnowledgeContextBundle {
	return application.KnowledgeContextBundle{
		Items:          []application.KnowledgeContextItem{},
		Degraded:       true,
		DegradedReason: reason,
		SchemaVersion:  application.KnowledgeContextSchemaVersion,
	}
}
