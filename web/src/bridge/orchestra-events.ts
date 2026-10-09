import { OrchestraEventNames } from '../types/orchestra'

export type OrchestraEventName = (typeof OrchestraEventNames)[keyof typeof OrchestraEventNames]
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

/** Subscribe to Orchestra task changes; state refresh remains owned by the board. */
export function onOrchestraEvent(eventName: OrchestraEventName, handler: OrchestraEventHandler): () => void {
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

export function onOrchestraTaskChanged(handler: OrchestraEventHandler): () => void {
  return onOrchestraEvent(OrchestraEventNames.TaskChanged, handler)
}
