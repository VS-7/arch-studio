package svgexport

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func macroFixture() *model.Diagram {
	return &model.Diagram{
		Nodes: []model.Node{
			{ID: "api", Type: "compute", Position: model.Position{X: 0, Y: 0},
				Data: model.NodeData{Label: "Core API", Technology: "Go 1.23", Description: "Regras de negócio.", Status: model.StatusCompleted}},
			{ID: "db", Type: "database", Position: model.Position{X: 320, Y: 0},
				Data: model.NodeData{Label: "PostgreSQL", Technology: "PostgreSQL 16", Description: "Persistência transacional do domínio."}},
		},
		Edges: []model.Edge{{ID: "e1", Source: "api", Target: "db", Data: model.EdgeData{Protocol: "SQL", Port: 5432}}},
	}
}

func TestRenderEstiloDocumento(t *testing.T) {
	svg := Render(macroFixture(), Options{Document: true})
	wellFormed(t, []byte(svg))

	for _, want := range []string{
		`«serviço»`, `«banco de dados»`, "Core API", "[Go 1.23]", "Persistência transacional do",
		`fill="#ffffff"`, "Arial", "SQL :5432", `text-anchor="middle"`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG de documento sem %q:\n%s", want, svg)
		}
	}
	for _, unwanted := range []string{"feDropShadow", "🗄", "▢", "#f7f9fc", "Inter", "COMPUTE", `<circle`} {
		if strings.Contains(svg, unwanted) {
			t.Errorf("SVG de documento não deveria conter %q", unwanted)
		}
	}
	// Margem enxuta: a figura ocupa a página, não o canvas.
	if !strings.Contains(svg, `viewBox="0 0 588 158"`) {
		t.Errorf("dimensões inesperadas: %s", svg[:120])
	}
}

func TestRenderEstiloCanvasPreservado(t *testing.T) {
	svg := Render(macroFixture(), Options{})
	wellFormed(t, []byte(svg))
	for _, want := range []string{"feDropShadow", "COMPUTE", "DATABASE", `<circle`, "#f7f9fc"} {
		if !strings.Contains(svg, want) {
			t.Errorf("SVG do canvas sem %q", want)
		}
	}
	if strings.Contains(svg, "«") {
		t.Error("o estilo do canvas não usa estereótipos")
	}
}

func TestRenderDocumentoVazio(t *testing.T) {
	svg := Render(&model.Diagram{}, Options{Document: true})
	wellFormed(t, []byte(svg))
	if !strings.Contains(svg, "Arial") || !strings.Contains(svg, "#ffffff") {
		t.Errorf("SVG vazio fora do estilo de documento: %s", svg)
	}
}
