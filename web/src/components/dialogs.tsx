// The forms that open over a page.
import { useEffect, useMemo, useState } from 'react'
import qrcode from 'qrcode-generator'
import { api, uploadFiles, V1 } from '../lib/api'
import { useApp } from '../lib/app-context'
import { announceChange, openAction, useAppState } from '../lib/data'
import type { Asset, Game, Template } from '../lib/types'
import { fmtBytes } from '../lib/format'
import { CopyField, Field, Modal, Segmented, Spinner, Switch, Tabs, useToast, errMsg, download } from './ui'
import { Select, type SelectOption } from './Select'
import { Icon } from './icons'
import { AppHint } from './bits'
import { SIZES } from '../engine/types'

export function useList<T>(path: string, key: string, on = true): [T[] | null, () => void] {
  const [v, setV] = useState<T[] | null>(null)
  const [n, setN] = useState(0)
  useEffect(() => { if (on) api.get<Record<string, T[]>>(`${V1}${path}`).then(r => setV(r[key] || [])).catch(() => setV([])) }, [path, key, on, n])
  return [v, () => setN(x => x + 1)]
}

const SIZE_OPTS = Object.entries(SIZES).map(([id, [w, h, label]]) => ({ value: id, label: label.split(' (')[0], title: `${w} × ${h} — ${label}` }))

// ---------- making a game ----------

type NewTab = 'template' | 'idea' | 'quiz' | 'maze' | 'file'

