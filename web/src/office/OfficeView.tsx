import { useEffect, useRef, useState, useCallback, type ReactNode } from 'react'
import { BrainCircuit, FolderKanban, UsersRound, X } from 'lucide-react'
import { useBrainDispatch } from '../hooks/useBrainDispatch'
import { type StaffSummary, type StaffBrainState, type OfficeActivityEvent, type TerminalOverlayState } from '../types/office'
import { type CoordinationSnapshot, type StaffAssignmentStatus } from '../types/coordination'
import { loadOfficeSnapshot, createOfficeEventSource, staffById, type OfficeState } from '../bridge/office'
import { loadCoordinationSnapshot, onCoordinationChanged } from '../bridge/coordination'
import { CoordinationPanel } from './CoordinationPanel'
import { OrchestrationGraph } from './OrchestrationGraph'
import { OrchestraPromptHud } from './OrchestraPromptHud'
import { WorkerCardsPanel } from './WorkerCardsPanel'
import { wailsRuntimeBridge } from '../types'

interface OfficeSceneHandle {
  setOfficeState(state: { staff: readonly StaffSummary[]; activity: readonly OfficeActivityEvent[]; brainState?: StaffBrainState; coordination?: readonly StaffAssignmentStatus[]; onStaffSelect?: (id: string) => void }): void
}

interface OfficeGameHandle {
  scene: {
    getScene(key: string): OfficeSceneHandle
  }
  destroy(removeCanvas: boolean): void
}

interface OfficeViewProps {
  className?: string
  projectId?: string
}

function StaffDirectory({ staff, selectedId, onSelect }: { staff: readonly StaffSummary[]; selectedId?: string; onSelect: (id: string) => void }) {
  return (
    <aside className="office-directory" aria-label="Staff directory">
      <header className="directory-header">
        <h3 className="directory-title">Staff</h3>
        <span className="directory-count">{staff.length}</span>
      </header>
      <ul className="staff-list">
        {staff.map((member) => (
          <li key={member.id} className={`staff-item ${selectedId === member.id ? 'selected' : ''}`} onClick={() => onSelect(member.id)}>
            <div className="staff-avatar" style={{ borderColor: availabilityColor(member.availability) }} aria-hidden="true">
              <span className="staff-initial">{member.name.slice(0, 1).toUpperCase()}</span>
            </div>
            <div className="staff-info">
              <div className="staff-name-row">
                <strong>{member.name}</strong>
                <span className={`availability-badge ${member.availability}`}>{member.availability}</span>
              </div>
              <div className="staff-meta">
                <span className={`role-badge ${member.role}`}>{member.role}</span>
                <span className="workspace-ref">{member.workspaceId}</span>
              </div>
            </div>
          </li>
        ))}
      </ul>
    </aside>
  )
}

function ZoneLegend() {
  return (
    <aside className="zone-legend" aria-label="Room zones">
      <h3 className="legend-title">Zones</h3>
      <ul className="zone-list">
        {[
          { id: 'coordination', label: 'Coordination', color: '#273445' },
          { id: 'planning', label: 'Planning', color: '#344b4e' },
          { id: 'delivery', label: 'Delivery', color: '#4d4544' },
          { id: 'review', label: 'Review', color: '#3e465a' },
        ].map((zone) => (
          <li key={zone.id} className="zone-item">
            <span className="zone-swatch" style={{ background: zone.color }} aria-hidden="true" />
            <span className="zone-label">{zone.label}</span>
          </li>
        ))}
      </ul>
    </aside>
  )
}

function ActivityFeed({ events }: { events: readonly OfficeActivityEvent[] }) {
  return (
    <section className="activity-feed" aria-live="polite" aria-label="Activity feed">
      <header className="feed-header"><h3 className="feed-title">Activity</h3></header>
      <ul className="feed-list" role="log">
        {events.slice().reverse().map((event) => (
          <li key={event.id} className={`feed-item ${event.severity}`}>
            <time className="feed-time">{new Date(event.timestamp).toLocaleTimeString()}</time>
            <span className="feed-kind">{event.kind.toUpperCase()}</span>
            <span className="feed-label">{event.label}</span>
            {event.staffId && <span className="feed-staff">@{event.staffId}</span>}
          </li>
        ))}
        {events.length === 0 && <li className="feed-empty">No activity yet.</li>}
      </ul>
    </section>
  )
}

