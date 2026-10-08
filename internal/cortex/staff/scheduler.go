package staff

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// AssignmentPriority controls scheduling order. Higher values are dispatched first.
type AssignmentPriority int

const (
	PriorityLow      AssignmentPriority = 1
	PriorityNormal   AssignmentPriority = 2
	PriorityHigh     AssignmentPriority = 3
	PriorityCritical AssignmentPriority = 4
)

// AssignmentStatus is the lifecycle of a logical task assignment.
type AssignmentStatus string

const (
	AssignmentQueued   AssignmentStatus = "queued"
	AssignmentAssigned AssignmentStatus = "assigned"
	AssignmentCanceled AssignmentStatus = "canceled"
	AssignmentRejected AssignmentStatus = "rejected"
	AssignmentReleased AssignmentStatus = "released"
)

// RejectionReason is deliberately explicit and stable so callers can explain
// why a task was not assigned without exposing worker/process details.
type RejectionReason struct {
	Code    string  `json:"code"`
	Message string  `json:"message"`
	StaffID StaffID `json:"staffId,omitempty"`
}

const (
	RejectInvalidRequest = "invalid_request"
	RejectNoMatch        = "no_match"
	RejectWorkspace      = "workspace_scope"
	RejectProject        = "project_scope"
	RejectCapability     = "capability"
	RejectPermission     = "permission"
	RejectInactive       = "inactive"
	RejectUnavailable    = "unavailable"
	RejectResource       = "resource_constraint"
	RejectBusy           = "busy"
	RejectCanceled       = "canceled"
)

// AssignmentRequest contains only logical task requirements. It never carries
// a process ID, worker handle, or tool authorization. Harness remains the sole
// authority for tool permissions.
type AssignmentRequest struct {
	TaskID               string
	WorkspaceID          WorkspaceID
	ProjectID            ProjectID
	WorktreeID           WorktreeID
	StaffID              StaffID
	RoleHint             Role
	RequiredCapabilities []Capability
	Action               string
	Resource             string
	Priority             AssignmentPriority
	ResourceAvailable    bool
}

func (r AssignmentRequest) Validate() error {
	if r.TaskID == "" || r.WorkspaceID == "" {
		return newError(ErrInvalidRequest, "taskId and workspaceId are required")
	}
	if err := validateToken(r.TaskID, "task id"); err != nil {
		return err
	}
	if err := validateToken(string(r.WorkspaceID), "workspace id"); err != nil {
		return err
	}
	if r.ProjectID != "" {
		if err := validateToken(string(r.ProjectID), "project id"); err != nil {
			return err
		}
	}
	if r.StaffID != "" {
		if err := validateToken(string(r.StaffID), "staff id"); err != nil {
			return err
		}
	}
	if len(r.RequiredCapabilities) == 0 {
		return newError(ErrInvalidRequest, "at least one required capability is required")
	}
	if r.Priority < PriorityLow || r.Priority > PriorityCritical {
		return newError(ErrInvalidRequest, "priority must be between low and critical")
	}
	return nil
}

// Assignment is an immutable snapshot of a scheduler decision.
type Assignment struct {
	AssignmentID string
	Request      AssignmentRequest
	StaffID      StaffID
	Status       AssignmentStatus
	Reason       string
	Rejections   []RejectionReason
	EnqueuedAt   time.Time
	AssignedAt   time.Time
	FinishedAt   time.Time
	Sequence     uint64
}

// AssignmentEvent is emitted after each state transition. Consumers may audit
// it, but cannot use it to grant tools or mark an Orchestra task successful.
type AssignmentEvent struct {
	Type       string
	Assignment Assignment
	Previous   AssignmentStatus
	OccurredAt time.Time
}

const (
	EventAssignmentQueued     = "assignment.queued"
	EventAssignmentAssigned   = "assignment.assigned"
	EventAssignmentRejected   = "assignment.rejected"
	EventAssignmentCanceled   = "assignment.canceled"
	EventAssignmentReleased   = "assignment.released"
	EventAssignmentReassigned = "assignment.reassigned"
)

