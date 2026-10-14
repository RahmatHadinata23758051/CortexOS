import { useEffect, useState, useCallback, useRef, type ReactNode } from 'react'
import { X, Check, Terminal, FileText, GitBranch, Search, FolderOpen, Settings, Zap } from 'lucide-react'
import { wailsRuntimeBridge } from '../types'
import { listPendingToolApprovals, decideToolCall } from '../bridge/phase13'
import type { ToolProposalRecord } from '../types/phase13'

export interface ApprovalDrawerProps {
  isOpen: boolean
  onClose: () => void
  className?: string
}

const DEV_MOCK_PROPOSALS: ToolProposalRecord[] = [
  {
    proposalId: 'prop-dev-01',
    correlationId: 'corr-101',
    taskId: 'task-102',
    worktreeId: 'worktree-alpha',
    toolName: 'bash',
    capability: 'workspace:exec',
    redactedArgsSummary: 'git checkout -b feat/authority-hardening',
    argByteSize: 42,
    status: 'PENDING',
    proposedAt: '2026-10-14T01:00:00Z',
    schemaVersion: 'cortexos.tool-approval.proposal.v1',
  },
  {
    proposalId: 'prop-dev-02',
    correlationId: 'corr-102',
    taskId: 'task-103',
    worktreeId: 'worktree-alpha',
    toolName: 'file_write',
    capability: 'filesystem:write',
    redactedArgsSummary: 'internal/cortex/orchestra/policy.json',
    argByteSize: 256,
    status: 'PENDING',
    proposedAt: '2026-10-14T01:02:00Z',
    schemaVersion: 'cortexos.tool-approval.proposal.v1',
  },
]

export function ApprovalDrawer({ isOpen, onClose, className = '' }: ApprovalDrawerProps): ReactNode {
  const [approvals, setApprovals] = useState<ToolProposalRecord[]>(DEV_MOCK_PROPOSALS)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [deciding, setDeciding] = useState<string | null>(null)
  const counterRef = useRef(0)

  const hasNativeBridge = typeof window !== 'undefined' && Boolean((window as unknown as { go?: { platform?: { Bridge?: unknown } } }).go?.platform?.Bridge)

  const loadApprovals = useCallback(async () => {
    if (!hasNativeBridge) {
      return
    }
    setLoading(true)
    setError(null)
    try {
      const result = await listPendingToolApprovals(wailsRuntimeBridge)
      setApprovals(result)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load approvals')
    } finally {
      setLoading(false)
    }
  }, [hasNativeBridge])

  useEffect(() => {
    if (isOpen) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      void loadApprovals()
    }
  }, [isOpen, loadApprovals])

  const handleDecide = async (proposalId: string, decision: 'approve' | 'deny', reason: string) => {
    setDeciding(proposalId)
    try {
      if (hasNativeBridge) {
        counterRef.current += 1
        const idempotencyKey = `${decision}-${proposalId}-${counterRef.current}`
        await decideToolCall(wailsRuntimeBridge, {
          proposalId,
          decision,
          actor: 'operator',
          reason,
          idempotencyKey,
        })
        await loadApprovals()
      } else {
        // Dev browser simulation
        setApprovals((prev) => prev.filter((item) => item.proposalId !== proposalId))
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Decision failed')
    } finally {
      setDeciding(null)
    }
  }

  const handleApprove = (proposal: ToolProposalRecord) => {
    const reason = window.prompt(`Approve "${proposal.toolName}"? Enter reason:`, 'Approved by operator')
    if (reason !== null) handleDecide(proposal.proposalId, 'approve', reason)
  }

  const handleDeny = (proposal: ToolProposalRecord) => {
    const reason = window.prompt(`Deny "${proposal.toolName}"? Enter reason:`, 'Denied by operator')
    if (reason !== null) handleDecide(proposal.proposalId, 'deny', reason)
  }

  if (!isOpen) return null

  return (
    <aside className={`approval-drawer ${className}`} aria-label="Tool Approval Queue" role="dialog" aria-modal="true">
      <div className="drawer-backdrop" onClick={onClose} aria-hidden="true" />
      <div className="drawer-panel">
        <div className="drawer-header">
          <div>
            <span className="eyebrow">TOOL APPROVAL</span>
            <h3>Pending approvals <span className="badge-count">{approvals.length}</span></h3>
          </div>
          <button type="button" className="drawer-close" onClick={onClose} aria-label="Close approval drawer">
            <X size={15} aria-hidden="true" />
          </button>
        </div>

        <div className="drawer-body">
          {error ? (
            <div className="drawer-error" role="alert">{error}</div>
          ) : loading ? (
            <div className="drawer-loading">Loading approval queue…</div>
          ) : approvals.length === 0 ? (
            <div className="drawer-empty">
              <Check size={24} aria-hidden="true" className="text-success" />
              <strong>No pending approvals</strong>
              <p>All tool calls are cleared.</p>
            </div>
          ) : (
            <ul className="approval-list" aria-label="Pending tool proposals">
            {approvals.map((proposal) => (
              <li key={proposal.proposalId} className="approval-item">
                <div className="approval-main">
                  <div className="approval-icon" aria-hidden="true">
                    <span className={`tool-glyph tool-${proposal.toolName?.toLowerCase() || 'default'}`}>
                      {getToolGlyph(proposal.toolName)}
                    </span>
                  </div>
                  <div className="approval-info">
                    <div className="approval-title-row">
                      <strong>{proposal.toolName || 'Unknown tool'}</strong>
                      <span className={`approval-status status-${proposal.status?.toLowerCase() || 'pending'}`}>
                        {proposal.status || 'PENDING'}
                      </span>
                    </div>
                    <div className="approval-meta">
                      <span className="approval-task">{proposal.taskId ? `Task: ${proposal.taskId}` : 'No task'}</span>
                      <span className="approval-capability">Cap: {proposal.capability || '—'}</span>
                      <span className="approval-worktree">Worktree: {proposal.worktreeId || '—'}</span>
                    </div>
                    {proposal.redactedArgsSummary && (
                      <div className="approval-args">
                        <code>{proposal.redactedArgsSummary}</code>
                      </div>
                    )}
                    {proposal.reason && (
                      <div className="approval-reason">
                        <small>Previous: {proposal.reason}</small>
                      </div>
                    )}
                  </div>
                </div>
                <div className="approval-actions">
                  <button
                    type="button"
                    className="btn-deny"
                    onClick={() => handleDeny(proposal)}
                    disabled={deciding === proposal.proposalId}
                  >
                    {deciding === proposal.proposalId ? 'Denying…' : 'Deny'}
                  </button>
                  <button
                    type="button"
                    className="btn-approve"
                    onClick={() => handleApprove(proposal)}
                    disabled={deciding === proposal.proposalId}
                  >
                    {deciding === proposal.proposalId ? 'Approving…' : 'Approve'}
                  </button>
                </div>
              </li>
            ))}
            </ul>
          )}
        </div>
      </div>
    </aside>
  )
}

function getToolGlyph(toolName?: string): ReactNode {
  const glyphs: Record<string, ReactNode> = {
    bash: <Terminal size={16} />,
    file_read: <FileText size={16} />,
    file_write: <FileText size={16} />,
    file_edit: <FileText size={16} />,
    git: <GitBranch size={16} />,
    grep: <Search size={16} />,
    list: <FolderOpen size={16} />,
    glob: <Search size={16} />,
    task: <Settings size={16} />,
    default: <Zap size={16} />,
  }
  return glyphs[toolName?.toLowerCase() || 'default'] ?? glyphs.default
}