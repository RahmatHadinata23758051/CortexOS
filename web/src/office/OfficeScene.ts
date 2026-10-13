import Phaser from 'phaser'
import type { OfficeActivityEvent, StaffBrainState, StaffSummary } from '../types/office'

export type OfficeSceneState = {
  staff: readonly StaffSummary[]
  activity: readonly OfficeActivityEvent[]
  brainState?: StaffBrainState
  onStaffSelect?: (staffId: string) => void
}

const zones = [
  { id: 'coordination', label: 'Coordination', x: 120, y: 120, width: 240, height: 150, color: 0x273445 },
  { id: 'planning', label: 'Planning', x: 390, y: 120, width: 240, height: 150, color: 0x344b4e },
  { id: 'delivery', label: 'Delivery', x: 120, y: 310, width: 240, height: 150, color: 0x4d4544 },
  { id: 'review', label: 'Review', x: 390, y: 310, width: 240, height: 150, color: 0x3e465a },
]

const roleZone: Record<StaffSummary['role'], string> = {
  coordinator: 'coordination', planner: 'planning', implementer: 'delivery', reviewer: 'review', specialist: 'review',
}

const availabilityColor: Record<StaffSummary['availability'], number> = {
  available: 0xd7f158, busy: 0xff806d, unavailable: 0xe0b54d, offline: 0x89939c,
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
    this.cameras.main.setBackgroundColor('#151d28')
    this.renderState()
  }

  private renderState() {
    this.children.removeAll(true)
    this.avatars.clear()
    this.speechBubbles.clear()

    zones.forEach((zone) => {
      const panel = this.add.rectangle(zone.x, zone.y, zone.width, zone.height, zone.color).setOrigin(0, 0).setStrokeStyle(1, 0x63707b, 0.6)
      panel.setInteractive({ useHandCursor: false })
      this.add.text(zone.x + 16, zone.y + 14, zone.label.toUpperCase(), { color: '#afbdc6', fontFamily: 'monospace', fontSize: '12px', letterSpacing: 2 })
    })

    const used: Record<string, number> = {}
    this.officeState.staff.forEach((member) => {
      const zone = zones.find((candidate) => candidate.id === roleZone[member.role]) ?? zones[0]
      const index = used[zone.id] ?? 0
      used[zone.id] = index + 1
      const x = zone.x + 38 + (index % 4) * 50
      const y = zone.y + 68 + Math.floor(index / 4) * 55
      const avatar = this.add.container(x, y)

      // Base body circle
      const body = this.add.circle(0, 0, 15, 0x92a5b4).setStrokeStyle(3, availabilityColor[member.availability])
      // Name label
      const name = this.add.text(0, 21, member.name.slice(0, 12), { color: '#e9eef0', fontFamily: 'monospace', fontSize: '10px' }).setOrigin(0.5, 0)

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

  private brainStateColor(state: StaffBrainState['state']): number {
    switch (state) {
      case 'planning':
      case 'executing':
        return 0x8faeff // blue
      case 'streaming':
        return 0xd7f158 // acid green
      case 'completed':
        return 0x88bf48 // green
      case 'failed':
      case 'canceled':
        return 0xff806d // coral
      default:
        return 0x92a5b4 // default gray
    }
  }

  private createSpeechBubble(container: Phaser.GameObjects.Container, staffId: string, text: string, state: StaffBrainState['state']) {
    const maxWidth = 140
    const padding = 8
    // Truncate for performance
    const displayText = text.length > 120 ? text.slice(0, 117) + '…' : text

    const bubble = this.add.container(0, -38)
    const bg = this.add.rectangle(0, 0, maxWidth + padding * 2, 28, 0x151d28, 0.95).setStrokeStyle(1, this.brainStateColor(state), 0.8).setOrigin(0.5)
    const txt = this.add.text(0, 0, displayText, {
      color: '#e9eef0',
      fontFamily: 'monospace',
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
  const game = new Phaser.Game({
    type: Phaser.AUTO,
    parent,
    width: 720,
    height: 510,
    backgroundColor: '#151d28',
    scene: OfficeScene,
    banner: false,
    render: { antialias: true, pixelArt: false },
    audio: { noAudio: true },
  })
  game.events.once('ready', () => (game.scene.getScene('office') as OfficeScene).setOfficeState(state))
  return game
}
