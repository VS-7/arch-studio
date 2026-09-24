// Renderização do Documento de Requisitos em HTML com aparência de página de
// processador de texto (A4, Arial, margens de 2,5 cm). O mesmo HTML alimenta a
// pré-visualização e a impressão em PDF, então o que se vê é o que se exporta.

import { platform } from '../../../lib/platform'
import type { DocBlock, PriorityLevel, ReqDocument } from '../../../lib/types'
import { escapeHtml, inlineHtml } from './inline'

const PRIORITIES: [PriorityLevel, string][] = [['essencial', 'Essencial'], ['importante', 'Importante'], ['desejavel', 'Desejável']]

export const DOC_CSS = `
.rd { font-family: Arial, 'Liberation Sans', Helvetica, sans-serif; font-size: 11pt; line-height: 1.5; color: #111; background: #fff; }
.rd .page { padding: 2.5cm 2.2cm; }
.rd p { margin: 0 0 8pt; text-align: justify; }
.rd h1, .rd h2, .rd h3, .rd h4 { font-weight: 700; margin: 18pt 0 8pt; line-height: 1.25; page-break-after: avoid; }
.rd h1 { font-size: 16pt; }
.rd h2 { font-size: 13.5pt; }
.rd h3 { font-size: 12pt; }
.rd h4 { font-size: 11pt; }
.rd .num { margin-right: 8pt; }
.rd ul, .rd ol { margin: 0 0 8pt; padding-left: 22pt; }
.rd li { margin: 2pt 0; }
.rd table { border-collapse: collapse; width: 100%; margin: 6pt 0 10pt; font-size: 10pt; page-break-inside: avoid; }
.rd th, .rd td { border: 1px solid #9a9a9a; padding: 4pt 6pt; text-align: left; vertical-align: top; }
.rd th { background: #efefef; font-weight: 700; }
.rd table.prio { width: auto; margin: 4pt 0 8pt; }
.rd table.prio td { border: none; padding: 1pt 5pt 1pt 0; font-size: 11pt; }
.rd table.prio td.box { font-size: 12pt; padding-right: 3pt; }
.rd figure { margin: 10pt 0 14pt; text-align: center; page-break-inside: avoid; }
.rd figure img { max-width: 100%; max-height: 22cm; border: 1px solid #ddd; }
.rd figcaption { font-size: 9.5pt; color: #444; margin-top: 4pt; }
.rd code { font-family: 'Courier New', monospace; font-size: 10pt; }
.rd a { color: #1a1a1a; text-decoration: none; }
.rd .cover { min-height: 24cm; display: flex; flex-direction: column; justify-content: center; text-align: center; }
.rd .cover .kind { font-size: 26pt; font-weight: 700; margin: 0 0 6pt; text-align: center; }
.rd .cover .project { font-size: 20pt; margin: 0 0 90pt; text-align: center; }
.rd .cover p { text-align: center; margin: 0 0 4pt; }
.rd .cover .authors { margin-top: 40pt; font-weight: 700; text-transform: uppercase; }
.rd .plain-title { font-size: 14pt; font-weight: 700; text-align: center; margin: 0 0 12pt; }
.rd .toc a { display: flex; gap: 6pt; padding: 1pt 0; }
.rd .toc .l1 { font-weight: 700; margin-top: 5pt; }
.rd .toc .l2 { padding-left: 14pt; }
.rd .toc .l3 { padding-left: 28pt; }
.rd .toc .l4 { padding-left: 42pt; }
.rd .toc .dots { flex: 1; border-bottom: 1px dotted #999; transform: translateY(-4pt); }
.rd .pb { height: 0; }
@media screen {
  .rd .pb { border-top: 1px dashed #c8c8c8; margin: 28pt -2.2cm; position: relative; }
  .rd .pb::after { content: 'quebra de página'; position: absolute; right: 8px; top: -9px; font-size: 8pt; color: #aaa; background: #fff; padding: 0 4px; }
}
@media print {
  .rd .page { padding: 0; }
  .rd .pb { page-break-after: always; break-after: page; }
}
`

