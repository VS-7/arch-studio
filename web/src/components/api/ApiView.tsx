// Contratos de API (RF006). A tabela é o espelho editável de api/endpoints.yaml.

import { FileCode2, Network, Pencil, Plus, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { Endpoint, Snapshot } from '../../lib/types'
import { Badge, Button, Card, EmptyState, Field, Input, Modal, Select, Textarea, useToast } from '../ui'

const METHOD_COLOR: Record<string, string> = {
  GET: '#10b981', POST: '#3b82f6', PUT: '#f59e0b',
  PATCH: '#a855f7', DELETE: '#ef4444', HEAD: '#64748b', OPTIONS: '#64748b',
}

const EMPTY: Endpoint = {
  id: '', method: 'GET', path: '/api/v1/', summary: '', auth: '', request: '', response: '', status_codes: [200],
}

export function ApiView({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const [editing, setEditing] = useState<Endpoint | null>(null)
  const [exporting, setExporting] = useState(false)

  const exportOpenAPI = async () => {
    setExporting(true)
    try {
      const res = await api.exportOpenAPI()
      toast('success', `OpenAPI 3.1 exportado em ${res.file_path}`)
    } catch (err) { toast('error', (err as Error).message) } finally { setExporting(false) }
  }

  const remove = async (id: string) => {
    if (!window.confirm('Remover este contrato?')) return
    try {
      await api.deleteEndpoint(id)
      toast('success', 'Contrato removido')
    } catch (err) { toast('error', (err as Error).message) }
  }

  const save = async (ep: Endpoint) => {
    try {
      await api.upsertEndpoint(ep)
      toast('success', `${ep.method} ${ep.path} salvo`)
      setEditing(null)
    } catch (err) { toast('error', (err as Error).message) }
  }

  const label = (id?: string) =>
    id ? snapshot.diagram.nodes.find((n) => n.id === id)?.data.label ?? id : '—'

  return (
    <div className="mx-auto h-full w-full max-w-6xl overflow-y-auto px-5 py-5">
      <Card
        title={`Contratos de API (${snapshot.endpoints.endpoints.length})`}
        actions={
          <>
            <Button size="sm" variant="ghost" icon={FileCode2} loading={exporting} onClick={() => void exportOpenAPI()}>
              Exportar OpenAPI 3.1
            </Button>
            <Button size="sm" icon={Plus} onClick={() => setEditing({ ...EMPTY })}>Novo contrato</Button>
          </>
        }
        dense
      >
        {snapshot.endpoints.endpoints.length === 0 ? (
          <EmptyState icon={Network} title="Nenhum contrato declarado"
            description="Contratos podem ser criados aqui, pela aresta do canvas ou por um agente de IA via connect_nodes. Todos acabam no mesmo api/endpoints.yaml."
            action={<Button variant="primary" icon={Plus} onClick={() => setEditing({ ...EMPTY })}>Criar contrato</Button>} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-app text-left text-[10.5px] uppercase tracking-wider text-muted-app">
                  <th className="px-4 py-2 font-semibold">Método</th>
                  <th className="px-4 py-2 font-semibold">Rota</th>
                  <th className="px-4 py-2 font-semibold">Auth</th>
                  <th className="px-4 py-2 font-semibold">Origem → Destino</th>
                  <th className="px-4 py-2 font-semibold">Resumo</th>
                  <th className="px-4 py-2" />
                </tr>
              </thead>
              <tbody>
                {snapshot.endpoints.endpoints.map((ep) => (
                  <tr key={ep.id} className="group border-b border-app/60 last:border-0 hover:surface-3">
                    <td className="px-4 py-2">
                      <Badge color={METHOD_COLOR[ep.method.toUpperCase()] ?? '#64748b'}>{ep.method.toUpperCase()}</Badge>
                    </td>
                    <td className="px-4 py-2 font-mono text-[12px] text-app">{ep.path}</td>
                    <td className="px-4 py-2 text-[12px] text-muted-app">{ep.auth || '—'}</td>
                    <td className="px-4 py-2 text-[12px] text-muted-app">
                      {label(ep.source)} <span className="opacity-50">→</span> {label(ep.target)}
                    </td>
                    <td className="max-w-xs truncate px-4 py-2 text-[12px] text-muted-app">{ep.summary || '—'}</td>
                    <td className="px-2 py-2">
                      <div className="flex justify-end gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
                        <Button variant="ghost" size="icon" onClick={() => setEditing(ep)} aria-label="Editar"><Pencil size={13} /></Button>
                        <Button variant="ghost" size="icon" className="text-rose-400" aria-label="Remover"
                          onClick={() => void remove(ep.id)}><Trash2 size={13} /></Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {editing && <EndpointModal endpoint={editing} snapshot={snapshot} onClose={() => setEditing(null)} onSave={save} />}
    </div>
  )
}

function EndpointModal({ endpoint, snapshot, onClose, onSave }: {
  endpoint: Endpoint; snapshot: Snapshot; onClose: () => void; onSave: (ep: Endpoint) => Promise<void>
}) {
  const [form, setForm] = useState<Endpoint>(endpoint)
  useEffect(() => setForm(endpoint), [endpoint])
  const nodes = snapshot.diagram.nodes.filter((n) => n.type !== 'group')

  return (
    <Modal open onClose={onClose} wide
      title={endpoint.id ? 'Editar contrato' : 'Novo contrato de API'}
      description="Contratos alimentam o AI-PRD e a exportação OpenAPI 3.1."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" onClick={() => void onSave(form)} disabled={!form.path.trim()}>Salvar</Button>
      </>}>
      <div className="space-y-3">
        <div className="grid grid-cols-4 gap-2.5">
          <Field label="Método">
            <Select value={form.method} onChange={(e) => setForm({ ...form, method: e.target.value })}>
              {Object.keys(METHOD_COLOR).map((m) => <option key={m} value={m}>{m}</option>)}
            </Select>
          </Field>
          <Field label="Rota" className="col-span-3">
            <Input value={form.path} placeholder="/api/v1/payments/charge" className="font-mono"
              onChange={(e) => setForm({ ...form, path: e.target.value })} />
          </Field>
        </div>
        <Field label="Resumo">
          <Input value={form.summary} placeholder="Cria uma cobrança no provedor de pagamentos"
            onChange={(e) => setForm({ ...form, summary: e.target.value })} />
        </Field>
        <div className="grid grid-cols-3 gap-2.5">
          <Field label="Componente de origem">
            <Select value={form.source ?? ''} onChange={(e) => setForm({ ...form, source: e.target.value })}>
              <option value="">— não definido —</option>
              {nodes.map((n) => <option key={n.id} value={n.id}>{n.data.label}</option>)}
            </Select>
          </Field>
          <Field label="Componente de destino">
            <Select value={form.target ?? ''} onChange={(e) => setForm({ ...form, target: e.target.value })}>
              <option value="">— não definido —</option>
              {nodes.map((n) => <option key={n.id} value={n.id}>{n.data.label}</option>)}
            </Select>
          </Field>
          <Field label="Autenticação">
            <Select value={form.auth ?? ''} onChange={(e) => setForm({ ...form, auth: e.target.value })}>
              <option value="">— pública —</option>
              <option value="bearer">Bearer (JWT)</option>
              <option value="api_key">API Key</option>
              <option value="oauth2">OAuth2</option>
              <option value="mtls">mTLS</option>
            </Select>
          </Field>
        </div>
        <div className="grid grid-cols-2 gap-2.5">
          <Field label="Request">
            <Textarea rows={3} className="font-mono text-xs" value={form.request}
              placeholder='{"email": string, "password": string}'
              onChange={(e) => setForm({ ...form, request: e.target.value })} />
          </Field>
          <Field label="Response">
            <Textarea rows={3} className="font-mono text-xs" value={form.response}
              placeholder='{"token": string, "expires_in": number}'
              onChange={(e) => setForm({ ...form, response: e.target.value })} />
          </Field>
        </div>
        <Field label="Status codes" hint="Separados por vírgula.">
          <Input value={(form.status_codes ?? []).join(', ')} placeholder="200, 401, 422"
            onChange={(e) => setForm({
              ...form,
              status_codes: e.target.value.split(',').map((s) => Number(s.trim())).filter((n) => Number.isFinite(n) && n > 0),
            })} />
        </Field>
      </div>
    </Modal>
  )
}
