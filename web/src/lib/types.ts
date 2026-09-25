// Espelho TypeScript do modelo canônico definido em internal/model/types.go.
// Mantenha os dois lados em sincronia: eles descrevem os mesmos arquivos em disco.

export type NodeType =
  | 'compute' | 'database' | 'cache' | 'queue'
  | 'gateway' | 'storage' | 'client' | 'external_service' | 'group'

export type Tier = 'data' | 'domain' | 'backend' | 'integration' | 'frontend' | 'devops'
export type Complexity = 'low' | 'medium' | 'high'
export type TaskStatus = 'pending' | 'in_progress' | 'review' | 'completed' | 'blocked'

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
  /** Agrupamento dos RNFs no documento (Usabilidade, Desempenho…). */
  category?: string
  /** "Requisitos associados"; ["Todos"] = todos. */
  related?: string[]
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
  description?: string
  /** Requisitos associados (RF/RNF). */
  requirements?: string[]
  post_conditions?: string[]
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

// ---------------------------------------------------------------------------
// Diagramas UML (.arch/diagrams/{usecase,class,sequence,state}/<id>.json)
// ---------------------------------------------------------------------------

export type UMLKind = 'usecase' | 'class' | 'sequence' | 'state'

export type UMLElementType =
  | 'actor' | 'usecase' | 'boundary' | 'note'
  | 'class' | 'interface' | 'enum' | 'package'
  | 'lifeline' | 'fragment'
  | 'state' | 'initial' | 'final' | 'choice' | 'fork' | 'join' | 'history'

export type UMLRelationType =
  | 'association' | 'directed_association' | 'include' | 'extend' | 'generalization'
  | 'realization' | 'dependency' | 'aggregation' | 'composition'
  | 'message' | 'transition' | 'note_link'

export type Visibility = '+' | '-' | '#' | '~'
export type MessageKind = 'sync' | 'async' | 'reply' | 'create' | 'destroy'
export type LifelineKind = 'participant' | 'actor' | 'boundary' | 'control' | 'entity' | 'database'
export type FragmentOperator = 'alt' | 'opt' | 'loop' | 'par' | 'break' | 'critical' | 'ref'

export interface UMLMember {
  name: string
  type?: string
  visibility?: Visibility
  static?: boolean
  abstract?: boolean
  default?: string
  params?: string
}

export interface UMLElement {
  id: string
  type: UMLElementType
  name: string
  position: Position
  width?: number
  height?: number
  parent_id?: string
  stereotype?: string
  documentation?: string
  abstract?: boolean
  attributes?: UMLMember[]
  operations?: UMLMember[]
  literals?: string[]
  entry?: string
  do?: string
  exit?: string
  lifeline_kind?: LifelineKind
  operator?: FragmentOperator
  guard?: string
  use_case?: string
  component_id?: string
}

export interface UMLRelation {
  id: string
  type: UMLRelationType
  source: string
  target: string
  name?: string
  source_multiplicity?: string
  target_multiplicity?: string
  source_role?: string
  target_role?: string
  message_kind?: MessageKind
  order?: number
  trigger?: string
  guard?: string
  effect?: string
  documentation?: string
}

export interface UMLDiagram {
  id: string
  kind: UMLKind
  name: string
  description?: string
  version: string
  last_modified: string
  viewport: Viewport
  elements: UMLElement[]
  relations: UMLRelation[]
  file?: string
}

// ---------------------------------------------------------------------------
// Documento de Requisitos (.arch/document.yaml + gerador internal/reqdoc)
// ---------------------------------------------------------------------------

export interface DocRevision { date: string; version: string; description: string; author: string }
export interface GlossaryTerm { term: string; definition: string }

export interface DocumentMeta {
  title: string
  version: string
  date: string
  authors: string[]
  client: string
  users: string
  introduction: string
  history: DocRevision[]
  references: string[]
  glossary: GlossaryTerm[]
}

export type PriorityLevel = 'essencial' | 'importante' | 'desejavel'

export type DocBlock =
  | { type: 'heading'; level: 1 | 2 | 3 | 4; number?: string; text: string; id: string }
  | { type: 'paragraph'; text: string }
  | { type: 'list'; ordered: boolean; start?: number; items: string[] }
  | { type: 'table'; header: string[]; rows: string[][] }
  | { type: 'priority'; value: PriorityLevel }
  | { type: 'image'; src: string; caption: string; diagram: string }
  | { type: 'pagebreak' }

