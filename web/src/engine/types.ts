// The shape of a game — the same as internal/gamedoc on the server.

export type LookType = 'box' | 'circle' | 'triangle' | 'emoji' | 'image' | 'text' | 'none'

export interface Look {
  type: LookType
  color?: string
  line?: string
  emoji?: string
  image?: number
  fps?: number
  round?: number
  size?: number
  bold?: boolean
  textColor?: string
}

export interface Beh { t: string; p?: Record<string, unknown> }

export interface Obj {
  id: string
  name: string
  role: string
  w: number
  h: number
  look: Look
  behaviors: Beh[]
  z: number
  text?: string
}

export interface Inst { id: string; o: string; x: number; y: number; w?: number; h?: number; flip?: boolean; text?: string; color?: string }

export type Step = { t: string; [k: string]: unknown }

export interface Rule { id: string; on: boolean; once?: boolean; note?: string; when: Step[]; do: Step[] }

export interface Scene {
  id: string; name: string; w: number; h: number; bg: string; bgImage?: number; music?: string; grid: number
  items: Inst[]; rules: Rule[]; camera?: string
}

export interface Sound { id: string; name: string; sfx?: Record<string, unknown>; asset?: number; loop?: boolean }

export interface Hud { id: string; text: string; x: number; y: number; size: number; color: string; align: 'left' | 'center' | 'right' }

export interface GameDoc {
  v: 1
  title: string
  width: number
  height: number
  pixel: boolean
  font: 'round' | 'pixel' | 'plain'
  start: string
  controls: 'auto' | 'none' | 'dpad' | 'sides' | 'jump'
  score: { var: string; kind: 'high' | 'low' | 'time'; label: string }
  intro: { on: boolean; title: string; text: string; button: string }
  vars: { name: string; value: number }[]
  hud: Hud[]
  objects: Obj[]
  scenes: Scene[]
  rules: Rule[]
  sounds: Sound[]
  ends: { won: string; lost: string }
}

export const SIZES: Record<string, [number, number, string]> = {
  wide: [480, 270, 'Wide (phone on its side, computer)'],
  tall: [270, 480, 'Tall (phone upright)'],
  square: [360, 360, 'Square'],
  big: [640, 360, 'Big wide'],
}

/** Fill in what an older or hand-made document leaves out. */
export function normalize(d: Partial<GameDoc>): GameDoc {
  const doc = {
    v: 1, title: 'My game', width: 480, height: 270, pixel: false, font: 'round', start: '', controls: 'auto',
    score: { var: 'score', kind: 'high', label: 'Score' }, intro: { on: true, title: '', text: '', button: 'Play' },
    vars: [], hud: [], objects: [], scenes: [], rules: [], sounds: [], ends: { won: 'You did it!', lost: 'Game over' },
    ...d,
  } as GameDoc
  doc.score = { var: 'score', kind: 'high', label: 'Score', ...(d.score || {}) }
  doc.intro = { on: true, title: '', text: '', button: 'Play', ...(d.intro || {}) }
  doc.ends = { won: 'You did it!', lost: 'Game over', ...(d.ends || {}) }
  for (const k of ['vars', 'hud', 'objects', 'scenes', 'rules', 'sounds'] as const) if (!Array.isArray(doc[k])) (doc as unknown as Record<string, unknown[]>)[k] = []
  for (const o of doc.objects) { o.behaviors ||= []; o.look ||= { type: 'box', color: '#888888' }; o.z ||= 0 }
  for (const s of doc.scenes) { s.items ||= []; s.rules ||= []; s.grid ||= 24; s.w ||= doc.width; s.h ||= doc.height; s.bg ||= '#222222' }
  for (const r of [...doc.rules, ...doc.scenes.flatMap(s => s.rules)]) { r.when ||= []; r.do ||= []; if (r.on === undefined) r.on = true }
  if (!doc.scenes.find(s => s.id === doc.start)) doc.start = doc.scenes[0]?.id || ''
  return doc
}

export function newId() {
  const abc = 'abcdefghijkmnpqrstuvwxyz23456789'
  let s = ''
  for (let i = 0; i < 8; i++) s += abc[Math.floor(Math.random() * abc.length)]
  return s
}

/** Asset ids a document uses. */
export function assetIds(doc: GameDoc): number[] {
  const out = new Set<number>()
  for (const o of doc.objects) if (o.look.image) out.add(o.look.image)
  for (const s of doc.scenes) if (s.bgImage) out.add(s.bgImage)
  for (const s of doc.sounds) if (s.asset) out.add(s.asset)
  return [...out]
}

/** A strip of animation frames side by side: how many frames a picture has. */
export interface AssetInfo { id: number; frames: number; width: number; height: number; kind: string; name: string }
