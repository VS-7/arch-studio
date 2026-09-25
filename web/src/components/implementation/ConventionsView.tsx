// Convenções e Git: o .arch/conventions.yaml editável com prévia ao vivo
// (branch, commit, PR, tag), o nível de autonomia da IA e as ferramentas que
// aplicam a convenção no repositório — hooks, lint, reconciliação do quadro,
// changelog, template de PR, agentes e diagnóstico.

import {
  Bot, FileText, GitBranch, GitPullRequestArrow, RefreshCw, Save, ShieldCheck, Stethoscope, Wrench,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type {
  Autonomy, Conventions, ConventionsPreview, DoctorReport, GitOverview, LintIssue, ReconcileChange,
} from '../../lib/types'
import { useLive } from '../../lib/useLive'
import { ViewFrame } from '../shell/ViewFrame'
import { Badge, Button, Card, Field, Input, Select, Spinner, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'
import { STATUS } from './planMeta'

const AUTONOMY: { value: Autonomy; label: string; text: string }[] = [
  { value: 'assistido', label: 'Assistido', text: 'O agente propõe cada commit; um humano confirma. Push e PR são humanos.' },
  { value: 'supervisionado', label: 'Supervisionado', text: 'O agente faz commits locais; publicar a branch e abrir o PR é com um humano.' },
  { value: 'autonomo', label: 'Autônomo', text: 'O agente publica a branch e abre o PR em rascunho. Merge, tag e release continuam humanos.' },
]

export function ConventionsView() {
  const toast = useToast()
  const confirm = useConfirm()
  const conv = useLive(() => api.getConventions(), ['conventions_changed'])
  const git = useLive(() => api.git(), ['git_changed', 'plan_changed'])
  const [form, setForm] = useState<Conventions | null>(null)
  const [preview, setPreview] = useState<ConventionsPreview | null>(null)
  const [saving, setSaving] = useState(false)
  const timer = useRef<number | null>(null)

  useEffect(() => {
    if (conv.data && !form) { setForm(conv.data.conventions); setPreview(conv.data.preview) }
  }, [conv.data, form])

  // Prévia ao vivo enquanto edita (sem gravar).
  useEffect(() => {
    if (!form) return
    if (timer.current) window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => { api.previewConventions(form).then(setPreview).catch(() => {}) }, 250)
    return () => { if (timer.current) window.clearTimeout(timer.current) }
  }, [form])

  if (!form) return <Spinner label="Carregando convenções…" />
  const dirty = JSON.stringify(form) !== JSON.stringify(conv.data?.conventions)
  const set = <K extends keyof Conventions>(key: K, patch: Partial<Conventions[K]>) => setForm({ ...form, [key]: { ...(form[key] as object), ...patch } })
  const csv = (list?: string[]) => (list ?? []).join(', ')
  const fromCsv = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean)

  const save = async () => {
    setSaving(true)
    try {
      const saved = await api.saveConventions(form)
      setForm(saved)
      toast('success', 'Convenções salvas — a skill convencoes-git foi regenerada')
      void conv.reload()
    } catch (err) { toast('error', errorMessage(err)) } finally { setSaving(false) }
  }

  const applyPreset = async (preset: string) => {
    if (!(await confirm({ title: 'Trocar de preset?', description: 'Os padrões de branch, commit e PR voltam aos do preset escolhido. O resto do arquivo também volta ao padrão.', confirmLabel: 'Trocar' }))) return
    try {
      const c = await api.initConventions(preset, true)
      setForm(c)
      toast('success', `Preset ${preset} aplicado`)
    } catch (err) { toast('error', errorMessage(err)) }
  }

  return (
    <ViewFrame icon={GitPullRequestArrow} title="Convenções e Git"
      meta={<>{conv.data?.exists ? '.arch/conventions.yaml' : 'Sem .arch/conventions.yaml — valem os padrões abaixo até salvar'} · preset {form.preset}</>}
      actions={<Button size="sm" variant="primary" icon={Save} loading={saving} disabled={!dirty && !!conv.data?.exists} onClick={() => void save()}>Salvar</Button>}>
      <div className="grid gap-3 p-3 @5xl:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)]">
        <div className="space-y-3">
          <Card title="Prévia">
            {preview && (
              <div className="grid gap-x-3 gap-y-1 text-[12.5px]" style={{ gridTemplateColumns: '9rem minmax(0,1fr)' }}>
                {([['Branch', preview.branch], ['Branch fora de sprint', preview.branch_off_sprint], ['Commit', preview.commit],
                  ['Commit fora de sprint', preview.commit_off_sprint], ['Pull request', preview.pr_title], ['Tag de sprint', preview.sprint_tag]] as const).map(([k, v]) => (
                  <div key={k} className="contents"><span className="text-muted-foreground">{k}</span><code className="truncate font-mono text-[12px]">{v}</code></div>
                ))}
                {preview.errors?.map((e) => <p key={e} className="col-span-2 text-destructive">⚠ {e}</p>)}
              </div>
            )}
          </Card>

          <Card title="Autonomia da IA">
            <div className="grid gap-2 @2xl:grid-cols-3">
              {AUTONOMY.map((a) => (
                <label key={a.value} className={`cursor-pointer rounded-md border p-2.5 text-[12px] ${form.ai.autonomy === a.value ? 'border-foreground bg-accent' : 'hover:bg-muted'}`}>
                  <input type="radio" className="sr-only" checked={form.ai.autonomy === a.value} onChange={() => set('ai', { autonomy: a.value })} />
                  <p className="font-semibold">{a.label}</p><p className="mt-0.5 text-muted-foreground">{a.text}</p>
                </label>
              ))}
            </div>
          </Card>

          <Card title="Git">
            <div className="grid gap-3" style={{ gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)' }}>
              <Field label="Preset">
                <Select value={form.preset} onChange={(e) => void applyPreset(e.target.value)}>
                  <option value="archcode-sprint">ArchCode Sprint (Sprint 01 - …)</option>
                  <option value="conventional-commits">Conventional Commits (feat: …)</option>
                </Select>
              </Field>
              <div className="grid grid-cols-2 gap-3">
                <Field label="Branch principal"><Input value={form.git.main_branch} onChange={(e) => set('git', { main_branch: e.target.value })} /></Field>
                <Field label="Remoto"><Input value={form.git.remote} onChange={(e) => set('git', { remote: e.target.value })} /></Field>
              </div>
              <Field label="Branch" hint="{sprint} {id} {slug} {type} {kind}"><Input className="font-mono text-[12px]" value={form.git.branch} onChange={(e) => set('git', { branch: e.target.value })} /></Field>
              <Field label="Branch fora de sprint"><Input className="font-mono text-[12px]" value={form.git.branch_off_sprint} onChange={(e) => set('git', { branch_off_sprint: e.target.value })} /></Field>
              <Field label="Commit" hint="{sprint} {summary} {id} {type}"><Input className="font-mono text-[12px]" value={form.git.commit} onChange={(e) => set('git', { commit: e.target.value })} /></Field>
              <Field label="Commit fora de sprint" hint="{kind}: Hotfix, Chore…"><Input className="font-mono text-[12px]" value={form.git.commit_off_sprint} onChange={(e) => set('git', { commit_off_sprint: e.target.value })} /></Field>
              <Field label="Tipos fora de sprint (vírgula)"><Input value={csv(form.git.off_sprint_kinds)} onChange={(e) => set('git', { off_sprint_kinds: fromCsv(e.target.value) })} /></Field>
              <div className="grid grid-cols-2 gap-3">
                <Field label="Máx. caracteres"><Input type="number" min={20} value={form.git.max_subject} onChange={(e) => set('git', { max_subject: Number(e.target.value) })} /></Field>
                <label className="flex items-end gap-2 pb-2 text-[12px]"><input type="checkbox" checked={form.git.require_id} onChange={(e) => set('git', { require_id: e.target.checked })} />Exigir [ID] do item</label>
              </div>
              <Field label="Verbos aceitos (vírgula; vazio = qualquer)" className="col-span-2"><Input value={csv(form.git.verbs)} onChange={(e) => set('git', { verbs: fromCsv(e.target.value) })} /></Field>
            </div>
          </Card>

          <Card title="Pull request, tags e planejamento">
            <div className="grid gap-3" style={{ gridTemplateColumns: 'repeat(3, minmax(0,1fr))' }}>
              <Field label="Título do PR" className="col-span-2"><Input className="font-mono text-[12px]" value={form.pull_request.title} onChange={(e) => set('pull_request', { title: e.target.value })} /></Field>
              <Field label="Um PR por"><Select value={form.pull_request.granularity} onChange={(e) => set('pull_request', { granularity: e.target.value as 'task' | 'story' })}>
                <option value="task">Tarefa</option><option value="story">História</option></Select></Field>
              <Field label="Merge"><Select value={form.pull_request.merge_strategy} onChange={(e) => set('pull_request', { merge_strategy: e.target.value })}>
                <option value="squash">Squash</option><option value="rebase">Rebase</option><option value="merge">Merge commit</option></Select></Field>
              <Field label="Tag de sprint"><Input className="font-mono text-[12px]" value={form.tags.sprint} onChange={(e) => set('tags', { sprint: e.target.value })} /></Field>
              <Field label="Tag de release"><Input className="font-mono text-[12px]" value={form.tags.release} onChange={(e) => set('tags', { release: e.target.value })} /></Field>
              <Field label="Dias úteis por sprint"><Input type="number" min={1} value={form.planning.sprint_days} onChange={(e) => set('planning', { sprint_days: Number(e.target.value) })} /></Field>
              <Field label="Fatiamento do backlog"><Select value={form.planning.slicing} onChange={(e) => set('planning', { slicing: e.target.value as 'component' | 'hybrid' })}>
                <option value="hybrid">Híbrido (fundação + fatias por requisito)</option><option value="component">Por componente (AI-PRD)</option></Select></Field>
              <Field label="Reserva parada após (dias úteis)"><Input type="number" min={1} value={form.planning.stale_days} onChange={(e) => set('planning', { stale_days: Number(e.target.value) })} /></Field>
            </div>
          </Card>
        </div>

        <div className="space-y-3">
          <GitPanel overview={git} />
          <ToolsPanel />
        </div>
      </div>
    </ViewFrame>
  )
}

function GitPanel({ overview }: { overview: { data: GitOverview | null; reload: () => Promise<void> } }) {
  const toast = useToast()
  const g = overview.data
  const [lint, setLint] = useState<{ checked: number; issues: LintIssue[]; ok: boolean } | null>(null)
  const [busy, setBusy] = useState<string | null>(null)
  const run = async (key: string, fn: () => Promise<void>) => {
    setBusy(key)
    try { await fn() } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(null) }
  }
  if (!g) return <Card title="Repositório"><Spinner /></Card>
  if (!g.available) return <Card title="Repositório"><p className="text-[12.5px] text-muted-foreground">O projeto não é um repositório Git (ou o git não está instalado). Planejamento e memória funcionam; convenções, reservas por branch e hooks precisam do Git.</p></Card>
  const st = g.status!
  return (
    <Card title="Repositório">
      <div className="space-y-1.5 text-[12.5px]">
        <p className="flex flex-wrap items-center gap-1.5"><GitBranch size={13} /><code className="font-mono">{st.branch || '(detached)'}</code>
          {st.upstream && <span className="text-muted-foreground">→ {st.upstream} (+{st.ahead}/−{st.behind})</span>}
          {!!g.branch_issues?.length && <Badge color="#ef4444">fora da convenção</Badge>}</p>
        {g.current_task && <p>Tarefa da branch: <b>{g.current_task.id}</b> {g.current_task.title} <Badge color={STATUS[g.current_task.status].color}>{STATUS[g.current_task.status].label}</Badge></p>}
        <p className="text-muted-foreground">{st.changed.length} arquivo(s) alterado(s) · {st.staged} preparado(s){g.person && ` · você: ${g.person}`}</p>
        <p className="flex flex-wrap items-center gap-1.5">Hooks: {g.hooks.length ? g.hooks.map((h) => <Badge key={h} color="#10b981">{h}</Badge>) : <span className="text-muted-foreground">não instalados</span>}
          · GitHub CLI: {g.forge ? <Badge color="#10b981">conectado</Badge> : <span className="text-muted-foreground">indisponível</span>}</p>
        {!!g.recent?.length && (
          <details><summary className="cursor-pointer text-muted-foreground">Últimos commits</summary>
            <ul className="mt-1 space-y-0.5 font-mono text-[11px]">{g.recent.map((c) => <li key={c.hash} className="truncate">{c.hash.slice(0, 7)} {c.subject}</li>)}</ul>
          </details>
        )}
      </div>
      <div className="mt-3 flex flex-wrap gap-1.5">
        {g.hooks.length < 2
          ? <Button size="sm" icon={ShieldCheck} loading={busy === 'hooks'} onClick={() => void run('hooks', async () => {
            const r = await api.installHooks(false)
            toast('success', `Hooks instalados: ${r.installed.join(', ') || 'nenhum'}`)
            for (const n of r.notes ?? []) toast('info', n)
            void overview.reload()
          })}>Instalar hooks</Button>
          : <Button size="sm" variant="ghost" loading={busy === 'hooks'} onClick={() => void run('hooks', async () => { await api.uninstallHooks(); void overview.reload() })}>Remover hooks</Button>}
        <Button size="sm" loading={busy === 'lint'} onClick={() => void run('lint', async () => setLint(await api.gitLint()))}>Validar commits da branch</Button>
      </div>
      {lint && (
        <div className="mt-2 rounded-md border p-2 text-[12px]">
          {lint.ok && !lint.issues.length ? <p className="text-emerald-600 dark:text-emerald-400">✓ {lint.checked} verificação(ões) na convenção</p> : (
            <ul className="space-y-1">{lint.issues.map((i, k) => (
              <li key={k}><Badge color={i.level === 'error' ? '#ef4444' : '#f59e0b'}>{i.level === 'error' ? 'erro' : 'aviso'}</Badge> {i.ref && <code className="font-mono">{i.ref}</code>} <span className="font-mono">{i.subject}</span><br /><span className="text-muted-foreground">{i.message}</span></li>
            ))}</ul>
          )}
        </div>
      )}
    </Card>
  )
}

