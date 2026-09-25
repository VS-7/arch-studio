// Chamadas de domínio tipadas. Nenhum componente monta URLs manualmente.

import { client } from './transport'
import type {
  ADR, ArchEdge, ArchNode, CheckResult, ClaimResult, CloseResult, CommitProposal, CompleteResult, Conventions,
  ConventionsPreview, Criterion, Diagram, DoctorReport, Endpoint, EndpointsSpec, Estimate, DocumentMeta, GitOverview,
  Handoff, LintIssue, LintReport, Note, Plan, PricingConfig, PRProposal, ReconcileChange, ReqDocument, Requirement,
  RequiredCheck, RequirementsDoc, ResumeView, Session, Skill, SkillsOverview, Snapshot, Sprint, SprintPlanResult, SprintStatusView,
  SyncResult, Task, UMLDiagram, UMLElement, UMLKind, UMLRelation, UseCase, WorkItem,
} from './types'

const request = client.request.bind(client)

export interface NodeInput {
  label?: string
  type?: string
  technology?: string
  description?: string
  tier?: string
  tags?: string[]
  complexity?: string
  estimated_hours?: number
  cloud_tier?: string
  monthly_cost?: number
  requirements?: string[]
  use_cases?: string[]
  executive?: boolean
  status?: string
  connect_to?: string
  protocol?: string
  position?: { x: number; y: number }
}

export interface SvgOptions {
  mode?: 'executive' | 'engineering'
  theme?: 'light' | 'dark'
  transparent?: boolean
  download?: boolean
  title?: boolean
}

export interface EdgeInput {
  source_id: string
  target_id: string
  protocol?: string
  port?: number
  description?: string
  security?: string
  label?: string
  complexity?: string
  estimated_hours?: number
  animated?: boolean
}

/** Campos que a interface altera num item do backlog (ausente = não muda). */
export interface ItemInput {
  type?: string
  title?: string
  description?: string
  status?: string
  priority?: string
  sprint?: number
  parent?: string
  assignee?: string
  estimate_h?: number
  component_id?: string
  dependencies?: string[]
  requirements?: string[]
  acceptance?: Criterion[]
  notes?: string
  archived?: boolean
}

const enc = encodeURIComponent

function svgPath(opts: SvgOptions): string {
  const params = new URLSearchParams()
  if (opts.mode) params.set('mode', opts.mode)
  if (opts.theme) params.set('theme', opts.theme)
  if (opts.transparent) params.set('transparent', '1')
  if (opts.download) params.set('download', '1')
  if (opts.title === false) params.set('title', '0')
  return `/api/export/svg?${params.toString()}`
}

/**
 * As figuras do documento vêm com caminhos do servidor; viram URLs absolutas
 * para que a pré-visualização, a impressão (iframe sem origem própria) e o
 * DOCX as encontrem em qualquer host.
 */
function withFigureUrls(doc: ReqDocument): ReqDocument {
  return {
    ...doc,
    blocks: doc.blocks.map((b) => (b.type === 'image' ? { ...b, src: client.resourceUrl(b.src) } : b)),
  }
}

/**
 * Servidores antigos serializavam listas vazias como `null` (slice nil do Go).
 * Normaliza aqui para que nenhuma tela precise se defender campo a campo.
 */
function normalizeEstimate(e: Estimate): Estimate {
  return {
    ...e,
    roles: e.roles ?? [],
    cloud_items: e.cloud_items ?? [],
    items: e.items ?? [],
    by_tier: e.by_tier ?? {},
    by_type: e.by_type ?? {},
  }
}

