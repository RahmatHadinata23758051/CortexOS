export const coordinationSchemaVersion = 'cortexos.coordination.v1' as const

export type CoordinationTaskStatus =
  | 'draft'
  | 'ready'
  | 'running'
  | 'awaitingInspection'
  | 'success'
  | 'failed'
  | 'canceled'

export type CoordinationTask = {
  id: string
  projectId: string
  title: string
  assignedStaffId?: string
  assignedStaffRole?: string
  status: CoordinationTaskStatus | string
  phase: number
  dependencies: string[]
  isBlocked: boolean
  blockedReason?: string
  attempt: number
  maxAttempts: number
  progress: number
  updatedAt: string
}

export type StaffAssignmentStatus = {
  staffId: string
  name: string
  role: string
  availability: string
  currentTaskId?: string
  currentTaskTitle?: string
  state: 'idle' | 'running' | 'blocked' | 'awaiting_inspection' | 'completed' | 'failed' | string
  assignedTaskCount: number
  completedTaskCount: number
}

export type CoordinationMetrics = {
  totalTasks: number
  readyTasks: number
  runningTasks: number
  blockedTasks: number
  awaitingInspection: number
  completedTasks: number
  failedTasks: number
  canceledTasks: number
  activeStaffCount: number
  totalStaffCount: number
}

export type CoordinationSnapshot = {
  schemaVersion: typeof coordinationSchemaVersion
  projectId?: string
  activePhase: number
  totalPhases: number
  tasks: CoordinationTask[]
  assignments: StaffAssignmentStatus[]
  metrics: CoordinationMetrics
  generatedAt: string
}

export type CoordinationSnapshotRequest = {
  schemaVersion: typeof coordinationSchemaVersion
  projectId?: string
}

export type CoordinationEventPayload = {
  schemaVersion: typeof coordinationSchemaVersion
  eventType: 'assigned' | 'reassigned' | 'progress' | 'blocked' | 'completed' | 'canceled'
  taskId: string
  projectId?: string
  staffId?: string
  previousStaffId?: string
  role?: string
  status: string
  phase: number
  isBlocked: boolean
  blockedReason?: string
  progress: number
  timestamp: string
}

export type CoordinationBridge = {
  GetCoordinationSnapshot(request: CoordinationSnapshotRequest): Promise<CoordinationSnapshot>
}

export const CoordinationEventNames = {
  Changed: 'cortexos:coordination:changed',
  Assigned: 'cortexos:coordination:assigned',
  Reassigned: 'cortexos:coordination:reassigned',
  Progress: 'cortexos:coordination:progress',
  Blocked: 'cortexos:coordination:blocked',
  Completed: 'cortexos:coordination:completed',
  Canceled: 'cortexos:coordination:canceled',
} as const

export type CoordinationEventName = (typeof CoordinationEventNames)[keyof typeof CoordinationEventNames]

export function isCoordinationSnapshot(value: unknown): value is CoordinationSnapshot {
  if (!value || typeof value !== 'object') return false
  const snapshot = value as Partial<CoordinationSnapshot>
  return snapshot.schemaVersion === coordinationSchemaVersion &&
    typeof snapshot.activePhase === 'number' && Number.isInteger(snapshot.activePhase) && snapshot.activePhase >= 0 &&
    typeof snapshot.totalPhases === 'number' && Number.isInteger(snapshot.totalPhases) && snapshot.totalPhases >= 0 &&
    Array.isArray(snapshot.tasks) && snapshot.tasks.length <= 100 && snapshot.tasks.every(isCoordinationTask) &&
    Array.isArray(snapshot.assignments) && snapshot.assignments.length <= 50 && snapshot.assignments.every(isStaffAssignmentStatus) &&
    isCoordinationMetrics(snapshot.metrics)
}

function isCoordinationTask(value: unknown): value is CoordinationTask {
  if (!value || typeof value !== 'object') return false
  const task = value as Partial<CoordinationTask>
  return typeof task.id === 'string' && task.id.length > 0 &&
    typeof task.projectId === 'string' && typeof task.title === 'string' && task.title.length <= 128 &&
    typeof task.status === 'string' && typeof task.phase === 'number' && task.phase >= 0 &&
    Array.isArray(task.dependencies) && task.dependencies.every((dependency) => typeof dependency === 'string') &&
    typeof task.isBlocked === 'boolean' && typeof task.progress === 'number' && task.progress >= 0 && task.progress <= 100
}

function isStaffAssignmentStatus(value: unknown): value is StaffAssignmentStatus {
  if (!value || typeof value !== 'object') return false
  const assignment = value as Partial<StaffAssignmentStatus>
  return typeof assignment.staffId === 'string' && assignment.staffId.length > 0 &&
    typeof assignment.name === 'string' && typeof assignment.role === 'string' &&
    typeof assignment.availability === 'string' && typeof assignment.state === 'string' &&
    typeof assignment.assignedTaskCount === 'number' && assignment.assignedTaskCount >= 0 &&
    typeof assignment.completedTaskCount === 'number' && assignment.completedTaskCount >= 0
}

function isCoordinationMetrics(value: unknown): value is CoordinationMetrics {
  if (!value || typeof value !== 'object') return false
  const metrics = value as Partial<CoordinationMetrics>
  return ['totalTasks', 'readyTasks', 'runningTasks', 'blockedTasks', 'awaitingInspection', 'completedTasks', 'failedTasks', 'canceledTasks', 'activeStaffCount', 'totalStaffCount']
    .every((key) => typeof metrics[key as keyof CoordinationMetrics] === 'number' && (metrics[key as keyof CoordinationMetrics] as number) >= 0)
}
