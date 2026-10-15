/** Orchestra plan proposal/confirmation bridge contract. */
export const orchestraPlanSchemaVersion = 'cortexos.orchestra.plan.v1' as const

export type OrchestraPlanTask = {
  index: number
  title: string
  role: string
  callSign?: string
  action: string
  resource?: string
  acceptanceCriteria: string[]
  estimatedComplexity?: 'trivial' | 'standard' | 'complex' | string
  dependencies: number[]
}

export type OrchestraPlanProposalRequest = {
  correlationId: string
  projectId?: string
  userPrompt: string
  title: string
  summary?: string
  requireConfirmation?: boolean
  tasks: OrchestraPlanTask[]
  schemaVersion: typeof orchestraPlanSchemaVersion
}

export type OrchestraPlanProposal = {
  planId: string
  correlationId: string
  projectId: string
  userPrompt: string
  title: string
  summary: string
  riskLevel: 'low' | 'medium' | 'high' | string
  requiresConfirmation: boolean
  status: 'proposed' | 'confirmed' | 'rejected' | 'auto_dispatched' | string
  tasks: OrchestraPlanTask[]
  createdAt: string
  confirmedAt?: string
  dispatchedTaskIds: string[]
  schemaVersion: typeof orchestraPlanSchemaVersion
}

export type OrchestraPlanConfirmRequest = {
  planId: string
  schemaVersion: typeof orchestraPlanSchemaVersion
}

export type OrchestraPlanConfirmResult = {
  planId: string
  status: string
  dispatchedTaskIds: string[]
  confirmedAt: string
  schemaVersion: typeof orchestraPlanSchemaVersion
}

export type OrchestraPlanRejectRequest = {
  planId: string
  reason?: string
  schemaVersion: typeof orchestraPlanSchemaVersion
}

export type OrchestraPlanGetRequest = {
  planId: string
  schemaVersion?: typeof orchestraPlanSchemaVersion
}

export type OrchestraPlanBridge = {
  ProposeOrchestraPlan(
    request: OrchestraPlanProposalRequest,
  ): Promise<OrchestraPlanProposal>
  ConfirmOrchestraPlan(
    request: OrchestraPlanConfirmRequest,
  ): Promise<OrchestraPlanConfirmResult>
  RejectOrchestraPlan(request: OrchestraPlanRejectRequest): Promise<void>
  GetOrchestraPlan(request: OrchestraPlanGetRequest): Promise<OrchestraPlanProposal>
}

export const OrchestraPlanEventNames = {
  Proposed: 'cortexos:orchestra:plan:proposed',
  Confirmed: 'cortexos:orchestra:plan:confirmed',
  Rejected: 'cortexos:orchestra:plan:rejected',
  AutoDispatched: 'cortexos:orchestra:plan:auto-dispatched',
  WorkerSpawned: 'cortexos:orchestra:worker:spawned',
  WorkerUpdated: 'cortexos:orchestra:worker:updated',
  WorkerUnspawned: 'cortexos:orchestra:worker:unspawned',
} as const
