package app

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
)

func newTestApp(t *testing.T) (*App, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, err := project.Init(st, project.Options{ProjectName: "Teste", Empty: true}); err != nil {
		t.Fatalf("init: %v", err)
	}
	return New(st, nil), st
}

// Percorre exatamente o caminho que um agente de IA faz via MCP.
func TestFluxoCompletoDoAgente(t *testing.T) {
	a, st := newTestApp(t)

	api, err := a.AddNode(NodeInput{
		Label: "Core API", Type: "compute", Technology: "Go 1.23", Complexity: "high",
	}, hub.SourceAI)
	if err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	db, err := a.AddNode(NodeInput{
		Label: "PostgreSQL", Type: "database", Technology: "PostgreSQL 16",
		ConnectTo: "Core API", Protocol: "SQL", CloudTier: "db.t4g.small",
	}, hub.SourceAI)
	if err != nil {
		t.Fatalf("AddNode com connect_to: %v", err)
	}

	// connect_to deve ter criado a aresta automaticamente.
	d, _ := st.LoadDiagram()
	if !d.HasEdge(db.ID, api.ID) {
		t.Error("connect_to não criou a conexão")
	}

	// Posicionamento inteligente: o nó novo não pode cair em cima do existente.
	if db.Position == api.Position {
		t.Error("nó novo recebeu a mesma posição do nó existente")
	}

	// connect_nodes por rótulo, registrando contrato de API.
	edge, endpoints, err := a.ConnectNodes(EdgeInput{
		SourceID: "Core API", TargetID: "PostgreSQL", Protocol: "SQL", Port: 5432,
		Endpoints: []EndpointInput{{Method: "post", Path: "api/v1/auth/login", Auth: "none", StatusCodes: []int{200, 401}}},
	}, hub.SourceAI)
	if err != nil {
		t.Fatalf("ConnectNodes: %v", err)
	}
	if len(endpoints) != 1 || endpoints[0].Path != "/api/v1/auth/login" || endpoints[0].Method != "POST" {
		t.Errorf("endpoint normalizado incorretamente: %+v", endpoints)
	}
	if edge.Data.Port != 5432 {
		t.Errorf("porta não gravada: %+v", edge.Data)
	}

	spec, _ := st.LoadEndpoints()
	if len(spec.Endpoints) != 1 {
		t.Errorf("api/endpoints.yaml deveria ter 1 contrato, got %d", len(spec.Endpoints))
	}

	// Requisito e caso de uso justificando os componentes.
	req, created, err := a.UpsertRequirement(model.Requirement{
		Title: "Autenticação", Type: "RF", Components: []string{api.ID, db.ID},
	}, hub.SourceAI)
	if err != nil || !created {
		t.Fatalf("UpsertRequirement: %v (created=%v)", err, created)
	}
	if req.ID != "RF001" {
		t.Errorf("id gerado: got %q, want RF001", req.ID)
	}

	if _, _, err := a.UpsertUseCase(model.UseCase{
		Name: "Autenticar Usuário", Components: []string{api.ID},
		MainFlow: []string{"envia credenciais", "recebe token"},
	}, hub.SourceAI); err != nil {
		t.Fatalf("UpsertUseCase: %v", err)
	}

	// Compilação do blueprint.
	res, err := a.GenerateAIPRD(prd.Options{IncludeTestScenarios: true, Granularity: "detailed"}, hub.SourceAI)
	if err != nil {
		t.Fatalf("GenerateAIPRD: %v", err)
	}
	if res.TotalTasks < 2 {
		t.Errorf("tarefas geradas: got %d", res.TotalTasks)
	}
	if !st.Exists(store.FileAIPRD) || !st.Exists(store.FileTasks) {
		t.Error("ai-prd.md ou tasks.json não foram gravados")
	}

	// Fila de tarefas e marcação de progresso.
	tasks, board, err := a.ImplementationTasks("pending")
	if err != nil {
		t.Fatalf("ImplementationTasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("nenhuma tarefa pendente")
	}
	var ready *TaskView
	for i := range tasks {
		if tasks[i].Ready {
			ready = &tasks[i]
			break
		}
	}
	if ready == nil {
		t.Fatal("nenhuma tarefa marcada como pronta para começar")
	}

	upd, err := a.MarkTaskStatus(ready.ID, "completed", "go test ./... ok", hub.SourceAI)
	if err != nil {
		t.Fatalf("MarkTaskStatus: %v", err)
	}
	if !upd.Updated || upd.Progress == 0 {
		t.Errorf("progresso não avançou: %+v", upd)
	}

	// RF022: o status precisa refletir no canvas e nos checkboxes do documento.
	d, _ = st.LoadDiagram()
	if n := d.NodeByID(ready.ComponentID); n != nil && n.Data.Status != model.StatusCompleted {
		t.Errorf("status do nó no canvas: got %q, want completed", n.Data.Status)
	}
	prdDoc, _ := st.ReadFile(store.FileAIPRD)
	if !strings.Contains(string(prdDoc), "- [x] **"+ready.ID) {
		t.Error("checkbox do ai-prd.md não foi atualizado")
	}
	_ = board
}

// RF017: nenhuma operação de IA pode mover um nó já posicionado pelo usuário.
func TestCoordenadasSaoPreservadas(t *testing.T) {
	a, st := newTestApp(t)

	for _, label := range []string{"A", "B", "C"} {
		if _, err := a.AddNode(NodeInput{Label: label, Type: "compute"}, hub.SourceAI); err != nil {
			t.Fatalf("AddNode %s: %v", label, err)
		}
	}

	// O usuário reposiciona tudo manualmente pelo canvas.
	d, _ := st.LoadDiagram()
	for i := range d.Nodes {
		d.Nodes[i].Position = model.Position{X: float64(i) * 1000, Y: 42}
	}
	if err := a.ReplaceDiagram(d, hub.SourceUI); err != nil {
		t.Fatalf("ReplaceDiagram: %v", err)
	}

	before := map[string]model.Position{}
	saved, _ := st.LoadDiagram()
	for _, n := range saved.Nodes {
		before[n.ID] = n.Position
	}

	// A IA então mexe na arquitetura de várias formas.
	if _, err := a.AddNode(NodeInput{Label: "D", Type: "database", ConnectTo: "A"}, hub.SourceAI); err != nil {
		t.Fatalf("AddNode D: %v", err)
	}
	if _, err := a.UpdateNode("B", NodeInput{Technology: "Rust", Description: "reescrito"}, hub.SourceAI); err != nil {
		t.Fatalf("UpdateNode: %v", err)
	}
	if _, _, err := a.ConnectNodes(EdgeInput{SourceID: "A", TargetID: "C", Protocol: "gRPC"}, hub.SourceAI); err != nil {
		t.Fatalf("ConnectNodes: %v", err)
	}

	after, _ := st.LoadDiagram()
	for _, n := range after.Nodes {
		want, existed := before[n.ID]
		if !existed {
			continue
		}
		if n.Position != want {
			t.Errorf("nó %s foi movido de %v para %v", n.ID, want, n.Position)
		}
	}
}

func TestRotuloDuplicadoEhRejeitado(t *testing.T) {
	a, _ := newTestApp(t)
	if _, err := a.AddNode(NodeInput{Label: "API", Type: "compute"}, hub.SourceAI); err != nil {
		t.Fatalf("primeiro AddNode: %v", err)
	}
	_, err := a.AddNode(NodeInput{Label: "api", Type: "compute"}, hub.SourceAI)
	if err == nil {
		t.Error("rótulo duplicado (case-insensitive) deveria ser rejeitado")
	}
	if !strings.Contains(err.Error(), "update_node_metadata") {
		t.Errorf("a mensagem de erro deveria orientar o agente: %v", err)
	}
}

func TestTipoInvalidoEhRejeitado(t *testing.T) {
	a, _ := newTestApp(t)
	_, err := a.AddNode(NodeInput{Label: "X", Type: "quantum_computer"}, hub.SourceAI)
	if err == nil || !strings.Contains(err.Error(), "válidos") {
		t.Errorf("tipo inválido deveria listar os válidos, got %v", err)
	}
}

// Remover um componente limpa arestas e contratos órfãos.
func TestRemocaoLimpaDependentes(t *testing.T) {
	a, st := newTestApp(t)
	_, _ = a.AddNode(NodeInput{Label: "API", Type: "compute"}, hub.SourceAI)
	_, _ = a.AddNode(NodeInput{Label: "DB", Type: "database"}, hub.SourceAI)
	_, _, _ = a.ConnectNodes(EdgeInput{
		SourceID: "API", TargetID: "DB", Protocol: "SQL",
		Endpoints: []EndpointInput{{Method: "GET", Path: "/api/v1/items"}},
	}, hub.SourceAI)

	if err := a.RemoveNode("DB", hub.SourceAI); err != nil {
		t.Fatalf("RemoveNode: %v", err)
	}
	d, _ := st.LoadDiagram()
	if len(d.Edges) != 0 {
		t.Errorf("arestas órfãs remanescentes: %+v", d.Edges)
	}
	spec, _ := st.LoadEndpoints()
	if len(spec.Endpoints) != 0 {
		t.Errorf("contratos órfãos remanescentes: %+v", spec.Endpoints)
	}
}

// As escritas são serializadas: chamadas concorrentes não podem se perder.
func TestEscritasConcorrentesNaoSePerdem(t *testing.T) {
	a, st := newTestApp(t)

	const n = 12
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = a.AddNode(NodeInput{
				Label: string(rune('A'+i)) + "-service", Type: "compute",
			}, hub.SourceAI)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("AddNode %d falhou: %v", i, err)
		}
	}
	d, _ := st.LoadDiagram()
	if len(d.Nodes) != n {
		t.Errorf("got %d nós gravados, want %d — houve perda de escrita", len(d.Nodes), n)
	}
}

