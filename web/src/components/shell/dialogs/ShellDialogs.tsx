// Diálogos abertos a partir do Shell (menus, Model Explorer, Editor). Um único
// estado diz qual está aberto; todos ficam montados para animar ao fechar.

import type { Snapshot, UMLDiagram, UMLKind } from '../../../lib/types'
import { MermaidPanel } from '../../canvas/MermaidPanel'
import { NewDiagramDialog, RenameDiagramDialog, UmlMermaidDialog } from '../../uml/UmlDialogs'
import { AboutModal, ShortcutsModal } from './InfoModals'
import { LintModal } from './LintModal'
import { McpModal } from './McpModal'

export type ShellDialog =
  | { type: 'lint' }
  | { type: 'mcp' }
  | { type: 'shortcuts' }
  | { type: 'about' }
  | { type: 'archMermaid' }
  | { type: 'newDiagram'; kind: UMLKind }
  | { type: 'renameDiagram'; diagram: UMLDiagram }
  | { type: 'umlMermaid'; diagram: UMLDiagram }

export function ShellDialogs({ dialog, onClose, snapshot, onDiagramCreated }: {
  dialog: ShellDialog | null
  onClose: () => void
  snapshot: Snapshot
  onDiagramCreated: (d: UMLDiagram) => void
}) {
  const is = (type: ShellDialog['type']) => dialog?.type === type
  return (
    <>
      <MermaidPanel open={is('archMermaid')} onClose={onClose} snapshot={snapshot} />
      <LintModal open={is('lint')} onClose={onClose} />
      <McpModal open={is('mcp')} onClose={onClose} />
      <ShortcutsModal open={is('shortcuts')} onClose={onClose} />
      <AboutModal open={is('about')} onClose={onClose} snapshot={snapshot} />
      <NewDiagramDialog open={is('newDiagram')} initialKind={dialog?.type === 'newDiagram' ? dialog.kind : 'usecase'}
        onClose={onClose} onCreated={onDiagramCreated} />
      <RenameDiagramDialog diagram={dialog?.type === 'renameDiagram' ? dialog.diagram : null} onClose={onClose} />
      <UmlMermaidDialog diagram={dialog?.type === 'umlMermaid' ? dialog.diagram : null} onClose={onClose} />
    </>
  )
}
