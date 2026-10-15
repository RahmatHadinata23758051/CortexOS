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
  it('renders prompt form with required fields and bridge badge', () => {
    const markup = renderToStaticMarkup(
      <OrchestraPromptHud projectId="proj-1" bridge={mockPlanBridge()} />,
    )
    expect(markup).toContain('Prompt a governed plan')
    expect(markup).toContain('LIVE BRIDGE')
    expect(markup).toContain('Intent')
    expect(markup).toContain('Plan title')
    expect(markup).toContain('Task title')
    expect(markup).toContain('Role')
    expect(markup).toContain('Action')
    expect(markup).toContain('Acceptance criteria')
    expect(markup).toContain('Plan contract v1 · 1 task')
    expect(markup).toContain('Propose plan')
  })
})
