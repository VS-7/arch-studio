import { ReactFlowProvider } from '@xyflow/react'
import { AlertTriangle, PanelLeftClose } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import { api } from './lib/api'
import { errorMessage } from './lib/errors'
import { useHistory } from './lib/history'
import { ProjectProvider, useProject } from './lib/project'
import { VIEW_LABEL, type TabRef } from './lib/tabs'
import { ThemeProvider } from './lib/theme'
import { SELECT_TOOL, type Selection, type Tool } from './lib/tools'
import type { ServerEvent, UMLDiagram } from './lib/types'
import { ApiView } from './components/api/ApiView'
import { ArchCanvas, type CanvasHandle } from './components/canvas/ArchCanvas'
import { AdrPanel } from './components/docs/AdrPanel'
import { AiPrdView, ProposalView } from './components/docs/GeneratedDocs'
import { DocumentPanel } from './components/docs/reqdoc/DocumentPanel'
import { RequirementsPanel } from './components/docs/RequirementsPanel'
import { UseCasesPanel } from './components/docs/UseCasesPanel'
import { PitchMode } from './components/pitch/PitchMode'
import { PricingView } from './components/pricing/PricingView'
import type { MenuActions } from './components/shell/AppMenubar'
import { useDesktopProjects, WelcomeScreen } from './components/shell/DesktopProjects'
import { ShellDialogs, type ShellDialog } from './components/shell/dialogs/ShellDialogs'
import { EditorPanel } from './components/shell/EditorPanel'
import { ModelExplorer } from './components/shell/ModelExplorer'
import { PanelHeader, SectionHeader, Splitter } from './components/shell/panels'
import { StatusBar } from './components/shell/StatusBar'
import { TabStrip } from './components/shell/TabStrip'
import { TitleBar } from './components/shell/TitleBar'
import { Toolbox, type ToolboxContext } from './components/shell/Toolbox'
import { useEditCommands } from './components/shell/useEditCommands'
import { useShortcuts } from './components/shell/useShortcuts'
import {
  EXPLORER_DEFAULT, LEFT_DEFAULT, LEFT_MIN, RIGHT_DEFAULT, RIGHT_MIN, useWorkspaceLayout,
} from './components/shell/useWorkspaceLayout'
import { useWorkspaceTabs } from './components/shell/useWorkspaceTabs'
import { TasksView } from './components/tasks/TasksView'
import { Button, IconAction, Spinner, ToastProvider, useToast } from './components/ui'
import { ConfirmProvider, useConfirm } from './components/ui/confirm'
import { ErrorBoundary } from './components/ui/error-boundary'
import { TooltipProvider } from './components/ui/tooltip'
import { UmlCanvas, type UmlCanvasHandle } from './components/uml/UmlCanvas'

