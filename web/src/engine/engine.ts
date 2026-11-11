// The player: runs a game in a box on the page. The same code plays in the
// editor, on the shared page and in a game saved as one HTML file.
import { drawLook, fontFamily, image, wrap } from './draw'
import { audio, bufferOf, playBuffer, sfxOf } from './sfx'
import { normalize, type GameDoc, type Obj, type Rule, type Scene, type Step } from './types'

export interface EndResult { result: 'won' | 'lost' | 'quit'; score: number; seconds: number; level: string; text: string }

export interface PlayOptions {
  assetURL: (id: number) => string
  frames?: (id: number) => number          // animation frames in a picture
  sound?: boolean
  startScene?: string                        // test from this scene
  skipIntro?: boolean
  onEnd?: (r: EndResult) => void
  endExtra?: (r: EndResult, again: () => void) => HTMLElement | null   // a name box, best scores…
  introExtra?: () => HTMLElement | null
  onError?: (msg: string) => void
}

export interface Player { destroy: () => void; restart: () => void; focus: () => void; doc: GameDoc }

type Key = 'left' | 'right' | 'up' | 'down' | 'jump' | 'action'

interface Thing {
  n: number; id: string; o: Obj; x: number; y: number; w: number; h: number; vx: number; vy: number
  px: number; py: number; pvy: number
  dead: boolean; hidden: boolean; flip: boolean; text?: string; color?: string; emoji?: string
  ground: boolean; sx: number; sy: number; born: number; rot: number; dir: number; cool: number
  beh: Map<string, Record<string, unknown>>
  dragging?: boolean; blink: number
}

const num = (v: unknown, d: number) => (typeof v === 'number' && isFinite(v) ? v : typeof v === 'string' && v !== '' && isFinite(Number(v)) ? Number(v) : d)
const overlap = (a: Thing, b: Thing, e = 0) => a.x < b.x + b.w + e && a.x + a.w + e > b.x && a.y < b.y + b.h + e && a.y + a.h + e > b.y

const KEYMAP: Record<string, Key[]> = {
  ArrowLeft: ['left'], KeyA: ['left'], ArrowRight: ['right'], KeyD: ['right'],
  ArrowUp: ['up', 'jump'], KeyW: ['up', 'jump'], ArrowDown: ['down'], KeyS: ['down'],
  Space: ['jump', 'action'], KeyX: ['action'], KeyZ: ['jump', 'action'], Enter: ['action'], KeyF: ['action'],
}

const EVENT_CONDS = new Set(['start', 'every', 'after', 'touch', 'stomp', 'tap', 'leaves', 'chance'])

