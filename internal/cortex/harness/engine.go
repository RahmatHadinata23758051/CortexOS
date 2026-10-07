package harness

// StaticPolicy is a simple in-memory PolicyEngine backed by the existing
// deterministic Policy contract. It is intentionally small and can be replaced
// by the persistence layer in a later issue.
type StaticPolicy struct {
	policy Policy
}

// NewStaticPolicy validates and creates a policy engine.
func NewStaticPolicy(policy Policy) (*StaticPolicy, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &StaticPolicy{policy: policy}, nil
}

func (p *StaticPolicy) Evaluate(request PolicyRequest) (PolicyDecision, error) {
	decision, err := p.policy.Evaluate(Request{Action: request.Action, Resource: request.Resource})
	if err != nil {
		return PolicyDecision{}, err
	}
	return PolicyDecision{
		Allowed:     decision.Effect == EffectAllow,
		RuleID:      decision.RuleID,
		Effect:      decision.Effect,
		Reason:      decision.Reason,
		RequiresAsk: decision.Effect == EffectAsk,
	}, nil
}

func (p *StaticPolicy) LoadPolicy(policy Policy) error {
	if err := policy.Validate(); err != nil {
		return err
	}
	p.policy = policy
	return nil
}

func (p *StaticPolicy) CurrentPolicy() Policy { return p.policy }
