// Editor de propriedades de elementos e relações UML (painel "Editors" do
// StarUML). Cada campo grava ao sair do foco ou com Enter, via PATCH parcial.

import { ArrowDown, ArrowLeftRight, ArrowUp, ExternalLink, Plus, Trash2, X } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { api } from '../../lib/api'
import { errorMessage } from '../../lib/errors'
import type {
  FragmentOperator, LifelineKind, MessageKind, Snapshot, UMLDiagram, UMLElement, UMLMember,
  UMLRelation, UMLRelationType, Visibility,
} from '../../lib/types'
import {
  ELEMENT_LABEL, FRAGMENT_OPERATORS, KIND_META, LIFELINE_KINDS, MESSAGE_KINDS, RELATION_LABEL,
  UNNAMED_TYPES, sortedMessages,
} from '../../lib/umlMeta'
import { Checkbox } from '../ui/checkbox'
import { Input as UIInput } from '../ui/input'
import { Button, IconAction, Input, Select, Textarea, Tip, useToast } from '../ui'
import { Switch } from '../ui/switch'
import { ElementGlyph, RelationGlyph } from './UmlGlyph'

interface Props {
  snapshot: Snapshot
  diagram: UMLDiagram
  selection: { kind: 'element' | 'relation'; id: string }
  /** Token para focar o campo Nome (muda a cada pedido; 0 = não focar). */
  focusName?: number
  onClose: () => void
  onOpenUseCase?: (code: string) => void
  onFocusArch?: (nodeId: string) => void
}

export function UmlInspector({ snapshot, diagram, selection, focusName, onClose, onOpenUseCase, onFocusArch }: Props) {
  if (selection.kind === 'element') {
    const el = diagram.elements.find((e) => e.id === selection.id)
    if (!el) return <Gone onClose={onClose} />
    return <ElementEditor key={el.id} snapshot={snapshot} diagram={diagram} el={el} focusName={focusName}
      onClose={onClose} onOpenUseCase={onOpenUseCase} onFocusArch={onFocusArch} />
  }
  const rel = diagram.relations.find((r) => r.id === selection.id)
  if (!rel) return <Gone onClose={onClose} />
  return <RelationEditor key={rel.id} diagram={diagram} rel={rel} onClose={onClose} />
}

function Gone({ onClose }: { onClose: () => void }) {
  return (
    <div className="flex items-center justify-between px-3 py-3 text-xs text-muted-foreground">
      O item selecionado não existe mais.
      <Button size="sm" variant="ghost" onClick={onClose}>Fechar</Button>
    </div>
  )
}

/* Campos ---------------------------------------------------------------------- */

/** Linha de propriedade no estilo "property grid" do StarUML. */
function Prop({ label, children, top }: { label: string; children: ReactNode; top?: boolean }) {
  return (
    <div className={cn('grid grid-cols-[92px_1fr] gap-2 px-3 py-1', top ? 'items-start' : 'items-center')}>
      <span className={cn('truncate text-[11.5px] text-muted-foreground', top && 'pt-1.5')}>{label}</span>
      <div className="min-w-0">{children}</div>
    </div>
  )
}

function Section({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <section className="border-b pb-2">
      <header className="flex h-7 items-center justify-between bg-panel-header/60 px-3">
        <span className="text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">{title}</span>
        {action}
      </header>
      <div className="pt-1.5">{children}</div>
    </section>
  )
}

/** Input que só grava ao confirmar (blur/Enter), evitando uma requisição por tecla. */
function CommitInput({ value, onCommit, placeholder, autoFocus, mono, className }: {
  value: string; onCommit: (v: string) => void; placeholder?: string; autoFocus?: number; mono?: boolean; className?: string
}) {
  const [draft, setDraft] = useState(value)
  const ref = useRef<HTMLInputElement>(null)
  useEffect(() => setDraft(value), [value])
  useEffect(() => {
    if (autoFocus) { ref.current?.focus(); ref.current?.select() }
  }, [autoFocus])
  const commit = () => { if (draft !== value) onCommit(draft) }
  return (
    // data-pristine: sem edição pendente, Ctrl+Z/Y vão para o histórico do diagrama.
    <UIInput ref={ref} value={draft} placeholder={placeholder} data-pristine={draft === value ? 'true' : undefined}
      className={cn('h-7 text-[12.5px]', mono && 'font-mono', className)}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === 'Enter') { e.preventDefault(); commit(); (e.target as HTMLInputElement).blur() }
        if (e.key === 'Escape') { setDraft(value); (e.target as HTMLInputElement).blur() }
      }} />
  )
}

