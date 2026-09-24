// Exportação do Documento de Requisitos para .docx (Word/LibreOffice/Google Docs).
//
// Constrói o arquivo a partir dos mesmos blocos da pré-visualização, com a
// mesma identidade visual (THEME): capa em seção própria com faixa de
// identidade, páginas de conteúdo com cabeçalho e rodapé "Página N de M",
// títulos numerados em azul-marinho e tabelas com cabeçalho sombreado. Os
// diagramas chegam do servidor como SVG e são rasterizados em PNG no navegador
// (o Word não aceita SVG sem uma versão PNG de apoio). A biblioteca `docx` é
// carregada sob demanda para não pesar no carregamento do Studio.

import { api } from '../../../lib/api'
import type { DocBlock, PriorityLevel, ReqDocument } from '../../../lib/types'
import { parseInline, type InlineRun } from './inline'
import { BODY_MM, mmToPx, mmToTwip, THEME } from './theme'

type Docx = typeof import('docx')
type Block = import('docx').Paragraph | import('docx').Table

const { font: FONT, color: C, size: S, page: P } = THEME
/** Tamanho em meios-pontos, unidade das fontes no DOCX. */
const pt = (points: number) => Math.round(points * 2)
/** Largura útil da página em pixels (96 dpi), limite das figuras. */
const MAX_IMAGE_WIDTH = Math.floor(mmToPx(BODY_MM.width))
const MAX_IMAGE_HEIGHT = Math.floor(mmToPx(190))

/** Ids de marcador do Word: começam com letra, só [A-Za-z0-9_], até 40 caracteres. */
function bookmarkId(anchor: string): string {
  return ('b_' + anchor.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/[^A-Za-z0-9]+/g, '_')).slice(0, 40)
}

