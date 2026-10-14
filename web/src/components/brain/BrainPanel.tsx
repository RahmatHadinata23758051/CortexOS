import { useMemo, useState } from 'react'
import { useBrainDispatch } from '../../hooks/useBrainDispatch'
import { wailsRuntimeBridge } from '../../types'
import { ContextInspector, type ContextSnippet } from './ContextInspector'
import { ReasoningTimeline } from './ReasoningTimeline'
import { BrainStreamViewer } from './StreamViewer'

export type BrainPanelProps = {
  bridge?: Parameters<typeof useBrainDispatch>[0]['bridge']
  staffId?: string
  contextSnippets?: ContextSnippet[]
}

export function BrainPanel({ bridge = wailsRuntimeBridge, staffId, contextSnippets = [] }: BrainPanelProps) {
  const [prompt, setPrompt] = useState('')
  const [contextOpen, setContextOpen] = useState(false)
  const { state, output, chunks, steps, tokens, durationMs, error, isRunning, run, reset } = useBrainDispatch({ bridge })

  const canRun = useMemo(() => prompt.trim().length > 0 && !isRunning, [prompt, isRunning])

  const submit = async () => {
    if (!canRun) return
    try {
      await run({ userPrompt: prompt, staffId })
    } catch {
      // The hook exposes the failure state and error message to the panel.
    }
  }

  return (
    <section className="brain-panel" aria-labelledby="brain-panel-heading">
      <header className="brain-panel-header">
        <div className="tool-heading">
          <span className="eyebrow">CORTEXOS / BRAIN</span>
          <h1 id="brain-panel-heading">Reasoning cockpit</h1>
        </div>
        <div className={`brain-status ${isRunning ? 'active' : state}`} role="status">
          <span aria-hidden="true" /> {state}
        </div>
      </header>

      <div className="brain-layout">
        <div className="brain-main-column">
          <form
            className="brain-prompt-form"
            onSubmit={(event) => {
              event.preventDefault()
              void submit()
            }}
          >
            <label htmlFor="brain-prompt">Instruction</label>
            <textarea
              id="brain-prompt"
              value={prompt}
              onChange={(event) => setPrompt(event.target.value)}
              placeholder="Ask Brain to inspect, plan, or explain…"
              rows={4}
              disabled={isRunning}
            />
            <div className="prompt-actions">
              <span className="prompt-hint">Local bridge · provider policy remains fail-closed</span>
              <div className="prompt-buttons">
                {state !== 'idle' && (
                  <button type="button" className="secondary-button" onClick={reset} disabled={isRunning}>
                    Clear
                  </button>
                )}
                <button type="submit" className="primary-button" disabled={!canRun}>
                  {isRunning ? 'Reasoning…' : 'Run Brain'}
                </button>
              </div>
            </div>
          </form>

          <BrainStreamViewer output={output} chunks={chunks} isRunning={isRunning} />

          {error && (
            <div className="brain-error" role="alert">
              <strong>Dispatch failed.</strong> {error}
            </div>
          )}
        </div>

        <div className="brain-side-column">
          <ReasoningTimeline state={state} steps={steps} tokens={tokens} durationMs={durationMs} />
          <ContextInspector
            snippets={contextSnippets}
            isOpen={contextOpen}
            onToggle={() => setContextOpen((open) => !open)}
          />
        </div>
      </div>
    </section>
  )
}
