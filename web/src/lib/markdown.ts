// Renderizador Markdown mínimo e sem dependências.
//
// Cobre exatamente o subconjunto que o ArchCode Studio gera e consome: títulos,
// listas (incluindo checkboxes), tabelas, blocos de código, citações, regras
// horizontais e formatação inline. Todo texto é escapado antes de virar HTML,
// então conteúdo de arquivo não consegue injetar marcação.

function escapeHtml(input: string): string {
  return input
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

function inline(text: string): string {
  let out = escapeHtml(text)
  // Código inline primeiro, para que seu conteúdo não sofra outras substituições.
  const codes: string[] = []
  out = out.replace(/`([^`]+)`/g, (_, code: string) => {
    codes.push(code)
    return `\u0000CODE${codes.length - 1}\u0000`
  })
  out = out
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, '<a href="$2" target="_blank" rel="noreferrer">$1</a>')
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    .replace(/(^|[^*])\*([^*\n]+)\*/g, '$1<em>$2</em>')
    .replace(/~~([^~]+)~~/g, '<del>$1</del>')
  out = out.replace(/\u0000CODE(\d+)\u0000/g, (_, i: string) => `<code>${codes[Number(i)]}</code>`)
  return out
}

interface RenderOptions {
  /** Blocos ```mermaid viram <div class="mermaid"> para renderização posterior. */
  mermaid?: boolean
}

export function renderMarkdown(source: string, options: RenderOptions = {}): string {
  const lines = source.replace(/\r\n/g, '\n').split('\n')
  const html: string[] = []
  let i = 0

  const closeList = (stack: string[]) => {
    while (stack.length) html.push(`</${stack.pop()}>`)
  }
  const listStack: string[] = []

  while (i < lines.length) {
    const line = lines[i]

    // Bloco de código
    const fence = line.match(/^\s*```(\w*)/)
    if (fence) {
      closeList(listStack)
      const lang = fence[1]
      const body: string[] = []
      i++
      while (i < lines.length && !/^\s*```/.test(lines[i])) {
        body.push(lines[i])
        i++
      }
      i++
      if (lang === 'mermaid' && options.mermaid) {
        html.push(`<div class="mermaid">${escapeHtml(body.join('\n'))}</div>`)
      } else {
        html.push(`<pre><code data-lang="${escapeHtml(lang)}">${escapeHtml(body.join('\n'))}</code></pre>`)
      }
      continue
    }

    // Tabela
    if (/^\s*\|/.test(line) && i + 1 < lines.length && /^\s*\|[\s:|-]+\|\s*$/.test(lines[i + 1])) {
      closeList(listStack)
      const cells = (row: string) =>
        row.trim().replace(/^\||\|$/g, '').split('|').map((c) => c.trim())
      const header = cells(line)
      i += 2
      const rows: string[][] = []
      while (i < lines.length && /^\s*\|/.test(lines[i])) {
        rows.push(cells(lines[i]))
        i++
      }
      html.push('<table><thead><tr>')
      for (const h of header) html.push(`<th>${inline(h)}</th>`)
      html.push('</tr></thead><tbody>')
      for (const row of rows) {
        html.push('<tr>')
        for (const c of row) html.push(`<td>${inline(c)}</td>`)
        html.push('</tr>')
      }
      html.push('</tbody></table>')
      continue
    }

    // Título
    const heading = line.match(/^(#{1,6})\s+(.*)$/)
    if (heading) {
      closeList(listStack)
      const level = heading[1].length
      html.push(`<h${level}>${inline(heading[2])}</h${level}>`)
      i++
      continue
    }

    // Regra horizontal
    if (/^\s*([-*_])\1{2,}\s*$/.test(line)) {
      closeList(listStack)
      html.push('<hr/>')
      i++
      continue
    }

    // Citação
    if (/^\s*>\s?/.test(line)) {
      closeList(listStack)
      const body: string[] = []
      while (i < lines.length && /^\s*>\s?/.test(lines[i])) {
        body.push(lines[i].replace(/^\s*>\s?/, ''))
        i++
      }
      html.push(`<blockquote>${renderMarkdown(body.join('\n'), options)}</blockquote>`)
      continue
    }

    // Listas (com suporte a checkbox de tarefa)
    const listItem = line.match(/^(\s*)([-*+]|\d+[.)])\s+(.*)$/)
    if (listItem) {
      const ordered = /\d/.test(listItem[2])
      const tag = ordered ? 'ol' : 'ul'
      if (listStack[listStack.length - 1] !== tag) {
        closeList(listStack)
        html.push(`<${tag}>`)
        listStack.push(tag)
      }
      let content = listItem[3]
      const check = content.match(/^\[([ xX~!])\]\s*(.*)$/)
      if (check) {
        const mark = check[1].toLowerCase()
        const icon = mark === 'x' ? '✅' : mark === '~' ? '🟡' : mark === '!' ? '⛔' : '⬜'
        content = `${icon} ${check[2]}`
      }
      html.push(`<li>${inline(content)}</li>`)
      i++
      continue
    }

    // Linha em branco encerra a lista corrente
    if (!line.trim()) {
      closeList(listStack)
      i++
      continue
    }

    // Parágrafo
    closeList(listStack)
    const paragraph: string[] = []
    while (i < lines.length && lines[i].trim() && !/^(\s*([-*+]|\d+[.)])\s|#{1,6}\s|\s*>|\s*```|\s*\|)/.test(lines[i])) {
      paragraph.push(lines[i])
      i++
    }
    if (paragraph.length) html.push(`<p>${inline(paragraph.join(' '))}</p>`)
  }

  closeList(listStack)
  return html.join('\n')
}
