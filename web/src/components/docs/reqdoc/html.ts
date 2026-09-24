// Renderização do Documento de Requisitos em HTML com aparência de documento
// impresso: folhas A4 reais (capa própria, depois páginas com cabeçalho e
// rodapé numerado), tipografia e cores de THEME. O HTML paginado alimenta a
// pré-visualização e a impressão em PDF, então o que se vê é o que se exporta.
//
// A paginação (medir blocos e distribuí-los pelas folhas) fica em paginate.ts;
// este módulo só sabe desenhar: CSS, capa, cabeçalho/rodapé e cada bloco.

import { platform } from '../../../lib/platform'
import type { DocBlock, PriorityLevel, ReqDocument } from '../../../lib/types'
import { escapeHtml, inlineHtml } from './inline'
import { BODY_MM, THEME } from './theme'

const PRIORITIES: [PriorityLevel, string][] = [['essencial', 'Essencial'], ['importante', 'Importante'], ['desejavel', 'Desejável']]

const c = (hex: string) => `#${hex}`
const { size, color, page } = THEME

export const DOC_CSS = `
.rd { font-family: ${THEME.fontStack}; font-size: ${size.body}pt; line-height: ${THEME.lineHeight}; color: ${c(color.text)};
  -webkit-print-color-adjust: exact; print-color-adjust: exact; }
.rd * { box-sizing: border-box; }

/* Folha A4: margens fixas, corpo com altura útil, cabeçalho e rodapé absolutos. */
.rd .sheet { position: relative; width: ${page.width}mm; height: ${page.height}mm; overflow: hidden; background: #fff;
  padding: ${page.top}mm ${page.right}mm ${page.bottom}mm ${page.left}mm; }
.rd .sheet .body { width: ${BODY_MM.width}mm; height: ${BODY_MM.height}mm; overflow: hidden; }
.rd .hdr, .rd .ftr { position: absolute; left: ${page.left}mm; right: ${page.right}mm; display: flex; justify-content: space-between;
  gap: 8mm; font-size: ${size.running}pt; line-height: 1.2; color: ${c(color.muted)}; }
.rd .hdr { top: ${page.header}mm; border-bottom: 0.5pt solid ${c(color.rule)}; padding-bottom: 1.5mm; }
.rd .ftr { bottom: ${page.footer}mm; border-top: 0.5pt solid ${c(color.rule)}; padding-top: 1.5mm; }
.rd .hdr span, .rd .ftr span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rd .hdr .doc { font-weight: 700; color: ${c(color.primary)}; }
.rd .ftr .pg { font-variant-numeric: tabular-nums; }

/* Cada bloco é medido e desenhado dentro de um contêiner próprio (sem colapso
   de margens), para que a paginação bata com a impressão. */
.rd .mi { display: flow-root; }
.rd .body > .mi:first-child > :first-child { margin-top: 0; }

/* Tipografia */
.rd p { margin: 0 0 6pt; text-align: justify; }
.rd h1, .rd h2, .rd h3, .rd h4 { font-weight: 700; line-height: 1.25; text-align: left; }
.rd h1 { font-size: ${size.h1}pt; color: ${c(color.primary)}; margin: 0 0 12pt; padding-bottom: 4pt; border-bottom: 1.5pt solid ${c(color.primary)}; }
.rd h2 { font-size: ${size.h2}pt; color: ${c(color.accent)}; margin: 14pt 0 6pt; }
.rd h3 { font-size: ${size.h3}pt; color: ${c(color.accent)}; margin: 12pt 0 5pt; }
.rd h4 { font-size: ${size.h4}pt; color: ${c(color.primary)}; margin: 10pt 0 4pt; }
.rd .num { margin-right: 8pt; }
.rd ul, .rd ol { margin: 0 0 6pt; padding-left: 20pt; }
.rd li { padding: 1pt 0; }
.rd table { border-collapse: collapse; width: 100%; margin: 4pt 0 10pt; font-size: ${size.table}pt; line-height: 1.3; }
.rd th, .rd td { border: 0.75pt solid ${c(color.tableBorder)}; padding: 3pt 6pt; text-align: left; vertical-align: top; }
.rd th { background: ${c(color.tableHead)}; color: ${c(color.primary)}; font-weight: 700; }
.rd table.prio { width: auto; margin: 2pt 0 6pt; font-size: ${size.body}pt; }
.rd table.prio td { border: none; padding: 1pt 5pt 1pt 0; }
.rd table.prio td.box { font-size: 12pt; padding-right: 3pt; }
.rd figure { margin: 8pt 0 12pt; text-align: center; }
.rd figure img { max-width: 100%; max-height: 190mm; }
.rd figcaption { font-size: ${size.small}pt; color: ${c(color.muted)}; margin-top: 5pt; text-align: center; }
.rd figcaption b { color: ${c(color.primary)}; }
.rd code { font-family: ${THEME.monoStack}; font-size: ${size.table}pt; }
.rd a { color: inherit; text-decoration: none; }
.rd .plain-title { font-size: ${size.plainTitle}pt; font-weight: 700; color: ${c(color.primary)}; text-align: center; margin: 0 0 12pt; }

/* Sumário: número, título, pontilhado e página. */
.rd .toc a { display: flex; align-items: baseline; gap: 4pt; padding: 1.5pt 0; }
.rd .toc .l1 { font-weight: 700; color: ${c(color.primary)}; padding-top: 6pt; }
.rd .toc .l2 { padding-left: 14pt; }
.rd .toc .l3 { padding-left: 28pt; }
.rd .toc .l4 { padding-left: 42pt; }
.rd .toc .n { flex: 0 0 auto; }
.rd .toc .t { flex: 0 1 auto; }
.rd .toc .dots { flex: 1 0 12pt; border-bottom: 1px dotted #999; transform: translateY(-3pt); }
.rd .toc .pg { flex: 0 0 auto; min-width: 18pt; text-align: right; font-variant-numeric: tabular-nums; }

/* Capa: faixas de identidade no topo e na base, conteúdo centralizado. */
.rd .sheet.cover { padding: 0; display: flex; flex-direction: column; }
.rd .cover .band { flex: 0 0 ${page.cover.band}mm; background: ${c(color.band)}; color: ${c(color.bandText)}; display: flex; align-items: center;
  justify-content: space-between; padding: 0 ${page.cover.inset}mm; font-size: ${size.coverBand}pt; font-weight: 700; letter-spacing: 0.18em; text-transform: uppercase; }
.rd .cover .band span { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.rd .cover .foot { flex: 0 0 ${page.cover.foot}mm; background: ${c(color.band)}; }
.rd .cover .cover-body { flex: 1 1 auto; display: flex; flex-direction: column; justify-content: center; align-items: center;
  text-align: center; padding: 0 ${page.cover.inset}mm 20mm; }
.rd .cover .kind { font-size: ${size.coverTitle}pt; font-weight: 700; color: ${c(color.primary)}; line-height: 1.2; margin: 0; text-align: center; }
.rd .cover .rule { width: 60mm; border-top: 1.5pt solid ${c(color.accent)}; margin: 8mm 0; }
.rd .cover .project { font-size: ${size.coverProject}pt; color: ${c(color.accent)}; margin: 0; text-align: center; }
.rd .cover .meta { margin-top: 36mm; font-size: ${size.coverMeta}pt; }
.rd .cover .meta p { margin: 0 0 3pt; text-align: center; }
.rd .cover .lbl { color: ${c(color.muted)}; font-size: ${size.small}pt; text-transform: uppercase; letter-spacing: 0.12em; }
.rd .cover .authors { margin-top: 24mm; font-size: ${size.coverMeta}pt; }
.rd .cover .authors p { margin: 0 0 3pt; text-align: center; font-weight: 700; text-transform: uppercase; }

@media screen {
  .rd .sheet { margin: 0 auto 6mm; box-shadow: 0 1px 3px rgba(0, 0, 0, .25), 0 0 0 1px rgba(0, 0, 0, .06); }
  .rd .sheet:last-child { margin-bottom: 0; }
}
@media print {
  .rd .sheet { margin: 0; box-shadow: none; break-after: page; page-break-after: always; }
  .rd .sheet:last-child { break-after: auto; page-break-after: auto; }
}
`

