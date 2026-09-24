// Sincronização bidirecional com Mermaid (RF007): visualizar o espelho gerado e
// importar um snippet colado (C4, flowchart ou graph).

import { ArrowDownToLine, Copy, Import } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { mermaidErrorMessage, renderMermaid } from '../../lib/mermaid'
import { platform } from '../../lib/platform'
import type { Snapshot } from '../../lib/types'
import { Button, Modal, Textarea, useToast } from '../ui'

export function MermaidPanel({ open, onClose, snapshot }: {
  open: boolean; onClose: () => void; snapshot: Snapshot
}) {
  const toast = useToast()
  const [tab, setTab] = useState<'view' | 'import'>('view')
  const [source, setSource] = useState('')
  const [importing, setImporting] = useState(false)
  // Estado (e não ref): o conteúdo do diálogo monta depois do efeito de abertura.
  const [preview, setPreview] = useState<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!open || tab !== 'view' || !preview) return
    let cancelled = false
    void (async () => {
      try {
        const svg = await renderMermaid(snapshot.mermaid || 'flowchart LR\n  vazio["Sem componentes"]')
        if (!cancelled) preview.innerHTML = svg
      } catch (err) {
        if (!cancelled) preview.textContent = `Mermaid inválido: ${mermaidErrorMessage(err)}`
      }
    })()
    return () => { cancelled = true }
  }, [open, tab, snapshot.mermaid, preview])

  const doImport = async () => {
    if (!source.trim()) return
    setImporting(true)
    try {
      const diagram = await api.importMermaid(source)
      toast('success', `Importado: ${diagram.nodes.length} componentes, ${diagram.edges.length} conexões · Ctrl+Z desfaz`)
      onClose()
      setSource('')
    } catch (err) { toast('error', errorMessage(err)) } finally { setImporting(false) }
  }

  const copy = async () => {
    await platform.copyText(snapshot.mermaid)
    toast('success', 'Mermaid copiado — cole no README do projeto')
  }

  const download = () => void platform.saveFile(new Blob([snapshot.mermaid], { type: 'text/plain' }), 'macro.mermaid')

  return (
    <Modal open={open} onClose={onClose} wide title="Mermaid"
      description=".arch/diagrams/macro.mermaid é regenerado a cada gravação do diagrama."
      footer={
        tab === 'view'
          ? <>
              <Button variant="ghost" icon={Copy} onClick={() => void copy()}>Copiar</Button>
              <Button variant="secondary" icon={ArrowDownToLine} onClick={download}>Baixar</Button>
            </>
          : <>
              <Button variant="ghost" onClick={onClose}>Cancelar</Button>
              <Button variant="primary" icon={Import} loading={importing} disabled={!source.trim()}
                onClick={() => void doImport()}>Importar diagrama</Button>
            </>
      }>
      <div className="mb-3 flex gap-1 rounded-lg surface-3 p-1">
        {(['view', 'import'] as const).map((id) => (
          <button key={id} onClick={() => setTab(id)}
            className={`flex-1 rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
              tab === id ? 'surface text-app shadow-sm' : 'text-muted-app hover:text-app'}`}>
            {id === 'view' ? 'Espelho atual' : 'Importar snippet'}
          </button>
        ))}
      </div>

      {tab === 'view' ? (
        <div className="space-y-3">
          <div ref={setPreview} className="overflow-x-auto rounded-lg border border-app p-4 text-xs text-muted-foreground [&_svg]:mx-auto" />
          <pre className="max-h-56 overflow-auto rounded-lg surface-3 p-3 font-mono text-[11px] leading-relaxed text-muted-app">
            {snapshot.mermaid || '// diagrama vazio'}
          </pre>
        </div>
      ) : (
        <div className="space-y-2">
          <Textarea rows={16} className="font-mono text-xs" value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder={'flowchart LR\n  web(["Web App"]) --> api["Core API"]\n  api --> db[("PostgreSQL")]'} />
          <p className="text-[11px] leading-relaxed text-muted-app">
            O formato dos nós define o tipo do componente: <code>["…"]</code> serviço, <code>[("…")]</code> banco,
            {' '}<code>(("…"))</code> cache, <code>{'{{"…"}}'}</code> gateway, <code>(["…"])</code> cliente,
            {' '}<code>[["…"]]</code> serviço externo, <code>{'>"…"]'}</code> fila.
          </p>
        </div>
      )}
    </Modal>
  )
}
