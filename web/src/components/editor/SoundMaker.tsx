// Make a sound effect: start from a kind of sound, change it with sliders, listen.
import { useState } from 'react'
import { PRESETS, SFX_DEFAULT, playSfx, randomSfx, sfxOf, type Sfx, type Wave } from '../../engine/sfx'
import { Field, Segmented } from '../ui'
import { Icon } from '../icons'

const SLIDERS: { k: keyof Sfx; label: string; min: number; max: number; step: number; unit?: string }[] = [
  { k: 'freq', label: 'Pitch', min: 60, max: 2400, step: 10, unit: 'Hz' },
  { k: 'slide', label: 'Slide up or down', min: -6, max: 6, step: 0.1 },
  { k: 'attack', label: 'Fade in', min: 0, max: 0.5, step: 0.01, unit: 's' },
  { k: 'sustain', label: 'Hold', min: 0, max: 1, step: 0.01, unit: 's' },
  { k: 'decay', label: 'Fade out', min: 0.01, max: 1.5, step: 0.01, unit: 's' },
  { k: 'vibrato', label: 'Wobble', min: 0, max: 0.8, step: 0.01 },
  { k: 'vibSpeed', label: 'Wobble speed', min: 1, max: 30, step: 0.5, unit: 'Hz' },
  { k: 'arp', label: 'Jump in pitch', min: 0.5, max: 2.5, step: 0.01, unit: '×' },
  { k: 'arpTime', label: 'Jump after', min: 0.01, max: 0.5, step: 0.01, unit: 's' },
  { k: 'duty', label: 'Buzz', min: 0.1, max: 0.9, step: 0.01 },
  { k: 'volume', label: 'Loudness', min: 0.05, max: 1, step: 0.01 },
]

/** The settings stored in a game: the preset plus what was changed. */
export function sfxParams(s: Sfx, preset?: string): Record<string, unknown> {
  const base = sfxOf(preset ? { preset } : undefined)
  const out: Record<string, unknown> = preset ? { preset } : {}
  for (const k of Object.keys(SFX_DEFAULT) as (keyof Sfx)[]) if (s[k] !== base[k]) out[k] = s[k]
  return out
}

export function SoundMaker({ value, onChange }: { value?: Record<string, unknown>; onChange: (params: Record<string, unknown>, preset?: string) => void }) {
  const [preset, setPreset] = useState<string | undefined>(typeof value?.preset === 'string' ? value.preset : undefined)
  const [s, setS] = useState<Sfx>(() => sfxOf(value))
  const set = (n: Sfx, p = preset) => { setS(n); onChange(sfxParams(n, p), p) }
  return (
    <div className="sound-maker">
      <div className="preset-row">
        {Object.entries(PRESETS).map(([id, p]) => (
          <button key={id} type="button" className={`chip ${preset === id ? 'on' : ''}`} onClick={() => { const n = sfxOf({ preset: id }); setPreset(id); set(n, id); playSfx(n) }}>{p.name}</button>
        ))}
        <button type="button" className="chip" onClick={() => { const n = randomSfx(); setPreset(undefined); set(n, undefined); playSfx(n) }}><Icon name="shuffle" size={13} />Surprise me</button>
      </div>
      <div className="row"><button type="button" className="btn primary" onClick={() => playSfx(s)}><Icon name="play" size={15} />Listen</button></div>
      <Field label="Sound"><Segmented size="sm" value={s.wave} onChange={(w: Wave) => { const n = { ...s, wave: w }; set(n); playSfx(n) }} options={[{ value: 'square', label: 'Beepy' }, { value: 'triangle', label: 'Soft' }, { value: 'sine', label: 'Pure' }, { value: 'saw', label: 'Sharp' }, { value: 'noise', label: 'Noise' }]} /></Field>
      <div className="sliders">
        {SLIDERS.map(sl => (
          <label key={sl.k} className="slider">
            <span className="slider-l">{sl.label}</span>
            <input type="range" min={sl.min} max={sl.max} step={sl.step} value={Number(s[sl.k])} onChange={e => set({ ...s, [sl.k]: Number(e.target.value) })} onPointerUp={() => playSfx(s)} onKeyUp={() => playSfx(s)} />
            <span className="slider-v tnum">{Number(s[sl.k]).toFixed(sl.step < 1 ? 2 : 0)}{sl.unit ? ' ' + sl.unit : ''}</span>
          </label>
        ))}
      </div>
    </div>
  )
}
