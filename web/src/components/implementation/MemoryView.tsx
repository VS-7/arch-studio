// Memória do projeto: o diário das sessões (humanas e de agentes) e as
// memórias duráveis (decisões pequenas, convenções, armadilhas, contexto,
// glossário). Tudo vai para o Git: nada de segredos aqui.

import { Brain, NotebookPen, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type { Note, Session } from '../../lib/types'
import { useLive } from '../../lib/useLive'
import { ViewFrame } from '../shell/ViewFrame'
import { Badge, Button, EmptyState, Field, Input, Modal, Select, Spinner, StringList, Textarea, useToast } from '../ui'
import { useConfirm } from '../ui/confirm'
import { displayName, formatDate } from './planMeta'

const NOTE_TYPES: Record<Note['type'], { label: string; color: string }> = {
  decisao: { label: 'Decisão', color: '#3b82f6' },
  convencao: { label: 'Convenção', color: '#10b981' },
  armadilha: { label: 'Armadilha', color: '#ef4444' },
  contexto: { label: 'Contexto', color: '#64748b' },
  glossario: { label: 'Glossário', color: '#8b5cf6' },
}

export function MemoryView() {
  const [tab, setTab] = useState<'sessions' | 'memory'>('sessions')
  const [query, setQuery] = useState('')
  const sessions = useLive(() => api.sessions(50), ['session_logged'])
  const notes = useLive(() => api.memory(query), ['memory_changed'], [query])
  const [dialog, setDialog] = useState<'session' | Note | 'note' | null>(null)

  return (
    <ViewFrame icon={Brain} title="Memória e Sessões"
      meta={<>{sessions.data?.length ?? 0} sessão(ões) registrada(s) · {notes.data?.length ?? 0} memória(s) · em .arch/sessions/ e .arch/memory/</>}
      actions={<>
        <div className="flex rounded-md border p-0.5">
          {(['sessions', 'memory'] as const).map((t) => (
            <button key={t} onClick={() => setTab(t)}
              className={cn('rounded-sm px-2.5 py-0.5 text-[12px] font-medium', tab === t ? 'bg-accent' : 'text-muted-foreground hover:text-foreground')}>
              {t === 'sessions' ? 'Sessões' : 'Memórias'}
            </button>
          ))}
        </div>
        {tab === 'sessions'
          ? <Button size="sm" variant="primary" icon={NotebookPen} onClick={() => setDialog('session')}>Registrar sessão</Button>
          : <Button size="sm" variant="primary" icon={Plus} onClick={() => setDialog('note')}>Nova memória</Button>}
      </>}>
      {tab === 'sessions' && (
        sessions.loading ? <Spinner /> : !sessions.data?.length
          ? <EmptyState icon={NotebookPen} title="Nenhuma sessão registrada" description="Agentes registram com log_session ao parar; pessoas, aqui ou com archcode-studio session log. É o que permite retomar de onde parou." />
          : <div className="space-y-2 p-3">{sessions.data.map((s) => <SessionCard key={s.id} session={s} />)}</div>
      )}
      {tab === 'memory' && (
        <div className="p-3">
          <Input className="mb-3 h-8 max-w-md" placeholder="Buscar nas memórias" value={query} onChange={(e) => setQuery(e.target.value)} />
          {notes.loading ? <Spinner /> : !notes.data?.length
            ? <EmptyState icon={Brain} title={query ? 'Nada encontrado' : 'Nenhuma memória'} description="Fatos duráveis do projeto que o time e os agentes precisam lembrar: decisões pequenas, convenções, armadilhas, contexto de negócio. Decisões grandes viram ADR." />
            : (
              <div className="grid gap-2 @3xl:grid-cols-2">
                {notes.data.map((n) => (
                  <button key={n.slug} onClick={() => setDialog(n)} className="rounded-lg border bg-card p-3 text-left hover:border-foreground/30">
                    <div className="flex items-center gap-2">
                      <Badge color={NOTE_TYPES[n.type]?.color}>{NOTE_TYPES[n.type]?.label ?? n.type}</Badge>
                      <span className="min-w-0 flex-1 truncate text-[13px] font-semibold">{n.title}</span>
                    </div>
                    <p className="mt-1.5 line-clamp-3 whitespace-pre-line text-[12px] text-muted-foreground">{n.body}</p>
                    <p className="mt-1.5 text-[11px] text-muted-foreground">
                      {[...(n.components ?? []), ...(n.tags ?? []).map((t) => `#${t}`)].join(' · ')}{n.author && ` · ${n.author}`} · {formatDate(n.updated)}
                    </p>
                  </button>
                ))}
              </div>
            )}
        </div>
      )}
      {dialog === 'session' && <SessionDialog onClose={() => setDialog(null)} />}
      {(dialog === 'note' || (dialog && typeof dialog === 'object')) && (
        <NoteDialog note={dialog === 'note' ? null : (dialog as Note)} onClose={() => setDialog(null)} />
      )}
    </ViewFrame>
  )
}

function SessionCard({ session: s }: { session: Session }) {
  const [open, setOpen] = useState(false)
  const lists: [string, string[] | undefined][] = [
    ['Feito', s.done], ['Decisões', s.decisions], ['Próximos passos', s.next_steps], ['Bloqueios', s.blockers],
    ['Comandos', s.commands], ['Arquivos', s.files],
  ]
  return (
    <div className="rounded-lg border bg-card">
      <button className="flex w-full items-start gap-3 px-3 py-2 text-left" onClick={() => setOpen(!open)}>
        <div className="min-w-0 flex-1">
          <p className="text-[12.5px]"><span className="font-semibold">{displayName(s.author)}</span>
            {s.agent && <span className="text-muted-foreground"> com {s.agent}</span>}
            <span className="text-muted-foreground"> · {s.started ? new Date(s.started).toLocaleString('pt-BR', { dateStyle: 'short', timeStyle: 'short' }) : ''}</span></p>
          <p className="mt-0.5 text-[12.5px]">{s.summary}</p>
        </div>
        <div className="flex shrink-0 flex-wrap justify-end gap-1">
          {s.sprint ? <Badge>S{String(s.sprint).padStart(2, '0')}</Badge> : null}
          {s.tasks?.map((t) => <Badge key={t} color="#3b82f6">{t}</Badge>)}
          {s.branch && <Badge><code className="font-mono">{s.branch}</code></Badge>}
        </div>
      </button>
      {open && (
        <div className="grid gap-3 border-t px-3 py-2 text-[12px] @3xl:grid-cols-2">
          {lists.filter(([, v]) => v?.length).map(([label, v]) => (
            <div key={label}><p className="mb-0.5 font-semibold text-muted-foreground">{label}</p>
              <ul className="list-disc space-y-0.5 pl-4">{v!.map((x) => <li key={x}>{x}</li>)}</ul></div>
          ))}
          {!!s.commits?.length && <p className="text-muted-foreground">Commits: <span className="font-mono">{s.commits.join(' ')}</span></p>}
          <p className="text-[11px] text-muted-foreground">{s.file}</p>
        </div>
      )}
    </div>
  )
}

function SessionDialog({ onClose }: { onClose: () => void }) {
  const toast = useToast()
  const [summary, setSummary] = useState('')
  const [done, setDone] = useState<string[]>([])
  const [decisions, setDecisions] = useState<string[]>([])
  const [next, setNext] = useState<string[]>([])
  const [blockers, setBlockers] = useState<string[]>([])
  const [busy, setBusy] = useState(false)
  const save = async () => {
    setBusy(true)
    try {
      await api.logSession({ summary, done, decisions, next_steps: next, blockers })
      toast('success', 'Sessão registrada')
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }
  return (
    <Modal open onClose={onClose} wide title="Registrar sessão de trabalho"
      description="Tarefas em andamento, branch, commits e sprint entram sozinhos. Quem retomar lê isto primeiro."
      footer={<><Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={busy} disabled={!summary.trim()} onClick={() => void save()}>Registrar</Button></>}>
      <div className="max-h-[60vh] space-y-3 overflow-y-auto pr-1">
        <Field label="Resumo (uma ou duas frases)"><Textarea rows={2} value={summary} onChange={(e) => setSummary(e.target.value)} autoFocus /></Field>
        <div className="grid gap-3 @2xl:grid-cols-2" style={{ gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)' }}>
          <Field label="Feito"><StringList value={done} onChange={setDone} /></Field>
          <Field label="Decisões"><StringList value={decisions} onChange={setDecisions} /></Field>
          <Field label="Próximos passos"><StringList value={next} onChange={setNext} /></Field>
          <Field label="Bloqueios"><StringList value={blockers} onChange={setBlockers} /></Field>
        </div>
      </div>
    </Modal>
  )
}

function NoteDialog({ note, onClose }: { note: Note | null; onClose: () => void }) {
  const toast = useToast()
  const confirm = useConfirm()
  const [form, setForm] = useState({
    title: note?.title ?? '', body: note?.body ?? '', type: note?.type ?? 'contexto',
    tags: (note?.tags ?? []).join(', '), components: (note?.components ?? []).join(', '),
  })
  const [busy, setBusy] = useState(false)
  const split = (s: string) => s.split(',').map((x) => x.trim()).filter(Boolean)
  const save = async () => {
    setBusy(true)
    try {
      await api.remember({ slug: note?.slug, title: form.title, body: form.body, type: form.type, tags: split(form.tags), components: split(form.components) })
      toast('success', 'Memória gravada')
      onClose()
    } catch (err) { toast('error', errorMessage(err)) } finally { setBusy(false) }
  }
  const remove = async () => {
    if (!note || !(await confirm({ title: `Apagar "${note.title}"?`, confirmLabel: 'Apagar', destructive: true }))) return
    try {
      await api.forget(note.slug)
      onClose()
    } catch (err) { toast('error', errorMessage(err)) }
  }
  return (
    <Modal open onClose={onClose} title={note ? 'Memória do projeto' : 'Nova memória'}
      footer={<>
        {note && <Button className="mr-auto" variant="ghost" icon={Trash2} onClick={() => void remove()}>Apagar</Button>}
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" loading={busy} disabled={!form.title.trim() || !form.body.trim()} onClick={() => void save()}>Salvar</Button>
      </>}>
      <div className="space-y-3">
        <div className="grid gap-3" style={{ gridTemplateColumns: 'minmax(0,1fr) 10rem' }}>
          <Field label="Título"><Input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} /></Field>
          <Field label="Tipo">
            <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value as Note['type'] })}>
              {Object.entries(NOTE_TYPES).map(([k, v]) => <option key={k} value={k}>{v.label}</option>)}
            </Select>
          </Field>
        </div>
        <Field label="O fato (curto e verificável)"><Textarea rows={5} value={form.body} onChange={(e) => setForm({ ...form, body: e.target.value })} /></Field>
        <div className="grid gap-3" style={{ gridTemplateColumns: 'minmax(0,1fr) minmax(0,1fr)' }}>
          <Field label="Componentes (vírgula)"><Input value={form.components} onChange={(e) => setForm({ ...form, components: e.target.value })} /></Field>
          <Field label="Tags (vírgula)"><Input value={form.tags} onChange={(e) => setForm({ ...form, tags: e.target.value })} /></Field>
        </div>
      </div>
    </Modal>
  )
}
