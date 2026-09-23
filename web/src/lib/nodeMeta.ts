import {
  Boxes, Database, Zap, Send, ShieldCheck, HardDrive, MonitorSmartphone, Link2, Group,
  type LucideIcon,
} from 'lucide-react'
import type { NodeType, Tier } from './types'

export interface NodeMeta {
  label: string
  icon: LucideIcon
  color: string
  tier: Tier
  hint: string
}

export const NODE_META: Record<NodeType, NodeMeta> = {
  compute: { label: 'Serviço', icon: Boxes, color: '#3b82f6', tier: 'backend', hint: 'Microserviço, monólito, worker ou função serverless' },
  database: { label: 'Banco de Dados', icon: Database, color: '#a855f7', tier: 'data', hint: 'SQL ou NoSQL, fonte de verdade transacional' },
  cache: { label: 'Cache', icon: Zap, color: '#f59e0b', tier: 'data', hint: 'Redis, Memcached — dados voláteis de acesso rápido' },
  queue: { label: 'Fila / Evento', icon: Send, color: '#10b981', tier: 'backend', hint: 'Kafka, RabbitMQ, SQS, webhooks' },
  gateway: { label: 'Gateway', icon: ShieldCheck, color: '#f43f5e', tier: 'integration', hint: 'API Gateway, load balancer, reverse proxy' },
  storage: { label: 'Object Storage', icon: HardDrive, color: '#06b6d4', tier: 'data', hint: 'S3, GCS, blob storage' },
  client: { label: 'Cliente', icon: MonitorSmartphone, color: '#94a3b8', tier: 'frontend', hint: 'Web SPA, app mobile, CLI, agente de IA' },
  external_service: { label: 'Serviço Externo', icon: Link2, color: '#eab308', tier: 'integration', hint: 'Stripe, Twilio, provedores SaaS de terceiros' },
  group: { label: 'Agrupamento', icon: Group, color: '#64748b', tier: 'backend', hint: 'VPC, cluster Kubernetes, fronteira de contexto' },
}

export const NODE_TYPES = Object.keys(NODE_META) as NodeType[]

export const TIERS: { value: Tier; label: string }[] = [
  { value: 'frontend', label: 'Frontend' },
  { value: 'integration', label: 'Integração' },
  { value: 'backend', label: 'Backend' },
  { value: 'domain', label: 'Domínio' },
  { value: 'devops', label: 'DevOps' },
  { value: 'data', label: 'Dados' },
]

export const PROTOCOLS = [
  'REST', 'gRPC', 'GraphQL', 'WebSocket', 'SQL', 'NoSQL',
  'Redis', 'AMQP', 'Kafka', 'Webhook', 'S3', 'TCP',
]

export const SECURITY_SCHEMES = ['', 'JWT', 'mTLS', 'OAuth2', 'API Key', 'Basic Auth', 'Nenhuma']

export const STATUS_META: Record<string, { label: string; color: string; dot: string }> = {
  pending: { label: 'Pendente', color: 'text-slate-400', dot: '#64748b' },
  in_progress: { label: 'Em andamento', color: 'text-amber-400', dot: '#f59e0b' },
  completed: { label: 'Concluído', color: 'text-emerald-400', dot: '#10b981' },
  blocked: { label: 'Bloqueado', color: 'text-rose-400', dot: '#ef4444' },
}

export function nodeMeta(type: string): NodeMeta {
  return NODE_META[type as NodeType] ?? NODE_META.compute
}
