import { useCallback, useMemo, useState } from 'react'

export type TerminalSessionStatus = 'idle' | 'connecting' | 'ready' | 'running' | 'cancel_requested' | 'terminated' | 'error'
export type WorkerMessageType = 'handshake' | 'capabilities' | 'health' | 'task' | 'progress' | 'evidence' | 'diagnostic' | 'heartbeat' | 'terminal' | 'cancel' | 'shutdown'

export type WorkerMessage = {
  version: 'cortexos.worker.v1'
  type: WorkerMessageType
  correlationId: string
  sequence: number
  payload: Record<string, unknown>
}

export type TerminalLine = {
  id: string
  raw: string
  message?: WorkerMessage
  error?: string
  receivedAt: string
}

export type TerminalResourceSnapshot = {
  memoryMB: number
  memoryLimitMB: number
  cpuPriority: number
  cpuLimit: number
  activeLeases: number
}

export type TerminalSession = {
  id: string
  taskId: string
  workerId: string
  status: TerminalSessionStatus
  startedAt: string
  endedAt?: string
  lines: TerminalLine[]
  resources: TerminalResourceSnapshot
}

export type UseTerminalOptions = {
  initialSession?: Partial<TerminalSession>
  onCancel?: (sessionId: string, reason: string) => Promise<void> | void
  historyKey?: string
}

const protocolVersion = 'cortexos.worker.v1' as const
const messageTypes = new Set<WorkerMessageType>([
  'handshake', 'capabilities', 'health', 'task', 'progress', 'evidence',
  'diagnostic', 'heartbeat', 'terminal', 'cancel', 'shutdown',
])

const defaultResources: TerminalResourceSnapshot = {
  memoryMB: 0,
  memoryLimitMB: 2400,
  cpuPriority: 0,
  cpuLimit: 4,
  activeLeases: 0,
}

function makeId() {
  return `session-${Date.now().toString(36)}`
}

function createSession(initial?: Partial<TerminalSession>): TerminalSession {
  return {
    id: initial?.id ?? makeId(),
    taskId: initial?.taskId ?? 'awaiting-task',
    workerId: initial?.workerId ?? 'broker.pending',
    status: initial?.status ?? 'idle',
    startedAt: initial?.startedAt ?? new Date().toISOString(),
    endedAt: initial?.endedAt,
    lines: initial?.lines ?? [],
    resources: { ...defaultResources, ...initial?.resources },
  }
}

function readHistory(historyKey: string): TerminalSession[] {
  if (typeof window === 'undefined') return []
  try {
    const stored = JSON.parse(window.localStorage.getItem(historyKey) ?? '[]') as unknown
    if (Array.isArray(stored)) {
      return stored.filter((item): item is TerminalSession => Boolean(item && typeof item === 'object' && typeof (item as TerminalSession).id === 'string')).slice(0, 20)
    }
  } catch { /* malformed history is disposable UI state */ }
  return []
}

function validateWorkerMessage(value: unknown): WorkerMessage {
  if (!value || typeof value !== 'object') throw new Error('record must be an object')
  const candidate = value as Partial<WorkerMessage>
  if (candidate.version !== protocolVersion || typeof candidate.type !== 'string' || !messageTypes.has(candidate.type as WorkerMessageType)) {
    throw new Error('unsupported worker protocol record')
  }
  if (typeof candidate.correlationId !== 'string' || !candidate.correlationId || typeof candidate.sequence !== 'number' || !Number.isInteger(candidate.sequence) || candidate.sequence < 1) {
    throw new Error('invalid correlation or sequence')
  }
  if (!candidate.payload || typeof candidate.payload !== 'object' || Array.isArray(candidate.payload)) throw new Error('payload must be an object')
  return candidate as WorkerMessage
}

export function parseWorkerLine(raw: string): Omit<TerminalLine, 'id' | 'receivedAt'> {
  try {
    const parsed: unknown = JSON.parse(raw)
    return { raw, message: validateWorkerMessage(parsed) }
  } catch (error) {
    return { raw, error: error instanceof Error ? error.message : 'malformed JSONL record' }
  }
}

/**
 * Terminal state is deliberately a UI seam: execution remains owned by Harness.
 * The hook accepts broker stream records and forwards cancellation without
 * inventing lifecycle transitions or treating terminal success as task success.
 */
export function useTerminal(options: UseTerminalOptions = {}) {
  const historyKey = options.historyKey ?? 'cortexos-terminal-history'
  const [session, setSession] = useState<TerminalSession>(() => createSession(options.initialSession))
  const [history, setHistory] = useState<TerminalSession[]>(() => readHistory(historyKey))
  const [selectedHistoryId, setSelectedHistoryId] = useState<string | null>(null)
  const [cancelError, setCancelError] = useState('')

  const persistHistory = useCallback((next: TerminalSession[]) => {
    setHistory(next)
    if (typeof window !== 'undefined') window.localStorage.setItem(historyKey, JSON.stringify(next.slice(0, 20)))
  }, [historyKey])

  const appendLine = useCallback((raw: string) => {
    const parsed = parseWorkerLine(raw)
    const line: TerminalLine = { ...parsed, id: `${Date.now()}-${Math.random().toString(36).slice(2)}`, receivedAt: new Date().toISOString() }
    setSession((current) => {
      const nextStatus: TerminalSessionStatus = line.message?.type === 'terminal' ? 'terminated' : line.message?.type === 'cancel' ? 'cancel_requested' : line.message?.type === 'task' || line.message?.type === 'progress' ? 'running' : current.status === 'idle' ? 'ready' : current.status
      return { ...current, status: nextStatus, endedAt: nextStatus === 'terminated' ? line.receivedAt : current.endedAt, lines: [...current.lines, line] }
    })
  }, [])

  const startSession = useCallback((taskId: string, workerId = 'broker.pending') => {
    setCancelError('')
    setSession(createSession({ taskId, workerId, status: 'connecting' }))
  }, [])

  const cancelSession = useCallback(async (reason = 'Canceled from Terminal') => {
    setCancelError('')
    try {
      await options.onCancel?.(session.id, reason)
      setSession((current) => ({ ...current, status: 'cancel_requested' }))
      appendLine(JSON.stringify({ version: protocolVersion, type: 'cancel', correlationId: session.id, sequence: session.lines.length + 1, payload: { reason } }))
    } catch (error) {
      setCancelError(error instanceof Error ? error.message : 'Harness cancellation failed')
    }
  }, [appendLine, options, session.id, session.lines.length])

  const archiveSession = useCallback(() => {
    if (session.status === 'idle' && session.lines.length === 0) return
    persistHistory([session, ...history.filter((item) => item.id !== session.id)])
  }, [history, persistHistory, session])

  const selectHistory = useCallback((id: string) => {
    const selected = history.find((item) => item.id === id)
    if (selected) { setSelectedHistoryId(id); setSession(selected) }
  }, [history])

  const setResources = useCallback((resources: Partial<TerminalResourceSnapshot>) => {
    setSession((current) => ({ ...current, resources: { ...current.resources, ...resources } }))
  }, [])

  const progress = useMemo(() => {
    const record = [...session.lines].reverse().find((line) => line.message?.type === 'progress')
    const percent = record?.message?.payload.percent
    return typeof percent === 'number' ? Math.max(0, Math.min(100, percent)) : 0
  }, [session.lines])

  return {
    session,
    history,
    selectedHistoryId,
    progress,
    cancelError,
    startSession,
    appendLine,
    cancelSession,
    archiveSession,
    selectHistory,
    setResources,
  }
}