export const api = {
  /**
   * `mcp_sse_url` vazio (ou ausente em servidores antigos) = MCP via SSE não exposto;
   * `mcp_command` é o executável do MCP via stdio (a CLI ou o app desktop).
   */
  health: () => request<{
    status: string; version: string; root: string; frontend: boolean
    host?: 'web' | 'desktop'; mcp_sse_url?: string; mcp_command?: string
  }>('GET', '/api/health'),
  snapshot: () => request<Snapshot>('GET', '/api/snapshot'),

  // Diagrama
  getDiagram: () => request<Diagram>('GET', '/api/diagram'),
  saveDiagram: (d: Diagram) => request<{ saved: boolean }>('PUT', '/api/diagram', d),
  addNode: (input: NodeInput) => request<ArchNode>('POST', '/api/diagram/nodes', input),
  updateNode: (id: string, input: NodeInput) =>
    request<ArchNode>('PATCH', `/api/diagram/nodes/${encodeURIComponent(id)}`, input),
  deleteNode: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/diagram/nodes/${encodeURIComponent(id)}`),
  addEdge: (input: EdgeInput) =>
    request<{ edge: ArchEdge; endpoints: Endpoint[] }>('POST', '/api/diagram/edges', input),
  deleteEdge: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/diagram/edges/${encodeURIComponent(id)}`),
  autoLayout: () => request<Diagram>('POST', '/api/diagram/autolayout', {}),
  /** Reorganiza a arquitetura e todos os diagramas UML; devolve o que mudou. */
  autoLayoutAll: () => request<{ architecture: boolean; diagrams: string[] }>('POST', '/api/autolayout', {}),
  importMermaid: (source: string) => request<Diagram>('POST', '/api/diagram/import-mermaid', { source }),

  // Diagramas UML
  listUML: () => request<UMLDiagram[]>('GET', '/api/uml'),
  getUML: (id: string) => request<UMLDiagram>('GET', `/api/uml/${encodeURIComponent(id)}`),
  createUML: (input: { kind: UMLKind; name: string; description?: string }) =>
    request<UMLDiagram>('POST', '/api/uml', input),
  saveUML: (d: UMLDiagram) => request<{ saved: boolean }>('PUT', `/api/uml/${encodeURIComponent(d.id)}`, d),
  renameUML: (id: string, input: { name?: string; description?: string }) =>
    request<UMLDiagram>('PATCH', `/api/uml/${encodeURIComponent(id)}`, input),
  deleteUML: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/uml/${encodeURIComponent(id)}`),
  autoLayoutUML: (id: string) => request<UMLDiagram>('POST', `/api/uml/${encodeURIComponent(id)}/autolayout`, {}),
  umlMermaid: (id: string) => request<{ mermaid: string }>('GET', `/api/uml/${encodeURIComponent(id)}/mermaid`),
  generateUseCaseDiagram: (name?: string) =>
    request<UMLDiagram>('POST', '/api/uml/generate/use-cases', name ? { name } : {}),
  addUMLElement: (diagramId: string, el: Partial<UMLElement>) =>
    request<UMLElement>('POST', `/api/uml/${encodeURIComponent(diagramId)}/elements`, el),
  updateUMLElement: (diagramId: string, id: string, patch: Partial<UMLElement>) =>
    request<UMLElement>('PATCH', `/api/uml/${encodeURIComponent(diagramId)}/elements/${encodeURIComponent(id)}`, patch),
  deleteUMLElement: (diagramId: string, id: string) =>
    request<{ deleted: boolean }>('DELETE', `/api/uml/${encodeURIComponent(diagramId)}/elements/${encodeURIComponent(id)}`),
  addUMLRelation: (diagramId: string, rel: Partial<UMLRelation>) =>
    request<UMLRelation>('POST', `/api/uml/${encodeURIComponent(diagramId)}/relations`, rel),
  updateUMLRelation: (diagramId: string, id: string, patch: Partial<UMLRelation>) =>
    request<UMLRelation>('PATCH', `/api/uml/${encodeURIComponent(diagramId)}/relations/${encodeURIComponent(id)}`, patch),
  deleteUMLRelation: (diagramId: string, id: string) =>
    request<{ deleted: boolean }>('DELETE', `/api/uml/${encodeURIComponent(diagramId)}/relations/${encodeURIComponent(id)}`),

  // Documento de Requisitos
  reqDocument: () => request<ReqDocument>('GET', '/api/reqdoc').then(withFigureUrls),
  getDocumentMeta: () => request<DocumentMeta>('GET', '/api/reqdoc/meta'),
  saveDocumentMeta: (meta: DocumentMeta) => request<DocumentMeta>('PUT', '/api/reqdoc/meta', meta),
  saveReqDocument: () => request<{ file: string; images: string[] }>('POST', '/api/reqdoc/save', {}),
  reqDocumentMarkdown: (embed = true) => client.fetchBlob(`/api/reqdoc/markdown${embed ? '?embed=1' : ''}`),
  /** Figura do documento (URL já resolvida por `reqDocument`). */
  fetchFigure: (url: string) => client.fetchBlob(url),

  // Documentação
  getRequirements: () => request<{ doc: RequirementsDoc; raw: string }>('GET', '/api/requirements'),
  upsertRequirement: (req: Requirement) => request<Requirement>('POST', '/api/requirements', req),
  saveRequirementsRaw: (content: string) => request<{ saved: boolean }>('PUT', '/api/requirements/raw', { content }),
  deleteRequirement: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/requirements/${encodeURIComponent(id)}`),

  listUseCases: () => request<UseCase[]>('GET', '/api/use-cases'),
  upsertUseCase: (uc: UseCase) => request<{ use_case: UseCase; file: string }>('POST', '/api/use-cases', uc),
  deleteUseCase: (code: string) => request<{ deleted: boolean }>('DELETE', `/api/use-cases/${encodeURIComponent(code)}`),

  listADRs: () => request<ADR[]>('GET', '/api/adrs'),
  upsertADR: (adr: ADR) => request<{ adr: ADR; file: string }>('POST', '/api/adrs', adr),
  deleteADR: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/adrs/${encodeURIComponent(id)}`),

  // Contratos
  listEndpoints: () => request<EndpointsSpec>('GET', '/api/endpoints'),
  upsertEndpoint: (ep: Endpoint) => request<Endpoint>('POST', '/api/endpoints', ep),
  deleteEndpoint: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/endpoints/${encodeURIComponent(id)}`),
  exportOpenAPI: () => request<{ file_path: string }>('POST', '/api/endpoints/openapi', {}),

  // Precificação
  getPricing: () => request<PricingConfig>('GET', '/api/pricing'),
  savePricing: (cfg: PricingConfig) => request<PricingConfig>('PUT', '/api/pricing', cfg),
  estimate: (margin?: number) =>
    request<Estimate>('GET', `/api/estimate${margin !== undefined && margin >= 0 ? `?margin=${margin}` : ''}`)
      .then(normalizeEstimate),
  generateProposal: (opts: {
    client_name?: string; validity_days?: number; margin?: number
    include_diagram: boolean; include_cloud: boolean; notes?: string
  }) => request<{ file_path: string; markdown: string; total_cost: number; currency: string }>('POST', '/api/proposal', opts),

  // AI-PRD
  generatePRD: (opts: { target_stack?: string; include_test_scenarios?: boolean; granularity?: string }) =>
    request<{ file_path: string; total_tasks: number; hash: string; overall_progress_percentage: number; summary: string }>(
      'POST', '/api/ai-prd', opts),
  getTasks: (status = 'all') =>
    request<{ tasks: Task[]; total: number; progress: number; generated_at: string; target_stack: string }>(
      'GET', `/api/tasks?status=${encodeURIComponent(status)}`),
  setTaskStatus: (id: string, status: string, notes?: string) =>
    request<{ task_id: string; updated: boolean; status: string; overall_progress_percentage: number }>(
      'POST', `/api/tasks/${encodeURIComponent(id)}/status`, { status, notes }),

  // Qualidade
  validate: () => request<LintReport>('GET', '/api/validate'),

  // Arquivos brutos
  readFile: (path: string) => request<{ path: string; content: string }>('GET', `/api/file?path=${encodeURIComponent(path)}`),
  writeFile: (path: string, content: string) => request<{ saved: boolean }>('PUT', '/api/file', { path, content }),

  // Exportação
  exportSvg: (opts: SvgOptions): Promise<string> => client.fetchBlob(svgPath(opts)).then((b) => b.text()),

  // Módulo de Implementação — backlog
  getPlan: () => request<Plan>('GET', '/api/plan'),
  syncBacklog: (dryRun: boolean) => request<SyncResult>('POST', '/api/plan/sync', { dry_run: dryRun }),
  createItem: (input: ItemInput) => request<WorkItem>('POST', '/api/plan/items', input),
  getItem: (id: string) => request<WorkItem>('GET', `/api/plan/items/${enc(id)}`),
  updateItem: (id: string, input: ItemInput) => request<WorkItem>('PATCH', `/api/plan/items/${enc(id)}`, input),
  deleteItem: (id: string) => request<{ deleted: boolean }>('DELETE', `/api/plan/items/${enc(id)}`),
  moveItem: (id: string, input: { before?: string; after?: string; sprint?: number; status?: string }) =>
    request<WorkItem>('POST', `/api/plan/items/${enc(id)}/move`, input),
  claimItem: (id: string, opts: { switch?: boolean; push?: boolean; force?: boolean }) =>
    request<ClaimResult>('POST', `/api/plan/items/${enc(id)}/claim`, opts),
  releaseItem: (id: string, opts: { reason?: string; force?: boolean }) =>
    request<WorkItem>('POST', `/api/plan/items/${enc(id)}/release`, opts),
  checkpointItem: (id: string, handoff: Handoff) => request<WorkItem>('POST', `/api/plan/items/${enc(id)}/checkpoint`, handoff),
  completeItem: (id: string, input: { checks?: CheckResult[]; notes?: string; status?: string; force?: boolean }) =>
    request<CompleteResult>('POST', `/api/plan/items/${enc(id)}/complete`, input),
  itemPrompt: (id: string) => request<{ prompt: string }>('GET', `/api/plan/items/${enc(id)}/prompt`),
  itemChecks: (id: string) => request<RequiredCheck[]>('GET', `/api/plan/items/${enc(id)}/checks`),

  // Sprints
  createSprint: (input: { goal?: string; start?: string; end?: string; capacity_h?: number }) =>
    request<Sprint>('POST', '/api/sprints', input),
  planSprint: (input: { number?: number; goal?: string; capacity_h?: number; ids?: string[]; apply: boolean }) =>
    request<SprintPlanResult>('POST', '/api/sprints/plan', input),
  getSprint: (n: number) => request<SprintStatusView>('GET', `/api/sprints/${n}`),
  updateSprint: (n: number, input: { goal?: string; start?: string; end?: string; capacity_h?: number }) =>
    request<Sprint>('PATCH', `/api/sprints/${n}`, input),
  startSprint: (n: number) => request<Sprint>('POST', `/api/sprints/${n}/start`, {}),
  closeSprint: (n: number, carryTo: number) => request<CloseResult>('POST', `/api/sprints/${n}/close`, { carry_to: carryTo }),

  // Memória
  resume: (detail: 'brief' | 'full' = 'brief') => request<ResumeView>('GET', `/api/resume?detail=${detail}`),
  sessions: (limit = 30) => request<Session[]>('GET', `/api/sessions?limit=${limit}`),
  logSession: (input: Partial<Session> & { summary: string }) => request<Session>('POST', '/api/sessions', input),
  memory: (q = '') => request<Note[]>('GET', `/api/memory${q ? `?q=${enc(q)}` : ''}`),
  remember: (input: { slug?: string; title: string; body: string; type?: string; tags?: string[]; components?: string[] }) =>
    request<Note>('POST', '/api/memory', input),
  forget: (slug: string) => request<{ deleted: boolean }>('DELETE', `/api/memory/${enc(slug)}`),

  // Convenções e Git
  getConventions: () => request<{ conventions: Conventions; exists: boolean; preview: ConventionsPreview }>('GET', '/api/conventions'),
  saveConventions: (c: Conventions) => request<Conventions>('PUT', '/api/conventions', c),
  initConventions: (preset: string, force = false) => request<Conventions>('POST', '/api/conventions/init', { preset, force }),
  previewConventions: (c: Conventions) => request<ConventionsPreview>('POST', '/api/conventions/preview', c),
  applyConventions: (ci: boolean) => request<{ written: string[] }>('POST', '/api/conventions/apply', { ci }),
  git: () => request<GitOverview>('GET', '/api/git'),
  gitLint: (input: { range?: string; branch?: string; pr_title?: string } = {}) =>
    request<{ checked: number; issues: LintIssue[]; errors: number; ok: boolean }>('POST', '/api/git/lint', input),
  installHooks: (force = false) => request<{ dir: string; installed: string[]; skipped?: string[]; notes?: string[] }>('POST', '/api/git/hooks', { force }),
  uninstallHooks: () => request<{ removed: string[] }>('DELETE', '/api/git/hooks'),
  commit: (input: { task_id?: string; summary?: string; body?: string; commit?: boolean; plan?: boolean }) =>
    request<CommitProposal>('POST', '/api/git/commit', input),
  pullRequest: (input: { ids?: string[]; open?: boolean; draft?: boolean }) => request<PRProposal>('POST', '/api/git/pr', input),
  reconcile: (apply: boolean) => request<{ changes: ReconcileChange[]; applied: boolean }>('POST', '/api/git/reconcile', { apply }),
  changelog: (write: boolean) => request<{ content: string; file: string; written: boolean }>('POST', '/api/git/changelog', { write }),

  // Skills e agentes
  skills: () => request<SkillsOverview>('GET', '/api/skills'),
  getSkill: (name: string) => request<{ skill: Skill; content: string }>('GET', `/api/skills/${enc(name)}`),
  createSkill: (input: { name: string; description: string; category: string; trigger: string }) =>
    request<Skill>('POST', '/api/skills', input),
  installSkills: (names: string[], update = false) => request<{ installed: string[] }>('POST', '/api/skills/install', { names, update }),
  saveSkill: (name: string, content: string) => request<Skill>('PUT', `/api/skills/${enc(name)}`, { content }),
  setSkillEnabled: (name: string, enabled: boolean) => request<Skill>('PATCH', `/api/skills/${enc(name)}`, { enabled }),
  removeSkill: (name: string) => request<{ deleted: boolean }>('DELETE', `/api/skills/${enc(name)}`),
  syncSkills: (targets: string[]) => request<{ written: string[]; removed: string[] }>('POST', '/api/skills/sync', { targets }),
  agentSetup: (agent: 'claude' | 'cursor') => request<{ agent: string; written: string[]; notes?: string[] }>('POST', '/api/agents/setup', { agent }),
  doctor: (input: { prefer?: string; fix?: boolean } = {}) => request<DoctorReport>('POST', '/api/doctor', input),
}
