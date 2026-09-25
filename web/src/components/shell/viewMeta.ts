// Ícones das telas de apoio: os mesmos na aba, no Model Explorer, no menu e no
// cabeçalho de cada tela.

import {
  BookOpen, Bot, Brain, FileSignature, FileText, GitBranch, GitPullRequestArrow, History, KanbanSquare, ListChecks,
  Network, Receipt, Sparkles, Workflow, type LucideIcon,
} from 'lucide-react'
import type { ViewId } from '../../lib/tabs'

export const VIEW_ICON: Record<ViewId, LucideIcon> = {
  document: FileText,
  requirements: BookOpen,
  'use-cases': ListChecks,
  adrs: GitBranch,
  'ai-prd': Bot,
  proposal: FileSignature,
  api: Network,
  pricing: Receipt,
  tasks: Workflow,
  planning: KanbanSquare,
  resume: History,
  memory: Brain,
  skills: Sparkles,
  conventions: GitPullRequestArrow,
}

/** Ordem de exibição nos menus e no Model Explorer. */
export const VIEW_ORDER: ViewId[] = ['document', 'requirements', 'use-cases', 'adrs', 'api', 'pricing', 'proposal', 'ai-prd', 'tasks']

/** Telas do Módulo de Implementação, na ordem dos menus e do Model Explorer. */
export const IMPLEMENTATION_VIEWS: ViewId[] = ['resume', 'planning', 'memory', 'skills', 'conventions']
