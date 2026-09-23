// Vocabulário do Documento de Requisitos: prioridades (Essencial/Importante/
// Desejável, como no modelo IEEE usado pelo documento) e categorias de RNF.
// Espelha reqdoc.PriorityLevel do backend.

import type { PriorityLevel } from './types'

/** Valores gravados em disco (compatíveis com projetos existentes) e seus rótulos. */
export const PRIORITIES: { value: string; label: string; level: PriorityLevel }[] = [
  { value: 'Alta', label: 'Essencial (alta)', level: 'essencial' },
  { value: 'Média', label: 'Importante (média)', level: 'importante' },
  { value: 'Baixa', label: 'Desejável (baixa)', level: 'desejavel' },
]

export const PRIORITY_LABEL: Record<PriorityLevel, string> = {
  essencial: 'Essencial', importante: 'Importante', desejavel: 'Desejável',
}

export function priorityLevel(p?: string): PriorityLevel {
  const v = (p ?? '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').trim()
  if (['alta', 'essencial', 'critica', 'high', 'must'].includes(v)) return 'essencial'
  if (['baixa', 'desejavel', 'low', 'could', "won't", 'wont'].includes(v)) return 'desejavel'
  return 'importante'
}

export const RNF_CATEGORIES = [
  'Usabilidade', 'Confiabilidade', 'Desempenho', 'Segurança', 'Software', 'Suportabilidade',
  'Portabilidade', 'Interface', 'Legal',
]
