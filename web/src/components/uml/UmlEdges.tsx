// Relações UML desenhadas como arestas "flutuantes": o ponto de ancoragem é a
// interseção da reta entre os centros com o contorno real da forma (elipse,
// círculo ou retângulo), como no StarUML — não uma alça fixa.

import {
  BaseEdge, EdgeLabelRenderer, useInternalNode, type Edge, type EdgeProps, type InternalNode, type Node,
} from '@xyflow/react'
import { memo } from 'react'
import { RELATION_STYLE, SEQ, transitionLabel, type MarkerId } from '../../lib/umlMeta'
import type { UMLElementType, UMLRelation } from '../../lib/types'

export interface UmlEdgeData extends Record<string, unknown> {
  rel: UMLRelation
  /** Posição da mensagem na sequência (0-based), só para `message`. */
  index?: number
}

export type UmlFlowEdge = Edge<UmlEdgeData, 'uml' | 'message'>

/* Marcadores --------------------------------------------------------------- */

export function UmlMarkers() {
  const stroke = 'var(--uml-stroke)'
  const common = { markerUnits: 'userSpaceOnUse' as const, orient: 'auto-start-reverse' }
  return (
    <svg width="0" height="0" className="absolute" aria-hidden>
      <defs>
        <marker id="uml-open" viewBox="0 0 12 12" refX="11.5" refY="6" markerWidth="12" markerHeight="12" {...common}>
          <path d="M1 1L11.5 6L1 11" fill="none" stroke={stroke} strokeWidth="1.3" strokeLinejoin="miter" />
        </marker>
        <marker id="uml-filled" viewBox="0 0 12 12" refX="11.5" refY="6" markerWidth="11" markerHeight="11" {...common}>
          <path d="M0.5 1L11.5 6L0.5 11Z" fill="var(--uml-solid)" stroke={stroke} strokeWidth="1" />
        </marker>
        <marker id="uml-triangle" viewBox="0 0 16 16" refX="15.3" refY="8" markerWidth="16" markerHeight="16" {...common}>
          <path d="M0.8 1L15.3 8L0.8 15Z" fill="var(--uml-fill)" stroke={stroke} strokeWidth="1.3" />
        </marker>
        <marker id="uml-diamond" viewBox="0 0 20 12" refX="19.3" refY="6" markerWidth="20" markerHeight="12" {...common}>
          <path d="M0.8 6L10 0.8L19.3 6L10 11.2Z" fill="var(--uml-fill)" stroke={stroke} strokeWidth="1.3" />
        </marker>
        <marker id="uml-diamond-filled" viewBox="0 0 20 12" refX="19.3" refY="6" markerWidth="20" markerHeight="12" {...common}>
          <path d="M0.8 6L10 0.8L19.3 6L10 11.2Z" fill="var(--uml-solid)" stroke={stroke} strokeWidth="1.3" />
        </marker>
      </defs>
    </svg>
  )
}

const markerUrl = (id?: MarkerId) => (id ? `url(#${id})` : undefined)

/* Geometria -------------------------------------------------------------- */

interface Box { x: number; y: number; w: number; h: number; shape: 'rect' | 'ellipse' }

const ELLIPTIC = new Set<UMLElementType>(['usecase', 'initial', 'final', 'history'])

function boxOf(node: InternalNode<Node>): Box {
  const { x, y } = node.internals.positionAbsolute
  const w = node.measured.width ?? node.width ?? 0
  const h = node.measured.height ?? node.height ?? 0
  const type = (node.data as { el?: { type?: UMLElementType } }).el?.type
  if (type === 'actor') {
    // O ator ocupa só a figura; o nome fica abaixo e não deve prender a aresta.
    return { x: x + w / 2 - 20, y, w: 40, h: 58, shape: 'rect' }
  }
  return { x, y, w, h, shape: type && ELLIPTIC.has(type) ? 'ellipse' : 'rect' }
}

function center(b: Box) { return { x: b.x + b.w / 2, y: b.y + b.h / 2 } }

