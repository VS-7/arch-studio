// Package lint implementa o validador de regras arquiteturais (RF016:
// validate_architecture_rules). As regras detectam problemas estruturais que
// costumam passar despercebidos em diagramas feitos à mão.
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
)

const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

type Finding struct {
	Rule     string `json:"rule"`
	Severity string `json:"severity"`
	Target   string `json:"target,omitempty"`
	TargetID string `json:"target_id,omitempty"`
	Message  string `json:"message"`
	Fix      string `json:"fix,omitempty"`
}

type Report struct {
	Findings []Finding `json:"findings"`
	Errors   int       `json:"errors"`
	Warnings int       `json:"warnings"`
	Infos    int       `json:"infos"`
	Passed   bool      `json:"passed"`
	Score    int       `json:"score"`
}

type Input struct {
	Diagram      *model.Diagram
	Requirements *model.RequirementsDoc
	UseCases     []model.UseCase
	Endpoints    *model.EndpointsSpec
}

// Run executa todas as regras e devolve um relatório ordenado por severidade.
func Run(in Input) *Report {
	rep := &Report{Findings: []Finding{}}
	d := in.Diagram
	if d == nil {
		d = model.NewDiagram()
	}

	add := func(f Finding) { rep.Findings = append(rep.Findings, f) }

	degree := map[string]int{}
	outgoing := map[string][]model.Edge{}
	incoming := map[string][]model.Edge{}
	for _, e := range d.Edges {
		degree[e.Source]++
		degree[e.Target]++
		outgoing[e.Source] = append(outgoing[e.Source], e)
		incoming[e.Target] = append(incoming[e.Target], e)
	}

	requirementComponents := map[string]bool{}
	if in.Requirements != nil {
		for _, r := range in.Requirements.Requirements {
			for _, c := range r.Components {
				requirementComponents[strings.ToLower(c)] = true
			}
		}
	}
	useCaseComponents := map[string]bool{}
	for _, uc := range in.UseCases {
		for _, c := range uc.Components {
			useCaseComponents[strings.ToLower(c)] = true
		}
	}

	hasPersistence := false
	for _, n := range d.Nodes {
		if n.Type == "database" || n.Type == "storage" {
			hasPersistence = true
		}
	}

	for _, n := range d.Nodes {
		if n.Type == "group" {
			continue
		}

		// R001 — componente isolado
		if degree[n.ID] == 0 {
			add(Finding{
				Rule: "R001-isolated-node", Severity: SeverityError,
				Target: n.Data.Label, TargetID: n.ID,
				Message: "Componente sem nenhuma conexão: não participa de nenhum fluxo do sistema.",
				Fix:     "Conecte-o com `connect_nodes` ou remova-o da arquitetura.",
			})
		}

		// R002 — serviço de backend sem persistência alcançável
		if n.Type == "compute" && layout.ResolveTier(n) != "frontend" {
			reachesData := false
			for _, e := range outgoing[n.ID] {
				if t := d.NodeByID(e.Target); t != nil &&
					(t.Type == "database" || t.Type == "storage" || t.Type == "cache" || t.Type == "external_service" || t.Type == "queue") {
					reachesData = true
					break
				}
			}
			if !reachesData && hasPersistence {
				add(Finding{
					Rule: "R002-service-without-storage", Severity: SeverityWarning,
					Target: n.Data.Label, TargetID: n.ID,
					Message: "Serviço não acessa nenhum banco, cache, fila ou serviço externo — verifique se o estado dele está modelado.",
					Fix:     "Se o serviço for stateless, marque a tag `stateless` no nó para silenciar este aviso.",
				})
			}
		}

		// R003 — tecnologia não declarada
		if n.Data.Technology == "" {
			add(Finding{
				Rule: "R003-missing-technology", Severity: SeverityWarning,
				Target: n.Data.Label, TargetID: n.ID,
				Message: "Componente sem tecnologia declarada: a IA não tem como escolher o stack correto.",
				Fix:     "Preencha `technology` com `update_node_metadata`.",
			})
		}

		// R004 — rastreabilidade com requisitos (RF/CDU)
		key := strings.ToLower(n.ID)
		keyLabel := strings.ToLower(n.Data.Label)
		if !requirementComponents[key] && !requirementComponents[keyLabel] &&
			!useCaseComponents[key] && !useCaseComponents[keyLabel] {
			add(Finding{
				Rule: "R004-untraced-component", Severity: SeverityWarning,
				Target: n.Data.Label, TargetID: n.ID,
				Message: "Nenhum requisito ou caso de uso justifica este componente.",
				Fix:     "Crie um RF com `upsert_requirement` ou um CDU com `upsert_use_case` referenciando este nó.",
			})
		}

		// R005 — complexidade ausente compromete a precificação
		if n.Data.Pricing == nil || (n.Data.Pricing.Complexity == "" && n.Data.Pricing.EstimatedHours == 0) {
			add(Finding{
				Rule: "R005-missing-complexity", Severity: SeverityInfo,
				Target: n.Data.Label, TargetID: n.ID,
				Message: "Sem complexidade nem horas estimadas: a calculadora usará o valor padrão do tipo.",
				Fix:     "Defina `complexity` (low/medium/high) ou `estimated_hours` no nó.",
			})
		}

		// R006 — banco exposto diretamente ao cliente
		if n.Type == "database" {
			for _, e := range incoming[n.ID] {
				if s := d.NodeByID(e.Source); s != nil && s.Type == "client" {
					add(Finding{
						Rule: "R006-client-direct-database", Severity: SeverityError,
						Target: n.Data.Label, TargetID: n.ID,
						Message: fmt.Sprintf("Cliente `%s` acessa o banco diretamente, sem camada de serviço.", s.Data.Label),
						Fix:     "Insira um serviço ou gateway entre o cliente e o banco.",
					})
				}
			}
		}

		// R007 — serviço externo sem tratamento de falha declarado
		if n.Type == "external_service" && n.Data.Description == "" {
			add(Finding{
				Rule: "R007-external-without-contract", Severity: SeverityInfo,
				Target: n.Data.Label, TargetID: n.ID,
				Message: "Serviço externo sem descrição do contrato, SLA ou política de falha.",
				Fix:     "Descreva timeout, retry e comportamento de fallback na descrição do nó.",
			})
		}
	}

	// R008 — arestas sem protocolo
	for _, e := range d.Edges {
		if strings.TrimSpace(e.Data.Protocol) == "" {
			add(Finding{
				Rule: "R008-edge-without-protocol", Severity: SeverityWarning,
				Target:   fmt.Sprintf("%s → %s", nodeLabel(d, e.Source), nodeLabel(d, e.Target)),
				TargetID: e.ID,
				Message:  "Conexão sem protocolo definido.",
				Fix:      "Defina o protocolo (REST, gRPC, SQL, AMQP…) na aresta.",
			})
		}
		if d.NodeByID(e.Source) == nil || d.NodeByID(e.Target) == nil {
			add(Finding{
				Rule: "R009-dangling-edge", Severity: SeverityError, TargetID: e.ID,
				Message: "Conexão aponta para um nó inexistente.",
				Fix:     "Remova a aresta órfã ou recrie o nó ausente.",
			})
		}
	}

	// R010 — autenticação ausente em superfícies públicas
	publicEntry := false
	authPresent := false
	for _, n := range d.Nodes {
		low := strings.ToLower(n.Data.Label + " " + n.Data.Description + " " + strings.Join(n.Data.Tags, " "))
		if strings.Contains(low, "auth") || strings.Contains(low, "identity") ||
			strings.Contains(low, "login") || strings.Contains(low, "jwt") || strings.Contains(low, "oauth") {
			authPresent = true
		}
		if n.Type == "gateway" || n.Type == "client" {
			publicEntry = true
		}
	}
	if in.Endpoints != nil {
		for _, e := range in.Endpoints.Endpoints {
			if e.Auth != "" && !strings.EqualFold(e.Auth, "none") {
				authPresent = true
			}
		}
	}
	if publicEntry && !authPresent {
		add(Finding{
			Rule: "R010-missing-authentication", Severity: SeverityError,
			Message: "O sistema tem superfície pública (cliente/gateway) mas nenhum componente ou endpoint de autenticação.",
			Fix:     "Adicione um serviço de autenticação ou marque os endpoints com o esquema de auth correspondente.",
		})
	}

	// R011 — endpoints órfãos
	if in.Endpoints != nil {
		for _, e := range in.Endpoints.Endpoints {
			if e.Target != "" && d.NodeByID(e.Target) == nil {
				add(Finding{
					Rule: "R011-endpoint-without-component", Severity: SeverityWarning,
					Target: e.Method + " " + e.Path, TargetID: e.ID,
					Message: "Endpoint aponta para um componente que não existe no diagrama.",
					Fix:     "Recrie o componente ou ajuste o campo `target` em `api/endpoints.yaml`.",
				})
			}
		}
	}

	// R012 — caso de uso sem componente
	for _, uc := range in.UseCases {
		if len(uc.Components) == 0 {
			add(Finding{
				Rule: "R012-usecase-without-component", Severity: SeverityInfo,
				Target:  uc.Code,
				Message: "Caso de uso não está vinculado a nenhum componente da arquitetura.",
				Fix:     "Liste os componentes envolvidos no campo `components` do caso de uso.",
			})
		}
	}

	sevRank := map[string]int{SeverityError: 0, SeverityWarning: 1, SeverityInfo: 2}
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		if sevRank[rep.Findings[i].Severity] != sevRank[rep.Findings[j].Severity] {
			return sevRank[rep.Findings[i].Severity] < sevRank[rep.Findings[j].Severity]
		}
		return rep.Findings[i].Rule < rep.Findings[j].Rule
	})

	for _, f := range rep.Findings {
		switch f.Severity {
		case SeverityError:
			rep.Errors++
		case SeverityWarning:
			rep.Warnings++
		default:
			rep.Infos++
		}
	}
	rep.Passed = rep.Errors == 0
	rep.Score = 100 - rep.Errors*15 - rep.Warnings*5 - rep.Infos*1
	if rep.Score < 0 {
		rep.Score = 0
	}
	return rep
}

func nodeLabel(d *model.Diagram, id string) string {
	if n := d.NodeByID(id); n != nil {
		return n.Data.Label
	}
	return id
}
