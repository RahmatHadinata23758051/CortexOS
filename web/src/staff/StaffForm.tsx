import { useState } from 'react'
import { type StaffSummary } from './types'



interface StaffFormProps {
  mode: 'create' | 'edit'
  initial?: StaffSummary
  onSubmit: (form: {
    name: string
    role: StaffSummary['role']
    capabilities: string[]
    workspaceId: string
    projectId?: string
    worktreeId?: string
    lifecycle: 'active' | 'inactive' | 'retired'
    availability: StaffSummary['availability']
  }) => void
  onCancel: () => void
  loading?: boolean
  error?: string
}

export function StaffForm({ mode, initial, onSubmit, onCancel, loading, error }: StaffFormProps) {
  const [form, setForm] = useState({
    name: initial?.name ?? '',
    role: initial?.role ?? 'implementer',
    capabilities: initial?.capabilities ?? [],
    workspaceId: initial?.workspaceId ?? '',
    projectId: initial?.projectId ?? '',
    worktreeId: initial?.worktreeId ?? '',
    lifecycle: initial?.lifecycle ?? 'active',
    availability: initial?.availability ?? 'available',
  })

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target
    setForm((prev) => ({ ...prev, [name]: value }))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name || !form.workspaceId) {
      setForm((prev) => ({ ...prev, name: prev.name }))
      // Error handled by parent
    }
    onSubmit(form)
  }

  return (
    <form className="staff-form" onSubmit={handleSubmit}>
      <fieldset disabled={loading} style={{ border: 'none', margin: 0, padding: 0 }}>
      <h3>{mode === 'create' ? 'Create Staff Member' : 'Edit Staff Member'}</h3>

      <div className="form-field">
        <label>Name <span className="required">*</span>
          <input
            type="text"
            name="name"
            value={form.name}
            onChange={handleChange}
            placeholder="e.g. Lead Implementer"
            required
          />
        </label>
      </div>

      <div className="form-field">
        <label>Role
          <select name="role" value={form.role} onChange={handleChange} required>
            <option value="coordinator">Coordinator</option>
            <option value="planner">Planner</option>
            <option value="implementer">Implementer</option>
            <option value="reviewer">Reviewer</option>
            <option value="specialist">Specialist</option>
          </select>
        </label>
      </div>

      <div className="form-field">
        <label>Capabilities
          <span className="hint">Comma-separated (e.g. coding, analysis, refactor)</span>
          <input
            type="text"
            name="capabilities"
            value={form.capabilities.join(', ')}
            onChange={handleChange}
            placeholder="coding, analysis, refactor"
          />
        </label>
      </div>

      <div className="form-field">
        <label>Workspace ID <span className="required">*</span>
          <input
            type="text"
            name="workspaceId"
            value={form.workspaceId}
            onChange={handleChange}
            placeholder="ws-main"
            required
          />
        </label>
      </div>

      <div className="form-field">
        <label>Project ID
          <input
            type="text"
            name="projectId"
            value={form.projectId}
            onChange={handleChange}
            placeholder="proj-core"
          />
        </label>
      </div>

      <div className="form-field">
        <label>Worktree ID
          <input
            type="text"
            name="worktreeId"
            value={form.worktreeId}
            onChange={handleChange}
            placeholder="wt-1"
          />
        </label>
      </div>

      <div className="form-field">
        <label>Lifecycle
          <select name="lifecycle" value={form.lifecycle} onChange={handleChange} required>
            <option value="active">Active</option>
            <option value="inactive">Inactive</option>
            <option value="retired">Retired</option>
          </select>
        </label>
      </div>

      <div className="form-field">
        <label>Availability
          <select name="availability" value={form.availability} onChange={handleChange} required>
            <option value="available">Available</option>
            <option value="busy">Busy</option>
            <option value="unavailable">Unavailable</option>
            <option value="offline">Offline</option>
          </select>
        </label>
      </div>

      <div className="form-actions">
        <button type="submit" className="btn-submit" disabled={loading}>
          {loading ? 'Saving…' : mode === 'create' ? 'Create Staff' : 'Update Staff'}
        </button>
        <button
          type="button"
          className="btn-cancel"
          onClick={onCancel}
          disabled={loading}
        >
          Cancel
        </button>
      </div>

      {error && <div className="form-error">{error}</div>}
      </fieldset>
    </form>
  )
}