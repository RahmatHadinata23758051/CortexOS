import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { OrchestrationGraph } from './OrchestrationGraph'
import { coordinationSchemaVersion, type CoordinationSnapshot } from '../types/coordination'
import type { StaffSummary } from '../types/office'

const mockSnapshot: CoordinationSnapshot = {
  schemaVersion: coordinationSchemaVersion,
  projectId: 'proj-omega',
  activePhase: 1,
  totalPhases: 3,
  tasks: [
    {
      id: 'task-101',
      projectId: 'proj-omega',
      title: 'Architect system boundaries',
      status: 'success',
      phase: 0,
      dependencies: [],
      isBlocked: false,
      attempt: 1,
      maxAttempts: 3,
      progress: 100,
      updatedAt: '2026-10-14T00:00:00Z',
    },
    {
      id: 'task-102',
      projectId: 'proj-omega',
      title: 'Build authority chain adapter',
      assignedStaffId: 'staff-lead',
      assignedStaffRole: 'coordinator',
      status: 'running',
      phase: 1,
      dependencies: ['task-101'],
      isBlocked: false,
      attempt: 1,
      maxAttempts: 3,
      progress: 55,
      updatedAt: '2026-10-14T00:01:00Z',
    },
  ],
  assignments: [
    {
      staffId: 'staff-lead',
      name: 'Lead Coordinator',
      role: 'coordinator',
      availability: 'busy',
      currentTaskId: 'task-102',
      currentTaskTitle: 'Build authority chain adapter',
      state: 'running',
      assignedTaskCount: 1,
      completedTaskCount: 0,
    },
  ],
  metrics: {
    totalTasks: 2,
    readyTasks: 0,
    runningTasks: 1,
    blockedTasks: 0,
    awaitingInspection: 0,
    completedTasks: 1,
    failedTasks: 0,
    canceledTasks: 0,
    activeStaffCount: 1,
    totalStaffCount: 4,
  },
  generatedAt: '2026-10-14T00:05:00Z',
}

const customStaff: StaffSummary[] = [
  {
    id: 'staff-coord-01',
    name: 'Sarah Connor',
    role: 'coordinator',
    workspaceId: 'ws-main',
    capabilities: ['orchestration'],
    lifecycle: 'active',
    availability: 'available',
    profile: 'ops-lead',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
  {
    id: 'staff-plan-01',
    name: 'Kyle Reese',
    role: 'planner',
    workspaceId: 'ws-main',
    capabilities: ['planning'],
    lifecycle: 'active',
    availability: 'busy',
    parentId: 'staff-coord-01',
    profile: 'tactical',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
  {
    id: 'staff-impl-01',
    name: 'T-800',
    role: 'implementer',
    workspaceId: 'ws-main',
    capabilities: ['execution'],
    lifecycle: 'active',
    availability: 'available',
    parentId: 'staff-plan-01',
    profile: 'heavy-worker',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
]

const seedStaff: StaffSummary[] = [
  {
    id: 'staff-lead',
    name: 'Lead Coordinator',
    role: 'coordinator',
    capabilities: ['coordination'],
    workspaceId: 'default',
    lifecycle: 'active',
    availability: 'available',
    profile: 'default',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
  {
    id: 'staff-planner',
    name: 'Staff Planner',
    role: 'planner',
    capabilities: ['planning'],
    workspaceId: 'default',
    lifecycle: 'active',
    availability: 'available',
    parentId: 'staff-lead',
    profile: 'default',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
  {
    id: 'staff-reviewer',
    name: 'Staff Reviewer',
    role: 'reviewer',
    capabilities: ['review'],
    workspaceId: 'default',
    lifecycle: 'active',
    availability: 'available',
    parentId: 'staff-lead',
    profile: 'default',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
  {
    id: 'staff-core',
    name: 'Core Implementer',
    role: 'implementer',
    capabilities: ['delivery'],
    workspaceId: 'default',
    lifecycle: 'active',
    availability: 'available',
    parentId: 'staff-planner',
    profile: 'default',
    schemaVersion: 'cortexos.staff.bridge.v1',
  },
]

describe('OrchestrationGraph', () => {
  it('renders top-down agent hierarchy toolbar with node and edge counts', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={seedStaff} />)
    expect(markup).toContain('GRAPH · 4 NODES · 4 EDGES')
    expect(markup).toContain('aria-label="Orchestration hierarchy graph"')
  })

  it('renders hierarchy lane labels from L0 down to workers', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={seedStaff} />)
    expect(markup).toContain('L0 · COORDINATOR')
    expect(markup).toContain('L1 · LEADS')
    expect(markup).toContain('L2 · WORKERS')
  })

  it('renders canonical agent node cards with roles, avatars, profiles, and zones', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={seedStaff} />)
    expect(markup).toContain('Lead Coordinator')
    expect(markup).toContain('COORDINATOR · default')
    expect(markup).toContain('COORDINATION')

    expect(markup).toContain('Staff Planner')
    expect(markup).toContain('PLANNER · default')
    expect(markup).toContain('PLANNING')

    expect(markup).toContain('Staff Reviewer')
    expect(markup).toContain('REVIEWER · default')
    expect(markup).toContain('REVIEW')

    expect(markup).toContain('Core Implementer')
    expect(markup).toContain('IMPLEMENTER · default')
    expect(markup).toContain('DELIVERY')
  })

  it('renders SVG connectors with orthogonal paths, markers, edge labels, and dispatch animations', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={seedStaff} />)
    expect(markup).toContain('<svg class="hierarchy-edges"')
    expect(markup).toContain('id="hierarchy-arrow"')
    expect(markup).toContain('id="hierarchy-arrow-violet"')
    expect(markup).toContain('DELEGATES')
    expect(markup).toContain('ASSIGNS WORK')
    expect(markup).toContain('REVIEW HANDOFF')
    expect(markup).toContain('hierarchy-edge-handoff')
    expect(markup).toContain('<animateMotion')
  })

  it('renders bottom legend with zone swatches and animation indicators', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={seedStaff} />)
    expect(markup).toContain('ZONES')
    expect(markup).toContain('Coordination')
    expect(markup).toContain('Planning')
    expect(markup).toContain('Delivery')
    expect(markup).toContain('Review')
    expect(markup).toContain('Heartbeat = node alive')
    expect(markup).toContain('Dot = task dispatch')
    expect(markup).toContain('Review handoff')
  })

  it('renders custom staff hierarchies with dynamic BFS depth and profiles', () => {
    const markup = renderToStaticMarkup(
      <OrchestrationGraph snapshot={mockSnapshot} staff={customStaff} />
    )
    expect(markup).toContain('GRAPH · 3 NODES · 2 EDGES')
    expect(markup).toContain('Sarah Connor')
    expect(markup).toContain('COORDINATOR · ops-lead')
    expect(markup).toContain('Kyle Reese')
    expect(markup).toContain('PLANNER · tactical')
    expect(markup).toContain('T-800')
    expect(markup).toContain('IMPLEMENTER · heavy-worker')
  })

  it('reflects busy status from live assignment state', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={seedStaff} />)
    expect(markup).toContain('hierarchy-node-busy')
    expect(markup).toContain('BUSY')
  })

  it('renders empty message when staff list is empty', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} staff={[]} />)
    expect(markup).toContain('No staff hierarchy is available')
  })
})
