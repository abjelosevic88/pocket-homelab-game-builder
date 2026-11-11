// Sound effects made from a few numbers (in the spirit of sfxr): a wave,
// a pitch that slides, an envelope, a little vibrato and an arpeggio jump.

export type Wave = 'square' | 'saw' | 'sine' | 'triangle' | 'noise'

export interface Sfx {
  wave: Wave
  freq: number      // start pitch, Hz
  slide: number     // pitch change per second, in octaves (-4 … 4)
  attack: number    // seconds
  sustain: number
  decay: number
  vibrato: number   // depth, 0 … 1
  vibSpeed: number  // Hz
  arp: number       // pitch multiplier after arpTime (1 = none)
  arpTime: number
  duty: number      // square wave width, 0.1 … 0.9
  volume: number    // 0 … 1
}

export const SFX_DEFAULT: Sfx = { wave: 'square', freq: 440, slide: 0, attack: 0, sustain: 0.1, decay: 0.15, vibrato: 0, vibSpeed: 6, arp: 1, arpTime: 0.1, duty: 0.5, volume: 0.5 }

export const PRESETS: Record<string, { name: string; sfx: Partial<Sfx> }> = {
  coin: { name: 'Coin', sfx: { wave: 'square', freq: 880, attack: 0, sustain: 0.05, decay: 0.2, arp: 1.5, arpTime: 0.06, duty: 0.4 } },
  jump: { name: 'Jump', sfx: { wave: 'square', freq: 300, slide: 2.2, attack: 0, sustain: 0.08, decay: 0.12, duty: 0.3 } },
  hit: { name: 'Hit', sfx: { wave: 'noise', freq: 400, slide: -2, attack: 0, sustain: 0.04, decay: 0.18 } },
  explode: { name: 'Explosion', sfx: { wave: 'noise', freq: 180, slide: -1.2, attack: 0, sustain: 0.15, decay: 0.5, volume: 0.6 } },
  powerup: { name: 'Power-up', sfx: { wave: 'square', freq: 330, slide: 1.5, attack: 0, sustain: 0.2, decay: 0.25, vibrato: 0.15, vibSpeed: 14 } },
  laser: { name: 'Laser', sfx: { wave: 'saw', freq: 1200, slide: -3.5, attack: 0, sustain: 0.05, decay: 0.12, volume: 0.35 } },
  blip: { name: 'Blip', sfx: { wave: 'square', freq: 660, attack: 0, sustain: 0.03, decay: 0.05, duty: 0.5, volume: 0.4 } },
  win: { name: 'Fanfare', sfx: { wave: 'triangle', freq: 523, attack: 0.01, sustain: 0.25, decay: 0.4, arp: 1.335, arpTime: 0.12, vibrato: 0.08, vibSpeed: 7 } },
  lose: { name: 'Sad', sfx: { wave: 'triangle', freq: 330, slide: -0.9, attack: 0.01, sustain: 0.3, decay: 0.4, vibrato: 0.1, vibSpeed: 5 } },
  click: { name: 'Click', sfx: { wave: 'sine', freq: 1400, attack: 0, sustain: 0.01, decay: 0.03, volume: 0.4 } },
  bounce: { name: 'Bounce', sfx: { wave: 'sine', freq: 220, slide: 1.8, attack: 0, sustain: 0.04, decay: 0.12 } },
}

/** A preset, with the changes someone made on top of it. */
export function sfxOf(p?: Record<string, unknown>): Sfx {
  const preset = p && typeof p.preset === 'string' ? PRESETS[p.preset]?.sfx : undefined
  const out = { ...SFX_DEFAULT, ...(preset || {}) } as Sfx
  if (p) for (const k of Object.keys(SFX_DEFAULT) as (keyof Sfx)[]) if (p[k] !== undefined) (out as unknown as Record<string, unknown>)[k] = p[k]
  return out
}

export function randomSfx(): Sfx {
  const r = Math.random
  const waves: Wave[] = ['square', 'saw', 'sine', 'triangle', 'noise']
  return {
    ...SFX_DEFAULT, wave: waves[Math.floor(r() * waves.length)], freq: 120 + r() * 1400, slide: (r() - 0.5) * 6,
    attack: r() < 0.7 ? 0 : r() * 0.1, sustain: 0.02 + r() * 0.25, decay: 0.05 + r() * 0.4, vibrato: r() < 0.6 ? 0 : r() * 0.4,
    vibSpeed: 3 + r() * 15, arp: r() < 0.6 ? 1 : 0.5 + r() * 1.5, arpTime: 0.03 + r() * 0.2, duty: 0.2 + r() * 0.6, volume: 0.5,
  }
}

