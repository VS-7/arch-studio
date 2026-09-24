package layout

import (
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Casos de uso
// ---------------------------------------------------------------------------
//
// Cada fronteira do sistema vira uma grade de casos de uso e os atores ficam em
// colunas laterais, na altura média dos casos de uso que usam. Duas famílias
// de disposição concorrem, cada uma com 1 a 3 colunas:
//
//   - clássica: todos os atores à esquerda, casos de uso coluna a coluna;
//   - dividida: atores repartidos entre esquerda e direita, cada lado com a
//     sua coluna de casos de uso (e uma central para os que têm atores dos
//     dois lados), para que as linhas não atravessem as outras colunas.
//
// Vence a que ocupa melhor a página, descontadas as linhas que cruzam elipses
// ou atores; em empate, a clássica com menos colunas.

// Espaçamentos do diagrama de casos de uso.
const (
	ucColGap   = 56.0  // entre colunas de casos de uso
	ucRowGap   = 30.0  // entre linhas de casos de uso
	ucGroupGap = 60.0  // entre fronteiras empilhadas
	ucSideGap  = 110.0 // entre a coluna de atores e a fronteira
	ucActorGap = 30.0  // entre atores da mesma coluna
	ucNoteGap  = 30.0

	ucFigureMid     = 29.0  // centro vertical da figura do ator (40x58)
	ucCrossPenalty  = 0.015 // desconto na nota por linha que cruza um elemento
	ucBruteForceMax = 14    // até quantos atores a divisão é exaustiva
)

// ucGroup é uma fronteira do sistema (ou o conjunto de casos de uso soltos,
// boundary = -1) com os seus casos de uso já ordenados.
type ucGroup struct {
	boundary int
	ucs      []int
}

type ucPlan struct {
	pos   map[int]model.Position
	size  map[int][2]float64 // fronteiras redimensionadas
	score float64
}

// ucLayout reúne o que a disposição precisa saber do diagrama.
type ucLayout struct {
	d      *model.UMLDiagram
	m      UMLMetrics
	groups []ucGroup
	actors []int // ordem da primeira aparição nos casos de uso ordenados
	loose  []int // notas (e tipos inesperados): rodapé

	ucActors, actorUCs, ucLinks, actorLinks map[int][]int
}

func layoutUseCase(d *model.UMLDiagram, m UMLMetrics) {
	// Casos de uso com nome longo ganham largura para caber em duas linhas.
	for i := range d.Elements {
		e := &d.Elements[i]
		if e.Type != "usecase" || e.Width > 0 {
			continue
		}
		w, _ := m.Size(*e)
		text := m.TextWidth(e.Name, 12)
		if text > 2*(w-28)*0.85 {
			e.Width = math.Min(320, math.Ceil((text/2/0.85+28)/10)*10)
		}
	}

	u := newUCLayout(d, m)
	maxCols, connected := 1, 0
	for _, g := range u.groups {
		maxCols = max(maxCols, min(len(g.ucs), 3))
	}
	for _, a := range u.actors {
		if len(u.actorUCs[a]) > 0 {
			connected++
		}
	}
	var best *ucPlan
	for cols := 1; cols <= maxCols; cols++ {
		for _, split := range []bool{false, true} {
			if split && connected < 2 {
				continue
			}
			if p := u.plan(cols, split); best == nil || p.score > best.score+0.01 {
				best = p
			}
		}
	}
	for i, pos := range best.pos {
		d.Elements[i].Position = pos
	}
	for i, s := range best.size {
		d.Elements[i].Width, d.Elements[i].Height = math.Ceil(s[0]), math.Ceil(s[1])
	}
}

func newUCLayout(d *model.UMLDiagram, m UMLMetrics) *ucLayout {
	u := &ucLayout{
		d: d, m: m,
		ucActors: map[int][]int{}, actorUCs: map[int][]int{},
		ucLinks: map[int][]int{}, actorLinks: map[int][]int{},
	}
	idx := map[string]int{}
	for i, e := range d.Elements {
		idx[e.ID] = i
	}
	var actors, boundaries []int
	groupOf := map[int][]int{} // fronteira (-1 = nenhuma) → casos de uso
	for _, i := range inputOrder(d, m) {
		e := d.Elements[i]
		switch e.Type {
		case "actor":
			actors = append(actors, i)
		case "boundary":
			boundaries = append(boundaries, i)
		case "usecase":
			g := -1
			if p, ok := idx[e.ParentID]; ok && d.Elements[p].Type == "boundary" {
				g = p
			}
			groupOf[g] = append(groupOf[g], i)
		default:
			u.loose = append(u.loose, i)
		}
	}

	for _, r := range d.Relations {
		a, aok := idx[r.Source]
		b, bok := idx[r.Target]
		if !aok || !bok || a == b || r.Type == "note_link" {
			continue
		}
		ta, tb := d.Elements[a].Type, d.Elements[b].Type
		switch {
		case ta == "actor" && tb == "usecase":
			u.ucActors[b] = appendUnique(u.ucActors[b], a)
			u.actorUCs[a] = appendUnique(u.actorUCs[a], b)
		case ta == "usecase" && tb == "actor":
			u.ucActors[a] = appendUnique(u.ucActors[a], b)
			u.actorUCs[b] = appendUnique(u.actorUCs[b], a)
		case ta == "usecase" && tb == "usecase":
			u.ucLinks[a] = appendUnique(u.ucLinks[a], b)
			u.ucLinks[b] = appendUnique(u.ucLinks[b], a)
		case ta == "actor" && tb == "actor":
			u.actorLinks[a] = appendUnique(u.actorLinks[a], b)
			u.actorLinks[b] = appendUnique(u.actorLinks[b], a)
		}
	}

	for _, b := range boundaries {
		u.groups = append(u.groups, ucGroup{b, orderUseCases(d, groupOf[b], u.ucActors, u.ucLinks)})
	}
	if free := groupOf[-1]; len(free) > 0 {
		u.groups = append(u.groups, ucGroup{-1, orderUseCases(d, free, u.ucActors, u.ucLinks)})
	}

	// Atores na ordem em que aparecem nos casos de uso; os sem caso de uso depois.
	for _, g := range u.groups {
		for _, uc := range g.ucs {
			for _, a := range u.ucActors[uc] {
				u.actors = appendUnique(u.actors, a)
			}
		}
	}
	for _, a := range actors {
		u.actors = appendUnique(u.actors, a)
	}
	return u
}

func appendUnique(list []int, v int) []int {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

// plan calcula uma disposição candidata e a sua nota.
func (u *ucLayout) plan(cols int, split bool) *ucPlan {
	side := map[int]int{} // ator → 0 (esquerda) ou 1 (direita)
	if split && cols > 1 {
		side = u.splitActors(cols)
	}
	columns := make([][][]int, len(u.groups))
	for gi, g := range u.groups {
		columns[gi] = u.columns(g.ucs, cols, split, side)
	}

	var p *ucPlan
	var actorMid map[int]float64
	for iter := 0; iter < 4; iter++ {
		p, actorMid = u.place(columns, side)
		if iter == 0 && split && cols == 1 {
			// Coluna única: atores alternam de lado pela altura, o que dobra o
			// espaço vertical de cada um.
			order := append([]int(nil), u.actors...)
			sort.SliceStable(order, func(a, b int) bool { return actorMid[order[a]] < actorMid[order[b]] })
			for k, a := range order {
				side[a] = k % 2
			}
			continue
		}
		// Cada coluna segue a altura dos seus atores (baricentro).
		centers := map[int]float64{}
		for _, g := range u.groups {
			for _, uc := range g.ucs {
				_, h := u.m.Size(u.d.Elements[uc])
				centers[uc] = p.pos[uc].Y + h/2
			}
		}
		for gi := range columns {
			for ci := range columns[gi] {
				col := columns[gi][ci]
				key := map[int]float64{}
				for _, uc := range col {
					key[uc] = centers[uc]
					if as := u.ucActors[uc]; len(as) > 0 {
						sum := 0.0
						for _, a := range as {
							sum += actorMid[a]
						}
						key[uc] = sum / float64(len(as))
					}
				}
				sort.SliceStable(col, func(a, b int) bool { return key[col[a]] < key[col[b]] })
			}
		}
	}
	u.placeNotes(p)
	return p
}

// splitActors reparte os atores entre esquerda e direita minimizando a altura
// das colunas, os casos de uso com atores dos dois lados e o desequilíbrio de
// atores. Até ucBruteForceMax atores, testa todas as divisões (o primeiro
// ator fica sempre à esquerda); acima disso, melhora uma divisão alternada.
func (u *ucLayout) splitActors(cols int) map[int]int {
	connected := []int{}
	for _, a := range u.actors {
		if len(u.actorUCs[a]) > 0 {
			connected = append(connected, a)
		}
	}
	cost := func(side map[int]int) float64 {
		total := 0.0
		for _, g := range u.groups {
			tallest := 0
			for _, col := range u.columns(g.ucs, cols, true, side) {
				tallest = max(tallest, len(col))
			}
			total += float64(tallest)
		}
		mixed := 0
		for _, g := range u.groups {
			for _, uc := range g.ucs {
				if u.ucClass(uc, side) == 'M' {
					mixed++
				}
			}
		}
		left := 0
		for _, a := range connected {
			if side[a] == 0 {
				left++
			}
		}
		return total + 0.8*float64(mixed) + 0.3*math.Abs(float64(2*left-len(connected)))
	}
	assign := func(mask int) map[int]int {
		side := map[int]int{}
		for k, a := range connected {
			side[a] = (mask >> k) & 1
		}
		return side
	}

	var best map[int]int
	if len(connected) <= ucBruteForceMax {
		bestCost := math.Inf(1)
		for mask := 0; mask < 1<<len(connected); mask += 2 {
			side := assign(mask)
			if c := cost(side); c < bestCost-1e-9 {
				best, bestCost = side, c
			}
		}
	} else {
		mask := 0
		for k := range connected {
			mask |= (k % 2) << k
		}
		best = assign(mask)
		bestCost := cost(best)
		for improved := true; improved; {
			improved = false
			for k := 1; k < len(connected); k++ {
				try := map[int]int{}
				for a, s := range best {
					try[a] = s
				}
				try[connected[k]] ^= 1
				if c := cost(try); c < bestCost-1e-9 {
					best, bestCost, improved = try, c, true
				}
			}
		}
	}
	// Atores sem caso de uso ficam do lado de um ator relacionado, ou à esquerda.
	for _, a := range u.actors {
		if _, ok := best[a]; ok {
			continue
		}
		best[a] = 0
		for _, b := range u.actorLinks[a] {
			if s, ok := best[b]; ok {
				best[a] = s
				break
			}
		}
	}
	return best
}

// ucClass classifica um caso de uso pelos lados dos seus atores: 'L', 'R',
// 'M' (dos dois lados) ou 'N' (sem ator).
func (u *ucLayout) ucClass(uc int, side map[int]int) byte {
	left, right := false, false
	for _, a := range u.ucActors[uc] {
		if side[a] == 1 {
			right = true
		} else {
			left = true
		}
	}
	switch {
	case left && right:
		return 'M'
	case left:
		return 'L'
	case right:
		return 'R'
	}
	return 'N'
}

// columns distribui os casos de uso (já ordenados) nas colunas da grade.
func (u *ucLayout) columns(ucs []int, cols int, split bool, side map[int]int) [][]int {
	if len(ucs) == 0 {
		return nil
	}
	if cols == 1 {
		return [][]int{append([]int(nil), ucs...)}
	}
	out := [][]int{}
	if !split {
		rows := (len(ucs) + cols - 1) / cols
		for start := 0; start < len(ucs); start += rows {
			out = append(out, append([]int(nil), ucs[start:min(start+rows, len(ucs))]...))
		}
		return out
	}

	var left, mid, right []int
	shorter := func() *[]int {
		if len(right) < len(left) {
			return &right
		}
		return &left
	}
	for _, uc := range ucs {
		switch u.ucClass(uc, side) {
		case 'L':
			left = append(left, uc)
		case 'R':
			right = append(right, uc)
		case 'M':
			if cols >= 3 {
				mid = append(mid, uc)
				continue
			}
			nl, nr := 0, 0
			for _, a := range u.ucActors[uc] {
				if side[a] == 1 {
					nr++
				} else {
					nl++
				}
			}
			switch {
			case nl > nr:
				left = append(left, uc)
			case nr > nl:
				right = append(right, uc)
			default:
				*shorter() = append(*shorter(), uc)
			}
		default:
			if cols >= 3 {
				mid = append(mid, uc)
			} else {
				*shorter() = append(*shorter(), uc)
			}
		}
	}
	for _, col := range [][]int{left, mid, right} {
		if len(col) > 0 {
			out = append(out, col)
		}
	}
	return out
}

// place monta a geometria de uma distribuição em colunas: grades centradas
// verticalmente, fronteiras empilhadas e atores nas laterais. Devolve também
// a altura do centro da figura de cada ator.
func (u *ucLayout) place(columns [][][]int, side map[int]int) (*ucPlan, map[int]float64) {
	d, m := u.d, u.m
	p := &ucPlan{pos: map[int]model.Position{}, size: map[int][2]float64{}}

	type grid struct {
		w, h    float64
		centers map[int][2]float64
		inner   [2]float64 // deslocamento do conteúdo dentro da fronteira
	}
	grids := make([]grid, len(u.groups))
	areaW := 0.0
	for gi, g := range u.groups {
		gr := grid{centers: map[int][2]float64{}}
		if cols := columns[gi]; len(cols) > 0 {
			cellW, cellH := 0.0, 0.0
			for _, uc := range g.ucs {
				w, h := m.Size(d.Elements[uc])
				cellW, cellH = math.Max(cellW, w), math.Max(cellH, h)
			}
			rows := 0
			for _, col := range cols {
				rows = max(rows, len(col))
			}
			for c, col := range cols {
				offset := float64(rows-len(col)) * (cellH + ucRowGap) / 2
				for k, uc := range col {
					gr.centers[uc] = [2]float64{
						float64(c)*(cellW+ucColGap) + cellW/2,
						offset + float64(k)*(cellH+ucRowGap) + cellH/2,
					}
				}
			}
			gr.w = float64(len(cols))*cellW + float64(len(cols)-1)*ucColGap
			gr.h = float64(rows)*cellH + float64(rows-1)*ucRowGap
		}
		if g.boundary >= 0 {
			pad, minW := containerPad(d.Elements[g.boundary], m)
			if len(g.ucs) == 0 {
				gr.w, gr.h = m.Size(d.Elements[g.boundary])
			} else {
				content := gr.w
				gr.w = math.Max(content+pad[1]+pad[3], minW)
				gr.inner = [2]float64{pad[3] + (gr.w-content-pad[1]-pad[3])/2, pad[0]}
				gr.h += pad[0] + pad[2]
			}
		}
		grids[gi] = gr
		areaW = math.Max(areaW, gr.w)
	}

	// Grupos empilhados e centralizados.
	centers := map[int][2]float64{}
	y := 0.0
	for gi, g := range u.groups {
		gr := grids[gi]
		x := (areaW - gr.w) / 2
		if g.boundary >= 0 {
			p.pos[g.boundary] = model.Position{X: x, Y: y}
			if len(g.ucs) > 0 {
				p.size[g.boundary] = [2]float64{gr.w, gr.h}
			}
		}
		for uc, c := range gr.centers {
			cx, cy := x+gr.inner[0]+c[0], y+gr.inner[1]+c[1]
			centers[uc] = [2]float64{cx, cy}
			w, h := m.Size(d.Elements[uc])
			p.pos[uc] = model.Position{X: cx - w/2, Y: cy - h/2}
		}
		y += gr.h + ucGroupGap
	}
	areaH := math.Max(0, y-ucGroupGap)

	// Atores: a figura na altura média dos seus casos de uso.
	desired := map[int]float64{}
	for _, a := range u.actors {
		if ucs := u.actorUCs[a]; len(ucs) > 0 {
			sum := 0.0
			for _, uc := range ucs {
				sum += centers[uc][1]
			}
			desired[a] = sum/float64(len(ucs)) - ucFigureMid
		}
	}
	for _, a := range u.actors {
		if _, ok := desired[a]; ok {
			continue
		}
		desired[a] = areaH // sem caso de uso: embaixo, ou junto a um ator relacionado
		for _, b := range u.actorLinks[a] {
			if y, ok := desired[b]; ok {
				desired[a] = y + 1
				break
			}
		}
	}
	mid := map[int]float64{}
	for s := 0; s < 2; s++ {
		slots := []int{}
		for _, a := range u.actors {
			if side[a] == s {
				slots = append(slots, a)
			}
		}
		if len(slots) == 0 {
			continue
		}
		sort.SliceStable(slots, func(i, j int) bool { return desired[slots[i]] < desired[slots[j]] })
		colW := 80.0
		want := make([]float64, len(slots))
		seps := make([]float64, len(slots))
		for k, a := range slots {
			e := d.Elements[a]
			w, _ := m.Size(e)
			colW = math.Max(colW, math.Max(w, m.TextWidth(e.Name, 12)+8))
			want[k] = desired[a]
			if k > 0 {
				_, prevH := m.Size(d.Elements[slots[k-1]])
				seps[k] = math.Max(prevH, 100) + ucActorGap
			}
		}
		ys := pack1D(want, seps)
		cx := -ucSideGap - colW/2
		if s == 1 {
			cx = areaW + ucSideGap + colW/2
		}
		for k, a := range slots {
			w, _ := m.Size(d.Elements[a])
			p.pos[a] = model.Position{X: cx - w/2, Y: ys[k]}
			mid[a] = ys[k] + ucFigureMid
		}
	}
	return p, mid
}

// bounds devolve a caixa ocupada pelo plano, com os nomes dos atores.
func (u *ucLayout) bounds(p *ucPlan) (minX, minY, maxX, maxY float64) {
	minX, minY = math.Inf(1), math.Inf(1)
	maxX, maxY = math.Inf(-1), math.Inf(-1)
	for i, pos := range p.pos {
		e := u.d.Elements[i]
		w, h := u.m.Size(e)
		if s, ok := p.size[i]; ok {
			w, h = s[0], s[1]
		}
		left, right := pos.X, pos.X+w
		if e.Type == "actor" {
			name := u.m.TextWidth(e.Name, 12)
			left = math.Min(left, pos.X+w/2-name/2)
			right = math.Max(right, pos.X+w/2+name/2)
		}
		minX, maxX = math.Min(minX, left), math.Max(maxX, right)
		minY, maxY = math.Min(minY, pos.Y), math.Max(maxY, pos.Y+h)
	}
	return
}

// placeNotes põe as notas no rodapé, na ordem horizontal do que anotam, e
// calcula a nota final do plano.
func (u *ucLayout) placeNotes(p *ucPlan) {
	d, m := u.d, u.m
	left, _, right, bottom := u.bounds(p)
	if len(u.loose) > 0 {
		placed := map[string]float64{}
		for i, pos := range p.pos {
			placed[d.Elements[i].ID] = pos.X
		}
		anchorX := map[int]float64{}
		for _, n := range u.loose {
			id := d.Elements[n].ID
			anchorX[n] = d.Elements[n].Position.X
			for _, r := range d.Relations {
				other := r.Target
				if r.Target == id {
					other = r.Source
				} else if r.Source != id {
					continue
				}
				if x, ok := placed[other]; ok {
					anchorX[n] = x
					break
				}
			}
		}
		notes := append([]int(nil), u.loose...)
		sort.SliceStable(notes, func(a, b int) bool { return anchorX[notes[a]] < anchorX[notes[b]] })
		limit := math.Max(right-left, 560)
		x, y, rowH := left, bottom+ucGroupGap, 0.0
		for _, n := range notes {
			w, h := m.Size(d.Elements[n])
			if x > left && x+w > left+limit {
				x, y = left, y+rowH+ucNoteGap
				rowH = 0
			}
			p.pos[n] = model.Position{X: x, Y: y}
			x += w + ucNoteGap
			rowH = math.Max(rowH, h)
		}
	}
	minX, minY, maxX, maxY := u.bounds(p)
	p.score = fitScore(maxX-minX, maxY-minY, umlSVGMargin) - ucCrossPenalty*float64(u.crossings(p))
}

// crossings conta as linhas (ator–caso de uso e entre casos de uso) que
// atravessam outra elipse ou outro ator.
func (u *ucLayout) crossings(p *ucPlan) int {
	d, m := u.d, u.m
	type shape struct {
		cx, cy, rx, ry float64
		ellipse        bool
	}
	shapes := map[int]shape{}
	anchor := map[int][2]float64{}
	byID := map[string]int{}
	for i, pos := range p.pos {
		byID[d.Elements[i].ID] = i
		e := d.Elements[i]
		w, h := m.Size(e)
		switch e.Type {
		case "usecase":
			shapes[i] = shape{pos.X + w/2, pos.Y + h/2, w/2 - 3, h/2 - 3, true}
			anchor[i] = [2]float64{pos.X + w/2, pos.Y + h/2}
		case "actor":
			shapes[i] = shape{pos.X + w/2, pos.Y + ucFigureMid, 20, ucFigureMid, false}
			anchor[i] = [2]float64{pos.X + w/2, pos.Y + ucFigureMid}
		}
	}
	count := 0
	for _, r := range d.Relations {
		a, aok := byID[r.Source]
		b, bok := byID[r.Target]
		_, aShape := anchor[a]
		_, bShape := anchor[b]
		if !aok || !bok || !aShape || !bShape || a == b {
			continue
		}
		x1, y1, x2, y2 := anchor[a][0], anchor[a][1], anchor[b][0], anchor[b][1]
		for i, s := range shapes {
			if i == a || i == b {
				continue
			}
			if s.ellipse {
				if segmentHitsEllipse(x1, y1, x2, y2, s.cx, s.cy, s.rx, s.ry) {
					count++
				}
			} else if segmentHitsRect(x1, y1, x2, y2, s.cx-s.rx, s.cy-s.ry, 2*s.rx, 2*s.ry) {
				count++
			}
		}
	}
	return count
}

// segmentHitsEllipse informa se o segmento cruza a elipse (em coordenadas em
// que ela vira o círculo unitário, é a distância do centro ao segmento).
func segmentHitsEllipse(x1, y1, x2, y2, cx, cy, rx, ry float64) bool {
	if rx <= 0 || ry <= 0 {
		return false
	}
	ax, ay := (x1-cx)/rx, (y1-cy)/ry
	bx, by := (x2-cx)/rx, (y2-cy)/ry
	dx, dy := bx-ax, by-ay
	t := 0.0
	if l := dx*dx + dy*dy; l > 0 {
		t = math.Max(0, math.Min(1, -(ax*dx+ay*dy)/l))
	}
	px, py := ax+t*dx, ay+t*dy
	return px*px+py*py < 1
}

// orderUseCases ordena pelo código (CDU001, CDU002…) e agrupa os casos de uso
// com o mesmo conjunto de atores, para que as linhas de cada ator fiquem
// juntas. Casos sem ator (incluídos/estendidos) seguem o caso relacionado.
func orderUseCases(d *model.UMLDiagram, ucs []int, ucActors, ucLinks map[int][]int) []int {
	code := func(i int) string {
		if c := strings.TrimSpace(d.Elements[i].UseCase); c != "" {
			return c
		}
		return d.Elements[i].Name
	}
	sorted := append([]int(nil), ucs...)
	sort.SliceStable(sorted, func(a, b int) bool { return naturalLess(code(sorted[a]), code(sorted[b])) })

	rank := map[int]int{}
	for k, u := range sorted {
		rank[u] = k
	}
	signature := func(u int) string {
		ids := []string{}
		for _, a := range ucActors[u] {
			ids = append(ids, d.Elements[a].ID)
		}
		sort.Strings(ids)
		return strings.Join(ids, "\x00")
	}
	sigRank := map[string]int{}
	for _, u := range sorted {
		if len(ucActors[u]) > 0 {
			if _, ok := sigRank[signature(u)]; !ok {
				sigRank[signature(u)] = len(sigRank)
			}
		}
	}
	type key struct{ group, anchor, own int }
	keys := map[int]key{}
	for _, u := range sorted {
		if len(ucActors[u]) > 0 {
			keys[u] = key{sigRank[signature(u)], rank[u], -1}
			continue
		}
		anchor := -1
		for _, v := range ucLinks[u] {
			if _, in := rank[v]; in && len(ucActors[v]) > 0 && (anchor < 0 || rank[v] < rank[anchor]) {
				anchor = v
			}
		}
		if anchor >= 0 {
			keys[u] = key{sigRank[signature(anchor)], rank[anchor], rank[u]}
		} else {
			keys[u] = key{len(sigRank), rank[u], -1}
		}
	}
	sort.SliceStable(sorted, func(a, b int) bool {
		ka, kb := keys[sorted[a]], keys[sorted[b]]
		if ka.group != kb.group {
			return ka.group < kb.group
		}
		if ka.anchor != kb.anchor {
			return ka.anchor < kb.anchor
		}
		return ka.own < kb.own
	})
	return sorted
}

// naturalLess compara textos tratando sequências de dígitos como números
// ("CDU2" < "CDU10") e ignorando maiúsculas.
func naturalLess(a, b string) bool {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			si, sj := i, j
			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}
			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}
			na := strings.TrimLeft(string(ra[si:i]), "0")
			nb := strings.TrimLeft(string(rb[sj:j]), "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if ra[i] != rb[j] {
			return ra[i] < rb[j]
		}
		i++
		j++
	}
	return len(ra)-i < len(rb)-j
}
