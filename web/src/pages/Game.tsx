// One game: play it, share it, its best scores, plays, to-dos and saved copies.
import { useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, openAction, useAppState } from '../lib/data'
import type { Asset, Game, Play, Todo, Version } from '../lib/types'
import { ago, fmtDateTime } from '../lib/format'
import { Card, Confirm, Empty, Field, Loading, MenuButton, Modal, Spinner, useToast, errMsg, type MenuItem } from '../components/ui'
import { Icon } from '../components/icons'
import { GameCover, TEMPLATE_NAME, fmtScore } from '../components/bits'
import { ShareDialog, TranslateDialog } from '../components/dialogs'
import { Bars } from '../components/Chart'
import { useCrumb } from '../components/Layout'
import { play as playGame, type Player } from '../engine/engine'

export function useFrames() {
  const [assets, setAssets] = useState<Asset[]>([])
  useEffect(() => { api.get<{ assets: Asset[] }>(`${V1}/assets?kind=image`).then(r => setAssets(r.assets)).catch(() => {}) }, [])
  return useCallback((id: number) => assets.find(a => a.id === id)?.frames || 1, [assets])
}

/** The game, playable in a box on the page; plays are written down as yours. */
export function PlayBox({ game, startScene, onEnd, className = '' }: { game: Game; startScene?: string; onEnd?: () => void; className?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const { settings } = useApp()
  const frames = useFrames()
  const framesRef = useRef(frames)
  framesRef.current = frames
  useEffect(() => {
    if (!ref.current || !game.doc) return
    let p: Player | null = playGame(ref.current, game.doc, {
      assetURL: id => `${V1}/assets/${id}/file`, frames: id => framesRef.current(id), sound: settings?.sound_on !== false, startScene,
      onEnd: r => { void api.post(`${V1}/games/${game.id}/plays`, r).then(() => onEnd?.()).catch(() => {}) },
    })
    return () => { p?.destroy(); p = null }
  }, [game.id, game.saves, startScene]) // eslint-disable-line react-hooks/exhaustive-deps
  const d = game.doc
  return <div ref={ref} className={`play-box ${className}`} style={{ aspectRatio: d ? `${d.width} / ${d.height}` : undefined }} />
}

export default function GamePage() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const state = useAppState()
  const [game, setGame] = useState<Game | null>(null)
  const [plays, setPlays] = useState<{ plays: Play[]; best: Play[]; days: { day: string; plays: number }[]; low_is_best: boolean } | null>(null)
  const [todos, setTodos] = useState<Todo[]>([])
  const [versions, setVersions] = useState<Version[]>([])
  const [share, setShare] = useState(false)
  const [translate, setTranslate] = useState(false)
  const [del, setDel] = useState(false)
  const [issue, setIssue] = useState(false)
  const [newTodo, setNewTodo] = useState('')
  const [ideas, setIdeas] = useState<{ title: string; how: string }[] | null>(null)
  const [thinking, setThinking] = useState(false)
  useCrumb(game?.title)

  const load = useCallback(() => {
    api.get<Game>(`${V1}/games/${id}`).then(setGame).catch(e => { toast(errMsg(e), 'err'); navigate('/') })
    api.get<typeof plays>(`${V1}/games/${id}/plays?limit=30`).then(setPlays).catch(() => {})
    api.get<{ todos: Todo[] }>(`${V1}/games/${id}/todos`).then(r => setTodos(r.todos)).catch(() => {})
    api.get<{ versions: Version[] }>(`${V1}/games/${id}/versions`).then(r => setVersions(r.versions)).catch(() => {})
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps
  useEffect(() => { load() }, [load])

  if (!game) return <Loading />
  const update = async (p: Partial<Game>, msg?: string) => { try { setGame(await api.put<Game>(`${V1}/games/${game.id}`, p)); announceChange(); if (msg) toast(msg) } catch (e) { toast(errMsg(e), 'err') } }
  const copy = async () => { try { const g = await api.post<Game>(`${V1}/games/${game.id}/copy`); announceChange(); toast('Copy made', 'ok', { label: 'Open', run: () => navigate(`/games/${g.id}`) }) } catch (e) { toast(errMsg(e), 'err') } }
  const addTodo = async () => {
    if (!newTodo.trim()) return
    try { const t = await api.post<Todo>(`${V1}/games/${game.id}/todos`, { text: newTodo }); setTodos([...todos, t]); setNewTodo(''); announceChange() } catch (e) { toast(errMsg(e), 'err') }
  }
  const tick = async (t: Todo) => { try { const n = await api.put<Todo>(`${V1}/todos/${t.id}`, { done: !t.done }); setTodos(todos.map(x => x.id === t.id ? n : x)); announceChange() } catch (e) { toast(errMsg(e), 'err') } }
  const toTasks = async (t: Todo) => { try { const n = await api.post<Todo>(`${V1}/todos/${t.id}/task`); setTodos(todos.map(x => x.id === t.id ? n : x)); toast('In Pocket Tasks', 'ok', openAction(n.task_url)) } catch (e) { toast(errMsg(e), 'err') } }
  const dropTodo = async (t: Todo) => { try { await api.del(`${V1}/todos/${t.id}`); setTodos(todos.filter(x => x.id !== t.id)); announceChange() } catch (e) { toast(errMsg(e), 'err') } }
  const keep = async () => { try { await api.post(`${V1}/games/${game.id}/versions`, { note: 'Saved by hand' }); load(); toast('A copy of this moment is kept') } catch (e) { toast(errMsg(e), 'err') } }
  const restore = async (v: Version) => { try { await api.post(`${V1}/versions/${v.id}/restore`); load(); toast('Back to ' + fmtDateTime(v.created_at)) } catch (e) { toast(errMsg(e), 'err') } }
  const askIdeas = async () => { setThinking(true); try { setIdeas((await api.post<{ ideas: { title: string; how: string }[] }>(`${V1}/games/${game.id}/ideas`)).ideas || []) } catch (e) { toast(errMsg(e), 'err') } finally { setThinking(false) } }

  const more = (): MenuItem[] => [
    { label: 'Make a copy', icon: 'copy', onSelect: copy },
    { label: 'A copy in another language', icon: 'globe', onSelect: () => setTranslate(true), disabled: !state('translate', 'translate:run').ok },
    { label: 'Make an issue in Pocket Projects', icon: 'layers', onSelect: () => setIssue(true), disabled: !state('projects', 'projects:write').ok },
    'sep',
    game.archived ? { label: 'Take out', icon: 'archive', onSelect: () => update({ archived: false }, 'Back with your games') } : { label: 'Put away', icon: 'archive', onSelect: () => update({ archived: true }, 'Put away') },
    { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setDel(true) },
  ]
  const best = plays?.best || []
  return (
    <div className="page">
      <div className="page-head">
        <GameCover game={game} className="sm" />
        <div className="grow min0">
          <h1 className="page-title ellipsis">{game.title}</h1>
          <p className="muted small">{TEMPLATE_NAME[game.template] || 'Game'} · {game.scenes} {game.scenes === 1 ? 'level' : 'levels'} · {game.objects} things · {game.rules} rules · changed {ago(game.updated_at)}</p>
        </div>
        <div className="row">
          <button type="button" className={`btn ghost icon-only ${game.favorite ? 'fav-on' : ''}`} aria-label="Favourite" title={game.favorite ? 'Not a favourite' : 'Favourite'} onClick={() => update({ favorite: !game.favorite })}><Icon name="star" size={17} /></button>
          <button type="button" className="btn" onClick={() => setShare(true)}><Icon name="share" size={15} />{game.share_on ? 'Shared' : 'Share'}</button>
          <Link className="btn primary" to={`/games/${game.id}/edit`}><Icon name="edit" size={15} />Edit</Link>
          <MenuButton label="More" align="right" items={more}><Icon name="more" size={18} /></MenuButton>
        </div>
      </div>
      <div className="cols2">
        <div className="stack">
          <Card flush className="play-card"><PlayBox game={game} onEnd={load} /></Card>
          {game.description && <p className="pre">{game.description}</p>}
          {game.share_on && <div className="share-strip"><Icon name="link" size={15} /><a href={game.share_url} target="_blank" rel="noopener" className="ellipsis grow">{game.share_url.replace(/^https?:\/\//, '')}</a>{game.changed_since_share && <button type="button" className="btn sm" onClick={() => setShare(true)}>Share the new version</button>}</div>}
          <Card title="Still to do" sub={todos.filter(t => !t.done).length ? `${todos.filter(t => !t.done).length} open` : undefined}>
            <ul className="todo-list">
              {todos.map(t => (
                <li key={t.id} className={t.done ? 'done' : ''}>
                  <input type="checkbox" checked={t.done} onChange={() => tick(t)} aria-label="Done" />
                  <span className="grow">{t.text}</span>
                  {t.task_url ? <a className="btn ghost sm" href={t.task_url} target="_blank" rel="noopener" title="Open in Pocket Tasks"><Icon name="check-square" size={14} /></a>
                    : <button type="button" className="btn ghost sm icon-only" disabled={!state('tasks', 'tasks:write').ok} title={state('tasks', 'tasks:write').ok ? 'Put it in Pocket Tasks' : 'Pocket Tasks is not connected'} onClick={() => toTasks(t)}><Icon name="send" size={14} /></button>}
                  <button type="button" className="btn ghost sm icon-only" aria-label="Remove" onClick={() => dropTodo(t)}><Icon name="x" size={14} /></button>
                </li>
              ))}
            </ul>
            <form className="row mt" onSubmit={e => { e.preventDefault(); void addTodo() }}><input className="grow" value={newTodo} onChange={e => setNewTodo(e.target.value)} placeholder="Add a boss, a third level, music…" /><button type="submit" className="btn" disabled={!newTodo.trim()}><Icon name="plus" size={14} />Add</button></form>
            <div className="row mt"><button type="button" className="btn sm" onClick={askIdeas} disabled={thinking}>{thinking ? <Spinner /> : <Icon name="sparkles" size={14} />}Ideas from Pocket AI</button></div>
            {ideas && <ul className="idea-list">{ideas.map((i, k) => <li key={k}><b>{i.title}</b><span className="muted small block-line">{i.how}</span><button type="button" className="btn ghost sm" onClick={() => { setNewTodo(i.title) }}><Icon name="plus" size={13} />To do</button></li>)}</ul>}
          </Card>
        </div>
        <div className="stack">
          <Card title="Best scores" sub={game.share_on ? (game.scores_on ? 'From the shared page' : 'The table is off for this game') : 'Share the game to get a table'}>
            {best.length === 0 ? <p className="muted small">Nobody yet.</p> : (
              <ol className="best-list">{best.map((b, i) => <li key={b.who + i}><span className="best-rank">{i + 1}</span><span className="grow ellipsis">{b.who}</span><b className="tnum">{fmtScore(b.score)}</b></li>)}</ol>
            )}
          </Card>
          <Card title="Plays" sub={`${game.plays} by others, ${game.my_plays} by you`}>
            {plays && <Bars height={110} items={plays.days.map(d => ({ label: d.day.slice(8), value: d.plays, hint: d.day }))} />}
            {plays && plays.plays.length > 0 && <ul className="play-list">{plays.plays.slice(0, 8).map(p => (
              <li key={p.id}><Icon name={p.result === 'won' ? 'trophy' : p.result === 'lost' ? 'x-circle' : 'minus-circle'} size={14} className={p.result === 'won' ? 'ok' : 'muted'} /><span className="grow ellipsis">{p.mine ? 'You' : p.who || 'Someone'}{p.level ? <span className="muted"> · {p.level}</span> : null}</span><span className="tnum">{fmtScore(p.score)}</span><span className="muted small nowrap">{ago(p.created_at)}</span></li>
            ))}</ul>}
          </Card>
          <Card title="Saved copies" sub="Kept when you share, and when you ask." actions={<button type="button" className="btn sm" onClick={keep}><Icon name="history" size={14} />Keep this moment</button>}>
            {versions.length === 0 ? <p className="muted small">None yet.</p> : <ul className="play-list">{versions.slice(0, 10).map(v => (
              <li key={v.id}><Icon name="history" size={14} className="muted" /><span className="grow">{v.note}</span><span className="muted small nowrap">{fmtDateTime(v.created_at)}</span><button type="button" className="btn ghost sm" onClick={() => restore(v)}>Go back</button></li>
            ))}</ul>}
          </Card>
        </div>
      </div>
      {share && <ShareDialog game={game} onClose={() => { setShare(false); load() }} onChange={setGame} />}
      {translate && <TranslateDialog game={game} onClose={() => setTranslate(false)} onMade={g => navigate(`/games/${g.id}`)} />}
      {issue && <IssueDialog game={game} onClose={() => setIssue(false)} />}
      {del && <Confirm title="Delete this game?" message={<>“{game.title}” goes, with its plays, best scores and saved copies. Its pictures and sounds stay in your library.</>} confirmLabel="Delete" onCancel={() => setDel(false)} onConfirm={async () => { try { await api.del(`${V1}/games/${game.id}`); announceChange(); navigate('/') } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}

function IssueDialog({ game, onClose }: { game: Game; onClose: () => void }) {
  const { toast } = useToast()
  const [title, setTitle] = useState('')
  const [text, setText] = useState('')
  const [bug, setBug] = useState(true)
  const [busy, setBusy] = useState(false)
  const go = async () => {
    setBusy(true)
    try { const r = await api.post<{ identifier: string; open_url: string }>(`${V1}/games/${game.id}/issue`, { title, text, bug }); toast(`Issue ${r.identifier || ''} made`.trim(), 'ok', openAction(r.open_url)); onClose() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="An issue in Pocket Projects" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!title.trim() || busy} onClick={go}>{busy ? <Spinner /> : <Icon name="layers" size={15} />}Make the issue</button></>}>
      <div className="stack">
        <Field label="What is it?"><input value={title} onChange={e => setTitle(e.target.value)} placeholder="The ghost gets stuck in the corner" autoFocus /></Field>
        <Field label="More (optional)"><textarea rows={3} value={text} onChange={e => setText(e.target.value)} /></Field>
        <label className="check-row"><input type="checkbox" checked={bug} onChange={e => setBug(e.target.checked)} /> It is a bug (else: something to build)</label>
      </div>
    </Modal>
  )
}

export { Empty }
