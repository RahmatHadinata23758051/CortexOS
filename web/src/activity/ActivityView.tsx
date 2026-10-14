import { useEffect, useMemo, useState } from 'react'
import { onCockpitEvent } from '../bridge/events'

const events = [
  ['cortexos:orchestra:task:changed', 'task'],
  ['cortexos:tool:proposed', 'approval'], ['cortexos:tool:approved', 'approval'],
  ['cortexos:tool:denied', 'approval'], ['cortexos:tool:executed', 'approval'], ['cortexos:tool:failed', 'approval'],
  ['cortexos:brain:route:accepted', 'system'], ['cortexos:brain:route:rejected', 'system'],
  ['cortexos:brain:route:queued', 'system'], ['cortexos:brain:route:canceled', 'system'],
  ['cortexos:coordination:changed', 'system'], ['cortexos:coordination:assigned', 'system'],
  ['cortexos:coordination:reassigned', 'system'], ['cortexos:coordination:progress', 'system'],
  ['cortexos:coordination:blocked', 'system'], ['cortexos:coordination:completed', 'system'],
  ['cortexos:coordination:canceled', 'system'],
] as const

type Entry = { id: number; name: string; category: string; receivedAt: string; payload: unknown }
type Filter = 'all' | 'task' | 'approval' | 'system'

export function ActivityView() {
  const [entries, setEntries] = useState<Entry[]>([])
  const [filter, setFilter] = useState<Filter>('all')
  useEffect(() => {
    const unsubscribers = events.map(([name, category]) => onCockpitEvent(name, (payload) => {
      setEntries((current) => [{ id: Date.now() + Math.random(), name, category, receivedAt: new Date().toISOString(), payload }, ...current].slice(0, 200))
    }))
    return () => unsubscribers.forEach((unsubscribe) => unsubscribe())
  }, [])
  const visible = useMemo(() => entries.filter((entry) => filter === 'all' || entry.category === filter), [entries, filter])
  return <section className="activity-page reveal-1" aria-labelledby="activity-title">
    <header className="tool-heading"><span className="eyebrow">CORTEXOS / AUDIT STREAM</span><h1 id="activity-title">Activity</h1><p>Live bridge events from this session. The stream is local and clears when this view closes.</p></header>
    <div className="activity-toolbar"><div className="activity-filters" role="group" aria-label="Filter activity">
      {(['all', 'task', 'approval', 'system'] as const).map((item) => <button key={item} type="button" aria-pressed={filter === item} onClick={() => setFilter(item)}>{item[0].toUpperCase() + item.slice(1)}</button>)}
    </div><span className="activity-count">{visible.length} / {entries.length} events</span></div>
    {visible.length === 0 ? <div className="activity-empty"><span className="eyebrow">NO EVENTS YET</span><p>Events will appear here as tasks, approvals, and routing changes arrive.</p></div> :
      <ol className="activity-list">{visible.map((entry) => {
        const payload = entry.payload && typeof entry.payload === 'object' ? entry.payload as Record<string, unknown> : {}
        const correlation = typeof payload.correlationId === 'string' ? payload.correlationId : typeof payload.correlationID === 'string' ? payload.correlationID : ''
        const status = typeof payload.status === 'string' ? payload.status : entry.name.split(':').at(-1) ?? 'event'
        return <li key={entry.id} className="activity-row"><span className={`activity-marker activity-marker-${entry.category}`} aria-hidden="true"/><div className="activity-detail"><div className="activity-event"><code>{entry.name}</code><span className={`activity-status status-${status.toLowerCase()}`}>{status}</span></div>{correlation && <small>Correlation <code>{correlation}</code></small>}</div><time dateTime={entry.receivedAt}>{new Date(entry.receivedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })}</time></li>
      })}</ol>}
  </section>
}
