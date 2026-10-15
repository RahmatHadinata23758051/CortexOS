import { useMemo, type CSSProperties } from 'react'
import type { CoordinationSnapshot, CoordinationTask } from '../types/coordination'

export interface OrchestrationGraphProps {
  snapshot: CoordinationSnapshot
  /** @deprecated Staff profiles are intentionally not rendered in the execution graph. */
  staff?: readonly unknown[]
  onSelectTask?: (taskId: string) => void
  onSelectStaff?: (staffId: string) => void
  className?: string
}

type GraphZone = 'coordination' | 'planning' | 'delivery' | 'review'
type GraphStatus = 'available' | 'busy' | 'unavailable' | 'offline' | 'error'

type GraphNode = {
  id: string
  title: string
  role: string
  zone: GraphZone
  level: number
  status: GraphStatus
  statusLabel: string
  meta: string
  detail: string
  task?: CoordinationTask
}

type GraphEdge = {
  id: string
  kind: 'dispatch' | 'dependency'
  path: string
  label: string
  labelX: number
  labelY: number
  sequence: number
}

type Position = { x: number; y: number }

const ROOT_ID = 'orchestra-root'
const CARD_WIDTH = 280
const CARD_HEIGHT = 100
const HORIZONTAL_GAP = 80
const VERTICAL_GAP = 60
const TOP = 68
const LEFT = 90
const ZONE_COLORS: Record<GraphZone, string> = {
  coordination: '#2563eb',
  planning: '#0f766e',
  delivery: '#b45309',
  review: '#6d28d9',
}
const ZONE_LABELS: Record<GraphZone, string> = {
  coordination: 'Coordination',
  planning: 'Planning',
  delivery: 'Delivery',
  review: 'Review',
}

function roleForTask(task: CoordinationTask): string {
  const explicit = (task.assignedStaffRole ?? '').trim().toLowerCase()
  if (explicit) return explicit
  // ponytail: Orchestra tasks carry no role field; derive it from the call sign stem (e.g. "implementer-1" -> "implementer"). Upgrade path: add a role field to orchestra.Task.
  const callSign = (task.assignedStaffId ?? '').trim().toLowerCase()
  return callSign.replace(/-\d+$/, '')
}

function workerNameForTask(task: CoordinationTask): string {
  const callSign = (task.assignedStaffId ?? '').trim()
  if (callSign) return callSign
  const role = roleForTask(task)
  return role ? `${role} worker` : 'Worker'
}

function zoneForTask(task: CoordinationTask): GraphZone {
  const role = roleForTask(task)
  if (role === 'coordinator') return 'coordination'
  if (role === 'planner') return 'planning'
  if (role === 'reviewer' || role === 'specialist') return 'review'
  return 'delivery'
}

function statusForTask(task: CoordinationTask): GraphStatus {
  if (task.isBlocked || task.status === 'canceled') return 'unavailable'
  if (task.status === 'failed') return 'error'
  if (task.status === 'running' || task.status === 'awaitingInspection') return 'busy'
  return 'available'
}

function statusLabelForTask(task: CoordinationTask): string {
  if (task.isBlocked) return 'BLOCKED'
  switch (task.status) {
    case 'awaitingInspection': return 'REVIEW'
    case 'running': return 'RUNNING'
    case 'ready': return 'READY'
    case 'draft': return 'QUEUED'
    case 'failed': return 'FAILED'
    default: return task.status.toUpperCase()
  }
}

function activeTasks(snapshot: CoordinationSnapshot): CoordinationTask[] {
  return snapshot.tasks
    .filter((task) => task.status !== 'success' && task.status !== 'canceled')
    .sort((a, b) => a.phase - b.phase || a.id.localeCompare(b.id))
}

