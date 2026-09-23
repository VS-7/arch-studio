package lint

import (
	"testing"

	"github.com/archcode/studio/internal/model"
)

func findingsByRule(rep *Report) map[string]int {
	out := map[string]int{}
	for _, f := range rep.Findings {
		out[f.Rule]++
	}
	return out
}

func TestDetectaComponenteIsolado(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "orfao", Type: "compute", Data: model.NodeData{Label: "Órfão", Technology: "Go"}},
	}
	rep := Run(Input{Diagram: d})
	if findingsByRule(rep)["R001-isolated-node"] == 0 {
		t.Error("componente sem conexões deveria gerar erro")
	}
	if rep.Passed {
		t.Error("relatório com erro não pode passar")
	}
}

func TestDetectaClienteAcessandoBancoDiretamente(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "web", Type: "client", Data: model.NodeData{Label: "Web", Technology: "React"}},
		{ID: "db", Type: "database", Data: model.NodeData{Label: "DB", Technology: "PostgreSQL"}},
	}
	d.Edges = []model.Edge{{ID: "e", Source: "web", Target: "db", Data: model.EdgeData{Protocol: "SQL"}}}

	rep := Run(Input{Diagram: d})
	if findingsByRule(rep)["R006-client-direct-database"] == 0 {
		t.Error("acesso direto de cliente a banco deveria gerar erro")
	}
}

func TestDetectaAusenciaDeAutenticacao(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "gw", Type: "gateway", Data: model.NodeData{Label: "Gateway", Technology: "Traefik"}},
		{ID: "api", Type: "compute", Data: model.NodeData{Label: "API", Technology: "Go"}},
	}
	d.Edges = []model.Edge{{ID: "e", Source: "gw", Target: "api", Data: model.EdgeData{Protocol: "REST"}}}

	rep := Run(Input{Diagram: d, Endpoints: model.NewEndpointsSpec()})
	if findingsByRule(rep)["R010-missing-authentication"] == 0 {
		t.Error("superfície pública sem autenticação deveria gerar erro")
	}

	// Com um serviço de autenticação presente, a regra silencia.
	d.Nodes = append(d.Nodes, model.Node{ID: "auth", Type: "compute",
		Data: model.NodeData{Label: "Auth Service", Technology: "Go"}})
	d.Edges = append(d.Edges, model.Edge{ID: "e2", Source: "gw", Target: "auth",
		Data: model.EdgeData{Protocol: "REST"}})
	rep = Run(Input{Diagram: d, Endpoints: model.NewEndpointsSpec()})
	if findingsByRule(rep)["R010-missing-authentication"] != 0 {
		t.Error("com serviço de autenticação, a regra não deveria disparar")
	}
}

func TestDetectaArestaSemProtocoloEComponenteSemRastreabilidade(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "a", Type: "compute", Data: model.NodeData{Label: "A", Technology: "Go"}},
		{ID: "b", Type: "compute", Data: model.NodeData{Label: "B", Technology: "Go"}},
	}
	d.Edges = []model.Edge{{ID: "e", Source: "a", Target: "b"}}

	rep := Run(Input{Diagram: d})
	rules := findingsByRule(rep)
	if rules["R008-edge-without-protocol"] == 0 {
		t.Error("aresta sem protocolo deveria gerar aviso")
	}
	if rules["R004-untraced-component"] != 2 {
		t.Errorf("ambos os componentes deveriam ser marcados como não rastreados, got %d",
			rules["R004-untraced-component"])
	}
}

func TestArquiteturaCompletaPassa(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "web", Type: "client", Data: model.NodeData{Label: "Web", Technology: "React 19",
			Pricing: &model.NodePricing{Complexity: "medium"}}},
		{ID: "auth", Type: "compute", Data: model.NodeData{Label: "Auth Service", Technology: "Go 1.23",
			Pricing: &model.NodePricing{Complexity: "high"}}},
		{ID: "db", Type: "database", Data: model.NodeData{Label: "PostgreSQL", Technology: "PostgreSQL 16",
			Pricing: &model.NodePricing{Complexity: "medium"}}},
	}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "web", Target: "auth", Data: model.EdgeData{Protocol: "REST"}},
		{ID: "e2", Source: "auth", Target: "db", Data: model.EdgeData{Protocol: "SQL"}},
	}

	rep := Run(Input{
		Diagram: d,
		Requirements: &model.RequirementsDoc{Requirements: []model.Requirement{
			{ID: "RF001", Components: []string{"web", "auth", "db"}},
		}},
		Endpoints: model.NewEndpointsSpec(),
	})
	if !rep.Passed {
		t.Errorf("arquitetura completa não deveria ter erros: %+v", rep.Findings)
	}
	if rep.Score < 100 {
		t.Errorf("score: got %d, want 100 — achados: %+v", rep.Score, rep.Findings)
	}
}
