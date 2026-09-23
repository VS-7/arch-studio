// Camada de abstração de transporte (RNF005).
//
// Toda a aplicação fala com o núcleo Go exclusivamente através desta interface.
// Hoje existe a implementação HTTP/WebSocket, usada no browser. Ao empacotar com
// Wails v3, basta registrar uma implementação de IPC (`window.go.*`) aqui: nenhum
// componente de UI precisa ser tocado.

import type { ServerEvent } from './types'

export interface Transport {
  readonly kind: 'http' | 'ipc'
  request<T>(method: string, path: string, body?: unknown): Promise<T>
  subscribe(handler: (event: ServerEvent) => void): () => void
  /** URL absoluta de um recurso do servidor (usada por downloads e <img>). */
  resourceUrl(path: string): string
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly hint?: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

class HttpTransport implements Transport {
  readonly kind = 'http' as const

  private socket: WebSocket | null = null
  private handlers = new Set<(event: ServerEvent) => void>()
  private reconnectDelay = 500
  private closed = false

  async request<T>(method: string, path: string, body?: unknown): Promise<T> {
    const res = await fetch(path, {
      method,
      headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    })
    const text = await res.text()
    let parsed: unknown = null
    if (text) {
      try {
        parsed = JSON.parse(text)
      } catch {
        parsed = text
      }
    }
    if (!res.ok) {
      const payload = parsed as { error?: string; hint?: string } | null
      throw new ApiError(payload?.error ?? `${res.status} ${res.statusText}`, res.status, payload?.hint)
    }
    return parsed as T
  }

  resourceUrl(path: string): string {
    return path
  }

  subscribe(handler: (event: ServerEvent) => void): () => void {
    this.handlers.add(handler)
    this.ensureSocket()
    return () => {
      this.handlers.delete(handler)
      if (this.handlers.size === 0) {
        this.closed = true
        this.socket?.close()
        this.socket = null
      }
    }
  }

  private ensureSocket() {
    if (this.socket && this.socket.readyState <= WebSocket.OPEN) return
    this.closed = false
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const socket = new WebSocket(`${proto}://${location.host}/ws`)
    this.socket = socket

    socket.onopen = () => {
      this.reconnectDelay = 500
      this.emit({ type: 'connection', source: 'ui', message: 'conectado', at: new Date().toISOString() })
    }
    socket.onmessage = (ev) => {
      try {
        this.emit(JSON.parse(ev.data as string) as ServerEvent)
      } catch {
        /* mensagem malformada é ignorada de propósito */
      }
    }
    socket.onclose = () => {
      this.socket = null
      if (this.closed || this.handlers.size === 0) return
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

/**
 * Detecta o runtime do Wails v3. Quando presente, uma implementação IPC pode ser
 * registrada aqui sem alterar nenhum componente de interface.
 */
function detectTransport(): Transport {
  const wails = (globalThis as Record<string, unknown>).wails
  if (wails) {
    // Ponto de extensão da Fase 5: `new IpcTransport()`.
    // Enquanto os bindings não existem, o Wails serve o mesmo backend HTTP local.
    return new HttpTransport()
  }
  return new HttpTransport()
}

export const transport: Transport = detectTransport()
