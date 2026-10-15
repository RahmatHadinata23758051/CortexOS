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

type PlanDraft = {
  title: string
  taskTitle: string
  role: string
  action: string
  acceptanceCriteria: string
  complexity: string
}

export type OrchestraPromptHudProps = {
  projectId: string
  bridge: OrchestraPlanBridge
}

const initialDraft: PlanDraft = {
  title: '',
  taskTitle: '',
  role: 'implementer',
  action: 'complete the requested project work',
  acceptanceCriteria: 'Requested outcome is complete\nEvidence is ready for review',
  complexity: 'standard',
}

function deriveDraft(prompt: string, draft: PlanDraft): PlanDraft {
  const clean = prompt.trim().replace(/[.!?]+$/, '')
  const taskTitle = draft.taskTitle.trim() || clean.slice(0, 80)
  return {
    ...draft,
    title: draft.title.trim() || (taskTitle ? `Orchestrate: ${taskTitle}` : ''),
    taskTitle,
  }
}

// ponytail: one prompt creates one task until backend decomposition is exposed to the UI.
export function OrchestraPromptHud({ projectId, bridge }: OrchestraPromptHudProps) {
  const [prompt, setPrompt] = useState('')
  const [draft, setDraft] = useState<PlanDraft>(initialDraft)
  const [showDetails, setShowDetails] = useState(false)
  const [state, setState] = useState<HudState>({ status: 'idle' })
  const busy = state.status === 'submitting' || state.status === 'confirming' || state.status === 'rejecting'
  const proposal = 'proposal' in state ? state.proposal : undefined

  const updateDraft = (key: keyof PlanDraft, value: string) => setDraft((current) => ({ ...current, [key]: value }))

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const userPrompt = prompt.trim()
    const nextDraft = deriveDraft(userPrompt, draft)
    const criteria = nextDraft.acceptanceCriteria.split('\n').map((item) => item.trim()).filter(Boolean)
    if (!projectId || !userPrompt || !nextDraft.title || !nextDraft.taskTitle || !nextDraft.role || !nextDraft.action || criteria.length === 0) {
      setState({ status: 'error', message: 'Tell Orchestra what outcome you want.' })
      return
    }

    const task: OrchestraPlanTask = {
      index: 0,
      title: nextDraft.taskTitle,
      role: nextDraft.role,
      action: nextDraft.action,
      acceptanceCriteria: criteria,
      dependencies: [],
      estimatedComplexity: nextDraft.complexity,
    }

    setState({ status: 'submitting' })
    try {
      const nextProposal = await bridge.ProposeOrchestraPlan({
        correlationId: `ui-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        projectId,
        userPrompt,
        title: nextDraft.title,
        tasks: [task],
        schemaVersion: orchestraPlanSchemaVersion,
      })
      setDraft(nextDraft)
      setState(nextProposal.requiresConfirmation && nextProposal.status === 'proposed'
        ? { status: 'proposal', proposal: nextProposal }
        : { status: 'dispatched', proposal: nextProposal, message: 'Orchestra accepted the request and dispatched the work.' })
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra could not create a plan.' })
    }
  }

  const confirm = async () => {
    if (!proposal) return
    setState({ status: 'confirming', proposal })
    try {
      const result = await bridge.ConfirmOrchestraPlan({ planId: proposal.planId, schemaVersion: orchestraPlanSchemaVersion })
      setState({ status: 'dispatched', proposal: { ...proposal, status: result.status, confirmedAt: result.confirmedAt, dispatchedTaskIds: result.dispatchedTaskIds }, message: 'Plan confirmed. Orchestra owns the dispatch.' })
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra could not dispatch the plan.' })
    }
  }

  const reject = async () => {
    if (!proposal) return
    setState({ status: 'rejecting', proposal })
    try {
      await bridge.RejectOrchestraPlan({ planId: proposal.planId, reason: 'Rejected from Orchestra prompt.', schemaVersion: orchestraPlanSchemaVersion })
      setState({ status: 'idle' })
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra could not reject the plan.' })
    }
  }

  return (
    <section className="orchestra-prompt-hud" aria-labelledby="orchestra-prompt-title">
      <header className="orchestra-prompt-header">
        <div>
          <span className="eyebrow">ORCHESTRA / COMMAND</span>
          <h2 id="orchestra-prompt-title">What should we orchestrate?</h2>
          <p>Describe the outcome. Orchestra will plan, assign, and review the work.</p>
        </div>
        <span className="prompt-bridge-badge">LIVE</span>
      </header>
      <form className="orchestra-prompt-form" onSubmit={submit}>
        <label className="prompt-field prompt-command-field">
          <span>Command</span>
          <textarea autoFocus value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="Build, fix, inspect, or review something…" rows={5} disabled={busy} />
        </label>
        <button type="button" className="prompt-advanced-toggle" onClick={() => setShowDetails((value) => !value)} aria-expanded={showDetails} disabled={busy}>
          {showDetails ? <ChevronUp size={14} /> : <ChevronDown size={14} />} Tune plan details
        </button>
        {showDetails && (
          <div className="prompt-details">
            <label className="prompt-field"><span>Plan name</span><input value={draft.title} onChange={(event) => updateDraft('title', event.target.value)} placeholder="Generated from command" disabled={busy} /></label>
            <label className="prompt-field"><span>Worker role</span><input value={draft.role} onChange={(event) => updateDraft('role', event.target.value)} placeholder="implementer" disabled={busy} /></label>
            <label className="prompt-field"><span>Success looks like</span><textarea value={draft.acceptanceCriteria} onChange={(event) => updateDraft('acceptanceCriteria', event.target.value)} rows={3} disabled={busy} /></label>
          </div>
        )}
        <div className="prompt-form-footer">
          <span className="prompt-contract-note">Orchestra plan · approval-aware</span>
          <button type="submit" className="prompt-submit" disabled={busy || !prompt.trim()}>
            {state.status === 'submitting' ? <LoaderCircle size={14} className="spin" /> : <Send size={14} aria-hidden="true" />}
            {state.status === 'submitting' ? 'Planning…' : 'Send to Orchestra'}
          </button>
        </div>
      </form>
      {state.status === 'error' && <div className="prompt-feedback prompt-feedback-error" role="alert">{state.message}</div>}
      {proposal && <PlanProposal proposal={proposal} busy={busy} onConfirm={confirm} onReject={reject} dispatched={state.status === 'dispatched'} />}
    </section>
  )
}

function PlanProposal({ proposal, busy, onConfirm, onReject, dispatched }: { proposal: OrchestraPlanProposal; busy: boolean; onConfirm: () => void; onReject: () => void; dispatched: boolean }) {
  const requiresDecision = proposal.requiresConfirmation && proposal.status === 'proposed' && !dispatched
  return (
    <section className="orchestra-plan-proposal" aria-live="polite" aria-label="Orchestra plan proposal">
      <div className="plan-proposal-header"><div><span className="eyebrow">ORCHESTRA PLAN</span><h3>{proposal.title}</h3></div><span className={`plan-risk risk-${proposal.riskLevel}`}>{proposal.riskLevel} risk</span></div>
      <p className="plan-summary">{proposal.summary || proposal.userPrompt}</p>
      <ol className="plan-task-list">{proposal.tasks.map((item) => <li key={`${proposal.planId}-${item.index}`}><span className="plan-task-index">{String(item.index + 1).padStart(2, '0')}</span><div><strong>{item.title}</strong><span>{item.role} · {item.action}</span><small>{item.acceptanceCriteria.join(' · ')}</small></div></li>)}</ol>
      {requiresDecision && <div className="plan-decision-actions"><button type="button" className="plan-reject-button" onClick={onReject} disabled={busy}><X size={14} /> Reject</button><button type="button" className="plan-confirm-button" onClick={onConfirm} disabled={busy}>{busy ? <LoaderCircle size={14} className="spin" /> : <Check size={14} />} Confirm dispatch</button></div>}
      {!requiresDecision && <div className="prompt-feedback prompt-feedback-success"><Check size={14} /> {dispatched ? 'Dispatched. Watch the graph for active work.' : `Plan status: ${proposal.status}`}</div>}
    </section>
  )
}
