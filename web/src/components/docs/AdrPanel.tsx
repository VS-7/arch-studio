// Architecture Decision Records (RF003), no formato Nygard/MADR.

import { GitBranch, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { renderMarkdown } from '../../lib/markdown'
import type { ADR, Snapshot } from '../../lib/types'
import { Badge, Button, Card, EmptyState, Field, Input, Modal, Select, Textarea, useToast } from '../ui'

const STATUS_COLOR: Record<string, string> = {
  Aceito: '#10b981', Proposto: '#f59e0b', Rejeitado: '#ef4444',
  Substituído: '#64748b', Depreciado: '#64748b',
}

const EMPTY: ADR = { id: '', title: '', status: 'Proposto', context: '', decision: '', consequences: '' }

export function AdrPanel({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const [editing, setEditing] = useState<ADR | null>(null)

  const save = async (adr: ADR) => {
    try {
      const res = await api.upsertADR(adr)
      toast('success', `${res.adr.id} salvo`)
      setEditing(null)
    } catch (err) { toast('error', (err as Error).message) }
  }

  const remove = async (id: string) => {
    if (!window.confirm(`Remover ${id}?`)) return
    try {
      await api.deleteADR(id)
      toast('success', `${id} removido`)
    } catch (err) { toast('error', (err as Error).message) }
  }

  return (
    <div className="space-y-4">
      <Card title={`Decisões Arquiteturais (${snapshot.adrs.length})`}
        actions={<Button size="sm" icon={Plus} onClick={() => setEditing({ ...EMPTY })}>Nova decisão</Button>}>
        {snapshot.adrs.length === 0 ? (
          <EmptyState icon={GitBranch} title="Nenhum ADR registrado"
            description="Registre por que a arquitetura é como é. Os ADRs entram no AI-PRD para que agentes não revertam decisões deliberadas."
            action={<Button variant="primary" icon={Plus} onClick={() => setEditing({ ...EMPTY })}>Registrar decisão</Button>} />
        ) : (
          <ul className="space-y-2">
            {snapshot.adrs.map((adr) => (
              <li key={adr.id} className="group rounded-lg border border-app px-3 py-2.5 transition-colors hover:surface-3">
                <div className="flex items-start gap-2.5">
                  <span className="mt-0.5 font-mono text-[11px] font-bold text-amber-400">{adr.id}</span>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-[13px] font-semibold text-app">{adr.title}</p>
                      <Badge color={STATUS_COLOR[adr.status ?? 'Proposto'] ?? '#64748b'}>{adr.status}</Badge>
                      {adr.date && <span className="text-[11px] text-muted-app">{adr.date}</span>}
                    </div>
                    {adr.decision && (
                      <div className="md-body mt-1 line-clamp-3 text-[12px] text-muted-app"
                        dangerouslySetInnerHTML={{ __html: renderMarkdown(adr.decision) }} />
                    )}
                  </div>
                  <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button variant="ghost" size="icon" onClick={() => setEditing(adr)} aria-label="Editar"><Pencil size={13} /></Button>
                    <Button variant="ghost" size="icon" className="text-rose-400" aria-label="Remover"
                      onClick={() => void remove(adr.id)}><Trash2 size={13} /></Button>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Card>

      {editing && <AdrModal adr={editing} onClose={() => setEditing(null)} onSave={save} />}
    </div>
  )
}

function AdrModal({ adr, onClose, onSave }: { adr: ADR; onClose: () => void; onSave: (a: ADR) => Promise<void> }) {
  const [form, setForm] = useState<ADR>(adr)
  useEffect(() => setForm(adr), [adr])

  return (
    <Modal open onClose={onClose} wide
      title={form.id ? `Editar ${form.id}` : 'Nova decisão arquitetural'}
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" onClick={() => void onSave(form)} disabled={!form.title.trim()}>Salvar</Button>
      </>}>
      <div className="space-y-3">
        <div className="grid grid-cols-4 gap-2.5">
          <Field label="Identificador" hint="Vazio = próximo">
            <Input value={form.id} placeholder="ADR-002" onChange={(e) => setForm({ ...form, id: e.target.value.toUpperCase() })} />
          </Field>
          <Field label="Título" className="col-span-2">
            <Input value={form.title} placeholder="Banco vetorial para busca semântica"
              onChange={(e) => setForm({ ...form, title: e.target.value })} />
          </Field>
          <Field label="Status">
            <Select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value })}>
              <option>Proposto</option><option>Aceito</option><option>Rejeitado</option>
              <option>Substituído</option><option>Depreciado</option>
            </Select>
          </Field>
        </div>
        <Field label="Contexto" hint="Quais forças e restrições motivaram a decisão?">
          <Textarea rows={4} value={form.context} onChange={(e) => setForm({ ...form, context: e.target.value })} />
        </Field>
        <Field label="Decisão" hint="O que foi decidido, de forma afirmativa.">
          <Textarea rows={4} value={form.decision} onChange={(e) => setForm({ ...form, decision: e.target.value })} />
        </Field>
        <Field label="Consequências" hint="Ganhos, perdas e trade-offs aceitos.">
          <Textarea rows={4} value={form.consequences} onChange={(e) => setForm({ ...form, consequences: e.target.value })} />
        </Field>
      </div>
    </Modal>
  )
}
