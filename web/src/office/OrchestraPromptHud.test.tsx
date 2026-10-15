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
  it('renders command prompt with live indicator and single-command focus', () => {
    const markup = renderToStaticMarkup(
      <OrchestraPromptHud projectId="proj-1" bridge={mockPlanBridge()} />,
    )
    expect(markup).toContain('What should we orchestrate?')
    expect(markup).toContain('LIVE')
    expect(markup).toContain('Command')
    expect(markup).toContain('Tune plan details')
    expect(markup).toContain('Send to Orchestra')
  })
})
