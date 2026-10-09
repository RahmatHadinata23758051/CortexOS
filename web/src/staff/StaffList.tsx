import { type StaffSummary } from './types'

function StaffListItem({ staff, selectedId, onSelect, onEdit, onAvailabilityChange }: {
  staff: StaffSummary
  selectedId?: string
  onSelect: (id: string) => void
  onEdit: (id: string) => void
  onAvailabilityChange: (id: string, availability: StaffSummary['availability']) => void
}) {
  const availabilityColor = {
    available: '#d7f158',
    busy: '#ff806d',
    unavailable: '#e0b54d',
    offline: '#89939c',
  }

  return (
    <li
      key={staff.id}
      className={`staff-item ${selectedId === staff.id ? 'selected' : ''}`}
      onClick={() => onSelect(staff.id)}
      style={{ borderColor: availabilityColor[staff.availability] }}
    >
      <div className="staff-avatar" aria-hidden="true">
        <span className="staff-initial">{staff.name.slice(0, 1).toUpperCase()}</span>
      </div>
      <div className="staff-info">
        <div className="staff-name-row">
          <strong>{staff.name}</strong>
          <span className={`availability-badge ${staff.availability}`}>{staff.availability}</span>
        </div>
        <div className="staff-meta">
          <span className={`role-badge ${staff.role}`}>{staff.role}</span>
          <span className="workspace-ref">{staff.workspaceId}</span>
          {staff.projectId && <span className="workspace-ref">• {staff.projectId}</span>}
        </div>
      </div>
      <div className="staff-actions">
        <button
          className="btn-edit"
          onClick={(e) => {
            e.stopPropagation()
            onEdit(staff.id)
          }}
          aria-label={`Edit ${staff.name}`}
        >
          ✎
        </button>
        <button
          className="btn-availability"
          onClick={(e) => {
            e.stopPropagation()
            // Cycle availability: available -> busy -> unavailable -> offline -> available
            const cycle: StaffSummary['availability'][] = ['available', 'busy', 'unavailable', 'offline']
            const currentIndex = cycle.indexOf(staff.availability)
            const nextIndex = (currentIndex + 1) % cycle.length
            onAvailabilityChange(staff.id, cycle[nextIndex])
          }}
          aria-label={`Change availability for ${staff.name}`}
        >
          ↻
        </button>
      </div>
    </li>
  )
}

function EmptyState() {
  return (
    <div className="empty-staff">
      <span className="empty-glyph" aria-hidden="true">∅</span>
      <div>
        <strong>No staff members</strong>
        <p>Add your first staff member using the form below.</p>
      </div>
    </div>
  )
}

interface StaffListProps {
  staff: readonly StaffSummary[]
  selectedId?: string
  onSelect: (id: string) => void
  onEdit: (id: string) => void
  onAvailabilityChange: (id: string, availability: StaffSummary['availability']) => void
  loading: boolean
  error?: string
}

export function StaffList({ staff, selectedId, onSelect, onEdit, onAvailabilityChange, loading, error }: StaffListProps) {
  if (loading) {
    return (
      <div className="staff-list-loading">
        <span className="skeleton" style={{ width: '3rem', height: '1rem' }} aria-hidden="true" />
        <span className="skeleton" style={{ width: '12rem', height: '1rem' }} aria-hidden="true" />
        <span className="skeleton" style={{ width: '8rem', height: '1rem' }} aria-hidden="true" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="staff-list-error">
        <strong>Error:</strong> {error}
      </div>
    )
  }

  if (staff.length === 0) {
    return EmptyState()
  }

  return (
    <ul className="staff-list">
      {staff.map((member) => (
        <StaffListItem
          key={member.id}
          staff={member}
          selectedId={selectedId}
          onSelect={onSelect}
          onEdit={onEdit}
          onAvailabilityChange={onAvailabilityChange}
        />
      ))}
    </ul>
  )
}