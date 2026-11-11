// The games: a shelf of covers; a new game; a copy someone shared.
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, uploadBlob, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import type { Asset, Game } from '../lib/types'
import { ago } from '../lib/format'
import { Empty, Loading, Segmented, useToast, errMsg } from '../components/ui'
import { Icon } from '../components/icons'
import { GameCover, TEMPLATE_NAME } from '../components/bits'
import { NewGameDialog } from '../components/dialogs'
import { openCreate } from '../components/Layout'
import { snapshot } from '../engine/engine'

type Filter = 'all' | 'shared' | 'fav' | 'archived'

/** Games without a picture get one, drawn from their first scene. */
async function makeCovers(games: Game[], done: (id: number) => void) {
  const need = games.filter(g => !g.has_cover).slice(0, 8)
  if (!need.length) return
  const assets = (await api.get<{ assets: Asset[] }>(`${V1}/assets?kind=image`).catch(() => ({ assets: [] as Asset[] }))).assets
  const frames = (id: number) => assets.find(a => a.id === id)?.frames || 1
  for (const g of need) {
    try {
      const full = await api.get<Game>(`${V1}/games/${g.id}`)
      if (!full.doc) continue
      const blob = await snapshot(full.doc, id => `${V1}/assets/${id}/file`, frames)
      if (blob) { await uploadBlob(`${V1}/games/${g.id}/cover`, blob, 'cover.png'); done(g.id) }
    } catch { /* a picture is a nice-to-have */ }
  }
}

export default function GamesPage() {
  const [params, setParams] = useSearchParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const filter = (params.get('filter') as Filter) || 'all'
  const [games, setGames] = useState<Game[] | null>(null)
  const [newIdea, setNewIdea] = useState(params.get('new') === 'idea')
  const covered = useRef(new Set<number>())
  const load = useCallback(() => {
    api.get<{ games: Game[] }>(`${V1}/games${filter !== 'all' ? `?filter=${filter}` : ''}`).then(r => {
      setGames(r.games)
      const fresh = r.games.filter(g => !g.has_cover && !covered.current.has(g.id))
      fresh.forEach(g => covered.current.add(g.id))
      if (fresh.length) void makeCovers(fresh, () => api.get<{ games: Game[] }>(`${V1}/games${filter !== 'all' ? `?filter=${filter}` : ''}`).then(x => setGames(x.games)).catch(() => {}))
    }).catch(e => { setGames([]); toast(errMsg(e), 'err') })
  }, [filter]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { setGames(null); load() }, [load])
  useOnChange(load)

  // a copy of someone's shared game (/?remix=KEY from the shared page)
  const remix = params.get('remix')
  useEffect(() => {
    if (!remix) return
    api.post<Game>(`${V1}/remix/${encodeURIComponent(remix)}`).then(g => { announceChange(); toast('Your own copy is ready'); navigate(`/games/${g.id}/edit`, { replace: true }) })
      .catch(e => { toast(errMsg(e), 'err'); setParams({}, { replace: true }) })
  }, [remix]) // eslint-disable-line react-hooks/exhaustive-deps

  const fav = async (g: Game) => { try { await api.put(`${V1}/games/${g.id}`, { favorite: !g.favorite }); load(); announceChange() } catch (e) { toast(errMsg(e), 'err') } }
  const title = { all: 'My games', shared: 'Shared games', fav: 'Favourites', archived: 'Put away' }[filter]
  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">{title}</h1><p className="muted">{filter === 'shared' ? 'Games anyone with the link can play.' : filter === 'archived' ? 'Games you put away. They come back with “Take out”.' : 'Make a game, play it, share it.'}</p></div>
        <div className="row">
          <button type="button" className="btn" onClick={() => setNewIdea(true)}><Icon name="sparkles" size={15} />From an idea</button>
          <button type="button" className="btn primary hide-sm" onClick={() => openCreate()}><Icon name="plus" size={15} />Make a game</button>
        </div>
      </div>
      <div className="scroll-x"><Segmented size="sm" value={filter} onChange={v => setParams(v === 'all' ? {} : { filter: v })} options={[{ value: 'all', label: 'All' }, { value: 'shared', label: 'Shared' }, { value: 'fav', label: 'Favourites' }, { value: 'archived', label: 'Put away' }]} /></div>
      {games === null ? <Loading /> : games.length === 0 ? (
        filter === 'all' ? <Empty title="No games yet" icon="gamepad">Start from a template — a jump-and-run, a maze, a space shooter — or tell Pocket AI your idea.<div className="row center mt"><button type="button" className="btn primary" onClick={() => openCreate()}><Icon name="plus" size={15} />Make a game</button></div></Empty>
          : <Empty title="Nothing here" icon="gamepad" />
      ) : (
        <div className="game-grid">
          {games.map(g => (
            <div key={g.id} className="game-card">
              <Link to={`/games/${g.id}`} className="game-card-link" aria-label={g.title}>
                <GameCover game={g} />
                <div className="game-card-body">
                  <div className="game-card-title ellipsis">{g.title}</div>
                  <div className="game-card-meta muted small">{TEMPLATE_NAME[g.template] || 'Game'} · {g.scenes} {g.scenes === 1 ? 'level' : 'levels'} · {ago(g.updated_at)}</div>
                </div>
              </Link>
              <div className="game-card-foot">
                {g.share_on ? <span className="pill st-open" title="Anyone with the link can play"><Icon name="share" size={12} />{g.plays ? `${g.plays} ${g.plays === 1 ? 'play' : 'plays'}` : 'Shared'}</span> : <span className="pill st-closed">Only you</span>}
                {g.todo > 0 && <span className="pill st-seen" title="Still to do"><Icon name="check-square" size={12} />{g.todo}</span>}
                <span className="spacer" />
                <button type="button" className={`btn ghost icon-only sm ${g.favorite ? 'fav-on' : ''}`} aria-label={g.favorite ? 'Not a favourite' : 'Favourite'} title={g.favorite ? 'Not a favourite' : 'Favourite'} onClick={() => fav(g)}><Icon name="star" size={15} /></button>
                <Link className="btn sm" to={`/games/${g.id}/edit`}><Icon name="edit" size={14} />Edit</Link>
              </div>
            </div>
          ))}
        </div>
      )}
      {newIdea && <NewGameDialog initial="idea" onClose={() => { setNewIdea(false); if (params.get('new')) setParams({}) }} onMade={g => navigate(`/games/${g.id}/edit`)} />}
    </div>
  )
}
