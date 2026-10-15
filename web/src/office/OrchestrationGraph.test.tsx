import { describe, expect, it } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { OrchestrationGraph } from './OrchestrationGraph'
import { coordinationSchemaVersion, type CoordinationSnapshot } from '../types/coordination'

const emptySnapshot: CoordinationSnapshot = {
  schemaVersion: coordinationSchemaVersion,
  projectId: 'proj-omega',
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

const activeSnapshot: CoordinationSnapshot = {
  schemaVersion: coordinationSchemaVersion,
  projectId: 'proj-omega',
  activePhase: 1,
  totalPhases: 2,
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
      assignedStaffId: 'implementer-1',
      assignedStaffRole: 'implementer',
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
  assignments: [],
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
    totalStaffCount: 1,
  },
  generatedAt: '2026-10-14T00:05:00Z',
}

describe('OrchestrationGraph', () => {
  it('renders single idle Orchestra node when no work is active', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={emptySnapshot} />)
    expect(markup).toContain('GRAPH · 1 NODES · 0 EDGES')
    expect(markup).toContain('IDLE')
    expect(markup).toContain('Orchestra')
    expect(markup).toContain('NO ACTIVE WORK')
    expect(markup).not.toContain('Lead Coordinator')
    expect(markup).not.toContain('Staff Planner')
  })

  it('renders Orchestra root connected to active task nodes', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={activeSnapshot} />)
    expect(markup).toContain('GRAPH · 2 NODES · 1 EDGES')
    expect(markup).toContain('LIVE EXECUTION')
    expect(markup).toContain('Build authority chain adapter')
    expect(markup).toContain('implementer-1')
    expect(markup).toContain('RUNNING')
    expect(markup).toContain('DISPATCH')
  })

  it('renders execution lane labels L0 Orchestra and execution levels', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={activeSnapshot} />)
    expect(markup).toContain('L0 · ORCHESTRA')
    expect(markup).toContain('L2 · EXECUTION')
  })

  it('renders bottom legend with zone swatches and animation indicators', () => {
    const markup = renderToStaticMarkup(<OrchestrationGraph snapshot={activeSnapshot} />)
    expect(markup).toContain('ZONES')
    expect(markup).toContain('Coordination')
    expect(markup).toContain('Planning')
    expect(markup).toContain('Delivery')
    expect(markup).toContain('Review')
    expect(markup).toContain('Heartbeat = node alive')
    expect(markup).toContain('Dot = task dispatch')
  })
})