/** CSS extra da página impressa: folha A4 sem margens do navegador (as margens são das folhas). */
export const PRINT_CSS = `@page { size: ${page.width}mm ${page.height}mm; margin: 0; } html, body { margin: 0; padding: 0; background: #fff; }`

function priorityHtml(level: PriorityLevel): string {
  const cells = PRIORITIES.map(([v, label]) =>
    `<td class="box">${v === level ? '■' : '◻'}</td><td>${label}</td>`).join('')
  return `<table class="prio"><tr><td><strong>Prioridade:</strong></td>${cells}</tr></table>`
}

/** "Figura 3 – Nome" com o rótulo em destaque. */
export function captionHtml(caption: string): string {
  const m = /^(Figura\s+\d+)(\s+[–-]\s+)(.*)$/s.exec(caption)
  if (!m) return inlineHtml(caption)
  return `<b>${escapeHtml(m[1])}</b>${escapeHtml(m[2])}${inlineHtml(m[3])}`
}

export function headingHtml(b: Extract<DocBlock, { type: 'heading' }>): string {
  const tag = `h${b.level}`
  const num = b.number ? `<span class="num">${escapeHtml(b.number)}</span>` : ''
  return `<${tag} id="${escapeHtml(b.id)}">${num}${inlineHtml(b.text)}</${tag}>`
}

