import { onCockpitEvent } from './events'
import {
  coordinationSchemaVersion,
  isCoordinationSnapshot,
  CoordinationEventNames,
  type CoordinationBridge,
  type CoordinationEventName,
  type CoordinationEventPayload,
  type CoordinationSnapshot,
} from '../types/coordination'

export type CoordinationState =
  | { status: 'loading' }
  | { status: 'success'; snapshot: CoordinationSnapshot }
  | { status: 'error'; code: string; message: string }

export async function loadCoordinationSnapshot(
  bridge: CoordinationBridge,
  projectId?: string,
): Promise<CoordinationState> {
  try {
    const snapshot = await bridge.GetCoordinationSnapshot({
      schemaVersion: coordinationSchemaVersion,
      projectId,
    })
    if (!isCoordinationSnapshot(snapshot)) {
      return {
        status: 'error',
        code: 'coordination.invalid_contract',
        message: 'Coordination snapshot contract violation',
      }
    }
    return { status: 'success', snapshot }
  } catch (error) {
    return {
      status: 'error',
      code: 'coordination.bridge_error',
      message: error instanceof Error ? error.message : 'Coordination query failed',
    }
  }
}

/** Subscribe to snapshot invalidation/update events. */
export function onCoordinationChanged(handler: (snapshot: CoordinationSnapshot) => void): () => void {
  return onCockpitEvent(CoordinationEventNames.Changed, handler as (payload: unknown) => void)
}

/** Subscribe to a specific coordination lifecycle event. */
export function onCoordinationLifecycle(
  eventName: CoordinationEventName,
  handler: (payload: CoordinationEventPayload) => void,
): () => void {
  return onCockpitEvent(eventName, handler as (payload: unknown) => void)
}
