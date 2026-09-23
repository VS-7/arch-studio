package svgexport

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Diagramas UML (casos de uso, classes, sequência e estados)
// ---------------------------------------------------------------------------
//
// Renderização fiel ao canvas da interface (notação StarUML): mesmas formas,
// mesmos tamanhos padrão e a mesma geometria de sequência. Serve às figuras do
// documento de requisitos e à exportação via CLI, sem depender do browser.
// A medição de texto é aproximada (largura média por caractere), simples e
// determinística: a mesma entrada sempre produz o mesmo SVG.

// UMLOptions controla o tema do SVG dos diagramas UML.
type UMLOptions struct {
	Dark        bool
	Transparent bool
}

type umlPalette struct {
	bg, fill, stroke, text, muted, halo string
}

func umlPaletteFor(dark bool) umlPalette {
	if dark {
		return umlPalette{bg: "#1e1f22", fill: "#2b2d31", stroke: "#e6e6e6", text: "#f2f2f2", muted: "#a9adb5", halo: "#1e1f22"}
	}
	return umlPalette{bg: "#ffffff", fill: "#ffffff", stroke: "#000000", text: "#000000", muted: "#555555", halo: "#ffffff"}
}

// umlSizes são os tamanhos padrão do canvas (usados quando width/height = 0).
var umlSizes = map[string][2]float64{
	"actor": {80, 100}, "usecase": {160, 70}, "boundary": {420, 400}, "note": {180, 80},
	"class": {200, 120}, "interface": {200, 120}, "enum": {200, 120}, "package": {240, 160},
	"lifeline": {140, 44}, "fragment": {420, 180},
	"state": {160, 70}, "initial": {24, 24}, "final": {28, 28}, "choice": {36, 36},
	"fork": {120, 8}, "join": {120, 8}, "history": {32, 32},
}

// Geometria do diagrama de sequência (a mesma de web/src/lib/umlMeta.ts).
const (
	seqTop      = 40.0
	seqHeader   = 44.0
	seqFirstGap = 36.0
	seqStep     = 44.0
	seqTail     = 48.0
)

const (
	fontSize   = 12.0
	charWidth  = 6.8 // largura média de um caractere a 12px
	lineHeight = 17.0
	margin     = 30.0
	foldSize   = 12.0
)

// textWidth estima a largura do texto na fonte de `size` px.
func textWidth(s string, size float64) float64 {
	return float64(len([]rune(s))) * charWidth * size / fontSize
}

func isClassLike(t string) bool { return t == "class" || t == "interface" || t == "enum" }

// memberText formata um membro como no canvas (visibilidade padrão "+").
func memberText(m model.UMLMember, operation bool) string {
	vis := m.Visibility
	if vis == "" {
		vis = "+"
	}
	if operation {
		s := fmt.Sprintf("%s %s(%s)", vis, m.Name, m.Params)
		if m.Type != "" {
			s += ": " + m.Type
		}
		return s
	}
	s := vis + " " + m.Name
	if m.Type != "" {
		s += ": " + m.Type
	}
	if m.Default != "" {
		s += " = " + m.Default
	}
	return s
}

func classStereotype(e model.UMLElement) string {
	switch e.Type {
	case "interface":
		return "interface"
	case "enum":
		return "enumeration"
	}
	return e.Stereotype
}

// compartment devolve a altura de um compartimento com n linhas.
func compartment(n int) float64 {
	if n == 0 {
		return 10
	}
	return float64(n)*lineHeight + 6
}

// classSize calcula o tamanho de classe/interface/enum a partir do conteúdo
// quando width/height não foram definidos.
func classSize(e model.UMLElement) (float64, float64) {
	header := 28.0
	if classStereotype(e) != "" {
		header = 40
	}
	first := len(e.Attributes)
	if e.Type == "enum" {
		first = len(e.Literals)
	}
	autoH := header + compartment(first) + compartment(len(e.Operations))

	longest := textWidth(e.Name, fontSize) * 1.08 // negrito
	if st := classStereotype(e); st != "" {
		longest = math.Max(longest, textWidth("«"+st+"»", 11))
	}
	for _, m := range e.Attributes {
		longest = math.Max(longest, textWidth(memberText(m, false), 11))
	}
	for _, m := range e.Operations {
		longest = math.Max(longest, textWidth(memberText(m, true), 11))
	}
	for _, l := range e.Literals {
		longest = math.Max(longest, textWidth(l, 11))
	}
	autoW := math.Max(160, math.Ceil(longest+24))

	w, h := e.Width, e.Height
	if w <= 0 {
		w = autoW
	}
	if h <= 0 {
		h = autoH
	}
	return w, h
}

