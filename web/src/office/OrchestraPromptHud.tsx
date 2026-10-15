import { useState, type FormEvent } from 'react'
import { Check, ChevronDown, ChevronUp, LoaderCircle, Send, X } from 'lucide-react'
import {
  orchestraPlanSchemaVersion,
  type OrchestraPlanBridge,
  type OrchestraPlanProposal,
  type OrchestraPlanTask,
} from '../types/orchestra-plan'

type HudState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'proposal'; proposal: OrchestraPlanProposal }
  | { status: 'confirming'; proposal: OrchestraPlanProposal }
  | { status: 'rejecting'; proposal: OrchestraPlanProposal }
  | { status: 'dispatched'; proposal: OrchestraPlanProposal; message: string }
  | { status: 'error'; message: string }

type TaskDraft = {
  title: string
  role: string
  action: string
  acceptanceCriteria: string
  callSign: string
  estimatedComplexity: string
}

export type OrchestraPromptHudProps = {
  projectId: string
  bridge: OrchestraPlanBridge
}

const initialTask: TaskDraft = {
  title: '',
  role: '',
  action: '',
  acceptanceCriteria: '',
  callSign: '',
  estimatedComplexity: 'standard',
}

// ponytail: single-task prompt submission. Upgrade path: multi-task DAG editor when backend adds automated decomposition planner.
export function OrchestraPromptHud({ projectId, bridge }: OrchestraPromptHudProps) {
  const [prompt, setPrompt] = useState('')
  const [title, setTitle] = useState('')
  const [task, setTask] = useState<TaskDraft>(initialTask)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [state, setState] = useState<HudState>({ status: 'idle' })
  const busy = state.status === 'submitting' || state.status === 'confirming' || state.status === 'rejecting'
  const proposal = 'proposal' in state ? state.proposal : undefined

  const updateTask = (key: keyof TaskDraft, value: string) => setTask((current) => ({ ...current, [key]: value }))

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const userPrompt = prompt.trim()
    const planTitle = title.trim()
    const taskTitle = task.title.trim()
    const role = task.role.trim()
    const action = task.action.trim()
    const criteria = task.acceptanceCriteria.split('\n').map((item) => item.trim()).filter(Boolean)
    if (!projectId || !userPrompt || !planTitle || !taskTitle || !role || !action || criteria.length === 0) {
      setState({ status: 'error', message: 'Prompt, plan title, and one complete task are required.' })
      return
    }

    const taskDefinition: OrchestraPlanTask = {
      index: 0,
      title: taskTitle,
      role,
      action,
      acceptanceCriteria: criteria,
      dependencies: [],
      ...(task.callSign.trim() ? { callSign: task.callSign.trim() } : {}),
      ...(task.estimatedComplexity ? { estimatedComplexity: task.estimatedComplexity } : {}),
    }

    setState({ status: 'submitting' })
    try {
      const nextProposal = await bridge.ProposeOrchestraPlan({
        correlationId: `ui-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        projectId,
        userPrompt,
        title: planTitle,
        tasks: [taskDefinition],
        schemaVersion: orchestraPlanSchemaVersion,
      })
      if (nextProposal.requiresConfirmation && nextProposal.status === 'proposed') {
        setState({ status: 'proposal', proposal: nextProposal })
      } else {
        setState({ status: 'dispatched', proposal: nextProposal, message: 'Plan accepted by Orchestra and dispatched.' })
      }
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra plan proposal failed.' })
    }
  }

  const confirm = async () => {
    if (!proposal) return
    setState({ status: 'confirming', proposal })
    try {
      const result = await bridge.ConfirmOrchestraPlan({ planId: proposal.planId, schemaVersion: orchestraPlanSchemaVersion })
      setState({ status: 'dispatched', proposal: { ...proposal, status: result.status, confirmedAt: result.confirmedAt, dispatchedTaskIds: result.dispatchedTaskIds }, message: 'Plan confirmed and dispatched.' })
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra plan confirmation failed.' })
    }
  }

  const reject = async () => {
    if (!proposal) return
    setState({ status: 'rejecting', proposal })
    try {
      await bridge.RejectOrchestraPlan({ planId: proposal.planId, reason: 'Rejected from Orchestra prompt HUD.', schemaVersion: orchestraPlanSchemaVersion })
      setState({ status: 'idle' })
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra plan rejection failed.' })
    }
  }

  return (
    <section className="orchestra-prompt-hud" aria-labelledby="orchestra-prompt-title">
      <header className="orchestra-prompt-header">
        <div>
          <span className="eyebrow">ORCHESTRA / DISPATCH</span>
          <h2 id="orchestra-prompt-title">Prompt a governed plan</h2>
          <p>Submit explicit task scope. Execution state stays owned by the bridge.</p>
        </div>
        <span className="prompt-bridge-badge">LIVE BRIDGE</span>
      </header>
      <form className="orchestra-prompt-form" onSubmit={submit}>
        <label className="prompt-field prompt-field-wide">
          <span>Intent</span>
          <textarea value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="What should Orchestra coordinate?" rows={2} disabled={busy} />
        </label>
        <div className="prompt-form-grid">
          <label className="prompt-field"><span>Plan title</span><input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Release readiness" disabled={busy} /></label>
          <label className="prompt-field"><span>Task title</span><input value={task.title} onChange={(event) => updateTask('title', event.target.value)} placeholder="Inspect release checks" disabled={busy} /></label>
          <label className="prompt-field"><span>Role</span><input value={task.role} onChange={(event) => updateTask('role', event.target.value)} placeholder="reviewer" disabled={busy} /></label>
          <label className="prompt-field"><span>Action</span><input value={task.action} onChange={(event) => updateTask('action', event.target.value)} placeholder="Review CI and acceptance evidence" disabled={busy} /></label>
        </div>
        <label className="prompt-field prompt-field-wide"><span>Acceptance criteria <small>(one per line)</small></span><textarea value={task.acceptanceCriteria} onChange={(event) => updateTask('acceptanceCriteria', event.target.value)} placeholder="All required checks are green\nEvidence is attached" rows={2} disabled={busy} /></label>
        <button type="button" className="prompt-advanced-toggle" onClick={() => setShowAdvanced((value) => !value)} aria-expanded={showAdvanced} disabled={busy}>{showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />} Optional worker metadata</button>
        {showAdvanced && <div className="prompt-form-grid prompt-advanced-fields"><label className="prompt-field"><span>Call sign</span><input value={task.callSign} onChange={(event) => updateTask('callSign', event.target.value)} placeholder="Scout" disabled={busy} /></label><label className="prompt-field"><span>Complexity</span><select value={task.estimatedComplexity} onChange={(event) => updateTask('estimatedComplexity', event.target.value)} disabled={busy}><option value="trivial">Trivial</option><option value="standard">Standard</option><option value="complex">Complex</option></select></label></div>}
        <div className="prompt-form-footer"><span className="prompt-contract-note">Plan contract v1 · 1 task</span><button type="submit" className="prompt-submit" disabled={busy}><Send size={14} aria-hidden="true" />{state.status === 'submitting' ? 'Proposing…' : 'Propose plan'}</button></div>
      </form>
      {state.status === 'error' && <div className="prompt-feedback prompt-feedback-error" role="alert">{state.message}</div>}
      {proposal && <PlanProposal proposal={proposal} busy={busy} onConfirm={confirm} onReject={reject} dispatched={state.status === 'dispatched'} />}
    </section>
  )
}

function PlanProposal({ proposal, busy, onConfirm, onReject, dispatched }: { proposal: OrchestraPlanProposal; busy: boolean; onConfirm: () => void; onReject: () => void; dispatched: boolean }) {
  const requiresDecision = proposal.requiresConfirmation && proposal.status === 'proposed' && !dispatched
  return <section className="orchestra-plan-proposal" aria-live="polite" aria-label="Orchestra plan proposal">
    <div className="plan-proposal-header"><div><span className="eyebrow">PLAN {proposal.planId}</span><h3>{proposal.title}</h3></div><span className={`plan-risk risk-${proposal.riskLevel}`}>{proposal.riskLevel} risk</span></div>
    <p className="plan-summary">{proposal.summary || proposal.userPrompt}</p>
    <ol className="plan-task-list">{proposal.tasks.map((item) => <li key={`${proposal.planId}-${item.index}`}><span className="plan-task-index">{String(item.index + 1).padStart(2, '0')}</span><div><strong>{item.title}</strong><span>{item.role} · {item.action}</span><small>{item.acceptanceCriteria.join(' · ')}</small></div></li>)}</ol>
    {requiresDecision && <div className="plan-decision-actions"><button type="button" className="plan-reject-button" onClick={onReject} disabled={busy}><X size={14} /> Reject</button><button type="button" className="plan-confirm-button" onClick={onConfirm} disabled={busy}>{busy ? <LoaderCircle size={14} className="spin" /> : <Check size={14} />} Confirm & dispatch</button></div>}
    {!requiresDecision && <div className="prompt-feedback prompt-feedback-success"><Check size={14} /> {dispatched ? 'Plan confirmed and dispatch is now backend-owned.' : `Plan status: ${proposal.status}`}</div>}
  </section>
}
