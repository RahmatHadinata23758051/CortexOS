import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { OrchestrationGraph } from './OrchestrationGraph'
import { coordinationSchemaVersion, type CoordinationSnapshot } from '../types/coordination'

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
    {
      id: 'task-103',
      projectId: 'proj-omega',
      title: 'Verify approval gates',
      assignedStaffId: 'staff-sec',
      assignedStaffRole: 'reviewer',
      status: 'ready',
      phase: 2,
      dependencies: ['task-102'],
      isBlocked: true,
      blockedReason: 'Awaiting task-102 completion',
      attempt: 0,
      maxAttempts: 3,
      progress: 0,
      updatedAt: '2026-10-14T00:02:00Z',
    },
  ],
  assignments: [
    {
      staffId: 'staff-lead',
      name: 'Sarah Connor',
      role: 'coordinator',
      availability: 'busy',
      currentTaskId: 'task-102',
      currentTaskTitle: 'Build authority chain adapter',
      state: 'running',
      assignedTaskCount: 1,
      completedTaskCount: 0,
    },
    {
      staffId: 'staff-sec',
      name: 'John Matrix',
      role: 'reviewer',
      availability: 'available',
      currentTaskId: 'task-103',
      currentTaskTitle: 'Verify approval gates',
      state: 'blocked',
      assignedTaskCount: 1,
      completedTaskCount: 0,
    },
  ],
  metrics: {
    totalTasks: 3,
    readyTasks: 1,
    runningTasks: 1,
    blockedTasks: 1,
    awaitingInspection: 0,
    completedTasks: 1,
    failedTasks: 0,
    canceledTasks: 0,
    activeStaffCount: 2,
    totalStaffCount: 2,
  },
  generatedAt: '2026-10-14T00:05:00Z',
}

describe('OrchestrationGraph', () => {
  it('renders node hierarchy toolbar and phase columns', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} />)
    expect(markup).toContain('NODE HIERARCHY')
    expect(markup).toContain('3</strong> tasks')
    expect(markup).toContain('1 running')
    expect(markup).toContain('1 blocked')
    expect(markup).toContain('Phase 1')
    expect(markup).toContain('Phase 2')
    expect(markup).toContain('Phase 3')
  })

  it('renders node cards with IDs, titles, and worker chips', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} />)
    expect(markup).toContain('task-101')
    expect(markup).toContain('Architect system boundaries')
    expect(markup).toContain('task-102')
    expect(markup).toContain('Build authority chain adapter')
    expect(markup).toContain('Sarah Connor')
    expect(markup).toContain('task-103')
    expect(markup).toContain('Verify approval gates')
    expect(markup).toContain('John Matrix')
  })

  it('renders SVG cable connector paths between dependent nodes', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} />)
    expect(markup).toContain('<svg')
    // Check that bezier paths exist
    expect(markup).toContain('d="M')
    // Check edge classes
    expect(markup).toContain('edge-path')
    expect(markup).toContain('edge-active')
  })

  it('renders blocked warning with custom blocked reason', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={mockSnapshot} />)
    expect(markup).toContain('Awaiting task-102 completion')
    expect(markup).toContain('is-blocked')
  })

  it('handles empty snapshot cleanly without crashing', () => {
    const emptySnapshot: CoordinationSnapshot = {
      schemaVersion: coordinationSchemaVersion,
      activePhase: 0,
      totalPhases: 0,
      tasks: [],
      assignments: [],
      metrics: {
        totalTasks: 0,
        readyTasks: 0,
        runningTasks: 0,
        blockedTasks: 0,
        awaitingInspection: 0,
        completedTasks: 0,
        failedTasks: 0,
        canceledTasks: 0,
        activeStaffCount: 0,
        totalStaffCount: 0,
      },
      generatedAt: '2026-10-14T00:00:00Z',
    }
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={emptySnapshot} />)
    expect(markup).toContain('NODE HIERARCHY')
    expect(markup).toContain('0</strong> tasks')
  })
})
