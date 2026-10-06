import { describe, expect, it, vi } from 'vitest'

import { loadWorkspaceSnapshot, queryWorkspace } from './workspace'
import { workspaceSchemaVersion, type WorkspaceBridge } from '../types'

const bridge = (): WorkspaceBridge => ({
  GetWorkspaceSnapshot: vi.fn().mockResolvedValue({
    schemaVersion: workspaceSchemaVersion,
    projects: [],
    worktrees: [],
    vaultNoteCount: 0,
    watcherState: 'unavailable',
    retrievalVersion: 'cortexos.retrieval.v1',
    retrievalState: 'unknown',
  }),
  QueryWorkspace: vi.fn().mockResolvedValue([]),
  RebuildWorkspaceRetrieval: vi.fn().mockResolvedValue(undefined),
  RegisterWorkspaceProject: vi.fn(),
})

describe('workspace bridge', () => {
  it('loads and validates a typed snapshot', async () => {
    const runtime = bridge()
    await expect(loadWorkspaceSnapshot(runtime)).resolves.toMatchObject({ status: 'success' })
    expect(runtime.GetWorkspaceSnapshot).toHaveBeenCalledWith({ schemaVersion: workspaceSchemaVersion })
  })

  it('rejects malformed snapshots as structured errors', async () => {
    const runtime = bridge()
    vi.mocked(runtime.GetWorkspaceSnapshot).mockResolvedValue({
      schemaVersion: workspaceSchemaVersion,
      projects: [{}],
      worktrees: [],
      vaultNoteCount: 0,
      watcherState: 'unavailable',
      retrievalVersion: 'cortexos.retrieval.v1',
      retrievalState: 'unknown',
    } as never)
    await expect(loadWorkspaceSnapshot(runtime)).resolves.toMatchObject({
      status: 'error',
      code: 'workspace.internal',
    })
  })

  it('returns structured errors without requiring a raw exception', async () => {
    const runtime = bridge()
    vi.mocked(runtime.GetWorkspaceSnapshot).mockRejectedValue({
      code: 'workspace.path_denied',
      message: 'workspace path was denied',
    })
    await expect(loadWorkspaceSnapshot(runtime)).resolves.toEqual({
      status: 'error',
      code: 'workspace.path_denied',
      message: 'workspace path was denied',
    })
  })

  it('keeps query requests versioned and bounded by the caller contract', async () => {
    const runtime = bridge()
    await queryWorkspace(runtime, 'project-1', 'storage', 5)
    expect(runtime.QueryWorkspace).toHaveBeenCalledWith({
      schemaVersion: workspaceSchemaVersion,
      projectId: 'project-1',
      query: 'storage',
      limit: 5,
    })
  })
})
