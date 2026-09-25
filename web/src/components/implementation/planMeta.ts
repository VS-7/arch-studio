// Rótulos, cores e utilidades do Módulo de Implementação (backlog, quadro,
// roadmap, retomada). Um só lugar para a interface falar a mesma língua do CLI.

import {
  Bug, BookOpen, Hammer, Layers, Lightbulb, ShieldAlert, SquareCheckBig, type LucideIcon,
} from 'lucide-react'
import type { ItemType, Priority, Sprint, TaskStatus, WorkItem } from '../../lib/types'

export const ITEM_TYPE: Record<ItemType, { label: string; icon: LucideIcon; color: string }> = {
  epic: { label: 'Épico', icon: Layers, color: '#8b5cf6' },
  story: { label: 'História', icon: BookOpen, color: '#0ea5e9' },
  task: { label: 'Tarefa', icon: SquareCheckBig, color: '#3b82f6' },
  bug: { label: 'Bug', icon: Bug, color: '#ef4444' },
  debt: { label: 'Débito técnico', icon: Hammer, color: '#f59e0b' },
  spike: { label: 'Spike', icon: Lightbulb, color: '#14b8a6' },
  security: { label: 'Segurança', icon: ShieldAlert, color: '#e11d48' },
}

export const WORK_TYPES: ItemType[] = ['task', 'bug', 'debt', 'spike', 'security']
export const ALL_TYPES: ItemType[] = ['epic', 'story', ...WORK_TYPES]

export const STATUS: Record<TaskStatus, { label: string; color: string }> = {
  pending: { label: 'A fazer', color: '#64748b' },
  in_progress: { label: 'Em andamento', color: '#f59e0b' },
  review: { label: 'Em revisão', color: '#8b5cf6' },
  completed: { label: 'Concluída', color: '#10b981' },
  blocked: { label: 'Bloqueada', color: '#ef4444' },
}

/** Colunas do quadro da sprint, na ordem do ciclo de vida. */
export const BOARD_COLUMNS: TaskStatus[] = ['pending', 'in_progress', 'review', 'completed', 'blocked']

export const PRIORITY: Record<Priority, { label: string; short: string; color: string }> = {
  must: { label: 'Must (essencial)', short: 'Must', color: '#dc2626' },
  should: { label: 'Should (importante)', short: 'Should', color: '#d97706' },
  could: { label: 'Could (desejável)', short: 'Could', color: '#0891b2' },
  wont: { label: "Won't (fora agora)", short: "Won't", color: '#64748b' },
}

export const SPRINT_STATUS: Record<Sprint['status'], string> = {
  planned: 'Planejada', active: 'Ativa', closed: 'Encerrada',
}

export function isWork(item: WorkItem): boolean {
  return item.type !== 'epic' && item.type !== 'story'
}

export function sprintName(n: number): string {
  return `Sprint ${String(n).padStart(2, '0')}`
}

/** Nome de uma identidade "Nome <email>". */
export function displayName(identity?: string): string {
  if (!identity) return ''
  const i = identity.indexOf('<')
  return (i >= 0 ? identity.slice(0, i) : identity).trim() || identity
}

export function initials(identity?: string): string {
  const name = displayName(identity)
  if (!name) return ''
  const parts = name.split(/\s+/).filter(Boolean)
  return ((parts[0]?.[0] ?? '') + (parts.length > 1 ? parts[parts.length - 1][0] : '')).toUpperCase()
}

/** Mesma pessoa? Compara pelo e-mail quando os dois têm. */
export function samePerson(a?: string, b?: string): boolean {
  if (!a || !b) return false
  const email = (s: string) => /<([^>]+)>/.exec(s)?.[1]?.toLowerCase()
  const ea = email(a), eb = email(b)
  if (ea && eb) return ea === eb
  return displayName(a).toLowerCase() === displayName(b).toLowerCase()
}

export function formatDate(iso?: string): string {
  if (!iso) return '—'
  const d = new Date(iso.length === 10 ? `${iso}T12:00:00` : iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleDateString('pt-BR', { day: '2-digit', month: 'short' })
}

export function hoursLabel(h?: number): string {
  if (!h) return '—'
  return Number.isInteger(h) ? `${h} h` : `${h.toFixed(1)} h`
}

/** Dependências ainda abertas do item (arquivadas e inexistentes não travam). */
export function blockers(item: WorkItem, byId: Map<string, WorkItem>): string[] {
  return (item.dependencies ?? []).filter((dep) => {
    const d = byId.get(dep)
    return d && !d.archived && d.status !== 'completed'
  })
}
