//go:build !bindings

package main

import (
	"log"

	"github.com/RahmatHadinata23758051/CortexOS/internal/bootstrap"
)

// The Wails CLI builds the repository root package. The cmd/cortexos entrypoint
// remains the explicit Go command entrypoint for non-Wails tooling.
func main() {
	if err := bootstrap.Run(); err != nil {
		log.Fatalf("cortexos application stopped with error: %v", err)
	}
}
