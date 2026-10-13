/**
 * Brain bridge contract types.
 * Matches internal/cortex/application/brain.go (cortexos.brain.bridge.v1)
 */
export const brainBridgeSchemaVersion = 'cortexos.brain.bridge.v1' as const

export type BrainDispatchRequest = {
  taskId: string
  userPrompt: string
  staffId?: string
  config?: BrainSessionConfig
  schemaVersion: typeof brainBridgeSchemaVersion
}

export type BrainSessionConfig = {
  timeoutSec?: number
  stepLimit?: number
  maxTokens?: number
}

export type BrainDispatchResult = {
  taskId: string
  staffId: string
  state: BrainState
  output: string
  steps: number
  tokens: number
  durationMs: number
  error: string
  schemaVersion: typeof brainBridgeSchemaVersion
}

export type BrainState =
  | 'idle'
  | 'planning'
  | 'executing'
  | 'streaming'
  | 'tool_calling'
  | 'completed'
  | 'failed'
  | 'canceled'

export type BrainStreamChunk = {
  taskId: string
  type: 'token' | 'tool_call' | 'state_change' | 'done' | 'error'
  data: string
  sequence: number
  timestamp: string
}

export type BrainStreamChunkHandler = (chunk: BrainStreamChunk) => void

export type BrainBridge = {
  DispatchBrainTask(request: BrainDispatchRequest): Promise<BrainDispatchResult>
  DispatchBrainTaskStream(request: BrainDispatchRequest): Promise<BrainDispatchResult>
}

export const BrainEventNames = {
  StreamChunk: 'cortexos:brain:stream:chunk',
  StateChanged: 'cortexos:brain:state:changed',
} as const

export type BrainEventName = (typeof BrainEventNames)[keyof typeof BrainEventNames]