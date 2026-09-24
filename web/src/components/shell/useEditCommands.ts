// Edição do diagrama aberto: desfazer/refazer e comandos do canvas ativo.

import { useCallback } from 'react'
import { errorMessage } from '../../lib/errors'
import type { useHistory } from '../../lib/history'
import type { TabRef } from '../../lib/tabs'
import type { UMLDiagram } from '../../lib/types'
import type { CanvasHandle } from '../canvas/ArchCanvas'
import type { CanvasCommands } from '../canvas/CanvasMenu'
import { useToast } from '../ui'
import type { UmlCanvasHandle } from '../uml/UmlCanvas'

export type EditCommand = keyof Pick<CanvasCommands, 'deleteSelected' | 'copy' | 'cut' | 'paste' | 'duplicate'>

export function useEditCommands({ history, active, activeDiagram, archHandle, umlHandle, onRemoved }: {
  history: ReturnType<typeof useHistory>
  active: TabRef | null
  activeDiagram: UMLDiagram | null
  archHandle: CanvasHandle | null
  umlHandle: UmlCanvasHandle | null
  /** Chamado quando excluir/recortar tira a seleção do diagrama. */
  onRemoved: () => void
}) {
  const toast = useToast()

  // Chave de histórico e comandos do diagrama aberto (arquitetura ou UML).
  const historyKey = active?.type === 'arch' ? 'arch' : activeDiagram ? `uml:${activeDiagram.id}` : null
  const commands: CanvasCommands | null = active?.type === 'arch' ? archHandle : activeDiagram ? umlHandle : null

  const undo = useCallback(async () => {
    if (!historyKey) return
    if (!history.canUndo(historyKey)) { toast('info', 'Nada para desfazer'); return }
    try { await history.undo(historyKey) } catch (err) { toast('error', `Não foi possível desfazer: ${errorMessage(err)}`) }
  }, [history, historyKey, toast])

  const redo = useCallback(async () => {
    if (!historyKey) return
    if (!history.canRedo(historyKey)) { toast('info', 'Nada para refazer'); return }
    try { await history.redo(historyKey) } catch (err) { toast('error', `Não foi possível refazer: ${errorMessage(err)}`) }
  }, [history, historyKey, toast])

  /** Executa um comando do canvas e anuncia o resultado; mudanças oferecem "Desfazer". */
  const run = useCallback(async (name: EditCommand) => {
    if (!commands || !historyKey) return
    const key = historyKey
    try {
      const msg = await commands[name]()
      if (!msg) return
      if (name === 'copy') toast('info', msg)
      else toast('success', msg, { label: 'Desfazer', onClick: () => void history.undo(key) })
      if (name === 'deleteSelected' || name === 'cut') onRemoved()
    } catch (err) { toast('error', errorMessage(err)) }
  }, [commands, history, historyKey, onRemoved, toast])

  return {
    commands,
    undo,
    redo,
    run,
    canUndo: !!historyKey && history.canUndo(historyKey),
    canRedo: !!historyKey && history.canRedo(historyKey),
  }
}
