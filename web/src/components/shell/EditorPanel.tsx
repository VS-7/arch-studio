// Painel Editor: propriedades do que está selecionado no diagrama aberto.

import { ClipboardPaste, Copy, CopyPlus, Trash2 } from 'lucide-react'
import type { TabRef } from '../../lib/tabs'
import type { Selection } from '../../lib/tools'
import type { Snapshot, UMLDiagram } from '../../lib/types'
import { ELEMENT_LABEL, KIND_META } from '../../lib/umlMeta'
import { Inspector } from '../canvas/Inspector'
import { Badge, Button } from '../ui'
import { KindGlyph } from '../uml/UmlGlyph'
import { UmlInspector } from '../uml/UmlInspector'
import type { EditCommand } from './useEditCommands'

export function EditorPanel({
  snapshot, active, diagram, selection, focusName, multiCount, onCommand, onClose, onFocusArch, onOpenUseCase, onRename, onMermaid, onExport, archFocus,
}: {
  snapshot: Snapshot; active: TabRef | null; diagram: UMLDiagram | null; selection: Selection; focusName: number
  multiCount: number; onCommand: (name: EditCommand) => void
  onClose: () => void; onFocusArch: (id: string) => void; onOpenUseCase: (code: string) => void
  onRename: (d: UMLDiagram) => void; onMermaid: (d: UMLDiagram) => void; onExport: (f: 'png' | 'svg') => void
  archFocus: (id: string) => void
}) {
  if (multiCount > 1 && (active?.type === 'arch' || diagram)) {
    return (
      <div className="space-y-3 p-3">
        <p className="text-[13px] font-semibold">{multiCount} itens selecionados</p>
        <p className="text-[12px] leading-relaxed text-muted-foreground">
          Arraste para mover todos juntos. Use o clique direito para mais ações.
        </p>
        <div className="flex flex-wrap gap-1.5">
          <Button size="sm" variant="secondary" icon={Copy} onClick={() => onCommand('copy')}>Copiar</Button>
          <Button size="sm" variant="secondary" icon={CopyPlus} onClick={() => onCommand('duplicate')}>Duplicar</Button>
          <Button size="sm" variant="secondary" icon={ClipboardPaste} onClick={() => onCommand('paste')}>Colar</Button>
          <Button size="sm" variant="danger" icon={Trash2} onClick={() => onCommand('deleteSelected')}>Excluir</Button>
        </div>
      </div>
    )
  }
  if (active?.type === 'arch') {
    return (
      <Inspector snapshot={snapshot}
        selected={selection?.scope === 'arch' ? { id: selection.id, kind: selection.kind } : null}
        onClose={onClose} onFocus={archFocus} />
    )
  }
  if (diagram) {
    if (selection?.scope === 'uml' && selection.diagramId === diagram.id) {
      return (
        <UmlInspector snapshot={snapshot} diagram={diagram} selection={{ kind: selection.kind, id: selection.id }}
          focusName={focusName} onClose={onClose} onOpenUseCase={onOpenUseCase} onFocusArch={onFocusArch} />
      )
    }
    return <DiagramSummary diagram={diagram} onRename={onRename} onMermaid={onMermaid} onExport={onExport} />
  }
  return (
    <p className="px-4 py-6 text-center text-[12px] leading-relaxed text-muted-foreground">
      Selecione um elemento em um diagrama para editar suas propriedades.
    </p>
  )
}

function DiagramSummary({ diagram, onRename, onMermaid, onExport }: {
  diagram: UMLDiagram; onRename: (d: UMLDiagram) => void; onMermaid: (d: UMLDiagram) => void; onExport: (f: 'png' | 'svg') => void
}) {
  const counts = new Map<string, number>()
  for (const el of diagram.elements) counts.set(ELEMENT_LABEL[el.type], (counts.get(ELEMENT_LABEL[el.type]) ?? 0) + 1)
  return (
    <div className="space-y-3 p-3">
      <div className="flex items-start gap-2">
        <span className="mt-0.5"><KindGlyph kind={diagram.kind} size={18} /></span>
        <div className="min-w-0">
          <p className="truncate text-[13px] font-semibold">{diagram.name}</p>
          <p className="text-[11.5px] text-muted-foreground">{KIND_META[diagram.kind].label}</p>
        </div>
      </div>
      {diagram.description && <p className="text-[12px] leading-relaxed text-muted-foreground">{diagram.description}</p>}
      <div className="flex flex-wrap gap-1">
        {[...counts].map(([label, n]) => <Badge key={label}>{n} {label.toLowerCase()}</Badge>)}
        <Badge>{diagram.relations.length} relações</Badge>
      </div>
      <p className="break-all font-mono text-[10.5px] text-muted-foreground">{diagram.file}</p>
      <div className="flex flex-wrap gap-1.5">
        <Button size="sm" variant="secondary" onClick={() => onRename(diagram)}>Propriedades…</Button>
        <Button size="sm" variant="secondary" onClick={() => onMermaid(diagram)}>Mermaid</Button>
        <Button size="sm" variant="secondary" onClick={() => onExport('png')}>PNG</Button>
        <Button size="sm" variant="secondary" onClick={() => onExport('svg')}>SVG</Button>
      </div>
      <p className="border-t pt-3 text-[11.5px] leading-relaxed text-muted-foreground">
        Clique em um elemento para editá-lo. Use a Toolbox para criar formas e relações, ou peça a um agente de IA
        via MCP (<code className="font-mono">add_uml_element</code>, <code className="font-mono">add_uml_relation</code>).
      </p>
    </div>
  )
}
