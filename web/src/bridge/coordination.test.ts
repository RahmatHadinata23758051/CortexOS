import { describe, expect, it, vi } from 'vitest'

import {
  loadCoordinationSnapshot,
  onCoordinationChanged,
  onCoordinationLifecycle,
} from './coordination'
import {
  coordinationSchemaVersion,
  CoordinationEventNames,
  type CoordinationBridge,
  type CoordinationSnapshot,
} from '../types/coordination'

const validSnapshot: CoordinationSnapshot = {
  schemaVersion: coordinationSchemaVersion,
  projectId: 'proj-1',
  activePhase: 0,
  totalPhases: 2,
  tasks: [
    {
      id: 'task-1',
      projectId: 'proj-1',
      title: 'Plan execution',
      status: 'success',
      phase: 0,
      dependencies: [],
      isBlocked: false,
      attempt: 1,
      maxAttempts: 3,
      progress: 100,
      updatedAt: '2026-10-10T00:00:00Z',
    },
    {
      id: 'task-2',
      projectId: 'proj-1',
      title: 'Execute code',
      assignedStaffId: 'staff-dev',
      status: 'running',
      phase: 1,
      dependencies: ['task-1'],
      isBlocked: false,
      attempt: 1,
      maxAttempts: 3,
      progress: 50,
      updatedAt: '2026-10-10T00:01:00Z',
    },
  ],
  assignments: [
    {
      staffId: 'staff-dev',
      name: 'Developer Staff',
      role: 'implementer',
      availability: 'busy',
      currentTaskId: 'task-2',
      currentTaskTitle: 'Execute code',
      state: 'running',
      assignedTaskCount: 1,
      completedTaskCount: 0,
    },
  ],
  metrics: {
    totalTasks: 2,
    readyTasks: 0,
    runningTasks: 1,
    blockedTasks: 0,
    awaitingInspection: 0,
    completedTasks: 1,
    failedTasks: 0,
    canceledTasks: 0,
    activeStaffCount: 1,
    totalStaffCount: 1,
  },
  generatedAt: '2026-10-10T00:01:00Z',
}

const mockBridge = (snapshot: unknown = validSnapshot): CoordinationBridge => ({
  GetCoordinationSnapshot: vi.fn().mockResolvedValue(snapshot),
})

describe('coordination bridge operations', () => {
  it('loads valid coordination snapshot with schemaVersion', async () => {
    const bridge = mockBridge()
    const result = await loadCoordinationSnapshot(bridge, 'proj-1')
    expect(result.status).toBe('success')
    if (result.status === 'success') {
      expect(result.snapshot.schemaVersion).toBe(coordinationSchemaVersion)
      expect(result.snapshot.tasks).toHaveLength(2)
      expect(result.snapshot.assignments).toHaveLength(1)
    }
  })

  it('rejects invalid contract response with error state', async () => {
    const bridge = mockBridge({ schemaVersion: 'cortexos.coordination.v0' })
    const result = await loadCoordinationSnapshot(bridge)
    expect(result.status).toBe('error')
    if (result.status === 'error') {
      expect(result.code).toBe('coordination.invalid_contract')
    }
  })

  it('handles bridge exceptions gracefully with error state', async () => {
    const bridge: CoordinationBridge = {
      GetCoordinationSnapshot: vi.fn().mockRejectedValue(new Error('backend offline')),
    }
    const result = await loadCoordinationSnapshot(bridge)
    expect(result.status).toBe('error')
    if (result.status === 'error') {
      expect(result.code).toBe('coordination.bridge_error')
      expect(result.message).toContain('backend offline')
    }
  })

  it('subscribes and receives event callbacks via runtime.EventsOn', () => {
    let capturedHandler: ((payload: unknown) => void) | undefined
    const eventsOn = vi.fn().mockImplementation((_event: string, handler: (payload: unknown) => void) => {
      capturedHandler = handler
      return () => { capturedHandler = undefined }
    })
    ;(globalThis as { runtime?: unknown }).runtime = { EventsOn: eventsOn }

    const received: CoordinationSnapshot[] = []
    const unsub = onCoordinationChanged((snapshot) => received.push(snapshot))

    expect(eventsOn).toHaveBeenCalledWith(CoordinationEventNames.Changed, expect.any(Function))
    expect(capturedHandler).toBeDefined()

    capturedHandler!(validSnapshot)
    expect(received).toHaveLength(1)
    expect(received[0].tasks).toHaveLength(2)

    unsub()
    expect(capturedHandler).toBeUndefined()
  })

  it('subscribes to lifecycle events with stable cleanup', () => {
    let offCalled = false
    const eventsOn = vi.fn().mockImplementation(() => {
      return () => { offCalled = true }
    })
    ;(globalThis as { runtime?: unknown }).runtime = { EventsOn: eventsOn }

    const unsub = onCoordinationLifecycle(CoordinationEventNames.Assigned, vi.fn())
    expect(eventsOn).toHaveBeenCalledWith(CoordinationEventNames.Assigned, expect.any(Function))

    unsub()
    expect(offCalled).toBe(true)
  })
})
