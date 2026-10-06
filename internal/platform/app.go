package platform

import (
	"context"
	"embed"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/application"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed assets/*
var assets embed.FS

// Application owns the Wails lifecycle boundary. Domain services are passed in
// as bindings and remain independent from the desktop framework.
type Application struct {
	wails *application.Application

	shutdownOnce sync.Once
}

// New creates a desktop application with deterministic lifecycle callbacks.
func New() *Application {
	wails := application.NewWithOptions(&options.App{
		Title:            "CortexOS",
		Width:            1280,
		Height:           800,
		MinWidth:         960,
		MinHeight:        640,
		Assets:           assets,
		BackgroundColour: options.NewRGB(16, 21, 29),
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
		OnStartup:  func(context.Context) {},
		OnShutdown: func(context.Context) {},
	})

	return &Application{wails: wails}
}

// Bind exposes an application service through the Wails binding boundary.
func (a *Application) Bind(service any) {
	if a == nil || a.wails == nil {
		return
	}
	a.wails.Bind(service)
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
