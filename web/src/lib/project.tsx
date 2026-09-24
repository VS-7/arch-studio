// Estado global do projeto: snapshot em memória, conexão em tempo real e
// recarga reativa a eventos do servidor (RF023).

import {
  createContext, useCallback, useContext, useEffect, useMemo, useRef, useState,
  type ReactNode,
} from 'react'
import { api } from './api'
import { ApiError } from './errors'
import { events } from './transport'
import type { LintReport, ServerEvent, Snapshot } from './types'

/** Falha ao carregar o projeto; status é o HTTP (409 = nenhum projeto aberto no app desktop). */
export interface ProjectError { message: string; hint?: string; status?: number }

interface ProjectState {
  snapshot: Snapshot | null
  lint: LintReport | null
  loading: boolean
  error: ProjectError | null
  connected: boolean
  /** Id do nó recém-criado por um agente de IA, para destacar no canvas. */
  highlight: string | null
  lastEvent: ServerEvent | null
  refresh: (options?: { silent?: boolean }) => Promise<void>
}

const Context = createContext<ProjectState | null>(null)

export function useProject(): ProjectState {
  const ctx = useContext(Context)
  if (!ctx) throw new Error('useProject precisa estar dentro de <ProjectProvider>')
  return ctx
}

/** Eventos que exigem recarregar o snapshot. */
const RELOAD_EVENTS = new Set([
  'diagram_changed', 'docs_changed', 'endpoints_changed', 'pricing_changed',
  'tasks_changed', 'manifest_changed', 'node_added', 'node_updated',
  'node_removed', 'edge_added', 'ai_prd_generated', 'uml_changed',
])

export function ProjectProvider({
  children, onEvent,
}: { children: ReactNode; onEvent?: (event: ServerEvent) => void }) {
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [lint, setLint] = useState<LintReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<ProjectError | null>(null)
  const [connected, setConnected] = useState(false)
  const [highlight, setHighlight] = useState<string | null>(null)
  const [lastEvent, setLastEvent] = useState<ServerEvent | null>(null)

  const reloadTimer = useRef<number | null>(null)
  const onEventRef = useRef(onEvent)
  onEventRef.current = onEvent

  const refresh = useCallback(async (options?: { silent?: boolean }) => {
    if (!options?.silent) setLoading(true)
    try {
      const [snap, report] = await Promise.all([api.snapshot(), api.validate().catch(() => null)])
      setSnapshot(snap)
      if (report) setLint(report)
      setError(null)
    } catch (err) {
      if (err instanceof ApiError) setError({ message: err.message, hint: err.hint, status: err.status })
      else setError({ message: err instanceof Error ? err.message : 'falha desconhecida' })
    } finally {
      setLoading(false)
    }
  }, [])

  // Agrupa rajadas de eventos numa única recarga.
  const scheduleReload = useCallback(() => {
    if (reloadTimer.current) window.clearTimeout(reloadTimer.current)
    reloadTimer.current = window.setTimeout(() => {
      reloadTimer.current = null
      void refresh({ silent: true })
    }, 120)
  }, [refresh])

  useEffect(() => {
    void refresh()
    const unsubscribe = events.subscribe((event) => {
      onEventRef.current?.(event)
      if (event.type === 'connection') { setConnected(true); scheduleReload(); return }
      if (event.type === 'disconnection') { setConnected(false); return }
      setLastEvent(event)
      if (event.type === 'node_added' && event.source === 'ai') {
        const payload = event.payload as { node?: { id?: string } } | undefined
        if (payload?.node?.id) {
          setHighlight(payload.node.id)
          window.setTimeout(() => setHighlight((cur) => (cur === payload.node?.id ? null : cur)), 3000)
        }
      }
      if (RELOAD_EVENTS.has(event.type)) scheduleReload()
    })
    return () => {
      unsubscribe()
      if (reloadTimer.current) window.clearTimeout(reloadTimer.current)
    }
  }, [refresh, scheduleReload])

  const value = useMemo<ProjectState>(() => ({
    snapshot, lint, loading, error, connected, highlight, lastEvent, refresh,
  }), [snapshot, lint, loading, error, connected, highlight, lastEvent, refresh])

  return <Context.Provider value={value}>{children}</Context.Provider>
}
