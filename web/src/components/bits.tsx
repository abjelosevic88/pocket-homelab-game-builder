// Small pieces used on many pages.
import React from 'react'
import { Icon } from './icons'
import { useAppState, openAction } from '../lib/data'
import { useToast, CoreHint } from './ui'
import { api } from '../lib/api'
import { useState } from 'react'
import type { Game } from '../lib/types'

export const TEMPLATE_EMOJI: Record<string, string> = { platformer: '🐱', maze: '💎', shooter: '🚀', bricks: '🧱', catch: '🍎', quiz: '❓', blank: '✨', import: '📦' }
export const TEMPLATE_NAME: Record<string, string> = { platformer: 'Jump and run', maze: 'Maze', shooter: 'Shooter', bricks: 'Brick breaker', catch: 'Catch', quiz: 'Quiz', blank: 'Game', import: 'Brought in' }

/** A game's picture (or its kind's emoji until it has one). */
export function GameCover({ game, className = '' }: { game: Pick<Game, 'cover_url' | 'template' | 'title'>; className?: string }) {
  const [bad, setBad] = useState(false)
  return (
    <span className={`game-cover ${className}`}>
      {game.cover_url && !bad ? <img src={game.cover_url} alt="" loading="lazy" onError={() => setBad(true)} /> : <span className="game-cover-emoji" aria-hidden>{TEMPLATE_EMOJI[game.template] || '🎮'}</span>}
    </span>
  )
}

export const fmtScore = (v: number) => Number.isInteger(v) ? v.toLocaleString() : v.toFixed(1)

/** A button that sends something to another Pocket app — greyed with the reason when that app is not connected. */
export function AppButton({ slug, scope, label, icon, path, body, onDone, className = 'btn sm', confirm }: {
  slug: string; scope?: string; label: string; icon: string; path: string; body?: unknown; onDone?: (r: { url?: string; text?: string }) => void; className?: string; confirm?: () => unknown
}) {
  const state = useAppState()
  const { ok, app, loading } = state(slug, scope)
  const { toast } = useToast()
  const [busy, setBusy] = useState(false)
  const run = async () => {
    setBusy(true)
    try {
      const b = confirm ? confirm() : body
      const r = await api.post<{ url?: string; text?: string }>(path, b ?? {})
      toast(r.text || 'Done', 'ok', openAction(r.url))
      onDone?.(r)
    } catch (e) { toast((e as Error).message, 'err') } finally { setBusy(false) }
  }
  const why = !ok && !loading ? (app ? `${app.name}: ${app.detail || app.fix || 'not connected'}` : 'Needs Pocket Core or a direct connection (Settings → Connected apps)') : label
  return <button type="button" className={className} disabled={!ok || busy} title={why} onClick={run}><Icon name={icon} size={14} />{label}</button>
}

export function AppHint({ slug, scope, children }: { slug: string; scope?: string; children?: React.ReactNode }) {
  const state = useAppState()
  const { ok, app, loading } = state(slug, scope)
  if (ok || loading) return null
  return <div className="app-hint muted small"><Icon name="link" size={13} />{children || <>{app?.name || 'That app'} is not connected: {app?.fix || 'see Settings → Connected apps'}.</>}{!app && <CoreHint />}</div>
}

export function InfoGrid({ items }: { items: [React.ReactNode, React.ReactNode][] }) {
  const shown = items.filter(([, v]) => v !== '' && v !== null && v !== undefined && v !== false)
  if (!shown.length) return null
  return <dl className="info-grid">{shown.map(([k, v], i) => <React.Fragment key={i}><dt>{k}</dt><dd>{v}</dd></React.Fragment>)}</dl>
}
