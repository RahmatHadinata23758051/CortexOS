package harness

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// EngineClass identifies the execution tier and its operational profile.
type EngineClass string

const (
	EngineClassNative EngineClass = "native"
	EngineClassPi     EngineClass = "pi"
	EngineClassOMP    EngineClass = "omp"
)

// ResourceRequirement describes the declared resource envelope of an adapter.
type ResourceRequirement struct {
	MinMemoryMB  int `json:"minMemoryMB"`
	PeakMemoryMB int `json:"peakMemoryMB"`
	CPUPriority  int `json:"cpuPriority"`
}

func (r ResourceRequirement) Validate() error {
	if r.MinMemoryMB < 0 || r.PeakMemoryMB < 0 || r.CPUPriority < 0 {
		return fmt.Errorf("%w: resource requirements must not be negative", ErrInvalidContract)
	}
	if r.PeakMemoryMB > 0 && r.MinMemoryMB > r.PeakMemoryMB {
		return fmt.Errorf("%w: minimum memory exceeds peak memory", ErrInvalidContract)
	}
	return nil
}

func DefaultResourceRequirements(class EngineClass) ResourceRequirement {
	switch class {
	case EngineClassNative:
		return ResourceRequirement{MinMemoryMB: 50, PeakMemoryMB: 150, CPUPriority: 2}
	case EngineClassPi:
		return ResourceRequirement{MinMemoryMB: 200, PeakMemoryMB: 600, CPUPriority: 1}
	case EngineClassOMP:
		return ResourceRequirement{MinMemoryMB: 300, PeakMemoryMB: 1000, CPUPriority: 1}
	default:
		return ResourceRequirement{}
	}
}

// VersionConstraint is an inclusive compatibility range for adapter versions.
type VersionConstraint struct {
	Minimum string `json:"minimum,omitempty"`
	Maximum string `json:"maximum,omitempty"`
}

func (c VersionConstraint) IsCompatible(version string) bool {
	if c.Minimum != "" && compareVersions(version, c.Minimum) < 0 {
		return false
	}
	if c.Maximum != "" && compareVersions(version, c.Maximum) > 0 {
		return false
	}
	return true
}

// AdapterIdentity is stable identity for a registered adapter instance.
type AdapterIdentity struct {
	Name          string      `json:"name"`
	InstanceID    string      `json:"instanceId"`
	Kind          ToolKind    `json:"kind"`
	Class         EngineClass `json:"class"`
	Version       string      `json:"version"`
	SchemaVersion string      `json:"schemaVersion"`
	BuiltAt       string      `json:"builtAt,omitempty"`
}

// ExtendedAdapterDescriptor adds routing metadata without exposing engine types.
type ExtendedAdapterDescriptor struct {
	AdapterDescriptor
	Identity          AdapterIdentity     `json:"identity"`
	ResourceReq       ResourceRequirement `json:"resourceRequirement"`
	VersionConstraint VersionConstraint   `json:"versionConstraint,omitempty"`
}

func (d ExtendedAdapterDescriptor) Validate() error {
	if d.Name == "" || d.Kind == "" || d.Version == "" || d.SchemaVersion == "" {
		return fmt.Errorf("%w: adapter descriptor is incomplete", ErrInvalidContract)
	}
	if d.Identity.Name == "" || d.Identity.InstanceID == "" || d.Identity.Version == "" {
		return fmt.Errorf("%w: adapter identity name, instanceId, and version are required", ErrInvalidContract)
	}
	if d.Identity.Kind != d.Kind {
		return fmt.Errorf("%w: adapter identity kind does not match descriptor", ErrInvalidContract)
	}
	if d.Identity.Class == "" {
		return fmt.Errorf("%w: adapter engine class is required", ErrInvalidContract)
	}
	return d.ResourceReq.Validate()
}

type HealthStatus struct {
	Healthy          bool      `json:"healthy"`
	LastCheckAt      time.Time `json:"lastCheckAt"`
	CheckCount       int64     `json:"checkCount"`
	FailCount        int64     `json:"failCount"`
	ConsecutiveFails int64     `json:"consecutiveFails"`
	Reason           string    `json:"reason,omitempty"`
}

type RoutingPolicy struct {
	AllowedClasses           []EngineClass `json:"allowedClasses,omitempty"`
	DeniedClasses            []EngineClass `json:"deniedClasses,omitempty"`
	AllowedAdapters          []string      `json:"allowedAdapters,omitempty"`
	DeniedAdapters           []string      `json:"deniedAdapters,omitempty"`
	PreferredClasses         []EngineClass `json:"preferredClasses,omitempty"`
	MaxFallbackAttempts      int           `json:"maxFallbackAttempts"`
	RequireHealthEnforcement bool          `json:"requireHealthEnforcement"`
}

