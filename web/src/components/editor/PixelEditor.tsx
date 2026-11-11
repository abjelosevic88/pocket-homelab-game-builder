// Draw a sprite pixel by pixel, with frames for an animation.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api, V1 } from '../../lib/api'
import type { Asset } from '../../lib/types'
import { Field, Modal, Segmented, Spinner, useToast, errMsg } from '../ui'
import { Icon } from '../icons'

export interface Pixels { w: number; h: number; palette: string[]; frames: number[][] }

const PALETTE = ['', '#000000', '#ffffff', '#7c7c7c', '#c2c3c7', '#5f574f', '#ab5236', '#ffccaa', '#ff004d', '#ff77a8', '#ffa300', '#ffec27', '#00e436', '#008751', '#29adff', '#1d2b53', '#83769c', '#7e2553']

type Tool = 'pen' | 'eraser' | 'fill' | 'picker'

function blank(w: number, h: number) { return new Array(w * h).fill(0) }

function toPNG(p: Pixels): string {
  const c = document.createElement('canvas')
  c.width = p.w * p.frames.length
  c.height = p.h
  const ctx = c.getContext('2d')!
  p.frames.forEach((f, fi) => f.forEach((ci, i) => {
    const col = p.palette[ci]
    if (!ci || !col) return
    ctx.fillStyle = col
    ctx.fillRect(fi * p.w + (i % p.w), Math.floor(i / p.w), 1, 1)
  }))
  return c.toDataURL('image/png')
}

