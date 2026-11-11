// The shared page (/p/<key>, /e/<key>) and games saved as one HTML file.
import '../engine/player.css'
import './page.css'
import { play, type EndResult } from '../engine/engine'
import type { GameDoc } from '../engine/types'

interface Best { who: string; score: number }
interface Shared { title: string; description: string; doc: GameDoc; by: string; scores_on: boolean; remix_on: boolean; plays: number; key: string; best?: Best[]; low_is_best: boolean; frames?: Record<string, number> }
declare global { interface Window { __GAME__?: { doc: GameDoc; assets: Record<string, string>; frames?: Record<string, number>; title: string; by: string } } }

const NAME_KEY = 'pgb-player-name'
const h = (tag: string, cls = '', text?: string) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text !== undefined) e.textContent = text; return e }
const fmt = (v: number) => Number.isInteger(v) ? String(v) : v.toFixed(1)

function bestList(best: Best[], me?: { who: string; score: number }) {
  const ol = h('ol', 'gb-best')
  best.slice(0, 10).forEach((b, i) => {
    const li = h('li', me && b.who.toLowerCase() === me.who.toLowerCase() && b.score === me.score ? 'me' : '')
    li.append(h('span', '', `${i + 1}. ${b.who}`), h('span', '', fmt(b.score)))
    ol.append(li)
  })
  return ol
}

function mount(box: HTMLElement, doc: GameDoc, assetURL: (id: number) => string, frames: Record<string, number>, shared?: Shared, onBest?: (b: Best[]) => void) {
  let best = shared?.best || []
  play(box, doc, {
    assetURL, frames: id => frames[String(id)] || 1,
    onEnd: r => {
      if (!shared) return
      if (!shared.scores_on || r.result === 'quit') void send(shared.key, { ...r, who: '' })
    },
    endExtra: (r: EndResult) => {
      if (!shared?.scores_on || r.result === 'quit') return null
      const wrap = h('div')
      const form = h('form', 'gb-form') as HTMLFormElement
      const input = h('input') as HTMLInputElement
      input.placeholder = 'Your name'
      input.maxLength = 24
      input.value = localStorage.getItem(NAME_KEY) || ''
      const btn = h('button', '', 'Save my score') as HTMLButtonElement
      btn.type = 'submit'
      form.append(input, btn)
      const list = h('div')
      if (best.length) list.append(bestList(best))
      form.onsubmit = async e => {
        e.preventDefault()
        const who = input.value.trim()
        if (!who) { input.focus(); return }
        localStorage.setItem(NAME_KEY, who)
        btn.disabled = true
        const res = await send(shared.key, { ...r, who })
        form.replaceWith(h('p', 'gb-note', res?.rank ? `You are number ${res.rank}!` : 'Saved.'))
        const fresh = await load(shared.key).catch(() => null)
        if (fresh?.best) { best = fresh.best; onBest?.(best); list.innerHTML = ''; list.append(bestList(best, { who, score: r.score })) }
      }
      wrap.append(form, list)
      setTimeout(() => input.focus(), 50)
      return wrap
    },
  })
}

async function load(key: string): Promise<Shared> {
  const r = await fetch(`/api/v1/public/${encodeURIComponent(key)}`, { cache: 'no-store' })
  if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || 'This game is not shared any more.')
  return r.json()
}

async function send(key: string, r: EndResult & { who: string }) {
  try {
    const res = await fetch(`/api/v1/public/${encodeURIComponent(key)}/plays`, { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ who: r.who, score: r.score, seconds: r.seconds, result: r.result, level: r.level }) })
    return res.ok ? res.json() as Promise<{ rank: number }> : null
  } catch { return null }
}

function page(shared: Shared, embed: boolean) {
  const root = document.getElementById('player')!
  root.className = embed ? 'pg embed' : 'pg'
  const box = h('div', 'pg-box')
  if (!embed) {
    const head = h('header', 'pg-head')
    head.append(h('h1', '', shared.title))
    head.append(h('p', 'pg-by', [shared.by && `Made by ${shared.by}`, shared.plays ? `${shared.plays} ${shared.plays === 1 ? 'play' : 'plays'}` : ''].filter(Boolean).join(' · ')))
    root.append(head, box)
    // the box is as tall as the game (plus room for the buttons on a touch screen)
    box.style.height = 'auto'
    box.style.flex = 'none'
    box.style.boxSizing = 'content-box'
    box.style.aspectRatio = `${shared.doc.width} / ${shared.doc.height}`
    box.style.maxHeight = 'calc(100dvh - 160px)'
    if (matchMedia('(pointer: coarse)').matches && shared.doc.controls !== 'none') box.style.paddingBottom = '130px'
    const foot = h('footer', 'pg-foot')
    if (shared.description) foot.append(h('p', 'pg-about', shared.description))
    const bestBox = h('section', 'pg-best')
    const drawBest = (b: Best[]) => {
      bestBox.innerHTML = ''
      if (!shared.scores_on || !b.length) return
      bestBox.append(h('h2', '', 'Best scores'), bestList(b))
    }
    drawBest(shared.best || [])
    foot.append(bestBox)
    const links = h('p', 'pg-links')
    const made = h('a', '', 'Made with Pocket Game Builder') as HTMLAnchorElement
    made.href = '/'
    links.append(made)
    if (shared.remix_on) {
      const copy = h('a', '', 'Make your own copy') as HTMLAnchorElement
      copy.href = `/?remix=${encodeURIComponent(shared.key)}`
      links.append(document.createTextNode(' · '), copy)
    }
    foot.append(links)
    root.append(foot)
    mount(box, shared.doc, id => `/api/v1/public/${encodeURIComponent(shared.key)}/assets/${id}`, shared.frames || {}, shared, drawBest)
  } else {
    root.append(box)
    mount(box, shared.doc, id => `/api/v1/public/${encodeURIComponent(shared.key)}/assets/${id}`, shared.frames || {}, shared)
  }
  document.title = shared.title
}

function start() {
  const root = document.getElementById('player')!
  const g = window.__GAME__
  if (g) {
    root.className = 'pg file'
    const box = h('div', 'pg-box')
    root.append(box)
    mount(box, g.doc, id => g.assets[String(id)] || '', g.frames || {})
    return
  }
  const m = location.pathname.match(/^\/(p|e)\/([^/]+)/)
  if (!m) { root.textContent = 'No game here.'; return }
  load(decodeURIComponent(m[2])).then(s => page(s, m[1] === 'e')).catch(e => {
    root.className = 'pg'
    const card = h('div', 'pg-gone')
    card.append(h('div', 'pg-emoji', '🎮'), h('h1', '', 'This game is not here'), h('p', '', (e as Error).message))
    root.append(card)
  })
}

start()
