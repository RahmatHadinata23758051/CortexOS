import { type StaffSummary } from './types'

interface StaffFormProps {
  mode: 'edit'
  initial: StaffSummary
  onCancel: () => void
  loading?: boolean
}

export function StaffForm({ mode, initial, onCancel, loading }: StaffFormProps) {
  const capabilities = initial.capabilities.join(', ')

  return (
    <form className="staff-form" onSubmit={(event) => event.preventDefault()}>
      <fieldset disabled={loading} className="staff-form-fields">
        <header className="staff-form-header">
          <h3>{mode === 'edit' ? 'Staff Record' : 'Staff Registry'}</h3>
          <span className="staff-readonly-badge">Read-only</span>
        </header>
        <p className="staff-form-notice">Records are managed by the connected runtime. Changes from this view are disabled.</p>

        <div className="form-field">
          <label htmlFor="staff-name">Name</label>
          <input id="staff-name" type="text" value={initial.name} placeholder="—" readOnly />
        </div>
        <div className="form-field">
          <label htmlFor="staff-role">Role</label>
          <input id="staff-role" type="text" value={initial.role} readOnly />
        </div>
        <div className="form-field">
          <div className="field-label-row">
            <label htmlFor="staff-capabilities">Capabilities</label>
            <span className="hint">Runtime managed</span>
          </div>
          <input id="staff-capabilities" type="text" value={capabilities || '—'} readOnly />
        </div>
        <div className="form-field">
          <label htmlFor="staff-workspace-id">Workspace ID</label>
          <input id="staff-workspace-id" type="text" value={initial.workspaceId} placeholder="—" readOnly />
        </div>
        <div className="form-field">
          <label htmlFor="staff-project-id">Project ID</label>
          <input id="staff-project-id" type="text" value={initial.projectId || '—'} readOnly />
        </div>
        <div className="form-field">
          <label htmlFor="staff-worktree-id">Worktree ID</label>
          <input id="staff-worktree-id" type="text" value={initial.worktreeId || '—'} readOnly />
        </div>
        <div className="form-field">
          <label htmlFor="staff-lifecycle">Lifecycle</label>
          <input id="staff-lifecycle" type="text" value={initial.lifecycle} readOnly />
        </div>
        <div className="form-field">
          <label htmlFor="staff-availability">Availability</label>
          <input id="staff-availability" type="text" value={initial.availability} readOnly />
        </div>
        <div className="form-actions">
          <button type="button" className="btn-cancel" onClick={onCancel}>Close inspector</button>
        </div>
      </fieldset>
    </form>
  )
}
