// Gestão de requisitos (RF001).
//
// Nota de engenharia: o PRD sugeria um editor WYSIWYG em blocos. Optamos por um
// editor estruturado (um formulário por requisito) somado a um editor Markdown
// bruto, porque a serialização de editores WYSIWYG genéricos é lossy e quebraria
// o formato determinístico que o parser Go e o Git dependem (RNF001). O usuário
// ganha edição em blocos de verdade — e o arquivo continua estável.

import { Code2, FileText, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { Requirement, Snapshot } from '../../lib/types'
import { renderMarkdown } from '../../lib/markdown'
import { Badge, Button, Card, Field, Input, Modal, Select, Textarea, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'

const PRIORITY_COLOR: Record<string, string> = { Alta: '#f43f5e', Média: '#f59e0b', Baixa: '#64748b' }

const EMPTY: Requirement = { id: '', type: 'RF', title: '', priority: 'Média', status: 'pending', components: [], description: '' }

export function RequirementsPanel({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const confirm = useConfirm()
  const [editing, setEditing] = useState<Requirement | null>(null)
  const [rawMode, setRawMode] = useState(false)
  const [raw, setRaw] = useState('')
  const [overview, setOverview] = useState(snapshot.requirements.overview)
  const [savingOverview, setSavingOverview] = useState(false)

  useEffect(() => { setOverview(snapshot.requirements.overview) }, [snapshot.requirements.overview])

  const openRaw = async () => {
    const { raw: content } = await api.getRequirements()
    setRaw(content)
    setRawMode(true)
  }

  const saveRaw = async () => {
    try {
      await api.saveRequirementsRaw(raw)
      toast('success', 'docs/requisitos.md salvo')
      setRawMode(false)
    } catch (err) { toast('error', (err as Error).message) }
  }

  const saveOverview = async () => {
    setSavingOverview(true)
    try {
      const { raw: content } = await api.getRequirements()
      // Substitui só o bloco "Visão Geral", preservando o restante do arquivo.
      const next = content.replace(
        /(## Visão Geral\n\n)([\s\S]*?)(\n## )/,
        (_match: string, head: string, _body: string, tail: string) => `${head}${overview.trim()}\n${tail}`,
      )
      await api.saveRequirementsRaw(next)
      toast('success', 'Visão geral atualizada')
    } catch (err) { toast('error', (err as Error).message) } finally { setSavingOverview(false) }
  }

  const save = async (req: Requirement) => {
    try {
      await api.upsertRequirement(req)
      toast('success', `${req.id || 'Requisito'} salvo`)
      setEditing(null)
    } catch (err) { toast('error', (err as Error).message) }
  }

  const remove = async (id: string) => {
    if (!await confirm({ title: `Remover o requisito ${id}?`, description: 'O requisito sai de docs/requisitos.md.', confirmLabel: 'Remover', destructive: true })) return
    try {
      await api.deleteRequirement(id)
      toast('success', `${id} removido`)
    } catch (err) { toast('error', (err as Error).message) }
  }

  const functional = snapshot.requirements.requirements.filter((r) => r.type === 'RF')
  const nonFunctional = snapshot.requirements.requirements.filter((r) => r.type === 'RNF')

  const group = (title: string, items: Requirement[], kind: 'RF' | 'RNF') => (
    <Card
      title={`${title} (${items.length})`}
      actions={<Button size="sm" icon={Plus} onClick={() => setEditing({ ...EMPTY, type: kind })}>Novo</Button>}
    >
      {items.length === 0 ? (
        <p className="py-4 text-center text-xs text-muted-app">Nenhum requisito cadastrado.</p>
      ) : (
        <ul className="space-y-2">
          {items.map((r) => (
            <li key={r.id} className="group rounded-lg border border-app px-3 py-2.5 transition-colors hover:surface-3">
              <div className="flex items-start gap-2.5">
                <span className="mt-0.5 font-mono text-[11px] font-bold text-primary">{r.id}</span>
                <div className="min-w-0 flex-1">
                  <p className="text-[13px] font-semibold leading-snug text-app">{r.title}</p>
                  {r.description && (
                    <div className="md-body mt-1 text-[12px] text-muted-app"
                      dangerouslySetInnerHTML={{ __html: renderMarkdown(r.description) }} />
                  )}
                  <div className="mt-1.5 flex flex-wrap items-center gap-1.5">
                    <Badge color={PRIORITY_COLOR[r.priority ?? 'Média'] ?? '#64748b'}>{r.priority ?? 'Média'}</Badge>
                    {(r.components ?? []).map((c) => {
                      const label = snapshot.diagram.nodes.find((n) => n.id === c)?.data.label ?? c
                      return <Badge key={c}>{label}</Badge>
                    })}
                  </div>
                </div>
                <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                  <Button variant="ghost" size="icon" onClick={() => setEditing(r)} aria-label="Editar"><Pencil size={13} /></Button>
                  <Button variant="ghost" size="icon" onClick={() => void remove(r.id)} aria-label="Remover"
                    className="text-destructive"><Trash2 size={13} /></Button>
                </div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </Card>
  )

  return (
    <div className="space-y-4">
      <Card
        title="Visão Geral do Produto"
        actions={
          <>
            <Button size="sm" variant="ghost" icon={Code2} onClick={() => void openRaw()}>Markdown</Button>
            <Button size="sm" variant="primary" onClick={() => void saveOverview()} loading={savingOverview}>Salvar</Button>
          </>
        }
      >
        <Textarea rows={5} value={overview} onChange={(e) => setOverview(e.target.value)}
          placeholder="Objetivo do sistema, problema de negócio e público-alvo. Os agentes de IA leem este texto como contexto de produto." />
      </Card>

      {group('Requisitos Funcionais', functional, 'RF')}
      {group('Requisitos Não Funcionais', nonFunctional, 'RNF')}

      <RequirementModal
        requirement={editing}
        snapshot={snapshot}
        onClose={() => setEditing(null)}
        onSave={save}
      />

      <Modal open={rawMode} onClose={() => setRawMode(false)} wide
        title="docs/requisitos.md" description="Edição direta do arquivo. O parser reconhece títulos ### RF001 — Título e bullets de metadados."
        footer={<>
          <Button variant="ghost" onClick={() => setRawMode(false)}>Cancelar</Button>
          <Button variant="primary" onClick={() => void saveRaw()}>Salvar arquivo</Button>
        </>}>
        <Textarea rows={24} value={raw} onChange={(e) => setRaw(e.target.value)} className="font-mono text-xs" />
      </Modal>
    </div>
  )
}

function RequirementModal({ requirement, snapshot, onClose, onSave }: {
  requirement: Requirement | null
  snapshot: Snapshot
  onClose: () => void
  onSave: (r: Requirement) => Promise<void>
}) {
  const [form, setForm] = useState<Requirement>(requirement ?? EMPTY)
  useEffect(() => { if (requirement) setForm(requirement) }, [requirement])
  if (!requirement) return null

  const toggleComponent = (id: string) => {
    const components = form.components ?? []
    setForm({
      ...form,
      components: components.includes(id) ? components.filter((c) => c !== id) : [...components, id],
    })
  }

  return (
    <Modal open onClose={onClose}
      title={form.id ? `Editar ${form.id}` : 'Novo requisito'}
      description="Requisitos vinculados a componentes alimentam a matriz de rastreabilidade do AI-PRD."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" onClick={() => void onSave(form)} disabled={!form.title.trim()}>Salvar</Button>
      </>}>
      <div className="space-y-3">
        <div className="grid grid-cols-3 gap-2.5">
          <Field label="Identificador" hint="Vazio = próximo livre">
            <Input value={form.id} placeholder="RF005" onChange={(e) => setForm({ ...form, id: e.target.value.toUpperCase() })} />
          </Field>
          <Field label="Tipo">
            <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value as 'RF' | 'RNF' })}>
              <option value="RF">Funcional</option>
              <option value="RNF">Não funcional</option>
            </Select>
          </Field>
          <Field label="Prioridade">
            <Select value={form.priority} onChange={(e) => setForm({ ...form, priority: e.target.value })}>
              <option>Alta</option><option>Média</option><option>Baixa</option>
            </Select>
          </Field>
        </div>
        <Field label="Título">
          <Input value={form.title} placeholder="Processamento de pagamentos assíncrono"
            onChange={(e) => setForm({ ...form, title: e.target.value })} />
        </Field>
        <Field label="Descrição">
          <Textarea rows={4} value={form.description}
            placeholder="O sistema deve processar webhooks do Stripe com idempotência."
            onChange={(e) => setForm({ ...form, description: e.target.value })} />
        </Field>
        <Field label="Componentes responsáveis">
          <div className="flex flex-wrap gap-1.5 rounded-lg border border-app p-2">
            {snapshot.diagram.nodes.filter((n) => n.type !== 'group').map((n) => {
              const active = (form.components ?? []).includes(n.id)
              return (
                <button key={n.id} onClick={() => toggleComponent(n.id)}
                  className={`rounded-full border px-2.5 py-1 text-[11px] font-medium transition-colors ${
                    active ? 'border-primary bg-primary/15 text-primary' : 'border-app text-muted-app hover:surface-3'}`}>
                  {n.data.label}
                </button>
              )
            })}
            {snapshot.diagram.nodes.length === 0 && (
              <span className="text-[11px] text-muted-app">Nenhum componente no diagrama ainda.</span>
            )}
          </div>
        </Field>
      </div>
    </Modal>
  )
}

export { FileText }
