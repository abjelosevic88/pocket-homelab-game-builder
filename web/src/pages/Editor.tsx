// The editor: levels on a grid, the things in them, rules, sounds and the game's settings.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api, uploadBlob, V1, ApiError } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange } from '../lib/data'
import type { Asset, Game } from '../lib/types'
import { normalize, newId, SIZES, type GameDoc, type Inst, type Obj, type Scene } from '../engine/types'
import { play as playGame, snapshot, type Player } from '../engine/engine'
import { playSfx, sfxOf } from '../engine/sfx'
import { Field, Loading, MenuButton, Modal, Segmented, Switch, Tabs, useIsMobile, useToast, errMsg, type MenuItem } from '../components/ui'
import { Select } from '../components/Select'
import { Icon } from '../components/icons'
import { AskButton } from '../components/AskButton'
import { PickerDialog, ShareDialog } from '../components/dialogs'
import { LevelView, type Tool } from '../components/editor/LevelView'
import { InstanceEditor, ObjectEditor, SceneEditor, STARTERS, newObject } from '../components/editor/Inspector'
import { RulesEditor } from '../components/editor/Rules'
import { SoundMaker } from '../components/editor/SoundMaker'

type Tab = 'level' | 'rules' | 'sounds' | 'game'
type SaveState = 'saved' | 'saving' | 'dirty' | 'error' | 'conflict'

