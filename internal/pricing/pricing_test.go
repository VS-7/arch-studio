package pricing

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func diagram() *model.Diagram {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "api", Type: "compute", Data: model.NodeData{Label: "API", Tier: "backend",
			Pricing: &model.NodePricing{Complexity: "medium", CloudTier: "t4g.small"}}},
		{ID: "db", Type: "database", Data: model.NodeData{Label: "DB", Tier: "data",
			Pricing: &model.NodePricing{Complexity: "low", CloudTier: "db.t4g.micro"}}},
		{ID: "manual", Type: "compute", Data: model.NodeData{Label: "Manual", Tier: "backend",
			Pricing: &model.NodePricing{EstimatedHours: 100}}},
		{ID: "grupo", Type: "group", Data: model.NodeData{Label: "VPC"}},
	}
	d.Edges = []model.Edge{
		{ID: "e1", Source: "api", Target: "db", Data: model.EdgeData{Protocol: "SQL", Complexity: "medium"}},
	}
	return d
}

func TestFormulaDeEsforco(t *testing.T) {
	cfg := model.DefaultPricing()
	est := Calculate(diagram(), []model.UseCase{
		{Code: "CDU001", Name: "A", Complexity: "high"},
	}, cfg, -1)

	// compute médio = 32 × 1.0 = 32 ; database baixo = 12 × 0.6 = 7.2 ; manual = 100
	wantNodes := 32.0 + 7.2 + 100.0
	if math.Abs(est.NodeHours-wantNodes) > 0.01 {
		t.Errorf("horas de nós: got %v, want %v", est.NodeHours, wantNodes)
	}
	// SQL médio = 4 × 1.0
	if math.Abs(est.EdgeHours-4) > 0.01 {
		t.Errorf("horas de arestas: got %v, want 4", est.EdgeHours)
	}
	// caso de uso alto = 32
	if math.Abs(est.UseCaseHours-32) > 0.01 {
		t.Errorf("horas de casos de uso: got %v, want 32", est.UseCaseHours)
	}

	wantBase := wantNodes + 4 + 32
	if math.Abs(est.BaseHours-wantBase) > 0.01 {
		t.Errorf("subtotal: got %v, want %v", est.BaseHours, wantBase)
	}
	wantTotal := wantBase * 1.20 // margem padrão de 20%
	if math.Abs(est.TotalHours-wantTotal) > 0.05 {
		t.Errorf("total com margem: got %v, want %v", est.TotalHours, wantTotal)
	}

	// Nós de agrupamento não custam horas.
	for _, item := range est.Items {
		if item.ID == "grupo" {
			t.Error("nó de agrupamento não deveria entrar na estimativa")
		}
	}
}

func TestMargemAceitaFracaoEPercentual(t *testing.T) {
	cfg := model.DefaultPricing()
	comFracao := Calculate(diagram(), nil, cfg, 0.30)
	comPercentual := Calculate(diagram(), nil, cfg, 30)
	if comFracao.TotalHours != comPercentual.TotalHours {
		t.Errorf("0.30 e 30 deveriam ser equivalentes: %v vs %v", comFracao.TotalHours, comPercentual.TotalHours)
	}
	if comFracao.RiskMarginPercentage != 30 {
		t.Errorf("margem normalizada: got %v, want 30", comFracao.RiskMarginPercentage)
	}
}

func TestCustoTotalEImpostos(t *testing.T) {
	cfg := model.DefaultPricing()
	est := Calculate(diagram(), nil, cfg, 0)

	var soma float64
	for _, r := range est.Roles {
		soma += r.Subtotal
	}
	if math.Abs(soma-est.PersonnelCost) > 0.05 {
		t.Errorf("custo de pessoal não bate com a soma dos perfis: %v vs %v", est.PersonnelCost, soma)
	}
	wantTax := est.PersonnelCost * cfg.TaxPercentage / 100
	if math.Abs(est.TaxAmount-wantTax) > 0.05 {
		t.Errorf("impostos: got %v, want %v", est.TaxAmount, wantTax)
	}
	if math.Abs(est.TotalCost-(est.PersonnelCost+est.TaxAmount)) > 0.05 {
		t.Errorf("total: got %v", est.TotalCost)
	}

	// As horas distribuídas por perfil somam o total de horas.
	var horas float64
	for _, r := range est.Roles {
		horas += r.Hours
	}
	if math.Abs(horas-est.TotalHours) > 0.5 {
		t.Errorf("distribuição de horas: got %v, want %v", horas, est.TotalHours)
	}
}

func TestCustoDeNuvem(t *testing.T) {
	cfg := model.DefaultPricing()
	est := Calculate(diagram(), nil, cfg, -1)

	want := cfg.CloudCatalog["t4g.small"] + cfg.CloudCatalog["db.t4g.micro"]
	if math.Abs(est.CloudMonthlyCost-want) > 0.01 {
		t.Errorf("custo mensal: got %v, want %v", est.CloudMonthlyCost, want)
	}
	if math.Abs(est.CloudYearlyCost-want*12) > 0.01 {
		t.Errorf("custo anual: got %v", est.CloudYearlyCost)
	}
}

func TestTierDesconhecidoGeraAviso(t *testing.T) {
	cfg := model.DefaultPricing()
	d := diagram()
	d.Nodes[0].Data.Pricing.CloudTier = "instancia-inexistente"
	est := Calculate(d, nil, cfg, -1)
	if len(est.Warnings) == 0 {
		t.Error("tier fora do catálogo deveria gerar aviso")
	}
}

func TestMoneyFormatacaoBrasileira(t *testing.T) {
	cases := map[float64]string{
		1234.5:  "R$ 1.234,50",
		0:       "R$ 0,00",
		1000000: "R$ 1.000.000,00",
		-50.25:  "-R$ 50,25",
	}
	for value, want := range cases {
		if got := Money("BRL", value); got != want {
			t.Errorf("Money(%v) = %q, want %q", value, got, want)
		}
	}
}

// Projeto sem tier de nuvem, papéis ou componentes: as listas precisam sair
// como [] no JSON. Um `null` em cloud_items derrubava a tela de Precificação.
func TestListasVaziasNuncaSaemNull(t *testing.T) {
	cfg := model.DefaultPricing()
	cfg.RoleDistribution = map[string]float64{}
	d := model.NewDiagram()
	d.Nodes = []model.Node{{ID: "api", Type: "compute", Data: model.NodeData{Label: "API"}}}

	for _, diag := range []*model.Diagram{d, model.NewDiagram()} {
		data, err := json.Marshal(Calculate(diag, nil, cfg, -1))
		if err != nil {
			t.Fatal(err)
		}
		out := string(data)
		for _, field := range []string{"roles", "cloud_items", "items"} {
			if strings.Contains(out, `"`+field+`":null`) {
				t.Errorf("%s saiu como null: %s", field, out)
			}
		}
	}
}
