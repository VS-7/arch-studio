// Package pricing implementa o motor de dimensionamento de esforço e custo
// (RF011–RF014). A fórmula base é:
//
//	Horas Totais = (Σ Horas(Nós) + Σ Horas(Arestas) + Σ Horas(Casos de Uso)) × (1 + Margem)
//
// Cada parcela pode ser sobrescrita manualmente no nó/aresta/caso de uso; na
// ausência de valor explícito, aplica-se o catálogo de .arch/pricing.yaml.
package pricing

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/model"
)

type LineItem struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Kind       string  `json:"kind"` // node | edge | use_case
	Category   string  `json:"category"`
	Complexity string  `json:"complexity,omitempty"`
	Hours      float64 `json:"hours"`
	Explicit   bool    `json:"explicit"`
}

type RoleCost struct {
	Role     string  `json:"role"`
	Share    float64 `json:"share"`
	Hours    float64 `json:"hours"`
	Rate     float64 `json:"rate"`
	Subtotal float64 `json:"subtotal"`
}

type CloudItem struct {
	NodeID      string  `json:"node_id"`
	Label       string  `json:"label"`
	Tier        string  `json:"cloud_tier"`
	MonthlyCost float64 `json:"monthly_cost"`
}

type Estimate struct {
	Currency string `json:"currency"`

	NodeHours    float64 `json:"node_hours"`
	EdgeHours    float64 `json:"edge_hours"`
	UseCaseHours float64 `json:"use_case_hours"`
	BaseHours    float64 `json:"base_hours"`

	RiskMarginPercentage float64 `json:"risk_margin_percentage"`
	MarginHours          float64 `json:"margin_hours"`
	TotalHours           float64 `json:"total_hours"`

	Roles         []RoleCost `json:"roles"`
	PersonnelCost float64    `json:"personnel_cost"`
	TaxPercentage float64    `json:"tax_percentage"`
	TaxAmount     float64    `json:"tax_amount"`
	TotalCost     float64    `json:"total_cost"`
	BlendedRate   float64    `json:"blended_rate"`

	CloudMonthlyCost float64     `json:"cloud_monthly_cost"`
	CloudYearlyCost  float64     `json:"cloud_yearly_cost"`
	CloudItems       []CloudItem `json:"cloud_items"`

	TeamSize        float64 `json:"team_size"`
	HoursPerDay     float64 `json:"hours_per_day"`
	WorkingDays     float64 `json:"working_days"`
	CalendarWeeks   float64 `json:"calendar_weeks"`
	CalendarMonths  float64 `json:"calendar_months"`
	EstimatedFinish string  `json:"estimated_finish"`

	Items     []LineItem         `json:"items"`
	ByTier    map[string]float64 `json:"by_tier"`
	ByType    map[string]float64 `json:"by_type"`
	Generated string             `json:"generated_at"`
	Warnings  []string           `json:"warnings,omitempty"`
}

