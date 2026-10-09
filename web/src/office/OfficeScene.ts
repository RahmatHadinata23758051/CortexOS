import Phaser from 'phaser'
import type { OfficeActivityEvent, StaffSummary } from '../types/office'

export type OfficeSceneState = {
  staff: readonly StaffSummary[]
  activity: readonly OfficeActivityEvent[]
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

  create() {
    this.cameras.main.setBackgroundColor('#151d28')
    this.renderState()
  }

  private renderState() {
    this.children.removeAll(true)
    this.avatars.clear()
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
      const body = this.add.circle(0, 0, 15, 0x92a5b4).setStrokeStyle(3, availabilityColor[member.availability])
      const name = this.add.text(0, 21, member.name.slice(0, 12), { color: '#e9eef0', fontFamily: 'monospace', fontSize: '10px' }).setOrigin(0.5, 0)
      avatar.add([body, name]).setSize(42, 44).setInteractive({ useHandCursor: true })
      avatar.on('pointerup', () => this.officeState.onStaffSelect?.(member.id))
      this.avatars.set(member.id, avatar)
    })
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
