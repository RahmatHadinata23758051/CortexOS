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

const initialState: InspectorState = { decision: 'pending', mergeState: 'idle', audit: [] }

export function useInspector(task: OrchestraTaskDetail | null) {
  const [state, setState] = useState<InspectorState>(initialState)
  const [note, setNote] = useState('')
  const evidence = useMemo(() => buildEvidence(task), [task])

  const decide = useCallback((decision: Exclude<InspectorDecision, 'pending'>) => {
    const label = decision === 'accepted' ? 'Inspector accepted evidence' : decision === 'rejected' ? 'Inspector rejected evidence' : 'Changes requested from worker'
    setState((current) => ({
      ...current,
      decision,
      audit: [{ id: `${Date.now()}`, decision, note: note.trim() || label, occurredAt: new Date().toISOString() }, ...current.audit],
    }))
  }, [note])

  const merge = useCallback(async () => {
    if (state.decision !== 'accepted' || state.mergeState !== 'idle') return false
    setState((current) => ({ ...current, mergeState: 'merging' }))
    await new Promise<void>((resolve) => window.setTimeout(resolve, 620))
    setState((current) => ({
      ...current,
      mergeState: 'merged',
      audit: [{ id: `${Date.now()}`, decision: 'accepted', note: 'Orchestra merge completed after Inspector acceptance', occurredAt: new Date().toISOString() }, ...current.audit],
    }))
    return true
  }, [state.decision, state.mergeState])

  const reset = useCallback(() => {
    setState(initialState)
    setNote('')
  }, [])

  return { ...state, evidence, note, setNote, decide, merge, reset, mergeEnabled: state.decision === 'accepted' && state.mergeState === 'idle' }
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
