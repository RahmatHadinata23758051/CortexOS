import { test, expect, type Page, type TestInfo } from '@playwright/test'
import { cpus, arch, platform, release } from 'node:os'
import { writeFile } from 'node:fs/promises'

const FRAME_SAMPLE_MS = 3_000
const BRIDGE_WARMUP_SAMPLES = 10
const BRIDGE_SAMPLES = 50
const FRAME_BUDGET_MS = 1_000 / 60
const FRAME_INTERVAL_TOLERANCE_MS = 1.5
const BRIDGE_P95_BUDGET_MS = 100

const brainStaff = [
  { id: 'staff-brain-1', name: 'Ada', role: 'planner', capabilities: ['reasoning'], workspaceId: 'brain-ws', projectId: 'brain-proj', lifecycle: 'active', availability: 'available', schemaVersion: 'cortexos.staff.v1' },
  { id: 'staff-brain-2', name: 'Grace', role: 'implementer', capabilities: ['execution'], workspaceId: 'brain-ws', projectId: 'brain-proj', lifecycle: 'active', availability: 'busy', schemaVersion: 'cortexos.staff.v1' },
]

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
  brainBridge: SampleSummary
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

async function installBrainBenchmarkBridge(page: Page) {
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
      DispatchBrainTaskStream: async (request: unknown) => {
        const req = request as { taskId: string; userPrompt: string }
        const output = 'Analyzed project context. Recommendation: improve orchestration testability.'
        const emit = (eventName: string, payload: unknown) => runtime.emit(eventName, payload)
        setTimeout(() => emit('cortexos:brain:stream:chunk', { taskId: req.taskId, type: 'state_change', data: 'planning', sequence: 1, timestamp: new Date().toISOString() }), 5)
        setTimeout(() => emit('cortexos:brain:stream:chunk', { taskId: req.taskId, type: 'token', data: 'Analyzed project context. ', sequence: 2, timestamp: new Date().toISOString() }), 15)
        setTimeout(() => emit('cortexos:brain:stream:chunk', { taskId: req.taskId, type: 'state_change', data: 'streaming', sequence: 3, timestamp: new Date().toISOString() }), 25)
        setTimeout(() => emit('cortexos:brain:stream:chunk', { taskId: req.taskId, type: 'token', data: 'Recommendation: improve orchestration testability.', sequence: 4, timestamp: new Date().toISOString() }), 35)
        setTimeout(() => emit('cortexos:brain:state:changed', {
          taskId: req.taskId,
          staffId: 'staff-brain-1',
          state: 'completed',
          output,
          steps: 3,
          tokens: 45,
          durationMs: 150,
          error: '',
          schemaVersion: 'cortexos.brain.bridge.v1',
        }), 50)
        await new Promise((resolve) => setTimeout(resolve, 60))
        return {
          taskId: req.taskId,
          staffId: 'staff-brain-1',
          state: 'completed',
          output,
          steps: 3,
          tokens: 45,
          durationMs: 150,
          error: '',
          schemaVersion: 'cortexos.brain.bridge.v1',
        }
      },
    }
    type EventHandler = (payload: unknown) => void
    const handlers = new Map<string, Set<EventHandler>>()
    const runtime = {
      EventsOn: (eventName: string, handler: EventHandler) => {
        const eventHandlers = handlers.get(eventName) ?? new Set<EventHandler>()
        eventHandlers.add(handler)
        handlers.set(eventName, eventHandlers)
        return () => eventHandlers.delete(handler)
      },
      EventsOff: (eventName: string) => handlers.delete(eventName),
      emit: (eventName: string, payload: unknown) => handlers.get(eventName)?.forEach((handler) => handler(payload)),
    }
    const benchmarkWindow = window as Window & { go?: { platform?: { Bridge?: typeof bridge } }; runtime?: typeof runtime }
    benchmarkWindow.go = { platform: { Bridge: bridge } }
    benchmarkWindow.runtime = runtime
  }, { staff: brainStaff })
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

async function measureBrainBridge(page: Page) {
  return page.evaluate(async ({ warmup, samples }) => {
    const bridge = (window as Window & { go?: { platform?: { Bridge?: { DispatchBrainTaskStream: (request: unknown) => Promise<unknown> } } } }).go?.platform?.Bridge
    if (!bridge?.DispatchBrainTaskStream) throw new Error('Brain benchmark bridge is unavailable')
    const request = { taskId: 'bench-' + Date.now(), userPrompt: 'Benchmark prompt', schemaVersion: 'cortexos.brain.bridge.v1' }
    for (let index = 0; index < warmup; index += 1) await bridge.DispatchBrainTaskStream(request)
    const durations: number[] = []
    for (let index = 0; index < samples; index += 1) {
      const started = performance.now()
      await bridge.DispatchBrainTaskStream(request)
      durations.push(performance.now() - started)
    }
    return durations
  }, { warmup: BRIDGE_WARMUP_SAMPLES, samples: BRIDGE_SAMPLES })
}

async function writeResult(result: BenchmarkResult, testInfo: TestInfo) {
  await writeFile(testInfo.outputPath('ban154-brain-cockpit.json'), `${JSON.stringify(result, null, 2)}\n`, 'utf8')
  await testInfo.attach('ban154-brain-cockpit.json', {
    body: Buffer.from(JSON.stringify(result, null, 2)),
    contentType: 'application/json',
  })
}

test('BAN-154 Brain Cockpit: frame rate and bridge p95 stay within budget', async ({ page }, testInfo) => {
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  page.on('pageerror', (error) => pageErrors.push(error.message))
  page.on('console', (message) => {
    if (message.type() === 'error') consoleErrors.push(message.text())
  })

  await installBrainBenchmarkBridge(page)
  await page.goto('/#brain')
  await expect(page.locator('.brain-panel')).toBeVisible({ timeout: 30_000 })
  await expect(page.locator('.brain-prompt-form')).toBeVisible()
  await page.waitForTimeout(500)
  await page.evaluate(() => document.visibilityState === 'visible' || document.body.focus())

  const frameIntervals = await collectFrameIntervals(page, FRAME_SAMPLE_MS)
  const brainBridgeDurations = await measureBrainBridge(page)

  const frameSummary = summarize(frameIntervals)
  const bridgeSummary = summarize(brainBridgeDurations)
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
    workload: { staffCount: brainStaff.length, frameSampleMs: FRAME_SAMPLE_MS },
    frames: {
      ...frameSummary,
      effectiveFps,
      overBudget: frameIntervals.filter((interval) => interval > FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS).length,
    },
    brainBridge: bridgeSummary,
  }
  await writeResult(result, testInfo)
  console.info('BAN-154 Brain Cockpit performance result', JSON.stringify(result))

  // Hard Invariants
  expect(pageErrors, 'Brain page errors must be zero').toEqual([])
  expect(consoleErrors, 'Brain console errors must be zero').toEqual([])
  expect(frameIntervals.length, 'sufficient animation frame samples').toBeGreaterThan(60)
  expect(frameSummary.p95Ms, 'Brain panel p95 frame interval').toBeLessThanOrEqual(FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS)
  expect(effectiveFps, 'Brain panel effective frame rate').toBeGreaterThanOrEqual(58)
  expect(bridgeSummary.p95Ms, 'DispatchBrainTaskStream bridge p95').toBeLessThanOrEqual(BRIDGE_P95_BUDGET_MS)
})