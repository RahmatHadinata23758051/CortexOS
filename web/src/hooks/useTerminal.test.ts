import { describe, expect, it } from 'vitest'
import { parseWorkerLine } from './useTerminal'

describe('terminal worker JSONL seam', () => {
  it('parses a valid cortexos.worker.v1 message', () => {
    const result = parseWorkerLine('{"version":"cortexos.worker.v1","type":"progress","correlationId":"exec-1","sequence":1,"payload":{"phase":"run","percent":10}}')
    expect(result.error).toBeUndefined()
    expect(result.message?.type).toBe('progress')
    expect(result.message?.payload.percent).toBe(10)
  })

  it('fails closed for malformed and unsupported messages', () => {
    expect(parseWorkerLine('not json').error).toContain('Unexpected')
    expect(parseWorkerLine('{"version":"cortexos.worker.v1","type":"unknown","correlationId":"x","sequence":1,"payload":{}}').error).toContain('unsupported')
    expect(parseWorkerLine('{"version":"cortexos.worker.v1","type":"progress","correlationId":"x","sequence":0,"payload":{}}').error).toContain('sequence')
  })
})
