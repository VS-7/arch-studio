package layout

import (
	"math"
	"testing"
)

func TestPack1DRespeitaOrdemEDistancia(t *testing.T) {
	// Todos querem o mesmo lugar: ficam lado a lado, centrados no desejado.
	got := pack1D([]float64{100, 100, 100}, []float64{0, 50, 50})
	want := []float64{50, 100, 150}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Fatalf("pack1D = %v, want %v", got, want)
		}
	}
	// Sem conflito, cada um fica onde quer.
	got = pack1D([]float64{0, 200}, []float64{0, 50})
	if got[0] != 0 || got[1] != 200 {
		t.Fatalf("pack1D sem conflito = %v", got)
	}
}

func TestNaturalLess(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"CDU2", "CDU10", true},
		{"CDU010", "CDU9", false},
		{"cdu001", "CDU002", true},
		{"Emitir", "emitir recibo", true},
	} {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Errorf("naturalLess(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

// Arestas longas ganham corredor: a reta entre as pontas não atravessa os nós
// das camadas intermediárias.
func TestLayeredArestaLongaNaoAtravessaNos(t *testing.T) {
	// 0 → 1 → 2 → 3 e o atalho 0 → 3, com vizinhos extras nas camadas do meio.
	nodes := make([]lgNode, 6)
	for i := range nodes {
		nodes[i] = lgNode{200, 100}
	}
	edges := []lgEdge{{0, 1, 0}, {1, 2, 0}, {2, 3, 0}, {0, 3, 0}, {0, 4, 0}, {4, 5, 0}}
	pos, _, _ := layered(nodes, edges, lgOptions{layerGap: 80, nodeGap: 60})
	if n := through(nodes, edges, pos); n != 0 {
		t.Errorf("%d aresta(s) atravessam nós: %v", n, pos)
	}
}

func TestCompoundCicloDeContencaoNaoTrava(t *testing.T) {
	boxes := []cbox{{w: 100, h: 50, parent: 1}, {w: 100, h: 50, parent: 0}, {w: 100, h: 50, parent: -1}}
	pos, _ := compound(boxes, nil, fitSpec{
		orients: []Orientation{TopDown},
		options: func(Orientation) lgOptions { return lgOptions{layerGap: 50, nodeGap: 50} },
	})
	if len(pos) != 3 {
		t.Fatalf("posições: %v", pos)
	}
}
