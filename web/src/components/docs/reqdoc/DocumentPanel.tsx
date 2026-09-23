// Documento de Requisitos: pré-visualização fiel ao arquivo final, metadados que
// só existem aqui (cliente, usuários, histórico, referências, glossário) e
// exportação em Markdown, PDF e DOCX. Todo o resto — requisitos, casos de uso,
// diagramas, arquitetura, ADRs e contratos — é puxado do projeto na hora.

import {
  AlertTriangle, CheckCircle2, ChevronDown, FileDown, FileText, FileType2, Loader2, Printer, RefreshCw, Save,
  Settings2,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { api } from '../../../lib/api'
import type { ReqDocument, Snapshot } from '../../../lib/types'
import { Button, EmptyState, useToast } from '../../ui'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from '../../ui/dropdown-menu'
import { DocumentMetaEditor } from './DocumentMetaEditor'
import { buildDocx } from './docx'
import { DOC_CSS, documentHtml, printDocument } from './html'

function download(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 4000)
}

function fileBase(doc: ReqDocument): string {
  const slug = doc.project.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
  return `documento-de-requisitos-${slug || 'projeto'}`
}

/** Lacunas que empobrecem o documento, calculadas a partir do projeto. */
function pendencies(snapshot: Snapshot): string[] {
  const out: string[] = []
  const meta = snapshot.document
  const reqs = snapshot.requirements.requirements
  if (!meta?.client?.trim()) out.push('Seção "Cliente" vazia — preencha em Dados do documento.')
  if (!meta?.users?.trim()) out.push('Seção "Usuário" vazia — preencha em Dados do documento.')
  if (!snapshot.requirements.overview?.trim() || snapshot.requirements.overview.includes('Descreva aqui')) {
    out.push('"Visão Geral do Sistema" ainda com o texto padrão (aba Requisitos).')
  }
  const noCategory = reqs.filter((r) => r.type === 'RNF' && !r.category).length
  if (noCategory) out.push(`${noCategory} requisito(s) não funcional(is) sem categoria — aparecem em "Gerais".`)
  const noDesc = snapshot.use_cases.filter((uc) => !uc.description?.trim()).length
  if (noDesc) out.push(`${noDesc} caso(s) de uso sem "Descrição do caso de uso".`)
  const noReq = snapshot.use_cases.filter((uc) => !uc.requirements?.length).length
  if (noReq) out.push(`${noReq} caso(s) de uso sem requisitos associados.`)
  const noPost = snapshot.use_cases.filter((uc) => !uc.post_conditions?.length).length
  if (noPost) out.push(`${noPost} caso(s) de uso sem "Saídas e pós-condições".`)
  const covered = new Set(snapshot.use_cases.flatMap((uc) => uc.requirements ?? []))
  const orphanRF = reqs.filter((r) => r.type === 'RF' && !covered.has(r.id)).length
  if (orphanRF) out.push(`${orphanRF} requisito(s) funcional(is) sem caso de uso que os detalhe.`)
  if (!(snapshot.uml_diagrams ?? []).some((d) => d.kind === 'usecase')) out.push('Nenhum diagrama de casos de uso — a seção 5 ficará de fora.')
  return out
}

