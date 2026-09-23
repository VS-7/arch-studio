// Sincronização bidirecional com Mermaid (RF007): visualizar o espelho gerado e
// importar um snippet colado (C4, flowchart ou graph).

import { ArrowDownToLine, Copy, Import } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import mermaid from 'mermaid'
import { api } from '../../lib/api'
import type { Snapshot } from '../../lib/types'
import { Button, Modal, Textarea, useToast } from '../ui'

export function MermaidPanel({ open, onClose, snapshot }: {
  open: boolean; onClose: () => void; snapshot: Snapshot
}) {
  const toast = useToast()
  const [tab, setTab] = useState<'view' | 'import'>('view')
  const [source, setSource] = useState('')
  const [importing, setImporting] = useState(false)
  const preview = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open || tab !== 'view' || !preview.current) return
    const dark = document.documentElement.classList.contains('dark')
    mermaid.initialize({ startOnLoad: false, theme: dark ? 'dark' : 'default', securityLevel: 'strict' })
    let cancelled = false
    void (async () => {
      try {
        const { svg } = await mermaid.render(`mmd-panel-${Date.now()}`, snapshot.mermaid || 'flowchart LR\n  vazio["Sem componentes"]')
        if (!cancelled && preview.current) preview.current.innerHTML = svg
      } catch (err) {
        if (!cancelled && preview.current) {
          preview.current.innerHTML = `<p class="text-xs text-rose-400">Mermaid inválido: ${(err as Error).message}</p>`
        }
      }
    })()
    return () => { cancelled = true }
  }, [open, tab, snapshot.mermaid])

  const doImport = async () => {
    if (!source.trim()) return
    if (!window.confirm('Importar substitui o diagrama atual. Componentes com o mesmo nome mantêm suas posições. Continuar?')) return
    setImporting(true)
    try {
      const diagram = await api.importMermaid(source)
      toast('success', `Importado: ${diagram.nodes.length} componentes, ${diagram.edges.length} conexões`)
      onClose()
      setSource('')
    } catch (err) { toast('error', (err as Error).message) } finally { setImporting(false) }
  }

  const copy = async () => {
    await navigator.clipboard.writeText(snapshot.mermaid)
    toast('success', 'Mermaid copiado — cole no README do projeto')
  }

  const download = () => {
    const blob = new Blob([snapshot.mermaid], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = 'macro.mermaid'
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 2000)
  }

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
          <div ref={preview} className="overflow-x-auto rounded-xl border border-app p-4" />
          <pre className="max-h-56 overflow-auto rounded-xl surface-3 p-3 font-mono text-[11px] leading-relaxed text-muted-app">
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