function priorityHtml(level: PriorityLevel): string {
  const cells = PRIORITIES.map(([v, label]) =>
    `<td class="box">${v === level ? '■' : '◻'}</td><td>${label}</td>`).join('')
  return `<table class="prio"><tr><td><strong>Prioridade:</strong></td>${cells}</tr></table>`
}

function blockHtml(b: DocBlock, figure: { n: number }): string {
  switch (b.type) {
    case 'heading': {
      const tag = `h${b.level}`
      const num = b.number ? `<span class="num">${escapeHtml(b.number)}</span>` : ''
      return `<${tag} id="${escapeHtml(b.id)}">${num}${inlineHtml(b.text)}</${tag}>`
    }
    case 'paragraph':
      return `<p>${inlineHtml(b.text)}</p>`
    case 'list': {
      const tag = b.ordered ? 'ol' : 'ul'
      const start = b.ordered && b.start && b.start > 1 ? ` start="${b.start}"` : ''
      return `<${tag}${start}>${b.items.map((it) => `<li>${inlineHtml(it)}</li>`).join('')}</${tag}>`
    }
    case 'table':
      return `<table><thead><tr>${b.header.map((h) => `<th>${inlineHtml(h)}</th>`).join('')}</tr></thead>` +
        `<tbody>${b.rows.map((r) => `<tr>${r.map((c) => `<td>${inlineHtml(c)}</td>`).join('')}</tr>`).join('')}</tbody></table>`
    case 'priority':
      return priorityHtml(b.value)
    case 'image':
      figure.n++
      return `<figure><img src="${escapeHtml(b.src)}" alt="${escapeHtml(b.caption)}" /><figcaption>${inlineHtml(b.caption)}</figcaption></figure>`
    case 'pagebreak':
      return '<div class="pb"></div>'
  }
}

export function documentHtml(doc: ReqDocument): string {
  const cover = `<section class="cover">
    <p class="kind">${escapeHtml(doc.title)}</p>
    <p class="project">${escapeHtml(doc.project)}</p>
    <p>${escapeHtml(doc.date)}</p>
    <p>Versão ${escapeHtml(doc.version)}</p>
    <div class="authors">${doc.authors.map((a) => `<p>${escapeHtml(a)}</p>`).join('')}</div>
  </section><div class="pb"></div>`

  const history = `<p class="plain-title">Histórico de Alterações</p>
    <table><thead><tr><th>Data</th><th>Versão</th><th>Descrição</th><th>Autor</th></tr></thead><tbody>
    ${doc.history.map((h) => `<tr><td>${escapeHtml(h.date)}</td><td>${escapeHtml(h.version)}</td><td>${inlineHtml(h.description)}</td><td>${escapeHtml(h.author)}</td></tr>`).join('')}
    </tbody></table><div class="pb"></div>`

  const toc = `<p class="plain-title">Conteúdo</p><nav class="toc">
    ${doc.toc.map((t) => `<a class="l${t.level}" href="#${escapeHtml(t.id)}"><span>${escapeHtml(t.number)}</span><span>${escapeHtml(t.title)}</span><span class="dots"></span></a>`).join('')}
    </nav>`

  const figure = { n: 0 }
  // O backend já emite a quebra depois do sumário e antes de cada seção nível 1.
  const body = doc.blocks.map((b) => blockHtml(b, figure)).join('\n')
  return `<article class="rd"><div class="page">${cover}${history}${toc}${body}</div></article>`
}

/** Abre o diálogo de impressão (Salvar como PDF) com o documento. As figuras já
 * chegam com URL absoluta (api.reqDocument), então o iframe não precisa de <base>. */
export function printDocument(doc: ReqDocument): Promise<void> {
  return platform.printHtml(`<!doctype html><html lang="pt-BR"><head><meta charset="utf-8">
    <title>${escapeHtml(`${doc.title} - ${doc.project}`)}</title>
    <style>@page { size: A4; margin: 2.5cm 2.2cm; } html, body { margin: 0; background: #fff; }
    ${DOC_CSS}</style></head><body>${documentHtml(doc)}</body></html>`)
}
