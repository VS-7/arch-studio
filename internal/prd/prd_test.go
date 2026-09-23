package prd

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func buildInput() Input {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "node-web", Type: "client", Data: model.NodeData{Label: "Web App", Technology: "React 19", Tier: "frontend"}},
		{ID: "node-gw", Type: "gateway", Data: model.NodeData{Label: "Gateway", Technology: "Traefik", Tier: "integration"}},
		{ID: "node-api", Type: "compute", Data: model.NodeData{Label: "Core API", Technology: "Go 1.23", Tier: "backend"}},
		{ID: "node-db", Type: "database", Data: model.NodeData{Label: "PostgreSQL", Technology: "PostgreSQL 16", Tier: "data"}},
		{ID: "node-cache", Type: "cache", Data: model.NodeData{Label: "Redis", Technology: "Redis 7", Tier: "data"}},
	}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "node-web", Target: "node-gw", Data: model.EdgeData{Protocol: "REST"}},
		{ID: "e2", Source: "node-gw", Target: "node-api", Data: model.EdgeData{Protocol: "REST"}},
		{ID: "e3", Source: "node-api", Target: "node-db", Data: model.EdgeData{Protocol: "SQL"}},
		{ID: "e4", Source: "node-api", Target: "node-cache", Data: model.EdgeData{Protocol: "Redis"}},
	}

	spec := model.NewEndpointsSpec()
	spec.Endpoints = []model.Endpoint{
		{ID: "post-login", Method: "POST", Path: "/api/v1/auth/login", Source: "node-gw", Target: "node-api", Auth: "none"},
		{ID: "get-me", Method: "GET", Path: "/api/v1/me", Source: "node-gw", Target: "node-api", Auth: "bearer"},
	}

	return Input{
		Manifest: model.DefaultManifest("Teste"),
		Diagram:  d,
		Requirements: &model.RequirementsDoc{Requirements: []model.Requirement{
			{ID: "RF001", Type: "RF", Title: "Autenticação", Components: []string{"node-api"}},
			{ID: "RNF001", Type: "RNF", Title: "P95 abaixo de 300ms", Components: []string{"node-api"}},
		}},
		UseCases: []model.UseCase{{
			Code: "CDU001", Name: "Autenticar", Components: []string{"node-api"},
			PreConditions: []string{"usuário cadastrado"},
			MainFlow:      []string{"envia credenciais", "recebe token"},
			Exceptions:    []string{"senha inválida"},
		}},
		Endpoints: spec,
	}
}

// RF020: uma tarefa nunca pode aparecer antes daquilo de que ela depende.
func TestOrdemTopologica(t *testing.T) {
	res := Compile(buildInput(), Options{IncludeTestScenarios: true, Granularity: "detailed"})

	position := map[string]int{}
	for i, task := range res.Board.Tasks {
		position[task.ID] = i
	}
	for _, task := range res.Board.Tasks {
		for _, dep := range task.Dependencies {
			depPos, ok := position[dep]
			if !ok {
				t.Errorf("%s depende de %s, que não existe no board", task.ID, dep)
				continue
			}
			if depPos > position[task.ID] {
				t.Errorf("%s (pos %d) aparece antes da sua dependência %s (pos %d)",
					task.ID, position[task.ID], dep, depPos)
			}
		}
	}

	// A camada de dados precisa vir antes da camada de apresentação.
	var dataPos, frontPos = -1, -1
	for i, task := range res.Board.Tasks {
		if task.ComponentID == "node-db" {
			dataPos = i
		}
		if task.ComponentID == "node-web" {
			frontPos = i
		}
	}
	if dataPos < 0 || frontPos < 0 || dataPos > frontPos {
		t.Errorf("banco (pos %d) deveria vir antes do frontend (pos %d)", dataPos, frontPos)
	}
}

func TestTarefasCarregamRastreabilidade(t *testing.T) {
	res := Compile(buildInput(), Options{IncludeTestScenarios: true, Granularity: "detailed"})

	var apiTask *model.Task
	for i := range res.Board.Tasks {
		if res.Board.Tasks[i].ComponentID == "node-api" {
			apiTask = &res.Board.Tasks[i]
		}
	}
	if apiTask == nil {
		t.Fatal("tarefa da Core API não foi gerada")
	}
	if len(apiTask.Requirements) != 2 {
		t.Errorf("requisitos vinculados: got %v, want RF001 e RNF001", apiTask.Requirements)
	}
	if len(apiTask.UseCases) != 1 || apiTask.UseCases[0] != "CDU001" {
		t.Errorf("casos de uso vinculados: got %v", apiTask.UseCases)
	}
	if len(apiTask.Endpoints) != 2 {
		t.Errorf("contratos vinculados: got %v", apiTask.Endpoints)
	}
	hasGWT := false
	for _, a := range apiTask.Acceptance {
		if strings.Contains(a, "**Given**") {
			hasGWT = true
		}
	}
	if !hasGWT {
		t.Error("critérios Given-When-Then do caso de uso não foram incorporados")
	}
}

