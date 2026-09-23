// Rastreabilidade de progresso da IA (RF022).
//
// A mesma fila consumida por agentes via `get_implementation_tasks` é mostrada
// aqui; mudar o status pela interface tem exatamente o mesmo efeito que a IA
// chamar `mark_task_status`.

import { Bot, CheckCircle2, ChevronDown, ChevronRight, CircleDashed, Lock, PlayCircle, XCircle } from 'lucide-react'
import { useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { STATUS_META } from '../../lib/nodeMeta'
import type { Snapshot, Task, TaskStatus } from '../../lib/types'
import { Badge, Button, Card, EmptyState, Select, useToast } from '../ui'

const TIER_LABEL: Record<string, string> = {
  data: 'Data Tier', domain: 'Domain Tier', backend: 'Service Tier',
  integration: 'Integration Tier', devops: 'DevOps Tier', frontend: 'Presentation Tier',
}

const STATUS_ICON: Record<TaskStatus, typeof CircleDashed> = {
  pending: CircleDashed, in_progress: PlayCircle, completed: CheckCircle2, blocked: XCircle,
}

export function TasksView({ snapshot, onGeneratePRD }: { snapshot: Snapshot; onGeneratePRD: () => void }) {
  const toast = useToast()
  const [filter, setFilter] = useState<'all' | TaskStatus>('all')
  const [expanded, setExpanded] = useState<string | null>(null)

  const board = snapshot.tasks
  const byId = useMemo(() => new Map(board.tasks.map((t) => [t.id, t])), [board.tasks])

  const readyOf = (task: Task) =>
    (task.dependencies ?? []).every((dep) => byId.get(dep)?.status === 'completed')

  const tasks = useMemo(
    () => board.tasks.filter((t) => filter === 'all' || t.status === filter).sort((a, b) => a.order - b.order),
    [board.tasks, filter],
  )

  const completed = board.tasks.filter((t) => t.status === 'completed').length
  const progress = board.tasks.length ? Math.round((completed / board.tasks.length) * 100) : 0

  const setStatus = async (task: Task, status: TaskStatus) => {
    try {
      const res = await api.setTaskStatus(task.id, status)
      toast('success', `${task.id} → ${STATUS_META[status].label} (${res.overall_progress_percentage}% do projeto)`)
    } catch (err) { toast('error', (err as Error).message) }
  }

  if (board.tasks.length === 0) {
    return (
      <div className="mx-auto h-full w-full max-w-5xl px-5 py-5">
        <Card>
          <EmptyState icon={Bot} title="Nenhuma fila de implementação"
            description="Gere o AI-PRD para compilar a arquitetura em tarefas ordenadas topologicamente, com dependências e critérios de aceite verificáveis."
            action={<Button variant="primary" icon={Bot} onClick={onGeneratePRD}>Gerar AI-PRD</Button>} />
        </Card>
      </div>
    )
  }

  return (
    <div className="mx-auto h-full w-full max-w-5xl overflow-y-auto px-5 py-5">
      <div className="mb-4 surface rounded-lg border border-app p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-base font-bold text-app">Fila de Implementação</h2>
            <p className="text-xs text-muted-app">
              {board.tasks.length} tarefas · stack {board.target_stack || 'não definido'} · hash{' '}
              <code className="font-mono text-[11px]">{board.source_hash}</code>
            </p>
          </div>
          <div className="flex items-center gap-2">
            <Select value={filter} onChange={(e) => setFilter(e.target.value as typeof filter)} className="w-40">
              <option value="all">Todas</option>
              <option value="pending">Pendentes</option>
              <option value="in_progress">Em andamento</option>
              <option value="completed">Concluídas</option>
              <option value="blocked">Bloqueadas</option>
            </Select>
            <Button size="sm" variant="ghost" icon={Bot} onClick={onGeneratePRD}>Recompilar</Button>
          </div>
        </div>
        <div className="mt-3 flex items-center gap-3">
          <div className="h-2 flex-1 overflow-hidden rounded-full surface-3">
            <div className="h-full rounded-full bg-success transition-all duration-700" style={{ width: `${progress}%` }} />
          </div>
          <span className="w-24 text-right text-xs font-semibold tabular-nums text-app">
            {completed}/{board.tasks.length} · {progress}%
          </span>
        </div>
      </div>

      <ol className="space-y-2">
        {tasks.map((task) => {
          const Icon = STATUS_ICON[task.status]
          const meta = STATUS_META[task.status]
          const ready = readyOf(task)
          const open = expanded === task.id
          const blockedBy = (task.dependencies ?? []).filter((d) => byId.get(d)?.status !== 'completed')

          return (
            <li key={task.id} className="surface rounded-lg border border-app">
              <div className="flex items-start gap-3 px-4 py-3">
                <Icon size={17} className="mt-0.5 shrink-0" style={{ color: meta.dot }} />
                <div className="min-w-0 flex-1">
                  <button onClick={() => setExpanded(open ? null : task.id)}
                    className="flex w-full items-start gap-2 text-left">
                    {open ? <ChevronDown size={13} className="mt-1 shrink-0 text-muted-app" />
                          : <ChevronRight size={13} className="mt-1 shrink-0 text-muted-app" />}
                    <div className="min-w-0 flex-1">
                      <div className="flex flex-wrap items-center gap-2">
                        <code className="font-mono text-[11px] font-bold text-primary">{task.id}</code>
                        <span className="text-[13px] font-semibold text-app">{task.title}</span>
                      </div>
                      <div className="mt-1 flex flex-wrap items-center gap-1.5">
                        <Badge color="#64748b">{TIER_LABEL[task.tier] ?? task.tier}</Badge>
                        {task.status !== 'completed' && (
                          ready
                            ? <Badge color="#10b981">pronta para começar</Badge>
                            : <Badge color="#f59e0b"><Lock size={9} /> aguarda {blockedBy.length}</Badge>
                        )}
                        {(task.requirements ?? []).map((r) => <Badge key={r} color="#0ea5e9">{r}</Badge>)}
                        {(task.use_cases ?? []).map((u) => <Badge key={u} color="#8b5cf6">{u}</Badge>)}
                      </div>
                    </div>
                  </button>

                  {open && (
                    <div className="mt-3 space-y-3 border-t border-app pt-3">
                      {(task.dependencies ?? []).length > 0 && (
                        <Section title="Dependências">
                          <div className="flex flex-wrap gap-1.5">
                            {task.dependencies!.map((dep) => {
                              const d = byId.get(dep)
                              return (
                                <Badge key={dep} color={d?.status === 'completed' ? '#10b981' : '#64748b'}>
                                  {dep}
                                </Badge>
                              )
                            })}
                          </div>
                        </Section>
                      )}
                      {(task.endpoints ?? []).length > 0 && (
                        <Section title="Contratos">
                          <div className="flex flex-wrap gap-1.5">
                            {task.endpoints!.map((ep) => (
                              <code key={ep} className="rounded surface-3 px-1.5 py-0.5 font-mono text-[10.5px] text-app">{ep}</code>
                            ))}
                          </div>
                        </Section>
                      )}
                      {(task.acceptance ?? []).length > 0 && (
                        <Section title="Critérios de aceite">
                          <ul className="space-y-1">
                            {task.acceptance!.map((a, i) => (
                              <li key={i} className="flex gap-2 text-[12px] leading-relaxed text-muted-app">
                                <span className="text-success">✓</span>
                                <span dangerouslySetInnerHTML={{ __html: a.replace(/\*\*([^*]+)\*\*/g, '<strong class="text-app">$1</strong>').replace(/`([^`]+)`/g, '<code>$1</code>') }} />
                              </li>
                            ))}
                          </ul>
                        </Section>
                      )}
                      {task.notes && (
                        <Section title="Notas do agente">
                          <p className="text-[12px] italic leading-relaxed text-muted-app">{task.notes}</p>
                        </Section>
                      )}
                    </div>
                  )}
                </div>

                <Select value={task.status} className="w-36 shrink-0"
                  onChange={(e) => void setStatus(task, e.target.value as TaskStatus)}>
                  {Object.entries(STATUS_META).map(([value, m]) => (
                    <option key={value} value={value}>{m.label}</option>
                  ))}
                </Select>
              </div>
            </li>
          )
        })}
      </ol>
    </div>
  )
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1 text-[10px] font-bold uppercase tracking-wider text-muted-app">{title}</p>
      {children}
    </div>
  )
}
