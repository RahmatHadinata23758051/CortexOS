#!/usr/bin/env bash
# Generate versioned artifact, SHA-256 checksum, and provenance metadata.
# Intended for CI after a Wails packaged binary has been built.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version_file="$repo_root/VERSION"
artifact_dir="${ARTIFACT_DIR:-$repo_root/build/bin}"
artifact_name="${ARTIFACT_NAME:-cortexos-artifact}"
platform="${ARTIFACT_PLATFORM:-unknown}"
version="$(tr -d '[:space:]' < "$version_file")"
commit="${GITHUB_SHA:-$(git -C "$repo_root" rev-parse HEAD 2>/dev/null || echo unknown)}"
builder="${BUILDER:-github-actions}"
built_at="${BUILT_AT:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"

auto_source="${SOURCE_BINARY:-}"
if [ -z "$auto_source" ]; then
    if [ -f "$artifact_dir/cortexos.exe" ]; then
        auto_source="$artifact_dir/cortexos.exe"
    elif [ -f "$artifact_dir/cortexos" ]; then
        auto_source="$artifact_dir/cortexos"
    else
        echo "ERROR: Wails binary not found in $artifact_dir" >&2
        exit 1
    fi
fi

extension=""
case "$auto_source" in
    *.exe) extension=".exe" ;;
esac
artifact="$artifact_dir/${artifact_name}-${version}${extension}"
cp "$auto_source" "$artifact"

if command -v sha256sum >/dev/null 2>&1; then
    checksum="$(sha256sum "$artifact" | awk '{print $1}')"
else
    checksum="$(shasum -a 256 "$artifact" | awk '{print $1}')"
fi
printf '%s  %s\n' "$checksum" "$(basename "$artifact")" > "$artifact.sha256"

# node is installed by the desktop CI job and avoids platform-specific JSON tooling.
ARTIFACT_PATH="$artifact" \
ARTIFACT_NAME="$(basename "$artifact")" \
ARTIFACT_SHA256="$checksum" \
ARTIFACT_VERSION="$version" \
ARTIFACT_COMMIT="$commit" \
ARTIFACT_PLATFORM="$platform" \
ARTIFACT_BUILDER="$builder" \
ARTIFACT_BUILT_AT="$built_at" \
ARTIFACT_GO_VERSION="$(go version 2>/dev/null || echo unknown)" \
node <<'NODE'
const fs = require('fs');
const path = require('path');
const output = `${process.env.ARTIFACT_PATH}.provenance.json`;
const provenance = {
  version: process.env.ARTIFACT_VERSION,
  artifact: process.env.ARTIFACT_NAME,
  sha256: process.env.ARTIFACT_SHA256,
  builtAt: process.env.ARTIFACT_BUILT_AT,
  commit: process.env.ARTIFACT_COMMIT,
  goVersion: process.env.ARTIFACT_GO_VERSION,
  builder: process.env.ARTIFACT_BUILDER,
  platform: process.env.ARTIFACT_PLATFORM,
};
fs.writeFileSync(output, JSON.stringify(provenance, null, 2) + '\n');
console.log(`Wrote ${path.basename(output)}`);
NODE

echo "Prepared $(basename "$artifact")"
echo "  checksum: $(basename "$artifact.sha256")"
echo "  provenance: $(basename "$artifact.provenance.json")"