package harness

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ErrCapacityExceeded indicates that a task could not be admitted before its
// context expired, or that its declared requirements can never fit the budget.
var ErrCapacityExceeded = errors.New("harness: governor capacity exceeded")

// Priority controls task ordering in a governed worker pool. Higher values run
// first; tasks with equal priority are admitted in FIFO order.
type Priority int

const (
	PriorityLow      Priority = 10
	PriorityNormal   Priority = 50
	PriorityHigh     Priority = 100
	PriorityCritical Priority = 200
)

// ResourceBudget is the independent capacity limit for one EngineClass. A zero
// limit means unlimited for that dimension. MaxWorkers is the worker-pool cap.
type ResourceBudget struct {
	MaxWorkers     int
	MaxMemoryMB    int
	MaxCPUPriority int
}

// Validate validates a resource budget. Negative values are invalid; zero is
// the documented unlimited value.
func (b ResourceBudget) Validate() error {
	if b.MaxWorkers < 0 || b.MaxMemoryMB < 0 || b.MaxCPUPriority < 0 {
		return fmt.Errorf("%w: resource budget values must not be negative", ErrInvalidContract)
	}
	return nil
}

// DefaultResourceBudgets returns conservative laptop-safe defaults. Native
// tools are lightweight and are limited by their own callers; Pi and OMP are
// governed because they may own long-lived engine processes.
func DefaultResourceBudgets() map[EngineClass]ResourceBudget {
	return map[EngineClass]ResourceBudget{
		EngineClassNative: {MaxWorkers: 0, MaxMemoryMB: 0, MaxCPUPriority: 0},
		EngineClassPi:     {MaxWorkers: 4, MaxMemoryMB: 2400, MaxCPUPriority: 4},
		EngineClassOMP:    {MaxWorkers: 1, MaxMemoryMB: 1000, MaxCPUPriority: 1},
	}
}

// GovernorConfig is the constructor configuration for a Governor.
type GovernorConfig struct {
	Budgets map[EngineClass]ResourceBudget
	Clock   func() time.Time
}

// TaskDescriptor is the admission request for a governed task.
type TaskDescriptor struct {
	TaskID               string
	EngineClass          EngineClass
	Priority             Priority
	EstimatedMemoryMB    int
	EstimatedCPUPriority int
	// Requirements is an optional router-compatible spelling of the resource
	// estimate. Explicit Estimated* fields take precedence when non-zero.
	Requirements ResourceRequirement
	TraceID      string
}

// Reservation is the task's lease on worker-pool capacity. Every successful
// Admit call must release its reservation exactly once, regardless of whether
// execution completes, fails, times out, or is canceled.
type Reservation struct {
	TaskID      string
	EngineClass EngineClass
	Priority    Priority
	MemoryMB    int
	CPUPriority int
	TraceID     string

	governor *Governor
	released atomic.Bool
}

// Release returns the reservation's worker, memory, and CPU capacity. Release
// is idempotent so defer reservation.Release() is safe on every exit path.
func (r *Reservation) Release() {
	if r == nil || !r.released.CompareAndSwap(false, true) {
		return
	}
	r.governor.release(r)
}

// IsReleased reports whether Release has completed (or is in progress).
func (r *Reservation) IsReleased() bool {
	return r == nil || r.released.Load()
}

type Governor struct {
	mu        sync.Mutex
	budgets   map[EngineClass]ResourceBudget
	classes   map[EngineClass]*governorClass
	clock     func() time.Time
	telemetry telemetryState
}

type governorClass struct {
	budget ResourceBudget

	activeWorkers int
	activeMemory  int
	activeCPU     int
	queue         []*waitingTask

	admitted     int64
	rejected     int64
	released     int64
	timeouts     int64
	cancels      int64
	peakWorkers  int
	peakMemory   int
	peakCPU      int
	queueSamples int64
	queueTotal   int64
	waitSamples  int64
	waitTotal    time.Duration
}

type waitingTask struct {
	task        TaskDescriptor
	sequence    uint64
	enqueued    time.Time
	ready       chan struct{}
	admitted    bool
	cancelled   bool
	reservation *Reservation
}

type telemetryState struct {
	nextSequence  uint64
	totalAdmitted int64
	totalRejected int64
	totalReleased int64
	totalTimeouts int64
	totalCancels  int64
}

// GovernorOption configures a Governor created by NewGovernor.
type GovernorOption func(*Governor)

