// Package httpapi expõe o núcleo do ArchCode Studio via HTTP + WebSocket.
//
// É um adaptador (RNF005): cada handler só traduz HTTP ⇄ chamadas do pacote
// `app`. O mesmo handler atende o navegador (`archcode-studio serve`) e a janela
// do app desktop, que o usa como servidor de assets do Wails.
package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/store"
)

// Options configura o que o servidor anuncia e entrega além da API.
type Options struct {
	Version string
	// Assets é o frontend compilado; nil responde "frontend não embutido".
	Assets fs.FS
	// Host identifica quem abre a interface ("web" ou "desktop"). Vai para o
	// index.html como <meta name="archcode-host">, e o frontend escolhe o
	// transporte de eventos a partir dele.
	Host string
	// MCPSSEURL é o endereço do MCP via SSE, anunciado em /api/health ("" se
	// o transporte não estiver exposto).
	MCPSSEURL string
	// MCPCommand é o executável que agentes de IA rodam em stdio
	// (`<comando> mcp --dir <projeto>`): o CLI ou o próprio app desktop.
	MCPCommand string
}

type Server struct {
	app  *app.App
	hub  *hub.Hub
	mux  *http.ServeMux
	opts Options
}

func New(a *app.App, h *hub.Hub, opts Options) *Server {
	s := &Server{app: a, hub: h, mux: http.NewServeMux(), opts: opts}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler { return guardOrigin(logging(s.mux)) }

// Mount permite pendurar handlers extras (ex.: o transporte SSE do MCP).
func (s *Server) Mount(pattern string, h http.Handler) { s.mux.Handle(pattern, h) }

func (s *Server) routes() {
	m := s.mux

	m.HandleFunc("GET /api/health", s.health)
	m.HandleFunc("GET /api/snapshot", s.snapshot)

	// Diagrama
	m.HandleFunc("GET /api/diagram", s.getDiagram)
	m.HandleFunc("PUT /api/diagram", s.putDiagram)
	m.HandleFunc("POST /api/diagram/nodes", s.addNode)
	m.HandleFunc("PATCH /api/diagram/nodes/{id}", s.updateNode)
	m.HandleFunc("DELETE /api/diagram/nodes/{id}", s.deleteNode)
	m.HandleFunc("POST /api/diagram/edges", s.addEdge)
	m.HandleFunc("DELETE /api/diagram/edges/{id}", s.deleteEdge)
	m.HandleFunc("POST /api/diagram/autolayout", s.autoLayout)
	m.HandleFunc("POST /api/diagram/import-mermaid", s.importMermaid)
	m.HandleFunc("GET /api/diagram/mermaid", s.getMermaid)

	// Diagramas UML
	m.HandleFunc("GET /api/uml", s.listUML)
	m.HandleFunc("POST /api/uml", s.createUML)
	m.HandleFunc("POST /api/uml/generate/use-cases", s.generateUseCaseUML)
	m.HandleFunc("GET /api/uml/{id}", s.getUML)
	m.HandleFunc("GET /api/uml/{id}/mermaid", s.getUMLMermaid)
	m.HandleFunc("PUT /api/uml/{id}", s.putUML)
	m.HandleFunc("PATCH /api/uml/{id}", s.patchUML)
	m.HandleFunc("DELETE /api/uml/{id}", s.deleteUML)
	m.HandleFunc("POST /api/uml/{id}/elements", s.addUMLElement)
	m.HandleFunc("PATCH /api/uml/{id}/elements/{eid}", s.updateUMLElement)
	m.HandleFunc("DELETE /api/uml/{id}/elements/{eid}", s.deleteUMLElement)
	m.HandleFunc("POST /api/uml/{id}/relations", s.addUMLRelation)
	m.HandleFunc("PATCH /api/uml/{id}/relations/{rid}", s.updateUMLRelation)
	m.HandleFunc("DELETE /api/uml/{id}/relations/{rid}", s.deleteUMLRelation)

	// Documentação
	m.HandleFunc("GET /api/requirements", s.getRequirements)
	m.HandleFunc("POST /api/requirements", s.upsertRequirement)
	m.HandleFunc("PUT /api/requirements/raw", s.putRequirementsRaw)
	m.HandleFunc("DELETE /api/requirements/{id}", s.deleteRequirement)

	m.HandleFunc("GET /api/use-cases", s.listUseCases)
	m.HandleFunc("POST /api/use-cases", s.upsertUseCase)
	m.HandleFunc("DELETE /api/use-cases/{code}", s.deleteUseCase)

	// Documento de requisitos
	m.HandleFunc("GET /api/reqdoc", s.getReqDoc)
	m.HandleFunc("GET /api/reqdoc/meta", s.getReqDocMeta)
	m.HandleFunc("PUT /api/reqdoc/meta", s.putReqDocMeta)
	m.HandleFunc("POST /api/reqdoc/save", s.saveReqDoc)
	m.HandleFunc("GET /api/reqdoc/markdown", s.getReqDocMarkdown)

	m.HandleFunc("GET /api/adrs", s.listADRs)
	m.HandleFunc("POST /api/adrs", s.upsertADR)
	m.HandleFunc("DELETE /api/adrs/{id}", s.deleteADR)

	// Contratos de API
	m.HandleFunc("GET /api/endpoints", s.listEndpoints)
	m.HandleFunc("POST /api/endpoints", s.upsertEndpoint)
	m.HandleFunc("DELETE /api/endpoints/{id}", s.deleteEndpoint)
	m.HandleFunc("POST /api/endpoints/openapi", s.exportOpenAPI)

	// Precificação
	m.HandleFunc("GET /api/pricing", s.getPricing)
	m.HandleFunc("PUT /api/pricing", s.putPricing)
	m.HandleFunc("GET /api/estimate", s.getEstimate)
	m.HandleFunc("POST /api/proposal", s.generateProposal)

	// PRD para IA e tarefas
	m.HandleFunc("POST /api/ai-prd", s.generatePRD)
	m.HandleFunc("GET /api/tasks", s.getTasks)
	m.HandleFunc("POST /api/tasks/{id}/status", s.setTaskStatus)

	// Qualidade
	m.HandleFunc("GET /api/validate", s.validate)

	// Arquivos brutos (editor de documentos)
	m.HandleFunc("GET /api/file", s.getFile)
	m.HandleFunc("PUT /api/file", s.putFile)

	// Exportação
	m.HandleFunc("GET /api/export/svg", s.exportSVG)
	m.HandleFunc("GET /api/export/uml/{file}", s.exportUMLSVG)

	// Tempo real
	m.HandleFunc("/ws", s.serveWS)

	// Frontend embutido
	m.Handle("/", s.staticHandler())
}

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		// A resposta já começou; só resta registrar.
		fmt.Printf("httpapi: erro ao serializar resposta: %v\n", err)
	}
}

