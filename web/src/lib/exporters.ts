// Exportação de alta resolução (RF010).
//
// O SVG é gerado pelo backend Go, então a saída é idêntica no browser, no app
// desktop e na CLI. PNG é rasterizado no cliente a partir desse mesmo SVG; o PDF
// usa o diálogo de impressão do sistema, que permite escolher tamanho e margens.

import { api } from './api'
import { slugify } from './format'
import { platform } from './platform'

export type ExportMode = 'executive' | 'engineering'

const fileName = (projectName: string, ext: string) => `${slugify(projectName, 'arquitetura')}-arquitetura.${ext}`

export async function exportSvg(projectName: string, mode: ExportMode, dark: boolean) {
  const svg = await api.exportSvg({ mode, theme: dark ? 'dark' : 'light' })
  await platform.saveFile(new Blob([svg], { type: 'image/svg+xml' }), fileName(projectName, 'svg'))
}

export async function exportPng(projectName: string, mode: ExportMode, options?: { scale?: number; transparent?: boolean; dark?: boolean }) {
  const scale = options?.scale ?? 2
  const svg = await api.exportSvg({
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
  await platform.saveFile(blob, fileName(projectName, 'png'))
}

/**
 * Imprime uma página com o diagrama e o sumário executivo, para que o usuário
 * salve como PDF pelo diálogo nativo do sistema.
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
  const svg = await api.exportSvg({ mode, theme: 'light', title: false })
  const esc = (s: string) => s.replace(/[<>&]/g, (c) => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[c] ?? c))
  const row = (label: string, value?: string) =>
    value ? `<tr><th>${esc(label)}</th><td>${esc(value)}</td></tr>` : ''

  await platform.printHtml(`<!doctype html>
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
</body></html>`)
}
