import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { AlertTriangle, ChevronLeft, FileCode2, FileText, Folder, FolderTree, GitBranch, RefreshCw, ShieldCheck } from 'lucide-react'
import type { WorkspaceState } from '../bridge/workspace'
import { wailsRuntimeBridge, filesBridgeSchemaVersion } from '../types'
import type {
  FilesArea as FilesContractArea,
  FilesBridge,
  FilesEntry,
  FilesListResult,
  FilesReadResult,
  WorkspaceWorktree,
} from '../types'
import { loadFilesListing, readFileContent } from '../bridge/files'
import { onOrchestraPlanLifecycle } from '../bridge/orchestra-events'

export type FilesArea = FilesContractArea

export interface FilesViewProps {
  workspace: WorkspaceState
  selectedProjectId: string
  onRefresh: () => void
  bridge?: FilesBridge
}

type ListingState =
  | { status: 'loading' }
  | { status: 'success'; result: FilesListResult }
  | { status: 'error'; message: string }

type ReadState =
  | { status: 'idle' }
  | { status: 'loading'; path: string }
  | { status: 'success'; result: FilesReadResult }
  | { status: 'error'; path: string; message: string }

const areaCopy: Record<FilesArea, { label: string; description: string; icon: ReactNode }> = {
  repository: { label: 'Repository', description: 'Source code in the selected worktree.', icon: <FileCode2 size={16} aria-hidden="true" /> },
  support: { label: 'Support', description: 'User-provided reference files, read-only to workers.', icon: <ShieldCheck size={16} aria-hidden="true" /> },
  vault: { label: 'Vault', description: 'Project knowledge in purpose-named Markdown files.', icon: <FileText size={16} aria-hidden="true" /> },
}

