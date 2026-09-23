// Canvas de arquitetura (RF004).
//
// O React Flow trabalha sobre uma cópia local dos nós para que arrastar seja
// fluido; a gravação em disco só acontece ao soltar o nó. Colar, excluir em lote
// e criar componente conectado gravam o diagrama inteiro num único PUT, o que
// vira um único passo de desfazer (Ctrl+Z).

import {
  Background, BackgroundVariant, ConnectionLineType, MiniMap, ReactFlow,
  addEdge, useEdgesState, useNodesState,
  type Connection, type Edge, type NodeChange, type OnConnect, type ReactFlowInstance,
} from '@xyflow/react'
import {
  ClipboardPaste, Copy, CopyPlus, Maximize, Plus, Scissors, SquareDashedMousePointer, Trash2,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { addConnectedNode, copyArch, defaultProtocol, pasteArch } from '../../lib/archOps'
import { clipboard } from '../../lib/clipboard'
import { NODE_META, NODE_TYPES, nodeMeta } from '../../lib/nodeMeta'
import type { Tool } from '../../lib/tools'
import type { ArchEdge, ArchNode, Diagram, NodeType } from '../../lib/types'
import { useToast } from '../ui'
import { ArchActionsContext, ArchNodeView, GroupNodeView, type ArchFlowNode, type ArchNodeActions } from './ArchNodeView'
import { CanvasControls } from './CanvasControls'
import { CanvasMenu, type CanvasCommands, type CanvasMenuItem, type CanvasMenuState } from './CanvasMenu'

const nodeTypes = { arch: ArchNodeView, group: GroupNodeView }

export interface CanvasHandle extends CanvasCommands {
  focusNode: (id: string) => void
}

/** Tipos oferecidos na barra rápida "criar conectado". */
export const QUICK_TYPES: NodeType[] = ['compute', 'database', 'cache', 'queue', 'gateway', 'external_service']

function plural(n: number, one: string, many: string) { return `${n} ${n === 1 ? one : many}` }

interface Props {
  diagram: Diagram
  executive: boolean
  highlight: string | null
  /** Ids visíveis; quando definido, os demais aparecem esmaecidos (Modo Pitch). */
  spotlight?: Set<string> | null
  selectedId: string | null
  onSelect: (id: string | null, kind: 'node' | 'edge') => void
  onReady?: (handle: CanvasHandle) => void
  interactive?: boolean
  onDirty?: () => void
  /** Ferramenta da Toolbox: com um tipo de componente ativo, clicar no canvas o cria. */
  tool?: Tool
  onToolDone?: () => void
  onZoom?: (zoom: number) => void
  /** Seleção completa (ids de componentes e conexões). */
  onSelectionChange?: (sel: { nodes: string[]; edges: string[] }) => void
}

/** Nome inicial de um componente criado pela Toolbox ("Serviço 1", "Cache 2"…). */
function nextLabel(diagram: Diagram, type: string): string {
  const base = nodeMeta(type).label
  const used = new Set(diagram.nodes.map((n) => n.data.label.toLowerCase()))
  for (let i = 1; ; i++) if (!used.has(`${base} ${i}`.toLowerCase())) return `${base} ${i}`
}

function toFlowNodes(
  diagram: Diagram, executive: boolean, highlight: string | null, spotlight?: Set<string> | null, selected?: Set<string>,
): ArchFlowNode[] {
  const visible = diagram.nodes.filter((n) => !executive || n.data.executive !== false)
  // Grupos primeiro: React Flow desenha na ordem do array, então eles ficam atrás.
  const ordered = [...visible].sort((a, b) => Number(b.type === 'group') - Number(a.type === 'group'))
  return ordered.map((n) => ({
    id: n.id,
    type: n.type === 'group' ? 'group' : 'arch',
    position: n.position,
    draggable: true,
    selectable: true,
    selected: selected?.has(n.id) ?? false,
    zIndex: n.type === 'group' ? 0 : 1,
    ...(n.type === 'group'
      ? { width: n.width || 520, height: n.height || 360, style: { width: n.width || 520, height: n.height || 360 } }
      : {}),
    data: {
      ...n.data,
      __type: n.type,
      __executive: executive,
      __highlight: highlight === n.id,
      __dimmed: spotlight ? !spotlight.has(n.id) : false,
    },
  })) as ArchFlowNode[]
}

function toFlowEdges(diagram: Diagram, executive: boolean, spotlight?: Set<string> | null, selected?: Set<string>): Edge[] {
  const visibleNodes = new Set(
    diagram.nodes.filter((n) => !executive || n.data.executive !== false).map((n) => n.id),
  )
  return diagram.edges
    .filter((e) => visibleNodes.has(e.source) && visibleNodes.has(e.target))
    .filter((e) => !executive || e.data.executive !== false)
    .map((e) => {
      const dimmed = spotlight ? !(spotlight.has(e.source) && spotlight.has(e.target)) : false
      const label = executive
        ? e.data.description || e.label || e.data.protocol
        : e.label || [e.data.protocol, e.data.port ? `:${e.data.port}` : ''].filter(Boolean).join(' ')
      return {
        id: e.id,
        source: e.source,
        target: e.target,
        type: e.type || 'smoothstep',
        animated: e.animated,
        selected: selected?.has(e.id) ?? false,
        label,
        labelShowBg: true,
        style: { opacity: dimmed ? 0.12 : 1 },
        labelStyle: { opacity: dimmed ? 0.12 : 1 },
        data: e.data,
      } satisfies Edge
    })
}

export function ArchCanvas({
  diagram, executive, highlight, spotlight, selectedId, onSelect, onReady, interactive = true, onDirty,
  tool, onToolDone, onZoom, onSelectionChange,
}: Props) {
  const toast = useToast()
  // Callbacks do Shell mudam a cada render; ref mantém os comandos estáveis.
  const cb = useRef({ onSelect, onSelectionChange, onDirty, onToolDone })
  cb.current = { onSelect, onSelectionChange, onDirty, onToolDone }
  const [nodes, setNodes, onNodesChange] = useNodesState<ArchFlowNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [instance, setInstance] = useState<ReactFlowInstance<ArchFlowNode, Edge> | null>(null)
  const [menu, setMenu] = useState<CanvasMenuState | null>(null)
  const [single, setSingle] = useState<string | null>(null)
  const wrapper = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const selection = useRef<{ nodes: string[]; edges: string[] }>({ nodes: [], edges: [] })
  const pendingSelect = useRef<string[] | null>(null)
  const diagramRef = useRef(diagram)
  diagramRef.current = diagram
  const signature = `${diagram.last_modified}|${executive}|${highlight}|${spotlight ? [...spotlight].join(',') : ''}`
  const lastSignature = useRef('')

  // Sincroniza com o servidor apenas quando o diagrama realmente mudou e o
  // usuário não está no meio de um arrasto; a seleção atual é preservada.
  useEffect(() => {
    if (dragging.current || lastSignature.current === signature) return
    lastSignature.current = signature
    const want = pendingSelect.current
    if (want && want.every((id) => diagram.nodes.some((n) => n.id === id))) {
      selection.current = { nodes: want, edges: [] }
      pendingSelect.current = null
      cb.current.onSelectionChange?.(selection.current)
      if (want.length === 1) cb.current.onSelect(want[0], 'node')
    }
    selection.current = {
      nodes: selection.current.nodes.filter((id) => diagram.nodes.some((n) => n.id === id)),
      edges: selection.current.edges.filter((id) => diagram.edges.some((e) => e.id === id)),
    }
    const set = new Set([...selection.current.nodes, ...selection.current.edges])
    setNodes(toFlowNodes(diagram, executive, highlight, spotlight, set))
    setEdges(toFlowEdges(diagram, executive, spotlight, set))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, diagram, executive, highlight, spotlight, setNodes, setEdges])

  // Seleção vinda de fora (Model Explorer) reflete no canvas.
  useEffect(() => {
    if (selectedId === null) return
    const isEdge = diagramRef.current.edges.some((e) => e.id === selectedId)
    selection.current = isEdge ? { nodes: [], edges: [selectedId] } : { nodes: [selectedId], edges: [] }
    setNodes((ns) => ns.map((n) => (n.selected === (n.id === selectedId) ? n : { ...n, selected: n.id === selectedId })))
    setEdges((es) => es.map((e) => (e.selected === (e.id === selectedId) ? e : { ...e, selected: e.id === selectedId })))
  }, [selectedId, setNodes, setEdges])

  const onFlowSelectionChange = useCallback(({ nodes: ns, edges: es }: { nodes: ArchFlowNode[]; edges: Edge[] }) => {
    selection.current = { nodes: ns.map((n) => n.id), edges: es.map((e) => e.id) }
    setSingle(ns.length === 1 && es.length === 0 ? ns[0].id : null)
    cb.current.onSelectionChange?.(selection.current)
  }, [])

  /** Grava o diagrama inteiro num único PUT (= um passo de desfazer). */
  const commit = useCallback(async (next: Diagram, select?: string[]) => {
    if (select) pendingSelect.current = select
    try {
      await api.saveDiagram(next)
      cb.current.onDirty?.()
      return true
    } catch (err) {
      pendingSelect.current = null
      toast('error', (err as Error).message)
      return false
    }
  }, [toast])

  const persistPositions = useCallback(async (moved: ArchFlowNode[]) => {
    const byId = new Map(moved.map((n) => [n.id, n.position]))
    const d = diagramRef.current
    await commit({ ...d, nodes: d.nodes.map((n) => (byId.has(n.id) ? { ...n, position: byId.get(n.id)! } : n)) })
  }, [commit])

  const handleNodesChange = useCallback((changes: NodeChange<ArchFlowNode>[]) => {
    if (!interactive) return
    onNodesChange(changes)
  }, [interactive, onNodesChange])

  const onConnect: OnConnect = useCallback((connection: Connection) => {
    if (!connection.source || !connection.target || connection.source === connection.target) return
    const sourceLabel = diagram.nodes.find((n) => n.id === connection.source)?.data.label ?? connection.source
    const targetNode = diagram.nodes.find((n) => n.id === connection.target)
    // Protocolo inicial coerente com o tipo do destino; ajustável no inspetor.
    const protocol = targetNode ? defaultProtocol(targetNode.type) : 'REST'
    setEdges((eds) => addEdge({ ...connection, type: 'smoothstep', label: protocol }, eds))
    api.addEdge({ source_id: connection.source, target_id: connection.target, protocol })
      .then(() => toast('success', `${sourceLabel} → ${targetNode?.data.label ?? ''} conectados via ${protocol}`))
      .catch((err: unknown) => toast('error', (err as Error).message))
  }, [diagram.nodes, setEdges, toast])

  const createAtFlow = useCallback((type: NodeType, p: { x: number; y: number }) => {
    const meta = nodeMeta(type)
    const position = { x: Math.round(p.x - 110), y: Math.round(p.y - 55) }
    api.addNode({ label: nextLabel(diagramRef.current, type), type, tier: meta.tier, position })
      .then((node: ArchNode) => { pendingSelect.current = [node.id]; cb.current.onSelect(node.id, 'node'); toast('success', `Componente "${node.data.label}" criado — renomeie no Editor`) })
      .catch((err: unknown) => toast('error', (err as Error).message))
  }, [toast])

  const createAt = useCallback((type: NodeType, screen: { x: number; y: number }) => {
    if (instance) createAtFlow(type, instance.screenToFlowPosition(screen))
  }, [createAtFlow, instance])

  const quickAdd = useCallback(async (sourceId: string, type: NodeType) => {
    const res = addConnectedNode(diagramRef.current, sourceId, type)
    if (res && await commit(res.next, [res.id])) cb.current.onSelect(res.id, 'node')
  }, [commit])

  /* Comandos (atalhos e menus) ------------------------------------------------ */

  const commands = useMemo<CanvasCommands>(() => {
    const deleteSelected = async () => {
      const { nodes: ns, edges: es } = selection.current
      if (!ns.length && !es.length) return null
      const d = diagramRef.current
      const gone = new Set(ns)
      const goneEdges = new Set(es)
      const next: Diagram = {
        ...d,
        nodes: d.nodes.filter((n) => !gone.has(n.id)).map((n) => (n.parentId && gone.has(n.parentId) ? { ...n, parentId: undefined } : n)),
        edges: d.edges.filter((e) => !goneEdges.has(e.id) && !gone.has(e.source) && !gone.has(e.target)),
      }
      const msg = [ns.length && plural(d.nodes.length - next.nodes.length, 'componente', 'componentes'),
        d.edges.length - next.edges.length && plural(d.edges.length - next.edges.length, 'conexão', 'conexões')].filter(Boolean).join(' e ')
      selection.current = { nodes: [], edges: [] }
      cb.current.onSelectionChange?.(selection.current)
      cb.current.onSelect(null, 'node')
      return (await commit(next)) ? `${msg} excluído(s)` : null
    }
    const copy = () => {
      const clip = copyArch(diagramRef.current, selection.current.nodes)
      if (!clip || clip.scope !== 'arch') return null
      clipboard.set(clip)
      return `${plural(clip.nodes.length, 'componente copiado', 'componentes copiados')}`
    }
    const paste = async () => {
      const clip = clipboard.get()
      if (!clip) return null
      if (clip.scope !== 'arch') { toast('error', 'O conteúdo copiado é de um diagrama UML'); return null }
      const res = pasteArch(diagramRef.current, clip, clipboard.nextOffset())
      if (!res) return null
      return (await commit(res.next, res.ids)) ? `${plural(res.ids.length, 'componente colado', 'componentes colados')}` : null
    }
    return {
      selectAll: () => {
        setNodes((ns) => ns.map((n) => ({ ...n, selected: true })))
        setEdges((es) => es.map((e) => ({ ...e, selected: true })))
      },
      deleteSelected,
      copy,
      cut: async () => (copy() ? deleteSelected() : null),
      paste,
      duplicate: async () => (copy() ? paste() : null),
      fitAll: () => void instance?.fitView({ padding: 0.18, duration: 500 }),
      zoomIn: () => void instance?.zoomIn({ duration: 200 }),
      zoomOut: () => void instance?.zoomOut({ duration: 200 }),
    }
  }, [commit, instance, setEdges, setNodes, toast])

  useEffect(() => {
    if (!instance || !onReady) return
    onReady({
      ...commands,
      focusNode: (id: string) => {
        const node = instance.getNode(id)
        if (node) void instance.setCenter(node.position.x + 110, node.position.y + 55, { zoom: 1.35, duration: 650 })
      },
    })
  }, [instance, onReady, commands])

  const run = useCallback((fn: () => Promise<string | null> | string | null) => {
    void Promise.resolve(fn()).then((msg) => { if (msg) toast('success', `${msg} · Ctrl+Z desfaz`) })
  }, [toast])

  /* Menus de contexto ------------------------------------------------------------- */

  const typeItems = useCallback((onPick: (type: NodeType) => void, types: NodeType[] = NODE_TYPES): CanvasMenuItem[] =>
    types.map((type) => {
      const Icon = NODE_META[type].icon
      return { type: 'item' as const, label: NODE_META[type].label, icon: <Icon />, onSelect: () => onPick(type) }
    }), [])

  const editItems = useCallback((): CanvasMenuItem[] => [
    { type: 'item', label: 'Recortar', icon: <Scissors />, shortcut: 'Ctrl+X', onSelect: () => run(commands.cut) },
    { type: 'item', label: 'Copiar', icon: <Copy />, shortcut: 'Ctrl+C', onSelect: () => run(commands.copy) },
    { type: 'item', label: 'Duplicar', icon: <CopyPlus />, shortcut: 'Ctrl+D', onSelect: () => run(commands.duplicate) },
    { type: 'separator' },
    { type: 'item', label: 'Excluir', icon: <Trash2 />, shortcut: 'Del', destructive: true, onSelect: () => run(commands.deleteSelected) },
  ], [commands, run])

  const openPaneMenu = useCallback((clientX: number, clientY: number, title?: string) => {
    if (!instance || !interactive) return
    const at = instance.screenToFlowPosition({ x: clientX, y: clientY })
    setMenu({
      x: clientX, y: clientY,
      items: [
        ...(title ? [{ type: 'label' as const, label: title }] : []),
        { type: 'sub', label: 'Adicionar componente', icon: <Plus />, items: typeItems((type) => createAtFlow(type, at)) },
        { type: 'separator' },
        { type: 'item', label: 'Colar', icon: <ClipboardPaste />, shortcut: 'Ctrl+V', disabled: !clipboard.get(), onSelect: () => run(commands.paste) },
        { type: 'item', label: 'Selecionar tudo', icon: <SquareDashedMousePointer />, shortcut: 'Ctrl+A', onSelect: commands.selectAll },
        { type: 'item', label: 'Ajustar à tela', icon: <Maximize />, shortcut: 'Shift+1', onSelect: commands.fitAll },
      ],
    })
  }, [commands, createAtFlow, instance, interactive, run, typeItems])

  const nodeMenu = useCallback((id: string, x: number, y: number) => {
    const node = diagramRef.current.nodes.find((n) => n.id === id)
    if (!node) return
    const multi = selection.current.nodes.length + selection.current.edges.length > 1
    setMenu({
      x, y,
      items: multi
        ? [{ type: 'label', label: `${selection.current.nodes.length + selection.current.edges.length} itens selecionados` }, ...editItems()]
        : [
            { type: 'label', label: node.data.label },
            ...(node.type !== 'group'
              ? [{ type: 'sub' as const, label: 'Adicionar conectado', icon: <Plus />, items: typeItems((type) => void quickAdd(id, type), NODE_TYPES.filter((t) => t !== 'group')) },
                 { type: 'separator' as const }]
              : []),
            ...editItems(),
          ],
    })
  }, [editItems, quickAdd, typeItems])

  const selectOnly = useCallback((sel: { nodes: string[]; edges: string[] }) => {
    selection.current = sel
    const set = new Set([...sel.nodes, ...sel.edges])
    setNodes((ns) => ns.map((n) => ({ ...n, selected: set.has(n.id) })))
    setEdges((es) => es.map((e) => ({ ...e, selected: set.has(e.id) })))
    cb.current.onSelectionChange?.(sel)
  }, [setEdges, setNodes])

  const actions = useMemo<ArchNodeActions>(() => ({
    single: interactive ? single : null,
    types: QUICK_TYPES,
    onQuick: (id, type) => void quickAdd(id, type),
    onMore: (id, x, y) => nodeMenu(id, x, y),
  }), [interactive, nodeMenu, quickAdd, single])

  const onDrop = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    const raw = event.dataTransfer.getData('application/archcode-node')
    if (!raw) return
    const { type } = JSON.parse(raw) as { type: NodeType }
    createAt(type, { x: event.clientX, y: event.clientY })
  }, [createAt])

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    event.dataTransfer.dropEffect = 'copy'
  }, [])

  const defaultViewport = useMemo(
    () => ({ x: diagram.viewport?.x ?? 0, y: diagram.viewport?.y ?? 0, zoom: diagram.viewport?.zoom || 1 }),
    // Viewport inicial só na montagem; movimentos posteriores são do usuário.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  )

  // Viewport nunca ajustado pelo usuário (0,0,1): enquadra o diagrama inteiro.
  const fitOnMount = useMemo(() => {
    const vp = diagram.viewport
    return !vp || (vp.x === 0 && vp.y === 0 && (!vp.zoom || vp.zoom === 1))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <ArchActionsContext.Provider value={actions}>
    <div className="h-full w-full" ref={wrapper} onDrop={onDrop} onDragOver={onDragOver}
      style={{ cursor: tool?.mode === 'arch' ? 'crosshair' : undefined }}
      onDoubleClick={(e) => {
        if ((e.target as HTMLElement).classList.contains('react-flow__pane')) openPaneMenu(e.clientX, e.clientY, 'Adicionar à arquitetura')
      }}>
      <ReactFlow<ArchFlowNode, Edge>
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
        onSelectionChange={interactive ? onFlowSelectionChange : undefined}
        onConnect={interactive ? onConnect : undefined}
        onInit={setInstance}
        onNodeDragStart={() => { dragging.current = true; setMenu(null) }}
        onNodeDragStop={(_, __, dragged) => {
          dragging.current = false
          if (dragged?.length) void persistPositions(dragged as ArchFlowNode[])
        }}
        onNodeClick={(e, node) => { if (!(e.ctrlKey || e.metaKey || e.shiftKey)) cb.current.onSelect(node.id, 'node') }}
        onEdgeClick={(e, edge) => { if (!(e.ctrlKey || e.metaKey || e.shiftKey)) cb.current.onSelect(edge.id, 'edge') }}
        onPaneClick={(event) => {
          if (tool?.mode === 'arch') {
            createAt(tool.type, { x: event.clientX, y: event.clientY })
            cb.current.onToolDone?.()
            return
          }
          cb.current.onSelect(null, 'node')
        }}
        onPaneContextMenu={(e) => { e.preventDefault(); openPaneMenu(e.clientX, e.clientY) }}
        onNodeContextMenu={(e, node) => {
          e.preventDefault()
          if (!interactive) return
          if (!selection.current.nodes.includes(node.id)) {
            selectOnly({ nodes: [node.id], edges: [] })
            cb.current.onSelect(node.id, 'node')
          }
          nodeMenu(node.id, e.clientX, e.clientY)
        }}
        onEdgeContextMenu={(e, edge) => {
          e.preventDefault()
          if (!interactive) return
          if (!selection.current.edges.includes(edge.id)) {
            selectOnly({ nodes: [], edges: [edge.id] })
            cb.current.onSelect(edge.id, 'edge')
          }
          const data = edge.data as { protocol?: string } | undefined
          setMenu({
            x: e.clientX, y: e.clientY,
            items: [
              { type: 'label', label: `Conexão ${data?.protocol ?? ''}`.trim() },
              { type: 'item', label: 'Excluir', icon: <Trash2 />, shortcut: 'Del', destructive: true, onSelect: () => run(commands.deleteSelected) },
            ],
          })
        }}
        onSelectionContextMenu={(e) => {
          e.preventDefault()
          setMenu({ x: e.clientX, y: e.clientY, items: [{ type: 'label', label: `${selection.current.nodes.length + selection.current.edges.length} itens selecionados` }, ...editItems()] })
        }}
        onMove={(_, vp) => onZoom?.(vp.zoom)}
        onMoveStart={() => setMenu(null)}
        connectionLineType={ConnectionLineType.SmoothStep}
        defaultViewport={defaultViewport}
        fitView={fitOnMount}
        fitViewOptions={{ padding: 0.15, maxZoom: 1.1 }}
        minZoom={0.15}
        maxZoom={2.5}
        snapToGrid
        snapGrid={[20, 20]}
        nodesDraggable={interactive}
        nodesConnectable={interactive}
        elementsSelectable={interactive}
        panOnScroll
        selectionOnDrag={interactive}
        panOnDrag={interactive ? [1, 2] : true}
        multiSelectionKeyCode={['Control', 'Meta', 'Shift']}
        selectionKeyCode={null}
        deleteKeyCode={null}
        zoomOnDoubleClick={false}
        proOptions={{ hideAttribution: true }}
        className="h-full w-full"
      >
        {interactive && (
          <>
            <Background variant={BackgroundVariant.Lines} gap={20} lineWidth={0.6} color="var(--canvas-grid)" />
            <CanvasControls />
            <MiniMap
              position="bottom-right"
              pannable
              zoomable
              nodeStrokeWidth={3}
              nodeColor="var(--uml-fill-alt)" nodeStrokeColor="var(--uml-stroke)" nodeBorderRadius={2}
              maskColor="color-mix(in oklab, var(--canvas-bg) 70%, transparent)"
            />
          </>
        )}
      </ReactFlow>
      <CanvasMenu menu={menu} onClose={() => setMenu(null)} />
      {selectedId === null && interactive && nodes.length === 0 && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <p className="text-[13px] text-muted-foreground">
            Duplo clique ou clique direito no canvas para adicionar um componente, ou peça a um agente de IA.
          </p>
        </div>
      )}
    </div>
    </ArchActionsContext.Provider>
  )
}

export type { ArchNode, ArchEdge }
