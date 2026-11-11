// Plays: how often your games were played, and which.
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, V1 } from '../lib/api'
import { Card, Loading, Segmented } from '../components/ui'
import { Bars, HBars } from '../components/Chart'
import { fmtScore } from '../components/bits'

interface Stats { days: { day: string; plays: number }[]; plays: number; top: { id: number; title: string; plays: number; best: number }[]; games: number; assets: Record<string, number> }

export default function StatsPage() {
  const [days, setDays] = useState(30)
  const [s, setS] = useState<Stats | null>(null)
  useEffect(() => { api.get<Stats>(`${V1}/stats?days=${days}`).then(setS).catch(() => {}) }, [days])
  if (!s) return <Loading />
  const step = Math.max(1, Math.ceil(s.days.length / 30))
  const buckets: { label: string; value: number; hint: string }[] = []
  for (let i = 0; i < s.days.length; i += step) {
    const part = s.days.slice(i, i + step)
    buckets.push({ label: part[0].day.slice(5).replace('-', '.'), value: part.reduce((a, d) => a + d.plays, 0), hint: step > 1 ? `${part[0].day} – ${part[part.length - 1].day}` : part[0].day })
  }
  return (
    <div className="page">
      <div className="page-head"><div className="grow"><h1 className="page-title">Plays</h1><p className="muted">Plays by other people on your shared games.</p></div>
        <Segmented size="sm" value={days} onChange={setDays} options={[{ value: 7, label: 'Week' }, { value: 30, label: 'Month' }, { value: 90, label: '3 months' }, { value: 365, label: 'Year' }]} /></div>
      <div className="stats-row">
        <div className="stat"><div className="stat-l">Plays</div><div className="stat-v tnum">{s.plays.toLocaleString()}</div></div>
        <div className="stat"><div className="stat-l">Games</div><div className="stat-v tnum">{s.games}</div></div>
        <div className="stat"><div className="stat-l">Pictures</div><div className="stat-v tnum">{s.assets.image || 0}</div></div>
        <div className="stat"><div className="stat-l">Sounds & music</div><div className="stat-v tnum">{(s.assets.sound || 0) + (s.assets.music || 0)}</div></div>
      </div>
      <Card title="Plays per day"><Bars height={150} items={buckets} /></Card>
      <Card title="Most played">
        {s.top.length === 0 ? <p className="muted small">Share a game to see it here.</p> : <>
          <HBars items={s.top.map(t => ({ label: t.title, value: t.plays, hint: t.best ? `best ${fmtScore(t.best)}` : undefined }))} />
          <p className="small mt">{s.top.map((t, i) => <span key={t.id}>{i > 0 && ' · '}<Link to={`/games/${t.id}`}>{t.title}</Link></span>)}</p>
        </>}
      </Card>
    </div>
  )
}
