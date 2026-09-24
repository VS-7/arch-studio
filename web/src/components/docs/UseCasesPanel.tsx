// Gestão modular de casos de uso (RF002). Cada caso de uso é um arquivo próprio
// em docs/casos-de-uso/, com os campos padronizados exigidos pelo PRD.

import { ListChecks, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { Snapshot, UseCase } from '../../lib/types'
import { PRIORITIES } from '../../lib/requirements'
import { ViewFrame } from '../shell/ViewFrame'
import { Badge, Button, ChipPicker, EmptyState, Field, Input, Modal, Select, StringList, Textarea, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'

const EMPTY: UseCase = {
  code: '', name: '', description: '', requirements: [], post_conditions: [],
  actors: [], components: [], complexity: 'medium', estimated_hours: 16,
  priority: 'Média', status: 'pending', pre_conditions: [], main_flow: [], alternate_flows: [],
  exceptions: [], business_rules: [], acceptance: [],
}

export function UseCasesPanel({ snapshot, focusCode }: { snapshot: Snapshot; focusCode?: string }) {
  const toast = useToast()
  const confirm = useConfirm()
  const [editing, setEditing] = useState<UseCase | null>(
    () => (focusCode ? snapshot.use_cases.find((uc) => uc.code === focusCode) ?? null : null),
  )

  const remove = async (code: string) => {
    if (!await confirm({ title: `Remover o caso de uso ${code}?`, description: 'O arquivo da ficha em docs/casos-de-uso/ será apagado.', confirmLabel: 'Remover', destructive: true })) return
    try {
      await api.deleteUseCase(code)
      toast('success', `${code} removido`)
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const save = async (uc: UseCase) => {
    try {
      const res = await api.upsertUseCase(uc)
      toast('success', `${res.use_case.code} salvo em ${res.file}`)
      setEditing(null)
    } catch (err) { toast('error', errorMessage(err)) }
  }

  return (
    <ViewFrame icon={ListChecks} title="Casos de Uso"
      meta={`${snapshot.use_cases.length} ficha(s) · docs/casos-de-uso/`}
      actions={<Button size="sm" variant="primary" icon={Plus} onClick={() => setEditing({ ...EMPTY })}>Novo caso de uso</Button>}>
      {snapshot.use_cases.length === 0 ? (
        <EmptyState icon={ListChecks} title="Nenhum caso de uso"
          description="Casos de uso viram critérios de aceite Given-When-Then no AI-PRD e alimentam a estimativa de esforço."
          action={<Button variant="primary" icon={Plus} onClick={() => setEditing({ ...EMPTY })}>Criar o primeiro</Button>} />
      ) : (
        <ul className="grid gap-2 p-4 @3xl:grid-cols-2 @7xl:grid-cols-3">
          {snapshot.use_cases.map((uc) => (
            <li key={uc.code} onClick={() => setEditing(uc)}
              className="group cursor-pointer rounded-lg border bg-card px-3 py-2.5 shadow-xs transition-colors hover:bg-muted">
              <div className="flex items-start gap-2.5">
                <span className="mt-0.5 font-mono text-[11px] font-bold text-ai">{uc.code}</span>
                <div className="min-w-0 flex-1">
                  <p className="text-[13px] font-semibold text-app">{uc.name}</p>
                  {uc.description && <p className="mt-0.5 line-clamp-2 text-[12px] text-muted-app">{uc.description}</p>}
                  <p className="mt-0.5 text-[11.5px] text-muted-app">
                    {(uc.actors ?? []).join(', ') || 'sem atores'} · {uc.main_flow?.length ?? 0} passos ·{' '}
                    {uc.exceptions?.length ?? 0} exceções
                  </p>
                  <div className="mt-1.5 flex flex-wrap gap-1.5">
                    <Badge color="#8b5cf6">{uc.complexity ?? 'medium'}</Badge>
                    {uc.estimated_hours ? <Badge>{uc.estimated_hours}h</Badge> : null}
                    {(uc.requirements ?? []).map((r) => <Badge key={r}>{r}</Badge>)}
                    {(uc.components ?? []).map((c) => {
                      const label = snapshot.diagram.nodes.find((n) => n.id === c)?.data.label ?? c
                      return <Badge key={c} color="#10b981">{label}</Badge>
                    })}
                  </div>
                </div>
                <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100"
                  onClick={(e) => e.stopPropagation()}>
                  <Button variant="ghost" size="icon" onClick={() => setEditing(uc)} aria-label="Editar"><Pencil size={13} /></Button>
                  <Button variant="ghost" size="icon" className="text-destructive" aria-label="Remover"
                    onClick={() => void remove(uc.code)}><Trash2 size={13} /></Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}

      {editing && <UseCaseModal useCase={editing} snapshot={snapshot} onClose={() => setEditing(null)} onSave={save} />}
    </ViewFrame>
  )
}

function UseCaseModal({ useCase, snapshot, onClose, onSave }: {
  useCase: UseCase; snapshot: Snapshot; onClose: () => void; onSave: (uc: UseCase) => Promise<void>
}) {
  const [form, setForm] = useState<UseCase>(useCase)
  useEffect(() => setForm(useCase), [useCase])

  const set = <K extends keyof UseCase>(key: K, value: UseCase[K]) => setForm((f) => ({ ...f, [key]: value }))

  return (
    <Modal open onClose={onClose} wide
      title={form.code ? `Editar ${form.code}` : 'Novo caso de uso'}
      description="Os campos abaixo são gravados em docs/casos-de-uso/ no formato lido pelo compilador de PRD."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" onClick={() => void onSave(form)} disabled={!form.name.trim()}>Salvar</Button>
      </>}>
      <div className="space-y-3">
        <div className="grid grid-cols-4 gap-2.5">
          <Field label="Código" hint="Vazio = próximo">
            <Input value={form.code} placeholder="CDU003" onChange={(e) => set('code', e.target.value.toUpperCase())} />
          </Field>
          <Field label="Nome" className="col-span-3">
            <Input value={form.name} placeholder="Processar Assinatura Recorrente" onChange={(e) => set('name', e.target.value)} />
          </Field>
        </div>

        <Field label="Descrição do caso de uso">
          <Textarea rows={2} value={form.description ?? ''}
            placeholder="Permitir que o usuário … de forma organizada e acessível."
            onChange={(e) => set('description', e.target.value)} />
        </Field>

        <div className="grid grid-cols-3 gap-2.5">
          <Field label="Complexidade">
            <Select value={form.complexity} onChange={(e) => set('complexity', e.target.value as UseCase['complexity'])}>
              <option value="low">Baixa</option><option value="medium">Média</option><option value="high">Alta</option>
            </Select>
          </Field>
          <Field label="Horas estimadas">
            <Input type="number" min={0} value={form.estimated_hours ?? 0}
              onChange={(e) => set('estimated_hours', Number(e.target.value))} />
          </Field>
          <Field label="Prioridade">
            <Select value={form.priority} onChange={(e) => set('priority', e.target.value)}>
              {PRIORITIES.map((p) => <option key={p.value} value={p.value}>{p.label}</option>)}
            </Select>
          </Field>
        </div>

        <Field label="Atores">
          <Input value={(form.actors ?? []).join(', ')} placeholder="Cliente, Webhook Stripe"
            onChange={(e) => set('actors', e.target.value.split(',').map((s) => s.trim()).filter(Boolean))} />
        </Field>

        <Field label="Requisitos associados" hint="Liga a ficha aos requisitos no Documento de Requisitos e na rastreabilidade.">
          <ChipPicker value={form.requirements ?? []} onChange={(v) => set('requirements', v)}
            options={snapshot.requirements.requirements.map((r) => ({ value: r.id, label: r.id, title: r.title }))}
            empty="Nenhum requisito cadastrado." />
        </Field>

        <Field label="Componentes envolvidos">
          <ChipPicker value={form.components ?? []} onChange={(v) => set('components', v)}
            options={snapshot.diagram.nodes.filter((n) => n.type !== 'group').map((n) => ({ value: n.id, label: n.data.label }))}
            empty="Nenhum componente no diagrama ainda." />
        </Field>

        <p className="rounded-md bg-muted px-3 py-2 text-[11.5px] text-muted-foreground">
          Nos fluxos, um item que começa com <code className="font-mono">#</code> vira subtítulo
          (ex.: <code className="font-mono"># Cadastro de paciente</code>) e a numeração dos passos continua — como no documento.
        </p>

        <div className="grid grid-cols-2 gap-4">
          <Field label="Entradas e pré-condições">
            <StringList value={form.pre_conditions ?? []} onChange={(v) => set('pre_conditions', v)} placeholder="Cliente cadastrado" />
          </Field>
          <Field label="Saídas e pós-condições">
            <StringList value={form.post_conditions ?? []} onChange={(v) => set('post_conditions', v)} placeholder="O pagamento é registrado" />
          </Field>
          <Field label="Fluxo de eventos principal">
            <StringList ordered value={form.main_flow ?? []} onChange={(v) => set('main_flow', v)} placeholder="Recebe webhook" />
          </Field>
          <Field label="Fluxos alternativos">
            <StringList value={form.alternate_flows ?? []} onChange={(v) => set('alternate_flows', v)} placeholder="Login social" />
          </Field>
          <Field label="Exceções">
            <StringList value={form.exceptions ?? []} onChange={(v) => set('exceptions', v)} placeholder="Cartão recusado → notifica" />
          </Field>
          <Field label="Regras de negócio">
            <StringList value={form.business_rules ?? []} onChange={(v) => set('business_rules', v)} placeholder="Senhas sempre com hash" />
          </Field>
          <Field label="Critérios de aceite" hint="Given-When-Then. Vazio = derivado do fluxo principal.">
            <StringList value={form.acceptance ?? []} onChange={(v) => set('acceptance', v)}
              placeholder="**Given** … **When** … **Then** …" />
          </Field>
        </div>
      </div>
    </Modal>
  )
}