function ToolsPanel() {
  const toast = useToast()
  const [busy, setBusy] = useState<string | null>(null)
  const [reconcile, setReconcile] = useState<ReconcileChange[] | null>(null)
  const [doctor, setDoctor] = useState<DoctorReport | null>(null)
  const [ci, setCi] = useState(false)
  const run = async (key: string, fn: () => Promise<void>) => {
    setBusy(key)
    try { await fn() } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(null) }
  }
  return (
    <Card title="Ferramentas">
      <div className="space-y-3 text-[12.5px]">
        <div>
          <p className="mb-1 font-medium">Reconciliar o quadro com o Git</p>
          <p className="mb-1.5 text-muted-foreground">Commit na branch principal citando a tarefa → concluída; PR aberto → em revisão; commit na branch atual → em andamento.</p>
          <Button size="sm" icon={RefreshCw} loading={busy === 'rec'} onClick={() => void run('rec', async () => setReconcile((await api.reconcile(false)).changes))}>Verificar</Button>
          {reconcile && (reconcile.length === 0 ? <p className="mt-1.5 text-emerald-600 dark:text-emerald-400">✓ O quadro já está de acordo com o Git.</p> : (
            <div className="mt-1.5 space-y-1">
              {reconcile.map((c) => <p key={c.id}><code className="font-mono">{c.id}</code> {STATUS[c.from].label} → <b>{STATUS[c.to].label}</b> <span className="text-muted-foreground">({c.reason})</span></p>)}
              <Button size="sm" variant="primary" loading={busy === 'rec-apply'} onClick={() => void run('rec-apply', async () => {
                await api.reconcile(true); toast('success', `${reconcile.length} tarefa(s) atualizada(s)`); setReconcile(null)
              })}>Aplicar</Button>
            </div>
          ))}
        </div>
        <div>
          <p className="mb-1 font-medium">Arquivos da convenção</p>
          <div className="flex flex-wrap items-center gap-1.5">
            <Button size="sm" icon={FileText} loading={busy === 'apply'} onClick={() => void run('apply', async () => {
              const r = await api.applyConventions(ci); toast('success', `Gravado: ${r.written.join(', ')}`)
            })}>Template de PR{ci ? ' + CI' : ''}</Button>
            <label className="flex items-center gap-1.5"><input type="checkbox" checked={ci} onChange={(e) => setCi(e.target.checked)} />workflow de git lint no GitHub Actions</label>
            <Button size="sm" icon={Wrench} loading={busy === 'changelog'} onClick={() => void run('changelog', async () => {
              await api.changelog(true); toast('success', 'CHANGELOG.md atualizado')
            })}>Gerar CHANGELOG</Button>
          </div>
        </div>
        <div>
          <p className="mb-1 font-medium">Agentes de IA</p>
          <p className="mb-1.5 text-muted-foreground">MCP do projeto, hooks de retomada (o agente abre a sessão já sabendo onde parou) e skills exportadas.</p>
          <div className="flex gap-1.5">
            <Button size="sm" icon={Bot} loading={busy === 'claude'} onClick={() => void run('claude', async () => { const r = await api.agentSetup('claude'); toast('success', `Claude Code configurado: ${r.written.join(', ')}`) })}>Configurar Claude Code</Button>
            <Button size="sm" icon={Bot} loading={busy === 'cursor'} onClick={() => void run('cursor', async () => { const r = await api.agentSetup('cursor'); toast('success', `Cursor configurado: ${r.written.join(', ')}`) })}>Configurar Cursor</Button>
          </div>
        </div>
        <div>
          <p className="mb-1 font-medium">Diagnóstico</p>
          <Button size="sm" icon={Stethoscope} loading={busy === 'doctor'} onClick={() => void run('doctor', async () => setDoctor(await api.doctor({ fix: true })))}>Rodar doctor</Button>
          {doctor && (
            <div className="mt-1.5 space-y-0.5">
              {doctor.conflicts.map((c) => <p key={c} className="text-destructive">✗ conflito de merge: {c} (resolva com archcode-studio doctor --prefer ours|theirs)</p>)}
              {doctor.problems.map((p) => <p key={p} className="text-amber-600 dark:text-amber-400">⚠ {p}</p>)}
              {!doctor.conflicts.length && !doctor.problems.length && <p className="text-emerald-600 dark:text-emerald-400">✓ Backlog, sprints e memória sem problemas (derivados regenerados).</p>}
            </div>
          )}
        </div>
      </div>
    </Card>
  )
}
