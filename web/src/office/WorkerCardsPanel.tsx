import { useCallback, useEffect, useState } from 'react'
import { Activity, AlertTriangle, Clock, Cpu, RefreshCw } from 'lucide-react'
import {
  type WorkerCard,
  type WorkerLifecycleBridge,
  type WorkerLifecycleState,
} from '../types/worker-lifecycle'
import { loadWorkerCardsSnapshot } from '../bridge/worker-lifecycle'
import { onOrchestraPlanLifecycle } from '../bridge/orchestra-events'

export type WorkerCardsPanelProps = {
  projectId: string
  bridge: WorkerLifecycleBridge
}

type PanelState =
  | { status: 'loading' }
  | { status: 'success'; workers: WorkerCard[]; generatedAt: string }
  | { status: 'error'; message: string }

// ponytail: worker cards remain a compact side panel. Upgrade path: promote cards into graph nodes after backend supplies stable layout metadata.
export function WorkerCardsPanel({ projectId, bridge }: WorkerCardsPanelProps) {
  const [state, setState] = useState<PanelState>({ status: 'loading' })

  const loadWorkers = useCallback(async (active = true) => {
    try {
      const result = await loadWorkerCardsSnapshot(bridge, projectId, false)
      if (!active) return
      setState({ status: 'success', workers: result.workers, generatedAt: result.generatedAt })
    } catch (error) {
      if (!active) return
      setState({ status: 'error', message: error instanceof Error ? error.message : 'Failed to query worker cards.' })
    }
  }, [bridge, projectId])

  useEffect(() => {
    let active = true
    void Promise.resolve().then(() => loadWorkers(active))
    const unsubscribe = onOrchestraPlanLifecycle(() => {
      void loadWorkers(active)
    })
    return () => {
      active = false
      unsubscribe()
    }
  }, [loadWorkers])

  const activeWorkers = state.status === 'success'
    ? state.workers.filter((worker) => worker.state !== 'unspawned' && worker.state !== 'accepted')
    : []

  return (
    <aside className="worker-cards-panel" aria-label="Ephemeral active workers">
      <header className="worker-cards-header">
        <div className="worker-cards-title-group">
          <Cpu size={16} aria-hidden="true" />
          <h3 className="worker-cards-title">Ephemeral Workers</h3>
          <span className="worker-cards-count" aria-live="polite">
            {state.status === 'success' ? activeWorkers.length : '…'}
          </span>
        </div>
        <button
          type="button"
          className="worker-cards-refresh"
          onClick={() => { setState({ status: 'loading' }); void loadWorkers(true) }}
          aria-label="Refresh active workers"
          title="Refresh active workers"
        >
          <RefreshCw size={13} aria-hidden="true" />
        </button>
      </header>

      {state.status === 'loading' && (
        <div className="worker-cards-loading" role="status">
          Loading worker cards…
        </div>
      )}

      {state.status === 'error' && (
        <div className="worker-cards-error" role="alert">
          <AlertTriangle size={14} aria-hidden="true" />
          <span>{state.message}</span>
        </div>
      )}

      {state.status === 'success' && activeWorkers.length === 0 && (
        <div className="worker-cards-empty">
          <Activity size={18} aria-hidden="true" />
          <p>No active ephemeral workers. Workers spawn upon task dispatch and unspawn after acceptance.</p>
        </div>
      )}

      {state.status === 'success' && activeWorkers.length > 0 && (
        <ul className="worker-cards-list" role="list">
          {activeWorkers.map((worker) => (
            <li key={worker.workerId} className={`worker-card state-${worker.state}`}>
              <div className="worker-card-header">
                <span className="worker-card-role">{worker.role || 'worker'}</span>
                <span className={`worker-state-badge state-${worker.state}`}>
                  {formatStateLabel(worker.state)}
                </span>
              </div>
              <div className="worker-card-main">
                <strong className="worker-card-callsign">{worker.callSign || worker.workerId}</strong>
                <span className="worker-card-task">Task: {worker.taskId}</span>
              </div>
              {worker.reason && (
                <small className="worker-card-reason" title={worker.reason}>
                  {worker.reason}
                </small>
              )}
              <footer className="worker-card-footer">
                <span className="worker-card-time">
                  <Clock size={11} aria-hidden="true" />
                  {new Date(worker.updatedAt).toLocaleTimeString()}
                </span>
                <span className="worker-card-id">{worker.workerId}</span>
              </footer>
            </li>
          ))}
        </ul>
      )}
    </aside>
  )
}

function formatStateLabel(state: WorkerLifecycleState): string {
  switch (state) {
    case 'spawned':
      return 'Spawned'
    case 'running':
      return 'Running'
    case 'awaiting_approval':
      return 'Awaiting Approval'
    case 'reviewing':
      return 'Reviewing'
    case 'blocked':
      return 'Blocked'
    case 'failed':
      return 'Failed'
    case 'accepted':
      return 'Accepted'
    case 'unspawned':
      return 'Unspawned'
    default:
      return state
  }
}
