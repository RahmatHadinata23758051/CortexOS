package platform

import (
	"context"

	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
)

// CockpitBridge is the thin Wails-facing adapter for Cockpit application operations.
// It owns no business logic; validation, mapping, and persistence remain in application services.
type CockpitBridge struct {
	service *application.Service
	ctx     context.Context
	emitter *EventEmitter
}

// NewCockpitBridge creates a Wails-compatible Cockpit adapter.
func NewCockpitBridge(service *application.Service) *CockpitBridge {
	return &CockpitBridge{
		service: service,
		ctx:     context.Background(),
		emitter: NewEventEmitter(),
	}
}

// WithContext returns a shallow copy bound to a Wails lifecycle context.
func (b *CockpitBridge) WithContext(ctx context.Context) *CockpitBridge {
	if ctx == nil {
		ctx = context.Background()
	}
	return &CockpitBridge{service: b.service, ctx: ctx, emitter: b.emitter}
}

func (b *CockpitBridge) getContext() context.Context {
	if b != nil && b.ctx != nil {
		return b.ctx
	}
	return context.Background()
}

// SetRuntimeContext installs the Wails runtime context for event emission.
func (b *CockpitBridge) SetRuntimeContext(ctx context.Context) {
	if b != nil && b.emitter != nil {
		b.emitter.SetContext(ctx)
	}
}

// GetCockpitRuntime exposes runtime status through a typed DTO.
func (b *CockpitBridge) GetCockpitRuntime() (application.CockpitRuntimeSnapshot, error) {
	runtime, err := b.service.GetCockpitRuntime(b.getContext())
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitRuntimeChanged, runtime)
	}
	return runtime, err
}

// GetCockpitWorkspace exposes workspace facts through a typed DTO.
func (b *CockpitBridge) GetCockpitWorkspace(request application.CockpitWorkspaceRequest) (application.CockpitWorkspaceSnapshot, error) {
	ws, err := b.service.GetCockpitWorkspace(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitWorkspaceChanged, ws)
	}
	return ws, err
}

// RegisterCockpitProject exposes the controlled project-registration operation.
func (b *CockpitBridge) RegisterCockpitProject(request application.CockpitProjectRequest) (application.CockpitProject, error) {
	project, err := b.service.RegisterCockpitProject(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitProjectRegistered, project)
	}
	return project, err
}

// QueryCockpitWorkspace exposes bounded workspace retrieval.
func (b *CockpitBridge) QueryCockpitWorkspace(request application.CockpitQueryRequest) ([]application.CockpitQueryResult, error) {
	return b.service.QueryCockpitWorkspace(b.getContext(), request)
}

// RebuildCockpitRetrieval exposes a controlled retrieval mutation.
func (b *CockpitBridge) RebuildCockpitRetrieval(request application.CockpitMutationRequest) error {
	err := b.service.RebuildCockpitRetrieval(b.getContext(), request)
	if err == nil && b.emitter != nil {
		b.emitter.Emit(EventCockpitRetrievalRebuilt, map[string]string{"projectId": request.ProjectID})
	}
	return err
}

// RegisterEventHook installs a runtime event hook.
func (b *CockpitBridge) RegisterEventHook(hook RuntimeEventHook) func() {
	if b == nil || b.emitter == nil {
		return func() {}
	}
	return b.emitter.RegisterHook(hook)
}

// EmitEvent allows manual emission of bridge runtime events.
func (b *CockpitBridge) EmitEvent(eventName string, payload any) {
	if b != nil && b.emitter != nil {
		b.emitter.Emit(eventName, payload)
	}
}
