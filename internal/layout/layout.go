// Package layout resolve o posicionamento de nós no canvas.
//
// Regra central (RF017): coordenadas de nós já existentes NUNCA são alteradas
// por operações de escrita de agentes de IA. Nós novos recebem uma posição livre
// calculada por proximidade a um nó âncora (tipicamente o nó ao qual serão
// conectados), ou por coluna de tier quando não há âncora.
//
// A exceção é explícita: "Reorganizar" (AutoLayout e UML) redispõe o diagrama
// inteiro para caber na página do Documento de Requisitos — a pedido do
// usuário, na importação de Mermaid ou ao criar um diagrama gerado.
package layout

import (
	"fmt"
	"math"

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

// Recuo interno dos grupos: o mesmo com que o import de Mermaid materializa
// um subgraph em volta dos filhos (mermaid.Import), para os dois concordarem.
var groupPad = [4]float64{60, 40, 40, 40}

// Margem do SVG de arquitetura (svgexport.Options.Padding padrão).
const archSVGMargin = 64.0

// AutoLayout recalcula todas as posições por camadas topológicas, escolhendo
// entre fluxo da esquerda para a direita (preferido) e de cima para baixo o que
// deixa o diagrama maior numa página A4 do documento. Grupos viram contêineres:
// os membros (parentId, ou nós cujo centro está dentro do grupo) são dispostos
// juntos e o grupo é redimensionado para envolvê-los.
//
// Usado apenas em importação de Mermaid ou quando o usuário pede explicitamente
// "reorganizar", nunca em escritas incrementais de agentes.
func AutoLayout(d *model.Diagram) {
	if len(d.Nodes) == 0 {
		return
	}
	// Ordem de entrada: a de leitura da disposição atual; o baricentro parte dela.
	rects := make([][4]float64, len(d.Nodes))
	for i, n := range d.Nodes {
		w, h := groupSize(n)
		rects[i] = [4]float64{n.Position.X, n.Position.Y, w, h}
	}
	idx := readingOrder(rects)

	boxes := []cbox{}
	item := map[string]int{} // id do nó (ou do grupo virtual) → índice em boxes
	nodeOf := []int{}        // índice em boxes → índice em d.Nodes (-1 = grupo virtual)
	for _, i := range idx {
		n := d.Nodes[i]
		w, h := groupSize(n)
		item[n.ID] = len(boxes)
		boxes = append(boxes, cbox{w: w, h: h, parent: -1, pad: groupPad, minW: 180})
		nodeOf = append(nodeOf, i)
	}
	// parentId sem nó de grupo (subgraph do Mermaid antes de materializado)
	// também agrupa: um contêiner virtual mantém os membros juntos.
	for _, i := range idx {
		p := d.Nodes[i].ParentID
		if p == "" {
			continue
		}
		if _, ok := item[p]; !ok {
			item[p] = len(boxes)
			boxes = append(boxes, cbox{parent: -1, pad: groupPad})
			nodeOf = append(nodeOf, -1)
		}
	}
	for bi, ni := range nodeOf {
		if ni < 0 {
			continue
		}
		boxes[bi].parent = archParent(d, ni, item)
	}

	edges := []lgEdge{}
	for _, e := range d.Edges {
		a, aok := item[e.Source]
		b, bok := item[e.Target]
		if aok && bok && a != b {
			edges = append(edges, lgEdge{a, b, edgeLabelWidth(e)})
		}
	}

	spec := fitSpec{
		orients: []Orientation{LeftRight, TopDown},
		options: func(o Orientation) lgOptions {
			if o == LeftRight {
				return lgOptions{layerGap: GapX, nodeGap: GapY, wrapGap: GapY}
			}
			return lgOptions{layerGap: 90, nodeGap: 60, wrapGap: 50}
		},
		margin: archSVGMargin,
	}
	pos, size := compound(boxes, edges, spec)

	for bi, ni := range nodeOf {
		if ni < 0 {
			continue
		}
		n := &d.Nodes[ni]
		n.Position = model.Position{X: 120 + pos[bi].X, Y: 120 + pos[bi].Y}
		if n.Type == "group" && hasChildren(boxes, bi) {
			n.Width, n.Height = size[bi][0], size[bi][1]
		}
	}
}

// edgeLabelWidth estima a largura da pílula de rótulo que o SVG desenha no
// meio da conexão (rótulo, ou protocolo com a porta).
func edgeLabelWidth(e model.Edge) float64 {
	label := e.Label
	if label == "" {
		label = e.Data.Protocol
		if e.Data.Port > 0 {
			label = fmt.Sprintf("%s :%d", label, e.Data.Port)
		}
	}
	if label == "" {
		return 0
	}
	return float64(len([]rune(label)))*6.2 + 14
}

// groupSize devolve o tamanho de um nó como o canvas o desenha.
func groupSize(n model.Node) (float64, float64) {
	w, h := n.Width, n.Height
	if n.Type == "group" {
		if w <= 0 {
			w = 520
		}
		if h <= 0 {
			h = 360
		}
		return w, h
	}
	if w <= 0 {
		w = NodeWidth
	}
	if h <= 0 {
		h = NodeHeight
	}
	return w, h
}

// archParent devolve o contêiner de um nó: o parentId, se existir, ou o menor
// grupo que contém o centro do nó e é maior que ele (grupos só contêm itens
// menores, o que impede contenção cíclica).
func archParent(d *model.Diagram, i int, item map[string]int) int {
	n := d.Nodes[i]
	if n.ParentID != "" && n.ParentID != n.ID {
		if p, ok := item[n.ParentID]; ok {
			return p
		}
	}
	w, h := groupSize(n)
	cx, cy := n.Position.X+w/2, n.Position.Y+h/2
	best, bestArea := -1, math.Inf(1)
	for j, g := range d.Nodes {
		if j == i || g.Type != "group" {
			continue
		}
		gw, gh := groupSize(g)
		area := gw * gh
		if area <= w*h || area >= bestArea {
			continue
		}
		if cx >= g.Position.X && cx <= g.Position.X+gw && cy >= g.Position.Y && cy <= g.Position.Y+gh {
			best, bestArea = item[g.ID], area
		}
	}
	return best
}

func hasChildren(boxes []cbox, i int) bool {
	for _, b := range boxes {
		if b.parent == i {
			return true
		}
	}
	return false
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
