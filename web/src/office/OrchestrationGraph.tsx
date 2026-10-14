import { useState, useMemo } from 'react'
import { X, AlertTriangle } from 'lucide-react'
import type { CoordinationSnapshot, CoordinationTask, StaffAssignmentStatus } from '../types/coordination'

export interface OrchestrationGraphProps {
  snapshot: CoordinationSnapshot
  onSelectTask?: (taskId: string) => void
  onSelectStaff?: (staffId: string) => void
  className?: string
}

type NodePosition = {
  task: CoordinationTask
  assignment?: StaffAssignmentStatus
  x: number
  y: number
  width: number
  height: number
  colIndex: number
  rowIndex: number
}

type EdgePath = {
  id: string
  fromId: string
  toId: string
  path: string
  isActive: boolean
  isBlocked: boolean
  isCompleted: boolean
}

const CARD_WIDTH = 280
const CARD_HEIGHT = 142
const COL_GAP = 96
const ROW_GAP = 28
const PADDING_X = 48
const PADDING_Y = 56

export function OrchestrationGraph({
  snapshot,
  onSelectTask,
  onSelectStaff,
  className = '',
}: OrchestrationGraphProps) {
  const [selectedTaskId, setSelectedTaskId] = useState<string | null>(null)
  const [filterState, setFilterState] = useState<'all' | 'running' | 'blocked' | 'done'>('all')

  const taskMap = useMemo(() => {
    const map = new Map<string, CoordinationTask>()
    snapshot.tasks.forEach((t) => map.set(t.id, t))
    return map
  }, [snapshot.tasks])

  const assignmentMap = useMemo(() => {
    const map = new Map<string, StaffAssignmentStatus>()
    snapshot.assignments.forEach((a) => {
      if (a.currentTaskId) map.set(a.currentTaskId, a)
      map.set(a.staffId, a)
    })
    return map
  }, [snapshot.assignments])

  // ponytail: compute layout columns from phase or topological depth. Upgrade to dynamic dagre layout if arbitrary branch cycles occur.
  const { nodePositions, edges, bounds, phaseHeaders } = useMemo(() => {
    const phaseGroups = new Map<number, CoordinationTask[]>()

    snapshot.tasks.forEach((task) => {
      let p = task.phase ?? 0
      if (p === 0 && task.dependencies.length > 0) {
        // Fallback depth calculation if phase unassigned
        p = 1
      }
      const list = phaseGroups.get(p) ?? []
      list.push(task)
      phaseGroups.set(p, list)
    })

    const sortedPhases = [...phaseGroups.keys()].sort((a, b) => a - b)
    const positions = new Map<string, NodePosition>()
    const headers: Array<{ phase: number; x: number; label: string; count: number }> = []

    let maxColY = 0

    sortedPhases.forEach((phase, colIdx) => {
      const tasksInPhase = phaseGroups.get(phase) ?? []
      const colX = PADDING_X + colIdx * (CARD_WIDTH + COL_GAP)

      headers.push({
        phase,
        x: colX,
        label: `Phase ${phase + 1}`,
        count: tasksInPhase.length,
      })

      tasksInPhase.forEach((task, rowIdx) => {
        const rowY = PADDING_Y + rowIdx * (CARD_HEIGHT + ROW_GAP)
        positions.set(task.id, {
          task,
          assignment: task.assignedStaffId ? assignmentMap.get(task.assignedStaffId) : undefined,
          x: colX,
          y: rowY,
          width: CARD_WIDTH,
          height: CARD_HEIGHT,
          colIndex: colIdx,
          rowIndex: rowIdx,
        })
        if (rowY + CARD_HEIGHT > maxColY) {
          maxColY = rowY + CARD_HEIGHT
        }
      })
    })

    // Compute SVG bezier edges between task dependencies
    const calculatedEdges: EdgePath[] = []
    snapshot.tasks.forEach((targetTask) => {
      const targetPos = positions.get(targetTask.id)
      if (!targetPos) return

      targetTask.dependencies.forEach((depId) => {
        const sourcePos = positions.get(depId)
        if (!sourcePos) return

        const startX = sourcePos.x + sourcePos.width
        const startY = sourcePos.y + sourcePos.height / 2
        const endX = targetPos.x
        const endY = targetPos.y + targetPos.height / 2
        const deltaX = Math.max(32, (endX - startX) * 0.5)

        const path = `M ${startX} ${startY} C ${startX + deltaX} ${startY}, ${endX - deltaX} ${endY}, ${endX} ${endY}`
        const isActive = sourcePos.task.status === 'running' || targetPos.task.status === 'running'
        const isBlocked = targetPos.task.isBlocked
        const isCompleted = sourcePos.task.status === 'success'

        calculatedEdges.push({
          id: `${depId}->${targetTask.id}`,
          fromId: depId,
          toId: targetTask.id,
          path,
          isActive,
          isBlocked,
          isCompleted,
        })
      })
    })

    const totalWidth = PADDING_X * 2 + Math.max(1, sortedPhases.length) * (CARD_WIDTH + COL_GAP)
    const totalHeight = Math.max(480, maxColY + PADDING_Y)

    return {
      nodePositions: [...positions.values()],
      edges: calculatedEdges,
      bounds: { width: totalWidth, height: totalHeight },
      phaseHeaders: headers,
    }
  }, [snapshot.tasks, assignmentMap])

  const filteredNodes = useMemo(() => {
    if (filterState === 'all') return nodePositions
    if (filterState === 'running') return nodePositions.filter((n) => n.task.status === 'running')
    if (filterState === 'blocked') return nodePositions.filter((n) => n.task.isBlocked)
    if (filterState === 'done') return nodePositions.filter((n) => n.task.status === 'success')
    return nodePositions
  }, [nodePositions, filterState])

  const selectedNode = selectedTaskId ? nodePositions.find((n) => n.task.id === selectedTaskId) : null

  const handleCardClick = (taskId: string) => {
    const next = selectedTaskId === taskId ? null : taskId
    setSelectedTaskId(next)
    if (next) onSelectTask?.(next)
  }

  return (
    <div className={`orchestration-graph ${className}`} aria-label="Orchestration Hierarchy Graph">
      {/* Control / Toolbar Strip */}
      <div className="graph-toolbar">
        <div className="graph-toolbar-group">
          <span className="graph-title-badge">NODE HIERARCHY</span>
          <span className="graph-stat-pill">
            <strong>{snapshot.metrics.totalTasks}</strong> tasks
          </span>
          <span className="graph-stat-pill pill-running">
            <span className="dot pulse" /> {snapshot.metrics.runningTasks} running
          </span>
          {snapshot.metrics.blockedTasks > 0 && (
            <span className="graph-stat-pill pill-blocked">
              <span className="dot" /> {snapshot.metrics.blockedTasks} blocked
            </span>
          )}
        </div>

        <div className="graph-filter-pills" role="tablist" aria-label="Filter tasks">
          {(['all', 'running', 'blocked', 'done'] as const).map((mode) => (
            <button
              key={mode}
              type="button"
              className={`filter-pill ${filterState === mode ? 'active' : ''}`}
              onClick={() => setFilterState(mode)}
              role="tab"
              aria-selected={filterState === mode}
            >
              {mode.toUpperCase()}
            </button>
          ))}
        </div>
      </div>

      {/* Main Canvas Scroll Area */}
      <div className="graph-viewport">
        <div
          className="graph-canvas"
          style={{ width: bounds.width, minHeight: bounds.height }}
        >
          {/* Phase Column Headers */}
          <div className="phase-headers-track">
            {phaseHeaders.map((hdr) => (
              <div key={hdr.phase} className="phase-header" style={{ left: hdr.x, width: CARD_WIDTH }}>
                <div className="phase-header-inner">
                  <span className="phase-title">{hdr.label}</span>
                  <span className="phase-count">{hdr.count} nodes</span>
                </div>
              </div>
            ))}
          </div>

          {/* SVG Cable Connector Layer */}
          <svg
            className="graph-svg-layer"
            width={bounds.width}
            height={bounds.height}
            aria-hidden="true"
          >
            <defs>
              <linearGradient id="edge-running" x1="0%" y1="0%" x2="100%" y2="0%">
                <stop offset="0%" stopColor="var(--accent-cool)" stopOpacity="0.8" />
                <stop offset="100%" stopColor="var(--accent-warm)" stopOpacity="0.9" />
              </linearGradient>
              <linearGradient id="edge-complete" x1="0%" y1="0%" x2="100%" y2="0%">
                <stop offset="0%" stopColor="var(--status-success)" stopOpacity="0.4" />
                <stop offset="100%" stopColor="var(--status-success)" stopOpacity="0.7" />
              </linearGradient>
            </defs>

            {edges.map((edge) => {
              const isHighlighted =
                selectedTaskId && (edge.fromId === selectedTaskId || edge.toId === selectedTaskId)

              let stroke = 'var(--border-strong)'
              let strokeClass = 'edge-idle'

              if (edge.isActive) {
                stroke = 'url(#edge-running)'
                strokeClass = 'edge-active'
              } else if (edge.isBlocked) {
                stroke = 'var(--status-warning)'
                strokeClass = 'edge-blocked'
              } else if (edge.isCompleted) {
                stroke = 'url(#edge-complete)'
                strokeClass = 'edge-completed'
              }

              return (
                <g key={edge.id} className={`edge-group ${isHighlighted ? 'highlighted' : ''}`}>
                  <path
                    d={edge.path}
                    className={`edge-path-bg`}
                    strokeWidth={isHighlighted ? 4 : 2}
                  />
                  <path
                    d={edge.path}
                    className={`edge-path ${strokeClass}`}
                    stroke={stroke}
                    strokeWidth={isHighlighted ? 3 : 1.5}
                    fill="none"
                  />
                  <circle
                    className={`edge-junction ${strokeClass}`}
                    cx={edge.path.match(/M\s*([\d.]+)\s+([\d.]+)/)?.[1] ?? 0}
                    cy={edge.path.match(/M\s*([\d.]+)\s+([\d.]+)/)?.[2] ?? 0}
                    r={isHighlighted ? 3 : 2}
                  />
                </g>
              )
            })}
          </svg>

          {/* Node Cards Layer */}
          <div className="graph-nodes-layer">
            {filteredNodes.map(({ task, assignment, x, y }) => {
              const isSelected = selectedTaskId === task.id
              const hasWorker = Boolean(task.assignedStaffId || assignment)
              const workerName = assignment?.name ?? task.assignedStaffId ?? 'Unassigned'
              const workerRole = assignment?.role ?? task.assignedStaffRole ?? 'Specialist'
              const status = task.status

              return (
                <article
                  key={task.id}
                  className={`node-card state-${status} ${task.isBlocked ? 'is-blocked' : ''} ${
                    isSelected ? 'is-selected' : ''
                  }`}
                  style={{
                    transform: `translate3d(${x}px, ${y}px, 0)`,
                    width: CARD_WIDTH,
                    height: CARD_HEIGHT,
                  }}
                  onClick={() => handleCardClick(task.id)}
                  tabIndex={0}
                  role="button"
                  aria-pressed={isSelected}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault()
                      handleCardClick(task.id)
                    }
                  }}
                >
                  {/* Port Connectors */}
                  <span className="port port-in" aria-hidden="true" />
                  <span className="port port-out" aria-hidden="true" />

                  {/* Card Header */}
                  <div className="node-card-header">
                    <span className="node-id-badge">{task.id}</span>
                    <span className={`node-status-pill status-${status}`}>
                      {status === 'running' && <span className="status-ping" />}
                      {status}
                    </span>
                  </div>

                  {/* Card Title */}
                  <h4 className="node-title" title={task.title}>
                    {task.title || 'Untitled task'}
                  </h4>

                  {/* Blocked Alert Banner if blocked */}
                  {task.isBlocked && (
                    <div className="node-blocked-reason" title={task.blockedReason || 'Awaiting dependency'}>
                      <AlertTriangle size={12} aria-hidden="true" /> {task.blockedReason || 'Blocked by upstream'}
                    </div>
                  )}

                  {/* Card Footer: Worker Avatar Chip & Progress */}
                  <div className="node-card-footer">
                    <div
                      className={`worker-chip ${hasWorker ? 'assigned' : 'unassigned'}`}
                      onClick={(e) => {
                        if (task.assignedStaffId) {
                          e.stopPropagation()
                          onSelectStaff?.(task.assignedStaffId)
                        }
                      }}
                      title={`Assigned to ${workerName} (${workerRole})`}
                    >
                      <span className="worker-avatar" aria-hidden="true">
                        {workerName.slice(0, 1).toUpperCase()}
                      </span>
                      <div className="worker-meta">
                        <span className="worker-name">{workerName}</span>
                        <span className="worker-role">{workerRole}</span>
                      </div>
                    </div>

                    <div className="node-progress-wrap" title={`${task.progress}% complete`}>
                      <span className="node-progress-text">{task.progress}%</span>
                      <div className="node-progress-track">
                        <div
                          className={`node-progress-fill ${status === 'running' ? 'animated' : ''}`}
                          style={{ width: `${Math.min(100, Math.max(0, task.progress))}%` }}
                        />
                      </div>
                    </div>
                  </div>
                </article>
              )
            })}
          </div>
        </div>
      </div>

      {/* Selected Node Inspector Drawer */}
      {selectedNode && (
        <aside className="graph-drawer" aria-label="Task Inspector Drawer">
          <div className="drawer-header">
            <div>
              <span className="eyebrow">NODE DETAILS</span>
              <h3>{selectedNode.task.title || selectedNode.task.id}</h3>
            </div>
            <button
              type="button"
              className="drawer-close"
              onClick={() => setSelectedTaskId(null)}
              aria-label="Close inspector drawer"
            >
              <X size={15} aria-hidden="true" />
            </button>
          </div>

          <div className="drawer-body">
            <div className="drawer-section">
              <span className="drawer-label">STATUS</span>
              <div className="drawer-field">
                <span className={`node-status-pill status-${selectedNode.task.status}`}>
                  {selectedNode.task.status}
                </span>
                {selectedNode.task.isBlocked && (
                  <span className="drawer-warning">
                    Blocked: {selectedNode.task.blockedReason || 'Awaiting prerequisites'}
                  </span>
                )}
              </div>
            </div>

            <div className="drawer-section">
              <span className="drawer-label">ASSIGNED WORKER</span>
              <div className="drawer-worker-box">
                <span className="worker-avatar">
                  {(selectedNode.assignment?.name ?? selectedNode.task.assignedStaffId ?? 'U')
                    .slice(0, 1)
                    .toUpperCase()}
                </span>
                <div>
                  <strong>{selectedNode.assignment?.name ?? selectedNode.task.assignedStaffId ?? 'Unassigned'}</strong>
                  <span>{selectedNode.assignment?.role ?? selectedNode.task.assignedStaffRole ?? 'Specialist'}</span>
                </div>
              </div>
            </div>

            <div className="drawer-section">
              <span className="drawer-label">DEPENDENCY CHAIN</span>
              {selectedNode.task.dependencies.length === 0 ? (
                <span className="drawer-empty-text">Root node (no incoming dependencies)</span>
              ) : (
                <ul className="drawer-dep-list">
                  {selectedNode.task.dependencies.map((depId) => {
                    const depTask = taskMap.get(depId)
                    return (
                      <li key={depId} className="dep-item">
                        <span className="dep-tag">{depId}</span>
                        <span className="dep-title">{depTask?.title ?? 'Prerequisite task'}</span>
                      </li>
                    )
                  })}
                </ul>
              )}
            </div>

            <div className="drawer-section">
              <span className="drawer-label">EXECUTION METRICS</span>
              <div className="drawer-metrics-grid">
                <div>
                  <small>Phase</small>
                  <strong>Phase {selectedNode.task.phase + 1}</strong>
                </div>
                <div>
                  <small>Attempts</small>
                  <strong>
                    {selectedNode.task.attempt} / {selectedNode.task.maxAttempts || 3}
                  </strong>
                </div>
                <div>
                  <small>Progress</small>
                  <strong>{selectedNode.task.progress}%</strong>
                </div>
              </div>
            </div>
          </div>
        </aside>
      )}
    </div>
  )
}
