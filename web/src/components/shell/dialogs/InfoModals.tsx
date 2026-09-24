// Diálogos informativos do menu Ajuda.

import type { Snapshot } from '../../../lib/types'
import { Modal } from '../../ui'
import { SHORTCUT_ROWS } from '../useShortcuts'

export function ShortcutsModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Modal open={open} onClose={onClose} title="Atalhos de teclado">
      <table className="w-full text-[12.5px]">
        <tbody>
          {SHORTCUT_ROWS.map(([k, v]) => (
            <tr key={k} className="border-b last:border-0">
              <td className="py-1.5 pr-4"><kbd className="rounded border bg-muted px-1.5 py-0.5 font-mono text-[11px]">{k}</kbd></td>
              <td className="py-1.5 text-muted-foreground">{v}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Modal>
  )
}

export function AboutModal({ open, onClose, snapshot }: { open: boolean; onClose: () => void; snapshot: Snapshot }) {
  return (
    <Modal open={open} onClose={onClose} title="ArchCode Studio"
      description="Arquitetura-como-código, local-first, orientada a agentes de IA (MCP).">
      <div className="space-y-2 text-[12.5px] text-muted-foreground">
        <p>Projeto: <span className="text-foreground">{snapshot.manifest.project_name}</span> · schema {snapshot.manifest.schema_version}</p>
        <p>
          Diagramas: arquitetura macro + {snapshot.uml_diagrams?.length ?? 0} UML (casos de uso, classes, sequência e estados).
          Tudo é gravado em arquivos texto na pasta do projeto — versionável em Git.
        </p>
      </div>
    </Modal>
  )
}
