import { type StaffSummary, type StaffStatus } from './types'
import type { WailsWindow } from '../types'

const staffBridgeSchemaVersion = 'cortexos.staff.bridge.v1'
type BridgeError = { message?: string }
type StaffBridge = NonNullable<NonNullable<WailsWindow['go']>['platform']>['Bridge'] & {
  GetStaffSummary(request: { schemaVersion: string; staffId: string }): Promise<StaffSummary>
  ListStaffCapabilities(request: { schemaVersion: string }): Promise<Array<{ capability: string }>>
  ListStaffByWorkspace(request: { schemaVersion: string; workspaceId: string }): Promise<StaffSummary[]>
  GetStaffAssignment(request: { schemaVersion: string; staffId: string }): Promise<{ staffId: string; workspaceId: string; projectId?: string; worktreeId?: string; schemaVersion: string }>
}

export type StaffAPI = {
  GetStaffSummary: (id: string) => Promise<{ status: StaffStatus; summary?: StaffSummary; error?: string }>
  ListStaffSummaries: () => Promise<{ status: StaffStatus; staff: StaffSummary[]; error?: string }>
  ListStaffCapabilities: () => Promise<{ status: StaffStatus; capabilities: string[]; error?: string }>
  ListStaffByWorkspace: (workspaceId: string) => Promise<{ status: StaffStatus; staff: StaffSummary[]; error?: string }>
  GetStaffAssignment: (staffId: string) => Promise<{ status: StaffStatus; assignment?: { staffId: string; workspaceId: string; projectId?: string; worktreeId?: string; schemaVersion: string }; error?: string }>
}

export type StaffBridgeAPI = { getAPI: () => StaffAPI }

function errorMessage(error: unknown, fallback: string) {
  return (error as BridgeError)?.message || fallback
}

export function createStaffBridgeAPI(): StaffAPI {
  const bridge = (typeof window !== 'undefined' ? (window as WailsWindow).go?.platform?.Bridge : undefined) as StaffBridge | undefined
  const api: StaffAPI = {
    GetStaffSummary: async (id) => {
      if (!bridge) return { status: 'error', error: 'Staff bridge unavailable' }
      try { return { status: 'success', summary: await bridge.GetStaffSummary({ schemaVersion: staffBridgeSchemaVersion, staffId: id }) } }
      catch (error: unknown) { return { status: 'error', error: errorMessage(error, 'Failed to get staff summary') } }
    },
    ListStaffSummaries: async () => {
      if (!bridge) return { status: 'error', error: 'Staff bridge unavailable', staff: [] }
      try { return { status: 'success', staff: await bridge.ListStaffSummaries({ schemaVersion: staffBridgeSchemaVersion }) as StaffSummary[] } }
      catch (error: unknown) { return { status: 'error', error: errorMessage(error, 'Failed to list staff'), staff: [] } }
    },
    ListStaffCapabilities: async () => {
      if (!bridge) return { status: 'error', error: 'Staff bridge unavailable', capabilities: [] }
      try {
        const response = await bridge.ListStaffCapabilities({ schemaVersion: staffBridgeSchemaVersion })
        return { status: 'success', capabilities: response.map((item: { capability: string }) => item.capability) }
      } catch (error: unknown) { return { status: 'error', error: errorMessage(error, 'Failed to list capabilities'), capabilities: [] } }
    },
    ListStaffByWorkspace: async (workspaceId) => {
      if (!bridge) return { status: 'error', error: 'Staff bridge unavailable', staff: [] }
      try { return { status: 'success', staff: await bridge.ListStaffByWorkspace({ schemaVersion: staffBridgeSchemaVersion, workspaceId }) as StaffSummary[] } }
      catch (error: unknown) { return { status: 'error', error: errorMessage(error, 'Failed to list staff by workspace'), staff: [] } }
    },
    GetStaffAssignment: async (staffId) => {
      if (!bridge) return { status: 'error', error: 'Staff bridge unavailable' }
      try { return { status: 'success', assignment: await bridge.GetStaffAssignment({ schemaVersion: staffBridgeSchemaVersion, staffId }) } }
      catch (error: unknown) { return { status: 'error', error: errorMessage(error, 'Failed to get staff assignment') } }
    },
  }
  return api
}