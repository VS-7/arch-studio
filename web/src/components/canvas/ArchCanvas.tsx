// Canvas de arquitetura (RF004).
//
// O React Flow trabalha sobre uma cópia local dos nós para que arrastar seja
// fluido; a gravação em disco só acontece ao soltar o nó, e apenas as posições
// são alteradas — nenhum outro campo do macro.json é tocado pela UI de canvas.

import {
  Background, BackgroundVariant, ConnectionLineType, Controls, MiniMap, ReactFlow,
  addEdge, useEdgesState, useNodesState,
  type Connection, type Edge, type Node, type NodeChange, type OnConnect, type ReactFlowInstance,
} from '@xyflow/react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { nodeMeta } from '../../lib/nodeMeta'
import type { ArchEdge, ArchNode, Diagram } from '../../lib/types'
import { useToast } from '../ui'
import { ArchNodeView, GroupNodeView, type ArchFlowNode } from './ArchNodeView'

const nodeTypes = { arch: ArchNodeView, group: GroupNodeView }

export interface CanvasHandle {
  focusNode: (id: string) => void
  fitAll: () => void
}

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
}

function toFlowNodes(
  diagram: Diagram, executive: boolean, highlight: string | null, spotlight?: Set<string> | null,
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

function toFlowEdges(diagram: Diagram, executive: boolean, spotlight?: Set<string> | null): Edge[] {
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
}: Props) {
  const toast = useToast()
  const [nodes, setNodes, onNodesChange] = useNodesState<ArchFlowNode>([])
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([])
  const [instance, setInstance] = useState<ReactFlowInstance<ArchFlowNode, Edge> | null>(null)
  const wrapper = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const signature = `${diagram.last_modified}|${executive}|${highlight}|${spotlight ? [...spotlight].join(',') : ''}`
  const lastSignature = useRef('')

  // Sincroniza com o servidor apenas quando o diagrama realmente mudou e o
  // usuário não está no meio de um arrasto.
  useEffect(() => {
    if (dragging.current || lastSignature.current === signature) return
    lastSignature.current = signature
    setNodes(toFlowNodes(diagram, executive, highlight, spotlight))
    setEdges(toFlowEdges(diagram, executive, spotlight))
  }, [signature, diagram, executive, highlight, spotlight, setNodes, setEdges])

  useEffect(() => {
    if (!instance || !onReady) return
    onReady({
      focusNode: (id: string) => {
        const node = instance.getNode(id)
        if (node) void instance.setCenter(node.position.x + 110, node.position.y + 55, { zoom: 1.35, duration: 650 })
      },
      fitAll: () => void instance.fitView({ padding: 0.18, duration: 500 }),
    })
  }, [instance, onReady])

  const persistPositions = useCallback(async (moved: ArchFlowNode[]) => {
    const byId = new Map(moved.map((n) => [n.id, n.position]))
    const next: Diagram = {
      ...diagram,
      nodes: diagram.nodes.map((n) => (byId.has(n.id) ? { ...n, position: byId.get(n.id)! } : n)),
    }
    try {
      await api.saveDiagram(next)
      onDirty?.()
    } catch (err) {
      toast('error', `Não foi possível salvar o layout: ${(err as Error).message}`)
    }
  }, [diagram, onDirty, toast])

  const handleNodesChange = useCallback((changes: NodeChange<ArchFlowNode>[]) => {
    if (!interactive) return
    onNodesChange(changes)
  }, [interactive, onNodesChange])

  const onConnect: OnConnect = useCallback((connection: Connection) => {
    if (!connection.source || !connection.target || connection.source === connection.target) return
    const sourceLabel = diagram.nodes.find((n) => n.id === connection.source)?.data.label ?? connection.source
    const targetNode = diagram.nodes.find((n) => n.id === connection.target)
    // Protocolo inicial coerente com o tipo do destino; ajustável no inspetor.
    const protocol = targetNode
      ? ({ database: 'SQL', cache: 'Redis', queue: 'AMQP', storage: 'S3', external_service: 'REST' } as Record<string, string>)[targetNode.type] ?? 'REST'
      : 'REST'
    setEdges((eds) => addEdge({ ...connection, type: 'smoothstep', label: protocol }, eds))
    api.addEdge({ source_id: connection.source, target_id: connection.target, protocol })
      .then(() => toast('success', `${sourceLabel} → ${targetNode?.data.label ?? ''} conectados via ${protocol}`))
      .catch((err: unknown) => toast('error', (err as Error).message))
  }, [diagram.nodes, setEdges, toast])

  const onDrop = useCallback((event: React.DragEvent) => {
    event.preventDefault()
    const raw = event.dataTransfer.getData('application/archcode-node')
    if (!raw || !instance) return
    const { type } = JSON.parse(raw) as { type: string }
    const position = instance.screenToFlowPosition({ x: event.clientX, y: event.clientY })
    const meta = nodeMeta(type)
    const label = window.prompt(`Nome do novo componente (${meta.label})`, '')
    if (!label?.trim()) return
    api.addNode({ label: label.trim(), type, tier: meta.tier, position })
      .then((node: ArchNode) => { onSelect(node.id, 'node'); toast('success', `Componente "${node.data.label}" criado`) })
      .catch((err: unknown) => toast('error', (err as Error).message))
  }, [instance, onSelect, toast])

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

  return (
    <div className="h-full w-full" ref={wrapper} onDrop={onDrop} onDragOver={onDragOver}>
      <ReactFlow<ArchFlowNode, Edge>
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={interactive ? onConnect : undefined}
        onInit={setInstance}
        onNodeDragStart={() => { dragging.current = true }}
        onNodeDragStop={(_, __, dragged) => {
          dragging.current = false
          if (dragged?.length) void persistPositions(dragged as ArchFlowNode[])
        }}
        onNodeClick={(_, node) => onSelect(node.id, 'node')}
        onEdgeClick={(_, edge) => onSelect(edge.id, 'edge')}
        onPaneClick={() => onSelect(null, 'node')}
        connectionLineType={ConnectionLineType.SmoothStep}
        defaultViewport={defaultViewport}
        minZoom={0.15}
        maxZoom={2.5}
        snapToGrid
        snapGrid={[20, 20]}
        nodesDraggable={interactive}
        nodesConnectable={interactive}
        elementsSelectable={interactive}
        panOnScroll
        selectionOnDrag={interactive}
        deleteKeyCode={null}
        proOptions={{ hideAttribution: true }}
        className="h-full w-full"
      >
        {interactive && (
          <>
            <Background variant={BackgroundVariant.Dots} gap={22} size={1.2} color="var(--canvas-dot)" />
            <Controls position="bottom-left" showInteractive={false} />
            <MiniMap
              position="bottom-right"
              pannable
              zoomable
              nodeStrokeWidth={3}
              nodeColor={(node: Node) => nodeMeta(String((node.data as { __type?: string }).__type ?? 'compute')).color}
              maskColor="color-mix(in oklab, var(--canvas-bg) 72%, transparent)"
            />
          </>
        )}
      </ReactFlow>
      {selectedId === null && interactive && nodes.length === 0 && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <p className="text-sm text-muted-app">
            Arraste um componente da paleta ou peça a um agente de IA para modelar a arquitetura.
          </p>
        </div>
      )}
    </div>
  )
}

export type { ArchNode, ArchEdge }