/** Ponto do contorno de `b` na direção do ponto `to`. */
function borderPoint(b: Box, to: { x: number; y: number }) {
  const c = center(b)
  const dx = to.x - c.x
  const dy = to.y - c.y
  if (dx === 0 && dy === 0) return c
  const hw = b.w / 2
  const hh = b.h / 2
  let t: number
  if (b.shape === 'ellipse') {
    t = 1 / Math.sqrt((dx * dx) / (hw * hw) + (dy * dy) / (hh * hh))
  } else {
    t = Math.min(dx !== 0 ? hw / Math.abs(dx) : Infinity, dy !== 0 ? hh / Math.abs(dy) : Infinity)
  }
  return { x: c.x + dx * t, y: c.y + dy * t }
}

/**
 * Meio do segmento deslocado para o lado "de cima" (ou à direita, em linhas
 * verticais), para o rótulo não ficar sobre a linha — como o StarUML faz.
 */
function sideOf(p1: { x: number; y: number }, p2: { x: number; y: number }, offset: number) {
  const dx = p2.x - p1.x
  const dy = p2.y - p1.y
  const len = Math.hypot(dx, dy) || 1
  let nx = -dy / len
  let ny = dx / len
  if (ny > 0.2 || (Math.abs(ny) <= 0.2 && nx < 0)) { nx = -nx; ny = -ny }
  return { x: (p1.x + p2.x) / 2 + nx * offset, y: (p1.y + p2.y) / 2 + ny * offset, nx, ny }
}

/** Ponto ao longo do segmento, deslocado perpendicularmente (rótulos de ponta). */
function along(p1: { x: number; y: number }, p2: { x: number; y: number }, t: number, offset: number) {
  const dx = p2.x - p1.x
  const dy = p2.y - p1.y
  const len = Math.hypot(dx, dy) || 1
  const dist = Math.min(t * len, 28 + t * 4)
  return {
    x: p1.x + (dx / len) * dist + (-dy / len) * offset,
    y: p1.y + (dy / len) * dist + (dx / len) * offset,
  }
}

/* Aresta UML genérica -------------------------------------------------------- */

function EdgeText({ x, y, children, strong, anchor }: {
  x: number; y: number; children: React.ReactNode; strong?: boolean
  /** Direção normal à linha: o texto cresce para longe dela. */
  anchor?: { nx: number; ny: number }
}) {
  const tx = anchor ? -50 + anchor.nx * 50 : -50
  const ty = anchor ? -50 + anchor.ny * 50 : -50
  return (
    <div
      className="nodrag nopan pointer-events-none absolute whitespace-nowrap px-0.5 text-[11px] leading-tight"
      style={{
        transform: `translate(${tx}%, ${ty}%) translate(${x}px, ${y}px)`,
        color: 'var(--uml-text)',
        background: strong ? 'var(--canvas-bg)' : undefined,
      }}
    >
      {children}
    </div>
  )
}

function UmlEdgeImpl({ id, source, target, data, selected, style }: EdgeProps<UmlFlowEdge>) {
  const sourceNode = useInternalNode(source)
  const targetNode = useInternalNode(target)
  if (!sourceNode || !targetNode || !data) return null
  const rel = data.rel
  const look = RELATION_STYLE[rel.type]
  const s = boxOf(sourceNode)
  const t = boxOf(targetNode)

  let path: string
  let labelPos: { x: number; y: number; nx?: number; ny?: number }
  let p1: { x: number; y: number }
  let p2: { x: number; y: number }

  if (source === target) {
    // Auto-relação: laço no canto superior direito.
    p1 = { x: s.x + s.w * 0.72, y: s.y }
    p2 = { x: s.x + s.w, y: s.y + Math.min(s.h * 0.3, 24) }
    path = `M${p1.x},${p1.y} C${p1.x},${p1.y - 42} ${p2.x + 42},${p2.y} ${p2.x},${p2.y}`
    labelPos = { x: s.x + s.w + 16, y: s.y - 22 }
  } else {
    p1 = borderPoint(s, center(t))
    p2 = borderPoint(t, center(s))
    path = `M${p1.x},${p1.y} L${p2.x},${p2.y}`
    labelPos = sideOf(p1, p2, 11)
  }

  const label = rel.type === 'transition' ? transitionLabel(rel) : rel.name
  const color = selected ? 'var(--uml-select)' : 'var(--uml-stroke)'

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        markerEnd={markerUrl(look.marker)}
        interactionWidth={14}
        style={{ ...style, stroke: color, strokeWidth: selected ? 1.8 : 1.3, strokeDasharray: look.dashed ? '6 4' : undefined, fill: 'none' }}
      />
      <EdgeLabelRenderer>
        {(look.stereotype || label) && (
          <EdgeText x={labelPos.x} y={labelPos.y} strong
            anchor={labelPos.nx !== undefined ? { nx: labelPos.nx, ny: labelPos.ny ?? 0 } : undefined}>
            {look.stereotype && <div className="text-center">«{look.stereotype}»</div>}
            {label && <div className="text-center">{label}</div>}
          </EdgeText>
        )}
        {source !== target && (rel.source_multiplicity || rel.source_role) && (
          <EdgeText {...along(p1, p2, 0.12, 11)}>
            {[rel.source_role, rel.source_multiplicity].filter(Boolean).join('  ')}
          </EdgeText>
        )}
        {source !== target && (rel.target_multiplicity || rel.target_role) && (
          <EdgeText {...along(p2, p1, 0.12, -11)}>
            {[rel.target_role, rel.target_multiplicity].filter(Boolean).join('  ')}
          </EdgeText>
        )}
      </EdgeLabelRenderer>
    </>
  )
}