export function play(container: HTMLElement, rawDoc: GameDoc, opts: PlayOptions): Player {
  const doc = normalize(JSON.parse(JSON.stringify(rawDoc)))
  const W = doc.width, H = doc.height
  const FONT = fontFamily(doc.font)
  container.innerHTML = ''
  const root = document.createElement('div')
  root.className = 'gb-player'
  root.tabIndex = 0
  const canvas = document.createElement('canvas')
  canvas.className = 'gb-canvas'
  const overlay = document.createElement('div')
  overlay.className = 'gb-overlay'
  const pad = document.createElement('div')
  pad.className = 'gb-pad'
  root.append(canvas, overlay, pad)
  container.append(root)
  const ctx = canvas.getContext('2d')!

  // ---- sizing: the screen keeps its shape and fits the box ----
  let scale = 1
  const fit = () => {
    const r = container.getBoundingClientRect()
    const cw = Math.max(50, r.width), ch = Math.max(50, r.height || r.width * H / W)
    let s = Math.min(cw / W, ch / H)
    if (doc.pixel && s >= 1) s = Math.floor(s)
    scale = s
    const dpr = Math.min(3, window.devicePixelRatio || 1)
    canvas.style.width = `${Math.round(W * s)}px`
    canvas.style.height = `${Math.round(H * s)}px`
    canvas.width = Math.round(W * s * dpr)
    canvas.height = Math.round(H * s * dpr)
    root.style.setProperty('--gb-w', `${Math.round(W * s)}px`)
    root.style.setProperty('--gb-h', `${Math.round(H * s)}px`)
    // an upright phone with a wide game: the buttons go under the game, not over it
    root.classList.toggle('pad-below', ch - H * s >= 120)
  }
  const ro = new ResizeObserver(fit)
  ro.observe(container)
  fit()

  // ---- sounds ----
  const soundOn = opts.sound !== false
  const sfxBuf = new Map<string, AudioBuffer | null>()
  const fileBuf = new Map<number, AudioBuffer | null>()
  let music: HTMLAudioElement | null = null
  let musicId = ''
  const loadSoundFile = (id: number) => {
    if (fileBuf.has(id)) return
    fileBuf.set(id, null)
    const ac = audio()
    if (!ac) return
    fetch(opts.assetURL(id)).then(r => r.arrayBuffer()).then(b => ac.decodeAudioData(b)).then(buf => fileBuf.set(id, buf)).catch(() => {})
  }
  for (const s of doc.sounds) if (s.asset && !s.loop) loadSoundFile(s.asset)
  const playSound = (id: string) => {
    if (!soundOn) return
    const s = doc.sounds.find(x => x.id === id)
    if (!s) return
    if (s.asset) { const b = fileBuf.get(s.asset); if (b) playBuffer(b); else loadSoundFile(s.asset); return }
    if (!sfxBuf.has(id)) sfxBuf.set(id, bufferOf(sfxOf(s.sfx)))
    playBuffer(sfxBuf.get(id) || null)
  }
  const setMusic = (id: string) => {
    if (id === musicId) return
    musicId = id
    music?.pause()
    music = null
    if (!id || !soundOn) return
    const s = doc.sounds.find(x => x.id === id)
    if (!s?.asset) return
    music = new Audio(opts.assetURL(s.asset))
    music.loop = true
    music.volume = 0.5
    void music.play().catch(() => {})
  }

  // ---- input ----
  const held = new Set<Key>()
  let pressed = new Set<Key>()
  const down = (k: Key) => { if (!held.has(k)) pressed.add(k); held.add(k) }
  const up = (k: Key) => { held.delete(k) }
  const onKey = (e: KeyboardEvent) => {
    const ks = KEYMAP[e.code]
    if (!ks) return
    if (state !== 'play') { if (e.type === 'keydown' && (e.code === 'Space' || e.code === 'Enter')) { e.preventDefault(); overlay.querySelector<HTMLButtonElement>('button.gb-go')?.click() } return }
    e.preventDefault()
    for (const k of ks) (e.type === 'keydown' ? down : up)(k)
  }
  root.addEventListener('keydown', onKey)
  root.addEventListener('keyup', onKey)
  const blur = () => held.clear()
  window.addEventListener('blur', blur)

  let pointer: { x: number; y: number; down: boolean; active: boolean } = { x: 0, y: 0, down: false, active: false }
  let taps: { x: number; y: number }[] = []
  let drag: Thing | null = null
  const toWorld = (e: PointerEvent) => {
    const r = canvas.getBoundingClientRect()
    return { x: (e.clientX - r.left) / scale + cam.x, y: (e.clientY - r.top) / scale + cam.y }
  }
  canvas.addEventListener('pointerdown', e => {
    root.focus({ preventScroll: true })
    if (state !== 'play') return
    const p = toWorld(e)
    pointer = { ...p, down: true, active: true }
    taps.push(p)
    const hit = topAt(p.x, p.y, t => t.beh.has('drag'))
    if (hit) { drag = hit; hit.dragging = true; canvas.setPointerCapture(e.pointerId) }
  })
  canvas.addEventListener('pointermove', e => { const p = toWorld(e); pointer.x = p.x; pointer.y = p.y; if (e.pointerType === 'mouse' || pointer.down) pointer.active = true })
  const release = () => { pointer.down = false; if (drag) { drag.dragging = false; drag = null } }
  canvas.addEventListener('pointerup', release)
  canvas.addEventListener('pointercancel', release)
  canvas.addEventListener('pointerleave', () => { if (!pointer.down) pointer.active = false })

  // on-screen buttons on touch screens
  const touch = window.matchMedia('(pointer: coarse)').matches
  const behs = new Set(doc.objects.flatMap(o => o.behaviors.map(b => b.t)))
  let layout = doc.controls
  if (layout === 'auto') layout = behs.has('platformer') ? 'jump' : behs.has('topdown') ? 'dpad' : (behs.has('paddle') || behs.has('shooter')) ? 'sides' : 'none'
  const button = (label: string, keys: Key[], cls: string) => {
    const b = document.createElement('button')
    b.type = 'button'
    b.className = `gb-btn ${cls}`
    b.textContent = label
    b.setAttribute('aria-label', keys.join(' '))
    const on = (e: Event) => { e.preventDefault(); keys.forEach(down); b.classList.add('on') }
    const off = (e: Event) => { e.preventDefault(); keys.forEach(up); b.classList.remove('on') }
    b.addEventListener('pointerdown', on)
    b.addEventListener('pointerup', off)
    b.addEventListener('pointercancel', off)
    b.addEventListener('pointerleave', off)
    pad.append(b)
  }
  if (touch && layout !== 'none') {
    pad.dataset.layout = layout
    if (layout === 'dpad') { button('◀', ['left'], 'l'); button('▶', ['right'], 'r'); button('▲', ['up'], 'u'); button('▼', ['down'], 'd') } else { button('◀', ['left'], 'l'); button('▶', ['right'], 'r') }
    if (layout === 'jump') button('▲', ['jump', 'action'], 'a')
    else if (behs.has('shooter') || layout === 'dpad') button('●', ['action', 'jump'], 'a')
  }

  // ---- the world ----
  let things: Thing[] = []
  let serial = 0
  let scene: Scene = doc.scenes[0]
  let vars: Record<string, number> = {}
  let time = 0, total = 0
  let state: 'intro' | 'play' | 'won' | 'lost' = 'intro'
  const cam = { x: 0, y: 0 }
  let shake = 0
  let message: { text: string; until: number } | null = null
  let touching = new Set<string>()
  let fired = new Set<string>()
  let wasTrue = new Map<string, boolean>()
  let later: { at: number; fn: () => void }[] = []
  let pairs: [string, string][] = []
  let endText = ''
  const objById = new Map(doc.objects.map(o => [o.id, o]))
  const startOf = new Map<string, { x: number; y: number }>()

  const behP = (t: Thing, b: string) => t.beh.get(b)
  const make = (o: Obj, x: number, y: number, extra: Partial<Thing> = {}): Thing => {
    const t: Thing = {
      n: serial++, id: '', o, x, y, w: o.w, h: o.h, vx: 0, vy: 0, px: x, py: y, pvy: 0, dead: false, hidden: false, flip: false,
      ground: false, sx: x, sy: y, born: time, rot: 0, dir: 1, cool: 0, beh: new Map(o.behaviors.map(b => [b.t, b.p || {}])), blink: 0, ...extra,
    }
    t.sx = t.x; t.sy = t.y
    const mv = behP(t, 'move')
    if (mv) {
      const spread = num(mv.spread, 0)
      t.vx = num(mv.vx, 0); t.vy = num(mv.vy, 0)
      if (spread) { const k = 1 + (Math.random() * 2 - 1) * spread / Math.max(1, Math.hypot(t.vx, t.vy)); t.vx *= k; t.vy *= k }
    }
    const bo = behP(t, 'bounce')
    if (bo) { const a = num(bo.angle, -60) * Math.PI / 180, s = num(bo.speed, 220); t.vx = Math.cos(a) * s; t.vy = Math.sin(a) * s }
    if (o.role === 'enemy' || behP(t, 'patrol')) t.dir = -1
    return t
  }

  const enter = (sc: Scene) => {
    scene = sc
    time = 0
    things = []
    startOf.clear()
    for (const it of sc.items) {
      const o = objById.get(it.o)
      if (!o) continue
      const t = make(o, it.x, it.y, { id: it.id, w: it.w || o.w, h: it.h || o.h, flip: !!it.flip, text: it.text ?? o.text, color: it.color })
      things.push(t)
      if (!startOf.has(o.id)) startOf.set(o.id, { x: t.x, y: t.y })
    }
    touching = new Set()
    fired = new Set()
    wasTrue = new Map()
    later = []
    message = null
    const ps = new Set<string>()
    for (const r of [...doc.rules, ...sc.rules]) for (const c of r.when) if ((c.t === 'touch' || c.t === 'stomp') && c.a && c.b) ps.add(`${c.a}|${c.b}`)
    pairs = [...ps].map(p => p.split('|') as [string, string])
    setMusic(sc.music || '')
    const focus = camTarget()
    if (focus) centerOn(focus, true)
    else { cam.x = 0; cam.y = 0 }
  }

  const resetVars = () => { vars = {}; for (const v of doc.vars) vars[v.name] = Number(v.value) || 0 }

  const begin = () => {
    resetVars()
    total = 0
    state = 'play'
    overlay.innerHTML = ''
    overlay.classList.remove('show')
    const first = doc.scenes.find(s => s.id === (opts.startScene || doc.start)) || doc.scenes[0]
    if (!first) { opts.onError?.('This game has no scenes yet.'); return }
    enter(first)
    root.focus({ preventScroll: true })
    const ac = audio()
    if (ac?.state === 'suspended') void ac.resume()
  }

  // ---- overlays (intro and the end) ----
  const el = (tag: string, cls: string, text?: string) => { const e = document.createElement(tag); e.className = cls; if (text !== undefined) e.textContent = text; return e }
  const showIntro = () => {
    state = 'intro'
    overlay.innerHTML = ''
    const card = el('div', 'gb-card')
    card.append(el('h2', 'gb-title', doc.intro.title || doc.title))
    if (doc.intro.text) card.append(el('p', 'gb-text', doc.intro.text))
    const extra = opts.introExtra?.()
    if (extra) card.append(extra)
    const go = el('button', 'gb-go', doc.intro.button || 'Play') as HTMLButtonElement
    go.type = 'button'
    go.onclick = begin
    card.append(go)
    overlay.append(card)
    overlay.classList.add('show')
  }
  const scoreNow = () => doc.score.kind === 'time' || !doc.score.var ? Math.round(total * 10) / 10 : (vars[doc.score.var] ?? 0)
  const finish = (result: 'won' | 'lost', text: string) => {
    if (state !== 'play') return
    state = result
    endText = text
    held.clear()
    playSound(result === 'won' ? 'win' : 'lose')
    const r: EndResult = { result, score: scoreNow(), seconds: Math.round(total * 10) / 10, level: scene.name, text }
    opts.onEnd?.(r)
    setTimeout(() => {
      overlay.innerHTML = ''
      const card = el('div', `gb-card ${result}`)
      card.append(el('div', 'gb-big', result === 'won' ? '🏆' : '💫'))
      card.append(el('h2', 'gb-title', text || (result === 'won' ? doc.ends.won : doc.ends.lost)))
      const label = doc.score.kind === 'time' || !doc.score.var ? 'Time' : (doc.score.label || 'Score')
      card.append(el('p', 'gb-score', `${label}: ${doc.score.kind === 'time' || !doc.score.var ? r.score + ' s' : r.score}`))
      const extra = opts.endExtra?.(r, again)
      if (extra) card.append(extra)
      const go = el('button', 'gb-go', 'Play again') as HTMLButtonElement
      go.type = 'button'
      go.onclick = again
      card.append(go)
      overlay.append(card)
      overlay.classList.add('show')
    }, 350)
  }
  const again = () => begin()

  // ---- looking for things ----
  const live = (o?: string) => things.filter(t => !t.dead && (!o || t.o.id === o))
  const solids = () => things.filter(t => !t.dead && !t.hidden && t.beh.has('solid'))
  const oneways = () => things.filter(t => !t.dead && !t.hidden && t.beh.has('oneway'))
  const topAt = (x: number, y: number, ok: (t: Thing) => boolean = () => true) => {
    let best: Thing | null = null
    for (const t of things) if (!t.dead && !t.hidden && ok(t) && x >= t.x && x <= t.x + t.w && y >= t.y && y <= t.y + t.h && (!best || t.o.z >= best.o.z)) best = t
    return best
  }

  // ---- moving with walls ----
  const moveX = (t: Thing, dx: number, ss: Thing[]): Thing | null => {
    t.x += dx
    for (const s of ss) {
      if (s === t || !overlap(t, s)) continue
      t.x = dx > 0 ? s.x - t.w : s.x + s.w
      return s
    }
    return null
  }
  const moveY = (t: Thing, dy: number, ss: Thing[], ow: Thing[]): Thing | null => {
    const prevBottom = t.y + t.h
    t.y += dy
    for (const s of ss) {
      if (s === t || !overlap(t, s)) continue
      t.y = dy > 0 ? s.y - t.h : s.y + s.h
      return s
    }
    if (dy > 0) for (const s of ow) {
      if (s === t || !overlap(t, s)) continue
      if (prevBottom <= s.y + 0.5) { t.y = s.y - t.h; return s }
    }
    return null
  }
  const groundAhead = (t: Thing, ss: Thing[], ow: Thing[]) => {
    const fx = t.dir > 0 ? t.x + t.w + 1 : t.x - 1
    const fy = t.y + t.h + 2
    return [...ss, ...ow].some(s => s !== t && fx >= s.x && fx <= s.x + s.w && fy >= s.y && fy <= s.y + s.h)
  }

  const camTarget = (): Thing | null => {
    if (scene.camera) { const t = live(scene.camera)[0]; if (t) return t }
    return things.find(t => !t.dead && t.beh.has('camera')) || null
  }
  const centerOn = (t: Thing, snap = false) => {
    const tx = Math.max(0, Math.min(scene.w - W, t.x + t.w / 2 - W / 2))
    const ty = Math.max(0, Math.min(scene.h - H, t.y + t.h / 2 - H / 2))
    if (snap) { cam.x = tx; cam.y = ty } else { cam.x += (tx - cam.x) * 0.15; cam.y += (ty - cam.y) * 0.15 }
  }

  // ---- one step of the world ----
  const step = (dt: number) => {
    const ss = solids(), ow = oneways()
    const left: Thing[] = []
    for (const t of things) {
      if (t.dead) continue
      t.px = t.x; t.py = t.y; t.pvy = t.vy
      if (t.blink > 0) t.blink -= dt
      if (t.dragging) { t.x = pointer.x - t.w / 2; t.y = pointer.y - t.h / 2; continue }
      const pf = behP(t, 'platformer')
      if (pf) {
        const speed = num(pf.speed, 150), jump = num(pf.jump, 380), g = num(pf.gravity, 1000)
        const dir = (held.has('right') ? 1 : 0) - (held.has('left') ? 1 : 0)
        t.vx = dir * speed
        if (dir) t.flip = dir < 0
        const st = pf as Record<string, number>
        st._coyote = t.ground ? 0.1 : Math.max(0, (st._coyote || 0) - dt)
        st._buffer = pressed.has('jump') ? 0.12 : Math.max(0, (st._buffer || 0) - dt)
        if (st._buffer > 0 && st._coyote > 0) { t.vy = -jump; st._buffer = 0; st._coyote = 0; t.ground = false }
        else if (st._buffer > 0 && pf.doubleJump && !st._double && !t.ground) { t.vy = -jump * 0.9; st._buffer = 0; st._double = 1 }
        if (!held.has('jump') && t.vy < -jump * 0.45) t.vy = -jump * 0.45
        t.vy = Math.min(t.vy + g * dt, 900)
        moveX(t, t.vx * dt, ss)
        const hit = moveY(t, t.vy * dt, ss, ow)
        t.ground = !!hit && t.vy > 0
        if (hit) { if (t.vy > 0) st._double = 0; t.vy = 0 }
      }
      const td = behP(t, 'topdown')
      if (td) {
        const speed = num(td.speed, 120)
        let dx = (held.has('right') ? 1 : 0) - (held.has('left') ? 1 : 0)
        let dy = (held.has('down') ? 1 : 0) - (held.has('up') ? 1 : 0)
        if (dx && dy) { dx *= Math.SQRT1_2; dy *= Math.SQRT1_2 }
        if (dx) t.flip = dx < 0
        t.vx = dx * speed; t.vy = dy * speed
        moveX(t, t.vx * dt, ss)
        moveY(t, t.vy * dt, ss, [])
      }
      const pa = behP(t, 'patrol')
      if (pa) {
        const speed = num(pa.speed, 50)
        if (pa.axis === 'y') { if (moveY(t, t.dir * speed * dt, ss, [])) t.dir *= -1 }
        else {
          if (moveX(t, t.dir * speed * dt, ss)) t.dir *= -1
          else if (pa.edges && t.ground && !groundAhead(t, ss, ow)) t.dir *= -1
          t.flip = t.dir > 0
        }
      }
      const gr = behP(t, 'gravity')
      if (gr && !pf) {
        t.vy = Math.min(t.vy + num(gr.g, 1000) * dt, 900)
        const hit = moveY(t, t.vy * dt, ss, ow)
        t.ground = !!hit && t.vy > 0
        if (hit) t.vy = 0
      }
      const fo = behP(t, 'follow')
      if (fo) {
        const tg = live(String(fo.target || ''))
        let best: Thing | null = null, bd = Infinity
        for (const x of tg) { const d = Math.hypot(x.x - t.x, x.y - t.y); if (d < bd) { bd = d; best = x } }
        const range = num(fo.range, 0)
        if (best && (!range || bd < range) && bd > 1) {
          const sp = num(fo.speed, 50) * dt
          const dx = (best.x + best.w / 2 - t.x - t.w / 2) / bd * sp, dy = (best.y + best.h / 2 - t.y - t.h / 2) / bd * sp
          if (fo.walls) { moveX(t, dx, ss); moveY(t, dy, ss, []) } else { t.x += dx; t.y += dy }
          if (dx) t.flip = dx < 0
        }
      }
      if (behP(t, 'move')) { t.x += t.vx * dt; t.y += t.vy * dt }
      const bo = behP(t, 'bounce')
      if (bo) {
        const hx = moveX(t, t.vx * dt, ss)
        if (hx) {
          if (hx.beh.has('paddle')) bounceOff(t, hx, true)
          else t.vx = -t.vx
        }
        const hy = moveY(t, t.vy * dt, ss, [])
        if (hy) {
          if (hy.beh.has('paddle')) bounceOff(t, hy, false)
          else t.vy = -t.vy
        }
        if (t.x < 0) { t.x = 0; t.vx = Math.abs(t.vx) }
        if (t.x + t.w > scene.w) { t.x = scene.w - t.w; t.vx = -Math.abs(t.vx) }
        if (t.y < 0) { t.y = 0; t.vy = Math.abs(t.vy) }
        if (bo.floor && t.y + t.h > scene.h) { t.y = scene.h - t.h; t.vy = -Math.abs(t.vy) }
        if (Math.abs(t.vy) < 30) t.vy = t.vy < 0 ? -30 : 30
      }
      const pd = behP(t, 'paddle')
      if (pd) {
        const speed = num(pd.speed, 260)
        const axisY = pd.axis === 'y'
        const dir = axisY ? (held.has('down') ? 1 : 0) - (held.has('up') ? 1 : 0) : (held.has('right') ? 1 : 0) - (held.has('left') ? 1 : 0)
        if (dir) { if (axisY) t.y += dir * speed * dt; else t.x += dir * speed * dt }
        else if (pd.pointer !== false && pointer.active && pointer.down) {
          const target = axisY ? pointer.y - t.h / 2 : pointer.x - t.w / 2
          const cur = axisY ? t.y : t.x
          const mv = Math.max(-speed * 2 * dt, Math.min(speed * 2 * dt, target - cur))
          if (axisY) t.y += mv; else t.x += mv
        }
      }
      const sh = behP(t, 'shooter')
      if (sh) {
        t.cool -= dt
        const want = sh.auto ? true : held.has('action')
        if (want && t.cool <= 0) {
          const bo2 = objById.get(String(sh.bullet || ''))
          if (bo2) {
            t.cool = num(sh.rate, 0.3)
            const sp = num(sh.speed, 360)
            const d = String(sh.dir || 'up')
            let vx = 0, vy = -sp
            if (d === 'down') vy = sp
            else if (d === 'facing') { vy = 0; vx = t.flip ? -sp : sp }
            const b = make(bo2, t.x + t.w / 2 - bo2.w / 2, d === 'down' ? t.y + t.h : t.y - bo2.h / 2)
            if (d === 'facing') b.y = t.y + t.h / 2 - bo2.h / 2
            b.vx = vx; b.vy = vy
            if (!b.beh.has('move')) b.beh.set('move', { gone: true })
            things.push(b)
          }
        }
      }
      const sp = behP(t, 'spin')
      if (sp) t.rot += num(sp.speed, 90) * dt
      if (behP(t, 'wrap') || (behP(t, 'move') && behP(t, 'move')!.wrap)) {
        if (t.x > scene.w) t.x = -t.w; else if (t.x + t.w < 0) t.x = scene.w
        if (t.y > scene.h) t.y = -t.h; else if (t.y + t.h < 0) t.y = scene.h
      }
      if (behP(t, 'stayin')) { t.x = Math.max(0, Math.min(scene.w - t.w, t.x)); t.y = Math.max(-t.h * 4, Math.min(scene.h - t.h, t.y)) }
      const fa = behP(t, 'fade')
      if (fa && time - t.born > num(fa.after, 1)) t.dead = true
      // leaving the level
      const out = t.x + t.w < 0 ? 'left' : t.x > scene.w ? 'right' : t.y + t.h < -40 ? 'top' : t.y > scene.h ? 'bottom' : ''
      if (out) {
        const wasIn = !(t.px + t.w < 0 || t.px > scene.w || t.py + t.h < -40 || t.py > scene.h)
        if (wasIn) { left.push(t); (t as Thing & { side?: string }).side = out }
        const mv = behP(t, 'move')
        if (mv && mv.gone !== false && !mv.wrap && (t.y > scene.h + 60 || t.y + t.h < -200 || t.x > scene.w + 60 || t.x + t.w < -60)) t.dead = true
      }
    }
    const focus = camTarget()
    if (focus) centerOn(focus)

    // what touches what (just began)
    const now = new Set<string>()
    const began: Map<string, [Thing, Thing][]> = new Map()
    for (const [A, B] of pairs) {
      const as = live(A), bs = live(B)
      const list: [Thing, Thing][] = []
      for (const a of as) for (const b of bs) {
        if (a === b || a.hidden || b.hidden) continue
        if (overlap(a, b, 1)) {
          const k = `${a.n}|${b.n}`
          now.add(k)
          if (!touching.has(k)) list.push([a, b])
        }
      }
      began.set(`${A}|${B}`, list)
    }
    touching = now
    const tapped: Thing[] = []
    for (const p of taps) { const t = topAt(p.x, p.y); if (t) tapped.push(t) }
    taps = []

    // the rules
    const prevTime = time - dt
    for (const r of [...doc.rules, ...scene.rules]) {
      if (!r.on || state !== 'play') continue
      if (r.once && fired.has(r.id)) continue
      const binds = evaluate(r, began, tapped, left, prevTime)
      const isEvent = r.when.some(c => EVENT_CONDS.has(c.t) || (c.t === 'key' && c.how !== 'hold'))
      if (!isEvent) {
        const ok = binds.length > 0
        const before = wasTrue.get(r.id) || false
        wasTrue.set(r.id, ok)
        if (!ok || (before && !r.when.some(c => c.t === 'key'))) continue
      }
      for (const bnd of binds) {
        if (bnd.a?.dead || bnd.b?.dead) continue
        run(r.do, bnd)
        fired.add(r.id)
        if (state !== 'play') break
      }
    }
    things = things.filter(t => !t.dead)
    for (const l of later.filter(x => x.at <= total)) l.fn()
    later = later.filter(x => x.at > total)
    pressed = new Set()
  }

  const bounceOff = (ball: Thing, pad: Thing, sideways: boolean) => {
    const sp = Math.hypot(ball.vx, ball.vy)
    if (sideways) { ball.vx = -ball.vx; return }
    const rel = Math.max(-1, Math.min(1, ((ball.x + ball.w / 2) - (pad.x + pad.w / 2)) / (pad.w / 2)))
    const a = (-90 + rel * 60) * Math.PI / 180
    const dirDown = ball.y > pad.y
    ball.vx = Math.cos(a) * sp
    ball.vy = (dirDown ? 1 : -1) * Math.abs(Math.sin(a) * sp)
  }

  type Bind = { a?: Thing; b?: Thing }
  const cmp = (v: number, op: unknown, x: number) => {
    switch (op) {
      case '=': case '==': return Math.abs(v - x) < 1e-9
      case '!=': return Math.abs(v - x) >= 1e-9
      case '<': return v < x
      case '<=': return v <= x
      case '>': return v > x
      default: return v >= x
    }
  }
  const evaluate = (r: Rule, began: Map<string, [Thing, Thing][]>, tapped: Thing[], left: Thing[], prevTime: number): Bind[] => {
    let binds: Bind[] | null = null
    for (const c of r.when) {
      let ok = true
      switch (c.t) {
        case 'start': ok = prevTime < 0 || time <= 1 / 60 + 1e-6; if (fired.has(r.id + ':start')) ok = false; else if (ok) fired.add(r.id + ':start'); break
        case 'every': { const s = Math.max(0.05, num(c.s, 1)); ok = Math.floor(time / s) > Math.floor(Math.max(0, prevTime) / s); break }
        case 'after': { const s = num(c.s, 1); ok = time >= s && prevTime < s; break }
        case 'key': { const k = (c.key || 'action') as Key | 'any'; ok = k === 'any' ? (c.how === 'hold' ? held.size > 0 : pressed.size > 0) : c.how === 'hold' ? held.has(k as Key) : pressed.has(k as Key); break }
        case 'var': ok = cmp(vars[String(c.name)] ?? 0, c.op, num(c.value, 0)); break
        case 'count': ok = cmp(live(String(c.o)).filter(t => !t.hidden).length, c.op, num(c.value, 0)); break
        case 'chance': ok = Math.random() < num(c.p, 10) / 100 * (1 / 60); break
        case 'touch': case 'stomp': {
          let list = began.get(`${c.a}|${c.b}`) || []
          if (c.t === 'stomp') list = list.filter(([a, b]) => a.pvy > 0 && a.py + a.h <= b.y + 6)
          else list = list.filter(([a, b]) => !(objStomps(r, a, b)))
          const bs = list.map(([a, b]) => ({ a, b }))
          if (binds === null) binds = bs; else if (!bs.length) ok = false
          if (!bs.length) ok = false
          break
        }
        case 'tap': {
          const bs = tapped.filter(t => !c.a || t.o.id === c.a).map(t => ({ a: t }))
          if (binds === null) binds = bs
          if (!bs.length) ok = false
          break
        }
        case 'leaves': {
          const bs = left.filter(t => t.o.id === c.a && (!c.side || c.side === 'any' || (t as Thing & { side?: string }).side === c.side)).map(t => ({ a: t }))
          if (binds === null) binds = bs
          if (!bs.length) ok = false
          break
        }
      }
      if (!ok) return []
    }
    return binds || [{}]
  }
  // a stomp rule for the same pair takes the touch (jumping on a snail is not touching it)
  const stompRules = () => [...doc.rules, ...scene.rules].filter(r => r.on && r.when.some(c => c.t === 'stomp'))
  const objStomps = (_r: Rule, a: Thing, b: Thing) => a.pvy > 0 && a.py + a.h <= b.y + 6 && stompRules().some(sr => sr.when.some(c => c.t === 'stomp' && c.a === a.o.id && c.b === b.o.id))

  const who = (w: unknown, bnd: Bind): Thing[] => {
    if (w === 'a') return bnd.a ? [bnd.a] : []
    if (w === 'b') return bnd.b ? [bnd.b] : []
    return live(String(w || ''))
  }
  const at = (where: unknown, o: Obj, bnd: Bind): { x: number; y: number } => {
    const of = (t?: Thing) => t ? { x: t.x + t.w / 2 - o.w / 2, y: t.y + t.h / 2 - o.h / 2 } : { x: cam.x + W / 2 - o.w / 2, y: cam.y + H / 2 - o.h / 2 }
    switch (where) {
      case 'a': return of(bnd.a)
      case 'b': return of(bnd.b)
      case 'top': return { x: cam.x + Math.random() * Math.max(1, W - o.w), y: cam.y - o.h }
      case 'random': return { x: cam.x + Math.random() * Math.max(1, W - o.w), y: cam.y + Math.random() * Math.max(1, H - o.h) }
      case 'start': return startOf.get(o.id) || of()
    }
    return of()
  }
  const run = (acts: Step[], bnd: Bind) => {
    for (const a of acts) {
      if (state !== 'play') return
      switch (a.t) {
        case 'add': vars[String(a.name)] = (vars[String(a.name)] ?? 0) + num(a.value, 1); break
        case 'set': vars[String(a.name)] = num(a.value, 0); break
        case 'destroy': for (const t of who(a.who, bnd)) t.dead = true; break
        case 'hide': for (const t of who(a.who, bnd)) t.hidden = true; break
        case 'show': for (const t of things) if (t.o.id === a.who || (a.who === 'a' && t === bnd.a) || (a.who === 'b' && t === bnd.b)) t.hidden = false; break
        case 'create': {
          const o = objById.get(String(a.o))
          if (!o) break
          const p = at(a.at, o, bnd)
          const t = make(o, p.x, p.y, { text: o.text })
          if (a.vx !== undefined || a.vy !== undefined) { t.vx = num(a.vx, 0); t.vy = num(a.vy, 0) }
          things.push(t)
          break
        }
        case 'moveto': for (const t of who(a.who, bnd)) { t.x = t.sx; t.y = t.sy; t.vx = 0; t.vy = 0; t.px = t.x; t.py = t.y; t.blink = 1; const bo = behP(t, 'bounce'); if (bo) { const an = num(bo.angle, -60) * Math.PI / 180, s = num(bo.speed, 220); t.vx = Math.cos(an) * s; t.vy = Math.sin(an) * s } } break
        case 'jump': for (const t of who(a.who, bnd)) { t.vy = -num(a.power, 300); t.ground = false } break
        case 'push': for (const t of who(a.who, bnd)) { t.vx = num(a.vx, 0); t.vy = num(a.vy, 0) } break
        case 'look': for (const t of who(a.who, bnd)) { if (a.color) t.color = String(a.color); if (a.emoji) t.emoji = String(a.emoji) } break
        case 'sound': playSound(String(a.s)); break
        case 'music': setMusic(a.s === 'stop' ? '' : String(a.s || '')); break
        case 'say': message = { text: fill(String(a.text || '')), until: total + num(a.s, 1.5) }; break
        case 'shake': shake = Math.max(shake, num(a.s, 0.3)); break
        case 'win': finish('won', fill(String(a.text || ''))); return
        case 'lose': finish('lost', fill(String(a.text || ''))); return
        case 'scene': {
          const go = () => {
            if (state !== 'play') return
            if (a.scene === 'restart') { enter(scene); return }
            if (a.scene === 'next') {
              const i = doc.scenes.indexOf(scene)
              const next = doc.scenes[i + 1]
              if (next) enter(next); else finish('won', '')
              return
            }
            const sc = doc.scenes.find(s => s.id === a.scene)
            if (sc) enter(sc)
          }
          const wait = num(a.wait, 0)
          if (wait > 0) later.push({ at: total + wait, fn: go }); else { go(); return }
          break
        }
      }
    }
  }
  const fill = (text: string) => text.replace(/\{(\w+)\}/g, (_, k) => k === 'time' ? String(Math.floor(total)) : k in vars ? String(Math.round(vars[k] * 100) / 100) : `{${k}}`)

  // ---- drawing ----
  const render = () => {
    const k = canvas.width / W
    ctx.setTransform(k, 0, 0, k, 0, 0)
    ctx.imageSmoothingEnabled = !doc.pixel
    ctx.fillStyle = scene?.bg || '#222'
    ctx.fillRect(0, 0, W, H)
    if (scene?.bgImage) {
      const im = image(opts.assetURL(scene.bgImage))
      if (im.complete && im.naturalWidth) {
        const s = Math.max(W / im.naturalWidth, H / im.naturalHeight)
        const px = scene.w > W ? (cam.x / (scene.w - W)) * (im.naturalWidth * s - W) : (im.naturalWidth * s - W) / 2
        ctx.drawImage(im, -px * 0.5 - (im.naturalWidth * s - W) / 4, (H - im.naturalHeight * s) / 2, im.naturalWidth * s, im.naturalHeight * s)
      }
    }
    let ox = 0, oy = 0
    if (shake > 0) { ox = (Math.random() - 0.5) * 6; oy = (Math.random() - 0.5) * 6 }
    ctx.save()
    ctx.translate(-Math.round(cam.x) + ox, -Math.round(cam.y) + oy)
    const list = things.filter(t => !t.dead && !t.hidden).sort((a, b) => a.o.z - b.o.z || a.n - b.n)
    for (const t of list) {
      if (t.x + t.w < cam.x - 20 || t.x > cam.x + W + 20 || t.y + t.h < cam.y - 20 || t.y > cam.y + H + 20) continue
      if (t.blink > 0 && Math.floor(t.blink * 10) % 2 === 0) continue
      const bob = behP(t, 'bob')
      const dy = bob ? Math.sin(total * num(bob.speed, 2) * 2) * num(bob.amount, 3) : 0
      const moving = Math.abs(t.vx) > 1 || Math.abs(t.vy) > 1 || !(behP(t, 'platformer') || behP(t, 'topdown'))
      const fps = t.o.look.fps || 6
      const look = t.emoji ? { ...t.o.look, type: 'emoji' as const, emoji: t.emoji } : t.o.look
      const frames = look.image ? opts.frames?.(look.image) || 1 : 1
      drawLook(ctx, look, t.x, t.y + dy, t.w, t.h, { text: t.text, color: t.color, flip: t.flip, rot: t.rot, frames, frame: moving ? total * fps : 0, imageURL: opts.assetURL, font: FONT,
        alpha: behP(t, 'fade') ? 1 - (time - t.born) / num(behP(t, 'fade')!.after, 1) : undefined })
    }
    ctx.restore()
    // the words on top
    for (const hd of doc.hud) {
      const size = hd.size || 16
      ctx.font = `700 ${size}px ${FONT}`
      ctx.textAlign = hd.align || 'left'
      ctx.textBaseline = 'top'
      ctx.lineWidth = Math.max(2, size / 6)
      ctx.strokeStyle = 'rgba(0,0,0,.35)'
      const text = fill(hd.text)
      ctx.strokeText(text, hd.x, hd.y)
      ctx.fillStyle = hd.color || '#fff'
      ctx.fillText(text, hd.x, hd.y)
    }
    if (message && total < message.until) {
      ctx.font = `700 15px ${FONT}`
      const lines = wrap(ctx, message.text, W - 60)
      const lh = 20, bw = Math.min(W - 30, Math.max(...lines.map(l => ctx.measureText(l).width)) + 32), bh = lines.length * lh + 16
      const bx = (W - bw) / 2, by = H - bh - 18
      ctx.fillStyle = 'rgba(15,18,30,.82)'
      ctx.beginPath()
      ctx.roundRect?.(bx, by, bw, bh, 10)
      ctx.fill()
      ctx.fillStyle = '#fff'
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      lines.forEach((l, i) => ctx.fillText(l, W / 2, by + 8 + lh / 2 + i * lh))
    }
    if (state === 'won' || state === 'lost') { ctx.fillStyle = 'rgba(0,0,0,.25)'; ctx.fillRect(0, 0, W, H) }
    void endText
  }

  // ---- the loop ----
  let raf = 0, last = 0, acc = 0, alive = true
  const DT = 1 / 60
  const frame = (now: number) => {
    if (!alive) return
    raf = requestAnimationFrame(frame)
    const el = last ? Math.min(0.1, (now - last) / 1000) : DT
    last = now
    if (state === 'play') {
      acc += el
      let n = 0
      while (acc >= DT && n < 6) { time += DT; total += DT; step(DT); acc -= DT; n++; if (shake > 0) shake -= DT }
    }
    render()
  }
  // the first scene behind the intro
  resetVars()
  const first = doc.scenes.find(s => s.id === (opts.startScene || doc.start)) || doc.scenes[0]
  if (first) enter(first)
  if (opts.skipIntro || !doc.intro.on) begin(); else showIntro()
  raf = requestAnimationFrame(frame)
  // pictures arriving: draw again
  for (const o of doc.objects) if (o.look.image) image(opts.assetURL(o.look.image))

  return {
    doc,
    focus: () => root.focus({ preventScroll: true }),
    restart: () => begin(),
    destroy: () => {
      if (state === 'play' && total > 10) opts.onEnd?.({ result: 'quit', score: scoreNow(), seconds: Math.round(total), level: scene?.name || '', text: '' })
      alive = false
      cancelAnimationFrame(raf)
      ro.disconnect()
      window.removeEventListener('blur', blur)
      music?.pause()
      container.innerHTML = ''
    },
  }
}

