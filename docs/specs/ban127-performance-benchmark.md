# BAN-127 Office performance benchmark

This benchmark is the repeatable Phase 7 gate for the Virtual Office presentation
layer. It measures the production Vite build in Chromium with a deterministic
read-only Staff bridge fixture:

- Phaser `OfficeScene` animation-frame intervals over a three-second steady-state
  window after the lazy-loaded office canvas is ready;
- `ListStaffSummaries` bridge round trips after ten warm-up calls and 100 measured
  calls, using the browser monotonic clock;
- p50, p95, p99, mean, minimum, maximum, sample count, and machine/browser metadata
  in a JSON attachment.

## Baseline and workload

The named development baseline is the Windows 11 x64 workstation used for BAN-127
validation: AMD Ryzen 5 PRO 5650U with Radeon Graphics, Chromium installed by
Playwright, 1280x720 viewport, and hardware-backed Chromium rendering through
`--use-angle=gl`. CI must run this job on a Windows runner before treating the
numeric evidence as release evidence; Linux or software-rendered runs are useful
for diagnostics but are not comparable to this baseline.

The fixed workload contains eight active Staff summaries distributed across the
five supported roles. Phaser is measured after the canvas is visible and a short
warm-up has elapsed, so the result describes steady-state rendering rather than
lazy chunk download time.

## Budgets and pass criteria

- Effective frame rate is at least 60 frames per second. The benchmark also
  records frame p50/p95/p99; a p95 interval over one 60 Hz frame plus one
  millisecond fails the gate. The one-millisecond allowance accounts for
  `requestAnimationFrame` timestamp quantization while retaining the 60fps
  throughput gate.
- `ListStaffSummaries` p95 round-trip latency is at most 100 ms.
- Any page error, browser console error, missing canvas, failed bridge call, or
  insufficient frame sample count fails the benchmark.

The bridge fixture is intentionally deterministic and browser-local. It proves
that the typed bridge invocation and timing harness are working, but it is not a
replacement for a Wails desktop measurement of Go/service latency. A future
packaged desktop benchmark can reuse the reporting contract while replacing the
fixture with `window.go.platform.Bridge`.

## Running locally

From `web/`:

```powershell
npm ci
npx playwright install chromium
npm run benchmark
```

The command builds and serves the production bundle, runs one worker, and writes
`test-results/ban127-playwright.json` plus an attached `ban127-performance.json`
result. The benchmark intentionally fails on a noisy or underpowered machine;
record the machine metadata and investigate the environment rather than weakening
the budget.

A representative BAN-127 Windows validation result on the named baseline was:

```text
frames: 180 samples, effectiveFps 60.247, p95 16.7ms
bridge: 100 samples, p95 0.0ms
```

These numbers are local evidence from the benchmark harness, not a CI claim. CI
should publish its JSON result and the runner's hardware/browser metadata as the
release evidence linked from the Phase 6 review.
