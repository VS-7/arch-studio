// Visualizador de Markdown com suporte a blocos ```mermaid.

import { useEffect, useMemo, useRef } from 'react'
import mermaid from 'mermaid'
import { renderMarkdown } from '../../lib/markdown'

let mermaidReady = false

function ensureMermaid(dark: boolean) {
  mermaid.initialize({
    startOnLoad: false,
    theme: dark ? 'dark' : 'default',
    securityLevel: 'strict',
    fontFamily: "Inter, ui-sans-serif, system-ui, sans-serif",
    flowchart: { curve: 'basis', htmlLabels: true, padding: 14 },
  })
  mermaidReady = true
}

export function MarkdownView({ source, className }: { source: string; className?: string }) {
  const ref = useRef<HTMLDivElement>(null)
  const html = useMemo(() => renderMarkdown(source, { mermaid: true }), [source])

  useEffect(() => {
    const container = ref.current
    if (!container) return
    const blocks = Array.from(container.querySelectorAll<HTMLElement>('.mermaid'))
    if (blocks.length === 0) return

    const dark = document.documentElement.classList.contains('dark')
    if (!mermaidReady) ensureMermaid(dark)

    let cancelled = false
    void (async () => {
      for (const [index, block] of blocks.entries()) {
        const code = block.textContent ?? ''
        try {
          const { svg } = await mermaid.render(`mmd-${Date.now()}-${index}`, code)
          if (cancelled) return
          block.innerHTML = svg
          block.classList.add('overflow-x-auto', 'rounded-lg', 'p-2')
        } catch {
          if (cancelled) return
          // Um diagrama inválido não pode derrubar a leitura do documento.
          block.innerHTML = `<pre class="text-xs opacity-70">${code.replace(/[<>&]/g, '')}</pre>`
        }
      }
    })()
    return () => { cancelled = true }
  }, [html])

  return <div ref={ref} className={`md-body text-app ${className ?? ''}`} dangerouslySetInnerHTML={{ __html: html }} />
}
