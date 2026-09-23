import { ReactFlowProvider } from '@xyflow/react'
import {
  AlertTriangle, Bot, CheckCircle2, Info, LayoutGrid, Moon, Network, Play, Plug,
  Presentation, ShieldCheck, Sun, Terminal, Wand2, Waypoints, Workflow,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from './lib/api'
import { relativeTime } from './lib/format'
import { ProjectProvider, useProject } from './lib/project'
import type { Estimate, ServerEvent } from './lib/types'
import { ApiView } from './components/api/ApiView'
import { ArchCanvas, type CanvasHandle } from './components/canvas/ArchCanvas'
import { Inspector } from './components/canvas/Inspector'
import { MermaidPanel } from './components/canvas/MermaidPanel'
import { Palette } from './components/canvas/Palette'
import { DocsView } from './components/docs/DocsView'
import { PitchMode } from './components/pitch/PitchMode'
import { PricingView } from './components/pricing/PricingView'
import { TasksView } from './components/tasks/TasksView'
import { Badge, Button, Modal, Spinner, ToastProvider, useToast } from './components/ui'

type View = 'canvas' | 'docs' | 'api' | 'pricing' | 'tasks'

const VIEWS: { id: View; label: string; icon: typeof LayoutGrid }[] = [
  { id: 'canvas', label: 'Arquitetura', icon: Waypoints },
  { id: 'docs', label: 'Documentação', icon: Bot },
  { id: 'api', label: 'Contratos', icon: Network },
  { id: 'pricing', label: 'Precificação', icon: LayoutGrid },
  { id: 'tasks', label: 'Implementação', icon: Workflow },
]

export default function App() {
  return (
    <ToastProvider>
      <AppWithEvents />
    </ToastProvider>
  )
}

function AppWithEvents() {
  const toast = useToast()

  // Mudanças feitas por agentes de IA ou por edições externas de arquivo são
  // anunciadas: o usuário precisa perceber que o canvas mudou sob seus pés.
  const onEvent = useCallback((event: ServerEvent) => {
    if (event.source === 'ai' && event.message) toast('ai', `IA: ${event.message}`)
    else if (event.source === 'disk' && event.path) toast('info', `Arquivo alterado fora do Studio: ${event.path}`)
    else if (event.source === 'cli' && event.message) toast('info', event.message)
  }, [toast])

  return (
    <ProjectProvider onEvent={onEvent}>
      <ReactFlowProvider>
        <Shell />
      </ReactFlowProvider>
    </ProjectProvider>
  )
}

function Shell() {
  const { snapshot, lint, loading, error, connected, highlight, lastEvent, refresh } = useProject()
  const toast = useToast()

  const [view, setView] = useState<View>('canvas')
  const [selected, setSelected] = useState<{ id: string; kind: 'node' | 'edge' } | null>(null)
  const [executive, setExecutive] = useState(false)
  const [pitch, setPitch] = useState(false)
  const [mermaidOpen, setMermaidOpen] = useState(false)
  const [lintOpen, setLintOpen] = useState(false)
  const [mcpOpen, setMcpOpen] = useState(false)
  const [dark, setDark] = useState(() => !document.documentElement.classList.contains('light'))
  const [canvasHandle, setCanvasHandle] = useState<CanvasHandle | null>(null)
  const [generating, setGenerating] = useState(false)
  const [estimate, setEstimate] = useState<Estimate | null>(null)

  useEffect(() => {
    document.documentElement.classList.toggle('dark', dark)
    document.documentElement.classList.toggle('light', !dark)
  }, [dark])

  // A estimativa alimenta o cabeçalho e o sumário do PDF do Modo Pitch.
  useEffect(() => {
    if (!snapshot) return
    api.estimate().then(setEstimate).catch(() => setEstimate(null))
  }, [snapshot?.diagram.last_modified, snapshot?.use_cases.length, snapshot?.pricing])

  const generatePRD = useCallback(async () => {
    setGenerating(true)
    try {
      const res = await api.generatePRD({ include_test_scenarios: true, granularity: 'detailed' })
      toast('success', `AI-PRD gerado: ${res.total_tasks} tarefas (hash ${res.hash})`)
      await refresh({ silent: true })
    } catch (err) { toast('error', (err as Error).message) } finally { setGenerating(false) }
  }, [refresh, toast])

  const autoLayout = useCallback(async () => {
    if (!window.confirm('Reorganizar reposiciona todos os componentes por camadas topológicas. Continuar?')) return
    try {
      await api.autoLayout()
      toast('success', 'Layout reorganizado')
      setTimeout(() => canvasHandle?.fitAll(), 250)
    } catch (err) { toast('error', (err as Error).message) }
  }, [canvasHandle, toast])

  const onCanvasReady = useCallback((handle: CanvasHandle) => setCanvasHandle(handle), [])

  const progress = useMemo(() => {
    if (!snapshot || snapshot.tasks.tasks.length === 0) return null
    const done = snapshot.tasks.tasks.filter((t) => t.status === 'completed').length
    return { done, total: snapshot.tasks.tasks.length, pct: Math.round((done / snapshot.tasks.tasks.length) * 100) }
  }, [snapshot])

  if (loading && !snapshot) return <div className="grid h-full place-items-center"><Spinner label="Carregando projeto…" /></div>

  if (error && !snapshot) {
    return (
      <div className="grid h-full place-items-center px-6">
        <div className="max-w-md text-center">
          <AlertTriangle size={30} className="mx-auto text-amber-400" />
          <h1 className="mt-3 text-lg font-bold text-app">Não foi possível carregar o projeto</h1>
          <p className="mt-1.5 text-sm text-muted-app">{error.message}</p>
          {error.hint && (
            <pre className="mt-3 rounded-lg surface-3 px-3 py-2 text-left text-xs text-muted-app">{error.hint}</pre>
          )}
          <Button className="mt-4" variant="primary" onClick={() => void refresh()}>Tentar novamente</Button>
        </div>
      </div>
    )
  }
  if (!snapshot) return null

  if (pitch) return <PitchMode snapshot={snapshot} estimate={estimate} onExit={() => setPitch(false)} />

  return (
    <div className="flex h-full flex-col">
      <header className="flex shrink-0 items-center gap-3 border-b border-app surface px-4 py-2">
        <div className="flex items-center gap-2.5">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-sky-500 text-white">
            <Waypoints size={17} />
          </span>
          <div className="leading-tight">
            <p className="text-[13.5px] font-bold text-app">{snapshot.manifest.project_name}</p>
            <p className="text-[10.5px] text-muted-app">
              ArchCode Studio · v{snapshot.manifest.version}
              {lastEvent?.at && <span className="ml-1.5 opacity-70">· {relativeTime(lastEvent.at)}</span>}
            </p>
          </div>
        </div>

        <nav className="ml-4 flex items-center gap-0.5 rounded-lg surface-3 p-0.5">
          {VIEWS.map(({ id, label, icon: Icon }) => (
            <button key={id} onClick={() => setView(id)}
              className={`flex items-center gap-1.5 rounded-md px-2.5 py-1.5 text-xs font-medium transition-colors ${
                view === id ? 'surface text-app shadow-sm' : 'text-muted-app hover:text-app'}`}>
              <Icon size={13} />
              <span className="hidden lg:inline">{label}</span>
            </button>
          ))}
        </nav>

        <div className="ml-auto flex items-center gap-1.5">
          {progress && (
            <button onClick={() => setView('tasks')}
              className="hidden items-center gap-2 rounded-lg surface-3 px-2.5 py-1.5 text-[11px] font-medium text-muted-app transition-colors hover:text-app md:flex">
              <span className="h-1.5 w-16 overflow-hidden rounded-full bg-black/20">
                <span className="block h-full rounded-full bg-emerald-500" style={{ width: `${progress.pct}%` }} />
              </span>
              {progress.done}/{progress.total}
            </button>
          )}

          {lint && (
            <button onClick={() => setLintOpen(true)}
              className="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] font-semibold transition-colors hover:surface-3"
              title="Relatório de validação da arquitetura">
              {lint.errors > 0
                ? <AlertTriangle size={13} className="text-rose-400" />
                : lint.warnings > 0
                  ? <AlertTriangle size={13} className="text-amber-400" />
                  : <CheckCircle2 size={13} className="text-emerald-400" />}
              <span className={lint.errors > 0 ? 'text-rose-400' : lint.warnings > 0 ? 'text-amber-400' : 'text-emerald-400'}>
                {lint.score}
              </span>
            </button>
          )}

          <span className="flex items-center gap-1.5 rounded-lg px-2 py-1.5 text-[11px] text-muted-app"
            title={connected ? 'Sincronização em tempo real ativa' : 'Reconectando ao servidor…'}>
            <span className={`h-1.5 w-1.5 rounded-full ${connected ? 'bg-emerald-500' : 'animate-pulse bg-amber-500'}`} />
            <span className="hidden xl:inline">{connected ? 'ao vivo' : 'reconectando'}</span>
          </span>

          <div className="mx-1 h-5 w-px" style={{ background: 'var(--border)' }} />

          {view === 'canvas' && (
            <>
              <Button size="sm" variant="ghost" icon={Wand2} onClick={() => void autoLayout()}
                title="Reorganizar por camadas topológicas">Layout</Button>
              <Button size="sm" variant="ghost" icon={Network} onClick={() => setMermaidOpen(true)}>Mermaid</Button>
              <Button size="sm" variant={executive ? 'secondary' : 'ghost'} icon={ShieldCheck}
                onClick={() => setExecutive((v) => !v)}
                title="Alterna entre visão de negócio e visão de engenharia">
                {executive ? 'Executivo' : 'Engenharia'}
              </Button>
            </>
          )}
          <Button size="sm" variant="ghost" icon={Plug} onClick={() => setMcpOpen(true)} title="Conectar agente de IA">MCP</Button>
          <Button size="sm" variant="secondary" icon={Presentation} onClick={() => setPitch(true)}>Pitch</Button>
          <Button size="sm" variant="primary" icon={Bot} loading={generating} onClick={() => void generatePRD()}>
            Gerar AI-PRD
          </Button>
          <Button size="icon" variant="ghost" icon={dark ? Sun : Moon} onClick={() => setDark((v) => !v)}
            aria-label="Alternar tema" />
        </div>
      </header>

      <main className="min-h-0 flex-1">
        {view === 'canvas' && (
          <div className="flex h-full">
            <aside className="hidden w-52 shrink-0 border-r border-app surface md:block">
              <Palette onCreated={(id) => setSelected({ id, kind: 'node' })} />
            </aside>
            <div className="relative min-w-0 flex-1">
              <ArchCanvas
                diagram={snapshot.diagram}
                executive={executive}
                highlight={highlight}
                selectedId={selected?.id ?? null}
                onSelect={(id, kind) => setSelected(id ? { id, kind } : null)}
                onReady={onCanvasReady}
              />
            </div>
            <aside className="hidden w-80 shrink-0 border-l border-app surface lg:block">
              <Inspector snapshot={snapshot} selected={selected} onClose={() => setSelected(null)}
                onFocus={(id) => canvasHandle?.focusNode(id)} />
            </aside>
          </div>
        )}
        {view === 'docs' && <DocsView snapshot={snapshot} onGeneratePRD={() => void generatePRD()} />}
        {view === 'api' && <ApiView snapshot={snapshot} />}
        {view === 'pricing' && <PricingView snapshot={snapshot} />}
        {view === 'tasks' && <TasksView snapshot={snapshot} onGeneratePRD={() => void generatePRD()} />}
      </main>

      <MermaidPanel open={mermaidOpen} onClose={() => setMermaidOpen(false)} snapshot={snapshot} />
      <LintModal open={lintOpen} onClose={() => setLintOpen(false)} />
      <McpModal open={mcpOpen} onClose={() => setMcpOpen(false)} />
    </div>
  )
}

function LintModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { lint } = useProject()
  if (!lint) return null
  const color = { error: '#ef4444', warning: '#f59e0b', info: '#64748b' }

  return (
    <Modal open={open} onClose={onClose} wide title={`Validação da arquitetura — ${lint.score}/100`}
      description={`${lint.errors} erro(s), ${lint.warnings} aviso(s), ${lint.infos} informativo(s). As mesmas regras rodam via MCP em validate_architecture_rules.`}>
      {lint.findings.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-8">
          <CheckCircle2 size={28} className="text-emerald-400" />
          <p className="text-sm font-medium text-app">Nenhum problema encontrado.</p>
        </div>
      ) : (
        <ul className="max-h-[60vh] space-y-2 overflow-y-auto">
          {lint.findings.map((f, i) => (
            <li key={i} className="rounded-lg border border-app px-3 py-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <Badge color={color[f.severity]}>{f.severity}</Badge>
                <code className="font-mono text-[10.5px] text-muted-app">{f.rule}</code>
                {f.target && <span className="text-[12.5px] font-semibold text-app">{f.target}</span>}
              </div>
              <p className="mt-1 text-[12.5px] text-muted-app">{f.message}</p>
              {f.fix && <p className="mt-1 text-[11.5px] text-sky-400">→ {f.fix}</p>}
            </li>
          ))}
        </ul>
      )}
    </Modal>
  )
}

function McpModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [root, setRoot] = useState('.')
  useEffect(() => {
    if (open) api.health().then((h: { root: string }) => setRoot(h.root)).catch(() => {})
  }, [open])

  const json = `{
  "mcpServers": {
    "archcode-studio": {
      "command": "archcode-studio",
      "args": ["mcp", "--dir", "${root}"]
    }
  }
}`

  return (
    <Modal open={open} onClose={onClose} wide title="Conectar um agente de IA"
      description="O ArchCode Studio é um servidor Model Context Protocol: o agente lê e modifica esta arquitetura com ferramentas atômicas, sem corromper o layout do canvas.">
      <div className="space-y-4 text-sm">
        <Section icon={Terminal} title="Claude Code">
          <Code>{`claude mcp add archcode-studio -- archcode-studio mcp --dir ${root}`}</Code>
        </Section>
        <Section icon={Plug} title="Cursor, Antigravity, Windsurf, Roo Code">
          <p className="mb-1.5 text-xs text-muted-app">Adicione ao <code>.mcp.json</code> do projeto ou à configuração global:</p>
          <Code>{json}</Code>
        </Section>
        <Section icon={Play} title="Transporte SSE (servidor já rodando)">
          <Code>{`${location.origin}/mcp/sse`}</Code>
        </Section>
        <Section icon={Info} title="Fluxo recomendado para o agente">
          <ol className="list-decimal space-y-0.5 pl-4 text-xs leading-relaxed text-muted-app">
            <li><code>get_system_context</code> — entender o sistema antes de agir</li>
            <li><code>add_architecture_node</code> / <code>connect_nodes</code> — modelar o pedido</li>
            <li><code>upsert_requirement</code> / <code>upsert_use_case</code> — justificar cada componente</li>
            <li><code>validate_architecture_rules</code> — corrigir os erros apontados</li>
            <li><code>generate_ai_prd</code> — compilar o blueprint em ordem topológica</li>
            <li><code>get_implementation_tasks</code> → codificar → <code>mark_task_status</code></li>
          </ol>
        </Section>
      </div>
    </Modal>
  )
}

function Section({ icon: Icon, title, children }: { icon: typeof Info; title: string; children: React.ReactNode }) {
  return (
    <div>
      <p className="mb-1.5 flex items-center gap-1.5 text-[12px] font-semibold text-app">
        <Icon size={13} className="text-sky-400" />
        {title}
      </p>
      {children}
    </div>
  )
}

function Code({ children }: { children: string }) {
  const toast = useToast()
  return (
    <button onClick={() => { void navigator.clipboard.writeText(children); toast('success', 'Copiado') }}
      className="block w-full cursor-copy overflow-x-auto rounded-lg surface-3 px-3 py-2 text-left font-mono text-[11px] leading-relaxed text-app transition-colors hover:brightness-110"
      title="Clique para copiar">
      <pre className="whitespace-pre-wrap">{children}</pre>
    </button>
  )
}