type apiError struct {
	Error string `json:"error"`
	Hint  string `json:"hint,omitempty"`
}

// fail responde o erro no formato {error, hint}. O tipo do erro decide o status
// (404 para inexistente, 400 para inválido, 403 para caminho proibido); o
// status informado vale para os demais.
func fail(w http.ResponseWriter, status int, err error) {
	switch {
	case errors.Is(err, model.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, model.ErrInvalid):
		status = http.StatusBadRequest
	case errors.Is(err, app.ErrPathNotAllowed):
		status = http.StatusForbidden
	}
	hint := ""
	if errors.Is(err, store.ErrNotAProject) {
		hint = "Rode `archcode-studio init` neste diretório para criar a estrutura .arch/."
	}
	writeJSON(w, status, apiError{Error: err.Error(), Hint: hint})
}

func decode[T any](r *http.Request, dst *T) error {
	defer func() { _ = r.Body.Close() }()
	dec := json.NewDecoder(io.LimitReader(r.Body, 16<<20))
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("corpo da requisição inválido: %w", err)
	}
	return nil
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Local-first: nenhuma origem externa é aceita por padrão (RNF007).
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------------------
// Handlers — estado geral
// ---------------------------------------------------------------------------

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "ok",
		"version":     s.opts.Version,
		"root":        s.app.Root(),
		"is_project":  s.app.IsProject(),
		"clients":     s.hub.Count(),
		"frontend":    s.opts.Assets != nil,
		"host":        s.host(),
		"mcp_sse_url": s.opts.MCPSSEURL,
		"mcp_command": s.opts.MCPCommand,
		"server_time": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) snapshot(w http.ResponseWriter, r *http.Request) {
	snap, err := s.app.Snapshot()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// ---------------------------------------------------------------------------
// Handlers — diagrama
// ---------------------------------------------------------------------------

func (s *Server) getDiagram(w http.ResponseWriter, r *http.Request) {
	d, err := s.app.Diagram()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) putDiagram(w http.ResponseWriter, r *http.Request) {
	var d model.Diagram
	if err := decode(r, &d); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.app.ReplaceDiagram(&d, hub.SourceUI); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": true, "nodes": len(d.Nodes), "edges": len(d.Edges)})
}

func (s *Server) addNode(w http.ResponseWriter, r *http.Request) {
	var in app.NodeInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	node, err := s.app.AddNode(in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) updateNode(w http.ResponseWriter, r *http.Request) {
	var in app.NodeInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	node, err := s.app.UpdateNode(r.PathValue("id"), in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) deleteNode(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RemoveNode(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) addEdge(w http.ResponseWriter, r *http.Request) {
	var in app.EdgeInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	edge, endpoints, err := s.app.ConnectNodes(in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"edge": edge, "endpoints": endpoints})
}

func (s *Server) deleteEdge(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RemoveEdge(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) autoLayout(w http.ResponseWriter, r *http.Request) {
	d, err := s.app.AutoLayout(hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) importMermaid(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source string `json:"source"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	d, err := s.app.ImportMermaid(body.Source, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) getMermaid(w http.ResponseWriter, r *http.Request) {
	text, err := s.app.DiagramMermaid()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(text))
}

// ---------------------------------------------------------------------------
// Handlers — documentação
// ---------------------------------------------------------------------------

func (s *Server) getRequirements(w http.ResponseWriter, r *http.Request) {
	doc, err := s.app.Requirements()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"doc": doc, "raw": s.app.RequirementsRaw()})
}

func (s *Server) upsertRequirement(w http.ResponseWriter, r *http.Request) {
	var req model.Requirement
	if err := decode(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, created, err := s.app.UpsertRequirement(req, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, saved)
}

func (s *Server) putRequirementsRaw(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.app.SaveRequirementsMarkdown(body.Content, hub.SourceUI); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteRequirement(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteRequirement(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listUseCases(w http.ResponseWriter, r *http.Request) {
	list, err := s.app.UseCases()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) upsertUseCase(w http.ResponseWriter, r *http.Request) {
	var uc model.UseCase
	if err := decode(r, &uc); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, path, err := s.app.UpsertUseCase(uc, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"use_case": saved, "file": path})
}

func (s *Server) deleteUseCase(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteUseCase(r.PathValue("code"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listADRs(w http.ResponseWriter, r *http.Request) {
	list, err := s.app.ADRs()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) upsertADR(w http.ResponseWriter, r *http.Request) {
	var adr model.ADR
	if err := decode(r, &adr); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, path, err := s.app.UpsertADR(adr, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"adr": saved, "file": path})
}

func (s *Server) deleteADR(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteADR(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// ---------------------------------------------------------------------------
// Handlers — contratos de API
// ---------------------------------------------------------------------------

func (s *Server) listEndpoints(w http.ResponseWriter, r *http.Request) {
	spec, err := s.app.Endpoints()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, spec)
}

func (s *Server) upsertEndpoint(w http.ResponseWriter, r *http.Request) {
	var ep model.Endpoint
	if err := decode(r, &ep); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.app.UpsertEndpoint(ep, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deleteEndpoint(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteEndpoint(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) exportOpenAPI(w http.ResponseWriter, r *http.Request) {
	path, err := s.app.ExportOpenAPI(hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"file_path": path})
}

// ---------------------------------------------------------------------------
// Handlers — precificação
// ---------------------------------------------------------------------------

func (s *Server) getPricing(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.app.PricingConfig()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) putPricing(w http.ResponseWriter, r *http.Request) {
	var cfg model.PricingConfig
	if err := decode(r, &cfg); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.app.SavePricingConfig(&cfg, hub.SourceUI); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cfg)
}

func (s *Server) getEstimate(w http.ResponseWriter, r *http.Request) {
	margin := -1.0
	if v := r.URL.Query().Get("margin"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			margin = f
		}
	}
	est, err := s.app.Estimate(margin)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, est)
}

func (s *Server) generateProposal(w http.ResponseWriter, r *http.Request) {
	var opts app.ProposalOptions
	if err := decode(r, &opts); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.GenerateProposal(opts, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// Handlers — AI-PRD e tarefas
// ---------------------------------------------------------------------------

func (s *Server) generatePRD(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetStack          string `json:"target_stack"`
		IncludeTestScenarios *bool  `json:"include_test_scenarios"`
		Granularity          string `json:"granularity"`
	}
	if r.ContentLength > 0 {
		if err := decode(r, &body); err != nil {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	res, err := s.app.GenerateAIPRD(prd.Options{
		TargetStack: body.TargetStack,
		// Cenários de teste vêm por padrão, como no CLI e no MCP.
		IncludeTestScenarios: body.IncludeTestScenarios == nil || *body.IncludeTestScenarios,
		Granularity:          body.Granularity,
	}, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) getTasks(w http.ResponseWriter, r *http.Request) {
	list, err := s.app.ImplementationTasks(app.TaskQuery{Status: r.URL.Query().Get("status")})
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	tasks, board := list.Tasks, list.Board
	writeJSON(w, http.StatusOK, map[string]any{
		"tasks":        tasks,
		"total":        len(board.Tasks),
		"progress":     board.ProgressPercentage(),
		"generated_at": board.GeneratedAt,
		"target_stack": board.TargetStack,
	})
}

func (s *Server) setTaskStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.MarkTaskStatus(r.PathValue("id"), body.Status, body.Notes, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) validate(w http.ResponseWriter, r *http.Request) {
	rep, err := s.app.Validate()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, rep)
}

// ---------------------------------------------------------------------------
// Handlers — arquivos brutos
// ---------------------------------------------------------------------------

func (s *Server) getFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	content, err := s.app.ReadProjectFile(path)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"path": path, "content": content})
}

func (s *Server) putFile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.app.WriteProjectFile(body.Path, body.Content, hub.SourceUI); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

// ---------------------------------------------------------------------------
// Handlers — exportação
// ---------------------------------------------------------------------------

func (s *Server) exportSVG(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	svg, filename, err := s.app.DiagramSVG(app.DiagramSVGOptions{
		Executive:   q.Get("mode") == "executive",
		Dark:        q.Get("theme") == "dark",
		Transparent: q.Get("transparent") == "1",
		NoTitle:     q.Get("title") == "0",
	})
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	if q.Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	_, _ = w.Write([]byte(svg))
}

// ---------------------------------------------------------------------------
// Frontend embutido (SPA fallback)
// ---------------------------------------------------------------------------

func (s *Server) host() string {
	if s.opts.Host == "" {
		return "web"
	}
	return s.opts.Host
}

func (s *Server) staticHandler() http.Handler { return Frontend(s.opts.Assets, s.host()) }

// Frontend serve o frontend compilado como SPA: arquivos existentes vão como
// estão (assets/ com cache longo) e qualquer outra rota recebe o index.html,
// com <meta name="archcode-host" content="host"> para o frontend saber se
// roda no navegador ou no app desktop.
func Frontend(assets fs.FS, host string) http.Handler {
	if assets == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "frontend não embutido neste binário", http.StatusNotImplemented)
		})
	}
	index := indexHTML(assets, host)
	fileServer := http.FileServer(http.FS(assets))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path != "" && path != "index.html" {
			if f, err := assets.Open(path); err == nil {
				info, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !info.IsDir() {
					if strings.HasPrefix(path, "assets/") {
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					}
					fileServer.ServeHTTP(w, r)
					return
				}
			}
		}
		// SPA: a raiz e qualquer rota desconhecida devolvem o index.
		if index == nil {
			http.Error(w, "frontend não encontrado", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

func indexHTML(assets fs.FS, host string) []byte {
	index, err := fs.ReadFile(assets, "index.html")
	if err != nil {
		return nil
	}
	meta := fmt.Sprintf(`<meta name="archcode-host" content="%s">`, host)
	return bytes.Replace(index, []byte("<head>"), []byte("<head>\n    "+meta), 1)
}
