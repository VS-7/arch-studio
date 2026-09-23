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

// Diagramas UML colados em "Importar Mermaid" da arquitetura eram lidos como
// flowchart e viravam componentes sem sentido ("<|", "motivo: String").
func TestImportRecusaDiagramasNaoArquiteturais(t *testing.T) {
	for _, src := range []string{
		"classDiagram\n  class Pedido {\n    +motivo: String\n  }\n  Pedido <|-- Cliente",
		"```mermaid\n%% comentário\nsequenceDiagram\n  A->>B: oi\n```",
		"stateDiagram-v2\n  [*] --> Ativo",
		"erDiagram\n  CLIENTE ||--o{ PEDIDO : faz",
		"   \n",
	} {
		if _, err := Import(src, nil); err == nil {
			t.Errorf("esperava erro para %q", src)
		}
	}
	if _, err := Import("%% arquitetura\nflowchart LR\n  a[\"API\"] --> b[(\"DB\")]", nil); err != nil {
		t.Fatalf("flowchart válido recusado: %v", err)
	}
}

// edgeArch reúne nomes que já quebraram o Mermaid 11 ("Syntax error in text"):
// rótulos vazios, ids que são palavras reservadas e rótulos só de espaços.
func edgeArch() *model.Diagram {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "node-login", Type: "client", Position: model.Position{X: 0}},
		{ID: "node-vazio", Type: "compute", Position: model.Position{X: 10}, Data: model.NodeData{Label: "   ", Technology: "Go"}},
		{ID: "end", Type: "database", Position: model.Position{X: 20}, Data: model.NodeData{Label: "Fim"}},
		{ID: "Class", Type: "queue", Position: model.Position{X: 30}, Data: model.NodeData{Label: `a "b" | c %%{init}%% <br/> ] ) }`}},
		{ID: "node-q", Type: "cache", Position: model.Position{X: 40}, Data: model.NodeData{Label: "</"}},
		// "<b" + um `="` adiante: o Mermaid reescreveria as aspas das linhas seguintes.
		{ID: "node-tag", Type: "compute", Position: model.Position{X: 50}, Data: model.NodeData{Label: "List<String> <b"}},
		{ID: "node-igual", Type: "compute", Position: model.Position{X: 60}, Data: model.NodeData{Label: "x="}},
		{ID: "grp", Type: "group", Position: model.Position{X: -100, Y: -100}, Width: 800, Height: 600},
	}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "node-login", Target: "end", Label: "   "},
		{ID: "e2", Source: "end", Target: "Class", Label: "x (y) | z"},
		{ID: "e3", Source: "Class", Target: "node-q"},
	}
	return d
}

func TestExportRotulosVaziosEIdsReservados(t *testing.T) {
	out := Export(edgeArch())
	for _, bad := range []string{`[""]`, `([""])`, `|""|`, "\n    end[", " --> end\n", "%%{"} {
		if strings.Contains(out, bad) {
			t.Errorf("saída contém %q, que o Mermaid 11 recusa:\n%s", bad, out)
		}
	}
	for _, want := range []string{
		`node_login(["node-login"])`,                     // sem rótulo: mostra o id
		`node_vazio["node-vazio<br/><small>Go</small>"]`, // só espaços também
		`subgraph grp["grp"]`,
		`end_[("Fim")]`,
		`Class_>"a 'b' / c %{init}% #60;br/> ] ) }"]`, // HTML digitado pelo usuário vira texto
		`node_tag["List#60;String> #60;b"]`,
		"node_login --> end_\n", // rótulo só de espaços é omitido
		`end_ -->|"x (y) / z"| Class_`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("saída sem %q:\n%s", want, out)
		}
	}
}

// O escape de "<" feito na exportação é desfeito na importação.
func TestRoundTripPreservaMenorQue(t *testing.T) {
	d := sample()
	d.Nodes[1].Data.Label = "Core<API>"
	imported, err := Import(Export(d), nil)
	if err != nil {
		t.Fatalf("importação falhou: %v", err)
	}
	for _, n := range imported.Nodes {
		if n.Data.Label == "Core<API>" {
			return
		}
	}
	t.Errorf("rótulo com \"<\" não sobreviveu à ida e volta: %+v", imported.Nodes)
}

// Um nó `id[" "]` importado virava componente sem nome.
func TestImportRotuloVazioUsaID(t *testing.T) {
	d, err := Import("flowchart LR\n  login[\" \"] --> api([\"API\"])", nil)
	if err != nil {
		t.Fatalf("importação falhou: %v", err)
	}
	labels := map[string]bool{}
	for _, n := range d.Nodes {
		labels[n.Data.Label] = true
	}
	if !labels["login"] || !labels["API"] || labels[""] {
		t.Errorf("rótulos inesperados: %v", labels)
	}
}
