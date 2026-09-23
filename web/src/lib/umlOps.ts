// Operações de edição de diagramas UML feitas no cliente e gravadas com um único
// PUT (uma entrada no histórico de desfazer): excluir seleção, copiar/colar e
// criar um elemento já conectado ao selecionado.

import { shortId, type ClipboardData } from './clipboard'
import type { UMLDiagram, UMLElement, UMLElementType, UMLKind, UMLRelation, UMLRelationType } from './types'
import { SEQ, elementSize, nextElementName, sortedMessages } from './umlMeta'

export interface Selected { elements: string[]; relations: string[] }

/* Excluir --------------------------------------------------------------------- */

export function removeSelection(d: UMLDiagram, sel: Selected): UMLDiagram {
  const gone = new Set(sel.elements)
  const goneRel = new Set(sel.relations)
  const elements = d.elements
    .filter((e) => !gone.has(e.id))
    .map((e) => (e.parent_id && gone.has(e.parent_id) ? { ...e, parent_id: undefined } : e))
  const relations = d.relations.filter((r) => !goneRel.has(r.id) && !gone.has(r.source) && !gone.has(r.target))
  return { ...d, elements, relations }
}

/* Copiar / colar -------------------------------------------------------------- */

export function copySelection(d: UMLDiagram, sel: Selected): ClipboardData | null {
  const ids = new Set(sel.elements)
  const elements = d.elements.filter((e) => ids.has(e.id))
  if (elements.length === 0) return null
  // Relações vão junto quando as duas pontas foram copiadas.
  const relations = d.relations.filter((r) => ids.has(r.source) && ids.has(r.target))
  return { scope: 'uml', kind: d.kind, elements: structuredClone(elements), relations: structuredClone(relations) }
}

export function pasteInto(d: UMLDiagram, clip: ClipboardData, offset: number): { next: UMLDiagram; ids: string[] } | null {
  if (clip.scope !== 'uml' || clip.kind !== d.kind) return null
  const map = new Map<string, string>()
  for (const el of clip.elements) map.set(el.id, `el-${shortId()}`)
  const lifelines = d.elements.filter((e) => e.type === 'lifeline').length
  let lifelineIndex = 0
  const pasted: UMLElement[] = clip.elements.map((el) => {
    const parent = el.parent_id ? map.get(el.parent_id) ?? (d.elements.some((x) => x.id === el.parent_id) ? el.parent_id : undefined) : undefined
    const position = el.type === 'lifeline'
      ? { x: 40 + (lifelines + lifelineIndex++) * 200, y: SEQ.top }
      : { x: el.position.x + offset, y: el.position.y + offset }
    return { ...el, id: map.get(el.id)!, parent_id: parent, position }
  })
  let order = sortedMessages(d).length
  const relations: UMLRelation[] = clip.relations.map((r) => ({
    ...r,
    id: `rel-${shortId()}`,
    source: map.get(r.source)!,
    target: map.get(r.target)!,
    ...(r.type === 'message' ? { order: ++order } : {}),
  }))
  return {
    next: { ...d, elements: [...d.elements, ...pasted], relations: [...d.relations, ...relations] },
    ids: pasted.map((e) => e.id),
  }
}

/* Criar conectado ------------------------------------------------------------- */

export interface ConnectOption {
  label: string
  /** Tipo do novo elemento; ausente = relação consigo mesmo (auto-mensagem). */
  element?: UMLElementType
  relation: UMLRelationType
  /** `out`: selecionado → novo. `in`: novo → selecionado. */
  direction: 'out' | 'in'
  place: 'right' | 'left' | 'above' | 'below'
  extra?: Partial<UMLRelation>
}

const note: ConnectOption = { label: 'Nota', element: 'note', relation: 'note_link', direction: 'out', place: 'right' }

