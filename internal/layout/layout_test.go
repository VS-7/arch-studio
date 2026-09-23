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

	x := map[string]float64{}
	for _, n := range d.Nodes {
		x[n.ID] = n.Position.X
	}
	if !(x["web"] < x["api"] && x["api"] < x["db"]) {
		t.Errorf("camadas fora de ordem: web=%v api=%v db=%v", x["web"], x["api"], x["db"])
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
