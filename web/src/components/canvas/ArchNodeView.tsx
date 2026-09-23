// Nó visual do canvas. Um único componente parametrizado pelo tipo, para que
// adicionar um novo tipo de componente exija apenas uma entrada em NODE_META.

import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { memo } from 'react'
import { nodeMeta, STATUS_META } from '../../lib/nodeMeta'
import type { NodeData } from '../../lib/types'
import { cn } from '@/lib/utils'

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

function ArchNodeViewImpl({ data, selected, type }: NodeProps<ArchFlowNode>) {
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
