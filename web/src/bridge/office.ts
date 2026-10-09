import {
  officeSchemaVersion,
  staffBridgeSchemaVersion,
  type OfficeBridge,
  type OfficeSnapshot,
  type OfficeActivityEvent,
  type StaffSummary,
  type TerminalOverlayPort,
} from '../types/office'

export type OfficeState =
  | { status: 'loading' }
  | { status: 'success'; snapshot: OfficeSnapshot }
  | { status: 'error'; code: string; message: string }

export async function loadOfficeSnapshot(bridge: OfficeBridge): Promise<OfficeState> {
  try {
    const staff = await bridge.ListStaffSummaries({
      schemaVersion: staffBridgeSchemaVersion,
      filter: { activeOnly: false },
    })
    return {
      status: 'success',
      snapshot: {
        schemaVersion: officeSchemaVersion,
        staff,
        activity: [],
        terminal: { visible: false, lines: [] },
      },
    }
  } catch (error) {
    return {
      status: 'error',
      code: 'office.bridge',
      message: error instanceof Error ? error.message : 'Office bridge failed',
    }
  }
}

export function createOfficeEventSource(initial: readonly OfficeActivityEvent[] = []) {
  let listeners: Array<(event: OfficeActivityEvent) => void> = []
  const history = [...initial]
  return {
    history,
    subscribe(listener: (event: OfficeActivityEvent) => void) {
      listeners = [...listeners, listener]
      return () => { listeners = listeners.filter((candidate) => candidate !== listener) }
    },
    publish(event: OfficeActivityEvent) {
      history.push(event)
      listeners.forEach((listener) => listener(event))
    },
  }
}

export function createTerminalOverlayPort(
  setState: (state: { visible: boolean; staffId?: string; title?: string; lines: readonly string[] }) => void,
): TerminalOverlayPort {
  return {
    open(staffId) {
      setState({ visible: true, staffId, title: 'Staff terminal', lines: ['Read-only activity stream connected.'] })
    },
    close() {
      setState({ visible: false, lines: [] })
    },
  }
}

export function staffById(staff: readonly StaffSummary[], staffId: string) {
  return staff.find((member) => member.id === staffId)
}
