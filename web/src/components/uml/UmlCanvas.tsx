// Editor de diagramas UML (casos de uso, classes, sequência e estados).
//
// Como no ArchCanvas, o React Flow trabalha sobre uma cópia local para que
// arrastar seja fluido; a gravação acontece ao soltar/redimensionar. Criação e
// edição de elementos passam pelas rotas granulares da API, e o snapshot volta
// pelo evento `uml_changed`.

import {
  Background, BackgroundVariant, ConnectionMode, MiniMap, ReactFlow, ViewportPortal,
  getNodesBounds, getViewportForBounds, useEdgesState, useNodesState,
  type Connection, type NodeChange, type ReactFlowInstance, type XYPosition,
} from '@xyflow/react'
import { toPng, toSvg } from 'html-to-image'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../../lib/api'
import type { Tool } from '../../lib/tools'
import type { UMLDiagram, UMLElement, UMLElementType, UMLRelationType } from '../../lib/types'
import {
  CONTAINS, KIND_META, RELATION_LABEL, SEQ, elementSize, isContainer, lifelineHeight, nextElementName,
  relationFor, sortedMessages,
} from '../../lib/umlMeta'
import { CanvasControls } from '../canvas/CanvasControls'
import { useToast } from '../ui'
import { MessageEdge, UmlEdge, UmlMarkers, type UmlFlowEdge } from './UmlEdges'
import { UmlNode, type UmlFlowNode } from './UmlNodes'

const nodeTypes = { uml: UmlNode }
const edgeTypes = { uml: UmlEdge, message: MessageEdge }

export interface UmlCanvasHandle {
  focus: (id: string) => void
  fitAll: () => void
  zoomIn: () => void
  zoomOut: () => void
  exportImage: (format: 'png' | 'svg') => Promise<void>
}

/** Rasteriza o diagrama inteiro (não só a área visível), no tema atual. */
async function exportDiagram(
  instance: ReactFlowInstance<UmlFlowNode, UmlFlowEdge>, wrapper: HTMLElement, name: string, format: 'png' | 'svg',
) {
  const viewport = wrapper.querySelector<HTMLElement>('.react-flow__viewport')
  const nodes = instance.getNodes()
  if (!viewport || nodes.length === 0) return
  const bounds = getNodesBounds(nodes)
  const pad = 40
  const width = Math.ceil(bounds.width + pad * 2)
  const height = Math.ceil(bounds.height + pad * 2)
  const vp = getViewportForBounds(bounds, width, height, 0.1, 2, 0)
  const background = getComputedStyle(wrapper).getPropertyValue('--canvas-bg').trim() || '#ffffff'
  const options = {
    backgroundColor: background,
    width, height,
    pixelRatio: format === 'png' ? 2 : 1,
    style: { width: `${width}px`, height: `${height}px`, transform: `translate(${vp.x}px, ${vp.y}px) scale(${vp.zoom})` },
    // Alças de conexão e redimensionamento não fazem parte do desenho.
    filter: (node: HTMLElement) => !node.classList?.contains('react-flow__handle') && !node.classList?.contains('react-flow__resize-control'),
  }
  const url = format === 'png' ? await toPng(viewport, options) : await toSvg(viewport, options)
  const a = document.createElement('a')
  a.href = url
  a.download = `${name.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'diagrama'}.${format}`
  a.click()
}

interface Props {
  diagram: UMLDiagram
  tool: Tool
  onToolDone: () => void
  selectedId: string | null
  onSelect: (sel: { kind: 'element' | 'relation'; id: string } | null) => void
  onCreated: (id: string) => void
  onReady?: (handle: UmlCanvasHandle) => void
  onZoom?: (zoom: number) => void
  highlight?: string | null
}

