import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'

import { AuditTrail } from './AuditTrail'
import { DecisionPanel } from './DecisionPanel'
import { EvidenceViewer } from './EvidenceViewer'
import { InspectorReview } from './InspectorReview'
import { MergeDialog } from './MergeDialog'
import {
  orchestraSchemaVersion,
  type OrchestraTaskDetail,
  type OrchestraTaskSummary,
} from '../../types/orchestra'

const sampleTask: OrchestraTaskSummary = {
  id: 'task-ban-121',
  projectId: 'proj-omega',
  status: 'awaitingInspection',
  attempt: 1,
  maxAttempts: 2,
  progress: 80,
  schemaVersion: orchestraSchemaVersion,
}

const sampleDetail: OrchestraTaskDetail = {
  ...sampleTask,
  acceptanceCount: 3,
  dependencyCount: 1,
  evidence: [
    { id: 'ev-validation-1', count: 2 },
    { id: 'ev-test-2', count: 4 },
  ],
  timeline: [
    {
      sequence: 1,
      type: 'task.dispatched',
      occurredAt: '2026-10-10T00:00:00Z',
    },
    {
      sequence: 2,
      type: 'evidence.collected',
      occurredAt: '2026-10-10T00:01:00Z',
      evidenceIds: ['ev-validation-1', 'ev-test-2'],
    },
  ],
}

describe('InspectorReview component', () => {
  it('renders Inspector header with ADR-0005 authority badge and task ID', () => {
    const markup = renderToStaticMarkup(
      <InspectorReview task={sampleTask} detail={sampleDetail} />,
    )
    expect(markup).toContain('CORTEXOS / INSPECTOR')
    expect(markup).toContain('Review the evidence.')
    expect(markup).toContain('ADR-0005')
    expect(markup).toContain('Inspector → Orchestra')
    expect(markup).toContain('task-ban-121')
    expect(markup).toContain('MERGE GATE: LOCKED')
  })

  it('renders EvidenceViewer with syntax highlighting and redaction counter', () => {
    const markup = renderToStaticMarkup(
      <InspectorReview task={sampleTask} detail={sampleDetail} />,
    )
    expect(markup).toContain('01 / EVIDENCE VIEWER')
    expect(markup).toContain('Bounded artifacts')
    expect(markup).toContain('2 references')
    expect(markup).toContain('redacted')
    expect(markup).toContain('syntax-key')
  })

  it('renders DecisionPanel with Accept, Reject, and Request Changes', () => {
    const markup = renderToStaticMarkup(
      <InspectorReview task={sampleTask} detail={sampleDetail} />,
    )
    expect(markup).toContain('02 / DECISION')
    expect(markup).toContain('Inspector verdict')
    expect(markup).toContain('Accept')
    expect(markup).toContain('Reject')
    expect(markup).toContain('Request changes')
  })

  it('enforces ADR-0005: merge button is disabled while pending', () => {
    const markup = renderToStaticMarkup(
      <InspectorReview task={sampleTask} detail={sampleDetail} />,
    )
    expect(markup).toContain('disabled=""')
    expect(markup).toContain('Ready only after acceptance')
  })

  it('renders close button when onClose is passed', () => {
    const onClose = vi.fn()
    const markup = renderToStaticMarkup(
      <InspectorReview task={sampleTask} detail={sampleDetail} onClose={onClose} />,
    )
    expect(markup).toContain('Back to task board')
  })
})

describe('EvidenceViewer component', () => {
  it('renders empty evidence message when array is empty', () => {
    const markup = renderToStaticMarkup(
      <EvidenceViewer evidence={[]} selectedIndex={0} onSelect={() => {}} />,
    )
    expect(markup).toContain('No evidence attached')
    expect(markup).toContain('Inspector cannot accept a task without governed evidence')
  })

  it('renders tabs and selected code frame with redaction details', () => {
    const items = [
      {
        id: 'ev-1',
        kind: 'validation',
        language: 'typescript',
        content: 'const result = true;',
        redactions: 5,
        passed: true,
      },
    ]
    const markup = renderToStaticMarkup(
      <EvidenceViewer
        evidence={items}
        selectedIndex={0}
        onSelect={() => {}}
        selected={items[0]}
      />,
    )
    expect(markup).toContain('ev-1')
    expect(markup).toContain('validation')
    expect(markup).toContain('⌁ 5 redacted')
    expect(markup).toContain('PASS / bounded and redacted')
    expect(markup).toContain('syntax-key')
  })
})

