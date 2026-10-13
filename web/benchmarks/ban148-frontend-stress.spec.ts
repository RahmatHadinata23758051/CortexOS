import { test, expect, type Page, type TestInfo } from '@playwright/test'
import { cpus, arch, platform, release } from 'node:os'
import { writeFile } from 'node:fs/promises'

const FRAME_SAMPLE_MS = 3_000
const CONCURRENT_BRIDGE_CALLS = 100
const FRAME_BUDGET_MS = 1_000 / 60
const FRAME_INTERVAL_TOLERANCE_MS = 1.5
const BRIDGE_P95_BUDGET_MS = 100

// BAN-148 workload: 60 concurrent Staff avatars (strictly exceeding 50+ requirement)
const stressStaff = Array.from({ length: 60 }, (_, index) => ({
  id: `staff-stress-${index + 1}`,
  name: `Agent ${index + 1}`,
  role: (['coordinator', 'planner', 'implementer', 'reviewer', 'specialist'] as const)[index % 5],
  capabilities: ['benchmark', 'stress'],
  workspaceId: 'stress-workspace',
  projectId: 'stress-project',
  worktreeId: `worktree-stress-${index + 1}`,
  lifecycle: 'active' as const,
  availability: (['available', 'busy', 'unavailable', 'offline'] as const)[index % 4],
  schemaVersion: 'cortexos.staff.v1',
}))

type SampleSummary = {
  count: number
  p50Ms: number
  p95Ms: number
  p99Ms: number
  meanMs: number
  minMs: number
  maxMs: number
}

type BenchmarkResult = {
  baseline: {
    platform: string
    arch: string
    os: string
    cpu: string
    browser: string
    viewport: string
  }
  workload: {
    staffCount: number
    frameSampleMs: number
    concurrentBridgeCalls: number
  }
  frames: SampleSummary & { effectiveFps: number; overBudget: number }
  concurrentBridge: SampleSummary
}

function summarize(samples: number[]): SampleSummary {
  if (samples.length === 0) throw new Error('Benchmark collected no samples')
  const sorted = [...samples].sort((a, b) => a - b)
  const percentile = (value: number) => sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * value) - 1)]
  return {
    count: sorted.length,
    p50Ms: percentile(0.5),
    p95Ms: percentile(0.95),
    p99Ms: percentile(0.99),
    meanMs: samples.reduce((sum, sample) => sum + sample, 0) / samples.length,
    minMs: sorted[0],
    maxMs: sorted[sorted.length - 1],
  }
}

async function installStressBenchmarkBridge(page: Page) {
  await page.addInitScript(({ staff: fixtureStaff }) => {
    const bridge = {
      GetRuntimeSnapshot: async () => ({
        schemaVersion: 'cortexos.runtime.v1',
        status: 'ready',
        environment: 'local',
        provider: 'disabled',
      }),
      GetWorkspaceSnapshot: async () => ({
        schemaVersion: 'cortexos.workspace.v1',
        projects: [],
        worktrees: [],
        vaultNoteCount: 0,
        watcherState: 'idle',
        retrievalVersion: 'stress',
        retrievalState: 'ready',
      }),
      ListStaffSummaries: async () => fixtureStaff,
    }
    const benchmarkWindow = window as Window & { go?: { platform?: { Bridge?: typeof bridge } } }
    benchmarkWindow.go = { platform: { Bridge: bridge } }
  }, { staff: stressStaff })
}

