import { useEffect, useRef, useState, useCallback, type ReactNode } from 'react'
import { type StaffSummary, type OfficeActivityEvent, type TerminalOverlayState } from '../types/office'
import { loadOfficeSnapshot, createOfficeEventSource, staffById, type OfficeState } from '../bridge/office'
import { wailsRuntimeBridge } from '../types'

interface OfficeSceneHandle {
  setOfficeState(state: { staff: readonly StaffSummary[]; activity: readonly OfficeActivityEvent[]; onStaffSelect?: (id: string) => void }): void
}

interface OfficeGameHandle {
  scene: {
    getScene(key: string): OfficeSceneHandle
  }
  destroy(removeCanvas: boolean): void
}

interface OfficeViewProps {
  className?: string
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

function TerminalOverlay({ state, onClose }: { state: TerminalOverlayState; onClose: () => void }) {
  if (!state.visible) return null
  return (
    <div className="terminal-overlay" role="dialog" aria-modal="true" aria-label="Staff terminal">
      <header className="terminal-header">
        <h4 className="terminal-title">{state.title ?? `Staff: ${state.staffId}`}</h4>
        <button className="terminal-close" onClick={onClose} aria-label="Close terminal">✕</button>
      </header>
      <pre className="terminal-body" aria-live="polite">
        {state.lines.map((line, i) => <div key={i}>{line}</div>)}
      </pre>
    </div>
  )
}

function ReadOnlyBanner() {
  return (
    <div className="readonly-banner" aria-live="polite">
      <span className="badge">READ-ONLY</span>
      <span>Phaser consumes typed bridge/event state — no domain mutation</span>
    </div>
  )
}

function OfficeCanvas({ staff, onSelect }: { staff: readonly StaffSummary[]; onSelect: (id: string) => void }) {
  const canvasRef = useRef<HTMLDivElement>(null)
  const gameRef = useRef<OfficeGameHandle | null>(null)
  const sceneRef = useRef<OfficeSceneHandle | null>(null)

  useEffect(() => {
    let mounted = true
    void import('../office/OfficeScene').then(({ createOfficeGame }) => {
      if (!mounted || !canvasRef.current) return
      const game = createOfficeGame(canvasRef.current, { staff, activity: [], onStaffSelect: onSelect }) as unknown as OfficeGameHandle
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
  }, [onSelect, staff])

  useEffect(() => {
    if (sceneRef.current) {
      sceneRef.current.setOfficeState({ staff, activity: [], onStaffSelect: onSelect })
    }
  }, [staff, onSelect])

  return <div ref={canvasRef} className="office-canvas" role="img" aria-label="Virtual office canvas" />
}

export function OfficeView({ className = '' }: OfficeViewProps): ReactNode {
  const [officeState, setOfficeState] = useState<OfficeState>({ status: 'loading' })
  const [selectedStaffId, setSelectedStaffId] = useState<string | undefined>()
  const [activityEvents, setActivityEvents] = useState<readonly OfficeActivityEvent[]>([])
  const [terminalState, setTerminalState] = useState<TerminalOverlayState>({ visible: false, lines: [] })
  const eventSource = useRef(createOfficeEventSource())

  const handleStaffSelect = useCallback((staffId: string) => {
    setSelectedStaffId(staffId)
    const member = staffById(officeState.status === 'success' ? officeState.snapshot.staff : [], staffId)
    if (member) {
      setTerminalState({ visible: true, staffId: member.id, title: `${member.name} · ${member.role}`, lines: ['Read-only activity stream connected.', `Role: ${member.role}`, `Availability: ${member.availability}`] })
    }
  }, [officeState])

  const closeTerminal = useCallback(() => setTerminalState({ visible: false, lines: [] }), [])

  useEffect(() => {
    let active = true
    loadOfficeSnapshot(wailsRuntimeBridge).then((state) => { if (active) setOfficeState(state) })
    const unsubscribe = eventSource.current.subscribe((event) => { setActivityEvents((prev) => [...prev.slice(-99), event]) })
    return () => { active = false; unsubscribe() }
  }, [])

  if (officeState.status === 'loading') return <div className={`office-view ${className}`}><div className="office-loading">Loading virtual office…</div></div>
  if (officeState.status === 'error') return <div className={`office-view ${className}`}><div className="office-error">{officeState.code}: {officeState.message}</div></div>

  const staff = officeState.snapshot.staff

  return (
    <div className={`office-view ${className}`}>
      <ReadOnlyBanner />
      <div className="office-grid">
        <main className="office-main">
          <OfficeCanvas staff={staff} onSelect={handleStaffSelect} />
        </main>
        <div className="office-side">
          <StaffDirectory staff={staff} selectedId={selectedStaffId} onSelect={handleStaffSelect} />
          <ZoneLegend />
          <ActivityFeed events={activityEvents} />
        </div>
      </div>
      <TerminalOverlay state={terminalState} onClose={closeTerminal} />
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