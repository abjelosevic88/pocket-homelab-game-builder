// What a thing is: how it looks, how it behaves; one placed thing; the level itself.
import { useState } from 'react'
import { newId, type Beh, type GameDoc, type Inst, type Look, type Obj, type Scene } from '../../engine/types'
import type { Asset } from '../../lib/types'
import { Field, MenuButton, Segmented, type MenuItem } from '../ui'
import { Select } from '../Select'
import { Icon } from '../icons'
import { PickerDialog } from '../dialogs'
import { PixelEditor } from './PixelEditor'

type Param = { k: string; label: string; type: 'number' | 'bool' | 'select' | 'object'; min?: number; max?: number; step?: number; def: unknown; options?: { value: string; label: string }[]; help?: string }
export const BEHAVIORS: { t: string; name: string; about: string; icon: string; params: Param[] }[] = [
  { t: 'platformer', name: 'Runs and jumps', about: 'Moves left and right with the arrows, jumps with Space or Up, falls, stands on walls.', icon: 'run', params: [
    { k: 'speed', label: 'Speed', type: 'number', min: 20, max: 600, step: 10, def: 150 }, { k: 'jump', label: 'Jump', type: 'number', min: 50, max: 1000, step: 10, def: 380 },
    { k: 'gravity', label: 'Gravity', type: 'number', min: 100, max: 3000, step: 50, def: 1000 }, { k: 'doubleJump', label: 'Jump again in the air', type: 'bool', def: false }] },
  { t: 'topdown', name: 'Walks around', about: 'Moves in four directions with the arrows (seen from above). Walls stop it.', icon: 'move', params: [{ k: 'speed', label: 'Speed', type: 'number', min: 20, max: 600, step: 10, def: 120 }] },
  { t: 'solid', name: 'Is a wall', about: 'Nothing that moves can go through it.', icon: 'square', params: [] },
  { t: 'oneway', name: 'Is a ledge', about: 'Can be jumped onto from below and stood on.', icon: 'minus', params: [] },
  { t: 'gravity', name: 'Falls', about: 'Falls down and lands on walls and ledges.', icon: 'arrow-down', params: [{ k: 'g', label: 'Gravity', type: 'number', min: 100, max: 3000, step: 50, def: 1000 }] },
  { t: 'patrol', name: 'Walks back and forth', about: 'Turns round at walls (and at edges, if you like).', icon: 'repeat', params: [
    { k: 'speed', label: 'Speed', type: 'number', min: 5, max: 400, step: 5, def: 50 }, { k: 'axis', label: 'Direction', type: 'select', def: 'x', options: [{ value: 'x', label: 'Left and right' }, { value: 'y', label: 'Up and down' }] },
    { k: 'edges', label: 'Turn round at edges', type: 'bool', def: true }] },
  { t: 'follow', name: 'Follows', about: 'Goes after the nearest one of something.', icon: 'navigation', params: [
    { k: 'target', label: 'Follows', type: 'object', def: '' }, { k: 'speed', label: 'Speed', type: 'number', min: 5, max: 400, step: 5, def: 50 },
    { k: 'range', label: 'Only when nearer than (0 = always)', type: 'number', min: 0, max: 2000, step: 10, def: 0 }, { k: 'walls', label: 'Walls stop it', type: 'bool', def: false }] },
  { t: 'move', name: 'Moves by itself', about: 'Keeps going at a speed: falling things, bullets, clouds.', icon: 'send', params: [
    { k: 'vx', label: 'Sideways (− left, + right)', type: 'number', min: -800, max: 800, step: 10, def: 0 }, { k: 'vy', label: 'Up and down (− up, + down)', type: 'number', min: -800, max: 800, step: 10, def: 100 },
    { k: 'spread', label: 'Some faster, some slower', type: 'number', min: 0, max: 300, step: 5, def: 0 }, { k: 'gone', label: 'Gone once it leaves the level', type: 'bool', def: true },
    { k: 'wrap', label: 'Comes back on the other side', type: 'bool', def: false }] },
  { t: 'bounce', name: 'Bounces', about: 'Flies and bounces off walls and the edges (a ball).', icon: 'circle', params: [
    { k: 'speed', label: 'Speed', type: 'number', min: 20, max: 800, step: 10, def: 220 }, { k: 'angle', label: 'Starts going (degrees, −90 = up)', type: 'number', min: -180, max: 180, step: 5, def: -60 },
    { k: 'floor', label: 'Bounces off the bottom too', type: 'bool', def: false }] },
  { t: 'paddle', name: 'Slides', about: 'Moves along one line with the arrows, or follows a finger or the mouse.', icon: 'columns', params: [
    { k: 'axis', label: 'Along', type: 'select', def: 'x', options: [{ value: 'x', label: 'Left and right' }, { value: 'y', label: 'Up and down' }] },
    { k: 'speed', label: 'Speed', type: 'number', min: 20, max: 800, step: 10, def: 260 }, { k: 'pointer', label: 'Follows a finger or the mouse', type: 'bool', def: true }] },
  { t: 'shooter', name: 'Shoots', about: 'Fires something with Space (or by itself).', icon: 'target', params: [
    { k: 'bullet', label: 'Shoots', type: 'object', def: '' }, { k: 'rate', label: 'Seconds between shots', type: 'number', min: 0.05, max: 5, step: 0.05, def: 0.3 },
    { k: 'speed', label: 'Speed', type: 'number', min: 50, max: 1200, step: 10, def: 360 },
    { k: 'dir', label: 'Which way', type: 'select', def: 'up', options: [{ value: 'up', label: 'Up' }, { value: 'down', label: 'Down' }, { value: 'facing', label: 'Where it looks' }] },
    { k: 'auto', label: 'Shoots by itself', type: 'bool', def: false }] },
  { t: 'stayin', name: 'Stays in the level', about: 'Cannot leave by the sides or the bottom.', icon: 'maximize', params: [] },
  { t: 'wrap', name: 'Goes round', about: 'Leaving one side brings it back on the other.', icon: 'refresh', params: [] },
  { t: 'drag', name: 'Can be dragged', about: 'A finger or the mouse can pick it up and move it.', icon: 'hand', params: [] },
  { t: 'camera', name: 'The camera follows it', about: 'The screen moves with it through a big level.', icon: 'video', params: [] },
  { t: 'bob', name: 'Floats', about: 'Moves gently up and down (just for looks).', icon: 'waves', params: [{ k: 'amount', label: 'How far', type: 'number', min: 1, max: 30, step: 1, def: 3 }, { k: 'speed', label: 'How fast', type: 'number', min: 0.5, max: 10, step: 0.5, def: 2 }] },
  { t: 'spin', name: 'Spins', about: 'Turns round and round (just for looks).', icon: 'rotate-cw', params: [{ k: 'speed', label: 'Degrees a second', type: 'number', min: -720, max: 720, step: 10, def: 90 }] },
  { t: 'fade', name: 'Disappears', about: 'Fades away and is gone after a moment (sparks, explosions).', icon: 'eye-off', params: [{ k: 'after', label: 'After (seconds)', type: 'number', min: 0.1, max: 10, step: 0.1, def: 0.5 }] },
]

