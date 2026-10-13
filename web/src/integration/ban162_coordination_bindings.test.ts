import { describe, expect, it } from 'vitest'
import {
  coordinationSchemaVersion,
  isCoordinationSnapshot,
  CoordinationEventNames,
  type CoordinationBridge,
  type CoordinationSnapshot,
  type CoordinationEventPayload,
} from '../types/coordination'

describe('BAN-162 / Sprint 13.3: Coordination Go ↔ TypeScript bridge contract', () => {
  it('coordinationSchemaVersion matches Go CoordinationSchemaVersion constant', () => {
    expect(coordinationSchemaVersion).toBe('cortexos.coordination.v1')
  })

  it('validates a conformant CoordinationSnapshot contract', () => {
    const valid: CoordinationSnapshot = {
      schemaVersion: coordinationSchemaVersion,
      projectId: 'proj-alpha',
      activePhase: 0,
      totalPhases: 2,
      tasks: [
        {
          id: 'task-1',
          projectId: 'proj-alpha',
          title: 'Plan architecture',
          status: 'success',
          phase: 0,
          dependencies: [],
          isBlocked: false,
          attempt: 1,
          maxAttempts: 3,
          progress: 100,
          updatedAt: '2026-10-13T00:00:00Z',
        },
      ],
      assignments: [
        {
          staffId: 'staff-lead',
          name: 'Lead',
          role: 'coordinator',
          availability: 'available',
          state: 'completed',
          assignedTaskCount: 1,
          completedTaskCount: 1,
        },
      ],
      metrics: {
        totalTasks: 1,
        readyTasks: 0,
        runningTasks: 0,
        blockedTasks: 0,
        awaitingInspection: 0,
        completedTasks: 1,
        failedTasks: 0,
        canceledTasks: 0,
        activeStaffCount: 1,
        totalStaffCount: 1,
      },
      generatedAt: '2026-10-13T00:00:00Z',
    }
    expect(isCoordinationSnapshot(valid)).toBe(true)
  })

  it('CoordinationBridge interface declares GetCoordinationSnapshot', () => {
    const bridgeShape: CoordinationBridge = {
      GetCoordinationSnapshot: async () => ({} as CoordinationSnapshot),
    }
    expect(typeof bridgeShape.GetCoordinationSnapshot).toBe('function')
  })

  it('CoordinationEventNames exposes all canonical lifecycle event names', () => {
    expect(CoordinationEventNames.Changed).toBe('cortexos:coordination:changed')
    expect(CoordinationEventNames.Assigned).toBe('cortexos:coordination:assigned')
    expect(CoordinationEventNames.Reassigned).toBe('cortexos:coordination:reassigned')
    expect(CoordinationEventNames.Progress).toBe('cortexos:coordination:progress')
    expect(CoordinationEventNames.Blocked).toBe('cortexos:coordination:blocked')
    expect(CoordinationEventNames.Completed).toBe('cortexos:coordination:completed')
    expect(CoordinationEventNames.Canceled).toBe('cortexos:coordination:canceled')
  })

  it('CoordinationEventPayload matches Go struct semantics', () => {
    const payload: CoordinationEventPayload = {
      schemaVersion: coordinationSchemaVersion,
      eventType: 'blocked',
      taskId: 'task-2',
      projectId: 'proj-alpha',
      staffId: 'staff-dev',
      role: 'implementer',
      status: 'ready',
      phase: 1,
      isBlocked: true,
      blockedReason: 'awaiting dependencies: task-1',
      progress: 20,
      timestamp: '2026-10-13T00:00:00Z',
    }
    expect(payload.schemaVersion).toBe(coordinationSchemaVersion)
    expect(payload.isBlocked).toBe(true)
    expect(payload.blockedReason).toContain('task-1')
  })
})
