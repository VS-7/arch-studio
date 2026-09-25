// Ficha de um item do backlog: ver e editar, com as ações do ciclo de vida
// (reservar, checkpoint, concluir, liberar, prompt para o agente, arquivar).
// Em itens gerados da arquitetura, editar um campo que vem dela marca o
// override: a sincronização não desfaz a edição.

import { Archive, Bot, Check, GitBranchPlus, Save, Undo2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { api, type ItemInput } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { Criterion, Plan, Snapshot, WorkItem } from '../../lib/types'
import { Badge, Button, Field, Input, Modal, Select, Textarea, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'
import {
  ALL_TYPES, ITEM_TYPE, PRIORITY, STATUS, WORK_TYPES, blockers, displayName, formatDate, samePerson, sprintName,
} from './planMeta'

export type ItemAction = 'claim' | 'complete' | 'prompt'

export function ItemDialog({ item, plan, snapshot, me, onClose, onAction }: {
  item: WorkItem | null
  plan: Plan
  snapshot: Snapshot
  me?: string
  onClose: () => void
  onAction: (action: ItemAction, item: WorkItem) => void
}) {
  const toast = useToast()
  const confirm = useConfirm()
  const creating = !item
  const [form, setForm] = useState(() => ({
    type: item?.type ?? 'task',
    title: item?.title ?? '',
    description: item?.description ?? '',
    status: item?.status ?? 'pending',
    priority: item?.priority ?? '',
    sprint: String(item?.sprint ?? 0),
    estimate: item?.estimate_h ? String(item.estimate_h) : '',
    component: item?.component_id ?? '',
    parent: item?.parent ?? '',
    dependencies: (item?.dependencies ?? []).join(', '),
    acceptance: item?.acceptance ?? ([] as Criterion[]),
    notes: item?.notes ?? '',
  }))
  const [handoff, setHandoff] = useState({ last: item?.handoff?.last_step ?? '', next: item?.handoff?.next_step ?? '' })
  const [saving, setSaving] = useState(false)

  const byId = useMemo(() => new Map(plan.items.map((i) => [i.id, i])), [plan.items])
  const parents = plan.items.filter((i) => (i.type === 'epic' || i.type === 'story') && i.id !== item?.id && !i.archived)
  const components = snapshot.diagram.nodes.filter((n) => n.type !== 'group')
  const openBlockers = item ? blockers(item, byId) : []
  const mine = item?.assignee ? samePerson(item.assignee, me) : false
  const set = (patch: Partial<typeof form>) => setForm((f) => ({ ...f, ...patch }))

  const save = async () => {
    const input: ItemInput = {}
    const deps = form.dependencies.split(/[,\s]+/).map((d) => d.trim().toUpperCase()).filter(Boolean)
    if (creating || form.type !== item!.type) input.type = form.type
    if (creating || form.title !== item!.title) input.title = form.title
    if (creating || form.description !== (item!.description ?? '')) input.description = form.description
    if (!creating && form.status !== item!.status) input.status = form.status
    if (creating || form.priority !== (item!.priority ?? '')) input.priority = form.priority
    if (creating || Number(form.sprint) !== (item!.sprint ?? 0)) input.sprint = Number(form.sprint)
    const est = form.estimate ? Number(form.estimate) : 0
    if (creating ? est > 0 : est !== (item!.estimate_h ?? 0)) input.estimate_h = est
    if (creating ? !!form.component : form.component !== (item!.component_id ?? '')) input.component_id = form.component
    if (creating ? !!form.parent : form.parent !== (item!.parent ?? '')) input.parent = form.parent
    if (creating ? deps.length > 0 : deps.join(',') !== (item!.dependencies ?? []).join(',')) input.dependencies = deps
    const acc = form.acceptance.filter((c) => c.text.trim())
    if (creating ? acc.length > 0 : JSON.stringify(acc) !== JSON.stringify(item!.acceptance ?? [])) input.acceptance = acc
    if (creating ? !!form.notes : form.notes !== (item!.notes ?? '')) input.notes = form.notes
    setSaving(true)
    try {
      if (creating) {
        const created = await api.createItem(input)
        toast('success', `${created.id} criado`)
      } else {
        if (Object.keys(input).length > 0) await api.updateItem(item!.id, input)
        const h = item!.handoff
        if (handoff.last !== (h?.last_step ?? '') || handoff.next !== (h?.next_step ?? '')) {
          await api.checkpointItem(item!.id, { last_step: handoff.last, next_step: handoff.next })
        }
        toast('success', `${item!.id} salvo`)
      }
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setSaving(false) }
  }

  const release = async () => {
    if (!item) return
    try {
      await api.releaseItem(item.id, { force: !mine })
      toast('success', `${item.id} liberada`)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const remove = async () => {
    if (!item) return
    const generated = !!item.source
    const ok = await confirm({
      title: generated ? `Arquivar ${item.id}?` : `Excluir ${item.id}?`,
      description: generated
        ? 'O item veio da arquitetura: ele é arquivado (some das listas) e pode ser restaurado depois.'
        : 'O arquivo do item é apagado do projeto. O histórico continua no Git.',
      confirmLabel: generated ? 'Arquivar' : 'Excluir', destructive: true,
    })
    if (!ok) return
    try {
      await api.deleteItem(item.id)
      toast('success', `${item.id} ${generated ? 'arquivado' : 'excluído'}`)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const title = creating ? 'Novo item no backlog' : `${item!.id} · ${ITEM_TYPE[item!.type]?.label}`
  return (
    <Modal open onClose={onClose} wide title={title}
      description={creating ? 'Bug, débito técnico, spike, risco de segurança ou tarefa que não vem da arquitetura.' : undefined}
      footer={<>
        {!creating && (
          <div className="mr-auto flex flex-wrap gap-1.5">
            {item!.status !== 'completed' && isWorkItem(item!) && (!item!.assignee || !mine) &&
              <Button size="sm" icon={GitBranchPlus} onClick={() => onAction('claim', item!)}>Reservar</Button>}
            {item!.assignee && item!.status !== 'completed' &&
              <Button size="sm" icon={Undo2} onClick={() => void release()}>Liberar</Button>}
            {isWorkItem(item!) && item!.status !== 'completed' &&
              <Button size="sm" icon={Check} onClick={() => onAction('complete', item!)}>Concluir…</Button>}
            {isWorkItem(item!) && <Button size="sm" icon={Bot} onClick={() => onAction('prompt', item!)}>Prompt para agente</Button>}
            <Button size="sm" variant="ghost" icon={Archive} onClick={() => void remove()}>{item!.source ? 'Arquivar' : 'Excluir'}</Button>
          </div>
        )}
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon={Save} loading={saving} disabled={!form.title.trim()} onClick={() => void save()}>
          {creating ? 'Criar' : 'Salvar'}
        </Button>
      </>}>
      <div className="max-h-[62vh] space-y-3 overflow-y-auto pr-1">
        {!creating && (
          <div className="flex flex-wrap items-center gap-1.5 text-[11.5px] text-muted-foreground">
            {item!.source ? <Badge color="#0ea5e9">gerado da arquitetura · {item!.source}</Badge> : <Badge>criado à mão</Badge>}
            {!!item!.overrides?.length && <Badge color="#f59e0b">editado à mão: {item!.overrides.join(', ')}</Badge>}
            {item!.assignee && <Badge color="#8b5cf6">com {displayName(item!.assignee)}{item!.agent ? ` (${item!.agent})` : ''} desde {formatDate(item!.claimed_at)}</Badge>}
            {item!.branch && <Badge><code className="font-mono">{item!.branch}</code></Badge>}
            {openBlockers.length > 0 && <Badge color="#ef4444">aguarda {openBlockers.join(', ')}</Badge>}
            {item!.archived && <Badge color="#64748b">arquivado: {item!.archive_reason}</Badge>}
            <span className="ml-auto">{item!.file}</span>
          </div>
        )}

        <div className="grid gap-3" style={{ gridTemplateColumns: 'minmax(0,1fr) 10rem' }}>
          <Field label="Título"><Input value={form.title} onChange={(e) => set({ title: e.target.value })} autoFocus={creating} /></Field>
          <Field label="Tipo">
            <Select value={form.type} onChange={(e) => set({ type: e.target.value as typeof form.type })}>
              {(creating ? WORK_TYPES : ALL_TYPES).map((t) => <option key={t} value={t}>{ITEM_TYPE[t].label}</option>)}
            </Select>
          </Field>
        </div>

        <div className="grid grid-cols-2 gap-3 @lg:grid-cols-5" style={{ gridTemplateColumns: 'repeat(5, minmax(0,1fr))' }}>
          {!creating && (
            <Field label="Status">
              <Select value={form.status} onChange={(e) => set({ status: e.target.value as typeof form.status })}>
                {Object.entries(STATUS).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
              </Select>
            </Field>
          )}
          <Field label="Prioridade">
            <Select value={form.priority} onChange={(e) => set({ priority: e.target.value as typeof form.priority })}>
              <option value="">—</option>
              {Object.entries(PRIORITY).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
            </Select>
          </Field>
          <Field label="Sprint">
            <Select value={form.sprint} onChange={(e) => set({ sprint: e.target.value })}>
              <option value="0">Backlog</option>
              {plan.sprints.filter((s) => s.status !== 'closed' || s.number === item?.sprint).map((s) =>
                <option key={s.number} value={s.number}>{s.name}{s.status === 'active' ? ' (ativa)' : ''}</option>)}
            </Select>
          </Field>
          <Field label="Estimativa (h)"><Input type="number" min={0} step={0.5} value={form.estimate} onChange={(e) => set({ estimate: e.target.value })} /></Field>
          <Field label="Pai">
            <Select value={form.parent} onChange={(e) => set({ parent: e.target.value })}>
              <option value="">—</option>
              {parents.map((p) => <option key={p.id} value={p.id}>{p.id} · {p.title}</option>)}
            </Select>
          </Field>
        </div>

        <div className="grid gap-3" style={{ gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)' }}>
          <Field label="Componente do diagrama">
            <Select value={form.component} onChange={(e) => set({ component: e.target.value })}>
              <option value="">—</option>
              {components.map((n) => <option key={n.id} value={n.id}>{n.data.label}</option>)}
            </Select>
          </Field>
          <Field label="Depende de (ids separados por vírgula)">
            <Input value={form.dependencies} onChange={(e) => set({ dependencies: e.target.value })} placeholder="TASK-DB-01, TASK-CORE-01" />
          </Field>
        </div>

        <Field label="Descrição"><Textarea rows={3} value={form.description} onChange={(e) => set({ description: e.target.value })} /></Field>

        <Field label={`Critérios de aceite (${form.acceptance.filter((c) => c.done).length}/${form.acceptance.length} atendidos)`}>
          <div className="space-y-1">
            {form.acceptance.map((c, i) => (
              <div key={i} className="flex items-center gap-2">
                <input type="checkbox" checked={c.done} onChange={(e) => set({ acceptance: form.acceptance.map((x, j) => (j === i ? { ...x, done: e.target.checked } : x)) })} />
                <Input className="h-7 text-[12.5px]" value={c.text} onChange={(e) => set({ acceptance: form.acceptance.map((x, j) => (j === i ? { ...x, text: e.target.value } : x)) })} />
                <Button size="icon" variant="ghost" aria-label="Remover critério" onClick={() => set({ acceptance: form.acceptance.filter((_, j) => j !== i) })}>×</Button>
              </div>
            ))}
            <Button size="sm" variant="ghost" onClick={() => set({ acceptance: [...form.acceptance, { text: '', done: false }] })}>+ critério</Button>
          </div>
        </Field>

        {!creating && isWorkItem(item!) && (
          <div className="rounded-md border p-2.5">
            <p className="mb-2 text-[11.5px] font-medium text-muted-foreground">
              Checkpoint (onde parou){item!.handoff?.updated_at && ` · ${displayName(item!.handoff.by)} em ${formatDate(item!.handoff.updated_at)}`}
            </p>
            <div className="grid gap-2" style={{ gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)' }}>
              <Field label="Último passo"><Input value={handoff.last} onChange={(e) => setHandoff({ ...handoff, last: e.target.value })} /></Field>
              <Field label="Próximo passo"><Input value={handoff.next} onChange={(e) => setHandoff({ ...handoff, next: e.target.value })} /></Field>
            </div>
            {(item!.handoff?.files?.length || item!.handoff?.failing_tests?.length) ? (
              <p className="mt-1.5 text-[11.5px] text-muted-foreground">
                {item!.handoff?.files?.length ? <>Arquivos: {item!.handoff.files.join(', ')}. </> : null}
                {item!.handoff?.failing_tests?.length ? <>Testes falhando: {item!.handoff.failing_tests.join(', ')}.</> : null}
              </p>
            ) : null}
            {item!.handoff?.notes && <p className="mt-1 text-[11.5px] text-muted-foreground">{item!.handoff.notes}</p>}
          </div>
        )}

        {!creating && !!item!.checks?.length && (
          <div>
            <p className="mb-1 text-[11.5px] font-medium text-muted-foreground">Checks das skills</p>
            <ul className="space-y-0.5 text-[12px]">
              {item!.checks!.map((c) => (
                <li key={`${c.skill}/${c.check}`}>
                  <Badge color={c.result === 'ok' ? '#10b981' : c.result === 'fail' ? '#ef4444' : '#64748b'}>{c.result}</Badge>{' '}
                  <code className="font-mono text-[11px]">{c.skill}/{c.check}</code>{c.evidence && <span className="text-muted-foreground"> — {c.evidence}</span>}
                </li>
              ))}
            </ul>
          </div>
        )}

        <Field label="Notas"><Textarea rows={3} value={form.notes} onChange={(e) => set({ notes: e.target.value })} /></Field>

        {!creating && (
          <p className="text-[11px] text-muted-foreground">
            {item!.requirements?.length ? `Requisitos: ${item!.requirements.join(', ')} · ` : ''}
            {item!.use_cases?.length ? `Casos de uso: ${item!.use_cases.join(', ')} · ` : ''}
            {item!.endpoints?.length ? `Endpoints: ${item!.endpoints.join(', ')} · ` : ''}
            {item!.sprint ? `${sprintName(item!.sprint)} · ` : ''}criado {formatDate(item!.created_at)} · atualizado {formatDate(item!.updated_at)}
          </p>
        )}
      </div>
    </Modal>
  )
}

function isWorkItem(item: WorkItem): boolean {
  return item.type !== 'epic' && item.type !== 'story'
}
