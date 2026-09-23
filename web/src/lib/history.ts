// Desfazer/Refazer (Ctrl+Z / Ctrl+Y) por diagrama.
//
// Em vez de instrumentar cada operação, o histórico observa o snapshot: sempre
// que o conteúdo de um diagrama muda (pela UI, por um agente de IA ou no disco),
// o estado anterior entra na pilha de desfazer. Desfazer regrava esse estado com
// um PUT do diagrama inteiro — o mesmo caminho usado ao arrastar elementos —,
// então funciona para qualquer edição, inclusive as feitas pelo Editor.

import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from './api'
import type { Snapshot } from './types'

const LIMIT = 100

/** 'arch' para o diagrama macro, 'uml:<id>' para os diagramas UML (mesmo formato das abas). */
export type HistoryKey = string

interface Stacks { undo: string[]; redo: string[] }

/** Conteúdo versionável de cada diagrama (sem viewport nem carimbo de data). */
function collect(snapshot: Snapshot): Map<HistoryKey, string> {
  const out = new Map<HistoryKey, string>()
  out.set('arch', JSON.stringify({ nodes: snapshot.diagram.nodes, edges: snapshot.diagram.edges }))
  for (const d of snapshot.uml_diagrams ?? []) {
    out.set(`uml:${d.id}`, JSON.stringify({ elements: d.elements, relations: d.relations }))
  }
  return out
}

export interface History {
  undo: (key: HistoryKey) => Promise<boolean>
  redo: (key: HistoryKey) => Promise<boolean>
  canUndo: (key: HistoryKey) => boolean
  canRedo: (key: HistoryKey) => boolean
}

export function useHistory(snapshot: Snapshot | null): History {
  const stacks = useRef(new Map<HistoryKey, Stacks>())
  const last = useRef(new Map<HistoryKey, string>())
  // Diagramas cuja próxima mudança é o próprio desfazer/refazer (não empilhar).
  const applying = useRef(new Map<HistoryKey, number>())
  const snap = useRef<Snapshot | null>(snapshot)
  const [, setVersion] = useState(0)
  snap.current = snapshot

  const get = (key: HistoryKey): Stacks => {
    let st = stacks.current.get(key)
    if (!st) { st = { undo: [], redo: [] }; stacks.current.set(key, st) }
    return st
  }

  useEffect(() => {
    if (!snapshot) return
    const current = collect(snapshot)
    for (const [key, content] of current) {
      const prev = last.current.get(key)
      if (prev !== undefined && prev !== content) {
        if (applying.current.has(key)) {
          window.clearTimeout(applying.current.get(key))
          applying.current.delete(key)
        } else {
          const st = get(key)
          st.undo.push(prev)
          if (st.undo.length > LIMIT) st.undo.shift()
          st.redo = []
        }
      }
      last.current.set(key, content)
    }
    // Diagramas excluídos levam o histórico junto.
    for (const key of [...last.current.keys()]) {
      if (!current.has(key)) { last.current.delete(key); stacks.current.delete(key) }
    }
    setVersion((v) => v + 1)
  }, [snapshot])

  const save = useCallback(async (key: HistoryKey, content: string) => {
    const s = snap.current
    if (!s) throw new Error('projeto não carregado')
    const parsed = JSON.parse(content)
    if (key === 'arch') {
      await api.saveDiagram({ ...s.diagram, ...parsed })
      return
    }
    const d = s.uml_diagrams.find((x) => `uml:${x.id}` === key)
    if (!d) throw new Error('o diagrama não existe mais')
    await api.saveUML({ ...d, ...parsed })
  }, [])

  const step = useCallback(async (key: HistoryKey, dir: 'undo' | 'redo') => {
    const st = get(key)
    const from = dir === 'undo' ? st.undo : st.redo
    const to = dir === 'undo' ? st.redo : st.undo
    const target = from.pop()
    const current = last.current.get(key)
    if (target === undefined || current === undefined) return false
    to.push(current)
    // Se o servidor não gerar mudança (conteúdo idêntico), libera a marca.
    applying.current.set(key, window.setTimeout(() => applying.current.delete(key), 4000))
    try {
      await save(key, target)
    } catch (err) {
      to.pop()
      from.push(target)
      applying.current.delete(key)
      throw err
    } finally {
      setVersion((v) => v + 1)
    }
    return true
  }, [save])

  return {
    undo: useCallback((key) => step(key, 'undo'), [step]),
    redo: useCallback((key) => step(key, 'redo'), [step]),
    canUndo: (key) => (stacks.current.get(key)?.undo.length ?? 0) > 0,
    canRedo: (key) => (stacks.current.get(key)?.redo.length ?? 0) > 0,
  }
}
