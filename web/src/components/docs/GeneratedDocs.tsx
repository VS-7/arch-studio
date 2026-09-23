// Documentos gerados pelo Studio e gravados em docs/: o AI-PRD (blueprint para
// agentes de IA) e a proposta comercial. Cada um abre na sua própria aba.

import { Bot, FileSignature, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { api } from '../../lib/api'
import type { Snapshot } from '../../lib/types'
import { ViewFrame } from '../shell/ViewFrame'
import { Button, EmptyState, Spinner, useToast } from '../ui'
import { MarkdownView } from './MarkdownView'

/** Lê um arquivo do projeto; `null` quando ainda não existe. */
function useProjectFile(path: string, refreshKey?: string) {
  const [content, setContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const load = useCallback(() => {
    setLoading(true)
    api.readFile(path)
      .then((res: { content: string }) => setContent(res.content))
      .catch(() => setContent(null))
      .finally(() => setLoading(false))
  }, [path])
  useEffect(load, [load, refreshKey])
  return { content, setContent, loading, load }
}

/** Corpo de leitura: coluna confortável, mas sem cartão nem margens sobrando. */
function Reading({ children }: { children: ReactNode }) {
  return <div className="mx-auto w-full max-w-5xl px-6 py-5">{children}</div>
}

export function AiPrdView({ snapshot, generating, onGeneratePRD }: {
  snapshot: Snapshot; generating: boolean; onGeneratePRD: () => void
}) {
  const { content, loading, load } = useProjectFile('docs/ai-prd.md', snapshot.tasks.source_hash)
  const tasks = snapshot.tasks.tasks.length

  return (
    <ViewFrame icon={Bot} title="AI-PRD"
      meta={content ? `docs/ai-prd.md · ${tasks} tarefa(s) · hash ${snapshot.tasks.source_hash}` : 'docs/ai-prd.md'}
      actions={
        <>
          {content && <Button size="sm" variant="ghost" icon={RefreshCw} onClick={load}>Recarregar</Button>}
          <Button size="sm" variant="primary" icon={Bot} loading={generating} onClick={onGeneratePRD}>
            {content ? 'Recompilar AI-PRD' : 'Gerar AI-PRD'}
          </Button>
        </>
      }>
      {loading && content === null ? <Spinner label="Carregando documento…" />
        : content ? <Reading><MarkdownView source={content} /></Reading>
          : (
            <EmptyState icon={Bot} title="AI-PRD ainda não gerado"
              description="O compilador consolida diagrama, contratos, requisitos e casos de uso em um blueprint com ordem topológica de implementação."
              action={<Button variant="primary" icon={Bot} loading={generating} onClick={onGeneratePRD}>Gerar AI-PRD</Button>} />
          )}
    </ViewFrame>
  )
}

export function ProposalView({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const { content, setContent, loading } = useProjectFile('docs/proposta-comercial.md')
  const [generating, setGenerating] = useState(false)
  const [client, setClient] = useState('')

  const generate = async () => {
    setGenerating(true)
    try {
      const res = await api.generateProposal({
        client_name: client.trim() || undefined,
        include_diagram: true, include_cloud: true,
      })
      setContent(res.markdown)
      toast('success', `Proposta gerada em ${res.file_path}`)
    } catch (err) { toast('error', (err as Error).message) } finally { setGenerating(false) }
  }

  return (
    <ViewFrame icon={FileSignature} title="Proposta Comercial" meta="docs/proposta-comercial.md"
      actions={
        <>
          <input value={client} onChange={(e) => setClient(e.target.value)} placeholder="Nome do cliente"
            className="h-7 w-44 rounded-md border border-input bg-background px-2 text-xs placeholder:text-muted-foreground/70 focus:border-ring focus:outline-none" />
          <Button size="sm" variant="primary" icon={FileSignature} loading={generating} onClick={() => void generate()}>
            {content ? 'Regerar' : 'Gerar proposta'}
          </Button>
        </>
      }>
      {loading && content === null ? <Spinner label="Carregando proposta…" />
        : content ? <Reading><MarkdownView source={content} /></Reading>
          : (
            <EmptyState icon={FileSignature} title="Nenhuma proposta gerada"
              description={`A proposta consolida escopo, componentes, esforço e investimento do projeto "${snapshot.manifest.project_name}" em um documento pronto para enviar ao cliente.`} />
          )}
    </ViewFrame>
  )
}
