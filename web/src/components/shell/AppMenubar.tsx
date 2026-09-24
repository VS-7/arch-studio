// Barra de menus (Arquivo, Editar, Exibir, Modelo, Ferramentas, Ajuda), como a
// do StarUML. Cada item só dispara callbacks; o estado vive no Shell.

import {
  Bot, ClipboardPaste, Code2, Copy, CopyPlus, Download, FileImage, FilePlus2, FolderOpen, FolderPlus, FolderX, History,
  Keyboard, LayoutPanelLeft, Maximize,
  Monitor, Moon, Network, PanelsTopLeft, Plug, Presentation, Redo2, Scissors, ShieldCheck, Sparkles,
  SquareDashedMousePointer, Sun, Trash2, Undo2, Wand2, ZoomIn, ZoomOut,
} from 'lucide-react'
import { VIEW_LABEL, type ViewId } from '../../lib/tabs'
import type { ThemePreference } from '../../lib/theme'
import type { UMLKind } from '../../lib/types'
import { KIND_META, UML_KINDS } from '../../lib/umlMeta'
import {
  Menubar, MenubarCheckboxItem, MenubarContent, MenubarItem, MenubarLabel, MenubarMenu, MenubarSeparator,
  MenubarShortcut, MenubarSub, MenubarSubContent, MenubarSubTrigger, MenubarTrigger,
} from '../ui/menubar'
import { KindGlyph } from '../uml/UmlGlyph'
import type { ProjectActions } from './DesktopProjects'
import { VIEW_ICON, VIEW_ORDER } from './viewMeta'

export interface MenuActions {
  newDiagram: (kind: UMLKind) => void
  generateUseCases: () => void
  exportImage: ((format: 'png' | 'svg') => void) | null
  exportMermaid: (() => void) | null
  generatePRD: () => void
  undo: (() => void) | null
  redo: (() => void) | null
  cut: (() => void) | null
  copy: (() => void) | null
  paste: (() => void) | null
  duplicate: (() => void) | null
  selectAll: (() => void) | null
  deleteSelection: (() => void) | null
  fit: (() => void) | null
  zoomIn: (() => void) | null
  zoomOut: (() => void) | null
  autoLayout: (() => void) | null
  importMermaid: (() => void) | null
  validate: () => void
  mcp: () => void
  pitch: () => void
  shortcuts: () => void
  about: () => void
  openView: (view: ViewId) => void
  resetLayout: () => void
  /** Abrir, criar e fechar projetos — só no app desktop (null no navegador). */
  project: ProjectActions | null
}

