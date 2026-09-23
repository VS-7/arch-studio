import { ReactFlowProvider } from '@xyflow/react'
import {
  AlertTriangle, BookOpen, Bot, CheckCircle2, Info, Moon, Network, PanelLeft, PanelRight, Play, Plug,
  Presentation, Receipt, Sun, Terminal, Waypoints, Workflow, X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { api } from './lib/api'
import { relativeTime } from './lib/format'
import { ProjectProvider, useProject } from './lib/project'
import { tabKey, parseTabKey, VIEW_LABEL, type TabRef, type ViewId } from './lib/tabs'
import { ThemeProvider, useTheme } from './lib/theme'
import { SELECT_TOOL, type Selection, type Tool } from './lib/tools'
import type { Estimate, ServerEvent, Snapshot, UMLDiagram, UMLKind } from './lib/types'
import { ELEMENT_LABEL, KIND_META } from './lib/umlMeta'
import { ApiView } from './components/api/ApiView'
import { ArchCanvas, type CanvasHandle } from './components/canvas/ArchCanvas'
import { Inspector } from './components/canvas/Inspector'
import { MermaidPanel } from './components/canvas/MermaidPanel'
import { DocsView } from './components/docs/DocsView'
import { PitchMode } from './components/pitch/PitchMode'
import { PricingView } from './components/pricing/PricingView'
import { AppMenubar, type MenuActions } from './components/shell/AppMenubar'
import { ModelExplorer } from './components/shell/ModelExplorer'
import { Toolbox, type ToolboxContext } from './components/shell/Toolbox'
import { TasksView } from './components/tasks/TasksView'
import { Badge, Button, IconAction, Modal, Spinner, Tip, ToastProvider, useToast } from './components/ui'
import { Button as UIButton } from './components/ui/button'
import { ErrorBoundary } from './components/ui/error-boundary'
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from './components/ui/tooltip'
import { UmlCanvas, type UmlCanvasHandle } from './components/uml/UmlCanvas'
import { NewDiagramDialog, RenameDiagramDialog, UmlMermaidDialog } from './components/uml/UmlDialogs'
import { KindGlyph } from './components/uml/UmlGlyph'
import { UmlInspector } from './components/uml/UmlInspector'

export default function App() {
  return (
    <ThemeProvider>
      <TooltipProvider>
        <ToastProvider>
          <AppWithEvents />
        </ToastProvider>
      </TooltipProvider>
    </ThemeProvider>
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

/* -------------------------------------------------------------------------- */
/* Preferências de layout (por navegador)                                      */
/* -------------------------------------------------------------------------- */

function useStored<T>(key: string, initial: T): [T, (v: T) => void] {
  const [value, setValue] = useState<T>(() => {
    try {
      const raw = localStorage.getItem(key)
      return raw ? (JSON.parse(raw) as T) : initial
    } catch { return initial }
  })
  const set = useCallback((v: T) => {
    setValue(v)
    try { localStorage.setItem(key, JSON.stringify(v)) } catch { /* ignora */ }
  }, [key])
  return [value, set]
}

/** Divisor arrastável entre painéis. */
function Splitter({ onDrag, vertical }: { onDrag: (delta: number) => void; vertical?: boolean }) {
  const start = (e: React.MouseEvent) => {
    e.preventDefault()
    let last = vertical ? e.clientY : e.clientX
    const move = (ev: MouseEvent) => {
      const cur = vertical ? ev.clientY : ev.clientX
      onDrag(cur - last)
      last = cur
    }
    const up = () => {
      window.removeEventListener('mousemove', move)
      window.removeEventListener('mouseup', up)
      document.body.style.cursor = ''
    }
    document.body.style.cursor = vertical ? 'row-resize' : 'col-resize'
    window.addEventListener('mousemove', move)
    window.addEventListener('mouseup', up)
  }
  return (
    <div onMouseDown={start}
      className={cn('relative z-10 shrink-0 bg-border transition-colors hover:bg-primary/60',
        vertical ? 'h-px cursor-row-resize before:absolute before:inset-x-0 before:-inset-y-1' : 'w-px cursor-col-resize before:absolute before:inset-y-0 before:-inset-x-1')} />
  )
}

function PanelHeader({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="flex h-7 shrink-0 items-center justify-between border-b bg-panel-header px-2.5">
      <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{title}</span>
      <div className="flex items-center gap-0.5">{children}</div>
    </div>
  )
}

/* -------------------------------------------------------------------------- */
/* Shell                                                                       */
/* -------------------------------------------------------------------------- */

function Shell() {
  const { snapshot, lint, loading, error, connected, highlight, lastEvent, refresh } = useProject()
  const toast = useToast()
  const theme = useTheme()

  const [tabKeys, setTabKeys] = useStored<string[]>('archcode-tabs', ['arch'])
  const [activeKey, setActiveKey] = useStored<string>('archcode-active-tab', 'arch')
  const [panels, setPanels] = useStored('archcode-panels', { left: true, right: true })
  const [leftWidth, setLeftWidth] = useStored('archcode-left-width', 220)
  const [rightWidth, setRightWidth] = useStored('archcode-right-width', 310)
  const [explorerRatio, setExplorerRatio] = useStored('archcode-explorer-ratio', 0.45)

  const [tool, setTool] = useState<Tool>(SELECT_TOOL)
  const [selection, setSelection] = useState<Selection>(null)
  const [focusName, setFocusName] = useState(false)
  const [executive, setExecutive] = useState(false)
  const [pitch, setPitch] = useState(false)
  const [zoom, setZoom] = useState(1)
  const [docsFocus, setDocsFocus] = useState<{ tab: 'use-cases'; code: string; at: number } | null>(null)

  const [archMermaidOpen, setArchMermaidOpen] = useState(false)
  const [lintOpen, setLintOpen] = useState(false)
  const [mcpOpen, setMcpOpen] = useState(false)
  const [shortcutsOpen, setShortcutsOpen] = useState(false)
  const [aboutOpen, setAboutOpen] = useState(false)
  const [newDiagram, setNewDiagram] = useState<UMLKind | null>(null)
  const [renaming, setRenaming] = useState<UMLDiagram | null>(null)
  const [mermaidOf, setMermaidOf] = useState<UMLDiagram | null>(null)

  const [archHandle, setArchHandle] = useState<CanvasHandle | null>(null)
  const [umlHandle, setUmlHandle] = useState<UmlCanvasHandle | null>(null)
  const [generating, setGenerating] = useState(false)
  const [estimate, setEstimate] = useState<Estimate | null>(null)
  const rightPanel = useRef<HTMLDivElement>(null)

  const diagrams = useMemo(() => snapshot?.uml_diagrams ?? [], [snapshot?.uml_diagrams])

  // Abas cujo diagrama deixou de existir (excluído por IA ou no disco) somem.
  const tabs = useMemo(() => tabKeys
    .map(parseTabKey)
    .filter((t): t is TabRef => !!t && (t.type !== 'uml' || diagrams.some((d) => d.id === t.id))),
  [tabKeys, diagrams])
  const active = tabs.find((t) => tabKey(t) === activeKey) ?? tabs[0] ?? null
  const activeDiagram = active?.type === 'uml' ? diagrams.find((d) => d.id === active.id) ?? null : null

  // A estimativa alimenta o sumário do Modo Pitch.
  useEffect(() => {
    if (!snapshot) return
    api.estimate().then(setEstimate).catch(() => setEstimate(null))
  }, [snapshot?.diagram.last_modified, snapshot?.use_cases.length, snapshot?.pricing])

  const openTab = useCallback((tab: TabRef) => {
    const key = tabKey(tab)
    if (!tabKeys.includes(key)) setTabKeys([...tabKeys, key])
    setActiveKey(key)
    setTool(SELECT_TOOL)
  }, [setActiveKey, setTabKeys, tabKeys])

  const closeTab = useCallback((key: string) => {
    const idx = tabKeys.indexOf(key)
    const next = tabKeys.filter((k) => k !== key)
    setTabKeys(next)
    if (activeKey === key) setActiveKey(next[Math.max(0, idx - 1)] ?? '')
  }, [activeKey, setActiveKey, setTabKeys, tabKeys])

  // Trocar de aba limpa a seleção e a ferramenta.
  useEffect(() => { setSelection(null); setTool(SELECT_TOOL); setUmlHandle(null) }, [activeKey])

  /* Ações ------------------------------------------------------------------ */

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
      setTimeout(() => archHandle?.fitAll(), 250)
    } catch (err) { toast('error', (err as Error).message) }
  }, [archHandle, toast])

  const generateUseCases = useCallback(async () => {
    try {
      const d = await api.generateUseCaseDiagram()
      toast('success', `Diagrama "${d.name}" sincronizado com as fichas de caso de uso`)
      await refresh({ silent: true })
      openTab({ type: 'uml', id: d.id })
    } catch (err) { toast('error', (err as Error).message) }
  }, [openTab, refresh, toast])

  const deleteDiagram = useCallback(async (d: UMLDiagram) => {
    if (!window.confirm(`Excluir o diagrama "${d.name}"? Os arquivos .json e .mermaid serão removidos.`)) return
    try {
      await api.deleteUML(d.id)
      closeTab(`uml:${d.id}`)
      toast('success', `Diagrama "${d.name}" excluído`)
    } catch (err) { toast('error', (err as Error).message) }
  }, [closeTab, toast])

  const deleteSelection = useCallback(async () => {
    if (!selection) return
    try {
      if (selection.scope === 'uml') {
        const d = diagrams.find((x) => x.id === selection.diagramId)
        if (selection.kind === 'element') {
          const el = d?.elements.find((e) => e.id === selection.id)
          if (!window.confirm(`Excluir "${el?.name || (el ? ELEMENT_LABEL[el.type] : 'elemento')}" e suas relações?`)) return
          await api.deleteUMLElement(selection.diagramId, selection.id)
        } else {
          await api.deleteUMLRelation(selection.diagramId, selection.id)
        }
      } else if (selection.kind === 'node') {
        const n = snapshot?.diagram.nodes.find((x) => x.id === selection.id)
        if (!window.confirm(`Excluir o componente "${n?.data.label ?? selection.id}" e suas conexões?`)) return
        await api.deleteNode(selection.id)
      } else {
        await api.deleteEdge(selection.id)
      }
      setSelection(null)
    } catch (err) { toast('error', (err as Error).message) }
  }, [diagrams, selection, snapshot?.diagram.nodes, toast])

  const selectUmlElement = useCallback((diagramId: string, elementId: string) => {
    openTab({ type: 'uml', id: diagramId })
    // Depois que a aba monta, seleciona e centraliza.
    setTimeout(() => {
      setSelection({ scope: 'uml', diagramId, kind: 'element', id: elementId })
      setFocusName(false)
    }, 0)
  }, [openTab])

  const selectArchNode = useCallback((nodeId: string) => {
    openTab({ type: 'arch' })
    setTimeout(() => {
      setSelection({ scope: 'arch', kind: 'node', id: nodeId })
      archHandle?.focusNode(nodeId)
    }, 0)
  }, [archHandle, openTab])

  // Centraliza no elemento selecionado a partir do Model Explorer.
  useEffect(() => {
    if (selection?.scope === 'uml' && selection.kind === 'element' && umlHandle && !focusName) umlHandle.focus(selection.id)
  }, [selection, umlHandle, focusName])

  /* Atalhos de teclado ------------------------------------------------------- */

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement
      const typing = target.closest('input, textarea, select, [contenteditable="true"], [role="dialog"]')
      if (e.key === 'Escape' && !typing) { setTool(SELECT_TOOL); return }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'b') { e.preventDefault(); setPanels({ ...panels, left: !panels.left }); return }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'j') { e.preventDefault(); setPanels({ ...panels, right: !panels.right }); return }
      if (typing) return
      if ((e.key === 'Delete' || e.key === 'Backspace') && selection) { e.preventDefault(); void deleteSelection(); return }
      if (e.shiftKey && e.code === 'Digit1') { (activeDiagram ? umlHandle?.fitAll : archHandle?.fitAll)?.(); return }
      if (activeDiagram && (e.key === '+' || e.key === '=')) umlHandle?.zoomIn()
      if (activeDiagram && e.key === '-') umlHandle?.zoomOut()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeDiagram, archHandle, deleteSelection, panels, selection, setPanels, umlHandle])

  /* Estados de carregamento ---------------------------------------------------- */

  if (loading && !snapshot) return <div className="grid h-full place-items-center"><Spinner label="Carregando projeto…" /></div>

  if (error && !snapshot) {
    return (
      <div className="grid h-full place-items-center px-6">
        <div className="max-w-md text-center">
          <AlertTriangle size={30} className="mx-auto text-warning" />
          <h1 className="mt-3 text-lg font-bold">Não foi possível carregar o projeto</h1>
          <p className="mt-1.5 text-sm text-muted-foreground">{error.message}</p>
          {error.hint && <pre className="mt-3 rounded-md bg-muted px-3 py-2 text-left text-xs text-muted-foreground">{error.hint}</pre>}
          <Button className="mt-4" variant="primary" onClick={() => void refresh()}>Tentar novamente</Button>
        </div>
      </div>
    )
  }
  if (!snapshot) return null

  if (pitch) return <PitchMode snapshot={snapshot} estimate={estimate} onExit={() => setPitch(false)} />

  const archActive = active?.type === 'arch'
  const toolboxContext: ToolboxContext = archActive ? { type: 'arch' } : activeDiagram ? { type: 'uml', kind: activeDiagram.kind } : null
  const fit = archActive ? archHandle?.fitAll : activeDiagram ? umlHandle?.fitAll : undefined

  const actions: MenuActions = {
    newDiagram: (kind) => setNewDiagram(kind),
    generateUseCases: () => void generateUseCases(),
    exportImage: activeDiagram && umlHandle ? (format) => void umlHandle.exportImage(format) : null,
    exportMermaid: activeDiagram ? () => setMermaidOf(activeDiagram) : archActive ? () => setArchMermaidOpen(true) : null,
    generatePRD: () => void generatePRD(),
    deleteSelection: selection ? () => void deleteSelection() : null,
    fit: fit ?? null,
    zoomIn: activeDiagram ? umlHandle?.zoomIn ?? null : null,
    zoomOut: activeDiagram ? umlHandle?.zoomOut ?? null : null,
    autoLayout: () => { openTab({ type: 'arch' }); void autoLayout() },
    importMermaid: () => setArchMermaidOpen(true),
    validate: () => setLintOpen(true),
    mcp: () => setMcpOpen(true),
    pitch: () => setPitch(true),
    shortcuts: () => setShortcutsOpen(true),
    about: () => setAboutOpen(true),
  }

  const statusPath = archActive ? '.arch/diagrams/macro.json'
    : activeDiagram ? activeDiagram.file ?? `.arch/diagrams/${activeDiagram.kind}/${activeDiagram.id}.json`
      : active?.type === 'view' ? VIEW_LABEL[active.view] : ''

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      {/* Barra de título + menus ------------------------------------------------ */}
      <header className="flex h-9 shrink-0 items-center gap-2 border-b bg-chrome px-2">
        <span className="flex size-6 items-center justify-center rounded-sm border bg-background text-foreground">
          <Waypoints size={14} />
        </span>
        <AppMenubar actions={actions} theme={theme.preference} onTheme={theme.setPreference}
          panels={panels} onPanels={setPanels} executive={executive} onExecutive={setExecutive} archActive={archActive} />

        <div className="mx-auto hidden min-w-0 items-center gap-1.5 truncate text-[12px] text-muted-foreground lg:flex">
          <span className="truncate font-medium text-foreground">{snapshot.manifest.project_name}</span>
          <span>· v{snapshot.manifest.version}</span>
        </div>

        <div className="ml-auto flex items-center gap-1">
          <IconButton label={panels.left ? 'Ocultar Toolbox (Ctrl+B)' : 'Mostrar Toolbox (Ctrl+B)'} active={panels.left}
            onClick={() => setPanels({ ...panels, left: !panels.left })}><PanelLeft /></IconButton>
          <IconButton label={panels.right ? 'Ocultar painéis (Ctrl+J)' : 'Mostrar painéis (Ctrl+J)'} active={panels.right}
            onClick={() => setPanels({ ...panels, right: !panels.right })}><PanelRight /></IconButton>
          <IconButton label="Conectar agente de IA (MCP)" onClick={() => setMcpOpen(true)}><Plug /></IconButton>
          <IconButton label={theme.resolved === 'dark' ? 'Tema claro' : 'Tema escuro'} onClick={theme.toggle}>
            {theme.resolved === 'dark' ? <Sun /> : <Moon />}
          </IconButton>
          <div className="mx-1 h-4 w-px bg-border" />
          <Button size="sm" variant="secondary" icon={Presentation} onClick={() => setPitch(true)}>Pitch</Button>
          <Button size="sm" variant="primary" icon={Bot} loading={generating} onClick={() => void generatePRD()}>Gerar AI-PRD</Button>
        </div>
      </header>

      {/* Área de trabalho ------------------------------------------------------- */}
      <div className="flex min-h-0 flex-1">
        {panels.left && (
          <>
            <aside className="flex shrink-0 flex-col bg-chrome" style={{ width: leftWidth }}>
              <PanelHeader title="Toolbox" />
              <div className="min-h-0 flex-1 overflow-y-auto">
                <Toolbox context={toolboxContext} tool={tool} onTool={setTool} />
              </div>
            </aside>
            <Splitter onDrag={(d) => setLeftWidth(Math.min(360, Math.max(170, leftWidth + d)))} />
          </>
        )}

        <main className="flex min-w-0 flex-1 flex-col">
          <TabStrip tabs={tabs} active={active} diagrams={diagrams} onActivate={(t) => setActiveKey(tabKey(t))} onClose={closeTab} />
          <div className="relative min-h-0 flex-1">
            <ErrorBoundary key={activeKey} area={active?.type === 'view' ? VIEW_LABEL[active.view] : activeDiagram?.name ?? 'Arquitetura'}>
            {!active && (
              <div className="grid h-full place-items-center text-[13px] text-muted-foreground">
                Abra um diagrama pelo Model Explorer.
              </div>
            )}
            {archActive && (
              <ArchCanvas
                diagram={snapshot.diagram}
                executive={executive}
                highlight={highlight}
                selectedId={selection?.scope === 'arch' ? selection.id : null}
                onSelect={(id, kind) => setSelection(id ? { scope: 'arch', kind, id } : null)}
                onReady={setArchHandle}
                tool={tool}
                onToolDone={() => setTool(SELECT_TOOL)}
                onZoom={setZoom}
              />
            )}
            {activeDiagram && (
              <UmlCanvas
                key={activeDiagram.id}
                diagram={activeDiagram}
                tool={tool}
                onToolDone={() => setTool(SELECT_TOOL)}
                selectedId={selection?.scope === 'uml' && selection.diagramId === activeDiagram.id ? selection.id : null}
                onSelect={(sel) => { setFocusName(false); setSelection(sel ? { scope: 'uml', diagramId: activeDiagram.id, ...sel } : null) }}
                onCreated={(id) => { setFocusName(true); setSelection({ scope: 'uml', diagramId: activeDiagram.id, kind: 'element', id }) }}
                onReady={setUmlHandle}
                onZoom={setZoom}
                highlight={highlight}
              />
            )}
            {active?.type === 'view' && (
              <div className="h-full overflow-hidden bg-background">
                {active.view === 'docs' && (
                  <DocsView key={docsFocus?.at ?? 'docs'} snapshot={snapshot} onGeneratePRD={() => void generatePRD()}
                    initialTab={docsFocus?.tab} focusUseCase={docsFocus?.code} />
                )}
                {active.view === 'api' && <ApiView snapshot={snapshot} />}
                {active.view === 'pricing' && <PricingView snapshot={snapshot} />}
                {active.view === 'tasks' && <TasksView snapshot={snapshot} onGeneratePRD={() => void generatePRD()} />}
              </div>
            )}
            </ErrorBoundary>
          </div>
        </main>

        {panels.right && (
          <>
            <Splitter onDrag={(d) => setRightWidth(Math.min(520, Math.max(240, rightWidth - d)))} />
            <aside ref={rightPanel} className="flex shrink-0 flex-col bg-chrome" style={{ width: rightWidth }}>
              <div className="flex min-h-0 flex-col" style={{ height: `${explorerRatio * 100}%` }}>
                <PanelHeader title="Model Explorer" />
                <div className="min-h-0 flex-1 bg-background">
                  <ModelExplorer
                    snapshot={snapshot}
                    activeTab={active}
                    selection={selection}
                    onOpen={openTab}
                    onSelectElement={selectUmlElement}
                    onSelectArchNode={selectArchNode}
                    onCreateDiagram={(kind) => setNewDiagram(kind)}
                    onRenameDiagram={setRenaming}
                    onDeleteDiagram={(d) => void deleteDiagram(d)}
                    onShowMermaid={setMermaidOf}
                    onGenerateUseCases={() => void generateUseCases()}
                  />
                </div>
              </div>
              <Splitter vertical onDrag={(d) => {
                const h = rightPanel.current?.clientHeight ?? 800
                setExplorerRatio(Math.min(0.8, Math.max(0.2, explorerRatio + d / h)))
              }} />
              <div className="flex min-h-0 flex-1 flex-col">
                <PanelHeader title="Editor" />
                <div className="min-h-0 flex-1 overflow-y-auto bg-background">
                  <ErrorBoundary key={`${activeKey}|${selection?.id ?? ''}`} area="Editor">
                  <EditorPanel
                    snapshot={snapshot}
                    active={active}
                    diagram={activeDiagram}
                    selection={selection}
                    focusName={focusName}
                    onClose={() => setSelection(null)}
                    onFocusArch={(id) => selectArchNode(id)}
                    onOpenUseCase={(code) => { setDocsFocus({ tab: 'use-cases', code, at: Date.now() }); openTab({ type: 'view', view: 'docs' }) }}
                    onRename={setRenaming}
                    onMermaid={setMermaidOf}
                    onExport={(f) => void umlHandle?.exportImage(f)}
                    archFocus={(id) => archHandle?.focusNode(id)}
                  />
                  </ErrorBoundary>
                </div>
              </div>
            </aside>
          </>
        )}
      </div>

      {/* Barra de status ------------------------------------------------------------ */}
      <footer className="flex h-6 shrink-0 items-center gap-3 bg-statusbar px-2.5 text-[11.5px] text-statusbar-foreground">
        <Tip label={connected ? 'Sincronização em tempo real ativa' : 'Reconectando ao servidor…'} side="top">
          <span className="flex items-center gap-1.5">
            <span className={cn('size-1.5 rounded-full', connected ? 'bg-success' : 'animate-pulse bg-warning')} />
            {connected ? 'Sincronizado' : 'Reconectando…'}
          </span>
        </Tip>
        {statusPath && <span className="truncate font-mono opacity-90">{statusPath}</span>}
        {activeDiagram && (
          <span className="opacity-90">{activeDiagram.elements.length} elementos · {activeDiagram.relations.length} relações</span>
        )}
        {archActive && (
          <span className="opacity-90">{snapshot.diagram.nodes.length} componentes · {snapshot.diagram.edges.length} conexões</span>
        )}
        {tool.mode !== 'select' && <span className="rounded-sm bg-accent px-1.5">Ferramenta ativa · Esc cancela</span>}
        <div className="ml-auto flex items-center gap-3">
          {lastEvent?.at && <span className="hidden opacity-80 md:inline">última alteração {relativeTime(lastEvent.at)}</span>}
          {lint && (
            <Tip label="Relatório de validação da arquitetura" side="top">
            <button onClick={() => setLintOpen(true)} className="flex items-center gap-1 rounded-sm px-1 hover:bg-accent">
              {lint.errors > 0 || lint.warnings > 0 ? <AlertTriangle size={12} /> : <CheckCircle2 size={12} />}
              Qualidade {lint.score}
            </button>
            </Tip>
          )}
          {(archActive || activeDiagram) && <span className="tabular-nums">{Math.round(zoom * 100)}%</span>}
        </div>
      </footer>

      <MermaidPanel open={archMermaidOpen} onClose={() => setArchMermaidOpen(false)} snapshot={snapshot} />
      <LintModal open={lintOpen} onClose={() => setLintOpen(false)} />
      <McpModal open={mcpOpen} onClose={() => setMcpOpen(false)} />
      <ShortcutsModal open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
      <AboutModal open={aboutOpen} onClose={() => setAboutOpen(false)} snapshot={snapshot} />
      <NewDiagramDialog open={newDiagram !== null} initialKind={newDiagram ?? 'usecase'} onClose={() => setNewDiagram(null)}
        onCreated={(d) => { void refresh({ silent: true }).then(() => openTab({ type: 'uml', id: d.id })) }} />
      <RenameDiagramDialog diagram={renaming} onClose={() => setRenaming(null)} />
      <UmlMermaidDialog diagram={mermaidOf} onClose={() => setMermaidOf(null)} />
    </div>
  )
}