function toFlowNodes(d: UMLDiagram, selectedId: string | null, pending: string | null, highlight?: string | null): UmlFlowNode[] {
  const msgCount = d.kind === 'sequence' ? sortedMessages(d).length : 0
  // A linha de vida desce até cobrir mensagens, fragmentos e notas do diagrama.
  const below = d.elements
    .filter((e) => e.type === 'fragment' || e.type === 'note')
    .reduce((max, e) => Math.max(max, e.position.y + elementSize(e).h - SEQ.top + 24), 0)
  const llHeight = Math.max(lifelineHeight(msgCount), below)
  const containers = new Set(d.elements.filter((e) => isContainer(e, d)).map((e) => e.id))
  // Contêineres primeiro e atrás das arestas (zIndex 0 < arestas 1 < elementos 2).
  const ordered = [...d.elements].sort((a, b) => Number(containers.has(b.id)) - Number(containers.has(a.id)))
  return ordered.map((el) => {
    const size = elementSize(el)
    const classifier = el.type === 'class' || el.type === 'interface' || el.type === 'enum'
    const isLifeline = el.type === 'lifeline'
    // Classificadores sem tamanho explícito crescem com o conteúdo, como no StarUML.
    const width = classifier && !el.width ? undefined : size.w
    const height = isLifeline ? llHeight : classifier && !el.height ? undefined : size.h
    return {
      id: el.id,
      type: 'uml' as const,
      position: isLifeline ? { x: el.position.x, y: SEQ.top } : el.position,
      width,
      height,
      style: { width, height, minWidth: classifier ? 160 : undefined },
      zIndex: containers.has(el.id) ? 0 : 2,
      selected: el.id === selectedId,
      data: {
        el,
        composite: el.type === 'state' && containers.has(el.id),
        lifelineHeight: isLifeline ? llHeight : undefined,
        pendingSource: pending === el.id,
        highlight: highlight === el.id,
      },
    }
  })
}

function toFlowEdges(d: UMLDiagram, selectedId: string | null): UmlFlowEdge[] {
  const order = new Map(sortedMessages(d).map((m, i) => [m.id, i]))
  return d.relations.map((rel) => ({
    id: rel.id,
    source: rel.source,
    target: rel.target,
    type: rel.type === 'message' ? 'message' as const : 'uml' as const,
    selected: rel.id === selectedId,
    zIndex: d.kind === 'sequence' ? 3 : 1,
    data: { rel, index: order.get(rel.id) },
  }))
}

/** Descendentes (por parent_id) de um elemento, em qualquer profundidade. */
function descendants(d: UMLDiagram, id: string): string[] {
  const out: string[] = []
  const stack = [id]
  while (stack.length) {
    const cur = stack.pop()!
    for (const e of d.elements) {
      if (e.parent_id === cur && !out.includes(e.id)) { out.push(e.id); stack.push(e.id) }
    }
  }
  return out
}

/** Contêiner mais interno que envolve o centro do elemento, respeitando CONTAINS. */
function findContainer(d: UMLDiagram, el: UMLElement, pos: XYPosition, sizes: Map<string, { w: number; h: number }>): string | undefined {
  const size = sizes.get(el.id) ?? elementSize(el)
  const cx = pos.x + size.w / 2
  const cy = pos.y + size.h / 2
  const exclude = new Set([el.id, ...descendants(d, el.id)])
  let best: { id: string; area: number } | undefined
  for (const c of d.elements) {
    if (exclude.has(c.id) || !CONTAINS[c.type]?.includes(el.type)) continue
    // Um estado simples só vira contêiner se já for composto ou for grande.
    const s = sizes.get(c.id) ?? elementSize(c)
    if (c.type === 'state' && !isContainer(c, d) && s.w < 240) continue
    if (cx >= c.position.x && cx <= c.position.x + s.w && cy >= c.position.y && cy <= c.position.y + s.h) {
      const area = s.w * s.h
      if (!best || area < best.area) best = { id: c.id, area }
    }
  }
  return best?.id
}

