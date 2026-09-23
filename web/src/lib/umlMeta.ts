// Metadados da notação UML: o que cada tipo de diagrama oferece na Toolbox,
// tamanhos padrão, nomes iniciais e o estilo visual de cada relação.
// Espelha as tabelas de internal/model/uml.go.

import type {
  FragmentOperator, LifelineKind, MessageKind, UMLDiagram, UMLElement, UMLElementType, UMLKind,
  UMLRelation, UMLRelationType,
} from './types'

export interface KindMeta {
  label: string
  plural: string
  short: string
  elements: { title: string; types: UMLElementType[] }[]
  relations: UMLRelationType[]
  defaultRelation: UMLRelationType
}

export const UML_KINDS: UMLKind[] = ['usecase', 'class', 'sequence', 'state']

export const KIND_META: Record<UMLKind, KindMeta> = {
  usecase: {
    label: 'Diagrama de Casos de Uso', plural: 'Casos de Uso', short: 'Casos de uso',
    elements: [{ title: 'Casos de Uso', types: ['actor', 'usecase', 'boundary'] }, { title: 'Anotações', types: ['note'] }],
    relations: ['association', 'include', 'extend', 'generalization', 'dependency', 'note_link'],
    defaultRelation: 'association',
  },
  class: {
    label: 'Diagrama de Classes', plural: 'Classes', short: 'Classes',
    elements: [{ title: 'Classificadores', types: ['class', 'interface', 'enum', 'package'] }, { title: 'Anotações', types: ['note'] }],
    relations: ['association', 'directed_association', 'aggregation', 'composition', 'generalization', 'realization', 'dependency', 'note_link'],
    defaultRelation: 'association',
  },
  sequence: {
    label: 'Diagrama de Sequência', plural: 'Sequência', short: 'Sequência',
    elements: [{ title: 'Interação', types: ['lifeline', 'fragment'] }, { title: 'Anotações', types: ['note'] }],
    relations: ['message', 'note_link'],
    defaultRelation: 'message',
  },
  state: {
    label: 'Diagrama de Estados', plural: 'Estados', short: 'Estados',
    elements: [
      { title: 'Estados', types: ['state', 'initial', 'final'] },
      { title: 'Pseudoestados', types: ['choice', 'fork', 'join', 'history'] },
      { title: 'Anotações', types: ['note'] },
    ],
    relations: ['transition', 'note_link'],
    defaultRelation: 'transition',
  },
}

export const ELEMENT_LABEL: Record<UMLElementType, string> = {
  actor: 'Ator', usecase: 'Caso de Uso', boundary: 'Fronteira do Sistema', note: 'Nota',
  class: 'Classe', interface: 'Interface', enum: 'Enumeração', package: 'Pacote',
  lifeline: 'Linha de Vida', fragment: 'Fragmento Combinado',
  state: 'Estado', initial: 'Estado Inicial', final: 'Estado Final', choice: 'Escolha',
  fork: 'Fork', join: 'Join', history: 'Histórico',
}

export const RELATION_LABEL: Record<UMLRelationType, string> = {
  association: 'Associação', directed_association: 'Associação Direcionada', include: 'Inclusão',
  extend: 'Extensão', generalization: 'Generalização', realization: 'Realização',
  dependency: 'Dependência', aggregation: 'Agregação', composition: 'Composição',
  message: 'Mensagem', transition: 'Transição', note_link: 'Ligação de Nota',
}

/** Tamanhos padrão (mesmos da colocação automática do backend). */
export const DEFAULT_SIZE: Record<UMLElementType, { w: number; h: number }> = {
  actor: { w: 80, h: 100 }, usecase: { w: 160, h: 70 }, boundary: { w: 420, h: 400 }, note: { w: 180, h: 80 },
  class: { w: 200, h: 120 }, interface: { w: 200, h: 120 }, enum: { w: 200, h: 120 }, package: { w: 240, h: 160 },
  lifeline: { w: 140, h: 44 }, fragment: { w: 420, h: 180 },
  state: { w: 160, h: 70 }, initial: { w: 24, h: 24 }, final: { w: 28, h: 28 }, choice: { w: 36, h: 36 },
  fork: { w: 120, h: 8 }, join: { w: 120, h: 8 }, history: { w: 32, h: 32 },
}

/** Tipos que funcionam como contêiner visual (fundo, atrás das arestas). */
export const CONTAINER_TYPES = new Set<UMLElementType>(['boundary', 'package', 'fragment'])

/** Tipos cujo tamanho é fixo pela notação (não redimensionáveis). */
export const FIXED_SIZE_TYPES = new Set<UMLElementType>(['actor', 'initial', 'final', 'choice', 'history', 'lifeline'])

/** Pseudoestados e anotações dispensam nome. */
export const UNNAMED_TYPES = new Set<UMLElementType>(['initial', 'final', 'choice', 'fork', 'join', 'history', 'note'])

/** Quais tipos podem ficar dentro de quais contêineres (parent_id). */
export const CONTAINS: Partial<Record<UMLElementType, UMLElementType[]>> = {
  boundary: ['usecase'],
  package: ['class', 'interface', 'enum', 'package', 'note'],
  state: ['state', 'initial', 'final', 'choice', 'fork', 'join', 'history'],
}

