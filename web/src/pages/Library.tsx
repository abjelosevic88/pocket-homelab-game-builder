// Pictures and sounds: everything your games can use.
import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, uploadBlob, V1 } from '../lib/api'
import { announceChange, useOnChange } from '../lib/data'
import type { Asset } from '../lib/types'
import { fmtBytes, ago } from '../lib/format'
import { Confirm, Empty, Loading, Modal, Segmented, Field, Spinner, useToast, errMsg, MenuButton, type MenuItem } from '../components/ui'
import { Icon } from '../components/icons'
import { PickerDialog } from '../components/dialogs'
import { PixelEditor } from '../components/editor/PixelEditor'
import { SoundMaker } from '../components/editor/SoundMaker'
import { sfxOf, toWav } from '../engine/sfx'

const ORIGIN: Record<string, string> = { upload: 'Uploaded', drawn: 'Drawn here', photos: 'Pocket Photos', files: 'Pocket Files', draw: 'Pocket Draw', photolab: 'Photo Lab', music: 'Pocket Music', voice: 'Pocket Voice', link: 'A copy' }

export default function Library() {
  const [params, setParams] = useSearchParams()
  const { toast } = useToast()
  const [kind, setKind] = useState<'image' | 'sound'>(params.get('sound') ? 'sound' : 'image')
  const [assets, setAssets] = useState<Asset[] | null>(null)
  const [draw, setDraw] = useState<Asset | 'new' | null>(params.get('draw') ? 'new' : null)
  const [pick, setPick] = useState<'image' | 'sound' | 'music' | null>(null)
  const [sound, setSound] = useState(!!params.get('sound'))
  const [del, setDel] = useState<Asset | null>(null)
  const [rename, setRename] = useState<Asset | null>(null)
  const load = useCallback(() => { api.get<{ assets: Asset[] }>(`${V1}/assets`).then(r => setAssets(r.assets)).catch(() => setAssets([])) }, [])
  useEffect(() => { load() }, [load])
  useOnChange(load)
  const shown = (assets || []).filter(a => kind === 'image' ? a.kind === 'image' : a.kind !== 'image')
  const done = () => { load(); announceChange(); if (params.toString()) setParams({}) }
  const edit = async (a: Asset) => { try { setDraw(await api.get<Asset>(`${V1}/assets/${a.id}`)) } catch (e) { toast(errMsg(e), 'err') } }
  const listen = (a: Asset) => { const au = new Audio(a.url); void au.play() }
  const menu = (a: Asset): MenuItem[] => [
    ...(a.origin === 'drawn' ? [{ label: 'Draw on it', icon: 'brush', onSelect: () => edit(a) }] : []),
    { label: 'Rename', icon: 'edit', onSelect: () => setRename(a) },
    { label: 'Download', icon: 'download', onSelect: () => window.open(`${a.url}?download=1`) },
    'sep', { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setDel(a) },
  ]
  return (
    <div className="page">
      <div className="page-head">
        <div className="grow"><h1 className="page-title">Pictures & sounds</h1><p className="muted">Everything here can be used in any of your games.</p></div>
        <div className="row wrap">
          {kind === 'image' ? <>
            <button type="button" className="btn" onClick={() => setPick('image')}><Icon name="upload" size={15} />Add pictures</button>
            <button type="button" className="btn primary" onClick={() => setDraw('new')}><Icon name="brush" size={15} />Draw a sprite</button>
          </> : <>
            <button type="button" className="btn" onClick={() => setPick('music')}><Icon name="music" size={15} />Add music</button>
            <button type="button" className="btn" onClick={() => setPick('sound')}><Icon name="upload" size={15} />Add sounds</button>
            <button type="button" className="btn primary" onClick={() => setSound(true)}><Icon name="wand" size={15} />Make a sound</button>
          </>}
        </div>
      </div>
      <Segmented size="sm" value={kind} onChange={setKind} options={[{ value: 'image', label: 'Pictures' }, { value: 'sound', label: 'Sounds & music' }]} />
      {assets === null ? <Loading /> : shown.length === 0 ? <Empty title={kind === 'image' ? 'No pictures yet' : 'No sounds yet'} icon={kind === 'image' ? 'image' : 'volume'}>{kind === 'image' ? 'Draw a sprite, upload a picture, or bring one from Pocket Photos, Draw or Photo Lab. Games can also use emoji and shapes without any pictures.' : 'Make a sound effect, upload one, or bring music from Pocket Music and recordings from Pocket Voice. Games come with sound effects of their own.'}</Empty>
        : kind === 'image' ? (
          <div className="asset-grid">
            {shown.map(a => (
              <div key={a.id} className="asset-card">
                <button type="button" className="asset-pic plain" onClick={() => a.origin === 'drawn' ? edit(a) : setRename(a)} title={a.name}><img src={a.url} alt="" loading="lazy" className={a.width <= 128 ? 'pixelated' : ''} /></button>
                <div className="asset-meta"><span className="ellipsis grow">{a.name}</span><MenuButton label="More" align="right" items={() => menu(a)} className="btn ghost icon-only sm"><Icon name="more" size={15} /></MenuButton></div>
                <div className="muted small">{a.frames > 1 ? `${a.frames} frames · ` : ''}{a.width && a.height ? `${Math.round(a.width / a.frames)}×${a.height} · ` : ''}{ORIGIN[a.origin] || a.origin}</div>
              </div>
            ))}
          </div>
        ) : (
          <div className="card flush"><ul className="list">{shown.map(a => (
            <li key={a.id} className="list-row">
              <button type="button" className="btn ghost icon-only" aria-label="Listen" onClick={() => listen(a)}><Icon name="play" size={16} /></button>
              <div className="grow min0"><div className="ellipsis">{a.name}</div><div className="muted small">{a.kind === 'music' ? 'Music' : 'Sound'} · {fmtBytes(a.size)} · {ORIGIN[a.origin] || a.origin} · {ago(a.created_at)}</div></div>
              <MenuButton label="More" align="right" items={() => menu(a)}><Icon name="more" size={16} /></MenuButton>
            </li>
          ))}</ul></div>
        )}
      {draw && <PixelEditor asset={draw === 'new' ? undefined : draw} onClose={() => { setDraw(null); if (params.get('draw')) setParams({}) }} onSaved={() => { toast('Saved'); done() }} />}
      {pick && <PickerDialog kind={pick} onClose={() => setPick(null)} onPicked={() => { toast('Added'); done() }} />}
      {sound && <SoundDialog onClose={() => { setSound(false); if (params.get('sound')) setParams({}) }} onSaved={() => { toast('Sound kept'); setKind('sound'); done() }} />}
      {rename && <RenameDialog asset={rename} onClose={() => setRename(null)} onDone={done} />}
      {del && <Confirm title="Delete this?" message={<>“{del.name}” goes from your library. Games that use it show an empty box or stay quiet there.</>} confirmLabel="Delete" onCancel={() => setDel(null)} onConfirm={async () => { try { await api.del(`${V1}/assets/${del.id}`); setDel(null); done() } catch (e) { toast(errMsg(e), 'err') } }} />}
    </div>
  )
}

