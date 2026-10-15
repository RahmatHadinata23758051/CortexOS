import { filesBridgeSchemaVersion } from '../types'
import type {
  FilesArea,
  FilesBridge,
  FilesListResult,
  FilesReadResult,
} from '../types'

export async function loadFilesListing(
  bridge: FilesBridge,
  projectId: string,
  area: FilesArea,
  worktreeId?: string,
  path?: string,
): Promise<FilesListResult> {
  return bridge.ListFiles({
    schemaVersion: filesBridgeSchemaVersion,
    projectId,
    ...(area === 'repository' && worktreeId ? { worktreeId } : {}),
    area,
    ...(path ? { path } : {}),
  })
}

export async function readFileContent(
  bridge: FilesBridge,
  projectId: string,
  area: FilesArea,
  path: string,
  worktreeId?: string,
): Promise<FilesReadResult> {
  return bridge.ReadFile({
    schemaVersion: filesBridgeSchemaVersion,
    projectId,
    ...(area === 'repository' && worktreeId ? { worktreeId } : {}),
    area,
    path,
  })
}
