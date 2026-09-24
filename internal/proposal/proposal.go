// Package proposal redige a proposta técnica e comercial (RF014) a partir do
// escopo modelado e da estimativa. É uma função pura: quem grava o arquivo e
// anuncia a mudança é o pacote app.
package proposal

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/pricing"
)

type Options struct {
	ClientName     string  `json:"client_name,omitempty"`
	ValidityDays   int     `json:"validity_days,omitempty"`
	Margin         float64 `json:"margin,omitempty"`
	IncludeDiagram bool    `json:"include_diagram"`
	IncludeCloud   bool    `json:"include_cloud"`
	Notes          string  `json:"notes,omitempty"`
}

// Input é o escopo do projeto já estimado.
type Input struct {
	Manifest     *model.Manifest
	Diagram      *model.Diagram
	Requirements *model.RequirementsDoc
	UseCases     []model.UseCase
	Estimate     *pricing.Estimate
	Date         time.Time
}

// Render devolve o Markdown de docs/proposta-comercial.md.
func Render(in Input, opts Options) string {
	if opts.ValidityDays <= 0 {
		opts.ValidityDays = 15
	}
	client := opts.ClientName
	if client == "" {
		client = "Cliente"
	}
	cur := in.Estimate.Currency

	var b strings.Builder
	fmt.Fprintf(&b, "# Proposta Técnica e Comercial — %s\n\n", in.Manifest.ProjectName)
	fmt.Fprintf(&b, "**Cliente:** %s  \n", client)
	fmt.Fprintf(&b, "**Data:** %s  \n", in.Date.Format("02/01/2006"))
	fmt.Fprintf(&b, "**Validade da proposta:** %d dias  \n", opts.ValidityDays)
	fmt.Fprintf(&b, "**Versão do escopo:** %s\n\n", in.Manifest.Version)
	b.WriteString("---\n\n")

	b.WriteString("## 1. Escopo do Projeto\n\n")
	if in.Manifest.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", in.Manifest.Description)
	}
	if in.Requirements != nil && in.Requirements.Overview != "" {
		fmt.Fprintf(&b, "%s\n\n", in.Requirements.Overview)
	}

	if opts.IncludeDiagram {
		b.WriteString("### Arquitetura Proposta\n\n```mermaid\n")
		b.WriteString(strings.TrimRight(mermaid.Export(in.Diagram), "\n"))
		b.WriteString("\n```\n\n")
	}

	b.WriteString("## 2. Componentes Entregues\n\n")
	b.WriteString("| Componente | Tipo | Tecnologia | Complexidade | Esforço |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | ---: |\n")
	nodeHours := map[string]float64{}
	for _, item := range in.Estimate.Items {
		if item.Kind == "node" {
			nodeHours[item.ID] = item.Hours
		}
	}
	nodes := append([]model.Node(nil), in.Diagram.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool { return nodeHours[nodes[i].ID] > nodeHours[nodes[j].ID] })
	for _, n := range nodes {
		if n.Type == "group" {
			continue
		}
		complexity := "medium"
		if n.Data.Pricing != nil && n.Data.Pricing.Complexity != "" {
			complexity = n.Data.Pricing.Complexity
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n",
			n.Data.Label, n.Type, orDashStr(n.Data.Technology), complexity, pricing.Hours(nodeHours[n.ID]))
	}
	b.WriteString("\n")

	if len(in.UseCases) > 0 {
		b.WriteString("## 3. Casos de Uso Contemplados\n\n")
		for _, uc := range in.UseCases {
			fmt.Fprintf(&b, "- **%s — %s**", uc.Code, uc.Name)
			if len(uc.Actors) > 0 {
				fmt.Fprintf(&b, " (atores: %s)", strings.Join(uc.Actors, ", "))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("## 4. Dimensionamento de Esforço\n\n")
	b.WriteString("| Origem do esforço | Horas |\n| :--- | ---: |\n")
	fmt.Fprintf(&b, "| Componentes de arquitetura | %s |\n", pricing.Hours(in.Estimate.NodeHours))
	fmt.Fprintf(&b, "| Integrações e contratos de API | %s |\n", pricing.Hours(in.Estimate.EdgeHours))
	fmt.Fprintf(&b, "| Casos de uso e regras de negócio | %s |\n", pricing.Hours(in.Estimate.UseCaseHours))
	fmt.Fprintf(&b, "| **Subtotal** | **%s** |\n", pricing.Hours(in.Estimate.BaseHours))
	fmt.Fprintf(&b, "| Margem de contingência (%.0f%%) | %s |\n", in.Estimate.RiskMarginPercentage, pricing.Hours(in.Estimate.MarginHours))
	fmt.Fprintf(&b, "| **Total** | **%s** |\n\n", pricing.Hours(in.Estimate.TotalHours))

	b.WriteString("## 5. Investimento\n\n")
	b.WriteString("| Perfil | Horas | Valor/hora | Subtotal |\n| :--- | ---: | ---: | ---: |\n")
	for _, r := range in.Estimate.Roles {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			roleLabel(r.Role), pricing.Hours(r.Hours), pricing.Money(cur, r.Rate), pricing.Money(cur, r.Subtotal))
	}
	fmt.Fprintf(&b, "| **Subtotal de desenvolvimento** | | | **%s** |\n", pricing.Money(cur, in.Estimate.PersonnelCost))
	fmt.Fprintf(&b, "| Impostos (%.0f%%) | | | %s |\n", in.Estimate.TaxPercentage, pricing.Money(cur, in.Estimate.TaxAmount))
	fmt.Fprintf(&b, "| **TOTAL DO PROJETO** | | | **%s** |\n\n", pricing.Money(cur, in.Estimate.TotalCost))

	if opts.IncludeCloud && in.Estimate.CloudMonthlyCost > 0 {
		b.WriteString("## 6. Custo Mensal de Infraestrutura (estimado)\n\n")
		b.WriteString("| Componente | Tier | Custo mensal |\n| :--- | :--- | ---: |\n")
		for _, c := range in.Estimate.CloudItems {
			fmt.Fprintf(&b, "| %s | `%s` | %s |\n", c.Label, orDashStr(c.Tier), pricing.Money(cur, c.MonthlyCost))
		}
		fmt.Fprintf(&b, "| **Total mensal** | | **%s** |\n", pricing.Money(cur, in.Estimate.CloudMonthlyCost))
		fmt.Fprintf(&b, "| Total anual | | %s |\n\n", pricing.Money(cur, in.Estimate.CloudYearlyCost))
		b.WriteString("> Custos de infraestrutura são repassados pelo provedor de nuvem e não estão inclusos no valor do projeto.\n\n")
	}

	b.WriteString("## 7. Prazo Estimado\n\n")
	fmt.Fprintf(&b, "- Equipe considerada: **%.0f pessoa(s)** a %.0f h/dia produtivas\n", in.Estimate.TeamSize, in.Estimate.HoursPerDay)
	fmt.Fprintf(&b, "- Duração estimada: **%.0f dias úteis** (~%.1f semanas / %.1f meses)\n",
		in.Estimate.WorkingDays, in.Estimate.CalendarWeeks, in.Estimate.CalendarMonths)
	if in.Estimate.EstimatedFinish != "" {
		fmt.Fprintf(&b, "- Entrega projetada (se iniciar hoje): **%s**\n", in.Estimate.EstimatedFinish)
	}
	b.WriteString("\n")

	if opts.Notes != "" {
		fmt.Fprintf(&b, "## 8. Observações\n\n%s\n\n", opts.Notes)
	}

	b.WriteString("---\n\n")
	b.WriteString("> Estimativa gerada automaticamente pelo ArchCode Studio a partir do escopo modelado.\n")
	b.WriteString("> Alterações no escopo visual recalculam automaticamente esforço e investimento.\n")

	return b.String()
}

func roleLabel(role string) string {
	labels := map[string]string{
		"tech_lead":       "Tech Lead",
		"senior_engineer": "Engenheiro Sênior",
		"pleno_engineer":  "Engenheiro Pleno",
		"junior_engineer": "Engenheiro Júnior",
		"cloud_architect": "Arquiteto de Nuvem",
		"designer":        "Designer de Produto",
		"qa_engineer":     "Engenheiro de QA",
	}
	if l, ok := labels[role]; ok {
		return l
	}
	return strings.ReplaceAll(role, "_", " ")
}

func orDashStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
