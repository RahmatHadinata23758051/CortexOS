package staff

import (
	"fmt"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/harness"
)

// PolicyTranslator converts Staff permission sets into Harness policy objects.
// It exists as a narrow boundary so Staff contracts do not import Harness types
// in the model layer, but application/adapter code can perform the translation.
type PolicyTranslator struct{}

func (PolicyTranslator) ToHarnessPolicy(ps PermissionSet) (harness.Policy, error) {
	if err := ps.Validate(); err != nil {
		return harness.Policy{}, fmt.Errorf("staff: invalid permission set: %w", err)
	}
	rules := make([]harness.Rule, 0, len(ps))
	for _, p := range ps {
		var effect harness.Effect
		switch p.Effect {
		case EffectAllow:
			effect = harness.EffectAllow
		case EffectAsk:
			effect = harness.EffectAsk
		case EffectDeny:
			effect = harness.EffectDeny
		default:
			return harness.Policy{}, fmt.Errorf("staff: unknown permission effect %q", p.Effect)
		}
		rules = append(rules, harness.Rule{
			ID:       p.ID,
			Action:   p.Action,
			Resource: p.Resource,
			Effect:   effect,
			Priority: p.Priority,
		})
	}
	return harness.NewPolicy(rules)
}

// FromHarnessDecision translates a Harness policy decision into a Staff
// permission decision. It preserves the deny-by-default semantics.
func (PolicyTranslator) FromHarnessDecision(d harness.Decision, action, resource string) (Decision, error) {
	decision := Decision{
		ContractVersion: ContractVersion,
		Action:          action,
		Resource:        resource,
		Effect:          EffectDeny,
		Reason:          "harness: default deny",
		RequiresAsk:     false,
	}
	switch d.Effect {
	case harness.EffectAllow:
		decision.Effect = EffectAllow
		decision.Reason = d.Reason
	case harness.EffectAsk:
		decision.Effect = EffectAsk
		decision.Reason = d.Reason
		decision.RequiresAsk = true
	case harness.EffectDeny:
		decision.Effect = EffectDeny
		decision.Reason = d.Reason
	default:
		decision.Effect = EffectDeny
		decision.Reason = "harness: unknown effect treated as deny"
	}
	if d.RuleID != "" {
		decision.PermissionID = d.RuleID
	}
	return decision, nil
}

// EnvelopeAdapter translates a Staff execution permission decision into the
// policy decision carried by a Harness execution envelope. The translation
// enforces the Staff contract's rule that only unconditional allow becomes
// execution authority.
type EnvelopeAdapter struct{}

func (EnvelopeAdapter) ToEnvelopePolicyDecision(decision Decision, action, resource string) (harness.PolicyDecision, error) {
	if err := ValidateExecutionPermission(decision, action, resource); err != nil {
		return harness.PolicyDecision{Allowed: false, Effect: harness.EffectDeny, Reason: err.Error()}, nil
	}
	return harness.PolicyDecision{
		Allowed:     true,
		RuleID:      decision.PermissionID,
		Effect:      harness.EffectAllow,
		Reason:      decision.Reason,
		RequiresAsk: false,
	}, nil
}