export function NewGameDialog({ onClose, onMade, initial = 'template' }: { onClose: () => void; onMade: (g: Game) => void; initial?: NewTab }) {
  const { settings } = useApp()
  const { toast } = useToast()
  const state = useAppState()
  const [tab, setTab] = useState<NewTab>(initial)
  const [tpls] = useList<Template>('/templates', 'templates')
  const [tpl, setTpl] = useState('platformer')
  const [title, setTitle] = useState('')
  const [size, setSize] = useState('')
  const [idea, setIdea] = useState('')
  const [topic, setTopic] = useState('')
  const [questions, setQuestions] = useState<{ q: string; answers: string[] }[]>([])
  const [typed, setTyped] = useState('')
  const [busy, setBusy] = useState(false)
  const aiOn = settings?.ai_help !== false
  const [mazes] = useList<{ id: string; name: string }>('/apps/mazes', 'mazes', tab === 'maze' && state('puzzlecraft').ok)
  const [maze, setMaze] = useState('')

  const made = (g: Game) => { announceChange(); toast('Your game is ready'); onMade(g); onClose() }
  const run = async (fn: () => Promise<Game>) => { setBusy(true); try { made(await fn()) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) } }
  const chosen = tpls?.find(t => t.id === tpl)

  // "Question? / right / wrong / wrong" blocks, a blank line between
  const parsed = useMemo(() => typed.split(/\n\s*\n/).map(b => b.split('\n').map(l => l.trim()).filter(Boolean)).filter(b => b.length >= 3).map(b => ({ q: b[0], answers: b.slice(1, 5) })), [typed])
  const quizQs = questions.length ? questions : parsed

  const go = () => {
    switch (tab) {
      case 'template': return run(() => api.post<Game>(`${V1}/games`, { template: tpl, title, size: size || undefined }))
      case 'idea': return run(() => api.post<Game>(`${V1}/ai/game`, { idea }))
      case 'quiz': return run(() => api.post<Game>(`${V1}/games`, { template: 'quiz', title: title || (topic ? `Quiz: ${topic}` : 'My quiz'), questions: quizQs, size: size || undefined }))
      case 'maze': return run(() => api.post<Game>(`${V1}/apps/mazes/game`, { id: maze }))
    }
  }
  const pickFile = (f: File | undefined) => {
    if (!f) return
    void run(async () => {
      const text = await f.text()
      return api.post<Game>(`${V1}/games/import`, JSON.parse(text))
    })
  }
  const askQuestions = async () => {
    setBusy(true)
    try { const r = await api.post<{ questions: { q: string; answers: string[] }[] }>(`${V1}/ai/questions`, { topic, count: 8 }); setQuestions(r.questions) } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const can = tab === 'template' ? !!tpl : tab === 'idea' ? idea.trim().length > 3 && aiOn : tab === 'quiz' ? quizQs.length > 0 : tab === 'maze' ? !!maze : false
  return (
    <Modal title="Make a game" onClose={onClose} size="lg" footer={tab !== 'file' && <>
      <button type="button" className="btn" onClick={onClose}>Cancel</button>
      <button type="button" className="btn primary" disabled={!can || busy} onClick={go}>{busy ? <Spinner /> : <Icon name={tab === 'idea' ? 'sparkles' : 'plus'} size={15} />}{tab === 'idea' ? 'Make it' : 'Make the game'}</button>
    </>}>
      <div className="stack">
        <Tabs tabs={[{ id: 'template', label: 'Start from', icon: 'layout' }, { id: 'idea', label: 'From an idea', icon: 'sparkles' }, { id: 'quiz', label: 'Quiz', icon: 'help' }, { id: 'maze', label: 'From a maze', icon: 'maze' }, { id: 'file', label: 'Open a file', icon: 'upload' }]} value={tab} onChange={setTab} />
        {tab === 'template' && <>
          <div className="tpl-grid">
            {(tpls || []).map(t => (
              <button key={t.id} type="button" className={`tpl ${tpl === t.id ? 'on' : ''}`} onClick={() => setTpl(t.id)} aria-pressed={tpl === t.id}>
                <span className="tpl-emoji" aria-hidden>{t.emoji}</span>
                <span className="tpl-name">{t.name}</span>
                <span className="tpl-about">{t.about}</span>
              </button>
            ))}
            {!tpls && <Spinner />}
          </div>
          <div className="cols2 even">
            <Field label="Name"><input value={title} onChange={e => setTitle(e.target.value)} placeholder={chosen?.name || 'My game'} maxLength={120} /></Field>
            <Field label="Screen" help={tpl !== 'blank' ? 'Each template has a shape it plays best in.' : undefined}><Segmented size="sm" value={size || chosen?.size || settings?.default_size || 'wide'} onChange={setSize} options={SIZE_OPTS} /></Field>
          </div>
        </>}
        {tab === 'idea' && <>
          <Field label="What should the game be?" help="Pocket AI picks the kind of game, draws the levels, names things and writes the words. You can change everything afterwards.">
            <textarea rows={4} value={idea} onChange={e => setIdea(e.target.value)} placeholder="A frog who jumps over logs to get home, collecting flies. Three levels, the last one hard." autoFocus />
          </Field>
          {!aiOn && <p className="muted small">AI help is off (Settings → AI).</p>}
        </>}
        {tab === 'quiz' && <>
          <div className="row wrap">
            <input className="grow" value={topic} onChange={e => setTopic(e.target.value)} placeholder="A topic: dinosaurs, capitals of Europe…" />
            <button type="button" className="btn" disabled={!aiOn || !topic.trim() || busy} onClick={askQuestions}><Icon name="sparkles" size={14} />Questions from Pocket AI</button>
          </div>
          {questions.length > 0 ? <ol className="quiz-preview">{questions.map((q, i) => <li key={i}><b>{q.q}</b> <span className="ok">{q.answers[0]}</span> <span className="muted">· {q.answers.slice(1).join(' · ')}</span></li>)}</ol> : (
            <Field label="Or write your own" help="A question, then the right answer, then the wrong ones — one per line. A blank line between questions.">
              <textarea rows={8} value={typed} onChange={e => setTyped(e.target.value)} placeholder={'How many legs does a spider have?\nEight\nSix\nTen\n\nWhat do bees make?\nHoney\nMilk\nSilk'} />
            </Field>
          )}
          <p className="muted small">{quizQs.length ? `${quizQs.length} questions` : 'No questions yet.'}</p>
        </>}
        {tab === 'maze' && <>
          <p className="muted">One of your mazes from Pocket Puzzlecraft becomes a maze to walk through, with gems in its dead ends.</p>
          <AppHint slug="puzzlecraft" scope="puzzlecraft:read" />
          {mazes && (mazes.length ? <div className="pick-list short">{mazes.map(m => (
            <button key={m.id} type="button" className={`pick-row ${maze === m.id ? 'on' : ''}`} onClick={() => setMaze(m.id)}><Icon name={maze === m.id ? 'check-circle' : 'maze'} size={16} /><span className="grow">{m.name}</span></button>
          ))}</div> : <p className="muted small">No mazes in Pocket Puzzlecraft yet.</p>)}
        </>}
        {tab === 'file' && <>
          <p className="muted">A game saved from Pocket Game Builder (<code>.game.json</code>), with its pictures and sounds.</p>
          <label className="drop"><Icon name="upload" size={22} /><b>Choose a game file</b><input type="file" accept=".json,application/json" hidden onChange={e => pickFile(e.target.files?.[0])} /></label>
        </>}
      </div>
    </Modal>
  )
}