// elementSize devolve largura e altura efetivas no SVG.
func elementSize(e model.UMLElement) (float64, float64) {
	if isClassLike(e.Type) {
		return classSize(e)
	}
	def, ok := umlSizes[e.Type]
	if !ok {
		def = [2]float64{160, 70}
	}
	w, h := e.Width, e.Height
	if w <= 0 {
		w = def[0]
	}
	if h <= 0 {
		h = def[1]
	}
	return w, h
}

// shapeKind define como a borda é calculada para ancorar relações.
type shapeKind int

const (
	shapeRect shapeKind = iota
	shapeEllipse
	shapeDiamond
)

type umlBox struct {
	x, y, w, h float64
	kind       shapeKind
}

func (b umlBox) cx() float64 { return b.x + b.w/2 }
func (b umlBox) cy() float64 { return b.y + b.h/2 }

// border devolve o ponto da borda na direção de (tx, ty), pela interseção da
// reta centro→alvo com o retângulo, a elipse ou o losango.
func (b umlBox) border(tx, ty float64) (float64, float64) {
	cx, cy := b.cx(), b.cy()
	dx, dy := tx-cx, ty-cy
	if dx == 0 && dy == 0 {
		return cx, b.y
	}
	rx, ry := math.Max(b.w/2, 0.5), math.Max(b.h/2, 0.5)
	var s float64
	switch b.kind {
	case shapeEllipse:
		s = 1 / math.Sqrt((dx*dx)/(rx*rx)+(dy*dy)/(ry*ry))
	case shapeDiamond:
		s = 1 / (math.Abs(dx)/rx + math.Abs(dy)/ry)
	default:
		sx, sy := math.Inf(1), math.Inf(1)
		if dx != 0 {
			sx = rx / math.Abs(dx)
		}
		if dy != 0 {
			sy = ry / math.Abs(dy)
		}
		s = math.Min(sx, sy)
	}
	return cx + dx*s, cy + dy*s
}

// anchorBox é a caixa usada para ancorar relações: o ator usa a figura 40x58
// centralizada no topo; círculos e elipses usam a elipse inscrita.
func anchorBox(e model.UMLElement) umlBox {
	w, h := elementSize(e)
	x, y := e.Position.X, e.Position.Y
	switch e.Type {
	case "actor":
		return umlBox{x + (w-40)/2, y, 40, 58, shapeRect}
	case "usecase", "initial", "final", "history":
		return umlBox{x, y, w, h, shapeEllipse}
	case "choice":
		return umlBox{x, y, w, h, shapeDiamond}
	}
	return umlBox{x, y, w, h, shapeRect}
}

// ---------------------------------------------------------------------------
// Escrita
// ---------------------------------------------------------------------------

type umlCanvas struct {
	b                      strings.Builder
	pal                    umlPalette
	minX, minY, maxX, maxY float64
}

