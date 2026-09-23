package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Proposta comercial (RF014)
// ---------------------------------------------------------------------------

type ProposalOptions struct {
	ClientName     string  `json:"client_name,omitempty"`
	ValidityDays   int     `json:"validity_days,omitempty"`
	Margin         float64 `json:"margin,omitempty"`
	IncludeDiagram bool    `json:"include_diagram"`
	IncludeCloud   bool    `json:"include_cloud"`
	Notes          string  `json:"notes,omitempty"`
}

type ProposalResult struct {
	FilePath  string  `json:"file_path"`
	Markdown  string  `json:"markdown"`
	TotalCost float64 `json:"total_cost"`
	Currency  string  `json:"currency"`
}

// GenerateProposal produz docs/proposta-comercial.md a partir do escopo visual.
func (a *App) GenerateProposal(opts ProposalOptions, source string) (*ProposalResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	margin := -1.0
	if opts.Margin > 0 {
		margin = opts.Margin
	}
	est := pricing.Calculate(snap.Diagram, snap.UseCases, snap.Pricing, margin)

	if opts.ValidityDays <= 0 {
		opts.ValidityDays = 15
	}
	client := opts.ClientName
	if client == "" {
		client = "Cliente"
	}
	cur := est.Currency

	var b strings.Builder
	fmt.Fprintf(&b, "# Proposta Técnica e Comercial — %s\n\n", snap.Manifest.ProjectName)
	fmt.Fprintf(&b, "**Cliente:** %s  \n", client)
	fmt.Fprintf(&b, "**Data:** %s  \n", time.Now().Format("02/01/2006"))
	fmt.Fprintf(&b, "**Validade da proposta:** %d dias  \n", opts.ValidityDays)
	fmt.Fprintf(&b, "**Versão do escopo:** %s\n\n", snap.Manifest.Version)
	b.WriteString("---\n\n")

	b.WriteString("## 1. Escopo do Projeto\n\n")
	if snap.Manifest.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", snap.Manifest.Description)
	}
	if snap.Requirements != nil && snap.Requirements.Overview != "" {
		fmt.Fprintf(&b, "%s\n\n", snap.Requirements.Overview)
	}

	if opts.IncludeDiagram {
		b.WriteString("### Arquitetura Proposta\n\n```mermaid\n")
		b.WriteString(strings.TrimRight(mermaid.Export(snap.Diagram), "\n"))
		b.WriteString("\n```\n\n")
	}

	b.WriteString("## 2. Componentes Entregues\n\n")
	b.WriteString("| Componente | Tipo | Tecnologia | Complexidade | Esforço |\n")
	b.WriteString("| :--- | :--- | :--- | :--- | ---: |\n")
	nodeHours := map[string]float64{}
	for _, item := range est.Items {
		if item.Kind == "node" {
			nodeHours[item.ID] = item.Hours
		}
	}
	nodes := append([]model.Node(nil), snap.Diagram.Nodes...)
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

	if len(snap.UseCases) > 0 {
		b.WriteString("## 3. Casos de Uso Contemplados\n\n")
		for _, uc := range snap.UseCases {
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
	fmt.Fprintf(&b, "| Componentes de arquitetura | %s |\n", pricing.Hours(est.NodeHours))
	fmt.Fprintf(&b, "| Integrações e contratos de API | %s |\n", pricing.Hours(est.EdgeHours))
	fmt.Fprintf(&b, "| Casos de uso e regras de negócio | %s |\n", pricing.Hours(est.UseCaseHours))
	fmt.Fprintf(&b, "| **Subtotal** | **%s** |\n", pricing.Hours(est.BaseHours))
	fmt.Fprintf(&b, "| Margem de contingência (%.0f%%) | %s |\n", est.RiskMarginPercentage, pricing.Hours(est.MarginHours))
	fmt.Fprintf(&b, "| **Total** | **%s** |\n\n", pricing.Hours(est.TotalHours))

	b.WriteString("## 5. Investimento\n\n")
	b.WriteString("| Perfil | Horas | Valor/hora | Subtotal |\n| :--- | ---: | ---: | ---: |\n")
	for _, r := range est.Roles {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			roleLabel(r.Role), pricing.Hours(r.Hours), pricing.Money(cur, r.Rate), pricing.Money(cur, r.Subtotal))
	}
	fmt.Fprintf(&b, "| **Subtotal de desenvolvimento** | | | **%s** |\n", pricing.Money(cur, est.PersonnelCost))
	fmt.Fprintf(&b, "| Impostos (%.0f%%) | | | %s |\n", est.TaxPercentage, pricing.Money(cur, est.TaxAmount))
	fmt.Fprintf(&b, "| **TOTAL DO PROJETO** | | | **%s** |\n\n", pricing.Money(cur, est.TotalCost))

	if opts.IncludeCloud && est.CloudMonthlyCost > 0 {
		b.WriteString("## 6. Custo Mensal de Infraestrutura (estimado)\n\n")
		b.WriteString("| Componente | Tier | Custo mensal |\n| :--- | :--- | ---: |\n")
		for _, c := range est.CloudItems {
			fmt.Fprintf(&b, "| %s | `%s` | %s |\n", c.Label, orDashStr(c.Tier), pricing.Money(cur, c.MonthlyCost))
		}
		fmt.Fprintf(&b, "| **Total mensal** | | **%s** |\n", pricing.Money(cur, est.CloudMonthlyCost))
		fmt.Fprintf(&b, "| Total anual | | %s |\n\n", pricing.Money(cur, est.CloudYearlyCost))
		b.WriteString("> Custos de infraestrutura são repassados pelo provedor de nuvem e não estão inclusos no valor do projeto.\n\n")
	}

	b.WriteString("## 7. Prazo Estimado\n\n")
	fmt.Fprintf(&b, "- Equipe considerada: **%.0f pessoa(s)** a %.0f h/dia produtivas\n", est.TeamSize, est.HoursPerDay)
	fmt.Fprintf(&b, "- Duração estimada: **%.0f dias úteis** (~%.1f semanas / %.1f meses)\n",
		est.WorkingDays, est.CalendarWeeks, est.CalendarMonths)
	if est.EstimatedFinish != "" {
		fmt.Fprintf(&b, "- Entrega projetada (se iniciar hoje): **%s**\n", est.EstimatedFinish)
	}
	b.WriteString("\n")

	if opts.Notes != "" {
		fmt.Fprintf(&b, "## 8. Observações\n\n%s\n\n", opts.Notes)
	}

	b.WriteString("---\n\n")
	b.WriteString("> Estimativa gerada automaticamente pelo ArchCode Studio a partir do escopo modelado.\n")
	b.WriteString("> Alterações no escopo visual recalculam automaticamente esforço e investimento.\n")

	md := b.String()
	if err := a.st.WriteFile(store.FileProposal, []byte(md)); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileProposal,
		Message: "Proposta comercial gerada"})

	return &ProposalResult{
		FilePath: store.FileProposal, Markdown: md,
		TotalCost: est.TotalCost, Currency: cur,
	}, nil
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