/** A picture of a scene (the first moment), for a game's cover. */
export function snapshot(doc: GameDoc, assetURL: (id: number) => string, frames: (id: number) => number, width = 480): Promise<Blob | null> {
  const d = normalize(JSON.parse(JSON.stringify(doc)))
  const sc = d.scenes.find(s => s.id === d.start) || d.scenes[0]
  // a 16:9 picture; a tall or square screen sits in the middle of it
  const c = document.createElement('canvas')
  c.width = width
  c.height = Math.round(width * 9 / 16)
  const ctx = c.getContext('2d')!
  const k = Math.min(c.width / d.width, c.height / d.height)
  const ox = (c.width - d.width * k) / 2, oy = (c.height - d.height * k) / 2
  const drawAll = () => {
    ctx.fillStyle = sc?.bg || '#222'
    ctx.fillRect(0, 0, c.width, c.height)
    if (ox > 1 || oy > 1) { ctx.fillStyle = 'rgba(0,0,0,.28)'; ctx.fillRect(0, 0, c.width, c.height) }
    ctx.save()
    ctx.translate(ox, oy)
    ctx.scale(k, k)
    ctx.beginPath(); ctx.rect(0, 0, d.width, d.height); ctx.clip()
    ctx.fillStyle = sc?.bg || '#222'
    ctx.fillRect(0, 0, d.width, d.height)
    if (!sc) { ctx.restore(); return }
    const objs = new Map(d.objects.map(o => [o.id, o]))
    const focus = sc.items.find(it => objs.get(it.o)?.behaviors.some(b => b.t === 'camera'))
    let cx = 0, cy = 0
    if (focus) { cx = Math.max(0, Math.min(sc.w - d.width, focus.x - d.width / 3)); cy = Math.max(0, Math.min(sc.h - d.height, focus.y - d.height / 2)) }
    ctx.translate(-cx, -cy)
    const items = [...sc.items].sort((a, b) => (objs.get(a.o)?.z || 0) - (objs.get(b.o)?.z || 0))
    for (const it of items) {
      const o = objs.get(it.o)
      if (!o) continue
      drawLook(ctx, o.look, it.x, it.y, it.w || o.w, it.h || o.h, { text: it.text ?? o.text, color: it.color, flip: it.flip, imageURL: assetURL, frames: o.look.image ? frames(o.look.image) : 1, font: fontFamily(d.font) })
    }
    ctx.restore()
  }
  const urls = d.objects.filter(o => o.look.image).map(o => assetURL(o.look.image!))
  return Promise.all(urls.map(u => new Promise<void>(res => { const im = image(u); if (im.complete) res(); else { im.addEventListener('load', () => res(), { once: true }); im.addEventListener('error', () => res(), { once: true }) } }))).then(() => new Promise(res => {
    drawAll()
    c.toBlob(b => res(b), 'image/png')
  }))
}
