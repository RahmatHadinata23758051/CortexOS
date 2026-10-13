import { describe, expect, it, vi, beforeEach } from 'vitest'

import { makeBrainRequest, dispatchBrainTask, dispatchBrainTaskStream, onBrainStreamForTask, onBrainStateForTask, isBrainTerminalState } from './brain'
import {
  brainBridgeSchemaVersion,
  BrainEventNames,
  type BrainBridge,
  type BrainState,
} from '../types'

const makeMockBridge = (overrides: Partial<BrainBridge> = {}): BrainBridge => ({
  DispatchBrainTask: vi.fn().mockResolvedValue({
    taskId: 'mock-task',
    staffId: '',
    state: 'completed',
    output: 'mock output',
    steps: 1,
    tokens: 10,
    durationMs: 100,
    error: '',
    schemaVersion: brainBridgeSchemaVersion,
  }),
  DispatchBrainTaskStream: vi.fn().mockResolvedValue({
    taskId: 'mock-stream-task',
    staffId: '',
    state: 'completed',
    output: 'mock stream output',
    steps: 1,
    tokens: 10,
    durationMs: 100,
    error: '',
    schemaVersion: brainBridgeSchemaVersion,
  }),
  ...overrides,
})

const mockRuntime = {
  EventsOn: vi.fn().mockImplementation((_event: string, handler: (p: unknown) => void) => {
    // Store handler for manual triggering in tests
    handler({ taskId: 'test-task', type: 'token', data: 'hello', sequence: 1, timestamp: new Date().toISOString() })
    return vi.fn()
  }),
  EventsOff: vi.fn(),
}

declare global {
  interface Window {
    runtime?: typeof mockRuntime
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  if (typeof window !== 'undefined') {
    window.runtime = mockRuntime
  }
})

describe('brain bridge request builder', () => {
  it('generates a taskId when omitted', () => {
    const req = makeBrainRequest({ userPrompt: 'test' })
    expect(req.taskId).toBeDefined()
    expect(req.taskId.length).toBeGreaterThan(0)
  })

  it('preserves provided taskId', () => {
    const req = makeBrainRequest({ userPrompt: 'test', taskId: 'my-task' })
    expect(req.taskId).toBe('my-task')
  })

  it('includes schema version', () => {
    const req = makeBrainRequest({ userPrompt: 'test' })
    expect(req.schemaVersion).toBe(brainBridgeSchemaVersion)
  })

  it('includes optional config when provided', () => {
    const req = makeBrainRequest({ userPrompt: 'test', stepLimit: 5, maxTokens: 1000, timeoutSec: 30 })
    expect(req.config).toEqual({ stepLimit: 5, maxTokens: 1000, timeoutSec: 30 })
  })
})

describe('brain bridge dispatch', () => {
  it('throws on empty userPrompt', async () => {
    const bridge = makeMockBridge()
    await expect(dispatchBrainTask(bridge, { userPrompt: '' })).rejects.toThrow('userPrompt is required')
  })

  it('calls bridge.DispatchBrainTask with correct request', async () => {
    const bridge = makeMockBridge()
    await dispatchBrainTask(bridge, { userPrompt: 'hello', taskId: 't1', staffId: 'staff-1' })
    expect(bridge.DispatchBrainTask).toHaveBeenCalledWith(
      expect.objectContaining({
        userPrompt: 'hello',
        taskId: 't1',
        staffId: 'staff-1',
        schemaVersion: brainBridgeSchemaVersion,
      }),
    )
  })

  it('returns result from bridge', async () => {
    const bridge = makeMockBridge({
      DispatchBrainTask: vi.fn().mockResolvedValue({
        taskId: 't1',
        staffId: 's1',
        state: 'completed',
        output: 'result',
        steps: 2,
        tokens: 20,
        durationMs: 200,
        error: '',
        schemaVersion: brainBridgeSchemaVersion,
      }),
    })
    const result = await dispatchBrainTask(bridge, { userPrompt: 'x' })
    expect(result.output).toBe('result')
  })
})

describe('brain bridge stream dispatch', () => {
  it('calls bridge.DispatchBrainTaskStream with correct request', async () => {
    const bridge = makeMockBridge()
    await dispatchBrainTaskStream(bridge, { userPrompt: 'stream', taskId: 'stream-1' })
    expect(bridge.DispatchBrainTaskStream).toHaveBeenCalledWith(
      expect.objectContaining({
        userPrompt: 'stream',
        taskId: 'stream-1',
        schemaVersion: brainBridgeSchemaVersion,
      }),
    )
  })
})

describe('brain event subscriptions', () => {
  let capturedStreamHandler: ((payload: unknown) => void) | null = null
  let capturedStateHandler: ((payload: unknown) => void) | null = null

  beforeEach(() => {
    capturedStreamHandler = null
    capturedStateHandler = null
    const runtimeMock = {
      EventsOn: vi.fn().mockImplementation((event: string, handler: (p: unknown) => void) => {
        if (event === BrainEventNames.StreamChunk) capturedStreamHandler = handler
        if (event === BrainEventNames.StateChanged) capturedStateHandler = handler
        return vi.fn()
      }),
      EventsOff: vi.fn(),
    }
    // Re-setup base runtime, then let the subscription overwrite it below
    if (typeof globalThis !== 'undefined') {
      ;(globalThis as { runtime?: typeof runtimeMock }).runtime = runtimeMock
    }
    if (typeof window !== 'undefined') {
      ;(window as unknown as { runtime?: typeof runtimeMock }).runtime = runtimeMock
    }
  })

  it('filters chunks by taskId in onBrainStreamForTask', () => {
    const handled: unknown[] = []
    const unsub = onBrainStreamForTask('target-task', (chunk) => handled.push(chunk))

    // Drive the captured handler registered during the call
    capturedStreamHandler?.({ taskId: 'target-task', type: 'token', data: 'a', sequence: 1, timestamp: '' })
    capturedStreamHandler?.({ taskId: 'other-task', type: 'token', data: 'b', sequence: 1, timestamp: '' })

    expect(handled.length).toBe(1)
    expect((handled[0] as { taskId: string }).taskId).toBe('target-task')
    unsub()
  })

  it('filters state by taskId in onBrainStateForTask', () => {
    const handled: unknown[] = []
    const unsub = onBrainStateForTask('target-task', (result) => handled.push(result))

    capturedStateHandler?.({ taskId: 'target-task', state: 'completed', output: 'done', steps: 1, tokens: 5, durationMs: 50, error: '', schemaVersion: brainBridgeSchemaVersion, staffId: '' })
    capturedStateHandler?.({ taskId: 'other-task', state: 'completed', output: 'other', steps: 1, tokens: 5, durationMs: 50, error: '', schemaVersion: brainBridgeSchemaVersion, staffId: '' })

    expect(handled.length).toBe(1)
    expect((handled[0] as { taskId: string }).taskId).toBe('target-task')
    unsub()
  })
})

describe('isBrainTerminalState', () => {
  const terminal: BrainState[] = ['completed', 'failed', 'canceled']
  const nonTerminal: BrainState[] = ['idle', 'planning', 'executing', 'streaming', 'tool_calling']

  terminal.forEach((s) => {
    it(`returns true for ${s}`, () => expect(isBrainTerminalState(s)).toBe(true))
  })

  nonTerminal.forEach((s) => {
    it(`returns false for ${s}`, () => expect(isBrainTerminalState(s)).toBe(false))
  })
})