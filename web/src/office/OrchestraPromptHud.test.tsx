import { describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import {
  type OrchestraPlanBridge,
} from '../types/orchestra-plan'
import { OrchestraPromptHud } from './OrchestraPromptHud'

function mockPlanBridge(overrides: Partial<OrchestraPlanBridge> = {}): OrchestraPlanBridge {
  return {
    ProposeOrchestraPlan: vi.fn(),
    ConfirmOrchestraPlan: vi.fn(),
    RejectOrchestraPlan: vi.fn(),
    GetOrchestraPlan: vi.fn(),
    ...overrides,
  }
}

describe('OrchestraPromptHud', () => {
  it('renders a conversation surface with live indicator and no manual worker fields', () => {
    const markup = renderToStaticMarkup(
      <OrchestraPromptHud projectId="proj-1" bridge={mockPlanBridge()} />,
    )
    expect(markup).toContain('Talk to Orchestra')
    expect(markup).toContain('LIVE')
    expect(markup).toContain('Message')
    expect(markup).toContain('Send to Orchestra')
    expect(markup).not.toContain('Tune plan details')
    expect(markup).not.toContain('Worker role')
  })
})
