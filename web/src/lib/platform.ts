// Recursos do sistema usados pela interface: salvar arquivo, imprimir e copiar.
//
// No navegador, salvar é um <a download> e imprimir usa um iframe oculto.
// Webviews embutidas tratam downloads e janelas de impressão de forma irregular,
// então o app desktop troca essas implementações por diálogos nativos — num
// único lugar, a escolha no fim do arquivo. Os chamadores não mudam.

import { desktop } from './desktop'
import { HOST } from './host'

export interface Platform {
  saveFile(data: Blob, filename: string): Promise<void>
  /** Imprime um documento HTML completo (o usuário pode salvar como PDF). */
  printHtml(html: string): Promise<void>
  copyText(text: string): Promise<void>
}

const browser: Platform = {
  async saveFile(data, filename) {
    const url = URL.createObjectURL(data)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = filename
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    // Revoga depois do clique para não cancelar o download em navegadores lentos.
    setTimeout(() => URL.revokeObjectURL(url), 4000)
  },

  async printHtml(html) {
    const iframe = document.createElement('iframe')
    iframe.style.cssText = 'position:fixed;right:0;bottom:0;width:0;height:0;border:0;visibility:hidden'
    document.body.appendChild(iframe)
    const w = iframe.contentWindow!
    w.document.open()
    w.document.write(html)
    w.document.close()
    // Espera as figuras carregarem antes de abrir o diálogo de impressão.
    await Promise.all([...w.document.images].map((img) =>
      img.complete ? null : new Promise((resolve) => { img.onload = resolve; img.onerror = resolve })))
    w.focus()
    w.print()
    setTimeout(() => iframe.remove(), 60_000)
  },

  async copyText(text) {
    await navigator.clipboard.writeText(text)
  },
}

/** App desktop: "Salvar como" e impressão nativos, área de transferência do sistema. */
const native: Platform = {
  async saveFile(data, filename) {
    await desktop.saveFile(filename, data)
  },
  async printHtml(html) {
    const title = /<title>([^<]*)<\/title>/i.exec(html)?.[1]?.trim() || 'Documento'
    await desktop.printHtml(title, html)
  },
  async copyText(text) {
    await desktop.copyText(text)
  },
}

function createPlatform(): Platform {
  return HOST === 'desktop' ? native : browser
}

export const platform: Platform = createPlatform()