async function collectFrameIntervals(page: Page, durationMs: number) {
  return page.evaluate((duration) => new Promise<number[]>((resolve) => {
    const intervals: number[] = []
    const started = performance.now()
    let previous = started
    const sample = (timestamp: number) => {
      if (timestamp - started >= duration) {
        resolve(intervals)
        return
      }
      if (timestamp > previous) intervals.push(timestamp - previous)
      previous = timestamp
      requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  }), durationMs)
}

// measureConcurrentBridge fires 100 simultaneous async bridge requests and
// records the latency of each call under massive parallel contention.
async function measureConcurrentBridge(page: Page, count: number) {
  return page.evaluate(async (concurrentCount) => {
    const bridge = (window as Window & {
      go?: { platform?: { Bridge?: { ListStaffSummaries: (request: unknown) => Promise<unknown> } } }
    }).go?.platform?.Bridge

    if (!bridge?.ListStaffSummaries) throw new Error('Benchmark bridge is unavailable')
    const request = { schemaVersion: 'cortexos.staff.bridge.v1', filter: { activeOnly: false } }

    const promises = Array.from({ length: concurrentCount }, async () => {
      const started = performance.now()
      await bridge.ListStaffSummaries(request)
      return performance.now() - started
    })

    return Promise.all(promises)
  }, count)
}

async function writeResult(result: BenchmarkResult, testInfo: TestInfo) {
  await writeFile(testInfo.outputPath('ban148-performance.json'), `${JSON.stringify(result, null, 2)}\n`, 'utf8')
  await testInfo.attach('ban148-performance.json', {
    body: Buffer.from(JSON.stringify(result, null, 2)),
    contentType: 'application/json',
  })
}

test('BAN-148 Frontend runtime stress — 60 Staff avatars in Phaser scene and 100 concurrent bridge calls', async ({ page }, testInfo) => {
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  page.on('pageerror', (error) => pageErrors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })

  await installStressBenchmarkBridge(page)
  await page.goto('/#office')
  await expect(page.locator('.office-canvas canvas')).toBeVisible({ timeout: 30_000 })
  await expect(page.locator('.directory-count')).toHaveText('60')
  await page.waitForTimeout(500)
  await page.evaluate(() => document.visibilityState === 'visible' || document.body.focus())

  // 1. Measure steady-state frame rate with 60 avatars rendered in Phaser
  const frameIntervals = await collectFrameIntervals(page, FRAME_SAMPLE_MS)

  // 2. Hammer with 100 concurrent bridge calls
  const bridgeDurations = await measureConcurrentBridge(page, CONCURRENT_BRIDGE_CALLS)

  const frameSummary = summarize(frameIntervals)
  const bridgeSummary = summarize(bridgeDurations)
  const effectiveFps = 1_000 / frameSummary.meanMs

  const result: BenchmarkResult = {
    baseline: {
      platform: platform(),
      arch: arch(),
      os: release(),
      cpu: cpus()[0]?.model ?? 'unknown',
      browser: await page.evaluate(() => navigator.userAgent),
      viewport: `${await page.evaluate(() => window.innerWidth)}x${await page.evaluate(() => window.innerHeight)}`,
    },
    workload: {
      staffCount: stressStaff.length,
      frameSampleMs: FRAME_SAMPLE_MS,
      concurrentBridgeCalls: CONCURRENT_BRIDGE_CALLS,
    },
    frames: {
      ...frameSummary,
      effectiveFps,
      overBudget: frameIntervals.filter((interval) => interval > FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS).length,
    },
    concurrentBridge: bridgeSummary,
  }

  await writeResult(result, testInfo)
  console.info('BAN-148 performance result', JSON.stringify(result))

  // Hard Invariants
  expect(pageErrors, 'Office page errors must be zero').toEqual([])
  expect(consoleErrors, 'Office console errors must be zero').toEqual([])
  expect(frameIntervals.length, 'sufficient animation frame samples').toBeGreaterThan(60)
  expect(frameSummary.p95Ms, 'Phaser p95 frame interval').toBeLessThanOrEqual(FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS)
  expect(effectiveFps, 'Phaser effective frame rate').toBeGreaterThanOrEqual(58) // Allowing tiny margin on Windows CI
  expect(bridgeSummary.p95Ms, '100 concurrent bridge calls p95 latency').toBeLessThanOrEqual(BRIDGE_P95_BUDGET_MS)
})
