// Package svgexport renderiza o diagrama macro em SVG vetorial (RF010).
//
// A renderização acontece no servidor para que a exportação funcione também via
// CLI (`archcode-studio export svg`) e para garantir saída idêntica entre o
// browser, o app desktop e pipelines de CI.
package svgexport

import (
	"fmt"
	"html"
	"math"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
)

type Options struct {
	// Executive oculta detalhes técnicos (portas, tecnologias, nós marcados
	// como não-executivos), conforme RF009.
	Executive   bool
	Dark        bool
	Transparent bool
	Padding     float64
	Title       string
	Subtitle    string
}

type palette struct {
	bg, text, muted, edge, groupFill, groupStroke string
	nodeFill, nodeStroke                          map[string]string
}

func paletteFor(dark bool) palette {
	if dark {
		return palette{
			bg: "#0b1020", text: "#e8edf7", muted: "#9aa7c2", edge: "#5b6b90",
			groupFill: "#121a33", groupStroke: "#2b3a60",
			nodeFill: map[string]string{
				"compute": "#152a4a", "database": "#2a1a45", "cache": "#3d2a12",
				"queue": "#123a2e", "gateway": "#3d1522", "storage": "#123239",
				"client": "#1d2233", "external_service": "#332f14",
			},
			nodeStroke: map[string]string{
				"compute": "#3b82f6", "database": "#a855f7", "cache": "#f59e0b",
				"queue": "#10b981", "gateway": "#f43f5e", "storage": "#06b6d4",
				"client": "#94a3b8", "external_service": "#eab308",
			},
		}
	}
	return palette{
		bg: "#f7f9fc", text: "#0f172a", muted: "#5b6b86", edge: "#94a3b8",
		groupFill: "#eef2f9", groupStroke: "#cbd5e1",
		nodeFill: map[string]string{
			"compute": "#e6f0ff", "database": "#f5e9ff", "cache": "#fff3e0",
			"queue": "#e6fff5", "gateway": "#ffe9ee", "storage": "#e6fbff",
			"client": "#eef2f7", "external_service": "#fcfbe6",
		},
		nodeStroke: map[string]string{
			"compute": "#2563eb", "database": "#9333ea", "cache": "#d97706",
			"queue": "#059669", "gateway": "#e11d48", "storage": "#0891b2",
			"client": "#64748b", "external_service": "#ca8a04",
		},
	}
}

var typeIcon = map[string]string{
	"compute": "▢", "database": "🗄", "cache": "⚡", "queue": "✉",
	"gateway": "🛡", "storage": "📦", "client": "🖥", "external_service": "🔗",
}

func esc(s string) string { return html.EscapeString(s) }

// wrap quebra um texto em no máximo `maxLines` linhas de ~`width` caracteres.
func wrap(s string, width, maxLines int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	lines := []string{}
	cur := ""
	for _, w := range words {
		candidate := w
		if cur != "" {
			candidate = cur + " " + w
		}
		if len(candidate) > width && cur != "" {
			lines = append(lines, cur)
			cur = w
			if len(lines) == maxLines {
				break
			}
		} else {
			cur = candidate
		}
	}
	if len(lines) < maxLines && cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == maxLines && len(strings.Join(lines, " ")) < len(s) {
		last := lines[maxLines-1]
		if len(last) > width-1 {
			last = last[:width-1]
		}
		lines[maxLines-1] = last + "…"
	}
	return lines
}

type box struct {
	x, y, w, h float64
}

func (b box) cx() float64 { return b.x + b.w/2 }
func (b box) cy() float64 { return b.y + b.h/2 }

// anchor devolve o ponto de saída na borda do retângulo em direção a (tx,ty).
func (b box) anchor(tx, ty float64) (float64, float64) {
	dx, dy := tx-b.cx(), ty-b.cy()
	if dx == 0 && dy == 0 {
		return b.cx(), b.y
	}
	// Escala o vetor até tocar a borda do retângulo.
	scaleX, scaleY := math.Inf(1), math.Inf(1)
	if dx != 0 {
		scaleX = (b.w / 2) / math.Abs(dx)
	}
	if dy != 0 {
		scaleY = (b.h / 2) / math.Abs(dy)
	}
	s := math.Min(scaleX, scaleY)
	return b.cx() + dx*s, b.cy() + dy*s
}

