import { Info, Play, Plug, Terminal } from 'lucide-react'
import { useEffect, useState, type ReactNode } from 'react'
import { api } from '../../../lib/api'
import { platform } from '../../../lib/platform'
import { Modal, useToast } from '../../ui'

export function McpModal({ open, onClose }: { open: boolean; onClose: () => void }) {
  const [root, setRoot] = useState('.')
  // Executável que o agente roda para o MCP via stdio: a CLI ou, no app desktop,
  // o próprio executável do app.
  const [command, setCommand] = useState('archcode-studio')
  // URL do transporte SSE anunciada pelo servidor; vazia quando não está exposto
  // (ex.: app desktop sem servidor MCP local, ou `serve --no-mcp`).
  const [sseUrl, setSseUrl] = useState('')
  useEffect(() => {
    if (!open) return
    api.health().then((h) => {
      setRoot(h.root)
      setCommand(h.mcp_command || 'archcode-studio')
      setSseUrl(h.mcp_sse_url ?? '')
    }).catch(() => {})
  }, [open])

  const json = JSON.stringify({ mcpServers: { 'archcode-studio': { command, args: ['mcp', '--dir', root] } } }, null, 2)

  return (
    <Modal open={open} onClose={onClose} wide title="Conectar um agente de IA"
      description="O ArchCode Studio é um servidor Model Context Protocol: o agente lê e modifica arquitetura e diagramas UML com ferramentas atômicas, sem corromper o layout.">
      <div className="space-y-4 text-sm">
        <Section icon={Terminal} title="Claude Code">
          <Code>{`claude mcp add archcode-studio -- ${command} mcp --dir ${root}`}</Code>
        </Section>
        <Section icon={Plug} title="Cursor, Antigravity, Windsurf, Roo Code">
          <p className="mb-1.5 text-xs text-muted-foreground">Adicione ao <code>.mcp.json</code> do projeto ou à configuração global:</p>
          <Code>{json}</Code>
        </Section>
        {sseUrl && (
          <Section icon={Play} title="Transporte SSE (servidor já rodando)">
            <Code>{sseUrl}</Code>
          </Section>
        )}
        <Section icon={Info} title="Fluxo recomendado para o agente">
          <ol className="list-decimal space-y-0.5 pl-4 text-xs leading-relaxed text-muted-foreground">
            <li><code>get_system_context</code> — entender o sistema antes de agir</li>
            <li><code>add_architecture_node</code> / <code>connect_nodes</code> — modelar a arquitetura</li>
            <li><code>upsert_requirement</code> / <code>upsert_use_case</code> — justificar cada componente</li>
            <li><code>generate_use_case_diagram</code>, <code>create_uml_diagram</code>, <code>add_uml_element</code>, <code>add_uml_relation</code> — casos de uso, classes, sequência e estados</li>
            <li><code>auto_layout_diagram</code> — organizar um diagrama recém-criado para caber no documento</li>
            <li><code>validate_architecture_rules</code> — corrigir os erros apontados</li>
            <li><code>generate_ai_prd</code> — compilar o blueprint em ordem topológica</li>
            <li><code>get_implementation_tasks</code> → codificar → <code>mark_task_status</code></li>
          </ol>
        </Section>
      </div>
    </Modal>
  )
}

function Section({ icon: Icon, title, children }: { icon: typeof Info; title: string; children: ReactNode }) {
  return (
    <div>
      <p className="mb-1.5 flex items-center gap-1.5 text-[12px] font-semibold">
        <Icon size={13} className="text-primary" />
        {title}
      </p>
      {children}
    </div>
  )
}

function Code({ children }: { children: string }) {
  const toast = useToast()
  return (
    <button onClick={() => { void platform.copyText(children).then(() => toast('success', 'Copiado')) }}
      className="block w-full cursor-copy overflow-x-auto rounded-md border bg-muted px-3 py-2 text-left font-mono text-[11px] leading-relaxed transition-colors hover:bg-accent"
      title="Clique para copiar">
      <pre className="whitespace-pre-wrap">{children}</pre>
    </button>
  )
}
