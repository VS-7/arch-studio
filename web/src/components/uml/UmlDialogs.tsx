// Diálogos de gestão de diagramas UML: criar, renomear e ver o espelho Mermaid.

import { ArrowDownToLine, Copy } from 'lucide-react'
import { useEffect, useState } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import { mermaidErrorMessage, renderMermaid } from '../../lib/mermaid'
import { platform } from '../../lib/platform'
import { useTheme } from '../../lib/theme'
import type { UMLDiagram, UMLKind } from '../../lib/types'
import { KIND_META, UML_KINDS } from '../../lib/umlMeta'
import { Button, Field, Input, Modal, Textarea, useToast } from '../ui'
import { KindGlyph } from './UmlGlyph'

export function NewDiagramDialog({ open, initialKind, onClose, onCreated }: {
  open: boolean; initialKind: UMLKind; onClose: () => void; onCreated: (d: UMLDiagram) => void
}) {
  const toast = useToast()
  const [kind, setKind] = useState<UMLKind>(initialKind)
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (open) { setKind(initialKind); setName(''); setDescription('') }
  }, [open, initialKind])

  const create = async () => {
    if (!name.trim()) return
    setSaving(true)
    try {
      const d = await api.createUML({ kind, name: name.trim(), description: description.trim() || undefined })
      toast('success', `${KIND_META[kind].label} "${d.name}" criado`)
      onCreated(d)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setSaving(false) }
  }

  return (
    <Modal open={open} onClose={onClose} title="Novo diagrama"
      description="O diagrama é gravado em .arch/diagrams/<tipo>/ com um espelho Mermaid, versionável em Git."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={saving} disabled={!name.trim()} onClick={() => void create()}>Criar diagrama</Button>
      </>}>
      <div className="space-y-4">
        <div className="grid grid-cols-2 gap-2">
          {UML_KINDS.map((k) => (
            <button key={k} onClick={() => setKind(k)}
              className={cn(
                'flex items-center gap-2.5 rounded-md border px-3 py-2.5 text-left text-[12.5px] transition-colors',
                kind === k ? 'border-primary bg-accent text-accent-foreground' : 'hover:bg-muted',
              )}>
              <span className={kind === k ? 'text-primary' : 'text-muted-foreground'}><KindGlyph kind={k} size={20} /></span>
              <span className="font-medium">{KIND_META[k].label}</span>
            </button>
          ))}
        </div>
        <Field label="Nome">
          <Input autoFocus value={name} placeholder={PLACEHOLDER[kind]} onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') void create() }} />
        </Field>
        <Field label="Descrição (opcional)">
          <Textarea rows={2} value={description} onChange={(e) => setDescription(e.target.value)} />
        </Field>
      </div>
    </Modal>
  )
}

const PLACEHOLDER: Record<UMLKind, string> = {
  usecase: 'Visão geral de casos de uso',
  class: 'Modelo de domínio',
  sequence: 'CDU001 — Autenticar usuário',
  state: 'Ciclo de vida do pedido',
}

export function RenameDiagramDialog({ diagram, onClose }: { diagram: UMLDiagram | null; onClose: () => void }) {
  const toast = useToast()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  useEffect(() => {
    if (diagram) { setName(diagram.name); setDescription(diagram.description ?? '') }
  }, [diagram])

  const save = async () => {
    if (!diagram || !name.trim()) return
    try {
      await api.renameUML(diagram.id, { name: name.trim(), description })
      onClose()
    } catch (err) { toast('error', errorMessage(err)) }
  }

  return (
    <Modal open={!!diagram} onClose={onClose} title="Propriedades do diagrama"
      description={diagram?.file}
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" disabled={!name.trim()} onClick={() => void save()}>Salvar</Button>
      </>}>
      <div className="space-y-4">
        <Field label="Nome"><Input autoFocus value={name} onChange={(e) => setName(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') void save() }} /></Field>
        <Field label="Descrição"><Textarea rows={3} value={description} onChange={(e) => setDescription(e.target.value)} /></Field>
      </div>
    </Modal>
  )
}

export function UmlMermaidDialog({ diagram, onClose }: { diagram: UMLDiagram | null; onClose: () => void }) {
  const toast = useToast()
  const dark = useTheme().resolved === 'dark'
  const [code, setCode] = useState('')
  // Estado (e não ref): o conteúdo do diálogo monta depois do efeito de abertura.
  const [preview, setPreview] = useState<HTMLDivElement | null>(null)

  useEffect(() => {
    if (!diagram) return
    let cancelled = false
    api.umlMermaid(diagram.id)
      .then((res) => { if (!cancelled) setCode(res.mermaid) })
      .catch((err: unknown) => toast('error', errorMessage(err)))
    return () => { cancelled = true }
  }, [diagram, toast])

  useEffect(() => {
    if (!code || !preview) return
    let cancelled = false
    void (async () => {
      try {
        const svg = await renderMermaid(code, dark ? 'dark' : 'neutral')
        if (!cancelled) preview.innerHTML = svg
      } catch (err) {
        if (!cancelled) preview.textContent = `Pré-visualização indisponível: ${mermaidErrorMessage(err)}`
      }
    })()
    return () => { cancelled = true }
  }, [code, preview, dark])

  const download = () => {
    if (diagram) void platform.saveFile(new Blob([code], { type: 'text/plain' }), `${diagram.id}.mermaid`)
  }

  return (
    <Modal open={!!diagram} onClose={onClose} wide title={`Mermaid — ${diagram?.name ?? ''}`}
      description="Espelho textual regenerado a cada gravação; cole em READMEs, PRs ou no contexto de uma IA."
      footer={<>
        <Button variant="ghost" icon={Copy} onClick={() => { void platform.copyText(code).then(() => toast('success', 'Mermaid copiado')) }}>Copiar</Button>
        <Button variant="secondary" icon={ArrowDownToLine} onClick={download}>Baixar .mermaid</Button>
      </>}>
      <div className="space-y-3">
        <div ref={setPreview} className="max-h-[45vh] overflow-auto rounded-md border bg-background p-4 text-xs text-muted-foreground [&_svg]:mx-auto" />
        <pre className="max-h-56 overflow-auto rounded-md bg-muted p-3 font-mono text-[11px] leading-relaxed">{code || '…'}</pre>
      </div>
    </Modal>
  )
}
