package main

import (
	"log"

	"github.com/RahmatHadinata23758051/CortexOS/internal/platform"
)

func main() {
	app := platform.New()
	if err := app.Run(); err != nil {
		log.Fatalf("cortexos application stopped with error: %v", err)
	}
}
