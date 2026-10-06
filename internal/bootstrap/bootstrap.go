package bootstrap

import (
	"github.com/RahmatHadinata23758051/CortexOS/internal/cortex/application"
	"github.com/RahmatHadinata23758051/CortexOS/internal/platform"
)

// Run constructs the application graph and starts the desktop lifecycle.
func Run() error {
	service := application.NewService()
	app := platform.New()
	app.Bind(platform.NewBridge(service))
	return app.Run()
}
