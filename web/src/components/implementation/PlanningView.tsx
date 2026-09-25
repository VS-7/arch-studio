// Planejamento: backlog, quadro da sprint e roadmap sobre o mesmo backlog em
// .arch/plan/ — o que agentes de IA leem e alteram via MCP. Toda mudança feita
// aqui chega a eles, e as deles chegam aqui em tempo real.

import { CalendarPlus, KanbanSquare, Plus, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import type { Plan, Snapshot, Sprint, WorkItem } from '../../lib/types'
import { useLive } from '../../lib/useLive'
import { ViewFrame } from '../shell/ViewFrame'
import { Button, EmptyState, Input, Select } from '../ui'
import { BacklogList, type BacklogFilters } from './BacklogList'
import { ItemDialog, type ItemAction } from './ItemDialog'
import { ITEM_TYPE, WORK_TYPES, isWork } from './planMeta'
import { Roadmap } from './Roadmap'
import { SprintBoard } from './SprintBoard'
import {
  ClaimDialog, CloseSprintDialog, CompleteDialog, PlanSprintDialog, PromptDialog, SyncDialog,
} from './WorkflowDialogs'

type Tab = 'board' | 'backlog' | 'roadmap'

type Dialog =
  | { type: 'item'; item: WorkItem | null }
  | { type: 'sync' }
  | { type: 'plan'; sprint?: Sprint }
  | { type: 'close'; sprint: Sprint }
  | { type: 'claim' | 'complete' | 'prompt'; item: WorkItem }

const EMPTY_PLAN: Plan = { items: [], sprints: [] }

export function PlanningView({ snapshot }: { snapshot: Snapshot }) {
  const plan = snapshot.plan ?? EMPTY_PLAN
  const git = useLive(() => api.git(), ['git_changed'])
  const me = git.data?.person

  const active = plan.sprints.find((s) => s.status === 'active')
  const nextPlanned = plan.sprints.filter((s) => s.status === 'planned').sort((a, b) => a.number - b.number)[0]
  const [tab, setTab] = useState<Tab>(() => (active ? 'board' : 'backlog'))
  const [sprintNumber, setSprintNumber] = useState(active?.number ?? nextPlanned?.number ?? 0)
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const [filters, setFilters] = useState<BacklogFilters>({ text: '', type: '', hideDone: true, structure: false })

  // A sprint mostrada acompanha a ativa quando ela muda (iniciar/encerrar).
  useEffect(() => {
    if (!plan.sprints.some((s) => s.number === sprintNumber)) setSprintNumber(active?.number ?? nextPlanned?.number ?? 0)
  }, [plan.sprints, sprintNumber, active, nextPlanned])

  const work = plan.items.filter((i) => isWork(i) && !i.archived)
  const done = work.filter((i) => i.status === 'completed').length
  const byId = useMemo(() => new Map(plan.items.map((i) => [i.id, i])), [plan.items])
  // Diálogos abertos a partir de um item sempre mostram a versão mais recente.
  const fresh = (item: WorkItem | null) => (item ? byId.get(item.id) ?? item : null)

  const onAction = (action: ItemAction, item: WorkItem) => setDialog({ type: action, item })
  const open = (item: WorkItem) => setDialog({ type: 'item', item })

  const newSprint = async () => {
    const sp = await api.createSprint({})
    setSprintNumber(sp.number)
    setDialog({ type: 'plan', sprint: sp })
  }

  const meta = (
    <>
      {work.length} item(ns) de trabalho · {done} concluído(s)
      {active ? ` · ${active.name} ativa` : ' · nenhuma sprint ativa'}
      {plan.warnings?.length ? ` · ⚠ ${plan.warnings.length} arquivo(s) com problema (rode archcode-studio doctor)` : ''}
    </>
  )

  return (
    <ViewFrame icon={KanbanSquare} title="Planejamento" meta={meta} bodyClassName="overflow-hidden"
      actions={
        <>
          <div className="flex rounded-md border p-0.5">
            {(['board', 'backlog', 'roadmap'] as Tab[]).map((t) => (
              <button key={t} onClick={() => setTab(t)}
                className={cn('rounded-sm px-2.5 py-0.5 text-[12px] font-medium', tab === t ? 'bg-accent text-accent-foreground' : 'text-muted-foreground hover:text-foreground')}>
                {{ board: 'Quadro', backlog: 'Backlog', roadmap: 'Roadmap' }[t]}
              </button>
            ))}
          </div>
          <Button size="sm" icon={RefreshCw} onClick={() => setDialog({ type: 'sync' })}>Sincronizar com a arquitetura</Button>
          <Button size="sm" icon={CalendarPlus} onClick={() => void newSprint()}>Nova sprint</Button>
          <Button size="sm" variant="primary" icon={Plus} onClick={() => setDialog({ type: 'item', item: null })}>Novo item</Button>
        </>
      }>
      {plan.items.length === 0 ? (
        <EmptyState icon={KanbanSquare} title="Backlog vazio"
          description="Gere o backlog a partir da arquitetura: casos de uso viram épicos, requisitos funcionais viram histórias e componentes viram tarefas, com dependências, critérios de aceite e estimativa."
          action={<Button variant="primary" icon={RefreshCw} onClick={() => setDialog({ type: 'sync' })}>Gerar backlog</Button>} />
      ) : (
        <div className="h-full min-h-0">
          {tab === 'board' && (
            <SprintBoard plan={plan} me={me} sprintNumber={sprintNumber} onSprint={setSprintNumber} onOpen={open} onAction={onAction}
              onPlan={(sprint) => setDialog({ type: 'plan', sprint })} onClose={(sprint) => setDialog({ type: 'close', sprint })} />
          )}
          {tab === 'backlog' && (
            <div className="flex h-full min-h-0 flex-col">
              <div className="flex flex-wrap items-center gap-2 border-b px-3 py-2">
                <Input className="h-7 w-64 text-[12px]" placeholder="Filtrar por id, título, componente ou dono" value={filters.text}
                  onChange={(e) => setFilters({ ...filters, text: e.target.value })} />
                <Select className="h-7 w-40 text-[12px]" value={filters.type} onChange={(e) => setFilters({ ...filters, type: e.target.value })}>
                  <option value="">Todos os tipos</option>
                  {WORK_TYPES.map((t) => <option key={t} value={t}>{ITEM_TYPE[t].label}</option>)}
                </Select>
                <label className="flex items-center gap-1.5 text-[12px]">
                  <input type="checkbox" checked={filters.hideDone} onChange={(e) => setFilters({ ...filters, hideDone: e.target.checked })} />Ocultar concluídos
                </label>
                <label className="flex items-center gap-1.5 text-[12px]">
                  <input type="checkbox" checked={filters.structure} onChange={(e) => setFilters({ ...filters, structure: e.target.checked })} />Épicos e histórias
                </label>
                <span className="ml-auto text-[11.5px] text-muted-foreground">Arraste para priorizar ou mover entre sprints</span>
              </div>
              <div className="min-h-0 flex-1 overflow-y-auto">
                <BacklogList plan={plan} filters={filters} onOpen={open} />
              </div>
            </div>
          )}
          {tab === 'roadmap' && <div className="h-full overflow-y-auto"><Roadmap plan={plan} onOpen={open} /></div>}
        </div>
      )}

      {dialog?.type === 'item' && (
        <ItemDialog key={dialog.item?.id ?? 'new'} item={fresh(dialog.item)} plan={plan} snapshot={snapshot} me={me}
          onClose={() => setDialog(null)} onAction={onAction} />
      )}
      {dialog?.type === 'sync' && <SyncDialog onClose={() => setDialog(null)} />}
      {dialog?.type === 'plan' && <PlanSprintDialog plan={plan} sprint={dialog.sprint} onClose={() => setDialog(null)} />}
      {dialog?.type === 'close' && <CloseSprintDialog plan={plan} sprint={dialog.sprint} onClose={() => setDialog(null)} />}
      {dialog?.type === 'claim' && <ClaimDialog item={dialog.item} onClose={() => setDialog(null)} />}
      {dialog?.type === 'complete' && <CompleteDialog item={dialog.item} onClose={() => setDialog(null)} />}
      {dialog?.type === 'prompt' && <PromptDialog item={dialog.item} onClose={() => setDialog(null)} />}
    </ViewFrame>
  )
}
