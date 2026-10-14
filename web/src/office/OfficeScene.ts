import Phaser from 'phaser'
import type { OfficeActivityEvent, StaffBrainState, StaffSummary } from '../types/office'
import type { StaffAssignmentStatus } from '../types/coordination'

export type OfficeSceneState = {
  staff: readonly StaffSummary[]
  activity: readonly OfficeActivityEvent[]
  brainState?: StaffBrainState
  coordination?: readonly StaffAssignmentStatus[]
  onStaffSelect?: (staffId: string) => void
}

const zoneDefinitions = [
  { id: 'coordination', label: 'Coordination', x: 120, y: 120, width: 240, height: 150, token: '--surface-card' },
  { id: 'planning', label: 'Planning', x: 390, y: 120, width: 240, height: 150, token: '--surface-inset' },
  { id: 'delivery', label: 'Delivery', x: 120, y: 310, width: 240, height: 150, token: '--surface-active' },
  { id: 'review', label: 'Review', x: 390, y: 310, width: 240, height: 150, token: '--accent-cool-soft' },
]

const fallbackPalette = {
  base: 0x151d28,
  text: '#e9eef0',
  muted: '#afbdc6',
  border: 0x63707b,
  avatar: 0x92a5b4,
  accent: 0xd4a359,
}

