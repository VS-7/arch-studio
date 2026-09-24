package layout

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Reorganização dos diagramas UML
// ---------------------------------------------------------------------------
//
// Como no diagrama macro, só roda a pedido explícito do usuário ("Reorganizar")
// ou ao criar um diagrama gerado — nunca em escritas incrementais de agentes.
// Cada tipo tem a sua disposição, sempre pensada para a figura do Documento de
// Requisitos: caber numa página A4 retrato com folga entre os elementos.

// UMLMetrics mede elementos e textos como o renderizador os desenha, para que
// o layout reserve o espaço real (classes crescem com os membros, nomes longos
// alargam casos de uso e linhas de vida).
type UMLMetrics interface {
	Size(e model.UMLElement) (w, h float64)
	TextWidth(s string, size float64) float64
}

// defaultMetrics é a medição de reserva: tamanhos padrão do modelo e largura
// média de caractere da fonte de 12px.
type defaultMetrics struct{}

func (defaultMetrics) Size(e model.UMLElement) (float64, float64) { return e.Size() }
func (defaultMetrics) TextWidth(s string, size float64) float64 {
	return float64(len([]rune(s))) * 6.8 * size / 12
}

const (
	umlSVGMargin = 30.0 // margem do SVG UML em cada lado (svgexport)
	umlOrigin    = 40.0 // canto superior esquerdo do diagrama reorganizado
)

// UML reorganiza o diagrama inteiro. Não altera ids, nomes, contenção
// (parent_id) nem a ordem das mensagens: só posições e, quando necessário,
// tamanhos (contêineres envolvem o conteúdo; nomes longos ganham largura).
func UML(d *model.UMLDiagram, m UMLMetrics) {
	if m == nil {
		m = defaultMetrics{}
	}
	if len(d.Elements) == 0 {
		return
	}
	switch d.Kind {
	case model.UMLKindUseCase:
		layoutUseCase(d, m)
	case model.UMLKindClass:
		layoutClass(d, m)
	case model.UMLKindState:
		layoutState(d, m)
	case model.UMLKindSequence:
		layoutSequence(d, m) // a geometria vertical é fixa: não translada
		return
	}
	moveToOrigin(d, m)
}

// moveToOrigin translada o diagrama para que o desenho comece em (40, 40).
func moveToOrigin(d *model.UMLDiagram, m UMLMetrics) {
	minX, minY := math.Inf(1), math.Inf(1)
	for _, e := range d.Elements {
		w, _ := m.Size(e)
		left := e.Position.X
		if e.Type == "actor" {
			// O nome do ator é centralizado e pode passar da largura da figura.
			left = math.Min(left, e.Position.X+w/2-m.TextWidth(e.Name, 12)/2)
		}
		minX = math.Min(minX, left)
		minY = math.Min(minY, e.Position.Y)
	}
	dx, dy := umlOrigin-minX, umlOrigin-minY
	for i := range d.Elements {
		d.Elements[i].Position.X = math.Round(d.Elements[i].Position.X + dx)
		d.Elements[i].Position.Y = math.Round(d.Elements[i].Position.Y + dy)
	}
}

// inputOrder devolve os índices dos elementos na ordem de leitura atual: o
// ponto de partida do layout, que assim respeita a disposição que o usuário
// já tinha e é estável (reorganizar de novo não troca ninguém de lugar).
func inputOrder(d *model.UMLDiagram, m UMLMetrics) []int {
	boxes := make([][4]float64, len(d.Elements))
	for i, e := range d.Elements {
		w, h := m.Size(e)
		boxes[i] = [4]float64{e.Position.X, e.Position.Y, w, h}
	}
	return readingOrder(boxes)
}

// ---------------------------------------------------------------------------
// Classes e estados: layout em camadas com contêineres
// ---------------------------------------------------------------------------

// umlEdge é uma aresta de layout entre dois elementos (por id), já no sentido
// do fluxo desejado, com a largura do rótulo do meio (0 = sem rótulo).
type umlEdge struct {
	from, to string
	label    float64
}