// Render produz o SVG completo do diagrama.
func Render(d *model.Diagram, opts Options) string {
	pal := paletteFor(opts.Dark)
	if opts.Padding <= 0 {
		opts.Padding = 64
	}

	visible := []model.Node{}
	for _, n := range d.Nodes {
		if opts.Executive && !n.Data.ShowInExecutive() {
			continue
		}
		visible = append(visible, n)
	}
	if len(visible) == 0 {
		return emptySVG(pal, opts)
	}

	boxes := map[string]box{}
	for _, n := range visible {
		w, h := n.Width, n.Height
		if w <= 0 {
			w = layout.NodeWidth
		}
		if h <= 0 {
			h = layout.NodeHeight
		}
		if n.Type == "group" {
			if n.Width <= 0 {
				w = 520
			}
			if n.Height <= 0 {
				h = 360
			}
		}
		boxes[n.ID] = box{n.Position.X, n.Position.Y, w, h}
	}

	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, b := range boxes {
		minX, minY = math.Min(minX, b.x), math.Min(minY, b.y)
		maxX, maxY = math.Max(maxX, b.x+b.w), math.Max(maxY, b.y+b.h)
	}

	headerH := 0.0
	if opts.Title != "" {
		headerH = 92
	}
	offX := opts.Padding - minX
	offY := opts.Padding + headerH - minY
	width := maxX - minX + opts.Padding*2
	height := maxY - minY + opts.Padding*2 + headerH

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" font-family="Inter, ui-sans-serif, system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif">`,
		width, height, width, height)
	b.WriteString("\n<defs>\n")
	fmt.Fprintf(&b, `<marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" fill="%s"/></marker>`, pal.edge)
	b.WriteString("\n")
	fmt.Fprintf(&b, `<filter id="shadow" x="-30%%" y="-30%%" width="160%%" height="160%%"><feDropShadow dx="0" dy="2" stdDeviation="4" flood-opacity="0.18"/></filter>`)
	b.WriteString("\n</defs>\n")

	if !opts.Transparent {
		fmt.Fprintf(&b, `<rect width="100%%" height="100%%" fill="%s"/>`, pal.bg)
		b.WriteString("\n")
	}

	if opts.Title != "" {
		fmt.Fprintf(&b, `<text x="%.0f" y="48" font-size="26" font-weight="700" fill="%s">%s</text>`,
			opts.Padding, pal.text, esc(opts.Title))
		b.WriteString("\n")
		if opts.Subtitle != "" {
			fmt.Fprintf(&b, `<text x="%.0f" y="74" font-size="14" fill="%s">%s</text>`,
				opts.Padding, pal.muted, esc(opts.Subtitle))
			b.WriteString("\n")
		}
	}

	// Grupos primeiro (camada de fundo).
	for _, n := range visible {
		if n.Type != "group" {
			continue
		}
		bx := boxes[n.ID]
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="16" fill="%s" stroke="%s" stroke-width="1.5" stroke-dasharray="6 4"/>`,
			bx.x+offX, bx.y+offY, bx.w, bx.h, pal.groupFill, pal.groupStroke)
		b.WriteString("\n")
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="13" font-weight="600" fill="%s" letter-spacing="0.5">%s</text>`,
			bx.x+offX+16, bx.y+offY+24, pal.muted, esc(strings.ToUpper(n.Data.Label)))
		b.WriteString("\n")
	}

	// Arestas.
	edges := append([]model.Edge(nil), d.Edges...)
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	for _, e := range edges {
		if opts.Executive && !e.Data.ShowInExecutive() {
			continue
		}
		sb, sok := boxes[e.Source]
		tb, tok := boxes[e.Target]
		if !sok || !tok {
			continue
		}
		x1, y1 := sb.anchor(tb.cx(), tb.cy())
		x2, y2 := tb.anchor(sb.cx(), sb.cy())
		x1, y1, x2, y2 = x1+offX, y1+offY, x2+offX, y2+offY

		// Curva suave no sentido em que a aresta sai do nó: horizontal quando
		// sai pela lateral, vertical quando sai pelo topo ou pela base (fluxo
		// de cima para baixo), sempre apontando para o alvo.
		var path string
		if math.Abs(tb.cy()-sb.cy())*sb.w > math.Abs(tb.cx()-sb.cx())*sb.h {
			c := math.Copysign(math.Max(40, math.Abs(y2-y1)*0.45), y2-y1)
			path = fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f",
				x1, y1, x1, y1+c, x2, y2-c, x2, y2)
		} else {
			c := math.Copysign(math.Max(40, math.Abs(x2-x1)*0.45), x2-x1)
			path = fmt.Sprintf("M %.1f %.1f C %.1f %.1f, %.1f %.1f, %.1f %.1f",
				x1, y1, x1+c, y1, x2-c, y2, x2, y2)
		}
		dash := ""
		if strings.EqualFold(e.Data.Protocol, "webhook") || e.Animated {
			dash = ` stroke-dasharray="7 5"`
		}
		fmt.Fprintf(&b, `<path d="%s" fill="none" stroke="%s" stroke-width="1.8"%s marker-end="url(#arrow)"/>`,
			path, pal.edge, dash)
		b.WriteString("\n")

		label := e.Label
		if label == "" {
			label = e.Data.Protocol
			if !opts.Executive && e.Data.Port > 0 {
				label = fmt.Sprintf("%s :%d", label, e.Data.Port)
			}
		}
		if opts.Executive && e.Data.Description != "" {
			label = e.Data.Description
		}
		if label != "" {
			mx, my := (x1+x2)/2, (y1+y2)/2
			tw := float64(len([]rune(label)))*6.2 + 14
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="18" rx="9" fill="%s" opacity="0.92"/>`,
				mx-tw/2, my-9, tw, pal.bg)
			b.WriteString("\n")
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="11" fill="%s" text-anchor="middle">%s</text>`,
				mx, my+4, pal.muted, esc(label))
			b.WriteString("\n")
		}
	}

	// Nós.
	for _, n := range visible {
		if n.Type == "group" {
			continue
		}
		bx := boxes[n.ID]
		fill := pal.nodeFill[n.Type]
		stroke := pal.nodeStroke[n.Type]
		if fill == "" {
			fill, stroke = pal.nodeFill["compute"], pal.nodeStroke["compute"]
		}
		x, y := bx.x+offX, bx.y+offY

		fmt.Fprintf(&b, `<g filter="url(#shadow)"><rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="14" fill="%s" stroke="%s" stroke-width="2"/></g>`,
			x, y, bx.w, bx.h, fill, stroke)
		b.WriteString("\n")
		fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="4" height="%.1f" rx="2" fill="%s"/>`,
			x, y+14, bx.h-28, stroke)
		b.WriteString("\n")

		icon := typeIcon[n.Type]
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="13" fill="%s">%s</text>`,
			x+18, y+26, pal.muted, esc(icon))
		b.WriteString("\n")
		fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="10" font-weight="600" fill="%s" letter-spacing="0.8">%s</text>`,
			x+38, y+26, pal.muted, esc(strings.ToUpper(strings.ReplaceAll(n.Type, "_", " "))))
		b.WriteString("\n")

		titleLines := wrap(n.Data.Label, 22, 2)
		ty := y + 52
		for _, line := range titleLines {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="15" font-weight="700" fill="%s">%s</text>`,
				x+18, ty, pal.text, esc(line))
			b.WriteString("\n")
			ty += 18
		}

		sub := ""
		if opts.Executive {
			sub = n.Data.Description
		} else if n.Data.Technology != "" {
			sub = n.Data.Technology
		} else {
			sub = n.Data.Description
		}
		for _, line := range wrap(sub, 28, 2) {
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="11.5" fill="%s">%s</text>`,
				x+18, ty, pal.muted, esc(line))
			b.WriteString("\n")
			ty += 15
		}

		if !opts.Executive && n.Data.Status != "" && n.Data.Status != model.StatusPending {
			dotColor := map[string]string{
				model.StatusCompleted:  "#10b981",
				model.StatusInProgress: "#f59e0b",
				model.StatusBlocked:    "#ef4444",
			}[n.Data.Status]
			if dotColor != "" {
				fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="5" fill="%s"/>`, x+bx.w-18, y+22, dotColor)
				b.WriteString("\n")
			}
		}
	}

	b.WriteString("</svg>\n")
	return b.String()
}

func emptySVG(pal palette, opts Options) string {
	bg := pal.bg
	if opts.Transparent {
		bg = "none"
	}
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="640" height="240" viewBox="0 0 640 240" font-family="Inter, sans-serif"><rect width="100%%" height="100%%" fill="%s"/><text x="320" y="120" text-anchor="middle" font-size="16" fill="%s">Nenhum componente para exportar</text></svg>`, bg, pal.muted)
}