export default function Editor() {
  const { id } = useParams()
  const navigate = useNavigate()
  const { toast } = useToast()
  const { settings } = useApp()
  const mobile = useIsMobile()
  const [game, setGame] = useState<Game | null>(null)
  const [doc, setDocRaw] = useState<GameDoc | null>(null)
  const [assets, setAssets] = useState<Asset[]>([])
  const [tab, setTab] = useState<Tab>('level')
  const [sceneId, setSceneId] = useState('')
  const [brush, setBrush] = useState('')
  const [tool, setTool] = useState<Tool>('place')
  const [selected, setSelected] = useState<string | null>(null)
  const [save, setSave] = useState<SaveState>('saved')
  const [playing, setPlaying] = useState<string | false>(false)
  const [share, setShare] = useState(false)
  const [rulesScope, setRulesScope] = useState('all')
  const [sheet, setSheet] = useState(false)
  const [grid, setGrid] = useState(settings?.show_grid !== false)
  const hist = useRef<{ past: string[]; future: string[]; last: number }>({ past: [], future: [], last: 0 })
  const saves = useRef(0)
  const dirty = useRef(false)
  const timer = useRef<number | undefined>(undefined)
  const docRef = useRef<GameDoc | null>(null)
  docRef.current = doc

  const loadAssets = useCallback(() => { api.get<{ assets: Asset[] }>(`${V1}/assets`).then(r => setAssets(r.assets)).catch(() => {}) }, [])
  useEffect(() => {
    api.get<Game>(`${V1}/games/${id}`).then(g => {
      setGame(g)
      const d = normalize(g.doc || {})
      setDocRaw(d)
      saves.current = g.saves
      setSceneId(d.start || d.scenes[0]?.id || '')
      setBrush(d.objects.find(o => o.role === 'wall')?.id || d.objects[0]?.id || '')
    }).catch(e => { toast(errMsg(e), 'err'); navigate('/') })
    loadAssets()
  }, [id]) // eslint-disable-line react-hooks/exhaustive-deps

  const assetURL = useCallback((aid: number) => `${V1}/assets/${aid}/file`, [])
  const frames = useCallback((aid: number) => assets.find(a => a.id === aid)?.frames || 1, [assets])

  // ---- saving ----
  const doSave = useCallback(async () => {
    const d = docRef.current
    if (!d || !game) return
    window.clearTimeout(timer.current)
    setSave('saving')
    try {
      const r = await api.put<{ saves: number }>(`${V1}/games/${game.id}/doc`, { doc: d, saves: saves.current })
      saves.current = r.saves
      dirty.current = false
      setSave('saved')
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) { setSave('conflict'); toast('This game was saved somewhere else since you opened it. Open it again to see the newest.', 'err') }
      else { setSave('error'); toast(errMsg(e), 'err') }
    }
  }, [game]) // eslint-disable-line react-hooks/exhaustive-deps

  const setDoc = useCallback((d: GameDoc, merge = false) => {
    const h = hist.current
    const now = Date.now()
    if (docRef.current && (!merge || now - h.last > 600)) { h.past.push(JSON.stringify(docRef.current)); if (h.past.length > 80) h.past.shift(); h.future = [] }
    h.last = now
    setDocRaw(d)
    dirty.current = true
    setSave(s => s === 'conflict' ? s : 'dirty')
    window.clearTimeout(timer.current)
    if (settings?.auto_save !== false) timer.current = window.setTimeout(() => { void doSave() }, 1200)
  }, [doSave, settings?.auto_save])

  const undo = () => { const h = hist.current; const p = h.past.pop(); if (!p || !doc) return; h.future.push(JSON.stringify(doc)); setDocRaw(JSON.parse(p)); dirty.current = true; setSave('dirty'); window.clearTimeout(timer.current); timer.current = window.setTimeout(() => { void doSave() }, 1200) }
  const redo = () => { const h = hist.current; const f = h.future.pop(); if (!f || !doc) return; h.past.push(JSON.stringify(doc)); setDocRaw(JSON.parse(f)); dirty.current = true; setSave('dirty'); window.clearTimeout(timer.current); timer.current = window.setTimeout(() => { void doSave() }, 1200) }

  // leaving: save what is left, and draw a new cover
  useEffect(() => {
    const before = (e: BeforeUnloadEvent) => { if (dirty.current) { e.preventDefault() } }
    window.addEventListener('beforeunload', before)
    return () => window.removeEventListener('beforeunload', before)
  }, [])
  const leave = async (to: string) => {
    if (dirty.current) await doSave()
    if (doc && game) void snapshot(doc, assetURL, frames).then(b => b && uploadBlob(`${V1}/games/${game.id}/cover`, b, 'cover.png')).then(() => announceChange()).catch(() => {})
    navigate(to)
  }

  // keys
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).closest('input, textarea, [contenteditable="true"], .modal-backdrop') || playing) return
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'z') { e.preventDefault(); if (e.shiftKey) redo(); else undo(); return }
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 's') { e.preventDefault(); void doSave(); return }
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (tab !== 'level') return
      if (e.key === '1') setTool('place')
      else if (e.key === '2') setTool('move')
      else if (e.key === '3') setTool('erase')
      else if (e.key === '4') setTool('pan')
      else if (e.key.toLowerCase() === 'p') setPlaying(sceneId)
      else if ((e.key === 'Delete' || e.key === 'Backspace') && selected) { e.preventDefault(); removeItem(selected) }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  })

  const scene = doc?.scenes.find(s => s.id === sceneId) || doc?.scenes[0]
  const brushObj = doc?.objects.find(o => o.id === brush) || null
  const item = scene?.items.find(i => i.id === selected) || null

  const setScene = useCallback((s: Scene, merge = false) => { if (doc) setDoc({ ...doc, scenes: doc.scenes.map(x => x.id === s.id ? s : x) }, merge) }, [doc, setDoc])
  const setItems = useCallback((items: Inst[], merge = false) => { if (scene) setScene({ ...scene, items }, merge) }, [scene, setScene])
  const setObj = (o: Obj) => { if (doc) setDoc({ ...doc, objects: doc.objects.map(x => x.id === o.id ? o : x) }, true) }
  const removeItem = (iid: string) => { if (scene) { setItems(scene.items.filter(i => i.id !== iid)); setSelected(null) } }
  const removeObj = (oid: string) => {
    if (!doc) return
    const objects = doc.objects.filter(o => o.id !== oid)
    setDoc({ ...doc, objects, scenes: doc.scenes.map(s => ({ ...s, items: s.items.filter(i => i.o !== oid) })) })
    setBrush(objects[0]?.id || '')
    setSelected(null)
  }
  const addObj = (base: Omit<Obj, 'id'>) => {
    if (!doc) return
    const o = newObject(doc, base)
    setDoc({ ...doc, objects: [...doc.objects, o] })
    setBrush(o.id)
    setTool('place')
    setSelected(null)
  }
  const addScene = (copyOf?: Scene) => {
    if (!doc) return
    const g = copyOf?.grid || scene?.grid || 24
    const s: Scene = copyOf ? { ...JSON.parse(JSON.stringify(copyOf)), id: newId(), name: `${copyOf.name} (copy)`, items: copyOf.items.map(i => ({ ...i, id: newId() })) }
      : { id: newId(), name: `Level ${doc.scenes.length + 1}`, w: scene?.w || doc.width, h: scene?.h || doc.height, bg: scene?.bg || '#2b3a55', grid: g, items: [], rules: [] }
    const at = doc.scenes.findIndex(x => x.id === sceneId) + 1
    const scenes = [...doc.scenes]
    scenes.splice(at, 0, s)
    setDoc({ ...doc, scenes })
    setSceneId(s.id)
  }
  const moveScene = (d: number) => {
    if (!doc || !scene) return
    const i = doc.scenes.indexOf(scene)
    const scenes = [...doc.scenes]
    scenes.splice(i, 1)
    scenes.splice(Math.max(0, Math.min(scenes.length, i + d)), 0, scene)
    setDoc({ ...doc, scenes })
  }
  const dropScene = () => {
    if (!doc || !scene || doc.scenes.length < 2) return
    const scenes = doc.scenes.filter(s => s.id !== scene.id)
    setDoc({ ...doc, scenes, start: doc.start === scene.id ? scenes[0].id : doc.start })
    setSceneId(scenes[0].id)
  }
  const duplicateItem = () => { if (item && scene) { const n = { ...item, id: newId(), x: item.x + (scene.grid || 24) }; setItems([...scene.items, n]); setSelected(n.id) } }

  if (!doc || !game || !scene) return <div className="boot"><Loading /></div>

  const saveWords: Record<SaveState, string> = { saved: 'Saved', saving: 'Saving…', dirty: settings?.auto_save === false ? 'Not saved' : 'Saving soon', error: 'Not saved', conflict: 'Saved elsewhere' }
  const sceneMenu = (): MenuItem[] => [
    { label: 'New level', icon: 'plus', onSelect: () => addScene() },
    { label: 'Copy this level', icon: 'copy', onSelect: () => addScene(scene) },
    { label: 'Move earlier', icon: 'arrow-up', disabled: doc.scenes[0] === scene, onSelect: () => moveScene(-1) },
    { label: 'Move later', icon: 'arrow-down', disabled: doc.scenes[doc.scenes.length - 1] === scene, onSelect: () => moveScene(1) },
    { label: 'The game starts here', icon: 'flag', disabled: doc.start === scene.id, onSelect: () => setDoc({ ...doc, start: scene.id }) },
    'sep', { label: 'Delete this level', icon: 'trash', danger: true, disabled: doc.scenes.length < 2, onSelect: dropScene },
  ]
  const inspector = (
    <>
      {item && <InstanceEditor doc={doc} inst={item} onChange={i => setItems(scene.items.map(x => x.id === i.id ? i : x), true)} onRemove={() => removeItem(item.id)} onDuplicate={duplicateItem} />}
      {(item ? doc.objects.find(o => o.id === item.o) : brushObj) ? <ObjectEditor key={(item?.o || brush)} doc={doc} obj={(item ? doc.objects.find(o => o.id === item.o) : brushObj)!} gameId={game.id} assets={assets} onChange={setObj} onRemove={() => removeObj(item?.o || brush)} onAssets={loadAssets} />
        : <SceneEditor doc={doc} scene={scene} gameId={game.id} assets={assets} onChange={s => setScene(s, true)} onAssets={loadAssets} />}
    </>
  )
  const palette = (
    <div className="palette">
      {doc.objects.map(o => {
        const pic = o.look.type === 'image' ? assets.find(a => a.id === o.look.image) : undefined
        return (
          <button key={o.id} type="button" className={`pal-item ${brush === o.id ? 'on' : ''}`} onClick={() => { setBrush(o.id); setSelected(null); if (tool === 'erase' || tool === 'pan') setTool('place') }} title={o.name}>
            <span className="pal-look" style={o.look.type === 'box' || o.look.type === 'circle' || o.look.type === 'triangle' ? { background: o.look.color, borderRadius: o.look.type === 'circle' ? '50%' : o.look.type === 'triangle' ? 0 : 4, clipPath: o.look.type === 'triangle' ? 'polygon(50% 8%, 100% 92%, 0 92%)' : undefined } : undefined}>
              {o.look.type === 'emoji' ? o.look.emoji : pic ? <img src={pic.url} alt="" className="pixelated" style={pic.frames > 1 ? { objectFit: 'cover', objectPosition: 'left' } : undefined} /> : o.look.type === 'text' ? 'Aa' : ''}
            </span>
            <span className="pal-name ellipsis">{o.name}</span>
          </button>
        )
      })}
      <MenuButton label="Add a thing" className="pal-item add" width={240} items={STARTERS.map(s => ({ label: <span>{s.emoji} {s.name}</span>, onSelect: () => addObj(s.make()) }))}><span className="pal-look"><Icon name="plus" size={18} /></span><span className="pal-name">Add</span></MenuButton>
    </div>
  )

  return (
    <div className="ed">
      <header className="ed-head">
        <button type="button" className="btn ghost icon-only" aria-label="Back" title="Back" onClick={() => leave(`/games/${game.id}`)}><Icon name="chevron-left" size={18} /></button>
        <img src="/favicon.svg" alt="" width={22} height={22} className="hide-sm" />
        <input className="ed-title" value={doc.title} onChange={e => setDoc({ ...doc, title: e.target.value }, true)} aria-label="Name of the game" />
        <span className={`save-state ${save === 'error' || save === 'conflict' ? 'error' : ''}`}>{saveWords[save]}</span>
        <span className="spacer" />
        <button type="button" className="btn ghost icon-only" onClick={undo} disabled={!hist.current.past.length} aria-label="Undo" title="Undo (⌘Z)"><Icon name="undo" size={17} /></button>
        <button type="button" className="btn ghost icon-only hide-sm" onClick={redo} disabled={!hist.current.future.length} aria-label="Redo" title="Redo (⌘⇧Z)"><Icon name="rotate-cw" size={17} /></button>
        {settings?.auto_save === false && <button type="button" className="btn" onClick={() => doSave()} disabled={save === 'saved' || save === 'saving'}>Save</button>}
        <button type="button" className="btn hide-sm" onClick={async () => { await doSave(); setShare(true) }}><Icon name="share" size={15} />Share</button>
        <button type="button" className="btn primary" onClick={() => setPlaying(doc.start)} title="Play from the start (P plays this level)"><Icon name="play" size={15} />Play</button>
        <AskButton />
      </header>
      <div className="ed-tabs"><Tabs tabs={[{ id: 'level', label: mobile ? 'Level' : 'Levels', icon: 'layout' }, { id: 'rules', label: 'Rules', icon: 'list' }, { id: 'sounds', label: 'Sounds', icon: 'volume' }, { id: 'game', label: 'Game', icon: 'settings' }]} value={tab} onChange={t => { setTab(t); setSheet(false) }} /></div>

      {tab === 'level' && <div className="ed-level">
        <aside className="ed-left">
          <div className="ed-scenes">
            <Select ariaLabel="Level" value={scene.id} onChange={v => { setSceneId(v); setSelected(null) }} options={doc.scenes.map((s, i) => ({ value: s.id, label: `${i + 1}. ${s.name}${doc.start === s.id ? ' ★' : ''}` }))} />
            <MenuButton label="Levels" items={sceneMenu} className="btn icon-only"><Icon name="more" size={16} /></MenuButton>
          </div>
          {!mobile && <><h4 className="ed-h">Things</h4>{palette}</>}
        </aside>
        <main className="ed-center">
          <div className="ed-tools">
            <Segmented size="sm" value={tool} onChange={setTool} options={[
              { value: 'place', label: <><Icon name="brush" size={14} /><span className="hide-sm">Place</span></>, title: 'Place things on the grid (1)' },
              { value: 'move', label: <><Icon name="pointer" size={14} /><span className="hide-sm">Move</span></>, title: 'Pick and move things (2)' },
              { value: 'erase', label: <><Icon name="eraser" size={14} /><span className="hide-sm">Rub out</span></>, title: 'Rub things out (3)' },
              { value: 'pan', label: <><Icon name="hand" size={14} /><span className="hide-sm">Look around</span></>, title: 'Move around the level (4, or hold Space)' },
            ]} />
            <label className="check-row small hide-sm"><input type="checkbox" checked={grid} onChange={e => setGrid(e.target.checked)} /> Grid</label>
            <span className="spacer" />
            {mobile && <button type="button" className={`btn sm ${sheet ? 'on' : ''}`} onClick={() => setSheet(!sheet)}><Icon name="sliders" size={14} />{item ? 'This one' : brushObj ? brushObj.name : 'Level'}</button>}
            <button type="button" className="btn sm" onClick={() => setPlaying(scene.id)} title="Play this level (P)"><Icon name="play" size={14} /><span className="hide-sm">This level</span></button>
          </div>
          {mobile && palette}
          <LevelView doc={doc} scene={scene} brush={brushObj} tool={tool} selected={selected} grid={grid} assetURL={assetURL} frames={frames} onItems={setItems} onSelect={id => { setSelected(id); if (id && mobile) setSheet(true) }} />
        </main>
        {!mobile && <aside className="ed-right">
          <div className="ed-right-tabs">
            <button type="button" className={`chip ${!item && brushObj ? 'on' : ''}`} onClick={() => setSelected(null)} disabled={!brushObj}>{brushObj?.name || 'Thing'}</button>
            <button type="button" className={`chip ${!item && !brushObj ? 'on' : ''}`} onClick={() => { setSelected(null); setBrush('') }}>The level</button>
          </div>
          {inspector}
        </aside>}
        {mobile && sheet && <div className="ed-sheet"><div className="ed-sheet-head"><b className="grow">{item ? 'This one' : brushObj ? brushObj.name : 'The level'}</b>{brushObj && !item && <button type="button" className="btn ghost sm" onClick={() => setBrush('')}>The level</button>}<button type="button" className="btn ghost icon-only" aria-label="Close" onClick={() => setSheet(false)}><Icon name="x" size={16} /></button></div>{inspector}</div>}
      </div>}

      {tab === 'rules' && <div className="ed-page">
        <div className="row wrap mb">
          <span className="muted small">Rules for</span>
          <Select ariaLabel="Where the rules work" value={rulesScope} onChange={setRulesScope} options={[{ value: 'all', label: `Every level (${doc.rules.length})` }, ...doc.scenes.map(s => ({ value: s.id, label: `Only ${s.name} (${s.rules.length})` }))]} />
        </div>
        <RulesEditor doc={doc} scope={rulesScope === 'all' ? '' : rulesScope} onDoc={d => setDoc(d, true)} />
      </div>}

      {tab === 'sounds' && <SoundsTab doc={doc} gameId={game.id} assets={assets} onDoc={d => setDoc(d, true)} onAssets={loadAssets} />}
      {tab === 'game' && <GameTab doc={doc} game={game} onDoc={d => setDoc(d, true)} onGame={setGame} />}

      {playing !== false && <PlayLayer doc={doc} game={game} start={playing || doc.start} frames={frames} onClose={() => setPlaying(false)} />}
      {share && <ShareDialog game={game} onClose={() => setShare(false)} onChange={setGame} />}
    </div>
  )
}