// layoutCompoundUML aplica o motor composto: pacotes e estados compostos são
// contêineres redimensionados para envolver o conteúdo.
func layoutCompoundUML(d *model.UMLDiagram, m UMLMetrics, edges []umlEdge, spec fitSpec) {
	order := inputOrder(d, m)
	item := map[string]int{}
	boxes := make([]cbox, len(order))
	for bi, i := range order {
		item[d.Elements[i].ID] = bi
	}
	for bi, i := range order {
		e := d.Elements[i]
		w, h := m.Size(e)
		pad, minW := containerPad(e, m)
		parent := -1
		if p, ok := item[e.ParentID]; ok && e.ParentID != e.ID {
			parent = p
		}
		boxes[bi] = cbox{w: w, h: h, parent: parent, pad: pad, minW: minW}
	}
	lg := []lgEdge{}
	for _, e := range edges {
		a, aok := item[e.from]
		b, bok := item[e.to]
		if aok && bok && a != b {
			lg = append(lg, lgEdge{a, b, e.label})
		}
	}
	pos, size := compound(boxes, lg, spec)
	for bi, i := range order {
		e := &d.Elements[i]
		e.Position = pos[bi]
		if hasChildren(boxes, bi) {
			e.Width, e.Height = math.Ceil(size[bi][0]), math.Ceil(size[bi][1])
		}
	}
}

// containerPad é o recuo interno de cada contêiner, pelo que o SVG desenha no
// topo (aba do pacote, nome e atividades do estado composto).
func containerPad(e model.UMLElement, m UMLMetrics) ([4]float64, float64) {
	switch e.Type {
	case "package":
		top := 22.0 + 26
		if e.Stereotype != "" {
			top += 18
		}
		return [4]float64{top, 24, 24, 24}, m.TextWidth(e.Name, 11.5)*1.08 + 60
	case "state":
		top := 28.0
		if e.Stereotype != "" {
			top += 13
		}
		for _, a := range []string{e.Entry, e.Do, e.Exit} {
			if strings.TrimSpace(a) != "" {
				top += 16.5
			}
		}
		return [4]float64{top + 22, 24, 24, 24}, m.TextWidth(e.Name, 12)*1.08 + 48
	case "boundary":
		return [4]float64{50, 40, 30, 40}, m.TextWidth(e.Name, 12)*1.08 + 40
	}
	return [4]float64{24, 24, 24, 24}, 0
}

// layoutClass: hierarquias de cima para baixo (superclasse e interface acima,
// todo acima da parte); as demais relações seguem origem → destino. Notas
// ficam logo acima do elemento que anotam.
func layoutClass(d *model.UMLDiagram, m UMLMetrics) {
	edges := []umlEdge{}
	for _, r := range d.Relations {
		label := 0.0
		if r.Name != "" {
			label = m.TextWidth(r.Name, 11)
		}
		switch r.Type {
		case "generalization", "realization", "aggregation", "composition":
			edges = append(edges, umlEdge{r.Target, r.Source, label})
		default:
			edges = append(edges, umlEdge{r.Source, r.Target, label})
		}
	}
	layoutCompoundUML(d, m, edges, fitSpec{
		orients: []Orientation{TopDown},
		options: func(Orientation) lgOptions {
			return lgOptions{layerGap: 90, nodeGap: 60, wrapGap: 60}
		},
		margin: umlSVGMargin,
	})
}

// layoutState: transições de cima para baixo (ou da esquerda para a direita,
// se couber melhor na página), com espaço para os rótulos `evento [guarda] /
// efeito` entre as camadas.
func layoutState(d *model.UMLDiagram, m UMLMetrics) {
	fitStates(d, m)
	edges := []umlEdge{}
	label := 0.0
	for _, r := range d.Relations {
		w := 0.0
		if r.Type == "transition" {
			w = m.TextWidth(transitionText(r), 11)
		}
		edges = append(edges, umlEdge{r.Source, r.Target, w})
		label = math.Max(label, w)
	}
	layoutCompoundUML(d, m, edges, fitSpec{
		orients: []Orientation{TopDown, LeftRight},
		options: func(o Orientation) lgOptions {
			if o == LeftRight {
				return lgOptions{layerGap: clamp(label+40, 90, 280), nodeGap: 50, wrapGap: 50}
			}
			return lgOptions{layerGap: 76, nodeGap: clamp(label*0.6, 60, 180), wrapGap: 50}
		},
		margin: umlSVGMargin,
	})
}

