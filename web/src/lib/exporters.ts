// Exportação de alta resolução (RF010).
//
// O SVG é gerado pelo backend Go, então a saída é idêntica no browser, no app
// desktop e na CLI. PNG é rasterizado no cliente a partir desse mesmo SVG; o PDF
// usa a janela de impressão do sistema, que permite escolher tamanho e margens.

import { api } from './api'

export type ExportMode = 'executive' | 'engineering'

function triggerDownload(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  document.body.appendChild(anchor)
  anchor.click()
  anchor.remove()
  // Revoga depois do clique para não cancelar o download em navegadores lentos.
  setTimeout(() => URL.revokeObjectURL(url), 4000)
}

function slugify(value: string): string {
  return value.toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '') || 'arquitetura'
}

export async function exportSvg(projectName: string, mode: ExportMode, dark: boolean) {
  const svg = await api.fetchSvg({ mode, theme: dark ? 'dark' : 'light' })
  triggerDownload(new Blob([svg], { type: 'image/svg+xml' }), `${slugify(projectName)}-arquitetura.svg`)
}

export async function exportPng(projectName: string, mode: ExportMode, options?: { scale?: number; transparent?: boolean; dark?: boolean }) {
  const scale = options?.scale ?? 2
  const svg = await api.fetchSvg({
    mode, theme: options?.dark ? 'dark' : 'light', transparent: options?.transparent,
  })

  const sizes = svg.match(/width="(\d+)"\s+height="(\d+)"/)
  const width = sizes ? Number(sizes[1]) : 1600
  const height = sizes ? Number(sizes[2]) : 900

  const image = new Image()
  const blobUrl = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml;charset=utf-8' }))

  await new Promise<void>((resolve, reject) => {
    image.onload = () => resolve()
    image.onerror = () => reject(new Error('não foi possível rasterizar o SVG'))
    image.src = blobUrl
  })

  const canvas = document.createElement('canvas')
  canvas.width = width * scale
  canvas.height = height * scale
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('canvas indisponível neste navegador')
  ctx.scale(scale, scale)
  ctx.drawImage(image, 0, 0, width, height)
  URL.revokeObjectURL(blobUrl)

  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/png'))
  if (!blob) throw new Error('falha ao gerar PNG')
  triggerDownload(blob, `${slugify(projectName)}-arquitetura.png`)
}

/**
 * Abre uma página de impressão com o diagrama e o sumário executivo, para que o
 * usuário salve como PDF pelo diálogo nativo do sistema.
 */
export async function exportPdf(projectName: string, mode: ExportMode, summary: {
  description?: string
  components: number
  connections: number
  useCases: number
  totalHours?: string
  totalCost?: string
  duration?: string
}) {
  const svg = await api.fetchSvg({ mode, theme: 'light', title: false })
  const win = window.open('', '_blank', 'width=1200,height=860')
  if (!win) throw new Error('o navegador bloqueou a janela de impressão')

  const esc = (s: string) => s.replace(/[<>&]/g, (c) => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[c] ?? c))
  const row = (label: string, value?: string) =>
    value ? `<tr><th>${esc(label)}</th><td>${esc(value)}</td></tr>` : ''

  win.document.write(`<!doctype html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>${esc(projectName)} — Arquitetura</title>
<style>
  @page { size: A4 landscape; margin: 14mm; }
  * { box-sizing: border-box; }
  body { font-family: Inter, system-ui, -apple-system, 'Segoe UI', sans-serif; color: #0f172a; margin: 0; padding: 24px; }
  h1 { font-size: 26px; margin: 0 0 4px; }
  .sub { color: #5b6b86; font-size: 13px; margin: 0 0 20px; }
  .diagram { border: 1px solid #dde4ee; border-radius: 12px; padding: 12px; margin-bottom: 20px; page-break-inside: avoid; }
  .diagram svg { width: 100%; height: auto; }
  table { border-collapse: collapse; width: 100%; max-width: 620px; font-size: 13px; }
  th, td { border: 1px solid #dde4ee; padding: 7px 11px; text-align: left; }
  th { background: #f4f7fb; width: 220px; font-weight: 600; }
  footer { margin-top: 22px; color: #94a3b8; font-size: 11px; }
  @media print { body { padding: 0; } }
</style></head><body>
  <h1>${esc(projectName)}</h1>
  <p class="sub">${esc(summary.description ?? 'Arquitetura de sistema')} — ${mode === 'executive' ? 'visão executiva' : 'visão de engenharia'}</p>
  <div class="diagram">${svg}</div>
  <table>
    ${row('Componentes', String(summary.components))}
    ${row('Integrações', String(summary.connections))}
    ${row('Casos de uso', String(summary.useCases))}
    ${row('Esforço estimado', summary.totalHours)}
    ${row('Investimento estimado', summary.totalCost)}
    ${row('Prazo estimado', summary.duration)}
  </table>
  <footer>Gerado pelo ArchCode Studio em ${new Date().toLocaleDateString('pt-BR')}.</footer>
  <script>window.onload = () => { window.focus(); window.print(); }<\/script>
</body></html>`)
  win.document.close()
}
