// Quadro kanban da sprint: uma coluna por estado do ciclo de vida. Arrastar
// um cartão dispara a ação certa — reservar ao entrar em andamento, concluir
// (com os checks das skills) ao ir para revisão ou concluída, liberar ao
// voltar para "a fazer".

import { Bot, CalendarRange, Flag, Lock, Play, Square } from 'lucide-react'
import { useMemo, useState, type DragEvent } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { Plan, Sprint, TaskStatus, WorkItem } from '../../lib/types'
import { Badge, Button, EmptyState, Select, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'
import type { ItemAction } from './ItemDialog'
import {
  BOARD_COLUMNS, ITEM_TYPE, SPRINT_STATUS, STATUS, blockers, displayName, formatDate, hoursLabel, initials, isWork, samePerson,
} from './planMeta'

export function SprintBoard({ plan, me, sprintNumber, onSprint, onOpen, onAction, onPlan, onClose }: {
  plan: Plan
  me?: string
  sprintNumber: number
  onSprint: (n: number) => void
  onOpen: (item: WorkItem) => void
  onAction: (action: ItemAction, item: WorkItem) => void
  onPlan: (sprint?: Sprint) => void
  onClose: (sprint: Sprint) => void
}) {
  const toast = useToast()
  const confirm = useConfirm()
  const [over, setOver] = useState<TaskStatus | null>(null)
  const byId = useMemo(() => new Map(plan.items.map((i) => [i.id, i])), [plan.items])
  const sprint = plan.sprints.find((s) => s.number === sprintNumber)

  if (!sprint) {
    return (
      <EmptyState icon={CalendarRange} title="Nenhuma sprint"
        description="Planeje a primeira sprint: o Studio propõe os itens que cabem na capacidade do time, na ordem do backlog e respeitando as dependências."
        action={<Button variant="primary" onClick={() => onPlan()}>Planejar sprint</Button>} />
    )
  }

  const items = plan.items.filter((i) => i.sprint === sprint.number && isWork(i) && !i.archived)
  const done = items.filter((i) => i.status === 'completed').length
  const load = items.reduce((s, i) => s + (i.estimate_h ?? 0), 0)
  const cap = sprint.capacity_h ?? 0
  const pct = items.length ? Math.round((done / items.length) * 100) : 0

  const start = async () => {
    try {
      await api.startSprint(sprint.number)
      toast('success', `${sprint.name} iniciada`)
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const drop = async (e: DragEvent, to: TaskStatus) => {
    e.preventDefault()
    setOver(null)
    const item = byId.get(e.dataTransfer.getData('text/plain'))
    if (!item || item.status === to) return
    try {
      switch (to) {
        case 'in_progress':
          if (!item.assignee || !samePerson(item.assignee, me)) { onAction('claim', item); return }
          await api.updateItem(item.id, { status: to })
          break
        case 'review':
        case 'completed':
          onAction('complete', item)
          return
        case 'pending':
          if (item.assignee) {
            const other = !samePerson(item.assignee, me)
            if (other && !(await confirm({
              title: `Liberar ${item.id}?`, description: `A tarefa está com ${displayName(item.assignee)}. O checkpoint fica para quem assumir.`,
              confirmLabel: 'Liberar', destructive: true,
            }))) return
            await api.releaseItem(item.id, { force: other })
          } else {
            await api.updateItem(item.id, { status: to })
          }
          break
        case 'blocked':
          await api.updateItem(item.id, { status: to })
          toast('info', `${item.id} bloqueada — registre o impedimento nas notas`)
          break
      }
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const choosable = plan.sprints.filter((s) => s.status !== 'closed' || s.number === sprint.number)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-b px-3 py-2">
        <Select value={String(sprint.number)} onChange={(e) => onSprint(Number(e.target.value))} className="h-7 w-44 text-[12px]">
          {choosable.map((s) => <option key={s.number} value={s.number}>{s.name} · {SPRINT_STATUS[s.status]}</option>)}
          {plan.sprints.filter((s) => s.status === 'closed' && s.number !== sprint.number).map((s) =>
            <option key={s.number} value={s.number}>{s.name} · encerrada</option>)}
        </Select>
        <div className="min-w-0 flex-1">
          <p className="truncate text-[12.5px] font-medium">{sprint.goal || <span className="text-muted-foreground">Sem meta — defina em Planejar</span>}</p>
          <p className="text-[11.5px] text-muted-foreground">{formatDate(sprint.start)} – {formatDate(sprint.end)} · {done}/{items.length} concluídas · carga {hoursLabel(load)}{cap > 0 && ` de ${hoursLabel(cap)}`}</p>
        </div>
        <div className="h-2 w-40 overflow-hidden rounded-full bg-muted" title={`${pct}% concluído`}>
          <div className="h-full rounded-full bg-emerald-500 transition-all" style={{ width: `${pct}%` }} />
        </div>
        {sprint.status !== 'closed' && <Button size="sm" onClick={() => onPlan(sprint)}>Planejar itens</Button>}
        {sprint.status === 'planned' && <Button size="sm" variant="primary" icon={Play} onClick={() => void start()}>Iniciar</Button>}
        {sprint.status === 'active' && <Button size="sm" variant="outline" icon={Flag} onClick={() => onClose(sprint)}>Encerrar</Button>}
      </div>

      <div className="grid min-h-0 flex-1 gap-2 overflow-x-auto p-2" style={{ gridTemplateColumns: `repeat(${BOARD_COLUMNS.length}, minmax(13rem, 1fr))` }}>
        {BOARD_COLUMNS.map((col) => {
          const cards = items.filter((i) => i.status === col)
          const meta = STATUS[col]
          return (
            <div key={col}
              onDragOver={(e) => { e.preventDefault(); setOver(col) }}
              onDragLeave={() => setOver((cur) => (cur === col ? null : cur))}
              onDrop={(e) => void drop(e, col)}
              className={cn('flex min-h-0 flex-col rounded-lg border bg-muted/40', over === col && 'ring-2 ring-primary/40')}>
              <div className="flex items-center gap-2 border-b px-2.5 py-1.5">
                <span className="size-2 rounded-full" style={{ backgroundColor: meta.color }} />
                <span className="text-[12px] font-semibold">{meta.label}</span>
                <span className="ml-auto text-[11px] tabular-nums text-muted-foreground">{cards.length}</span>
              </div>
              <div className="min-h-0 flex-1 space-y-1.5 overflow-y-auto p-1.5">
                {cards.map((it) => <Card key={it.id} item={it} byId={byId} me={me} onOpen={onOpen} />)}
                {cards.length === 0 && <div className="py-6 text-center text-[11px] text-muted-foreground/70">Solte aqui</div>}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}

function Card({ item, byId, me, onOpen }: { item: WorkItem; byId: Map<string, WorkItem>; me?: string; onOpen: (i: WorkItem) => void }) {
  const Icon = ITEM_TYPE[item.type].icon
  const waiting = item.status === 'pending' ? blockers(item, byId) : []
  const criteria = item.acceptance ?? []
  const doneCriteria = criteria.filter((c) => c.done).length
  const mine = samePerson(item.assignee, me)
  return (
    <div draggable onDragStart={(e) => { e.dataTransfer.setData('text/plain', item.id); e.dataTransfer.effectAllowed = 'move' }}
      onClick={() => onOpen(item)}
      className={cn('cursor-grab rounded-md border bg-card p-2 text-[12px] shadow-xs hover:border-foreground/30 active:cursor-grabbing',
        mine && item.status === 'in_progress' && 'border-l-2 border-l-amber-500')}>
      <div className="flex items-center gap-1.5">
        <Icon size={12} style={{ color: ITEM_TYPE[item.type].color }} />
        <code className="min-w-0 flex-1 truncate font-mono text-[10.5px] font-semibold text-muted-foreground">{item.id}</code>
        {waiting.length > 0 && <span title={`Aguarda ${waiting.join(', ')}`}><Lock size={11} className="text-amber-500" /></span>}
        {item.agent && <span title={`Agente: ${item.agent}`}><Bot size={11} className="text-violet-500" /></span>}
        {item.assignee && (
          <span title={displayName(item.assignee)} className="flex size-5 items-center justify-center rounded-full bg-accent text-[9px] font-bold">{initials(item.assignee)}</span>
        )}
      </div>
      <p className="mt-1 line-clamp-2 leading-snug">{item.title}</p>
      {item.handoff?.next_step && item.status !== 'completed' && (
        <p className="mt-1 line-clamp-2 text-[11px] italic text-muted-foreground">→ {item.handoff.next_step}</p>
      )}
      <div className="mt-1.5 flex flex-wrap items-center gap-1">
        {item.component && <Badge>{item.component}</Badge>}
        {item.stale && <Badge color="#f59e0b">parada</Badge>}
        {criteria.length > 0 && (
          <span className="inline-flex items-center gap-0.5 text-[10.5px] text-muted-foreground"><Square size={9} />{doneCriteria}/{criteria.length}</span>
        )}
        <span className="ml-auto text-[10.5px] tabular-nums text-muted-foreground">{hoursLabel(item.estimate_h)}</span>
      </div>
    </div>
  )
}