export function DocumentPanel({ snapshot }: { snapshot: Snapshot }) {
  const toast = useToast()
  const [doc, setDoc] = useState<ReqDocument | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [metaOpen, setMetaOpen] = useState(false)
  const [busy, setBusy] = useState<string | null>(null)
  const [showPending, setShowPending] = useState(true)

  const load = useCallback(async () => {
    try {
      setDoc(await api.reqDocument())
      setError(null)
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }, [])

  // Regenera sempre que o projeto muda (requisitos, fichas, diagramas, metadados…).
  useEffect(() => {
    const t = window.setTimeout(() => void load(), 150)
    return () => window.clearTimeout(t)
  }, [load, snapshot])

  const html = useMemo(() => (doc ? documentHtml(doc) : ''), [doc])
  const pending = useMemo(() => pendencies(snapshot), [snapshot])

  const run = async (label: string, fn: () => Promise<void>) => {
    setBusy(label)
    try { await fn() } catch (err) { toast('error', (err as Error).message) } finally { setBusy(null) }
  }

  const exportMarkdown = () => run('md', async () => {
    const res = await fetch(api.reqDocumentMarkdownUrl(true))
    if (!res.ok) throw new Error('falha ao gerar o Markdown')
    download(await res.blob(), `${fileBase(doc!)}.md`)
    toast('success', 'Markdown exportado com os diagramas embutidos')
  })
  const exportPdf = () => run('pdf', async () => {
    await printDocument(doc!)
    toast('info', 'Escolha "Salvar como PDF" na janela de impressão')
  })
  const exportDocx = () => run('docx', async () => {
    download(await buildDocx(doc!), `${fileBase(doc!)}.docx`)
    toast('success', 'DOCX exportado')
  })
  const saveToProject = () => run('save', async () => {
    const res = await api.saveReqDocument()
    toast('success', `Gravado em ${res.file} com ${res.images.length} diagrama(s) — versionável no Git`)
  })

  if (loading && !doc) return <div className="flex justify-center py-16"><Loader2 className="animate-spin text-muted-foreground" /></div>
  if (error && !doc) {
    return <EmptyState icon={AlertTriangle} title="Não foi possível gerar o documento" description={error}
      action={<Button size="sm" icon={RefreshCw} onClick={() => void load()}>Tentar novamente</Button>} />
  }
  if (!doc) return null

  const counts = {
    rf: snapshot.requirements.requirements.filter((r) => r.type === 'RF').length,
    rnf: snapshot.requirements.requirements.filter((r) => r.type === 'RNF').length,
    cdu: snapshot.use_cases.length,
    fig: doc.blocks.filter((b) => b.type === 'image').length,
  }

  return (
    <div className="space-y-3">
      {/* Barra de ações */}
      <div className="flex flex-wrap items-center gap-2 rounded-lg border bg-card px-3 py-2 shadow-xs">
        <FileText size={16} className="text-muted-foreground" />
        <div className="min-w-0">
          <p className="text-[13px] font-semibold">{doc.title} — {doc.project}</p>
          <p className="text-[11.5px] text-muted-foreground">
            Versão {doc.version} · {doc.date} · {counts.rf} RF · {counts.rnf} RNF · {counts.cdu} casos de uso · {counts.fig} figura(s)
          </p>
        </div>
        <div className="ml-auto flex flex-wrap items-center gap-1.5">
          <Button size="sm" variant="ghost" icon={RefreshCw} onClick={() => void load()}>Atualizar</Button>
          <Button size="sm" variant="secondary" icon={Settings2} onClick={() => setMetaOpen(true)}>Dados do documento</Button>
          <Button size="sm" variant="secondary" icon={Save} loading={busy === 'save'} onClick={() => void saveToProject()}>Salvar no projeto</Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button size="sm" variant="primary" icon={FileDown} loading={busy !== null && busy !== 'save'}>
                Exportar <ChevronDown size={13} />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-64">
              <DropdownMenuLabel>Exportar documento</DropdownMenuLabel>
              <DropdownMenuItem onSelect={() => void exportDocx()}><FileType2 />Word (.docx)</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => void exportPdf()}><Printer />PDF (imprimir / salvar como PDF)</DropdownMenuItem>
              <DropdownMenuItem onSelect={() => void exportMarkdown()}><FileText />Markdown (.md, diagramas embutidos)</DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onSelect={() => void saveToProject()}><Save />Salvar em docs/ (Markdown + SVGs)</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      {/* Pendências: o que falta para o documento ficar completo */}
      {pending.length > 0 ? (
        <div className="rounded-lg border bg-card shadow-xs">
          <button className="flex w-full items-center gap-2 px-3 py-2 text-left text-[12.5px] font-medium"
            onClick={() => setShowPending(!showPending)}>
            <AlertTriangle size={14} className="text-warning" />
            {pending.length} ponto(s) para completar o documento
            <ChevronDown size={14} className={`ml-auto transition-transform ${showPending ? '' : '-rotate-90'}`} />
          </button>
          {showPending && (
            <ul className="list-disc space-y-0.5 border-t px-3 py-2 pl-8 text-[12px] text-muted-foreground">
              {pending.map((p) => <li key={p}>{p}</li>)}
            </ul>
          )}
        </div>
      ) : (
        <p className="flex items-center gap-2 px-1 text-[12px] text-muted-foreground">
          <CheckCircle2 size={14} className="text-success" /> Documento completo: todas as seções têm conteúdo.
        </p>
      )}

      {/* Pré-visualização em "papel" — o mesmo HTML usado no PDF */}
      <div className="overflow-x-auto rounded-lg border bg-muted/60 p-4">
        <style>{DOC_CSS}</style>
        <div className="mx-auto w-full max-w-[21cm] shadow-md" dangerouslySetInnerHTML={{ __html: html }} />
      </div>

      <DocumentMetaEditor open={metaOpen} onClose={() => setMetaOpen(false)} snapshot={snapshot} />
    </div>
  )
}