export function isContainer(el: UMLElement, diagram: UMLDiagram): boolean {
  if (CONTAINER_TYPES.has(el.type)) return true
  // Estado composto: um estado que contém subestados.
  return el.type === 'state' && diagram.elements.some((c) => c.parent_id === el.id)
}

export function elementSize(el: UMLElement): { w: number; h: number } {
  const d = DEFAULT_SIZE[el.type] ?? { w: 160, h: 80 }
  return { w: el.width || d.w, h: el.height || d.h }
}

/** Nome inicial ao criar pela Toolbox: "Class1", "Ator2"… como no StarUML. */
export function nextElementName(type: UMLElementType, diagram: UMLDiagram): string {
  if (UNNAMED_TYPES.has(type) && type !== 'note') return ''
  const base: Partial<Record<UMLElementType, string>> = {
    actor: 'Ator', usecase: 'CasoDeUso', boundary: 'Sistema', class: 'Classe', interface: 'Interface',
    enum: 'Enumeracao', package: 'Pacote', lifeline: 'Objeto', fragment: 'Fragmento', state: 'Estado', note: '',
  }
  const prefix = base[type] ?? 'Elemento'
  if (!prefix) return ''
  const used = new Set(diagram.elements.map((e) => e.name))
  for (let i = 1; ; i++) if (!used.has(`${prefix}${i}`)) return `${prefix}${i}`
}

/* -------------------------------------------------------------------------- */
/* Estilo das relações                                                         */
/* -------------------------------------------------------------------------- */

export type MarkerId = 'uml-open' | 'uml-filled' | 'uml-triangle' | 'uml-diamond' | 'uml-diamond-filled'

export interface RelationStyle {
  dashed: boolean
  marker?: MarkerId
  stereotype?: string
}

export const RELATION_STYLE: Record<UMLRelationType, RelationStyle> = {
  association: { dashed: false },
  directed_association: { dashed: false, marker: 'uml-open' },
  include: { dashed: true, marker: 'uml-open', stereotype: 'include' },
  extend: { dashed: true, marker: 'uml-open', stereotype: 'extend' },
  generalization: { dashed: false, marker: 'uml-triangle' },
  realization: { dashed: true, marker: 'uml-triangle' },
  dependency: { dashed: true, marker: 'uml-open' },
  aggregation: { dashed: false, marker: 'uml-diamond' },
  composition: { dashed: false, marker: 'uml-diamond-filled' },
  message: { dashed: false, marker: 'uml-filled' },
  transition: { dashed: false, marker: 'uml-open' },
  note_link: { dashed: true },
}

export const MESSAGE_KINDS: { value: MessageKind; label: string }[] = [
  { value: 'sync', label: 'Síncrona' },
  { value: 'async', label: 'Assíncrona' },
  { value: 'reply', label: 'Retorno' },
  { value: 'create', label: 'Criação' },
  { value: 'destroy', label: 'Destruição' },
]

export const LIFELINE_KINDS: { value: LifelineKind; label: string }[] = [
  { value: 'participant', label: 'Participante' },
  { value: 'actor', label: 'Ator' },
  { value: 'boundary', label: 'Boundary' },
  { value: 'control', label: 'Control' },
  { value: 'entity', label: 'Entity' },
  { value: 'database', label: 'Banco de dados' },
]

export const FRAGMENT_OPERATORS: FragmentOperator[] = ['alt', 'opt', 'loop', 'par', 'break', 'critical', 'ref']

/** Rótulo de uma transição: `trigger [guard] / effect`. */
export function transitionLabel(r: UMLRelation): string {
  const parts = [r.trigger || r.name || '']
  if (r.guard) parts.push(`[${r.guard}]`)
  let out = parts.filter(Boolean).join(' ')
  if (r.effect) out += ` / ${r.effect}`
  return out.trim()
}

/** Relação padrão ao conectar dois elementos sem ferramenta explícita. */
export function relationFor(kind: UMLKind, source: UMLElement | undefined, target: UMLElement | undefined): UMLRelationType {
  if (source?.type === 'note' || target?.type === 'note') return 'note_link'
  return KIND_META[kind].defaultRelation
}

/** Mensagens ordenadas (a ordem define a posição vertical no diagrama). */
export function sortedMessages(diagram: UMLDiagram): UMLRelation[] {
  return diagram.relations
    .filter((r) => r.type === 'message')
    .sort((a, b) => (a.order ?? 0) - (b.order ?? 0))
}

/* Geometria do diagrama de sequência ---------------------------------------- */

export const SEQ = {
  top: 40,
  header: 44,
  firstGap: 36,
  step: 44,
  tail: 48,
}

export function messageY(index: number): number {
  return SEQ.top + SEQ.header + SEQ.firstGap + index * SEQ.step
}

export function lifelineHeight(messageCount: number): number {
  return SEQ.header + SEQ.firstGap + Math.max(messageCount, 1) * SEQ.step + SEQ.tail
}