type AssignmentEventSink func(AssignmentEvent)

// Scheduler provides race-safe queueing and logical Staff reservations. A busy
// Staff member is never double-assigned; queued work is ordered by priority,
// then FIFO sequence, then task ID for deterministic replay.
type Scheduler struct {
	mu        sync.Mutex
	store     Store
	router    Router
	clock     func() time.Time
	nextSeq   uint64
	assign    map[string]Assignment
	byStaff   map[StaffID]string
	counts    map[StaffID]uint64
	eventSink AssignmentEventSink
}

func NewScheduler(store Store, router Router) (*Scheduler, error) {
	if store == nil || router == nil {
		return nil, newError(ErrInvalidRequest, "staff store and router are required")
	}
	return &Scheduler{store: store, router: router, clock: time.Now, assign: make(map[string]Assignment), byStaff: make(map[StaffID]string), counts: make(map[StaffID]uint64)}, nil
}

func (s *Scheduler) SetEventSink(sink AssignmentEventSink) {
	s.mu.Lock()
	s.eventSink = sink
	s.mu.Unlock()
}

func (s *Scheduler) emit(eventType string, a Assignment, previous AssignmentStatus) {
	if s.eventSink != nil {
		s.eventSink(AssignmentEvent{Type: eventType, Assignment: cloneAssignment(a), Previous: previous, OccurredAt: s.clock().UTC()})
	}
}

func cloneAssignment(a Assignment) Assignment {
	a.Rejections = append([]RejectionReason(nil), a.Rejections...)
	return a
}