export const ROLES = [
  { value: 'player', label: 'The player' }, { value: 'wall', label: 'A wall or the ground' }, { value: 'coin', label: 'Something to pick up' }, { value: 'enemy', label: 'Someone to avoid' },
  { value: 'hazard', label: 'Something that hurts' }, { value: 'goal', label: 'The way out' }, { value: 'bullet', label: 'A shot' }, { value: 'ball', label: 'A ball' },
  { value: 'paddle', label: 'A paddle' }, { value: 'button', label: 'A button' }, { value: 'text', label: 'Words' }, { value: 'deco', label: 'Just for looks' },
]

/** Ready-made things to add, each with a look and behaviours. */
export const STARTERS: { name: string; emoji: string; make: () => Omit<Obj, 'id'> }[] = [
  { name: 'Player who jumps', emoji: '🐱', make: () => ({ name: 'Player', role: 'player', w: 22, h: 22, z: 5, look: { type: 'emoji', emoji: '🐱' }, behaviors: [{ t: 'platformer', p: { speed: 150, jump: 380, gravity: 1000 } }, { t: 'camera' }, { t: 'stayin' }] }) },
  { name: 'Player who walks', emoji: '🧒', make: () => ({ name: 'Player', role: 'player', w: 20, h: 20, z: 5, look: { type: 'emoji', emoji: '🧒' }, behaviors: [{ t: 'topdown', p: { speed: 120 } }, { t: 'camera' }, { t: 'stayin' }] }) },
  { name: 'Wall', emoji: '🧱', make: () => ({ name: 'Wall', role: 'wall', w: 24, h: 24, z: 0, look: { type: 'box', color: '#6b7280' }, behaviors: [{ t: 'solid' }] }) },
  { name: 'Ledge', emoji: '➖', make: () => ({ name: 'Ledge', role: 'wall', w: 24, h: 8, z: 0, look: { type: 'box', color: '#a77b4f', round: 3 }, behaviors: [{ t: 'oneway' }] }) },
  { name: 'Coin', emoji: '🪙', make: () => ({ name: 'Coin', role: 'coin', w: 14, h: 14, z: 1, look: { type: 'circle', color: '#f5c518', line: '#c99a06' }, behaviors: [{ t: 'bob' }] }) },
  { name: 'Star', emoji: '⭐', make: () => ({ name: 'Star', role: 'coin', w: 18, h: 18, z: 1, look: { type: 'emoji', emoji: '⭐' }, behaviors: [{ t: 'spin', p: { speed: 60 } }] }) },
  { name: 'Enemy', emoji: '👾', make: () => ({ name: 'Enemy', role: 'enemy', w: 22, h: 22, z: 4, look: { type: 'emoji', emoji: '👾' }, behaviors: [{ t: 'patrol', p: { speed: 50, axis: 'x', edges: true } }, { t: 'gravity' }] }) },
  { name: 'Ghost', emoji: '👻', make: () => ({ name: 'Ghost', role: 'enemy', w: 20, h: 20, z: 4, look: { type: 'emoji', emoji: '👻' }, behaviors: [{ t: 'follow', p: { target: 'player', speed: 40, range: 150 } }] }) },
  { name: 'Spikes', emoji: '🔺', make: () => ({ name: 'Spikes', role: 'hazard', w: 24, h: 12, z: 1, look: { type: 'triangle', color: '#d9480f' }, behaviors: [] }) },
  { name: 'Goal', emoji: '🏁', make: () => ({ name: 'Goal', role: 'goal', w: 24, h: 24, z: 1, look: { type: 'emoji', emoji: '🏁' }, behaviors: [] }) },
  { name: 'Ball', emoji: '⚪', make: () => ({ name: 'Ball', role: 'ball', w: 10, h: 10, z: 5, look: { type: 'circle', color: '#ffd43b' }, behaviors: [{ t: 'bounce', p: { speed: 220, angle: -60 } }] }) },
  { name: 'Words', emoji: '🔤', make: () => ({ name: 'Words', role: 'text', w: 160, h: 30, z: 6, look: { type: 'text', color: '#ffffff', size: 16, bold: true }, behaviors: [], text: 'Hello!' }) },
  { name: 'Button', emoji: '🔘', make: () => ({ name: 'Button', role: 'button', w: 120, h: 34, z: 6, look: { type: 'box', color: '#4c6ef5', round: 10, size: 15, textColor: '#ffffff' }, behaviors: [], text: 'Press me' }) },
  { name: 'Tree', emoji: '🌳', make: () => ({ name: 'Tree', role: 'deco', w: 32, h: 32, z: -1, look: { type: 'emoji', emoji: '🌳' }, behaviors: [] }) },
  { name: 'Empty', emoji: '◻️', make: () => ({ name: 'Thing', role: 'deco', w: 24, h: 24, z: 0, look: { type: 'box', color: '#9ca3af' }, behaviors: [] }) },
]

