import { type StaffSummary } from './types'

function StaffListItem({ staff, selectedId, onSelect }: {
  staff: StaffSummary
  selectedId?: string
  onSelect: (id: string) => void
}) {
  return (
    <li className="staff-item-wrapper">
      <button
        type="button"
        className={`staff-item ${selectedId === staff.id ? 'selected' : ''}`}
        onClick={() => onSelect(staff.id)}
        aria-pressed={selectedId === staff.id}
      >
        <span className="staff-avatar" aria-hidden="true">
          {staff.name.slice(0, 1).toUpperCase()}
        </span>
        <span className="staff-info">
          <span className="staff-name-row">
            <strong>{staff.name}</strong>
            <span className={`availability-badge ${staff.availability}`}>{staff.availability}</span>
          </span>
          <span className="staff-meta">
            <span className="role-badge">{staff.role}</span>
            <span className="workspace-ref">{staff.workspaceId}</span>
            {staff.projectId && <span className="workspace-ref">{staff.projectId}</span>}
          </span>
        </span>
      </button>
    </li>
  )
}

function EmptyState() {
  return (
    <div className="empty-staff">
      <span className="empty-glyph" aria-hidden="true">∅</span>
      <div>
        <strong>No staff records</strong>
        <p>The connected runtime returned no staff records.</p>
      </div>
    </div>
  )
}

interface StaffListProps {
  staff: readonly StaffSummary[]
  selectedId?: string
  onSelect: (id: string) => void
  loading: boolean
  error?: string
}

export function StaffList({ staff, selectedId, onSelect, loading, error }: StaffListProps) {
  if (loading) {
    return (
      <div className="staff-list-loading" aria-label="Loading staff records">
        <span className="skeleton" style={{ width: '3rem', height: '1rem' }} aria-hidden="true" />
        <span className="skeleton" style={{ width: '12rem', height: '1rem' }} aria-hidden="true" />
        <span className="skeleton" style={{ width: '8rem', height: '1rem' }} aria-hidden="true" />
      </div>
    )
  }

  if (error) {
    return (
      <div className="staff-list-error" role="alert">
        <strong>Runtime error:</strong> {error}
      </div>
    )
  }

  if (staff.length === 0) {
    return <EmptyState />
  }

  return (
    <ul className="staff-list" aria-label="Staff roster">
      {staff.map((member) => (
        <StaffListItem
          key={member.id}
          staff={member}
          selectedId={selectedId}
          onSelect={onSelect}
        />
      ))}
    </ul>
  )
}
