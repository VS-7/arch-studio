// Abas do editor central (diagramas abertos e telas de apoio).

export type ViewId = 'docs' | 'api' | 'pricing' | 'tasks'

export type TabRef =
  | { type: 'arch' }
  | { type: 'uml'; id: string }
  | { type: 'view'; view: ViewId }

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
    if (view === 'docs' || view === 'api' || view === 'pricing' || view === 'tasks') return { type: 'view', view }
  }
  return null
}

export const VIEW_LABEL: Record<ViewId, string> = {
  docs: 'Documentação',
  api: 'Contratos de API',
  pricing: 'Precificação',
  tasks: 'Implementação',
}
