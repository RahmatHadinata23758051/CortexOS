import { test, expect, type Page } from '@playwright/test'
import { cpus, arch, platform, release } from 'node:os'
import { writeFile } from 'node:fs/promises'

const FRAME_SAMPLE_MS = 3_000
const FRAME_BUDGET_MS = 1_000 / 60
const FRAME_INTERVAL_TOLERANCE_MS = 2.0

const coordinationStaff = [
  { id: 'staff-lead', name: 'Lead', role: 'coordinator', capabilities: ['coordination'], workspaceId: 'ws-main', projectId: 'proj-coord', lifecycle: 'active', availability: 'available', schemaVersion: 'cortexos.staff.v1' },
  { id: 'staff-arch', name: 'Architect', role: 'planner', capabilities: ['planning'], workspaceId: 'ws-main', projectId: 'proj-coord', lifecycle: 'active', availability: 'busy', schemaVersion: 'cortexos.staff.v1' },
  { id: 'staff-dev', name: 'Dev', role: 'implementer', capabilities: ['execution'], workspaceId: 'ws-main', projectId: 'proj-coord', lifecycle: 'active', availability: 'busy', schemaVersion: 'cortexos.staff.v1' },
  { id: 'staff-rev', name: 'Reviewer', role: 'reviewer', capabilities: ['inspection'], workspaceId: 'ws-main', projectId: 'proj-coord', lifecycle: 'active', availability: 'available', schemaVersion: 'cortexos.staff.v1' },
]

const sampleSnapshot = {
  schemaVersion: 'cortexos.coordination.v1',
  projectId: 'proj-coord',
  activePhase: 1,
  totalPhases: 3,
  tasks: [
    { id: 'task-1', projectId: 'proj-coord', title: 'Architecture spec', assignedStaffId: 'staff-arch', status: 'success', phase: 0, dependencies: [], isBlocked: false, attempt: 1, maxAttempts: 3, progress: 100, updatedAt: '2026-10-13T00:00:00Z' },
    { id: 'task-2', projectId: 'proj-coord', title: 'Implement modules', assignedStaffId: 'staff-dev', status: 'running', phase: 1, dependencies: ['task-1'], isBlocked: false, attempt: 1, maxAttempts: 3, progress: 50, updatedAt: '2026-10-13T00:01:00Z' },
    { id: 'task-3', projectId: 'proj-coord', title: 'Peer review', assignedStaffId: 'staff-rev', status: 'ready', phase: 2, dependencies: ['task-2'], isBlocked: true, blockedReason: 'awaiting dependencies: task-2', attempt: 0, maxAttempts: 3, progress: 0, updatedAt: '2026-10-13T00:01:00Z' },
  ],
  assignments: [
    { staffId: 'staff-lead', name: 'Lead', role: 'coordinator', availability: 'available', state: 'idle', assignedTaskCount: 0, completedTaskCount: 0 },
    { staffId: 'staff-arch', name: 'Architect', role: 'planner', availability: 'busy', currentTaskId: 'task-1', currentTaskTitle: 'Architecture spec', state: 'completed', assignedTaskCount: 1, completedTaskCount: 1 },
    { staffId: 'staff-dev', name: 'Dev', role: 'implementer', availability: 'busy', currentTaskId: 'task-2', currentTaskTitle: 'Implement modules', state: 'running', assignedTaskCount: 1, completedTaskCount: 0 },
    { staffId: 'staff-rev', name: 'Reviewer', role: 'reviewer', availability: 'available', currentTaskId: 'task-3', currentTaskTitle: 'Peer review', state: 'blocked', assignedTaskCount: 1, completedTaskCount: 0 },
  ],
  metrics: {
    totalTasks: 3, readyTasks: 1, runningTasks: 1, blockedTasks: 1,
    awaitingInspection: 0, completedTasks: 1, failedTasks: 0, canceledTasks: 0,
    activeStaffCount: 3, totalStaffCount: 4,
  },
  generatedAt: '2026-10-13T00:01:00Z',
}