// ---------------------------------------------------------------------------
// OpenAPI 3.1 (RF: exportação de contratos)
// ---------------------------------------------------------------------------

const FileOpenAPI = "api/openapi.yaml"

// ExportOpenAPI converte api/endpoints.yaml em uma especificação OpenAPI 3.1.
func (a *App) ExportOpenAPI(source string) (string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	snap, err := a.st.Snapshot()
	if err != nil {
		return "", err
	}

	type oaResponse struct {
		Description string `yaml:"description"`
	}
	type oaOperation struct {
		Summary     string                `yaml:"summary,omitempty"`
		Description string                `yaml:"description,omitempty"`
		OperationID string                `yaml:"operationId"`
		Tags        []string              `yaml:"tags,omitempty"`
		Security    []map[string][]string `yaml:"security,omitempty"`
		Responses   map[string]oaResponse `yaml:"responses"`
	}

	paths := map[string]map[string]oaOperation{}
	needsAuth := false
	for _, e := range snap.Endpoints.Endpoints {
		method := strings.ToLower(e.Method)
		if _, ok := paths[e.Path]; !ok {
			paths[e.Path] = map[string]oaOperation{}
		}
		responses := map[string]oaResponse{}
		codes := e.StatusCodes
		if len(codes) == 0 {
			codes = []int{200}
		}
		for _, c := range codes {
			desc := e.Response
			if desc == "" {
				desc = statusText(c)
			}
			responses[fmt.Sprintf("%d", c)] = oaResponse{Description: desc}
		}
		op := oaOperation{
			Summary:     e.Summary,
			Description: e.Description,
			OperationID: model.Slugify(e.Method + "-" + e.Path),
			Responses:   responses,
		}
		if t := snap.Diagram.NodeByID(e.Target); t != nil {
			op.Tags = []string{t.Data.Label}
		}
		if e.Auth != "" && !strings.EqualFold(e.Auth, "none") {
			needsAuth = true
			op.Security = []map[string][]string{{"bearerAuth": {}}}
		}
		paths[e.Path][method] = op
	}

	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       snap.Manifest.ProjectName + " API",
			"version":     snap.Manifest.Version,
			"description": snap.Manifest.Description,
		},
		"servers": []map[string]string{
			{"url": orDefault(snap.Endpoints.BaseURL, "/api/v1"), "description": "Servidor padrão"},
		},
		"paths": paths,
	}
	if needsAuth {
		doc["components"] = map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]string{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			},
		}
	}

	var sb strings.Builder
	sb.WriteString("# Gerado pelo ArchCode Studio a partir de api/endpoints.yaml — não edite à mão.\n")
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}
	_ = enc.Close()

	if err := a.st.WriteFile(FileOpenAPI, []byte(sb.String())); err != nil {
		return "", err
	}
	a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: FileOpenAPI,
		Message: "Especificação OpenAPI 3.1 exportada"})
	return FileOpenAPI, nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Criado"
	case 202:
		return "Aceito para processamento assíncrono"
	case 204:
		return "Sem conteúdo"
	case 400:
		return "Requisição inválida"
	case 401:
		return "Não autenticado"
	case 403:
		return "Não autorizado"
	case 404:
		return "Não encontrado"
	case 409:
		return "Conflito"
	case 422:
		return "Entidade não processável"
	case 429:
		return "Limite de requisições excedido"
	case 500:
		return "Erro interno"
	case 502:
		return "Erro no serviço upstream"
	default:
		return fmt.Sprintf("Status %d", code)
	}
}

