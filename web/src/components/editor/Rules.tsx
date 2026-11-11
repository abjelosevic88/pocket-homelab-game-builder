// Rules as sentences: "When the Cat touches a Coin: add 1 to score, remove the Coin."
import { useState } from 'react'
import { api, V1 } from '../../lib/api'
import { newId, type GameDoc, type Rule, type Step } from '../../engine/types'
import { MenuButton, Spinner, Switch, useToast, errMsg, type MenuItem } from '../ui'
import { Select, type SelectOption } from '../Select'
import { Icon } from '../icons'
import { objectOptions } from './Inspector'

const CONDS: { t: string; label: string; make: (d: GameDoc) => Step }[] = [
  { t: 'touch', label: 'Something touches something', make: d => ({ t: 'touch', a: first(d, 'player'), b: first(d, 'coin') }) },
  { t: 'stomp', label: 'Something lands on top of something', make: d => ({ t: 'stomp', a: first(d, 'player'), b: first(d, 'enemy') }) },
  { t: 'tap', label: 'Something is tapped or clicked', make: d => ({ t: 'tap', a: first(d, 'button') }) },
  { t: 'key', label: 'A key is pressed', make: () => ({ t: 'key', key: 'action', how: 'press' }) },
  { t: 'leaves', label: 'Something leaves the level', make: d => ({ t: 'leaves', a: first(d, 'player'), side: 'bottom' }) },
  { t: 'var', label: 'A number is …', make: d => ({ t: 'var', name: d.vars[0]?.name || 'score', op: '>=', value: 10 }) },
  { t: 'count', label: 'How many there are of something', make: d => ({ t: 'count', o: first(d, 'coin'), op: '=', value: 0 }) },
  { t: 'start', label: 'The level starts', make: () => ({ t: 'start' }) },
  { t: 'every', label: 'Every few seconds', make: () => ({ t: 'every', s: 1 }) },
  { t: 'after', label: 'Some seconds after the start', make: () => ({ t: 'after', s: 10 }) },
  { t: 'chance', label: 'By chance', make: () => ({ t: 'chance', p: 20 }) },
]
const ACTS: { t: string; label: string; make: (d: GameDoc, r: Rule) => Step }[] = [
  { t: 'add', label: 'Add to a number', make: d => ({ t: 'add', name: d.vars[0]?.name || 'score', value: 1 }) },
  { t: 'set', label: 'Set a number', make: d => ({ t: 'set', name: d.vars[0]?.name || 'score', value: 0 }) },
  { t: 'destroy', label: 'Remove something', make: (_d, r) => ({ t: 'destroy', who: pairOf(r) ? 'b' : 'a' }) },
  { t: 'create', label: 'Make something appear', make: d => ({ t: 'create', o: d.objects[0]?.id || '', at: 'top' }) },
  { t: 'sound', label: 'Play a sound', make: d => ({ t: 'sound', s: d.sounds[0]?.id || '' }) },
  { t: 'say', label: 'Say something', make: () => ({ t: 'say', text: 'Well done!', s: 1.5 }) },
  { t: 'moveto', label: 'Put back at the start', make: () => ({ t: 'moveto', who: 'a', at: 'start' }) },
  { t: 'jump', label: 'Make it jump', make: () => ({ t: 'jump', who: 'a', power: 300 }) },
  { t: 'push', label: 'Push it', make: () => ({ t: 'push', who: 'a', vx: 0, vy: -200 }) },
  { t: 'hide', label: 'Hide something', make: () => ({ t: 'hide', who: 'b' }) },
  { t: 'show', label: 'Show something again', make: d => ({ t: 'show', who: d.objects[0]?.id || '' }) },
  { t: 'look', label: 'Change how it looks', make: () => ({ t: 'look', who: 'a', emoji: '😎' }) },
  { t: 'shake', label: 'Shake the screen', make: () => ({ t: 'shake', s: 0.3 }) },
  { t: 'music', label: 'Start or stop music', make: () => ({ t: 'music', s: 'stop' }) },
  { t: 'scene', label: 'Go to another level', make: () => ({ t: 'scene', scene: 'next' }) },
  { t: 'win', label: 'Win the game', make: () => ({ t: 'win', text: '' }) },
  { t: 'lose', label: 'Lose the game', make: () => ({ t: 'lose', text: '' }) },
]

