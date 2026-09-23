// Abas do editor central (diagramas abertos e telas de apoio).
//
// Cada tela de documentação é uma aba própria, como os diagramas: dá para abrir
// o Documento de Requisitos ao lado dos Casos de Uso e alternar entre eles sem
// perder o espaço da área central para um segundo nível de abas.

export type ViewId =
  | 'document' | 'requirements' | 'use-cases' | 'adrs' | 'ai-prd' | 'proposal'
  | 'api' | 'pricing' | 'tasks'

export type TabRef =
  | { type: 'arch' }
  | { type: 'uml'; id: string }
  | { type: 'view'; view: ViewId }

export const VIEW_LABEL: Record<ViewId, string> = {
  document: 'Documento de Requisitos',
  requirements: 'Requisitos',
  'use-cases': 'Casos de Uso',
  adrs: 'Decisões (ADR)',
  'ai-prd': 'AI-PRD',
  proposal: 'Proposta Comercial',
  api: 'Contratos de API',
  pricing: 'Precificação',
  tasks: 'Implementação',
}

const VIEWS = new Set(Object.keys(VIEW_LABEL))

export function tabKey(tab: TabRef): string {
  switch (tab.type) {
    case 'arch': return 'arch'
    case 'uml': return `uml:${tab.id}`
    case 'view': return `view:${tab.view}`
  }
}

export function parseTabKey(key: string): TabRef | null {
  if (key === 'arch') return { type: 'arch' }
  if (key.startsWith('uml:')) return { type: 'uml', id: key.slice(4) }
  if (key.startsWith('view:')) {
    const view = key.slice(5)
    // "view:docs" é a antiga aba única de Documentação, com sub-abas internas.
    if (view === 'docs') return { type: 'view', view: 'document' }
    if (VIEWS.has(view)) return { type: 'view', view: view as ViewId }
  }
  return null
}

/** Reescreve chaves gravadas por versões anteriores no formato atual. */
export function normalizeTabKey(key: string): string | null {
  const tab = parseTabKey(key)
  return tab ? tabKey(tab) : null
}
