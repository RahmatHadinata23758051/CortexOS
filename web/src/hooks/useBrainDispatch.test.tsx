import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { BrainBridge } from '../types'
import { brainBridgeSchemaVersion } from '../types'
import { useBrainDispatch } from './useBrainDispatch'

const bridge: BrainBridge = {
  DispatchBrainTask: vi.fn(),
  DispatchBrainTaskStream: vi.fn().mockResolvedValue({
    taskId: 'task-1',
    staffId: '',
    state: 'completed',
    output: 'done',
    steps: 1,
    tokens: 4,
    durationMs: 12,
    error: '',
    schemaVersion: brainBridgeSchemaVersion,
  }),
}

describe('useBrainDispatch hook', () => {
  it('initializes fail-closed in idle state', () => {
    let snapshot: ReturnType<typeof useBrainDispatch> | undefined

    function Probe() {
      snapshot = useBrainDispatch({ bridge })
      return null
    }

    renderToStaticMarkup(<Probe />)
    expect(snapshot?.state).toBe('idle')
    expect(snapshot?.taskId).toBeNull()
    expect(snapshot?.output).toBe('')
    expect(snapshot?.isRunning).toBe(false)
  })

  it('exposes run, reset, and a stable initial session contract', () => {
    let snapshot: ReturnType<typeof useBrainDispatch> | undefined

    function Probe() {
      snapshot = useBrainDispatch({ bridge })
      return null
    }

    renderToStaticMarkup(<Probe />)
    expect(typeof snapshot?.run).toBe('function')
    expect(typeof snapshot?.reset).toBe('function')
    expect(snapshot?.chunks).toEqual([])
    expect(snapshot?.error).toBeNull()
  })
})