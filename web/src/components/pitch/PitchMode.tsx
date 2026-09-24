// Modo Apresentação (RF008–RF010).
//
// Interface limpa, sem barras de ferramentas: apenas o diagrama, um passo a
// passo com zoom cinematográfico em cada componente e a alternância entre a
// visão de negócio e a visão de engenharia.

import {
  ChevronLeft, ChevronRight, Download, Eye, FileImage, FileText, Maximize2, X,
} from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { hours as fmtHours, money } from '../../lib/format'
import { exportPdf, exportPng, exportSvg, type ExportMode } from '../../lib/exporters'
import { nodeMeta } from '../../lib/nodeMeta'
import { useTheme } from '../../lib/theme'
import type { Estimate, Snapshot } from '../../lib/types'
import { ArchCanvas, type CanvasHandle } from '../canvas/ArchCanvas'
import { Button, useToast } from '../ui'

interface Slide {
  id: string | null
  title: string
  body: string
  meta?: string
  spotlight: Set<string> | null
}

export function PitchMode({ snapshot, onExit }: { snapshot: Snapshot; onExit: () => void }) {
  const toast = useToast()
  const theme = useTheme()
  const [executive, setExecutive] = useState(true)
  // A estimativa alimenta o sumário do PDF.
  const [estimate, setEstimate] = useState<Estimate | null>(null)
  const [index, setIndex] = useState(0)
  const [busy, setBusy] = useState(false)
  const canvas = useRef<CanvasHandle | null>(null)

  const visibleNodes = useMemo(
    () => snapshot.diagram.nodes.filter((n) => n.type !== 'group' && (!executive || n.data.executive !== false)),
    [snapshot.diagram.nodes, executive],
  )

  const slides = useMemo<Slide[]>(() => {
    const overview: Slide = {
      id: null,
      title: snapshot.manifest.project_name,
      body: snapshot.manifest.description || 'Arquitetura do sistema',
      meta: [
        `${visibleNodes.length} componentes`,
        `${snapshot.diagram.edges.length} integrações`,
        snapshot.use_cases.length ? `${snapshot.use_cases.length} casos de uso` : '',
      ].filter(Boolean).join(' · '),
      spotlight: null,
    }
    // Ordem de leitura: da esquerda (cliente) para a direita (dados).
    const ordered = [...visibleNodes].sort((a, b) => a.position.x - b.position.x || a.position.y - b.position.y)
    const perNode = ordered.map<Slide>((node) => {
      const neighbours = new Set<string>([node.id])
      for (const edge of snapshot.diagram.edges) {
        if (edge.source === node.id) neighbours.add(edge.target)
        if (edge.target === node.id) neighbours.add(edge.source)
      }
      const connections = snapshot.diagram.edges
        .filter((e) => e.source === node.id || e.target === node.id)
        .map((e) => {
          const otherId = e.source === node.id ? e.target : e.source
          const other = snapshot.diagram.nodes.find((n) => n.id === otherId)?.data.label ?? otherId
          return executive ? other : `${other} (${e.data.protocol ?? '—'})`
        })
      return {
        id: node.id,
        title: node.data.label,
        body: node.data.description || (executive ? 'Componente do sistema' : node.data.technology || ''),
        meta: executive
          ? connections.length ? `Conecta-se a ${connections.join(', ')}` : ''
          : [node.data.technology, `tier ${node.data.tier ?? '—'}`, connections.join(' · ')].filter(Boolean).join(' · '),
        spotlight: neighbours,
      }
    })
    return [overview, ...perNode]
  }, [snapshot, visibleNodes, executive])

  const current = slides[Math.min(index, slides.length - 1)]

  const go = useCallback((next: number) => {
    const clamped = Math.max(0, Math.min(next, slides.length - 1))
    setIndex(clamped)
    const slide = slides[clamped]
    if (slide.id) canvas.current?.focusNode(slide.id)
    else canvas.current?.fitAll()
  }, [slides])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onExit()
      if (event.key === 'ArrowRight' || event.key === ' ') { event.preventDefault(); go(index + 1) }
      if (event.key === 'ArrowLeft') { event.preventDefault(); go(index - 1) }
      if (event.key.toLowerCase() === 'e') setExecutive((v) => !v)
      if (event.key.toLowerCase() === 'f') void document.documentElement.requestFullscreen?.().catch(() => {})
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [go, index, onExit])

  useEffect(() => {
    api.estimate().then(setEstimate).catch(() => setEstimate(null))
  }, [snapshot.diagram.last_modified, snapshot.use_cases.length, snapshot.pricing])

  // Trocar de modo reposiciona a apresentação no início, evitando slides órfãos.
  useEffect(() => { setIndex(0); canvas.current?.fitAll() }, [executive])

  const mode: ExportMode = executive ? 'executive' : 'engineering'
  const dark = theme.resolved === 'dark'

  const run = async (label: string, task: () => Promise<void>) => {
    setBusy(true)
    try {
      await task()
      toast('success', `${label} exportado`)
    } catch (err) {
      toast('error', errorMessage(err))
    } finally { setBusy(false) }
  }

  return (
    <div className="fixed inset-0 z-40 flex flex-col" style={{ background: 'var(--canvas-bg)' }}>
      <div className="relative flex-1">
        <ArchCanvas
          diagram={snapshot.diagram}
          executive={executive}
          highlight={null}
          spotlight={current?.spotlight ?? null}
          selectedId={null}
          onSelect={() => {}}
          interactive={false}
          onReady={(handle) => { canvas.current = handle; handle?.fitAll() }}
        />

        {/* Controles discretos, só visíveis ao aproximar o cursor. */}
        <div className="absolute right-4 top-4 flex items-center gap-1.5 opacity-25 transition-opacity hover:opacity-100">
          <Button size="sm" variant="secondary" icon={Eye} onClick={() => setExecutive((v) => !v)}>
            {executive ? 'Visão executiva' : 'Visão de engenharia'}
          </Button>
          <Button size="sm" variant="secondary" icon={FileImage} disabled={busy}
            onClick={() => void run('SVG', () => exportSvg(snapshot.manifest.project_name, mode, dark))}>SVG</Button>
          <Button size="sm" variant="secondary" icon={Download} disabled={busy}
            onClick={() => void run('PNG', () => exportPng(snapshot.manifest.project_name, mode, { scale: 3, transparent: true, dark }))}>PNG</Button>
          <Button size="sm" variant="secondary" icon={FileText} disabled={busy}
            onClick={() => void run('PDF', () => exportPdf(snapshot.manifest.project_name, mode, {
              description: snapshot.manifest.description,
              components: visibleNodes.length,
              connections: snapshot.diagram.edges.length,
              useCases: snapshot.use_cases.length,
              totalHours: estimate ? fmtHours(estimate.total_hours) : undefined,
              totalCost: estimate ? money(estimate.currency, estimate.total_cost) : undefined,
              duration: estimate ? `${Math.round(estimate.working_days)} dias úteis (~${estimate.calendar_months} meses)` : undefined,
            }))}>PDF</Button>
          <Button size="sm" variant="secondary" icon={Maximize2}
            onClick={() => void document.documentElement.requestFullscreen?.().catch(() => {})} aria-label="Tela cheia" />
          <Button size="sm" variant="secondary" icon={X} onClick={onExit}>Sair</Button>
        </div>
      </div>

      {/* Legenda do slide corrente. */}
      <footer className="surface border-t border-app px-8 py-5">
        <div className="mx-auto flex max-w-5xl items-end justify-between gap-8">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              {current?.id && (() => {
                const node = snapshot.diagram.nodes.find((n) => n.id === current.id)
                if (!node) return null
                const meta = nodeMeta(node.type)
                return (
                  <span className="flex h-6 w-6 items-center justify-center rounded-md"
                    style={{ backgroundColor: `${meta.color}22`, color: meta.color }}>
                    <meta.icon size={13} />
                  </span>
                )
              })()}
              <h2 className="truncate text-2xl font-bold text-app">{current?.title}</h2>
            </div>
            <p className="mt-1 text-sm leading-relaxed text-muted-app">{current?.body}</p>
            {current?.meta && <p className="mt-1 text-xs text-muted-app/80">{current.meta}</p>}
          </div>

          <div className="flex shrink-0 items-center gap-3">
            <div className="flex gap-1">
              {slides.map((slide, i) => (
                <button key={slide.id ?? 'overview'} onClick={() => go(i)} aria-label={`Ir para ${slide.title}`}
                  className={`h-1.5 rounded-full transition-all ${i === index ? 'w-6 bg-primary' : 'w-1.5 surface-3 hover:bg-primary/40'}`} />
              ))}
            </div>
            <span className="w-14 text-right text-xs tabular-nums text-muted-app">{index + 1} / {slides.length}</span>
            <Button variant="secondary" size="icon" icon={ChevronLeft} aria-label="Slide anterior (←)" onClick={() => go(index - 1)}
              disabled={index === 0} />
            <Button variant="primary" size="icon" icon={ChevronRight} aria-label="Próximo slide (→)" onClick={() => go(index + 1)}
              disabled={index === slides.length - 1} />
          </div>
        </div>
      </footer>
    </div>
  )
}
