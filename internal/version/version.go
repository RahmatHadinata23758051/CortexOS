package version

// Version is the semantic application version without the leading v.
// Release builds may override it with -ldflags; this default keeps local builds
// deterministic and aligned with VERSION.
var Version = "0.1.0"

// VersionFull is the display/tag form of Version.
var VersionFull = "v0.1.0"

// Commit is set via ldflags at build time.
var Commit = "unknown"

// BuiltAt is set via ldflags at build time (RFC3339).
var BuiltAt = "unknown"
