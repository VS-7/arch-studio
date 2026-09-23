// Ícones das telas de apoio: os mesmos na aba, no Model Explorer, no menu e no
// cabeçalho de cada tela.

import {
  BookOpen, Bot, FileSignature, FileText, GitBranch, ListChecks, Network, Receipt, Workflow, type LucideIcon,
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
}

/** Ordem de exibição nos menus e no Model Explorer. */
export const VIEW_ORDER: ViewId[] = ['document', 'requirements', 'use-cases', 'adrs', 'api', 'pricing', 'proposal', 'ai-prd', 'tasks']