const EMOJI = '🐱🐶🐸🐵🦊🐼🐧🐤🦄🐲🧒🧙🤖👽👾👻💀🐌🐝🦇🕷️🐟🦈🐙🪙💎⭐🔑🚪🏁🚩❤️🍎🍌🍒🍄🌳🌵🌸☁️🌙☀️🔥💧⚡💣💥🪨🧱📦🎁🏆🚀🛸⚽🏀🎈'

const SWATCHES = ['#ff6b6b', '#ffa94d', '#ffd43b', '#69db7c', '#38d9a9', '#4dabf7', '#748ffc', '#b197fc', '#f783ac', '#ffffff', '#adb5bd', '#495057', '#212529', '#7a5a3a']

function Num({ value, onChange, min, max, step = 1 }: { value: number; onChange: (v: number) => void; min?: number; max?: number; step?: number }) {
  return <input type="number" value={Number.isFinite(value) ? value : 0} min={min} max={max} step={step} onChange={e => { const n = Number(e.target.value); if (Number.isFinite(n)) onChange(n) }} />
}

export function ColorPick({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  return (
    <div className="swatches sm">
      {SWATCHES.map(c => <button key={c} type="button" className={`swatch ${value?.toLowerCase() === c ? 'on' : ''}`} style={{ background: c }} onClick={() => onChange(c)} aria-label={c} />)}
      <label className="swatch custom" title="Another colour"><input type="color" value={value || '#888888'} onChange={e => onChange(e.target.value)} /></label>
    </div>
  )
}

export function objectOptions(doc: GameDoc) { return doc.objects.map(o => ({ value: o.id, label: `${o.look.type === 'emoji' ? o.look.emoji + ' ' : ''}${o.name}` })) }

export function ObjectEditor({ doc, obj, gameId, assets, onChange, onRemove, onAssets }: {
  doc: GameDoc; obj: Obj; gameId: number; assets: Asset[]; onChange: (o: Obj) => void; onRemove: () => void; onAssets: () => void
}) {
  const [pick, setPick] = useState(false)
  const [draw, setDraw] = useState<Asset | 'new' | null>(null)
  const look = obj.look
  const setLook = (l: Partial<Look>) => onChange({ ...obj, look: { ...look, ...l } })
  const setBeh = (t: string, p: Record<string, unknown> | null) => {
    const has = obj.behaviors.find(b => b.t === t)
    let behaviors: Beh[]
    if (p === null) behaviors = obj.behaviors.filter(b => b.t !== t)
    else if (has) behaviors = obj.behaviors.map(b => b.t === t ? { ...b, p } : b)
    else behaviors = [...obj.behaviors, { t, p }]
    onChange({ ...obj, behaviors })
  }
  const pic = look.image ? assets.find(a => a.id === look.image) : undefined
  const addable: MenuItem[] = BEHAVIORS.filter(b => !obj.behaviors.some(x => x.t === b.t)).map(b => ({
    label: <span><b>{b.name}</b><span className="muted small block-line">{b.about}</span></span>, icon: b.icon,
    onSelect: () => setBeh(b.t, Object.fromEntries(b.params.map(p => [p.k, p.type === 'object' ? (doc.objects.find(o => o.role === (b.t === 'shooter' ? 'bullet' : 'player'))?.id || '') : p.def]))),
  }))
  const used = doc.scenes.reduce((n, s) => n + s.items.filter(i => i.o === obj.id).length, 0)
  return (
    <div className="insp">
      <div className="insp-sec">
        <Field label="Name"><input value={obj.name} onChange={e => onChange({ ...obj, name: e.target.value })} maxLength={40} /></Field>
        <Field label="What it is" help="Helps Pocket AI and the rules read well; it changes nothing by itself."><Select ariaLabel="What it is" value={obj.role} onChange={v => onChange({ ...obj, role: v })} options={ROLES} /></Field>
      </div>
      <div className="insp-sec">
        <h4>Looks</h4>
        <Segmented size="sm" value={look.type} onChange={v => setLook({ type: v })} options={[{ value: 'emoji', label: '🙂', title: 'An emoji' }, { value: 'box', label: '■', title: 'A box' }, { value: 'circle', label: '●', title: 'A circle' }, { value: 'triangle', label: '▲', title: 'Spikes' }, { value: 'image', label: <Icon name="image" size={14} />, title: 'A picture' }, { value: 'text', label: 'Aa', title: 'Words' }, { value: 'none', label: '∅', title: 'Not seen' }]} />
        {look.type === 'emoji' && <>
          <div className="row"><input className="emoji-in" value={look.emoji || ''} onChange={e => setLook({ emoji: [...e.target.value].slice(-2).join('') })} aria-label="Emoji" /><span className="muted small">Type or pick one</span></div>
          <div className="emoji-grid">{[...EMOJI.matchAll(/\p{Extended_Pictographic}️?/gu)].map(m => m[0]).map(em => <button key={em} type="button" className={look.emoji === em ? 'on' : ''} onClick={() => setLook({ emoji: em })}>{em}</button>)}</div>
        </>}
        {(look.type === 'box' || look.type === 'circle' || look.type === 'triangle' || look.type === 'text') && <Field label={look.type === 'text' ? 'Colour of the words' : 'Colour'}><ColorPick value={look.color || '#888888'} onChange={c => setLook({ color: c })} /></Field>}
        {(look.type === 'box' || look.type === 'circle') && <div className="row wrap">
          <label className="check-row small"><input type="checkbox" checked={!!look.line} onChange={e => setLook({ line: e.target.checked ? '#ffffff' : undefined })} /> {look.type === 'box' ? 'A top edge' : 'A ring'}</label>
          {look.line && <input type="color" className="color-in" value={look.line} onChange={e => setLook({ line: e.target.value })} />}
          {look.type === 'box' && <label className="row small">Round corners <input type="range" min={0} max={20} value={look.round || 0} onChange={e => setLook({ round: Number(e.target.value) })} /></label>}
        </div>}
        {look.type === 'image' && <>
          <div className="pic-pick">
            {pic ? <img src={pic.url} alt="" className={pic.width <= 128 ? 'pixelated' : ''} /> : <span className="muted small">No picture yet</span>}
            <div className="stack">
              <button type="button" className="btn sm" onClick={() => setPick(true)}><Icon name="image" size={14} />Choose</button>
              <button type="button" className="btn sm" onClick={() => setDraw(pic?.origin === 'drawn' ? pic : 'new')}><Icon name="brush" size={14} />{pic?.origin === 'drawn' ? 'Draw on it' : 'Draw one'}</button>
            </div>
          </div>
          {pic && pic.frames > 1 && <Field label="Frames a second"><Num value={look.fps || 6} min={1} max={30} onChange={v => setLook({ fps: v })} /></Field>}
        </>}
        {(look.type === 'text' || look.type === 'box' || look.type === 'circle') && obj.role !== 'wall' && <>
          <Field label="Words on it" help="Each one placed in a level can have its own words."><input value={obj.text || ''} onChange={e => onChange({ ...obj, text: e.target.value })} /></Field>
          {(obj.text || look.type === 'text') && <div className="row wrap"><label className="row small">Size <Num value={look.size || 16} min={8} max={64} onChange={v => setLook({ size: v })} /></label><label className="check-row small"><input type="checkbox" checked={!!look.bold} onChange={e => setLook({ bold: e.target.checked })} /> Bold</label>{look.type !== 'text' && <label className="row small">Colour <input type="color" className="color-in" value={look.textColor || '#ffffff'} onChange={e => setLook({ textColor: e.target.value })} /></label>}</div>}
        </>}
        <div className="row wrap">
          <label className="row small">Width <Num value={obj.w} min={2} max={2000} onChange={v => onChange({ ...obj, w: v })} /></label>
          <label className="row small">Height <Num value={obj.h} min={2} max={2000} onChange={v => onChange({ ...obj, h: v })} /></label>
        </div>
        <Field label="In front or behind"><Segmented size="sm" value={obj.z <= -1 ? -1 : obj.z >= 5 ? 5 : 0} onChange={v => onChange({ ...obj, z: v })} options={[{ value: -1, label: 'Behind' }, { value: 0, label: 'Middle' }, { value: 5, label: 'In front' }]} /></Field>
      </div>
      <div className="insp-sec">
        <h4>What it does</h4>
        {obj.behaviors.length === 0 && <p className="muted small">Nothing yet: it just stands there. Add a behaviour, or make rules for it.</p>}
        {obj.behaviors.map(b => {
          const spec = BEHAVIORS.find(x => x.t === b.t)
          if (!spec) return null
          const p = b.p || {}
          return (
            <div key={b.t} className="beh">
              <div className="beh-head"><Icon name={spec.icon} size={15} /><b className="grow">{spec.name}</b><button type="button" className="btn ghost icon-only sm" aria-label="Remove" title="Remove" onClick={() => setBeh(b.t, null)}><Icon name="x" size={14} /></button></div>
              {spec.params.length > 0 && <div className="beh-params">{spec.params.map(pr => (
                <label key={pr.k} className={`beh-param ${pr.type === 'bool' ? 'bool' : ''}`}>
                  {pr.type === 'bool' ? <><input type="checkbox" checked={p[pr.k] === undefined ? !!pr.def : !!p[pr.k]} onChange={e => setBeh(b.t, { ...p, [pr.k]: e.target.checked })} />{pr.label}</> : <>
                    <span className="small muted">{pr.label}</span>
                    {pr.type === 'number' && <Num value={Number(p[pr.k] ?? pr.def)} min={pr.min} max={pr.max} step={pr.step} onChange={v => setBeh(b.t, { ...p, [pr.k]: v })} />}
                    {pr.type === 'select' && <Select ariaLabel={pr.label} value={String(p[pr.k] ?? pr.def)} onChange={v => setBeh(b.t, { ...p, [pr.k]: v })} options={pr.options!} />}
                    {pr.type === 'object' && <Select ariaLabel={pr.label} value={String(p[pr.k] ?? '')} onChange={v => setBeh(b.t, { ...p, [pr.k]: v })} options={objectOptions(doc).filter(o => o.value !== obj.id || b.t === 'follow')} placeholder="Choose a thing" />}
                  </>}
                </label>
              ))}</div>}
            </div>
          )
        })}
        {addable.length > 0 && <MenuButton label="Add a behaviour" className="btn sm" width={300} items={addable}><Icon name="plus" size={14} />Add a behaviour</MenuButton>}
      </div>
      <div className="insp-sec">
        <button type="button" className="btn sm danger-text" onClick={onRemove}><Icon name="trash" size={14} />Delete “{obj.name}”{used ? ` and the ${used} placed` : ''}</button>
      </div>
      {pick && <PickerDialog kind="image" gameId={gameId} onClose={() => setPick(false)} onPicked={a => { onAssets(); onChange({ ...obj, look: { ...look, type: 'image', image: a.id }, ...(obj.look.image ? {} : sizeFor(a, obj)) }) }} />}
      {draw && <PixelEditor asset={draw === 'new' ? undefined : draw} gameId={gameId} onClose={() => setDraw(null)} onSaved={a => { onAssets(); onChange({ ...obj, look: { ...look, type: 'image', image: a.id }, ...(obj.look.image ? {} : sizeFor(a, obj)) }) }} />}
    </div>
  )
}

/** A first size for a new picture: its shape, about as big as the thing was. */
function sizeFor(a: Asset, obj: Obj) {
  const fw = a.width / Math.max(1, a.frames), fh = a.height
  if (!fw || !fh) return {}
  const big = Math.max(obj.w, obj.h, 16)
  const k = big / Math.max(fw, fh)
  return { w: Math.round(fw * k), h: Math.round(fh * k) }
}

export function InstanceEditor({ doc, inst, onChange, onRemove, onDuplicate }: { doc: GameDoc; inst: Inst; onChange: (i: Inst) => void; onRemove: () => void; onDuplicate: () => void }) {
  const o = doc.objects.find(x => x.id === inst.o)
  if (!o) return null
  const textual = o.look.type === 'text' || ((o.look.type === 'box' || o.look.type === 'circle') && o.role !== 'wall')
  return (
    <div className="insp-sec inst">
      <h4>This {o.name}</h4>
      <div className="row wrap">
        <label className="row small">x <Num value={inst.x} onChange={v => onChange({ ...inst, x: v })} /></label>
        <label className="row small">y <Num value={inst.y} onChange={v => onChange({ ...inst, y: v })} /></label>
      </div>
      <div className="row wrap">
        <label className="row small">Width <Num value={inst.w || o.w} min={2} onChange={v => onChange({ ...inst, w: v === o.w ? undefined : v })} /></label>
        <label className="row small">Height <Num value={inst.h || o.h} min={2} onChange={v => onChange({ ...inst, h: v === o.h ? undefined : v })} /></label>
      </div>
      {textual && <Field label="Words"><input value={inst.text ?? ''} placeholder={o.text || ''} onChange={e => onChange({ ...inst, text: e.target.value || undefined })} /></Field>}
      {(o.look.type === 'box' || o.look.type === 'circle' || o.look.type === 'triangle') && <Field label="Its own colour"><ColorPick value={inst.color || o.look.color || ''} onChange={c => onChange({ ...inst, color: c })} /></Field>}
      <div className="row wrap">
        <label className="check-row small"><input type="checkbox" checked={!!inst.flip} onChange={e => onChange({ ...inst, flip: e.target.checked || undefined })} /> Turned round</label>
        <span className="spacer" />
        <button type="button" className="btn sm" onClick={onDuplicate}><Icon name="copy" size={14} />Copy</button>
        <button type="button" className="btn sm" onClick={onRemove}><Icon name="trash" size={14} />Remove</button>
      </div>
    </div>
  )
}

export function SceneEditor({ doc, scene, gameId, assets, onChange, onAssets }: { doc: GameDoc; scene: Scene; gameId: number; assets: Asset[]; onChange: (s: Scene) => void; onAssets: () => void }) {
  const [pick, setPick] = useState(false)
  const g = scene.grid || 24
  const bg = scene.bgImage ? assets.find(a => a.id === scene.bgImage) : undefined
  const music = doc.sounds.filter(s => s.asset && assets.find(a => a.id === s.asset)?.kind === 'music' || s.loop)
  return (
    <div className="insp">
      <div className="insp-sec">
        <h4>This level</h4>
        <Field label="Name"><input value={scene.name} onChange={e => onChange({ ...scene, name: e.target.value })} /></Field>
        <div className="row wrap">
          <label className="row small">Wide <Num value={Math.round(scene.w / g)} min={1} max={400} onChange={v => onChange({ ...scene, w: Math.max(v * g, 40) })} /> cells</label>
          <label className="row small">High <Num value={Math.round(scene.h / g)} min={1} max={400} onChange={v => onChange({ ...scene, h: Math.max(v * g, 40) })} /> cells</label>
        </div>
        <p className="muted small">{scene.w} × {scene.h} pixels; the screen shows {doc.width} × {doc.height}. A bigger level scrolls with the camera.</p>
        <Field label="Grid"><Segmented size="sm" value={g} onChange={v => onChange({ ...scene, grid: v })} options={[8, 10, 16, 24, 32, 48].map(n => ({ value: n, label: String(n) }))} /></Field>
        <Field label="Background"><ColorPick value={scene.bg} onChange={c => onChange({ ...scene, bg: c })} /></Field>
        <div className="pic-pick">
          {bg ? <img src={bg.url} alt="" /> : <span className="muted small">No background picture</span>}
          <div className="stack">
            <button type="button" className="btn sm" onClick={() => setPick(true)}><Icon name="image" size={14} />Background picture</button>
            {bg && <button type="button" className="btn ghost sm" onClick={() => onChange({ ...scene, bgImage: undefined })}>Take it away</button>}
          </div>
        </div>
        <Field label="Music" help="Add music in the Sounds tab.">
          <Select ariaLabel="Music" value={scene.music || ''} onChange={v => onChange({ ...scene, music: v || undefined })} options={[{ value: '', label: 'None' }, ...music.map(s => ({ value: s.id, label: s.name }))]} />
        </Field>
        <Field label="The camera follows"><Select ariaLabel="Camera" value={scene.camera || ''} onChange={v => onChange({ ...scene, camera: v || undefined })} options={[{ value: '', label: 'The thing that has “the camera follows it”' }, ...objectOptions(doc)]} /></Field>
      </div>
      {pick && <PickerDialog kind="image" gameId={gameId} onClose={() => setPick(false)} onPicked={a => { onAssets(); onChange({ ...scene, bgImage: a.id }) }} />}
    </div>
  )
}

export function newObject(doc: GameDoc, base: Omit<Obj, 'id'>): Obj {
  let id = base.name.toLowerCase().replace(/[^a-z0-9]+/g, '').slice(0, 20) || 'thing'
  if (doc.objects.some(o => o.id === id)) id = `${id}-${newId().slice(0, 4)}`
  let name = base.name
  let n = 2
  while (doc.objects.some(o => o.name === name)) name = `${base.name} ${n++}`
  return { ...base, id, name }
}