func newUMLCanvas(pal umlPalette) *umlCanvas {
	return &umlCanvas{pal: pal, minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
}

// extend inclui o ponto na caixa delimitadora usada no viewBox.
func (c *umlCanvas) extend(x, y float64) {
	c.minX, c.minY = math.Min(c.minX, x), math.Min(c.minY, y)
	c.maxX, c.maxY = math.Max(c.maxX, x), math.Max(c.maxY, y)
}

func (c *umlCanvas) extendRect(x, y, w, h float64) {
	c.extend(x, y)
	c.extend(x+w, y+h)
}

func (c *umlCanvas) printf(format string, args ...any) {
	fmt.Fprintf(&c.b, format, args...)
	c.b.WriteByte('\n')
}

// text escreve um texto. anchor: start | middle | end. extra: atributos adicionais.
func (c *umlCanvas) text(x, y float64, s, anchor string, size float64, extra string) {
	if s == "" {
		return
	}
	w := textWidth(s, size)
	switch anchor {
	case "middle":
		c.extendRect(x-w/2, y-size, w, size+4)
	case "end":
		c.extendRect(x-w, y-size, w, size+4)
	default:
		c.extendRect(x, y-size, w, size+4)
	}
	sizeAttr := ""
	if size != fontSize {
		sizeAttr = fmt.Sprintf(` font-size="%s"`, num(size))
	}
	c.printf(`<text x="%s" y="%s" text-anchor="%s"%s fill="%s"%s>%s</text>`,
		num(x), num(y), anchor, sizeAttr, c.pal.text, extra, esc(s))
}

// label escreve o texto de uma relação com um halo do fundo, para não ser
// cortado pelas linhas que cruzam.
func (c *umlCanvas) label(x, y float64, s, anchor string) {
	c.text(x, y, s, anchor, 11, fmt.Sprintf(` stroke="%s" stroke-width="3" stroke-linejoin="round" paint-order="stroke"`, c.pal.halo))
}

// num formata coordenadas com no máximo uma casa decimal.
func num(f float64) string {
	s := fmt.Sprintf("%.1f", f)
	s = strings.TrimSuffix(s, ".0")
	if s == "-0" {
		return "0"
	}
	return s
}

// wrapWidth quebra o texto para caber em `width` px.
func wrapWidth(s string, width float64, maxLines int) []string {
	chars := int(width / charWidth)
	if chars < 4 {
		chars = 4
	}
	return wrap(s, chars, maxLines)
}

// ---------------------------------------------------------------------------
// RenderUML
// ---------------------------------------------------------------------------

// RenderUML produz o SVG autocontido de um diagrama UML.
func RenderUML(d *model.UMLDiagram, opts UMLOptions) []byte {
	pal := umlPaletteFor(opts.Dark)
	c := newUMLCanvas(pal)

	if d.Kind == model.UMLKindSequence {
		renderSequence(c, d)
	} else {
		renderFloating(c, d)
	}

	if math.IsInf(c.minX, 1) {
		c.text(0, 0, "Diagrama vazio", "middle", 14, ` fill-opacity="0.6"`)
	}
	x, y := c.minX-margin, c.minY-margin
	w, h := c.maxX-c.minX+2*margin, c.maxY-c.minY+2*margin

	var out strings.Builder
	fmt.Fprintf(&out, `<svg xmlns="http://www.w3.org/2000/svg" width="%s" height="%s" viewBox="%s %s %s %s" font-family="Inter, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="12">`,
		num(w), num(h), num(x), num(y), num(w), num(h))
	out.WriteString("\n")
	fmt.Fprintf(&out, "<title>%s</title>\n", esc(d.Name))
	out.WriteString(umlDefs(pal))
	if !opts.Transparent {
		fmt.Fprintf(&out, `<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`+"\n", num(x), num(y), num(w), num(h), pal.bg)
	}
	out.WriteString(c.b.String())
	out.WriteString("</svg>\n")
	return []byte(out.String())
}

// umlDefs declara as pontas de seta. Todas apontam para a ponta TARGET
// (marker-end) e usam coordenadas do usuário, independentes da espessura.
func umlDefs(p umlPalette) string {
	var b strings.Builder
	b.WriteString("<defs>\n")
	fmt.Fprintf(&b, `<marker id="m-arrow" viewBox="0 0 12 12" refX="11" refY="6" markerWidth="12" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto"><path d="M1 1 L11 6 L1 11" fill="none" stroke="%s" stroke-width="1.3"/></marker>`+"\n", p.stroke)
	fmt.Fprintf(&b, `<marker id="m-arrow-filled" viewBox="0 0 12 12" refX="11" refY="6" markerWidth="11" markerHeight="11" markerUnits="userSpaceOnUse" orient="auto"><path d="M1 1 L11 6 L1 11 z" fill="%s" stroke="%s" stroke-width="1"/></marker>`+"\n", p.stroke, p.stroke)
	fmt.Fprintf(&b, `<marker id="m-triangle" viewBox="0 0 16 16" refX="15" refY="8" markerWidth="16" markerHeight="16" markerUnits="userSpaceOnUse" orient="auto"><path d="M1 1 L15 8 L1 15 z" fill="%s" stroke="%s" stroke-width="1.3"/></marker>`+"\n", p.fill, p.stroke)
	fmt.Fprintf(&b, `<marker id="m-diamond" viewBox="0 0 20 12" refX="19" refY="6" markerWidth="20" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto"><path d="M1 6 L10 1 L19 6 L10 11 z" fill="%s" stroke="%s" stroke-width="1.3"/></marker>`+"\n", p.fill, p.stroke)
	fmt.Fprintf(&b, `<marker id="m-diamond-filled" viewBox="0 0 20 12" refX="19" refY="6" markerWidth="20" markerHeight="12" markerUnits="userSpaceOnUse" orient="auto"><path d="M1 6 L10 1 L19 6 L10 11 z" fill="%s" stroke="%s" stroke-width="1.3"/></marker>`+"\n", p.stroke, p.stroke)
	b.WriteString("</defs>\n")
	return b.String()
}

// relationStyle devolve o tracejado e a ponta de cada tipo de relação.
func relationStyle(typ string) (dashed bool, marker string) {
	switch typ {
	case "include", "extend", "dependency":
		return true, "m-arrow"
	case "realization":
		return true, "m-triangle"
	case "note_link":
		return true, ""
	case "generalization":
		return false, "m-triangle"
	case "aggregation":
		return false, "m-diamond"
	case "composition":
		return false, "m-diamond-filled"
	case "directed_association", "transition":
		return false, "m-arrow"
	}
	return false, ""
}

// ---------------------------------------------------------------------------
// Casos de uso, classes e estados (relações "flutuantes")
// ---------------------------------------------------------------------------

// depthOf devolve a profundidade de contenção (parent_id) do elemento.
func depthOf(d *model.UMLDiagram, e model.UMLElement) int {
	depth := 0
	for id := e.ParentID; id != "" && depth < 64; depth++ {
		p := d.ElementByID(id)
		if p == nil {
			break
		}
		id = p.ParentID
	}
	return depth
}

var containerTypes = map[string]bool{"boundary": true, "package": true, "fragment": true}

// drawOrder ordena os elementos: contêineres e ancestrais primeiro, para que
// os filhos fiquem por cima; empate mantém a ordem do arquivo.
func drawOrder(d *model.UMLDiagram) []model.UMLElement {
	els := append([]model.UMLElement(nil), d.Elements...)
	hasChildren := map[string]bool{}
	for _, e := range els {
		if e.ParentID != "" {
			hasChildren[e.ParentID] = true
		}
	}
	rank := func(e model.UMLElement) int {
		r := depthOf(d, e) * 2
		if !containerTypes[e.Type] && !hasChildren[e.ID] {
			r++
		}
		return r
	}
	sort.SliceStable(els, func(i, j int) bool { return rank(els[i]) < rank(els[j]) })
	return els
}

func renderFloating(c *umlCanvas, d *model.UMLDiagram) {
	composite := map[string]bool{}
	for _, e := range d.Elements {
		if e.ParentID != "" {
			composite[e.ParentID] = true
		}
	}
	for _, e := range drawOrder(d) {
		drawElement(c, e, composite[e.ID])
	}
	for _, r := range d.Relations {
		src, dst := d.ElementByID(r.Source), d.ElementByID(r.Target)
		if src == nil || dst == nil {
			continue
		}
		drawRelation(c, d.Kind, r, *src, *dst)
	}
}

func drawElement(c *umlCanvas, e model.UMLElement, composite bool) {
	p := c.pal
	w, h := elementSize(e)
	x, y := e.Position.X, e.Position.Y
	c.printf(`<g data-id="%s" data-type="%s">`, esc(e.ID), esc(e.Type))
	defer c.printf(`</g>`)

	switch e.Type {
	case "actor":
		fx, fy := x+(w-40)/2, y
		c.extendRect(fx, fy, 40, 58)
		c.printf(`<g fill="none" stroke="%s" stroke-width="1.4" stroke-linecap="round" transform="translate(%s %s)"><circle cx="20" cy="8" r="7" fill="%s"/><path d="M20 15v22M5 24h30M20 37L8 56M20 37l12 19"/></g>`,
			p.stroke, num(fx), num(fy), p.fill)
		c.text(x+w/2, fy+58+14, e.Name, "middle", fontSize, "")

	case "usecase":
		c.extendRect(x, y, w, h)
		c.printf(`<ellipse cx="%s" cy="%s" rx="%s" ry="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x+w/2), num(y+h/2), num(w/2), num(h/2), p.fill, p.stroke)
		lines := wrapWidth(e.Name, w-28, 3)
		total := float64(len(lines)) * 15
		if e.UseCase != "" {
			total += 13
		}
		ty := y + h/2 - total/2 + 11
		if e.UseCase != "" {
			c.text(x+w/2, ty, strings.ToUpper(e.UseCase), "middle", 10, ` fill-opacity="0.7"`)
			ty += 13
		}
		for _, l := range lines {
			c.text(x+w/2, ty, l, "middle", fontSize, "")
			ty += 15
		}

	case "boundary":
		c.extendRect(x, y, w, h)
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x), num(y), num(w), num(h), p.fill, p.stroke)
		c.text(x+w/2, y+20, e.Name, "middle", fontSize, ` font-weight="700"`)

	case "note":
		c.extendRect(x, y, w, h)
		c.printf(`<path d="M%s %s H%s L%s %s V%s H%s Z" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x), num(y), num(x+w-foldSize), num(x+w), num(y+foldSize), num(y+h), num(x), p.fill, p.stroke)
		c.printf(`<path d="M%s %s V%s H%s" fill="none" stroke="%s" stroke-width="1.3"/>`,
			num(x+w-foldSize), num(y), num(y+foldSize), num(x+w), p.stroke)
		ty := y + 18
		if e.Name != "" {
			for _, l := range wrapWidth(e.Name, w-26, 2) {
				c.text(x+10, ty, l, "start", fontSize, ` font-weight="600"`)
				ty += 15
			}
		}
		for _, para := range strings.Split(e.Documentation, "\n") {
			for _, l := range wrapWidth(para, w-26, 6) {
				if ty > y+h-4 {
					break
				}
				c.text(x+10, ty, l, "start", fontSize, "")
				ty += 15
			}
		}

	case "class", "interface", "enum":
		drawClass(c, e, x, y, w, h)

	case "package":
		c.extendRect(x, y, w, h)
		tabW := math.Min(math.Max(60, textWidth(e.Name, 11.5)*1.08+20), w*0.7)
		c.printf(`<rect x="%s" y="%s" width="%s" height="22" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x), num(y), num(tabW), p.fill, p.stroke)
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x), num(y+22), num(w), num(math.Max(h-22, 10)), p.fill, p.stroke)
		c.text(x+10, y+15, e.Name, "start", 11.5, ` font-weight="600"`)
		if e.Stereotype != "" {
			c.text(x+10, y+40, "«"+e.Stereotype+"»", "start", 11, "")
		}

	case "state":
		c.extendRect(x, y, w, h)
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" rx="12" ry="12" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x), num(y), num(w), num(h), p.fill, p.stroke)
		activities := []string{}
		for _, a := range [][2]string{{"entry", e.Entry}, {"do", e.Do}, {"exit", e.Exit}} {
			if strings.TrimSpace(a[1]) != "" {
				activities = append(activities, a[0]+" / "+a[1])
			}
		}
		head := 0.0
		if e.Stereotype != "" {
			head = 13
		}
		if len(activities) == 0 && !composite {
			ty := y + h/2 + 4 - head/2
			if e.Stereotype != "" {
				c.text(x+w/2, ty, "«"+e.Stereotype+"»", "middle", 11, "")
				ty += 13
			}
			c.text(x+w/2, ty, e.Name, "middle", fontSize, ` font-weight="600"`)
			break
		}
		ty := y + 19
		if e.Stereotype != "" {
			c.text(x+w/2, ty, "«"+e.Stereotype+"»", "middle", 11, "")
			ty += 13
		}
		c.text(x+w/2, ty, e.Name, "middle", fontSize, ` font-weight="600"`)
		sep := ty + 9
		c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.3"/>`, num(x), num(sep), num(x+w), num(sep), p.stroke)
		ay := sep + 15
		for _, a := range activities {
			c.text(x+10, ay, a, "start", 11, "")
			ay += 16.5
		}
		// Estado composto com atividades: outra divisória separa a região dos subestados.
		if composite && len(activities) > 0 {
			line := ay - 10
			c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.3"/>`, num(x), num(line), num(x+w), num(line), p.stroke)
		}

	case "initial":
		c.extendRect(x, y, w, h)
		c.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, num(x+w/2), num(y+h/2), num(math.Min(w, h)/2), p.stroke)

	case "final":
		c.extendRect(x, y, w, h)
		r := math.Min(w, h) / 2
		c.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`, num(x+w/2), num(y+h/2), num(r-0.65), p.fill, p.stroke)
		c.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s"/>`, num(x+w/2), num(y+h/2), num(math.Max(r-5, 2)), p.stroke)

	case "choice":
		c.extendRect(x, y, w, h)
		c.printf(`<path d="M%s %s L%s %s L%s %s L%s %s Z" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x+w/2), num(y), num(x+w), num(y+h/2), num(x+w/2), num(y+h), num(x), num(y+h/2), p.fill, p.stroke)

	case "fork", "join":
		c.extendRect(x, y, w, h)
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s"/>`, num(x), num(y), num(w), num(h), p.stroke)

	case "history":
		c.extendRect(x, y, w, h)
		c.printf(`<circle cx="%s" cy="%s" r="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`, num(x+w/2), num(y+h/2), num(math.Min(w, h)/2-0.65), p.fill, p.stroke)
		c.text(x+w/2, y+h/2+4.5, "H", "middle", 13, ` font-weight="700"`)

	default:
		c.extendRect(x, y, w, h)
		c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`,
			num(x), num(y), num(w), num(h), p.fill, p.stroke)
		c.text(x+w/2, y+h/2+4, e.Name, "middle", fontSize, "")
	}
}

// drawClass desenha classe/interface/enum em três compartimentos.
func drawClass(c *umlCanvas, e model.UMLElement, x, y, w, h float64) {
	p := c.pal
	c.extendRect(x, y, w, h)
	c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`,
		num(x), num(y), num(w), num(h), p.fill, p.stroke)

	header := 28.0
	ty := y + 19
	if st := classStereotype(e); st != "" {
		header = 40
		c.text(x+w/2, y+15, "«"+st+"»", "middle", 11, "")
		ty = y + 31
	}
	nameAttrs := ` font-weight="700"`
	if e.Abstract {
		nameAttrs += ` font-style="italic"`
	}
	c.text(x+w/2, ty, e.Name, "middle", fontSize, nameAttrs)

	sep := func(yy float64) {
		if yy < y+h {
			c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.3"/>`, num(x), num(yy), num(x+w), num(yy), p.stroke)
		}
	}
	row := func(yy float64, s string, extra string) {
		if yy <= y+h-3 {
			c.text(x+8, yy, s, "start", 11, extra)
		}
	}

	top := y + header
	sep(top)
	first := []string{}
	firstStatic := []bool{}
	if e.Type == "enum" {
		first = append(first, e.Literals...)
		firstStatic = make([]bool, len(first))
	} else {
		for _, m := range e.Attributes {
			first = append(first, memberText(m, false))
			firstStatic = append(firstStatic, m.Static)
		}
	}
	for i, s := range first {
		extra := ""
		if firstStatic[i] {
			extra = ` text-decoration="underline"`
		}
		row(top+3+float64(i+1)*lineHeight-4, s, extra)
	}
	top += compartment(len(first))
	sep(top)
	for i, m := range e.Operations {
		extra := ""
		if m.Static {
			extra += ` text-decoration="underline"`
		}
		if m.Abstract || e.Type == "interface" {
			extra += ` font-style="italic"`
		}
		row(top+3+float64(i+1)*lineHeight-4, memberText(m, true), extra)
	}
}

