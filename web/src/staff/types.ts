export type StaffStatus = 'loading' | 'success' | 'error'

export type StaffSummary = {
  id: string
  name: string
  role: 'coordinator' | 'planner' | 'implementer' | 'reviewer' | 'specialist'
  capabilities: string[]
  workspaceId: string
  projectId?: string
  worktreeId?: string
  lifecycle: 'active' | 'inactive' | 'retired'
  availability: 'available' | 'busy' | 'unavailable' | 'offline'
  schemaVersion: string
  createdAt: string
  updatedAt: string
}

export type StaffFormState = {
  name: string
  role: StaffSummary['role']
  capabilities: string[]
  workspaceId: string
  projectId?: string
  worktreeId?: string
  lifecycle: StaffFormLifecycle
  availability: StaffSummary['availability']
}

export type StaffFormLifecycle = 'active' | 'inactive' | 'retire'

export type SkillReference = {
  id: string
  version: string
  name: string
}

export type MemoryReference = {
  id: string
  kind: string
  version: string
}

export type SkillApplicability = {
  allowedRoles: StaffSummary['role'][]
  allowedWorkspaces: string[]
  requiredCapabilities: string[]
  excludedWorkspaces: string[]
}

export type StaffSelection = {
  staffId: string
  staffName: string
  staffRole: StaffSummary['role']
}

export type StaffPanelState = {
  status: StaffStatus
  staff: readonly StaffSummary[]
  selectedStaff?: StaffSummary
  form: StaffFormState
  formMode: 'create' | 'edit'
  skills: readonly SkillReference[]
  selectedSkills: readonly SkillReference[]
  memory: readonly MemoryReference[]
  availabilityTimeline: readonly string[]
  error?: string
}