export function UmlCanvas({
  diagram, tool, onToolDone, selectedId, onSelect, onCreated, onReady, onZoom, highlight,
}: Props) {
  const toast = useToast()
  const [pending, setPending] = useState<string | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState<UmlFlowNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<UmlFlowEdge>([])
  const [instance, setInstance] = useState<ReactFlowInstance<UmlFlowNode, UmlFlowEdge> | null>(null)
  const wrapper = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const dragGroup = useRef<{ origin: XYPosition; children: Map<string, XYPosition> } | null>(null)
  const signature = `${diagram.id}|${diagram.last_modified}|${pending}|${highlight}`
  const lastSignature = useRef('')

  useEffect(() => {
    if (dragging.current || lastSignature.current === signature) return
    lastSignature.current = signature
    setNodes(toFlowNodes(diagram, selectedId, pending, highlight))
    setEdges(toFlowEdges(diagram, selectedId))
    // A seleção é aplicada no efeito abaixo; aqui só reagimos a mudanças do diagrama.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, diagram, setNodes, setEdges])

  // Seleção vinda de fora (Model Explorer) reflete no canvas.
  useEffect(() => {
    setNodes((ns) => ns.map((n) => (n.selected === (n.id === selectedId) ? n : { ...n, selected: n.id === selectedId })))
    setEdges((es) => es.map((e) => (e.selected === (e.id === selectedId) ? e : { ...e, selected: e.id === selectedId })))
  }, [selectedId, setNodes, setEdges])

  // Trocar de ferramenta cancela uma relação em construção.
  useEffect(() => { setPending(null) }, [tool])

  useEffect(() => {
    if (!instance || !onReady) return
    onReady({
      focus: (id) => {
        const node = instance.getNode(id)
        if (node) {
          const w = node.measured?.width ?? 160
          const h = Math.min(node.measured?.height ?? 80, 200)
          void instance.setCenter(node.position.x + w / 2, node.position.y + h / 2, { zoom: Math.max(instance.getZoom(), 1), duration: 450 })
        }
      },
      fitAll: () => void instance.fitView({ padding: 0.15, duration: 400, maxZoom: 1.2 }),
      zoomIn: () => void instance.zoomIn({ duration: 200 }),
      zoomOut: () => void instance.zoomOut({ duration: 200 }),
      exportImage: async (format) => {
        if (wrapper.current) await exportDiagram(instance, wrapper.current, diagram.name, format)
      },
    })
  }, [instance, onReady, diagram.name])

  /* Persistência ----------------------------------------------------------- */

  const persist = useCallback(async (current: UmlFlowNode[], touched: Set<string>) => {
    const byId = new Map(current.map((n) => [n.id, n]))
    const sizes = new Map(current.map((n) => [n.id, { w: n.measured?.width ?? n.width ?? 0, h: n.measured?.height ?? n.height ?? 0 }]))
    const elements = diagram.elements.map((el) => {
      const n = byId.get(el.id)
      if (!n || !touched.has(el.id)) return el
      const next: UMLElement = { ...el, position: { x: Math.round(n.position.x), y: Math.round(n.position.y) } }
      // Só grava tamanho quando difere do padrão (o JSON fica enxuto). Classes
      // sem altura explícita crescem com o conteúdo até serem redimensionadas.
      if (el.type !== 'lifeline' && n.width && n.height) {
        const current = elementSize(el)
        if (Math.round(n.width) !== current.w) next.width = Math.round(n.width)
        if (Math.round(n.height) !== current.h) next.height = Math.round(n.height)
      }
      return next
    })
    // Recalcula a contenção (usecase ⊂ fronteira, classe ⊂ pacote, subestado ⊂ estado).
    const draft: UMLDiagram = { ...diagram, elements }
    const withParents = elements.map((el) => {
      if (!touched.has(el.id)) return el
      const pos = byId.get(el.id)?.position ?? el.position
      const parent = findContainer(draft, el, pos, sizes)
      return parent === el.parent_id ? el : { ...el, parent_id: parent }
    })
    try {
      await api.saveUML({ ...diagram, elements: withParents })
    } catch (err) {
      toast('error', `Não foi possível salvar o diagrama: ${(err as Error).message}`)
    }
  }, [diagram, toast])

  const handleNodesChange = useCallback((changes: NodeChange<UmlFlowNode>[]) => {
    const lifelines = new Set(diagram.elements.filter((e) => e.type === 'lifeline').map((e) => e.id))
    const fixed = changes.map((c) =>
      c.type === 'position' && c.position && lifelines.has(c.id) ? { ...c, position: { x: c.position.x, y: SEQ.top } } : c)
    onNodesChange(fixed)
    const resized = fixed.filter((c) => c.type === 'dimensions' && c.resizing === false)
    if (resized.length && instance) {
      const ids = new Set(resized.map((c) => (c as { id: string }).id))
      // Aguarda o React Flow aplicar a última dimensão antes de ler o estado.
      window.setTimeout(() => void persist(instance.getNodes(), ids), 0)
    }
  }, [diagram.elements, instance, onNodesChange, persist])

  /* Criação ---------------------------------------------------------------- */

  const createElement = useCallback(async (type: UMLElementType, at: XYPosition, parentId?: string) => {
    const size = elementSize({ type } as UMLElement)
    const position = type === 'lifeline'
      ? { x: Math.round(at.x - size.w / 2), y: SEQ.top }
      : { x: Math.round(at.x - size.w / 2), y: Math.round(at.y - size.h / 2) }
    try {
      const el = await api.addUMLElement(diagram.id, {
        type, name: nextElementName(type, diagram), position, parent_id: parentId,
      })
      onCreated(el.id)
    } catch (err) {
      toast('error', (err as Error).message)
    }
  }, [diagram, onCreated, toast])

  const createRelation = useCallback(async (source: string, target: string, explicit?: UMLRelationType) => {
    const s = diagram.elements.find((e) => e.id === source)
    const t = diagram.elements.find((e) => e.id === target)
    let type = explicit ?? relationFor(diagram.kind, s, t)
    if (s?.type === 'note' || t?.type === 'note') type = 'note_link'
    if (!KIND_META[diagram.kind].relations.includes(type)) {
      toast('error', `${RELATION_LABEL[type]} não se aplica a este diagrama`)
      return
    }
    if (source === target && type !== 'message' && type !== 'transition') {
      toast('error', 'Somente mensagens e transições podem ligar um elemento a ele mesmo')
      return
    }
    try {
      const rel = await api.addUMLRelation(diagram.id, { type, source, target })
      onSelect({ kind: 'relation', id: rel.id })
    } catch (err) {
      toast('error', (err as Error).message)
    }
  }, [diagram, onSelect, toast])

  const onConnect = useCallback((c: Connection) => {
    if (!c.source || !c.target) return
    void createRelation(c.source, c.target, tool.mode === 'relation' ? tool.type : undefined)
    if (tool.mode === 'relation') onToolDone()
  }, [createRelation, onToolDone, tool])

  const onPaneClick = useCallback((event: React.MouseEvent) => {
    if (tool.mode === 'element' && instance) {
      void createElement(tool.type, instance.screenToFlowPosition({ x: event.clientX, y: event.clientY }))
      onToolDone()
      return
    }
    if (pending) { setPending(null); return }
    onSelect(null)
  }, [createElement, instance, onSelect, onToolDone, pending, tool])

  const onNodeClick = useCallback((event: React.MouseEvent, node: UmlFlowNode) => {
    if (tool.mode === 'element' && instance) {
      const target = node.data.el
      const parent = CONTAINS[target.type]?.includes(tool.type) ? target.id : undefined
      void createElement(tool.type, instance.screenToFlowPosition({ x: event.clientX, y: event.clientY }), parent)
      onToolDone()
      return
    }
    if (tool.mode === 'relation') {
      if (!pending) { setPending(node.id); return }
      void createRelation(pending, node.id, tool.type)
      setPending(null)
      onToolDone()
      return
    }
    onSelect({ kind: 'element', id: node.id })
  }, [createElement, createRelation, instance, onSelect, onToolDone, pending, tool])

  /* Arrasto (contêineres levam os filhos junto) ------------------------------- */

  const onNodeDragStart = useCallback((_: unknown, node: UmlFlowNode) => {
    dragging.current = true
    const children = descendants(diagram, node.id)
    if (!children.length || !instance) { dragGroup.current = null; return }
    dragGroup.current = {
      origin: { ...node.position },
      children: new Map(children.map((id) => [id, { ...(instance.getNode(id)?.position ?? { x: 0, y: 0 }) }])),
    }
  }, [diagram, instance])

  const onNodeDrag = useCallback((_: unknown, node: UmlFlowNode) => {
    const group = dragGroup.current
    if (!group) return
    const dx = node.position.x - group.origin.x
    const dy = node.position.y - group.origin.y
    setNodes((ns) => ns.map((n) => {
      const start = group.children.get(n.id)
      if (!start) return n
      const isLifeline = n.data.el.type === 'lifeline'
      return { ...n, position: { x: start.x + dx, y: isLifeline ? SEQ.top : start.y + dy } }
    }))
  }, [setNodes])

  const onNodeDragStop = useCallback((_: unknown, node: UmlFlowNode, dragged: UmlFlowNode[]) => {
    dragging.current = false
    const touched = new Set<string>([node.id, ...dragged.map((n) => n.id)])
    dragGroup.current?.children.forEach((_, id) => touched.add(id))
    dragGroup.current = null
    if (instance) void persist(instance.getNodes(), touched)
  }, [instance, persist])

  /* Arrastar da Toolbox -------------------------------------------------------- */

  const onDrop = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    const raw = event.dataTransfer.getData('application/archcode-uml')
    if (!raw || !instance) return
    const { type } = JSON.parse(raw) as { type: UMLElementType }
    if (!KIND_META[diagram.kind].elements.some((g) => g.types.includes(type))) return
    void createElement(type, instance.screenToFlowPosition({ x: event.clientX, y: event.clientY }))
  }, [createElement, diagram.kind, instance])

  const cursor = tool.mode === 'element' ? 'crosshair' : tool.mode === 'relation' ? 'alias' : undefined
  const fitOnMount = useMemo(() => {
    const vp = diagram.viewport
    return !vp || (vp.x === 0 && vp.y === 0 && (vp.zoom === 1 || !vp.zoom))
    // Só na montagem: depois o viewport é do usuário.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [diagram.id])

  return (
    <div ref={wrapper} className="relative size-full" style={{ cursor }}
      onDrop={onDrop} onDragOver={(e) => { e.preventDefault(); e.dataTransfer.dropEffect = 'copy' }}>
      <ReactFlow<UmlFlowNode, UmlFlowEdge>
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={onConnect}
        onInit={setInstance}
        onNodeClick={onNodeClick}
        onEdgeClick={(_, edge) => onSelect({ kind: 'relation', id: edge.id })}
        onPaneClick={onPaneClick}
        onNodeDragStart={onNodeDragStart}
        onNodeDrag={onNodeDrag}
        onNodeDragStop={onNodeDragStop}
        onMove={(_, vp) => onZoom?.(vp.zoom)}
        connectionMode={ConnectionMode.Loose}
        fitView={fitOnMount}
        fitViewOptions={{ padding: 0.15, maxZoom: 1.1 }}
        defaultViewport={fitOnMount ? undefined : diagram.viewport}
        minZoom={0.2}
        maxZoom={3}
        snapToGrid
        snapGrid={[10, 10]}
        panOnScroll
        selectionOnDrag
        panOnDrag={[1, 2]}
        deleteKeyCode={null}
        zoomOnDoubleClick={false}
        proOptions={{ hideAttribution: true }}
        className="size-full"
      >
        {/* Marcadores dentro do viewport: entram também na exportação de imagem. */}
        <ViewportPortal><UmlMarkers /></ViewportPortal>
        <Background variant={BackgroundVariant.Lines} gap={20} lineWidth={0.6} color="var(--canvas-grid)" />
        <CanvasControls />
        <MiniMap position="bottom-right" pannable zoomable nodeStrokeWidth={1.5} nodeBorderRadius={2}
          nodeColor="var(--uml-fill-alt)" nodeStrokeColor="var(--uml-stroke)"
          maskColor="color-mix(in oklab, var(--canvas-bg) 60%, transparent)" />
      </ReactFlow>
      {pending && (
        <div className="pointer-events-none absolute left-1/2 top-3 -translate-x-1/2 rounded-md border bg-popover px-3 py-1.5 text-xs shadow-md">
          Clique no elemento de destino · <kbd className="font-mono">Esc</kbd> cancela
        </div>
      )}
      {diagram.elements.length === 0 && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <p className="max-w-sm text-center text-[13px] text-muted-foreground">
            Diagrama vazio. Escolha um elemento na <b>Toolbox</b> e clique no canvas para posicioná-lo,
            ou peça a um agente de IA via MCP.
          </p>
        </div>
      )}
    </div>
  )
}