describe('DecisionPanel component', () => {
  it('disables merge button when mergeEnabled is false', () => {
    const markup = renderToStaticMarkup(
      <DecisionPanel
        decision="pending"
        mergeState="idle"
        mergeEnabled={false}
        note=""
        onNoteChange={() => {}}
        onDecision={() => {}}
        onMerge={() => {}}
      />,
    )
    expect(markup).toContain('disabled=""')
    expect(markup).toContain('Ready only after acceptance')
  })

  it('enables merge button when mergeEnabled is true', () => {
    const markup = renderToStaticMarkup(
      <DecisionPanel
        decision="accepted"
        mergeState="idle"
        mergeEnabled={true}
        note="Approved"
        onNoteChange={() => {}}
        onDecision={() => {}}
        onMerge={() => {}}
      />,
    )
    expect(markup).not.toContain('disabled=""')
    expect(markup).toContain('Merge task')
  })
})

describe('MergeDialog component', () => {
  it('returns null when isOpen is false', () => {
    const markup = renderToStaticMarkup(
      <MergeDialog
        isOpen={false}
        decision="pending"
        mergeState="idle"
        mergeEnabled={false}
        taskId="task-ban-121"
        onMerge={() => {}}
        onClose={() => {}}
      />,
    )
    expect(markup).toBe('')
  })

  it('renders locked state and ADR-0005 warning when not accepted', () => {
    const markup = renderToStaticMarkup(
      <MergeDialog
        isOpen={true}
        decision="pending"
        mergeState="idle"
        mergeEnabled={false}
        taskId="task-ban-121"
        onMerge={() => {}}
        onClose={() => {}}
      />,
    )
    expect(markup).toContain('ORCHESTRA MERGE AUTHORITY')
    expect(markup).toContain('ADR-0005 Enforced')
    expect(markup).toContain('Merge button is locked')
    expect(markup).toContain('disabled=""')
  })

  it('renders merge animation during merging state', () => {
    const markup = renderToStaticMarkup(
      <MergeDialog
        isOpen={true}
        decision="accepted"
        mergeState="merging"
        mergeEnabled={false}
        taskId="task-ban-121"
        onMerge={() => {}}
        onClose={() => {}}
      />,
    )
    expect(markup).toContain('merge-spinner')
    expect(markup).toContain('merge-pulse-ring')
    expect(markup).toContain('Orchestra is integrating changes…')
  })

  it('renders success banner when merged', () => {
    const markup = renderToStaticMarkup(
      <MergeDialog
        isOpen={true}
        decision="accepted"
        mergeState="merged"
        mergeEnabled={false}
        taskId="task-ban-121"
        onMerge={() => {}}
        onClose={() => {}}
      />,
    )
    expect(markup).toContain('Integration Complete')
    expect(markup).toContain('Worktree cleanly integrated')
  })
})

describe('AuditTrail component', () => {
  it('renders empty message when no audit entries', () => {
    const markup = renderToStaticMarkup(<AuditTrail entries={[]} />)
    expect(markup).toContain('No decisions recorded for this review yet.')
  })

  it('renders chronological audit entries', () => {
    const entries = [
      {
        id: '1',
        decision: 'accepted' as const,
        note: 'Criteria verified against tests',
        occurredAt: '2026-10-10T12:00:00Z',
      },
    ]
    const markup = renderToStaticMarkup(<AuditTrail entries={entries} />)
    expect(markup).toContain('Accepted')
    expect(markup).toContain('Criteria verified against tests')
    expect(markup).toContain('marker-accepted')
  })
})

describe('CSS styles for BAN-121', () => {
  it('declares ADR-0005 styles and merge animation keyframes', () => {
    const css = readFileSync(resolve(import.meta.dirname, '../../styles.css'), 'utf8')
    expect(css).toContain('inspector-review')
    expect(css).toContain('merge-spinner')
    expect(css).toContain('@keyframes spin')
    expect(css).toContain('merge-pulse-ring')
    expect(css).toContain('@keyframes pulse-ring')
    expect(css).toContain('dialog-enter')
  })
})
