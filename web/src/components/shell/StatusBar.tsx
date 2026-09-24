// Barra de status: sincronização, arquivo aberto, contagens, qualidade e zoom.

import { AlertTriangle, CheckCircle2 } from 'lucide-react'
import { cn } from '@/lib/utils'
import { relativeTime } from '../../lib/format'
import type { LintReport, ServerEvent } from '../../lib/types'
import { Tip } from '../ui'

export function StatusBar({ connected, path, counts, toolActive, lastEvent, lint, onLint, zoom }: {
  connected: boolean
  path: string
  /** Contagens do diagrama aberto ("3 elementos · 2 relações"). */
  counts: string | null
  toolActive: boolean
  lastEvent: ServerEvent | null
  lint: LintReport | null
  onLint: () => void
  /** Zoom do canvas aberto; null fora dos diagramas. */
  zoom: number | null
}) {
  return (
    <footer className="flex h-6 shrink-0 items-center gap-3 bg-statusbar px-2.5 text-[11.5px] text-statusbar-foreground">
      <Tip label={connected ? 'Sincronização em tempo real ativa' : 'Reconectando ao servidor…'} side="top">
        <span className="flex items-center gap-1.5">
          <span className={cn('size-1.5 rounded-full', connected ? 'bg-success' : 'animate-pulse bg-warning')} />
          {connected ? 'Sincronizado' : 'Reconectando…'}
        </span>
      </Tip>
      {path && <span className="truncate font-mono opacity-90">{path}</span>}
      {counts && <span className="opacity-90">{counts}</span>}
      {toolActive && <span className="rounded-sm bg-accent px-1.5">Ferramenta ativa · Esc cancela</span>}
      <div className="ml-auto flex items-center gap-3">
        {lastEvent?.at && <span className="hidden opacity-80 md:inline">última alteração {relativeTime(lastEvent.at)}</span>}
        {lint && (
          <Tip label="Relatório de validação da arquitetura" side="top">
          <button onClick={onLint} className="flex items-center gap-1 rounded-sm px-1 hover:bg-accent">
            {lint.errors > 0 || lint.warnings > 0 ? <AlertTriangle size={12} /> : <CheckCircle2 size={12} />}
            Qualidade {lint.score}
          </button>
          </Tip>
        )}
        {zoom !== null && <span className="tabular-nums">{Math.round(zoom * 100)}%</span>}
      </div>
    </footer>
  )
}
