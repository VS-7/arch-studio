// Área de transferência do Studio (Ctrl+C / Ctrl+X / Ctrl+V).
//
// Guarda elementos e relações com ids originais; o colar gera ids novos. Fica em
// memória e é espelhada no localStorage para copiar entre abas do navegador.

import type { ArchEdge, ArchNode, UMLElement, UMLKind, UMLRelation } from './types'

export type ClipboardData =
  | { scope: 'uml'; kind: UMLKind; elements: UMLElement[]; relations: UMLRelation[] }
  | { scope: 'arch'; nodes: ArchNode[]; edges: ArchEdge[] }

const KEY = 'archcode-clipboard'

let data: ClipboardData | null = null
let pastes = 0

export const clipboard = {
  set(next: ClipboardData) {
    data = next
    pastes = 0
    try { localStorage.setItem(KEY, JSON.stringify(next)) } catch { /* sem armazenamento: só memória */ }
  },
  get(): ClipboardData | null {
    if (data) return data
    try {
      const raw = localStorage.getItem(KEY)
      if (raw) data = JSON.parse(raw) as ClipboardData
    } catch { /* ignora */ }
    return data
  },
  /** Deslocamento em cascata a cada colagem, como no StarUML (20, 40, 60…). */
  nextOffset(): number {
    pastes += 1
    return pastes * 20
  },
}

/** Sufixo curto e aleatório para ids gerados no cliente. */
export function shortId(): string {
  return Math.random().toString(36).slice(2, 8)
}
