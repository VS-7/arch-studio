package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/openapi"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/proposal"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Proposta comercial (RF014) e OpenAPI
// ---------------------------------------------------------------------------

type ProposalOptions = proposal.Options

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
	md := proposal.Render(proposal.Input{
		Manifest: snap.Manifest, Diagram: snap.Diagram, Requirements: snap.Requirements,
		UseCases: snap.UseCases, Estimate: est, Date: time.Now(),
	}, opts)

	if err := a.st.WriteFile(store.FileProposal, []byte(md)); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileProposal,
		Message: "Proposta comercial gerada"})

	return &ProposalResult{
		FilePath: store.FileProposal, Markdown: md,
		TotalCost: est.TotalCost, Currency: est.Currency,
	}, nil
}

// ExportOpenAPI converte api/endpoints.yaml em uma especificação OpenAPI 3.1.
func (a *App) ExportOpenAPI(source string) (string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	snap, err := a.st.Snapshot()
	if err != nil {
		return "", err
	}
	data, err := openapi.Build(snap.Manifest, snap.Diagram, snap.Endpoints)
	if err != nil {
		return "", err
	}
	if err := a.st.WriteFile(store.FileOpenAPI, data); err != nil {
		return "", err
	}
	a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileOpenAPI,
		Message: "Especificação OpenAPI 3.1 exportada"})
	return store.FileOpenAPI, nil
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
	UMLDiagrams  []string          `json:"uml_diagrams,omitempty"`
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

	for _, d := range snap.UMLDiagrams {
		sum.UMLDiagrams = append(sum.UMLDiagrams, fmt.Sprintf("%s [%s] %s — %d elementos, %d relações",
			d.ID, d.Kind, d.Name, len(d.Elements), len(d.Relations)))
	}

	sum.Counts["components"] = len(sum.Components)
	sum.Counts["connections"] = len(snap.Diagram.Edges)
	sum.Counts["endpoints"] = len(snap.Endpoints.Endpoints)
	sum.Counts["requirements"] = len(snap.Requirements.Requirements)
	sum.Counts["use_cases"] = len(snap.UseCases)
	sum.Counts["adrs"] = len(snap.ADRs)
	sum.Counts["uml_diagrams"] = len(snap.UMLDiagrams)

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
