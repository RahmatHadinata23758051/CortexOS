package platform

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/application"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// AssetFS is the frontend asset filesystem used by the Wails shell.
// The Wails CLI supplies the generated frontend bundle at build time.
var AssetFS fs.FS

// SetAssetFS installs the generated frontend bundle before the app starts.
func SetAssetFS(assets fs.FS) {
	AssetFS = assets
}

// Application owns the Wails lifecycle boundary. Domain services are passed in
// as bindings and remain independent from the desktop framework.
type Application struct {
	wails         *application.Application
	startupHooks  []func(context.Context)
	shutdownHooks []func(context.Context)
	shutdownOnce  sync.Once
	mu            sync.Mutex
}

// New creates a desktop application with deterministic lifecycle callbacks.
func New() *Application {
	app := &Application{
		startupHooks:  make([]func(context.Context), 0),
		shutdownHooks: make([]func(context.Context), 0),
	}
	wails := application.NewWithOptions(&options.App{
		Title:            "CortexOS",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        640,
		Assets:           AssetFS,
		BackgroundColour: options.NewRGB(16, 21, 29),
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
		OnStartup:  app.handleStartup,
		OnShutdown: app.handleShutdown,
	})
	app.wails = wails
	return app
}

// OnStartup registers a startup hook.
func (a *Application) OnStartup(hook func(context.Context)) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.startupHooks = append(a.startupHooks, hook)
}

// OnShutdown registers a shutdown hook.
func (a *Application) OnShutdown(hook func(context.Context)) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdownHooks = append(a.shutdownHooks, hook)
}

func (a *Application) handleStartup(ctx context.Context) {
	if a == nil {
		return
	}
	a.mu.Lock()
	hooks := make([]func(context.Context), len(a.startupHooks))
	copy(hooks, a.startupHooks)
	a.mu.Unlock()
	for _, hook := range hooks {
		if hook != nil {
			hook(ctx)
		}
	}
	// P7-07 packaged smoke harness: when CORTEXOS_SMOKE_QUIT_MS is set, schedule
	// a clean shutdown after the desktop shell and startup hooks finish.
	// Normal production runs leave the variable unset.
	if delay, ok := smokeQuitDelay(os.Getenv("CORTEXOS_SMOKE_QUIT_MS")); ok {
		go func() {
			time.Sleep(delay)
			a.Shutdown()
		}()
	}
}

// smokeQuitDelay parses the smoke-only auto-quit delay. It reports ok=false for
// unset, empty, or invalid values so normal runs never auto-exit.
func smokeQuitDelay(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, false
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 0 {
		return 0, false
	}
	return time.Duration(ms) * time.Millisecond, true
}

func (a *Application) handleShutdown(ctx context.Context) {
	if a == nil {
		return
	}
	a.mu.Lock()
	hooks := make([]func(context.Context), len(a.shutdownHooks))
	copy(hooks, a.shutdownHooks)
	a.mu.Unlock()
	for _, hook := range hooks {
		if hook != nil {
			hook(ctx)
		}
	}
}

// Bind exposes an application service through the Wails binding boundary.
func (a *Application) Bind(service any) {
	if a == nil || a.wails == nil {
		return
	}
	a.wails.Bind(service)
}

// BindCockpit exposes the CockpitBridge through Wails bindings.
func (a *Application) BindCockpit(bridge *CockpitBridge) {
	if a == nil || a.wails == nil {
		return
	}
	a.wails.Bind(bridge)
}

// Run starts the desktop application and blocks until Wails exits.
func (a *Application) Run() error {
	if a == nil || a.wails == nil {
		return fmt.Errorf("platform application is not initialized")
	}
	return a.wails.Run()
}

// Shutdown requests one clean shutdown. Repeated calls are safe.
func (a *Application) Shutdown() {
	if a == nil || a.wails == nil {
		return
	}
	a.shutdownOnce.Do(func() {
		a.wails.Quit()
	})
}