// A gravação é atômica: nenhum arquivo temporário sobra na árvore do projeto.
func TestGravacaoAtomicaNaoDeixaLixo(t *testing.T) {
	a, st := newTestApp(t)
	for i := 0; i < 5; i++ {
		if _, err := a.AddNode(NodeInput{Label: "N" + string(rune('0'+i)), Type: "compute"}, hub.SourceAI); err != nil {
			t.Fatal(err)
		}
	}
	var leftovers []string
	_ = filepath.Walk(st.Root(), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.Contains(filepath.Base(path), ".archcode-") || strings.HasSuffix(path, ".tmp") {
			leftovers = append(leftovers, path)
		}
		return nil
	})
	if len(leftovers) > 0 {
		t.Errorf("arquivos temporários remanescentes: %v", leftovers)
	}
}

// O Mermaid espelho é regenerado a cada gravação do diagrama (RF007).
func TestMermaidEspelhoAcompanhaDiagrama(t *testing.T) {
	a, st := newTestApp(t)
	if _, err := a.AddNode(NodeInput{Label: "Payments Service", Type: "compute"}, hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	data, err := st.ReadFile(store.FileMacroMmd)
	if err != nil {
		t.Fatalf("macro.mermaid não foi gerado: %v", err)
	}
	if !strings.Contains(string(data), "Payments Service") {
		t.Error("espelho Mermaid não contém o componente recém-criado")
	}
}

func TestExportOpenAPI(t *testing.T) {
	a, st := newTestApp(t)
	_, _ = a.AddNode(NodeInput{Label: "API", Type: "compute"}, hub.SourceAI)
	_, _ = a.AddNode(NodeInput{Label: "Web", Type: "client"}, hub.SourceAI)
	_, _, _ = a.ConnectNodes(EdgeInput{
		SourceID: "Web", TargetID: "API", Protocol: "REST",
		Endpoints: []EndpointInput{{Method: "POST", Path: "/api/v1/login", Auth: "bearer", StatusCodes: []int{200, 401}}},
	}, hub.SourceAI)

	path, err := a.ExportOpenAPI(hub.SourceCLI)
	if err != nil {
		t.Fatalf("ExportOpenAPI: %v", err)
	}
	data, err := st.ReadFile(path)
	if err != nil {
		t.Fatalf("leitura: %v", err)
	}
	out := string(data)
	for _, want := range []string{"openapi: 3.1.0", "/api/v1/login", "bearerAuth", "\"401\""} {
		if !strings.Contains(out, want) {
			t.Errorf("openapi.yaml não contém %q", want)
		}
	}
}

func TestPropostaComercial(t *testing.T) {
	a, st := newTestApp(t)
	_, _ = a.AddNode(NodeInput{Label: "API", Type: "compute", CloudTier: "t4g.small"}, hub.SourceAI)

	res, err := a.GenerateProposal(ProposalOptions{
		ClientName: "Acme Corp", IncludeDiagram: true, IncludeCloud: true,
	}, hub.SourceCLI)
	if err != nil {
		t.Fatalf("GenerateProposal: %v", err)
	}
	if res.TotalCost <= 0 {
		t.Errorf("custo total inválido: %v", res.TotalCost)
	}
	data, _ := st.ReadFile(res.FilePath)
	out := string(data)
	for _, want := range []string{"Acme Corp", "Escopo do Projeto", "Investimento", "TOTAL DO PROJETO", "```mermaid"} {
		if !strings.Contains(out, want) {
			t.Errorf("proposta não contém %q", want)
		}
	}
}

func TestContextSummaryEhCompacto(t *testing.T) {
	a, _ := newTestApp(t)
	_, _ = a.AddNode(NodeInput{Label: "API", Type: "compute", Technology: "Go"}, hub.SourceAI)

	sum, err := a.ContextSummary(true)
	if err != nil {
		t.Fatalf("ContextSummary: %v", err)
	}
	if sum.Counts["components"] != 1 {
		t.Errorf("contagem: %+v", sum.Counts)
	}
	if sum.Pricing == nil {
		t.Error("include_pricing=true deveria trazer o resumo financeiro")
	}
	if len(sum.Warnings) == 0 {
		t.Error("sem AI-PRD gerado, o resumo deveria avisar o agente")
	}
}