// fitStates alarga (e alonga) os estados de tamanho padrão cujo nome ou
// atividades (entry/do/exit) não cabem na caixa desenhada.
func fitStates(d *model.UMLDiagram, m UMLMetrics) {
	for i := range d.Elements {
		e := &d.Elements[i]
		if e.Type != "state" {
			continue
		}
		w, h := m.Size(*e)
		needW := m.TextWidth(e.Name, 12)*1.08 + 28
		if e.Stereotype != "" {
			needW = math.Max(needW, m.TextWidth("«"+e.Stereotype+"»", 11)+28)
		}
		lines := 0
		for _, a := range [][2]string{{"entry", e.Entry}, {"do", e.Do}, {"exit", e.Exit}} {
			if strings.TrimSpace(a[1]) != "" {
				needW = math.Max(needW, m.TextWidth(a[0]+" / "+a[1], 11)+24)
				lines++
			}
		}
		needH := 0.0
		if lines > 0 {
			needH = 28 + 16.5*float64(lines) + 10
			if e.Stereotype != "" {
				needH += 13
			}
		}
		if e.Width <= 0 && needW > w {
			e.Width = math.Ceil(needW/10) * 10
		}
		if e.Height <= 0 && needH > h {
			e.Height = math.Ceil(needH/2) * 2
		}
	}
}