// ---------- sharing and telling people ----------

function QR({ text }: { text: string }) {
  const svg = useMemo(() => { const q = qrcode(0, 'M'); q.addData(text); q.make(); return q.createSvgTag({ cellSize: 4, margin: 2, scalable: true }) }, [text])
  return <div className="qr" dangerouslySetInnerHTML={{ __html: svg }} />
}

type SendTab = 'link' | 'chat' | 'email' | 'post' | 'calendar' | 'save'

export function ShareDialog({ game, onClose, onChange }: { game: Game; onClose: () => void; onChange: (g: Game) => void }) {
  const { toast } = useToast()
  const state = useAppState()
  const { settings } = useApp()
  const [tab, setTab] = useState<SendTab>('link')
  const [busy, setBusy] = useState('')
  const [text, setText] = useState('')
  const call = async (what: string, fn: () => Promise<{ open_url?: string; play_url?: string } | void>, done = 'Done') => {
    setBusy(what)
    try { const r = await fn(); toast(done, 'ok', openAction(r?.open_url)); if (!game.share_on) { onChange(await api.get<Game>(`${V1}/games/${game.id}`)) } } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') }
  }
  const setShare = async (on: boolean) => { setBusy('share'); try { onChange(await api.post<Game>(`${V1}/games/${game.id}/share`, { on })); announceChange() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy('') } }
  const patch = async (p: Partial<Game>) => { try { onChange(await api.put<Game>(`${V1}/games/${game.id}`, p)) } catch (e) { toast(errMsg(e), 'err') } }

  const [chats] = useList<{ id: string; name: string }>('/apps/chats', 'chats', tab === 'chat' && state('chat', 'chat:read').ok)
  const [chat, setChat] = useState(settings?.chat_to || '')
  const [q, setQ] = useState('')
  const [contacts, setContacts] = useState<{ name: string; email: string }[]>([])
  const [to, setTo] = useState<{ name: string; email: string }[]>([])
  useEffect(() => { if (tab !== 'email' || !state('contacts', 'contacts:read').ok || q.trim().length < 2) { setContacts([]); return } const t = setTimeout(() => api.get<{ contacts: { name: string; email: string }[] }>(`${V1}/apps/contacts?q=${encodeURIComponent(q)}`).then(r => setContacts(r.contacts)).catch(() => {}), 250); return () => clearTimeout(t) }, [q, tab]) // eslint-disable-line react-hooks/exhaustive-deps
  const [forums] = useList<{ slug: string; name: string; categories: { id: string; name: string }[] }>('/apps/forums', 'forums', tab === 'post' && state('forum', 'forum:read').ok)
  const [forum, setForum] = useState('')
  const [cat, setCat] = useState('')
  const [cals] = useList<{ id: string; name: string }>('/apps/calendars', 'calendars', tab === 'calendar' && state('calendar', 'calendar:read').ok)
  const [cal, setCal] = useState('')
  const tomorrow = new Date(Date.now() + 86400000)
  const [when, setWhen] = useState(`${tomorrow.toISOString().slice(0, 10)}T18:00`)
  const [mins, setMins] = useState(60)
  const embed = game.embed_url ? `<iframe src="${game.embed_url}" width="640" height="400" style="border:0" allow="autoplay; fullscreen" title="${game.title.replace(/"/g, '')}"></iframe>` : ''
  const forumOpts: SelectOption[] = (forums || []).map(f => ({ value: f.slug, label: f.name }))
  const catOpts: SelectOption[] = [{ value: '', label: 'No category' }, ...((forums || []).find(f => f.slug === forum)?.categories || []).map(c => ({ value: c.id, label: c.name }))]
  const sendNote = !game.share_on ? <p className="muted small"><Icon name="info" size={13} /> Sending turns the game's link on, so whoever gets it can play.</p> : null

  return (
    <Modal title={`Share “${game.title}”`} onClose={onClose} size="lg">
      <div className="stack">
        <Tabs tabs={[{ id: 'link', label: 'Link', icon: 'link' }, { id: 'chat', label: 'Chat', icon: 'message' }, { id: 'email', label: 'Email', icon: 'mail' }, { id: 'post', label: 'Post', icon: 'file' }, { id: 'calendar', label: 'Game night', icon: 'calendar' }, { id: 'save', label: 'Save', icon: 'download' }]} value={tab} onChange={setTab} />
        {tab === 'link' && <>
          <div className="row between">
            <div><b>Anyone with the link can play</b><p className="muted small">No account needed — on a phone or a computer.</p></div>
            <Switch checked={game.share_on} disabled={busy === 'share'} onChange={setShare} label="Share" />
          </div>
          {game.share_on && <>
            <div className="share-row">
              <div className="grow stack">
                <CopyField value={game.share_url} label="Link to play" />
                {game.changed_since_share && <div className="warn-box"><Icon name="info" size={15} /><span className="grow">You changed the game since you shared it. People still play the shared version.</span><button type="button" className="btn sm" onClick={() => setShare(true)} disabled={!!busy}>Share the new version</button></div>}
                <div className="row wrap">
                  <a className="btn sm" href={game.share_url} target="_blank" rel="noopener"><Icon name="external" size={14} />Open</a>
                  {typeof navigator.share === 'function' && <button type="button" className="btn sm" onClick={() => navigator.share({ title: game.title, url: game.share_url }).catch(() => {})}><Icon name="share" size={14} />Send…</button>}
                  <button type="button" className="btn sm" onClick={() => api.post<Game>(`${V1}/games/${game.id}/share/new-key`).then(onChange).catch(e => toast(errMsg(e), 'err'))} title="The old link stops working"><Icon name="refresh" size={14} />New link</button>
                </div>
              </div>
              <QR text={game.share_url} />
            </div>
            <div className="set-list">
              <div className="check-row"><Switch checked={game.scores_on} onChange={v => patch({ scores_on: v })} label="Best scores" /><span><b>Best scores</b><span className="muted small block-line">Players can put their name on a table of the best scores.</span></span></div>
              <div className="check-row"><Switch checked={game.remix_on} onChange={v => patch({ remix_on: v })} label="Copies" /><span><b>Let people take a copy</b><span className="muted small block-line">Someone with a Game Builder account here can make their own copy to change.</span></span></div>
            </div>
            {embed && <Field label="Put it on a web page" help="Paste this where a page takes HTML."><CopyField value={embed} /></Field>}
          </>}
        </>}
        {tab === 'chat' && <>
          <AppHint slug="chat" scope="chat:write" />
          <Field label="Send to"><Select ariaLabel="Chat" value={chat} onChange={setChat} placeholder="Notes to self" options={[{ value: '', label: 'Notes to self' }, ...(chats || []).map(c => ({ value: c.id, label: c.name }))]} /></Field>
          <Field label="Message"><input value={text} onChange={e => setText(e.target.value)} placeholder="Play my game!" /></Field>
          {sendNote}
          <div className="row"><button type="button" className="btn primary" disabled={!state('chat', 'chat:write').ok || !!busy} onClick={() => call('chat', () => api.post(`${V1}/games/${game.id}/chat`, { to: chat || 'me', text }), 'Sent to Pocket Chat')}>{busy === 'chat' ? <Spinner /> : <Icon name="send" size={14} />}Send</button></div>
        </>}
        {tab === 'email' && <>
          <AppHint slug="mail" scope="mail:send" />
          <Field label="To" help={state('contacts', 'contacts:read').ok ? 'Type a name from your contacts, or an email address and press Enter.' : 'Type an email address and press Enter.'}>
            <div className="stack">
              {to.length > 0 && <div className="chips">{to.map(t => <span key={t.email} className="chip on">{t.name || t.email}<button type="button" className="plain" aria-label="Remove" onClick={() => setTo(to.filter(x => x !== t))}><Icon name="x" size={12} /></button></span>)}</div>}
              <input value={q} onChange={e => setQ(e.target.value)} onKeyDown={e => { if (e.key === 'Enter' && q.includes('@')) { e.preventDefault(); setTo([...to, { name: '', email: q.trim() }]); setQ('') } }} placeholder="Name or email" />
              {contacts.length > 0 && <div className="pick-list short">{contacts.map(c => <button key={c.email} type="button" className="pick-row" onClick={() => { setTo([...to, c]); setQ(''); setContacts([]) }}><Icon name="user" size={15} /><span className="grow">{c.name}</span><span className="muted small">{c.email}</span></button>)}</div>}
            </div>
          </Field>
          <Field label="Message"><textarea rows={3} value={text} onChange={e => setText(e.target.value)} placeholder="I made a game and I'd like you to play it." /></Field>
          {sendNote}
          <div className="row"><button type="button" className="btn primary" disabled={!state('mail', 'mail:send').ok || !to.length || !!busy} onClick={() => call('email', () => api.post(`${V1}/games/${game.id}/email`, { to, text }), 'Email sent')}>{busy === 'email' ? <Spinner /> : <Icon name="send" size={14} />}Send</button></div>
        </>}
        {tab === 'post' && <>
          <Field label="A few words (optional)"><textarea rows={3} value={text} onChange={e => setText(e.target.value)} placeholder="What the game is, how you made it, what you want to hear back." /></Field>
          {sendNote}
          <div className="post-targets">
            <section>
              <h4><Icon name="file" size={15} />Pocket Blog</h4>
              <p className="muted small">A draft post with the game's picture and the link to play. You publish it in Pocket Blog.</p>
              <AppHint slug="blog" scope="blog:write" />
              <button type="button" className="btn sm" disabled={!state('blog', 'blog:write').ok || !!busy} onClick={() => call('blog', () => api.post(`${V1}/games/${game.id}/blog`, { text }), 'Draft post made in Pocket Blog')}>{busy === 'blog' ? <Spinner /> : <Icon name="edit" size={14} />}Write a draft post</button>
            </section>
            <section>
              <h4><Icon name="forum" size={15} />Pocket Forum</h4>
              <p className="muted small">A topic asking what people think.</p>
              <AppHint slug="forum" scope="forum:write" />
              {forums && forums.length > 0 && <div className="row wrap"><Select ariaLabel="Forum" value={forum} onChange={v => { setForum(v); setCat('') }} options={forumOpts} placeholder="Choose a forum" />{forum && <Select ariaLabel="Category" value={cat} onChange={setCat} options={catOpts} />}</div>}
              <button type="button" className="btn sm" disabled={!state('forum', 'forum:write').ok || !forum || !!busy} onClick={() => call('forum', () => api.post(`${V1}/games/${game.id}/forum`, { forum, category_id: cat || undefined, text }), 'Topic started in Pocket Forum')}>{busy === 'forum' ? <Spinner /> : <Icon name="message-plus" size={14} />}Start a topic</button>
            </section>
            <section>
              <h4><Icon name="home" size={15} />Pocket Family</h4>
              <p className="muted small">On the family board for two weeks, so everyone at home can play.</p>
              <AppHint slug="family" scope="family:write" />
              <button type="button" className="btn sm" disabled={!state('family', 'family:write').ok || !!busy} onClick={() => call('family', () => api.post(`${V1}/games/${game.id}/family`, { text }), 'On the family board')}>{busy === 'family' ? <Spinner /> : <Icon name="pin" size={14} />}Put it on the board</button>
            </section>
          </div>
        </>}
        {tab === 'calendar' && <>
          <p className="muted">Plan a game night or a play-test: an event in Pocket Calendar with the link to play.</p>
          <AppHint slug="calendar" scope="calendar:write" />
          <div className="cols2 even">
            <Field label="When"><input type="datetime-local" value={when} onChange={e => setWhen(e.target.value)} /></Field>
            <Field label="How long"><Segmented size="sm" value={mins} onChange={setMins} options={[{ value: 30, label: '30 min' }, { value: 60, label: '1 h' }, { value: 120, label: '2 h' }]} /></Field>
          </div>
          {cals && cals.length > 1 && <Field label="Calendar"><Select ariaLabel="Calendar" value={cal} onChange={setCal} placeholder="The usual one" options={[{ value: '', label: 'The usual one' }, ...cals.map(c => ({ value: c.id, label: c.name }))]} /></Field>}
          {sendNote}
          <div className="row"><button type="button" className="btn primary" disabled={!state('calendar', 'calendar:write').ok || !!busy} onClick={() => call('cal', () => api.post(`${V1}/games/${game.id}/playtest`, { start: when, minutes: mins, calendar_id: cal || undefined }), 'In your calendar')}>{busy === 'cal' ? <Spinner /> : <Icon name="calendar-plus" size={14} />}Put it in the calendar</button></div>
        </>}
        {tab === 'save' && <div className="stack">
          <div className="save-opt">
            <Icon name="file" size={20} />
            <div className="grow"><b>One HTML file</b><p className="muted small">The game with its pictures and sounds in one file: opens in any browser, from a USB stick or any web space, without this server.</p></div>
            <button type="button" className="btn sm" onClick={() => download(`${V1}/games/${game.id}/export.html`)}><Icon name="download" size={14} />Download</button>
          </div>
          <div className="save-opt">
            <Icon name="archive" size={20} />
            <div className="grow"><b>In Pocket Files</b><p className="muted small">The same HTML file, in “{settings?.files_folder || 'Games'}”.</p><AppHint slug="files" scope="files:write" /></div>
            <button type="button" className="btn sm" disabled={!state('files', 'files:write').ok || !!busy} onClick={() => call('files', () => api.post(`${V1}/games/${game.id}/files`), 'Saved in Pocket Files')}>{busy === 'files' ? <Spinner /> : <Icon name="upload" size={14} />}Save</button>
          </div>
          <div className="save-opt">
            <Icon name="note" size={20} />
            <div className="grow"><b>As a note</b><p className="muted small">The game in words in Pocket Notes: how to play, the levels, the things and every rule.</p><AppHint slug="notes" scope="notes:write" /></div>
            <button type="button" className="btn sm" disabled={!state('notes', 'notes:write').ok || !!busy} onClick={() => call('notes', () => api.post(`${V1}/games/${game.id}/note`), 'Written down in Pocket Notes')}>{busy === 'notes' ? <Spinner /> : <Icon name="edit" size={14} />}Write it down</button>
          </div>
          <div className="save-opt">
            <Icon name="package" size={20} />
            <div className="grow"><b>A game file</b><p className="muted small">To open in Game Builder on another box (<code>.game.json</code>).</p></div>
            <button type="button" className="btn sm" onClick={() => download(`${V1}/games/${game.id}/export.json`)}><Icon name="download" size={14} />Download</button>
          </div>
        </div>}
      </div>
    </Modal>
  )
}

