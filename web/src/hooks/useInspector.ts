import { useCallback, useMemo, useState } from 'react'

import { type OrchestraTaskDetail } from '../types/orchestra'

export type InspectorDecision = 'pending' | 'accepted' | 'rejected' | 'changes_requested'
export type InspectorAuditEntry = {
  id: string
  decision: InspectorDecision
  note: string
  occurredAt: string
}

export type InspectorEvidence = {
  id: string
  kind: string
  language: string
  content: string
  redactions: number
  passed: boolean
}

export type InspectorState = {
  decision: InspectorDecision
  mergeState: 'idle' | 'merging' | 'merged'
  audit: InspectorAuditEntry[]
}

/**
 * Backend merge seam. The hook never decides that a merge happened on its own:
 * it only reports `merged` when this command resolves with backend success.
 */
export type InspectorMergeCommand = (note: string) => Promise<unknown>

export type UseInspectorOptions = {
  mergeCommand?: InspectorMergeCommand
  onMerged?: () => void
}

const initialState: InspectorState = { decision: 'pending', mergeState: 'idle', audit: [] }

/** Pure gate: merge is only allowed after Inspector acceptance and while idle (ADR-0005). */
export function canRequestMerge(state: Pick<InspectorState, 'decision' | 'mergeState'>, hasCommand: boolean): boolean {
  return state.decision === 'accepted' && state.mergeState === 'idle' && hasCommand
}

export function useInspector(task: OrchestraTaskDetail | null, options: UseInspectorOptions = {}) {
  const [state, setState] = useState<InspectorState>(initialState)
  const [note, setNote] = useState('')
  const [mergeError, setMergeError] = useState('')
  const evidence = useMemo(() => buildEvidence(task), [task])
  const { mergeCommand, onMerged } = options

  const decide = useCallback((decision: Exclude<InspectorDecision, 'pending'>) => {
    const label = decision === 'accepted' ? 'Inspector accepted evidence' : decision === 'rejected' ? 'Inspector rejected evidence' : 'Changes requested from worker'
    setState((current) => ({
      ...current,
      decision,
      audit: [{ id: `${Date.now()}`, decision, note: note.trim() || label, occurredAt: new Date().toISOString() }, ...current.audit],
    }))
  }, [note])

  const merge = useCallback(async () => {
    // Fail closed: no Inspector acceptance or no backend command means no merge.
    if (state.decision !== 'accepted' || state.mergeState !== 'idle') return false
    if (!mergeCommand) {
      setMergeError('Orchestra merge command is unavailable. Merge stays locked.')
      return false
    }
    setMergeError('')
    setState((current) => ({ ...current, mergeState: 'merging' }))
    try {
      await mergeCommand(note)
    } catch (error) {
      // Revert to the locked-but-retryable state; never leave a fake "merged" banner.
      setState((current) => ({ ...current, mergeState: 'idle' }))
      setMergeError(error instanceof Error ? error.message : 'Orchestra merge was rejected. Merge stays locked.')
      return false
    }
    setState((current) => ({
      ...current,
      mergeState: 'merged',
      audit: [{ id: `${Date.now()}`, decision: 'accepted', note: 'Orchestra merge completed after Inspector acceptance', occurredAt: new Date().toISOString() }, ...current.audit],
    }))
    onMerged?.()
    return true
  }, [state.decision, state.mergeState, mergeCommand, note, onMerged])

  const reset = useCallback(() => {
    setState(initialState)
    setNote('')
    setMergeError('')
  }, [])

  return { ...state, evidence, note, setNote, decide, merge, reset, mergeError, mergeEnabled: canRequestMerge(state, mergeCommand !== undefined) }
}

function buildEvidence(task: OrchestraTaskDetail | null): InspectorEvidence[] {
  if (!task) return []
  return task.evidence.map((entry, index) => ({
    id: entry.id,
    kind: index === 0 ? 'validation' : 'artifact',
    language: index % 2 === 0 ? 'typescript' : 'json',
    content: index === 0
      ? `const inspection = {\n  task: '${task.id}',\n  criteria: ${task.acceptanceCount},\n  status: '${task.status}',\n  evidence: '${entry.id}'\n}`
      : `{"evidenceId":"${entry.id}","count":${entry.count},"redacted":true}`,
    redactions: Math.max(1, entry.count),
    passed: task.status === 'awaitingInspection' || task.status === 'success',
  }))
}