function first(d: GameDoc, role: string) { return d.objects.find(o => o.role === role)?.id || d.objects[0]?.id || '' }
function pairOf(r: Rule) { return r.when.find(c => c.t === 'touch' || c.t === 'stomp') }
function boundOf(r: Rule) { return r.when.find(c => c.t === 'touch' || c.t === 'stomp' || c.t === 'tap' || c.t === 'leaves') }

const KEYS: SelectOption[] = [{ value: 'left', label: '←' }, { value: 'right', label: '→' }, { value: 'up', label: '↑' }, { value: 'down', label: '↓' }, { value: 'jump', label: 'Jump (Space, ↑)' }, { value: 'action', label: 'Action (Space, X)' }, { value: 'any', label: 'Any key' }]
const OPS: SelectOption[] = [{ value: '=', label: 'is' }, { value: '!=', label: 'is not' }, { value: '>=', label: 'is at least' }, { value: '<=', label: 'is at most' }, { value: '>', label: 'is more than' }, { value: '<', label: 'is less than' }]
const SIDES: SelectOption[] = [{ value: 'any', label: 'on any side' }, { value: 'bottom', label: 'at the bottom' }, { value: 'top', label: 'at the top' }, { value: 'left', label: 'on the left' }, { value: 'right', label: 'on the right' }]

function N({ v, on, step = 1, w = 64 }: { v: unknown; on: (n: number) => void; step?: number; w?: number }) {
  return <input type="number" className="rule-num" style={{ width: w }} step={step} value={Number(v ?? 0)} onChange={e => { const n = Number(e.target.value); if (Number.isFinite(n)) on(n) }} />
}
function T({ v, on, ph }: { v: unknown; on: (s: string) => void; ph?: string }) {
  return <input className="rule-text" value={String(v ?? '')} placeholder={ph} onChange={e => on(e.target.value)} />
}

function VarPick({ doc, v, on, onNewVar }: { doc: GameDoc; v: unknown; on: (s: string) => void; onNewVar: (n: string) => void }) {
  const opts = doc.vars.map(x => ({ value: x.name, label: x.name }))
  return <Select ariaLabel="Number" className="rule-sel" value={String(v || '')} onChange={on} options={opts} onCreate={q => { const n = q.trim().replace(/\s+/g, '_').toLowerCase() || 'number'; onNewVar(n); return n }} createLabel="New number" placeholder="a number" />
}

function CondRow({ doc, c, set, onNewVar }: { doc: GameDoc; c: Step; set: (s: Step) => void; onNewVar: (n: string) => void }) {
  const objs = objectOptions(doc)
  const O = (k: string, ph = 'something') => <Select ariaLabel="Thing" className="rule-sel" value={String(c[k] || '')} onChange={v => set({ ...c, [k]: v })} options={objs} placeholder={ph} />
  switch (c.t) {
    case 'touch': return <>{O('a')}<span>touches</span>{O('b')}</>
    case 'stomp': return <>{O('a')}<span>lands on top of</span>{O('b')}</>
    case 'tap': return <>{O('a')}<span>is tapped or clicked</span></>
    case 'key': return <><Select ariaLabel="Key" className="rule-sel" value={String(c.key || 'action')} onChange={v => set({ ...c, key: v })} options={KEYS} /><Select ariaLabel="How" className="rule-sel" value={String(c.how || 'press')} onChange={v => set({ ...c, how: v })} options={[{ value: 'press', label: 'is pressed' }, { value: 'hold', label: 'is held down' }]} /></>
    case 'leaves': return <>{O('a')}<span>leaves the level</span><Select ariaLabel="Side" className="rule-sel" value={String(c.side || 'any')} onChange={v => set({ ...c, side: v })} options={SIDES} /></>
    case 'var': return <><VarPick doc={doc} v={c.name} on={v => set({ ...c, name: v })} onNewVar={onNewVar} /><Select ariaLabel="Compare" className="rule-sel" value={String(c.op || '>=')} onChange={v => set({ ...c, op: v })} options={OPS} /><N v={c.value} on={v => set({ ...c, value: v })} /></>
    case 'count': return <><span>the number of</span>{O('o')}<Select ariaLabel="Compare" className="rule-sel" value={String(c.op || '=')} onChange={v => set({ ...c, op: v })} options={OPS} /><N v={c.value} on={v => set({ ...c, value: v })} /></>
    case 'start': return <span>the level starts</span>
    case 'every': return <><span>every</span><N v={c.s} step={0.1} on={v => set({ ...c, s: v })} /><span>seconds</span></>
    case 'after': return <><N v={c.s} step={0.5} on={v => set({ ...c, s: v })} /><span>seconds after the level starts</span></>
    case 'chance': return <><span>by chance,</span><N v={c.p} on={v => set({ ...c, p: v })} /><span>% each second</span></>
  }
  return <span className="muted">{c.t}</span>
}