const RATE = 44100

/** The sound as samples. */
export function render(s: Sfx): Float32Array {
  const len = Math.max(0.01, s.attack + s.sustain + s.decay)
  const n = Math.min(RATE * 3, Math.floor(len * RATE))
  const out = new Float32Array(n)
  let phase = 0
  let noise = 0
  let lastCycle = -1
  for (let i = 0; i < n; i++) {
    const t = i / RATE
    let f = s.freq * Math.pow(2, s.slide * t)
    if (s.arp !== 1 && t >= s.arpTime) f *= s.arp
    if (s.vibrato > 0) f *= 1 + s.vibrato * 0.5 * Math.sin(2 * Math.PI * s.vibSpeed * t)
    f = Math.max(20, Math.min(12000, f))
    phase += f / RATE
    const p = phase % 1
    let v = 0
    switch (s.wave) {
      case 'square': v = p < s.duty ? 0.6 : -0.6; break
      case 'saw': v = (p * 2 - 1) * 0.7; break
      case 'sine': v = Math.sin(2 * Math.PI * p); break
      case 'triangle': v = (p < 0.5 ? p * 4 - 1 : 3 - p * 4) * 0.9; break
      case 'noise': {
        const cycle = Math.floor(phase * 2)
        if (cycle !== lastCycle) { noise = Math.random() * 2 - 1; lastCycle = cycle }
        v = noise * 0.7
      }
    }
    let env: number
    if (t < s.attack) env = t / s.attack
    else if (t < s.attack + s.sustain) env = 1
    else env = Math.max(0, 1 - (t - s.attack - s.sustain) / Math.max(0.001, s.decay))
    out[i] = v * env * s.volume
  }
  return out
}

let ctx: AudioContext | null = null
export function audio(): AudioContext | null {
  if (ctx) return ctx
  const AC = (window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext)
  if (!AC) return null
  ctx = new AC()
  return ctx
}

const cache = new WeakMap<object, AudioBuffer>()

export function bufferOf(s: Sfx, key?: object): AudioBuffer | null {
  const ac = audio()
  if (!ac) return null
  if (key && cache.has(key)) return cache.get(key)!
  const data = render(s)
  const buf = ac.createBuffer(1, data.length, RATE)
  buf.copyToChannel(data as Float32Array<ArrayBuffer>, 0)
  if (key) cache.set(key, buf)
  return buf
}

export function playBuffer(buf: AudioBuffer | null, volume = 1) {
  const ac = audio()
  if (!ac || !buf) return
  if (ac.state === 'suspended') void ac.resume()
  const src = ac.createBufferSource()
  src.buffer = buf
  const g = ac.createGain()
  g.gain.value = volume
  src.connect(g).connect(ac.destination)
  src.start()
}

export function playSfx(s: Sfx) { playBuffer(bufferOf(s)) }

/** A WAV file of a sound effect (to keep or share). */
export function toWav(s: Sfx): Blob {
  const data = render(s)
  const buf = new ArrayBuffer(44 + data.length * 2)
  const v = new DataView(buf)
  const w = (o: number, str: string) => { for (let i = 0; i < str.length; i++) v.setUint8(o + i, str.charCodeAt(i)) }
  w(0, 'RIFF'); v.setUint32(4, 36 + data.length * 2, true); w(8, 'WAVE'); w(12, 'fmt ')
  v.setUint32(16, 16, true); v.setUint16(20, 1, true); v.setUint16(22, 1, true); v.setUint32(24, RATE, true); v.setUint32(28, RATE * 2, true)
  v.setUint16(32, 2, true); v.setUint16(34, 16, true); w(36, 'data'); v.setUint32(40, data.length * 2, true)
  for (let i = 0; i < data.length; i++) v.setInt16(44 + i * 2, Math.max(-1, Math.min(1, data[i])) * 0x7fff, true)
  return new Blob([buf], { type: 'audio/wav' })
}
