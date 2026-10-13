import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { CoordinationPanel } from './CoordinationPanel'
import { coordinationSchemaVersion, type CoordinationSnapshot } from '../types/coordination'

const mockSnapshot: CoordinationSnapshot = {
  schemaVersion: coordinationSchemaVersion,
  projectId: 'proj-alpha',
  activePhase: 1,
  totalPhases: 3,
  tasks: [
    {
      id: 'task-1',
      projectId: 'proj-alpha',
      title: 'Plan requirements',
      status: 'success',
      phase: 0,
      dependencies: [],
      isBlocked: false,
      attempt: 1,
      maxAttempts: 3,
      progress: 100,
      updatedAt: '2026-10-13T00:00:00Z',
    },
    {
      id: 'task-2',
      projectId: 'proj-alpha',
      title: 'Implement database schema',
      assignedStaffId: 'staff-dev',
      assignedStaffRole: 'implementer',
      status: 'running',
      phase: 1,
      dependencies: ['task-1'],
      isBlocked: false,
      attempt: 1,
      maxAttempts: 3,
      progress: 60,
      updatedAt: '2026-10-13T00:01:00Z',
    },
    {
      id: 'task-3',
      projectId: 'proj-alpha',
      title: 'Integrate API endpoints',
      assignedStaffId: 'staff-api',
      assignedStaffRole: 'implementer',
      status: 'ready',
      phase: 2,
      dependencies: ['task-2'],
      isBlocked: true,
      blockedReason: 'awaiting dependencies: task-2',
      attempt: 0,
      maxAttempts: 3,
      progress: 0,
      updatedAt: '2026-10-13T00:01:00Z',
    },
  ],
  assignments: [
    {
      staffId: 'staff-dev',
      name: 'Developer',
      role: 'implementer',
      availability: 'busy',
      currentTaskId: 'task-2',
      currentTaskTitle: 'Implement database schema',
      state: 'running',
      assignedTaskCount: 1,
      completedTaskCount: 0,
    },
    {
      staffId: 'staff-api',
      name: 'API Specialist',
      role: 'implementer',
      availability: 'available',
      currentTaskId: 'task-3',
      currentTaskTitle: 'Integrate API endpoints',
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
  generatedAt: '2026-10-13T00:01:00Z',
}

describe('CoordinationPanel', () => {
  it('renders execution map header with bounded phase progress', () => {
    const markup = renderToStaticMarkup(<CoordinationPanel snapshot={mockSnapshot} />)
    expect(markup).toContain('Execution map')
    expect(markup).toContain('ORCHESTRA / MULTI-STAFF')
    expect(markup).toContain('/ 3') // Total phases
  })

  it('renders aggregate metrics accurately', () => {
    const markup = renderToStaticMarkup(<CoordinationPanel snapshot={mockSnapshot} />)
    expect(markup).toContain('Running')
    expect(markup).toContain('Blocked')
    expect(markup).toContain('Done')
    expect(markup).toContain('Staff active')
    expect(markup).toContain('2/2')
  })

  it('renders Staff assignments with lifecycle state and roles', () => {
    const markup = renderToStaticMarkup(<CoordinationPanel snapshot={mockSnapshot} />)
    expect(markup).toContain('Developer')
    expect(markup).toContain('API Specialist')
    expect(markup).toContain('implementer · running')
    expect(markup).toContain('implementer · blocked')
    expect(markup).toContain('Implement database schema')
  })

  it('renders topological dependency phases with task statuses and blocked reasons', () => {
    const markup = renderToStaticMarkup(<CoordinationPanel snapshot={mockSnapshot} />)
    expect(markup).toContain('Phase 1')
    expect(markup).toContain('Phase 2')
    expect(markup).toContain('Phase 3')
    expect(markup).toContain('Plan requirements')
    expect(markup).toContain('Implement database schema')
    expect(markup).toContain('Integrate API endpoints')
    expect(markup).toContain('awaiting dependencies: task-2')
  })

  it('renders accessible empty states gracefully', () => {
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
      generatedAt: '2026-10-13T00:00:00Z',
    }
    const markup = renderToStaticMarkup(<CoordinationPanel snapshot={emptySnapshot} />)
    expect(markup).toContain('No Staff assignments in this project.')
    expect(markup).toContain('No admitted tasks.')
  })
})
