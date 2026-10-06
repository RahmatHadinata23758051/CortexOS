//go:build bindings

package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/RahmatHadinata23758051/CortexOS/internal/bootstrap"
	"github.com/RahmatHadinata23758051/CortexOS/internal/platform"
)

// Wails generates the frontend bundle at frontend/dist before compiling the
// root package. Keeping the embed directive here leaves platform code free of
// Wails-specific generated paths.
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	var assetFS fs.FS = assets
	platform.SetAssetFS(assetFS)
	if err := bootstrap.Run(); err != nil {
		log.Fatalf("cortexos application stopped with error: %v", err)
	}
}
