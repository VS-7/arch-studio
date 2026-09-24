// Exportação do Documento de Requisitos para .docx (Word/LibreOffice/Google Docs).
//
// Constrói o arquivo a partir dos mesmos blocos da pré-visualização. Os
// diagramas chegam do servidor como SVG e são rasterizados em PNG no navegador
// (o Word não aceita SVG sem uma versão PNG de apoio). A biblioteca `docx` é
// carregada sob demanda para não pesar no carregamento do Studio.

import { api } from '../../../lib/api'
import type { DocBlock, PriorityLevel, ReqDocument } from '../../../lib/types'
import { parseInline, type InlineRun } from './inline'

type Docx = typeof import('docx')
type Block = import('docx').Paragraph | import('docx').Table

const FONT = 'Arial'
/** Largura útil da página A4 com margens de 2,2 cm, em pixels (96 dpi). */
const MAX_IMAGE_WIDTH = 620
const MAX_IMAGE_HEIGHT = 820

/** Ids de marcador do Word: começam com letra, só [A-Za-z0-9_], até 40 caracteres. */
function bookmarkId(anchor: string): string {
  return ('b_' + anchor.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^A-Za-z0-9]+/g, '_')).slice(0, 40)
}

function runs(d: Docx, text: string, base: { bold?: boolean; size?: number } = {}) {
  return parseInline(text).map((r: InlineRun) => {
    const run = new d.TextRun({
      text: r.text,
      bold: base.bold || r.bold,
      italics: r.italic,
      font: r.code ? 'Courier New' : FONT,
      size: base.size,
    })
    return r.anchor ? new d.InternalHyperlink({ anchor: bookmarkId(r.anchor), children: [run] }) : run
  })
}

async function svgToPng(src: string): Promise<{ data: ArrayBuffer; width: number; height: number } | null> {
  try {
    const svg = await (await api.fetchFigure(src)).text()
    const url = URL.createObjectURL(new Blob([svg], { type: 'image/svg+xml' }))
    const img = new Image()
    await new Promise<void>((resolve, reject) => { img.onload = () => resolve(); img.onerror = () => reject(new Error('svg')); img.src = url })
    const scale = 2
    const w = img.naturalWidth || 800
    const h = img.naturalHeight || 600
    const canvas = document.createElement('canvas')
    canvas.width = w * scale
    canvas.height = h * scale
    const ctx = canvas.getContext('2d')!
    ctx.fillStyle = '#ffffff'
    ctx.fillRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
    URL.revokeObjectURL(url)
    const blob = await new Promise<Blob | null>((r) => canvas.toBlob(r, 'image/png'))
    if (!blob) return null
    return { data: await blob.arrayBuffer(), width: w, height: h }
  } catch {
    return null
  }
}

function fit(width: number, height: number) {
  const k = Math.min(1, MAX_IMAGE_WIDTH / width, MAX_IMAGE_HEIGHT / height)
  return { width: Math.round(width * k), height: Math.round(height * k) }
}

const BORDER = { style: 'single', size: 4, color: '9A9A9A' } as const

function table(d: Docx, header: string[], rows: string[][]) {
  const cell = (text: string, head: boolean) => new d.TableCell({
    children: [new d.Paragraph({ children: runs(d, text, { bold: head, size: 20 }) })],
    shading: head ? { fill: 'EFEFEF', type: d.ShadingType.CLEAR, color: 'auto' } : undefined,
    margins: { top: 60, bottom: 60, left: 100, right: 100 },
  })
  return new d.Table({
    width: { size: 100, type: d.WidthType.PERCENTAGE },
    borders: { top: BORDER, bottom: BORDER, left: BORDER, right: BORDER, insideHorizontal: BORDER, insideVertical: BORDER },
    rows: [
      new d.TableRow({ tableHeader: true, children: header.map((h) => cell(h, true)) }),
      ...rows.map((r) => new d.TableRow({ children: r.map((c) => cell(c, false)) })),
    ],
  })
}

function priority(d: Docx, level: PriorityLevel) {
  const none = { style: d.BorderStyle.NONE, size: 0, color: 'FFFFFF' }
  const items: [PriorityLevel, string][] = [['essencial', 'Essencial'], ['importante', 'Importante'], ['desejavel', 'Desejável']]
  const cell = (text: string, bold = false) => new d.TableCell({
    borders: { top: none, bottom: none, left: none, right: none },
    children: [new d.Paragraph({ children: [new d.TextRun({ text, bold, font: FONT })] })],
    margins: { right: 120 },
  })
  return new d.Table({
    borders: { top: none, bottom: none, left: none, right: none, insideHorizontal: none, insideVertical: none },
    rows: [new d.TableRow({
      children: [cell('Prioridade:', true), ...items.flatMap(([v, label]) => [cell(v === level ? '■' : '◻'), cell(label)])],
    })],
  })
}

