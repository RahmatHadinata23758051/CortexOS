export const officeSchemaVersion = 'cortexos.office.v1' as const
export const staffBridgeSchemaVersion = 'cortexos.staff.bridge.v1' as const

export type OfficeSnapshotRequest = {
  schemaVersion: typeof officeSchemaVersion
}

export type StaffSummary = {
  id: string
  name: string
  role: 'coordinator' | 'planner' | 'implementer' | 'reviewer' | 'specialist'
  capabilities: string[]
  workspaceId: string
  projectId?: string
  worktreeId?: string
  lifecycle: 'active' | 'inactive' | 'retired'
  availability: 'available' | 'busy' | 'unavailable' | 'offline'
  schemaVersion: string
}

export type StaffListRequest = {
  schemaVersion: typeof staffBridgeSchemaVersion
  filter?: { activeOnly?: boolean }
}

export type OfficeActivityEvent = {
  id: string
  staffId?: string
  kind: 'status' | 'task' | 'system'
  label: string
  timestamp: string
  severity: 'info' | 'success' | 'warning' | 'error'
}

export type TerminalOverlayState = {
  visible: boolean
  staffId?: string
  title?: string
  lines: readonly string[]
}

export type OfficeSnapshot = {
  schemaVersion: typeof officeSchemaVersion
  staff: readonly StaffSummary[]
  activity: readonly OfficeActivityEvent[]
  terminal: TerminalOverlayState
}

export type OfficeBridge = {
  ListStaffSummaries(request: StaffListRequest): Promise<StaffSummary[]>
}

export type OfficeEventSource = {
  subscribe(listener: (event: OfficeActivityEvent) => void): () => void
}

export type TerminalOverlayPort = {
  open(staffId: string): void
  close(): void
}

export function isStaffSummary(value: unknown): value is StaffSummary {
  if (!value || typeof value !== 'object') return false
  const summary = value as Partial<StaffSummary>
  return typeof summary.id === 'string' && typeof summary.name === 'string' &&
    typeof summary.role === 'string' && typeof summary.availability === 'string' &&
    typeof summary.lifecycle === 'string' && typeof summary.workspaceId === 'string'
}

export function isOfficeSnapshot(value: unknown): value is OfficeSnapshot {
  if (!value || typeof value !== 'object') return false
  const snapshot = value as Partial<OfficeSnapshot>
  return snapshot.schemaVersion === officeSchemaVersion &&
    Array.isArray(snapshot.staff) && snapshot.staff.every(isStaffSummary) &&
    Array.isArray(snapshot.activity) && Array.isArray(snapshot.terminal?.lines)
}
