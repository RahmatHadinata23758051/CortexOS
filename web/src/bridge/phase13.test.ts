import { describe, expect, it } from 'vitest'
import { decideToolCall, proposeToolCall, routeBrainTaskToOrchestra } from './phase13'
import {
  brainOrchestraBridgeSchemaVersion,
  toolDecisionSchemaVersion,
  toolProposalSchemaVersion,
  type Phase13Bridge,
} from '../types/phase13'

function makeBridge(overrides: Partial<Phase13Bridge> = {}): Phase13Bridge {
  return {
    RouteBrainTaskToOrchestra: async (request) => ({
      correlationId: request.correlationId,
      orchestraTaskId: 'task-1',
      status: 'accepted',
      schemaVersion: brainOrchestraBridgeSchemaVersion,
      submittedAt: '2026-10-13T00:00:00Z',
    }),
    ProposeToolCall: async (proposal) => ({
      proposalId: proposal.proposalId,
      status: 'pending',
      submittedAt: '2026-10-13T00:00:00Z',
      schemaVersion: toolProposalSchemaVersion,
    }),
    DecideToolCall: async (decision) => ({
      proposalId: decision.proposalId,
      status: 'executed',
      actor: decision.actor,
      decidedAt: '2026-10-13T00:00:00Z',
      schemaVersion: toolDecisionSchemaVersion,
    }),
    GetToolApproval: async () => ({
      proposalId: 'proposal-1',
      status: 'pending',
      schemaVersion: toolProposalSchemaVersion,
    }),
    ListPendingToolApprovals: async () => [],
    ...overrides,
  }
}

describe('Phase 13 bridge helpers', () => {
  it('routes a bounded request with the versioned contract', async () => {
    const calls: string[] = []
    const bridge = makeBridge({
      RouteBrainTaskToOrchestra: async (request) => {
        calls.push(request.schemaVersion)
        return {
          correlationId: request.correlationId,
          orchestraTaskId: 'task-1',
          status: 'accepted',
          schemaVersion: brainOrchestraBridgeSchemaVersion,
          submittedAt: '2026-10-13T00:00:00Z',
        }
      },
    })

    const result = await routeBrainTaskToOrchestra(bridge, {
      correlationId: 'corr-1',
      userPrompt: 'Inspect the module',
      title: 'Inspect module',
      acceptanceCriteria: ['tests pass'],
    })

    expect(result.orchestraTaskId).toBe('task-1')
    expect(calls).toEqual([brainOrchestraBridgeSchemaVersion])
  })

  it('rejects empty route input before crossing the bridge', async () => {
    await expect(routeBrainTaskToOrchestra(makeBridge(), {
      correlationId: '',
      userPrompt: 'Inspect',
      title: 'Inspect',
      acceptanceCriteria: ['tests pass'],
    })).rejects.toThrow('correlationId is required')
  })

  it('creates redacted tool proposals and decisions', async () => {
    const proposal = await proposeToolCall(makeBridge(), {
      proposalId: 'proposal-1',
      correlationId: 'corr-1',
      taskId: 'task-1',
      worktreeId: 'wt-1',
      toolName: 'file_read',
      capability: 'file_read',
      redactedArgsSummary: 'Read relative path src/main.go',
      argByteSize: 20,
    })
    expect(proposal.status).toBe('pending')

    const decision = await decideToolCall(makeBridge(), {
      proposalId: 'proposal-1',
      decision: 'approve',
      actor: 'operator-1',
      reason: 'Reviewed',
      idempotencyKey: 'idem-1',
    })
    expect(decision.status).toBe('executed')
  })
})
