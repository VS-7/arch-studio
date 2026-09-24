// Dados do Documento de Requisitos (.arch/document.yaml): capa, histórico de
// alterações, cliente, usuários, referências e glossário. É o único conteúdo do
// documento que não é derivado de requisitos, casos de uso e diagramas.

import { Plus, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { api } from '../../../lib/api'
import { errorMessage } from '../../../lib/errors'
import type { DocRevision, DocumentMeta, GlossaryTerm, Snapshot } from '../../../lib/types'
import { Button, Field, IconAction, Input, Modal, StringList, Textarea, useToast } from '../../ui'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../../ui/tabs'

const EMPTY: DocumentMeta = {
  title: 'Documento de Requisitos', version: '1.0', date: '', authors: [], client: '', users: '',
  introduction: '', history: [], references: [], glossary: [],
}

function today(): string {
  return new Date().toISOString().slice(0, 10)
}

export function DocumentMetaEditor({ open, onClose, snapshot }: { open: boolean; onClose: () => void; snapshot: Snapshot }) {
  const toast = useToast()
  const [form, setForm] = useState<DocumentMeta>(EMPTY)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    api.getDocumentMeta()
      .then((m) => setForm({ ...EMPTY, ...m }))
      .catch((err: unknown) => toast('error', errorMessage(err)))
  }, [open, toast])

  const set = <K extends keyof DocumentMeta>(key: K, value: DocumentMeta[K]) => setForm((f) => ({ ...f, [key]: value }))

  const save = async () => {
    setSaving(true)
    try {
      await api.saveDocumentMeta({
        ...form,
        authors: form.authors.filter((a) => a.trim()),
        references: form.references.filter((r) => r.trim()),
        history: form.history.filter((h) => h.version.trim() || h.description.trim()),
        glossary: form.glossary.filter((g) => g.term.trim()),
      })
      toast('success', 'Dados do documento salvos em .arch/document.yaml')
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setSaving(false) }
  }

  const actors = [...new Set(snapshot.use_cases.flatMap((uc) => uc.actors ?? []))]

  return (
    <Modal open={open} onClose={onClose} wide title="Dados do documento"
      description="Capa, histórico e as seções que não vêm do modelo. Requisitos, casos de uso e diagramas entram automaticamente."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={saving} onClick={() => void save()}>Salvar</Button>
      </>}>
      <Tabs defaultValue="capa">
        <TabsList>
          <TabsTrigger value="capa">Capa</TabsTrigger>
          <TabsTrigger value="historico">Histórico</TabsTrigger>
          <TabsTrigger value="descricao">Descrição geral</TabsTrigger>
          <TabsTrigger value="referencias">Referências e glossário</TabsTrigger>
        </TabsList>

        <TabsContent value="capa" className="space-y-3 pt-2">
          <div className="grid grid-cols-[1fr_120px_160px] gap-2.5">
            <Field label="Título"><Input value={form.title} onChange={(e) => set('title', e.target.value)} /></Field>
            <Field label="Versão"><Input value={form.version} onChange={(e) => set('version', e.target.value)} /></Field>
            <Field label="Data" hint="Vazio = data da geração">
              <Input type="date" value={form.date} onChange={(e) => set('date', e.target.value)} />
            </Field>
          </div>
          <Field label="Autores" hint="Um por linha, como na capa.">
            <StringList value={form.authors} onChange={(v) => set('authors', v)} placeholder="Nome completo" />
          </Field>
          <Field label="Introdução (opcional)" hint="Substitui o primeiro parágrafo padrão da seção 1.">
            <Textarea rows={3} value={form.introduction} onChange={(e) => set('introduction', e.target.value)}
              placeholder={`Este documento especifica os requisitos do ${snapshot.manifest.project_name}, fornecendo as informações necessárias para o projeto e implementação…`} />
          </Field>
        </TabsContent>

        <TabsContent value="historico" className="space-y-2 pt-2">
          <HistoryEditor rows={form.history} onChange={(v) => set('history', v)} defaultAuthor={form.authors.join(', ')} />
        </TabsContent>

        <TabsContent value="descricao" className="space-y-3 pt-2">
          <Field label="2.1 Cliente" hint="Quem contrata/usa o sistema e quais objetivos de negócio busca.">
            <Textarea rows={5} value={form.client} onChange={(e) => set('client', e.target.value)}
              placeholder="Os clientes do sistema são clínicas de estética especializadas…" />
          </Field>
          <Field label="2.2 Usuário" hint={actors.length ? `A tabela de atores é gerada automaticamente (${actors.join(', ')}).` : 'A tabela de atores é gerada a partir das fichas de caso de uso.'}>
            <Textarea rows={5} value={form.users} onChange={(e) => set('users', e.target.value)}
              placeholder="Os usuários do sistema são divididos em dois grupos principais…" />
          </Field>
          <p className="rounded-md bg-muted px-3 py-2 text-[11.5px] text-muted-foreground">
            A seção 2.3 <b>Visão Geral do Sistema</b> usa o texto "Visão Geral" da aba Requisitos.
          </p>
        </TabsContent>

        <TabsContent value="referencias" className="space-y-3 pt-2">
          <Field label="Referências">
            <StringList value={form.references} onChange={(v) => set('references', v)}
              placeholder="IEEE Std 830-1998 — Recommended Practice for Software Requirements Specifications" />
          </Field>
          <GlossaryEditor rows={form.glossary} onChange={(v) => set('glossary', v)} />
        </TabsContent>
      </Tabs>
    </Modal>
  )
}

