package harness

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

const PolicyContractVersion = "cortexos.harness.policy.v1"

type Effect string

const (
	EffectAllow Effect = "allow"
	EffectAsk   Effect = "ask"
	EffectDeny  Effect = "deny"
)

type Rule struct {
	ID       string `json:"id"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
	Effect   Effect `json:"effect"`
	Priority int    `json:"priority,omitempty"`
}

type Request struct {
	Action   string `json:"action"`
	Resource string `json:"resource"`
}

type Decision struct {
	ContractVersion string `json:"contractVersion"`
	Action          string `json:"action"`
	Resource        string `json:"resource"`
	Effect          Effect `json:"effect"`
	RuleID          string `json:"ruleId,omitempty"`
	Reason          string `json:"reason"`
}

type Policy struct {
	Rules []Rule `json:"rules"`
}

func NewPolicy(rules []Rule) (Policy, error) {
	p := Policy{Rules: append([]Rule(nil), rules...)}
	if err := p.Validate(); err != nil {
		return Policy{}, err
	}
	return p, nil
}

var (
	ErrInvalidPolicy  = errors.New("harness: invalid permission policy")
	ErrInvalidRequest = errors.New("harness: invalid permission request")
)

func (p Policy) Validate() error {
	seen := make(map[string]struct{}, len(p.Rules))
	for _, rule := range p.Rules {
		if rule.ID == "" || rule.Action == "" || rule.Resource == "" {
			return fmt.Errorf("%w: rule id, action, and resource are required", ErrInvalidPolicy)
		}
		if rule.Priority < 0 {
			return fmt.Errorf("%w: rule priority must not be negative", ErrInvalidPolicy)
		}
		if strings.ContainsAny(rule.Resource, "\x00\r\n") || strings.Contains(strings.ReplaceAll(rule.Resource, `\\`, "/"), "../") {
			return fmt.Errorf("%w: unsafe resource pattern", ErrInvalidPolicy)
		}
		if _, exists := seen[rule.ID]; exists {
			return fmt.Errorf("%w: duplicate rule id %q", ErrInvalidPolicy, rule.ID)
		}
		seen[rule.ID] = struct{}{}
		if !validEffect(rule.Effect) {
			return fmt.Errorf("%w: unsupported effect %q", ErrInvalidPolicy, rule.Effect)
		}
	}
	return nil
}

func (p Policy) Evaluate(request Request) (Decision, error) {
	action := strings.ToLower(strings.TrimSpace(request.Action))
	resource := normalizeResource(request.Resource)
	if action == "" || resource == "" {
		return Decision{}, fmt.Errorf("%w: action and resource are required", ErrInvalidRequest)
	}
	if err := p.Validate(); err != nil {
		return Decision{}, err
	}

	decision := Decision{
		ContractVersion: PolicyContractVersion,
		Action:          action,
		Resource:        resource,
		Effect:          EffectDeny,
		Reason:          "no matching rule; default deny",
	}
	rules := append([]Rule(nil), p.Rules...)
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		return rules[i].ID < rules[j].ID
	})
	for _, rule := range rules {
		if actionMatches(action, rule.Action) && wildcardMatch(normalizeResource(rule.Resource), resource) {
			decision.Effect = rule.Effect
			decision.RuleID = rule.ID
			decision.Reason = "matched highest-priority rule"
			break
		}
	}
	return decision, nil
}

func validEffect(effect Effect) bool {
	return effect == EffectAllow || effect == EffectAsk || effect == EffectDeny
}

func actionMatches(action, pattern string) bool {
	return wildcardMatch(action, strings.ToLower(strings.TrimSpace(pattern)))
}

func normalizeResource(resource string) string {
	return strings.ReplaceAll(strings.TrimSpace(resource), `\`, "/")
}

// wildcardMatch implements the whole-value * and ? matching used by the
// permission contract. It intentionally has no regex or shell evaluation.
func wildcardMatch(pattern, value string) bool {
	if strings.HasSuffix(pattern, " *") && value == strings.TrimSuffix(pattern, " *") {
		return true
	}
	p, v := []rune(pattern), []rune(value)
	pi, vi, star, match := 0, 0, -1, 0
	for vi < len(v) {
		if pi < len(p) && (p[pi] == '?' || p[pi] == v[vi]) {
			pi++
			vi++
			continue
		}
		if pi < len(p) && p[pi] == '*' {
			star = pi
			match = vi
			pi++
			continue
		}
		if star < 0 {
			return false
		}
		pi = star + 1
		match++
		vi = match
	}
	for pi < len(p) && p[pi] == '*' {
		pi++
	}
	return pi == len(p)
}