// transitionText é o rótulo de uma transição, como o SVG o escreve.
func transitionText(r model.UMLRelation) string {
	trigger := r.Trigger
	if trigger == "" {
		trigger = r.Name
	}
	s := strings.TrimSpace(trigger)
	if r.Guard != "" {
		s = strings.TrimSpace(s + " [" + r.Guard + "]")
	}
	if r.Effect != "" {
		s = strings.TrimSpace(s + " / " + r.Effect)
	}
	return s
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

// ---------------------------------------------------------------------------
// Sequência
// ---------------------------------------------------------------------------

// Geometria vertical do diagrama de sequência (web/src/lib/umlMeta.ts, SEQ).
const (
	seqTop      = 40.0
	seqHeader   = 44.0
	seqFirstGap = 36.0
	seqStep     = 44.0
	seqLaneGap  = 40.0 // folga mínima entre cabeçalhos vizinhos
)

// layoutSequence mantém a ordem dos participantes e das mensagens e recalcula
// só o espaçamento horizontal: cada par de linhas de vida fica afastado o
// bastante para os rótulos das mensagens entre elas não se sobreporem.
// Fragmentos e notas acompanham as linhas de vida que cobriam.
func layoutSequence(d *model.UMLDiagram, m UMLMetrics) {
	lanes := []int{}
	for i, e := range d.Elements {
		if e.Type == "lifeline" {
			lanes = append(lanes, i)
		}
	}
	if len(lanes) == 0 {
		return
	}
	sort.SliceStable(lanes, func(a, b int) bool {
		return d.Elements[lanes[a]].Position.X < d.Elements[lanes[b]].Position.X
	})
	laneOf := map[string]int{}
	oldCX := make([]float64, len(lanes))
	span := make([]float64, len(lanes)) // largura ocupada (caixa ou nome)
	for k, i := range lanes {
		e := &d.Elements[i]
		laneOf[e.ID] = k
		w, _ := m.Size(*e)
		oldCX[k] = e.Position.X + w/2
		name := m.TextWidth(e.Name, 12.5) + 24
		kind := e.LifelineKind
		if (kind == "" || kind == "participant") && e.Width <= 0 && name > w {
			e.Width = math.Ceil(name) // cabeçalho do participante cabe o nome
			w = e.Width
		}
		span[k] = math.Max(w, name)
	}

	// Mensagens na ordem vertical e a distância mínima que cada uma exige.
	msgs := []model.UMLRelation{}
	for _, r := range d.Relations {
		if r.Type == "message" {
			msgs = append(msgs, r)
		}
	}
	sort.SliceStable(msgs, func(a, b int) bool {
		oa, ob := msgs[a].Order, msgs[b].Order
		if oa == 0 {
			return false
		}
		if ob == 0 {
			return true
		}
		return oa < ob
	})
	need := map[[2]int]float64{}
	require := func(a, b int, dist float64) {
		key := [2]int{a, b}
		need[key] = math.Max(need[key], dist)
	}
	for n, r := range msgs {
		a, aok := laneOf[r.Source]
		b, bok := laneOf[r.Target]
		if !aok || !bok {
			continue
		}
		label := m.TextWidth(messageText(r, n+1), 11)
		if a == b {
			// Auto-mensagem: laço de 34px e rótulo à direita da linha de vida.
			if a+1 < len(lanes) {
				require(a, a+1, 40+label+24)
			}
			continue
		}
		if a > b {
			a, b = b, a
		}
		require(a, b, label+40)
	}

	cx := make([]float64, len(lanes))
	cx[0] = seqTop + span[0]/2
	for k := 1; k < len(lanes); k++ {
		cx[k] = cx[k-1] + (span[k-1]+span[k])/2 + seqLaneGap
		for j := 0; j < k; j++ {
			if dist, ok := need[[2]int{j, k}]; ok {
				cx[k] = math.Max(cx[k], cx[j]+dist)
			}
		}
	}
	for k, i := range lanes {
		e := &d.Elements[i]
		w, _ := m.Size(*e)
		e.Position = model.Position{X: math.Round(cx[k] - w/2), Y: seqTop}
	}

	// Fragmentos e notas: x remapeado pelas linhas de vida (interpolação entre
	// os centros antigos e os novos). Fragmentos também passam a cobrir todas
	// as linhas de vida das mensagens que estão dentro deles.
	remap := laneMapper(oldCX, cx)
	for i := range d.Elements {
		e := &d.Elements[i]
		w, h := m.Size(*e)
		switch e.Type {
		case "fragment":
			l, r := remap(e.Position.X), remap(e.Position.X+w)
			for n, msg := range msgs {
				y := seqTop + seqHeader + seqFirstGap + float64(n)*seqStep
				if y < e.Position.Y || y > e.Position.Y+h {
					continue
				}
				for _, id := range []string{msg.Source, msg.Target} {
					if k, ok := laneOf[id]; ok {
						l = math.Min(l, cx[k]-span[k]/2-12)
						r = math.Max(r, cx[k]+span[k]/2+12)
					}
				}
			}
			l, r = math.Round(l), math.Round(r)
			e.Position.X = l
			e.Width = math.Max(r-l, 120)
		case "note":
			e.Position.X = math.Round(remap(e.Position.X+w/2) - w/2)
		}
	}
}

// messageText é o rótulo de uma mensagem ("3: «create» nome").
func messageText(r model.UMLRelation, n int) string {
	text := r.Name
	switch r.MessageKind {
	case "create":
		text = strings.TrimSpace("«create» " + text)
	case "destroy":
		text = strings.TrimSpace("«destroy» " + text)
	}
	return strings.TrimSpace(strconv.Itoa(n) + ": " + text)
}

// laneMapper devolve a função linear por partes que leva uma coordenada x do
// espaço antigo das linhas de vida para o novo; fora das pontas, desloca junto.
func laneMapper(old, now []float64) func(float64) float64 {
	type pair struct{ o, n float64 }
	pts := make([]pair, len(old))
	for i := range old {
		pts[i] = pair{old[i], now[i]}
	}
	sort.SliceStable(pts, func(a, b int) bool { return pts[a].o < pts[b].o })
	return func(x float64) float64 {
		if x <= pts[0].o {
			return pts[0].n + (x - pts[0].o)
		}
		last := pts[len(pts)-1]
		if x >= last.o {
			return last.n + (x - last.o)
		}
		for i := 1; i < len(pts); i++ {
			a, b := pts[i-1], pts[i]
			if x <= b.o {
				if b.o == a.o {
					return b.n
				}
				t := (x - a.o) / (b.o - a.o)
				return a.n + t*(b.n-a.n)
			}
		}
		return x
	}
}
