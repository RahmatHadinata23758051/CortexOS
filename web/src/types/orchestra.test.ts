import { describe, expect, it } from 'vitest'

import {
  isOrchestraTaskDetail,
  isOrchestraTaskStatus,
  isOrchestraTaskSummary,
  orchestraSchemaVersion,
  type OrchestraTaskDetail,
  type OrchestraTaskStatus,
  type OrchestraTaskSummary,
} from './orchestra'

describe('orchestra contract and type guards', () => {
  it('validates all canonical Orchestra task statuses', () => {
    const canonicalStatuses: OrchestraTaskStatus[] = [
      'draft',
      'ready',
      'running',
      'awaitingInspection',
      'success',
      'failed',
      'canceled',
    ]

    for (const status of canonicalStatuses) {
      expect(isOrchestraTaskStatus(status)).toBe(true)
    }
  })

  it('rejects unsupported or arbitrary statuses', () => {
    expect(isOrchestraTaskStatus('pending')).toBe(false)
    expect(isOrchestraTaskStatus('done')).toBe(false)
    expect(isOrchestraTaskStatus('approved')).toBe(false)
    expect(isOrchestraTaskStatus('')).toBe(false)
    expect(isOrchestraTaskStatus(null)).toBe(false)
    expect(isOrchestraTaskStatus(undefined)).toBe(false)
  })

  it('validates a conformant OrchestraTaskSummary', () => {
    const valid: OrchestraTaskSummary = {
      id: 'task-101',
      projectId: 'proj-alpha',
      status: 'awaitingInspection',
      attempt: 1,
      maxAttempts: 3,
      progress: 80,
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskSummary(valid)).toBe(true)
  })

  it('rejects task summaries with invalid fields', () => {
    const invalidVersion = {
      id: 'task-101',
      projectId: 'proj-alpha',
      status: 'ready',
      attempt: 0,
      maxAttempts: 3,
      progress: 20,
      schemaVersion: 'cortexos.orchestra.bridge.v2',
    }
    expect(isOrchestraTaskSummary(invalidVersion)).toBe(false)

    const invalidProgress = {
      id: 'task-101',
      projectId: 'proj-alpha',
      status: 'ready',
      attempt: 0,
      maxAttempts: 3,
      progress: 150, // exceeds 100
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskSummary(invalidProgress)).toBe(false)

    const negativeAttempt = {
      id: 'task-101',
      projectId: 'proj-alpha',
      status: 'ready',
      attempt: -1,
      maxAttempts: 3,
      progress: 20,
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskSummary(negativeAttempt)).toBe(false)
  })

  it('validates a conformant OrchestraTaskDetail', () => {
    const detail: OrchestraTaskDetail = {
      id: 'task-202',
      projectId: 'proj-beta',
      status: 'running',
      attempt: 2,
      maxAttempts: 4,
      progress: 50,
      acceptanceCount: 3,
      dependencyCount: 1,
      evidence: [{ id: 'ev-1', count: 2 }],
      timeline: [
        {
          sequence: 1,
          type: 'task.dispatched',
          from: 'ready',
          to: 'running',
          occurredAt: '2026-10-10T00:00:00Z',
          evidenceIds: ['ev-1'],
        },
      ],
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskDetail(detail)).toBe(true)
  })

  it('rejects task details with non-array evidence or timeline', () => {
    const invalid = {
      id: 'task-202',
      projectId: 'proj-beta',
      status: 'running',
      attempt: 2,
      maxAttempts: 4,
      progress: 50,
      acceptanceCount: 3,
      dependencyCount: 1,
      evidence: 'not-an-array',
      timeline: [],
      schemaVersion: orchestraSchemaVersion,
    }
    expect(isOrchestraTaskDetail(invalid)).toBe(false)
  })
})