// relationLabel devolve o rótulo central de uma relação.
func relationLabel(r model.UMLRelation) []string {
	switch r.Type {
	case "include", "extend":
		out := []string{}
		if r.Name != "" {
			out = append(out, r.Name)
		}
		return append(out, "«"+r.Type+"»")
	case "transition":
		trigger := r.Trigger
		if trigger == "" {
			trigger = r.Name
		}
		parts := []string{}
		if trigger != "" {
			parts = append(parts, trigger)
		}
		if r.Guard != "" {
			parts = append(parts, "["+r.Guard+"]")
		}
		s := strings.Join(parts, " ")
		if r.Effect != "" {
			s = strings.TrimSpace(s + " / " + r.Effect)
		}
		if s == "" {
			return nil
		}
		return []string{s}
	}
	if r.Name != "" {
		return []string{r.Name}
	}
	return nil
}

func drawRelation(c *umlCanvas, kind string, r model.UMLRelation, src, dst model.UMLElement) {
	p := c.pal
	dashed, marker := relationStyle(r.Type)
	attrs := fmt.Sprintf(`fill="none" stroke="%s" stroke-width="1.3"`, p.stroke)
	if dashed {
		attrs += ` stroke-dasharray="6 4"`
	}
	if marker != "" {
		attrs += fmt.Sprintf(` marker-end="url(#%s)"`, marker)
	}
	labels := relationLabel(r)
	c.printf(`<g data-id="%s" data-type="%s">`, esc(r.ID), esc(r.Type))
	defer c.printf(`</g>`)

	// Auto-relação: laço no canto superior direito.
	if src.ID == dst.ID {
		b := anchorBox(src)
		sx, sy := b.x+b.w-20, b.y
		ex, ey := b.x+b.w, b.y+20
		c.extend(sx, sy-34)
		c.extend(ex+34, ey)
		c.printf(`<path d="M%s %s C%s %s, %s %s, %s %s" %s/>`,
			num(sx), num(sy), num(sx), num(sy-34), num(ex+34), num(ey), num(ex), num(ey), attrs)
		ly := sy - 30
		for i := len(labels) - 1; i >= 0; i-- {
			c.label(ex+8, ly, labels[i], "start")
			ly -= 13
		}
		return
	}

	sb, db := anchorBox(src), anchorBox(dst)
	x1, y1 := sb.border(db.cx(), db.cy())
	x2, y2 := db.border(sb.cx(), sb.cy())
	c.extend(x1, y1)
	c.extend(x2, y2)
	c.printf(`<path d="M%s %s L%s %s" %s/>`, num(x1), num(y1), num(x2), num(y2), attrs)

	// Rótulos centrais, deslocados para cima da linha.
	mx, my := (x1+x2)/2, (y1+y2)/2
	ly := my - 6
	for i := len(labels) - 1; i >= 0; i-- {
		c.label(mx, ly, labels[i], "middle")
		ly -= 13
	}

	// Multiplicidades e papéis perto das pontas, em lados opostos da linha.
	length := math.Hypot(x2-x1, y2-y1)
	if length == 0 {
		return
	}
	ux, uy := (x2-x1)/length, (y2-y1)/length
	nx, ny := -uy, ux
	end := func(px, py, dir float64, mult, role string) {
		bx, by := px+ux*dir*16, py+uy*dir*16
		if mult != "" {
			c.label(bx+nx*10, by+ny*10+4, mult, "middle")
		}
		if role != "" {
			c.label(bx-nx*10, by-ny*10+4, role, "middle")
		}
	}
	end(x1, y1, 1, r.SourceMultiplicity, r.SourceRole)
	end(x2, y2, -1, r.TargetMultiplicity, r.TargetRole)
}

