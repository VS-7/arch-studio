package proposal

import (
	"strings"
	"testing"
	"time"

	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/pricing"
)

func TestRenderPreencheCabecalhoEPadroes(t *testing.T) {
	d := &model.Diagram{Nodes: []model.Node{{ID: "node-api", Type: "compute", Data: model.NodeData{Label: "Core API"}}}}
	pc := &model.PricingConfig{}
	est := pricing.Calculate(d, nil, pc, -1)

	md := Render(Input{
		Manifest: &model.Manifest{ProjectName: "Loja", Version: "2.0"},
		Diagram:  d, Estimate: est, Date: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
	}, Options{IncludeDiagram: true})

	for _, want := range []string{
		"# Proposta Técnica e Comercial — Loja", "**Cliente:** Cliente", "**Data:** 23/09/2026",
		"**Validade da proposta:** 15 dias", "```mermaid", "| Core API | compute |", "## 5. Investimento",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("proposta sem %q", want)
		}
	}
	if strings.Contains(md, "## 8. Observações") {
		t.Error("seção de observações não deveria aparecer sem notas")
	}
}
