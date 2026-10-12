# BAN-132 Packaged Wails desktop smoke (P7-07)

## Status

CortexOS now has a packaged smoke harness that builds and runs the production
Wails binary on all three supported desktop targets.

## Local Windows smoke

`scripts/smoke-wails-packaged.ps1`

```powershell
# Full build + run (default quit after 2 s)
./scripts/smoke-wails-packaged.ps1

# Re-use an existing binary, custom quit delay
./scripts/smoke-wails-packaged.ps1 -SkipBuild -QuitAfterMs 3000
```

Behaviour:
1. `wails build -clean -nopackage` produces `build/bin/cortexos-smoke.exe`.
2. Starts the binary with `CORTEXOS_SMOKE_QUIT_MS=<QuitAfterMs>`.
3. Waits up to `TimeoutSeconds` (default 120 s) for clean exit (code 0).
4. On timeout or non-zero exit, kills the process and fails the script.
5. Prints OS description on success.

The variable `CORTEXOS_SMOKE_QUIT_MS` is **not set in normal production
runs**. It only activates the auto-quit when the script sets it.

## CI coverage

`.github/workflows/ci.yml` job `desktop`:

| OS | Runner | What runs |
|---|---|---|
| Windows | `windows-latest` | Build + full packaged smoke (binary starts, quits itself via env var) |
| macOS | `macos-latest` | Build only (headless; no GUI display available in CI) |
| Linux | `ubuntu-latest` | Build only (GTK deps installed; no X server available in CI) |

Build only means: `wails build -clean -nopackage` succeeds → the Go
compiler, Wails bindings, and embedded frontend assets are all correct for
that platform.

## Verification

```powershell
# Local Windows (recommended before push)
.\scripts\smoke-wails-packaged.ps1
```

```bash
# CI build-only check (Linux/macOS) — wails CLI must be installed
wails build -clean -nopackage
```

## Residual risk

- The Windows CI smoke runs a real WebView2 GUI on the GitHub-hosted
  Windows runner. It has worked on developer machines; CI runners may have
  different WebView2 versions or session isolation that cause flakes. If
  flakes appear, investigate and adjust the timeout or disable the
  Windows CI smoke step while keeping the build step.
- macOS and Linux CI do not execute the binary because headless WebKit/GTK
  WebView is not available in standard GitHub runners without Xvfb. Local
  testing on those OSes is the intended path for full runtime validation.
- The smoke only proves process start → auto-quit → exit 0. It does not
  yet assert that the Wails bridge is callable (that remains the manual
  BAN-129 browser-console snippet). A future improvement could embed a
  headless bridge probe behind the same env var.