// Documento de Requisitos: pré-visualização fiel ao arquivo final, metadados que
// só existem aqui (cliente, usuários, histórico, referências, glossário) e
// exportação em Markdown, PDF e DOCX. Todo o resto — requisitos, casos de uso,
// diagramas, arquitetura, ADRs e contratos — é puxado do projeto na hora.
//
// Layout de editor: as folhas A4 já paginadas (capa, depois páginas com
// cabeçalho e rodapé numerado) ocupam a área central, com zoom e "ajustar à
// largura"; um painel lateral traz a estrutura navegável e as pendências. O
// PDF imprime exatamente essas folhas.

import {
  AlertTriangle, CheckCircle2, ChevronDown, FileDown, FileText, FileType2, ListTree, Loader2, PanelRight, Printer,
  RefreshCw, Save, Settings2, Wand2, ZoomIn, ZoomOut,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../../lib/api'
import { errorMessage } from '../../../lib/errors'
import { slugify } from '../../../lib/format'
import { platform } from '../../../lib/platform'
import { useStored } from '../../../lib/storage'
import type { ReqDocument, Snapshot } from '../../../lib/types'
import { Button, EmptyState, IconAction, useToast } from '../../ui'
import {
  DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger,
} from '../../ui/dropdown-menu'
import { ViewFrame } from '../../shell/ViewFrame'
import { DocumentMetaEditor } from './DocumentMetaEditor'
import { buildDocx } from './docx'
import { DOC_CSS, printDocument } from './html'
import { paginateDocument, type PagedDocument } from './paginate'

/** Largura da folha A4 (21 cm) em pixels CSS. */
const PAGE_PX = (21 / 2.54) * 96
const ZOOM_STEPS = [0.5, 0.67, 0.75, 0.9, 1, 1.1, 1.25, 1.5, 1.75, 2]

function fileBase(doc: ReqDocument): string {
  return `documento-de-requisitos-${slugify(doc.project, 'projeto')}`
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

/** Hash curto (FNV-1a) de um texto. */
function hash(text: string): string {
  let h = 0x811c9dc5
  for (let i = 0; i < text.length; i++) h = Math.imul(h ^ text.charCodeAt(i), 0x01000193)
  return (h >>> 0).toString(36)
}

/**
 * Versiona a URL de cada figura pelo conteúdo do diagrama. O navegador
 * reaproveita imagens com a mesma URL dentro da página, e a prévia mostraria
 * o desenho antigo depois de editar ou reorganizar um diagrama.
 */
function withFigureVersions(doc: ReqDocument, snapshot: Snapshot): ReqDocument {
  const versions = new Map<string, string>([
    ['macro', hash(JSON.stringify([snapshot.diagram.nodes, snapshot.diagram.edges]))],
  ])
  for (const d of snapshot.uml_diagrams ?? []) versions.set(d.id, hash(JSON.stringify([d.elements, d.relations, d.name])))
  return {
    ...doc,
    blocks: doc.blocks.map((b) => {
      const v = b.type === 'image' ? versions.get(b.diagram) : undefined
      return b.type === 'image' && v ? { ...b, src: `${b.src}${b.src.includes('?') ? '&' : '?'}v=${v}` } : b
    }),
  }
}

export function DocumentPanel({ snapshot, onReorganize }: {
  snapshot: Snapshot
  /** Reorganiza todos os diagramas para caber na página. */
  onReorganize?: () => void
}) {
  const toast = useToast()
  const [doc, setDoc] = useState<ReqDocument | null>(null)
  const [paged, setPaged] = useState<PagedDocument | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [metaOpen, setMetaOpen] = useState(false)
  const [busy, setBusy] = useState<string | null>(null)
  const [sideOpen, setSideOpen] = useStored('archcode-doc-side', true)
  const [zoom, setZoom] = useStored<'fit' | number>('archcode-doc-zoom', 'fit')
  const [fitZoom, setFitZoom] = useState(1)
  const desk = useRef<HTMLDivElement>(null)
  const snapshotRef = useRef(snapshot)
  snapshotRef.current = snapshot

  const load = useCallback(async () => {
    try {
      setDoc(withFigureVersions(await api.reqDocument(), snapshotRef.current))
      setError(null)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [])

  // Regenera sempre que o projeto muda (requisitos, fichas, diagramas, metadados…).
  useEffect(() => {
    const t = window.setTimeout(() => void load(), 150)
    return () => window.clearTimeout(t)
  }, [load, snapshot])

  // Pagina em folhas A4 sempre que o documento muda. As folhas anteriores
  // continuam visíveis até as novas ficarem prontas (sem piscar).
  useEffect(() => {
    if (!doc) return
    let cancelled = false
    paginateDocument(doc)
      .then((p) => { if (!cancelled) setPaged(p) })
      .catch((err) => { if (!cancelled) setError(errorMessage(err)) })
    return () => { cancelled = true }
  }, [doc])

  // "Ajustar à largura": a folha acompanha a largura disponível da área central.
  const hasDoc = paged !== null
  useEffect(() => {
    const el = desk.current
    if (!el) return
    const measure = () => {
      const style = getComputedStyle(el)
      const inner = el.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)
      setFitZoom(Math.min(2, Math.max(0.25, inner / PAGE_PX)))
    }
    measure()
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => ro.disconnect()
  }, [hasDoc])

  const pending = useMemo(() => pendencies(snapshot), [snapshot])
  const scale = zoom === 'fit' ? fitZoom : zoom

  const stepZoom = (dir: 1 | -1) => {
    const next = dir > 0 ? ZOOM_STEPS.find((z) => z > scale + 0.001) : [...ZOOM_STEPS].reverse().find((z) => z < scale - 0.001)
    if (next) setZoom(next)
  }

  const goTo = (id: string) => {
    desk.current?.querySelector(`[id="${CSS.escape(id)}"]`)?.scrollIntoView({ block: 'start', behavior: 'smooth' })
  }

  const run = async (label: string, fn: () => Promise<void>) => {
    setBusy(label)
    try { await fn() } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(null) }
  }

  const exportMarkdown = () => run('md', async () => {
    await platform.saveFile(await api.reqDocumentMarkdown(true), `${fileBase(doc!)}.md`)
    toast('success', 'Markdown exportado com os diagramas embutidos')
  })
  const exportPdf = () => run('pdf', async () => {
    await printDocument(doc!, paged!.html)
    toast('info', 'Escolha "Salvar como PDF" na janela de impressão')
  })
  const exportDocx = () => run('docx', async () => {
    await platform.saveFile(await buildDocx(doc!), `${fileBase(doc!)}.docx`)
    toast('success', 'DOCX exportado')
  })
  const saveToProject = () => run('save', async () => {
    const res = await api.saveReqDocument()
    toast('success', `Gravado em ${res.file} com ${res.images.length} diagrama(s) — versionável no Git`)
  })

  if (!doc || !paged) {
    return (
      <ViewFrame icon={FileText} title="Documento de Requisitos" meta={snapshot.manifest.project_name}>
        {loading || (doc && !error) ? (
          <div className="flex justify-center py-16"><Loader2 className="animate-spin text-muted-foreground" /></div>
        ) : (
          <EmptyState icon={AlertTriangle} title="Não foi possível gerar o documento" description={error ?? undefined}
            action={<Button size="sm" icon={RefreshCw} onClick={() => void load()}>Tentar novamente</Button>} />
        )}
      </ViewFrame>
    )
  }

  const counts = {
    rf: snapshot.requirements.requirements.filter((r) => r.type === 'RF').length,
    rnf: snapshot.requirements.requirements.filter((r) => r.type === 'RNF').length,
    cdu: snapshot.use_cases.length,
    fig: doc.blocks.filter((b) => b.type === 'image').length,
  }

  return (
    <ViewFrame icon={FileText} title={`${doc.title} — ${doc.project}`}
      meta={`Versão ${doc.version} · ${doc.date} · ${paged.pages} páginas · ${counts.rf} RF · ${counts.rnf} RNF · ${counts.cdu} casos de uso · ${counts.fig} figura(s)`}
      bodyClassName="flex overflow-hidden"
      actions={
        <>
          {/* Zoom da folha */}
          <div className="flex h-7 items-center rounded-md border bg-background">
            <IconAction label="Afastar" onClick={() => stepZoom(-1)} className="h-full px-1.5"><ZoomOut size={14} /></IconAction>
            <button type="button" onClick={() => setZoom(zoom === 'fit' ? 1 : 'fit')}
              title={zoom === 'fit' ? 'Ajustado à largura — clique para 100%' : 'Clique para ajustar à largura'}
              className="h-full min-w-[4.5rem] border-x px-1.5 text-[11.5px] tabular-nums text-muted-foreground hover:bg-accent hover:text-foreground">
              {zoom === 'fit' ? `Ajustar · ${Math.round(scale * 100)}%` : `${Math.round(scale * 100)}%`}
            </button>
            <IconAction label="Aproximar" onClick={() => stepZoom(1)} className="h-full px-1.5"><ZoomIn size={14} /></IconAction>
          </div>
          <Button size="sm" variant="ghost" icon={RefreshCw} onClick={() => void load()}>Atualizar</Button>
          {onReorganize && counts.fig > 0 && (
            <Button size="sm" variant="ghost" icon={Wand2} onClick={onReorganize}
              title="Redispõe todos os diagramas com espaçamento, para caberem na página (o aviso oferece Desfazer)">
              Reorganizar diagramas
            </Button>
          )}
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
          <Button size="icon" variant={sideOpen ? 'secondary' : 'ghost'} aria-label={sideOpen ? 'Ocultar estrutura e pendências' : 'Mostrar estrutura e pendências'}
            onClick={() => setSideOpen(!sideOpen)}>
            <PanelRight size={14} />
          </Button>
        </>
      }>
      {/* Folhas A4 paginadas — o mesmo HTML usado no PDF */}
      <div ref={desk} className="min-w-0 flex-1 overflow-auto bg-muted px-6 py-6 dark:bg-chrome-2">
        <style>{DOC_CSS}</style>
        <div className="mx-auto w-[21cm]" style={{ zoom: scale }} dangerouslySetInnerHTML={{ __html: paged.html }} />
      </div>

      {sideOpen && (
        <aside className="flex w-64 shrink-0 flex-col overflow-y-auto border-l bg-chrome">
          <SideTitle icon={pending.length ? AlertTriangle : CheckCircle2}
            className={pending.length ? 'text-warning' : 'text-success'}>
            {pending.length ? `Pendências (${pending.length})` : 'Documento completo'}
          </SideTitle>
          {pending.length > 0 ? (
            <ul className="space-y-1.5 px-3 py-2.5 text-[12px] leading-snug text-muted-foreground">
              {pending.map((p) => (
                <li key={p} className="flex gap-1.5"><span className="mt-[7px] size-1 shrink-0 rounded-full bg-warning" />{p}</li>
              ))}
            </ul>
          ) : (
            <p className="px-3 py-2.5 text-[12px] text-muted-foreground">Todas as seções têm conteúdo.</p>
          )}

          <SideTitle icon={ListTree}>Estrutura</SideTitle>
          <nav className="py-1 text-[12px]">
            {doc.toc.map((t) => (
              <button key={t.id} type="button" onClick={() => goTo(t.id)}
                className={cn('flex w-full gap-1.5 truncate py-[3px] pr-2 text-left hover:bg-accent',
                  t.level === 1 ? 'font-semibold text-foreground' : 'text-muted-foreground hover:text-foreground')}
                style={{ paddingLeft: 12 + (t.level - 1) * 12 }}>
                <span className="shrink-0 tabular-nums">{t.number}</span>
                <span className="truncate">{t.title}</span>
              </button>
            ))}
          </nav>
        </aside>
      )}

      <DocumentMetaEditor open={metaOpen} onClose={() => setMetaOpen(false)} snapshot={snapshot} />
    </ViewFrame>
  )
}

function SideTitle({ icon: Icon, className, children }: { icon: typeof ListTree; className?: string; children: React.ReactNode }) {
  return (
    <p className="sticky top-0 z-10 flex h-7 shrink-0 items-center gap-1.5 border-b bg-panel-header px-3 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
      <Icon size={13} className={className} />{children}
    </p>
  )
}
