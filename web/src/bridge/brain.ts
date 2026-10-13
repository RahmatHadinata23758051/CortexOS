/**
 * Brain bridge wrapper.
 * Connects React to the Go platform.Bridge DispatchBrainTask / DispatchBrainTaskStream
 * methods and Wails runtime events for streaming chunks and state changes.
 */
import {
  BrainEventNames,
  brainBridgeSchemaVersion,
  type BrainBridge,
  type BrainDispatchRequest,
  type BrainDispatchResult,
  type BrainStreamChunk,
} from '../types'
import { onCockpitEvent } from './events'

export type BrainDispatchInput = {
  userPrompt: string
  taskId?: string
  staffId?: string
  stepLimit?: number
  maxTokens?: number
  timeoutSec?: number
}

export function makeBrainRequest(input: BrainDispatchInput): BrainDispatchRequest {
  return {
    taskId: input.taskId ?? `brain-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
    userPrompt: input.userPrompt,
    staffId: input.staffId,
    config: input.stepLimit || input.maxTokens || input.timeoutSec
      ? {
          stepLimit: input.stepLimit,
          maxTokens: input.maxTokens,
          timeoutSec: input.timeoutSec,
        }
      : undefined,
    schemaVersion: brainBridgeSchemaVersion,
  }
}

export async function dispatchBrainTask(
  bridge: BrainBridge,
  input: BrainDispatchInput,
): Promise<BrainDispatchResult> {
  const request = makeBrainRequest(input)
  if (!request.userPrompt.trim()) {
    throw new Error('userPrompt is required')
  }
  return bridge.DispatchBrainTask(request)
}

export async function dispatchBrainTaskStream(
  bridge: BrainBridge,
  input: BrainDispatchInput,
): Promise<BrainDispatchResult> {
  const request = makeBrainRequest(input)
  if (!request.userPrompt.trim()) {
    throw new Error('userPrompt is required')
  }
  return bridge.DispatchBrainTaskStream(request)
}

/**
 * Subscribe to streaming chunks for a specific task.
 * Chunks arrive in order via the shared Wails event bus; the taskId filter
 * drops chunks that belong to other dispatches.
 */
export function onBrainStreamForTask(
  taskId: string,
  handler: (chunk: BrainStreamChunk) => void,
): () => void {
  return onCockpitEvent(BrainEventNames.StreamChunk, (payload) => {
    if (!payload || typeof payload !== 'object') return
    const chunk = payload as BrainStreamChunk
    if (chunk.taskId !== taskId) return
    handler(chunk)
  })
}

/** Subscribe to the final state payload delivered after each dispatch. */
export function onBrainStateForTask(
  taskId: string,
  handler: (result: BrainDispatchResult) => void,
): () => void {
  return onCockpitEvent(BrainEventNames.StateChanged, (payload) => {
    if (!payload || typeof payload !== 'object') return
    const result = payload as BrainDispatchResult
    if (result.taskId !== taskId) return
    handler(result)
  })
}

export function isBrainTerminalState(state: string): boolean {
  return state === 'completed' || state === 'failed' || state === 'canceled'
}
