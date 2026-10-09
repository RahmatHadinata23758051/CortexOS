import { useState, type FormEvent } from 'react'
import type { TerminalSessionStatus } from '../../hooks/useTerminal'

export type SessionCancellationProps = {
  sessionId: string
  sessionStatus: TerminalSessionStatus
  onCancel: (reason: string) => Promise<void> | void
  error?: string
}

export function SessionCancellation({
  sessionId,
  sessionStatus,
  onCancel,
  error,
}: SessionCancellationProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [reason, setReason] = useState('Operator cancellation via Terminal')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const isCancellable = sessionStatus === 'connecting' || sessionStatus === 'ready' || sessionStatus === 'running'

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!reason.trim()) return
    setIsSubmitting(true)
    try {
      await onCancel(reason.trim())
      setIsOpen(false)
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <div className="session-cancellation-seam">
      {!isOpen ? (
        <button
          type="button"
          className="btn-cancel-session"
          disabled={!isCancellable || isSubmitting}
          onClick={() => setIsOpen(true)}
          title={!isCancellable ? `Session is ${sessionStatus}` : 'Request session cancellation'}
        >
          {sessionStatus === 'cancel_requested' ? 'Cancellation in progress…' : 'Cancel Session'}
        </button>
      ) : (
        <form className="cancellation-confirm-form" onSubmit={handleSubmit}>
          <div className="cancellation-form-header">
            <strong>Cancel Session {sessionId}</strong>
            <p>Harness will propagate a graceful cancellation signal to the worker process group.</p>
          </div>
          <label className="cancellation-reason-label">
            <span>Cancellation Reason</span>
            <input
              type="text"
              required
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="e.g. Timeout exceeded, user abort..."
              autoFocus
            />
          </label>
          <div className="cancellation-actions">
            <button
              type="button"
              className="btn-cancel-abort"
              onClick={() => setIsOpen(false)}
              disabled={isSubmitting}
            >
              Nevermind
            </button>
            <button
              type="submit"
              className="btn-cancel-confirm"
              disabled={isSubmitting || !reason.trim()}
            >
              {isSubmitting ? 'Canceling…' : 'Confirm Cancel'}
            </button>
          </div>
        </form>
      )}

      {error && (
        <div className="cancellation-error" role="alert">
          <strong>Cancellation failed:</strong> <span>{error}</span>
        </div>
      )}
    </div>
  )
}
