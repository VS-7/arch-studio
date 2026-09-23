package mermaid

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func sample() *model.Diagram {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "node-web", Type: "client", Position: model.Position{X: 100, Y: 100},
			Data: model.NodeData{Label: "Web App", Technology: "React 19", Tier: "frontend"}},
		{ID: "node-api", Type: "compute", Position: model.Position{X: 420, Y: 100},
			Data: model.NodeData{Label: "Core API", Technology: "Go 1.23", Tier: "backend"}},
		{ID: "node-db", Type: "database", Position: model.Position{X: 740, Y: 100},
			Data: model.NodeData{Label: "PostgreSQL", Technology: "PostgreSQL 16", Tier: "data"}},
		{ID: "node-cache", Type: "cache", Position: model.Position{X: 740, Y: 300},
			Data: model.NodeData{Label: "Redis", Tier: "data"}},
	}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "node-web", Target: "node-api", Data: model.EdgeData{Protocol: "REST", Port: 443}},
		{ID: "e2", Source: "node-api", Target: "node-db", Data: model.EdgeData{Protocol: "SQL", Port: 5432}},
		{ID: "e3", Source: "node-api", Target: "node-cache", Data: model.EdgeData{Protocol: "Redis"}},
	}
	return d
}

func TestExportDeterministico(t *testing.T) {
	d := sample()
	if Export(d) != Export(d) {
		t.Error("exportação não é determinística")
	}
}

func TestRoundTripPreservaTopologia(t *testing.T) {
	original := sample()
	imported, err := Import(Export(original), nil)
	if err != nil {
		t.Fatalf("importação falhou: %v", err)
	}

	if len(imported.Nodes) != len(original.Nodes) {
		t.Fatalf("got %d nós, want %d", len(imported.Nodes), len(original.Nodes))
	}
	if len(imported.Edges) != len(original.Edges) {
		t.Fatalf("got %d arestas, want %d", len(imported.Edges), len(original.Edges))
	}

	// Rótulos, tipos e tecnologias sobrevivem à ida e volta.
	byLabel := map[string]model.Node{}
	for _, n := range imported.Nodes {
		byLabel[n.Data.Label] = n
	}
	for _, want := range original.Nodes {
		got, ok := byLabel[want.Data.Label]
		if !ok {
			t.Errorf("nó %q sumiu na importação", want.Data.Label)
			continue
		}
		if got.Type != want.Type {
			t.Errorf("%s: tipo got %q, want %q", want.Data.Label, got.Type, want.Type)
		}
		if want.Data.Technology != "" && got.Data.Technology != want.Data.Technology {
			t.Errorf("%s: tecnologia got %q, want %q", want.Data.Label, got.Data.Technology, want.Data.Technology)
		}
	}

	// As conexões apontam para os mesmos pares de rótulos.
	label := func(d *model.Diagram, id string) string {
		if n := d.NodeByID(id); n != nil {
			return n.Data.Label
		}
		return id
	}
	pairs := map[string]bool{}
	for _, e := range imported.Edges {
		pairs[label(imported, e.Source)+"→"+label(imported, e.Target)] = true
	}
	for _, e := range original.Edges {
		key := label(original, e.Source) + "→" + label(original, e.Target)
		if !pairs[key] {
			t.Errorf("conexão %s perdida", key)
		}
	}
}

// RF017: importar um Mermaid não pode embaralhar o canvas que o usuário arrumou.
func TestImportPreservaCoordenadasExistentes(t *testing.T) {
	base := sample()
	base.Nodes[1].Position = model.Position{X: 1234, Y: 567}

	imported, err := Import(Export(base), base)
	if err != nil {
		t.Fatalf("importação falhou: %v", err)
	}
	for _, n := range imported.Nodes {
		if n.Data.Label == "Core API" {
			if n.Position.X != 1234 || n.Position.Y != 567 {
				t.Errorf("coordenadas do nó existente foram perdidas: %v", n.Position)
			}
			return
		}
	}
	t.Error("nó Core API não encontrado após importação")
}

func TestImportSnippetExterno(t *testing.T) {
	src := "```mermaid\n" + `
flowchart TD
    %% comentário ignorado
    subgraph vpc["VPC Produção"]
      svc["Orders Service"]
      q>"Order Events"]
    end
    mobile(["App Mobile"]) -->|"REST :443"| gw{{"API Gateway"}}
    gw --> svc
    svc --> db[("Orders DB")]
    svc -.-> q
` + "\n```"

	d, err := Import(src, nil)
	if err != nil {
		t.Fatalf("importação falhou: %v", err)
	}

	want := map[string]string{
		"App Mobile":     "client",
		"API Gateway":    "gateway",
		"Orders Service": "compute",
		"Orders DB":      "database",
		"Order Events":   "queue",
	}
	got := map[string]string{}
	groups := 0
	for _, n := range d.Nodes {
		if n.Type == "group" {
			groups++
			continue
		}
		got[n.Data.Label] = n.Type
	}
	for label, typ := range want {
		if got[label] != typ {
			t.Errorf("%s: tipo got %q, want %q", label, got[label], typ)
		}
	}
	if groups != 1 {
		t.Errorf("subgraph deveria virar 1 nó de grupo, got %d", groups)
	}
	if len(d.Edges) != 4 {
		t.Errorf("got %d arestas, want 4", len(d.Edges))
	}

	// O rótulo "REST :443" deve virar protocolo e porta estruturados.
	found := false
	for _, e := range d.Edges {
		if e.Data.Protocol == "REST" && e.Data.Port == 443 {
			found = true
		}
	}
	if !found {
		t.Error("protocolo/porta não foram inferidos do rótulo da aresta")
	}
}

func TestExportIncluiEstilos(t *testing.T) {
	out := Export(sample())
	for _, want := range []string{"flowchart LR", "classDef compute", "classDef database", `[("PostgreSQL`} {
		if !strings.Contains(out, want) {
			t.Errorf("saída não contém %q", want)
		}
	}
}
