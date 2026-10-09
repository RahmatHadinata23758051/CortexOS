import { useState } from 'react'

import { AuditTrail } from './AuditTrail'
import { DecisionPanel } from './DecisionPanel'
import { EvidenceViewer } from './EvidenceViewer'
import { MergeDialog } from './MergeDialog'
import { useInspector } from '../../hooks/useInspector'
import { type OrchestraTaskDetail, type OrchestraTaskSummary } from '../../types/orchestra'

export type InspectorReviewProps = {
  task: OrchestraTaskSummary
  detail: OrchestraTaskDetail
  onClose?: () => void
}

export function InspectorReview({ task, detail, onClose }: InspectorReviewProps) {
  const inspector = useInspector(detail)
  const [selectedEvidence, setSelectedEvidence] = useState(0)
  const [mergeDialogOpen, setMergeDialogOpen] = useState(false)
  const selected = inspector.evidence[selectedEvidence]

  return <section className="inspector-review" aria-labelledby="inspector-title">
    <header className="inspector-review-header">
      <div><span className="eyebrow">CORTEXOS / INSPECTOR</span><h1 id="inspector-title">Review the evidence.<br /><i>Then merge with confidence.</i></h1><p className="lede">Worker output is evidence, not a verdict. Inspector acceptance is the only path to an Orchestra-owned merge.</p></div>
      <div className="inspector-contract"><span>AUTHORITY</span><strong>ADR-0005</strong><small>Inspector → Orchestra</small></div>
    </header>
    <div className="inspector-meta"><code>{task.id}</code><span className={`inspector-state state-${inspector.decision}`}>{decisionLabel(inspector.decision)}</span><span className="inspector-gate">MERGE GATE: {inspector.mergeEnabled || inspector.mergeState === 'merged' ? 'OPEN' : 'LOCKED'}</span></div>
    <div className="inspector-grid">
      <EvidenceViewer evidence={inspector.evidence} selectedIndex={selectedEvidence} onSelect={setSelectedEvidence} selected={selected} />
      <DecisionPanel decision={inspector.decision} mergeState={inspector.mergeState} mergeEnabled={inspector.mergeEnabled} note={inspector.note} onNoteChange={inspector.setNote} onDecision={inspector.decide} onMerge={() => setMergeDialogOpen(true)} />
    </div>
    <AuditTrail entries={inspector.audit} />
    {onClose && <button className="inspector-close" type="button" onClick={onClose}>Back to task board</button>}
    <MergeDialog
      isOpen={mergeDialogOpen}
      decision={inspector.decision}
      mergeState={inspector.mergeState}
      mergeEnabled={inspector.mergeEnabled}
      taskId={task.id}
      onMerge={inspector.merge}
      onClose={() => setMergeDialogOpen(false)}
    />
  </section>
}

function decisionLabel(decision: ReturnType<typeof useInspector>['decision']): string {
  return decision === 'changes_requested' ? 'Changes requested' : decision === 'pending' ? 'Awaiting decision' : decision[0].toUpperCase() + decision.slice(1)
}
