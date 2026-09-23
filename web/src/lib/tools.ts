// Ferramenta ativa da Toolbox (compartilhada entre a Toolbox e os canvases).

import type { NodeType, UMLElementType, UMLRelationType } from './types'

export type Tool =
  | { mode: 'select' }
  | { mode: 'element'; type: UMLElementType }
  | { mode: 'relation'; type: UMLRelationType }
  | { mode: 'arch'; type: NodeType }

export const SELECT_TOOL: Tool = { mode: 'select' }

/** Seleção corrente em um editor de diagrama. */
export type Selection =
  | { scope: 'uml'; diagramId: string; kind: 'element' | 'relation'; id: string }
  | { scope: 'arch'; kind: 'node' | 'edge'; id: string }
  | null
