import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import { TaskBoard, TaskColumn, TaskCard, TaskDetail } from './TaskBoard'
import {
  orchestraSchemaVersion,
  type OrchestraBridge,
  type OrchestraTaskDetail,
  type OrchestraTaskStatus,
  type OrchestraTaskSummary,
} from '../types/orchestra'

const sampleTasks: OrchestraTaskSummary[] = [
  {
    id: 'task-draft-1',
    projectId: 'proj-omega',
    status: 'draft',
    attempt: 0,
    maxAttempts: 2,
    progress: 0,
    schemaVersion: orchestraSchemaVersion,
  },
  {
    id: 'task-ready-1',
    projectId: 'proj-omega',
    status: 'ready',
    attempt: 0,
    maxAttempts: 2,
    progress: 20,
    schemaVersion: orchestraSchemaVersion,
  },
  {
    id: 'task-run-1',
    projectId: 'proj-omega',
    status: 'running',
    attempt: 1,
    maxAttempts: 2,
    progress: 50,
    schemaVersion: orchestraSchemaVersion,
  },
  {
    id: 'task-inspect-1',
    projectId: 'proj-alpha',
    status: 'awaitingInspection',
    attempt: 1,
    maxAttempts: 2,
    progress: 80,
    schemaVersion: orchestraSchemaVersion,
  },
  {
    id: 'task-done-1',
    projectId: 'proj-alpha',
    status: 'success',
    attempt: 1,
    maxAttempts: 2,
    progress: 100,
    schemaVersion: orchestraSchemaVersion,
  },
  {
    id: 'task-fail-1',
    projectId: 'proj-beta',
    status: 'failed',
    attempt: 2,
    maxAttempts: 2,
    progress: 100,
    schemaVersion: orchestraSchemaVersion,
  },
  {
    id: 'task-cancel-1',
    projectId: 'proj-gamma',
    status: 'canceled',
    attempt: 1,
    maxAttempts: 2,
    progress: 100,
    schemaVersion: orchestraSchemaVersion,
  },
]

const sampleDetail: OrchestraTaskDetail = {
  ...sampleTasks[3],
  acceptanceCount: 3,
  dependencyCount: 1,
  evidence: [
    { id: 'ev-test-1', count: 2 },
    { id: 'ev-test-2', count: 1 },
  ],
  timeline: [
    {
      sequence: 1,
      type: 'task.dispatched',
      from: 'ready',
      to: 'running',
      occurredAt: '2026-10-10T00:00:00Z',
    },
    {
      sequence: 2,
      type: 'evidence.collected',
      from: 'running',
      to: 'awaitingInspection',
      occurredAt: '2026-10-10T00:01:00Z',
      evidenceIds: ['ev-test-1', 'ev-test-2'],
    },
  ],
}

const mockBridge = (tasks: OrchestraTaskSummary[] = sampleTasks): OrchestraBridge => ({
  ListOrchestraTasks: vi.fn().mockResolvedValue(tasks),
  GetOrchestraTask: vi.fn().mockResolvedValue(sampleDetail),
  CancelOrchestraTask: vi.fn().mockResolvedValue(undefined),
  RetryOrchestraTask: vi.fn().mockResolvedValue(undefined),
})

const columns: Array<{ status: OrchestraTaskStatus; label: string; description: string }> = [
  { status: 'draft', label: 'Draft', description: 'Needs validation' },
  { status: 'ready', label: 'Ready', description: 'Eligible for dispatch' },
  { status: 'running', label: 'Running', description: 'Execution in flight' },
  { status: 'awaitingInspection', label: 'Inspection', description: 'Evidence awaiting review' },
  { status: 'success', label: 'Success', description: 'Accepted by Inspector' },
  { status: 'failed', label: 'Failed', description: 'Needs a governed retry' },
  { status: 'canceled', label: 'Canceled', description: 'Terminal by command' },
]

describe('TaskBoard Kanban view', () => {
  it('renders initial loading state and heading contract stamp', () => {
    const bridge = mockBridge()
    const markup = renderToStaticMarkup(<TaskBoard bridge={bridge} />)
    expect(markup).toContain('Task board')
    expect(markup).toContain('orchestra.bridge.v1')
    expect(markup).toContain('Reading Orchestra task state…')
  })

  it('declares the search bar and filter controls', () => {
    const bridge = mockBridge()
    const markup = renderToStaticMarkup(<TaskBoard bridge={bridge} />)
    expect(markup).toContain('Search task or project…')
    expect(markup).toContain('All states')
    expect(markup).toContain('Draft')
    expect(markup).toContain('Ready')
    expect(markup).toContain('Running')
    expect(markup).toContain('Inspection')
    expect(markup).toContain('Success')
    expect(markup).toContain('Failed')
    expect(markup).toContain('Canceled')
  })

  it('renders with reassign and inspect seams', () => {
    const bridge = mockBridge()
    const reassignFn = vi.fn()
    const inspectFn = vi.fn()
    const markup = renderToStaticMarkup(
      <TaskBoard
        bridge={bridge}
        commands={{ onReassign: reassignFn, onInspect: inspectFn }}
      />,
    )
    expect(markup).toContain('orchestra.bridge.v1')
  })
})