function ActRow({ doc, r, a, set, onNewVar }: { doc: GameDoc; r: Rule; a: Step; set: (s: Step) => void; onNewVar: (n: string) => void }) {
  const name = (id: unknown) => doc.objects.find(o => o.id === id)?.name || 'thing'
  const b = boundOf(r)
  const whoOpts: SelectOption[] = [
    ...(b ? [{ value: 'a', label: `the ${name(b.a)}${b.t === 'touch' || b.t === 'stomp' ? ' (that touched)' : ''}` }] : []),
    ...(b && (b.t === 'touch' || b.t === 'stomp') ? [{ value: 'b', label: `the ${name(b.b)} (that was touched)` }] : []),
    ...doc.objects.map(o => ({ value: o.id, label: `every ${o.name}` })),
  ]
  const W = () => <Select ariaLabel="Who" className="rule-sel" value={String(a.who || '')} onChange={v => set({ ...a, who: v })} options={whoOpts} placeholder="which" />
  const sounds = doc.sounds.map(s => ({ value: s.id, label: s.name }))
  switch (a.t) {
    case 'add': return <><span>add</span><N v={a.value} on={v => set({ ...a, value: v })} /><span>to</span><VarPick doc={doc} v={a.name} on={v => set({ ...a, name: v })} onNewVar={onNewVar} /></>
    case 'set': return <><span>set</span><VarPick doc={doc} v={a.name} on={v => set({ ...a, name: v })} onNewVar={onNewVar} /><span>to</span><N v={a.value} on={v => set({ ...a, value: v })} /></>
    case 'destroy': return <><span>remove</span>{W()}</>
    case 'hide': return <><span>hide</span>{W()}</>
    case 'show': return <><span>show</span>{W()}<span>again</span></>
    case 'create': return <><span>make a</span><Select ariaLabel="Thing" className="rule-sel" value={String(a.o || '')} onChange={v => set({ ...a, o: v })} options={objectOptions(doc)} /><span>appear</span><Select ariaLabel="Where" className="rule-sel" value={String(a.at || 'top')} onChange={v => set({ ...a, at: v })} options={[...(b ? [{ value: 'a', label: `where the ${name(b.a)} is` }] : []), ...(b && b.b ? [{ value: 'b', label: `where the ${name(b.b)} is` }] : []), { value: 'top', label: 'at the top, anywhere' }, { value: 'random', label: 'anywhere on the screen' }, { value: 'start', label: 'where the first one was' }]} /></>
    case 'moveto': return <><span>put</span>{W()}<span>back where it started</span></>
    case 'jump': return <><span>make</span>{W()}<span>jump, power</span><N v={a.power} step={10} on={v => set({ ...a, power: v })} /></>
    case 'push': return <><span>push</span>{W()}<span>sideways</span><N v={a.vx} step={10} on={v => set({ ...a, vx: v })} /><span>up/down</span><N v={a.vy} step={10} on={v => set({ ...a, vy: v })} /></>
    case 'look': return <><span>turn</span>{W()}<span>into</span><input className="rule-text emoji-in sm" value={String(a.emoji || '')} onChange={e => set({ ...a, emoji: [...e.target.value].slice(-2).join('') })} aria-label="Emoji" /><span>or colour</span><input type="color" className="color-in sm" value={String(a.color || '#ffffff')} onChange={e => set({ ...a, color: e.target.value })} /></>
    case 'sound': return <><span>play</span><Select ariaLabel="Sound" className="rule-sel" value={String(a.s || '')} onChange={v => set({ ...a, s: v })} options={sounds} placeholder="a sound" /></>
    case 'music': return <><span>music:</span><Select ariaLabel="Music" className="rule-sel" value={String(a.s || 'stop')} onChange={v => set({ ...a, s: v })} options={[{ value: 'stop', label: 'stop' }, ...sounds]} /></>
    case 'say': return <><span>say</span><T v={a.text} on={v => set({ ...a, text: v })} ph="Well done! You have {score}" /><span>for</span><N v={a.s} step={0.5} w={56} on={v => set({ ...a, s: v })} /><span>s</span></>
    case 'shake': return <><span>shake the screen for</span><N v={a.s} step={0.1} w={56} on={v => set({ ...a, s: v })} /><span>s</span></>
    case 'scene': return <><span>go to</span><Select ariaLabel="Level" className="rule-sel" value={String(a.scene || 'next')} onChange={v => set({ ...a, scene: v })} options={[{ value: 'next', label: 'the next level' }, { value: 'restart', label: 'this level again' }, ...doc.scenes.map(s => ({ value: s.id, label: s.name }))]} /><span>after</span><N v={a.wait} step={0.5} w={56} on={v => set({ ...a, wait: v })} /><span>s</span></>
    case 'win': return <><span>win, saying</span><T v={a.text} on={v => set({ ...a, text: v })} ph={doc.ends.won} /></>
    case 'lose': return <><span>lose, saying</span><T v={a.text} on={v => set({ ...a, text: v })} ph={doc.ends.lost} /></>
  }
  return <span className="muted">{a.t}</span>
}

