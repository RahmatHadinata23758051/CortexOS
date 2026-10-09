package platform

import (
	"context"
	"sync"
	"sync/atomic"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Standard Cockpit runtime event names emitted across the bridge.
const (
	EventCockpitRuntimeChanged    = "cortexos:cockpit:runtime:changed"
	EventCockpitWorkspaceChanged  = "cortexos:cockpit:workspace:changed"
	EventCockpitProjectRegistered = "cortexos:cockpit:project:registered"
	EventCockpitRetrievalRebuilt  = "cortexos:cockpit:retrieval:rebuilt"
)

// RuntimeEventHook is a callback function invoked when a bridge runtime event occurs.
type RuntimeEventHook func(eventName string, payload any)

// EventEmitter manages bridge runtime event hooks and Wails runtime event emissions.
type EventEmitter struct {
	mu     sync.RWMutex
	hooks  map[uint64]RuntimeEventHook
	nextID atomic.Uint64
	ctx    context.Context
}

// NewEventEmitter creates a thread-safe EventEmitter.
func NewEventEmitter() *EventEmitter {
	return &EventEmitter{
		hooks: make(map[uint64]RuntimeEventHook),
		ctx:   context.Background(),
	}
}

// SetContext updates the Wails runtime context.
func (e *EventEmitter) SetContext(ctx context.Context) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}
	e.ctx = ctx
}

// GetContext returns the current context.
func (e *EventEmitter) GetContext() context.Context {
	if e == nil {
		return context.Background()
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.ctx != nil {
		return e.ctx
	}
	return context.Background()
}

// RegisterHook adds a runtime event hook and returns an unsubscribe function.
func (e *EventEmitter) RegisterHook(hook RuntimeEventHook) func() {
	if e == nil || hook == nil {
		return func() {}
	}
	id := e.nextID.Add(1)
	e.mu.Lock()
	e.hooks[id] = hook
	e.mu.Unlock()

	return func() {
		e.mu.Lock()
		delete(e.hooks, id)
		e.mu.Unlock()
	}
}

// Emit broadcasts an event to registered hooks and the Wails runtime if bound.
func (e *EventEmitter) Emit(eventName string, payload any) {
	if e == nil {
		return
	}
	e.mu.RLock()
	hooks := make([]RuntimeEventHook, 0, len(e.hooks))
	for _, h := range e.hooks {
		hooks = append(hooks, h)
	}
	ctx := e.ctx
	e.mu.RUnlock()

	for _, h := range hooks {
		if h != nil {
			h(eventName, payload)
		}
	}

	if ctx != nil && ctx != context.Background() {
		// Emit through Wails runtime if initialized
		wailsruntime.EventsEmit(ctx, eventName, payload)
	}
}