// ---------------------------------------------------------------------------
// Resumo compacto para LLMs (RNF004)
// ---------------------------------------------------------------------------

// ContextSummary é a resposta enxuta de get_system_context: apenas o essencial,
// sem campos nulos, para caber confortavelmente na janela de contexto.
type ContextSummary struct {
	Project      string            `json:"project"`
	Description  string            `json:"description,omitempty"`
	Version      string            `json:"version,omitempty"`
	Stack        []string          `json:"stack,omitempty"`
	Counts       map[string]int    `json:"counts"`
	Components   []componentBrief  `json:"components"`
	Connections  []connectionBrief `json:"connections,omitempty"`
	Endpoints    []string          `json:"endpoints,omitempty"`
	Requirements []string          `json:"requirements,omitempty"`
	UseCases     []string          `json:"use_cases,omitempty"`
	ADRs         []string          `json:"adrs,omitempty"`
	Pricing      *pricingBrief     `json:"pricing,omitempty"`
	Progress     *progressBrief    `json:"progress,omitempty"`
	Warnings     []string          `json:"warnings,omitempty"`
}

type componentBrief struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Type   string `json:"type"`
	Tier   string `json:"tier"`
	Tech   string `json:"tech,omitempty"`
	Status string `json:"status,omitempty"`
}

