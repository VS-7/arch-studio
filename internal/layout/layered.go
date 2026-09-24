package layout

import (
	"math"
	"sort"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Motor de layout em camadas (Sugiyama simplificado)
// ---------------------------------------------------------------------------
//
// Compartilhado pelo diagrama de arquitetura e pelos diagramas UML de classes
// e estados. As etapas são as clássicas: quebra de ciclos, atribuição de
// camadas pelo caminho mais longo, ordenação por baricentro (mantendo a ordem
// com menos cruzamentos) e posicionamento que alinha cada nó aos vizinhos.
//
// O objetivo é o documento: camadas largas demais quebram em sub-linhas e a
// combinação de orientação e largura escolhida é a que deixa o diagrama maior
// numa página A4 retrato (ver fitScore).

// Área útil da página A4 do Documento de Requisitos, em pixels CSS: 21 cm menos
// as margens laterais de 2,2 cm, e a altura máxima de uma figura (22 cm).
const (
	PageWidth  = 627.0
	PageHeight = 831.0
)

// Orientation é o sentido do fluxo entre camadas.
type Orientation int

const (
	TopDown Orientation = iota
	LeftRight
)

type lgNode struct{ w, h float64 }

// lgEdge é uma aresta de layout; label é a largura do rótulo desenhado no
// meio dela (0 = sem rótulo).
type lgEdge struct {
	from, to int
	label    float64
}

type lgOptions struct {
	orient   Orientation
	layerGap float64 // entre camadas, no eixo do fluxo
	nodeGap  float64 // entre vizinhos da mesma camada
	wrapGap  float64 // entre as sub-linhas de uma camada quebrada
	maxCross float64 // extensão máxima de uma camada (0 = sem limite)
}

// fitScore é a escala com que um diagrama W×H (mais a margem do SVG em cada
// lado) entra na página, limitada a 1: acima disso ele já cabe em tamanho real.
func fitScore(w, h, margin float64) float64 {
	s := math.Min(PageWidth/(w+2*margin), PageHeight/(h+2*margin))
	return math.Min(s, 1)
}

// layered posiciona os nós (canto superior esquerdo, origem em 0,0) e devolve
// a largura e a altura do conjunto.
func layered(nodes []lgNode, edges []lgEdge, o lgOptions) ([]model.Position, float64, float64) {
	n := len(nodes)
	if n == 0 {
		return nil, 0, 0
	}
	// Da esquerda para a direita é o mesmo cálculo com os eixos trocados.
	ns := nodes
	if o.orient == LeftRight {
		ns = make([]lgNode, n)
		for i, nd := range nodes {
			ns[i] = lgNode{nd.h, nd.w}
		}
	}
	valid := make([]lgEdge, 0, len(edges))
	for _, e := range edges {
		if e.from != e.to && e.from >= 0 && e.to >= 0 && e.from < n && e.to < n {
			valid = append(valid, e)
		}
	}

	layers := assignLayers(n, valid)
	rows, newLayer := wrapLayers(orderLayers(byLayer(layers), valid, layers), ns, o)

	// Arestas que pulam linhas (camadas intermediárias ou sub-linhas de uma
	// camada quebrada) ganham nós fictícios em cada linha do caminho: eles
	// reservam um corredor para a linha reta passar entre os nós.
	rowOf := make([]int, n)
	for r, row := range rows {
		for _, v := range row {
			rowOf[v] = r
		}
	}
	ext := append([]lgNode(nil), ns...)
	extEdges := []lgEdge{}
	indexIn := func(v int) float64 { // posição relativa (0..1) de v na sua linha
		row := rows[rowOf[v]]
		for i, x := range row {
			if x == v {
				return (float64(i) + 0.5) / float64(len(row))
			}
		}
		return 0.5
	}
	for _, e := range valid {
		a, b := e.from, e.to
		if rowOf[a] > rowOf[b] {
			a, b = b, a
		}
		prev := a
		for r := rowOf[a] + 1; r < rowOf[b]; r++ {
			// Entra na linha na mesma posição relativa do nó de cima: com
			// baricentros empatados, o corredor fica onde a reta passa.
			at := int(math.Round(indexIn(prev) * float64(len(rows[r]))))
			dummy := len(ext)
			ext = append(ext, lgNode{w: dummyWidth})
			rowOf = append(rowOf, r)
			rows[r] = append(rows[r][:at], append([]int{dummy}, rows[r][at:]...)...)
			extEdges = append(extEdges, lgEdge{from: prev, to: dummy})
			prev = dummy
		}
		extEdges = append(extEdges, lgEdge{from: prev, to: b})
	}
	if len(ext) > n {
		rows = orderLayers(rows, extEdges, rowOf)
	}
	pos, w, h := placeRows(rows, newLayer, ext, n, extEdges, o)
	pos = pos[:n]

	if o.orient == LeftRight {
		for i := range pos {
			pos[i].X, pos[i].Y = pos[i].Y, pos[i].X
		}
		w, h = h, w
	}
	return pos, w, h
}

// assignLayers quebra ciclos (arestas de retorno da DFS são invertidas), numera
// as camadas pelo caminho mais longo e aproxima as fontes dos seus sucessores.
// Nós isolados vão para uma camada final, depois do fluxo principal.
func assignLayers(n int, edges []lgEdge) []int {
	adj := make([][]int, n)
	indeg := make([]int, n)
	degree := make([]int, n)
	for _, e := range edges {
		adj[e.from] = append(adj[e.from], e.to)
		indeg[e.to]++
		degree[e.from]++
		degree[e.to]++
	}

	dag := make([][]int, n)
	state := make([]int, n) // 0 = novo, 1 = na pilha, 2 = concluído
	var visit func(u int)
	visit = func(u int) {
		state[u] = 1
		for _, v := range adj[u] {
			if state[v] == 1 {
				dag[v] = append(dag[v], u) // aresta de retorno: invertida
				continue
			}
			dag[u] = append(dag[u], v)
			if state[v] == 0 {
				visit(v)
			}
		}
		state[u] = 2
	}
	for i := 0; i < n; i++ {
		if indeg[i] == 0 && state[i] == 0 {
			visit(i)
		}
	}
	for i := 0; i < n; i++ {
		if state[i] == 0 {
			visit(i)
		}
	}

	preds := make([][]int, n)
	in := make([]int, n)
	for u, vs := range dag {
		for _, v := range vs {
			preds[v] = append(preds[v], u)
			in[v]++
		}
	}
	layer := make([]int, n)
	queue := []int{}
	for i := 0; i < n; i++ {
		if in[i] == 0 {
			queue = append(queue, i)
		}
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for _, v := range dag[u] {
			if layer[u]+1 > layer[v] {
				layer[v] = layer[u] + 1
			}
			in[v]--
			if in[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	// Fontes descem para logo acima do sucessor mais alto: evita arestas que
	// atravessam o diagrama inteiro a partir da primeira camada.
	for u := 0; u < n; u++ {
		if len(preds[u]) > 0 || len(dag[u]) == 0 {
			continue
		}
		lowest := math.MaxInt
		for _, v := range dag[u] {
			lowest = min(lowest, layer[v])
		}
		if lowest-1 > layer[u] {
			layer[u] = lowest - 1
		}
	}

	maxLayer, connected := 0, false
	for i := 0; i < n; i++ {
		if degree[i] > 0 {
			connected = true
			maxLayer = max(maxLayer, layer[i])
		}
	}
	if connected {
		for i := 0; i < n; i++ {
			if degree[i] == 0 {
				layer[i] = maxLayer + 1
			}
		}
	}
	return layer
}

// dummyWidth é a largura reservada para uma aresta que atravessa uma linha.
const dummyWidth = 24.0

// byLayer agrupa os índices por camada, na ordem dos índices (o chamador os
// entrega já em ordem estável).
func byLayer(layers []int) [][]int {
	maxLayer := 0
	for _, l := range layers {
		maxLayer = max(maxLayer, l)
	}
	order := make([][]int, maxLayer+1)
	for i, l := range layers {
		order[l] = append(order[l], i)
	}
	return order
}

// orderLayers reordena os nós de cada camada pelo baricentro dos vizinhos, em
// varreduras alternadas, e fica com a ordem de menor número de cruzamentos.
func orderLayers(order [][]int, edges []lgEdge, layers []int) [][]int {
	order = cloneOrder(order)
	maxLayer := len(order) - 1
	n := len(layers)
	neighbors := make([][]int, n)
	for _, e := range edges {
		neighbors[e.from] = append(neighbors[e.from], e.to)
		neighbors[e.to] = append(neighbors[e.to], e.from)
	}

	rel := make([]float64, n)
	refresh := func(l int) {
		for i, v := range order[l] {
			rel[v] = (float64(i) + 0.5) / float64(len(order[l]))
		}
	}
	for l := range order {
		refresh(l)
	}
	sortLayer := func(l int, before bool) {
		bary := make(map[int]float64, len(order[l]))
		for _, v := range order[l] {
			sum, cnt := 0.0, 0
			for _, u := range neighbors[v] {
				if (before && layers[u] < l) || (!before && layers[u] > l) {
					sum += rel[u]
					cnt++
				}
			}
			if cnt > 0 {
				bary[v] = sum / float64(cnt)
			} else {
				bary[v] = rel[v]
			}
		}
		sort.SliceStable(order[l], func(a, b int) bool { return bary[order[l][a]] < bary[order[l][b]] })
		refresh(l)
	}

	best := cloneOrder(order)
	bestCross := crossings(order, edges, layers)
	for iter := 0; iter < 6 && bestCross > 0; iter++ {
		for l := 1; l <= maxLayer; l++ {
			sortLayer(l, true)
		}
		for l := maxLayer - 1; l >= 0; l-- {
			sortLayer(l, false)
		}
		if c := crossings(order, edges, layers); c < bestCross {
			best, bestCross = cloneOrder(order), c
		}
	}
	return best
}

func cloneOrder(order [][]int) [][]int {
	out := make([][]int, len(order))
	for i, l := range order {
		out[i] = append([]int(nil), l...)
	}
	return out
}

// crossings conta os cruzamentos entre arestas de camadas vizinhas.
func crossings(order [][]int, edges []lgEdge, layers []int) int {
	idx := map[int]int{}
	for _, l := range order {
		for i, v := range l {
			idx[v] = i
		}
	}
	byLayer := map[int][][2]int{}
	for _, e := range edges {
		a, b := e.from, e.to
		if layers[a] > layers[b] {
			a, b = b, a
		}
		if layers[b]-layers[a] != 1 {
			continue
		}
		byLayer[layers[a]] = append(byLayer[layers[a]], [2]int{idx[a], idx[b]})
	}
	total := 0
	for _, es := range byLayer {
		for i := 0; i < len(es); i++ {
			for j := i + 1; j < len(es); j++ {
				if (es[i][0]-es[j][0])*(es[i][1]-es[j][1]) < 0 {
					total++
				}
			}
		}
	}
	return total
}

// wrapLayers quebra camadas mais largas que maxCross em sub-linhas de tamanho
// equilibrado. newLayer[i] indica se a linha i começa uma camada.
func wrapLayers(order [][]int, nodes []lgNode, o lgOptions) ([][]int, []bool) {
	rows := [][]int{}
	newLayer := []bool{}
	for _, layer := range order {
		if len(layer) == 0 {
			continue
		}
		chunks := 1
		if o.maxCross > 0 {
			width := 0.0
			for i, v := range layer {
				if i > 0 && width+o.nodeGap+nodes[v].w > o.maxCross {
					chunks++
					width = nodes[v].w
					continue
				}
				if i > 0 {
					width += o.nodeGap
				}
				width += nodes[v].w
			}
		}
		per := (len(layer) + chunks - 1) / chunks
		for start := 0; start < len(layer); start += per {
			// Cópia: cada linha recebe nós fictícios depois, sem pisar na vizinha.
			rows = append(rows, append([]int(nil), layer[start:min(start+per, len(layer))]...))
			newLayer = append(newLayer, start == 0)
		}
	}
	return rows, newLayer
}

// placeRows empilha as linhas e alinha cada nó ao centro dos seus vizinhos já
// posicionados, sem sobrepor os irmãos (ver pack1D). Só os `real` primeiros
// nós (os demais são fictícios) contam para a caixa ocupada.
func placeRows(rows [][]int, newLayer []bool, nodes []lgNode, real int, edges []lgEdge, o lgOptions) ([]model.Position, float64, float64) {
	n := len(nodes)
	rowOf := make([]int, n)
	for r, row := range rows {
		for _, v := range row {
			rowOf[v] = r
		}
	}
	neighbors := make([][]int, n)
	for _, e := range edges {
		neighbors[e.from] = append(neighbors[e.from], e.to)
		neighbors[e.to] = append(neighbors[e.to], e.from)
	}

	// Posição horizontal inicial: cada linha centralizada na mais larga.
	cx := make([]float64, n)
	rowWidth := func(row []int) float64 {
		w := 0.0
		for i, v := range row {
			if i > 0 {
				w += o.nodeGap
			}
			w += nodes[v].w
		}
		return w
	}
	widest := 0.0
	for _, row := range rows {
		widest = math.Max(widest, rowWidth(row))
	}
	for _, row := range rows {
		x := (widest - rowWidth(row)) / 2
		for _, v := range row {
			cx[v] = x + nodes[v].w/2
			x += nodes[v].w + o.nodeGap
		}
	}

	align := func(r int, fromAbove bool) {
		row := rows[r]
		desired := make([]float64, len(row))
		seps := make([]float64, len(row))
		for i, v := range row {
			sum, cnt := 0.0, 0
			for _, u := range neighbors[v] {
				if (fromAbove && rowOf[u] < r) || (!fromAbove && rowOf[u] > r) {
					sum += cx[u]
					cnt++
				}
			}
			desired[i] = cx[v]
			if cnt > 0 {
				desired[i] = sum / float64(cnt)
			}
			if i > 0 {
				seps[i] = (nodes[row[i-1]].w+nodes[v].w)/2 + o.nodeGap
			}
		}
		for i, x := range pack1D(desired, seps) {
			cx[row[i]] = x
		}
	}
	for pass := 0; pass < 3; pass++ {
		for r := 1; r < len(rows); r++ {
			align(r, true)
		}
		for r := len(rows) - 2; r >= 0; r-- {
			align(r, false)
		}
	}

	pos := make([]model.Position, n)
	minX, maxX := math.Inf(1), math.Inf(-1)
	for v := 0; v < real; v++ {
		minX = math.Min(minX, cx[v]-nodes[v].w/2)
		maxX = math.Max(maxX, cx[v]+nodes[v].w/2)
	}
	y := 0.0
	for r, row := range rows {
		if r > 0 {
			if newLayer[r] {
				y += o.layerGap
			} else {
				y += o.wrapGap
			}
		}
		height := 0.0
		for _, v := range row {
			height = math.Max(height, nodes[v].h)
		}
		for _, v := range row {
			pos[v] = model.Position{X: cx[v] - nodes[v].w/2 - minX, Y: y + (height-nodes[v].h)/2}
		}
		y += height
	}
	return pos, maxX - minX, y
}

// pack1D posiciona itens em ordem fixa o mais perto possível das posições
// desejadas, respeitando a distância mínima seps[i] entre o item i-1 e o i
// (mínimos quadrados com restrição de ordem: pool adjacent violators).
func pack1D(desired, seps []float64) []float64 {
	n := len(desired)
	offset := make([]float64, n)
	for i := 1; i < n; i++ {
		offset[i] = offset[i-1] + seps[i]
	}
	type block struct {
		start, end int
		sum        float64
	}
	blocks := []block{}
	for i := 0; i < n; i++ {
		blocks = append(blocks, block{i, i + 1, desired[i] - offset[i]})
		for len(blocks) > 1 {
			a, b := blocks[len(blocks)-2], blocks[len(blocks)-1]
			if a.sum/float64(a.end-a.start) <= b.sum/float64(b.end-b.start) {
				break
			}
			blocks = append(blocks[:len(blocks)-2], block{a.start, b.end, a.sum + b.sum})
		}
	}
	out := make([]float64, n)
	for _, b := range blocks {
		v := b.sum / float64(b.end-b.start)
		for i := b.start; i < b.end; i++ {
			out[i] = v + offset[i]
		}
	}
	return out
}

// readingOrder ordena caixas (x, y, largura, altura) como se lê: por linhas,
// de cima para baixo, e da esquerda para a direita em cada linha. Caixas cujos
// centros verticais distam até 12px contam como a mesma linha — um layout em
// camadas centraliza nós de alturas diferentes na linha, e é essa ordem que
// precisa sobreviver a uma nova reorganização.
func readingOrder(boxes [][4]float64) []int {
	idx := make([]int, len(boxes))
	for i := range idx {
		idx[i] = i
	}
	cy := func(i int) float64 { return boxes[i][1] + boxes[i][3]/2 }
	sort.SliceStable(idx, func(a, b int) bool { return cy(idx[a]) < cy(idx[b]) })
	row := make([]int, len(boxes))
	for k := 1; k < len(idx); k++ {
		row[idx[k]] = row[idx[k-1]]
		if cy(idx[k])-cy(idx[k-1]) > 12 {
			row[idx[k]]++
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		if row[idx[a]] != row[idx[b]] {
			return row[idx[a]] < row[idx[b]]
		}
		return boxes[idx[a]][0] < boxes[idx[b]][0]
	})
	return idx
}

// ---------------------------------------------------------------------------
// Escolha pela página
// ---------------------------------------------------------------------------

// fitSpec descreve as alternativas testadas por arrangeFit.
type fitSpec struct {
	orients []Orientation               // em ordem de preferência
	options func(Orientation) lgOptions // espaçamentos de cada orientação
	margin  float64                     // margem do SVG em cada lado
	// wrapAbove: só quebra camadas cuja extensão passe disto (0 = sempre
	// tenta). Dentro de contêineres, quebrar empurra membros para baixo de
	// irmãos e as arestas que vêm de fora passariam por cima deles.
	wrapAbove float64
}

// innerWrapAbove é a extensão a partir da qual o conteúdo de um contêiner
// também pode quebrar em sub-linhas.
const innerWrapAbove = PageWidth * 1.5

// throughPenalty é quanto cada aresta que atravessa um nó custa na escolha:
// uma linha passando por cima de outro elemento parece ligá-lo, então vale
// mais um diagrama menor na página (25 pontos de escala por linha).
const throughPenalty = 0.25

// labelPenalty é o custo de cada rótulo que encosta noutro rótulo ou num nó.
const labelPenalty = 0.1

// arrangeFit testa cada orientação sem quebra e com camadas cada vez mais
// estreitas — e, se há rótulos, também com espaçamento maior — e fica com a
// que ocupa melhor a página, descontadas as arestas que atravessam nós e os
// rótulos sobrepostos. Em empate, vence a primeira alternativa (orientação
// preferida, espaçamento normal, menos quebras).
func arrangeFit(nodes []lgNode, edges []lgEdge, spec fitSpec) ([]model.Position, float64, float64) {
	var bestPos []model.Position
	bestW, bestH, bestScore := 0.0, 0.0, math.Inf(-1)
	consider := func(pos []model.Position, w, h float64) {
		s := fitScore(w, h, spec.margin) -
			throughPenalty*float64(through(nodes, edges, pos)) -
			labelPenalty*float64(labelClashes(nodes, edges, pos))
		if s > bestScore+0.01 {
			bestPos, bestW, bestH, bestScore = pos, w, h, s
		}
	}
	spacings := []float64{1}
	for _, e := range edges {
		if e.label > 0 {
			spacings = append(spacings, 1.6)
			break
		}
	}
	for _, orient := range spec.orients {
		for _, spacing := range spacings {
			o := spec.options(orient)
			o.orient = orient
			o.nodeGap *= spacing
			o.layerGap *= spacing
			arrangeOrient(nodes, edges, o, spec.wrapAbove, consider)
		}
	}
	return bestPos, bestW, bestH
}

// arrangeOrient gera as alternativas de uma orientação: sem quebra e com
// camadas cada vez mais estreitas.
func arrangeOrient(nodes []lgNode, edges []lgEdge, o lgOptions, wrapAbove float64, consider func([]model.Position, float64, float64)) {
	orient := o.orient
	{
		o.maxCross = 0
		pos, w, h := layered(nodes, edges, o)
		consider(pos, w, h)

		cross, largest := w, 0.0
		if orient == LeftRight {
			cross = h
		}
		if cross <= wrapAbove {
			return
		}
		for _, nd := range nodes {
			if orient == LeftRight {
				largest = math.Max(largest, nd.h)
			} else {
				largest = math.Max(largest, nd.w)
			}
		}
		for _, f := range []float64{0.8, 0.65, 0.5, 0.4, 0.3, 0.22} {
			o.maxCross = cross * f
			if o.maxCross < largest {
				break
			}
			pos, w, h := layered(nodes, edges, o)
			consider(pos, w, h)
		}
	}
}

// labelClashes conta os rótulos (no meio da reta entre os centros) que
// encostam noutro rótulo ou num nó.
func labelClashes(nodes []lgNode, edges []lgEdge, pos []model.Position) int {
	type rect struct{ x, y, w, h float64 }
	labels := []rect{}
	for _, e := range edges {
		if e.label <= 0 {
			continue
		}
		a, b := nodes[e.from], nodes[e.to]
		mx := (pos[e.from].X + a.w/2 + pos[e.to].X + b.w/2) / 2
		my := (pos[e.from].Y + a.h/2 + pos[e.to].Y + b.h/2) / 2
		labels = append(labels, rect{mx - e.label/2 - 4, my - 18, e.label + 8, 20})
	}
	hit := func(p, q rect) bool { return p.x < q.x+q.w && q.x < p.x+p.w && p.y < q.y+q.h && q.y < p.y+p.h }
	count := 0
	for i, l := range labels {
		for _, m := range labels[i+1:] {
			if hit(l, m) {
				count++
			}
		}
		for v, nd := range nodes {
			if hit(l, rect{pos[v].X, pos[v].Y, nd.w, nd.h}) {
				count++
			}
		}
	}
	return count
}

// through conta os pares (aresta, nó) em que a reta entre os centros das
// pontas atravessa outro nó ou passa a menos de 4px dele (raspão também
// confunde: parece que a linha começa ou termina ali).
func through(nodes []lgNode, edges []lgEdge, pos []model.Position) int {
	const halo = 4.0
	count := 0
	for _, e := range edges {
		a, b := nodes[e.from], nodes[e.to]
		x1, y1 := pos[e.from].X+a.w/2, pos[e.from].Y+a.h/2
		x2, y2 := pos[e.to].X+b.w/2, pos[e.to].Y+b.h/2
		for v, nd := range nodes {
			if v == e.from || v == e.to || nd.w <= 0 || nd.h <= 0 {
				continue
			}
			if segmentHitsRect(x1, y1, x2, y2, pos[v].X-halo, pos[v].Y-halo, nd.w+2*halo, nd.h+2*halo) {
				count++
			}
		}
	}
	return count
}

// segmentHitsRect informa se o segmento cruza o retângulo (Liang–Barsky).
func segmentHitsRect(x1, y1, x2, y2, rx, ry, rw, rh float64) bool {
	dx, dy := x2-x1, y2-y1
	t0, t1 := 0.0, 1.0
	for _, c := range [][2]float64{{-dx, x1 - rx}, {dx, rx + rw - x1}, {-dy, y1 - ry}, {dy, ry + rh - y1}} {
		p, q := c[0], c[1]
		if p == 0 {
			if q < 0 {
				return false
			}
			continue
		}
		t := q / p
		if p < 0 {
			t0 = math.Max(t0, t)
		} else {
			t1 = math.Min(t1, t)
		}
		if t0 > t1 {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// Layout composto (contêineres)
// ---------------------------------------------------------------------------

// cbox é um item do layout composto: uma folha ou um contêiner (grupo, pacote,
// estado composto) cujo tamanho passa a ser o do conteúdo mais o recuo.
type cbox struct {
	w, h   float64
	parent int        // índice do contêiner; -1 = raiz
	pad    [4]float64 // recuo interno: topo, direita, base, esquerda
	minW   float64    // largura mínima do contêiner (rótulo)
}

// compound posiciona recursivamente o conteúdo de cada contêiner e depois o
// próprio contêiner no nível de cima, onde as arestas dos filhos passam a
// valer para ele. Devolve a posição absoluta e o tamanho final de cada item.
func compound(boxes []cbox, edges []lgEdge, spec fitSpec) ([]model.Position, [][2]float64) {
	n := len(boxes)
	// Pai efetivo: índices inválidos e contenção cíclica viram raiz.
	parent := make([]int, n)
	for i, b := range boxes {
		parent[i] = b.parent
		if b.parent < 0 || b.parent >= n {
			parent[i] = -1
		}
	}
	for i := range parent {
		for v, depth := parent[i], 0; v >= 0; v, depth = parent[v], depth+1 {
			if v == i || depth > n {
				parent[i] = -1
				break
			}
		}
	}
	children := map[int][]int{}
	for i := range boxes {
		children[parent[i]] = append(children[parent[i]], i)
	}
	size := make([][2]float64, n)
	for i, b := range boxes {
		size[i] = [2]float64{b.w, b.h}
	}
	rel := make([]model.Position, n) // relativo à área interna do contêiner
	inner := make([]model.Position, n)

	// top devolve o ancestral de v que é filho direto de `container` (ou -1).
	top := func(v, container int) int {
		for v >= 0 {
			if parent[v] == container {
				return v
			}
			v = parent[v]
		}
		return -1
	}

	var solve func(container int)
	solve = func(container int) {
		kids := children[container]
		if len(kids) == 0 {
			return
		}
		for _, k := range kids {
			if len(children[k]) > 0 {
				solve(k)
			}
		}
		local := map[int]int{}
		nodes := make([]lgNode, len(kids))
		for i, k := range kids {
			local[k] = i
			nodes[i] = lgNode{size[k][0], size[k][1]}
		}
		seen := map[[2]int]int{}
		projected := []lgEdge{}
		for _, e := range edges {
			a, b := top(e.from, container), top(e.to, container)
			if a < 0 || b < 0 || a == b {
				continue
			}
			key := [2]int{local[a], local[b]}
			if i, ok := seen[key]; ok {
				projected[i].label = math.Max(projected[i].label, e.label)
				continue
			}
			seen[key] = len(projected)
			projected = append(projected, lgEdge{local[a], local[b], e.label})
		}
		level := spec
		if container >= 0 {
			level.wrapAbove = innerWrapAbove
		}
		pos, w, h := arrangeFit(nodes, projected, level)
		for i, k := range kids {
			rel[k] = pos[i]
		}
		if container >= 0 {
			b := boxes[container]
			cw := math.Max(w+b.pad[1]+b.pad[3], b.minW)
			size[container] = [2]float64{cw, h + b.pad[0] + b.pad[2]}
			// Conteúdo mais estreito que o rótulo fica centralizado.
			inner[container] = model.Position{X: b.pad[3] + (cw-w-b.pad[1]-b.pad[3])/2, Y: b.pad[0]}
		}
	}
	solve(-1)

	abs := make([]model.Position, n)
	var place func(container int, origin model.Position)
	place = func(container int, origin model.Position) {
		for _, k := range children[container] {
			abs[k] = model.Position{X: origin.X + rel[k].X, Y: origin.Y + rel[k].Y}
			if len(children[k]) > 0 {
				place(k, model.Position{X: abs[k].X + inner[k].X, Y: abs[k].Y + inner[k].Y})
			}
		}
	}
	place(-1, model.Position{})
	return abs, size
}