// ---------------------------------------------------------------------------
// Sequência
// ---------------------------------------------------------------------------

func renderSequence(c *umlCanvas, d *model.UMLDiagram) {
	p := c.pal
	type lane struct {
		el     model.UMLElement
		cx     float64
		x, w   float64
		header float64
	}
	lanes := map[string]lane{}
	order := []string{}
	for _, e := range d.Elements {
		if e.Type != "lifeline" {
			continue
		}
		w, _ := elementSize(e)
		lanes[e.ID] = lane{el: e, x: e.Position.X, w: w, cx: e.Position.X + w/2}
		order = append(order, e.ID)
	}

	msgs := []model.UMLRelation{}
	for _, r := range d.Relations {
		if r.Type == "message" {
			msgs = append(msgs, r)
		}
	}
	// Mensagens sem ordem vão para o fim, preservando a ordem do arquivo.
	sort.SliceStable(msgs, func(i, j int) bool {
		oi, oj := msgs[i].Order, msgs[j].Order
		if oi == 0 {
			return false
		}
		if oj == 0 {
			return true
		}
		return oi < oj
	})

	// A linha de vida desce até cobrir a última mensagem e os fragmentos/notas.
	n := len(msgs)
	if n < 1 {
		n = 1
	}
	bottom := seqTop + seqHeader + seqFirstGap + float64(n)*seqStep + seqTail
	for _, e := range d.Elements {
		if e.Type == "fragment" || e.Type == "note" {
			_, h := elementSize(e)
			bottom = math.Max(bottom, e.Position.Y+h+seqTail)
		}
	}

	// Fragmentos atrás de tudo.
	for _, e := range d.Elements {
		if e.Type == "fragment" {
			drawFragment(c, e)
		}
	}

	// Linhas de vida.
	for _, id := range order {
		l := lanes[id]
		e := l.el
		c.printf(`<g data-id="%s" data-type="lifeline">`, esc(e.ID))
		kind := e.LifelineKind
		if kind == "" {
			kind = "participant"
		}
		lineTop := seqTop + seqHeader
		if kind == "participant" {
			c.extendRect(l.x, seqTop, l.w, seqHeader)
			c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="%s" stroke="%s" stroke-width="1.3"/>`,
				num(l.x), num(seqTop), num(l.w), num(seqHeader), p.fill, p.stroke)
			c.text(l.cx, seqTop+seqHeader/2+4, e.Name, "middle", 12.5, "")
		} else {
			iconH := drawLifelineIcon(c, kind, l.cx, seqTop)
			c.text(l.cx, seqTop+iconH+14, e.Name, "middle", fontSize, "")
			lineTop = math.Max(lineTop, seqTop+iconH+20)
		}
		c.extend(l.cx, bottom)
		c.printf(`<line x1="%s" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1.3" stroke-dasharray="5 4"/>`,
			num(l.cx), num(lineTop), num(l.cx), num(bottom), p.stroke)
		c.printf(`</g>`)
	}

	// Mensagens, de cima para baixo: y = 40 + 44 + 36 + i*44.
	for i, r := range msgs {
		src, sok := lanes[r.Source]
		dst, dok := lanes[r.Target]
		if !sok || !dok {
			continue
		}
		y := seqTop + seqHeader + seqFirstGap + float64(i)*seqStep
		drawMessage(c, r, i+1, src.cx, dst.cx, y)
	}

	// Notas por cima.
	for _, e := range d.Elements {
		if e.Type == "note" {
			drawElement(c, e, false)
		}
	}
}