function SoundDialog({ onClose, onSaved }: { onClose: () => void; onSaved: () => void }) {
  const { toast } = useToast()
  const [params, setParams] = useState<Record<string, unknown>>({ preset: 'coin' })
  const [name, setName] = useState('Coin')
  const [busy, setBusy] = useState(false)
  const save = async () => {
    setBusy(true)
    try {
      const fd = new FormData()
      fd.append('file', toWav(sfxOf(params)), `${name || 'Sound'}.wav`)
      fd.append('kind', 'sound')
      const r = await fetch(`${V1}/assets`, { method: 'POST', body: fd, credentials: 'same-origin' })
      if (!r.ok) throw new Error((await r.json().catch(() => ({}))).error || r.statusText)
      onSaved(); onClose()
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  void uploadBlob
  return (
    <Modal title="Make a sound effect" onClose={onClose} size="lg" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={save} disabled={busy}>{busy ? <Spinner /> : <Icon name="check" size={15} />}Keep it</button></>}>
      <div className="stack">
        <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} /></Field>
        <SoundMaker value={params} onChange={(p, preset) => { setParams(p); if (preset && (!name || name === 'Coin' || name === 'Sound')) setName(preset[0].toUpperCase() + preset.slice(1)) }} />
      </div>
    </Modal>
  )
}

function RenameDialog({ asset, onClose, onDone }: { asset: Asset; onClose: () => void; onDone: () => void }) {
  const { toast } = useToast()
  const [name, setName] = useState(asset.name)
  const [frames, setFrames] = useState(asset.frames)
  const save = async () => { try { await api.put(`${V1}/assets/${asset.id}`, { name, frames }); onDone(); onClose() } catch (e) { toast(errMsg(e), 'err') } }
  return (
    <Modal title="Picture" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" onClick={save}>Save</button></>}>
      <div className="stack">
        {asset.kind === 'image' && <img src={asset.url} alt="" className={`rename-pic ${asset.width <= 128 ? 'pixelated' : ''}`} />}
        <Field label="Name"><input value={name} onChange={e => setName(e.target.value)} /></Field>
        {asset.kind === 'image' && <Field label="Frames side by side" help="A strip of pictures in one file plays as an animation: say how many there are."><input type="number" min={1} max={64} value={frames} onChange={e => setFrames(Math.max(1, Number(e.target.value) || 1))} /></Field>}
      </div>
    </Modal>
  )
}