type connectionBrief struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Protocol string `json:"protocol,omitempty"`
}

type pricingBrief struct {
	Currency     string  `json:"currency"`
	TotalHours   float64 `json:"total_hours"`
	TotalCost    float64 `json:"total_cost"`
	CloudMonthly float64 `json:"cloud_monthly"`
}

type progressBrief struct {
	TotalTasks int `json:"total_tasks"`
	Completed  int `json:"completed"`
	Percentage int `json:"percentage"`
}

func (a *App) ContextSummary(includePricing bool) (*ContextSummary, error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	sum := &ContextSummary{
		Project:     snap.Manifest.ProjectName,
		Description: snap.Manifest.Description,
		Version:     snap.Manifest.Version,
		Counts:      map[string]int{},
		Components:  []componentBrief{},
	}

	techSeen := map[string]bool{}
	for _, n := range snap.Diagram.Nodes {
		if n.Type == "group" {
			continue
		}
		sum.Components = append(sum.Components, componentBrief{
			ID: n.ID, Label: n.Data.Label, Type: n.Type,
			Tier: layout.ResolveTier(n), Tech: n.Data.Technology, Status: n.Data.Status,
		})
		if n.Data.Technology != "" && !techSeen[n.Data.Technology] {
			techSeen[n.Data.Technology] = true
			sum.Stack = append(sum.Stack, n.Data.Technology)
		}
	}
	sort.Strings(sum.Stack)

	for _, e := range snap.Diagram.Edges {
		sum.Connections = append(sum.Connections, connectionBrief{
			From: labelOrID(snap.Diagram, e.Source), To: labelOrID(snap.Diagram, e.Target),
			Protocol: e.Data.Protocol,
		})
	}
	for _, e := range snap.Endpoints.Endpoints {
		sum.Endpoints = append(sum.Endpoints, strings.ToUpper(e.Method)+" "+e.Path)
	}
	for _, r := range snap.Requirements.Requirements {
		sum.Requirements = append(sum.Requirements, fmt.Sprintf("%s: %s", r.ID, r.Title))
	}
	for _, uc := range snap.UseCases {
		sum.UseCases = append(sum.UseCases, fmt.Sprintf("%s: %s", uc.Code, uc.Name))
	}
	for _, adr := range snap.ADRs {
		sum.ADRs = append(sum.ADRs, fmt.Sprintf("%s: %s", adr.ID, adr.Title))
	}

	sum.Counts["components"] = len(sum.Components)
	sum.Counts["connections"] = len(snap.Diagram.Edges)
	sum.Counts["endpoints"] = len(snap.Endpoints.Endpoints)
	sum.Counts["requirements"] = len(snap.Requirements.Requirements)
	sum.Counts["use_cases"] = len(snap.UseCases)
	sum.Counts["adrs"] = len(snap.ADRs)

	if len(snap.Tasks.Tasks) > 0 {
		completed := 0
		for _, t := range snap.Tasks.Tasks {
			if t.Status == model.StatusCompleted {
				completed++
			}
		}
		sum.Progress = &progressBrief{
			TotalTasks: len(snap.Tasks.Tasks), Completed: completed,
			Percentage: snap.Tasks.ProgressPercentage(),
		}
	} else {
		sum.Warnings = append(sum.Warnings, "Nenhum AI-PRD gerado ainda: rode generate_ai_prd antes de começar a implementar.")
	}

	if includePricing {
		est := pricing.Calculate(snap.Diagram, snap.UseCases, snap.Pricing, -1)
		sum.Pricing = &pricingBrief{
			Currency: est.Currency, TotalHours: est.TotalHours,
			TotalCost: est.TotalCost, CloudMonthly: est.CloudMonthlyCost,
		}
	}
	return sum, nil
}

func labelOrID(d *model.Diagram, id string) string {
	if n := d.NodeByID(id); n != nil {
		return n.Data.Label
	}
	return id
}
