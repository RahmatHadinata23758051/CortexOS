export const brainOrchestraBridgeSchemaVersion = 'cortexos.brain-orchestra.bridge.v1' as const
export const toolProposalSchemaVersion = 'cortexos.tool-approval.proposal.v1' as const
export const toolDecisionSchemaVersion = 'cortexos.tool-approval.decision.v1' as const

import type { GroundedContextRef } from './knowledge-context'
export type { GroundedContextRef }

export type BrainOrchestraRouteRequest = {
  correlationId: string
  userPrompt: string
  staffId?: string
  title: string
  acceptanceCriteria: string[]
  dependencies?: string[]
  priority?: string
  maxPlanSteps?: number
  maxPlanTokens?: number
  schemaVersion: typeof brainOrchestraBridgeSchemaVersion
}

export type BrainOrchestraRouteResult = {
  correlationId: string
  orchestraTaskId?: string
  status: 'accepted' | 'rejected' | 'queued' | 'canceled'
  message?: string
  groundedContext?: GroundedContextRef[]
  schemaVersion: string
  submittedAt: string
}

export type ToolProposal = {
  proposalId: string
  correlationId: string
  taskId: string
  worktreeId: string
  toolName: string
  capability: string
  redactedArgsSummary: string
  argByteSize: number
  proposedAt?: string
  schemaVersion: typeof toolProposalSchemaVersion
}

export type ToolProposalResult = {
  proposalId: string
  status: string
  submittedAt: string
  schemaVersion: string
}

export type ToolApprovalDecision = {
  proposalId: string
  decision: 'approve' | 'deny'
  actor: string
  reason: string
  idempotencyKey: string
  decidedAt?: string
  schemaVersion: typeof toolDecisionSchemaVersion
}

export type ToolApprovalResult = {
  proposalId: string
  status: string
  actor: string
  decidedAt: string
  schemaVersion: string
}

export type ToolProposalRecord = {
  proposalId: string
  correlationId?: string
  taskId?: string
  worktreeId?: string
  toolName?: string
  capability?: string
  redactedArgsSummary?: string
  argByteSize?: number
  status: string
  proposedAt?: string
  actor?: string
  reason?: string
  decidedAt?: string
  executionId?: string
  schemaVersion: string
}

export type Phase13Bridge = {
  RouteBrainTaskToOrchestra(request: BrainOrchestraRouteRequest): Promise<BrainOrchestraRouteResult>
  ProposeToolCall(proposal: ToolProposal): Promise<ToolProposalResult>
  DecideToolCall(decision: ToolApprovalDecision): Promise<ToolApprovalResult>
  GetToolApproval(proposalId: string): Promise<ToolProposalRecord>
  ListPendingToolApprovals(): Promise<ToolProposalRecord[]>
}
