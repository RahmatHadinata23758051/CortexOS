#!/usr/bin/env bash
# BAN-138 Packaged Wails smoke for Linux/macOS.
# Runs the production Wails binary with CORTEXOS_SMOKE_* env vars
# and requires a clean exit (code 0) within a timeout.

set -euo pipefail

QUIT_AFTER_MS=${QUIT_AFTER_MS:-2000}
TIMEOUT_SECONDS=${TIMEOUT_SECONDS:-120}
SKIP_BUILD=${SKIP_BUILD:-false}
OUTPUT="${OUTPUT:-build/bin/cortexos}"

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

if [ "$SKIP_BUILD" != "true" ]; then
    wails build -clean -nopackage -o cortexos
    if [ $? -ne 0 ]; then
        echo "wails build failed with exit code $?" >&2
        exit 1
    fi
fi

if [ ! -f "$OUTPUT" ]; then
    echo "packaged binary not found: $OUTPUT" >&2
    exit 1
fi

export CORTEXOS_SMOKE_QUIT_MS="$QUIT_AFTER_MS"

# Run the binary in the background so the smoke harness has a portable timeout
# on both GNU/Linux and macOS (neither platform-specific timeout flags are used).
if [ "$(uname)" = "Linux" ] && command -v xvfb-run >/dev/null 2>&1; then
    xvfb-run -a --server-args="-screen 0 1024x768x24" "$OUTPUT" &
else
    "$OUTPUT" &
fi

pid=$!
cleanup() {
    if kill -0 "$pid" 2>/dev/null; then
        kill "$pid" 2>/dev/null || true
        wait "$pid" 2>/dev/null || true
    fi
}
trap cleanup EXIT

deadline=$(( $(date +%s) + TIMEOUT_SECONDS ))
while kill -0 "$pid" 2>/dev/null; do
    if [ "$(date +%s)" -ge "$deadline" ]; then
        echo "packaged smoke timed out after ${TIMEOUT_SECONDS}s; process killed (PID $pid)" >&2
        exit 124
    fi
    sleep 1
done

set +e
wait "$pid"
EXIT_CODE=$?
set -e
if [ $EXIT_CODE -ne 0 ]; then
    echo "packaged smoke exited with code $EXIT_CODE" >&2
    exit $EXIT_CODE
fi

trap - EXIT
echo "BAN-138 packaged Wails smoke passed on $(uname -s)"