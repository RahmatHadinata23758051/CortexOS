import { useMemo, type CSSProperties } from 'react'
import type { StaffSummary } from '../types/office'
import type { CoordinationSnapshot, StaffAssignmentStatus } from '../types/coordination'

export interface OrchestrationGraphProps {
  snapshot: CoordinationSnapshot
  staff?: readonly StaffSummary[]
  onSelectTask?: (taskId: string) => void
  onSelectStaff?: (staffId: string) => void
  className?: string
}

type GraphZone = 'coordination' | 'planning' | 'delivery' | 'review'
type GraphStatus = 'available' | 'busy' | 'unavailable' | 'offline' | 'error'

type GraphNode = {
  staff: StaffSummary
  assignment?: StaffAssignmentStatus
  parentId?: string
  zone: GraphZone
  level: number
  status: GraphStatus
  profile: string
  currentTaskId?: string
}

type GraphEdge = {
  id: string
  kind: 'delegates' | 'assigns' | 'handoff'
  path: string
  label: string
  labelX: number
  labelY: number
  sequence: number
}

type Position = { x: number; y: number }

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
const ROLE_ZONES: Record<StaffSummary['role'], GraphZone> = {
  coordinator: 'coordination',
  planner: 'planning',
  implementer: 'delivery',
  reviewer: 'review',
  specialist: 'review',
}

function getFallbackParent(staff: readonly StaffSummary[], member: StaffSummary) {
  if (member.parentId !== undefined) return member.parentId ?? undefined
  const coordinator = staff.find((candidate) => candidate.role === 'coordinator')
  if (!coordinator || coordinator.id === member.id) return undefined
  if (member.role === 'planner' || member.role === 'reviewer') return coordinator.id
  if (member.role === 'implementer' || member.role === 'specialist') {
    return staff.find((candidate) => candidate.role === 'planner')?.id ?? coordinator.id
  }
  return undefined
}

function statusFor(member: StaffSummary, assignment?: StaffAssignmentStatus): GraphStatus {
  if (assignment?.state === 'failed') return 'error'
  if (member.availability === 'busy' || assignment?.state === 'running') return 'busy'
  if (member.availability === 'available') return 'available'
  return member.availability
}

function statusLabel(status: GraphStatus) {
  return status === 'error' ? 'ERROR' : status.toUpperCase()
}

function measureTree(id: string, children: Map<string, string[]>, seen = new Set<string>()): number {
  if (seen.has(id)) return CARD_WIDTH
  const nextSeen = new Set(seen).add(id)
  const childWidths = (children.get(id) ?? []).map((child) => measureTree(child, children, nextSeen))
  if (childWidths.length === 0) return CARD_WIDTH
  return Math.max(CARD_WIDTH, childWidths.reduce((sum, width) => sum + width, 0) + HORIZONTAL_GAP * (childWidths.length - 1))
}

function buildGraph(snapshot: CoordinationSnapshot, staff: readonly StaffSummary[]) {
  const assignments = new Map(snapshot.assignments.map((assignment) => [assignment.staffId, assignment]))
  const nodes: GraphNode[] = staff.map((member) => {
    const assignment = assignments.get(member.id)
    return {
      staff: member,
      assignment,
      parentId: getFallbackParent(staff, member),
      zone: ROLE_ZONES[member.role],
      level: 0,
      status: statusFor(member, assignment),
      profile: member.profile ?? 'default',
      currentTaskId: assignment?.currentTaskId,
    }
  })
  const byId = new Map(nodes.map((node) => [node.staff.id, node]))
  const children = new Map<string, string[]>()
  nodes.forEach((node) => {
    if (node.parentId && byId.has(node.parentId)) {
      const siblings = children.get(node.parentId) ?? []
      siblings.push(node.staff.id)
      children.set(node.parentId, siblings)
    } else {
      node.parentId = undefined
    }
  })

  const roots = nodes.filter((node) => !node.parentId)
  const positions = new Map<string, Position>()
  let cursor = LEFT
  const layout = (node: GraphNode, left: number, level: number) => {
    node.level = level
    const width = measureTree(node.staff.id, children)
    const childIds = children.get(node.staff.id) ?? []
    if (childIds.length === 0) {
      positions.set(node.staff.id, { x: left, y: TOP + level * (CARD_HEIGHT + VERTICAL_GAP) })
      return width
    }
    let childLeft = left
    childIds.forEach((childId) => {
      const childWidth = measureTree(childId, children)
      layout(byId.get(childId)!, childLeft, level + 1)
      childLeft += childWidth + HORIZONTAL_GAP
    })
    const first = positions.get(childIds[0])!
    const last = positions.get(childIds[childIds.length - 1])!
    positions.set(node.staff.id, {
      x: (first.x + last.x + CARD_WIDTH) / 2 - CARD_WIDTH / 2,
      y: TOP + level * (CARD_HEIGHT + VERTICAL_GAP),
    })
    return width
  }
  roots.forEach((root) => {
    const width = layout(root, cursor, 0)
    cursor += width + HORIZONTAL_GAP
  })

  const maxLevel = nodes.reduce((max, node) => Math.max(max, node.level), 0)
  const width = Math.max(1040, cursor + LEFT - HORIZONTAL_GAP)
  const height = Math.max(520, TOP + (maxLevel + 1) * (CARD_HEIGHT + VERTICAL_GAP) + 92)
  const edges: GraphEdge[] = []
  let sequence = 0
  nodes.forEach((node) => {
    if (!node.parentId) return
    const parent = positions.get(node.parentId)
    const child = positions.get(node.staff.id)
    if (!parent || !child) return
    const startX = parent.x + CARD_WIDTH / 2
    const startY = parent.y + CARD_HEIGHT
    const endX = child.x + CARD_WIDTH / 2
    const endY = child.y
    const busY = startY + VERTICAL_GAP / 2
    const kind = node.staff.role === 'implementer' || node.staff.role === 'specialist' ? 'assigns' : 'delegates'
    edges.push({
      id: `${node.parentId}->${node.staff.id}`,
      kind,
      path: `M ${startX} ${startY} V ${busY} H ${endX} V ${endY}`,
      label: kind === 'assigns' ? 'ASSIGNS WORK' : 'DELEGATES',
      labelX: (startX + endX) / 2,
      labelY: busY - 6,
      sequence: sequence++,
    })
  })

  const worker = nodes.find((node) => node.staff.role === 'implementer')
  const reviewer = nodes.find((node) => node.staff.role === 'reviewer')
  if (worker && reviewer && worker.staff.id !== reviewer.staff.id) {
    const from = positions.get(worker.staff.id)
    const to = positions.get(reviewer.staff.id)
    if (from && to) {
      const startX = from.x + CARD_WIDTH
      const startY = from.y + CARD_HEIGHT / 2
      const endX = to.x + CARD_WIDTH / 2
      const endY = to.y + CARD_HEIGHT
      const elbowX = Math.max(startX + 60, endX)
      edges.push({
        id: `${worker.staff.id}->${reviewer.staff.id}:handoff`,
        kind: 'handoff',
        path: `M ${startX} ${startY} H ${elbowX} V ${endY} H ${endX}`,
        label: 'REVIEW HANDOFF',
        labelX: (startX + elbowX) / 2,
        labelY: startY - 8,
        sequence: sequence++,
      })
    }
  }

  return { nodes, positions, edges, width, height }
}

