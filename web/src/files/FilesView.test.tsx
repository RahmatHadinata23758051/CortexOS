import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import { FilesView } from './FilesView'
import { loadFilesListing, readFileContent } from '../bridge/files'
import type { FilesBridge, FilesListResult, FilesReadResult } from '../types'
import type { WorkspaceState } from '../bridge/workspace'
import { filesBridgeSchemaVersion } from '../types'

const mockWorkspace: WorkspaceState = {
  status: 'success',
  snapshot: {
    schemaVersion: 'cortexos.workspace.v1',
    projects: [
      {
        id: 'prj-alpha',
        name: 'Project Alpha',
        status: 'ready',
        defaultBranch: 'main',
        createdAt: '2026-10-14T00:00:00Z',
        updatedAt: '2026-10-14T00:00:00Z',
        schemaVersion: 'cortexos.workspace.v1',
      },
    ],
    worktrees: [
      {
        id: 'wt-main',
        projectId: 'prj-alpha',
        branch: 'main',
        status: 'ready',
        dirty: false,
        createdAt: '2026-10-14T00:00:00Z',
        updatedAt: '2026-10-14T00:00:00Z',
      },
    ],
    vaultNoteCount: 12,
    watcherState: 'ready',
    retrievalVersion: '1.0',
    retrievalState: 'ready',
  },
}

const mockListResult: FilesListResult = {
  projectId: 'prj-alpha',
  worktreeId: 'wt-main',
  area: 'repository',
  rootLabel: 'Repository',
  entries: [
    {
      path: 'src',
      name: 'src',
      kind: 'directory',
      readable: true,
    },
    {
      path: 'README.md',
      name: 'README.md',
      kind: 'file',
      size: 1420,
      modifiedAt: '2026-10-14T10:00:00Z',
      readable: true,
    },
    {
      path: 'secret.bin',
      name: 'secret.bin',
      kind: 'file',
      size: 4096,
      readable: false,
    },
  ],
  hasMore: false,
  schemaVersion: filesBridgeSchemaVersion,
}

const mockReadResult: FilesReadResult = {
  projectId: 'prj-alpha',
  worktreeId: 'wt-main',
  area: 'repository',
  path: 'README.md',
  content: '# Project Alpha\nOperational Executive Console.',
  contentHash: 'hash-1234',
  modifiedAt: '2026-10-14T10:00:00Z',
  schemaVersion: filesBridgeSchemaVersion,
}

describe('FilesView', () => {
  it('renders static layout with sub-areas and worktree switcher', () => {
    const markup = renderToStaticMarkup(
      <FilesView
        workspace={mockWorkspace}
        selectedProjectId="prj-alpha"
        onRefresh={() => {}}
      />
    )
    expect(markup).toContain('CORTEXOS / PROJECT FILES')
    expect(markup).toContain('Repository')
    expect(markup).toContain('Support')
    expect(markup).toContain('Vault')
    expect(markup).toContain('Viewing')
    expect(markup).toContain('main · ready')
    expect(markup).toContain('Viewer')
  })

  it('calls ListFiles bridge with correct typed request', async () => {
    const listSpy = vi.fn().mockResolvedValue(mockListResult)
    const readSpy = vi.fn().mockResolvedValue(mockReadResult)
    const bridge: FilesBridge = {
      ListFiles: listSpy,
      ReadFile: readSpy,
    }

    const result = await loadFilesListing(bridge, 'prj-alpha', 'repository', 'wt-main', 'src')

    expect(listSpy).toHaveBeenCalledWith({
      schemaVersion: 'cortexos.files.bridge.v1',
      projectId: 'prj-alpha',
      worktreeId: 'wt-main',
      area: 'repository',
      path: 'src',
    })
    expect(result.entries).toHaveLength(3)
    expect(result.entries[1].name).toBe('README.md')
  })

  it('omits worktreeId for support and vault areas', async () => {
    const listSpy = vi.fn().mockResolvedValue({
      ...mockListResult,
      area: 'vault',
      worktreeId: undefined,
    })
    const bridge: FilesBridge = {
      ListFiles: listSpy,
      ReadFile: vi.fn(),
    }

    await loadFilesListing(bridge, 'prj-alpha', 'vault', 'wt-main')

    expect(listSpy).toHaveBeenCalledWith({
      schemaVersion: 'cortexos.files.bridge.v1',
      projectId: 'prj-alpha',
      area: 'vault',
    })
  })

  it('calls ReadFile bridge and returns content snapshot', async () => {
    const readSpy = vi.fn().mockResolvedValue(mockReadResult)
    const bridge: FilesBridge = {
      ListFiles: vi.fn(),
      ReadFile: readSpy,
    }

    const result = await readFileContent(bridge, 'prj-alpha', 'repository', 'README.md', 'wt-main')

    expect(readSpy).toHaveBeenCalledWith({
      schemaVersion: 'cortexos.files.bridge.v1',
      projectId: 'prj-alpha',
      worktreeId: 'wt-main',
      area: 'repository',
      path: 'README.md',
    })
    expect(result.content).toContain('Project Alpha')
  })
})