function TerminalOverlay({ state, onClose, onAskBrain }: { state: TerminalOverlayState; onClose: () => void; onAskBrain?: (staffId: string) => void }) {
  if (!state.visible) return null
  return (
    <div className="terminal-overlay" role="dialog" aria-modal="true" aria-label="Staff terminal">
      <header className="terminal-header">
        <h4 className="terminal-title">{state.title ?? `Staff: ${state.staffId}`}</h4>
        <div className="terminal-actions">
          {onAskBrain && state.staffId && (
            <button className="ask-brain-button" onClick={() => onAskBrain(state.staffId!)} aria-label={`Ask Brain with ${state.staffId}`}>
              <BrainCircuit size={14} aria-hidden="true" />
              <span>Ask Brain</span>
            </button>
          )}
          <button className="terminal-close" onClick={onClose} aria-label="Close terminal">
            <X size={14} aria-hidden="true" />
          </button>
        </div>
      </header>
      <pre className="terminal-body" aria-live="polite">
        {state.lines.map((line, i) => <div key={i}>{line}</div>)}
      </pre>
    </div>
  )
}

function OfficeCanvas({ staff, onSelect, brainState, coordination }: { staff: readonly StaffSummary[]; onSelect: (id: string) => void; brainState?: StaffBrainState; coordination?: readonly StaffAssignmentStatus[] }) {
  const canvasRef = useRef<HTMLDivElement>(null)
  const gameRef = useRef<OfficeGameHandle | null>(null)
  const sceneRef = useRef<OfficeSceneHandle | null>(null)
  // Event-driven Phaser update: Phaser only rebuilds its internal GameObjects when
  // the lifecycle phase or staffId changes (via phaseKey). Per-token output changes
  // are throttled by not including `brainState.speechText` in the key.
  const phaseKey = brainState ? `${brainState.staffId ?? 'none'}:${brainState.state}` : 'idle'
  // Coordination signature is derived from bounded assignment state (idle/running/
  // blocked/awaiting_inspection/completed/failed) and current task, so avatars update
  // on governed lifecycle events rather than per-token frequency.
  const coordinationKey = coordination ? coordination.map((item) => `${item.staffId}:${item.state}:${item.currentTaskId ?? ''}`).join('|') : ''
  const sceneKey = `${phaseKey}#${coordinationKey}`

  // Phaser game instance is created once; updates go through setOfficeState.
  useEffect(() => {
    let mounted = true
    void import('../office/OfficeScene').then(({ createOfficeGame }) => {
      if (!mounted || !canvasRef.current) return
      const game = createOfficeGame(canvasRef.current, { staff, activity: [], brainState, coordination, onStaffSelect: onSelect }) as unknown as OfficeGameHandle
      gameRef.current = game
      sceneRef.current = game.scene.getScene('office')
    })
    return () => {
      mounted = false
      if (gameRef.current) {
        gameRef.current.destroy(true)
        gameRef.current = null
        sceneRef.current = null
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // Only trigger setOfficeState when sceneKey (staffId + lifecycle + coordination) changes.
  useEffect(() => {
    if (sceneRef.current) {
      sceneRef.current.setOfficeState({ staff, activity: [], brainState, coordination, onStaffSelect: onSelect })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sceneKey])

  return <div ref={canvasRef} className="office-canvas" role="img" aria-label="Virtual office canvas" />
}

export function OfficeView({ className = '', projectId }: OfficeViewProps): ReactNode {
  const [viewMode, setViewMode] = useState<'graph' | 'office'>('graph')
  const [officeState, setOfficeState] = useState<OfficeState>({ status: 'loading' })
  const [selectedStaffId, setSelectedStaffId] = useState<string | undefined>()
  const [activityEvents, setActivityEvents] = useState<readonly OfficeActivityEvent[]>([])
  const [terminalState, setTerminalState] = useState<TerminalOverlayState>({ visible: false, lines: [] })
  const [coordination, setCoordination] = useState<{ status: 'loading' } | { status: 'success'; snapshot: CoordinationSnapshot } | { status: 'error'; message: string }>({ status: 'loading' })
  const eventSource = useRef(createOfficeEventSource())

  // Brain hook drives the local reasoning session and per-staff activity.
  const { state: brainState, output, isRunning, run, reset } = useBrainDispatch({ bridge: wailsRuntimeBridge });

  const handleStaffSelect = useCallback((staffId: string) => {
    setSelectedStaffId(staffId)
    const member = staffById(officeState.status === 'success' ? officeState.snapshot.staff : [], staffId)
    if (member) {
      setTerminalState({ visible: true, staffId: member.id, title: `${member.name} · ${member.role}`, lines: ['Read-only activity stream connected.', `Role: ${member.role}`, `Availability: ${member.availability}`] })
    }
  }, [officeState])

  const handleAskBrain = useCallback((staffId: string) => {
    const member = staffById(officeState.status === 'success' ? officeState.snapshot.staff : [], staffId)
    if (!member) return
    const userPrompt = `As ${member.name} (${member.role}), review the current project state and suggest the next actionable step.`
    reset()
    void run({ userPrompt, staffId: member.id })
  }, [officeState, run, reset])

  const closeTerminal = useCallback(() => setTerminalState({ visible: false, lines: [] }), [])

  const brainStateForPhaser: StaffBrainState | undefined = isRunning || brainState !== 'idle'
    ? { staffId: selectedStaffId, state: brainState, speechText: isRunning ? output.slice(-120) || 'Thinking…' : undefined }
    : undefined

  useEffect(() => {
    let active = true
    loadOfficeSnapshot(wailsRuntimeBridge).then((state) => { if (active) setOfficeState(state) })
    const unsubscribe = eventSource.current.subscribe((event) => { setActivityEvents((prev) => [...prev.slice(-99), event]) })
    return () => { active = false; unsubscribe() }
  }, [])

  // Coordination snapshot is loaded once and refreshed on lifecycle events rather
  // than per-token, keeping Office updates event-driven and bounded.
  useEffect(() => {
    let active = true
    const refresh = () => {
      loadCoordinationSnapshot(wailsRuntimeBridge, projectId).then((state) => {
        if (!active) return
        if (state.status === 'success') setCoordination({ status: 'success', snapshot: state.snapshot })
        else if (state.status === 'error') setCoordination({ status: 'error', message: state.message })
      })
    }
    refresh()
    const unsubscribe = onCoordinationChanged(refresh)
    return () => { active = false; unsubscribe() }
  }, [projectId])

  if (officeState.status === 'loading') return <div className={`office-view ${className}`}><div className="office-loading">Loading virtual office…</div></div>
  if (officeState.status === 'error') return <div className={`office-view ${className}`}><div className="office-error">{officeState.code}: {officeState.message}</div></div>

  const staff = officeState.snapshot.staff

  return (
    <div className={`office-view ${className}`}>
      <div className="office-mode-switcher" role="tablist" aria-label="Office presentation mode">
        <button type="button" className={viewMode === 'graph' ? 'active' : ''} onClick={() => setViewMode('graph')} role="tab" aria-selected={viewMode === 'graph'}>
          <FolderKanban size={14} aria-hidden="true" />
          <span>Orchestration graph</span>
        </button>
        <button type="button" className={viewMode === 'office' ? 'active' : ''} onClick={() => setViewMode('office')} role="tab" aria-selected={viewMode === 'office'}>
          <UsersRound size={14} aria-hidden="true" />
          <span>Virtual office</span>
        </button>
      </div>
      {viewMode === 'graph' ? (
        <div className="office-graph-layout">
          <aside className="office-prompt-column">
            {projectId && (
              <>
                <OrchestraPromptHud
                  projectId={projectId}
                  bridge={wailsRuntimeBridge}
                />
                <WorkerCardsPanel
                  projectId={projectId}
                  bridge={wailsRuntimeBridge}
                />
              </>
            )}
          </aside>
          <main className="office-graph-main">
            {coordination.status === 'success' ? (
              <OrchestrationGraph
                snapshot={coordination.snapshot}
                staff={staff}
                onSelectStaff={handleStaffSelect}
              />
            ) : coordination.status === 'error' ? (
              <div className="office-error">{coordination.message}</div>
            ) : (
              <div className="office-loading">Loading orchestration graph…</div>
            )}
          </main>
        </div>
      ) : (
        <div className="office-grid">
          <main className="office-main">
            <OfficeCanvas
              staff={staff}
              onSelect={handleStaffSelect}
              brainState={brainStateForPhaser}
              coordination={coordination.status === 'success' ? coordination.snapshot.assignments : undefined}
            />
          </main>
          <div className="office-side">
            {coordination.status === 'success' && <CoordinationPanel snapshot={coordination.snapshot} />}
            <StaffDirectory staff={staff} selectedId={selectedStaffId} onSelect={handleStaffSelect} />
            <ZoneLegend />
            <ActivityFeed events={activityEvents} />
          </div>
        </div>
      )}
      <TerminalOverlay
        state={terminalState}
        onClose={closeTerminal}
        onAskBrain={handleAskBrain}
      />
    </div>
  )
}

function availabilityColor(avail: StaffSummary['availability']) {
  switch (avail) {
    case 'available': return '#d7f158'
    case 'busy': return '#ff806d'
    case 'unavailable': return '#e0b54d'
    case 'offline': return '#89939c'
  }
}