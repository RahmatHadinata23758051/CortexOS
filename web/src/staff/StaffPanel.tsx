import { useState, useEffect, useMemo, useCallback } from 'react'
import { RefreshCw, UserCheck } from 'lucide-react'
import { type StaffSummary, type StaffStatus } from './types'
import { createStaffBridgeAPI, type StaffAPI } from './api'
import { StaffList } from './StaffList'
import { StaffForm } from './StaffForm'

interface StaffPanelProps {
  api?: StaffAPI
}

export function StaffPanel({ api: propApi }: StaffPanelProps = {}) {
  const api = useMemo(() => propApi ?? createStaffBridgeAPI(), [propApi])
  const [status, setStatus] = useState<StaffStatus>('loading')
  const [staff, setStaff] = useState<StaffSummary[]>([])
  const [selectedStaff, setSelectedStaff] = useState<StaffSummary | undefined>(undefined)
  const [error, setError] = useState<string | undefined>(undefined)

  const loadStaff = useCallback(async () => {
    setStatus('loading')
    const result = await api.ListStaffSummaries()
    if (result.status === 'error') {
      setStatus('error')
      setError(result.error)
      return
    }
    setStatus('success')
    setStaff(result.staff)
    setError(undefined)
  }, [api])

  useEffect(() => {
    const timer = window.setTimeout(() => void loadStaff(), 0)
    return () => window.clearTimeout(timer)
  }, [loadStaff])

  const handleSelect = async (id: string) => {
    const result = await api.GetStaffSummary(id)
    if (result.status === 'error') {
      setError(result.error)
      return
    }
    setSelectedStaff(result.summary)
  }

  const roleLabels: Record<StaffSummary['role'], string> = {
    coordinator: 'Coordinator',
    planner: 'Planner',
    implementer: 'Implementer',
    reviewer: 'Reviewer',
    specialist: 'Specialist',
  }

  const availabilityColors: Record<StaffSummary['availability'], string> = {
    available: 'var(--ok)',
    busy: 'var(--err)',
    unavailable: 'var(--warn)',
    offline: 'var(--text-faint)',
  }

  return (
    <div className="staff-panel">
      <header className="staff-panel-header tool-heading">
        <span className="eyebrow">CORTEXOS / STAFF</span>
        <h1>Staff</h1>
      </header>

      <div className="staff-panel-toolbar">
        <p className="staff-panel-description">Directory and assignment state from the connected runtime.</p>
        <button
          type="button"
          className="btn-toolbar-refresh"
          onClick={() => void loadStaff()}
          disabled={status === 'loading'}
          aria-label="Refresh staff roster"
        >
          <RefreshCw size={13} className={status === 'loading' ? 'spin' : ''} aria-hidden="true" />
          <span>Refresh</span>
        </button>
      </div>

      <div className="staff-panel-content">
        <section className="staff-panel-main" aria-labelledby="staff-roster-title">
          <header className="staff-section-header">
            <h2 id="staff-roster-title">Roster</h2>
            <span>{staff.length} members</span>
          </header>
          <StaffList
            staff={staff}
            selectedId={selectedStaff?.id}
            onSelect={handleSelect}
            loading={status === 'loading'}
            error={error}
          />
        </section>

        <div className="staff-form-column">
          {selectedStaff ? (
            <>
              <StaffForm
                mode="edit"
                initial={selectedStaff}
                onCancel={() => setSelectedStaff(undefined)}
                loading={status === 'loading'}
              />

              <div className="staff-panel-detail">
                <h3>Staff Detail: {selectedStaff.name}</h3>
                <div className="detail-grid">
                  <div className="detail-item">
                    <strong>Name:</strong> {selectedStaff.name}
                  </div>
                  <div className="detail-item">
                    <strong>Role:</strong> {roleLabels[selectedStaff.role]}
                  </div>
                  <div className="detail-item">
                    <strong>Availability:</strong>{' '}
                    <span
                      className="availability-dot"
                      style={{ color: availabilityColors[selectedStaff.availability] }}
                      aria-hidden="true"
                    />{' '}
                    {selectedStaff.availability}
                  </div>
                  <div className="detail-item">
                    <strong>Workspace:</strong> {selectedStaff.workspaceId}
                  </div>
                  <div className="detail-item">
                    <strong>Project:</strong> {selectedStaff.projectId || '—'}
                  </div>
                  <div className="detail-item">
                    <strong>Lifecycle:</strong> {selectedStaff.lifecycle}
                  </div>
                  <div className="detail-item">
                    <strong>Capabilities:</strong> {selectedStaff.capabilities.join(', ') || '—'}
                  </div>
                </div>

                <div className="detail-section">
                  <h4>Skill Injection</h4>
                  <p>Skill injection is not exposed by the current bridge contract (read-only Staff observability).</p>
                </div>
              </div>
            </>
          ) : (
            <div className="staff-inspector-placeholder">
              <UserCheck size={28} className="placeholder-icon" aria-hidden="true" />
              <h3>Staff Inspector</h3>
              <p>Select a staff member from the roster to inspect runtime state, assignments, and capabilities.</p>
              <div className="observability-badge-row">
                <span className="staff-readonly-badge">Observability Bridge</span>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