func DefaultRoutingPolicy() RoutingPolicy {
	return RoutingPolicy{
		AllowedClasses:      []EngineClass{EngineClassNative, EngineClassPi, EngineClassOMP},
		PreferredClasses:    []EngineClass{EngineClassNative, EngineClassPi, EngineClassOMP},
		MaxFallbackAttempts: 2, RequireHealthEnforcement: true,
	}
}

func (p RoutingPolicy) Validate() error {
	if p.MaxFallbackAttempts < 0 {
		return fmt.Errorf("%w: max fallback attempts must not be negative", ErrInvalidContract)
	}
	return nil
}

type RoutingConstraints struct {
	ExcludeClasses         []EngineClass
	MinMemoryMB            int
	PreferInstanceID       string
	RequiredSchemaVersion  string
	RequiredAdapterVersion VersionConstraint
}

type AdapterRegistration struct {
	Adapter    EngineAdapter
	Descriptor ExtendedAdapterDescriptor
	Health     HealthStatus
}

type AdapterSelection struct {
	Adapter        EngineAdapter
	Descriptor     ExtendedAdapterDescriptor
	Reason         string
	TieBreakKey    string
	RetryOrder     []string
	AttemptedCount int
}

type AdapterRouter interface {
	Register(adapter EngineAdapter, descriptor ExtendedAdapterDescriptor) error
	Unregister(name, instanceID string) error
	Find(ctx context.Context, requiredCaps []ToolCapability) (*AdapterSelection, error)
	FindWithConstraints(ctx context.Context, requiredCaps []ToolCapability, constraints RoutingConstraints) (*AdapterSelection, error)
	Health(ctx context.Context) map[string]HealthStatus
	CheckHealth(ctx context.Context, name string) HealthStatus
	List() []AdapterRegistration
}

type BasicRouter struct {
	mu            sync.RWMutex
	adapters      map[string]AdapterRegistration
	policy        RoutingPolicy
	healthChecker func(context.Context, EngineAdapter) error
	clock         func() time.Time
}

func NewBasicRouter(policy RoutingPolicy) *BasicRouter {
	if policy.MaxFallbackAttempts == 0 && len(policy.AllowedClasses) == 0 && len(policy.PreferredClasses) == 0 {
		policy = DefaultRoutingPolicy()
	}
	return &BasicRouter{adapters: make(map[string]AdapterRegistration), policy: policy, clock: time.Now}
}

func (r *BasicRouter) SetHealthChecker(fn func(context.Context, EngineAdapter) error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.healthChecker = fn
}

func adapterKey(d ExtendedAdapterDescriptor) string {
	return d.Identity.Name + "|" + d.Identity.InstanceID
}

func generateInstanceID(name string) string { return fmt.Sprintf("%s-%d", name, time.Now().UnixNano()) }

func engineClassFromToolKind(kind ToolKind) EngineClass {
	switch kind {
	case ToolKindNative:
		return EngineClassNative
	case ToolKindPi:
		return EngineClassPi
	case ToolKindOMP:
		return EngineClassOMP
	default:
		return EngineClassNative
	}
}

func (r *BasicRouter) Register(adapter EngineAdapter, descriptor ExtendedAdapterDescriptor) error {
	if adapter == nil {
		return fmt.Errorf("%w: adapter is nil", ErrInvalidContract)
	}
	if descriptor.Identity.Name != "" {
		descriptor.Name = descriptor.Identity.Name
	} else if descriptor.Name != "" {
		descriptor.Identity.Name = descriptor.Name
	}
	if descriptor.Identity.Version != "" {
		descriptor.Version = descriptor.Identity.Version
	} else if descriptor.Version != "" {
		descriptor.Identity.Version = descriptor.Version
	}
	if descriptor.Identity.Kind != "" {
		descriptor.Kind = descriptor.Identity.Kind
	} else if descriptor.Kind != "" {
		descriptor.Identity.Kind = descriptor.Kind
	}
	if descriptor.Identity.SchemaVersion != "" {
		descriptor.SchemaVersion = descriptor.Identity.SchemaVersion
	} else if descriptor.SchemaVersion != "" {
		descriptor.Identity.SchemaVersion = descriptor.SchemaVersion
	}
	if descriptor.Identity.Class == "" {
		descriptor.Identity.Class = engineClassFromToolKind(adapter.Kind())
	}
	if descriptor.Identity.InstanceID == "" {
		descriptor.Identity.InstanceID = generateInstanceID(descriptor.Identity.Name)
	}
	if len(descriptor.Capabilities) == 0 {
		descriptor.Capabilities = adapter.Capabilities()
	}
	if err := descriptor.Validate(); err != nil {
		return err
	}
	if adapter.Kind() != descriptor.Kind {
		return fmt.Errorf("%w: adapter kind does not match descriptor", ErrInvalidContract)
	}
	if !coversCapabilities(adapter.Capabilities(), descriptor.Capabilities) {
		return fmt.Errorf("%w: adapter capabilities do not match descriptor", ErrInvalidContract)
	}
	key := adapterKey(descriptor)
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[key]; exists {
		return fmt.Errorf("%w: adapter %q already registered", ErrDuplicateTool, key)
	}
	r.adapters[key] = AdapterRegistration{Adapter: adapter, Descriptor: descriptor, Health: HealthStatus{Healthy: true}}
	return nil
}

