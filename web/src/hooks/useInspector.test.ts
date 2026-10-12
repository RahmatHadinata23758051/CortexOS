import { describe, expect, it } from 'vitest'

import { canRequestMerge, type InspectorState } from './useInspector'

/**
 * P7-05: merge authority is fail-closed. The hook may only surface an enabled
 * merge when the Inspector accepted, the command is idle, and a real backend
 * command exists. Any missing piece must keep the gate locked (ADR-0005).
 */
describe('useInspector merge gate', () => {
  const base: Pick<InspectorState, 'decision' | 'mergeState'> = { decision: 'pending', mergeState: 'idle' }

  it('keeps the gate locked before Inspector acceptance', () => {
    expect(canRequestMerge({ decision: 'pending', mergeState: 'idle' }, true)).toBe(false)
    expect(canRequestMerge({ decision: 'rejected', mergeState: 'idle' }, true)).toBe(false)
    expect(canRequestMerge({ decision: 'changes_requested', mergeState: 'idle' }, true)).toBe(false)
  })

  it('opens the gate only after acceptance with an available command', () => {
    expect(canRequestMerge({ decision: 'accepted', mergeState: 'idle' }, true)).toBe(true)
  })

  it('fails closed when no backend command is wired', () => {
    expect(canRequestMerge({ decision: 'accepted', mergeState: 'idle' }, false)).toBe(false)
  })

  it('refuses a second merge while one is in flight or already merged', () => {
    expect(canRequestMerge({ decision: 'accepted', mergeState: 'merging' }, true)).toBe(false)
    expect(canRequestMerge({ decision: 'accepted', mergeState: 'merged' }, true)).toBe(false)
  })

  it('never opens the gate from a mutated base state', () => {
    expect(canRequestMerge(base, true)).toBe(false)
  })
})