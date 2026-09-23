// Visualizador de Markdown com suporte a blocos ```mermaid.

import { useEffect, useMemo, useRef } from 'react'
import { renderMarkdown } from '../../lib/markdown'
import { mermaidErrorHtml, renderMermaid } from '../../lib/mermaid'

export function MarkdownView({ source, className }: { source: string; className?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const html = useMemo(() => renderMarkdown(source, { mermaid: true }), [source])

  useEffect(() => {
    const container = ref.current
    if (!container) return
    const blocks = Array.from(container.querySelectorAll<HTMLElement>('.mermaid'))
    if (blocks.length === 0) return

    let cancelled = false
    void (async () => {
      for (const block of blocks) {
        const code = block.textContent ?? ''
        try {
          const svg = await renderMermaid(code)
          if (cancelled) return
          block.innerHTML = svg
          block.classList.add('overflow-x-auto', 'rounded-lg', 'p-2')
        } catch (err) {
          if (cancelled) return
          // Um diagrama inválido não pode derrubar a leitura do documento.
          block.innerHTML = mermaidErrorHtml(code, err)
        }
      }
    })()
    return () => { cancelled = true }
  }, [html])

  return <div ref={ref} className={`md-body text-app ${className ?? ''}`} dangerouslySetInnerHTML={{ __html: html }} />
}
