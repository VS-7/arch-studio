// Painel de dimensionamento e precificação (RF011–RF014).
// Os gráficos são SVG inline: nenhuma biblioteca extra entra no bundle embutido.

import { Calculator, Cloud, Coins, FileSignature, Settings2, Timer } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { api } from '../../lib/api'
import { hours as fmtHours, money } from '../../lib/format'
import { nodeMeta } from '../../lib/nodeMeta'
import type { Estimate, PricingConfig, Snapshot } from '../../lib/types'
import { Button, Card, Field, Input, Modal, Spinner, Stat, useToast } from '../ui'

const TIER_LABEL: Record<string, string> = {
  frontend: 'Frontend', integration: 'Integração', backend: 'Backend',
  domain: 'Domínio', devops: 'DevOps', data: 'Dados',
}

const TIER_COLOR: Record<string, string> = {
  frontend: '#94a3b8', integration: '#f43f5e', backend: '#3b82f6',
  domain: '#8b5cf6', devops: '#10b981', data: '#a855f7',
}

export function PricingView({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const [estimate, setEstimate] = useState<Estimate | null>(null)
  const [loading, setLoading] = useState(true)
  const [margin, setMargin] = useState<number>(snapshot.pricing.risk_margin_percentage)
  const [configOpen, setConfigOpen] = useState(false)

  useEffect(() => { setMargin(snapshot.pricing.risk_margin_percentage) }, [snapshot.pricing.risk_margin_percentage])

  useEffect(() => {
    let active = true
    setLoading(true)
    api.estimate(margin)
      .then((est: Estimate) => { if (active) setEstimate(est) })
      .catch((err: unknown) => toast('error', (err as Error).message))
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
    // Recalcula quando o diagrama, os casos de uso ou a margem mudam.
  }, [margin, snapshot.diagram.last_modified, snapshot.use_cases.length, snapshot.pricing, toast])

  const tierData = useMemo(() => {
    if (!estimate) return []
    return Object.entries(estimate.by_tier)
      .filter(([, v]) => v > 0)
      .sort((a, b) => b[1] - a[1])
  }, [estimate])

  if (loading && !estimate) return <Spinner label="Calculando estimativa…" />
  if (!estimate) return null

  const cur = estimate.currency
  const maxTier = Math.max(...tierData.map(([, v]) => v), 1)

  return (
    <div className="mx-auto h-full w-full max-w-6xl overflow-y-auto px-5 py-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-bold text-app">Dimensionamento & Precificação</h2>
          <p className="text-xs text-muted-app">
            Derivado de {snapshot.diagram.nodes.length} componentes, {snapshot.diagram.edges.length} integrações
            e {snapshot.use_cases.length} casos de uso.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-muted-app">
            Margem
            <input type="range" min={0} max={60} step={5} value={margin}
              onChange={(e) => setMargin(Number(e.target.value))} className="w-28 accent-sky-500" />
            <span className="w-9 text-right font-semibold tabular-nums text-app">{margin}%</span>
          </label>
          <Button size="sm" variant="ghost" icon={Settings2} onClick={() => setConfigOpen(true)}>Tabela de preços</Button>
        </div>
      </div>

      <div className="mb-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <Stat label="Esforço total" value={fmtHours(estimate.total_hours)}
          sub={`${fmtHours(estimate.base_hours)} + ${fmtHours(estimate.margin_hours)} de contingência`} accent="#0ea5e9" />
        <Stat label="Investimento" value={money(cur, estimate.total_cost)}
          sub={`${money(cur, estimate.personnel_cost)} + impostos`} accent="#10b981" />
        <Stat label="Infraestrutura" value={`${money(cur, estimate.cloud_monthly_cost)}/mês`}
          sub={`${money(cur, estimate.cloud_yearly_cost)} por ano`} accent="#a855f7" />
        <Stat label="Prazo" value={`${Math.round(estimate.working_days)} dias`}
          sub={`~${estimate.calendar_months} meses · equipe de ${estimate.team_size}`} accent="#f59e0b" />
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        <Card title="Esforço por camada">
          {tierData.length === 0 ? (
            <p className="py-6 text-center text-xs text-muted-app">Nada a distribuir ainda.</p>
          ) : (
            <div className="space-y-2.5">
              {tierData.map(([tier, value]) => (
                <div key={tier}>
                  <div className="mb-1 flex items-baseline justify-between text-xs">
                    <span className="font-medium text-app">{TIER_LABEL[tier] ?? tier}</span>
                    <span className="tabular-nums text-muted-app">
                      {fmtHours(value)} · {Math.round((value / estimate.base_hours) * 100)}%
                    </span>
                  </div>
                  <div className="h-2 overflow-hidden rounded-full surface-3">
                    <div className="h-full rounded-full transition-all duration-500"
                      style={{ width: `${(value / maxTier) * 100}%`, backgroundColor: TIER_COLOR[tier] ?? '#64748b' }} />
                  </div>
                </div>
              ))}
            </div>
          )}
        </Card>

        <Card title="Distribuição por perfil">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-left text-[10.5px] uppercase tracking-wider text-muted-app">
                <th className="pb-2 font-semibold">Perfil</th>
                <th className="pb-2 text-right font-semibold">Horas</th>
                <th className="pb-2 text-right font-semibold">Valor/h</th>
                <th className="pb-2 text-right font-semibold">Subtotal</th>
              </tr>
            </thead>
            <tbody>
              {estimate.roles.map((r) => (
                <tr key={r.role} className="border-t border-app/60">
                  <td className="py-1.5 text-[12.5px] text-app">{roleLabel(r.role)}</td>
                  <td className="py-1.5 text-right tabular-nums text-[12.5px] text-muted-app">{fmtHours(r.hours)}</td>
                  <td className="py-1.5 text-right tabular-nums text-[12.5px] text-muted-app">{money(cur, r.rate)}</td>
                  <td className="py-1.5 text-right tabular-nums text-[12.5px] font-semibold text-app">{money(cur, r.subtotal)}</td>
                </tr>
              ))}
              <tr className="border-t border-app">
                <td className="py-1.5 text-[12.5px] font-semibold text-app" colSpan={3}>Desenvolvimento</td>
                <td className="py-1.5 text-right tabular-nums text-[12.5px] font-bold text-app">{money(cur, estimate.personnel_cost)}</td>
              </tr>
              <tr>
                <td className="py-1.5 text-[12.5px] text-muted-app" colSpan={3}>Impostos ({estimate.tax_percentage}%)</td>
                <td className="py-1.5 text-right tabular-nums text-[12.5px] text-muted-app">{money(cur, estimate.tax_amount)}</td>
              </tr>
              <tr className="border-t border-app">
                <td className="py-2 text-[13px] font-bold text-app" colSpan={3}>Total do projeto</td>
                <td className="py-2 text-right tabular-nums text-[14px] font-bold text-emerald-400">{money(cur, estimate.total_cost)}</td>
              </tr>
            </tbody>
          </table>
        </Card>

        {estimate.cloud_items.length > 0 && (
          <Card title="Custo mensal de infraestrutura">
            <ul className="space-y-1.5">
              {estimate.cloud_items.map((item) => {
                const node = snapshot.diagram.nodes.find((n) => n.id === item.node_id)
                const color = node ? nodeMeta(node.type).color : '#64748b'
                return (
                  <li key={item.node_id} className="flex items-center gap-2.5 rounded-lg surface-3 px-3 py-2">
                    <span className="h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: color }} />
                    <span className="min-w-0 flex-1 truncate text-[12.5px] text-app">{item.label}</span>
                    <span className="font-mono text-[11px] text-muted-app">{item.cloud_tier}</span>
                    <span className="tabular-nums text-[12.5px] font-semibold text-app">{money(cur, item.monthly_cost)}</span>
                  </li>
                )
              })}
            </ul>
          </Card>
        )}

        <Card title="De onde vem cada hora">
          <div className="max-h-80 space-y-1 overflow-y-auto">
            {estimate.items.map((item) => (
              <div key={`${item.kind}-${item.id}`} className="flex items-center gap-2 rounded-md px-2 py-1 text-[12px] hover:surface-3">
                <span className="w-16 shrink-0 text-[10px] uppercase tracking-wider text-muted-app">
                  {item.kind === 'node' ? 'comp' : item.kind === 'edge' ? 'integr' : 'caso'}
                </span>
                <span className="min-w-0 flex-1 truncate text-app">{item.label}</span>
                {item.explicit && <span className="text-[10px] text-sky-400" title="Horas definidas manualmente">manual</span>}
                <span className="tabular-nums text-muted-app">{fmtHours(item.hours)}</span>
              </div>
            ))}
          </div>
        </Card>
      </div>

      {estimate.warnings && estimate.warnings.length > 0 && (
        <div className="mt-4 rounded-xl border border-amber-500/40 bg-amber-500/10 px-4 py-3">
          <p className="text-xs font-semibold text-amber-300">Avisos da estimativa</p>
          <ul className="mt-1 list-disc space-y-0.5 pl-4 text-[11.5px] text-amber-200/90">
            {estimate.warnings.map((w, i) => <li key={i}>{w}</li>)}
          </ul>
        </div>
      )}

      <PricingConfigModal open={configOpen} onClose={() => setConfigOpen(false)} config={snapshot.pricing} />
    </div>
  )
}

function roleLabel(role: string): string {
  const labels: Record<string, string> = {
    tech_lead: 'Tech Lead', senior_engineer: 'Engenheiro Sênior', pleno_engineer: 'Engenheiro Pleno',
    junior_engineer: 'Engenheiro Júnior', cloud_architect: 'Arquiteto de Nuvem',
    designer: 'Designer de Produto', qa_engineer: 'Engenheiro de QA',
  }
  return labels[role] ?? role.replace(/_/g, ' ')
}

function PricingConfigModal({ open, onClose, config }: { open: boolean; onClose: () => void; config: PricingConfig }) {
  const toast = useToast()
  const [form, setForm] = useState<PricingConfig>(config)
  const [saving, setSaving] = useState(false)
  useEffect(() => setForm(config), [config])

  const setRate = (role: string, value: number) =>
    setForm({ ...form, hourly_rates: { ...form.hourly_rates, [role]: value } })
  const setShare = (role: string, value: number) =>
    setForm({ ...form, role_distribution: { ...form.role_distribution, [role]: value / 100 } })

  const save = async () => {
    setSaving(true)
    try {
      await api.savePricing(form)
      toast('success', '.arch/pricing.yaml atualizado')
      onClose()
    } catch (err) { toast('error', (err as Error).message) } finally { setSaving(false) }
  }

  const roles = Object.keys(form.hourly_rates ?? {}).sort()
  const shareTotal = Object.values(form.role_distribution ?? {}).reduce((a, b) => a + b, 0)

  return (
    <Modal open={open} onClose={onClose} wide title=".arch/pricing.yaml"
      description="Estas tabelas alimentam a estimativa, a proposta comercial e a ferramenta MCP calculate_project_estimate."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={saving} onClick={() => void save()}>Salvar</Button>
      </>}>
      <div className="space-y-4">
        <div className="grid grid-cols-4 gap-2.5">
          <Field label="Moeda">
            <Input value={form.currency} onChange={(e) => setForm({ ...form, currency: e.target.value.toUpperCase() })} />
          </Field>
          <Field label="Margem (%)">
            <Input type="number" min={0} max={100} value={form.risk_margin_percentage}
              onChange={(e) => setForm({ ...form, risk_margin_percentage: Number(e.target.value) })} />
          </Field>
          <Field label="Impostos (%)">
            <Input type="number" min={0} max={100} value={form.tax_percentage}
              onChange={(e) => setForm({ ...form, tax_percentage: Number(e.target.value) })} />
          </Field>
          <Field label="Tamanho da equipe">
            <Input type="number" min={1} step={0.5} value={form.team_size}
              onChange={(e) => setForm({ ...form, team_size: Number(e.target.value) })} />
          </Field>
        </div>

        <div>
          <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-app">
            Perfis, valor/hora e distribuição {Math.abs(shareTotal - 1) > 0.01 && (
              <span className="ml-2 text-amber-400">(soma {Math.round(shareTotal * 100)}% — será normalizada)</span>
            )}
          </p>
          <div className="space-y-1.5">
            {roles.map((role) => (
              <div key={role} className="grid grid-cols-[1fr_auto_auto] items-center gap-2.5">
                <span className="text-[12.5px] text-app">{roleLabel(role)}</span>
                <Input type="number" min={0} step={10} className="w-28" value={form.hourly_rates[role] ?? 0}
                  onChange={(e) => setRate(role, Number(e.target.value))} />
                <div className="flex items-center gap-1">
                  <Input type="number" min={0} max={100} className="w-20"
                    value={Math.round((form.role_distribution?.[role] ?? 0) * 100)}
                    onChange={(e) => setShare(role, Number(e.target.value))} />
                  <span className="text-xs text-muted-app">%</span>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-app">Horas base por tipo de componente</p>
            <div className="space-y-1.5">
              {Object.keys(form.node_type_hours ?? {}).sort().map((type) => (
                <div key={type} className="flex items-center gap-2">
                  <span className="min-w-0 flex-1 truncate text-[12px] text-app">{nodeMeta(type).label}</span>
                  <Input type="number" min={0} className="w-24" value={form.node_type_hours[type] ?? 0}
                    onChange={(e) => setForm({
                      ...form, node_type_hours: { ...form.node_type_hours, [type]: Number(e.target.value) },
                    })} />
                </div>
              ))}
            </div>
          </div>
          <div>
            <p className="mb-2 text-[11px] font-semibold uppercase tracking-wider text-muted-app">Custo mensal por tier de nuvem</p>
            <div className="max-h-64 space-y-1.5 overflow-y-auto pr-1">
              {Object.keys(form.cloud_catalog ?? {}).sort().map((tier) => (
                <div key={tier} className="flex items-center gap-2">
                  <span className="min-w-0 flex-1 truncate font-mono text-[11px] text-app">{tier}</span>
                  <Input type="number" min={0} className="w-24" value={form.cloud_catalog[tier] ?? 0}
                    onChange={(e) => setForm({
                      ...form, cloud_catalog: { ...form.cloud_catalog, [tier]: Number(e.target.value) },
                    })} />
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </Modal>
  )
}

export { Calculator, Cloud, Coins, FileSignature, Timer }
