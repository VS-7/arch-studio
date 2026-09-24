// Inspetor do elemento selecionado. Toda alteração é gravada em disco na hora,
// pelo mesmo caminho que os agentes de IA usam via MCP.

import { Trash2, Unlink } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { NODE_META, NODE_TYPES, PROTOCOLS, SECURITY_SCHEMES, STATUS_META, TIERS, nodeMeta } from '../../lib/nodeMeta'
import type { ArchEdge, ArchNode, Snapshot } from '../../lib/types'
import { Badge, Button, Field, Input, Select, Textarea, useToast } from '../ui'

interface Props {
  snapshot: Snapshot
  selected: { id: string; kind: 'node' | 'edge' } | null
  onClose: () => void
  onFocus: (id: string) => void
}

export function Inspector({ snapshot, selected, onClose, onFocus }: Props) {
  const node = useMemo(
    () => (selected?.kind === 'node' ? snapshot.diagram.nodes.find((n) => n.id === selected.id) ?? null : null),
    [selected, snapshot.diagram.nodes],
  )
  const edge = useMemo(
    () => (selected?.kind === 'edge' ? snapshot.diagram.edges.find((e) => e.id === selected.id) ?? null : null),
    [selected, snapshot.diagram.edges],
  )

  if (node) return <NodeInspector key={node.id} node={node} snapshot={snapshot} onClose={onClose} onFocus={onFocus} />
  if (edge) return <EdgeInspector key={edge.id} edge={edge} snapshot={snapshot} onClose={onClose} />
  return (
    <div className="flex h-full items-center justify-center px-6 text-center">
      <p className="text-xs leading-relaxed text-muted-app">
        Selecione um componente ou conexão no canvas para editar seus metadados,
        complexidade e custo de infraestrutura.
      </p>
    </div>
  )
}

/* -------------------------------------------------------------------------- */

/** Campos editáveis do componente (tags como texto separado por vírgulas). */
function toNodeForm(node: ArchNode) {
  return {
    label: node.data.label,
    type: node.type as string,
    technology: node.data.technology ?? '',
    description: node.data.description ?? '',
    tier: node.data.tier ?? nodeMeta(node.type).tier,
    tags: (node.data.tags ?? []).join(', '),
    complexity: node.data.pricing?.complexity ?? 'medium',
    estimated_hours: node.data.pricing?.estimated_hours ?? 0,
    cloud_tier: node.data.pricing?.cloud_tier ?? '',
    monthly_cost: node.data.pricing?.monthly_cost ?? 0,
    executive: node.data.executive !== false,
    status: node.data.status ?? 'pending',
  }
}