/** Atalhos de criação por tipo de elemento, como a barra de "Quick Edit" do StarUML. */
export function connectOptions(kind: UMLKind, type: UMLElementType): ConnectOption[] {
  switch (kind) {
    case 'class':
      if (type === 'class') return [
        { label: 'Classe associada', element: 'class', relation: 'association', direction: 'out', place: 'right' },
        { label: 'Associação direcionada', element: 'class', relation: 'directed_association', direction: 'out', place: 'right' },
        { label: 'Superclasse', element: 'class', relation: 'generalization', direction: 'out', place: 'above' },
        { label: 'Subclasse', element: 'class', relation: 'generalization', direction: 'in', place: 'below' },
        { label: 'Interface realizada', element: 'interface', relation: 'realization', direction: 'out', place: 'above' },
        { label: 'Parte (composição)', element: 'class', relation: 'composition', direction: 'in', place: 'below' },
        { label: 'Parte (agregação)', element: 'class', relation: 'aggregation', direction: 'in', place: 'below' },
        { label: 'Dependência', element: 'class', relation: 'dependency', direction: 'out', place: 'right' },
        note,
      ]
      if (type === 'interface') return [
        { label: 'Classe que implementa', element: 'class', relation: 'realization', direction: 'in', place: 'below' },
        { label: 'Interface derivada', element: 'interface', relation: 'generalization', direction: 'in', place: 'below' },
        note,
      ]
      if (type === 'enum') return [note]
      return []
    case 'usecase':
      if (type === 'actor') return [
        { label: 'Caso de uso associado', element: 'usecase', relation: 'association', direction: 'out', place: 'right' },
        { label: 'Ator especializado', element: 'actor', relation: 'generalization', direction: 'in', place: 'below' },
        note,
      ]
      if (type === 'usecase') return [
        { label: 'Ator associado', element: 'actor', relation: 'association', direction: 'in', place: 'left' },
        { label: 'Caso incluído («include»)', element: 'usecase', relation: 'include', direction: 'out', place: 'right' },
        { label: 'Extensão («extend»)', element: 'usecase', relation: 'extend', direction: 'in', place: 'below' },
        { label: 'Caso especializado', element: 'usecase', relation: 'generalization', direction: 'in', place: 'below' },
        note,
      ]
      return []
    case 'sequence':
      if (type === 'lifeline') return [
        { label: 'Mensagem para nova linha de vida', element: 'lifeline', relation: 'message', direction: 'out', place: 'right' },
        { label: 'Mensagem assíncrona para nova linha', element: 'lifeline', relation: 'message', direction: 'out', place: 'right', extra: { message_kind: 'async' } },
        { label: 'Auto-mensagem', relation: 'message', direction: 'out', place: 'right' },
        note,
      ]
      return []
    case 'state':
      if (type === 'final' || type === 'note') return []
      return [
        { label: 'Próximo estado', element: 'state', relation: 'transition', direction: 'out', place: 'right' },
        { label: 'Escolha', element: 'choice', relation: 'transition', direction: 'out', place: 'right' },
        { label: 'Estado final', element: 'final', relation: 'transition', direction: 'out', place: 'right' },
        ...(type === 'state' ? [{ label: 'Auto-transição', relation: 'transition' as const, direction: 'out' as const, place: 'right' as const }, note] : []),
      ]
  }
}

function overlaps(d: UMLDiagram, x: number, y: number, w: number, h: number): boolean {
  return d.elements.some((e) => {
    if (e.type === 'boundary' || e.type === 'package' || e.type === 'fragment') return false
    const s = elementSize(e)
    return x < e.position.x + s.w + 20 && x + w + 20 > e.position.x && y < e.position.y + s.h + 20 && y + h + 20 > e.position.y
  })
}

/** Cria o elemento e a relação num único passo; devolve o id do que selecionar. */
export function addConnected(d: UMLDiagram, sourceId: string, opt: ConnectOption): { next: UMLDiagram; select: { kind: 'element' | 'relation'; id: string } } | null {
  const src = d.elements.find((e) => e.id === sourceId)
  if (!src) return null
  let target = src
  const elements = [...d.elements]

  if (opt.element) {
    const size = elementSize({ type: opt.element } as UMLElement)
    const s = elementSize(src)
    let x: number
    let y: number
    if (opt.element === 'lifeline') {
      const maxX = Math.max(...d.elements.filter((e) => e.type === 'lifeline').map((e) => e.position.x))
      x = maxX + 200
      y = SEQ.top
    } else {
      const gap = 110
      x = opt.place === 'right' ? src.position.x + s.w + gap
        : opt.place === 'left' ? src.position.x - size.w - gap
          : src.position.x + (s.w - size.w) / 2
      y = opt.place === 'above' ? src.position.y - size.h - gap
        : opt.place === 'below' ? src.position.y + s.h + gap
          : src.position.y + (s.h - size.h) / 2
      // Desce/afasta até achar espaço livre.
      for (let i = 0; i < 20 && overlaps(d, x, y, size.w, size.h); i++) {
        if (opt.place === 'above' || opt.place === 'below') x += size.w + 40
        else y += size.h + 40
      }
    }
    target = {
      id: `el-${shortId()}`,
      type: opt.element,
      name: nextElementName(opt.element, d),
      position: { x: Math.round(x), y: Math.round(y) },
      ...(opt.element === 'note' ? { documentation: 'Nota' } : {}),
      ...(src.parent_id && opt.element === src.type ? { parent_id: src.parent_id } : {}),
    }
    elements.push(target)
  }

  const [from, to] = opt.direction === 'out' ? [src.id, target.id] : [target.id, src.id]
  const relation: UMLRelation = {
    id: `rel-${shortId()}`,
    type: opt.relation,
    source: from,
    target: to,
    ...(opt.relation === 'message' ? { order: sortedMessages(d).length + 1, name: 'mensagem()' } : {}),
    ...opt.extra,
  }
  return {
    next: { ...d, elements, relations: [...d.relations, relation] },
    select: opt.element ? { kind: 'element', id: target.id } : { kind: 'relation', id: relation.id },
  }
}

/** Descrição curta usada em toasts ("2 elementos e 1 relação"). */
export function describeCount(elements: number, relations: number): string {
  const parts = []
  if (elements) parts.push(`${elements} ${elements === 1 ? 'elemento' : 'elementos'}`)
  if (relations) parts.push(`${relations} ${relations === 1 ? 'relação' : 'relações'}`)
  return parts.join(' e ') || 'nada'
}

