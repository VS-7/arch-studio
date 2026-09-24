package layout

import (
	"testing"

	"github.com/archcode/studio/internal/model"
)

func node(id string, x, y float64, typ string) model.Node {
	return model.Node{ID: id, Type: typ, Position: model.Position{X: x, Y: y},
		Data: model.NodeData{Label: id, Tier: TypeTier[typ]}}
}

// RF017: a posição calculada para um nó novo jamais pode colidir com nós já
// posicionados pelo usuário.
func TestFindFreePositionNuncaSobrepoe(t *testing.T) {
	d := model.NewDiagram()
	for i := 0; i < 40; i++ {
		pos := FindFreePosition(d, "", "backend")
		for _, existing := range d.Nodes {
			dx := pos.X - existing.Position.X
			dy := pos.Y - existing.Position.Y
			if dx < 0 {
				dx = -dx
			}
			if dy < 0 {
				dy = -dy
			}
			if dx < NodeWidth && dy < NodeHeight {
				t.Fatalf("nó %d em %v colide com %s em %v", i, pos, existing.ID, existing.Position)
			}
		}
		d.Nodes = append(d.Nodes, node(string(rune('a'+i%26))+string(rune('0'+i/26)), pos.X, pos.Y, "compute"))
	}
}

func TestFindFreePositionUsaAncora(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{node("node-api", 500, 300, "compute")}

	pos := FindFreePosition(d, "node-api", "data")
	// Deve ficar adjacente à âncora, não numa coluna distante.
	if pos.X < 500 || pos.X > 500+StepX*2 {
		t.Errorf("posição %v não está adjacente à âncora em x=500", pos)
	}
	if pos.Y != 300 {
		t.Errorf("primeira tentativa deveria ser à direita, na mesma linha: %v", pos)
	}
}

// Um tier mais à esquerda (frontend) ancorado num backend deve ir para trás.
func TestFindFreePositionRespeitaDirecaoDoTier(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{node("node-api", 800, 200, "compute")}
	pos := FindFreePosition(d, "node-api", "frontend")
	if pos.X >= 800 {
		t.Errorf("componente de frontend deveria ir à esquerda do backend, got %v", pos)
	}
}

func TestAutoLayoutOrdenaPorDependencia(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		node("web", 0, 0, "client"),
		node("api", 0, 0, "compute"),
		node("db", 0, 0, "database"),
	}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "web", Target: "api"},
		{ID: "e2", Source: "api", Target: "db"},
	}
	AutoLayout(d)

	// O fluxo segue a dependência no eixo escolhido (horizontal ou vertical).
	x, y := map[string]float64{}, map[string]float64{}
	for _, n := range d.Nodes {
		x[n.ID], y[n.ID] = n.Position.X, n.Position.Y
	}
	horizontal := x["web"] < x["api"] && x["api"] < x["db"]
	vertical := y["web"] < y["api"] && y["api"] < y["db"]
	if !horizontal && !vertical {
		t.Errorf("camadas fora de ordem: web=%v,%v api=%v,%v db=%v,%v",
			x["web"], y["web"], x["api"], y["api"], x["db"], y["db"])
	}
}

// Grafos cíclicos não podem travar nem produzir posições inválidas.
func TestAutoLayoutToleraCiclos(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{node("a", 0, 0, "compute"), node("b", 0, 0, "compute")}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "a", Target: "b"},
		{ID: "e2", Source: "b", Target: "a"},
	}
	AutoLayout(d)
	for _, n := range d.Nodes {
		if n.Position.X == 0 && n.Position.Y == 0 {
			t.Errorf("nó %s ficou sem posição", n.ID)
		}
	}
}

// Grupos são contêineres: os membros ficam juntos e o grupo passa a envolvê-los.
func TestAutoLayoutGrupoEnvolveMembros(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		node("web", 0, 0, "client"),
		node("api", 0, 200, "compute"),
		node("worker", 0, 400, "compute"),
		node("db", 0, 600, "database"),
		{ID: "grp", Type: "group", Position: model.Position{X: -50, Y: 150}, Width: 400, Height: 500,
			Data: model.NodeData{Label: "Backend"}},
	}
	d.Nodes[1].ParentID = "grp" // por parentId
	// worker: dentro do grupo só pela geometria.
	d.Edges = []model.Edge{
		{ID: "e1", Source: "web", Target: "api"},
		{ID: "e2", Source: "api", Target: "worker"},
		{ID: "e3", Source: "worker", Target: "db"},
	}
	AutoLayout(d)

	g := d.NodeByID("grp")
	inside := func(id string) bool {
		n := d.NodeByID(id)
		return n.Position.X >= g.Position.X && n.Position.Y >= g.Position.Y &&
			n.Position.X+NodeWidth <= g.Position.X+g.Width && n.Position.Y+NodeHeight <= g.Position.Y+g.Height
	}
	for _, id := range []string{"api", "worker"} {
		if !inside(id) {
			t.Errorf("%s ficou fora do grupo", id)
		}
	}
	for _, id := range []string{"web", "db"} {
		if inside(id) {
			t.Errorf("%s não deveria estar dentro do grupo", id)
		}
	}
}

// Diagramas largos passam a caber melhor na página que o layout em linha.
func TestAutoLayoutCabeMelhorNaPagina(t *testing.T) {
	d := model.NewDiagram()
	ids := []string{"a", "b", "c", "d", "e", "f"}
	for _, id := range ids {
		d.Nodes = append(d.Nodes, node(id, 0, 0, "compute"))
	}
	for i := 1; i < len(ids); i++ {
		d.Edges = append(d.Edges, model.Edge{ID: "e" + ids[i], Source: ids[i-1], Target: ids[i]})
	}
	AutoLayout(d)
	minX, minY, maxX, maxY := Bounds(d)
	w, h := maxX-minX, maxY-minY
	scale := min(PageWidth/(w+128), PageHeight/(h+128))
	// Em linha seriam 6*220+5*100 = 1820px de largura: escala 0,32.
	if scale < 0.5 {
		t.Errorf("cadeia de 6 nós deveria caber a ≥ 50%% na página, got %.2f (%.0fx%.0f)", scale, w, h)
	}
}