// RF022: recompilar o PRD não pode zerar o progresso já registrado por agentes.
func TestRecompilacaoPreservaProgresso(t *testing.T) {
	in := buildInput()
	first := Compile(in, Options{IncludeTestScenarios: true})

	first.Board.Tasks[0].Status = model.StatusCompleted
	first.Board.Tasks[0].Notes = "go test ./... ok"
	doneID := first.Board.Tasks[0].ID

	in.Previous = first.Board
	second := Compile(in, Options{IncludeTestScenarios: true})

	task := second.Board.ByID(doneID)
	if task == nil {
		t.Fatalf("tarefa %s sumiu na recompilação", doneID)
	}
	if task.Status != model.StatusCompleted || task.Notes != "go test ./... ok" {
		t.Errorf("progresso perdido: %+v", task)
	}
	if second.Board.ProgressPercentage() == 0 {
		t.Error("percentual de progresso zerou")
	}
}

func TestInvariantesDerivadasDaArquitetura(t *testing.T) {
	in := buildInput()
	in.Diagram.Nodes[3].Data.Tags = []string{"pci-dss"}
	res := Compile(in, Options{IncludeTestScenarios: true, Granularity: "detailed"})

	for _, want := range []string{
		"camada de domínio NUNCA importa",
		"middleware de autenticação", // deduzido do endpoint com auth=bearer
		"camada de repositório",      // deduzido do nó de banco
		"PCI-DSS",                    // deduzido da tag
		"[RNF001]",                   // requisito não funcional vira invariante
	} {
		if !strings.Contains(res.Markdown, want) {
			t.Errorf("invariante ausente no documento: %q", want)
		}
	}
}

func TestDocumentoContemSecoesObrigatorias(t *testing.T) {
	res := Compile(buildInput(), Options{IncludeTestScenarios: true, Granularity: "detailed"})
	for _, section := range []string{
		"# AI MASTER IMPLEMENTATION SPECIFICATION",
		"## 1. TECH STACK & SYSTEM INVARIANTS",
		"## 2. ARCHITECTURE TOPOLOGY",
		"## 3. API CONTRACTS",
		"## 4. TOPOLOGICAL IMPLEMENTATION SEQUENCE",
		"## 5. USE CASE ACCEPTANCE CRITERIA",
		"## 6. REQUIREMENTS TRACEABILITY",
		"## 8. AGENT EXECUTION PROTOCOL",
		"```mermaid",
	} {
		if !strings.Contains(res.Markdown, section) {
			t.Errorf("seção ausente: %q", section)
		}
	}
	if res.Hash == "" || strings.Contains(res.Markdown, "{{HASH}}") {
		t.Error("hash de integridade não foi substituído no documento")
	}
}

func TestSyncNodeStatus(t *testing.T) {
	in := buildInput()
	res := Compile(in, Options{IncludeTestScenarios: true})
	for i := range res.Board.Tasks {
		if res.Board.Tasks[i].ComponentID == "node-db" {
			res.Board.Tasks[i].Status = model.StatusCompleted
		}
	}
	if !SyncNodeStatus(in.Diagram, res.Board) {
		t.Fatal("SyncNodeStatus não reportou mudança")
	}
	if got := in.Diagram.NodeByID("node-db").Data.Status; got != model.StatusCompleted {
		t.Errorf("status do nó: got %q, want completed", got)
	}
	if got := in.Diagram.NodeByID("node-api").Data.Status; got != model.StatusPending {
		t.Errorf("nó não concluído deveria continuar pendente: got %q", got)
	}
}

// Ciclos no grafo não podem travar o compilador.
func TestCicloNaoTrava(t *testing.T) {
	in := buildInput()
	in.Diagram.Edges = append(in.Diagram.Edges,
		model.Edge{ID: "cycle", Source: "node-db", Target: "node-api"})

	res := Compile(in, Options{IncludeTestScenarios: false})
	if len(res.Board.Tasks) != 5 {
		t.Errorf("com ciclo, todas as 5 tarefas ainda devem ser geradas, got %d", len(res.Board.Tasks))
	}
}
