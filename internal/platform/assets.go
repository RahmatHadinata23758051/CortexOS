//go:build !bindings

package platform

import (
	"embed"
	"io/fs"
)

// AssetFS provides a small local fallback for direct Go execution and tests.
// Wails builds provide the generated frontend bundle from the root entrypoint.
//
//go:embed assets/*
var embeddedAssets embed.FS

func init() {
	AssetFS = embeddedAssets
}

var _ fs.FS = AssetFS