export function FilesView({ workspace, selectedProjectId, onRefresh, bridge = wailsRuntimeBridge }: FilesViewProps): ReactNode {
  const [area, setArea] = useState<FilesArea>('repository')
  const [selectedWorktreeId, setSelectedWorktreeId] = useState('')
  const [currentPath, setCurrentPath] = useState('')
  const [selectedPath, setSelectedPath] = useState('')
  const [refreshVersion, setRefreshVersion] = useState(0)
  const [listing, setListing] = useState<ListingState>({ status: 'loading' })
  const [readState, setReadState] = useState<ReadState>({ status: 'idle' })

  const projectWorktrees = useMemo(() => {
    if (workspace.status !== 'success') return []
    return workspace.snapshot.worktrees.filter((worktree) => worktree.projectId === selectedProjectId)
  }, [selectedProjectId, workspace])
  const selectedWorktree = projectWorktrees.find((worktree) => worktree.id === selectedWorktreeId)
    ?? projectWorktrees.find((worktree) => worktree.activeReference)
    ?? projectWorktrees[0]

  useEffect(() => {
    let active = true
    const load = async () => {
      setListing({ status: 'loading' })
      setReadState({ status: 'idle' })
      if (!selectedProjectId) {
        setListing({ status: 'success', result: emptyListing(selectedProjectId, area) })
        return
      }
      try {
        const result = await loadFilesListing(bridge, selectedProjectId, area, selectedWorktree?.id, currentPath)
        if (active) setListing({ status: 'success', result })
      } catch (error) {
        if (active) setListing({ status: 'error', message: errorMessage(error, 'File listing failed.') })
      }
    }
    void load()
    return () => {
      active = false
    }
  }, [area, bridge, currentPath, refreshVersion, selectedProjectId, selectedWorktree?.id])

  const handleRefresh = () => {
    setRefreshVersion((value) => value + 1)
    setSelectedPath('')
    setReadState({ status: 'idle' })
    onRefresh()
  }

  // Worker lifecycle events imply writes landed in the worktree; re-list instead
  // of polling. ponytail: reuses worker events rather than a dedicated
  // files-changed event. Add EventFilesChanged if non-worker writers appear.
  useEffect(() => {
    return onOrchestraPlanLifecycle(() => {
      setRefreshVersion((value) => value + 1)
    })
  }, [])

  const handleAreaChange = (nextArea: FilesArea) => {
    setArea(nextArea)
    setCurrentPath('')
    setSelectedPath('')
    setReadState({ status: 'idle' })
  }

  const handleWorktreeChange = (nextWorktreeId: string) => {
    setSelectedWorktreeId(nextWorktreeId)
    setCurrentPath('')
    setSelectedPath('')
    setReadState({ status: 'idle' })
  }

  const handleDirectoryOpen = (entry: FilesEntry) => {
    setCurrentPath(entry.path)
    setSelectedPath('')
    setReadState({ status: 'idle' })
  }

  const handleFileSelect = async (entry: FilesEntry) => {
    if (!entry.readable) return
    setSelectedPath(entry.path)
    setReadState({ status: 'loading', path: entry.path })
    try {
      const result = await readFileContent(bridge, selectedProjectId, area, entry.path, selectedWorktree?.id)
      setReadState({ status: 'success', result })
    } catch (error) {
      setReadState({ status: 'error', path: entry.path, message: errorMessage(error, 'File read failed.') })
    }
  }

  const entries = listing.status === 'success' ? listing.result.entries : []
  const selectedReadResult = readState.status === 'success' ? readState.result : undefined
  const currentArea = areaCopy[area]

  return (
    <section className="files-view reveal-1" aria-labelledby="files-title">
      <header className="files-header">
        <div className="tool-heading">
          <span className="eyebrow">CORTEXOS / PROJECT FILES</span>
          <h1 id="files-title">Files</h1>
          <p className="files-description">Browse project context without becoming a second execution authority.</p>
        </div>
        <button type="button" className="btn-secondary" onClick={handleRefresh}>
          <RefreshCw size={14} aria-hidden="true" />
          Refresh snapshot
        </button>
      </header>

      <div className="files-context-bar">
        <label className="files-worktree-select">
          <span>Viewing</span>
          <select
            value={selectedWorktree?.id ?? ''}
            onChange={(event) => handleWorktreeChange(event.target.value)}
            disabled={projectWorktrees.length === 0}
            aria-label="Select worktree to view"
          >
            {projectWorktrees.length === 0 ? <option value="">Main repository · unavailable</option> : projectWorktrees.map((worktree) => (
              <option key={worktree.id} value={worktree.id}>
                {worktree.branch || worktree.id} · {worktree.status}
              </option>
            ))}
          </select>
        </label>
        {selectedWorktree && <WorktreeMetadata worktree={selectedWorktree} />}
      </div>

      <nav className="files-area-tabs" aria-label="File areas">
        {(Object.keys(areaCopy) as FilesArea[]).map((candidate) => (
          <button
            key={candidate}
            type="button"
            className={`files-area-tab${area === candidate ? ' active' : ''}`}
            onClick={() => handleAreaChange(candidate)}
            aria-current={area === candidate ? 'page' : undefined}
          >
            {areaCopy[candidate].icon}
            <span>{areaCopy[candidate].label}</span>
          </button>
        ))}
      </nav>

      <div className="files-surface" aria-live="polite">
        <div className="files-surface-heading">
          <div>
            <h2>{currentArea.label}</h2>
            <p>{currentArea.description}</p>
          </div>
          <span className="files-contract-badge">{listing.status === 'error' ? 'Bridge error' : 'Bridge connected'}</span>
        </div>
        <div className="files-browser-layout">
          <FileListPane
            area={area}
            currentPath={currentPath}
            entries={entries}
            hasMore={listing.status === 'success' && listing.result.hasMore}
            listing={listing}
            selectedPath={selectedPath}
            onBack={() => setCurrentPath(parentPath(currentPath))}
            onDirectoryOpen={handleDirectoryOpen}
            onFileSelect={handleFileSelect}
          />
          <FileViewerPane readState={readState} selectedPath={selectedPath} selectedReadResult={selectedReadResult} />
        </div>
      </div>
    </section>
  )
}

