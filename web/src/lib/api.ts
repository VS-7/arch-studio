// Chamadas de domínio tipadas. Nenhum componente monta URLs manualmente.

import { client } from './transport'
import type {
  ADR, ArchEdge, ArchNode, Diagram, Endpoint, EndpointsSpec, Estimate,
  DocumentMeta, LintReport, PricingConfig, ReqDocument, Requirement, RequirementsDoc, Snapshot, Task, UMLDiagram, UMLElement,
  UMLKind, UMLRelation, UseCase,
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
}