// drawLifelineIcon desenha o ícone do cabeçalho e devolve sua altura.
func drawLifelineIcon(c *umlCanvas, kind string, cx, top float64) float64 {
	p := c.pal
	common := fmt.Sprintf(`fill="%s" stroke="%s" stroke-width="1.3"`, p.fill, p.stroke)
	switch kind {
	case "actor":
		// Boneco 24x36 (a figura 40x58 em escala 0.6).
		x := cx - 12
		c.extendRect(x, top, 24, 36)
		c.printf(`<g fill="none" stroke="%s" stroke-width="1.4" stroke-linecap="round" transform="translate(%s %s) scale(0.6)"><circle cx="20" cy="8" r="7" fill="%s"/><path d="M20 15v22M5 24h30M20 37L8 56M20 37l12 19"/></g>`,
			p.stroke, num(x), num(top), p.fill)
		return 36
	case "boundary":
		x := cx - 20
		c.extendRect(x, top, 40, 26)
		c.printf(`<g transform="translate(%s %s)"><path d="M4 3v20M4 13h8" stroke="%s" stroke-width="1.3" fill="none"/><circle cx="24" cy="13" r="11" %s/></g>`,
			num(x), num(top), p.stroke, common)
		return 26
	case "control":
		x := cx - 14
		c.extendRect(x, top, 28, 28)
		c.printf(`<g transform="translate(%s %s)"><circle cx="14" cy="15" r="11.5" %s/><path d="M11 1.5l4.5 2.5L11 6.5" fill="none" stroke="%s" stroke-width="1.3"/></g>`,
			num(x), num(top), common, p.stroke)
		return 28
	case "entity":
		x := cx - 14
		c.extendRect(x, top, 28, 28)
		c.printf(`<g transform="translate(%s %s)"><circle cx="14" cy="12.5" r="11" %s/><path d="M3 26.5h22" stroke="%s" stroke-width="1.3"/></g>`,
			num(x), num(top), common, p.stroke)
		return 28
	case "database":
		x := cx - 15
		c.extendRect(x, top, 30, 30)
		c.printf(`<g transform="translate(%s %s)"><path d="M2 6v18c0 2.5 5.8 4.5 13 4.5s13-2 13-4.5V6" %s/><ellipse cx="15" cy="6" rx="13" ry="4.5" %s/></g>`,
			num(x), num(top), common, common)
		return 30
	}
	return 0
}

