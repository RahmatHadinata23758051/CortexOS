package application

import (
	"errors"
	"strings"
	"testing"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/orchestra"
)

func validBrainOrchestraRouteRequest() BrainOrchestraRouteRequest {
	return BrainOrchestraRouteRequest{
		CorrelationID:      "route-1",
		UserPrompt:         "implement the requested change",
		Title:              "Implement requested change",
		AcceptanceCriteria: []string{"tests pass"},
		SchemaVersion:      BrainOrchestraBridgeSchemaVersion,
	}
}

func validBrainOrchestraPlan() BrainOrchestraPlan {
	return BrainOrchestraPlan{
		SchemaVersion: BrainOrchestraPlanSchemaVersion,
		Steps: []BrainOrchestraPlanStep{
			{Index: 0, Title: "Implement change", Action: "code", Resource: "module"},
			{Index: 1, Title: "Verify change", Action: "test", Resource: "module", Dependencies: []int{0}},
		},
		TotalEstimatedTokens: 100,
	}
}

func TestBrainOrchestraRouteRequestValidation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*BrainOrchestraRouteRequest)
	}{
		{"schema", func(r *BrainOrchestraRouteRequest) { r.SchemaVersion = "wrong" }},
		{"correlation", func(r *BrainOrchestraRouteRequest) { r.CorrelationID = "" }},
		{"prompt", func(r *BrainOrchestraRouteRequest) { r.UserPrompt = "" }},
		{"title", func(r *BrainOrchestraRouteRequest) { r.Title = "" }},
		{"criteria", func(r *BrainOrchestraRouteRequest) { r.AcceptanceCriteria = nil }},
		{"too many criteria", func(r *BrainOrchestraRouteRequest) { r.AcceptanceCriteria = make([]string, 21) }},
		{"too many dependencies", func(r *BrainOrchestraRouteRequest) { r.Dependencies = make([]orchestra.TaskID, 11) }},
		{"step budget", func(r *BrainOrchestraRouteRequest) { r.MaxPlanSteps = 21 }},
		{"token budget", func(r *BrainOrchestraRouteRequest) { r.MaxPlanTokens = 32769 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := validBrainOrchestraRouteRequest()
			tt.edit(&req)
			if err := req.Validate(); err == nil || !errors.Is(err, ErrBrainOrchestraInvalidRequest) {
				t.Fatalf("expected invalid request, got %v", err)
			}
		})
	}

	req := validBrainOrchestraRouteRequest().WithDefaults()
	if req.MaxPlanSteps != 8 || req.MaxPlanTokens != 4096 {
		t.Fatalf("unexpected defaults: %#v", req)
	}
}

func TestBrainOrchestraPlanValidation(t *testing.T) {
	plan := validBrainOrchestraPlan()
	if err := plan.Validate(); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}

	tests := []struct {
		name string
		edit func(*BrainOrchestraPlan)
	}{
		{"schema", func(p *BrainOrchestraPlan) { p.SchemaVersion = "wrong" }},
		{"empty", func(p *BrainOrchestraPlan) { p.Steps = nil }},
		{"index", func(p *BrainOrchestraPlan) { p.Steps[1].Index = 3 }},
		{"title", func(p *BrainOrchestraPlan) { p.Steps[0].Title = "" }},
		{"action", func(p *BrainOrchestraPlan) { p.Steps[0].Action = "" }},
		{"resource", func(p *BrainOrchestraPlan) { p.Steps[0].Resource = "" }},
		{"forward dependency", func(p *BrainOrchestraPlan) { p.Steps[0].Dependencies = []int{1} }},
		{"out of range dependency", func(p *BrainOrchestraPlan) { p.Steps[1].Dependencies = []int{2} }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := validBrainOrchestraPlan()
			tt.edit(&candidate)
			if err := candidate.Validate(); err == nil || !errors.Is(err, ErrBrainOrchestraInvalidRequest) {
				t.Fatalf("expected invalid plan, got %v", err)
			}
		})
	}
}

func TestBrainOrchestraContractsDoNotExposeSecretsOrPaths(t *testing.T) {
	req := validBrainOrchestraRouteRequest()
	plan := validBrainOrchestraPlan()

	encoded := req.CorrelationID + req.UserPrompt + req.Title + strings.Join(req.AcceptanceCriteria, "")
	for _, step := range plan.Steps {
		encoded += step.Title + step.Action + step.Resource + strings.Join(step.AcceptanceCriteria, "")
	}
	for _, forbidden := range []string{"apiKey", "authorization", "processHandle", `C:\`, "/Users/", "/home/"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("contract contains forbidden value %q: %s", forbidden, encoded)
		}
	}
}
