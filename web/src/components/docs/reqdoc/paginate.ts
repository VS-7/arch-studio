// Paginação do Documento de Requisitos em folhas A4.
//
// Mede cada bloco no DOM (na largura útil da página e com a fonte da
// impressão) e distribui os blocos pelas folhas: títulos ficam junto do que vem
// depois, listas, tabelas e o sumário são divididos por linha (o cabeçalho da
// tabela se repete), figuras nunca se partem e cada seção de nível 1 começa em
// folha nova (o backend marca com blocos "pagebreak"). Ao final, o sumário
// recebe a página de cada título.
//
// A medição e a impressão usam o mesmo CSS e a mesma largura, então a folha
// da pré-visualização é a folha do PDF.

import type { ReqDocument } from '../../../lib/types'
import {
  blockHtml, coverHtml, DOC_CSS, headingHtml, listItemHtml, listOpen, sheetHtml, tableHeadHtml, tableRowHtml, tocRowHtml,
} from './html'
import { BODY_MM, mmToPx } from './theme'

export interface PagedDocument {
  /** Todas as folhas, dentro de <article class="rd">. */
  html: string
  /** Número de folhas, capa incluída. */
  pages: number
  /** Página em que cada título (id da âncora) começa. */
  pageOf: Record<string, number>
}

/** Bloco divisível por linhas (lista, tabela, sumário). */
interface Rows {
  /** Abre o contêiner a partir da linha `firstRow` (listas numeradas continuam a contagem). */
  open: (firstRow: number) => string
  close: string
  rows: string[]
  /** Seletor das linhas dentro do contêiner, para a medição. */
  rowSelector: string
}

interface Item {
  html?: string
  rows?: Rows
  keepWithNext?: boolean
  /** Bloco indivisível que precisa caber inteiro junto do título anterior (figura). */
  atomic?: boolean
  breakBefore?: boolean
  /** Título de nível 1: passa a ser a seção corrente do rodapé. */
  section?: string
  /** Âncora do título, para o sumário. */
  id?: string
  height: number
  rowHeights: number[]
  /** Altura do contêiner além das linhas (margens, cabeçalho da tabela). */
  overhead: number
}

interface Page {
  parts: string[]
  section: string
  used: number
}

/** Altura útil da folha, com uma folga para arredondamentos da impressão. */
const BODY_PX = mmToPx(BODY_MM.height) - 2
/** Mínimo do bloco seguinte que precisa caber junto de um título (~3 linhas). */
const MIN_KEEP_PX = 60
const MAX_KEEP_CHAIN = 3

/** Parágrafo-rótulo ("**Fluxo de eventos principal**"), que fica junto da lista que introduz. */
const isLabel = (text: string) => /^\*\*[^*]+\*\*:?$/.test(text.trim())

function flowItems(doc: ReqDocument): Item[] {
  const out: Item[] = []
  const add = (partial: Omit<Item, 'height' | 'rowHeights' | 'overhead'>) =>
    out.push({ height: 0, rowHeights: [], overhead: 0, ...partial })

  add({ html: '<p class="plain-title">Histórico de Alterações</p>', keepWithNext: true, section: 'Histórico de Alterações' })
  add({ rows: {
    open: () => `<table>${tableHeadHtml(['Data', 'Versão', 'Descrição', 'Autor'])}<tbody>`,
    close: '</tbody></table>',
    rows: doc.history.map((h) => tableRowHtml([h.date, h.version, h.description, h.author])),
    rowSelector: 'tbody > tr',
  } })

  add({ html: '<p class="plain-title">Sumário</p>', keepWithNext: true, breakBefore: true, section: 'Sumário' })
  add({ rows: { open: () => '<nav class="toc">', close: '</nav>', rows: doc.toc.map(tocRowHtml), rowSelector: 'a' } })

  // O backend já emite "pagebreak" antes de cada seção de nível 1.
  let pendingBreak = true
  doc.blocks.forEach((b, i) => {
    if (b.type === 'pagebreak') {
      pendingBreak = true
      return
    }
    const next = doc.blocks[i + 1]
    switch (b.type) {
      case 'heading':
        add({
          html: headingHtml(b), keepWithNext: true, breakBefore: pendingBreak, id: b.id,
          section: b.level === 1 ? `${b.number ? `${b.number} ` : ''}${b.text}` : undefined,
        })
        break
      case 'paragraph':
        add({ html: blockHtml(b), keepWithNext: isLabel(b.text), breakBefore: pendingBreak })
        break
      case 'list':
        add({ rows: {
          open: (i) => listOpen(b.ordered, (b.start ?? 1) + i),
          close: b.ordered ? '</ol>' : '</ul>',
          rows: b.items.map(listItemHtml),
          rowSelector: 'li',
        }, breakBefore: pendingBreak })
        break
      case 'table':
        add({ rows: {
          open: () => `<table>${tableHeadHtml(b.header)}<tbody>`,
          close: '</tbody></table>',
          rows: b.rows.map(tableRowHtml),
          rowSelector: 'tbody > tr',
        }, breakBefore: pendingBreak })
        break
      case 'image':
        // A figura fica junto da descrição que a segue (e inteira junto do título anterior).
        add({ html: blockHtml(b), atomic: true, keepWithNext: next?.type === 'paragraph', breakBefore: pendingBreak })
        break
      default:
        add({ html: blockHtml(b), breakBefore: pendingBreak })
    }
    pendingBreak = false
  })
  return out
}

const wrap = (inner: string) => `<div class="mi">${inner}</div>`
const whole = (r: Rows) => r.open(0) + r.rows.join('') + r.close

// ---------------------------------------------------------------------------
// Medição
// ---------------------------------------------------------------------------

