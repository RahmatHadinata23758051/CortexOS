import { describe, expect, it } from 'vitest'
import {
  cockpitSchemaVersion,
  isCockpitRuntimeSnapshot,
  isCockpitWorkspaceSnapshot,
  type CockpitRuntimeSnapshot,
  type CockpitWorkspaceSnapshot,
  type CockpitProject,
  type CockpitWorktree,
} from './cockpit'

describe('cockpit type guards and serialization', () => {
  it('validates a correct runtime snapshot', () => {
    const valid: CockpitRuntimeSnapshot = {
      schemaVersion: cockpitSchemaVersion,
      status: 'ready',
      environment: 'local',
      provider: 'disabled',
    }
    expect(isCockpitRuntimeSnapshot(valid)).toBe(true)
  })

  it('rejects unknown runtime status', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      status: 'unknown',
      environment: 'local',
      provider: 'disabled',
    }
    expect(isCockpitRuntimeSnapshot(invalid)).toBe(false)
  })

  it('rejects unknown environment', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      status: 'ready',
      environment: 'cloud',
      provider: 'disabled',
    }
    expect(isCockpitRuntimeSnapshot(invalid)).toBe(false)
  })

  it('rejects unknown provider', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      status: 'ready',
      environment: 'local',
      provider: 'unknown-provider',
    }
    expect(isCockpitRuntimeSnapshot(invalid)).toBe(false)
  })

  it('rejects wrong schema version', () => {
    const invalid = {
      schemaVersion: 'cortexos.cockpit.bridge.v2',
      status: 'ready',
      environment: 'local',
      provider: 'disabled',
    }
    expect(isCockpitRuntimeSnapshot(invalid)).toBe(false)
  })

  it('validates a complete workspace snapshot', () => {
    const valid: CockpitWorkspaceSnapshot = {
      schemaVersion: cockpitSchemaVersion,
      projects: [
        {
          id: 'project-1',
          name: 'Project',
          defaultBranch: 'main',
          status: 'active',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
          schemaVersion: 'cortexos.workspace.v1',
        },
      ],
      worktrees: [
        {
          id: 'worktree-1',
          projectId: 'project-1',
          branch: 'main',
          revision: 'abc123',
          status: 'clean',
          dirty: false,
          activeReference: 'HEAD',
          createdAt: new Date().toISOString(),
          updatedAt: new Date().toISOString(),
        },
      ],
      vaultNoteCount: 42,
      watcherState: 'running',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'indexed',
    }
    expect(isCockpitWorkspaceSnapshot(valid)).toBe(true)
  })

  it('rejects workspace snapshot with missing fields', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      projects: [],
      worktrees: [],
      vaultNoteCount: 'not-a-number',
      watcherState: 'running',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'indexed',
    }
    expect(isCockpitWorkspaceSnapshot(invalid)).toBe(false)
  })

  it('rejects workspace with malformed project', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      projects: [{ id: 'project-1' }],
      worktrees: [],
      vaultNoteCount: 0,
      watcherState: 'running',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'indexed',
    }
    expect(isCockpitWorkspaceSnapshot(invalid)).toBe(false)
  })

  it('validates project with optional defaultBranch', () => {
    const project: CockpitProject = {
      id: 'project-1',
      name: 'Project',
      status: 'active',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      schemaVersion: 'cortexos.workspace.v1',
    }
    const snapshot: CockpitWorkspaceSnapshot = {
      schemaVersion: cockpitSchemaVersion,
      projects: [project],
      worktrees: [],
      vaultNoteCount: 0,
      watcherState: 'running',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'indexed',
    }
    expect(isCockpitWorkspaceSnapshot(snapshot)).toBe(true)
  })

  it('validates worktree with optional fields', () => {
    const worktree: CockpitWorktree = {
      id: 'worktree-1',
      projectId: 'project-1',
      status: 'clean',
      dirty: false,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    }
    const snapshot: CockpitWorkspaceSnapshot = {
      schemaVersion: cockpitSchemaVersion,
      projects: [],
      worktrees: [worktree],
      vaultNoteCount: 0,
      watcherState: 'running',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'indexed',
    }
    expect(isCockpitWorkspaceSnapshot(snapshot)).toBe(true)
  })

  it('requires watcherState as string', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      projects: [],
      worktrees: [],
      vaultNoteCount: 0,
      watcherState: 123,
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'indexed',
    }
    expect(isCockpitWorkspaceSnapshot(invalid)).toBe(false)
  })

  it('requires retrievalState as string', () => {
    const invalid = {
      schemaVersion: cockpitSchemaVersion,
      projects: [],
      worktrees: [],
      vaultNoteCount: 0,
      watcherState: 'running',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: null,
    }
    expect(isCockpitWorkspaceSnapshot(invalid)).toBe(false)
  })
})