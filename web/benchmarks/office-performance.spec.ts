import { test, expect, type Page, type TestInfo } from '@playwright/test'
import { cpus, arch, platform, release } from 'node:os'
import { writeFile } from 'node:fs/promises'

const FRAME_SAMPLE_MS = 3_000
const BRIDGE_WARMUP_SAMPLES = 10
const BRIDGE_SAMPLES = 100
const FRAME_BUDGET_MS = 1_000 / 60
const FRAME_INTERVAL_TOLERANCE_MS = 1
const BRIDGE_P95_BUDGET_MS = 100
const staff = Array.from({ length: 8 }, (_, index) => ({
  id: `staff-${index + 1}`,
  name: `Staff ${index + 1}`,
  role: (['coordinator', 'planner', 'implementer', 'reviewer', 'specialist'] as const)[index % 5],
  capabilities: ['benchmark'],
  workspaceId: 'benchmark-workspace',
  projectId: 'benchmark-project',
  worktreeId: `worktree-${index + 1}`,
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
  workload: { staffCount: number; frameSampleMs: number }
  frames: SampleSummary & { effectiveFps: number; overBudget: number }
  bridge: SampleSummary
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

async function installBenchmarkBridge(page: Page) {
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
        retrievalVersion: 'benchmark',
        retrievalState: 'ready',
      }),
      ListStaffSummaries: async () => fixtureStaff,
    }
    const benchmarkWindow = window as Window & { go?: { platform?: { Bridge?: typeof bridge } } }
    benchmarkWindow.go = { platform: { Bridge: bridge } }
  }, { staff })
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

async function measureBridge(page: Page) {
  return page.evaluate(async ({ warmup, samples }) => {
    const bridge = (window as Window & { go?: { platform?: { Bridge?: { ListStaffSummaries: (request: unknown) => Promise<unknown> } } } }).go?.platform?.Bridge
    if (!bridge?.ListStaffSummaries) throw new Error('Benchmark bridge is unavailable')
    const request = { schemaVersion: 'cortexos.staff.bridge.v1', filter: { activeOnly: false } }
    for (let index = 0; index < warmup; index += 1) await bridge.ListStaffSummaries(request)
    const durations: number[] = []
    for (let index = 0; index < samples; index += 1) {
      const started = performance.now()
      await bridge.ListStaffSummaries(request)
      durations.push(performance.now() - started)
    }
    return durations
  }, { warmup: BRIDGE_WARMUP_SAMPLES, samples: BRIDGE_SAMPLES })
}

async function writeResult(result: BenchmarkResult, testInfo: TestInfo) {
  await writeFile(testInfo.outputPath('ban127-performance.json'), `${JSON.stringify(result, null, 2)}\n`, 'utf8')
  await testInfo.attach('ban127-performance.json', {
    body: Buffer.from(JSON.stringify(result, null, 2)),
    contentType: 'application/json',
  })
}

test('BAN-127 OfficeScene frame rate and bridge p95 stay within budget', async ({ page }, testInfo) => {
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  page.on('pageerror', (error) => pageErrors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })

  await installBenchmarkBridge(page)
  await page.goto('/#office')
  await expect(page.locator('.office-canvas canvas')).toBeVisible({ timeout: 30_000 })
  await expect(page.locator('.directory-count')).toHaveText('8')
  await page.waitForTimeout(500)
  await page.evaluate(() => document.visibilityState === 'visible' || document.body.focus())

  const frameIntervals = await collectFrameIntervals(page, FRAME_SAMPLE_MS)
  const bridgeDurations = await measureBridge(page)
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
    workload: { staffCount: staff.length, frameSampleMs: FRAME_SAMPLE_MS },
    frames: {
      ...frameSummary,
      effectiveFps,
      overBudget: frameIntervals.filter((interval) => interval > FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS).length,
    },
    bridge: bridgeSummary,
  }
  await writeResult(result, testInfo)
  console.info('BAN-127 performance result', JSON.stringify(result))

  expect(pageErrors, 'Office page errors').toEqual([])
  expect(consoleErrors, 'Office console errors').toEqual([])
  expect(frameIntervals.length, 'sufficient animation frame samples').toBeGreaterThan(60)
  expect(frameSummary.p95Ms, 'OfficeScene p95 frame interval').toBeLessThanOrEqual(FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS)
  expect(effectiveFps, 'OfficeScene effective frame rate').toBeGreaterThanOrEqual(60)
  expect(bridgeSummary.p95Ms, 'ListStaffSummaries bridge p95').toBeLessThanOrEqual(BRIDGE_P95_BUDGET_MS)
})
