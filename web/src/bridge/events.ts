import { CockpitEventNames, KnowledgeEventNames } from '../types'

export type CockpitEventName = (typeof CockpitEventNames)[keyof typeof CockpitEventNames]
export type KnowledgeEventName = (typeof KnowledgeEventNames)[keyof typeof KnowledgeEventNames]
export type CockpitEventPayload = unknown
export type BridgeEventName = CockpitEventName | KnowledgeEventName
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
  if (typeof window !== 'undefined' && window.runtime) return window.runtime
  if (typeof globalThis !== 'undefined' && (globalThis as { runtime?: WailsRuntime }).runtime) return (globalThis as { runtime?: WailsRuntime }).runtime
  return undefined
}

/** Subscribe to a typed Wails bridge event. */
export function onCockpitEvent(eventName: BridgeEventName | string, handler: CockpitEventHandler): () => void {
  const runtime = getRuntime()
  if (!runtime?.EventsOn) return () => undefined
  let active = true
  const off = runtime.EventsOn(eventName, (payload) => { if (active) handler(payload) })
  return () => {
    if (!active) return
    active = false
    if (typeof off === 'function') off()
    else runtime.EventsOff?.(eventName)
  }
}

export function onCockpitWorkspaceChanged(handler: CockpitEventHandler): () => void { return onCockpitEvent(CockpitEventNames.WorkspaceChanged, handler) }
export function onCockpitRuntimeChanged(handler: CockpitEventHandler): () => void { return onCockpitEvent(CockpitEventNames.RuntimeChanged, handler) }
