// Formatação inline dos blocos do Documento de Requisitos. O backend só emite
// **negrito**, *itálico*, `código` e [texto](#ancora) — este parser cobre
// exatamente isso e é usado pela pré-visualização, pelo PDF e pelo DOCX.

export interface InlineRun {
  text: string
  bold?: boolean
  italic?: boolean
  code?: boolean
  /** Âncora interna (sem o #) quando o trecho é um link. */
  anchor?: string
}

const TOKEN = /(\*\*[^*]+\*\*|\*[^*\s][^*]*\*|`[^`]+`|\[[^\]]+\]\(#[^)]+\))/g

export function parseInline(text: string): InlineRun[] {
  const out: InlineRun[] = []
  let last = 0
  for (const m of text.matchAll(TOKEN)) {
    const idx = m.index ?? 0
    if (idx > last) out.push({ text: unescape(text.slice(last, idx)) })
    const tok = m[0]
    if (tok.startsWith('**')) {
      // Negrito pode conter itálico/código dentro: processa recursivamente.
      for (const inner of parseInline(tok.slice(2, -2))) out.push({ ...inner, bold: true })
    } else if (tok.startsWith('`')) {
      out.push({ text: tok.slice(1, -1), code: true })
    } else if (tok.startsWith('[')) {
      const lm = /^\[([^\]]+)\]\(#([^)]+)\)$/.exec(tok)
      if (lm) out.push({ text: unescape(lm[1]), anchor: lm[2] })
    } else {
      for (const inner of parseInline(tok.slice(1, -1))) out.push({ ...inner, italic: true })
    }
    last = idx + tok.length
  }
  if (last < text.length) out.push({ text: unescape(text.slice(last)) })
  return out
}

/** Remove escapes de Markdown (`\[RF001\]` → `[RF001]`). */
function unescape(s: string): string {
  return s.replace(/\\([[\]*_`#\\])/g, '$1')
}

export function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

export function inlineHtml(text: string): string {
  return parseInline(text).map((r) => {
    let h = escapeHtml(r.text)
    if (r.code) h = `<code>${h}</code>`
    if (r.italic) h = `<em>${h}</em>`
    if (r.bold) h = `<strong>${h}</strong>`
    if (r.anchor) h = `<a href="#${escapeHtml(r.anchor)}">${h}</a>`
    return h
  }).join('')
}