// WithBudgets sets resource budgets. The map is copied by NewGovernor.
func WithBudgets(budgets map[EngineClass]ResourceBudget) GovernorOption {
	return func(g *Governor) {
		// Preserve safe defaults for omitted classes while allowing callers to
		// override only the pool they are tuning.
		merged := copyBudgets(DefaultResourceBudgets())
		for class, budget := range budgets {
			merged[class] = budget
		}
		g.budgets = merged
	}
}

// WithClock injects a clock for deterministic tests.
func WithClock(clock func() time.Time) GovernorOption {
	return func(g *Governor) {
		if clock != nil {
			g.clock = clock
		}
	}
}

// NewGovernor creates a governor with safe defaults.
func NewGovernor(options ...GovernorOption) *Governor {
	g := &Governor{
		budgets: copyBudgets(DefaultResourceBudgets()),
		classes: make(map[EngineClass]*governorClass),
		clock:   time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(g)
		}
	}
	for class, budget := range g.budgets {
		g.classes[class] = &governorClass{budget: budget}
	}
	return g
}

// NewResourceGovernor is a config-oriented constructor for callers that do
// not need functional options.
func NewResourceGovernor(config GovernorConfig) (*Governor, error) {
	budgets := config.Budgets
	if budgets == nil {
		budgets = DefaultResourceBudgets()
	}
	for class, budget := range budgets {
		if err := budget.Validate(); err != nil {
			return nil, fmt.Errorf("%w for %s", err, class)
		}
	}
	options := []GovernorOption{WithBudgets(budgets)}
	if config.Clock != nil {
		options = append(options, WithClock(config.Clock))
	}
	return NewGovernor(options...), nil
}

// Admit reserves capacity, waiting in a deterministic priority/FIFO queue
// when the selected engine class is full. A context deadline returns an error
// matching ErrCapacityExceeded; cancellation returns context.Canceled.
func (g *Governor) Admit(ctx context.Context, task TaskDescriptor) (*Reservation, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	task, err := normalizeTask(task)
	if err != nil {
		return nil, err
	}

	g.mu.Lock()
	cs := g.classLocked(task.EngineClass)
	if exceedsBudget(cs.budget, task) {
		g.recordRejectedLocked(cs)
		g.mu.Unlock()
		return nil, ErrCapacityExceeded
	}
	if canAdmit(cs, task) {
		reservation := g.reserveLocked(cs, task)
		g.mu.Unlock()
		return reservation, nil
	}

	g.telemetry.nextSequence++
	waiter := &waitingTask{
		task: task, sequence: g.telemetry.nextSequence, enqueued: g.clock(), ready: make(chan struct{}),
	}
	cs.queue = append(cs.queue, waiter)
	sort.SliceStable(cs.queue, func(i, j int) bool {
		if cs.queue[i].task.Priority != cs.queue[j].task.Priority {
			return cs.queue[i].task.Priority > cs.queue[j].task.Priority
		}
		return cs.queue[i].sequence < cs.queue[j].sequence
	})
	cs.queueSamples++
	cs.queueTotal += int64(len(cs.queue))
	g.mu.Unlock()

	select {
	case <-waiter.ready:
		g.mu.Lock()
		reservation := waiter.reservation
		g.mu.Unlock()
		return reservation, nil
	case <-ctx.Done():
		g.mu.Lock()
		if waiter.admitted {
			// Admission won the race with cancellation. Return the lease so the
			// caller can release it; capacity is never leaked.
			reservation := waiter.reservation
			g.mu.Unlock()
			return reservation, nil
		}
		waiter.cancelled = true
		removeWaiter(cs, waiter)
		g.recordRejectedLocked(cs)
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			cs.timeouts++
			g.telemetry.totalTimeouts++
			g.mu.Unlock()
			return nil, errors.Join(ErrCapacityExceeded, ctx.Err())
		}
		cs.cancels++
		g.telemetry.totalCancels++
		g.mu.Unlock()
		return nil, ctx.Err()
	}
}

// AdmitTask is an explicit alias useful at call sites that distinguish task
// admission from other governor operations.
func (g *Governor) AdmitTask(ctx context.Context, task TaskDescriptor) (*Reservation, error) {
	return g.Admit(ctx, task)
}

func normalizeTask(task TaskDescriptor) (TaskDescriptor, error) {
	if task.TaskID == "" {
		return TaskDescriptor{}, fmt.Errorf("%w: task ID is required", ErrInvalidContract)
	}
	if task.EngineClass == "" {
		task.EngineClass = EngineClassNative
	}
	if task.Priority == 0 {
		task.Priority = PriorityNormal
	}
	if task.EstimatedMemoryMB == 0 {
		task.EstimatedMemoryMB = task.Requirements.PeakMemoryMB
	}
	if task.EstimatedCPUPriority == 0 {
		task.EstimatedCPUPriority = task.Requirements.CPUPriority
	}
	if task.EstimatedMemoryMB < 0 || task.EstimatedCPUPriority < 0 {
		return TaskDescriptor{}, fmt.Errorf("%w: task resource estimates must not be negative", ErrInvalidContract)
	}
	return task, nil
}

