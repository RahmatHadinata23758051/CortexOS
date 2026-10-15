export const filesBridgeSchemaVersion = 'cortexos.files.bridge.v1' as const

export type FilesArea = 'repository' | 'support' | 'vault'

export type FilesListRequest = {
  schemaVersion: typeof filesBridgeSchemaVersion
  projectId: string
  worktreeId?: string
  area: FilesArea
  path?: string
}

export type FilesReadRequest = {
  schemaVersion: typeof filesBridgeSchemaVersion
  projectId: string
  worktreeId?: string
  area: FilesArea
  path: string
}

export type FilesEntry = {
  path: string
  name: string
  kind: 'file' | 'directory'
  size?: number
  modifiedAt?: string
  readable: boolean
  source?: string
  contentHash?: string
}

export type FilesListResult = {
  projectId: string
  worktreeId?: string
  area: FilesArea
  rootLabel: string
  entries: FilesEntry[]
  hasMore: boolean
  schemaVersion: typeof filesBridgeSchemaVersion
}

export type FilesReadResult = {
  projectId: string
  worktreeId?: string
  area: FilesArea
  path: string
  content: string
  contentHash: string
  modifiedAt: string
  schemaVersion: typeof filesBridgeSchemaVersion
}

export type FilesBridge = {
  ListFiles(request: FilesListRequest): Promise<FilesListResult>
  ReadFile(request: FilesReadRequest): Promise<FilesReadResult>
}
