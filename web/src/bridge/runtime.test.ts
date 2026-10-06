import { describe, expect, it, vi } from 'vitest'

import { loadRuntimeSnapshot } from './runtime'
import { runtimeSchemaVersion, type RuntimeBridge } from '../types'

const readyBridge = (): RuntimeBridge => ({
  GetRuntimeSnapshot: vi.fn().mockResolvedValue({
    schemaVersion: runtimeSchemaVersion,
    status: 'ready',
    environment: 'local',
    provider: 'disabled',
  }),
})

describe('loadRuntimeSnapshot', () => {
  it('returns a success state for a valid bridge response', async () => {
    await expect(loadRuntimeSnapshot(readyBridge())).resolves.toMatchObject({
      status: 'success',
      snapshot: { status: 'ready' },
    })
  })

  it('returns a safe error state when the bridge rejects', async () => {
    const bridge: RuntimeBridge = {
      GetRuntimeSnapshot: vi.fn().mockRejectedValue(new Error('bridge offline')),
    }

    await expect(loadRuntimeSnapshot(bridge)).resolves.toEqual({
      status: 'error',
      message: 'bridge offline',
    })
  })
})
