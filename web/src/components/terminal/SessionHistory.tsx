import type { TerminalSession } from '../../hooks/useTerminal'

export type SessionHistoryProps = {
  history: TerminalSession[]
  currentSessionId: string
  selectedHistoryId: string | null
  onSelectSession: (sessionId: string) => void
  onArchiveSession?: () => void
}

export function SessionHistory({
  history,
  currentSessionId,
  selectedHistoryId,
  onSelectSession,
  onArchiveSession,
}: SessionHistoryProps) {
  return (
    <aside className="terminal-session-history" aria-labelledby="session-history-title">
      <div className="history-header">
        <div>
          <span className="eyebrow">PERSISTED RUNS</span>
          <h3 id="session-history-title">Session History</h3>
        </div>
        {onArchiveSession && (
          <button
            type="button"
            className="btn-archive-session"
            onClick={onArchiveSession}
            title="Archive current session into history"
          >
            Archive Current
          </button>
        )}
      </div>

      <div className="history-list" role="list">
        {history.length === 0 ? (
          <div className="history-empty">
            <span className="empty-glyph" aria-hidden="true">∅</span>
            <p>No recorded past sessions.</p>
          </div>
        ) : (
          history.map((sess) => {
            const isCurrent = sess.id === currentSessionId
            const isSelected = sess.id === selectedHistoryId
            return (
              <div
                key={sess.id}
                role="listitem"
                className={`history-item${isSelected ? ' selected' : ''}${isCurrent ? ' active-run' : ''}`}
                onClick={() => onSelectSession(sess.id)}
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') onSelectSession(sess.id)
                }}
              >
                <div className="history-item-top">
                  <span className={`status-pill status-${sess.status}`}>{sess.status}</span>
                  <span className="history-lines-count">{sess.lines.length} lines</span>
                </div>
                <div className="history-item-ids">
                  <strong className="history-session-id">{sess.id}</strong>
                  <span className="history-task-id">Task: {sess.taskId}</span>
                </div>
                <div className="history-item-footer">
                  <time>{new Date(sess.startedAt).toLocaleString()}</time>
                  {isCurrent && <span className="active-tag">Current</span>}
                </div>
              </div>
            )
          })
        )}
      </div>
    </aside>
  )
}
