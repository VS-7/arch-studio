// Peças comuns aos dois canvases (arquitetura e UML). Só o que é idêntico nos
// dois vive aqui; o que depende do modelo (seleção por componentes ou por
// elementos, criação, menus próprios) continua em cada canvas.

import type { Edge, Node } from '@xyflow/react'
import { Copy, CopyPlus, Scissors, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useRef, type Dispatch, type SetStateAction } from 'react'
import type { Viewport } from '../../lib/types'
import { useToast } from '../ui'
import type { CanvasCommands, CanvasMenuItem } from './CanvasMenu'

/** Props do React Flow no estilo StarUML: seleção com Ctrl/Shift, teclas tratadas pelo Shell. */
export const FLOW_DEFAULTS = {
  snapToGrid: true,
  panOnScroll: true,
  multiSelectionKeyCode: ['Control', 'Meta', 'Shift'],
  selectionKeyCode: null,
  deleteKeyCode: null,
  zoomOnDoubleClick: false,
  proOptions: { hideAttribution: true },
}

/** Viewport nunca ajustado pelo usuário (0,0,1): o canvas enquadra o diagrama ao montar. */
export function isFreshViewport(vp?: Viewport | null): boolean {
  return !vp || (vp.x === 0 && vp.y === 0 && (!vp.zoom || vp.zoom === 1))
}

type Setter<T> = Dispatch<SetStateAction<T[]>>

/** Marca como selecionados exatamente os ids informados. */
export function markSelected<N extends Node, E extends Edge>(setNodes: Setter<N>, setEdges: Setter<E>, ids: Set<string>) {
  setNodes((ns) => ns.map((n) => ({ ...n, selected: ids.has(n.id) })))
  setEdges((es) => es.map((e) => ({ ...e, selected: ids.has(e.id) })))
}

/**
 * Seleção vinda de fora (Model Explorer, Editor) reflete no canvas. `remember`
 * atualiza a seleção interna do canvas com o id escolhido.
 */
export function useExternalSelection<N extends Node, E extends Edge>(
  selectedId: string | null, setNodes: Setter<N>, setEdges: Setter<E>, remember: (id: string) => void,
) {
  const rememberRef = useRef(remember)
  rememberRef.current = remember
  useEffect(() => {
    if (selectedId === null) return
    rememberRef.current(selectedId)
    setNodes((ns) => ns.map((n) => (n.selected === (n.id === selectedId) ? n : { ...n, selected: n.id === selectedId })))
    setEdges((es) => es.map((e) => (e.selected === (e.id === selectedId) ? e : { ...e, selected: e.id === selectedId })))
  }, [selectedId, setNodes, setEdges])
}

/** Executa um comando do canvas e mostra o resultado (menus de contexto). */
export function useRunCommand() {
  const toast = useToast()
  return useCallback((fn: () => Promise<string | null> | string | null) => {
    void Promise.resolve(fn()).then((msg) => { if (msg) toast('success', `${msg} · Ctrl+Z desfaz`) })
  }, [toast])
}

/** Itens de edição comuns aos menus de contexto. */
export function editMenuItems(commands: CanvasCommands, run: ReturnType<typeof useRunCommand>): CanvasMenuItem[] {
  return [
    { type: 'item', label: 'Recortar', icon: <Scissors />, shortcut: 'Ctrl+X', onSelect: () => run(commands.cut) },
    { type: 'item', label: 'Copiar', icon: <Copy />, shortcut: 'Ctrl+C', onSelect: () => run(commands.copy) },
    { type: 'item', label: 'Duplicar', icon: <CopyPlus />, shortcut: 'Ctrl+D', onSelect: () => run(commands.duplicate) },
    { type: 'separator' },
    { type: 'item', label: 'Excluir', icon: <Trash2 />, shortcut: 'Del', destructive: true, onSelect: () => run(commands.deleteSelected) },
  ]
}
