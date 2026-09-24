// Projetos no app desktop: abrir uma pasta, criar um projeto novo, reabrir um
// recente e a tela inicial (sem projeto aberto). No navegador o projeto é o
// diretório do `archcode-studio serve`, então nada disto aparece.

import { FolderOpen, FolderPlus, History, Waypoints, X } from 'lucide-react'
import { useCallback, useEffect, useState, type ReactNode } from 'react'
import { desktop, type RecentProject } from '../../lib/desktop'
import { errorMessage } from '../../lib/errors'
import type { ProjectError } from '../../lib/project'
import { HOST } from '../../lib/host'
import { Button, Field, IconAction, Input, Modal, useToast } from '../ui'

export interface ProjectActions {
  open: () => void
  create: () => void
  close: () => void
  recent: RecentProject[]
  openRecent: (root: string) => void
  forgetRecent: (root: string) => void
}

/** Ações de projeto do app desktop (null no navegador) e o diálogo de criação. */
export function useDesktopProjects(): { actions: ProjectActions | null; dialog: ReactNode } {
  const toast = useToast()
  const [recent, setRecent] = useState<RecentProject[]>([])
  const [createIn, setCreateIn] = useState<string | null>(null)

  const loadRecent = useCallback(() => {
    if (HOST === 'desktop') desktop.recentProjects().then(setRecent).catch(() => setRecent([]))
  }, [])
  useEffect(loadRecent, [loadRecent])

  const run = (fn: () => Promise<void>) => { fn().catch((err: unknown) => toast('error', errorMessage(err))) }

  // Trocar de projeto recarrega a janela: o estado da interface recomeça limpo.
  const openDir = async (dir: string) => { await desktop.openProject(dir); location.reload() }

  // Uma pasta que já é projeto abre direto; as demais oferecem criar o projeto.
  const choose = (title: string) => run(async () => {
    const dir = await desktop.chooseFolder(title)
    if (!dir) return
    if (await desktop.isProject(dir)) await openDir(dir)
    else setCreateIn(dir)
  })

  const actions: ProjectActions | null = HOST !== 'desktop' ? null : {
    open: () => choose('Abrir projeto'),
    create: () => choose('Pasta do novo projeto'),
    close: () => run(async () => { await desktop.closeProject(); location.reload() }),
    recent,
    openRecent: (root) => run(() => openDir(root)),
    forgetRecent: (root) => run(async () => { await desktop.forgetProject(root); loadRecent() }),
  }

  const dialog = createIn === null ? null : (
    <NewProjectDialog dir={createIn} onClose={() => setCreateIn(null)}
      onCreate={(name) => run(async () => { await desktop.createProject(createIn, name); location.reload() })} />
  )
  return { actions, dialog }
}

function folderName(dir: string): string {
  return dir.split(/[\\/]/).filter(Boolean).pop() ?? ''
}

function NewProjectDialog({ dir, onClose, onCreate }: { dir: string; onClose: () => void; onCreate: (name: string) => void }) {
  const [name, setName] = useState(() => folderName(dir))
  return (
    <Modal open onClose={onClose} title="Novo projeto"
      description="A pasta ainda não é um projeto ArchCode Studio. A estrutura .arch/, docs/ e api/ será criada nela, com uma arquitetura de exemplo."
      footer={<>
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" disabled={!name.trim()} onClick={() => onCreate(name.trim())}>Criar projeto</Button>
      </>}>
      <div className="space-y-3">
        <Field label="Nome do projeto">
          <Input autoFocus value={name} onChange={(e) => setName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter' && name.trim()) onCreate(name.trim()) }} />
        </Field>
        <p className="break-all font-mono text-[11.5px] text-muted-foreground">{dir}</p>
      </div>
    </Modal>
  )
}

/** Tela inicial do app desktop, sem projeto aberto. */
export function WelcomeScreen({ actions, error }: { actions: ProjectActions; error: ProjectError | null }) {
  // 409 = nenhum projeto aberto (situação normal); outros erros vão para a tela.
  const failure = error && error.status !== 409 ? error : null
  return (
    <div className="grid h-full place-items-center overflow-y-auto bg-background px-6 py-10">
      <div className="w-full max-w-xl">
        <div className="flex items-center gap-3">
          <span className="flex size-11 items-center justify-center rounded-md border bg-muted"><Waypoints size={22} /></span>
          <div>
            <h1 className="text-xl font-semibold">ArchCode Studio</h1>
            <p className="text-[13px] text-muted-foreground">Arquitetura de software como código, local-first e nativa para IAs.</p>
          </div>
        </div>

        {failure && (
          <p className="mt-5 rounded-md border border-destructive/40 bg-destructive/10 px-3 py-2 text-[12.5px] text-destructive">
            {failure.hint ? `${failure.message} — ${failure.hint}` : failure.message}
          </p>
        )}

        <div className="mt-6 grid gap-2 sm:grid-cols-2">
          <StartButton icon={FolderOpen} title="Abrir projeto…" hint="Uma pasta com .arch/manifest.yaml" onClick={actions.open} />
          <StartButton icon={FolderPlus} title="Novo projeto…" hint="Cria a estrutura numa pasta" onClick={actions.create} />
        </div>

        {actions.recent.length > 0 && (
          <section className="mt-7">
            <h2 className="mb-1.5 flex items-center gap-1.5 text-[11px] font-semibold uppercase tracking-wide text-muted-foreground">
              <History size={13} /> Recentes
            </h2>
            <ul className="divide-y rounded-md border">
              {actions.recent.map((p) => (
                <li key={p.root} className="group flex items-center gap-2 px-3 py-2 hover:bg-muted">
                  <button type="button" onClick={() => actions.openRecent(p.root)} className="min-w-0 flex-1 text-left">
                    <p className="truncate text-[13px] font-medium">{p.name}</p>
                    <p className="truncate font-mono text-[11px] text-muted-foreground">{p.root}</p>
                  </button>
                  <IconAction label="Remover dos recentes" onClick={() => actions.forgetRecent(p.root)}
                    className="opacity-0 group-hover:opacity-100">
                    <X size={13} />
                  </IconAction>
                </li>
              ))}
            </ul>
          </section>
        )}
      </div>
    </div>
  )
}

function StartButton({ icon: Icon, title, hint, onClick }: {
  icon: typeof FolderOpen; title: string; hint: string; onClick: () => void
}) {
  return (
    <button type="button" onClick={onClick}
      className="flex items-center gap-3 rounded-md border bg-card px-3.5 py-3 text-left shadow-xs transition-colors hover:bg-muted">
      <Icon size={18} className="shrink-0 text-muted-foreground" />
      <span>
        <span className="block text-[13px] font-medium">{title}</span>
        <span className="block text-[11.5px] text-muted-foreground">{hint}</span>
      </span>
    </button>
  )
}
