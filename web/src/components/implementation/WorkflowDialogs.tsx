// Diálogos do fluxo de trabalho: sincronizar o backlog com a arquitetura,
// planejar e encerrar sprints, reservar e concluir tarefas, copiar o prompt
// para um agente sem MCP. Toda ação irreversível ou pública passa por aqui,
// com o que vai acontecer escrito antes do botão.

import { Check, ClipboardCopy, GitBranchPlus, RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { platform } from '../../lib/platform'
import type {
  CheckResult, CloseResult, CompleteResult, Plan, RequiredCheck, Sprint, SprintPlanResult, SyncResult, WorkItem,
} from '../../lib/types'
import { Badge, Button, Field, Input, Modal, Select, Spinner, Textarea, useToast } from '../ui'
import { ITEM_TYPE, hoursLabel, sprintName } from './planMeta'

/* -------------------------------------------------------------------------- */
/* Sincronizar backlog                                                         */
/* -------------------------------------------------------------------------- */

const CHANGE_LABEL: Record<string, { label: string; color: string }> = {
  added: { label: 'novo', color: '#10b981' },
  updated: { label: 'atualizado', color: '#3b82f6' },
  archived: { label: 'arquivado', color: '#64748b' },
  restored: { label: 'restaurado', color: '#8b5cf6' },
}

export function SyncDialog({ onClose }: { onClose: () => void }) {
  const toast = useToast()
  const [preview, setPreview] = useState<SyncResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [applying, setApplying] = useState(false)

  useEffect(() => {
    api.syncBacklog(true).then(setPreview).catch((err) => setError(errorMessage(err)))
  }, [])

  const apply = async () => {
    setApplying(true)
    try {
      const res = await api.syncBacklog(false)
      toast('success', `Backlog sincronizado: ${res.counts.added ?? 0} novo(s), ${res.counts.updated ?? 0} atualizado(s), ${res.counts.archived ?? 0} arquivado(s)`)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setApplying(false) }
  }

  const empty = preview && preview.changes.length === 0
  return (
    <Modal open onClose={onClose} title="Sincronizar backlog com a arquitetura" wide
      description="Casos de uso viram épicos, requisitos funcionais viram histórias e componentes viram tarefas. Status, sprint, dono e edições feitas à mão são preservados; nada é apagado."
      footer={<>
        <Button variant="ghost" onClick={onClose}>{empty ? 'Fechar' : 'Cancelar'}</Button>
        {!empty && <Button variant="primary" icon={RefreshCw} loading={applying} disabled={!preview} onClick={() => void apply()}>
          Aplicar {preview ? `${preview.changes.length} mudança(s)` : ''}
        </Button>}
      </>}>
      {error && <p className="text-sm text-destructive">{error}</p>}
      {!preview && !error && <Spinner label="Comparando com a arquitetura…" />}
      {preview && (
        <div className="space-y-3">
          <p className="text-[12.5px] text-muted-foreground">
            Fatiamento <b className="text-foreground">{preview.slicing === 'hybrid' ? 'híbrido' : 'por componente'}</b>
            {preview.slicing === 'hybrid' ? ' (tarefa de fundação por componente + fatias verticais por requisito)' : ' (uma tarefa por componente, como o AI-PRD)'} ·
            mude em Convenções e Git.
          </p>
          {empty && <p className="rounded-md border bg-muted px-3 py-6 text-center text-[13px]">O backlog já está em dia com a arquitetura.</p>}
          {!empty && (
            <div className="max-h-[50vh] overflow-y-auto rounded-md border">
              <table className="w-full text-[12.5px]">
                <tbody>
                  {preview.changes.map((c) => {
                    const meta = CHANGE_LABEL[c.kind]
                    return (
                      <tr key={`${c.kind}-${c.id}`} className="border-b last:border-b-0">
                        <td className="w-24 px-2.5 py-1.5"><Badge color={meta.color}>{meta.label}</Badge></td>
                        <td className="w-24 px-1 py-1.5 text-muted-foreground">{ITEM_TYPE[c.type]?.label ?? c.type}</td>
                        <td className="px-1 py-1.5"><code className="font-mono text-[11px] font-semibold">{c.id}</code> {c.title}</td>
                        <td className="px-2.5 py-1.5 text-right text-[11px] text-muted-foreground">{c.fields?.join(', ')}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </Modal>
  )
}

/* -------------------------------------------------------------------------- */
/* Planejar sprint                                                             */
/* -------------------------------------------------------------------------- */

export function PlanSprintDialog({ plan, sprint, onClose }: { plan: Plan; sprint?: Sprint; onClose: () => void }) {
  const toast = useToast()
  const [goal, setGoal] = useState(sprint?.goal ?? '')
  const [capacity, setCapacity] = useState<string>(sprint?.capacity_h ? String(sprint.capacity_h) : '')
  const [proposal, setProposal] = useState<SprintPlanResult | null>(null)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [busy, setBusy] = useState(false)
  const number = sprint?.number ?? 0

  const propose = async () => {
    setBusy(true)
    try {
      const res = await api.planSprint({ number, goal, capacity_h: capacity ? Number(capacity) : undefined, apply: false })
      setProposal(res)
      setSelected(new Set(res.proposal.items))
      if (!capacity && res.sprint.capacity_h) setCapacity(String(res.sprint.capacity_h))
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }
  useEffect(() => { void propose() }, [])

  const candidates = useMemo(() => {
    if (!proposal) return []
    const ids = new Set([...proposal.proposal.items, ...(proposal.proposal.skipped ?? [])])
    return plan.items.filter((it) => ids.has(it.id))
  }, [plan.items, proposal])

  const load = candidates.filter((it) => selected.has(it.id)).reduce((sum, it) => sum + (it.estimate_h || 4), 0)
  const cap = Number(capacity) || proposal?.sprint.capacity_h || 0

  const apply = async () => {
    setBusy(true)
    try {
      const res = await api.planSprint({ number: proposal?.sprint.number ?? number, goal, capacity_h: capacity ? Number(capacity) : undefined, ids: [...selected], apply: true })
      toast('success', `${res.sprint.name} planejada com ${res.proposal.items.length} item(ns)`)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }

  return (
    <Modal open onClose={onClose} wide title={proposal ? `Planejar a ${proposal.sprint.name}` : 'Planejar sprint'}
      description="A proposta segue a ordem do backlog, respeita dependências e para na capacidade (pessoas × horas por dia × dias úteis, da Precificação). Ajuste a seleção antes de aplicar."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={busy} disabled={!proposal || selected.size === 0} onClick={() => void apply()}>
          Aplicar à sprint ({selected.size})
        </Button>
      </>}>
      <div className="grid gap-3 @md:grid-cols-[1fr_9rem_auto]" style={{ gridTemplateColumns: 'minmax(0,1fr) 9rem auto' }}>
        <Field label="Meta da sprint (uma frase)">
          <Input value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="Ex.: Autenticação e catálogo" />
        </Field>
        <Field label="Capacidade (h)">
          <Input type="number" min={0} value={capacity} onChange={(e) => setCapacity(e.target.value)} />
        </Field>
        <div className="flex items-end"><Button size="sm" icon={RefreshCw} onClick={() => void propose()}>Recalcular</Button></div>
      </div>
      {proposal && (
        <p className="mt-2 text-[12px] text-muted-foreground">
          {formatRange(proposal.sprint)} · carga <b className={load > cap && cap > 0 ? 'text-destructive' : 'text-foreground'}>{hoursLabel(load)}</b> de {hoursLabel(cap)}
          {load > cap && cap > 0 && ` — passa da capacidade em ${hoursLabel(load - cap)}`}
        </p>
      )}
      {!proposal ? <Spinner label="Montando a proposta…" /> : (
        <div className="mt-3 max-h-[45vh] overflow-y-auto rounded-md border">
          {candidates.length === 0 && <p className="px-3 py-6 text-center text-[13px] text-muted-foreground">Nenhum item pendente e pronto no backlog sem sprint.</p>}
          {candidates.map((it) => {
            const Icon = ITEM_TYPE[it.type].icon
            const skipped = proposal.proposal.skipped?.includes(it.id)
            return (
              <label key={it.id} className="flex cursor-pointer items-center gap-2.5 border-b px-3 py-1.5 text-[12.5px] last:border-b-0 hover:bg-muted">
                <input type="checkbox" checked={selected.has(it.id)} onChange={(e) => {
                  const next = new Set(selected)
                  if (e.target.checked) next.add(it.id); else next.delete(it.id)
                  setSelected(next)
                }} />
                <Icon size={13} style={{ color: ITEM_TYPE[it.type].color }} />
                <code className="font-mono text-[11px] font-semibold">{it.id}</code>
                <span className="min-w-0 flex-1 truncate">{it.title}</span>
                {skipped && <Badge color="#f59e0b">não coube</Badge>}
                <span className="w-14 text-right tabular-nums text-muted-foreground">{hoursLabel(it.estimate_h)}</span>
              </label>
            )
          })}
        </div>
      )}
    </Modal>
  )
}

function formatRange(sp: Sprint): string {
  const f = (d?: string) => (d ? new Date(`${d}T12:00:00`).toLocaleDateString('pt-BR') : '—')
  return `${f(sp.start)} a ${f(sp.end)}`
}

/* -------------------------------------------------------------------------- */
/* Encerrar sprint                                                             */
/* -------------------------------------------------------------------------- */

export function CloseSprintDialog({ sprint, plan, onClose }: { sprint: Sprint; plan: Plan; onClose: () => void }) {
  const toast = useToast()
  const [carry, setCarry] = useState('next')
  const [result, setResult] = useState<CloseResult | null>(null)
  const [busy, setBusy] = useState(false)
  const open = plan.items.filter((it) => it.sprint === sprint.number && it.type !== 'epic' && it.type !== 'story' && !it.archived && it.status !== 'completed')
  const later = plan.sprints.filter((s) => s.number > sprint.number && s.status === 'planned')

  const close = async () => {
    setBusy(true)
    try {
      const to = carry === 'backlog' ? 0 : carry === 'next' ? -1 : Number(carry)
      setResult(await api.closeSprint(sprint.number, to))
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }

  if (result) {
    return (
      <Modal open onClose={onClose} title={`${result.sprint.name} encerrada`}
        footer={<Button variant="primary" onClick={onClose}>Fechar</Button>}>
        <div className="space-y-2 text-[13px]">
          <p>{result.stats.completed}/{result.stats.total} itens concluídos ({result.stats.progress}%).</p>
          <p>Relatório: <code className="font-mono text-[12px]">{result.report_file}</code>{result.changelog_file && <> · changelog: <code className="font-mono text-[12px]">{result.changelog_file}</code></>}</p>
          {result.carried.length > 0 && <p>Transferidos para {result.carried_to > 0 ? sprintName(result.carried_to) : 'o backlog'}: {result.carried.join(', ')}</p>}
          <div className="rounded-md border bg-muted p-2.5">
            <p className="mb-1 text-[12px] font-medium">Tag sugerida — criá-la é decisão sua:</p>
            <div className="flex items-center gap-2">
              <code className="min-w-0 flex-1 truncate font-mono text-[11.5px]">{result.tag_command}</code>
              <Button size="sm" icon={ClipboardCopy} onClick={() => void platform.copyText(result.tag_command).then(() => toast('success', 'Comando copiado'))}>Copiar</Button>
            </div>
          </div>
        </div>
      </Modal>
    )
  }
  return (
    <Modal open onClose={onClose} title={`Encerrar a ${sprint.name}`}
      description="Gera o relatório em docs/sprints/, atualiza o CHANGELOG.md a partir dos commits e move os itens não concluídos. A tag é só sugerida."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="danger" loading={busy} onClick={() => void close()}>Encerrar sprint</Button>
      </>}>
      <p className="mb-3 text-[13px]">{open.length === 0 ? 'Todos os itens foram concluídos.' : `${open.length} item(ns) não concluído(s): ${open.map((i) => i.id).join(', ')}.`}</p>
      {open.length > 0 && (
        <Field label="Levar os itens não concluídos para">
          <Select value={carry} onChange={(e) => setCarry(e.target.value)}>
            <option value="next">A próxima sprint ({later[0] ? later[0].name : `criar ${sprintName(sprint.number + 1)}`})</option>
            <option value="backlog">O backlog (sem sprint)</option>
            {later.slice(1).map((s) => <option key={s.number} value={s.number}>{s.name}</option>)}
          </Select>
        </Field>
      )}
    </Modal>
  )
}

/* -------------------------------------------------------------------------- */
/* Reservar tarefa                                                             */
/* -------------------------------------------------------------------------- */

export function ClaimDialog({ item, onClose }: { item: WorkItem; onClose: () => void }) {
  const toast = useToast()
  const [switchBranch, setSwitchBranch] = useState(true)
  const [push, setPush] = useState(false)
  const [force, setForce] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const claim = async () => {
    setBusy(true)
    setError(null)
    try {
      const res = await api.claimItem(item.id, { switch: switchBranch, push, force })
      const parts = [`${res.task.id} reservada · branch ${res.branch}`]
      if (res.pushed) parts.push('reserva publicada')
      toast('success', parts.join(' · '))
      for (const w of res.warnings ?? []) toast('info', w)
      onClose()
    } catch (err) { setError(errorMessage(err)) } finally { setBusy(false) }
  }

  return (
    <Modal open onClose={onClose} title={`Reservar ${item.id}`}
      description="A reserva marca você como dono e cria a branch da convenção. Antes, o Studio consulta o remoto: se outra pessoa já publicou a branch desta tarefa, a reserva é recusada."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon={GitBranchPlus} loading={busy} onClick={() => void claim()}>Reservar</Button>
      </>}>
      <p className="mb-3 text-[13px] font-medium">{item.title}</p>
      <div className="space-y-2 text-[12.5px]">
        <label className="flex items-start gap-2">
          <input type="checkbox" className="mt-0.5" checked={switchBranch} onChange={(e) => setSwitchBranch(e.target.checked)} />
          <span>Trocar para a branch da tarefa agora <span className="text-muted-foreground">(o seu editor passa a ver a branch nova; alterações não commitadas vão junto)</span></span>
        </label>
        <label className="flex items-start gap-2">
          <input type="checkbox" className="mt-0.5" checked={push} disabled={!switchBranch} onChange={(e) => setPush(e.target.checked)} />
          <span>Publicar a reserva no remoto <span className="text-muted-foreground">(commit vazio de reserva + git push; o time vê quem está com a tarefa)</span></span>
        </label>
        {item.assignee && (
          <label className="flex items-start gap-2 text-destructive">
            <input type="checkbox" className="mt-0.5" checked={force} onChange={(e) => setForce(e.target.checked)} />
            <span>Assumir mesmo reservada por {item.assignee}</span>
          </label>
        )}
      </div>
      {error && <p className="mt-3 rounded-md border border-destructive/40 bg-destructive/10 px-2.5 py-2 text-[12.5px] text-destructive">{error}</p>}
    </Modal>
  )
}

/* -------------------------------------------------------------------------- */
/* Concluir tarefa                                                             */
/* -------------------------------------------------------------------------- */

type CheckState = { result: CheckResult['result'] | ''; evidence: string }

export function CompleteDialog({ item, onClose }: { item: WorkItem; onClose: () => void }) {
  const toast = useToast()
  const [required, setRequired] = useState<RequiredCheck[] | null>(null)
  const [state, setState] = useState<Record<string, CheckState>>({})
  const [notes, setNotes] = useState('')
  const [status, setStatus] = useState('')
  const [busy, setBusy] = useState(false)
  const [result, setResult] = useState<CompleteResult | null>(null)

  useEffect(() => {
    api.itemChecks(item.id).then(setRequired).catch((err) => toast('error', errorMessage(err)))
  }, [item.id, toast])

  const key = (c: RequiredCheck) => `${c.skill}/${c.check}`
  const filled = (required ?? []).every((c) => state[key(c)]?.result)

  const submit = async (force: boolean) => {
    setBusy(true)
    try {
      const checks: CheckResult[] = (required ?? []).filter((c) => state[key(c)]?.result).map((c) => ({
        skill: c.skill, check: c.check, result: state[key(c)].result as CheckResult['result'], evidence: state[key(c)].evidence,
      }))
      const res = await api.completeItem(item.id, { checks, notes, status: status || undefined, force })
      if (!res.completed) { toast('error', res.message); return }
      setResult(res)
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }

  if (result) {
    return (
      <Modal open onClose={onClose} title={result.message} footer={<Button variant="primary" onClick={onClose}>Fechar</Button>}>
        <ul className="space-y-1 text-[13px]">
          {(result.next_steps ?? []).map((n) => <li key={n}>→ {n}</li>)}
          {(result.warnings ?? []).map((w) => <li key={w} className="text-amber-600 dark:text-amber-400">⚠ {w}</li>)}
          {!result.next_steps?.length && !result.warnings?.length && <li className="text-muted-foreground">O quadro, o canvas e o tasks.json já refletem a conclusão.</li>}
        </ul>
      </Modal>
    )
  }

  return (
    <Modal open onClose={onClose} wide title={`Concluir ${item.id}`}
      description="Informe cada check obrigatório das skills que valem para esta tarefa, com a evidência (comando e resultado). Os critérios de aceite são marcados como atendidos."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="outline" loading={busy} onClick={() => void submit(true)}>Concluir sem os checks (fica registrado)</Button>
        <Button variant="primary" icon={Check} loading={busy} disabled={!filled} onClick={() => void submit(false)}>Concluir</Button>
      </>}>
      {!required ? <Spinner label="Buscando os checks das skills…" /> : (
        <div className="space-y-3">
          {required.length === 0 && <p className="text-[13px] text-muted-foreground">Nenhum check obrigatório para esta tarefa.</p>}
          <div className="max-h-[42vh] space-y-1.5 overflow-y-auto pr-1">
            {required.map((c) => {
              const k = key(c)
              const cur = state[k] ?? { result: '', evidence: '' }
              const set = (patch: Partial<CheckState>) => setState({ ...state, [k]: { ...cur, ...patch } })
              return (
                <div key={k} className="rounded-md border px-2.5 py-2">
                  <div className="flex items-start gap-2">
                    <div className="min-w-0 flex-1">
                      <p className="text-[12.5px]">{c.text}</p>
                      <p className="mt-0.5 text-[11px] text-muted-foreground"><code className="font-mono">{c.skill}/{c.check}</code>{c.command && <> · <code className="font-mono">{c.command}</code></>}</p>
                    </div>
                    <div className="flex shrink-0 gap-1">
                      {(['ok', 'na', 'fail'] as const).map((r) => (
                        <button key={r} type="button" onClick={() => set({ result: r })}
                          className={`rounded border px-2 py-0.5 text-[11px] font-semibold ${cur.result === r ? 'border-foreground bg-accent' : 'text-muted-foreground hover:bg-muted'}`}>
                          {r === 'ok' ? 'ok' : r === 'na' ? 'n/a' : 'falhou'}
                        </button>
                      ))}
                    </div>
                  </div>
                  {cur.result && <Input className="mt-1.5 h-7 text-[12px]" placeholder="Evidência: comando executado e resultado" value={cur.evidence} onChange={(e) => set({ evidence: e.target.value })} />}
                </div>
              )
            })}
          </div>
          <div className="grid gap-3" style={{ gridTemplateColumns: 'minmax(0,1fr) 12rem' }}>
            <Field label="Notas (o que foi entregue e como foi testado)">
              <Textarea rows={2} value={notes} onChange={(e) => setNotes(e.target.value)} />
            </Field>
            <Field label="Destino">
              <Select value={status} onChange={(e) => setStatus(e.target.value)}>
                <option value="">Automático (revisão com branch)</option>
                <option value="review">Em revisão (PR)</option>
                <option value="completed">Concluída</option>
              </Select>
            </Field>
          </div>
        </div>
      )}
    </Modal>
  )
}

/* -------------------------------------------------------------------------- */
/* Prompt para o agente                                                        */
/* -------------------------------------------------------------------------- */

export function PromptDialog({ item, onClose }: { item: WorkItem; onClose: () => void }) {
  const toast = useToast()
  const [prompt, setPrompt] = useState<string | null>(null)
  useEffect(() => {
    api.itemPrompt(item.id).then((r) => setPrompt(r.prompt)).catch((err) => toast('error', errorMessage(err)))
  }, [item.id, toast])
  return (
    <Modal open onClose={onClose} wide title={`Prompt para implementar ${item.id}`}
      description="Contexto, critérios, skills e convenção prontos para colar em qualquer agente de IA — útil quando ele não está conectado ao MCP do Studio."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Fechar</Button>
        <Button variant="primary" icon={ClipboardCopy} disabled={!prompt}
          onClick={() => void platform.copyText(prompt ?? '').then(() => { toast('success', 'Prompt copiado'); onClose() })}>Copiar prompt</Button>
      </>}>
      {!prompt ? <Spinner /> : <Textarea readOnly rows={18} className="font-mono text-[11.5px]" value={prompt} />}
    </Modal>
  )
}
