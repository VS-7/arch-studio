// Camada de abstração de transporte (RNF005).
//
// Toda a aplicação fala com o núcleo Go por dois contratos: `ApiClient`
// (requisições e recursos) e `EventStream` (eventos em tempo real). No browser,
// ambos usam HTTP/WebSocket. No app desktop as requisições continuam em HTTP —
// o Wails serve o mesmo handler Go —, mas os eventos chegam pelo runtime do
// Wails. A escolha acontece só aqui, na composição do fim do arquivo: nenhum
// componente de UI precisa saber em que host está.

import { onServerEvent } from './desktop'
import { ApiError } from './errors'
import { HOST } from './host'
import type { ServerEvent } from './types'

export interface ApiClient {
  request<T>(method: string, path: string, body?: unknown): Promise<T>
  /** Baixa um recurso (exportações, figuras) como Blob. */
  fetchBlob(path: string): Promise<Blob>
  /** URL absoluta de um recurso do servidor (usada por <img> e impressão). */
  resourceUrl(path: string): string
}

export interface EventStream {
  subscribe(handler: (event: ServerEvent) => void): () => void
}

/** Prefixo das rotas da API; vazio = mesma origem da página. */
const API_BASE = ''

async function failure(res: Response): Promise<ApiError> {
  const text = await res.text().catch(() => '')
  let payload: { error?: string; hint?: string } | null = null
  try { payload = text ? JSON.parse(text) : null } catch { /* corpo não-JSON */ }
  return new ApiError(payload?.error ?? `${res.status} ${res.statusText}`, res.status, payload?.hint)
}

class HttpClient implements ApiClient {
  constructor(private readonly base: string) {}

  async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(this.base + path, {
      method,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    if (!res.ok) throw await failure(res)
    const text = await res.text()
    if (!text) return null as T
    try {
      return JSON.parse(text) as T
    } catch {
      return text as T
    }
  }

  async fetchBlob(path: string): Promise<Blob> {
    const res = await fetch(this.resourceUrl(path))
    if (!res.ok) throw await failure(res)
    return res.blob()
  }

  resourceUrl(path: string): string {
    if (/^[a-z][a-z0-9+.-]*:/i.test(path)) return path
    return new URL(this.base + path, document.baseURI).href
  }
}

/** Eventos do servidor via WebSocket, com reconexão e backoff. */
class WebSocketEvents implements EventStream {
  private socket: WebSocket | null = null
  private handlers = new Set<(event: ServerEvent) => void>()
  private reconnectDelay = 500

  constructor(private readonly url: () => string) {}

  subscribe(handler: (event: ServerEvent) => void): () => void {
    this.handlers.add(handler)
    this.ensureSocket()
    return () => {
      this.handlers.delete(handler)
      if (this.handlers.size === 0) {
        const socket = this.socket
        this.socket = null
        socket?.close()
      }
    }
  }

  private ensureSocket() {
    if (this.handlers.size === 0) return
    if (this.socket && this.socket.readyState <= WebSocket.OPEN) return
    const socket = new WebSocket(this.url())
    this.socket = socket
    // Callbacks de um socket substituído (ex.: montagem dupla do StrictMode)
    // são ignorados: sem isso, o fechamento tardio do antigo apagava a
    // referência ao novo e abria uma terceira conexão.
    const current = () => socket === this.socket

    socket.onopen = () => {
      if (!current()) return
      this.reconnectDelay = 500
      this.emit({ type: 'connection', source: 'ui', message: 'conectado', at: new Date().toISOString() })
    }
    socket.onmessage = (ev) => {
      if (!current()) return
      try {
        this.emit(JSON.parse(ev.data as string) as ServerEvent)
      } catch {
        /* mensagem malformada é ignorada de propósito */
      }
    }
    socket.onclose = () => {
      if (!current()) return
      this.socket = null
      this.emit({ type: 'disconnection', source: 'ui', message: 'reconectando…', at: new Date().toISOString() })
      // Backoff exponencial limitado: a reconexão precisa ser rápida em
      // desenvolvimento, mas não pode inundar o servidor se ele caiu de vez.
      setTimeout(() => this.ensureSocket(), this.reconnectDelay)
      this.reconnectDelay = Math.min(this.reconnectDelay * 2, 8000)
    }
  }

  private emit(event: ServerEvent) {
    for (const handler of this.handlers) handler(event)
  }
}

function webSocketUrl(): string {
  const url = new URL(`${API_BASE}/ws`, document.baseURI)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.href
}

/* Composição ------------------------------------------------------------------ */

export const client: ApiClient = new HttpClient(API_BASE)

function createEventStream(): EventStream {
  // App desktop: o Wails não faz upgrade de WebSocket no handler de assets, então
  // o Go repassa o barramento como eventos do runtime (desktop/main.go).
  if (HOST === 'desktop') return { subscribe: onServerEvent }
  return new WebSocketEvents(webSocketUrl)
}

export const events: EventStream = createEventStream()