export function OrchestrationGraph({ snapshot, staff = [], onSelectTask, onSelectStaff, className = '' }: OrchestrationGraphProps) {
  const graph = useMemo(() => buildGraph(snapshot, staff), [snapshot, staff])
  const { nodes, positions, edges, width, height } = graph
  const maxLevel = nodes.reduce((max, node) => Math.max(max, node.level), 0)
  const laneLabels = Array.from({ length: maxLevel + 1 }, (_, level) => {
    const role = level === 0 ? 'COORDINATOR' : level === 1 ? 'LEADS' : 'WORKERS'
    return { level, label: `L${level} · ${role}` }
  })

  return (
    <section className={`orchestration-graph ${className}`} aria-label="Orchestration hierarchy graph">
      <div className="hierarchy-graph-toolbar">
        <span>GRAPH · {nodes.length} NODES · {edges.length} EDGES</span>
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
                <path d={edge.path} markerEnd={`url(#hierarchy-arrow${edge.kind === 'handoff' ? '-violet' : ''})`} />
                <text x={edge.labelX} y={edge.labelY}>{edge.label}</text>
                <circle className="hierarchy-dispatch" r="4">
                  <animateMotion dur="4s" begin={`${edge.sequence * 0.64}s`} repeatCount="indefinite" path={edge.path} />
                </circle>
              </g>
            ))}
          </svg>
          <div className="hierarchy-nodes">
            {nodes.map((node, index) => {
              const position = positions.get(node.staff.id)
              if (!position) return null
              const style = {
                left: position.x,
                top: position.y,
                '--z': ZONE_COLORS[node.zone],
                '--beat': node.status === 'busy' ? '1.2s' : '2.4s',
                '--phase': `${index * -0.37}s`,
              } as CSSProperties
              const taskId = node.currentTaskId
              return (
                <button
                  key={node.staff.id}
                  type="button"
                  className={`hierarchy-node hierarchy-node-${node.status}`}
                  style={style}
                  aria-label={`${node.staff.name}, ${node.staff.role}, ${node.status}`}
                  title={`${node.staff.name} · ${node.staff.role} · ${statusLabel(node.status)}`}
                  onClick={() => {
                    onSelectStaff?.(node.staff.id)
                    if (taskId) onSelectTask?.(taskId)
                  }}
                >
                  <span className="hierarchy-avatar-wrap" aria-hidden="true">
                    <span className="hierarchy-avatar">{node.staff.name.slice(0, 1).toUpperCase()}</span>
                  </span>
                  <span className="hierarchy-node-copy">
                    <span className="hierarchy-node-title">
                      <strong>{node.staff.name}</strong>
                      <span className="hierarchy-status">{statusLabel(node.status)}</span>
                    </span>
                    <span className="hierarchy-node-meta">{node.staff.role.toUpperCase()} · {node.profile}</span>
                    <span className="hierarchy-node-zone"><i />{ZONE_LABELS[node.zone].toUpperCase()}</span>
                  </span>
                </button>
              )
            })}
          </div>
          {nodes.length === 0 && <p className="hierarchy-empty">No staff hierarchy is available from the connected runtime.</p>}
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
              <span>--- Review handoff</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  )
}