// ---------- pictures and sounds from anywhere ----------

type Kind = 'image' | 'sound' | 'music'
type Source = 'library' | 'upload' | 'photos' | 'files' | 'draw' | 'photolab' | 'music' | 'voice'

interface Pick { id: string; name: string; thumb?: string; sub?: string }

export function PickerDialog({ kind, gameId, onClose, onPicked, title }: { kind: Kind; gameId?: number; onClose: () => void; onPicked: (a: Asset) => void; title?: string }) {
  const { toast } = useToast()
  const state = useAppState()
  const sources: { id: Source; label: string; icon: string; app?: string; scope?: string }[] = kind === 'image'
    ? [{ id: 'library', label: 'Yours', icon: 'palette' }, { id: 'upload', label: 'Upload', icon: 'upload' }, { id: 'photos', label: 'Photos', icon: 'image', app: 'photos', scope: 'photos:read' }, { id: 'draw', label: 'Draw', icon: 'pen', app: 'draw', scope: 'drawings:read' }, { id: 'photolab', label: 'Photo Lab', icon: 'sliders', app: 'photolab', scope: 'photolab:read' }, { id: 'files', label: 'Files', icon: 'archive', app: 'files', scope: 'files:read' }]
    : [{ id: 'library', label: 'Yours', icon: 'volume' }, { id: 'upload', label: 'Upload', icon: 'upload' }, ...(kind === 'music' ? [{ id: 'music' as Source, label: 'Music', icon: 'music', app: 'music', scope: 'music:play' }] : []), { id: 'voice', label: 'Voice', icon: 'mic', app: 'voice', scope: 'voice:read' }, { id: 'files', label: 'Files', icon: 'archive', app: 'files', scope: 'files:read' }]
  const [src, setSrc] = useState<Source>('library')
  const [q, setQ] = useState('')
  const [items, setItems] = useState<Pick[] | null>(null)
  const [lib, setLib] = useState<Asset[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [space, setSpace] = useState('home')
  const [path, setPath] = useState('/')
  const cur = sources.find(s => s.id === src)!
  const ok = !cur.app || state(cur.app, cur.scope).ok

  useEffect(() => {
    if (src === 'library') { api.get<{ assets: Asset[] }>(`${V1}/assets${kind === 'image' ? '?kind=image' : ''}`).then(r => setLib(r.assets.filter(a => kind === 'image' ? true : a.kind !== 'image'))).catch(() => setLib([])); return }
    if (src === 'upload' || !ok) { setItems(null); return }
    setItems(null)
    const t = setTimeout(() => {
      const enc = encodeURIComponent(q)
      const load = async (): Promise<Pick[]> => {
        switch (src) {
          case 'photos': return (await api.get<{ photos: { id: string; name: string; thumb: string; date: string }[] }>(`${V1}/apps/photos?q=${enc}`)).photos.map(p => ({ id: p.id, name: p.name, thumb: p.thumb, sub: p.date }))
          case 'draw': return (await api.get<{ drawings: { id: string; name: string; thumb: string }[] }>(`${V1}/apps/draw?q=${enc}`)).drawings.map(p => ({ id: p.id, name: p.name, thumb: p.thumb }))
          case 'photolab': return (await api.get<{ images: { id: string; name: string; thumb: string }[] }>(`${V1}/apps/photolab?q=${enc}`)).images.map(p => ({ id: p.id, name: p.name, thumb: p.thumb }))
          case 'music': return q.trim() ? (await api.get<{ songs: { id: string; title: string; artist: string; seconds: number }[] }>(`${V1}/apps/music?q=${enc}`)).songs.map(s => ({ id: s.id, name: s.title, sub: `${s.artist} · ${Math.floor(s.seconds / 60)}:${String(Math.round(s.seconds % 60)).padStart(2, '0')}` })) : []
          case 'voice': return (await api.get<{ memos: { id: string; name: string; date: string; seconds: number }[] }>(`${V1}/apps/voice?q=${enc}`)).memos.map(m => ({ id: m.id, name: m.name, sub: `${m.date} · ${Math.round(m.seconds)} s` }))
          case 'files': {
            const r = await api.get<{ entries: { name: string; path: string; kind: string; size: number }[] }>(`${V1}/apps/files?space=${encodeURIComponent(space)}&path=${encodeURIComponent(path)}`)
            const want = kind === 'image' ? 'image' : 'sound'
            return r.entries.filter(e => e.kind === 'folder' || e.kind === want).map(e => ({ id: e.path, name: e.name, sub: e.kind === 'folder' ? 'folder' : fmtBytes(e.size) }))
          }
        }
        return []
      }
      load().then(setItems).catch(e => { setItems([]); toast(errMsg(e), 'err') })
    }, q ? 300 : 0)
    return () => clearTimeout(t)
  }, [src, q, ok, path, space, kind]) // eslint-disable-line react-hooks/exhaustive-deps

  const take = async (p: Pick) => {
    if (src === 'files' && p.sub === 'folder') { setPath(p.id); return }
    setBusy(true)
    try {
      const body = src === 'files' ? { paths: [p.id], space, game_id: gameId, kind } : { ids: [p.id], game_id: gameId }
      const route = src === 'photolab' ? 'photolab' : src
      const r = await api.post<{ assets: Asset[] }>(`${V1}/apps/${route}/import`, body)
      if (r.assets[0]) { announceChange(); onPicked(r.assets[0]); onClose() }
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const upload = async (files: FileList | null) => {
    if (!files?.length) return
    setBusy(true)
    try {
      const r = await uploadFiles<{ assets: Asset[]; errors: string[] }>(`${V1}/assets`, [...files], { kind, game_id: String(gameId || '') })
      if (r.errors?.length) toast(r.errors.join('; '), 'err')
      if (r.assets[0]) { announceChange(); onPicked(r.assets[0]); onClose() }
    } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  const accept = kind === 'image' ? 'image/png,image/jpeg,image/webp,image/gif,image/svg+xml' : 'audio/*,.mp3,.wav,.ogg,.m4a,.flac,.webm'
  return (
    <Modal title={title || (kind === 'image' ? 'Choose a picture' : kind === 'music' ? 'Choose music' : 'Choose a sound')} onClose={onClose} size="lg">
      <div className="stack">
        <div className="scroll-x"><Segmented size="sm" value={src} onChange={s => { setSrc(s); setQ('') }} options={sources.map(s => ({ value: s.id, label: <><Icon name={s.icon} size={13} />{s.label}</> }))} /></div>
        {cur.app && <AppHint slug={cur.app} scope={cur.scope} />}
        {src === 'upload' && <label className="drop"><Icon name="upload" size={22} /><b>{kind === 'image' ? 'PNG, JPEG, WebP, GIF or SVG' : 'MP3, WAV, OGG, M4A or FLAC'}</b><span className="muted small">{kind === 'image' ? 'A picture strip with frames side by side works as an animation.' : 'Up to 64 MB.'}</span><input type="file" accept={accept} multiple hidden onChange={e => upload(e.target.files)} /></label>}
        {src === 'library' && (lib === null ? <Spinner /> : lib.length === 0 ? <p className="muted">Nothing here yet — upload one, or draw one in Pictures & sounds.</p> : kind === 'image'
          ? <div className="pick-grid">{lib.map(a => <button key={a.id} type="button" className="pick-tile" onClick={() => { onPicked(a); onClose() }} title={a.name}><img src={a.url} alt="" className={a.width <= 128 ? 'pixelated' : ''} /><span className="ellipsis">{a.name}</span></button>)}</div>
          : <div className="pick-list">{lib.map(a => <button key={a.id} type="button" className="pick-row" onClick={() => { onPicked(a); onClose() }}><Icon name={a.kind === 'music' ? 'music' : 'volume'} size={15} /><span className="grow">{a.name}</span><span className="muted small">{a.kind === 'music' ? 'music' : 'sound'}</span></button>)}</div>)}
        {src !== 'library' && src !== 'upload' && ok && <>
          {src !== 'files' ? <div className="search-box"><Icon name="search" size={15} /><input value={q} onChange={e => setQ(e.target.value)} placeholder={src === 'music' ? 'Search your music' : 'Search'} autoFocus />{busy && <Spinner />}</div>
            : <div className="row"><button type="button" className="btn sm" disabled={path === '/'} onClick={() => setPath(path.replace(/\/[^/]+\/?$/, '') || '/')}><Icon name="arrow-up" size={14} />Up</button><span className="muted small ellipsis grow">{path}</span><button type="button" className="btn sm" onClick={() => { setSpace('home'); setPath('/') }}>Home</button></div>}
          {items === null ? <Spinner /> : items.length === 0 ? <p className="muted">{src === 'music' && !q ? 'Type to search your music.' : 'Nothing found.'}</p> : items.some(i => i.thumb)
            ? <div className="pick-grid">{items.map(p => <button key={p.id} type="button" className="pick-tile" disabled={busy} onClick={() => take(p)} title={p.name}><img src={p.thumb} alt="" loading="lazy" /><span className="ellipsis">{p.name}</span></button>)}</div>
            : <div className="pick-list">{items.map(p => <button key={p.id} type="button" className="pick-row" disabled={busy} onClick={() => take(p)}><Icon name={p.sub === 'folder' ? 'archive' : kind === 'image' ? 'image' : 'volume'} size={15} /><span className="grow">{p.name}</span>{p.sub && <span className="muted small">{p.sub}</span>}</button>)}</div>}
        </>}
      </div>
    </Modal>
  )
}

// ---------- a copy in another language ----------

const LANGS = [['sr', 'Serbian'], ['hr', 'Croatian'], ['bs', 'Bosnian'], ['de', 'German'], ['en', 'English'], ['fr', 'French'], ['es', 'Spanish'], ['it', 'Italian'], ['nl', 'Dutch'], ['pt', 'Portuguese'], ['sl', 'Slovenian'], ['pl', 'Polish'], ['tr', 'Turkish'], ['uk', 'Ukrainian']]

export function TranslateDialog({ game, onClose, onMade }: { game: Game; onClose: () => void; onMade: (g: Game) => void }) {
  const { toast } = useToast()
  const state = useAppState()
  const [to, setTo] = useState('de')
  const [busy, setBusy] = useState(false)
  const go = async () => {
    setBusy(true)
    try { const g = await api.post<Game>(`${V1}/games/${game.id}/translate`, { to, name: LANGS.find(l => l[0] === to)?.[1] }); announceChange(); toast('Translated copy made'); onMade(g); onClose() } catch (e) { toast(errMsg(e), 'err') } finally { setBusy(false) }
  }
  return (
    <Modal title="A copy in another language" onClose={onClose} size="sm" footer={<><button type="button" className="btn" onClick={onClose}>Cancel</button><button type="button" className="btn primary" disabled={!state('translate', 'translate:run').ok || busy} onClick={go}>{busy ? <Spinner /> : <Icon name="globe" size={15} />}Translate</button></>}>
      <div className="stack">
        <p className="muted">Pocket Translate translates every word a player reads — the title, the start and end screens, the score line, texts and messages — into a new copy. This game stays as it is.</p>
        <AppHint slug="translate" scope="translate:run" />
        <Field label="Language"><Select ariaLabel="Language" value={to} onChange={setTo} options={LANGS.map(([v, l]) => ({ value: v, label: l }))} /></Field>
      </div>
    </Modal>
  )
}
