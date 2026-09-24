// Ponte com o app desktop (Wails v3): o serviço Go `Desktop` (desktop/service.go)
// e os eventos do runtime.
//
// O runtime do Wails só é carregado dentro do app desktop (import dinâmico):
// no navegador este módulo nunca é executado e o bundle web não muda.

import type { ServerEvent } from './types'

type Runtime = typeof import('@wailsio/runtime')

let runtime: Promise<Runtime> | null = null
const load = () => (runtime ??= import('@wailsio/runtime'))

/** Nome do evento com que o Go repassa o barramento (desktop/main.go). */
const SERVER_EVENT = 'archcode:event'

export interface ProjectInfo { root: string; name: string }
export interface RecentProject { root: string; name: string; opened_at: string }

async function call<T>(method: string, ...args: unknown[]): Promise<T> {
  const { Call } = await load()
  return (await Call.ByName(`main.Desktop.${method}`, ...args)) as T
}

/** Métodos do serviço Go `Desktop`. */
export const desktop = {
  currentProject: () => call<ProjectInfo | null>('CurrentProject'),
  recentProjects: () => call<RecentProject[]>('RecentProjects'),
  forgetProject: (root: string) => call<void>('ForgetProject', root),
  isProject: (dir: string) => call<boolean>('IsProject', dir),
  /** Seletor de pastas nativo; "" quando o usuário cancela. */
  chooseFolder: (title: string) => call<string>('ChooseFolder', title),
  openProject: (dir: string) => call<ProjectInfo>('OpenProject', dir),
  createProject: (dir: string, name: string) => call<ProjectInfo>('CreateProject', dir, name),
  closeProject: () => call<void>('CloseProject'),
  /** Diálogo "Salvar como" nativo; devolve o caminho gravado ("" = cancelado). */
  saveFile: async (filename: string, data: Blob) => call<string>('SaveFile', filename, await toBase64(data)),
  printHtml: (title: string, html: string) => call<void>('PrintHTML', title, html),
  copyText: async (text: string) => { await (await load()).Clipboard.SetText(text) },
  openExternal: async (url: string) => { await (await load()).Browser.OpenURL(url) },
}

/** Assina os eventos do servidor repassados pelo Go. */
export function onServerEvent(handler: (event: ServerEvent) => void): () => void {
  let off: (() => void) | null = null
  let cancelled = false
  void load().then(({ Events }) => {
    if (cancelled) return
    off = Events.On(SERVER_EVENT, (ev) => handler(ev.data as ServerEvent))
    // Mesmo contrato do WebSocket: "connection" faz o projeto recarregar o
    // snapshot, cobrindo o intervalo entre a carga inicial e a assinatura.
    handler({ type: 'connection', source: 'ui', message: 'conectado', at: new Date().toISOString() })
  })
  return () => {
    cancelled = true
    off?.()
  }
}

/**
 * Links externos (ex.: no Markdown renderizado) abrem no navegador do sistema,
 * não dentro da janela do app.
 */
export function installExternalLinks() {
  document.addEventListener('click', (e) => {
    const anchor = (e.target as HTMLElement | null)?.closest?.('a[href]') as HTMLAnchorElement | null
    if (!anchor || !/^https?:/i.test(anchor.href) || new URL(anchor.href).origin === location.origin) return
    e.preventDefault()
    void desktop.openExternal(anchor.href)
  })
}

// Blob → base64 (o runtime entrega string base64 ao Go, decodificada em []byte).
async function toBase64(data: Blob): Promise<string> {
  const bytes = new Uint8Array(await data.arrayBuffer())
  let binary = ''
  for (let i = 0; i < bytes.length; i += 0x8000) {
    binary += String.fromCharCode(...bytes.subarray(i, i + 0x8000))
  }
  return btoa(binary)
}