func (g *Governor) classLocked(class EngineClass) *governorClass {
	if cs, ok := g.classes[class]; ok {
		return cs
	}
	// Unknown classes are isolated and conservative: they receive no implicit
	// finite cap, but never share accounting with another class.
	budget := g.budgets[class]
	cs := &governorClass{budget: budget}
	g.classes[class] = cs
	return cs
}

func canAdmit(cs *governorClass, task TaskDescriptor) bool {
	b := cs.budget
	return (b.MaxWorkers == 0 || cs.activeWorkers < b.MaxWorkers) &&
		(b.MaxMemoryMB == 0 || cs.activeMemory+task.EstimatedMemoryMB <= b.MaxMemoryMB) &&
		(b.MaxCPUPriority == 0 || cs.activeCPU+task.EstimatedCPUPriority <= b.MaxCPUPriority)
}

func exceedsBudget(b ResourceBudget, task TaskDescriptor) bool {
	return (b.MaxWorkers > 0 && b.MaxWorkers < 1) ||
		(b.MaxMemoryMB > 0 && task.EstimatedMemoryMB > b.MaxMemoryMB) ||
		(b.MaxCPUPriority > 0 && task.EstimatedCPUPriority > b.MaxCPUPriority)
}

func (g *Governor) reserveLocked(cs *governorClass, task TaskDescriptor) *Reservation {
	cs.activeWorkers++
	cs.activeMemory += task.EstimatedMemoryMB
	cs.activeCPU += task.EstimatedCPUPriority
	if cs.activeWorkers > cs.peakWorkers {
		cs.peakWorkers = cs.activeWorkers
	}
	if cs.activeMemory > cs.peakMemory {
		cs.peakMemory = cs.activeMemory
	}
	if cs.activeCPU > cs.peakCPU {
		cs.peakCPU = cs.activeCPU
	}
	cs.admitted++
	g.telemetry.totalAdmitted++
	return &Reservation{
		TaskID: task.TaskID, EngineClass: task.EngineClass, Priority: task.Priority,
		MemoryMB: task.EstimatedMemoryMB, CPUPriority: task.EstimatedCPUPriority,
		TraceID: task.TraceID, governor: g,
	}
}

func (g *Governor) release(reservation *Reservation) {
	g.mu.Lock()
	cs := g.classLocked(reservation.EngineClass)
	if cs.activeWorkers > 0 {
		cs.activeWorkers--
	}
	cs.activeMemory -= reservation.MemoryMB
	if cs.activeMemory < 0 {
		cs.activeMemory = 0
	}
	cs.activeCPU -= reservation.CPUPriority
	if cs.activeCPU < 0 {
		cs.activeCPU = 0
	}
	cs.released++
	g.telemetry.totalReleased++
	g.dispatchLocked(cs)
	g.mu.Unlock()
}

// dispatchLocked admits every currently feasible waiter, choosing priority and
// sequence order. Scanning beyond an oversized head avoids head-of-line
// blocking while preserving deterministic order among feasible tasks.
func (g *Governor) dispatchLocked(cs *governorClass) {
	for {
		index := -1
		for i, waiter := range cs.queue {
			if !waiter.cancelled && canAdmit(cs, waiter.task) {
				index = i
				break
			}
		}
		if index < 0 {
			return
		}
		waiter := cs.queue[index]
		cs.queue = append(cs.queue[:index], cs.queue[index+1:]...)
		waiter.admitted = true
		waiter.reservation = g.reserveLocked(cs, waiter.task)
		wait := g.clock().Sub(waiter.enqueued)
		if wait < 0 {
			wait = 0
		}
		cs.waitSamples++
		cs.waitTotal += wait
		close(waiter.ready)
	}
}

func removeWaiter(cs *governorClass, target *waitingTask) {
	for i, waiter := range cs.queue {
		if waiter == target {
			cs.queue = append(cs.queue[:i], cs.queue[i+1:]...)
			return
		}
	}
}

func (g *Governor) recordAdmissionLocked(cs *governorClass) {
	cs.admitted++
	g.telemetry.totalAdmitted++
}

