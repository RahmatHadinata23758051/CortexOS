import { type InspectorDecision } from '../../hooks/useInspector'

export type MergeDialogProps = {
  isOpen: boolean
  decision: InspectorDecision
  mergeState: 'idle' | 'merging' | 'merged'
  mergeEnabled: boolean
  taskId: string
  targetBranch?: string
  onMerge: () => unknown
  onClose: () => void
}

export function MergeDialog({
  isOpen,
  decision,
  mergeState,
  mergeEnabled,
  taskId,
  targetBranch = 'main',
  onMerge,
  onClose,
}: MergeDialogProps) {
  if (!isOpen) return null

  const isAccepted = decision === 'accepted'

  return (
    <div className="merge-dialog-overlay" role="dialog" aria-labelledby="merge-dialog-title" aria-modal="true">
      <div className="merge-dialog-card">
        <div className="merge-dialog-header">
          <div>
            <span className="eyebrow">ORCHESTRA MERGE AUTHORITY</span>
            <h2 id="merge-dialog-title">Merge Review & Integration</h2>
          </div>
          <button className="merge-dialog-close" type="button" onClick={onClose} aria-label="Close merge dialog">
            ✕
          </button>
        </div>

        <div className="merge-dialog-body">
          <div className="merge-dialog-meta">
            <div>
              <span className="meta-label">Task ID</span>
              <code>{taskId}</code>
            </div>
            <div>
              <span className="meta-label">Target Branch</span>
              <code>{targetBranch}</code>
            </div>
            <div>
              <span className="meta-label">Inspector Status</span>
              <span className={`status-pill pill-${decision}`}>
                {decision === 'accepted' ? 'Accepted ✓' : decision === 'rejected' ? 'Rejected ✗' : decision === 'changes_requested' ? 'Changes Requested' : 'Pending Review'}
              </span>
            </div>
          </div>

          {!isAccepted && (
            <div className="merge-gate-warning" role="alert">
              <strong>ADR-0005 Enforced</strong>
              <p>Merge button is locked. Inspector acceptance is required before Orchestra can commit or integrate workspace changes.</p>
            </div>
          )}

          {mergeState === 'merging' && (
            <div className="merge-animation-container" aria-live="polite">
              <div className="merge-spinner" />
              <div className="merge-pulse-ring" />
              <strong>Orchestra is integrating changes…</strong>
              <p>Applying inspected worktree patch into {targetBranch}.</p>
            </div>
          )}

          {mergeState === 'merged' && (
            <div className="merge-success-banner" role="status">
              <span className="merge-check-icon" aria-hidden="true">✓</span>
              <strong>Integration Complete</strong>
              <p>Worktree cleanly integrated. Task marked as successful in Orchestra.</p>
            </div>
          )}

          {mergeState === 'idle' && isAccepted && (
            <p className="merge-ready-notice">
              All acceptance criteria and evidence checks passed. Ready for governed integration into {targetBranch}.
            </p>
          )}
        </div>

        <div className="merge-dialog-footer">
          <button className="dialog-cancel-button" type="button" onClick={onClose} disabled={mergeState === 'merging'}>
            {mergeState === 'merged' ? 'Close' : 'Cancel'}
          </button>
          <button
            className={`dialog-merge-button${mergeState === 'merging' ? ' merging' : ''}${mergeState === 'merged' ? ' merged' : ''}`}
            type="button"
            disabled={!mergeEnabled || mergeState !== 'idle'}
            onClick={onMerge}
          >
            {mergeState === 'merging' ? (
              <span className="button-loader">Integrating…</span>
            ) : mergeState === 'merged' ? (
              'Integrated'
            ) : (
              'Confirm & Merge'
            )}
          </button>
        </div>
      </div>
    </div>
  )
}
