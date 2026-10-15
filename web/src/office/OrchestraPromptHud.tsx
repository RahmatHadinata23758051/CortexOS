import { useState, type FormEvent } from 'react'
import { Check, LoaderCircle, Send, X } from 'lucide-react'
import {
  orchestraPlanSchemaVersion,
  type OrchestraPlanBridge,
  type OrchestraPlanProposal,
} from '../types/orchestra-plan'

type HudState =
  | { status: 'idle' }
  | { status: 'submitting' }
  | { status: 'proposal'; proposal: OrchestraPlanProposal }
  | { status: 'confirming'; proposal: OrchestraPlanProposal }
  | { status: 'rejecting'; proposal: OrchestraPlanProposal }
  | { status: 'dispatched'; proposal: OrchestraPlanProposal; message: string }
  | { status: 'answered'; proposal: OrchestraPlanProposal }
  | { status: 'error'; message: string }

export type OrchestraPromptHudProps = {
  projectId: string
  bridge: OrchestraPlanBridge
}

// ponytail: the HUD sends only conversation text. Upgrade path: replace the plan bridge with a dedicated Orchestra turn contract when the agent runtime exposes one.
export function OrchestraPromptHud({ projectId, bridge }: OrchestraPromptHudProps) {
  const [prompt, setPrompt] = useState('')
  const [state, setState] = useState<HudState>({ status: 'idle' })
  const busy = state.status === 'submitting' || state.status === 'confirming' || state.status === 'rejecting'
  const proposal = 'proposal' in state ? state.proposal : undefined

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const userPrompt = prompt.trim()
    if (!projectId || !userPrompt) {
      setState({ status: 'error', message: 'Say something to Orchestra first.' })
      return
    }

    setState({ status: 'submitting' })
    try {
      const nextProposal = await bridge.ProposeOrchestraPlan({
        correlationId: `ui-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        projectId,
        userPrompt,
        tasks: [],
        schemaVersion: orchestraPlanSchemaVersion,
      })
      if (nextProposal.status === 'answered') {
        setState({ status: 'answered', proposal: nextProposal })
      } else if (nextProposal.requiresConfirmation && nextProposal.status === 'proposed') {
        setState({ status: 'proposal', proposal: nextProposal })
      } else {
        setState({ status: 'dispatched', proposal: nextProposal, message: 'Orchestra decided this request needs work and dispatched it.' })
      }
    } catch (error) {
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Orchestra could not process the message.' })
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
          <span className="eyebrow">ORCHESTRA / CHAT</span>
          <h2 id="orchestra-prompt-title">Talk to Orchestra</h2>
          <p>Ask a question or describe work. Orchestra decides whether a worker is needed.</p>
        </div>
        <span className="prompt-bridge-badge">LIVE</span>
      </header>
      <form className="orchestra-prompt-form" onSubmit={submit}>
        <label className="prompt-field prompt-command-field">
          <span>Message</span>
          <textarea autoFocus value={prompt} onChange={(event) => setPrompt(event.target.value)} placeholder="Ask Orchestra anything, or describe a change to make…" rows={5} disabled={busy} />
        </label>
        <div className="prompt-form-footer">
          <span className="prompt-contract-note">Direct answer first · workers only when needed</span>
          <button type="submit" className="prompt-submit" disabled={busy || !prompt.trim()}>
            {state.status === 'submitting' ? <LoaderCircle size={14} className="spin" /> : <Send size={14} aria-hidden="true" />}
            {state.status === 'submitting' ? 'Thinking…' : 'Send to Orchestra'}
          </button>
        </div>
      </form>
      {state.status === 'answered' && <div className="prompt-feedback prompt-feedback-success" role="status"><Check size={14} /> {state.proposal.reply || state.proposal.summary}</div>}
      {state.status === 'error' && <div className="prompt-feedback prompt-feedback-error" role="alert">{state.message}</div>}
      {proposal && proposal.status !== 'answered' && <PlanProposal proposal={proposal} busy={busy} onConfirm={confirm} onReject={reject} dispatched={state.status === 'dispatched'} />}
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