export function PixelEditor({ asset, gameId, onClose, onSaved }: { asset?: Asset; gameId?: number; onClose: () => void; onSaved: (a: Asset) => void }) {
  const { toast } = useToast()
  const start = useMemo<Pixels>(() => {
    if (asset?.pixels) { try { const p = JSON.parse(asset.pixels) as Pixels; if (p.w && p.frames?.length) return { ...p, palette: p.palette?.length ? p.palette : PALETTE } } catch { /* a fresh one */ } }
    return { w: 16, h: 16, palette: [...PALETTE], frames: [blank(16, 16)] }
  }, [asset])
  const [pix, setPix] = useState<Pixels>(start)
  const [frame, setFrame] = useState(0)
  const [color, setColor] = useState(1)
  const [tool, setTool] = useState<Tool>('pen')
  const [mirror, setMirror] = useState(false)
  const [onion, setOnion] = useState(true)
  const [name, setName] = useState(asset?.name || 'My sprite')
  const [busy, setBusy] = useState(false)
  const [undo, setUndo] = useState<Pixels[]>([])
  const [redo, setRedo] = useState<Pixels[]>([])
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const prevRef = useRef<HTMLCanvasElement>(null)
  const drawing = useRef(false)
  const last = useRef<{ x: number; y: number } | null>(null)
  const cell = Math.max(6, Math.floor(Math.min(420, window.innerWidth - 60) / Math.max(pix.w, pix.h)))

  const commit = (next: Pixels) => { setUndo(u => [...u.slice(-40), pix]); setRedo([]); setPix(next) }
  const cur = pix.frames[frame] || pix.frames[0]

  // draw the big grid
  useEffect(() => {
    const c = canvasRef.current
    if (!c) return
    c.width = pix.w * cell
    c.height = pix.h * cell
    const ctx = c.getContext('2d')!
    for (let y = 0; y < pix.h; y++) for (let x = 0; x < pix.w; x++) { ctx.fillStyle = (x + y) % 2 ? '#d9dce3' : '#eceef2'; ctx.fillRect(x * cell, y * cell, cell, cell) }
    if (onion && frame > 0) {
      ctx.globalAlpha = 0.25
      pix.frames[frame - 1].forEach((ci, i) => { if (ci && pix.palette[ci]) { ctx.fillStyle = pix.palette[ci]; ctx.fillRect((i % pix.w) * cell, Math.floor(i / pix.w) * cell, cell, cell) } })
      ctx.globalAlpha = 1
    }
    cur.forEach((ci, i) => { if (ci && pix.palette[ci]) { ctx.fillStyle = pix.palette[ci]; ctx.fillRect((i % pix.w) * cell, Math.floor(i / pix.w) * cell, cell, cell) } })
    ctx.strokeStyle = 'rgba(0,0,0,.08)'
    ctx.lineWidth = 1
    for (let x = 0; x <= pix.w; x++) { ctx.beginPath(); ctx.moveTo(x * cell + 0.5, 0); ctx.lineTo(x * cell + 0.5, pix.h * cell); ctx.stroke() }
    for (let y = 0; y <= pix.h; y++) { ctx.beginPath(); ctx.moveTo(0, y * cell + 0.5); ctx.lineTo(pix.w * cell, y * cell + 0.5); ctx.stroke() }
    if (mirror) { ctx.strokeStyle = 'rgba(11,132,198,.6)'; ctx.beginPath(); ctx.moveTo(pix.w * cell / 2, 0); ctx.lineTo(pix.w * cell / 2, pix.h * cell); ctx.stroke() }
  }, [pix, frame, cell, onion, mirror, cur])

  // the little moving preview
  useEffect(() => {
    let f = 0
    const t = window.setInterval(() => {
      const c = prevRef.current
      if (!c) return
      c.width = pix.w; c.height = pix.h
      const ctx = c.getContext('2d')!
      ctx.clearRect(0, 0, pix.w, pix.h)
      const fr = pix.frames[f % pix.frames.length]
      fr.forEach((ci, i) => { if (ci && pix.palette[ci]) { ctx.fillStyle = pix.palette[ci]; ctx.fillRect(i % pix.w, Math.floor(i / pix.w), 1, 1) } })
      f++
    }, 1000 / 6)
    return () => window.clearInterval(t)
  }, [pix])

  const at = (e: React.PointerEvent) => {
    const r = canvasRef.current!.getBoundingClientRect()
    const x = Math.floor((e.clientX - r.left) / (r.width / pix.w)), y = Math.floor((e.clientY - r.top) / (r.height / pix.h))
    return x >= 0 && y >= 0 && x < pix.w && y < pix.h ? { x, y } : null
  }
  const paint = useCallback((p: { x: number; y: number }, base: Pixels, fresh: boolean): Pixels => {
    const f = [...base.frames[frame]]
    const i = p.y * base.w + p.x
    if (tool === 'picker') { setColor(f[i]); setTool('pen'); return base }
    if (tool === 'fill') {
      const from = f[i], to = color
      if (from === to) return base
      const stack = [i]
      while (stack.length) {
        const k = stack.pop()!
        if (f[k] !== from) continue
        f[k] = to
        const x = k % base.w, y = Math.floor(k / base.w)
        if (x > 0) stack.push(k - 1); if (x < base.w - 1) stack.push(k + 1); if (y > 0) stack.push(k - base.w); if (y < base.h - 1) stack.push(k + base.w)
      }
    } else {
      const v = tool === 'eraser' ? 0 : color
      f[i] = v
      if (mirror) f[p.y * base.w + (base.w - 1 - p.x)] = v
    }
    void fresh
    const frames = [...base.frames]
    frames[frame] = f
    return { ...base, frames }
  }, [frame, tool, color, mirror])
  const down = (e: React.PointerEvent) => {
    const p = at(e)
    if (!p) return
    drawing.current = true
    last.current = p;
    (e.target as HTMLElement).setPointerCapture(e.pointerId)
    commit(paint(p, pix, true))
  }
  const move = (e: React.PointerEvent) => {
    if (!drawing.current || tool === 'fill' || tool === 'picker') return
    const p = at(e)
    if (!p) return
    const from = last.current || p
    last.current = p
    // every cell on the way, so a quick stroke has no gaps
    const n = Math.max(Math.abs(p.x - from.x), Math.abs(p.y - from.y))
    const cells: { x: number; y: number }[] = []
    for (let i = 1; i <= n; i++) cells.push({ x: Math.round(from.x + (p.x - from.x) * i / n), y: Math.round(from.y + (p.y - from.y) * i / n) })
    if (!n) cells.push(p)
    setPix(cur => cells.reduce((acc, c) => paint(c, acc, false), cur))
  }
  const up = () => { drawing.current = false; last.current = null }

  const resize = (n: number) => {
    const frames = pix.frames.map(f => { const out = blank(n, n); for (let y = 0; y < Math.min(n, pix.h); y++) for (let x = 0; x < Math.min(n, pix.w); x++) out[y * n + x] = f[y * pix.w + x]; return out })
    commit({ ...pix, w: n, h: n, frames })
  }
  const addFrame = () => { const frames = [...pix.frames]; frames.splice(frame + 1, 0, [...cur]); commit({ ...pix, frames }); setFrame(frame + 1) }
  const dropFrame = () => { if (pix.frames.length < 2) return; const frames = pix.frames.filter((_, i) => i !== frame); commit({ ...pix, frames }); setFrame(Math.max(0, frame - 1)) }
  const flip = () => { const f = cur.map((_, i) => cur[Math.floor(i / pix.w) * pix.w + (pix.w - 1 - i % pix.w)]); const frames = [...pix.frames]; frames[frame] = f; commit({ ...pix, frames }) }
  const clear = () => { const frames = [...pix.frames]; frames[frame] = blank(pix.w, pix.h); commit({ ...pix, frames }) }
  const doUndo = () => { const p = undo[undo.length - 1]; if (!p) return; setRedo(r => [...r, pix]); setUndo(u => u.slice(0, -1)); setPix(p); setFrame(f => Math.min(f, p.frames.length - 1)) }
  const doRedo = () => { const p = redo[redo.length - 1]; if (!p) return; setUndo(u => [...u, pix]); setRedo(r => r.slice(0, -1)); setPix(p) }
  const setCustom = (hex: string) => {
    const i = pix.palette.indexOf(hex)
    if (i > 0) { setColor(i); return }
    setPix({ ...pix, palette: [...pix.palette, hex] })
    setColor(pix.palette.length)
  }
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z') { e.preventDefault(); if (e.shiftKey) doRedo(); else doUndo() }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  })

  const save = async () => {
    setBusy(true)
    try {
      const body = { name, png: toPNG(pix), pixels: pix, frames: pix.frames.length, game_id: gameId }
      const a = asset ? await api.put<Asset>(`${V1}/assets/${asset.id}/drawn`, body) : await api.post<Asset>(`${V1}/assets/drawn`, body)
      onSaved(a)
      onClose()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title={asset ? `Draw: ${asset.name}` : 'Draw a sprite'} onClose={onClose} size="xl" footer={<>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" onClick={save} disabled={busy}>{busy ? <Spinner /> : <Icon name="check" size={15} />}Save</button>
    </>}>
      <div className="pixel-ed">
        <div className="pixel-tools">
          <Segmented size="sm" value={tool} onChange={setTool} options={[
            { value: 'pen', label: <Icon name="pen" size={15} />, title: 'Draw' }, { value: 'eraser', label: <Icon name="eraser" size={15} />, title: 'Rub out' },
            { value: 'fill', label: <Icon name="bucket" size={15} />, title: 'Fill' }, { value: 'picker', label: <Icon name="droplet" size={15} />, title: 'Pick a colour from the drawing' }]} />
          <button type="button" className={`btn sm ${mirror ? 'on' : ''}`} aria-pressed={mirror} onClick={() => setMirror(!mirror)} title="Draw both halves at once"><Icon name="columns" size={14} />Mirror</button>
          <button type="button" className="btn sm" onClick={flip} title="Turn the frame around"><Icon name="shuffle" size={14} />Flip</button>
          <button type="button" className="btn sm" onClick={clear}><Icon name="trash" size={14} />Clear</button>
          <span className="spacer" />
          <button type="button" className="btn ghost icon-only sm" onClick={doUndo} disabled={!undo.length} aria-label="Undo" title="Undo (⌘Z)"><Icon name="undo" size={15} /></button>
          <button type="button" className="btn ghost icon-only sm" onClick={doRedo} disabled={!redo.length} aria-label="Redo" title="Redo (⌘⇧Z)"><Icon name="rotate-cw" size={15} /></button>
        </div>
        <div className="pixel-main">
          <canvas ref={canvasRef} className="pixel-canvas" style={{ width: pix.w * cell, height: pix.h * cell }} onPointerDown={down} onPointerMove={move} onPointerUp={up} onPointerCancel={up} />
          <div className="pixel-side">
            <div className="pixel-palette">
              {pix.palette.map((c, i) => <button key={i} type="button" className={`pal ${color === i ? 'on' : ''} ${!c ? 'clear' : ''}`} style={c ? { background: c } : undefined} onClick={() => { setColor(i); if (tool === 'eraser' || tool === 'picker') setTool('pen') }} aria-label={c || 'See-through'} title={c || 'See-through'} />)}
              <label className="pal custom" title="Another colour"><input type="color" onChange={e => setCustom(e.target.value)} /></label>
            </div>
            <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} maxLength={80} /></Field>
            <Field label="Size"><Segmented size="sm" value={pix.w} onChange={resize} options={[16, 24, 32, 48].map(n => ({ value: n, label: `${n}` }))} /></Field>
            <div className="pixel-preview"><canvas ref={prevRef} style={{ width: pix.w * 3, height: pix.h * 3 }} /><span className="muted small">{pix.frames.length > 1 ? `Moving: ${pix.frames.length} frames` : 'One frame'}</span></div>
          </div>
        </div>
        <div className="pixel-frames">
          {pix.frames.map((f, i) => <FrameThumb key={i} pix={pix} f={f} on={i === frame} onClick={() => setFrame(i)} n={i + 1} />)}
          <button type="button" className="btn sm" onClick={addFrame} title="A copy of this frame, to change a little"><Icon name="plus" size={14} />Frame</button>
          {pix.frames.length > 1 && <button type="button" className="btn ghost sm" onClick={dropFrame}><Icon name="minus" size={14} />Remove frame</button>}
          <label className="check-row small"><input type="checkbox" checked={onion} onChange={e => setOnion(e.target.checked)} /> See the frame before</label>
        </div>
      </div>
    </Modal>
  )
}

function FrameThumb({ pix, f, on, onClick, n }: { pix: Pixels; f: number[]; on: boolean; onClick: () => void; n: number }) {
  const ref = useRef<HTMLCanvasElement>(null)
  useEffect(() => {
    const c = ref.current
    if (!c) return
    c.width = pix.w; c.height = pix.h
    const ctx = c.getContext('2d')!
    f.forEach((ci, i) => { if (ci && pix.palette[ci]) { ctx.fillStyle = pix.palette[ci]; ctx.fillRect(i % pix.w, Math.floor(i / pix.w), 1, 1) } })
  }, [pix, f])
  return <button type="button" className={`frame-thumb ${on ? 'on' : ''}`} onClick={onClick} title={`Frame ${n}`}><canvas ref={ref} /><span>{n}</span></button>
}