export function RulesEditor({ doc, scope, onDoc }: { doc: GameDoc; scope: string; onDoc: (d: GameDoc) => void }) {
  const { toast } = useToast()
  const [ask, setAsk] = useState('')
  const [busy, setBusy] = useState(false)
  const scene = doc.scenes.find(s => s.id === scope)
  const rules = scene ? scene.rules : doc.rules
  const setRules = (rs: Rule[]) => {
    if (scene) onDoc({ ...doc, scenes: doc.scenes.map(s => s.id === scene.id ? { ...s, rules: rs } : s) })
    else onDoc({ ...doc, rules: rs })
  }
  const setRule = (r: Rule) => setRules(rules.map(x => x.id === r.id ? r : x))
  const newVar = (n: string) => { if (!doc.vars.some(v => v.name === n)) onDoc({ ...doc, vars: [...doc.vars, { name: n, value: 0 }] }) }
  const move = (i: number, d: number) => { const rs = [...rules]; const [r] = rs.splice(i, 1); rs.splice(Math.max(0, Math.min(rs.length, i + d)), 0, r); setRules(rs) }
  const addRule = () => setRules([...rules, { id: newId(), on: true, when: [CONDS[0].make(doc)], do: [ACTS[0].make(doc, { id: '', on: true, when: [CONDS[0].make(doc)], do: [] })] }])
  const fromAI = async () => {
    setBusy(true)
    try {
      const r = await api.post<{ rules: Rule[]; explain: string }>(`${V1}/ai/rule`, { text: ask, objects: doc.objects.map(o => ({ id: o.id, name: o.name })), vars: doc.vars.map(v => v.name), sounds: doc.sounds.map(s => ({ id: s.id, name: s.name })), scenes: doc.scenes.map(s => ({ id: s.id, name: s.name })) })
      const vars = [...doc.vars]
      for (const ru of r.rules) for (const st of [...ru.when, ...ru.do]) if (typeof st.name === 'string' && !vars.some(v => v.name === st.name)) vars.push({ name: st.name, value: 0 })
      const rs = [...rules, ...r.rules.map(x => ({ ...x, id: newId(), on: true }))]
      onDoc(scene ? { ...doc, vars, scenes: doc.scenes.map(s => s.id === scene.id ? { ...s, rules: rs } : s) } : { ...doc, vars, rules: rs })
      setAsk('')
      toast(r.explain || 'Rule added')
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <div className="rules">
      <form className="rule-ai" onSubmit={e => { e.preventDefault(); if (ask.trim()) void fromAI() }}>
        <Icon name="sparkles" size={16} />
        <input className="grow" value={ask} onChange={e => setAsk(e.target.value)} placeholder="Say a rule: “when the ghost touches the player, lose a life”" />
        <button type="submit" className="btn sm" disabled={!ask.trim() || busy}>{busy ? <Spinner /> : 'Make the rule'}</button>
      </form>
      {rules.length === 0 && <p className="muted">No rules here yet. Rules say what happens: when something touches something, a key is pressed, a number gets to 10…</p>}
      {rules.map((r, i) => (
        <div key={r.id} className={`rule ${r.on ? '' : 'off'}`}>
          <div className="rule-head">
            <Switch size="sm" checked={r.on} onChange={v => setRule({ ...r, on: v })} label="On" />
            <input className="rule-note grow" value={r.note || ''} onChange={e => setRule({ ...r, note: e.target.value || undefined })} placeholder={`Rule ${i + 1}`} />
            <MenuButton label="Rule" align="right" className="btn ghost icon-only sm" items={(): MenuItem[] => [
              { label: r.once ? 'Every time' : 'Only the first time', icon: 'repeat', onSelect: () => setRule({ ...r, once: !r.once || undefined }) },
              { label: 'Copy', icon: 'copy', onSelect: () => setRules([...rules.slice(0, i + 1), { ...JSON.parse(JSON.stringify(r)), id: newId() }, ...rules.slice(i + 1)]) },
              { label: 'Move up', icon: 'arrow-up', disabled: i === 0, onSelect: () => move(i, -1) },
              { label: 'Move down', icon: 'arrow-down', disabled: i === rules.length - 1, onSelect: () => move(i, 1) },
              'sep', { label: 'Delete', icon: 'trash', danger: true, onSelect: () => setRules(rules.filter(x => x.id !== r.id)) },
            ]}><Icon name="more" size={15} /></MenuButton>
          </div>
          <div className="rule-part">
            <span className="rule-kw">When</span>
            <div className="rule-lines">
              {r.when.map((c, k) => (
                <div key={k} className="rule-line">
                  {k > 0 && <span className="rule-and">and</span>}
                  <CondRow doc={doc} c={c} set={s => setRule({ ...r, when: r.when.map((x, j) => j === k ? s : x) })} onNewVar={newVar} />
                  <button type="button" className="btn ghost icon-only xs-x" aria-label="Remove" onClick={() => setRule({ ...r, when: r.when.filter((_, j) => j !== k) })}><Icon name="x" size={13} /></button>
                </div>
              ))}
              <MenuButton label="Add a condition" className="btn ghost sm add-line" width={280} items={CONDS.map(c => ({ label: c.label, onSelect: () => setRule({ ...r, when: [...r.when, c.make(doc)] }) }))}><Icon name="plus" size={13} />{r.when.length ? 'and…' : 'when…'}</MenuButton>
            </div>
          </div>
          <div className="rule-part">
            <span className="rule-kw do">Do</span>
            <div className="rule-lines">
              {r.do.map((a, k) => (
                <div key={k} className="rule-line">
                  <ActRow doc={doc} r={r} a={a} set={s => setRule({ ...r, do: r.do.map((x, j) => j === k ? s : x) })} onNewVar={newVar} />
                  <button type="button" className="btn ghost icon-only xs-x" aria-label="Remove" onClick={() => setRule({ ...r, do: r.do.filter((_, j) => j !== k) })}><Icon name="x" size={13} /></button>
                </div>
              ))}
              <MenuButton label="Add an action" className="btn ghost sm add-line" width={260} items={ACTS.map(a => ({ label: a.label, onSelect: () => setRule({ ...r, do: [...r.do, a.make(doc, r)] }) }))}><Icon name="plus" size={13} />then…</MenuButton>
            </div>
          </div>
          {r.once && <p className="muted small">Only the first time in each level.</p>}
        </div>
      ))}
      <button type="button" className="btn" onClick={addRule}><Icon name="plus" size={15} />Add a rule</button>
    </div>
  )
}