export function AppMenubar({ actions, theme, onTheme, panels, onPanels, executive, onExecutive, archActive }: {
  actions: MenuActions
  theme: ThemePreference
  onTheme: (t: ThemePreference) => void
  panels: { left: boolean; right: boolean }
  onPanels: (p: { left: boolean; right: boolean }) => void
  executive: boolean
  onExecutive: (v: boolean) => void
  archActive: boolean
}) {
  return (
    <Menubar>
      <MenubarMenu>
        <MenubarTrigger>Arquivo</MenubarTrigger>
        <MenubarContent>
          {actions.project && <ProjectItems project={actions.project} />}
          <MenubarSub>
            <MenubarSubTrigger><FilePlus2 />Novo diagrama</MenubarSubTrigger>
            <MenubarSubContent>
              {UML_KINDS.map((k) => (
                <MenubarItem key={k} onSelect={() => actions.newDiagram(k)}>
                  <KindGlyph kind={k} size={14} />{KIND_META[k].label}
                </MenubarItem>
              ))}
            </MenubarSubContent>
          </MenubarSub>
          <MenubarItem onSelect={actions.generateUseCases}><Sparkles />Gerar casos de uso a partir das fichas</MenubarItem>
          <MenubarSeparator />
          <MenubarSub>
            <MenubarSubTrigger disabled={!actions.exportImage}><FileImage />Exportar diagrama</MenubarSubTrigger>
            <MenubarSubContent>
              <MenubarItem onSelect={() => actions.exportImage?.('png')}>PNG (2x)</MenubarItem>
              <MenubarItem onSelect={() => actions.exportImage?.('svg')}>SVG</MenubarItem>
            </MenubarSubContent>
          </MenubarSub>
          <MenubarItem disabled={!actions.exportMermaid} onSelect={() => actions.exportMermaid?.()}>
            <Code2 />Mermaid do diagrama…
          </MenubarItem>
          <MenubarSeparator />
          <MenubarItem onSelect={actions.generatePRD}><Bot />Gerar AI-PRD</MenubarItem>
        </MenubarContent>
      </MenubarMenu>

      <MenubarMenu>
        <MenubarTrigger>Editar</MenubarTrigger>
        <MenubarContent>
          <MenubarItem disabled={!actions.undo} onSelect={() => actions.undo?.()}><Undo2 />Desfazer<MenubarShortcut>Ctrl+Z</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.redo} onSelect={() => actions.redo?.()}><Redo2 />Refazer<MenubarShortcut>Ctrl+Y</MenubarShortcut></MenubarItem>
          <MenubarSeparator />
          <MenubarItem disabled={!actions.cut} onSelect={() => actions.cut?.()}><Scissors />Recortar<MenubarShortcut>Ctrl+X</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.copy} onSelect={() => actions.copy?.()}><Copy />Copiar<MenubarShortcut>Ctrl+C</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.paste} onSelect={() => actions.paste?.()}><ClipboardPaste />Colar<MenubarShortcut>Ctrl+V</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.duplicate} onSelect={() => actions.duplicate?.()}><CopyPlus />Duplicar<MenubarShortcut>Ctrl+D</MenubarShortcut></MenubarItem>
          <MenubarSeparator />
          <MenubarItem disabled={!actions.selectAll} onSelect={() => actions.selectAll?.()}><SquareDashedMousePointer />Selecionar tudo<MenubarShortcut>Ctrl+A</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.deleteSelection} onSelect={() => actions.deleteSelection?.()} variant="destructive">
            <Trash2 />Excluir seleção<MenubarShortcut>Del</MenubarShortcut>
          </MenubarItem>
        </MenubarContent>
      </MenubarMenu>

      <MenubarMenu>
        <MenubarTrigger>Exibir</MenubarTrigger>
        <MenubarContent>
          <MenubarCheckboxItem checked={panels.left} onCheckedChange={(v) => onPanels({ ...panels, left: v })}>
            Toolbox<MenubarShortcut>Ctrl+B</MenubarShortcut>
          </MenubarCheckboxItem>
          <MenubarCheckboxItem checked={panels.right} onCheckedChange={(v) => onPanels({ ...panels, right: v })}>
            Model Explorer e Editor<MenubarShortcut>Ctrl+J</MenubarShortcut>
          </MenubarCheckboxItem>
          <MenubarItem onSelect={actions.resetLayout}><LayoutPanelLeft />Redefinir layout dos painéis</MenubarItem>
          <MenubarSeparator />
          <MenubarSub>
            <MenubarSubTrigger><PanelsTopLeft />Documentação e gestão</MenubarSubTrigger>
            <MenubarSubContent>
              {VIEW_ORDER.map((view) => {
                const Icon = VIEW_ICON[view]
                return <MenubarItem key={view} onSelect={() => actions.openView(view)}><Icon />{VIEW_LABEL[view]}</MenubarItem>
              })}
            </MenubarSubContent>
          </MenubarSub>
          <MenubarSeparator />
          <MenubarItem disabled={!actions.fit} onSelect={() => actions.fit?.()}><Maximize />Ajustar à tela<MenubarShortcut>Shift+1</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.zoomIn} onSelect={() => actions.zoomIn?.()}><ZoomIn />Aproximar<MenubarShortcut>+</MenubarShortcut></MenubarItem>
          <MenubarItem disabled={!actions.zoomOut} onSelect={() => actions.zoomOut?.()}><ZoomOut />Afastar<MenubarShortcut>−</MenubarShortcut></MenubarItem>
          <MenubarSeparator />
          <MenubarCheckboxItem disabled={!archActive} checked={executive} onCheckedChange={onExecutive}>
            Visão executiva (arquitetura)
          </MenubarCheckboxItem>
          <MenubarSeparator />
          <MenubarLabel>Tema</MenubarLabel>
          <MenubarCheckboxItem checked={theme === 'light'} onCheckedChange={() => onTheme('light')}><Sun />Claro</MenubarCheckboxItem>
          <MenubarCheckboxItem checked={theme === 'dark'} onCheckedChange={() => onTheme('dark')}><Moon />Escuro</MenubarCheckboxItem>
          <MenubarCheckboxItem checked={theme === 'system'} onCheckedChange={() => onTheme('system')}><Monitor />Sistema</MenubarCheckboxItem>
        </MenubarContent>
      </MenubarMenu>

      <MenubarMenu>
        <MenubarTrigger>Modelo</MenubarTrigger>
        <MenubarContent>
          <MenubarItem onSelect={actions.validate}><ShieldCheck />Validar arquitetura…</MenubarItem>
          <MenubarSeparator />
          <MenubarItem disabled={!actions.autoLayout} onSelect={() => actions.autoLayout?.()}><Wand2 />Reorganizar arquitetura</MenubarItem>
          <MenubarItem disabled={!actions.importMermaid} onSelect={() => actions.importMermaid?.()}><Network />Mermaid da arquitetura…</MenubarItem>
        </MenubarContent>
      </MenubarMenu>

      <MenubarMenu>
        <MenubarTrigger>Ferramentas</MenubarTrigger>
        <MenubarContent>
          <MenubarItem onSelect={actions.mcp}><Plug />Conectar agente de IA (MCP)…</MenubarItem>
          <MenubarItem onSelect={actions.pitch}><Presentation />Modo apresentação</MenubarItem>
          <MenubarItem onSelect={actions.generatePRD}><Download />Gerar AI-PRD</MenubarItem>
        </MenubarContent>
      </MenubarMenu>

      <MenubarMenu>
        <MenubarTrigger>Ajuda</MenubarTrigger>
        <MenubarContent>
          <MenubarItem onSelect={actions.shortcuts}><Keyboard />Atalhos de teclado</MenubarItem>
          <MenubarItem onSelect={actions.about}>Sobre o ArchCode Studio</MenubarItem>
        </MenubarContent>
      </MenubarMenu>
    </Menubar>
  )
}

/** Itens de projeto do menu Arquivo no app desktop. */
function ProjectItems({ project }: { project: ProjectActions }) {
  return (
    <>
      <MenubarItem onSelect={project.open}><FolderOpen />Abrir projeto…<MenubarShortcut>Ctrl+O</MenubarShortcut></MenubarItem>
      <MenubarItem onSelect={project.create}><FolderPlus />Novo projeto…</MenubarItem>
      <MenubarSub>
        <MenubarSubTrigger disabled={project.recent.length === 0}><History />Projetos recentes</MenubarSubTrigger>
        <MenubarSubContent className="max-w-sm">
          {project.recent.map((p) => (
            <MenubarItem key={p.root} onSelect={() => project.openRecent(p.root)} title={p.root}>
              <span className="truncate">{p.name}</span>
            </MenubarItem>
          ))}
        </MenubarSubContent>
      </MenubarSub>
      <MenubarItem onSelect={project.close}><FolderX />Fechar projeto</MenubarItem>
      <MenubarSeparator />
    </>
  )
}