export const UmlEdge = memo(UmlEdgeImpl)

/* Mensagem de sequência ------------------------------------------------------ */

function MessageEdgeImpl({ id, source, target, data, selected }: EdgeProps<UmlFlowEdge>) {
  const sourceNode = useInternalNode(source)
  const targetNode = useInternalNode(target)
  if (!sourceNode || !targetNode || !data) return null
  const rel = data.rel
  const kind = rel.message_kind ?? 'sync'
  const sx = sourceNode.internals.positionAbsolute.x + (sourceNode.measured.width ?? 140) / 2
  const tx = targetNode.internals.positionAbsolute.x + (targetNode.measured.width ?? 140) / 2
  const top = Math.min(sourceNode.internals.positionAbsolute.y, targetNode.internals.positionAbsolute.y)
  const y = top + SEQ.header + SEQ.firstGap + (data.index ?? 0) * SEQ.step

  const dashed = kind === 'reply' || kind === 'create'
  const marker: MarkerId = kind === 'sync' || kind === 'destroy' ? 'uml-filled' : 'uml-open'
  const self = source === target
  const path = self
    ? `M${sx},${y} H${sx + 44} V${y + 18} H${sx + 2}`
    : `M${sx},${y} H${tx + (tx > sx ? -1 : 1)}`
  const prefix = kind === 'create' ? '«create» ' : kind === 'destroy' ? '«destroy» ' : ''
  const text = `${rel.order ?? (data.index ?? 0) + 1}: ${prefix}${rel.name ?? ''}`.trim()
  const labelX = self ? sx + 50 : (sx + tx) / 2
  const color = selected ? 'var(--uml-select)' : 'var(--uml-stroke)'

  return (
    <>
      <BaseEdge
        id={id}
        path={path}
        markerEnd={markerUrl(marker)}
        interactionWidth={16}
        style={{ stroke: color, strokeWidth: selected ? 1.8 : 1.3, strokeDasharray: dashed ? '6 4' : undefined, fill: 'none' }}
      />
      {kind === 'destroy' && (
        <path d={`M${tx - 8},${y - 8} L${tx + 8},${y + 8} M${tx + 8},${y - 8} L${tx - 8},${y + 8}`} stroke={color} strokeWidth="1.6" />
      )}
      <EdgeLabelRenderer>
        <div
          className="nodrag nopan pointer-events-none absolute whitespace-nowrap text-[11.5px]"
          style={{
            transform: self ? `translate(0, -50%) translate(${labelX}px, ${y + 9}px)` : `translate(-50%, -100%) translate(${labelX}px, ${y - 3}px)`,
            color: selected ? 'var(--uml-select)' : 'var(--uml-text)',
          }}
        >
          {text}
        </div>
      </EdgeLabelRenderer>
    </>
  )
}

export const MessageEdge = memo(MessageEdgeImpl)
