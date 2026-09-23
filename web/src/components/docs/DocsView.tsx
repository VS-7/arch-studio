import { BookOpen, Bot, FileSignature, GitBranch, ListChecks, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { Snapshot } from '../../lib/types'
import { Button, Card, EmptyState, Spinner, useToast } from '../ui'
import { AdrPanel } from './AdrPanel'
import { MarkdownView } from './MarkdownView'
import { RequirementsPanel } from './RequirementsPanel'
import { UseCasesPanel } from './UseCasesPanel'

type Tab = 'requirements' | 'use-cases' | 'adrs' | 'ai-prd' | 'proposal'

const TABS: { id: Tab; label: string; icon: typeof BookOpen }[] = [
  { id: 'requirements', label: 'Requisitos', icon: BookOpen },
  { id: 'use-cases', label: 'Casos de Uso', icon: ListChecks },
  { id: 'adrs', label: 'Decisões (ADR)', icon: GitBranch },
  { id: 'ai-prd', label: 'AI-PRD', icon: Bot },
  { id: 'proposal', label: 'Proposta', icon: FileSignature },
]

export function DocsView({ snapshot, onGeneratePRD }: { snapshot: Snapshot; onGeneratePRD: () => void }) {
  const [tab, setTab] = useState<Tab>('requirements')

  return (
    <div className="mx-auto h-full w-full max-w-5xl overflow-y-auto px-5 py-5">
      <nav className="mb-4 flex flex-wrap gap-1 rounded-xl surface p-1 border border-app">
        {TABS.map(({ id, label, icon: Icon }) => (
          <button key={id} onClick={() => setTab(id)}
            className={`flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs font-medium transition-colors ${
              tab === id ? 'bg-sky-500 text-white' : 'text-muted-app hover:surface-3 hover:text-app'}`}>
            <Icon size={13} />
            {label}
          </button>
        ))}
      </nav>

      {tab === 'requirements' && <RequirementsPanel snapshot={snapshot} />}
      {tab === 'use-cases' && <UseCasesPanel snapshot={snapshot} />}
      {tab === 'adrs' && <AdrPanel snapshot={snapshot} />}
      {tab === 'ai-prd' && <FileViewer path="docs/ai-prd.md" title="docs/ai-prd.md"
        emptyTitle="AI-PRD ainda não gerado"
        emptyDescription="O compilador consolida diagrama, contratos, requisitos e casos de uso em um blueprint com ordem topológica de implementação."
        emptyAction={<Button variant="primary" icon={Bot} onClick={onGeneratePRD}>Gerar AI-PRD</Button>}
        refreshKey={snapshot.tasks.source_hash} />}
      {tab === 'proposal' && <ProposalTab snapshot={snapshot} />}
    </div>
  )
}

function FileViewer({ path, title, emptyTitle, emptyDescription, emptyAction, refreshKey }: {
  path: string; title: string; emptyTitle: string; emptyDescription: string
  emptyAction?: React.ReactNode; refreshKey?: string
}) {
  const [content, setContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const load = () => {
    setLoading(true)
    api.readFile(path)
      .then((res: { content: string }) => setContent(res.content))
      .catch(() => setContent(null))
      .finally(() => setLoading(false))
  }

  useEffect(load, [path, refreshKey])

  if (loading) return <Spinner label="Carregando documento…" />
  if (!content) {
    return (
      <Card>
        <EmptyState icon={Bot} title={emptyTitle} description={emptyDescription} action={emptyAction} />
      </Card>
    )
  }
  return (
    <Card title={title} actions={<Button size="sm" variant="ghost" icon={RefreshCw} onClick={load}>Recarregar</Button>}>
      <MarkdownView source={content} />
    </Card>
  )
}

function ProposalTab({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const [content, setContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [generating, setGenerating] = useState(false)
  const [client, setClient] = useState('')

  const load = () => {
    setLoading(true)
    api.readFile('docs/proposta-comercial.md')
      .then((res: { content: string }) => setContent(res.content))
      .catch(() => setContent(null))
      .finally(() => setLoading(false))
  }
  useEffect(load, [])

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

  if (loading) return <Spinner label="Carregando proposta…" />

  return (
    <Card title="docs/proposta-comercial.md"
      actions={
        <>
          <input value={client} onChange={(e) => setClient(e.target.value)} placeholder="Nome do cliente"
            className="h-7 w-40 rounded-md border border-app surface px-2 text-xs text-app placeholder:text-muted-app/60 focus:outline-none focus:border-sky-500" />
          <Button size="sm" variant="primary" icon={FileSignature} loading={generating} onClick={() => void generate()}>
            {content ? 'Regerar' : 'Gerar proposta'}
          </Button>
        </>
      }>
      {content
        ? <MarkdownView source={content} />
        : <EmptyState icon={FileSignature} title="Nenhuma proposta gerada"
            description={`A proposta consolida escopo, componentes, esforço e investimento do projeto "${snapshot.manifest.project_name}" em um documento pronto para enviar ao cliente.`} />}
    </Card>
  )
}