function PlayLayer({ doc, game, start, frames, onClose }: { doc: GameDoc; game: Game; start: string; frames: (id: number) => number; onClose: () => void }) {
  const ref = useRef<HTMLDivElement>(null)
  const { settings } = useApp()
  useEffect(() => {
    if (!ref.current) return
    const p: Player = playGame(ref.current, doc, { assetURL: id => `${V1}/assets/${id}/file`, frames, startScene: start, skipIntro: start !== doc.start, sound: settings?.sound_on !== false,
      onEnd: r => { void api.post(`${V1}/games/${game.id}/plays`, r).catch(() => {}) } })
    p.focus()
    const esc = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', esc)
    return () => { window.removeEventListener('keydown', esc); p.destroy() }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  const sc = doc.scenes.find(s => s.id === start)
  return (
    <div className="play-layer" role="dialog" aria-label="Playing">
      <div className="play-layer-head"><b className="grow ellipsis">{doc.title}{start !== doc.start && sc ? ` — ${sc.name}` : ''}</b><button type="button" className="btn" onClick={onClose}><Icon name="stop" size={15} />Stop</button></div>
      <div ref={ref} className="play-layer-box" />
    </div>
  )
}

function SoundsTab({ doc, gameId, assets, onDoc, onAssets }: { doc: GameDoc; gameId: number; assets: Asset[]; onDoc: (d: GameDoc) => void; onAssets: () => void }) {
  const [open, setOpen] = useState<string | null>(null)
  const [pick, setPick] = useState<'sound' | 'music' | null>(null)
  const set = (sid: string, p: Partial<GameDoc['sounds'][number]>) => onDoc({ ...doc, sounds: doc.sounds.map(s => s.id === sid ? { ...s, ...p } : s) })
  const listen = (s: GameDoc['sounds'][number]) => { if (s.asset) { const a = new Audio(`${V1}/assets/${s.asset}/file`); void a.play() } else playSfx(sfxOf(s.sfx)) }
  const used = (sid: string) => [...doc.rules, ...doc.scenes.flatMap(s => s.rules)].some(r => r.do.some(a => (a.t === 'sound' || a.t === 'music') && a.s === sid)) || doc.scenes.some(s => s.music === sid)
  const add = () => { const s = { id: newId(), name: 'New sound', sfx: { preset: 'blip' } }; onDoc({ ...doc, sounds: [...doc.sounds, s] }); setOpen(s.id) }
  return (
    <div className="ed-page">
      <div className="row wrap mb">
        <p className="muted grow">Sounds play from rules (“play a sound”). Music plays while a level runs (choose it in the level). Sounds named <b>win</b> and <b>lose</b> play at the end.</p>
        <button type="button" className="btn" onClick={() => setPick('music')}><Icon name="music" size={15} />Add music</button>
        <button type="button" className="btn" onClick={() => setPick('sound')}><Icon name="upload" size={15} />Add a sound file</button>
        <button type="button" className="btn primary" onClick={add}><Icon name="wand" size={15} />Make a sound</button>
      </div>
      <div className="card flush"><ul className="list">
        {doc.sounds.length === 0 && <li className="list-row muted">No sounds yet.</li>}
        {doc.sounds.map(s => {
          const a = s.asset ? assets.find(x => x.id === s.asset) : undefined
          return (
            <li key={s.id}>
              <div className="list-row">
                <button type="button" className="btn ghost icon-only" aria-label="Listen" onClick={() => listen(s)}><Icon name="play" size={16} /></button>
                <input className="grow plain-input" value={s.name} onChange={e => set(s.id, { name: e.target.value })} aria-label="Name" />
                <span className="muted small nowrap">{s.asset ? (a?.kind === 'music' || s.loop ? 'Music' : 'A file') : 'Made here'} · id <code>{s.id}</code>{!used(s.id) && ' · not used yet'}</span>
                {!s.asset && <button type="button" className="btn sm" onClick={() => setOpen(open === s.id ? null : s.id)}><Icon name="sliders" size={14} />{open === s.id ? 'Done' : 'Change'}</button>}
                <button type="button" className="btn ghost icon-only sm" aria-label="Remove" onClick={() => onDoc({ ...doc, sounds: doc.sounds.filter(x => x.id !== s.id) })}><Icon name="trash" size={14} /></button>
              </div>
              {open === s.id && !s.asset && <div className="pad"><SoundMaker value={s.sfx} onChange={p => set(s.id, { sfx: p })} /></div>}
            </li>
          )
        })}
      </ul></div>
      {pick && <PickerDialog kind={pick} gameId={gameId} onClose={() => setPick(null)} onPicked={a => { onAssets(); onDoc({ ...doc, sounds: [...doc.sounds, { id: newId(), name: a.name, asset: a.id, loop: pick === 'music' || a.kind === 'music' || undefined }] }) }} />}
    </div>
  )
}

const HUD_SPOTS: Record<string, (d: GameDoc) => { x: number; y: number; align: 'left' | 'center' | 'right' }> = {
  'top-left': () => ({ x: 10, y: 10, align: 'left' }), 'top-center': d => ({ x: d.width / 2, y: 10, align: 'center' }), 'top-right': d => ({ x: d.width - 10, y: 10, align: 'right' }),
  'bottom-left': d => ({ x: 10, y: d.height - 28, align: 'left' }), 'bottom-right': d => ({ x: d.width - 10, y: d.height - 28, align: 'right' }),
}

function GameTab({ doc, game, onDoc, onGame }: { doc: GameDoc; game: Game; onDoc: (d: GameDoc) => void; onGame: (g: Game) => void }) {
  const { toast } = useToast()
  const [about, setAbout] = useState(game.description)
  const [confirmSize, setConfirmSize] = useState<string | null>(null)
  const sizeId = useMemo(() => Object.entries(SIZES).find(([, [w, h]]) => w === doc.width && h === doc.height)?.[0] || 'custom', [doc.width, doc.height])
  const saveAbout = async () => { if (about === game.description) return; try { onGame(await api.put<Game>(`${V1}/games/${game.id}`, { description: about })) } catch (e) { toast(errMsg(e), 'err') } }
  const resize = (id: string) => {
    const [w, h] = SIZES[id]
    onDoc({ ...doc, width: w, height: h, hud: doc.hud.map(x => x.align === 'right' ? { ...x, x: w - 10 } : x.align === 'center' ? { ...x, x: w / 2 } : x), scenes: doc.scenes.map(s => ({ ...s, w: Math.max(s.w, w), h: Math.max(s.h, h) })) })
  }
  const spotOf = (x: GameDoc['hud'][number]) => Object.entries(HUD_SPOTS).find(([, f]) => { const p = f(doc); return Math.abs(p.x - x.x) < 2 && Math.abs(p.y - x.y) < 2 })?.[0] || 'top-left'
  return (
    <div className="ed-page narrow">
      <section className="set-section"><div className="set-section-head"><div className="grow"><h3>The game</h3></div></div><div className="set-section-body stack">
        <Field label="Name"><input value={doc.title} onChange={e => onDoc({ ...doc, title: e.target.value })} /></Field>
        <Field label="What it is" help="Shown on the shared page."><textarea rows={2} value={about} onChange={e => setAbout(e.target.value)} onBlur={saveAbout} /></Field>
        <Field label="Screen"><Segmented size="sm" value={sizeId} onChange={v => v !== 'custom' && setConfirmSize(v)} options={[...Object.entries(SIZES).map(([id, [w, h, l]]) => ({ value: id, label: l.split(' (')[0], title: `${w} × ${h}` })), ...(sizeId === 'custom' ? [{ value: 'custom', label: `${doc.width}×${doc.height}` }] : [])]} /></Field>
        <div className="check-row"><Switch checked={doc.pixel} onChange={v => onDoc({ ...doc, pixel: v })} label="Pixels" /><span><b>Crisp pixels</b><span className="muted small block-line">For pixel-art games: the screen grows in whole steps and pixels stay sharp.</span></span></div>
        <Field label="Letters"><Segmented size="sm" value={doc.font} onChange={v => onDoc({ ...doc, font: v })} options={[{ value: 'round', label: 'Round' }, { value: 'plain', label: 'Plain' }, { value: 'pixel', label: 'Blocky' }]} /></Field>
        <Field label="Buttons on phones" help="The buttons drawn on a touch screen."><Select ariaLabel="Buttons" value={doc.controls} onChange={v => onDoc({ ...doc, controls: v as GameDoc['controls'] })} options={[{ value: 'auto', label: 'Chosen from the behaviours' }, { value: 'jump', label: 'Left, right and jump' }, { value: 'sides', label: 'Left, right and a button' }, { value: 'dpad', label: 'Four arrows and a button' }, { value: 'none', label: 'None (tapping only)' }]} /></Field>
      </div></section>
      <section className="set-section"><div className="set-section-head"><div className="grow"><h3>Start and end</h3></div></div><div className="set-section-body stack">
        <div className="check-row"><Switch checked={doc.intro.on} onChange={v => onDoc({ ...doc, intro: { ...doc.intro, on: v } })} label="Start screen" /><span><b>A start screen</b><span className="muted small block-line">The name, how to play, and a Play button. Phones need a tap before sounds can play.</span></span></div>
        {doc.intro.on && <>
          <Field label="How to play"><textarea rows={2} value={doc.intro.text} onChange={e => onDoc({ ...doc, intro: { ...doc.intro, text: e.target.value } })} /></Field>
          <Field label="Button"><input value={doc.intro.button} onChange={e => onDoc({ ...doc, intro: { ...doc.intro, button: e.target.value } })} /></Field>
        </>}
        <div className="cols2 even">
          <Field label="When you win"><input value={doc.ends.won} onChange={e => onDoc({ ...doc, ends: { ...doc.ends, won: e.target.value } })} /></Field>
          <Field label="When you lose"><input value={doc.ends.lost} onChange={e => onDoc({ ...doc, ends: { ...doc.ends, lost: e.target.value } })} /></Field>
        </div>
      </div></section>
      <section className="set-section"><div className="set-section-head"><div className="grow"><h3>Numbers</h3><p>Score, lives, keys… Rules add to them and check them.</p></div></div><div className="set-section-body stack">
        {doc.vars.map((v, i) => (
          <div key={i} className="row">
            <input className="grow" value={v.name} onChange={e => onDoc({ ...doc, vars: doc.vars.map((x, j) => j === i ? { ...x, name: e.target.value.replace(/\s+/g, '_') } : x) })} aria-label="Name" />
            <span className="muted small">starts at</span>
            <input type="number" style={{ width: 90 }} value={v.value} onChange={e => onDoc({ ...doc, vars: doc.vars.map((x, j) => j === i ? { ...x, value: Number(e.target.value) || 0 } : x) })} aria-label="Starts at" />
            <button type="button" className="btn ghost icon-only sm" aria-label="Remove" onClick={() => onDoc({ ...doc, vars: doc.vars.filter((_, j) => j !== i) })}><Icon name="x" size={14} /></button>
          </div>
        ))}
        <div className="row"><button type="button" className="btn sm" onClick={() => onDoc({ ...doc, vars: [...doc.vars, { name: `number${doc.vars.length + 1}`, value: 0 }] })}><Icon name="plus" size={14} />Add a number</button></div>
        <Field label="The score is" help="What goes on the best-scores table.">
          <div className="row wrap">
            <Select ariaLabel="Score" value={doc.score.kind === 'time' ? '' : doc.score.var} onChange={v => onDoc({ ...doc, score: { ...doc.score, var: v, kind: v ? (doc.score.kind === 'time' ? 'high' : doc.score.kind) : 'time' } })} options={[...doc.vars.map(v => ({ value: v.name, label: v.name })), { value: '', label: 'The time it took' }]} />
            {doc.score.kind !== 'time' && <Segmented size="sm" value={doc.score.kind} onChange={v => onDoc({ ...doc, score: { ...doc.score, kind: v } })} options={[{ value: 'high', label: 'More is better' }, { value: 'low', label: 'Less is better' }]} />}
            <input value={doc.score.label} onChange={e => onDoc({ ...doc, score: { ...doc.score, label: e.target.value } })} placeholder="Score" style={{ maxWidth: 140 }} aria-label="Called" />
          </div>
        </Field>
      </div></section>
      <section className="set-section"><div className="set-section-head"><div className="grow"><h3>On the screen</h3><p>Lines of words over the game. <code>{'{score}'}</code> shows a number, <code>{'{time}'}</code> the seconds.</p></div></div><div className="set-section-body stack">
        {doc.hud.map((h, i) => (
          <div key={h.id} className="row wrap">
            <input className="grow" value={h.text} onChange={e => onDoc({ ...doc, hud: doc.hud.map(x => x.id === h.id ? { ...x, text: e.target.value } : x) })} aria-label="Words" />
            <Select ariaLabel="Where" value={spotOf(h)} onChange={v => onDoc({ ...doc, hud: doc.hud.map(x => x.id === h.id ? { ...x, ...HUD_SPOTS[v](doc) } : x) })} options={[{ value: 'top-left', label: 'Top left' }, { value: 'top-center', label: 'Top middle' }, { value: 'top-right', label: 'Top right' }, { value: 'bottom-left', label: 'Bottom left' }, { value: 'bottom-right', label: 'Bottom right' }]} />
            <input type="color" className="color-in" value={h.color} onChange={e => onDoc({ ...doc, hud: doc.hud.map(x => x.id === h.id ? { ...x, color: e.target.value } : x) })} aria-label="Colour" />
            <input type="number" style={{ width: 64 }} value={h.size} min={8} max={48} onChange={e => onDoc({ ...doc, hud: doc.hud.map(x => x.id === h.id ? { ...x, size: Number(e.target.value) || 16 } : x) })} aria-label="Size" />
            <button type="button" className="btn ghost icon-only sm" aria-label="Remove" onClick={() => onDoc({ ...doc, hud: doc.hud.filter((_, j) => j !== i) })}><Icon name="x" size={14} /></button>
          </div>
        ))}
        <div className="row"><button type="button" className="btn sm" onClick={() => onDoc({ ...doc, hud: [...doc.hud, { id: newId(), text: `Score {${doc.vars[0]?.name || 'score'}}`, size: 16, color: '#ffffff', ...HUD_SPOTS['top-center'](doc) }] })}><Icon name="plus" size={14} />Add a line</button></div>
      </div></section>
      {confirmSize && <Modal title="Change the screen?" onClose={() => setConfirmSize(null)} size="sm" footer={<><button type="button" className="btn" onClick={() => setConfirmSize(null)}>Cancel</button><button type="button" className="btn primary" onClick={() => { resize(confirmSize); setConfirmSize(null) }}>Change it</button></>}>
        <p>The screen becomes {SIZES[confirmSize][0]} × {SIZES[confirmSize][1]}. Levels stay where they are (smaller levels grow to fit); words on the right move with the edge. Play it after to check that everything still fits.</p>
      </Modal>}
    </div>
  )
}
