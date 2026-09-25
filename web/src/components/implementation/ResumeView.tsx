// "Onde parei": a mesma resposta que o agente recebe de resume_work, para
// quem abre o projeto — sprint, trabalho em andamento com checkpoint, estado
// real do Git (que vence a memória), próxima tarefa, skills e memórias.

import {
  AlertTriangle, ArrowRight, Bot, GitBranch, GitCommitHorizontal, GitBranchPlus, History, Sparkles,
} from 'lucide-react'
import { useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { ResumeTask, Snapshot, WorkItem } from '../../lib/types'
import { useLive } from '../../lib/useLive'
import { ViewFrame } from '../shell/ViewFrame'
import { Badge, Button, Card, EmptyState, Spinner, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'
import { ItemDialog, type ItemAction } from './ItemDialog'
import { STATUS, displayName, formatDate, hoursLabel, sprintName } from './planMeta'
import { ClaimDialog, CompleteDialog, PromptDialog } from './WorkflowDialogs'

const AUTONOMY: Record<string, string> = {
  assistido: 'Assistido — o agente propõe e um humano confirma cada commit',
  supervisionado: 'Supervisionado — o agente commita; push e PR são humanos',
  autonomo: 'Autônomo — o agente publica e abre PR em rascunho',
}

const LIVE_EVENTS = ['plan_changed', 'tasks_changed', 'git_changed', 'session_logged', 'memory_changed', 'skills_changed', 'conventions_changed']

export function ResumeView({ snapshot, onOpenPlanning }: { snapshot: Snapshot; onOpenPlanning: () => void }) {
  const toast = useToast()
  const confirm = useConfirm()
  const [full, setFull] = useState(false)
  const { data: r, error, loading } = useLive(() => api.resume(full ? 'full' : 'brief'), LIVE_EVENTS, [full])
  const [dialog, setDialog] = useState<{ type: 'item' | ItemAction; item: WorkItem } | null>(null)
  const plan = snapshot.plan ?? { items: [], sprints: [] }

  const openItem = (task: ResumeTask, type: 'item' | ItemAction = 'item') => {
    const item = plan.items.find((i) => i.id === task.id)
    if (item) setDialog({ type, item })
  }

  const commitPlanning = async () => {
    try {
      const preview = await api.commit({ plan: true })
      if (!(await confirm({ title: 'Registrar o planejamento?', description: `Commit das alterações em .arch/ com a mensagem:\n${preview.subject}`, confirmLabel: 'Fazer commit' }))) return
      const res = await api.commit({ plan: true, commit: true })
      toast('success', `Commit ${res.hash?.slice(0, 7)}: ${res.subject}`)
    } catch (err) { toast('error', errorMessage(err)) }
  }

  if (loading && !r) return <Spinner label="Montando onde você parou…" />
  if (error && !r) return <EmptyState icon={AlertTriangle} title="Não foi possível carregar" description={error} />
  if (!r) return null

  const planningOnly = r.divergences?.some((d) => d.includes('commit --plan'))

  return (
    <ViewFrame icon={History} title="Onde parei"
      meta={<>{r.person ? displayName(r.person) : 'Sem identidade do Git'} · autonomia da IA: {AUTONOMY[r.autonomy] ?? r.autonomy}</>}
      actions={<Button size="sm" variant="ghost" onClick={() => setFull(!full)}>{full ? 'Menos detalhes' : 'Mais detalhes'}</Button>}>
      <div className="grid gap-3 p-3 @4xl:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <div className="space-y-3">
          {r.hints.length > 0 && (
            <Card title="Próximos passos">
              <ol className="space-y-1.5 text-[13px]">
                {r.hints.map((h, i) => (
                  <li key={h} className="flex gap-2"><span className="w-4 shrink-0 text-right font-semibold text-muted-foreground">{i + 1}.</span><span>{h}</span></li>
                ))}
              </ol>
            </Card>
          )}

          {!!r.divergences?.length && (
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-[12.5px]">
              <p className="mb-1 flex items-center gap-1.5 font-semibold"><AlertTriangle size={14} className="text-amber-500" />Atenção — o Git vence a memória</p>
              <ul className="list-disc space-y-0.5 pl-5">{r.divergences.map((d) => <li key={d}>{d}</li>)}</ul>
              {planningOnly && <Button size="sm" className="mt-2" icon={GitCommitHorizontal} onClick={() => void commitPlanning()}>Registrar planejamento (commit)</Button>}
            </div>
          )}

          {r.sprint ? (
            <Card title={`${r.sprint.name}${r.sprint.goal ? ` — ${r.sprint.goal}` : ''}`}
              actions={<Button size="sm" variant="ghost" icon={ArrowRight} onClick={onOpenPlanning}>Quadro</Button>}>
              <div className="flex items-center gap-3 text-[12.5px]">
                <div className="h-2 flex-1 overflow-hidden rounded-full bg-muted"><div className="h-full rounded-full bg-emerald-500" style={{ width: `${r.sprint.stats.progress}%` }} /></div>
                <span className="tabular-nums">{r.sprint.stats.completed}/{r.sprint.stats.total} · {r.sprint.stats.progress}%</span>
              </div>
              <p className="mt-1.5 text-[11.5px] text-muted-foreground">
                {r.sprint.days_left} dia(s) útil(eis) restante(s) · {r.sprint.stats.in_progress} em andamento · {r.sprint.stats.review} em revisão · {r.sprint.stats.blocked} bloqueada(s)
              </p>
            </Card>
          ) : (
            <Card title="Sem sprint ativa"><p className="text-[12.5px] text-muted-foreground">{r.backlog_size} item(ns) no backlog. <button className="underline" onClick={onOpenPlanning}>Planeje uma sprint</button>.</p></Card>
          )}

          <Card title="Seu trabalho">
            {r.my_work.length === 0 && <p className="text-[12.5px] text-muted-foreground">Nenhuma tarefa em andamento no seu nome.</p>}
            <div className="space-y-2">{r.my_work.map((t) => <TaskCard key={t.id} task={t} onOpen={() => openItem(t)} onComplete={() => openItem(t, 'complete')} onPrompt={() => openItem(t, 'prompt')} />)}</div>
          </Card>

          {r.next && (
            <Card title="Próxima tarefa pronta">
              <TaskCard task={r.next} onOpen={() => openItem(r.next!)} onClaim={() => openItem(r.next!, 'claim')} onPrompt={() => openItem(r.next!, 'prompt')} />
            </Card>
          )}
        </div>

        <div className="space-y-3">
          <Card title="Git">
            {!r.git.available ? <p className="text-[12.5px] text-muted-foreground">Git indisponível: o projeto não é um repositório ou o git não está instalado.</p> : (
              <div className="space-y-1.5 text-[12.5px]">
                <p className="flex items-center gap-1.5"><GitBranch size={13} /><code className="font-mono">{r.git.branch || '(detached)'}</code>
                  {r.git.upstream && <span className="text-muted-foreground">→ {r.git.upstream} (+{r.git.ahead ?? 0}/−{r.git.behind ?? 0})</span>}</p>
                {r.git.last_commit && <p className="truncate text-muted-foreground">Último commit: {r.git.last_commit}</p>}
                {r.git.changed_count > 0 && (
                  <details><summary className="cursor-pointer">{r.git.changed_count} arquivo(s) alterado(s)</summary>
                    <ul className="mt-1 max-h-40 overflow-y-auto font-mono text-[11px] text-muted-foreground">{r.git.changed?.map((f) => <li key={f}>{f}</li>)}</ul>
                  </details>
                )}
              </div>
            )}
          </Card>

          {(r.last_session || !!r.team_sessions?.length) && (
            <Card title="Sessões">
              <ul className="space-y-2 text-[12.5px]">
                {[...(r.last_session ? [{ ...r.last_session, mine: true }] : []), ...(r.team_sessions ?? []).map((s) => ({ ...s, mine: false }))].map((s) => (
                  <li key={s.id}>
                    <p><span className="font-medium">{s.mine ? 'Você' : s.author}</span>{s.agent && <span className="text-muted-foreground"> ({s.agent})</span>}
                      <span className="text-muted-foreground"> · {formatDate(s.started)}</span></p>
                    <p className="text-muted-foreground">{s.summary}</p>
                    {s.next_steps?.map((n) => <p key={n} className="text-[11.5px]">→ {n}</p>)}
                    {s.blockers?.map((b) => <p key={b} className="text-[11.5px] text-destructive">⚠ {b}</p>)}
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {!!r.stale_claims?.length && (
            <Card title="Reservas paradas">
              <ul className="space-y-1 text-[12.5px]">
                {r.stale_claims.map((t) => (
                  <li key={t.id} className="flex items-center gap-2">
                    <code className="font-mono text-[11px]">{t.id}</code><span className="min-w-0 flex-1 truncate">{t.title}</span>
                    <span className="text-muted-foreground">{displayName(t.assignee)}</span>
                    <Button size="sm" variant="ghost" onClick={() => openItem(t, 'claim')}>Assumir</Button>
                  </li>
                ))}
              </ul>
            </Card>
          )}

          {!!r.skills?.length && (
            <Card title="Skills que valem">
              <ul className="space-y-1 text-[12.5px]">
                {r.skills.map((s) => (
                  <li key={s.name} className="flex items-center gap-1.5"><Sparkles size={12} className="text-violet-500" /><code className="font-mono text-[11.5px]">{s.name}</code>
                    <span className="text-muted-foreground">· {s.trigger.replace(/_/g, ' ')}{s.required_checks ? ` · ${s.required_checks} check(s)` : ''}</span></li>
                ))}
              </ul>
            </Card>
          )}

          {!!r.memories?.length && (
            <Card title="Memórias do projeto">
              <ul className="space-y-1 text-[12.5px]">
                {r.memories.map((m) => <li key={m.slug}><Badge>{m.type}</Badge> <span className="font-medium">{m.title}</span>{m.summary && m.summary !== m.title && <span className="text-muted-foreground"> — {m.summary}</span>}</li>)}
              </ul>
            </Card>
          )}
        </div>
      </div>

      {dialog?.type === 'item' && <ItemDialog item={dialog.item} plan={plan} snapshot={snapshot} me={r.person} onClose={() => setDialog(null)} onAction={(a, i) => setDialog({ type: a, item: i })} />}
      {dialog?.type === 'claim' && <ClaimDialog item={dialog.item} onClose={() => setDialog(null)} />}
      {dialog?.type === 'complete' && <CompleteDialog item={dialog.item} onClose={() => setDialog(null)} />}
      {dialog?.type === 'prompt' && <PromptDialog item={dialog.item} onClose={() => setDialog(null)} />}
    </ViewFrame>
  )
}

function TaskCard({ task, onOpen, onClaim, onComplete, onPrompt }: {
  task: ResumeTask; onOpen: () => void; onClaim?: () => void; onComplete?: () => void; onPrompt?: () => void
}) {
  const h = task.handoff
  return (
    <div className="rounded-md border p-2.5 text-[12.5px]">
      <div className="flex flex-wrap items-center gap-2">
        <button className="font-mono text-[11.5px] font-semibold hover:underline" onClick={onOpen}>{task.id}</button>
        <span className="min-w-0 flex-1 font-medium">{task.title}</span>
        <Badge color={STATUS[task.status].color}>{STATUS[task.status].label}</Badge>
        {task.stale && <Badge color="#f59e0b">parada</Badge>}
      </div>
      <p className="mt-1 text-[11.5px] text-muted-foreground">
        {task.sprint ? sprintName(task.sprint) : 'sem sprint'} · {hoursLabel(task.estimate_h)}
        {task.component && ` · ${task.component}`}
        {(task.branch || task.suggested_branch) && <> · <code className="font-mono">{task.branch || task.suggested_branch}</code></>}
      </p>
      {!!task.blocked_by?.length && <p className="mt-1 text-[11.5px] text-amber-600 dark:text-amber-400">Aguarda {task.blocked_by.join(', ')}</p>}
      {h && (h.last_step || h.next_step) && (
        <div className="mt-1.5 space-y-0.5 border-l-2 pl-2 text-[12px]">
          {h.last_step && <p><span className="text-muted-foreground">Último passo:</span> {h.last_step}</p>}
          {h.next_step && <p><span className="text-muted-foreground">Próximo passo:</span> <b>{h.next_step}</b></p>}
          {!!h.failing_tests?.length && <p className="text-destructive">Testes falhando: {h.failing_tests.join(', ')}</p>}
          {!!h.files?.length && <p className="truncate font-mono text-[11px] text-muted-foreground">{h.files.join(', ')}</p>}
        </div>
      )}
      <div className="mt-2 flex flex-wrap gap-1.5">
        {onClaim && <Button size="sm" variant="primary" icon={GitBranchPlus} onClick={onClaim}>Reservar</Button>}
        {onComplete && task.status === 'in_progress' && <Button size="sm" onClick={onComplete}>Concluir…</Button>}
        {onPrompt && <Button size="sm" variant="ghost" icon={Bot} onClick={onPrompt}>Prompt para agente</Button>}
      </div>
    </div>
  )
}
