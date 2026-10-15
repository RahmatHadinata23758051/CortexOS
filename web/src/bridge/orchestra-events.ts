import { OrchestraEventNames } from '../types/orchestra'
import { OrchestraPlanEventNames } from '../types/orchestra-plan'

export type OrchestraEventName = (typeof OrchestraEventNames)[keyof typeof OrchestraEventNames]
export type OrchestraPlanEventName = (typeof OrchestraPlanEventNames)[keyof typeof OrchestraPlanEventNames]
export type OrchestraEventHandler = (payload: unknown) => void

type WailsRuntime = {
  EventsOn?: (eventName: string, callback: (payload: unknown) => void) => (() => void) | void
  EventsOff?: (eventName: string) => void
}

function getRuntime(): WailsRuntime | undefined {
  if (typeof window !== 'undefined' && (window as Window & { runtime?: WailsRuntime }).runtime) {
    return (window as Window & { runtime?: WailsRuntime }).runtime
  }
  return (globalThis as typeof globalThis & { runtime?: WailsRuntime }).runtime
}

function subscribe(eventName: string, handler: OrchestraEventHandler): () => void {
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

/** Subscribe to Orchestra task changes; state refresh remains owned by the board. */
export function onOrchestraEvent(eventName: OrchestraEventName, handler: OrchestraEventHandler): () => void {
  return subscribe(eventName, handler)
}

/** Subscribe to plan/worker lifecycle changes; consumers refresh authoritative snapshots. */
export function onOrchestraPlanEvent(eventName: OrchestraPlanEventName, handler: OrchestraEventHandler): () => void {
  return subscribe(eventName, handler)
}

/** Subscribe to every plan/worker event with one cleanup boundary. */
export function onOrchestraPlanLifecycle(handler: OrchestraEventHandler): () => void {
  const unsubscribers = Object.values(OrchestraPlanEventNames).map((eventName) => onOrchestraPlanEvent(eventName, handler))
  return () => unsubscribers.forEach((unsubscribe) => unsubscribe())
}

export function onOrchestraTaskChanged(handler: OrchestraEventHandler): () => void {
  return onOrchestraEvent(OrchestraEventNames.TaskChanged, handler)
}