export interface ReqDocument {
  title: string
  project: string
  version: string
  date: string
  authors: string[]
  history: DocRevision[]
  toc: { id: string; number: string; title: string; level: number }[]
  blocks: DocBlock[]
  markdown: string
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
  uml_diagrams: UMLDiagram[]
  document?: DocumentMeta
  /** Backlog e sprints (.arch/plan/). Ausente em servidores antigos. */
  plan?: Plan
  conventions?: Conventions
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

// ---------------------------------------------------------------------------
// Módulo de Implementação (.arch/plan/, .arch/conventions.yaml, .arch/skills/,
// .arch/sessions/, .arch/memory/) — espelho de internal/model/plan.go e cia.
// ---------------------------------------------------------------------------

export type ItemType = 'epic' | 'story' | 'task' | 'bug' | 'debt' | 'spike' | 'security'
export type Priority = 'must' | 'should' | 'could' | 'wont'

export interface Criterion { text: string; done: boolean }

export interface Handoff {
  last_step?: string
  next_step?: string
  files?: string[]
  failing_tests?: string[]
  notes?: string
  updated_at?: string
  by?: string
}

export interface CheckResult { skill: string; check: string; result: 'ok' | 'fail' | 'na'; evidence?: string }

export interface WorkItem {
  id: string
  type: ItemType
  title: string
  status: TaskStatus
  priority?: Priority | ''
  sprint?: number
  rank: string
  parent?: string
  assignee?: string
  agent?: string
  estimate_h?: number
  tier?: string
  component?: string
  component_id?: string
  dependencies?: string[]
  requirements?: string[]
  use_cases?: string[]
  endpoints?: string[]
  branch?: string
  source?: string
  overrides?: string[]
  aliases?: string[]
  order?: number
  archived?: boolean
  archive_reason?: string
  created_at?: string
  updated_at?: string
  claimed_at?: string
  completed_at?: string
  description?: string
  acceptance?: Criterion[]
  handoff?: Handoff
  checks?: CheckResult[]
  notes?: string
  file?: string
  // Calculados pelo servidor nas listagens.
  ready?: boolean
  blocked_by?: string[]
  stale?: boolean
}

export type SprintStatus = 'planned' | 'active' | 'closed'

export interface Sprint {
  number: number
  name: string
  goal?: string
  start?: string
  end?: string
  status: SprintStatus
  capacity_h?: number
  started_at?: string
  closed_at?: string
  file?: string
}

export interface Plan { items: WorkItem[]; sprints: Sprint[]; warnings?: string[] }

export interface PlanStats {
  total: number; pending: number; in_progress: number; review: number; completed: number; blocked: number
  planned_hours: number; done_hours: number; progress: number
}

export interface SyncChange { id: string; title: string; type: ItemType; kind: 'added' | 'updated' | 'archived' | 'restored'; fields?: string[] }
export interface SyncResult { dry_run: boolean; changes: SyncChange[]; counts: Record<string, number>; total_items: number; slicing: string }

export interface SprintPlanResult {
  sprint: Sprint
  proposal: { items: string[]; hours: number; capacity_h: number; skipped?: string[] }
  items: WorkItem[]
  applied: boolean
  warnings?: string[]
}

export interface SprintStatusView {
  sprint?: Sprint
  stats: PlanStats
  days_left: number
  items: WorkItem[]
  load_h: number
  sprints: Sprint[]
  backlog: PlanStats
}

export interface CloseResult {
  sprint: Sprint; report_file: string; carried: string[]; carried_to: number; stats: PlanStats
  changelog_file?: string; suggested_tag: string; tag_command: string
}

export interface ClaimResult {
  task: WorkItem; branch: string; branch_created: boolean; switched: boolean; pushed: boolean
  reservation_commit?: string; remote: 'confirmed' | 'unconfirmed' | 'none'
  warnings?: string[]; next_command?: string; skills?: string[]
}

export interface RequiredCheck { skill: string; check: string; text: string; command?: string }

export interface CompleteResult {
  task?: WorkItem; completed: boolean; status?: TaskStatus; missing_checks?: string[]; failed_checks?: string[]
  required_checks?: RequiredCheck[]; warnings?: string[]; message: string; next_steps?: string[]
}

export type Autonomy = 'assistido' | 'supervisionado' | 'autonomo'

export interface Conventions {
  preset: 'archcode-sprint' | 'conventional-commits'
  language: string
  planning: { sprint_days: number; slicing: 'component' | 'hybrid'; stale_days: number }
  git: {
    main_branch: string; remote: string; branch: string; branch_off_sprint: string
    commit: string; commit_off_sprint: string; off_sprint_kinds?: string[]; max_subject: number
    verbs?: string[]; require_id: boolean; branch_exempt?: string[]
  }
  pull_request: { title: string; granularity: 'task' | 'story'; merge_strategy: string }
  tags: { sprint: string; release: string }
  ai: { autonomy: Autonomy }
}

export interface ConventionsPreview {
  branch: string; branch_off_sprint: string; commit: string; commit_off_sprint: string
  pr_title: string; sprint_tag: string; errors?: string[]
}

export interface GitFileChange { path: string; code: string }
export interface GitStatus {
  branch: string; detached?: boolean; upstream?: string; ahead: number; behind: number
  changed: GitFileChange[]; staged: number; unstaged: number; untracked: number; conflicts: number
}
export interface GitCommit { hash: string; parents: number; author: string; email: string; date: string; subject: string; body?: string }
export interface LintIssue { level: 'error' | 'warning'; subject: string; ref?: string; message: string }

export interface GitOverview {
  available: boolean; status?: GitStatus; branch_issues?: LintIssue[]; current_task?: WorkItem
  person?: string; hooks: string[]; forge: boolean; recent?: GitCommit[]
}

export interface CommitProposal {
  task_id: string; subject: string; message: string; branch?: string; current_branch?: string
  staged: string[]; warnings?: string[]; committed: boolean; hash?: string; command?: string
}

export interface PRProposal {
  title: string; body: string; base: string; head: string; items: string[]
  warnings?: string[]; opened: boolean; url?: string; command?: string
}

export interface ReconcileChange { id: string; title: string; from: TaskStatus; to: TaskStatus; reason: string }

export interface SkillCheck { id: string; text: string; command?: string; required: boolean }
export interface Skill {
  name: string; description: string; category: 'seguranca' | 'organizacao' | 'padronizacao' | 'studio'
  version?: string; trigger: 'sempre' | 'ao_iniciar_tarefa' | 'antes_do_pr'; scope?: string
  applies_to?: { tiers?: string[]; stacks?: string[] }; checks?: SkillCheck[]; disabled?: boolean; source?: string
  body: string; installed: boolean; builtin: boolean; update_available?: boolean; file?: string
}
export interface SkillsOverview {
  installed: Skill[]; catalog: Skill[]; stacks: string[]; suggested: string[]; warnings?: string[]
  synced: { claude: boolean; agents: boolean; cursor: boolean }
}

export interface Session {
  id: string; author?: string; agent?: string; started?: string; ended?: string; sprint?: number
  tasks?: string[]; branch?: string; commits?: string[]; summary?: string; done?: string[]; decisions?: string[]
  next_steps?: string[]; blockers?: string[]; files?: string[]; commands?: string[]; checks?: CheckResult[]; file?: string
}

export interface Note {
  slug: string; title: string; type: 'decisao' | 'convencao' | 'armadilha' | 'contexto' | 'glossario'
  tags?: string[]; components?: string[]; author?: string; created?: string; updated?: string; body: string; file?: string
}

export interface ResumeTask {
  id: string; title: string; type: ItemType; status: TaskStatus; sprint?: number; assignee?: string
  component?: string; branch?: string; estimate_h?: number; stale?: boolean; blocked_by?: string[]
  handoff?: Handoff; acceptance?: Criterion[]; dependencies?: string[]; requirements?: string[]
  endpoints?: string[]; suggested_branch?: string
}

export interface ResumeView {
  project: string; person?: string; detail: 'brief' | 'full'; autonomy: Autonomy
  sprint?: { number: number; name: string; goal?: string; start?: string; end?: string; days_left: number; stats: PlanStats; capacity_h?: number }
  my_work: ResumeTask[]; next?: ResumeTask
  last_session?: { id: string; author?: string; agent?: string; started?: string; summary?: string; tasks?: string[]; next_steps?: string[]; blockers?: string[] }
  team_sessions?: { id: string; author?: string; agent?: string; started?: string; summary?: string; tasks?: string[]; next_steps?: string[]; blockers?: string[] }[]
  stale_claims?: ResumeTask[]
  git: { available: boolean; branch?: string; upstream?: string; ahead?: number; behind?: number; changed?: string[]; changed_count: number; last_commit?: string }
  divergences?: string[]; skills?: { name: string; trigger: string; description?: string; required_checks?: number }[]
  memories?: { slug: string; title: string; type: string; summary?: string }[]
  hints: string[]; backlog_size: number
}

export interface DoctorReport { problems: string[]; conflicts: string[]; resolved?: string[]; regenerated?: string[] }
