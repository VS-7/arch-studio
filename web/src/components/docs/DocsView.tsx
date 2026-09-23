import { BookOpen, Bot, FileSignature, FileText, GitBranch, ListChecks, RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../lib/api'
import type { Snapshot } from '../../lib/types'
import { Button, Card, EmptyState, Spinner, useToast } from '../ui'
import { Tabs, TabsList, TabsTrigger } from '../ui/tabs'
import { AdrPanel } from './AdrPanel'
import { MarkdownView } from './MarkdownView'
import { RequirementsPanel } from './RequirementsPanel'
import { UseCasesPanel } from './UseCasesPanel'
import { DocumentPanel } from './reqdoc/DocumentPanel'

export type DocsTab = 'document' | 'requirements' | 'use-cases' | 'adrs' | 'ai-prd' | 'proposal'
type Tab = DocsTab

const TABS: { id: Tab; label: string; icon: typeof BookOpen }[] = [
  { id: 'document', label: 'Documento de Requisitos', icon: FileText },
  { id: 'requirements', label: 'Requisitos', icon: BookOpen },
  { id: 'use-cases', label: 'Casos de Uso', icon: ListChecks },
  { id: 'adrs', label: 'Decisões (ADR)', icon: GitBranch },
  { id: 'ai-prd', label: 'AI-PRD', icon: Bot },
  { id: 'proposal', label: 'Proposta', icon: FileSignature },
]

export function DocsView({ snapshot, onGeneratePRD, initialTab, focusUseCase }: {
  snapshot: Snapshot; onGeneratePRD: () => void; initialTab?: Tab; focusUseCase?: string
}) {
  const [tab, setTab] = useState<Tab>(initialTab ?? 'document')

  return (
    <div className="h-full overflow-y-auto">
    <div className="mx-auto w-full max-w-6xl px-5 py-5">
      <Tabs value={tab} onValueChange={(v) => setTab(v as Tab)} className="mb-4">
        <TabsList>
          {TABS.map(({ id, label, icon: Icon }) => (
            <TabsTrigger key={id} value={id}><Icon />{label}</TabsTrigger>
          ))}
        </TabsList>
      </Tabs>

      {tab === 'document' && <DocumentPanel snapshot={snapshot} />}
      {tab === 'requirements' && <RequirementsPanel snapshot={snapshot} />}
      {tab === 'use-cases' && <UseCasesPanel snapshot={snapshot} focusCode={focusUseCase} />}
      {tab === 'adrs' && <AdrPanel snapshot={snapshot} />}
      {tab === 'ai-prd' && <FileViewer path="docs/ai-prd.md" title="docs/ai-prd.md"
        emptyTitle="AI-PRD ainda não gerado"
        emptyDescription="O compilador consolida diagrama, contratos, requisitos e casos de uso em um blueprint com ordem topológica de implementação."
        emptyAction={<Button variant="primary" icon={Bot} onClick={onGeneratePRD}>Gerar AI-PRD</Button>}
        refreshKey={snapshot.tasks.source_hash} />}
      {tab === 'proposal' && <ProposalTab snapshot={snapshot} />}
    </div>
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
            className="h-7 w-40 rounded-md border border-app surface px-2 text-xs text-app placeholder:text-muted-app/60 focus:outline-none focus:border-primary" />
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
