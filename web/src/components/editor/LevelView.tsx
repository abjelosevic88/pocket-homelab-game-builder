// The level on a grid: place things, move them, rub them out, look around.
import { useCallback, useEffect, useRef, useState } from 'react'
import { drawLook, fontFamily } from '../../engine/draw'
import { newId, type GameDoc, type Inst, type Obj, type Scene } from '../../engine/types'

export type Tool = 'place' | 'move' | 'erase' | 'pan'

interface Props {
  doc: GameDoc
  scene: Scene
  brush: Obj | null
  tool: Tool
  selected: string | null
  grid: boolean
  assetURL: (id: number) => string
  frames: (id: number) => number
  onItems: (items: Inst[], merge?: boolean) => void
  onSelect: (id: string | null) => void
}

const objOf = (doc: GameDoc, id: string) => doc.objects.find(o => o.id === id)
const sizeOf = (doc: GameDoc, it: Inst) => { const o = objOf(doc, it.o); return { w: it.w || o?.w || 24, h: it.h || o?.h || 24 } }

/** Where a thing sits when placed in a cell: bottom middle (smaller than the cell), or the cell's corner. */
export function cellPos(g: number, cx: number, cy: number, w: number, h: number) {
  return { x: cx * g + (w < g ? (g - w) / 2 : 0), y: cy * g + (h < g ? g - h : 0) }
}

