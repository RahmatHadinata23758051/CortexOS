import { useState, useEffect, useMemo, useCallback } from 'react'
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
  const [formMode, setFormMode] = useState<'create' | 'edit'>('create')
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
    setFormMode('edit')
  }

  const roleLabels: Record<StaffSummary['role'], string> = {
    coordinator: 'Coordinator',
    planner: 'Planner',
    implementer: 'Implementer',
    reviewer: 'Reviewer',
    specialist: 'Specialist',
  }

  const availabilityColors: Record<StaffSummary['availability'], string> = {
    available: '#d7f158',
    busy: '#ff806d',
    unavailable: '#e0b54d',
    offline: '#89939c',
  }

  return (
    <div className="staff-panel">
      <div className="staff-panel-header">
        <h2>Staff Management</h2>
        <span className="staff-panel-subtitle">Logical Staff observability — bridge contracts only</span>
      </div>

      <div className="staff-panel-toolbar">
        <button
          type="button"
          className="btn-toolbar-create"
          onClick={() => {
            setSelectedStaff(undefined)
            setFormMode('create')
          }}
        >
          + Add Staff
        </button>
        {selectedStaff && (
          <button
            type="button"
            className="btn-toolbar-detail"
            onClick={() => setFormMode('edit')}
          >
            Detail
          </button>
        )}
      </div>

      <div className="staff-panel-content">
        {formMode === 'create' || formMode === 'edit' ? (
          <StaffForm
            mode={formMode}
            initial={selectedStaff}
            onSubmit={() => {
              setError('Staff mutations are not permitted from presentation layer (observability-only bridge).')
            }}
            onCancel={() => {
              setSelectedStaff(undefined)
              setFormMode('create')
            }}
            loading={status === 'loading'}
            error={error}
          />
        ) : null}

        <div className="staff-panel-main">
          <StaffList
            staff={staff}
            selectedId={selectedStaff?.id}
            onSelect={handleSelect}
            onEdit={(id) => void handleSelect(id)}
            onAvailabilityChange={() => {
              setError('Staff availability mutation is not permitted from presentation layer.')
            }}
            loading={status === 'loading'}
            error={error}
          />
        </div>

        {selectedStaff && formMode === 'edit' && staff.length > 0 && (
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

            <button
              type="button"
              className="btn-close-detail"
              onClick={() => {
                setSelectedStaff(undefined)
                setFormMode('create')
              }}
            >
              Close
            </button>
          </div>
        )}

        {!selectedStaff && staff.length === 0 && formMode === 'create' && (
          <div className="staff-empty-hint">
            <p>No staff definitions found in this workspace.</p>
          </div>
        )}
      </div>
    </div>
  )
}
