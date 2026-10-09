import { describe, expect, it, vi } from 'vitest'

import {
  loadCockpitRuntime,
  loadCockpitWorkspace,
  registerCockpitProject,
  queryCockpitWorkspace,
  rebuildCockpitRetrieval,
  onCockpitEvent,
  onCockpitWorkspaceChanged,
  onCockpitRuntimeChanged,
} from './cockpit'
import {
  cockpitSchemaVersion,
  CockpitEventNames,
  type CockpitBridge,
  type CockpitRuntimeSnapshot,
  type CockpitWorkspaceSnapshot,
} from '../types'

const validRuntime: CockpitRuntimeSnapshot = {
  schemaVersion: cockpitSchemaVersion,
  status: 'ready',
  environment: 'local',
  provider: 'disabled',
}

const validWorkspace: CockpitWorkspaceSnapshot = {
  schemaVersion: cockpitSchemaVersion,
  projects: [
    {
      id: 'proj-1',
      name: 'Project One',
      defaultBranch: 'main',
      status: 'active',
      createdAt: '2026-10-09T00:00:00Z',
      updatedAt: '2026-10-09T00:00:00Z',
      schemaVersion: cockpitSchemaVersion,
    },
  ],
  worktrees: [],
  vaultNoteCount: 10,
  watcherState: 'ready',
  retrievalVersion: 'cortexos.retrieval.v1',
  retrievalState: 'ready',
}

const mockBridge = (): CockpitBridge => ({
  GetCockpitRuntime: vi.fn().mockResolvedValue(validRuntime),
  GetCockpitWorkspace: vi.fn().mockResolvedValue(validWorkspace),
  RegisterCockpitProject: vi.fn().mockResolvedValue({
    id: 'proj-new',
    name: 'New Project',
    status: 'active',
    createdAt: '2026-10-09T00:00:00Z',
    updatedAt: '2026-10-09T00:00:00Z',
    schemaVersion: cockpitSchemaVersion,
  }),
  QueryCockpitWorkspace: vi.fn().mockResolvedValue([]),
  RebuildCockpitRetrieval: vi.fn().mockResolvedValue(undefined),
})

describe('cockpit bridge operations', () => {
  it('loads valid runtime snapshot', async () => {
    const bridge = mockBridge()
    const state = await loadCockpitRuntime(bridge)
    expect(state).toEqual({
      status: 'success',
      snapshot: validRuntime,
    })
    expect(bridge.GetCockpitRuntime).toHaveBeenCalled()
  })

  it('handles invalid runtime snapshot with structured error', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.GetCockpitRuntime).mockResolvedValue({
      schemaVersion: 'wrong-version',
    } as never)
    const state = await loadCockpitRuntime(bridge)
    expect(state).toEqual({
      status: 'error',
      code: 'cockpit.internal',
      message: 'CortexOS returned an invalid cockpit runtime snapshot',
    })
  })

  it('handles runtime bridge rejection', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.GetCockpitRuntime).mockRejectedValue(new Error('network error'))
    const state = await loadCockpitRuntime(bridge)
    expect(state).toEqual({
      status: 'error',
      code: 'cockpit.internal',
      message: 'network error',
    })
  })

  it('loads valid workspace snapshot', async () => {
    const bridge = mockBridge()
    const state = await loadCockpitWorkspace(bridge)
    expect(state).toEqual({
      status: 'success',
      snapshot: validWorkspace,
    })
    expect(bridge.GetCockpitWorkspace).toHaveBeenCalledWith({
      schemaVersion: cockpitSchemaVersion,
    })
  })

  it('handles invalid workspace snapshot with structured error', async () => {
    const bridge = mockBridge()
    vi.mocked(bridge.GetCockpitWorkspace).mockResolvedValue({
      schemaVersion: cockpitSchemaVersion,
      projects: 'invalid',
    } as never)
    const state = await loadCockpitWorkspace(bridge)
    expect(state).toEqual({
      status: 'error',
      code: 'cockpit.internal',
      message: 'CortexOS returned an invalid cockpit workspace snapshot',
    })
  })

  it('registers project through typed bridge call', async () => {
    const bridge = mockBridge()
    const result = await registerCockpitProject(bridge, {
      id: 'proj-new',
      name: 'New Project',
      repositoryRoot: '/path/repo',
      vaultRoot: '/path/vault',
    })
    expect(result.id).toBe('proj-new')
    expect(bridge.RegisterCockpitProject).toHaveBeenCalledWith({
      schemaVersion: cockpitSchemaVersion,
      id: 'proj-new',
      name: 'New Project',
      repositoryRoot: '/path/repo',
      vaultRoot: '/path/vault',
    })
  })

  it('queries workspace with bounded limit', async () => {
    const bridge = mockBridge()
    await queryCockpitWorkspace(bridge, 'proj-1', 'test query', 5)
    expect(bridge.QueryCockpitWorkspace).toHaveBeenCalledWith({
      schemaVersion: cockpitSchemaVersion,
      projectId: 'proj-1',
      query: 'test query',
      limit: 5,
    })
  })

  it('rebuilds workspace retrieval index', async () => {
    const bridge = mockBridge()
    await rebuildCockpitRetrieval(bridge, 'proj-1')
    expect(bridge.RebuildCockpitRetrieval).toHaveBeenCalledWith({
      schemaVersion: cockpitSchemaVersion,
      projectId: 'proj-1',
    })
  })
})

describe('cockpit runtime event hooks', () => {
  it('returns no-op unsubscribe when runtime is not available', () => {
    const originalRuntime = (globalThis as { runtime?: unknown }).runtime
    delete (globalThis as { runtime?: unknown }).runtime
    const unsub = onCockpitEvent(CockpitEventNames.RuntimeChanged, () => {})
    expect(typeof unsub).toBe('function')
    unsub()
    ;(globalThis as { runtime?: unknown }).runtime = originalRuntime
  })

  it('subscribes and unsubscribes via runtime.EventsOn', () => {
    const offFn = vi.fn()
    const eventsOn = vi.fn().mockReturnValue(offFn)
    ;(globalThis as { runtime?: unknown }).runtime = { EventsOn: eventsOn }

    const handler = vi.fn()
    const unsub = onCockpitWorkspaceChanged(handler)
    expect(eventsOn).toHaveBeenCalledWith(CockpitEventNames.WorkspaceChanged, expect.any(Function))

    // Trigger callback
    const registeredCb = eventsOn.mock.calls[0][1]
    registeredCb({ data: 'test' })
    expect(handler).toHaveBeenCalledWith({ data: 'test' })

    // Unsubscribe
    unsub()
    expect(offFn).toHaveBeenCalled()
  })

  it('supports onCockpitRuntimeChanged helper', () => {
    const offFn = vi.fn()
    const eventsOn = vi.fn().mockReturnValue(offFn)
    ;(globalThis as { runtime?: unknown }).runtime = { EventsOn: eventsOn }

    const handler = vi.fn()
    const unsub = onCockpitRuntimeChanged(handler)
    expect(eventsOn).toHaveBeenCalledWith(CockpitEventNames.RuntimeChanged, expect.any(Function))
    unsub()
    expect(offFn).toHaveBeenCalled()
  })
})