func (r *BasicRouter) Unregister(name, instanceID string) error {
	key := name + "|" + instanceID
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.adapters[key]; !ok {
		return fmt.Errorf("%w: adapter %q", ErrToolNotFound, key)
	}
	delete(r.adapters, key)
	return nil
}

func (r *BasicRouter) List() []AdapterRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AdapterRegistration, 0, len(r.adapters))
	for _, reg := range r.adapters {
		out = append(out, reg)
	}
	sort.Slice(out, func(i, j int) bool { return adapterKey(out[i].Descriptor) < adapterKey(out[j].Descriptor) })
	return out
}

func (r *BasicRouter) Find(ctx context.Context, requiredCaps []ToolCapability) (*AdapterSelection, error) {
	return r.FindWithConstraints(ctx, requiredCaps, RoutingConstraints{})
}

func (r *BasicRouter) FindWithConstraints(ctx context.Context, requiredCaps []ToolCapability, constraints RoutingConstraints) (*AdapterSelection, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	candidates := r.candidates(requiredCaps, constraints)
	if len(candidates) == 0 {
		return nil, r.classificationError(requiredCaps, constraints)
	}
	ordered := r.order(candidates, constraints.PreferInstanceID)
	if !r.policy.RequireHealthEnforcement {
		return r.selection(ordered, 0, "eligible adapter; health enforcement disabled"), nil
	}
	maxChecks := r.policy.MaxFallbackAttempts + 1
	if maxChecks > len(ordered) {
		maxChecks = len(ordered)
	}
	for i := 0; i < maxChecks; i++ {
		if r.CheckHealthKey(ctx, adapterKey(ordered[i].Descriptor)).Healthy {
			return r.selection(ordered, i, "healthy eligible adapter"), nil
		}
	}
	return nil, fmt.Errorf("%w: bounded fallback exhausted", ErrAdapterUnhealthy)
}

func (r *BasicRouter) classificationError(required []ToolCapability, constraints RoutingConstraints) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	capabilityMatch, policyMatch, versionMatch := false, false, false
	for _, reg := range r.adapters {
		d := reg.Descriptor
		if !coversCapabilities(d.Capabilities, required) {
			continue
		}
		capabilityMatch = true
		if r.classAllowed(d.Identity.Class) && !containsClass(constraints.ExcludeClasses, d.Identity.Class) && (len(r.policy.AllowedAdapters) == 0 || containsString(r.policy.AllowedAdapters, d.Identity.Name)) && !containsString(r.policy.DeniedAdapters, d.Identity.Name) {
			policyMatch = true
		}
		if policyMatch && constraints.RequiredAdapterVersion.IsCompatible(d.Version) {
			versionMatch = true
		}
	}
	if capabilityMatch && !policyMatch {
		return ErrPolicyDeniedEngine
	}
	if capabilityMatch && !versionMatch {
		return ErrVersionIncompatible
	}
	return ErrNoEligibleAdapter
}

func (r *BasicRouter) candidates(required []ToolCapability, constraints RoutingConstraints) []AdapterRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []AdapterRegistration
	for _, reg := range r.adapters {
		d := reg.Descriptor
		if !coversCapabilities(d.Capabilities, required) || !r.classAllowed(d.Identity.Class) || containsClass(constraints.ExcludeClasses, d.Identity.Class) {
			continue
		}
		if constraints.MinMemoryMB > 0 && d.ResourceReq.PeakMemoryMB < constraints.MinMemoryMB {
			continue
		}
		if constraints.RequiredSchemaVersion != "" && d.SchemaVersion != constraints.RequiredSchemaVersion {
			continue
		}
		if !constraints.RequiredAdapterVersion.IsCompatible(d.Version) {
			continue
		}
		if len(r.policy.AllowedAdapters) > 0 && !containsString(r.policy.AllowedAdapters, d.Identity.Name) {
			continue
		}
		if containsString(r.policy.DeniedAdapters, d.Identity.Name) {
			continue
		}
		out = append(out, reg)
	}
	return out
}

func (r *BasicRouter) classAllowed(class EngineClass) bool {
	return !containsClass(r.policy.DeniedClasses, class) && (len(r.policy.AllowedClasses) == 0 || containsClass(r.policy.AllowedClasses, class))
}

