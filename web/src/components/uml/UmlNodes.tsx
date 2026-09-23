// Formas da notação UML renderizadas no canvas.
//
// Um componente por família de forma, todos lendo cores dos tokens --uml-*
// (claro/escuro). As arestas são "flutuantes" (UmlEdges.tsx): as alças aqui
// servem apenas para iniciar conexões, não definem o ponto de ancoragem.

import { Handle, NodeResizer, Position, type Node, type NodeProps } from '@xyflow/react'
import { memo, type CSSProperties, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { FIXED_SIZE_TYPES, SEQ } from '../../lib/umlMeta'
import type { LifelineKind, UMLElement, UMLMember } from '../../lib/types'

export interface UmlNodeData extends Record<string, unknown> {
  el: UMLElement
  /** Estado composto / contêiner com filhos. */
  composite?: boolean
  /** Altura total da linha de vida (diagrama de sequência). */
  lifelineHeight?: number
  /** Origem de uma relação em construção (clique-clique pela Toolbox). */
  pendingSource?: boolean
  highlight?: boolean
}

export type UmlFlowNode = Node<UmlNodeData, 'uml'>

const stroke = 'var(--uml-stroke)'
const fill = 'var(--uml-fill)'
const text = 'var(--uml-text)'

function Handles({ lifeline }: { lifeline?: boolean }) {
  // ConnectionMode.Loose: qualquer alça conecta com qualquer outra.
  if (lifeline) {
    return (
      <>
        <Handle type="source" position={Position.Left} id="l" style={{ top: SEQ.header / 2 }} />
        <Handle type="source" position={Position.Right} id="r" style={{ top: SEQ.header / 2 }} />
        <Handle type="source" position={Position.Bottom} id="b" style={{ top: SEQ.header + 6 }} />
      </>
    )
  }
  return (
    <>
      <Handle type="source" position={Position.Top} id="t" />
      <Handle type="source" position={Position.Right} id="r" />
      <Handle type="source" position={Position.Bottom} id="b" />
      <Handle type="source" position={Position.Left} id="l" />
    </>
  )
}

function Resizer({ el, selected }: { el: UMLElement; selected: boolean }) {
  if (FIXED_SIZE_TYPES.has(el.type)) return null
  const bar = el.type === 'fork' || el.type === 'join'
  return (
    <NodeResizer
      isVisible={selected}
      minWidth={bar ? 6 : 60}
      minHeight={bar ? 4 : 30}
      lineStyle={{ borderColor: 'var(--uml-select)' }}
      handleStyle={{ width: 7, height: 7, borderRadius: 1, background: 'var(--canvas-bg)', border: '1px solid var(--uml-select)' }}
    />
  )
}

function Stereotype({ value }: { value?: string }) {
  if (!value) return null
  return <div className="text-[11px] leading-tight" style={{ color: text }}>«{value}»</div>
}

function memberText(m: UMLMember, operation: boolean): string {
  const vis = m.visibility ?? '+'
  if (operation) {
    return `${vis} ${m.name}(${m.params ?? ''})${m.type ? `: ${m.type}` : ''}`
  }
  return `${vis} ${m.name}${m.type ? `: ${m.type}` : ''}${m.default ? ` = ${m.default}` : ''}`
}

/* -------------------------------------------------------------------------- */

function UmlNodeImpl({ data, selected }: NodeProps<UmlFlowNode>) {
  const { el } = data
  const ring = cn(data.pendingSource && 'outline outline-2 outline-offset-2 outline-[var(--uml-select)]', data.highlight && 'arch-pulse')
  return (
    <>
      <Resizer el={el} selected={!!selected} />
      <Shape data={data} className={ring} />
      <Handles lifeline={el.type === 'lifeline'} />
    </>
  )
}

export const UmlNode = memo(UmlNodeImpl)

function Shape({ data, className }: { data: UmlNodeData; className?: string }) {
  const { el } = data
  switch (el.type) {
    case 'actor': return <ActorShape name={el.name} className={className} />
    case 'usecase': return <UseCaseShape el={el} className={className} />
    case 'boundary': return <BoundaryShape el={el} className={className} />
    case 'note': return <NoteShape el={el} className={className} />
    case 'class':
    case 'interface':
    case 'enum': return <ClassShape el={el} className={className} />
    case 'package': return <PackageShape el={el} className={className} />
    case 'lifeline': return <LifelineShape el={el} height={data.lifelineHeight ?? 300} className={className} />
    case 'fragment': return <FragmentShape el={el} className={className} />
    case 'state': return <StateShape el={el} composite={!!data.composite} className={className} />
    case 'initial': return <Round className={className}><svg viewBox="0 0 24 24" className="size-full"><circle cx="12" cy="12" r="10" fill="var(--uml-solid)" /></svg></Round>
    case 'final': return (
      <Round className={className}>
        <svg viewBox="0 0 28 28" className="size-full"><circle cx="14" cy="14" r="12.5" fill={fill} stroke={stroke} strokeWidth="1.3" /><circle cx="14" cy="14" r="8" fill="var(--uml-solid)" /></svg>
      </Round>
    )
    case 'history': return (
      <Round className={className}>
        <svg viewBox="0 0 32 32" className="size-full"><circle cx="16" cy="16" r="14.5" fill={fill} stroke={stroke} strokeWidth="1.3" /><text x="16" y="21" textAnchor="middle" fontSize="14" fontWeight="600" fill={text}>H</text></svg>
      </Round>
    )
    case 'choice': return (
      <div className={cn('size-full', className)}>
        <svg viewBox="0 0 36 36" className="size-full" preserveAspectRatio="none"><path d="M18 1L35 18 18 35 1 18z" fill={fill} stroke={stroke} strokeWidth="1.3" /></svg>
      </div>
    )
    case 'fork':
    case 'join': return <div className={cn('size-full rounded-[1px]', className)} style={{ background: 'var(--uml-solid)' }} />
    default: return <div className={className}>{el.name}</div>
  }
}

function Round({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn('size-full rounded-full', className)}>{children}</div>
}

/* Casos de uso ------------------------------------------------------------- */

function StickFigure({ width = 40, height = 58 }: { width?: number; height?: number }) {
  return (
    <svg width={width} height={height} viewBox="0 0 40 58" aria-hidden>
      <g fill="none" stroke={stroke} strokeWidth="1.4" strokeLinecap="round">
        <circle cx="20" cy="8" r="7" fill={fill} />
        <path d="M20 15v22M5 24h30M20 37L8 56M20 37l12 19" />
      </g>
    </svg>
  )
}

function ActorShape({ name, className }: { name: string; className?: string }) {
  return (
    <div className={cn('flex size-full flex-col items-center', className)}>
      <StickFigure />
      <div className="mt-1 w-[140%] text-center text-[12px] leading-tight" style={{ color: text }}>{name}</div>
    </div>
  )
}

function UseCaseShape({ el, className }: { el: UMLElement; className?: string }) {
  return (
    <div className={cn('relative size-full', className)}>
      <svg className="absolute inset-0 size-full" preserveAspectRatio="none" viewBox="0 0 100 100">
        <ellipse cx="50" cy="50" rx="49.3" ry="49" fill={fill} stroke={stroke} strokeWidth="1.3" vectorEffect="non-scaling-stroke" />
      </svg>
      <div className="relative flex size-full flex-col items-center justify-center px-5 text-center">
        <Stereotype value={el.stereotype} />
        <span className="text-[12.5px] leading-tight" style={{ color: text }}>{el.name}</span>
        {el.use_case && <span className="mt-0.5 font-mono text-[9.5px]" style={{ color: 'var(--uml-muted)' }}>{el.use_case}</span>}
      </div>
    </div>
  )
}

function BoundaryShape({ el, className }: { el: UMLElement; className?: string }) {
  return (
    <div className={cn('size-full', className)} style={{ border: `1.3px solid ${stroke}`, background: 'color-mix(in oklab, var(--uml-fill) 55%, transparent)' }}>
      <div className="px-3 pt-2 text-center text-[12.5px] font-semibold" style={{ color: text }}>
        <Stereotype value={el.stereotype} />
        {el.name}
      </div>
    </div>
  )
}

function NoteShape({ el, className }: { el: UMLElement; className?: string }) {
  // Retângulo com canto dobrado: fundo recortado + bordas desenhadas à parte,
  // para a dobra manter 12px em qualquer tamanho.
  const fold = 12
  const line = `1.3px solid ${stroke}`
  return (
    <div className={cn('relative size-full', className)}>
      <div className="absolute inset-0" style={{
        background: 'var(--uml-note)',
        clipPath: `polygon(0 0, calc(100% - ${fold}px) 0, 100% ${fold}px, 100% 100%, 0 100%)`,
        borderLeft: line, borderBottom: line,
      }} />
      <div className="absolute left-0 top-0" style={{ right: fold, borderTop: line }} />
      <div className="absolute bottom-0 right-0" style={{ top: fold, borderRight: line }} />
      <svg className="absolute right-0 top-0" width={fold} height={fold} viewBox={`0 0 ${fold} ${fold}`}>
        <path d={`M0 0 V${fold} H${fold} Z`} fill="var(--uml-fill-alt)" stroke={stroke} strokeWidth="1.3" />
      </svg>
      <div className="relative whitespace-pre-wrap px-2.5 py-2 pr-4 text-[12px] leading-snug" style={{ color: text }}>
        {el.name && <div className="font-semibold">{el.name}</div>}
        {el.documentation}
      </div>
    </div>
  )
}

/* Classes -------------------------------------------------------------------- */

function ClassShape({ el, className }: { el: UMLElement; className?: string }) {
  const stereotype = el.type === 'interface' ? 'interface' : el.type === 'enum' ? 'enumeration' : el.stereotype
  const compartment: CSSProperties = { borderTop: `1.3px solid ${stroke}` }
  const attrs = el.attributes ?? []
  const ops = el.operations ?? []
  return (
    <div className={cn('flex min-h-full w-full flex-col text-[12px]', className)}
      style={{ border: `1.3px solid ${stroke}`, background: fill, color: text }}>
      <div className="px-2 py-1.5 text-center">
        <Stereotype value={stereotype} />
        <div className={cn('font-semibold leading-tight', el.abstract && 'italic')}>{el.name}</div>
      </div>
      {el.type === 'enum' ? (
        <div className="min-h-3 px-2 py-1 font-mono text-[11px] leading-[1.5]" style={compartment}>
          {(el.literals ?? []).map((lit) => <div key={lit}>{lit}</div>)}
        </div>
      ) : (
        <div className="min-h-3 px-2 py-1 font-mono text-[11px] leading-[1.5]" style={compartment}>
          {attrs.map((m, i) => (
            <div key={i} className={cn('truncate', m.static && 'underline')}>{memberText(m, false)}</div>
          ))}
        </div>
      )}
      <div className="min-h-3 flex-1 px-2 py-1 font-mono text-[11px] leading-[1.5]" style={compartment}>
        {ops.map((m, i) => (
          <div key={i} className={cn('truncate', m.static && 'underline', (m.abstract || el.type === 'interface') && 'italic')}>
            {memberText(m, true)}
          </div>
        ))}
      </div>
    </div>
  )
}

function PackageShape({ el, className }: { el: UMLElement; className?: string }) {
  return (
    <div className={cn('flex size-full flex-col', className)}>
      <div className="w-fit max-w-[70%] truncate px-2.5 py-0.5 text-[11.5px] font-semibold"
        style={{ border: `1.3px solid ${stroke}`, borderBottom: 'none', background: fill, color: text }}>
        {el.name}
      </div>
      <div className="flex-1" style={{ border: `1.3px solid ${stroke}`, background: 'color-mix(in oklab, var(--uml-fill) 55%, transparent)' }}>
        <div className="px-2 pt-1"><Stereotype value={el.stereotype} /></div>
      </div>
    </div>
  )
}

/* Sequência ------------------------------------------------------------------ */

function LifelineHead({ kind }: { kind: LifelineKind }) {
  const common = { fill, stroke, strokeWidth: 1.3 }
  switch (kind) {
    case 'boundary':
      return <svg width="40" height="26" viewBox="0 0 40 26"><path d="M4 3v20M4 13h8" stroke={stroke} strokeWidth="1.3" /><circle cx="24" cy="13" r="11" {...common} /></svg>
    case 'control':
      return <svg width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="15" r="11.5" {...common} /><path d="M11 1.5l4.5 2.5L11 6.5" fill="none" stroke={stroke} strokeWidth="1.3" /></svg>
    case 'entity':
      return <svg width="28" height="28" viewBox="0 0 28 28"><circle cx="14" cy="12.5" r="11" {...common} /><path d="M3 26.5h22" stroke={stroke} strokeWidth="1.3" /></svg>
    case 'database':
      return (
        <svg width="30" height="30" viewBox="0 0 30 30">
          <path d="M2 6v18c0 2.5 5.8 4.5 13 4.5s13-2 13-4.5V6" {...common} />
          <ellipse cx="15" cy="6" rx="13" ry="4.5" {...common} />
        </svg>
      )
    default:
      return null
  }
}

function LifelineShape({ el, height, className }: { el: UMLElement; height: number; className?: string }) {
  const kind = el.lifeline_kind ?? 'participant'
  const iconic = kind !== 'participant'
  return (
    <div className={cn('relative flex w-full flex-col items-center', className)} style={{ height }}>
      {kind === 'actor' ? (
        <div className="flex flex-col items-center" style={{ height: SEQ.header + 16 }}>
          <StickFigure width={24} height={36} />
          <span className="whitespace-nowrap text-[12px]" style={{ color: text }}>{el.name}</span>
        </div>
      ) : iconic ? (
        <div className="flex flex-col items-center" style={{ height: SEQ.header + 16 }}>
          <LifelineHead kind={kind} />
          <span className="whitespace-nowrap text-[12px]" style={{ color: text }}>{el.name}</span>
        </div>
      ) : (
        <div className="flex w-full items-center justify-center px-2 text-center text-[12.5px]"
          style={{ height: SEQ.header, border: `1.3px solid ${stroke}`, background: fill, color: text }}>
          <span className="truncate">{el.name}</span>
        </div>
      )}
      <div className="flex-1" style={{ borderLeft: `1.3px dashed ${stroke}` }} />
    </div>
  )
}

function FragmentShape({ el, className }: { el: UMLElement; className?: string }) {
  const op = el.operator ?? 'alt'
  return (
    <div className={cn('relative size-full', className)} style={{ border: `1.3px solid ${stroke}`, background: 'color-mix(in oklab, var(--uml-fill) 25%, transparent)' }}>
      <div className="absolute left-0 top-0 flex items-center gap-2">
        <span className="px-2 py-0.5 text-[11.5px] font-semibold"
          style={{ background: fill, color: text, borderRight: `1.3px solid ${stroke}`, borderBottom: `1.3px solid ${stroke}`, clipPath: 'polygon(0 0, 100% 0, 100% 65%, calc(100% - 7px) 100%, 0 100%)' }}>
          {op}
        </span>
        {el.guard && <span className="text-[11.5px]" style={{ color: text }}>[{el.guard}]</span>}
      </div>
      {el.name && <div className="absolute bottom-1 right-2 text-[10.5px]" style={{ color: 'var(--uml-muted)' }}>{el.name}</div>}
    </div>
  )
}

/* Estados -------------------------------------------------------------------- */

function StateShape({ el, composite, className }: { el: UMLElement; composite: boolean; className?: string }) {
  const activities = [
    el.entry && `entry / ${el.entry}`,
    el.do && `do / ${el.do}`,
    el.exit && `exit / ${el.exit}`,
  ].filter(Boolean) as string[]
  return (
    <div className={cn('flex size-full flex-col overflow-hidden text-[12px]', className)}
      style={{
        border: `1.3px solid ${stroke}`, borderRadius: 12, color: text,
        background: composite ? 'color-mix(in oklab, var(--uml-fill) 55%, transparent)' : fill,
      }}>
      <div className={cn('px-2 py-1.5 text-center', !composite && activities.length === 0 && 'flex flex-1 flex-col items-center justify-center')}>
        <Stereotype value={el.stereotype} />
        <div className="font-semibold leading-tight">{el.name}</div>
      </div>
      {activities.length > 0 && (
        <div className="px-2.5 py-1 text-[11px] leading-[1.5]" style={{ borderTop: `1.3px solid ${stroke}` }}>
          {activities.map((a) => <div key={a} className="truncate">{a}</div>)}
        </div>
      )}
      {composite && <div className="flex-1" style={{ borderTop: `1.3px solid ${stroke}` }} />}
    </div>
  )
}
