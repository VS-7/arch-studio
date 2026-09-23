// Nó visual do canvas. Um único componente parametrizado pelo tipo, para que
// adicionar um novo tipo de componente exija apenas uma entrada em NODE_META.

import { Handle, NodeToolbar, Position, type NodeProps, type Node } from '@xyflow/react'
import { MoreHorizontal } from 'lucide-react'
import { createContext, memo, useContext } from 'react'
import { nodeMeta, STATUS_META } from '../../lib/nodeMeta'
import type { NodeData, NodeType } from '../../lib/types'
import { cn } from '@/lib/utils'
import { IconAction } from '../ui'

/** Ações da barra rápida, fornecidas pelo ArchCanvas. */
export interface ArchNodeActions {
  single: string | null
  types: NodeType[]
  onQuick: (id: string, type: NodeType) => void
  onMore: (id: string, x: number, y: number) => void
}

export const ArchActionsContext = createContext<ArchNodeActions | null>(null)

/** Barra flutuante do componente selecionado: cria outro já conectado a ele. */
function QuickBar({ id }: { id: string }) {
  const actions = useContext(ArchActionsContext)
  if (!actions || actions.single !== id) return null
  return (
    <NodeToolbar isVisible position={Position.Top} offset={10}
      className="nodrag flex items-center gap-0.5 rounded-md border bg-popover p-0.5 text-popover-foreground shadow-md">
      {actions.types.map((type) => {
        const meta = nodeMeta(type)
        const Icon = meta.icon
        return (
          <IconAction key={type} label={`${meta.label} conectado`} side="top" className="size-7"
            onClick={() => actions.onQuick(id, type)}>
            <Icon size={14} />
          </IconAction>
        )
      })}
      <span className="mx-0.5 h-4 w-px bg-border" />
      <IconAction label="Mais ações…" side="top" className="size-7"
        onClick={(e) => { const r = (e.currentTarget as HTMLElement).getBoundingClientRect(); actions.onMore(id, r.left, r.bottom + 4) }}>
        <MoreHorizontal size={14} />
      </IconAction>
    </NodeToolbar>
  )
}

export type ArchFlowNodeData = NodeData & {
  __type?: string
  __highlight?: boolean
  __dimmed?: boolean
  __executive?: boolean
}

export type ArchFlowNode = Node<ArchFlowNodeData, 'arch'>

/** Estereótipos UML por tipo de componente (notação de componentes). */
const STEREOTYPE: Record<string, string> = {
  compute: 'service', database: 'database', cache: 'cache', queue: 'queue', gateway: 'gateway',
  storage: 'storage', client: 'client', external_service: 'external',
}

function ArchNodeViewImpl({ id, data, selected, type }: NodeProps<ArchFlowNode>) {
  const kind = String(data.__type ?? type ?? 'compute')
  const meta = nodeMeta(kind)
  const Icon = meta.icon
  const status = data.status && data.status !== 'pending' ? STATUS_META[data.status] : null
  const executive = data.__executive === true
  const hours = data.pricing?.estimated_hours

  return (
    <div
      className={cn(
        'group relative w-[220px] px-3 pb-2.5 pt-2 transition-shadow',
        selected && 'uml-selected',
        data.__highlight && 'arch-pulse',
        data.__dimmed && 'pitch-dim',
      )}
      style={{
        minHeight: 96,
        background: 'var(--uml-fill)',
        color: 'var(--uml-text)',
        border: `1.3px ${kind === 'external_service' ? 'dashed' : 'solid'} var(--uml-stroke)`,
        borderRadius: 2,
      }}
    >
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
      <Handle type="target" position={Position.Top} id="t" />
      <Handle type="source" position={Position.Bottom} id="b" />
      <QuickBar id={id} />

      <header className="flex items-start gap-1.5">
        <span className="min-w-0 flex-1 text-center text-[11px] leading-tight" style={{ color: 'var(--uml-muted)' }}>
          «{STEREOTYPE[kind] ?? kind}»
        </span>
        <Icon size={13} className="absolute right-2 top-2" style={{ color: 'var(--uml-muted)' }} />
        {status && (
          <span className="absolute left-2 top-2.5 size-2 rounded-full" style={{ backgroundColor: status.dot }}
            title={status.label} />
        )}
      </header>

      <p className="mt-0.5 text-center text-[13.5px] font-semibold leading-tight">{data.label}</p>

      {executive
        ? data.description && <p className="mt-1 line-clamp-2 text-center text-[11px] leading-snug" style={{ color: 'var(--uml-muted)' }}>{data.description}</p>
        : (
          <>
            {data.technology && (
              <p className="mt-0.5 truncate text-center font-mono text-[10.5px]" style={{ color: 'var(--uml-muted)' }}>{data.technology}</p>
            )}
            {(Boolean(hours) || data.pricing?.cloud_tier || (data.tags?.length ?? 0) > 0) && (
              <div className="mt-2 flex flex-wrap justify-center gap-1 border-t pt-1.5" style={{ borderColor: 'color-mix(in oklab, var(--uml-stroke) 30%, transparent)' }}>
                {typeof hours === 'number' && hours > 0 && (
                  <span className="rounded-sm border px-1 text-[9.5px] font-semibold tabular-nums" style={{ color: 'var(--uml-muted)' }}>
                    {hours}h
                  </span>
                )}
                {data.pricing?.cloud_tier && (
                  <span className="rounded-sm border px-1 font-mono text-[9.5px]" style={{ color: 'var(--uml-muted)' }}>
                    {data.pricing.cloud_tier}
                  </span>
                )}
                {data.tags?.slice(0, 2).map((tag) => (
                  <span key={tag} className="rounded-sm border px-1 text-[9.5px] font-semibold"
                    style={{ color: 'var(--uml-text)' }}>
                    {tag}
                  </span>
                ))}
              </div>
            )}
          </>
        )}
    </div>
  )
}

export const ArchNodeView = memo(ArchNodeViewImpl)

/** Contêiner de agrupamento (VPC, cluster, contexto delimitado) — moldura de pacote UML. */
function GroupNodeViewImpl({ data, selected }: NodeProps<ArchFlowNode>) {
  return (
    <div className={cn('flex size-full flex-col', data.__dimmed && 'pitch-dim')}>
      <span className="w-fit max-w-[70%] truncate px-2.5 py-0.5 text-[11.5px] font-semibold"
        style={{ border: '1.3px solid var(--uml-stroke)', borderBottom: 'none', background: 'var(--uml-fill)', color: 'var(--uml-text)' }}>
        {data.label}
      </span>
      <div className={cn('flex-1', selected && 'uml-selected')}
        style={{ border: '1.3px dashed var(--uml-stroke)', background: 'color-mix(in oklab, var(--uml-fill) 45%, transparent)' }} />
    </div>
  )
}

export const GroupNodeView = memo(GroupNodeViewImpl)
