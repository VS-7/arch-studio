import { CheckCircle2 } from 'lucide-react'
import { useProject } from '../../../lib/project'
import { Badge, Modal } from '../../ui'

export function LintModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { lint } = useProject()
  if (!lint) return null
  const color = { error: 'var(--destructive)', warning: 'var(--warning)', info: 'var(--muted-foreground)' }

  return (
    <Modal open={open} onClose={onClose} wide title={`Validação da arquitetura — ${lint.score}/100`}
      description={`${lint.errors} erro(s), ${lint.warnings} aviso(s), ${lint.infos} informativo(s). As mesmas regras rodam via MCP em validate_architecture_rules.`}>
      {lint.findings.length === 0 ? (
        <div className="flex flex-col items-center gap-2 py-8">
          <CheckCircle2 size={28} className="text-success" />
          <p className="text-sm font-medium">Nenhum problema encontrado.</p>
        </div>
      ) : (
        <ul className="max-h-[60vh] space-y-2 overflow-y-auto">
          {lint.findings.map((f, i) => (
            <li key={i} className="rounded-md border px-3 py-2.5">
              <div className="flex flex-wrap items-center gap-2">
                <Badge color={color[f.severity]}>{f.severity}</Badge>
                <code className="font-mono text-[10.5px] text-muted-foreground">{f.rule}</code>
                {f.target && <span className="text-[12.5px] font-semibold">{f.target}</span>}
              </div>
              <p className="mt-1 text-[12.5px] text-muted-foreground">{f.message}</p>
              {f.fix && <p className="mt-1 text-[11.5px] text-primary">→ {f.fix}</p>}
            </li>
          ))}
        </ul>
      )}
    </Modal>
  )
}
