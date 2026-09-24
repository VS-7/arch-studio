// Abas da área central: diagramas abertos e telas de apoio.

import { Waypoints, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { tabKey, VIEW_LABEL, type TabRef } from '../../lib/tabs'
import type { UMLDiagram } from '../../lib/types'
import { IconAction } from '../ui'
import { KindGlyph } from '../uml/UmlGlyph'
import { VIEW_ICON } from './viewMeta'

export function TabStrip({ tabs, active, diagrams, onActivate, onClose }: {
  tabs: TabRef[]; active: TabRef | null; diagrams: UMLDiagram[]
  onActivate: (t: TabRef) => void; onClose: (key: string) => void
}) {
  const activeKey = active ? tabKey(active) : ''
  return (
    <div className="flex h-8 shrink-0 items-end overflow-x-auto border-b bg-chrome-2">
      {tabs.map((t) => {
        const key = tabKey(t)
        const d = t.type === 'uml' ? diagrams.find((x) => x.id === t.id) : undefined
        const label = t.type === 'arch' ? 'Arquitetura' : t.type === 'uml' ? d?.name ?? t.id : VIEW_LABEL[t.view]
        const isActive = key === activeKey
        const ViewIcon = t.type === 'view' ? VIEW_ICON[t.view] : null
        return (
          <div key={key}
            onMouseDown={(e) => { if (e.button === 1) { e.preventDefault(); onClose(key) } }}
            onClick={() => onActivate(t)}
            className={cn(
              'group relative flex h-8 max-w-[220px] shrink-0 cursor-default items-center gap-1.5 border-r px-3 text-[12.5px]',
              isActive ? 'bg-tab-active text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
            )}
          >
            {isActive && <span className="absolute inset-x-0 top-0 h-0.5 bg-foreground/70" />}
            <span>
              {t.type === 'uml' && d ? <KindGlyph kind={d.kind} size={13} />
                : t.type === 'arch' ? <Waypoints size={13} />
                  : ViewIcon ? <ViewIcon size={13} /> : null}
            </span>
            <span className="truncate">{label}</span>
            <IconAction label="Fechar aba (clique do meio)"
              onClick={(e) => { e.stopPropagation(); onClose(key) }}
              className={cn('ml-0.5', isActive ? 'opacity-80' : 'opacity-0 group-hover:opacity-80')}>
              <X size={12} />
            </IconAction>
          </div>
        )
      })}
    </div>
  )
}