async function blockToDocx(d: Docx, b: DocBlock, out: Block[]) {
  switch (b.type) {
    case 'heading': {
      const levels = [d.HeadingLevel.HEADING_1, d.HeadingLevel.HEADING_2, d.HeadingLevel.HEADING_3, d.HeadingLevel.HEADING_4]
      const text = `${b.number ? `${b.number} ` : ''}${b.text}`
      out.push(new d.Paragraph({
        heading: levels[b.level - 1],
        keepNext: true,
        children: [new d.Bookmark({ id: bookmarkId(b.id), children: runs(d, text) })],
      }))
      return
    }
    case 'paragraph':
      out.push(new d.Paragraph({ alignment: d.AlignmentType.JUSTIFIED, spacing: { after: 120 }, children: runs(d, b.text) }))
      return
    case 'list':
      // Numeração explícita: preserva a continuidade dos passos através dos subtítulos.
      b.items.forEach((item, i) => {
        const marker = b.ordered ? `${(b.start ?? 1) + i}.` : '•'
        out.push(new d.Paragraph({
          indent: { left: 540, hanging: 300 },
          spacing: { after: 60 },
          children: [new d.TextRun({ text: `${marker}\t`, font: FONT }), ...runs(d, item)],
          tabStops: [{ type: d.TabStopType.LEFT, position: 540 }],
        }))
      })
      return
    case 'table':
      out.push(table(d, b.header, b.rows))
      out.push(new d.Paragraph({ children: [] }))
      return
    case 'priority':
      out.push(priority(d, b.value))
      out.push(new d.Paragraph({ children: [] }))
      return
    case 'image': {
      const png = await svgToPng(b.src)
      if (png) {
        const size = fit(png.width, png.height)
        out.push(new d.Paragraph({
          alignment: d.AlignmentType.CENTER,
          keepNext: true,
          children: [new d.ImageRun({ type: 'png', data: png.data, transformation: size })],
        }))
      }
      out.push(new d.Paragraph({
        alignment: d.AlignmentType.CENTER, spacing: { after: 200 },
        children: runs(d, b.caption, { size: 18 }),
      }))
      return
    }
    case 'pagebreak':
      out.push(new d.Paragraph({ children: [new d.PageBreak()] }))
  }
}

export async function buildDocx(doc: ReqDocument): Promise<Blob> {
  const d = await import('docx')
  const center = (text: string, size: number, bold = false, after = 80) => new d.Paragraph({
    alignment: d.AlignmentType.CENTER, spacing: { after },
    children: [new d.TextRun({ text, size, bold, font: FONT })],
  })

  const children: Block[] = [
    new d.Paragraph({ spacing: { before: 3600 }, children: [] }),
    center(doc.title, 52, true, 120),
    center(doc.project, 40, false, 2400),
    center(doc.date, 24),
    center(`Versão ${doc.version}`, 24, false, 1200),
    ...doc.authors.map((a) => center(a.toUpperCase(), 24, true)),
    new d.Paragraph({ children: [new d.PageBreak()] }),
    center('Histórico de Alterações', 28, true, 240),
    table(d, ['Data', 'Versão', 'Descrição', 'Autor'], doc.history.map((h) => [h.date, h.version, h.description, h.author])),
    new d.Paragraph({ children: [new d.PageBreak()] }),
    center('Conteúdo', 28, true, 240),
    // Sumário estático com links internos: não depende de "atualizar campos" no Word.
    ...doc.toc.map((t) => new d.Paragraph({
      indent: { left: (t.level - 1) * 360 },
      spacing: { after: 40, before: t.level === 1 ? 120 : 0 },
      children: [new d.InternalHyperlink({
        anchor: bookmarkId(t.id),
        children: [new d.TextRun({ text: `${t.number}  ${t.title}`, bold: t.level === 1, font: FONT })],
      })],
    })),
  ]
  for (const block of doc.blocks) await blockToDocx(d, block, children)

  const file = new d.Document({
    creator: doc.authors.join(', ') || 'ArchCode Studio',
    title: `${doc.title} - ${doc.project}`,
    description: 'Gerado pelo ArchCode Studio',
    styles: {
      default: { document: { run: { font: FONT, size: 22 }, paragraph: { spacing: { line: 300 } } } },
      paragraphStyles: [
        { id: 'Heading1', name: 'Heading 1', basedOn: 'Normal', next: 'Normal', quickFormat: true, run: { size: 32, bold: true, font: FONT }, paragraph: { spacing: { before: 360, after: 160 } } },
        { id: 'Heading2', name: 'Heading 2', basedOn: 'Normal', next: 'Normal', quickFormat: true, run: { size: 27, bold: true, font: FONT }, paragraph: { spacing: { before: 300, after: 120 } } },
        { id: 'Heading3', name: 'Heading 3', basedOn: 'Normal', next: 'Normal', quickFormat: true, run: { size: 24, bold: true, font: FONT }, paragraph: { spacing: { before: 240, after: 100 } } },
        { id: 'Heading4', name: 'Heading 4', basedOn: 'Normal', next: 'Normal', quickFormat: true, run: { size: 22, bold: true, font: FONT }, paragraph: { spacing: { before: 200, after: 80 } } },
      ],
    },
    sections: [{
      properties: { page: { margin: { top: 1418, bottom: 1418, left: 1247, right: 1247 } } },
      children,
    }],
  })
  return d.Packer.toBlob(file)
}
