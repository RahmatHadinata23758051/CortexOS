import { useMemo } from 'react'
import type { CoordinationSnapshot, CoordinationTask, StaffAssignmentStatus } from '../types/coordination'

export function CoordinationPanel({ snapshot }: { snapshot: CoordinationSnapshot }) {
  const phaseGroups = useMemo(() => {
    const groups = new Map<number, CoordinationTask[]>()
    snapshot.tasks.forEach((task) => {
      const current = groups.get(task.phase) ?? []
      groups.set(task.phase, [...current, task])
    })
    return [...groups.entries()].sort(([left], [right]) => left - right)
  }, [snapshot.tasks])

  return (
    <section className="coordination-panel" aria-labelledby="coordination-title">
      <header className="coordination-header">
        <div>
          <span className="eyebrow">ORCHESTRA / MULTI-STAFF</span>
          <h3 id="coordination-title">Execution map</h3>
          <p>Assignments and dependency phases update from governed lifecycle events.</p>
        </div>
        <div className="coordination-phase" aria-label={`Phase ${snapshot.activePhase + 1} of ${snapshot.totalPhases || 1}`}>
          <strong>{snapshot.totalPhases ? snapshot.activePhase + 1 : 0}</strong>
          <span>/ {snapshot.totalPhases}</span>
        </div>
      </header>

      <div className="coordination-metrics" aria-label="Coordination metrics">
        <Metric label="Running" value={snapshot.metrics.runningTasks} />
        <Metric label="Blocked" value={snapshot.metrics.blockedTasks} tone={snapshot.metrics.blockedTasks > 0 ? 'warning' : undefined} />
        <Metric label="Done" value={snapshot.metrics.completedTasks} tone="success" />
        <Metric label="Staff active" value={`${snapshot.metrics.activeStaffCount}/${snapshot.metrics.totalStaffCount}`} />
      </div>

      {snapshot.assignments.length === 0 ? (
        <div className="coordination-empty" role="status">No Staff assignments in this project.</div>
      ) : (
        <div className="coordination-staff" aria-label="Staff assignments">
          {snapshot.assignments.map((assignment) => <AssignmentCard key={assignment.staffId} assignment={assignment} />)}
        </div>
      )}

      <div className="coordination-phases" aria-label="Dependency phases">
        {phaseGroups.length === 0 ? <p className="coordination-empty">No admitted tasks.</p> : phaseGroups.map(([phase, tasks]) => (
          <section className="coordination-phase-group" key={phase}>
            <header><span>Phase {phase + 1}</span><small>{tasks.length} task{tasks.length === 1 ? '' : 's'}</small></header>
            <ul>
              {tasks.map((task) => <TaskRow key={task.id} task={task} />)}
            </ul>
          </section>
        ))}
      </div>
    </section>
  )
}

function AssignmentCard({ assignment }: { assignment: StaffAssignmentStatus }) {
  return (
    <article className={`coordination-assignment state-${assignment.state}`}>
      <div className="coordination-avatar" aria-hidden="true">{assignment.name.slice(0, 1).toUpperCase()}</div>
      <div>
        <strong>{assignment.name}</strong>
        <span>{assignment.role} · {assignment.state.replace('_', ' ')}</span>
        {assignment.currentTaskTitle && <small>{assignment.currentTaskTitle}</small>}
      </div>
    </article>
  )
}

function TaskRow({ task }: { task: CoordinationTask }) {
  return (
    <li className={`coordination-task ${task.isBlocked ? 'is-blocked' : ''}`}>
      <span className="coordination-task-status" aria-hidden="true" />
      <div>
        <strong>{task.title || task.id}</strong>
        <span>{task.status} · {task.progress}%{task.assignedStaffId ? ` · ${task.assignedStaffId}` : ''}</span>
        {task.isBlocked && <small role="status">{task.blockedReason || 'Awaiting dependency'}</small>}
      </div>
    </li>
  )
}

function Metric({ label, value, tone }: { label: string; value: number | string; tone?: 'warning' | 'success' }) {
  return <div className={`coordination-metric${tone ? ` ${tone}` : ''}`}><strong>{value}</strong><span>{label}</span></div>
}
