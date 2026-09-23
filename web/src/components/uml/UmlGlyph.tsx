// Miniaturas da notação UML usadas na Toolbox e no Model Explorer, no mesmo
// espírito dos ícones do StarUML: cada ferramenta mostra a forma que cria.

import type { UMLElementType, UMLKind, UMLRelationType } from '../../lib/types'

const S = { fill: 'none', stroke: 'currentColor', strokeWidth: 1.2, strokeLinecap: 'round', strokeLinejoin: 'round' } as const

export function ElementGlyph({ type, size = 16 }: { type: UMLElementType; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden className="shrink-0">
      {ELEMENT[type]}
    </svg>
  )
}

export function RelationGlyph({ type, size = 16 }: { type: UMLRelationType; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden className="shrink-0">
      {RELATION[type]}
    </svg>
  )
}

export function KindGlyph({ kind, size = 16 }: { kind: UMLKind; size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" aria-hidden className="shrink-0">
      {KIND[kind]}
    </svg>
  )
}

const ELEMENT: Record<UMLElementType, React.ReactNode> = {
  actor: (<g {...S}><circle cx="8" cy="3.2" r="1.9" /><path d="M8 5.1v5M4.5 7h7M8 10.1l-3 4M8 10.1l3 4" /></g>),
  usecase: (<ellipse cx="8" cy="8" rx="6.8" ry="4.2" {...S} />),
  boundary: (<g {...S}><rect x="1.5" y="1.5" width="13" height="13" /><path d="M4 4.2h8" /></g>),
  note: (<g {...S}><path d="M2 2h9l3 3v9H2z" /><path d="M11 2v3h3" /></g>),
  class: (<g {...S}><rect x="1.5" y="2" width="13" height="12" /><path d="M1.5 6h13M1.5 10h13" /></g>),
  interface: (<g {...S}><circle cx="8" cy="5" r="3" /><path d="M8 8v6" /></g>),
  enum: (<g {...S}><rect x="1.5" y="2" width="13" height="12" /><path d="M1.5 6h13M4 8.5h6M4 11h6" /></g>),
  package: (<g {...S}><path d="M1.5 4.5V2.5h6v2" /><rect x="1.5" y="4.5" width="13" height="9.5" /></g>),
  lifeline: (<g {...S}><rect x="3" y="1.5" width="10" height="4.5" /><path d="M8 6v8.5" strokeDasharray="1.6 1.4" /></g>),
  fragment: (<g {...S}><rect x="1.5" y="2" width="13" height="12" /><path d="M1.5 5.5h4.5l1-1V2" /></g>),
  state: (<rect x="1.5" y="3.5" width="13" height="9" rx="3" {...S} />),
  initial: (<circle cx="8" cy="8" r="4.2" fill="currentColor" />),
  final: (<g><circle cx="8" cy="8" r="5.5" {...S} /><circle cx="8" cy="8" r="3" fill="currentColor" /></g>),
  choice: (<path d="M8 2l6 6-6 6-6-6z" {...S} />),
  fork: (<rect x="2" y="6.8" width="12" height="2.4" fill="currentColor" />),
  join: (<rect x="6.8" y="2" width="2.4" height="12" fill="currentColor" />),
  history: (<g><circle cx="8" cy="8" r="5.5" {...S} /><path d="M6.2 5.5v5M9.8 5.5v5M6.2 8h3.6" {...S} /></g>),
}

const line = (dashed = false) => <path d="M2 14L13 3" {...S} strokeDasharray={dashed ? '2 1.6' : undefined} />
const openHead = <path d="M8.5 3.2L13 3l-.2 4.5" {...S} />
const triangle = <path d="M13.5 2.5l-1.2 5-3.8-3.8z" {...S} fill="var(--background)" />
const diamond = (filled: boolean) => (
  <path d="M14 2l-.5 3.7-3.4 3.4-2-2 3.3-3.4z" {...S} fill={filled ? 'currentColor' : 'var(--background)'} />
)

const RELATION: Record<UMLRelationType, React.ReactNode> = {
  association: line(),
  directed_association: (<g>{line()}{openHead}</g>),
  include: (<g>{line(true)}{openHead}</g>),
  extend: (<g>{line(true)}{openHead}</g>),
  generalization: (<g><path d="M2 14l8.4-8.4" {...S} />{triangle}</g>),
  realization: (<g><path d="M2 14l8.4-8.4" {...S} strokeDasharray="2 1.6" />{triangle}</g>),
  dependency: (<g>{line(true)}{openHead}</g>),
  aggregation: (<g><path d="M2 14l7.6-7.6" {...S} />{diamond(false)}</g>),
  composition: (<g><path d="M2 14l7.6-7.6" {...S} />{diamond(true)}</g>),
  message: (<g><path d="M1.5 8h11" {...S} /><path d="M10 5.5L14 8l-4 2.5z" fill="currentColor" /></g>),
  transition: (<g><path d="M2 12c3-7 7-8 11-8" {...S} /><path d="M10 2.3L13.2 4 10.6 6.6" {...S} /></g>),
  note_link: line(true),
}

const KIND: Record<UMLKind, React.ReactNode> = {
  usecase: (<g {...S}><circle cx="3.6" cy="4" r="1.5" /><path d="M3.6 5.5v4M1.6 7h4M3.6 9.5l-1.6 3M3.6 9.5l1.6 3" /><ellipse cx="11.3" cy="8" rx="4" ry="2.6" /></g>),
  class: (<g {...S}><rect x="1.5" y="1.5" width="8" height="9" /><path d="M1.5 4.5h8M1.5 7.5h8" /><rect x="10.5" y="9" width="4" height="5.5" /></g>),
  sequence: (<g {...S}><rect x="1.5" y="1.5" width="4.5" height="3" /><rect x="10" y="1.5" width="4.5" height="3" /><path d="M3.8 4.5v10M12.2 4.5v10" strokeDasharray="1.4 1.2" /><path d="M3.8 8h7.4M9.4 6.8l1.8 1.2-1.8 1.2" /></g>),
  state: (<g {...S}><rect x="1.5" y="2" width="7" height="4.5" rx="1.6" /><rect x="7.5" y="9.5" width="7" height="4.5" rx="1.6" /><path d="M5 6.5v5h2.5" /></g>),
}
