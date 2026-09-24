// Atalhos de teclado do Shell. Um único mapa alimenta o tratamento das teclas e
// a lista do diálogo "Atalhos de teclado", para que os dois nunca divirjam.
// As ações vêm de MenuActions: ação `null` (indisponível) não consome a tecla.

import { useEffect, useRef } from 'react'
import type { MenuActions } from './AppMenubar'

export interface ShortcutHandlers {
  actions: MenuActions
  undo: () => void
  redo: () => void
  toggleLeft: () => void
  toggleRight: () => void
  cancelTool: () => void
  /** Renomear o elemento UML selecionado; null quando não se aplica. */
  rename: (() => void) | null
}

interface Binding {
  keys: string
  description: string
  match: (e: KeyboardEvent, key: string, mod: boolean) => boolean
  run: (h: ShortcutHandlers) => (() => void) | null
  /** Vale também com o foco num campo de texto. */
  global?: boolean
  /** Deixa o navegador tratar a tecla também (padrão: não deixa). */
  passThrough?: boolean
}

const KEYMAP: Binding[] = [
  { keys: 'Ctrl + O', description: 'Abrir projeto (app desktop)', global: true,
    match: (_, k, mod) => mod && k === 'o', run: (h) => h.actions.project?.open ?? null },
  { keys: 'Esc', description: 'Voltar à ferramenta Selecionar / cancelar relação',
    match: (e) => e.key === 'Escape', run: (h) => h.cancelTool, passThrough: true },
  { keys: 'Ctrl + B', description: 'Mostrar/ocultar Toolbox', global: true,
    match: (_, k, mod) => mod && k === 'b', run: (h) => h.toggleLeft },
  { keys: 'Ctrl + J', description: 'Mostrar/ocultar Model Explorer e Editor', global: true,
    match: (_, k, mod) => mod && k === 'j', run: (h) => h.toggleRight },
  { keys: 'Ctrl + Z', description: 'Desfazer — vale para qualquer mudança, inclusive as feitas por IA',
    match: (e, k, mod) => mod && k === 'z' && !e.shiftKey, run: (h) => h.undo },
  { keys: 'Ctrl + Y / Ctrl + Shift + Z', description: 'Refazer',
    match: (e, k, mod) => mod && (k === 'y' || (k === 'z' && e.shiftKey)), run: (h) => h.redo },
  { keys: 'Ctrl + C', description: 'Copiar a seleção (com as relações entre os elementos)',
    match: (_, k, mod) => mod && k === 'c', run: (h) => h.actions.copy },
  { keys: 'Ctrl + X', description: 'Recortar a seleção',
    match: (_, k, mod) => mod && k === 'x', run: (h) => h.actions.cut },
  { keys: 'Ctrl + V', description: 'Colar',
    match: (_, k, mod) => mod && k === 'v', run: (h) => h.actions.paste },
  { keys: 'Ctrl + D', description: 'Duplicar a seleção',
    match: (_, k, mod) => mod && k === 'd', run: (h) => h.actions.duplicate },
  { keys: 'Ctrl + A', description: 'Selecionar tudo no diagrama',
    match: (_, k, mod) => mod && k === 'a', run: (h) => h.actions.selectAll },
  { keys: 'Del / Backspace', description: 'Excluir toda a seleção (desfazível)',
    match: (e) => e.key === 'Delete' || e.key === 'Backspace', run: (h) => h.actions.deleteSelection },
  { keys: 'F2', description: 'Renomear o elemento selecionado (também com duplo clique)',
    match: (e) => e.key === 'F2', run: (h) => h.rename },
  { keys: 'Ctrl + Shift + L', description: 'Reorganizar o diagrama para caber na página do documento',
    match: (e, k, mod) => mod && e.shiftKey && k === 'l', run: (h) => h.actions.autoLayout },
  { keys: 'Shift + 1', description: 'Ajustar o diagrama à tela',
    match: (e) => e.shiftKey && e.code === 'Digit1', run: (h) => h.actions.fit, passThrough: true },
  { keys: '+', description: 'Aproximar',
    match: (e, _, mod) => !mod && (e.key === '+' || e.key === '='), run: (h) => h.actions.zoomIn, passThrough: true },
  { keys: '−', description: 'Afastar',
    match: (e, _, mod) => !mod && e.key === '-', run: (h) => h.actions.zoomOut, passThrough: true },
]

/** Gestos de mouse, listados no diálogo junto com os atalhos. */
const MOUSE_HINTS: [string, string][] = [
  ['Clique direito', 'Menu de contexto: adicionar aqui, adicionar conectado, editar'],
  ['Duplo clique no vazio', 'Adicionar um elemento na posição do cursor'],
  ['Ctrl/Shift + clique', 'Adicionar ou remover da seleção'],
  ['Arrastar no vazio', 'Selecionar vários elementos com uma caixa'],
  ['Botão do meio / scroll', 'Mover o canvas'],
  ['Arrastar a borda do painel', 'Redimensionar Toolbox, barra lateral e a altura Model Explorer/Editor (duplo clique restaura)'],
  ['Clique no título do painel', 'Recolher/expandir o Model Explorer ou o Editor (o botão ⤢ maximiza)'],
]

/** Linhas do diálogo "Atalhos de teclado". */
export const SHORTCUT_ROWS: [string, string][] = [
  ...KEYMAP.map((b): [string, string] => [b.keys, b.description]),
  ...MOUSE_HINTS,
]

export function useShortcuts(handlers: ShortcutHandlers, { enabled }: { enabled: boolean }) {
  // Handlers mudam a cada render; o listener é registrado uma vez e lê o atual.
  const ref = useRef(handlers)
  ref.current = handlers

  useEffect(() => {
    if (!enabled) return
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement
      const mod = e.ctrlKey || e.metaKey
      const key = e.key.toLowerCase()
      // Campo recém-focado e ainda não editado (ex.: nome do elemento criado) não
      // "segura" o Ctrl+Z: o usuário espera desfazer a criação.
      const pristine = mod && (key === 'z' || key === 'y') && target.dataset.pristine === 'true'
      const typing = !pristine && !!target.closest('input, textarea, select, [contenteditable="true"], [role="dialog"]')

      const binding = KEYMAP.find((b) => b.match(e, key, mod))
      if (!binding || (typing && !binding.global)) return
      const action = binding.run(ref.current)
      if (!action) return
      if (!binding.passThrough) e.preventDefault()
      action()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [enabled])
}