describe('TaskColumn subcomponent', () => {
  it('renders column header with label, description, and task count', () => {
    const column = columns[1] // ready
    const tasks = sampleTasks.filter((t) => t.status === 'ready')
    const markup = renderToStaticMarkup(
      <TaskColumn
        column={column}
        tasks={tasks}
        selectedId={undefined}
        commandTask={null}
        onInspect={() => {}}
        onCancel={() => {}}
        onRetry={() => {}}
      />,
    )
    expect(markup).toContain('Ready')
    expect(markup).toContain('Eligible for dispatch')
    expect(markup).toContain(tasks.length.toString())
  })

  it('renders empty state message when no tasks in column', () => {
    const column = columns[0] // draft
    const markup = renderToStaticMarkup(
      <TaskColumn
        column={column}
        tasks={[]}
        selectedId={undefined}
        commandTask={null}
        onInspect={() => {}}
        onCancel={() => {}}
        onRetry={() => {}}
      />,
    )
    expect(markup).toContain('No tasks')
  })
})

describe('TaskCard read-only rendering', () => {
  const handlers = {
    onInspect: vi.fn(),
    onCancel: vi.fn(),
    onRetry: vi.fn(),
    onReassign: vi.fn(),
  }

  it('renders task id, project id, and progress bar', () => {
    const task = sampleTasks[2] // running
    const markup = renderToStaticMarkup(
      <TaskCard task={task} selected={false} busy={false} {...handlers} />,
    )
    expect(markup).toContain('task-run-1')
    expect(markup).toContain('proj-omega')
    expect(markup).toContain('50%')
    expect(markup).toContain('attempt 1/2')
  })

  it('shows Inspect button for every card', () => {
    const markup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[0]} selected={false} busy={false} {...handlers} />,
    )
    expect(markup).toContain('Inspect')
  })

  it('shows Retry button only for failed tasks', () => {
    const failedMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[5]} selected={false} busy={false} {...handlers} />,
    )
    expect(failedMarkup).toContain('Retry')

    const runningMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[2]} selected={false} busy={false} {...handlers} />,
    )
    expect(runningMarkup).not.toContain('Retry')
  })

  it('shows Cancel button for non-terminal non-failed tasks', () => {
    const runningMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[2]} selected={false} busy={false} {...handlers} />,
    )
    expect(runningMarkup).toContain('Cancel')

    const inspectMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[3]} selected={false} busy={false} {...handlers} />,
    )
    expect(inspectMarkup).toContain('Cancel')
  })

  it('hides Cancel button for terminal states (success, canceled)', () => {
    const successMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[4]} selected={false} busy={false} {...handlers} />,
    )
    expect(successMarkup).not.toContain('Cancel')

    const canceledMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[6]} selected={false} busy={false} {...handlers} />,
    )
    expect(canceledMarkup).not.toContain('Cancel')
  })

  it('shows Reassign button when seam provided and not terminal', () => {
    const markup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[1]} selected={false} busy={false} {...handlers} />,
    )
    expect(markup).toContain('Reassign')
  })

  it('hides Reassign button for terminal states', () => {
    const successMarkup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[4]} selected={false} busy={false} {...handlers} />,
    )
    expect(successMarkup).not.toContain('Reassign')
  })

  it('disables action buttons when busy', () => {
    const markup = renderToStaticMarkup(
      <TaskCard task={sampleTasks[2]} selected={false} busy={true} {...handlers} />,
    )
    expect(markup).toContain('disabled=""')
  })
})

describe('TaskDetail inspection panel', () => {
  it('renders loading state when detail is null', () => {
    const markup = renderToStaticMarkup(
      <TaskDetail task={sampleTasks[3]} detail={null} onClose={() => {}} />,
    )
    expect(markup).toContain('Loading bounded detail…')
  })

  it('renders detail fields and timeline when detail provided', () => {
    const markup = renderToStaticMarkup(
      <TaskDetail task={sampleTasks[3]} detail={sampleDetail} onClose={() => {}} />,
    )
    expect(markup).toContain('awaitingInspection')
    expect(markup).toContain('2 references')
    expect(markup).toContain('3 criteria')
    expect(markup).toContain('1')
    expect(markup).toContain('task.dispatched')
    expect(markup).toContain('evidence.collected')
  })

  it('shows close button with accessible label', () => {
    const markup = renderToStaticMarkup(
      <TaskDetail task={sampleTasks[3]} detail={sampleDetail} onClose={() => {}} />,
    )
    expect(markup).toContain('Close task detail')
  })
})
