// Roadmap: as sprints lado a lado (passadas, ativa e planejadas), o backlog
// restante com a projeção de quantas sprints ele ainda pede, e o avanço de
// cada épico.

import { useMemo } from 'react'
import type { Plan, WorkItem } from '../../lib/types'
import { Badge } from '../ui'
import { ITEM_TYPE, SPRINT_STATUS, formatDate, hoursLabel, isWork } from './planMeta'

export function Roadmap({ plan, onOpen }: { plan: Plan; onOpen: (item: WorkItem) => void }) {
  const work = plan.items.filter((i) => isWork(i) && !i.archived)
  const sprints = [...plan.sprints].sort((a, b) => a.number - b.number)
  const rest = work.filter((i) => !i.sprint && i.status !== 'completed')
  const restHours = rest.reduce((s, i) => s + (i.estimate_h || 4), 0)
  const caps = sprints.map((s) => s.capacity_h ?? 0).filter((c) => c > 0)
  const avgCap = caps.length ? caps.reduce((a, b) => a + b, 0) / caps.length : 0
  const moreSprints = avgCap > 0 ? Math.ceil(restHours / avgCap) : 0

  const epics = useMemo(() => {
    const byParent = new Map<string, WorkItem[]>()
    for (const it of plan.items) {
      if (it.parent && !it.archived) byParent.set(it.parent, [...(byParent.get(it.parent) ?? []), it])
    }
    const descendants = (id: string): WorkItem[] =>
      (byParent.get(id) ?? []).flatMap((c) => (c.type === 'story' ? [c, ...descendants(c.id)] : [c]))
    return plan.items.filter((i) => i.type === 'epic' && !i.archived).map((ep) => {
      const tasks = descendants(ep.id).filter(isWork)
      const done = tasks.filter((t) => t.status === 'completed').length
      const sprintsOf = [...new Set(tasks.map((t) => t.sprint).filter(Boolean))].sort() as number[]
      return { ep, tasks: tasks.length, done, sprints: sprintsOf }
    })
  }, [plan.items])

  return (
    <div className="space-y-4 p-3">
      <div className="flex gap-2 overflow-x-auto pb-1">
        {sprints.map((s) => {
          const items = work.filter((i) => i.sprint === s.number)
          const done = items.filter((i) => i.status === 'completed').length
          const pct = items.length ? Math.round((done / items.length) * 100) : 0
          const load = items.reduce((sum, i) => sum + (i.estimate_h ?? 0), 0)
          const color = s.status === 'active' ? '#10b981' : s.status === 'closed' ? '#64748b' : '#3b82f6'
          return (
            <div key={s.number} className="w-56 shrink-0 rounded-lg border bg-card p-3" style={{ borderTop: `3px solid ${color}` }}>
              <div className="flex items-center justify-between">
                <span className="text-[13px] font-semibold">{s.name}</span>
                <Badge color={color}>{SPRINT_STATUS[s.status]}</Badge>
              </div>
              <p className="mt-0.5 text-[11.5px] text-muted-foreground">{formatDate(s.start)} – {formatDate(s.end)}</p>
              <p className="mt-2 line-clamp-2 min-h-8 text-[12px]">{s.goal || <span className="text-muted-foreground">Sem meta</span>}</p>
              <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-emerald-500" style={{ width: `${pct}%` }} /></div>
              <p className="mt-1 text-[11px] tabular-nums text-muted-foreground">{done}/{items.length} itens · {hoursLabel(load)}{s.capacity_h ? ` de ${hoursLabel(s.capacity_h)}` : ''}</p>
            </div>
          )
        })}
        <div className="w-56 shrink-0 rounded-lg border border-dashed bg-card/50 p-3">
          <span className="text-[13px] font-semibold">Backlog restante</span>
          <p className="mt-2 text-[12px]">{rest.length} item(ns) · {hoursLabel(restHours)}</p>
          <p className="mt-1 text-[11.5px] text-muted-foreground">
            {moreSprints > 0 ? `≈ ${moreSprints} sprint(s) na capacidade média de ${hoursLabel(Math.round(avgCap))}` : 'Planeje uma sprint para projetar o prazo.'}
          </p>
        </div>
      </div>

      <section className="rounded-lg border bg-card">
        <header className="border-b bg-panel-header/60 px-3 py-2 text-[12.5px] font-semibold">Épicos</header>
        {epics.length === 0 && <p className="px-3 py-4 text-center text-[12px] text-muted-foreground">Os épicos vêm dos casos de uso do projeto.</p>}
        <ol>
          {epics.map(({ ep, tasks, done, sprints: sp }) => {
            const pct = tasks ? Math.round((done / tasks) * 100) : 0
            const Icon = ITEM_TYPE.epic.icon
            return (
              <li key={ep.id} onClick={() => onOpen(ep)} className="flex cursor-pointer items-center gap-2 border-b px-3 py-2 text-[12.5px] last:border-b-0 hover:bg-muted">
                <Icon size={13} style={{ color: ITEM_TYPE.epic.color }} />
                <code className="w-28 shrink-0 font-mono text-[11px] font-semibold">{ep.id}</code>
                <span className="min-w-0 flex-1 truncate">{ep.title}</span>
                <span className="text-[11px] text-muted-foreground">{sp.length ? sp.map((n) => `S${String(n).padStart(2, '0')}`).join(' · ') : 'sem sprint'}</span>
                <span className="w-20 text-right text-[11px] tabular-nums text-muted-foreground">{done}/{tasks} tarefas</span>
                <div className="h-1.5 w-28 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-emerald-500" style={{ width: `${pct}%` }} /></div>
              </li>
            )
          })}
        </ol>
      </section>
    </div>
  )
}
