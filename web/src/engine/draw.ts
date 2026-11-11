// Drawing things: shared by the player and the editor's level view.
import type { GameDoc, Look } from './types'

export const EMOJI_FONT = '"Apple Color Emoji","Segoe UI Emoji","Noto Color Emoji","Twemoji Mozilla",sans-serif'

export function fontFamily(f: GameDoc['font']) {
  if (f === 'pixel') return '"Press Start 2P","Courier New",ui-monospace,monospace'
  if (f === 'plain') return 'system-ui,-apple-system,"Segoe UI",Roboto,sans-serif'
  return 'ui-rounded,"SF Pro Rounded","Nunito","Varela Round",system-ui,-apple-system,"Segoe UI",sans-serif'
}

const images = new Map<string, HTMLImageElement>()

/** A picture, loaded once (draws nothing until it is there). */
export function image(url: string, onload?: () => void): HTMLImageElement {
  let im = images.get(url)
  if (!im) {
    im = new Image()
    im.decoding = 'async'
    im.crossOrigin = 'anonymous'
    im.src = url
    images.set(url, im)
  }
  if (onload && !im.complete) im.addEventListener('load', onload, { once: true })
  return im
}

export interface DrawOpts {
  text?: string
  color?: string
  flip?: boolean
  frame?: number   // animation frame (time-based); ignored for single pictures
  alpha?: number
  rot?: number     // degrees
  frames?: number  // frames in the picture strip
  font?: string
  imageURL?: (id: number) => string
}

function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number) {
  r = Math.max(0, Math.min(r, w / 2, h / 2))
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

/** Wraps text to lines that fit a width. */
export function wrap(ctx: CanvasRenderingContext2D, text: string, width: number): string[] {
  const out: string[] = []
  for (const para of text.split('\n')) {
    let line = ''
    for (const word of para.split(/\s+/)) {
      const t = line ? line + ' ' + word : word
      if (ctx.measureText(t).width > width && line) { out.push(line); line = word } else line = t
    }
    out.push(line)
  }
  return out
}

/** Draws one thing in its box. */
export function drawLook(ctx: CanvasRenderingContext2D, look: Look, x: number, y: number, w: number, h: number, o: DrawOpts = {}) {
  const color = o.color || look.color || '#888888'
  ctx.save()
  if (o.alpha !== undefined && o.alpha < 1) ctx.globalAlpha *= Math.max(0, o.alpha)
  const cx = x + w / 2, cy = y + h / 2
  if (o.rot) { ctx.translate(cx, cy); ctx.rotate(o.rot * Math.PI / 180); ctx.translate(-cx, -cy) }
  if (o.flip) { ctx.translate(cx, 0); ctx.scale(-1, 1); ctx.translate(-cx, 0) }
  switch (look.type) {
    case 'box': {
      ctx.fillStyle = color
      if (look.round) { roundRect(ctx, x, y, w, h, look.round); ctx.fill() } else ctx.fillRect(x, y, w, h)
      if (look.line) { ctx.fillStyle = look.line; ctx.fillRect(x, y, w, Math.min(4, h / 3)) }
      break
    }
    case 'circle': {
      ctx.fillStyle = color
      ctx.beginPath()
      ctx.ellipse(cx, cy, w / 2, h / 2, 0, 0, Math.PI * 2)
      ctx.fill()
      if (look.line) { ctx.strokeStyle = look.line; ctx.lineWidth = Math.max(1, Math.min(w, h) / 8); ctx.stroke() }
      break
    }
    case 'triangle': {
      ctx.fillStyle = color
      const teeth = h < w * 0.75 ? Math.max(1, Math.round(w / h / 1.2)) : 1
      ctx.beginPath()
      for (let i = 0; i < teeth; i++) {
        const tw = w / teeth
        ctx.moveTo(x + i * tw, y + h)
        ctx.lineTo(x + i * tw + tw / 2, y)
        ctx.lineTo(x + (i + 1) * tw, y + h)
      }
      ctx.closePath()
      ctx.fill()
      if (look.line) { ctx.strokeStyle = look.line; ctx.lineWidth = 1.5; ctx.stroke() }
      break
    }
    case 'emoji': {
      const size = Math.min(w, h) * 1.0
      ctx.font = `${size}px ${EMOJI_FONT}`
      ctx.textAlign = 'center'
      ctx.textBaseline = 'middle'
      ctx.fillStyle = '#000'
      ctx.fillText(look.emoji || '🙂', cx, cy + size * 0.06)
      break
    }
    case 'image': {
      if (look.image && o.imageURL) {
        const im = image(o.imageURL(look.image))
        if (im.complete && im.naturalWidth > 0) {
          const frames = Math.max(1, o.frames || 1)
          const fw = im.naturalWidth / frames
          const f = Math.floor(o.frame || 0) % frames
          ctx.imageSmoothingEnabled = fw > 64
          ctx.drawImage(im, f * fw, 0, fw, im.naturalHeight, x, y, w, h)
        } else {
          ctx.fillStyle = 'rgba(128,128,128,.35)'
          ctx.fillRect(x, y, w, h)
        }
      } else {
        ctx.strokeStyle = 'rgba(128,128,128,.8)'
        ctx.setLineDash([3, 3])
        ctx.strokeRect(x + 0.5, y + 0.5, w - 1, h - 1)
      }
      break
    }
    case 'text': break
    case 'none': break
  }
  const text = o.text ?? ''
  if (text && (look.type === 'text' || look.type === 'box' || look.type === 'circle')) {
    if (o.flip) { ctx.translate(cx, 0); ctx.scale(-1, 1); ctx.translate(-cx, 0) }
    const size = look.size || 16
    ctx.font = `${look.bold ? '700 ' : '600 '}${size}px ${o.font || fontFamily('round')}`
    ctx.fillStyle = look.type === 'text' ? color : (look.textColor || '#ffffff')
    ctx.textAlign = 'center'
    ctx.textBaseline = 'middle'
    const lines = wrap(ctx, text, w - 8)
    const lh = size * 1.2
    const top = cy - (lines.length - 1) * lh / 2
    lines.forEach((l, i) => ctx.fillText(l, cx, top + i * lh))
  }
  ctx.restore()
}
