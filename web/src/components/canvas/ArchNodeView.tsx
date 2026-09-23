// Nó visual do canvas. Um único componente parametrizado pelo tipo, para que
// adicionar um novo tipo de componente exija apenas uma entrada em NODE_META.

import { Handle, Position, type NodeProps, type Node } from '@xyflow/react'
import { memo } from 'react'
import { nodeMeta, STATUS_META } from '../../lib/nodeMeta'
import type { NodeData } from '../../lib/types'
import { cx } from '../ui'

export type ArchFlowNodeData = NodeData & {
  __type?: string
  __highlight?: boolean
  __dimmed?: boolean
  __executive?: boolean
}

export type ArchFlowNode = Node<ArchFlowNodeData, 'arch'>

function ArchNodeViewImpl({ data, selected, type }: NodeProps<ArchFlowNode>) {
  const meta = nodeMeta(String(data.__type ?? type ?? 'compute'))
  const Icon = meta.icon
  const status = data.status && data.status !== 'pending' ? STATUS_META[data.status] : null
  const executive = data.__executive === true
  const hours = data.pricing?.estimated_hours

  return (
    <div
      className={cx(
        'group relative w-[220px] rounded-xl border-2 px-3.5 py-3 transition-all',
        'surface shadow-sm',
        selected ? 'ring-2 ring-sky-400/70 ring-offset-2 ring-offset-transparent' : 'hover:shadow-lg',
        data.__highlight && 'arch-pulse',
        data.__dimmed && 'pitch-dim',
      )}
      style={{ borderColor: meta.color, minHeight: 110 }}
    >
      <Handle type="target" position={Position.Left} />
      <Handle type="source" position={Position.Right} />
      <Handle type="target" position={Position.Top} id="t" />
      <Handle type="source" position={Position.Bottom} id="b" />

      <div className="absolute left-0 top-3.5 h-[calc(100%-28px)] w-1 rounded-r" style={{ backgroundColor: meta.color }} />

      <header className="flex items-center gap-1.5">
        <Icon size={13} style={{ color: meta.color }} />
        <span className="text-[9.5px] font-bold uppercase tracking-[0.09em] text-muted-app">
          {meta.label}
        </span>
        {status && (
          <span className="ml-auto h-2 w-2 shrink-0 rounded-full" style={{ backgroundColor: status.dot }}
            title={status.label} />
        )}
      </header>

      <p className="mt-1.5 text-[14px] font-bold leading-tight text-app">{data.label}</p>

      {executive
        ? data.description && <p className="mt-1 line-clamp-2 text-[11px] leading-snug text-muted-app">{data.description}</p>
        : (
          <>
            {data.technology && (
              <p className="mt-1 truncate font-mono text-[10.5px] text-muted-app">{data.technology}</p>
            )}
            <div className="mt-1.5 flex flex-wrap items-center gap-1">
              {typeof hours === 'number' && hours > 0 && (
                <span className="rounded surface-3 px-1.5 py-0.5 text-[9.5px] font-semibold tabular-nums text-muted-app">
                  {hours}h
                </span>
              )}
              {data.pricing?.cloud_tier && (
                <span className="rounded surface-3 px-1.5 py-0.5 font-mono text-[9.5px] text-muted-app">
                  {data.pricing.cloud_tier}
                </span>
              )}
              {data.tags?.slice(0, 2).map((tag) => (
                <span key={tag} className="rounded px-1.5 py-0.5 text-[9.5px] font-semibold"
                  style={{ backgroundColor: `${meta.color}1c`, color: meta.color }}>
                  {tag}
                </span>
              ))}
            </div>
          </>
        )}
    </div>
  )
}

export const ArchNodeView = memo(ArchNodeViewImpl)

/** Contêiner de agrupamento (VPC, cluster, contexto delimitado). */
function GroupNodeViewImpl({ data, selected }: NodeProps<ArchFlowNode>) {
  return (
    <div
      className={cx(
        'rounded-2xl border-2 border-dashed transition-colors',
        selected ? 'border-sky-400' : 'border-app',
        data.__dimmed && 'pitch-dim',
      )}
      style={{
        width: '100%', height: '100%',
        background: 'color-mix(in oklab, var(--surface-3) 55%, transparent)',
      }}
    >
      <span className="absolute left-4 top-2.5 text-[10.5px] font-bold uppercase tracking-[0.12em] text-muted-app">
        {data.label}
      </span>
    </div>
  )
}

export const GroupNodeView = memo(GroupNodeViewImpl)
