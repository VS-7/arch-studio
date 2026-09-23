// Operações de edição do diagrama macro de arquitetura feitas no cliente e
// gravadas com um único PUT: colar e criar componente já conectado.

import { shortId, type ClipboardData } from './clipboard'
import { nodeMeta } from './nodeMeta'
import type { ArchEdge, ArchNode, Diagram, NodeType } from './types'

const NODE_W = 220
const NODE_H = 110

/** Protocolo inicial coerente com o tipo do destino (o mesmo do arrastar entre alças). */
export function defaultProtocol(targetType: string): string {
  return ({ database: 'SQL', cache: 'Redis', queue: 'AMQP', storage: 'S3', external_service: 'REST' } as Record<string, string>)[targetType] ?? 'REST'
}

export function uniqueLabel(d: Diagram, base: string): string {
  const used = new Set(d.nodes.map((n) => n.data.label.toLowerCase()))
  if (!used.has(base.toLowerCase())) return base
  for (let i = 2; ; i++) if (!used.has(`${base} ${i}`.toLowerCase())) return `${base} ${i}`
}

function slug(s: string): string {
  return s.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || shortId()
}

function uniqueId(d: Diagram, base: string): string {
  const used = new Set(d.nodes.map((n) => n.id))
  if (!used.has(base)) return base
  for (let i = 2; ; i++) if (!used.has(`${base}-${i}`)) return `${base}-${i}`
}

export function makeNode(d: Diagram, type: NodeType, label: string, position: { x: number; y: number }): ArchNode {
  const meta = nodeMeta(type)
  const finalLabel = uniqueLabel(d, label)
  return {
    id: uniqueId(d, `node-${slug(finalLabel)}`),
    type,
    position,
    data: { label: finalLabel, tier: meta.tier, status: 'pending', pricing: { complexity: 'medium' } },
    ...(type === 'group' ? { width: 520, height: 360 } : {}),
  }
}

export function makeEdge(source: ArchNode, target: ArchNode): ArchEdge {
  return {
    id: `edge-${source.id.replace(/^node-/, '')}-to-${target.id.replace(/^node-/, '')}`,
    source: source.id,
    target: target.id,
    type: 'smoothstep',
    animated: false,
    data: { protocol: defaultProtocol(target.type), complexity: 'medium' },
  }
}

function occupied(d: Diagram, x: number, y: number): boolean {
  return d.nodes.some((n) => n.type !== 'group' &&
    x < n.position.x + NODE_W + 30 && x + NODE_W + 30 > n.position.x &&
    y < n.position.y + NODE_H + 30 && y + NODE_H + 30 > n.position.y)
}

/** Novo componente à direita do selecionado, ligado a ele. */
export function addConnectedNode(d: Diagram, sourceId: string, type: NodeType): { next: Diagram; id: string } | null {
  const src = d.nodes.find((n) => n.id === sourceId)
  if (!src) return null
  let x = src.position.x + NODE_W + 100
  let y = src.position.y
  for (let i = 0; i < 20 && occupied(d, x, y); i++) y += NODE_H + 40
  const node = makeNode(d, type, nodeMeta(type).label, { x, y })
  const edge = makeEdge(src, node)
  return { next: { ...d, nodes: [...d.nodes, node], edges: [...d.edges, edge] }, id: node.id }
}

export function copyArch(d: Diagram, ids: string[]): ClipboardData | null {
  const set = new Set(ids)
  const nodes = d.nodes.filter((n) => set.has(n.id))
  if (!nodes.length) return null
  const edges = d.edges.filter((e) => set.has(e.source) && set.has(e.target))
  return { scope: 'arch', nodes: structuredClone(nodes), edges: structuredClone(edges) }
}

export function pasteArch(d: Diagram, clip: ClipboardData, offset: number): { next: Diagram; ids: string[] } | null {
  if (clip.scope !== 'arch') return null
  const map = new Map<string, ArchNode>()
  let draft: Diagram = { ...d, nodes: [...d.nodes] }
  for (const n of clip.nodes) {
    // Rótulos únicos: a IA e a API resolvem componentes pelo nome.
    const node: ArchNode = {
      ...n,
      ...makeNode(draft, n.type, n.data.label, { x: n.position.x + offset, y: n.position.y + offset }),
      data: { ...n.data, label: uniqueLabel(draft, n.data.label), status: 'pending' },
      parentId: undefined,
    }
    map.set(n.id, node)
    draft = { ...draft, nodes: [...draft.nodes, node] }
  }
  const edges = clip.edges.map((e) => ({
    ...e,
    ...makeEdge(map.get(e.source)!, map.get(e.target)!),
    data: { ...e.data, endpoints: undefined },
  }))
  return { next: { ...draft, edges: [...d.edges, ...edges] }, ids: [...map.values()].map((n) => n.id) }
}

export { NODE_H, NODE_W }