function NodeInspector({ node, snapshot, onClose, onFocus }: {
  node: ArchNode; snapshot: Snapshot; onClose: () => void; onFocus: (id: string) => void
}) {
  const toast = useToast()
  const meta = nodeMeta(node.type)
  const [form, setForm] = useState(() => toNodeForm(node))
  const [saving, setSaving] = useState(false)

  useEffect(() => { setForm(toNodeForm(node)) }, [node])

  const save = async () => {
    setSaving(true)
    try {
      // PATCH parcial não zera campos: enviamos o diagrama inteiro apenas quando
      // é preciso limpar valores (tags vazias, horas zeradas, flag executive).
      const updated: ArchNode = {
        ...node,
        type: form.type as ArchNode['type'],
        data: {
          ...node.data,
          label: form.label.trim() || node.data.label,
          technology: form.technology.trim(),
          description: form.description.trim(),
          tier: form.tier as ArchNode['data']['tier'],
          tags: form.tags.split(',').map((t) => t.trim()).filter(Boolean),
          executive: form.executive,
          status: form.status as ArchNode['data']['status'],
          pricing: {
            complexity: form.complexity as 'low' | 'medium' | 'high',
            estimated_hours: Number(form.estimated_hours) || 0,
            cloud_tier: form.cloud_tier.trim(),
            monthly_cost: Number(form.monthly_cost) || 0,
          },
        },
      }
      await api.saveDiagram({
        ...snapshot.diagram,
        nodes: snapshot.diagram.nodes.map((n) => (n.id === node.id ? updated : n)),
      })
      toast('success', 'Componente atualizado')
    } catch (err) {
      toast('error', errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    // Sem confirmação: a remoção entra no histórico (Ctrl+Z desfaz).
    try {
      await api.deleteNode(node.id)
      toast('success', `"${node.data.label}" removido · Ctrl+Z desfaz`)
      onClose()
    } catch (err) {
      toast('error', errorMessage(err))
    }
  }

  const connections = snapshot.diagram.edges.filter((e) => e.source === node.id || e.target === node.id)
  const relatedTasks = snapshot.tasks.tasks.filter((t) => t.component_id === node.id)
  const cloudOptions = Object.keys(snapshot.pricing.cloud_catalog ?? {}).sort()

  return (
    <div className="flex h-full flex-col">
      <header className="flex items-start justify-between gap-2 border-b border-app px-3.5 py-2.5">
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <meta.icon size={13} style={{ color: meta.color }} />
            <span className="text-[10px] font-bold uppercase tracking-wider text-muted-app">{meta.label}</span>
          </div>
          <p className="mt-0.5 truncate text-sm font-semibold text-app">{node.data.label}</p>
          <p className="truncate font-mono text-[10px] text-muted-app">{node.id}</p>
        </div>
        <Button variant="ghost" size="sm" onClick={() => onFocus(node.id)}>Focar</Button>
      </header>

      <div className="flex-1 space-y-3 overflow-y-auto px-3.5 py-3">
        <Field label="Nome">
          <Input value={form.label} onChange={(e) => setForm({ ...form, label: e.target.value })} />
        </Field>

        <div className="grid grid-cols-2 gap-2.5">
          <Field label="Tipo">
            <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
              {NODE_TYPES.map((t) => <option key={t} value={t}>{NODE_META[t].label}</option>)}
            </Select>
          </Field>
          <Field label="Camada">
            <Select value={form.tier} onChange={(e) => setForm({ ...form, tier: e.target.value as typeof form.tier })}>
              {TIERS.map((t) => <option key={t.value} value={t.value}>{t.label}</option>)}
            </Select>
          </Field>
        </div>

        <Field label="Tecnologia" hint="Alimenta o stack detectado no AI-PRD.">
          <Input value={form.technology} placeholder="Go 1.23 / Gin"
            onChange={(e) => setForm({ ...form, technology: e.target.value })} />
        </Field>

        <Field label="Responsabilidade">
          <Textarea rows={3} value={form.description} placeholder="O que este componente faz, em uma frase."
            onChange={(e) => setForm({ ...form, description: e.target.value })} />
        </Field>

        <Field label="Tags" hint="Separadas por vírgula. 'pci-dss' e 'lgpd' geram invariantes no AI-PRD.">
          <Input value={form.tags} placeholder="critical, pci-dss"
            onChange={(e) => setForm({ ...form, tags: e.target.value })} />
        </Field>

        <div className="rounded-lg border border-app p-2.5">
          <p className="mb-2 text-[10px] font-bold uppercase tracking-wider text-muted-app">Dimensionamento</p>
          <div className="grid grid-cols-2 gap-2.5">
            <Field label="Complexidade">
              <Select value={form.complexity} onChange={(e) => setForm({ ...form, complexity: e.target.value as typeof form.complexity })}>
                <option value="low">Baixa (XS/S)</option>
                <option value="medium">Média (M)</option>
                <option value="high">Alta (L/XL)</option>
              </Select>
            </Field>
            <Field label="Horas" hint="0 = derivar do tipo">
              <Input type="number" min={0} step={1} value={form.estimated_hours}
                onChange={(e) => setForm({ ...form, estimated_hours: Number(e.target.value) })} />
            </Field>
            <Field label="Tier de nuvem">
              <Select value={form.cloud_tier} onChange={(e) => setForm({ ...form, cloud_tier: e.target.value })}>
                <option value="">— nenhum —</option>
                {cloudOptions.map((t) => <option key={t} value={t}>{t}</option>)}
              </Select>
            </Field>
            <Field label="Custo/mês" hint="0 = usar catálogo">
              <Input type="number" min={0} step={1} value={form.monthly_cost}
                onChange={(e) => setForm({ ...form, monthly_cost: Number(e.target.value) })} />
            </Field>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-2.5">
          <Field label="Status">
            <Select value={form.status} onChange={(e) => setForm({ ...form, status: e.target.value as typeof form.status })}>
              {Object.entries(STATUS_META).map(([value, m]) => <option key={value} value={value}>{m.label}</option>)}
            </Select>
          </Field>
          <Field label="Modo Pitch" hint="Visível na apresentação executiva.">
            <Select value={form.executive ? '1' : '0'} onChange={(e) => setForm({ ...form, executive: e.target.value === '1' })}>
              <option value="1">Exibir para o cliente</option>
              <option value="0">Somente engenharia</option>
            </Select>
          </Field>
        </div>

        {connections.length > 0 && (
          <div>
            <p className="mb-1.5 text-[10px] font-bold uppercase tracking-wider text-muted-app">
              Conexões ({connections.length})
            </p>
            <div className="space-y-1">
              {connections.map((e) => {
                const other = e.source === node.id ? e.target : e.source
                const label = snapshot.diagram.nodes.find((n) => n.id === other)?.data.label ?? other
                return (
                  <div key={e.id} className="flex items-center gap-1.5 rounded-md surface-3 px-2 py-1 text-[11px]">
                    <span className="text-muted-app">{e.source === node.id ? '→' : '←'}</span>
                    <span className="min-w-0 flex-1 truncate text-app">{label}</span>
                    <Badge color={meta.color}>{e.data.protocol || '—'}</Badge>
                  </div>
                )
              })}
            </div>
          </div>
        )}

        {relatedTasks.length > 0 && (
          <div>
            <p className="mb-1.5 text-[10px] font-bold uppercase tracking-wider text-muted-app">Tarefas do AI-PRD</p>
            <div className="space-y-1">
              {relatedTasks.map((t) => (
                <div key={t.id} className="flex items-center gap-2 rounded-md surface-3 px-2 py-1 text-[11px]">
                  <span className="h-1.5 w-1.5 shrink-0 rounded-full" style={{ backgroundColor: STATUS_META[t.status]?.dot }} />
                  <span className="font-mono text-[10px] text-muted-app">{t.id}</span>
                  <span className="min-w-0 flex-1 truncate text-app">{t.title}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <footer className="flex items-center gap-2 border-t border-app px-3.5 py-2.5">
        <Button variant="primary" size="sm" onClick={() => void save()} loading={saving}>Salvar</Button>
        <Button variant="ghost" size="sm" icon={Trash2} onClick={() => void remove()} className="ml-auto text-destructive">
          Remover
        </Button>
      </footer>
    </div>
  )
}

/* -------------------------------------------------------------------------- */

function EdgeInspector({ edge, snapshot, onClose }: { edge: ArchEdge; snapshot: Snapshot; onClose: () => void }) {
  const toast = useToast()
  const source = snapshot.diagram.nodes.find((n) => n.id === edge.source)
  const target = snapshot.diagram.nodes.find((n) => n.id === edge.target)
  const [form, setForm] = useState({
    protocol: edge.data.protocol ?? 'REST',
    port: edge.data.port ?? 0,
    security: edge.data.security ?? '',
    description: edge.data.description ?? '',
    complexity: edge.data.complexity ?? 'medium',
    estimated_hours: edge.data.estimated_hours ?? 0,
    executive: edge.data.executive !== false,
  })
  const [saving, setSaving] = useState(false)

  const save = async () => {
    setSaving(true)
    try {
      const updated: ArchEdge = {
        ...edge,
        animated: form.protocol.toLowerCase() === 'webhook',
        data: {
          ...edge.data,
          protocol: form.protocol,
          port: Number(form.port) || 0,
          security: form.security,
          description: form.description,
          complexity: form.complexity as 'low' | 'medium' | 'high',
          estimated_hours: Number(form.estimated_hours) || 0,
          executive: form.executive,
        },
      }
      await api.saveDiagram({
        ...snapshot.diagram,
        edges: snapshot.diagram.edges.map((e) => (e.id === edge.id ? updated : e)),
      })
      toast('success', 'Conexão atualizada')
    } catch (err) {
      toast('error', errorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const remove = async () => {
    try {
      await api.deleteEdge(edge.id)
      toast('success', 'Conexão removida · Ctrl+Z desfaz')
      onClose()
    } catch (err) {
      toast('error', errorMessage(err))
    }
  }

  const linkedEndpoints = snapshot.endpoints.endpoints.filter((e) => e.edge_id === edge.id)

  return (
    <div className="flex h-full flex-col">
      <header className="border-b border-app px-3.5 py-2.5">
        <span className="text-[10px] font-bold uppercase tracking-wider text-muted-app">Conexão</span>
        <p className="mt-0.5 text-sm font-semibold leading-tight text-app">
          {source?.data.label ?? edge.source} <span className="text-muted-app">→</span> {target?.data.label ?? edge.target}
        </p>
      </header>

      <div className="flex-1 space-y-3 overflow-y-auto px-3.5 py-3">
        <div className="grid grid-cols-2 gap-2.5">
          <Field label="Protocolo">
            <Select value={form.protocol} onChange={(e) => setForm({ ...form, protocol: e.target.value })}>
              {PROTOCOLS.map((p) => <option key={p} value={p}>{p}</option>)}
            </Select>
          </Field>
          <Field label="Porta">
            <Input type="number" min={0} value={form.port} onChange={(e) => setForm({ ...form, port: Number(e.target.value) })} />
          </Field>
        </div>

        <Field label="Segurança">
          <Select value={form.security} onChange={(e) => setForm({ ...form, security: e.target.value })}>
            {SECURITY_SCHEMES.map((s) => <option key={s || 'none'} value={s}>{s || '— não definida —'}</option>)}
          </Select>
        </Field>

        <Field label="O que trafega">
          <Textarea rows={3} value={form.description} placeholder="Leitura e gravação de pedidos e assinaturas"
            onChange={(e) => setForm({ ...form, description: e.target.value })} />
        </Field>

        <div className="grid grid-cols-2 gap-2.5">
          <Field label="Complexidade">
            <Select value={form.complexity} onChange={(e) => setForm({ ...form, complexity: e.target.value as typeof form.complexity })}>
              <option value="low">Baixa</option>
              <option value="medium">Média</option>
              <option value="high">Alta</option>
            </Select>
          </Field>
          <Field label="Horas" hint="0 = derivar do protocolo">
            <Input type="number" min={0} value={form.estimated_hours}
              onChange={(e) => setForm({ ...form, estimated_hours: Number(e.target.value) })} />
          </Field>
        </div>

        <Field label="Modo Pitch">
          <Select value={form.executive ? '1' : '0'} onChange={(e) => setForm({ ...form, executive: e.target.value === '1' })}>
            <option value="1">Exibir para o cliente</option>
            <option value="0">Somente engenharia</option>
          </Select>
        </Field>

        {linkedEndpoints.length > 0 && (
          <div>
            <p className="mb-1.5 text-[10px] font-bold uppercase tracking-wider text-muted-app">
              Contratos de API ({linkedEndpoints.length})
            </p>
            <div className="space-y-1">
              {linkedEndpoints.map((ep) => (
                <div key={ep.id} className="flex items-center gap-2 rounded-md surface-3 px-2 py-1">
                  <span className="font-mono text-[10px] font-bold text-primary">{ep.method}</span>
                  <span className="min-w-0 flex-1 truncate font-mono text-[10.5px] text-app">{ep.path}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <footer className="flex items-center gap-2 border-t border-app px-3.5 py-2.5">
        <Button variant="primary" size="sm" onClick={() => void save()} loading={saving}>Salvar</Button>
        <Button variant="ghost" size="sm" icon={Unlink} onClick={() => void remove()} className="ml-auto text-destructive">
          Desconectar
        </Button>
      </footer>
    </div>
  )
}
