// Backlog em seções por sprint (ativa, planejadas, sem sprint), na ordem do
// rank. Arrastar uma linha reposiciona o item entre os vizinhos e muda a
// sprint dele; só o arquivo do item muda.

import { GripVertical, Lock } from 'lucide-react'
import { useMemo, useState, type DragEvent } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { Plan, Sprint, WorkItem } from '../../lib/types'
import { Badge, useToast } from '../ui'
import {
  ITEM_TYPE, PRIORITY, SPRINT_STATUS, STATUS, blockers, displayName, formatDate, hoursLabel, initials, isWork,
} from './planMeta'

export interface BacklogFilters {
  text: string
  type: string
  hideDone: boolean
  structure: boolean
}

export function BacklogList({ plan, filters, onOpen }: {
  plan: Plan
  filters: BacklogFilters
  onOpen: (item: WorkItem) => void
}) {
  const toast = useToast()
  const [dragging, setDragging] = useState<string | null>(null)
  const [over, setOver] = useState<string | null>(null)
  const byId = useMemo(() => new Map(plan.items.map((i) => [i.id, i])), [plan.items])

  const visible = useMemo(() => {
    const q = filters.text.trim().toLowerCase()
    return plan.items.filter((it) => {
      if (it.archived) return false
      if (!isWork(it)) return false
      if (filters.type && it.type !== filters.type) return false
      if (filters.hideDone && it.status === 'completed') return false
      if (q && !`${it.id} ${it.title} ${it.component ?? ''} ${it.assignee ?? ''}`.toLowerCase().includes(q)) return false
      return true
    })
  }, [plan.items, filters])

  const sections: { key: string; sprint?: Sprint; items: WorkItem[] }[] = useMemo(() => {
    const open = plan.sprints.filter((s) => s.status !== 'closed')
      .sort((a, b) => (a.status === 'active' ? -1 : b.status === 'active' ? 1 : a.number - b.number))
    return [
      ...open.map((s) => ({ key: `s${s.number}`, sprint: s, items: visible.filter((i) => i.sprint === s.number) })),
      { key: 'backlog', items: visible.filter((i) => !i.sprint || !open.some((s) => s.number === i.sprint)) },
    ]
  }, [plan.sprints, visible])

  const move = async (id: string, sprint: number, before?: WorkItem, after?: WorkItem) => {
    try {
      await api.moveItem(id, { sprint, before: before?.id, after: after?.id })
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const dropOnRow = (e: DragEvent, section: WorkItem[], index: number, sprint: number) => {
    e.preventDefault()
    e.stopPropagation()
    const id = e.dataTransfer.getData('text/plain')
    setOver(null)
    setDragging(null)
    if (!id || id === section[index]?.id) return
    const target = section[index]
    const prev = section.slice(0, index).reverse().find((i) => i.id !== id)
    void move(id, sprint, target, prev)
  }

  const dropOnSection = (e: DragEvent, section: WorkItem[], sprint: number) => {
    e.preventDefault()
    const id = e.dataTransfer.getData('text/plain')
    setOver(null)
    setDragging(null)
    if (!id) return
    const last = [...section].reverse().find((i) => i.id !== id)
    void move(id, sprint, undefined, last)
  }

  const structure = plan.items.filter((i) => !i.archived && (i.type === 'epic' || i.type === 'story'))

  return (
    <div className="space-y-4 p-3">
      {filters.structure && <StructureList items={structure} all={plan.items} onOpen={onOpen} />}
      {sections.map((sec) => {
        const n = sec.sprint?.number ?? 0
        const load = sec.items.reduce((sum, i) => sum + (i.estimate_h ?? 0), 0)
        const cap = sec.sprint?.capacity_h ?? 0
        return (
          <section key={sec.key}
            className={cn('rounded-lg border bg-card', over === sec.key && 'ring-2 ring-primary/40')}
            onDragOver={(e) => { e.preventDefault(); setOver(sec.key) }}
            onDragLeave={() => setOver((cur) => (cur === sec.key ? null : cur))}
            onDrop={(e) => dropOnSection(e, sec.items, n)}>
            <header className="flex flex-wrap items-center gap-x-3 gap-y-1 border-b bg-panel-header/60 px-3 py-2">
              <h3 className="text-[12.5px] font-semibold">{sec.sprint ? sec.sprint.name : 'Backlog (sem sprint)'}</h3>
              {sec.sprint && <Badge color={sec.sprint.status === 'active' ? '#10b981' : '#64748b'}>{SPRINT_STATUS[sec.sprint.status]}</Badge>}
              {sec.sprint?.goal && <span className="truncate text-[12px] text-muted-foreground">{sec.sprint.goal}</span>}
              <span className="ml-auto text-[11.5px] tabular-nums text-muted-foreground">
                {sec.sprint && `${formatDate(sec.sprint.start)} – ${formatDate(sec.sprint.end)} · `}
                {sec.items.length} item(ns) · {hoursLabel(load)}{cap > 0 && ` de ${hoursLabel(cap)}`}
              </span>
              {cap > 0 && (
                <div className="h-1.5 w-28 overflow-hidden rounded-full bg-muted" title={`Carga ${Math.round((load / cap) * 100)}% da capacidade`}>
                  <div className={cn('h-full rounded-full', load > cap ? 'bg-destructive' : 'bg-primary')} style={{ width: `${Math.min(100, (load / cap) * 100)}%` }} />
                </div>
              )}
            </header>
            {sec.items.length === 0 && (
              <p className="px-3 py-4 text-center text-[12px] text-muted-foreground">
                {sec.sprint ? 'Arraste itens do backlog para cá.' : 'Nenhum item sem sprint.'}
              </p>
            )}
            <ol>
              {sec.items.map((it, index) => {
                const Icon = ITEM_TYPE[it.type].icon
                const waiting = it.status === 'pending' ? blockers(it, byId) : []
                const status = STATUS[it.status]
                return (
                  <li key={it.id} draggable
                    onDragStart={(e) => { e.dataTransfer.setData('text/plain', it.id); e.dataTransfer.effectAllowed = 'move'; setDragging(it.id) }}
                    onDragEnd={() => { setDragging(null); setOver(null) }}
                    onDragOver={(e) => { e.preventDefault(); e.stopPropagation(); setOver(it.id) }}
                    onDrop={(e) => dropOnRow(e, sec.items, index, n)}
                    onClick={() => onOpen(it)}
                    className={cn(
                      'group flex h-8 cursor-pointer items-center gap-2 border-b px-2 text-[12.5px] last:border-b-0 hover:bg-muted',
                      dragging === it.id && 'opacity-40',
                      over === it.id && dragging !== it.id && 'border-t-2 border-t-primary',
                      it.status === 'completed' && 'text-muted-foreground',
                    )}>
                    <GripVertical size={13} className="shrink-0 cursor-grab text-muted-foreground/60 group-hover:text-muted-foreground" />
                    <Icon size={13} className="shrink-0" style={{ color: ITEM_TYPE[it.type].color }} />
                    <code className="w-40 shrink-0 truncate font-mono text-[11px] font-semibold">{it.id}</code>
                    <span className={cn('min-w-0 flex-1 truncate', it.status === 'completed' && 'line-through')}>{it.title}</span>
                    {waiting.length > 0 && <span title={`Aguarda ${waiting.join(', ')}`}><Lock size={12} className="shrink-0 text-amber-500" /></span>}
                    {it.stale && <Badge color="#f59e0b">parada</Badge>}
                    {it.component && <span className="hidden max-w-32 truncate text-[11px] text-muted-foreground @2xl:inline">{it.component}</span>}
                    {it.priority && <Badge color={PRIORITY[it.priority].color}>{PRIORITY[it.priority].short}</Badge>}
                    <span className="w-12 shrink-0 text-right tabular-nums text-[11.5px] text-muted-foreground">{hoursLabel(it.estimate_h)}</span>
                    <span className="flex w-6 shrink-0 justify-center">
                      {it.assignee && (
                        <span title={`${displayName(it.assignee)}${it.agent ? ` (${it.agent})` : ''}`}
                          className="flex size-5 items-center justify-center rounded-full bg-accent text-[9.5px] font-bold">{initials(it.assignee)}</span>
                      )}
                    </span>
                    <span className="w-28 shrink-0"><Badge className="whitespace-nowrap" color={status.color}>{status.label}</Badge></span>
                  </li>
                )
              })}
            </ol>
          </section>
        )
      })}
    </div>
  )
}

/** Épicos e histórias com o progresso das tarefas ligadas a cada um. */
function StructureList({ items, all, onOpen }: { items: WorkItem[]; all: WorkItem[]; onOpen: (item: WorkItem) => void }) {
  const children = (parent: WorkItem): WorkItem[] => {
    const direct = all.filter((i) => i.parent === parent.id && !i.archived)
    return direct.flatMap((c) => (c.type === 'story' ? [c, ...children(c)] : [c]))
  }
  const epics = items.filter((i) => i.type === 'epic')
  const orphans = items.filter((i) => i.type === 'story' && !i.parent)
  const row = (it: WorkItem, depth: number) => {
    const work = children(it).filter(isWork)
    const done = work.filter((w) => w.status === 'completed').length
    const pct = work.length ? Math.round((done / work.length) * 100) : 0
    const Icon = ITEM_TYPE[it.type].icon
    return (
      <li key={it.id} onClick={() => onOpen(it)} className="flex h-8 cursor-pointer items-center gap-2 border-b px-2 text-[12.5px] last:border-b-0 hover:bg-muted"
        style={{ paddingLeft: 8 + depth * 18 }}>
        <Icon size={13} style={{ color: ITEM_TYPE[it.type].color }} />
        <code className="w-28 shrink-0 font-mono text-[11px] font-semibold">{it.id}</code>
        <span className="min-w-0 flex-1 truncate">{it.title}</span>
        <span className="text-[11px] tabular-nums text-muted-foreground">{done}/{work.length} tarefas</span>
        <div className="h-1.5 w-24 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-emerald-500" style={{ width: `${pct}%` }} /></div>
      </li>
    )
  }
  return (
    <section className="rounded-lg border bg-card">
      <header className="border-b bg-panel-header/60 px-3 py-2 text-[12.5px] font-semibold">Épicos e histórias</header>
      <ol>
        {epics.map((ep) => [row(ep, 0), ...items.filter((s) => s.parent === ep.id).map((s) => row(s, 1))])}
        {orphans.map((s) => row(s, 0))}
        {items.length === 0 && <li className="px-3 py-4 text-center text-[12px] text-muted-foreground">Nenhum épico ou história. Eles vêm dos casos de uso e requisitos funcionais.</li>}
      </ol>
    </section>
  )
}
