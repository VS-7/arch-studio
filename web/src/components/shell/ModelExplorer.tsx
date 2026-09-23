// Model Explorer (painel direito superior do StarUML): árvore do projeto com a
// arquitetura, os diagramas UML e seus elementos, e os artefatos de apoio.

import {
  BookOpen, Box, ChevronRight, Code2, Copy, FileCode2, FolderClosed, FolderOpen, ListChecks,
  Network, Pencil, Plus, Receipt, Search, Sparkles, Trash2, Waypoints, Workflow,
} from 'lucide-react'
import { useMemo, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { nodeMeta } from '../../lib/nodeMeta'
import { tabKey, type TabRef, type ViewId } from '../../lib/tabs'
import type { Selection } from '../../lib/tools'
import type { Snapshot, UMLDiagram, UMLKind } from '../../lib/types'
import { ELEMENT_LABEL, KIND_META, UML_KINDS } from '../../lib/umlMeta'
import {
  ContextMenu, ContextMenuContent, ContextMenuItem, ContextMenuSeparator, ContextMenuTrigger,
} from '../ui/context-menu'
import { IconAction } from '../ui'
import { ElementGlyph, KindGlyph } from '../uml/UmlGlyph'

interface Props {
  snapshot: Snapshot
  activeTab: TabRef | null
  selection: Selection
  onOpen: (tab: TabRef) => void
  onSelectElement: (diagramId: string, elementId: string) => void
  onSelectArchNode: (nodeId: string) => void
  onCreateDiagram: (kind: UMLKind) => void
  onRenameDiagram: (d: UMLDiagram) => void
  onDeleteDiagram: (d: UMLDiagram) => void
  onShowMermaid: (d: UMLDiagram) => void
  onGenerateUseCases: () => void
}

export function ModelExplorer(props: Props) {
  const { snapshot, activeTab, selection, onOpen } = props
  const [query, setQuery] = useState('')
  const [open, setOpen] = useState<Set<string>>(() => new Set(['root', 'arch', ...UML_KINDS.map((k) => `kind:${k}`), 'support']))
  const toggle = (key: string) => setOpen((prev) => {
    const next = new Set(prev)
    if (next.has(key)) next.delete(key); else next.add(key)
    return next
  })

  const q = query.trim().toLowerCase()
  const matches = (text: string) => !q || text.toLowerCase().includes(q)
  const activeKey = activeTab ? tabKey(activeTab) : ''

  const byKind = useMemo(() => {
    const map = new Map<UMLKind, UMLDiagram[]>(UML_KINDS.map((k) => [k, []]))
    for (const d of snapshot.uml_diagrams ?? []) map.get(d.kind)?.push(d)
    return map
  }, [snapshot.uml_diagrams])

  const archNodes = snapshot.diagram.nodes.filter((n) => matches(n.data.label))

  return (
    <div className="flex h-full flex-col">
      <div className="border-b p-1.5">
        <div className="relative">
          <Search size={12} className="absolute left-2 top-1/2 -translate-y-1/2 text-muted-foreground" />
          <input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Filtrar modelo…"
            className="h-6 w-full rounded-sm border border-input bg-background pl-6 pr-2 text-[12px] outline-none focus:border-ring" />
        </div>
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto py-1 text-[12.5px]">
        <Row depth={0} icon={<Box size={14} className="text-foreground" />} label={snapshot.manifest.project_name} bold
          expandable open={open.has('root')} onToggle={() => toggle('root')} />
        {open.has('root') && (
          <>
            {/* Arquitetura macro */}
            <Row depth={1} icon={<Waypoints size={14} className="text-muted-foreground" />} label="Arquitetura (macro)"
              count={snapshot.diagram.nodes.length}
              expandable open={open.has('arch') || !!q} onToggle={() => toggle('arch')}
              active={activeKey === 'arch'} onClick={() => onOpen({ type: 'arch' })} />
            {(open.has('arch') || q) && archNodes.map((n) => {
              const meta = nodeMeta(n.type)
              const Icon = meta.icon
              return (
                <Row key={n.id} depth={2} icon={<Icon size={13} className="text-muted-foreground" />} label={n.data.label}
                  active={selection?.scope === 'arch' && selection.id === n.id}
                  onClick={() => props.onSelectArchNode(n.id)} />
              )
            })}

            {/* Diagramas UML por tipo */}
            {UML_KINDS.map((kind) => {
              const diagrams = byKind.get(kind) ?? []
              const key = `kind:${kind}`
              const isOpen = open.has(key) || !!q
              return (
                <div key={kind}>
                  <ContextMenu>
                    <ContextMenuTrigger asChild>
                      <div>
                        <Row depth={1}
                          icon={isOpen ? <FolderOpen size={14} className="text-warning" /> : <FolderClosed size={14} className="text-warning" />}
                          label={KIND_META[kind].plural} count={diagrams.length}
                          expandable open={isOpen} onToggle={() => toggle(key)} onClick={() => toggle(key)}
                          action={{ icon: <Plus size={13} />, title: `Novo ${KIND_META[kind].label.toLowerCase()}`, onClick: () => props.onCreateDiagram(kind) }} />
                      </div>
                    </ContextMenuTrigger>
                    <ContextMenuContent>
                      <ContextMenuItem onSelect={() => props.onCreateDiagram(kind)}><Plus />Novo {KIND_META[kind].label.toLowerCase()}</ContextMenuItem>
                      {kind === 'usecase' && (
                        <ContextMenuItem onSelect={props.onGenerateUseCases}><Sparkles />Gerar a partir das fichas de caso de uso</ContextMenuItem>
                      )}
                    </ContextMenuContent>
                  </ContextMenu>
                  {isOpen && diagrams.length === 0 && !q && (
                    <button onClick={() => props.onCreateDiagram(kind)}
                      className="block py-0.5 text-left text-[11.5px] text-muted-foreground hover:text-foreground hover:underline" style={{ paddingLeft: 3 * 14 + 6 }}>
                      + criar diagrama
                    </button>
                  )}
                  {isOpen && diagrams.map((d) => (
                    <DiagramNode key={d.id} diagram={d} {...props} query={q}
                      open={open.has(`d:${d.id}`) || (!!q && d.elements.some((e) => matches(e.name)))}
                      onToggle={() => toggle(`d:${d.id}`)} active={activeKey === `uml:${d.id}`} />
                  ))}
                </div>
              )
            })}

            {/* Artefatos de apoio */}
            <Row depth={1} icon={<FolderOpen size={14} className="text-muted-foreground" />} label="Documentação & Gestão"
              expandable open={open.has('support')} onToggle={() => toggle('support')} onClick={() => toggle('support')} />
            {open.has('support') && (
              <>
                <SupportRow view="docs" icon={<BookOpen size={13} />} label="Requisitos" count={snapshot.requirements.requirements.length} {...props} activeKey={activeKey} />
                <SupportRow view="docs" icon={<ListChecks size={13} />} label="Fichas de caso de uso" count={snapshot.use_cases.length} {...props} activeKey={activeKey} />
                <SupportRow view="docs" icon={<FileCode2 size={13} />} label="Decisões (ADR)" count={snapshot.adrs.length} {...props} activeKey={activeKey} />
                <SupportRow view="api" icon={<Network size={13} />} label="Contratos de API" count={snapshot.endpoints.endpoints.length} {...props} activeKey={activeKey} />
                <SupportRow view="pricing" icon={<Receipt size={13} />} label="Precificação" {...props} activeKey={activeKey} />
                <SupportRow view="tasks" icon={<Workflow size={13} />} label="Implementação (AI-PRD)" count={snapshot.tasks.tasks.length || undefined} {...props} activeKey={activeKey} />
              </>
            )}
          </>
        )}
      </div>
    </div>
  )
}

function SupportRow({ view, icon, label, count, onOpen, activeKey }: {
  view: ViewId; icon: ReactNode; label: string; count?: number; onOpen: (tab: TabRef) => void; activeKey: string
}) {
  return (
    <Row depth={2} icon={<span className="text-muted-foreground">{icon}</span>} label={label} count={count}
      active={activeKey === `view:${view}`} onClick={() => onOpen({ type: 'view', view })} />
  )
}

function DiagramNode({ diagram, query, open, onToggle, active, selection, onOpen, onSelectElement, onRenameDiagram, onDeleteDiagram, onShowMermaid }: Props & {
  diagram: UMLDiagram; query: string; open: boolean; onToggle: () => void; active: boolean
}) {
  const elements = diagram.elements
    .filter((e) => e.name || e.type === 'initial' || e.type === 'final')
    .filter((e) => !query || e.name.toLowerCase().includes(query))
  return (
    <>
      <ContextMenu>
        <ContextMenuTrigger asChild>
          <div>
            <Row depth={2} icon={<span className="text-foreground"><KindGlyph kind={diagram.kind} size={14} /></span>}
              label={diagram.name} count={diagram.elements.length}
              expandable open={open} onToggle={onToggle} active={active}
              onClick={() => onOpen({ type: 'uml', id: diagram.id })} />
          </div>
        </ContextMenuTrigger>
        <ContextMenuContent>
          <ContextMenuItem onSelect={() => onOpen({ type: 'uml', id: diagram.id })}><Pencil />Abrir</ContextMenuItem>
          <ContextMenuItem onSelect={() => onRenameDiagram(diagram)}><Pencil />Renomear…</ContextMenuItem>
          <ContextMenuItem onSelect={() => onShowMermaid(diagram)}><Code2 />Ver Mermaid</ContextMenuItem>
          <ContextMenuItem onSelect={() => void navigator.clipboard.writeText(diagram.file ?? diagram.id)}><Copy />Copiar caminho</ContextMenuItem>
          <ContextMenuSeparator />
          <ContextMenuItem variant="destructive" onSelect={() => onDeleteDiagram(diagram)}><Trash2 />Excluir diagrama</ContextMenuItem>
        </ContextMenuContent>
      </ContextMenu>
      {open && elements.map((el) => (
        <Row key={el.id} depth={3} icon={<span className="text-muted-foreground"><ElementGlyph type={el.type} size={13} /></span>}
          label={el.name || ELEMENT_LABEL[el.type]} muted={!el.name}
          active={selection?.scope === 'uml' && selection.diagramId === diagram.id && selection.id === el.id}
          onClick={() => onSelectElement(diagram.id, el.id)} />
      ))}
    </>
  )
}

function Row({ depth, icon, label, count, bold, muted, expandable, open, onToggle, onClick, active, action }: {
  depth: number; icon: ReactNode; label: string; count?: number; bold?: boolean; muted?: boolean
  expandable?: boolean; open?: boolean; onToggle?: () => void; onClick?: () => void; active?: boolean
  action?: { icon: ReactNode; title: string; onClick: () => void }
}) {
  return (
    <div
      className={cn(
        'group flex h-[22px] cursor-default items-center gap-1 pr-1.5',
        active ? 'bg-accent text-accent-foreground' : 'hover:bg-muted',
      )}
      style={{ paddingLeft: depth * 14 + 4 }}
      onClick={onClick}
      onDoubleClick={onToggle}
    >
      <button
        className={cn('flex size-4 shrink-0 items-center justify-center text-muted-foreground', !expandable && 'invisible')}
        onClick={(e) => { e.stopPropagation(); onToggle?.() }}
        tabIndex={-1}
      >
        <ChevronRight size={12} className={cn('transition-transform', open && 'rotate-90')} />
      </button>
      <span className="flex w-4 shrink-0 justify-center">{icon}</span>
      <span className={cn('min-w-0 flex-1 truncate', bold && 'font-semibold', muted && 'italic text-muted-foreground')}>{label}</span>
      {count !== undefined && <span className="text-[10.5px] tabular-nums text-muted-foreground">{count}</span>}
      {action && (
        <IconAction label={action.title} className="size-4 p-0 opacity-0 hover:bg-background focus-visible:opacity-100 group-hover:opacity-100"
          onClick={(e) => { e.stopPropagation(); action.onClick() }}>
          {action.icon}
        </IconAction>
      )}
    </div>
  )
}