func (r *BasicRouter) order(candidates []AdapterRegistration, preferredInstance string) []AdapterRegistration {
	rank := func(c EngineClass) int {
		for i, p := range r.policy.PreferredClasses {
			if c == p {
				return i
			}
		}
		return len(r.policy.PreferredClasses)
	}
	out := append([]AdapterRegistration(nil), candidates...)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if preferredInstance != "" && (a.Descriptor.Identity.InstanceID == preferredInstance) != (b.Descriptor.Identity.InstanceID == preferredInstance) {
			return a.Descriptor.Identity.InstanceID == preferredInstance
		}
		if ar, br := rank(a.Descriptor.Identity.Class), rank(b.Descriptor.Identity.Class); ar != br {
			return ar < br
		}
		if a.Health.FailCount != b.Health.FailCount {
			return a.Health.FailCount < b.Health.FailCount
		}
		return adapterKey(a.Descriptor) < adapterKey(b.Descriptor)
	})
	return out
}

func (r *BasicRouter) selection(ordered []AdapterRegistration, selected int, reason string) *AdapterSelection {
	alternatives := ordered[selected+1:]
	if max := r.policy.MaxFallbackAttempts; max < len(alternatives) {
		alternatives = alternatives[:max]
	}
	retry := make([]string, len(alternatives))
	for i := range alternatives {
		retry[i] = adapterKey(alternatives[i].Descriptor)
	}
	return &AdapterSelection{Adapter: ordered[selected].Adapter, Descriptor: ordered[selected].Descriptor, Reason: reason, TieBreakKey: adapterKey(ordered[selected].Descriptor), RetryOrder: retry, AttemptedCount: selected + 1}
}

func (r *BasicRouter) CheckHealth(ctx context.Context, name string) HealthStatus {
	r.mu.RLock()
	keys := make([]string, 0)
	for key, reg := range r.adapters {
		if reg.Descriptor.Identity.Name == name {
			keys = append(keys, key)
		}
	}
	r.mu.RUnlock()
	sort.Strings(keys)
	if len(keys) == 0 {
		return HealthStatus{Reason: "not found", LastCheckAt: r.clock()}
	}
	return r.CheckHealthKey(ctx, keys[0])
}

func (r *BasicRouter) CheckHealthKey(ctx context.Context, key string) HealthStatus {
	r.mu.RLock()
	reg, ok := r.adapters[key]
	checker := r.healthChecker
	r.mu.RUnlock()
	if !ok {
		return HealthStatus{Reason: "not found", LastCheckAt: r.clock()}
	}
	var err error
	if checker != nil {
		err = checker(ctx, reg.Adapter)
	} else {
		err = reg.Adapter.Health(NewExecutionContext(ctx, ""))
	}
	now := r.clock()
	r.mu.Lock()
	defer r.mu.Unlock()
	current, exists := r.adapters[key]
	if !exists {
		return HealthStatus{Reason: "not found", LastCheckAt: now}
	}
	current.Health.LastCheckAt = now
	current.Health.CheckCount++
	if err != nil {
		current.Health.Healthy = false
		current.Health.FailCount++
		current.Health.ConsecutiveFails++
		current.Health.Reason = "health check failed"
	} else {
		current.Health.Healthy = true
		current.Health.ConsecutiveFails = 0
		current.Health.Reason = ""
	}
	r.adapters[key] = current
	return current.Health
}

func (r *BasicRouter) Health(ctx context.Context) map[string]HealthStatus {
	result := make(map[string]HealthStatus)
	for _, reg := range r.List() {
		key := adapterKey(reg.Descriptor)
		result[key] = r.CheckHealthKey(ctx, key)
	}
	return result
}

func coversCapabilities(provided, required []ToolCapability) bool {
	set := make(map[ToolCapability]struct{}, len(provided))
	for _, c := range provided {
		set[c] = struct{}{}
	}
	for _, c := range required {
		if _, ok := set[c]; !ok {
			return false
		}
	}
	return true
}
func containsClass(values []EngineClass, value EngineClass) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}
func containsString(values []string, value string) bool {
	for _, v := range values {
		if v == value {
			return true
		}
	}
	return false
}

func compareVersions(left, right string) int {
	parse := func(version string) []int {
		parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(version), "v"), ".")
		result := make([]int, len(parts))
		for i, p := range parts {
			result[i], _ = strconv.Atoi(p)
		}
		return result
	}
	l, r := parse(left), parse(right)
	for i := 0; i < len(l) || i < len(r); i++ {
		lv, rv := 0, 0
		if i < len(l) {
			lv = l[i]
		}
		if i < len(r) {
			rv = r[i]
		}
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	return 0
}
