// Left column: make a game, the sections, and the latest games.
import { useCallback, useEffect, useState } from 'react'
import { NavLink, useLocation, useNavigate } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { useOnChange } from '../lib/data'
import type { Home } from '../lib/types'
import { Icon } from './icons'

export function Sidebar({ onCreate, onNavigate }: { onCreate: () => void; onNavigate: () => void }) {
  const { status, settings } = useApp()
  const navigate = useNavigate()
  const loc = useLocation()
  const [home, setHome] = useState<Home | null>(null)
  const load = useCallback(() => { api.get<Home>(`${V1}/home`).then(setHome).catch(() => {}) }, [])
  useEffect(() => { load() }, [load, loc.pathname])
  useOnChange(load)

  const title = settings?.title || status?.title || 'Pocket Game Builder'
  const path = loc.pathname
  const q = new URLSearchParams(loc.search)
  const nav: { to: string; label: string; icon: string; active: boolean; count?: number | string }[] = [
    { to: '/', label: 'My games', icon: 'gamepad', active: path === '/' && !q.get('filter'), count: home?.games || undefined },
    { to: '/?filter=shared', label: 'Shared', icon: 'share', active: path === '/' && q.get('filter') === 'shared', count: home?.shared || undefined },
    { to: '/?filter=fav', label: 'Favourites', icon: 'star', active: path === '/' && q.get('filter') === 'fav', count: home?.favorites || undefined },
    { to: '/library', label: 'Pictures & sounds', icon: 'palette', active: path.startsWith('/library'), count: home?.assets || undefined },
    { to: '/stats', label: 'Plays', icon: 'chart', active: path.startsWith('/stats'), count: home?.plays_week ? `${home.plays_week} this week` : undefined },
  ]
  return (
    <aside className="sidebar" aria-label="Pocket Game Builder">
      <div className="sb-brand">
        <img src="/favicon.svg" alt="" className="sb-logo" />
        <span className="sb-title">{title !== 'Pocket Game Builder' ? title : <>Pocket <b>Game Builder</b></>}</span>
        {status?.version && <span className="sb-version" title={`Version ${status.version}`}>{/^v?\d+\.\d+/.test(status.version) ? 'v' + status.version.replace(/^v/, '').replace(/[-+].*$/, '') : status.version}</span>}
      </div>
      <button type="button" className="btn primary create-btn" onClick={onCreate} title="Make a game (N)">
        <Icon name="plus" size={16} strokeWidth={2.2} />Make a game
      </button>

      <nav className="sb-nav" aria-label="Sections">
        {nav.map(x => (
          <NavLink key={x.to} to={x.to} onClick={onNavigate} className={() => `sb-nav-item ${x.active ? 'active' : ''}`}>
            <Icon name={x.icon} size={16} />{x.label}
            {!!x.count && <span className="sb-count">{x.count}</span>}
          </NavLink>
        ))}
      </nav>

      {!!home?.recent.length && <section className="sb-sec">
        <h4 className="sb-h">Latest</h4>
        {home.recent.map(g => (
          <button key={g.id} type="button" className={`sb-row sb-list ${path.startsWith(`/games/${g.id}`) ? 'active' : ''}`} onClick={() => { onNavigate(); navigate(`/games/${g.id}`) }} title={g.title}>
            <Icon name={g.shared ? 'share' : 'gamepad'} size={14} className="sb-list-ico" />
            <span className="grow ellipsis">{g.title}</span>
          </button>
        ))}
      </section>}

      <div className="sb-foot">
        <NavLink to="/settings" className="sb-foot-link" onClick={onNavigate}><Icon name="settings" size={15} />Settings</NavLink>
        <button type="button" className="sb-foot-link" onClick={() => { onNavigate(); window.dispatchEvent(new CustomEvent('pgb:shortcuts')) }}><Icon name="keyboard" size={15} />Shortcuts</button>
      </div>
    </aside>
  )
}