function CommitTextarea({ value, onCommit, placeholder, rows = 3 }: {
  value: string; onCommit: (v: string) => void; placeholder?: string; rows?: number
}) {
  const [draft, setDraft] = useState(value)
  useEffect(() => setDraft(value), [value])
  return (
    <Textarea value={draft} rows={rows} placeholder={placeholder} className="text-[12.5px]"
      onChange={(e) => setDraft(e.target.value)}
      onBlur={() => { if (draft !== value) onCommit(draft) }} />
  )
}

/* Elemento --------------------------------------------------------------------- */

function ElementEditor({ snapshot, diagram, el, focusName, onClose, onOpenUseCase, onFocusArch }: {
  snapshot: Snapshot; diagram: UMLDiagram; el: UMLElement; focusName?: number; onClose: () => void
  onOpenUseCase?: (code: string) => void; onFocusArch?: (nodeId: string) => void
}) {
  const toast = useToast()
  const patch = (p: Partial<UMLElement>) =>
    api.updateUMLElement(diagram.id, el.id, p).catch((err: unknown) => toast('error', errorMessage(err)))
  // Sem confirmação: a exclusão entra no histórico e o toast oferece desfazer (Ctrl+Z).
  const remove = async () => {
    try {
      await api.deleteUMLElement(diagram.id, el.id)
      onClose()
      toast('success', `"${el.name || ELEMENT_LABEL[el.type]}" excluído · Ctrl+Z desfaz`)
    } catch (err) { toast('error', errorMessage(err)) }
  }
  const classifier = el.type === 'class' || el.type === 'interface'
  const parent = el.parent_id ? diagram.elements.find((e) => e.id === el.parent_id) : undefined
  const related = diagram.relations.filter((r) => r.source === el.id || r.target === el.id)

  return (
    <div className="pb-4">
      <Header glyph={<ElementGlyph type={el.type} />} title={el.name || ELEMENT_LABEL[el.type]}
        subtitle={`${ELEMENT_LABEL[el.type]} · ${el.id}`} onClose={onClose} />

      <Section title="Propriedades">
        {(!UNNAMED_TYPES.has(el.type) || el.type === 'note') && (
          <Prop label={el.type === 'note' ? 'Título' : 'Nome'}>
            <CommitInput value={el.name} autoFocus={focusName} onCommit={(name) => void patch({ name })} />
          </Prop>
        )}
        {!['note', 'initial', 'final', 'choice', 'fork', 'join', 'history', 'fragment'].includes(el.type) && (
          <Prop label="Estereótipo">
            <CommitInput value={el.stereotype ?? ''} placeholder="ex.: entity, service" onCommit={(stereotype) => void patch({ stereotype })} />
          </Prop>
        )}
        {classifier && (
          <Prop label="Abstrata">
            <Switch checked={!!el.abstract} onCheckedChange={(abstract) => void patch({ abstract })} />
          </Prop>
        )}
        {el.type === 'lifeline' && (
          <Prop label="Tipo">
            <Select value={el.lifeline_kind ?? 'participant'} className="h-7 text-[12.5px]"
              onChange={(e) => void patch({ lifeline_kind: e.target.value as LifelineKind })}>
              {LIFELINE_KINDS.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
            </Select>
          </Prop>
        )}
        {el.type === 'fragment' && (
          <>
            <Prop label="Operador">
              <Select value={el.operator ?? 'alt'} className="h-7 text-[12.5px]"
                onChange={(e) => void patch({ operator: e.target.value as FragmentOperator })}>
                {FRAGMENT_OPERATORS.map((op) => <option key={op} value={op}>{op}</option>)}
              </Select>
            </Prop>
            <Prop label="Guarda">
              <CommitInput value={el.guard ?? ''} placeholder="condição" onCommit={(guard) => void patch({ guard })} />
            </Prop>
          </>
        )}
        {el.type === 'state' && (
          <>
            <Prop label="entry /"><CommitInput value={el.entry ?? ''} onCommit={(entry) => void patch({ entry })} /></Prop>
            <Prop label="do /"><CommitInput value={el.do ?? ''} onCommit={(v) => void patch({ do: v })} /></Prop>
            <Prop label="exit /"><CommitInput value={el.exit ?? ''} onCommit={(exit) => void patch({ exit })} /></Prop>
          </>
        )}
        {el.type === 'usecase' && (
          <Prop label="Ficha (CDU)">
            <div className="flex gap-1">
              <Select value={el.use_case ?? ''} className="h-7 text-[12.5px]"
                onChange={(e) => void patch({ use_case: e.target.value })}>
                <option value="">— nenhuma —</option>
                {snapshot.use_cases.map((uc) => <option key={uc.code} value={uc.code}>{uc.code} · {uc.name}</option>)}
              </Select>
              {el.use_case && onOpenUseCase && (
                <Button size="icon" variant="ghost" title="Abrir ficha" onClick={() => onOpenUseCase(el.use_case!)}>
                  <ExternalLink size={13} />
                </Button>
              )}
            </div>
          </Prop>
        )}
        {(classifier || el.type === 'lifeline' || el.type === 'state' || el.type === 'package') && (
          <Prop label="Componente">
            <div className="flex gap-1">
              <Select value={el.component_id ?? ''} className="h-7 text-[12.5px]"
                onChange={(e) => void patch({ component_id: e.target.value })}>
                <option value="">— nenhum —</option>
                {snapshot.diagram.nodes.filter((n) => n.type !== 'group').map((n) => (
                  <option key={n.id} value={n.id}>{n.data.label}</option>
                ))}
              </Select>
              {el.component_id && onFocusArch && (
                <Button size="icon" variant="ghost" title="Ver na arquitetura" onClick={() => onFocusArch(el.component_id!)}>
                  <ExternalLink size={13} />
                </Button>
              )}
            </div>
          </Prop>
        )}
        {parent && (
          <Prop label="Contido em">
            <span className="text-[12.5px]">{parent.name || ELEMENT_LABEL[parent.type]}</span>
          </Prop>
        )}
      </Section>

      {classifier && (
        <>
          <MembersEditor title="Atributos" members={el.attributes ?? []} onCommit={(attributes) => void patch({ attributes })} />
          <MembersEditor title="Operações" operations members={el.operations ?? []} onCommit={(operations) => void patch({ operations })} />
        </>
      )}
      {el.type === 'enum' && (
        <>
          <LiteralsEditor literals={el.literals ?? []} onCommit={(literals) => void patch({ literals })} />
          <MembersEditor title="Operações" operations members={el.operations ?? []} onCommit={(operations) => void patch({ operations })} />
        </>
      )}

      <Section title={el.type === 'note' ? 'Texto' : 'Documentação'}>
        <div className="px-3">
          <CommitTextarea value={el.documentation ?? ''} rows={el.type === 'note' ? 5 : 3}
            placeholder={el.type === 'note' ? 'Texto da nota' : 'Descrição, regras, observações…'}
            onCommit={(documentation) => void patch({ documentation })} />
        </div>
      </Section>

      {related.length > 0 && (
        <Section title={`Relações (${related.length})`}>
          <ul className="px-3 text-[12px]">
            {related.map((r) => {
              const other = diagram.elements.find((e) => e.id === (r.source === el.id ? r.target : r.source))
              return (
                <li key={r.id} className="flex items-center gap-1.5 py-0.5 text-muted-foreground">
                  <RelationGlyph type={r.type} size={14} />
                  <span className="truncate">{RELATION_LABEL[r.type]} {r.source === el.id ? '→' : '←'} <span className="text-foreground">{other?.name || '·'}</span></span>
                </li>
              )
            })}
          </ul>
        </Section>
      )}

      <div className="px-3 pt-3">
        <Button size="sm" variant="ghost" icon={Trash2} className="text-destructive hover:text-destructive" onClick={() => void remove()}>
          Excluir elemento
        </Button>
      </div>
    </div>
  )
}

function Header({ glyph, title, subtitle, onClose }: { glyph: ReactNode; title: string; subtitle: string; onClose: () => void }) {
  return (
    <div className="flex items-start gap-2 border-b px-3 py-2.5">
      <span className="mt-0.5 text-muted-foreground">{glyph}</span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-[13px] font-semibold">{title}</p>
        <p className="truncate font-mono text-[10.5px] text-muted-foreground">{subtitle}</p>
      </div>
      <IconAction label="Fechar editor" onClick={onClose}><X size={14} /></IconAction>
    </div>
  )
}

/* Membros de classe ------------------------------------------------------------ */

const VISIBILITIES: { value: Visibility; label: string }[] = [
  { value: '+', label: '+ public' }, { value: '-', label: '- private' },
  { value: '#', label: '# protected' }, { value: '~', label: '~ package' },
]

function MembersEditor({ title, members, operations, onCommit }: {
  title: string; members: UMLMember[]; operations?: boolean; onCommit: (next: UMLMember[]) => void
}) {
  const [rows, setRows] = useState<UMLMember[]>(members)
  useEffect(() => setRows(members), [members])
  const dirty = JSON.stringify(rows) !== JSON.stringify(members)
  const commit = (next = rows) => {
    const clean = next.filter((m) => m.name.trim()).map((m) => ({ ...m, name: m.name.trim() }))
    if (JSON.stringify(clean) !== JSON.stringify(members)) onCommit(clean)
  }
  const update = (i: number, p: Partial<UMLMember>, immediate = false) => {
    const next = rows.map((m, j) => (j === i ? { ...m, ...p } : m))
    setRows(next)
    if (immediate) commit(next)
  }

  return (
    <Section title={`${title} (${members.length})`} action={
      <IconAction label={`Adicionar ${operations ? 'operação' : 'atributo'}`}
        onClick={() => setRows([...rows, { name: '', visibility: operations ? '+' : '-', type: '' }])}>
        <Plus size={14} />
      </IconAction>
    }>
      <div className="space-y-1 px-3" onBlur={(e) => {
        // Grava quando o foco sai do bloco inteiro, não a cada campo.
        if (dirty && !e.currentTarget.contains(e.relatedTarget as Node)) commit()
      }}>
        {rows.length === 0 && <p className="py-1 text-[11.5px] text-muted-foreground">Nenhum {operations ? 'método' : 'atributo'}.</p>}
        {rows.map((m, i) => (
          <div key={i} className="grid grid-cols-[46px_1fr_auto] items-center gap-1">
            <Select value={m.visibility ?? '+'} className="h-7 px-1.5 pr-5 font-mono text-[12px]" title="Visibilidade"
              onChange={(e) => update(i, { visibility: e.target.value as Visibility }, true)}>
              {VISIBILITIES.map((v) => <option key={v.value} value={v.value}>{v.value}</option>)}
            </Select>
            <div className="flex min-w-0 gap-1">
              <Input value={m.name} placeholder="nome" className="h-7 min-w-0 flex-[1.2] font-mono text-[12px]"
                onChange={(e) => update(i, { name: e.target.value })} />
              {operations && (
                <Input value={m.params ?? ''} placeholder="params" className="h-7 min-w-0 flex-1 font-mono text-[12px]"
                  onChange={(e) => update(i, { params: e.target.value })} />
              )}
              <Input value={m.type ?? ''} placeholder="tipo" className="h-7 min-w-0 flex-1 font-mono text-[12px]"
                onChange={(e) => update(i, { type: e.target.value })} />
            </div>
            <div className="flex items-center gap-0.5">
              <Tip label="Estático (exibido sublinhado)">
                <label className="flex items-center gap-1 px-0.5 text-[10.5px] text-muted-foreground">
                  <Checkbox checked={!!m.static} onCheckedChange={(v) => update(i, { static: v === true }, true)} />S
                </label>
              </Tip>
              <IconAction label="Remover membro" className="hover:text-destructive"
                onClick={() => { const next = rows.filter((_, j) => j !== i); setRows(next); commit(next) }}>
                <X size={13} />
              </IconAction>
            </div>
          </div>
        ))}
      </div>
    </Section>
  )
}

function LiteralsEditor({ literals, onCommit }: { literals: string[]; onCommit: (next: string[]) => void }) {
  return (
    <Section title={`Literais (${literals.length})`}>
      <div className="px-3">
        <CommitTextarea value={literals.join('\n')} rows={Math.max(3, literals.length + 1)} placeholder="Um literal por linha"
          onCommit={(v) => onCommit(v.split('\n').map((s) => s.trim()).filter(Boolean))} />
      </div>
    </Section>
  )
}

/* Relação ---------------------------------------------------------------------- */

function RelationEditor({ diagram, rel, onClose }: { diagram: UMLDiagram; rel: UMLRelation; onClose: () => void }) {
  const toast = useToast()
  const patch = (p: Partial<UMLRelation>) =>
    api.updateUMLRelation(diagram.id, rel.id, p).catch((err: unknown) => toast('error', errorMessage(err)))
  const remove = async () => {
    try { await api.deleteUMLRelation(diagram.id, rel.id); onClose() } catch (err) { toast('error', errorMessage(err)) }
  }
  const name = (id: string) => diagram.elements.find((e) => e.id === id)?.name || '·'
  const ends = ['association', 'directed_association', 'aggregation', 'composition'].includes(rel.type)

  const move = async (dir: -1 | 1) => {
    const msgs = sortedMessages(diagram)
    const i = msgs.findIndex((m) => m.id === rel.id)
    const j = i + dir
    if (i < 0 || j < 0 || j >= msgs.length) return
    ;[msgs[i], msgs[j]] = [msgs[j], msgs[i]]
    const order = new Map(msgs.map((m, k) => [m.id, k + 1]))
    const relations = diagram.relations.map((r) => (order.has(r.id) ? { ...r, order: order.get(r.id) } : r))
    try { await api.saveUML({ ...diagram, relations }) } catch (err) { toast('error', errorMessage(err)) }
  }

  return (
    <div className="pb-4">
      <Header glyph={<RelationGlyph type={rel.type} />} title={rel.name || RELATION_LABEL[rel.type]}
        subtitle={`${RELATION_LABEL[rel.type]} · ${rel.id}`} onClose={onClose} />

      <Section title="Propriedades">
        <Prop label="Tipo">
          <Select value={rel.type} className="h-7 text-[12.5px]"
            onChange={(e) => void patch({ type: e.target.value as UMLRelationType })}>
            {KIND_META[diagram.kind].relations.map((t) => <option key={t} value={t}>{RELATION_LABEL[t]}</option>)}
          </Select>
        </Prop>
        <Prop label="Origem → Destino">
          <div className="flex items-center gap-1 text-[12.5px]">
            <span className="truncate">{name(rel.source)}</span>
            <span className="text-muted-foreground">→</span>
            <span className="truncate">{name(rel.target)}</span>
            {rel.source !== rel.target && (
              <IconAction label="Inverter sentido" className="ml-auto"
                onClick={() => void patch({ source: rel.target, target: rel.source })}>
                <ArrowLeftRight size={13} />
              </IconAction>
            )}
          </div>
        </Prop>
        {rel.type !== 'transition' && rel.type !== 'note_link' && (
          <Prop label={rel.type === 'message' ? 'Mensagem' : 'Nome'}>
            <CommitInput value={rel.name ?? ''} mono={rel.type === 'message'}
              placeholder={rel.type === 'message' ? 'login(email, senha)' : ''} onCommit={(v) => void patch({ name: v })} />
          </Prop>
        )}
        {rel.type === 'message' && (
          <>
            <Prop label="Tipo de msg.">
              <Select value={rel.message_kind ?? 'sync'} className="h-7 text-[12.5px]"
                onChange={(e) => void patch({ message_kind: e.target.value as MessageKind })}>
                {MESSAGE_KINDS.map((k) => <option key={k.value} value={k.value}>{k.label}</option>)}
              </Select>
            </Prop>
            <Prop label="Ordem">
              <div className="flex items-center gap-1">
                <span className="w-8 font-mono text-[12.5px]">{rel.order ?? '–'}</span>
                <Button size="icon" variant="ghost" title="Subir" onClick={() => void move(-1)}><ArrowUp size={13} /></Button>
                <Button size="icon" variant="ghost" title="Descer" onClick={() => void move(1)}><ArrowDown size={13} /></Button>
              </div>
            </Prop>
          </>
        )}
        {rel.type === 'transition' && (
          <>
            <Prop label="Gatilho"><CommitInput value={rel.trigger ?? ''} placeholder="evento" onCommit={(trigger) => void patch({ trigger })} /></Prop>
            <Prop label="Guarda"><CommitInput value={rel.guard ?? ''} placeholder="condição" onCommit={(guard) => void patch({ guard })} /></Prop>
            <Prop label="Efeito"><CommitInput value={rel.effect ?? ''} placeholder="ação" onCommit={(effect) => void patch({ effect })} /></Prop>
          </>
        )}
      </Section>

      {ends && (
        <Section title="Extremidades">
          <Prop label="Papel origem"><CommitInput value={rel.source_role ?? ''} onCommit={(v) => void patch({ source_role: v })} /></Prop>
          <Prop label="Mult. origem"><CommitInput value={rel.source_multiplicity ?? ''} placeholder="1, 0..1, *" mono onCommit={(v) => void patch({ source_multiplicity: v })} /></Prop>
          <Prop label="Papel destino"><CommitInput value={rel.target_role ?? ''} onCommit={(v) => void patch({ target_role: v })} /></Prop>
          <Prop label="Mult. destino"><CommitInput value={rel.target_multiplicity ?? ''} placeholder="1, 0..1, *" mono onCommit={(v) => void patch({ target_multiplicity: v })} /></Prop>
        </Section>
      )}

      <Section title="Documentação">
        <div className="px-3">
          <CommitTextarea value={rel.documentation ?? ''} onCommit={(documentation) => void patch({ documentation })} />
        </div>
      </Section>

      <div className="px-3 pt-3">
        <Button size="sm" variant="ghost" icon={Trash2} className="text-destructive hover:text-destructive" onClick={() => void remove()}>
          Excluir relação
        </Button>
      </div>
    </div>
  )
}