// Submit records a request. It is assigned immediately when an available Staff
// member exists, or queued when all otherwise eligible members are busy.
func (s *Scheduler) Submit(ctx context.Context, req AssignmentRequest) (Assignment, error) {
	if ctx == nil {
		return Assignment{}, newError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return Assignment{}, CanceledError(err)
	}
	if err := req.Validate(); err != nil {
		return Assignment{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.assign[req.TaskID]; ok && existing.Status != AssignmentCanceled && existing.Status != AssignmentReleased && existing.Status != AssignmentRejected {
		return Assignment{}, WrapError(ErrConflict, "task already has an assignment", nil)
	}
	s.nextSeq++
	now := s.clock().UTC()
	a := Assignment{AssignmentID: fmt.Sprintf("assignment-%s-%d", req.TaskID, s.nextSeq), Request: req, Status: AssignmentQueued, EnqueuedAt: now, Sequence: s.nextSeq}
	candidates, reasons, err := s.match(ctx, req)
	if err != nil {
		return Assignment{}, err
	}
	a.Rejections = reasons
	if len(candidates) == 0 {
		if hasBusyReason(reasons) {
			s.assign[req.TaskID] = a
			s.emit(EventAssignmentQueued, a, "")
			return cloneAssignment(a), nil
		}
		a.Status, a.Reason = AssignmentRejected, "no eligible Staff matched task requirements"
		s.assign[req.TaskID] = a
		s.emit(EventAssignmentRejected, a, "")
		if hasPermissionReason(reasons) {
			return cloneAssignment(a), WrapError(ErrPermissionDenied, "staff permissions denied requested action", nil)
		}
		return cloneAssignment(a), WrapError(ErrNotFound, a.Reason, nil)
	}
	chosen := s.choose(candidates)
	s.reserve(&a, chosen.ID, now)
	s.assign[req.TaskID] = a
	s.emit(EventAssignmentAssigned, a, "")
	return cloneAssignment(a), nil
}

func hasPermissionReason(reasons []RejectionReason) bool {
	for _, r := range reasons {
		if r.Code == RejectPermission {
			return true
		}
	}
	return false
}

func hasBusyReason(reasons []RejectionReason) bool {
	for _, r := range reasons {
		if r.Code == RejectBusy {
			return true
		}
	}
	return false
}

func (s *Scheduler) match(ctx context.Context, req AssignmentRequest) ([]Definition, []RejectionReason, error) {
	defs, err := s.store.ListDefinitions(ctx)
	if err != nil {
		return nil, nil, err
	}
	reasons := make([]RejectionReason, 0)
	eligible := make([]Definition, 0)
	for _, d := range defs {
		if req.StaffID != "" && d.ID != req.StaffID {
			continue
		}
		if d.Lifecycle != LifecycleActive {
			reasons = append(reasons, RejectionReason{Code: RejectInactive, Message: "Staff is not active", StaffID: d.ID})
			continue
		}
		if d.Workspace.WorkspaceID != req.WorkspaceID {
			reasons = append(reasons, RejectionReason{Code: RejectWorkspace, Message: "Staff is outside workspace scope", StaffID: d.ID})
			continue
		}
		if req.ProjectID != "" && d.Workspace.ProjectID != "" && d.Workspace.ProjectID != req.ProjectID {
			reasons = append(reasons, RejectionReason{Code: RejectProject, Message: "Staff is outside project scope", StaffID: d.ID})
			continue
		}
		if req.RoleHint != "" && d.Role != req.RoleHint {
			continue
		}
		if !hasAllCapabilities(d.Capabilities, req.RequiredCapabilities) {
			reasons = append(reasons, RejectionReason{Code: RejectCapability, Message: "Staff lacks required capabilities", StaffID: d.ID})
			continue
		}
		if req.Action != "" && req.Resource != "" {
			decision, e := d.Permissions.Evaluate(req.Action, req.Resource)
			if e != nil || decision.Effect == EffectDeny {
				reasons = append(reasons, RejectionReason{Code: RejectPermission, Message: "Staff permissions do not allow requested action", StaffID: d.ID})
				continue
			}
		}
		if req.ResourceAvailable == false {
			reasons = append(reasons, RejectionReason{Code: RejectResource, Message: "required scheduling resources are unavailable", StaffID: d.ID})
			continue
		}
		if _, busy := s.byStaff[d.ID]; busy {
			reasons = append(reasons, RejectionReason{Code: RejectBusy, Message: "Staff has an active assignment", StaffID: d.ID})
			continue
		}
		if d.Availability != AvailabilityAvailable {
			code := RejectUnavailable
			if d.Availability == AvailabilityBusy {
				code = RejectBusy
			}
			reasons = append(reasons, RejectionReason{Code: code, Message: fmt.Sprintf("Staff availability is %s", d.Availability), StaffID: d.ID})
			continue
		}
		eligible = append(eligible, d)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	return eligible, reasons, nil
}

func (s *Scheduler) choose(candidates []Definition) Definition {
	// Least-assigned first gives fair distribution while ID provides a stable tie-break.
	return candidates[sortCandidates(candidates, s.counts)]
}
func sortCandidates(c []Definition, counts map[StaffID]uint64) int {
	best := 0
	for i := 1; i < len(c); i++ {
		if counts[c[i].ID] < counts[c[best].ID] || (counts[c[i].ID] == counts[c[best].ID] && c[i].ID < c[best].ID) {
			best = i
		}
	}
	return best
}
func (s *Scheduler) reserve(a *Assignment, id StaffID, now time.Time) {
	a.StaffID, a.Status, a.AssignedAt, a.Reason = id, AssignmentAssigned, now, "matched capabilities, permissions, scope, availability, and fairness"
	s.byStaff[id] = a.Request.TaskID
	s.counts[id]++
}

// Cancel atomically cancels queued or assigned work and releases its Staff slot.
func (s *Scheduler) Cancel(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assign[taskID]
	if !ok {
		return newError(ErrNotFound, "assignment not found")
	}
	if a.Status == AssignmentCanceled || a.Status == AssignmentReleased {
		return WrapError(ErrConflict, "assignment is already terminal", nil)
	}
	prev := a.Status
	if a.StaffID != "" {
		delete(s.byStaff, a.StaffID)
	}
	a.Status, a.Reason, a.FinishedAt = AssignmentCanceled, "canceled by caller", s.clock().UTC()
	s.assign[taskID] = a
	s.emit(EventAssignmentCanceled, a, prev)
	return nil
}

// Release completes an assignment without asserting task success. Orchestra or
// Inspector remains responsible for task outcome and authority.
func (s *Scheduler) Release(ctx context.Context, taskID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assign[taskID]
	if !ok {
		return newError(ErrNotFound, "assignment not found")
	}
	if a.Status != AssignmentAssigned {
		return WrapError(ErrConflict, "only assigned work can be released", nil)
	}
	if a.StaffID != "" {
		delete(s.byStaff, a.StaffID)
	}
	prev := a.Status
	a.Status, a.FinishedAt = AssignmentReleased, s.clock().UTC()
	s.assign[taskID] = a
	s.emit(EventAssignmentReleased, a, prev)
	return nil
}

func (s *Scheduler) Get(taskID string) (Assignment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.assign[taskID]
	if !ok {
		return Assignment{}, newError(ErrNotFound, "assignment not found")
	}
	return cloneAssignment(a), nil
}

func (s *Scheduler) Pending() []Assignment {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Assignment, 0)
	for _, a := range s.assign {
		if a.Status == AssignmentQueued {
			out = append(out, cloneAssignment(a))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Request.Priority != out[j].Request.Priority {
			return out[i].Request.Priority > out[j].Request.Priority
		}
		if out[i].Sequence != out[j].Sequence {
			return out[i].Sequence < out[j].Sequence
		}
		return out[i].Request.TaskID < out[j].Request.TaskID
	})
	return out
}

// SchedulePending retries queued work in priority/FIFO order. It is safe to
// call after an availability or resource update and never changes Orchestra
// task state or authority.
func (s *Scheduler) SchedulePending(ctx context.Context) ([]Assignment, error) {
	if ctx == nil {
		return nil, newError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return nil, CanceledError(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := make([]Assignment, 0)
	for _, a := range s.assign {
		if a.Status == AssignmentQueued {
			pending = append(pending, a)
		}
	}
	sort.Slice(pending, func(i, j int) bool {
		if pending[i].Request.Priority != pending[j].Request.Priority {
			return pending[i].Request.Priority > pending[j].Request.Priority
		}
		if pending[i].Sequence != pending[j].Sequence {
			return pending[i].Sequence < pending[j].Sequence
		}
		return pending[i].Request.TaskID < pending[j].Request.TaskID
	})
	assigned := make([]Assignment, 0)
	for _, queued := range pending {
		candidates, reasons, err := s.match(ctx, queued.Request)
		if err != nil {
			return assigned, err
		}
		queued.Rejections = reasons
		if len(candidates) == 0 {
			s.assign[queued.Request.TaskID] = queued
			continue
		}
		previous := queued.Status
		s.reserve(&queued, s.choose(candidates).ID, s.clock().UTC())
		s.assign[queued.Request.TaskID] = queued
		s.emit(EventAssignmentAssigned, queued, previous)
		assigned = append(assigned, cloneAssignment(queued))
	}
	return assigned, nil
}

// Reassign cancels any active logical reservation and submits a fresh request.
func (s *Scheduler) Reassign(ctx context.Context, taskID string, req AssignmentRequest) (Assignment, error) {
	if ctx == nil {
		return Assignment{}, newError(ErrInvalidRequest, "context is required")
	}
	if err := ctx.Err(); err != nil {
		return Assignment{}, CanceledError(err)
	}
	s.mu.Lock()
	a, ok := s.assign[taskID]
	if ok && a.Status != AssignmentCanceled && a.Status != AssignmentReleased && a.Status != AssignmentRejected {
		prev := a.Status
		if a.StaffID != "" {
			delete(s.byStaff, a.StaffID)
		}
		a.Status, a.Reason, a.FinishedAt = AssignmentCanceled, "canceled for reassignment", s.clock().UTC()
		s.assign[taskID] = a
		s.emit(EventAssignmentCanceled, a, prev)
	}
	s.mu.Unlock()
	newA, err := s.Submit(ctx, req)
	if err == nil {
		s.mu.Lock()
		s.emit(EventAssignmentReassigned, newA, AssignmentCanceled)
		s.mu.Unlock()
	}
	return newA, err
}
