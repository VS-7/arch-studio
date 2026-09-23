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
import {
  ArrowLeftRight, ClipboardPaste, Copy, CopyPlus, Maximize, Pencil, Plus, Scissors, SquareDashedMousePointer,
  Trash2,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { clipboard } from '../../lib/clipboard'
import type { Tool } from '../../lib/tools'
import type { UMLDiagram, UMLElement, UMLElementType, UMLRelationType } from '../../lib/types'
import {
  CONTAINS, ELEMENT_LABEL, KIND_META, RELATION_LABEL, SEQ, elementSize, isContainer, lifelineHeight,
  nextElementName, relationFor, sortedMessages,
} from '../../lib/umlMeta'
import {
  addConnected, connectOptions, copySelection, describeCount, pasteInto, removeSelection,
  type ConnectOption, type Selected,
} from '../../lib/umlOps'
import { CanvasControls } from '../canvas/CanvasControls'
import { CanvasMenu, type CanvasCommands, type CanvasMenuItem, type CanvasMenuState } from '../canvas/CanvasMenu'
import { useToast } from '../ui'
import { MessageEdge, UmlEdge, UmlMarkers, type UmlFlowEdge } from './UmlEdges'
import { ElementGlyph, RelationGlyph } from './UmlGlyph'
import { UmlActionsContext, UmlNode, type UmlFlowNode, type UmlNodeActions } from './UmlNodes'

const nodeTypes = { uml: UmlNode }
const edgeTypes = { uml: UmlEdge, message: MessageEdge }

export interface UmlCanvasHandle extends CanvasCommands {
  focus: (id: string) => void
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
  /** Seleção completa (inclui seleção múltipla por caixa, Ctrl+clique e Ctrl+A). */
  onSelectionChange?: (sel: Selected) => void
  onCreated: (id: string) => void
  /** Pede ao Editor para focar o campo Nome do elemento (duplo clique / F2). */
  onRename?: (id: string) => void
  onReady?: (handle: UmlCanvasHandle) => void
  onZoom?: (zoom: number) => void
  highlight?: string | null
}

function toFlowNodes(d: UMLDiagram, selected: Set<string>, pending: string | null, highlight?: string | null): UmlFlowNode[] {
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
      selected: selected.has(el.id),
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

function toFlowEdges(d: UMLDiagram, selected: Set<string>): UmlFlowEdge[] {
  const order = new Map(sortedMessages(d).map((m, i) => [m.id, i]))
  return d.relations.map((rel) => ({
    id: rel.id,
    source: rel.source,
    target: rel.target,
    type: rel.type === 'message' ? 'message' as const : 'uml' as const,
    selected: selected.has(rel.id),
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
  diagram, tool, onToolDone, selectedId, onSelect, onSelectionChange, onCreated, onRename, onReady, onZoom, highlight,
}: Props) {
  const toast = useToast()
  // Callbacks do Shell mudam a cada render; guardá-los em ref mantém os comandos
  // estáveis (senão onReady → setHandle → novo render → novos callbacks: laço).
  const cb = useRef({ onSelect, onSelectionChange, onCreated, onRename, onToolDone })
  cb.current = { onSelect, onSelectionChange, onCreated, onRename, onToolDone }
  const [pending, setPending] = useState<string | null>(null)
  const [menu, setMenu] = useState<CanvasMenuState | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState<UmlFlowNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<UmlFlowEdge>([])
  const [instance, setInstance] = useState<ReactFlowInstance<UmlFlowNode, UmlFlowEdge> | null>(null)
  const wrapper = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const dragGroup = useRef<{ origin: XYPosition; children: Map<string, XYPosition> } | null>(null)
  // Seleção atual (elementos + relações), preservada entre recargas do snapshot.
  const selection = useRef<Selected>({ elements: selectedId ? [selectedId] : [], relations: [] })
  const [single, setSingle] = useState<string | null>(null)
  // Ids a selecionar assim que aparecerem no diagrama (depois de colar/criar).
  const pendingSelect = useRef<Selected | null>(null)
  const diagramRef = useRef(diagram)
  diagramRef.current = diagram
  const signature = `${diagram.id}|${diagram.last_modified}|${pending}|${highlight}`
  const lastSignature = useRef('')

  const selectedSet = () => new Set([...selection.current.elements, ...selection.current.relations])

  useEffect(() => {
    if (dragging.current || lastSignature.current === signature) return
    lastSignature.current = signature
    const want = pendingSelect.current
    if (want && want.elements.concat(want.relations).every((id) =>
      diagram.elements.some((e) => e.id === id) || diagram.relations.some((r) => r.id === id))) {
      selection.current = want
      pendingSelect.current = null
      cb.current.onSelectionChange?.(want)
    }
    // Descarta da seleção o que deixou de existir.
    selection.current = {
      elements: selection.current.elements.filter((id) => diagram.elements.some((e) => e.id === id)),
      relations: selection.current.relations.filter((id) => diagram.relations.some((r) => r.id === id)),
    }
    const set = selectedSet()
    setNodes(toFlowNodes(diagram, set, pending, highlight))
    setEdges(toFlowEdges(diagram, set))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [signature, diagram, setNodes, setEdges])

  // Seleção vinda de fora (Model Explorer, Editor) reflete no canvas.
  useEffect(() => {
    if (selectedId === null) return
    const isRel = diagramRef.current.relations.some((r) => r.id === selectedId)
    selection.current = isRel ? { elements: [], relations: [selectedId] } : { elements: [selectedId], relations: [] }
    setNodes((ns) => ns.map((n) => (n.selected === (n.id === selectedId) ? n : { ...n, selected: n.id === selectedId })))
    setEdges((es) => es.map((e) => (e.selected === (e.id === selectedId) ? e : { ...e, selected: e.id === selectedId })))
  }, [selectedId, setNodes, setEdges])

  const onFlowSelectionChange = useCallback(({ nodes: ns, edges: es }: { nodes: UmlFlowNode[]; edges: UmlFlowEdge[] }) => {
    selection.current = { elements: ns.map((n) => n.id), relations: es.map((e) => e.id) }
    setSingle(ns.length === 1 && es.length === 0 ? ns[0].id : null)
    cb.current.onSelectionChange?.(selection.current)
  }, [])

  // Trocar de ferramenta cancela uma relação em construção.
  useEffect(() => { setPending(null) }, [tool])

  /* Gravação --------------------------------------------------------------- */

  /** Grava o diagrama inteiro num único PUT (= uma entrada do histórico). */
  const commit = useCallback(async (next: UMLDiagram, select?: Selected) => {
    if (select) pendingSelect.current = select
    try {
      await api.saveUML(next)
      return true
    } catch (err) {
      pendingSelect.current = null
      toast('error', (err as Error).message)
      return false
    }
  }, [toast])

  const persist = useCallback(async (current: UmlFlowNode[], touched: Set<string>) => {
    const d = diagramRef.current
    const byId = new Map(current.map((n) => [n.id, n]))
    const sizes = new Map(current.map((n) => [n.id, { w: n.measured?.width ?? n.width ?? 0, h: n.measured?.height ?? n.height ?? 0 }]))
    const elements = d.elements.map((el) => {
      const n = byId.get(el.id)
      if (!n || !touched.has(el.id)) return el
      const next: UMLElement = { ...el, position: { x: Math.round(n.position.x), y: Math.round(n.position.y) } }
      // Só grava tamanho quando difere do padrão (o JSON fica enxuto). Classes
      // sem altura explícita crescem com o conteúdo até serem redimensionadas.
      if (el.type !== 'lifeline' && n.width && n.height) {
        const size = elementSize(el)
        if (Math.round(n.width) !== size.w) next.width = Math.round(n.width)
        if (Math.round(n.height) !== size.h) next.height = Math.round(n.height)
      }
      return next
    })
    // Recalcula a contenção (usecase ⊂ fronteira, classe ⊂ pacote, subestado ⊂ estado).
    const draft: UMLDiagram = { ...d, elements }
    const withParents = elements.map((el) => {
      if (!touched.has(el.id)) return el
      const pos = byId.get(el.id)?.position ?? el.position
      const parent = findContainer(draft, el, pos, sizes)
      return parent === el.parent_id ? el : { ...el, parent_id: parent }
    })
    await commit({ ...d, elements: withParents })
  }, [commit])

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
      pendingSelect.current = { elements: [el.id], relations: [] }
      cb.current.onCreated(el.id)
    } catch (err) {
      toast('error', (err as Error).message)
    }
  }, [diagram, toast])

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
      pendingSelect.current = { elements: [], relations: [rel.id] }
      cb.current.onSelect({ kind: 'relation', id: rel.id })
    } catch (err) {
      toast('error', (err as Error).message)
    }
  }, [diagram, toast])

  /** Barra rápida / menu: cria um elemento já ligado ao selecionado. */
  const quickAdd = useCallback(async (sourceId: string, opt: ConnectOption) => {
    const res = addConnected(diagramRef.current, sourceId, opt)
    if (!res) return
    const select: Selected = res.select.kind === 'element' ? { elements: [res.select.id], relations: [] } : { elements: [], relations: [res.select.id] }
    if (await commit(res.next, select)) {
      if (res.select.kind === 'element') cb.current.onCreated(res.select.id)
      else cb.current.onSelect({ kind: 'relation', id: res.select.id })
    }
  }, [commit])

  const addMember = useCallback(async (elId: string, which: 'attributes' | 'operations') => {
    const el = diagramRef.current.elements.find((e) => e.id === elId)
    if (!el) return
    const list = el[which] ?? []
    const name = which === 'attributes' ? `atributo${list.length + 1}` : `operacao${list.length + 1}`
    const member = which === 'attributes' ? { name, type: 'string', visibility: '-' as const } : { name, visibility: '+' as const }
    try {
      await api.updateUMLElement(diagram.id, elId, { [which]: [...list, member] })
      cb.current.onSelect({ kind: 'element', id: elId })
    } catch (err) { toast('error', (err as Error).message) }
  }, [diagram.id, toast])

  /* Comandos (atalhos e menus) -------------------------------------------------- */

  const commands = useMemo<CanvasCommands>(() => {
    const deleteSelected = async () => {
      const sel = selection.current
      if (!sel.elements.length && !sel.relations.length) return null
      const d = diagramRef.current
      const next = removeSelection(d, sel)
      const removed = describeCount(d.elements.length - next.elements.length, d.relations.length - next.relations.length)
      selection.current = { elements: [], relations: [] }
      cb.current.onSelectionChange?.(selection.current)
      cb.current.onSelect(null)
      return (await commit(next)) ? `${removed} excluído(s)` : null
    }
    const copy = () => {
      const clip = copySelection(diagramRef.current, selection.current)
      if (!clip || clip.scope !== 'uml') return null
      clipboard.set(clip)
      return `${describeCount(clip.elements.length, clip.relations.length)} copiado(s)`
    }
    const paste = async () => {
      const clip = clipboard.get()
      if (!clip) return null
      if (clip.scope !== 'uml' || clip.kind !== diagramRef.current.kind) {
        toast('error', 'O conteúdo copiado é de outro tipo de diagrama')
        return null
      }
      const res = pasteInto(diagramRef.current, clip, clipboard.nextOffset())
      if (!res) return null
      const ok = await commit(res.next, { elements: res.ids, relations: [] })
      return ok ? `${describeCount(res.ids.length, clip.relations.length)} colado(s)` : null
    }
    return {
      selectAll: () => {
        setNodes((ns) => ns.map((n) => ({ ...n, selected: true })))
        setEdges((es) => es.map((e) => ({ ...e, selected: true })))
      },
      deleteSelected,
      copy,
      cut: async () => (copy() ? deleteSelected().then((m) => (m ? m.replace('excluído', 'recortado') : m)) : null),
      paste,
      duplicate: async () => (copy() ? paste().then((m) => (m ? m.replace('colado', 'duplicado') : m)) : null),
      fitAll: () => void instance?.fitView({ padding: 0.15, duration: 400, maxZoom: 1.2 }),
      zoomIn: () => void instance?.zoomIn({ duration: 200 }),
      zoomOut: () => void instance?.zoomOut({ duration: 200 }),
    }
  }, [commit, instance, setEdges, setNodes, toast])

  useEffect(() => {
    if (!instance || !onReady) return
    onReady({
      ...commands,
      focus: (id) => {
        const node = instance.getNode(id)
        if (node) {
          const w = node.measured?.width ?? 160
          const h = Math.min(node.measured?.height ?? 80, 200)
          void instance.setCenter(node.position.x + w / 2, node.position.y + h / 2, { zoom: Math.max(instance.getZoom(), 1), duration: 450 })
        }
      },
      exportImage: async (format) => {
        if (wrapper.current) await exportDiagram(instance, wrapper.current, diagram.name, format)
      },
    })
  }, [instance, onReady, diagram.name, commands])

  /** Executa um comando e mostra o resultado (menus de contexto). */
  const run = useCallback((fn: () => Promise<string | null> | string | null) => {
    void Promise.resolve(fn()).then((msg) => { if (msg) toast('success', `${msg} · Ctrl+Z desfaz`) })
  }, [toast])

  /* Menus de contexto ------------------------------------------------------------ */

  const addItems = useCallback((at: XYPosition, parentId?: string, only?: UMLElementType[]): CanvasMenuItem[] => {
    const types = KIND_META[diagram.kind].elements.flatMap((g) => g.types).filter((t) => !only || only.includes(t))
    return types.map((type) => ({
      type: 'item' as const,
      label: ELEMENT_LABEL[type],
      icon: <ElementGlyph type={type} size={14} />,
      onSelect: () => void createElement(type, at, parentId),
    }))
  }, [createElement, diagram.kind])

  const editItems = useCallback((): CanvasMenuItem[] => [
    { type: 'item', label: 'Recortar', icon: <Scissors />, shortcut: 'Ctrl+X', onSelect: () => run(commands.cut) },
    { type: 'item', label: 'Copiar', icon: <Copy />, shortcut: 'Ctrl+C', onSelect: () => run(commands.copy) },
    { type: 'item', label: 'Duplicar', icon: <CopyPlus />, shortcut: 'Ctrl+D', onSelect: () => run(commands.duplicate) },
    { type: 'separator' },
    { type: 'item', label: 'Excluir', icon: <Trash2 />, shortcut: 'Del', destructive: true, onSelect: () => run(commands.deleteSelected) },
  ], [commands, run])

  const openPaneMenu = useCallback((clientX: number, clientY: number, title?: string) => {
    if (!instance) return
    const at = instance.screenToFlowPosition({ x: clientX, y: clientY })
    const clip = clipboard.get()
    setMenu({
      x: clientX, y: clientY,
      items: [
        ...(title ? [{ type: 'label' as const, label: title }] : []),
        { type: 'sub', label: 'Adicionar aqui', icon: <Plus />, items: addItems(at) },
        { type: 'separator' },
        { type: 'item', label: 'Colar', icon: <ClipboardPaste />, shortcut: 'Ctrl+V', disabled: !clip, onSelect: () => run(commands.paste) },
        { type: 'item', label: 'Selecionar tudo', icon: <SquareDashedMousePointer />, shortcut: 'Ctrl+A', onSelect: commands.selectAll },
        { type: 'item', label: 'Ajustar à tela', icon: <Maximize />, shortcut: 'Shift+1', onSelect: commands.fitAll },
      ],
    })
  }, [addItems, commands, instance, run])

  const nodeMenuItems = useCallback((el: UMLElement, at: XYPosition): CanvasMenuItem[] => {
    const options = connectOptions(diagram.kind, el.type)
    const inside = CONTAINS[el.type]
    const items: CanvasMenuItem[] = []
    if (options.length) {
      items.push({
        type: 'sub', label: 'Adicionar conectado', icon: <Plus />,
        items: options.map((opt) => ({
          type: 'item' as const, label: opt.label,
          icon: opt.element ? <ElementGlyph type={opt.element} size={14} /> : <RelationGlyph type={opt.relation} size={14} />,
          onSelect: () => void quickAdd(el.id, opt),
        })),
      })
    }
    if (inside) {
      const allowed = inside.filter((t) => KIND_META[diagram.kind].elements.some((g) => g.types.includes(t)))
      if (allowed.length) items.push({ type: 'sub', label: 'Adicionar dentro', icon: <Plus />, items: addItems(at, el.id, allowed) })
    }
    if (el.type === 'class' || el.type === 'interface') {
      if (el.type === 'class') items.push({ type: 'item', label: 'Adicionar atributo', icon: <Plus />, onSelect: () => void addMember(el.id, 'attributes') })
      items.push({ type: 'item', label: 'Adicionar operação', icon: <Plus />, onSelect: () => void addMember(el.id, 'operations') })
    }
    if (cb.current.onRename) items.push({ type: 'item', label: 'Renomear', icon: <Pencil />, shortcut: 'F2', onSelect: () => cb.current.onRename?.(el.id) })
    if (items.length) items.push({ type: 'separator' })
    return [...items, ...editItems()]
  }, [addItems, addMember, diagram, editItems, quickAdd])

  const selectOnly = useCallback((sel: Selected) => {
    selection.current = sel
    const set = new Set([...sel.elements, ...sel.relations])
    setNodes((ns) => ns.map((n) => ({ ...n, selected: set.has(n.id) })))
    setEdges((es) => es.map((e) => ({ ...e, selected: set.has(e.id) })))
    cb.current.onSelectionChange?.(sel)
  }, [setEdges, setNodes])

  const onNodeContextMenu = useCallback((event: React.MouseEvent, node: UmlFlowNode) => {
    event.preventDefault()
    if (!instance) return
    // Clique direito fora da seleção atual seleciona só o elemento clicado.
    if (!selection.current.elements.includes(node.id)) {
      selectOnly({ elements: [node.id], relations: [] })
      cb.current.onSelect({ kind: 'element', id: node.id })
    }
    const multi = selection.current.elements.length + selection.current.relations.length > 1
    const at = instance.screenToFlowPosition({ x: event.clientX, y: event.clientY })
    setMenu({
      x: event.clientX, y: event.clientY,
      items: multi
        ? [{ type: 'label', label: `${describeCount(selection.current.elements.length, selection.current.relations.length)} selecionado(s)` }, ...editItems()]
        : [{ type: 'label', label: node.data.el.name || ELEMENT_LABEL[node.data.el.type] }, ...nodeMenuItems(node.data.el, at)],
    })
  }, [editItems, instance, nodeMenuItems, selectOnly])

  const onEdgeContextMenu = useCallback((event: React.MouseEvent, edge: UmlFlowEdge) => {
    event.preventDefault()
    const rel = edge.data?.rel
    if (!rel) return
    if (!selection.current.relations.includes(edge.id)) {
      selectOnly({ elements: [], relations: [edge.id] })
      cb.current.onSelect({ kind: 'relation', id: edge.id })
    }
    setMenu({
      x: event.clientX, y: event.clientY,
      items: [
        { type: 'label', label: rel.name || RELATION_LABEL[rel.type] },
        ...(rel.source !== rel.target ? [{
          type: 'item' as const, label: 'Inverter sentido', icon: <ArrowLeftRight />,
          onSelect: () => void api.updateUMLRelation(diagram.id, rel.id, { source: rel.target, target: rel.source })
            .catch((err: unknown) => toast('error', (err as Error).message)),
        }] : []),
        { type: 'separator' },
        { type: 'item', label: 'Excluir', icon: <Trash2 />, shortcut: 'Del', destructive: true, onSelect: () => run(commands.deleteSelected) },
      ],
    })
  }, [commands, diagram.id, run, selectOnly, toast])

  /* Barra rápida sobre o elemento selecionado ------------------------------------- */

  const actions = useMemo<UmlNodeActions>(() => ({
    single,
    options: (el) => connectOptions(diagram.kind, el.type),
    onQuick: (id, opt) => void quickAdd(id, opt),
    onMore: (id, x, y) => {
      const el = diagramRef.current.elements.find((e) => e.id === id)
      if (!el || !instance) return
      setMenu({ x, y, items: nodeMenuItems(el, instance.screenToFlowPosition({ x, y })) })
    },
    onRename: (id) => cb.current.onRename?.(id),
  }), [diagram.kind, instance, nodeMenuItems, onRename, quickAdd, single])

  /* Cliques ------------------------------------------------------------------------ */

  const onConnect = useCallback((c: Connection) => {
    if (!c.source || !c.target) return
    void createRelation(c.source, c.target, tool.mode === 'relation' ? tool.type : undefined)
    if (tool.mode === 'relation') cb.current.onToolDone()
  }, [createRelation, tool])

  const onPaneClick = useCallback((event: React.MouseEvent) => {
    if (tool.mode === 'element' && instance) {
      void createElement(tool.type, instance.screenToFlowPosition({ x: event.clientX, y: event.clientY }))
      cb.current.onToolDone()
      return
    }
    if (pending) { setPending(null); return }
    cb.current.onSelect(null)
  }, [createElement, instance, pending, tool])

  const onNodeClick = useCallback((event: React.MouseEvent, node: UmlFlowNode) => {
    if (tool.mode === 'element' && instance) {
      const target = node.data.el
      const parent = CONTAINS[target.type]?.includes(tool.type) ? target.id : undefined
      void createElement(tool.type, instance.screenToFlowPosition({ x: event.clientX, y: event.clientY }), parent)
      cb.current.onToolDone()
      return
    }
    if (tool.mode === 'relation') {
      if (!pending) { setPending(node.id); return }
      void createRelation(pending, node.id, tool.type)
      setPending(null)
      cb.current.onToolDone()
      return
    }
    if (event.ctrlKey || event.metaKey || event.shiftKey) return
    cb.current.onSelect({ kind: 'element', id: node.id })
  }, [createElement, createRelation, instance, pending, tool])

  /* Arrasto (contêineres levam os filhos junto) ------------------------------- */

  const onNodeDragStart = useCallback((_: unknown, node: UmlFlowNode) => {
    dragging.current = true
    setMenu(null)
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
    <UmlActionsContext.Provider value={actions}>
    <div ref={wrapper} className="relative size-full" style={{ cursor }}
      onDrop={onDrop} onDragOver={(e) => { e.preventDefault(); e.dataTransfer.dropEffect = 'copy' }}
      onDoubleClick={(e) => {
        // Duplo clique no vazio abre "Adicionar aqui", como o Quick Add do StarUML.
        if ((e.target as HTMLElement).classList.contains('react-flow__pane')) openPaneMenu(e.clientX, e.clientY, 'Adicionar ao diagrama')
      }}>
      <ReactFlow<UmlFlowNode, UmlFlowEdge>
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
        onSelectionChange={onFlowSelectionChange}
        onConnect={onConnect}
        onInit={setInstance}
        onNodeClick={onNodeClick}
        onNodeDoubleClick={(_, node) => cb.current.onRename?.(node.id)}
        onEdgeClick={(e, edge) => { if (!(e.ctrlKey || e.metaKey || e.shiftKey)) cb.current.onSelect({ kind: 'relation', id: edge.id }) }}
        onPaneClick={onPaneClick}
        onPaneContextMenu={(e) => { e.preventDefault(); openPaneMenu(e.clientX, e.clientY) }}
        onNodeContextMenu={onNodeContextMenu}
        onEdgeContextMenu={onEdgeContextMenu}
        onSelectionContextMenu={(e) => {
          e.preventDefault()
          setMenu({ x: e.clientX, y: e.clientY, items: [{ type: 'label', label: `${describeCount(selection.current.elements.length, selection.current.relations.length)} selecionado(s)` }, ...editItems()] })
        }}
        onNodeDragStart={onNodeDragStart}
        onNodeDrag={onNodeDrag}
        onNodeDragStop={onNodeDragStop}
        onMove={(_, vp) => onZoom?.(vp.zoom)}
        onMoveStart={() => setMenu(null)}
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
        multiSelectionKeyCode={['Control', 'Meta', 'Shift']}
        deleteKeyCode={null}
        selectionKeyCode={null}
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
      <CanvasMenu menu={menu} onClose={() => setMenu(null)} />
      {pending && (
        <div className="pointer-events-none absolute left-1/2 top-3 -translate-x-1/2 rounded-md border bg-popover px-3 py-1.5 text-xs shadow-md">
          Clique no elemento de destino · <kbd className="font-mono">Esc</kbd> cancela
        </div>
      )}
      {diagram.elements.length === 0 && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <p className="max-w-sm text-center text-[13px] text-muted-foreground">
            Diagrama vazio. <b>Duplo clique</b> ou <b>clique direito</b> no canvas para adicionar um elemento,
            use a Toolbox ou peça a um agente de IA via MCP.
          </p>
        </div>
      )}
    </div>
    </UmlActionsContext.Provider>
  )
}