function runs(d: Docx, text: string, base: { bold?: boolean; size?: number; color?: string } = {}) {
  return parseInline(text).map((r: InlineRun) => {
    const run = new d.TextRun({
      text: r.text,
      bold: base.bold || r.bold,
      italics: r.italic,
      font: r.code ? 'Courier New' : FONT,
      size: base.size,
      color: base.color,
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

const BORDER = { style: 'single', size: 6, color: C.tableBorder } as const

function table(d: Docx, header: string[], rows: string[][]) {
  const cell = (text: string, head: boolean) => new d.TableCell({
    children: [new d.Paragraph({ children: runs(d, text, { bold: head, size: pt(S.table), color: head ? C.primary : undefined }) })],
    shading: head ? { fill: C.tableHead, type: d.ShadingType.CLEAR, color: 'auto' } : undefined,
    margins: { top: 50, bottom: 50, left: 100, right: 100 },
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

/** "Figura 3 – Nome": rótulo em destaque, como na prévia. */
function captionRuns(d: Docx, caption: string) {
  const m = /^(Figura\s+\d+)(\s+[–-]\s+)(.*)$/s.exec(caption)
  const size = pt(S.small)
  if (!m) return runs(d, caption, { size, color: C.muted })
  return [
    new d.TextRun({ text: m[1], bold: true, size, color: C.primary, font: FONT }),
    new d.TextRun({ text: m[2], size, color: C.muted, font: FONT }),
    ...runs(d, m[3], { size, color: C.muted }),
  ]
}

/** Parágrafo-rótulo ("**Fluxo de eventos principal**"), que fica junto da lista seguinte. */
const isLabel = (text: string) => /^\*\*[^*]+\*\*:?$/.test(text.trim())

async function blockToDocx(d: Docx, b: DocBlock, out: Block[]) {
  switch (b.type) {
    case 'heading': {
      const levels = [d.HeadingLevel.HEADING_1, d.HeadingLevel.HEADING_2, d.HeadingLevel.HEADING_3, d.HeadingLevel.HEADING_4]
      const text = `${b.number ? `${b.number} ` : ''}${b.text}`
      out.push(new d.Paragraph({
        heading: levels[b.level - 1],
        keepNext: true,
        border: b.level === 1 ? { bottom: { style: d.BorderStyle.SINGLE, size: 12, color: C.primary, space: 4 } } : undefined,
        children: [new d.Bookmark({ id: bookmarkId(b.id), children: runs(d, text) })],
      }))
      return
    }
    case 'paragraph':
      out.push(new d.Paragraph({
        alignment: d.AlignmentType.JUSTIFIED, spacing: { after: 120 }, keepNext: isLabel(b.text) || undefined,
        children: runs(d, b.text),
      }))
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
          spacing: { before: 120 },
          children: [new d.ImageRun({ type: 'png', data: png.data, transformation: size })],
        }))
      }
      out.push(new d.Paragraph({
        alignment: d.AlignmentType.CENTER, spacing: { before: 80, after: 240 },
        children: captionRuns(d, b.caption),
      }))
      return
    }
    case 'pagebreak':
      out.push(new d.Paragraph({ children: [new d.PageBreak()] }))
  }
}

/** Capa: faixa de identidade, título, projeto, versão, data e autores. */
function cover(d: Docx, doc: ReqDocument): Block[] {
  const none = { style: d.BorderStyle.NONE, size: 0, color: 'FFFFFF' }
  const bandRun = (text: string) => new d.TextRun({
    text: text.toUpperCase(), bold: true, color: C.bandText, size: pt(S.coverBand), font: FONT, characterSpacing: 40,
  })
  const band = new d.Table({
    width: { size: 100, type: d.WidthType.PERCENTAGE },
    borders: { top: none, bottom: none, left: none, right: none, insideHorizontal: none, insideVertical: none },
    rows: [new d.TableRow({ children: [new d.TableCell({
      shading: { fill: C.band, type: d.ShadingType.CLEAR, color: 'auto' },
      margins: { top: 160, bottom: 160, left: 240, right: 240 },
      children: [new d.Paragraph({
        tabStops: [{ type: d.TabStopType.RIGHT, position: d.TabStopPosition.MAX }],
        children: [bandRun(doc.project), new d.TextRun({ text: '\t', size: pt(S.coverBand) }), bandRun(doc.title)],
      })],
    })] })],
  })
  const center = (children: import('docx').TextRun[], spacing: { before?: number; after?: number }) =>
    new d.Paragraph({ alignment: d.AlignmentType.CENTER, spacing, children })
  const label = (text: string) => new d.TextRun({
    text: text.toUpperCase(), size: pt(S.small), color: C.muted, font: FONT, characterSpacing: 25,
  })
  const value = (text: string, bold = false) => new d.TextRun({ text, size: pt(S.coverMeta), bold, font: FONT })

  return [
    band,
    center([new d.TextRun({ text: doc.title, bold: true, size: pt(S.coverTitle), color: C.primary, font: FONT })], { before: 3400, after: 240 }),
    // Filete de 60 mm centralizado, como na prévia.
    new d.Paragraph({
      spacing: { after: 240 },
      indent: { left: mmToTwip((BODY_MM.width - 60) / 2), right: mmToTwip((BODY_MM.width - 60) / 2) },
      border: { bottom: { style: d.BorderStyle.SINGLE, size: 12, color: C.accent, space: 1 } },
      children: [],
    }),
    center([new d.TextRun({ text: doc.project, size: pt(S.coverProject), color: C.accent, font: FONT })], { after: 2000 }),
    center([label('Versão')], { after: 40 }),
    center([value(doc.version)], { after: 280 }),
    center([label('Data')], { after: 40 }),
    center([value(doc.date)], { after: 1300 }),
    ...(doc.authors.length ? [
      center([label(doc.authors.length > 1 ? 'Autores' : 'Autor')], { after: 40 }),
      ...doc.authors.map((a) => center([value(a.toUpperCase(), true)], { after: 60 })),
    ] : []),
  ]
}

export async function buildDocx(doc: ReqDocument): Promise<Blob> {
  const d = await import('docx')
  const plainTitle = (text: string) => new d.Paragraph({
    alignment: d.AlignmentType.CENTER, spacing: { after: 240 }, keepNext: true,
    children: [new d.TextRun({ text, size: pt(S.plainTitle), bold: true, color: C.primary, font: FONT })],
  })
  const running = (size: number) => ({ size: pt(size), color: C.muted, font: FONT })

  const header = new d.Header({ children: [new d.Paragraph({
    tabStops: [{ type: d.TabStopType.RIGHT, position: d.TabStopPosition.MAX }],
    border: { bottom: { style: d.BorderStyle.SINGLE, size: 4, color: C.rule, space: 4 } },
    children: [
      new d.TextRun({ text: doc.title, bold: true, ...running(S.running), color: C.primary }),
      new d.TextRun({ text: ` — ${doc.project}\tVersão ${doc.version}`, ...running(S.running) }),
    ],
  })] })
  const footer = new d.Footer({ children: [new d.Paragraph({
    tabStops: [{ type: d.TabStopType.RIGHT, position: d.TabStopPosition.MAX }],
    border: { top: { style: d.BorderStyle.SINGLE, size: 4, color: C.rule, space: 4 } },
    children: [
      new d.TextRun({ text: `${doc.project}\t`, ...running(S.running) }),
      new d.TextRun({ children: ['Página ', d.PageNumber.CURRENT, ' de ', d.PageNumber.TOTAL_PAGES], ...running(S.running) }),
    ],
  })] })

  const body: Block[] = [
    plainTitle('Histórico de Alterações'),
    table(d, ['Data', 'Versão', 'Descrição', 'Autor'], doc.history.map((h) => [h.date, h.version, h.description, h.author])),
    new d.Paragraph({ children: [new d.PageBreak()] }),
    plainTitle('Sumário'),
    // Sumário estático com links internos: não depende de "atualizar campos" no Word.
    ...doc.toc.map((t) => new d.Paragraph({
      indent: { left: (t.level - 1) * 360 },
      spacing: { after: 40, before: t.level === 1 ? 120 : 0 },
      children: [new d.InternalHyperlink({
        anchor: bookmarkId(t.id),
        children: [new d.TextRun({
          text: `${t.number}  ${t.title}`, bold: t.level === 1, color: t.level === 1 ? C.primary : undefined, font: FONT,
        })],
      })],
    })),
  ]
  for (const block of doc.blocks) await blockToDocx(d, block, body)

  const margin = {
    top: mmToTwip(P.top), bottom: mmToTwip(P.bottom), left: mmToTwip(P.left), right: mmToTwip(P.right),
    header: mmToTwip(P.header), footer: mmToTwip(P.footer),
  }
  const headingStyle = (id: string, name: string, size: number, color: string, before: number, after: number, outlineLevel: number) => ({
    id, name, basedOn: 'Normal', next: 'Normal', quickFormat: true,
    run: { size: pt(size), bold: true, color, font: FONT },
    paragraph: { spacing: { before, after }, keepNext: true, outlineLevel },
  })

  const file = new d.Document({
    creator: doc.authors.join(', ') || 'ArchCode Studio',
    title: `${doc.title} - ${doc.project}`,
    description: 'Gerado pelo ArchCode Studio',
    styles: {
      default: { document: { run: { font: FONT, size: pt(S.body), color: C.text }, paragraph: { spacing: { line: Math.round(THEME.lineHeight * 240) } } } },
      paragraphStyles: [
        headingStyle('Heading1', 'Heading 1', S.h1, C.primary, 0, 240, 0),
        headingStyle('Heading2', 'Heading 2', S.h2, C.accent, 280, 120, 1),
        headingStyle('Heading3', 'Heading 3', S.h3, C.accent, 240, 100, 2),
        headingStyle('Heading4', 'Heading 4', S.h4, C.primary, 200, 80, 3),
      ],
    },
    sections: [
      // Capa: seção própria, com margens de capa e sem cabeçalho nem rodapé.
      {
        properties: { page: { margin: { ...margin, top: mmToTwip(P.cover.band), left: mmToTwip(P.cover.inset), right: mmToTwip(P.cover.inset) } } },
        children: cover(d, doc),
      },
      // Conteúdo: começa em página nova, numeração contínua (a capa é a página 1, sem número).
      {
        properties: { type: d.SectionType.NEXT_PAGE, page: { margin } },
        headers: { default: header }, footers: { default: footer }, children: body,
      },
    ],
  })
  return d.Packer.toBlob(file)
}
