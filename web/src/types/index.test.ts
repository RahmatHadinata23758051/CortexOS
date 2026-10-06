import { describe, expect, it, vi } from 'vitest'

import {
  getRuntimeSnapshot,
  runtimeSchemaVersion,
  type RuntimeBridge,
} from './index'

describe('runtime bridge client', () => {
  it('requests the supported schema and returns the snapshot', async () => {
    const bridge: RuntimeBridge = {
      GetRuntimeSnapshot: vi.fn().mockResolvedValue({
        schemaVersion: runtimeSchemaVersion,
        status: 'ready',
        environment: 'local',
        provider: 'disabled',
      }),
    }

    await expect(getRuntimeSnapshot(bridge)).resolves.toMatchObject({
      status: 'ready',
    })
    expect(bridge.GetRuntimeSnapshot).toHaveBeenCalledWith({
      schemaVersion: runtimeSchemaVersion,
    })
  })
})