func factor(cfg *model.PricingConfig, complexity string) float64 {
	c := model.NormalizeComplexity(complexity)
	if c == "" {
		c = "medium"
	}
	if f, ok := cfg.ComplexityFactors[c]; ok && f > 0 {
		return f
	}
	return 1
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

// Calculate produz a estimativa completa do projeto.
// marginOverride < 0 significa "usar a margem configurada em pricing.yaml".
func Calculate(d *model.Diagram, useCases []model.UseCase, cfg *model.PricingConfig, marginOverride float64) *Estimate {
	cfg.Normalize()

	est := &Estimate{
		Currency:      cfg.Currency,
		TaxPercentage: cfg.TaxPercentage,
		TeamSize:      cfg.TeamSize,
		HoursPerDay:   cfg.HoursPerDay,
		// Listas sempre inicializadas: o JSON sai como [] (nunca null), que é
		// o que a UI e os clientes MCP esperam iterar.
		Roles:      []RoleCost{},
		CloudItems: []CloudItem{},
		Items:      []LineItem{},
		ByTier:     map[string]float64{},
		ByType:     map[string]float64{},
		Generated:  time.Now().UTC().Format(time.RFC3339),
	}

	margin := cfg.RiskMarginPercentage
	if marginOverride >= 0 {
		// Aceita tanto 0.20 quanto 20 como entrada.
		if marginOverride <= 1 {
			margin = marginOverride * 100
		} else {
			margin = marginOverride
		}
	}
	est.RiskMarginPercentage = margin

	// --- Nós -----------------------------------------------------------------
	for _, n := range d.Nodes {
		if n.Type == "group" {
			continue
		}
		complexity := "medium"
		hours := 0.0
		explicit := false
		if n.Data.Pricing != nil {
			if c := model.NormalizeComplexity(n.Data.Pricing.Complexity); c != "" {
				complexity = c
			}
			if n.Data.Pricing.EstimatedHours > 0 {
				hours = n.Data.Pricing.EstimatedHours
				explicit = true
			}
		}
		if !explicit {
			base, ok := cfg.NodeTypeHours[n.Type]
			if !ok {
				base = 16
				est.Warnings = append(est.Warnings,
					fmt.Sprintf("tipo de nó '%s' sem horas base em pricing.yaml; assumido 16h", n.Type))
			}
			hours = base * factor(cfg, complexity)
		}
		hours = round2(hours)
		est.NodeHours += hours
		tier := n.Data.Tier
		if tier == "" {
			tier = "backend"
		}
		est.ByTier[tier] += hours
		est.ByType[n.Type] += hours
		est.Items = append(est.Items, LineItem{
			ID: n.ID, Label: n.Data.Label, Kind: "node", Category: n.Type,
			Complexity: complexity, Hours: hours, Explicit: explicit,
		})
	}

	// --- Arestas / integrações ----------------------------------------------
	for _, e := range d.Edges {
		hours := 0.0
		explicit := false
		if e.Data.EstimatedHours > 0 {
			hours = e.Data.EstimatedHours
			explicit = true
		}
		protocol := strings.ToLower(strings.TrimSpace(e.Data.Protocol))
		if protocol == "" {
			protocol = "rest"
		}
		key := strings.Fields(strings.ReplaceAll(protocol, "/", " "))[0]
		if !explicit {
			base, ok := cfg.ProtocolHours[key]
			if !ok {
				base = 6
			}
			hours = base * factor(cfg, e.Data.Complexity)
		}
		hours = round2(hours)
		est.EdgeHours += hours
		est.ByTier["integration"] += hours
		label := e.Label
		if label == "" {
			src, dst := e.Source, e.Target
			if n := d.NodeByID(e.Source); n != nil {
				src = n.Data.Label
			}
			if n := d.NodeByID(e.Target); n != nil {
				dst = n.Data.Label
			}
			label = src + " → " + dst
		}
		est.Items = append(est.Items, LineItem{
			ID: e.ID, Label: label, Kind: "edge", Category: strings.ToUpper(key),
			Complexity: model.NormalizeComplexity(e.Data.Complexity), Hours: hours, Explicit: explicit,
		})
	}

	// --- Casos de uso --------------------------------------------------------
	for _, uc := range useCases {
		complexity := model.NormalizeComplexity(uc.Complexity)
		if complexity == "" {
			complexity = "medium"
		}
		hours := uc.EstimatedHours
		explicit := hours > 0
		if !explicit {
			base, ok := cfg.UseCaseHours[complexity]
			if !ok {
				base = 16
			}
			hours = base
		}
		hours = round2(hours)
		est.UseCaseHours += hours
		est.ByTier["domain"] += hours
		est.Items = append(est.Items, LineItem{
			ID: uc.Code, Label: uc.Name, Kind: "use_case", Category: "Caso de Uso",
			Complexity: complexity, Hours: hours, Explicit: explicit,
		})
	}

	est.BaseHours = round2(est.NodeHours + est.EdgeHours + est.UseCaseHours)
	est.MarginHours = round2(est.BaseHours * margin / 100)
	est.TotalHours = round2(est.BaseHours + est.MarginHours)

	// --- Distribuição por perfil profissional --------------------------------
	roles := make([]string, 0, len(cfg.RoleDistribution))
	totalShare := 0.0
	for role, share := range cfg.RoleDistribution {
		roles = append(roles, role)
		totalShare += share
	}
	sort.Strings(roles)
	if totalShare <= 0 {
		totalShare = 1
	}
	for _, role := range roles {
		share := cfg.RoleDistribution[role] / totalShare
		rate := cfg.HourlyRates[role]
		if rate == 0 {
			est.Warnings = append(est.Warnings,
				fmt.Sprintf("perfil '%s' sem taxa horária em pricing.yaml", role))
		}
		hours := round2(est.TotalHours * share)
		subtotal := round2(hours * rate)
		est.Roles = append(est.Roles, RoleCost{
			Role: role, Share: round2(share), Hours: hours, Rate: rate, Subtotal: subtotal,
		})
		est.PersonnelCost += subtotal
	}
	est.PersonnelCost = round2(est.PersonnelCost)
	est.TaxAmount = round2(est.PersonnelCost * cfg.TaxPercentage / 100)
	est.TotalCost = round2(est.PersonnelCost + est.TaxAmount)
	if est.TotalHours > 0 {
		est.BlendedRate = round2(est.PersonnelCost / est.TotalHours)
	}

	// --- Infraestrutura (Cloud TCO) ------------------------------------------
	for _, n := range d.Nodes {
		if n.Data.Pricing == nil {
			continue
		}
		cost := n.Data.Pricing.MonthlyCost
		tier := n.Data.Pricing.CloudTier
		if cost == 0 && tier != "" {
			if c, ok := cfg.CloudCatalog[tier]; ok {
				cost = c
			} else {
				est.Warnings = append(est.Warnings,
					fmt.Sprintf("cloud_tier '%s' (nó %s) não está no catálogo de pricing.yaml", tier, n.Data.Label))
			}
		}
		if cost <= 0 {
			continue
		}
		est.CloudItems = append(est.CloudItems, CloudItem{
			NodeID: n.ID, Label: n.Data.Label, Tier: tier, MonthlyCost: round2(cost),
		})
		est.CloudMonthlyCost += cost
	}
	est.CloudMonthlyCost = round2(est.CloudMonthlyCost)
	est.CloudYearlyCost = round2(est.CloudMonthlyCost * 12)

	// --- Cronograma ----------------------------------------------------------
	capacity := cfg.TeamSize * cfg.HoursPerDay
	if capacity > 0 {
		est.WorkingDays = round2(est.TotalHours / capacity)
		est.CalendarWeeks = round2(est.WorkingDays / 5)
		est.CalendarMonths = round2(est.WorkingDays / cfg.WorkingDaysPerMonth)
		calendarDays := int(math.Ceil(est.WorkingDays / 5 * 7))
		est.EstimatedFinish = time.Now().AddDate(0, 0, calendarDays).Format("2006-01-02")
	}

	sort.SliceStable(est.Items, func(i, j int) bool { return est.Items[i].Hours > est.Items[j].Hours })
	for k, v := range est.ByTier {
		est.ByTier[k] = round2(v)
	}
	for k, v := range est.ByType {
		est.ByType[k] = round2(v)
	}
	return est
}

var currencySymbols = map[string]string{"BRL": "R$", "USD": "US$", "EUR": "€", "GBP": "£"}

// Money formata um valor monetário no padrão brasileiro (1.234,56).
func Money(currency string, v float64) string {
	sym, ok := currencySymbols[strings.ToUpper(currency)]
	if !ok {
		sym = currency + " "
	}
	neg := v < 0
	if neg {
		v = -v
	}
	s := fmt.Sprintf("%.2f", v)
	parts := strings.SplitN(s, ".", 2)
	intPart, frac := parts[0], parts[1]
	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteRune('.')
		}
		b.WriteRune(r)
	}
	out := fmt.Sprintf("%s %s,%s", sym, b.String(), frac)
	if neg {
		return "-" + out
	}
	return out
}

func Hours(h float64) string {
	if h == math.Trunc(h) {
		return fmt.Sprintf("%.0fh", h)
	}
	return fmt.Sprintf("%.1fh", h)
}
