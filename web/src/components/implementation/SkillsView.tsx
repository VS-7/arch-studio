// Skills do projeto: o que o agente de IA precisa seguir para trabalhar ali
// (segurança, organização, padronização, uso do Studio). A fonte é
// .arch/skills/<nome>/SKILL.md; a sincronização exporta para o Claude Code,
// o AGENTS.md e o Cursor.

import { Download, Plus, RefreshCw, Save, Sparkles, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { Skill } from '../../lib/types'
import { useLive } from '../../lib/useLive'
import { MarkdownView } from '../docs/MarkdownView'
import { ViewFrame } from '../shell/ViewFrame'
import { Badge, Button, EmptyState, Field, Input, Modal, Select, Spinner, Textarea, useToast } from '../ui'
import { Switch } from '../ui/switch'
import { useConfirm } from '../ui/confirm'

const CATEGORY: Record<Skill['category'], { label: string; color: string }> = {
  studio: { label: 'Uso do Studio', color: '#8b5cf6' },
  seguranca: { label: 'Segurança', color: '#e11d48' },
  organizacao: { label: 'Organização', color: '#0ea5e9' },
  padronizacao: { label: 'Padronização', color: '#10b981' },
}
const TRIGGER: Record<Skill['trigger'], string> = { sempre: 'sempre', ao_iniciar_tarefa: 'ao iniciar a tarefa', antes_do_pr: 'antes do PR' }

export function SkillsView() {
  const toast = useToast()
  const confirm = useConfirm()
  const { data, loading, reload } = useLive(() => api.skills(), ['skills_changed', 'conventions_changed'])
  const [selected, setSelected] = useState<string | null>(null)
  const [dialog, setDialog] = useState<'sync' | 'new' | null>(null)

  const list = useMemo(() => {
    if (!data) return []
    const installed = new Map(data.installed.map((s) => [s.name, s]))
    const all = [...data.installed, ...data.catalog.filter((c) => !installed.has(c.name))]
    return all
  }, [data])
  const current = list.find((s) => s.name === selected) ?? list[0]

  const install = async (names: string[], update = false) => {
    try {
      const res = await api.installSkills(names, update)
      toast('success', res.installed.length ? `${res.installed.length} skill(s) ${update ? 'atualizada(s)' : 'instalada(s)'} — sincronize com os agentes` : 'Nada a instalar')
      void reload()
    } catch (err) { toast('error', errorMessage(err)) }
  }

  const suggestedMissing = data ? data.suggested.filter((n) => !data.installed.some((s) => s.name === n)) : []

  return (
    <ViewFrame icon={Sparkles} title="Skills"
      meta={data && <>{data.installed.length} instalada(s) · stacks detectados: {data.stacks.join(', ') || 'nenhum'} ·
        exportadas para {[data.synced.claude && 'Claude Code', data.synced.agents && 'AGENTS.md', data.synced.cursor && 'Cursor'].filter(Boolean).join(', ') || 'nenhum agente ainda'}</>}
      bodyClassName="overflow-hidden"
      actions={<>
        {suggestedMissing.length > 0 && <Button size="sm" icon={Download} onClick={() => void install(suggestedMissing)}>Instalar sugeridas ({suggestedMissing.length})</Button>}
        <Button size="sm" icon={Plus} onClick={() => setDialog('new')}>Nova skill</Button>
        <Button size="sm" variant="primary" icon={RefreshCw} onClick={() => setDialog('sync')}>Sincronizar com agentes</Button>
      </>}>
      {loading && !data ? <Spinner /> : !data ? null : (
        <div className="grid h-full min-h-0" style={{ gridTemplateColumns: 'minmax(15rem, 20rem) minmax(0, 1fr)' }}>
          <div className="min-h-0 overflow-y-auto border-r">
            {(['studio', 'seguranca', 'organizacao', 'padronizacao'] as const).map((cat) => {
              const items = list.filter((s) => s.category === cat)
              if (!items.length) return null
              return (
                <div key={cat}>
                  <p className="sticky top-0 border-b bg-chrome px-3 py-1 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{CATEGORY[cat].label}</p>
                  {items.map((s) => (
                    <button key={s.name} onClick={() => setSelected(s.name)}
                      className={cn('flex w-full items-center gap-2 border-b px-3 py-1.5 text-left text-[12.5px] hover:bg-muted', current?.name === s.name && 'bg-accent')}>
                      <span className={cn('size-2 shrink-0 rounded-full', s.installed ? (s.disabled ? 'bg-muted-foreground/40' : 'bg-emerald-500') : 'border border-muted-foreground/50')} />
                      <span className={cn('min-w-0 flex-1 truncate font-mono text-[12px]', !s.installed && 'text-muted-foreground')}>{s.name}</span>
                      {!s.installed && data.suggested.includes(s.name) && <Badge color="#f59e0b">sugerida</Badge>}
                      {s.update_available && <Badge color="#3b82f6">atualizar</Badge>}
                    </button>
                  ))}
                </div>
              )
            })}
            {!!data.warnings?.length && <div className="p-3 text-[11.5px] text-destructive">{data.warnings.map((w) => <p key={w}>⚠ {w}</p>)}</div>}
          </div>
          <div className="min-h-0 overflow-y-auto">
            {current ? <SkillDetail key={current.name} skill={current} onInstall={install} onChanged={() => void reload()}
              onRemove={async () => {
                if (!(await confirm({ title: `Desinstalar ${current.name}?`, description: 'A pasta .arch/skills/' + current.name + ' é apagada. Rode a sincronização para limpar os agentes.', confirmLabel: 'Desinstalar', destructive: true }))) return
                try { await api.removeSkill(current.name); toast('success', `${current.name} desinstalada`); void reload() } catch (err) { toast('error', errorMessage(err)) }
              }} />
              : <EmptyState icon={Sparkles} title="Nenhuma skill" />}
          </div>
        </div>
      )}
      {dialog === 'sync' && data && <SyncSkillsDialog synced={data.synced} onClose={() => { setDialog(null); void reload() }} />}
      {dialog === 'new' && <NewSkillDialog onClose={(name) => { setDialog(null); if (name) { setSelected(name); void reload() } }} />}
    </ViewFrame>
  )
}

function SkillDetail({ skill, onInstall, onRemove, onChanged }: {
  skill: Skill; onInstall: (names: string[], update?: boolean) => Promise<void>; onRemove: () => Promise<void>; onChanged: () => void
}) {
  const toast = useToast()
  const [mode, setMode] = useState<'view' | 'edit'>('view')
  const [content, setContent] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (mode === 'edit' && content === null) api.getSkill(skill.name).then((r) => setContent(r.content)).catch((err) => toast('error', errorMessage(err)))
  }, [mode, content, skill.name, toast])

  const toggle = async (enabled: boolean) => {
    try { await api.setSkillEnabled(skill.name, enabled); onChanged() } catch (err) { toast('error', errorMessage(err)) }
  }
  const save = async () => {
    setSaving(true)
    try { await api.saveSkill(skill.name, content ?? ''); toast('success', `${skill.name} salva`); setMode('view'); onChanged() } catch (err) { toast('error', errorMessage(err)) } finally { setSaving(false) }
  }

  return (
    <div className="p-4">
      <div className="flex flex-wrap items-center gap-2">
        <h3 className="font-mono text-[15px] font-semibold">{skill.name}</h3>
        <Badge color={CATEGORY[skill.category].color}>{CATEGORY[skill.category].label}</Badge>
        <Badge>{TRIGGER[skill.trigger]}</Badge>
        {skill.scope && <Badge color="#64748b">escopo: {skill.scope}</Badge>}
        {skill.source && <Badge color="#64748b">{skill.source}</Badge>}
        <div className="ml-auto flex items-center gap-2">
          {skill.installed ? <>
            <label className="flex items-center gap-1.5 text-[12px]"><Switch checked={!skill.disabled} onCheckedChange={(v) => void toggle(v)} />Ativa</label>
            {skill.update_available && <Button size="sm" onClick={() => void onInstall([skill.name], true)}>Atualizar do catálogo</Button>}
            <Button size="sm" variant={mode === 'edit' ? 'secondary' : 'ghost'} onClick={() => setMode(mode === 'edit' ? 'view' : 'edit')}>{mode === 'edit' ? 'Visualizar' : 'Editar'}</Button>
            <Button size="sm" variant="ghost" icon={Trash2} onClick={() => void onRemove()}>Desinstalar</Button>
          </> : <Button size="sm" variant="primary" icon={Download} onClick={() => void onInstall([skill.name])}>Instalar no projeto</Button>}
        </div>
      </div>
      <p className="mt-2 text-[13px] text-muted-foreground">{skill.description}</p>
      {(skill.applies_to?.tiers?.length || skill.applies_to?.stacks?.length) ? (
        <p className="mt-1 text-[12px] text-muted-foreground">Vale para {[
          skill.applies_to?.tiers?.length && `tiers ${skill.applies_to.tiers.join(', ')}`,
          skill.applies_to?.stacks?.length && `stacks ${skill.applies_to.stacks.join(', ')}`,
        ].filter(Boolean).join(' e ')}.</p>
      ) : null}

      {mode === 'edit' ? (
        <div className="mt-3 space-y-2">
          {content === null ? <Spinner /> : <Textarea className="min-h-[28rem] font-mono text-[12px]" value={content} onChange={(e) => setContent(e.target.value)} />}
          <div className="flex justify-end gap-2">
            <Button variant="ghost" onClick={() => { setMode('view'); setContent(null) }}>Descartar</Button>
            <Button variant="primary" icon={Save} loading={saving} onClick={() => void save()}>Salvar</Button>
          </div>
        </div>
      ) : (
        <>
          {!!skill.checks?.length && (
            <div className="mt-3 overflow-hidden rounded-md border">
              <table className="w-full text-[12px]">
                <thead className="bg-muted text-left text-[11px] text-muted-foreground"><tr><th className="px-2.5 py-1.5">Check</th><th className="px-2.5 py-1.5">Verificação</th><th className="px-2.5 py-1.5">Comando</th></tr></thead>
                <tbody>
                  {skill.checks.map((c) => (
                    <tr key={c.id} className="border-t">
                      <td className="px-2.5 py-1.5 align-top"><code className="font-mono text-[11px]">{c.id}</code>{c.required && <Badge className="ml-1" color="#e11d48">obrigatório</Badge>}</td>
                      <td className="px-2.5 py-1.5 align-top">{c.text}</td>
                      <td className="px-2.5 py-1.5 align-top font-mono text-[11px] text-muted-foreground">{c.command}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
          <div className="prose-sm mt-4 max-w-none"><MarkdownView source={skill.body} /></div>
        </>
      )}
    </div>
  )
}

function SyncSkillsDialog({ synced, onClose }: { synced: { claude: boolean; agents: boolean; cursor: boolean }; onClose: () => void }) {
  const toast = useToast()
  const [targets, setTargets] = useState({ claude: true, agents: true, cursor: synced.cursor })
  const [busy, setBusy] = useState<'sync' | 'claude' | 'cursor' | null>(null)
  const sync = async () => {
    setBusy('sync')
    try {
      const res = await api.syncSkills(Object.entries(targets).filter(([, v]) => v).map(([k]) => k))
      toast('success', `Skills sincronizadas: ${res.written.length} escrito(s), ${res.removed.length} removido(s)`)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(null) }
  }
  const setup = async (agent: 'claude' | 'cursor') => {
    setBusy(agent)
    try {
      const res = await api.agentSetup(agent)
      toast('success', `${agent === 'claude' ? 'Claude Code' : 'Cursor'} configurado: ${res.written.join(', ')}`)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(null) }
  }
  return (
    <Modal open onClose={onClose} title="Levar as skills aos agentes"
      description="Escreve só arquivos gerados pelo Studio (marcados como tal) e o bloco gerenciado do AGENTS.md; o resto dos arquivos fica intacto."
      footer={<><Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={busy === 'sync'} onClick={() => void sync()}>Sincronizar</Button></>}>
      <div className="space-y-2 text-[12.5px]">
        <label className="flex gap-2"><input type="checkbox" checked={targets.claude} onChange={(e) => setTargets({ ...targets, claude: e.target.checked })} />
          <span><b>Claude Code</b> — <code className="font-mono">.claude/skills/&lt;nome&gt;/SKILL.md</code></span></label>
        <label className="flex gap-2"><input type="checkbox" checked={targets.agents} onChange={(e) => setTargets({ ...targets, agents: e.target.checked })} />
          <span><b>AGENTS.md</b> — protocolo do Studio, convenção e tabela das skills (lido pela maioria dos agentes)</span></label>
        <label className="flex gap-2"><input type="checkbox" checked={targets.cursor} onChange={(e) => setTargets({ ...targets, cursor: e.target.checked })} />
          <span><b>Cursor</b> — <code className="font-mono">.cursor/rules/archcode-&lt;nome&gt;.mdc</code></span></label>
      </div>
      <div className="mt-4 rounded-md border bg-muted/50 p-2.5 text-[12px]">
        <p className="mb-2">Configuração completa do agente: MCP do projeto, hooks de retomada (Claude Code) e skills.</p>
        <div className="flex gap-2">
          <Button size="sm" loading={busy === 'claude'} onClick={() => void setup('claude')}>Configurar Claude Code</Button>
          <Button size="sm" loading={busy === 'cursor'} onClick={() => void setup('cursor')}>Configurar Cursor</Button>
        </div>
      </div>
    </Modal>
  )
}

function NewSkillDialog({ onClose }: { onClose: (name?: string) => void }) {
  const toast = useToast()
  const [form, setForm] = useState({ name: '', description: '', category: 'padronizacao', trigger: 'sempre' })
  const [busy, setBusy] = useState(false)
  const create = async () => {
    setBusy(true)
    try {
      const s = await api.createSkill(form)
      toast('success', `Skill ${s.name} criada — edite as regras e os checks`)
      onClose(s.name)
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }
  return (
    <Modal open onClose={() => onClose()} title="Nova skill do projeto"
      footer={<><Button variant="ghost" onClick={() => onClose()}>Cancelar</Button>
        <Button variant="primary" loading={busy} disabled={!form.name || !form.description} onClick={() => void create()}>Criar</Button></>}>
      <div className="space-y-3">
        <Field label="Nome" hint="Minúsculas, dígitos e hífens (vira a pasta e o nome da skill no Claude Code)">
          <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-') })} placeholder="ex.: padroes-de-api" />
        </Field>
        <Field label="O que ela garante e quando usar"><Textarea rows={3} value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} /></Field>
        <div className="grid grid-cols-2 gap-3">
          <Field label="Categoria"><Select value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })}>
            {Object.entries(CATEGORY).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}</Select></Field>
          <Field label="Carregar"><Select value={form.trigger} onChange={(e) => setForm({ ...form, trigger: e.target.value })}>
            {Object.entries(TRIGGER).map(([k, v]) => <option key={k} value={k}>{v}</option>)}</Select></Field>
        </div>
      </div>
    </Modal>
  )
}
