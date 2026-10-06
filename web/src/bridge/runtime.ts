import { getRuntimeSnapshot, type RuntimeBridge, type RuntimeSnapshot } from '../types'

export type RuntimeState =
  | { status: 'loading' }
  | { status: 'success'; snapshot: RuntimeSnapshot }
  | { status: 'error'; message: string }

export async function loadRuntimeSnapshot(
  bridge: RuntimeBridge,
): Promise<RuntimeState> {
  try {
    return {
      status: 'success',
      snapshot: await getRuntimeSnapshot(bridge),
    }
  } catch (error) {
    return {
      status: 'error',
      message: error instanceof Error ? error.message : 'Runtime bridge failed',
    }
  }
}