function buildGraph(snapshot: CoordinationSnapshot) {
  const tasks = activeTasks(snapshot)
  const nodes: GraphNode[] = [
    {
      id: ROOT_ID,
      title: 'Orchestra',
      role: 'coordinator',
      zone: 'coordination',
      level: 0,
      status: tasks.length > 0 ? 'busy' : 'available',
      statusLabel: tasks.length > 0 ? 'DISPATCHING' : 'IDLE',
      meta: tasks.length > 0 ? `${tasks.length} ACTIVE TASK${tasks.length === 1 ? '' : 'S'}` : 'NO ACTIVE WORK',
      detail: tasks.length > 0 ? 'Execution authority' : 'Ready for a command',
    },
    ...tasks.map((task) => ({
      id: task.id,
      title: workerNameForTask(task),
      role: roleForTask(task),
      zone: zoneForTask(task),
      level: Math.max(1, task.phase + 1),
      status: statusForTask(task),
      statusLabel: statusLabelForTask(task),
      meta: `${roleForTask(task).toUpperCase() || 'WORKER'} · PHASE ${task.phase + 1}`,
      detail: `${task.title} · ${Math.round(task.progress)}% · ATTEMPT ${task.attempt}/${task.maxAttempts}`,
      task,
    })),
  ]

  const byLevel = new Map<number, GraphNode[]>()
  nodes.forEach((node) => {
    const level = byLevel.get(node.level) ?? []
    level.push(node)
    byLevel.set(node.level, level)
  })
  const maxPerLevel = Math.max(...Array.from(byLevel.values()).map((level) => level.length), 1)
  const width = Math.max(1040, LEFT * 2 + maxPerLevel * CARD_WIDTH + (maxPerLevel - 1) * HORIZONTAL_GAP)
  const positions = new Map<string, Position>()
  byLevel.forEach((levelNodes, level) => {
    const rowWidth = levelNodes.length * CARD_WIDTH + (levelNodes.length - 1) * HORIZONTAL_GAP
    const start = Math.max(LEFT, (width - rowWidth) / 2)
    levelNodes.forEach((node, index) => {
      positions.set(node.id, { x: start + index * (CARD_WIDTH + HORIZONTAL_GAP), y: TOP + level * (CARD_HEIGHT + VERTICAL_GAP) })
    })
  })

  const height = Math.max(520, TOP + (Math.max(...nodes.map((node) => node.level), 0) + 1) * (CARD_HEIGHT + VERTICAL_GAP) + 92)
  const edges: GraphEdge[] = []
  let sequence = 0
  tasks.forEach((task) => {
    const child = positions.get(task.id)
    if (!child) return
    const dependencies = task.dependencies.filter((id) => positions.has(id))
    const parents = dependencies.length > 0 ? dependencies : [ROOT_ID]
    parents.forEach((parentId) => {
      const parent = positions.get(parentId)
      if (!parent) return
      const startX = parent.x + CARD_WIDTH / 2
      const startY = parent.y + CARD_HEIGHT
      const endX = child.x + CARD_WIDTH / 2
      const endY = child.y
      const busY = startY + VERTICAL_GAP / 2
      const kind = parentId === ROOT_ID ? 'dispatch' : 'dependency'
      edges.push({
        id: `${parentId}->${task.id}`,
        kind,
        path: `M ${startX} ${startY} V ${busY} H ${endX} V ${endY}`,
        label: kind === 'dispatch' ? 'DISPATCH' : 'DEPENDS ON',
        labelX: (startX + endX) / 2,
        labelY: busY - 6,
        sequence: sequence++,
      })
    })
  })

  return { nodes, positions, edges, width, height }
}

function statusLabel(status: GraphStatus) {
  return status === 'error' ? 'ERROR' : status.toUpperCase()
}

