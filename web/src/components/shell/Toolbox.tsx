// Toolbox contextual (painel esquerdo do StarUML): mostra as formas e relações
// do tipo de diagrama aberto. Clique escolhe a ferramenta e o próximo clique no
// canvas cria o elemento; também é possível arrastar a forma para o canvas.

import { ChevronDown, MousePointer2 } from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { NODE_META } from '../../lib/nodeMeta'
import type { Tool } from '../../lib/tools'
import type { NodeType, UMLKind } from '../../lib/types'
import { ELEMENT_LABEL, KIND_META, RELATION_LABEL } from '../../lib/umlMeta'
import { Tip } from '../ui'
import { ElementGlyph, RelationGlyph } from '../uml/UmlGlyph'

export type ToolboxContext = { type: 'uml'; kind: UMLKind } | { type: 'arch' } | null

const ARCH_GROUPS: { title: string; types: NodeType[] }[] = [
  { title: 'Computação', types: ['compute', 'queue'] },
  { title: 'Dados', types: ['database', 'cache', 'storage'] },
  { title: 'Borda & Integração', types: ['gateway', 'external_service'] },
  { title: 'Clientes', types: ['client'] },
  { title: 'Organização', types: ['group'] },
]

export function Toolbox({ context, tool, onTool }: {
  context: ToolboxContext; tool: Tool; onTool: (tool: Tool) => void
}) {
  if (!context) {
    return (
      <div className="px-3 py-4 text-[12px] leading-relaxed text-muted-foreground">
        Abra um diagrama para ver as ferramentas de modelagem.
      </div>
    )
  }

  const selectRow = (
    <ToolRow active={tool.mode === 'select'} onClick={() => onTool({ mode: 'select' })}
      icon={<MousePointer2 size={14} />} label="Selecionar" hint="Esc" />
  )

  if (context.type === 'arch') {
    return (
      <div className="py-1">
        <div className="px-1.5 pb-1">{selectRow}</div>
        {ARCH_GROUPS.map((group) => (
          <Group key={group.title} title={group.title}>
            {group.types.map((type) => {
              const meta = NODE_META[type]
              const Icon = meta.icon
              return (
                <ToolRow key={type} label={meta.label} title={meta.hint}
                  icon={<Icon size={14} />}
                  active={tool.mode === 'arch' && tool.type === type}
                  onClick={() => onTool(tool.mode === 'arch' && tool.type === type ? { mode: 'select' } : { mode: 'arch', type })}
                  drag={{ mime: 'application/archcode-node', payload: { type } }} />
              )
            })}
          </Group>
        ))}
      </div>
    )
  }

  const meta = KIND_META[context.kind]
  return (
    <div className="py-1">
      <div className="px-1.5 pb-1">{selectRow}</div>
      {meta.elements.map((group) => (
        <Group key={group.title} title={group.title}>
          {group.types.map((type) => (
            <ToolRow key={type} label={ELEMENT_LABEL[type]} icon={<ElementGlyph type={type} />}
              active={tool.mode === 'element' && tool.type === type}
              onClick={() => onTool(tool.mode === 'element' && tool.type === type ? { mode: 'select' } : { mode: 'element', type })}
              drag={{ mime: 'application/archcode-uml', payload: { type } }} />
          ))}
        </Group>
      ))}
      <Group title="Relações">
        {meta.relations.map((type) => (
          <ToolRow key={type} label={RELATION_LABEL[type]} icon={<RelationGlyph type={type} />}
            active={tool.mode === 'relation' && tool.type === type}
            onClick={() => onTool(tool.mode === 'relation' && tool.type === type ? { mode: 'select' } : { mode: 'relation', type })} />
        ))}
      </Group>
      <p className="px-3 pt-2 text-[11px] leading-relaxed text-muted-foreground">
        Relações: escolha a ferramenta e clique na origem e no destino, ou arraste entre as alças dos elementos.
      </p>
    </div>
  )
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  const [open, setOpen] = useState(true)
  return (
    <div className="mb-0.5">
      <button onClick={() => setOpen(!open)}
        className="flex h-6 w-full items-center gap-1 px-2 text-[11px] font-semibold text-muted-foreground hover:text-foreground">
        <ChevronDown size={12} className={cn('transition-transform', !open && '-rotate-90')} />
        {title}
      </button>
      {open && <div className="px-1.5 pb-1">{children}</div>}
    </div>
  )
}

function ToolRow({ label, icon, active, onClick, drag, hint, title }: {
  label: string; icon: ReactNode; active: boolean; onClick: () => void
  drag?: { mime: string; payload: unknown }; hint?: string; title?: string
}) {
  const row = (
    <button
      draggable={!!drag}
      onDragStart={drag ? (e) => {
        e.dataTransfer.setData(drag.mime, JSON.stringify(drag.payload))
        e.dataTransfer.effectAllowed = 'copy'
      } : undefined}
      onClick={onClick}
      aria-pressed={active}
      className={cn(
        'flex h-7 w-full items-center gap-2 rounded-sm border px-2 text-left text-[12.5px] transition-colors',
        active ? 'border-border bg-accent font-medium text-accent-foreground' : 'border-transparent text-foreground hover:bg-accent/70',
        drag && 'cursor-grab active:cursor-grabbing',
      )}
    >
      <span className={cn('flex w-4 justify-center', active ? 'text-foreground' : 'text-muted-foreground')}>{icon}</span>
      <span className="min-w-0 flex-1 truncate">{label}</span>
      {hint && <kbd className="font-mono text-[10px] opacity-60">{hint}</kbd>}
    </button>
  )
  // Descrições mais longas que o rótulo (ex.: "API Gateway, load balancer…") viram tooltip.
  return title && title !== label ? <Tip label={title} side="right">{row}</Tip> : row
}