function IconButton({ label, onClick, children, active }: { label: string; onClick: () => void; children: ReactNode; active?: boolean }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <UIButton size="icon-sm" variant="ghost" onClick={onClick} aria-label={label}
          className={cn(active && 'text-foreground')}>
          {children}
        </UIButton>
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  )
}

/* Abas de diagramas ---------------------------------------------------------- */

function TabStrip({ tabs, active, diagrams, onActivate, onClose }: {
  tabs: TabRef[]; active: TabRef | null; diagrams: UMLDiagram[]
  onActivate: (t: TabRef) => void; onClose: (key: string) => void
}) {
  const activeKey = active ? tabKey(active) : ''
  return (
    <div className="flex h-8 shrink-0 items-end overflow-x-auto border-b bg-chrome-2">
      {tabs.map((t) => {
        const key = tabKey(t)
        const d = t.type === 'uml' ? diagrams.find((x) => x.id === t.id) : undefined
        const label = t.type === 'arch' ? 'Arquitetura' : t.type === 'uml' ? d?.name ?? t.id : VIEW_LABEL[t.view]
        const isActive = key === activeKey
        return (
          <div key={key}
            onMouseDown={(e) => { if (e.button === 1) { e.preventDefault(); onClose(key) } }}
            onClick={() => onActivate(t)}
            className={cn(
              'group relative flex h-8 max-w-[220px] shrink-0 cursor-default items-center gap-1.5 border-r px-3 text-[12.5px]',
              isActive ? 'bg-tab-active text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            {isActive && <span className="absolute inset-x-0 top-0 h-0.5 bg-foreground/70" />}
            <span>
              {t.type === 'uml' && d ? <KindGlyph kind={d.kind} size={13} />
                : t.type === 'arch' ? <Waypoints size={13} />
                  : t.type === 'view' ? <ViewIcon view={t.view} /> : null}
            </span>
            <span className="truncate">{label}</span>
            <IconAction label="Fechar aba (clique do meio)"
              onClick={(e) => { e.stopPropagation(); onClose(key) }}
              className={cn('ml-0.5', isActive ? 'opacity-80' : 'opacity-0 group-hover:opacity-80')}>
              <X size={12} />
            </IconAction>
          </div>
        )
      })}
    </div>
  )
}

function ViewIcon({ view }: { view: ViewId }) {
  const Icon = { docs: BookOpen, api: Network, pricing: Receipt, tasks: Workflow }[view]
  return <Icon size={13} />
}

/* Painel Editor (propriedades do que está selecionado) ------------------------------ */

function EditorPanel({
  snapshot, active, diagram, selection, focusName, onClose, onFocusArch, onOpenUseCase, onRename, onMermaid, onExport, archFocus,
}: {
  snapshot: Snapshot; active: TabRef | null; diagram: UMLDiagram | null; selection: Selection; focusName: boolean
  onClose: () => void; onFocusArch: (id: string) => void; onOpenUseCase: (code: string) => void
  onRename: (d: UMLDiagram) => void; onMermaid: (d: UMLDiagram) => void; onExport: (f: 'png' | 'svg') => void
  archFocus: (id: string) => void
}) {
  if (active?.type === 'arch') {
    return (
      <Inspector snapshot={snapshot}
        selected={selection?.scope === 'arch' ? { id: selection.id, kind: selection.kind } : null}
        onClose={onClose} onFocus={archFocus} />
    )
  }
  if (diagram) {
    if (selection?.scope === 'uml' && selection.diagramId === diagram.id) {
      return (
        <UmlInspector snapshot={snapshot} diagram={diagram} selection={{ kind: selection.kind, id: selection.id }}
          focusName={focusName} onClose={onClose} onOpenUseCase={onOpenUseCase} onFocusArch={onFocusArch} />
      )
    }
    return <DiagramSummary diagram={diagram} onRename={onRename} onMermaid={onMermaid} onExport={onExport} />
  }
  return (
    <p className="px-4 py-6 text-center text-[12px] leading-relaxed text-muted-foreground">
      Selecione um elemento em um diagrama para editar suas propriedades.
    </p>
  )
}

function DiagramSummary({ diagram, onRename, onMermaid, onExport }: {
  diagram: UMLDiagram; onRename: (d: UMLDiagram) => void; onMermaid: (d: UMLDiagram) => void; onExport: (f: 'png' | 'svg') => void
}) {
  const counts = new Map<string, number>()
  for (const el of diagram.elements) counts.set(ELEMENT_LABEL[el.type], (counts.get(ELEMENT_LABEL[el.type]) ?? 0) + 1)
  return (
    <div className="space-y-3 p-3">
      <div className="flex items-start gap-2">
        <span className="mt-0.5"><KindGlyph kind={diagram.kind} size={18} /></span>
        <div className="min-w-0">
          <p className="truncate text-[13px] font-semibold">{diagram.name}</p>
          <p className="text-[11.5px] text-muted-foreground">{KIND_META[diagram.kind].label}</p>
        </div>
      </div>
      {diagram.description && <p className="text-[12px] leading-relaxed text-muted-foreground">{diagram.description}</p>}
      <div className="flex flex-wrap gap-1">
        {[...counts].map(([label, n]) => <Badge key={label}>{n} {label.toLowerCase()}</Badge>)}
        <Badge>{diagram.relations.length} relações</Badge>
      </div>
      <p className="break-all font-mono text-[10.5px] text-muted-foreground">{diagram.file}</p>
      <div className="flex flex-wrap gap-1.5">
        <Button size="sm" variant="secondary" onClick={() => onRename(diagram)}>Propriedades…</Button>
        <Button size="sm" variant="secondary" onClick={() => onMermaid(diagram)}>Mermaid</Button>
        <Button size="sm" variant="secondary" onClick={() => onExport('png')}>PNG</Button>
        <Button size="sm" variant="secondary" onClick={() => onExport('svg')}>SVG</Button>
      </div>
      <p className="border-t pt-3 text-[11.5px] leading-relaxed text-muted-foreground">
        Clique em um elemento para editá-lo. Use a Toolbox para criar formas e relações, ou peça a um agente de IA
        via MCP (<code className="font-mono">add_uml_element</code>, <code className="font-mono">add_uml_relation</code>).
      </p>
    </div>
  )
}

/* Modais ----------------------------------------------------------------------------- */

function LintModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { lint } = useProject()
  if (!lint) return null
  const color = { error: 'var(--destructive)', warning: 'var(--warning)', info: 'var(--muted-foreground)' }

  return (
    <Modal open={open} onClose={onClose} wide title={`Validação da arquitetura — ${lint.score}/100`}
      description={`${lint.errors} erro(s), ${lint.warnings} aviso(s), ${lint.infos} informativo(s). As mesmas regras rodam via MCP em validate_architecture_rules.`}>
      {lint.findings.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-8">
          <CheckCircle2 size={28} className="text-success" />
          <p className="text-sm font-medium">Nenhum problema encontrado.</p>
        </div>
      ) : (
        <ul className="max-h-[60vh] space-y-2 overflow-y-auto">
          {lint.findings.map((f, i) => (
            <li key={i} className="rounded-md border px-3 py-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <Badge color={color[f.severity]}>{f.severity}</Badge>
                <code className="font-mono text-[10.5px] text-muted-foreground">{f.rule}</code>
                {f.target && <span className="text-[12.5px] font-semibold">{f.target}</span>}
              </div>
              <p className="mt-1 text-[12.5px] text-muted-foreground">{f.message}</p>
              {f.fix && <p className="mt-1 text-[11.5px] text-primary">→ {f.fix}</p>}
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
      description="O ArchCode Studio é um servidor Model Context Protocol: o agente lê e modifica arquitetura e diagramas UML com ferramentas atômicas, sem corromper o layout.">
      <div className="space-y-4 text-sm">
        <Section icon={Terminal} title="Claude Code">
          <Code>{`claude mcp add archcode-studio -- archcode-studio mcp --dir ${root}`}</Code>
        </Section>
        <Section icon={Plug} title="Cursor, Antigravity, Windsurf, Roo Code">
          <p className="mb-1.5 text-xs text-muted-foreground">Adicione ao <code>.mcp.json</code> do projeto ou à configuração global:</p>
          <Code>{json}</Code>
        </Section>
        <Section icon={Play} title="Transporte SSE (servidor já rodando)">
          <Code>{`${location.origin}/mcp/sse`}</Code>
        </Section>
        <Section icon={Info} title="Fluxo recomendado para o agente">
          <ol className="list-decimal space-y-0.5 pl-4 text-xs leading-relaxed text-muted-foreground">
            <li><code>get_system_context</code> — entender o sistema antes de agir</li>
            <li><code>add_architecture_node</code> / <code>connect_nodes</code> — modelar a arquitetura</li>
            <li><code>upsert_requirement</code> / <code>upsert_use_case</code> — justificar cada componente</li>
            <li><code>generate_use_case_diagram</code>, <code>create_uml_diagram</code>, <code>add_uml_element</code>, <code>add_uml_relation</code> — casos de uso, classes, sequência e estados</li>
            <li><code>validate_architecture_rules</code> — corrigir os erros apontados</li>
            <li><code>generate_ai_prd</code> — compilar o blueprint em ordem topológica</li>
            <li><code>get_implementation_tasks</code> → codificar → <code>mark_task_status</code></li>
          </ol>
        </Section>
      </div>
    </Modal>
  )
}

function ShortcutsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const rows: [string, string][] = [
    ['Esc', 'Voltar à ferramenta Selecionar / cancelar relação'],
    ['Del / Backspace', 'Excluir o elemento ou relação selecionado'],
    ['Shift + 1', 'Ajustar o diagrama à tela'],
    ['+ / −', 'Aproximar / afastar (diagramas UML)'],
    ['Ctrl + B', 'Mostrar/ocultar Toolbox'],
    ['Ctrl + J', 'Mostrar/ocultar Model Explorer e Editor'],
    ['Arrastar no vazio', 'Selecionar vários elementos'],
    ['Botão do meio / scroll', 'Mover o canvas'],
  ]
  return (
    <Modal open={open} onClose={onClose} title="Atalhos de teclado">
      <table className="w-full text-[12.5px]">
        <tbody>
          {rows.map(([k, v]) => (
            <tr key={k} className="border-b last:border-0">
              <td className="py-1.5 pr-4"><kbd className="rounded border bg-muted px-1.5 py-0.5 font-mono text-[11px]">{k}</kbd></td>
              <td className="py-1.5 text-muted-foreground">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Modal>
  )
}

function AboutModal({ open, onClose, snapshot }: { open: boolean; onClose: () => void; snapshot: Snapshot }) {
  return (
    <Modal open={open} onClose={onClose} title="ArchCode Studio"
      description="Arquitetura-como-código, local-first, orientada a agentes de IA (MCP).">
      <div className="space-y-2 text-[12.5px] text-muted-foreground">
        <p>Projeto: <span className="text-foreground">{snapshot.manifest.project_name}</span> · schema {snapshot.manifest.schema_version}</p>
        <p>
          Diagramas: arquitetura macro + {snapshot.uml_diagrams?.length ?? 0} UML (casos de uso, classes, sequência e estados).
          Tudo é gravado em arquivos texto na pasta do projeto — versionável em Git.
        </p>
      </div>
    </Modal>
  )
}

function Section({ icon: Icon, title, children }: { icon: typeof Info; title: string; children: ReactNode }) {
  return (
    <div>
      <p className="mb-1.5 flex items-center gap-1.5 text-[12px] font-semibold">
        <Icon size={13} className="text-primary" />
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
      className="block w-full cursor-copy overflow-x-auto rounded-md border bg-muted px-3 py-2 text-left font-mono text-[11px] leading-relaxed transition-colors hover:bg-accent"
      title="Clique para copiar">
      <pre className="whitespace-pre-wrap">{children}</pre>
    </button>
  )
}