func (g *Governor) recordRejectedLocked(cs *governorClass) {
	cs.rejected++
	g.telemetry.totalRejected++
}

// UpdateUsage records observed peak usage for an active reservation. Observed
// usage never mutates the reservation's admission accounting; it informs
// telemetry and future tuning without making release arithmetic unsafe.
func (g *Governor) UpdateUsage(reservation *Reservation, memoryMB, cpuPriority int) error {
	if reservation == nil || reservation.IsReleased() {
		return nil
	}
	if memoryMB < 0 || cpuPriority < 0 {
		return fmt.Errorf("%w: observed resource usage must not be negative", ErrInvalidContract)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	cs := g.classLocked(reservation.EngineClass)
	if memoryMB > cs.peakMemory {
		cs.peakMemory = memoryMB
	}
	if cpuPriority > cs.peakCPU {
		cs.peakCPU = cpuPriority
	}
	return nil
}

// TelemetrySnapshot is a race-free point-in-time view of governor accounting.
type TelemetrySnapshot struct {
	Timestamp      time.Time
	TotalAdmitted  int64
	TotalRejected  int64
	TotalReleased  int64
	TotalTimeouts  int64
	TotalCancels   int64
	ClassTelemetry map[EngineClass]ClassTelemetry
}

type ClassTelemetry struct {
	EngineClass        EngineClass
	AdmittedCount      int64
	RejectedCount      int64
	ReleasedCount      int64
	TimeoutCount       int64
	CancelCount        int64
	PeakActiveWorkers  int64
	PeakMemoryMB       int64
	PeakCPUPriority    int64
	CurrentActive      int64
	CurrentMemoryMB    int64
	CurrentCPUPriority int64
	QueueDepth         int
	AvgQueueDepth      float64
	AvgWaitTimeMs      float64
	LastUpdated        time.Time
}

// GetTelemetry returns current observed usage and accounting.
func (g *Governor) GetTelemetry() TelemetrySnapshot {
	g.mu.Lock()
	defer g.mu.Unlock()
	snapshot := TelemetrySnapshot{
		Timestamp: g.clock(), TotalAdmitted: g.telemetry.totalAdmitted,
		TotalRejected: g.telemetry.totalRejected, TotalReleased: g.telemetry.totalReleased,
		TotalTimeouts: g.telemetry.totalTimeouts, TotalCancels: g.telemetry.totalCancels,
		ClassTelemetry: make(map[EngineClass]ClassTelemetry, len(g.classes)),
	}
	for class, cs := range g.classes {
		avgDepth := float64(0)
		if cs.queueSamples > 0 {
			avgDepth = float64(cs.queueTotal) / float64(cs.queueSamples)
		}
		avgWait := float64(0)
		if cs.waitSamples > 0 {
			avgWait = float64(cs.waitTotal.Milliseconds()) / float64(cs.waitSamples)
		}
		snapshot.ClassTelemetry[class] = ClassTelemetry{
			EngineClass: class, AdmittedCount: cs.admitted, RejectedCount: cs.rejected,
			ReleasedCount: cs.released, TimeoutCount: cs.timeouts, CancelCount: cs.cancels,
			PeakActiveWorkers: int64(cs.peakWorkers), PeakMemoryMB: int64(cs.peakMemory),
			PeakCPUPriority: int64(cs.peakCPU), CurrentActive: int64(cs.activeWorkers),
			CurrentMemoryMB: int64(cs.activeMemory), CurrentCPUPriority: int64(cs.activeCPU),
			QueueDepth: len(cs.queue), AvgQueueDepth: avgDepth, AvgWaitTimeMs: avgWait,
			LastUpdated: snapshot.Timestamp,
		}
	}
	return snapshot
}

// GetClassBudget returns the current budget for an engine class.
func (g *Governor) GetClassBudget(class EngineClass) ResourceBudget {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.classLocked(class).budget
}

// SetClassBudget changes a class budget. Existing reservations are not revoked;
// newly queued work observes the new budget.
func (g *Governor) SetClassBudget(class EngineClass, budget ResourceBudget) error {
	if err := budget.Validate(); err != nil {
		return err
	}
	g.mu.Lock()
	cs := g.classLocked(class)
	cs.budget = budget
	g.budgets[class] = budget
	g.dispatchLocked(cs)
	g.mu.Unlock()
	return nil
}

func copyBudgets(input map[EngineClass]ResourceBudget) map[EngineClass]ResourceBudget {
	output := make(map[EngineClass]ResourceBudget, len(input))
	for class, budget := range input {
		output[class] = budget
	}
	return output
}