function HistoryEditor({ rows, onChange, defaultAuthor }: { rows: DocRevision[]; onChange: (v: DocRevision[]) => void; defaultAuthor: string }) {
  const update = (i: number, patch: Partial<DocRevision>) => onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)))
  return (
    <div className="space-y-1.5">
      <div className="grid grid-cols-[140px_80px_1fr_1fr_24px] gap-1.5 px-0.5 text-[11px] font-medium text-muted-foreground">
        <span>Data</span><span>Versão</span><span>Descrição</span><span>Autor</span><span />
      </div>
      {rows.map((r, i) => (
        <div key={i} className="grid grid-cols-[140px_80px_1fr_1fr_24px] items-center gap-1.5">
          <Input type="date" value={r.date} onChange={(e) => update(i, { date: e.target.value })} />
          <Input value={r.version} placeholder="0.1" onChange={(e) => update(i, { version: e.target.value })} />
          <Input value={r.description} placeholder="Draft inicial do documento." onChange={(e) => update(i, { description: e.target.value })} />
          <Input value={r.author} onChange={(e) => update(i, { author: e.target.value })} />
          <IconAction label="Remover versão" onClick={() => onChange(rows.filter((_, j) => j !== i))}><X size={14} /></IconAction>
        </div>
      ))}
      {rows.length === 0 && <p className="py-2 text-[12px] text-muted-foreground">Sem histórico: o documento gera uma linha com a versão atual.</p>}
      <Button size="sm" variant="ghost" icon={Plus}
        onClick={() => onChange([...rows, { date: today(), version: '', description: '', author: defaultAuthor }])}>
        Adicionar versão
      </Button>
    </div>
  )
}

function GlossaryEditor({ rows, onChange }: { rows: GlossaryTerm[]; onChange: (v: GlossaryTerm[]) => void }) {
  const update = (i: number, patch: Partial<GlossaryTerm>) => onChange(rows.map((r, j) => (j === i ? { ...r, ...patch } : r)))
  return (
    <Field label="Glossário (Convenções, termos e abreviações)">
      <div className="space-y-1.5">
        {rows.map((g, i) => (
          <div key={i} className="grid grid-cols-[180px_1fr_24px] items-center gap-1.5">
            <Input value={g.term} placeholder="CDU" onChange={(e) => update(i, { term: e.target.value })} />
            <Input value={g.definition} placeholder="Caso de uso" onChange={(e) => update(i, { definition: e.target.value })} />
            <IconAction label="Remover termo" onClick={() => onChange(rows.filter((_, j) => j !== i))}><X size={14} /></IconAction>
          </div>
        ))}
        <Button size="sm" variant="ghost" icon={Plus} onClick={() => onChange([...rows, { term: '', definition: '' }])}>Adicionar termo</Button>
      </div>
    </Field>
  )
}