export function OrchestrationGraph({ snapshot, onSelectTask, className = '' }: OrchestrationGraphProps) {
  const graph = useMemo(() => buildGraph(snapshot), [snapshot])
  const { nodes, positions, edges, width, height } = graph
  const maxLevel = nodes.reduce((max, node) => Math.max(max, node.level), 0)
  const laneLabels = Array.from({ length: maxLevel + 1 }, (_, level) => ({
    level,
    label: level === 0 ? 'L0 · ORCHESTRA' : `L${level} · EXECUTION`,
  }))

  return (
    <section className={`orchestration-graph ${className}`} aria-label="Orchestra execution graph">
      <div className="hierarchy-graph-toolbar">
        <span>GRAPH · {nodes.length} NODES · {edges.length} EDGES</span>
        <span>{nodes.length === 1 ? 'IDLE' : 'LIVE EXECUTION'}</span>
      </div>
      <div className="hierarchy-graph-viewport">
        <div className="hierarchy-graph-canvas" style={{ width, minHeight: height }}>
          <div className="hierarchy-lane-labels" aria-hidden="true">
            {laneLabels.map((lane) => (
              <span key={lane.level} style={{ top: TOP + lane.level * (CARD_HEIGHT + VERTICAL_GAP) + 42 }}>{lane.label}</span>
            ))}
          </div>
          <svg className="hierarchy-edges" width={width} height={height} aria-hidden="true">
            <defs>
              <marker id="hierarchy-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto">
                <path d="M 0 0 L 8 4 L 0 8 z" />
              </marker>
              <marker id="hierarchy-arrow-violet" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="7" markerHeight="7" orient="auto">
                <path d="M 0 0 L 8 4 L 0 8 z" />
              </marker>
            </defs>
            {edges.map((edge) => (
              <g key={edge.id} className={`hierarchy-edge hierarchy-edge-${edge.kind}`}>
                <path d={edge.path} markerEnd={`url(#hierarchy-arrow${edge.kind === 'dependency' ? '-violet' : ''})`} />
                <text x={edge.labelX} y={edge.labelY}>{edge.label}</text>
                <circle className="hierarchy-dispatch" r="4">
                  <animateMotion dur="4s" begin={`${edge.sequence * 0.64}s`} repeatCount="indefinite" path={edge.path} />
                </circle>
              </g>
            ))}
          </svg>
          <div className="hierarchy-nodes">
            {nodes.map((node, index) => {
              const position = positions.get(node.id)
              if (!position) return null
              const style = {
                left: position.x,
                top: position.y,
                '--z': ZONE_COLORS[node.zone],
                '--beat': node.status === 'busy' ? '1.2s' : '2.4s',
                '--phase': `${index * -0.37}s`,
              } as CSSProperties
              return (
                <button
                  key={node.id}
                  type="button"
                  className={`hierarchy-node hierarchy-node-${node.status}`}
                  style={style}
                  aria-label={`${node.title}, ${node.role}, ${node.statusLabel}`}
                  title={`${node.title} · ${node.statusLabel}`}
                  onClick={() => { if (node.task) onSelectTask?.(node.task.id) }}
                >
                  <span className="hierarchy-avatar-wrap" aria-hidden="true">
                    <span className="hierarchy-avatar">{node.task ? node.title.slice(0, 1).toUpperCase() : 'O'}</span>
                  </span>
                  <span className="hierarchy-node-copy">
                    <span className="hierarchy-node-title">
                      <strong>{node.title}</strong>
                      <span className="hierarchy-status">{node.task ? node.statusLabel : statusLabel(node.status)}</span>
                    </span>
                    <span className="hierarchy-node-meta">{node.role.toUpperCase()} · {node.meta}</span>
                    <span className="hierarchy-node-zone"><i />{node.detail}</span>
                  </span>
                </button>
              )
            })}
          </div>
          <div className="hierarchy-legend">
            <div className="hierarchy-zone-legend">
              <span className="hierarchy-legend-label">ZONES</span>
              {(Object.keys(ZONE_COLORS) as GraphZone[]).map((zone) => (
                <span key={zone}><i style={{ background: ZONE_COLORS[zone] }} />{ZONE_LABELS[zone]}</span>
              ))}
            </div>
            <div className="hierarchy-animation-legend">
              <span>◯ Heartbeat = node alive</span>
              <span>● Dot = task dispatch</span>
              <span>--- Dependency = waiting on prior work</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
