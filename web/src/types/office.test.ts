import { describe, expect, it } from 'vitest'
import {
  officeSchemaVersion,
  staffBridgeSchemaVersion,
  isStaffSummary,
  isOfficeSnapshot,
  type StaffSummary,
  type OfficeSnapshot,
} from './office'

describe('Office types and type guards', () => {
  const sampleStaff: StaffSummary = {
    id: 'staff-lead-1',
    name: 'Ada Lovelace',
    role: 'coordinator',
    capabilities: ['orchestration', 'governance'],
    workspaceId: 'ws-main',
    projectId: 'cortexos-core',
    lifecycle: 'active',
    availability: 'available',
    schemaVersion: staffBridgeSchemaVersion,
  }

  const sampleSnapshot: OfficeSnapshot = {
    schemaVersion: officeSchemaVersion,
    staff: [sampleStaff],
    activity: [
      {
        id: 'act-1',
        staffId: 'staff-lead-1',
        kind: 'status',
        label: 'Ready for coordination',
        timestamp: new Date().toISOString(),
        severity: 'info',
      },
    ],
    terminal: {
      visible: false,
      lines: [],
    },
  }

  it('validates correct staff schema version and constants', () => {
    expect(officeSchemaVersion).toBe('cortexos.office.v1')
    expect(staffBridgeSchemaVersion).toBe('cortexos.staff.bridge.v1')
  })

  it('identifies valid StaffSummary', () => {
    expect(isStaffSummary(sampleStaff)).toBe(true)
  })

  it('rejects invalid StaffSummary', () => {
    expect(isStaffSummary(null)).toBe(false)
    expect(isStaffSummary({})).toBe(false)
    expect(isStaffSummary({ id: '123' })).toBe(false)
    expect(isStaffSummary({ ...sampleStaff, role: 123 as unknown as string })).toBe(false)
  })

  it('identifies valid OfficeSnapshot', () => {
    expect(isOfficeSnapshot(sampleSnapshot)).toBe(true)
  })

  it('rejects invalid OfficeSnapshot', () => {
    expect(isOfficeSnapshot(null)).toBe(false)
    expect(isOfficeSnapshot({ schemaVersion: 'cortexos.office.v0' })).toBe(false)
    expect(isOfficeSnapshot({ ...sampleSnapshot, staff: [{ invalid: true }] })).toBe(false)
  })
})
