import { CockpitEventNames } from '../types'

export type CockpitEventName = (typeof CockpitEventNames)[keyof typeof CockpitEventNames]
export type CockpitEventPayload = unknown
export type CockpitEventHandler = (payload: CockpitEventPayload) => void

type WailsRuntime = {
  EventsOn?: (eventName: string, callback: (payload: unknown) => void) => (() => void) | void
  EventsOff?: (eventName: string) => void
}

declare global {
  interface Window {
    runtime?: WailsRuntime
  }
}

function getRuntime(): WailsRuntime | undefined {
  if (typeof window !== 'undefined' && window.runtime) {
    return window.runtime
  }
  if (typeof globalThis !== 'undefined' && (globalThis as { runtime?: WailsRuntime }).runtime) {
    return (globalThis as { runtime?: WailsRuntime }).runtime
  }
  return undefined
}

/**
 * Subscribe to Wails Cockpit runtime events. Returns an idempotent unsubscribe
 * function, or a no-op when running outside the Wails desktop shell.
 */
export function onCockpitEvent(
  eventName: CockpitEventName,
  handler: CockpitEventHandler,
): () => void {
  const runtime = getRuntime()
  if (!runtime?.EventsOn) return () => undefined

  let active = true
  const off = runtime.EventsOn(eventName, (payload) => {
    if (active) handler(payload)
  })
  return () => {
    if (!active) return
    active = false
    if (typeof off === 'function') {
      off()
    } else {
      runtime.EventsOff?.(eventName)
    }
  }
}

export function onCockpitWorkspaceChanged(handler: CockpitEventHandler): () => void {
  return onCockpitEvent(CockpitEventNames.WorkspaceChanged, handler)
}

export function onCockpitRuntimeChanged(handler: CockpitEventHandler): () => void {
  return onCockpitEvent(CockpitEventNames.RuntimeChanged, handler)
}