func drawFragment(c *umlCanvas, e model.UMLElement) {
	p := c.pal
	w, h := elementSize(e)
	x, y := e.Position.X, e.Position.Y
	op := e.Operator
	if op == "" {
		op = "alt"
	}
	c.extendRect(x, y, w, h)
	c.printf(`<g data-id="%s" data-type="fragment">`, esc(e.ID))
	c.printf(`<rect x="%s" y="%s" width="%s" height="%s" fill="none" stroke="%s" stroke-width="1.3"/>`,
		num(x), num(y), num(w), num(h), p.stroke)
	tabW := textWidth(op, 11.5)*1.08 + 22
	tabH := 20.0
	c.printf(`<path d="M%s %s H%s V%s L%s %s H%s Z" fill="%s" stroke="%s" stroke-width="1.3"/>`,
		num(x), num(y), num(x+tabW), num(y+tabH*0.65), num(x+tabW-7), num(y+tabH), num(x), p.fill, p.stroke)
	c.text(x+8, y+14, op, "start", 11.5, ` font-weight="600"`)
	if e.Guard != "" {
		c.text(x+tabW+8, y+14, "["+e.Guard+"]", "start", 11.5, "")
	}
	if e.Name != "" {
		c.text(x+w-8, y+h-6, e.Name, "end", 10.5, ` fill-opacity="0.7"`)
	}
	c.printf(`</g>`)
}

