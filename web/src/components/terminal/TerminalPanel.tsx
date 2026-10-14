import { useEffect, useState, useCallback } from 'react'
import type { HarnessBridge, HarnessExecutionSummary } from '../../types/harness'
import { loadActiveHarnessExecutions } from '../../bridge/harness'
import { useTerminal } from '../../hooks/useTerminal'
import { ResourceMonitor } from './ResourceMonitor'
import { SessionCancellation } from './SessionCancellation'
import { SessionHistory } from './SessionHistory'
import { StreamViewer } from './StreamViewer'

export type TerminalPanelProps = {
  bridge?: HarnessBridge
  initialTaskId?: string
  onCancelSession?: (sessionId: string, reason: string) => Promise<void> | void
}

const SAMPLE_LINES = [
  '{"version":"cortexos.worker.v1","type":"handshake","correlationId":"exec-7f91","sequence":1,"payload":{"workerId":"pi-worker-03","workerVersion":"0.4.2","protocol":"cortexos.worker.v1"}}',
  '{"version":"cortexos.worker.v1","type":"progress","correlationId":"exec-7f91","sequence":2,"payload":{"phase":"inspect","percent":35,"message":"Reading bounded task context"}}',
  '{"version":"cortexos.worker.v1","type":"diagnostic","correlationId":"exec-7f91","sequence":3,"payload":{"level":"info","code":"broker.tool_allowed","message":"Tool request admitted by policy"}}',
]

export function TerminalPanel({ bridge, initialTaskId = 'task-terminal-01', onCancelSession }: TerminalPanelProps) {
  const terminal = useTerminal({
    initialSession: { taskId: initialTaskId, workerId: 'broker.pending', status: 'ready' },
    onCancel: onCancelSession,
  })
  const [executions, setExecutions] = useState<HarnessExecutionSummary[]>([])
  const [executionStatus, setExecutionStatus] = useState<'loading' | 'ready' | 'error'>(() => bridge ? 'loading' : 'ready')
  const [showCommandHint, setShowCommandHint] = useState(false)
  const [command, setCommand] = useState('')

  const fetchExecutions = useCallback(() => {
    if (!bridge) return
    void loadActiveHarnessExecutions(bridge).then((result) => {
      if (result.status === 'success') {
        setExecutions(result.executions)
        setExecutionStatus('ready')
      } else setExecutionStatus('error')
    })
  }, [bridge])

  useEffect(() => {
    fetchExecutions()
  }, [fetchExecutions])

  const sendCommand = () => {
    const normalized = command.trim()
    if (!normalized) return
    terminal.appendLine(JSON.stringify({
      version: 'cortexos.worker.v1',
      type: 'diagnostic',
      correlationId: terminal.session.id,
      sequence: terminal.session.lines.length + 1,
      payload: { level: 'info', code: 'terminal.input', message: normalized },
    }))
    setCommand('')
  }

  const loadFixture = () => SAMPLE_LINES.forEach((line) => terminal.appendLine(line))

  return (
    <section className="terminal-panel reveal-1" aria-labelledby="terminal-panel-title">
      <header className="terminal-panel-heading">
        <div className="tool-heading">
          <span className="eyebrow">CORTEXOS / HARNESS</span>
          <h1 id="terminal-panel-title">Terminal Workflow</h1>
        </div>
        <div className="terminal-contract-stamp">
          <span>WIRE CONTRACT</span>
          <strong>cortexos.worker.v1</strong>
        </div>
      </header>

      <div className="terminal-status-strip">
        <div className="terminal-status-cell">
          <span>Session</span>
          <strong><i className={`session-dot ${terminal.session.status}`} />{terminal.session.status.replace('_', ' ')}</strong>
        </div>
        <div className="terminal-status-cell">
          <span>Worker</span>
          <strong>{terminal.session.workerId}</strong>
        </div>
        <div className="terminal-status-cell">
          <span>Task</span>
          <strong>{terminal.session.taskId}</strong>
        </div>
        <div className="terminal-status-cell metric-cell">
          <span>Stream</span>
          <strong>{terminal.session.lines.length}<small> records</small></strong>
        </div>
      </div>

      <div className="terminal-actions-bar">
        <button type="button" className="terminal-action-primary" onClick={() => terminal.startSession(initialTaskId, 'broker.pending')}>
          New Session <span aria-hidden="true">↗</span>
        </button>
        <button type="button" className="terminal-action-secondary" onClick={loadFixture}>Load sample stream</button>
        <div className="terminal-action-spacer" />
        <button type="button" className="terminal-action-secondary" onClick={() => setShowCommandHint(!showCommandHint)}>
          {showCommandHint ? 'Hide input' : 'Send broker input'}
        </button>
        <SessionCancellation
          sessionId={terminal.session.id}
          sessionStatus={terminal.session.status}
          onCancel={terminal.cancelSession}
          error={terminal.cancelError}
        />
      </div>

      {showCommandHint && (
        <div className="terminal-input-seam">
          <label htmlFor="terminal-command-input">Broker input seam</label>
          <div className="terminal-input-row">
            <span className="prompt-symbol">›</span>
            <input
              id="terminal-command-input"
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') sendCommand() }}
              placeholder="Type a redacted broker message…"
            />
            <button type="button" onClick={sendCommand} disabled={!command.trim()}>Send</button>
          </div>
          <p>Input is forwarded through the Harness broker seam. This panel does not execute arbitrary local shell commands.</p>
        </div>
      )}

      <div className="terminal-main-grid">
        <div className="terminal-stream-column">
          <StreamViewer lines={terminal.session.lines} />
        </div>
        <div className="terminal-side-column">
          <ResourceMonitor resources={terminal.session.resources} activeExecutions={executions} />
          {executionStatus === 'error' && <div className="terminal-side-error" role="alert">Harness execution status unavailable.</div>}
          <SessionHistory
            history={terminal.history}
            currentSessionId={terminal.session.id}
            selectedHistoryId={terminal.selectedHistoryId}
            onSelectSession={terminal.selectHistory}
            onArchiveSession={terminal.archiveSession}
          />
        </div>
      </div>
    </section>
  )
}
