package staff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// PermissionEffect mirrors Harness's allow/ask/deny decision vocabulary
// without coupling the Staff contract package to the Harness implementation.
type PermissionEffect string

const (
	EffectAllow PermissionEffect = "allow"
	EffectAsk   PermissionEffect = "ask"
	EffectDeny  PermissionEffect = "deny"
)

// Permission is a declarative request boundary. It grants no authority by
// itself; Harness remains responsible for enforcing the evaluated decision.
type Permission struct {
	ID       string           `json:"id"`
	Action   string           `json:"action"`
	Resource string           `json:"resource"`
	Effect   PermissionEffect `json:"effect"`
	Priority int              `json:"priority,omitempty"`
}

// PermissionSet is named to make validation explicit rather than treating a
// raw slice as a complete policy.
type PermissionSet []Permission

func (p PermissionSet) Validate() error {
	seen := make(map[string]struct{}, len(p))
	for _, permission := range p {
		if permission.ID == "" || permission.Action == "" || strings.TrimSpace(permission.Resource) == "" {
			return newError(ErrInvalidPermission, "permission id, action, and resource are required")
		}
		if err := validateToken(permission.ID, "permission id"); err != nil {
			return WrapError(ErrInvalidPermission, err.Error(), err)
		}
		if err := validateToken(permission.Action, "permission action"); err != nil {
			return WrapError(ErrInvalidPermission, err.Error(), err)
		}
		if permission.Priority < 0 {
			return newError(ErrInvalidPermission, "permission priority must not be negative")
		}
		if err := validateResourcePattern(permission.Resource); err != nil {
			return WrapError(ErrInvalidPermission, "invalid permission resource", err)
		}
		if !validPermissionEffect(permission.Effect) {
			return newError(ErrInvalidPermission, fmt.Sprintf("unsupported permission effect %q", permission.Effect))
		}
		if _, exists := seen[permission.ID]; exists {
			return newError(ErrInvalidPermission, fmt.Sprintf("duplicate permission id %q", permission.ID))
		}
		seen[permission.ID] = struct{}{}
	}
	return nil
}

// Decision is the Staff-side result that can be translated to Harness's
// PolicyDecision or an Orchestra execution envelope. Deny is the zero-safe
// outcome for invalid or absent authority.
type Decision struct {
	ContractVersion string           `json:"contractVersion"`
	Action          string           `json:"action"`
	Resource        string           `json:"resource"`
	Effect          PermissionEffect `json:"effect"`
	PermissionID    string           `json:"permissionId,omitempty"`
	Reason          string           `json:"reason"`
	RequiresAsk     bool             `json:"requiresAsk"`
}

func (p PermissionSet) Evaluate(action, resource string) (Decision, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	resource = normalizeResource(resource)
	decision := Decision{
		ContractVersion: ContractVersion,
		Action:          action,
		Resource:        resource,
		Effect:          EffectDeny,
		Reason:          "invalid request or permission set; default deny",
	}
	if action == "" || resource == "" {
		return decision, newError(ErrInvalidRequest, "action and resource are required")
	}
	if err := p.Validate(); err != nil {
		decision.Reason = "invalid permission set; default deny"
		return decision, err
	}
	decision.Reason = "no matching permission; default deny"
	permissions := append(PermissionSet(nil), p...)
	sort.SliceStable(permissions, func(i, j int) bool {
		if permissions[i].Priority != permissions[j].Priority {
			return permissions[i].Priority > permissions[j].Priority
		}
		return permissions[i].ID < permissions[j].ID
	})
	for _, permission := range permissions {
		if wildcardMatch(strings.ToLower(strings.TrimSpace(permission.Action)), action) && wildcardMatch(normalizeResource(permission.Resource), resource) {
			decision.Effect = permission.Effect
			decision.PermissionID = permission.ID
			decision.RequiresAsk = permission.Effect == EffectAsk
			decision.Reason = "matched highest-priority permission"
			return decision, nil
		}
	}
	return decision, nil
}

// ValidateExecutionPermission requires a positive, complete decision before a
// Staff request can be placed in an execution envelope. Invalid, missing, ask,
// or deny decisions never become execution authority.
func ValidateExecutionPermission(decision Decision, action, resource string) error {
	if decision.ContractVersion != ContractVersion {
		return newError(ErrUnsupportedVersion, fmt.Sprintf("unsupported decision version %q", decision.ContractVersion))
	}
	if decision.Action != strings.ToLower(strings.TrimSpace(action)) || decision.Resource != normalizeResource(resource) {
		return newError(ErrInvalidPermission, "permission decision does not match request")
	}
	if decision.Effect != EffectAllow || decision.RequiresAsk || decision.PermissionID == "" {
		return newError(ErrPermissionDenied, "permission decision is not an unconditional allow")
	}
	return nil
}

func (p PermissionSet) MarshalJSON() ([]byte, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	type alias PermissionSet
	return json.Marshal(alias(p))
}

// UnmarshalJSON validates permission sets at the JSON boundary instead of
// allowing malformed policy data to remain latent until a later evaluation.
func (p *PermissionSet) UnmarshalJSON(data []byte) error {
	type alias PermissionSet
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	candidate := PermissionSet(decoded)
	if err := candidate.Validate(); err != nil {
		return err
	}
	*p = candidate
	return nil
}

func normalizeResource(resource string) string {
	return strings.ReplaceAll(strings.TrimSpace(resource), `\`, "/")
}

func validPermissionEffect(effect PermissionEffect) bool {
	return effect == EffectAllow || effect == EffectAsk || effect == EffectDeny
}

// validateResourcePattern validates that a resource pattern is safe and well-formed.
// It rejects control characters, path traversal attempts, and other unsafe patterns.
func validateResourcePattern(resource string) error {
	trimmed := strings.TrimSpace(resource)
	if trimmed == "" {
		return newError(ErrInvalidPermission, "resource is required")
	}
	if strings.ContainsAny(trimmed, "\x00\r\n") {
		return newError(ErrInvalidPermission, "resource contains control characters")
	}
	normalized := strings.ReplaceAll(trimmed, `\`, "/")
	for _, segment := range strings.Split(normalized, "/") {
		if segment == ".." {
			return newError(ErrInvalidPermission, "path traversal in resource pattern")
		}
	}
	// The wildcard matcher accepts * and ? anywhere in a safe resource pattern.
	return nil
}

func wildcardMatch(pattern, value string) bool {
	// Match Harness behavior: pattern ending with " *" matches value exactly
	// when the value equals the pattern without the trailing " *".
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