export default function App() {
  return (
    <ThemeProvider>
      <TooltipProvider>
        <ToastProvider>
          <ConfirmProvider>
            <AppWithEvents />
          </ConfirmProvider>
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
/* Shell                                                                       */
/* -------------------------------------------------------------------------- */

function Shell() {
  const { snapshot, lint, loading, error, connected, highlight, lastEvent, refresh } = useProject()
  const toast = useToast()
  const confirm = useConfirm()
  const history = useHistory(snapshot)
  const layout = useWorkspaceLayout()
  const projects = useDesktopProjects()

  const diagrams = useMemo(() => snapshot?.uml_diagrams ?? [], [snapshot?.uml_diagrams])
  const workspace = useWorkspaceTabs(diagrams)
  const { active, activeKey, activeDiagram, open: openWorkspaceTab, close: closeTab } = workspace

  const [tool, setTool] = useState<Tool>(SELECT_TOOL)
  const [selection, setSelection] = useState<Selection>(null)
  // Token (timestamp) que pede ao Editor para focar o campo Nome; 0 = não focar.
  const [focusName, setFocusName] = useState(0)
  const [multiCount, setMultiCount] = useState(0)
  const [executive, setExecutive] = useState(false)
  const [pitch, setPitch] = useState(false)
  const [zoom, setZoom] = useState(1)
  // Ficha de caso de uso a abrir para edição ao entrar na aba Casos de Uso.
  const [useCaseFocus, setUseCaseFocus] = useState<{ code: string; at: number } | null>(null)
  const [dialog, setDialog] = useState<ShellDialog | null>(null)

  const [archHandle, setArchHandle] = useState<CanvasHandle | null>(null)
  const [umlHandle, setUmlHandle] = useState<UmlCanvasHandle | null>(null)
  const [generating, setGenerating] = useState(false)
  const rightPanel = useRef<HTMLDivElement>(null)

  const edit = useEditCommands({
    history, active, activeDiagram, archHandle, umlHandle, onRemoved: () => setSelection(null),
  })

  const openTab = useCallback((tab: TabRef) => {
    openWorkspaceTab(tab)
    setTool(SELECT_TOOL)
  }, [openWorkspaceTab])

  // Trocar de aba limpa a seleção e a ferramenta (e o pedido de abrir uma ficha).
  useEffect(() => {
    setSelection(null); setTool(SELECT_TOOL); setUmlHandle(null)
    if (activeKey !== 'view:use-cases') setUseCaseFocus(null)
  }, [activeKey])

  /* Ações ------------------------------------------------------------------ */

  const generatePRD = useCallback(async () => {
    setGenerating(true)
    try {
      const res = await api.generatePRD({ include_test_scenarios: true, granularity: 'detailed' })
      toast('success', `AI-PRD gerado: ${res.total_tasks} tarefas (hash ${res.hash})`)
      await refresh({ silent: true })
    } catch (err) { toast('error', errorMessage(err)) } finally { setGenerating(false) }
  }, [refresh, toast])

  // Reorganizar redispõe o diagrama inteiro para caber na página do Documento
  // de Requisitos. Cada diagrama alterado é um passo de desfazer próprio.
  const reorganize = useCallback(async () => {
    const key = activeDiagram ? `uml:${activeDiagram.id}` : 'arch'
    try {
      if (activeDiagram) await api.autoLayoutUML(activeDiagram.id)
      else await api.autoLayout()
      toast('success', 'Diagrama reorganizado', { label: 'Desfazer', onClick: () => void history.undo(key) })
      setTimeout(() => (activeDiagram ? umlHandle : archHandle)?.fitAll(), 250)
    } catch (err) { toast('error', errorMessage(err)) }
  }, [activeDiagram, archHandle, umlHandle, history, toast])

  const reorganizeAll = useCallback(async () => {
    try {
      const res = await api.autoLayoutAll()
      const keys = [...(res.architecture ? ['arch'] : []), ...res.diagrams.map((id) => `uml:${id}`)]
      if (!keys.length) {
        toast('info', 'Os diagramas já estavam organizados')
        return
      }
      toast('success', keys.length === 1 ? '1 diagrama reorganizado' : `${keys.length} diagramas reorganizados`, {
        label: 'Desfazer', onClick: () => void Promise.all(keys.map((k) => history.undo(k))),
      })
      setTimeout(() => (activeDiagram ? umlHandle : archHandle)?.fitAll(), 250)
    } catch (err) { toast('error', errorMessage(err)) }
  }, [activeDiagram, archHandle, umlHandle, history, toast])

  const generateUseCases = useCallback(async () => {
    try {
      const d = await api.generateUseCaseDiagram()
      toast('success', `Diagrama "${d.name}" sincronizado com as fichas de caso de uso`)
      await refresh({ silent: true })
      openTab({ type: 'uml', id: d.id })
    } catch (err) { toast('error', errorMessage(err)) }
  }, [openTab, refresh, toast])

  const deleteDiagram = useCallback(async (d: UMLDiagram) => {
    const ok = await confirm({
      title: `Excluir o diagrama "${d.name}"?`,
      description: 'Os arquivos .json e .mermaid do diagrama serão removidos do projeto. Esta ação não pode ser desfeita pelo Ctrl+Z.',
      confirmLabel: 'Excluir diagrama', destructive: true,
    })
    if (!ok) return
    try {
      await api.deleteUML(d.id)
      closeTab(`uml:${d.id}`)
      toast('success', `Diagrama "${d.name}" excluído`)
    } catch (err) { toast('error', errorMessage(err)) }
  }, [closeTab, confirm, toast])

  /* Navegação ---------------------------------------------------------------- */

  const selectUmlElement = useCallback((diagramId: string, elementId: string) => {
    openTab({ type: 'uml', id: diagramId })
    // Depois que a aba monta, seleciona e centraliza.
    setTimeout(() => {
      setSelection({ scope: 'uml', diagramId, kind: 'element', id: elementId })
      setFocusName(0)
    }, 0)
  }, [openTab])

  const openUseCase = useCallback((code: string) => {
    setUseCaseFocus({ code, at: Date.now() })
    openTab({ type: 'view', view: 'use-cases' })
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
    if (selection?.scope === 'uml' && selection.kind === 'element' && umlHandle && focusName === 0) umlHandle.focus(selection.id)
  }, [selection, umlHandle, focusName])

  /* Menus e atalhos ------------------------------------------------------------ */

  const archActive = active?.type === 'arch'
  const commands = edit.commands
  const actions: MenuActions = {
    newDiagram: (kind) => setDialog({ type: 'newDiagram', kind }),
    generateUseCases: () => void generateUseCases(),
    exportImage: activeDiagram && umlHandle ? (format) => void umlHandle.exportImage(format) : null,
    exportMermaid: activeDiagram ? () => setDialog({ type: 'umlMermaid', diagram: activeDiagram })
      : archActive ? () => setDialog({ type: 'archMermaid' }) : null,
    generatePRD: () => void generatePRD(),
    undo: edit.canUndo ? () => void edit.undo() : null,
    redo: edit.canRedo ? () => void edit.redo() : null,
    cut: commands ? () => void edit.run('cut') : null,
    copy: commands ? () => void edit.run('copy') : null,
    paste: commands ? () => void edit.run('paste') : null,
    duplicate: commands ? () => void edit.run('duplicate') : null,
    selectAll: commands ? commands.selectAll : null,
    deleteSelection: commands ? () => void edit.run('deleteSelected') : null,
    fit: commands?.fitAll ?? null,
    zoomIn: commands?.zoomIn ?? null,
    zoomOut: commands?.zoomOut ?? null,
    autoLayout: archActive || activeDiagram ? () => void reorganize() : null,
    autoLayoutAll: () => void reorganizeAll(),
    importMermaid: () => setDialog({ type: 'archMermaid' }),
    validate: () => setDialog({ type: 'lint' }),
    mcp: () => setDialog({ type: 'mcp' }),
    pitch: () => setPitch(true),
    shortcuts: () => setDialog({ type: 'shortcuts' }),
    about: () => setDialog({ type: 'about' }),
    openView: (view) => openTab({ type: 'view', view }),
    resetLayout: layout.resetLayout,
    project: projects.actions,
  }

  // Desligados no Modo Pitch: o canvas da apresentação não é o do Shell.
  useShortcuts({
    actions,
    undo: () => void edit.undo(),
    redo: () => void edit.redo(),
    toggleLeft: () => layout.togglePanel('left'),
    toggleRight: () => layout.togglePanel('right'),
    cancelTool: () => setTool(SELECT_TOOL),
    rename: commands && selection?.scope === 'uml' && selection.kind === 'element' ? () => setFocusName(Date.now()) : null,
  }, { enabled: !pitch })

  /* Estados de carregamento ---------------------------------------------------- */

  if (loading && !snapshot) return <div className="grid h-full place-items-center"><Spinner label="Carregando projeto…" /></div>

  if (error && !snapshot) {
    // App desktop sem projeto (ou com a pasta indisponível): tela inicial.
    if (projects.actions) return <>{<WelcomeScreen actions={projects.actions} error={error} />}{projects.dialog}</>
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

  if (pitch) return <PitchMode snapshot={snapshot} onExit={() => setPitch(false)} />

  const { panels, explorerOpen, editorOpen } = layout
  const toolboxContext: ToolboxContext = archActive ? { type: 'arch' } : activeDiagram ? { type: 'uml', kind: activeDiagram.kind } : null

  const statusPath = archActive ? '.arch/diagrams/macro.json'
    : activeDiagram ? activeDiagram.file ?? `.arch/diagrams/${activeDiagram.kind}/${activeDiagram.id}.json`
      : active?.type === 'view' ? VIEW_LABEL[active.view] : ''
  const statusCounts = activeDiagram ? `${activeDiagram.elements.length} elementos · ${activeDiagram.relations.length} relações`
    : archActive ? `${snapshot.diagram.nodes.length} componentes · ${snapshot.diagram.edges.length} conexões` : null

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      <TitleBar actions={actions} projectName={snapshot.manifest.project_name} version={snapshot.manifest.version}
        panels={panels} onPanels={layout.setPanels} executive={executive} onExecutive={setExecutive}
        archActive={archActive} generating={generating} />

      {/* Área de trabalho ------------------------------------------------------- */}
      <div className="flex min-h-0 flex-1">
        {panels.left && (
          <>
            <aside className="flex shrink-0 flex-col bg-chrome" style={{ width: layout.left.width }}>
              <PanelHeader title="Toolbox">
                <IconAction label="Ocultar Toolbox (Ctrl+B)" onClick={() => layout.setPanels({ ...panels, left: false })}>
                  <PanelLeftClose size={13} />
                </IconAction>
              </PanelHeader>
              <div className="min-h-0 flex-1 overflow-y-auto">
                <Toolbox context={toolboxContext} tool={tool} onTool={setTool} />
              </div>
            </aside>
            <Splitter axis="x" label="Redimensionar Toolbox" value={layout.left.width} min={LEFT_MIN} max={layout.left.max}
              onChange={layout.left.setWidth} onReset={() => layout.left.setWidth(LEFT_DEFAULT)} />
          </>
        )}

        <main className="flex min-w-0 flex-1 flex-col">
          <TabStrip tabs={workspace.tabs} active={active} diagrams={diagrams} onActivate={workspace.activate} onClose={closeTab} />
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
                onSelectionChange={(sel) => setMultiCount(sel.nodes.length + sel.edges.length)}
                onReady={setArchHandle}
                onAutoLayout={() => void reorganize()}
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
                onSelect={(sel) => { setFocusName(0); setSelection(sel ? { scope: 'uml', diagramId: activeDiagram.id, ...sel } : null) }}
                onSelectionChange={(sel) => setMultiCount(sel.elements.length + sel.relations.length)}
                onCreated={(id) => { setFocusName(Date.now()); setSelection({ scope: 'uml', diagramId: activeDiagram.id, kind: 'element', id }) }}
                onRename={(id) => {
                  setSelection({ scope: 'uml', diagramId: activeDiagram.id, kind: 'element', id })
                  setFocusName(Date.now())
                  layout.revealEditor()
                }}
                onReady={setUmlHandle}
                onAutoLayout={() => void reorganize()}
                onZoom={setZoom}
                highlight={highlight}
              />
            )}
            {active?.type === 'view' && (
              <div className="h-full overflow-hidden bg-background">
                {active.view === 'document' && <DocumentPanel snapshot={snapshot} onReorganize={() => void reorganizeAll()} />}
                {active.view === 'requirements' && <RequirementsPanel snapshot={snapshot} />}
                {active.view === 'use-cases' && (
                  <UseCasesPanel key={useCaseFocus?.at ?? 'use-cases'} snapshot={snapshot} focusCode={useCaseFocus?.code} />
                )}
                {active.view === 'adrs' && <AdrPanel snapshot={snapshot} />}
                {active.view === 'ai-prd' && <AiPrdView snapshot={snapshot} generating={generating} onGeneratePRD={() => void generatePRD()} />}
                {active.view === 'proposal' && <ProposalView snapshot={snapshot} />}
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
            <Splitter axis="x" invert label="Redimensionar Model Explorer e Editor" value={layout.right.width} min={RIGHT_MIN} max={layout.right.max}
              onChange={layout.right.setWidth} onReset={() => layout.right.setWidth(RIGHT_DEFAULT)} />
            <aside ref={rightPanel} className="flex shrink-0 flex-col bg-chrome" style={{ width: layout.right.width }}>
              <div className={cn('flex min-h-0 flex-col', explorerOpen && !editorOpen && 'flex-1')}
                style={explorerOpen && editorOpen ? { height: `${layout.explorerRatio * 100}%` } : undefined}>
                <SectionHeader title="Model Explorer" open={explorerOpen} maximized={layout.maximized === 'explorer'}
                  onToggle={() => layout.toggleSection('explorer')} onMaximize={() => layout.maximizeSection('explorer')} />
                {explorerOpen && (
                  <div className="min-h-0 flex-1 bg-background">
                    <ModelExplorer
                      snapshot={snapshot}
                      activeTab={active}
                      selection={selection}
                      onOpen={openTab}
                      onSelectElement={selectUmlElement}
                      onSelectArchNode={selectArchNode}
                      onCreateDiagram={(kind) => setDialog({ type: 'newDiagram', kind })}
                      onRenameDiagram={(diagram) => setDialog({ type: 'renameDiagram', diagram })}
                      onDeleteDiagram={(d) => void deleteDiagram(d)}
                      onShowMermaid={(diagram) => setDialog({ type: 'umlMermaid', diagram })}
                      onGenerateUseCases={() => void generateUseCases()}
                    />
                  </div>
                )}
              </div>
              {explorerOpen && editorOpen && (
                <Splitter axis="y" label="Redimensionar a altura do Model Explorer e do Editor"
                  value={layout.explorerRatio} min={0.12} max={0.88}
                  scale={() => 1 / (rightPanel.current?.clientHeight || 800)}
                  onChange={layout.setExplorerRatio} onReset={() => layout.setExplorerRatio(EXPLORER_DEFAULT)} />
              )}
              <div className={cn('flex min-h-0 flex-col', editorOpen && 'flex-1', explorerOpen && !editorOpen && 'border-t')}>
                <SectionHeader title="Editor" open={editorOpen} maximized={layout.maximized === 'editor'}
                  onToggle={() => layout.toggleSection('editor')} onMaximize={() => layout.maximizeSection('editor')} />
                {editorOpen && (
                  <div className="min-h-0 flex-1 overflow-y-auto bg-background">
                    <ErrorBoundary key={`${activeKey}|${selection?.id ?? ''}`} area="Editor">
                    <EditorPanel
                      snapshot={snapshot}
                      active={active}
                      diagram={activeDiagram}
                      selection={selection}
                      focusName={focusName}
                      multiCount={multiCount}
                      onCommand={(name) => void edit.run(name)}
                      onClose={() => setSelection(null)}
                      onFocusArch={(id) => selectArchNode(id)}
                      onOpenUseCase={openUseCase}
                      onRename={(diagram) => setDialog({ type: 'renameDiagram', diagram })}
                      onMermaid={(diagram) => setDialog({ type: 'umlMermaid', diagram })}
                      onExport={(f) => void umlHandle?.exportImage(f)}
                      archFocus={(id) => archHandle?.focusNode(id)}
                    />
                    </ErrorBoundary>
                  </div>
                )}
              </div>
            </aside>
          </>
        )}
      </div>

      <StatusBar connected={connected} path={statusPath} counts={statusCounts} toolActive={tool.mode !== 'select'}
        lastEvent={lastEvent} lint={lint} onLint={() => setDialog({ type: 'lint' })}
        zoom={archActive || activeDiagram ? zoom : null} />

      {projects.dialog}

      <ShellDialogs dialog={dialog} onClose={() => setDialog(null)} snapshot={snapshot}
        onDiagramCreated={(d) => { void refresh({ silent: true }).then(() => openTab({ type: 'uml', id: d.id })) }} />
    </div>
  )
}
