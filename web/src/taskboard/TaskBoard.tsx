import { useCallback, useEffect, useMemo, useState } from 'react'

import { loadOrchestraTasks, cancelOrchestraTask, getOrchestraTask, retryOrchestraTask } from '../bridge/orchestra'
import { onOrchestraTaskChanged } from '../bridge/orchestra-events'
import { type OrchestraBridge, type OrchestraTaskDetail, type OrchestraTaskStatus, type OrchestraTaskSummary } from '../types/orchestra'

export type TaskBoardCommands = {
  /** Reassignment remains a bridge seam; Staff/Orchestra retain authority. */
  onReassign?: (task: OrchestraTaskSummary) => void
  /** Inspection is a read-only request to the Inspector-owned detail boundary. */
  onInspect?: (task: OrchestraTaskSummary, detail: OrchestraTaskDetail) => void
}

type Filter = 'all' | OrchestraTaskStatus

const columns: Array<{ status: OrchestraTaskStatus; label: string; description: string }> = [
  { status: 'draft', label: 'Draft', description: 'Needs validation' },
  { status: 'ready', label: 'Ready', description: 'Eligible for dispatch' },
  { status: 'running', label: 'Running', description: 'Execution in flight' },
  { status: 'awaitingInspection', label: 'Inspection', description: 'Evidence awaiting review' },
  { status: 'success', label: 'Success', description: 'Accepted by Inspector' },
  { status: 'failed', label: 'Failed', description: 'Needs a governed retry' },
  { status: 'canceled', label: 'Canceled', description: 'Terminal by command' },
]

export function TaskBoard({ bridge, commands = {} }: { bridge: OrchestraBridge; commands?: TaskBoardCommands }) {
  const [state, setState] = useState<Awaited<ReturnType<typeof loadOrchestraTasks>>>({ status: 'loading' })
  const [query, setQuery] = useState('')
  const [filter, setFilter] = useState<Filter>('all')
  const [selected, setSelected] = useState<OrchestraTaskSummary | null>(null)
  const [detail, setDetail] = useState<OrchestraTaskDetail | null>(null)
  const [commandError, setCommandError] = useState('')
  const [commandTask, setCommandTask] = useState<string | null>(null)

  const refresh = useCallback(() => {
    setState({ status: 'loading' })
    void loadOrchestraTasks(bridge).then(setState)
  }, [bridge])

  useEffect(() => {
    void loadOrchestraTasks(bridge).then(setState)
    return onOrchestraTaskChanged(refresh)
  }, [bridge, refresh])

  const tasks = useMemo(() => state.status === 'success' ? state.tasks : [], [state])
  const visibleTasks = useMemo(() => tasks.filter((task) => {
    const matchesFilter = filter === 'all' || task.status === filter
    const needle = query.trim().toLowerCase()
    return matchesFilter && (!needle || task.id.toLowerCase().includes(needle) || task.projectId.toLowerCase().includes(needle))
  }), [filter, query, tasks])

  const inspect = async (task: OrchestraTaskSummary) => {
    setSelected(task)
    setCommandError('')
    try {
      const nextDetail = await getOrchestraTask(bridge, task.id)
      setDetail(nextDetail)
      commands.onInspect?.(task, nextDetail)
    } catch (error) {
      setCommandError(error instanceof Error ? error.message : 'Inspection request failed.')
    }
  }

  const command = async (task: OrchestraTaskSummary, operation: 'cancel' | 'retry') => {
    setCommandTask(task.id)
    setCommandError('')
    try {
      if (operation === 'cancel') await cancelOrchestraTask(bridge, task.id)
      else await retryOrchestraTask(bridge, task.id)
      refresh()
    } catch (error) {
      setCommandError(error instanceof Error ? error.message : 'Orchestra command failed.')
    } finally { setCommandTask(null) }
  }

  return <section className="task-board reveal-1" aria-labelledby="task-board-title">
    <div className="task-board-heading">
      <div><span className="eyebrow">CORTEXOS / ORCHESTRA</span><h1 id="task-board-title">Task board<br /><i>facts in motion.</i></h1><p className="lede">A read-only view of Orchestra lifecycle state. Inspector and Orchestra remain the only authorities for transitions.</p></div>
      <div className="task-board-contract"><span>CONTRACT</span><strong>orchestra.bridge.v1</strong><small>typed boundary</small></div>
    </div>
    <div className="task-board-toolbar" role="search">
      <label className="task-search"><span className="sr-only">Search tasks</span><input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search task or project…" /></label>
      <label className="task-filter"><span className="sr-only">Filter task status</span><select value={filter} onChange={(event) => setFilter(event.target.value as Filter)}><option value="all">All states</option>{columns.map((column) => <option value={column.status} key={column.status}>{column.label}</option>)}</select></label>
      <button className="task-refresh" type="button" onClick={refresh} disabled={state.status === 'loading'}>{state.status === 'loading' ? 'Reading…' : 'Refresh'} ↻</button>
    </div>
    {commandError && <div className="task-error" role="alert">{commandError}</div>}
    {state.status === 'error' && <div className="task-empty" role="alert"><strong>{state.code}</strong><p>{state.message}</p><button type="button" onClick={refresh}>Retry</button></div>}
    {state.status === 'loading' && <div className="task-loading" aria-live="polite">Reading Orchestra task state…</div>}
    {state.status === 'success' && tasks.length === 0 && <div className="task-empty"><span className="empty-glyph" aria-hidden="true">∅</span><div><strong>No tasks in Orchestra</strong><p>New governed work will appear here when it is admitted.</p></div></div>}
    {state.status === 'success' && tasks.length > 0 && visibleTasks.length === 0 && <div className="task-empty"><strong>No matching tasks</strong><p>Try a different status or search term.</p></div>}
    {state.status === 'success' && visibleTasks.length > 0 && <div className="kanban" aria-label="Task status columns">{columns.map((column) => <TaskColumn key={column.status} column={column} tasks={visibleTasks.filter((task) => task.status === column.status)} selectedId={selected?.id} commandTask={commandTask} onInspect={inspect} onCancel={(task) => void command(task, 'cancel')} onRetry={(task) => void command(task, 'retry')} onReassign={commands.onReassign} />)}</div>}
    {selected && <TaskDetail task={selected} detail={detail} onClose={() => { setSelected(null); setDetail(null) }} />}
  </section>
}

