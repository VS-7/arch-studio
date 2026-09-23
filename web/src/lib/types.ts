// Espelho TypeScript do modelo canônico definido em internal/model/types.go.
// Mantenha os dois lados em sincronia: eles descrevem os mesmos arquivos em disco.

export type NodeType =
  | 'compute' | 'database' | 'cache' | 'queue'
  | 'gateway' | 'storage' | 'client' | 'external_service' | 'group'

export type Tier = 'data' | 'domain' | 'backend' | 'integration' | 'frontend' | 'devops'
export type Complexity = 'low' | 'medium' | 'high'
export type TaskStatus = 'pending' | 'in_progress' | 'completed' | 'blocked'

export interface Position { x: number; y: number }
export interface Viewport { x: number; y: number; zoom: number }

export interface NodePricing {
  complexity?: Complexity
  estimated_hours?: number
  cloud_tier?: string
  role?: string
  monthly_cost?: number
}

export interface NodeData {
  label: string
  technology?: string
  description?: string
  tier?: Tier
  tags?: string[]
  executive?: boolean
  status?: TaskStatus
  requirements?: string[]
  use_cases?: string[]
  pricing?: NodePricing
  [key: string]: unknown
}

export interface ArchNode {
  id: string
  type: NodeType
  position: Position
  data: NodeData
  parentId?: string
  width?: number
  height?: number
  style?: Record<string, unknown>
}

export interface EdgeData {
  protocol?: string
  port?: number
  description?: string
  security?: string
  endpoints?: string[]
  complexity?: Complexity
  estimated_hours?: number
  executive?: boolean
  [key: string]: unknown
}

export interface ArchEdge {
  id: string
  source: string
  target: string
  type?: string
  animated: boolean
  label?: string
  data: EdgeData
}

export interface Diagram {
  version: string
  last_modified: string
  viewport: Viewport
  nodes: ArchNode[]
  edges: ArchEdge[]
}

export interface Manifest {
  schema_version: string
  project_name: string
  description: string
  version: string
  authors: { name: string; role?: string }[]
  settings: {
    diagram_engine: string
    sync_mermaid: boolean
    auto_layout_on_import: boolean
    pricing_currency: string
  }
}

export interface Requirement {
  id: string
  type: 'RF' | 'RNF'
  title: string
  priority?: string
  status?: TaskStatus
  components?: string[]
  description?: string
}

export interface RequirementsDoc {
  project_name: string
  overview: string
  requirements: Requirement[]
}

export interface UseCase {
  code: string
  name: string
  file?: string
  actors?: string[]
  components?: string[]
  complexity?: Complexity
  estimated_hours?: number
  priority?: string
  status?: TaskStatus
  pre_conditions?: string[]
  main_flow?: string[]
  alternate_flows?: string[]
  exceptions?: string[]
  business_rules?: string[]
  acceptance?: string[]
}

export interface ADR {
  id: string
  title: string
  file?: string
  status?: string
  date?: string
  context?: string
  decision?: string
  consequences?: string
}

export interface Endpoint {
  id: string
  method: string
  path: string
  summary?: string
  description?: string
  source?: string
  target?: string
  edge_id?: string
  auth?: string
  request?: string
  response?: string
  status_codes?: number[]
  use_cases?: string[]
}

export interface EndpointsSpec {
  version: string
  base_url?: string
  endpoints: Endpoint[]
}

export interface PricingConfig {
  currency: string
  hourly_rates: Record<string, number>
  complexity_multipliers: Record<string, number>
  node_type_hours: Record<string, number>
  complexity_factors: Record<string, number>
  protocol_hours: Record<string, number>
  use_case_hours: Record<string, number>
  cloud_catalog: Record<string, number>
  role_distribution: Record<string, number>
  risk_margin_percentage: number
  tax_percentage: number
  team_size: number
  hours_per_day: number
  working_days_per_month: number
}

export interface LineItem {
  id: string
  label: string
  kind: 'node' | 'edge' | 'use_case'
  category: string
  complexity?: string
  hours: number
  explicit: boolean
}

export interface Estimate {
  currency: string
  node_hours: number
  edge_hours: number
  use_case_hours: number
  base_hours: number
  risk_margin_percentage: number
  margin_hours: number
  total_hours: number
  roles: { role: string; share: number; hours: number; rate: number; subtotal: number }[]
  personnel_cost: number
  tax_percentage: number
  tax_amount: number
  total_cost: number
  blended_rate: number
  cloud_monthly_cost: number
  cloud_yearly_cost: number
  cloud_items: { node_id: string; label: string; cloud_tier: string; monthly_cost: number }[]
  team_size: number
  hours_per_day: number
  working_days: number
  calendar_weeks: number
  calendar_months: number
  estimated_finish: string
  items: LineItem[]
  by_tier: Record<string, number>
  by_type: Record<string, number>
  generated_at: string
  warnings?: string[]
}

export interface Task {
  id: string
  title: string
  tier: string
  component?: string
  component_id?: string
  dependencies?: string[]
  acceptance?: string[]
  requirements?: string[]
  use_cases?: string[]
  endpoints?: string[]
  status: TaskStatus
  notes?: string
  updated_at?: string
  order: number
  ready?: boolean
  blocked_by?: string[]
}

export interface TaskBoard {
  version: string
  generated_at: string
  source_hash: string
  target_stack?: string
  tasks: Task[]
}

export interface Snapshot {
  manifest: Manifest
  diagram: Diagram
  requirements: RequirementsDoc
  use_cases: UseCase[]
  adrs: ADR[]
  endpoints: EndpointsSpec
  pricing: PricingConfig
  tasks: TaskBoard
  mermaid: string
  ai_prd_exists: boolean
}

export interface Finding {
  rule: string
  severity: 'error' | 'warning' | 'info'
  target?: string
  target_id?: string
  message: string
  fix?: string
}

export interface LintReport {
  findings: Finding[]
  errors: number
  warnings: number
  infos: number
  passed: boolean
  score: number
}

export interface ServerEvent {
  type: string
  source: 'ui' | 'ai' | 'disk' | 'cli'
  path?: string
  message?: string
  payload?: unknown
  at: string
}