function cssColor(name: string, fallback: string): string {
  if (typeof document === 'undefined') return fallback
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function hexNumber(value: string, fallback: number): number {
  const normalized = value.replace('#', '').slice(0, 6)
  const parsed = Number.parseInt(normalized, 16)
  return Number.isNaN(parsed) ? fallback : parsed
}

const roleZone: Record<StaffSummary['role'], string> = {
  coordinator: 'coordination', planner: 'planning', implementer: 'delivery', reviewer: 'review', specialist: 'review',
}

const availabilityToken: Record<StaffSummary['availability'], string> = {
  available: '--status-success', busy: '--status-danger', unavailable: '--status-warning', offline: '--status-idle',
}

export class OfficeScene extends Phaser.Scene {
  private officeState: OfficeSceneState = { staff: [], activity: [] }
  private avatars = new Map<string, Phaser.GameObjects.Container>()

  constructor() { super({ key: 'office' }) }

  setOfficeState(state: OfficeSceneState) {
    this.officeState = state
    if (this.scene.isActive()) this.renderState()
  }

  private speechBubbles = new Map<string, { bubble: Phaser.GameObjects.Container; text: Phaser.GameObjects.Text }>()

  create() {
    const bg = cssColor('--surface-base', '#151d28')
    this.cameras.main.setBackgroundColor(hexNumber(bg, 0x151d28))
    this.renderState()
  }

  private renderState() {
    this.children.removeAll(true)
    this.avatars.clear()
    this.speechBubbles.clear()

    const borderHex = hexNumber(cssColor('--border-default', '#63707b'), fallbackPalette.border)
    const mutedHex = cssColor('--text-muted', fallbackPalette.muted)

    zoneDefinitions.forEach((zone) => {
      const zoneColor = hexNumber(cssColor(zone.token, '#273445'), fallbackPalette.base)
      const panel = this.add.rectangle(zone.x, zone.y, zone.width, zone.height, zoneColor).setOrigin(0, 0).setStrokeStyle(1, borderHex, 0.6)
      panel.setInteractive({ useHandCursor: false })
      this.add.text(zone.x + 16, zone.y + 14, zone.label.toUpperCase(), { color: mutedHex, fontFamily: 'var(--font-mono), monospace', fontSize: '11px', letterSpacing: 2 })
    })

    const used: Record<string, number> = {}
    this.officeState.staff.forEach((member) => {
      const zone = zoneDefinitions.find((candidate) => candidate.id === roleZone[member.role]) ?? zoneDefinitions[0]
      const index = used[zone.id] ?? 0
      used[zone.id] = index + 1
      const x = zone.x + 38 + (index % 4) * 50
      const y = zone.y + 68 + Math.floor(index / 4) * 55
      const avatar = this.add.container(x, y)
      const assignment = this.officeState.coordination?.find((item) => item.staffId === member.id)

      // Base body circle. Orchestra coordination state takes precedence over
      // availability for the avatar ring, while availability remains the fallback.
      const defaultAvail = this.tokenColor(availabilityToken[member.availability] ?? '--status-idle', 0x64748b)
      const body = this.add.circle(0, 0, 15, hexNumber(cssColor('--surface-active', '#92a5b4'), fallbackPalette.avatar)).setStrokeStyle(3, this.coordinationColor(assignment) ?? defaultAvail)
      // Name label
      const name = this.add.text(0, 21, member.name.slice(0, 12), { color: cssColor('--text-primary', fallbackPalette.text), fontFamily: 'var(--font-mono), monospace', fontSize: '10px' }).setOrigin(0.5, 0)

      if (assignment && assignment.state !== 'idle') {
        const markerColor = this.coordinationColor(assignment) ?? this.tokenColor('--status-idle', 0x64748b)
        const marker = this.add.circle(12, -12, 4, markerColor).setStrokeStyle(1, this.tokenColor('--surface-base', fallbackPalette.base), 1)
        avatar.add(marker)
      }

      // Brain state visual: add a thinking indicator / pulse ring
      if (this.officeState.brainState && this.officeState.brainState.staffId === member.id) {
        const brainState = this.officeState.brainState
        const pulseColor = this.brainStateColor(brainState.state)
        // Outer pulse ring
        const pulse = this.add.circle(0, 0, 20, pulseColor, 0.35).setStrokeStyle(2, pulseColor, 0.8)
        // Add pulse animation
        this.tweens.add({
          targets: pulse,
          scale: 1.4,
          alpha: 0,
          duration: 1000,
          repeat: -1,
          ease: 'Sine.easeOut',
        })
        avatar.add(pulse)

        // Speech bubble with output summary
        if (brainState.speechText) {
          this.createSpeechBubble(avatar, member.id, brainState.speechText, brainState.state)
        }
      }

      avatar.add([body, name]).setSize(42, 44).setInteractive({ useHandCursor: true })
      avatar.on('pointerup', () => this.officeState.onStaffSelect?.(member.id))
      this.avatars.set(member.id, avatar)
    })
  }

  private tokenColor(token: string, fallback: number): number {
    return hexNumber(cssColor(token, ''), fallback)
  }

  private brainStateColor(state: StaffBrainState['state']): number {
    switch (state) {
      case 'planning':
      case 'executing':
        return this.tokenColor('--accent-cool', 0x4f7cb8)
      case 'streaming':
        return this.tokenColor('--accent-warm', fallbackPalette.accent)
      case 'completed':
        return this.tokenColor('--status-success', 0x3e9b5f)
      case 'failed':
      case 'canceled':
        return this.tokenColor('--status-danger', 0xdc2626)
      default:
        return this.tokenColor('--status-idle', 0x64748b)
    }
  }

  private coordinationColor(assignment?: StaffAssignmentStatus): number | undefined {
    switch (assignment?.state) {
      case 'running':
        return this.tokenColor('--accent-cool', 0x4f7cb8)
      case 'blocked':
        return this.tokenColor('--status-warning', 0xd97706)
      case 'awaiting_inspection':
        return this.tokenColor('--status-info', 0x4f7cb8)
      case 'completed':
        return this.tokenColor('--status-success', 0x3e9b5f)
      case 'failed':
        return this.tokenColor('--status-danger', 0xdc2626)
      default:
        return undefined
    }
  }

  private createSpeechBubble(container: Phaser.GameObjects.Container, staffId: string, text: string, state: StaffBrainState['state']) {
    const maxWidth = 140
    const padding = 8
    // Truncate for performance
    const displayText = text.length > 120 ? text.slice(0, 117) + '…' : text

    const bubble = this.add.container(0, -38)
    const bg = this.add.rectangle(0, 0, maxWidth + padding * 2, 28, this.tokenColor('--surface-base', fallbackPalette.base), 0.95).setStrokeStyle(1, this.brainStateColor(state), 0.8).setOrigin(0.5)
    const txt = this.add.text(0, 0, displayText, {
      color: cssColor('--text-primary', fallbackPalette.text),
      fontFamily: 'var(--font-mono), monospace',
      fontSize: '9px',
      wordWrap: { width: maxWidth, useAdvancedWrap: true },
      align: 'center',
    }).setOrigin(0.5)
    bubble.add([bg, txt])
    container.add(bubble)

    this.speechBubbles.set(staffId, { bubble, text: txt })
  }
}

export function createOfficeGame(parent: HTMLElement, state: OfficeSceneState) {
  const bg = cssColor('--surface-base', '#151d28')
  const game = new Phaser.Game({
    type: Phaser.AUTO,
    parent,
    width: 720,
    height: 510,
    backgroundColor: bg,
    scene: OfficeScene,
    banner: false,
    render: { antialias: true, pixelArt: false },
    audio: { noAudio: true },
  })
  game.events.once('ready', () => (game.scene.getScene('office') as OfficeScene).setOfficeState(state))
  return game
}