function FileListPane({ area, currentPath, entries, hasMore, listing, selectedPath, onBack, onDirectoryOpen, onFileSelect }: {
  area: FilesArea
  currentPath: string
  entries: FilesEntry[]
  hasMore: boolean
  listing: ListingState
  selectedPath: string
  onBack: () => void
  onDirectoryOpen: (entry: FilesEntry) => void
  onFileSelect: (entry: FilesEntry) => void
}): ReactNode {
  return (
    <section className="files-list-pane" aria-labelledby="files-list-title">
      <header className="files-pane-heading">
        <div>
          <h3 id="files-list-title">{currentPath || areaCopy[area].label}</h3>
          <span>{listing.status === 'success' ? `${entries.length} entries${hasMore ? ' · more available' : ''}` : 'Synchronizing listing'}</span>
        </div>
        {currentPath && <button type="button" className="files-back-button" onClick={onBack} aria-label="Go to parent folder"><ChevronLeft size={14} aria-hidden="true" />Back</button>}
      </header>
      {listing.status === 'loading' && <div className="files-list-state" role="status">Loading files…</div>}
      {listing.status === 'error' && <div className="files-list-state files-list-error" role="alert"><AlertTriangle size={16} aria-hidden="true" /><span>{listing.message}</span></div>}
      {listing.status === 'success' && entries.length === 0 && <div className="files-list-state" role="status"><FolderTree size={20} aria-hidden="true" /><span>{area === 'support' ? 'No support files are available.' : 'This folder is empty.'}</span></div>}
      {listing.status === 'success' && entries.length > 0 && <div className="files-entry-list" role="list" aria-label={`${areaCopy[area].label} entries`}>
        {entries.map((entry) => entry.kind === 'directory' ? (
          <button key={entry.path} type="button" className="files-entry files-entry-directory" onClick={() => onDirectoryOpen(entry)} role="listitem">
            <Folder size={15} aria-hidden="true" /><span className="files-entry-name">{entry.name}</span><span className="files-entry-meta">Directory</span>
          </button>
        ) : (
          <button key={entry.path} type="button" className={`files-entry files-entry-file${selectedPath === entry.path ? ' selected' : ''}`} onClick={() => onFileSelect(entry)} disabled={!entry.readable} title={entry.readable ? `Read ${entry.path}` : 'This file is not readable'} role="listitem">
            <FileText size={15} aria-hidden="true" /><span className="files-entry-name">{entry.name}</span><span className="files-entry-meta">{entry.readable ? `${formatFileSize(entry.size)} · ${formatModifiedAt(entry.modifiedAt)}` : 'Unreadable'}</span>
          </button>
        ))}
      </div>}
    </section>
  )
}

function FileViewerPane({ readState, selectedPath, selectedReadResult }: { readState: ReadState; selectedPath: string; selectedReadResult?: FilesReadResult }): ReactNode {
  return (
    <section className="files-viewer-pane" aria-labelledby="files-viewer-title">
      <header className="files-pane-heading"><div><h3 id="files-viewer-title">Viewer</h3><span>{selectedPath || 'Select a readable file'}</span></div></header>
      {readState.status === 'idle' && <div className="files-viewer-state"><FileText size={22} aria-hidden="true" /><span>Select a file to read its current snapshot.</span></div>}
      {readState.status === 'loading' && <div className="files-viewer-state" role="status">Reading {readState.path}…</div>}
      {readState.status === 'error' && <div className="files-viewer-state files-list-error" role="alert"><AlertTriangle size={16} aria-hidden="true" /><span>{readState.message}</span></div>}
      {readState.status === 'success' && selectedReadResult && <pre className="files-content-viewer"><code>{selectedReadResult.content}</code></pre>}
    </section>
  )
}

function WorktreeMetadata({ worktree }: { worktree: WorkspaceWorktree }): ReactNode {
  return (
    <div className="files-worktree-meta" aria-label="Selected worktree metadata">
      <span className="files-branch"><GitBranch size={13} aria-hidden="true" />{worktree.branch || 'detached'}</span>
      <span className={`files-dirty-state${worktree.dirty ? ' dirty' : ''}`}>{worktree.dirty ? 'Changes present' : 'Clean'}</span>
      <span className="files-worktree-status">{worktree.status}</span>
    </div>
  )
}

function emptyListing(projectId: string, area: FilesArea): FilesListResult {
  return { projectId, area, rootLabel: areaCopy[area].label, entries: [], hasMore: false, schemaVersion: filesBridgeSchemaVersion }
}

function parentPath(path: string): string {
  const segments = path.split('/').filter(Boolean)
  segments.pop()
  return segments.join('/')
}

function formatFileSize(size?: number): string {
  if (size === undefined) return 'Size unavailable'
  if (size < 1024) return `${size} B`
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`
  return `${(size / (1024 * 1024)).toFixed(1)} MB`
}

function formatModifiedAt(modifiedAt?: string): string {
  if (!modifiedAt) return 'Date unavailable'
  const date = new Date(modifiedAt)
  return Number.isNaN(date.getTime()) ? 'Date unavailable' : date.toLocaleDateString()
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}