function createHost(): { body: HTMLElement; dispose: () => void } {
  const root = document.createElement('div')
  root.setAttribute('aria-hidden', 'true')
  root.style.cssText = 'position:absolute;left:-100000px;top:0;visibility:hidden;pointer-events:none;'
  root.innerHTML = `<style>${DOC_CSS}</style><article class="rd"><section class="sheet" style="height:auto;overflow:visible">` +
    `<div class="body" style="height:auto;overflow:visible"></div></section></article>`
  document.body.appendChild(root)
  return { body: root.querySelector<HTMLElement>('.body')!, dispose: () => root.remove() }
}

function imagesLoaded(root: HTMLElement): Promise<void[]> {
  return Promise.all([...root.querySelectorAll('img')].map((img) =>
    img.complete ? Promise.resolve() : new Promise<void>((resolve) => { img.onload = () => resolve(); img.onerror = () => resolve() })))
}

async function measure(items: Item[], body: HTMLElement): Promise<void> {
  body.innerHTML = items.map((it) => wrap(it.rows ? whole(it.rows) : it.html ?? '')).join('')
  await imagesLoaded(body)
  const wrappers = body.querySelectorAll<HTMLElement>(':scope > .mi')
  wrappers.forEach((w, i) => {
    const it = items[i]
    it.height = w.getBoundingClientRect().height
    if (it.rows) {
      const container = w.firstElementChild
      const rows = container ? [...container.querySelectorAll(`:scope > ${it.rows.rowSelector}`)] : []
      it.rowHeights = rows.map((r) => r.getBoundingClientRect().height)
      it.overhead = Math.max(0, it.height - it.rowHeights.reduce((a, b) => a + b, 0))
    }
  })
}

// ---------------------------------------------------------------------------
// Distribuição pelas folhas
// ---------------------------------------------------------------------------

/** Quanto do que vem depois precisa caber junto do item i (títulos e rótulos). */
function lookahead(items: Item[], i: number, depth = 0): number {
  const it = items[i]
  const next = items[i + 1]
  if (!it.keepWithNext || !next || depth >= MAX_KEEP_CHAIN) return 0
  if (next.keepWithNext || next.atomic) return next.height + lookahead(items, i + 1, depth + 1)
  if (next.rows) return next.overhead + next.rowHeights.slice(0, 2).reduce((a, b) => a + b, 0)
  return Math.min(next.height, MIN_KEEP_PX)
}

function distribute(items: Item[]): { pages: Page[]; pageOf: Record<string, number> } {
  const pages: Page[] = []
  const pageOf: Record<string, number> = {}
  let section = ''
  // Folha que está recebendo blocos (null = a próxima colocação abre uma folha).
  const state: { cur: Page | null } = { cur: null }

  const ensure = (): Page => {
    if (!state.cur) {
      state.cur = { parts: [], section, used: 0 }
      pages.push(state.cur)
    }
    return state.cur
  }
  const flush = () => { state.cur = null }
  const place = (page: Page, html: string, height: number) => {
    page.parts.push(html)
    page.used += height
  }

  for (let i = 0; i < items.length; i++) {
    const it = items[i]
    if (it.breakBefore && state.cur?.parts.length) flush()
    if (it.section) section = it.section

    if (!it.rows || it.rows.rows.length === 0) {
      const html = wrap(it.rows ? whole(it.rows) : it.html ?? '')
      let page = ensure()
      if (it.height + lookahead(items, i) > BODY_PX - page.used && page.parts.length) {
        flush()
        page = ensure()
      }
      place(page, html, it.height)
      if (it.id) pageOf[it.id] = pages.length + 1 // capa é a página 1
      continue
    }

    const { rows } = it
    const n = rows.rows.length
    let start = 0
    while (start < n) {
      const page = ensure()
      const first = page.parts.length === 0
      const avail = BODY_PX - page.used - it.overhead
      let end = start
      let acc = 0
      while (end < n && acc + it.rowHeights[end] <= avail) {
        acc += it.rowHeights[end]
        end++
      }
      if (end === start) {
        if (!first) { flush(); continue }
        end = start + 1 // linha maior que a folha: entra mesmo assim
      }
      // Não deixa uma linha sozinha no fim da folha nem uma sozinha na próxima.
      if (end < n && end - start < 2 && !first && n - start >= 2) { flush(); continue }
      if (end < n && n - end === 1 && end - start >= 2) end--
      const chunk = rows.open(start) + rows.rows.slice(start, end).join('') + rows.close
      const height = it.overhead + it.rowHeights.slice(start, end).reduce((a, b) => a + b, 0)
      place(page, wrap(chunk), height)
      start = end
      if (start < n) flush()
    }
  }
  return { pages, pageOf }
}

function fillTocNumbers(html: string, pageOf: Record<string, number>): string {
  return html.replace(/(<span class="pg" data-pg="([^"]+)">)–(<\/span>)/g,
    (_m, open: string, id: string, close: string) => `${open}${pageOf[id] ?? '–'}${close}`)
}

/** Pagina o documento em folhas A4 (precisa do DOM: mede os blocos na página). */
export async function paginateDocument(doc: ReqDocument): Promise<PagedDocument> {
  const items = flowItems(doc)
  await (document.fonts?.ready ?? Promise.resolve())
  const { body, dispose } = createHost()
  try {
    await measure(items, body)
  } finally {
    dispose()
  }
  const { pages, pageOf } = distribute(items)
  const total = pages.length + 1
  const sheets = [
    coverHtml(doc),
    ...pages.map((p, i) => sheetHtml(doc, p.parts.join(''), { number: i + 2, total, section: p.section })),
  ]
  return { html: fillTocNumbers(`<article class="rd">${sheets.join('')}</article>`, pageOf), pages: total, pageOf }
}