func drawMessage(c *umlCanvas, r model.UMLRelation, n int, x1, x2, y float64) {
	p := c.pal
	kind := r.MessageKind
	if kind == "" {
		kind = "sync"
	}
	dashed, marker := false, "m-arrow-filled"
	switch kind {
	case "async":
		marker = "m-arrow"
	case "reply", "create":
		dashed, marker = true, "m-arrow"
	}
	attrs := fmt.Sprintf(`fill="none" stroke="%s" stroke-width="1.3" marker-end="url(#%s)"`, p.stroke, marker)
	if dashed {
		attrs += ` stroke-dasharray="6 4"`
	}
	text := r.Name
	switch kind {
	case "create":
		text = strings.TrimSpace("«create» " + text)
	case "destroy":
		text = strings.TrimSpace("«destroy» " + text)
	}
	label := fmt.Sprintf("%d: %s", n, text)
	if text == "" {
		label = fmt.Sprintf("%d:", n)
	}

	c.printf(`<g data-id="%s" data-type="message" data-order="%d">`, esc(r.ID), n)
	defer c.printf(`</g>`)

	if x1 == x2 {
		// Auto-mensagem: laço à direita da linha de vida.
		c.extend(x1+34, y+22)
		c.printf(`<path d="M%s %s H%s V%s H%s" %s/>`, num(x1), num(y), num(x1+34), num(y+22), num(x1+2), attrs)
		c.label(x1+40, y+4, label, "start")
		return
	}
	// Recuo de 1px da linha de vida alvo para a ponta não se sobrepor ao traço.
	dir := 1.0
	if x2 < x1 {
		dir = -1
	}
	c.extend(x1, y)
	c.extend(x2, y)
	c.printf(`<path d="M%s %s H%s" %s/>`, num(x1), num(y), num(x2-dir), attrs)
	c.label((x1+x2)/2, y-6, label, "middle")
	if kind == "destroy" {
		c.printf(`<path d="M%s %s L%s %s M%s %s L%s %s" stroke="%s" stroke-width="1.6"/>`,
			num(x2-8), num(y-8), num(x2+8), num(y+8), num(x2-8), num(y+8), num(x2+8), num(y-8), p.stroke)
	}
}