export function LevelView({ doc, scene, brush, tool, selected, grid, assetURL, frames, onItems, onSelect }: Props) {
  const wrap = useRef<HTMLDivElement>(null)
  const canvas = useRef<HTMLCanvasElement>(null)
  const [size, setSize] = useState({ w: 600, h: 400, measured: false })
  const view = useRef({ x: -20, y: -20, z: 1, fitted: '' })
  const [, setTick] = useState(0)
  const redraw = useCallback(() => setTick(t => t + 1), [])
  const drag = useRef<{ mode: 'paint' | 'erase' | 'move' | 'pan'; sx: number; sy: number; vx: number; vy: number; id?: string; ox?: number; oy?: number; items?: Inst[]; touched?: Set<string> } | null>(null)
  const pointers = useRef(new Map<number, { x: number; y: number }>())
  const pinch = useRef<{ d: number; z: number; cx: number; cy: number } | null>(null)
  const spaceDown = useRef(false)

  // measure the box
  useEffect(() => {
    const el = wrap.current
    if (!el) return
    const measure = () => { const r = el.getBoundingClientRect(); if (r.width > 0 && r.height > 0) setSize({ w: Math.round(r.width), h: Math.round(r.height), measured: true }) }
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [])

  // fit the level the first time a scene shows
  useEffect(() => {
    if (view.current.fitted === scene.id || !size.measured) return
    const z = Math.min(2, Math.max(0.25, Math.min(size.w / (Math.min(scene.w, doc.width * 1.3) + 40), size.h / (scene.h + 40))))
    view.current = { x: -20 / z, y: -Math.max(20 / z, (size.h / z - scene.h) / 3), z, fitted: scene.id }
    redraw()
  }, [scene.id, scene.w, scene.h, size, doc.width, redraw])

  // draw
  useEffect(() => {
    const c = canvas.current
    if (!c) return
    const dpr = Math.min(2, window.devicePixelRatio || 1)
    c.width = size.w * dpr
    c.height = size.h * dpr
    const ctx = c.getContext('2d')!
    const v = view.current
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    const dark = matchMedia('(prefers-color-scheme: dark)').matches || document.documentElement.dataset.theme === 'dark'
    ctx.fillStyle = dark ? '#1a1b20' : '#e9ebf0'
    ctx.fillRect(0, 0, size.w, size.h)
    ctx.setTransform(dpr * v.z, 0, 0, dpr * v.z, -v.x * v.z * dpr, -v.y * v.z * dpr)
    ctx.fillStyle = scene.bg || '#222'
    ctx.fillRect(0, 0, scene.w, scene.h)
    if (grid) {
      const g = scene.grid || 24
      ctx.strokeStyle = 'rgba(255,255,255,.09)'
      ctx.lineWidth = 1 / v.z
      ctx.beginPath()
      for (let x = 0; x <= scene.w; x += g) { ctx.moveTo(x, 0); ctx.lineTo(x, scene.h) }
      for (let y = 0; y <= scene.h; y += g) { ctx.moveTo(0, y); ctx.lineTo(scene.w, y) }
      ctx.stroke()
    }
    const items = [...scene.items].sort((a, b) => (objOf(doc, a.o)?.z || 0) - (objOf(doc, b.o)?.z || 0))
    const font = fontFamily(doc.font)
    for (const it of items) {
      const o = objOf(doc, it.o)
      if (!o) continue
      const { w, h } = sizeOf(doc, it)
      drawLook(ctx, o.look, it.x, it.y, w, h, { text: it.text ?? o.text, color: it.color, flip: it.flip, imageURL: assetURL, frames: o.look.image ? frames(o.look.image) : 1, font })
      if (o.look.type === 'none') { ctx.strokeStyle = 'rgba(255,255,255,.4)'; ctx.setLineDash([3 / v.z, 3 / v.z]); ctx.strokeRect(it.x, it.y, w, h); ctx.setLineDash([]) }
    }
    // the screen, where the game starts looking
    ctx.strokeStyle = 'rgba(255,212,59,.75)'
    ctx.lineWidth = 2 / v.z
    ctx.setLineDash([8 / v.z, 6 / v.z])
    const focus = scene.items.find(it => objOf(doc, it.o)?.behaviors.some(b => b.t === 'camera'))
    let sx = 0, sy = 0
    if (focus) { sx = Math.max(0, Math.min(scene.w - doc.width, focus.x - doc.width / 2)); sy = Math.max(0, Math.min(scene.h - doc.height, focus.y - doc.height / 2)) }
    ctx.strokeRect(sx, sy, doc.width, doc.height)
    ctx.setLineDash([])
    const sel = scene.items.find(i => i.id === selected)
    if (sel) {
      const { w, h } = sizeOf(doc, sel)
      ctx.strokeStyle = '#4fb3ea'
      ctx.lineWidth = 2 / v.z
      ctx.strokeRect(sel.x - 1 / v.z, sel.y - 1 / v.z, w + 2 / v.z, h + 2 / v.z)
    }
    ctx.strokeStyle = 'rgba(127,127,127,.6)'
    ctx.lineWidth = 1 / v.z
    ctx.strokeRect(0, 0, scene.w, scene.h)
  })

  // pictures arriving
  useEffect(() => { const t = setTimeout(redraw, 400); return () => clearTimeout(t) }, [doc.objects, redraw])

  const world = (e: { clientX: number; clientY: number }) => {
    const r = canvas.current!.getBoundingClientRect()
    const v = view.current
    return { x: (e.clientX - r.left) / v.z + v.x, y: (e.clientY - r.top) / v.z + v.y }
  }
  const hit = (p: { x: number; y: number }, items = scene.items) => {
    for (let i = items.length - 1; i >= 0; i--) {
      const it = items[i]
      const { w, h } = sizeOf(doc, it)
      if (p.x >= it.x && p.x <= it.x + w && p.y >= it.y && p.y <= it.y + h) return it
    }
    return null
  }
  const g = scene.grid || 24
  const placeAt = (p: { x: number; y: number }, items: Inst[], touched: Set<string>): Inst[] | null => {
    if (!brush || p.x < 0 || p.y < 0 || p.x >= scene.w || p.y >= scene.h) return null
    const cx = Math.floor(p.x / g), cy = Math.floor(p.y / g)
    const key = `${cx},${cy}`
    if (touched.has(key)) return null
    touched.add(key)
    const pos = cellPos(g, cx, cy, brush.w, brush.h)
    if (items.some(it => it.o === brush.id && Math.abs(it.x - pos.x) < 1 && Math.abs(it.y - pos.y) < 1)) return null
    return [...items, { id: newId(), o: brush.id, x: pos.x, y: pos.y }]
  }

  const onDown = (e: React.PointerEvent) => {
    canvas.current!.setPointerCapture(e.pointerId)
    pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (pointers.current.size === 2) {
      const [a, b] = [...pointers.current.values()]
      pinch.current = { d: Math.hypot(a.x - b.x, a.y - b.y), z: view.current.z, cx: (a.x + b.x) / 2, cy: (a.y + b.y) / 2 }
      drag.current = null
      return
    }
    const p = world(e)
    const v = view.current
    const panning = tool === 'pan' || e.button === 1 || spaceDown.current
    if (panning) { drag.current = { mode: 'pan', sx: e.clientX, sy: e.clientY, vx: v.x, vy: v.y }; return }
    if (tool === 'place') {
      const touched = new Set<string>()
      const next = placeAt(p, scene.items, touched)
      if (next) onItems(next)
      drag.current = { mode: 'paint', sx: e.clientX, sy: e.clientY, vx: v.x, vy: v.y, items: next || scene.items, touched }
      return
    }
    if (tool === 'erase') {
      const it = hit(p)
      const items = it ? scene.items.filter(x => x.id !== it.id) : scene.items
      if (it) onItems(items)
      drag.current = { mode: 'erase', sx: e.clientX, sy: e.clientY, vx: v.x, vy: v.y, items }
      return
    }
    const it = hit(p)
    if (it) { onSelect(it.id); drag.current = { mode: 'move', sx: e.clientX, sy: e.clientY, vx: v.x, vy: v.y, id: it.id, ox: p.x - it.x, oy: p.y - it.y, items: scene.items } }
    else { onSelect(null); drag.current = { mode: 'pan', sx: e.clientX, sy: e.clientY, vx: v.x, vy: v.y } }
  }
  const onMove = (e: React.PointerEvent) => {
    if (pointers.current.has(e.pointerId)) pointers.current.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (pinch.current && pointers.current.size === 2) {
      const [a, b] = [...pointers.current.values()]
      const d = Math.hypot(a.x - b.x, a.y - b.y)
      zoomAt(pinch.current.cx, pinch.current.cy, Math.min(4, Math.max(0.2, pinch.current.z * d / pinch.current.d)))
      return
    }
    const d = drag.current
    if (!d) return
    const v = view.current
    if (d.mode === 'pan') { v.x = d.vx - (e.clientX - d.sx) / v.z; v.y = d.vy - (e.clientY - d.sy) / v.z; redraw(); return }
    const p = world(e)
    if (d.mode === 'paint') { const next = placeAt(p, d.items!, d.touched!); if (next) { d.items = next; onItems(next, true) } return }
    if (d.mode === 'erase') { const it = hit(p, d.items!); if (it) { d.items = d.items!.filter(x => x.id !== it.id); onItems(d.items, true) } return }
    if (d.mode === 'move') {
      const it = d.items!.find(x => x.id === d.id)
      if (!it) return
      const { w, h } = sizeOf(doc, it)
      let x = p.x - d.ox!, y = p.y - d.oy!
      if (!e.shiftKey) {
        const cx = Math.round((x - (w < g ? (g - w) / 2 : 0)) / g), cy = Math.round((y - (h < g ? g - h : 0)) / g)
        const pos = cellPos(g, cx, cy, w, h)
        x = pos.x; y = pos.y
      }
      x = Math.max(-w / 2, Math.min(scene.w - w / 2, x)); y = Math.max(-h / 2, Math.min(scene.h - h / 2, y))
      if (x !== it.x || y !== it.y) onItems(d.items!.map(i => i.id === d.id ? { ...i, x: Math.round(x), y: Math.round(y) } : i), true)
    }
  }
  const onUp = (e: React.PointerEvent) => {
    pointers.current.delete(e.pointerId)
    if (pointers.current.size < 2) pinch.current = null
    drag.current = null
  }
  const zoomAt = (cx: number, cy: number, z: number) => {
    const r = canvas.current!.getBoundingClientRect()
    const v = view.current
    const wx = (cx - r.left) / v.z + v.x, wy = (cy - r.top) / v.z + v.y
    v.z = z
    v.x = wx - (cx - r.left) / z
    v.y = wy - (cy - r.top) / z
    redraw()
  }
  useEffect(() => {
    const c = canvas.current
    if (!c) return
    const wheel = (e: WheelEvent) => {
      e.preventDefault()
      if (e.ctrlKey || e.metaKey || Math.abs(e.deltaY) > 40 && !e.deltaX) zoomAt(e.clientX, e.clientY, Math.min(4, Math.max(0.2, view.current.z * Math.pow(0.998, e.deltaY))))
      else { view.current.x += e.deltaX / view.current.z; view.current.y += e.deltaY / view.current.z; redraw() }
    }
    c.addEventListener('wheel', wheel, { passive: false })
    const kd = (e: KeyboardEvent) => { if (e.code === 'Space' && !(e.target as HTMLElement).closest('input,textarea,[contenteditable]')) { spaceDown.current = true } }
    const ku = (e: KeyboardEvent) => { if (e.code === 'Space') spaceDown.current = false }
    window.addEventListener('keydown', kd)
    window.addEventListener('keyup', ku)
    return () => { c.removeEventListener('wheel', wheel); window.removeEventListener('keydown', kd); window.removeEventListener('keyup', ku) }
  }) // eslint-disable-line react-hooks/exhaustive-deps

  const fit = () => { view.current.fitted = ''; redraw(); setSize(s => ({ ...s })) }
  return (
    <div ref={wrap} className={`level-view tool-${tool}`}>
      <canvas ref={canvas} style={{ width: size.w, height: size.h }} onPointerDown={onDown} onPointerMove={onMove} onPointerUp={onUp} onPointerCancel={onUp} onContextMenu={e => e.preventDefault()} />
      <div className="level-zoom">
        <button type="button" className="btn sm icon-only" aria-label="Zoom out" onClick={() => zoomAt(canvas.current!.getBoundingClientRect().left + size.w / 2, canvas.current!.getBoundingClientRect().top + size.h / 2, Math.max(0.2, view.current.z / 1.25))}>−</button>
        <button type="button" className="btn sm" onClick={fit} title="Show the whole level">{Math.round(view.current.z * 100)}%</button>
        <button type="button" className="btn sm icon-only" aria-label="Zoom in" onClick={() => zoomAt(canvas.current!.getBoundingClientRect().left + size.w / 2, canvas.current!.getBoundingClientRect().top + size.h / 2, Math.min(4, view.current.z * 1.25))}>+</button>
      </div>
    </div>
  )
}