export function TaskColumn({ column, tasks, selectedId, commandTask, onInspect, onCancel, onRetry, onReassign }: { column: typeof columns[number]; tasks: OrchestraTaskSummary[]; selectedId?: string; commandTask: string | null; onInspect: (task: OrchestraTaskSummary) => void; onCancel: (task: OrchestraTaskSummary) => void; onRetry: (task: OrchestraTaskSummary) => void; onReassign?: (task: OrchestraTaskSummary) => void }) {
  return <section className={`kanban-column status-${column.status}`} aria-labelledby={`column-${column.status}`}><header><div><h2 id={`column-${column.status}`}>{column.label}</h2><p>{column.description}</p></div><strong>{tasks.length}</strong></header><div className="kanban-cards">{tasks.length === 0 ? <p className="column-empty">No tasks</p> : tasks.map((task) => <TaskCard key={task.id} task={task} selected={task.id === selectedId} busy={task.id === commandTask} onInspect={onInspect} onCancel={onCancel} onRetry={onRetry} onReassign={onReassign} />)}</div></section>
}

export function TaskCard({ task, selected, busy, onInspect, onCancel, onRetry, onReassign }: { task: OrchestraTaskSummary; selected: boolean; busy: boolean; onInspect: (task: OrchestraTaskSummary) => void; onCancel: (task: OrchestraTaskSummary) => void; onRetry: (task: OrchestraTaskSummary) => void; onReassign?: (task: OrchestraTaskSummary) => void }) {
  const terminal = task.status === 'success' || task.status === 'canceled'
  return <article className={`task-card${selected ? ' selected' : ''}`}><div className="task-card-top"><span className="task-status-dot" aria-hidden="true" /><code>{task.id}</code></div><strong className="task-project">{task.projectId}</strong><div className="task-progress"><span style={{ width: `${task.progress}%` }} /><small>{task.progress}% · attempt {task.attempt}/{task.maxAttempts}</small></div><div className="task-card-actions"><button type="button" onClick={() => onInspect(task)}>Inspect</button>{task.status === 'failed' && <button type="button" onClick={() => onRetry(task)} disabled={busy}>Retry</button>}{!terminal && task.status !== 'failed' && <button type="button" onClick={() => onCancel(task)} disabled={busy}>Cancel</button>}{onReassign && !terminal && <button type="button" onClick={() => onReassign(task)} disabled={busy}>Reassign</button>}</div></article>
}

export function TaskDetail({ task, detail, onClose }: { task: OrchestraTaskSummary; detail: OrchestraTaskDetail | null; onClose: () => void }) {
  return <aside className="task-detail" aria-label={`Inspection detail for ${task.id}`}><div className="task-detail-heading"><div><span className="eyebrow">INSPECTOR READOUT</span><h2>{task.id}</h2></div><button type="button" onClick={onClose} aria-label="Close task detail">×</button></div>{!detail ? <p className="task-loading">Loading bounded detail…</p> : <><dl><div><dt>State</dt><dd>{detail.status}</dd></div><div><dt>Evidence</dt><dd>{detail.evidence.length} references</dd></div><div><dt>Acceptance</dt><dd>{detail.acceptanceCount} criteria</dd></div><div><dt>Dependencies</dt><dd>{detail.dependencyCount}</dd></div></dl><h3>Timeline</h3><ol className="task-timeline">{detail.timeline.map((event) => <li key={event.sequence}><strong>{event.type}</strong><span>{event.from ?? '—'} → {event.to ?? '—'}</span></li>)}</ol></>}</aside>
}
