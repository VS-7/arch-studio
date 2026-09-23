// Renderização Mermaid compartilhada por Markdown, espelho da arquitetura e
// diagramas UML.
//
// O Mermaid 11, quando o texto não passa no parser, desenha um SVG de erro
// ("Syntax error in text") num <div> temporário do <body> e lança a exceção
// antes de removê-lo — o aviso ficava "vazando" no rodapé da aplicação. Aqui a
// renderização de erro é suprimida, os temporários são sempre limpos e as
// chamadas são serializadas (mermaid.render não é reentrante).

import mermaid from 'mermaid'

export type MermaidTheme = 'default' | 'dark' | 'neutral'

let queue: Promise<unknown> = Promise.resolve()
let seq = 0

function isDark(): boolean {
  return document.documentElement.classList.contains('dark')
}

/** Renderiza o código Mermaid e devolve o SVG; rejeita com a mensagem do parser. */
export function renderMermaid(code: string, theme?: MermaidTheme): Promise<string> {
  const run = async () => {
    const id = `mmd-${Date.now().toString(36)}-${++seq}`
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      suppressErrorRendering: true,
      theme: theme ?? (isDark() ? 'dark' : 'default'),
      fontFamily: 'Inter, ui-sans-serif, system-ui, sans-serif',
      flowchart: { curve: 'basis', htmlLabels: true, padding: 14 },
    })
    try {
      const { svg } = await mermaid.render(id, code)
      return svg
    } finally {
      for (const tmp of [id, `d${id}`, `i${id}`]) document.getElementById(tmp)?.remove()
    }
  }
  const result = queue.then(run, run)
  queue = result.catch(() => undefined)
  return result
}

/** Primeira linha útil do erro do parser ("Parse error on line 9: …"). */
export function mermaidErrorMessage(err: unknown): string {
  const text = err instanceof Error ? err.message : String(err)
  const lines = text.split('\n').map((l) => l.trim()).filter(Boolean)
  const expecting = lines.find((l) => l.startsWith('Expecting') || l.startsWith('Lexical error'))
  return [lines[0], expecting].filter(Boolean).join(' — ') || 'erro desconhecido'
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

/** Aviso inline (HTML) para um diagrama que não pôde ser desenhado. */
export function mermaidErrorHtml(code: string, err: unknown): string {
  return `<div class="mermaid-error">
    <p><strong>Diagrama Mermaid inválido.</strong> ${escapeHtml(mermaidErrorMessage(err))}</p>
    <details><summary>Ver código</summary><pre>${escapeHtml(code)}</pre></details>
  </div>`
}