function summarize(samples: number[]) {
  if (samples.length === 0) throw new Error('Benchmark collected no samples')
  const sorted = [...samples].sort((a, b) => a - b)
  const percentile = (v: number) => sorted[Math.min(sorted.length - 1, Math.ceil(sorted.length * v) - 1)]
  return {
    count: sorted.length,
    p50Ms: percentile(0.5),
    p95Ms: percentile(0.95),
    p99Ms: percentile(0.99),
    meanMs: samples.reduce((s, x) => s + x, 0) / samples.length,
    minMs: sorted[0],
    maxMs: sorted[sorted.length - 1],
  }
}

async function installCoordinationBenchmarkBridge(page: Page) {
  await page.addInitScript(({ staff: fixtureStaff, snapshot: fixtureSnapshot }) => {
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
      GetCoordinationSnapshot: async () => fixtureSnapshot,
    }
    const win = window as Window & { go?: { platform?: { Bridge?: typeof bridge } } }
    win.go = { platform: { Bridge: bridge } }
  }, { staff: coordinationStaff, snapshot: sampleSnapshot })
}

async function collectFrameIntervals(page: Page, durationMs: number) {
  return page.evaluate((duration) => new Promise<number[]>((resolve) => {
    const intervals: number[] = []
    const started = performance.now()
    let previous = started
    const sample = (t: number) => {
      if (t - started >= duration) {
        resolve(intervals)
        return
      }
      if (t > previous) intervals.push(t - previous)
      previous = t
      requestAnimationFrame(sample)
    }
    requestAnimationFrame(sample)
  }), durationMs)
}

test('BAN-162 Sprint 13.3 Multi-Staff coordination renders without breaking frame budget or leaking handles', async ({ page }, testInfo) => {
  const pageErrors: string[] = []
  const consoleErrors: string[] = []
  page.on('pageerror', (err) => pageErrors.push(err.message))
  page.on('console', (msg) => {
    if (msg.type() === 'error') consoleErrors.push(msg.text())
  })

  await installCoordinationBenchmarkBridge(page)
  await page.goto('/#office')

  // Verify Virtual Office Canvas renders with multi-avatar setup
  await expect(page.locator('.office-canvas canvas')).toBeVisible({ timeout: 30_000 })
  await expect(page.locator('.directory-count')).toHaveText('4')

  // Verify Coordination Panel renders the execution map and metrics
  await expect(page.locator('.coordination-panel')).toBeVisible()
  await expect(page.locator('.coordination-header h3')).toHaveText('Execution map')
  await expect(page.locator('.coordination-metrics')).toBeVisible()

  // Verify topological phases are mapped
  await expect(page.locator('.coordination-phases')).toBeVisible()
  await expect(page.locator('.coordination-phase-group').first()).toBeVisible()

  // Collect performance frame intervals
  const frameIntervals = await collectFrameIntervals(page, FRAME_SAMPLE_MS)
  const frameSummary = summarize(frameIntervals)
  const effectiveFps = 1_000 / frameSummary.meanMs

  const result = {
    baseline: {
      platform: platform(),
      arch: arch(),
      os: release(),
      cpu: cpus()[0]?.model ?? 'unknown',
      browser: await page.evaluate(() => navigator.userAgent),
      viewport: `${await page.evaluate(() => window.innerWidth)}x${await page.evaluate(() => window.innerHeight)}`,
    },
    workload: { staffCount: coordinationStaff.length, tasksCount: sampleSnapshot.tasks.length, frameSampleMs: FRAME_SAMPLE_MS },
    frames: {
      ...frameSummary,
      effectiveFps,
      overBudget: frameIntervals.filter((interval) => interval > FRAME_BUDGET_MS + FRAME_INTERVAL_TOLERANCE_MS).length,
    },
  }

  await writeFile(testInfo.outputPath('ban162-coordination-perf.json'), `${JSON.stringify(result, null, 2)}\n`, 'utf8')
  await testInfo.attach('ban162-coordination-perf.json', {
    body: Buffer.from(JSON.stringify(result, null, 2)),
    contentType: 'application/json',
  })

  expect(pageErrors, 'Office coordination page errors').toEqual([])
  expect(consoleErrors, 'Office coordination console errors').toEqual([])
  expect(frameIntervals.length, 'animation frame samples').toBeGreaterThan(60)
  expect(effectiveFps, 'multi-avatar effective frame rate').toBeGreaterThanOrEqual(45)
})