export function listItemHtml(item: string): string {
  return `<li>${inlineHtml(item)}</li>`
}

export function listOpen(ordered: boolean, start: number): string {
  if (!ordered) return '<ul>'
  return start > 1 ? `<ol start="${start}">` : '<ol>'
}

export function tableHeadHtml(header: string[]): string {
  return `<thead><tr>${header.map((h) => `<th>${inlineHtml(h)}</th>`).join('')}</tr></thead>`
}

export function tableRowHtml(row: string[]): string {
  return `<tr>${row.map((cell) => `<td>${inlineHtml(cell)}</td>`).join('')}</tr>`
}

export function figureHtml(b: Extract<DocBlock, { type: 'image' }>): string {
  return `<figure><img src="${escapeHtml(b.src)}" alt="${escapeHtml(b.caption)}" /><figcaption>${captionHtml(b.caption)}</figcaption></figure>`
}

/** HTML de um bloco inteiro (sem paginação), usado onde não é preciso dividir. */
export function blockHtml(b: DocBlock): string {
  switch (b.type) {
    case 'heading':
      return headingHtml(b)
    case 'paragraph':
      return `<p>${inlineHtml(b.text)}</p>`
    case 'list':
      return `${listOpen(b.ordered, b.start ?? 1)}${b.items.map(listItemHtml).join('')}${b.ordered ? '</ol>' : '</ul>'}`
    case 'table':
      return `<table>${tableHeadHtml(b.header)}<tbody>${b.rows.map(tableRowHtml).join('')}</tbody></table>`
    case 'priority':
      return priorityHtml(b.value)
    case 'image':
      return figureHtml(b)
    case 'pagebreak':
      return ''
  }
}

export function tocRowHtml(t: ReqDocument['toc'][number]): string {
  return `<a class="l${t.level}" href="#${escapeHtml(t.id)}"><span class="n">${escapeHtml(t.number)}</span>` +
    `<span class="t">${escapeHtml(t.title)}</span><span class="dots"></span><span class="pg" data-pg="${escapeHtml(t.id)}">–</span></a>`
}

/** Capa: folha própria, sem cabeçalho nem rodapé. */
export function coverHtml(doc: ReqDocument): string {
  return `<section class="sheet cover" data-page="1">
    <div class="band"><span>${escapeHtml(doc.project)}</span><span>${escapeHtml(doc.title)}</span></div>
    <div class="cover-body">
      <p class="kind">${escapeHtml(doc.title)}</p>
      <div class="rule"></div>
      <p class="project">${escapeHtml(doc.project)}</p>
      <div class="meta">
        <p><span class="lbl">Versão</span></p><p>${escapeHtml(doc.version)}</p>
        <p style="margin-top:5mm"><span class="lbl">Data</span></p><p>${escapeHtml(doc.date)}</p>
      </div>
      ${doc.authors.length ? `<div class="authors"><p class="lbl" style="font-weight:400">${doc.authors.length > 1 ? 'Autores' : 'Autor'}</p>${doc.authors.map((a) => `<p>${escapeHtml(a)}</p>`).join('')}</div>` : ''}
    </div>
    <div class="foot"></div>
  </section>`
}

/** Folha de conteúdo com cabeçalho (documento, versão) e rodapé (seção, página). */
export function sheetHtml(doc: ReqDocument, body: string, info: { number: number; total: number; section: string }): string {
  return `<section class="sheet" data-page="${info.number}">
    <header class="hdr"><span><span class="doc">${escapeHtml(doc.title)}</span> — ${escapeHtml(doc.project)}</span><span>Versão ${escapeHtml(doc.version)}</span></header>
    <div class="body">${body}</div>
    <footer class="ftr"><span>${escapeHtml(info.section)}</span><span class="pg">Página ${info.number} de ${info.total}</span></footer>
  </section>`
}

/** Abre o diálogo de impressão (Salvar como PDF) com as folhas já paginadas. As
 * figuras chegam com URL absoluta (api.reqDocument), então o iframe não precisa de <base>. */
export function printDocument(doc: ReqDocument, pagesHtml: string): Promise<void> {
  return platform.printHtml(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8">
    <title>${escapeHtml(`${doc.title} - ${doc.project}`)}</title>
    <style>${PRINT_CSS}${DOC_CSS}</style></head><body>${pagesHtml}</body></html>`)
}
