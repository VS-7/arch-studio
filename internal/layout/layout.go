// Package layout resolve o posicionamento de nós no canvas.
//
// Regra central (RF017): coordenadas de nós já existentes NUNCA são alteradas
// por operações de escrita de agentes de IA. Nós novos recebem uma posição livre
// calculada por proximidade a um nó âncora (tipicamente o nó ao qual serão
// conectados), ou por coluna de tier quando não há âncora.
package layout

import (
	"math"
	"sort"

	"github.com/archcode/studio/internal/model"
)

const (
	NodeWidth  = 220.0
	NodeHeight = 110.0
	GapX       = 100.0
	GapY       = 70.0
	StepX      = NodeWidth + GapX  // 320
	StepY      = NodeHeight + GapY // 180
)

// TierColumn define a coluna horizontal padrão de cada tier arquitetural.
var TierColumn = map[string]int{
	"frontend":    0,
	"client":      0,
	"integration": 1,
	"backend":     2,
	"domain":      2,
	"devops":      3,
	"data":        4,
}

// TypeTier mapeia o tipo visual do nó para o tier padrão, usado quando o tier
// não foi informado explicitamente.
var TypeTier = map[string]string{
	"client":           "frontend",
	"gateway":          "integration",
	"compute":          "backend",
	"queue":            "backend",
	"cache":            "data",
	"database":         "data",
	"storage":          "data",
	"external_service": "integration",
	"group":            "backend",
}

// ResolveTier devolve o tier efetivo de um nó.
func ResolveTier(n model.Node) string {
	if n.Data.Tier != "" {
		return n.Data.Tier
	}
	if t, ok := TypeTier[n.Type]; ok {
		return t
	}
	return "backend"
}

type rect struct{ x, y, w, h float64 }

func (r rect) overlaps(o rect) bool {
	return r.x < o.x+o.w && r.x+r.w > o.x && r.y < o.y+o.h && r.y+r.h > o.y
}

func boxOf(n model.Node) rect {
	w, h := n.Width, n.Height
	if w <= 0 {
		w = NodeWidth
	}
	if h <= 0 {
		h = NodeHeight
	}
	return rect{n.Position.X, n.Position.Y, w, h}
}

func free(nodes []model.Node, p model.Position) bool {
	candidate := rect{p.X - GapX/2, p.Y - GapY/2, NodeWidth + GapX, NodeHeight + GapY}
	for _, n := range nodes {
		if n.Type == "group" {
			continue // grupos são contêineres; sobreposição é intencional
		}
		if candidate.overlaps(boxOf(n)) {
			return false
		}
	}
	return true
}

// FindFreePosition calcula uma posição estética e livre para um novo nó.
// Quando anchorID aponta para um nó existente, o novo nó é colocado em anel
// crescente em volta dele, priorizando a direção coerente com o tier.
func FindFreePosition(d *model.Diagram, anchorID, tier string) model.Position {
	if len(d.Nodes) == 0 {
		return model.Position{X: 120, Y: 120}
	}

	if anchor := d.ResolveNode(anchorID); anchor != nil {
		base := anchor.Position
		// Direções ordenadas: à direita, abaixo, acima, à esquerda e diagonais.
		dirs := [][2]float64{
			{1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 0}, {-1, 1}, {-1, -1},
		}
		if TierColumn[tier] < TierColumn[ResolveTier(*anchor)] {
			// O novo nó pertence a um tier mais à esquerda: prefira ir para trás.
			dirs = [][2]float64{{-1, 0}, {0, 1}, {0, -1}, {-1, 1}, {-1, -1}, {1, 0}, {1, 1}, {1, -1}}
		}
		for ring := 1; ring <= 6; ring++ {
			for _, dir := range dirs {
				p := model.Position{
					X: base.X + dir[0]*StepX*float64(ring),
					Y: base.Y + dir[1]*StepY*float64(ring),
				}
				if free(d.Nodes, p) {
					return p
				}
			}
		}
	}

	// Sem âncora: empilha na coluna do tier.
	col, ok := TierColumn[tier]
	if !ok {
		col = 2
	}
	x := 120 + float64(col)*StepX
	for row := 0; row < 200; row++ {
		p := model.Position{X: x, Y: 120 + float64(row)*StepY}
		if free(d.Nodes, p) {
			return p
		}
	}

	// Fallback determinístico: abaixo de tudo.
	maxY := 0.0
	for _, n := range d.Nodes {
		if n.Position.Y > maxY {
			maxY = n.Position.Y
		}
	}
	return model.Position{X: x, Y: maxY + StepY}
}

// AutoLayout recalcula todas as posições por camadas topológicas.
// Usado apenas em importação de Mermaid ou quando o usuário pede explicitamente
// "reorganizar", nunca em escritas incrementais de agentes.
func AutoLayout(d *model.Diagram) {
	if len(d.Nodes) == 0 {
		return
	}
	indexByID := map[string]int{}
	for i, n := range d.Nodes {
		indexByID[n.ID] = i
	}

	indegree := make([]int, len(d.Nodes))
	adj := make([][]int, len(d.Nodes))
	for _, e := range d.Edges {
		si, sok := indexByID[e.Source]
		ti, tok := indexByID[e.Target]
		if !sok || !tok || si == ti {
			continue
		}
		adj[si] = append(adj[si], ti)
		indegree[ti]++
	}

	// Longest-path layering: camada(v) = 1 + max(camada(predecessores)).
	depth := make([]int, len(d.Nodes))
	queue := []int{}
	for i, deg := range indegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}
	if len(queue) == 0 { // grafo totalmente cíclico: começa pelo primeiro nó
		queue = append(queue, 0)
		indegree[0] = 0
	}
	remaining := append([]int(nil), indegree...)
	processed := 0
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		processed++
		for _, next := range adj[cur] {
			if depth[cur]+1 > depth[next] {
				depth[next] = depth[cur] + 1
			}
			remaining[next]--
			if remaining[next] == 0 {
				queue = append(queue, next)
			}
		}
	}
	if processed < len(d.Nodes) {
		// Ciclos remanescentes: usa o tier como profundidade de fallback.
		for i, n := range d.Nodes {
			if remaining[i] > 0 {
				depth[i] = TierColumn[ResolveTier(n)]
			}
		}
	}

	byDepth := map[int][]int{}
	for i := range d.Nodes {
		byDepth[depth[i]] = append(byDepth[depth[i]], i)
	}
	depths := make([]int, 0, len(byDepth))
	for k := range byDepth {
		depths = append(depths, k)
	}
	sort.Ints(depths)

	for _, lvl := range depths {
		idxs := byDepth[lvl]
		sort.SliceStable(idxs, func(a, b int) bool {
			return d.Nodes[idxs[a]].Data.Label < d.Nodes[idxs[b]].Data.Label
		})
		for row, idx := range idxs {
			d.Nodes[idx].Position = model.Position{
				X: 120 + float64(lvl)*StepX,
				Y: 120 + float64(row)*StepY,
			}
		}
	}
}

// Bounds devolve o retângulo que envolve todos os nós, útil para exportação.
func Bounds(d *model.Diagram) (minX, minY, maxX, maxY float64) {
	if len(d.Nodes) == 0 {
		return 0, 0, 0, 0
	}
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for _, n := range d.Nodes {
		b := boxOf(n)
		minX = math.Min(minX, b.x)
		minY = math.Min(minY, b.y)
		maxX = math.Max(maxX, b.x+b.w)
		maxY = math.Max(maxY, b.y+b.h)
	}
	return
}
