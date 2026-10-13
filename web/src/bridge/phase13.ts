import {
  brainOrchestraBridgeSchemaVersion,
  toolDecisionSchemaVersion,
  toolProposalSchemaVersion,
  type BrainOrchestraRouteRequest,
  type BrainOrchestraRouteResult,
  type Phase13Bridge,
  type ToolApprovalDecision,
  type ToolApprovalResult,
  type ToolProposal,
  type ToolProposalRecord,
  type ToolProposalResult,
} from '../types/phase13'

export type RouteBrainTaskInput = Omit<BrainOrchestraRouteRequest, 'schemaVersion'>
export type ProposeToolInput = Omit<ToolProposal, 'schemaVersion' | 'proposedAt'> & { proposedAt?: string }
export type DecideToolInput = Omit<ToolApprovalDecision, 'schemaVersion' | 'decidedAt'> & { decidedAt?: string }

export function makeRouteBrainTaskRequest(input: RouteBrainTaskInput): BrainOrchestraRouteRequest {
  if (!input.correlationId.trim()) throw new Error('correlationId is required')
  if (!input.userPrompt.trim()) throw new Error('userPrompt is required')
  if (!input.title.trim()) throw new Error('title is required')
  if (input.acceptanceCriteria.length === 0) throw new Error('acceptanceCriteria is required')
  return { ...input, schemaVersion: brainOrchestraBridgeSchemaVersion }
}

export async function routeBrainTaskToOrchestra(
  bridge: Phase13Bridge,
  input: RouteBrainTaskInput,
): Promise<BrainOrchestraRouteResult> {
  return bridge.RouteBrainTaskToOrchestra(makeRouteBrainTaskRequest(input))
}

export function makeToolProposal(input: ProposeToolInput): ToolProposal {
  if (!input.proposalId.trim()) throw new Error('proposalId is required')
  if (!input.correlationId.trim()) throw new Error('correlationId is required')
  if (!input.redactedArgsSummary.trim()) throw new Error('redactedArgsSummary is required')
  return {
    ...input,
    proposedAt: input.proposedAt ?? new Date().toISOString(),
    schemaVersion: toolProposalSchemaVersion,
  }
}

export async function proposeToolCall(
  bridge: Phase13Bridge,
  input: ProposeToolInput,
): Promise<ToolProposalResult> {
  return bridge.ProposeToolCall(makeToolProposal(input))
}

export function makeToolApprovalDecision(input: DecideToolInput): ToolApprovalDecision {
  if (!input.proposalId.trim()) throw new Error('proposalId is required')
  if (!input.actor.trim()) throw new Error('actor is required')
  if (!input.idempotencyKey.trim()) throw new Error('idempotencyKey is required')
  return {
    ...input,
    decidedAt: input.decidedAt ?? new Date().toISOString(),
    schemaVersion: toolDecisionSchemaVersion,
  }
}

export async function decideToolCall(
  bridge: Phase13Bridge,
  input: DecideToolInput,
): Promise<ToolApprovalResult> {
  return bridge.DecideToolCall(makeToolApprovalDecision(input))
}

export function getToolApproval(bridge: Phase13Bridge, proposalId: string): Promise<ToolProposalRecord> {
  if (!proposalId.trim()) throw new Error('proposalId is required')
  return bridge.GetToolApproval(proposalId)
}

export function listPendingToolApprovals(bridge: Phase13Bridge): Promise<ToolProposalRecord[]> {
  return bridge.ListPendingToolApprovals()
}
