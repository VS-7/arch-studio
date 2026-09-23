package model

import (
	"strings"
	"testing"
)

// O contrato central dos documentos é Parse(Render(x)) == x. Sem ele, uma edição
// feita por IA poderia corromper o arquivo que um humano acabou de editar.
func TestRequirementsRoundTrip(t *testing.T) {
	original := &RequirementsDoc{
		ProjectName: "E-Commerce",
		Overview:    "Plataforma de vendas com checkout assíncrono.\n\nSegunda linha da visão geral.",
		Requirements: []Requirement{
			{ID: "RF001", Type: "RF", Title: "Autenticação de usuários", Priority: "Alta",
				Status: StatusPending, Components: []string{"node-auth", "node-db"},
				Description: "O sistema deve autenticar por e-mail e senha."},
			{ID: "RNF001", Type: "RNF", Title: "P95 abaixo de 300ms", Priority: "Média",
				Status: StatusCompleted, Components: []string{"node-api"},
				Description: "Endpoints de leitura respondem rápido."},
		},
	}

	parsed := ParseRequirements(RenderRequirements(original))

	if parsed.ProjectName != original.ProjectName {
		t.Errorf("project name: got %q, want %q", parsed.ProjectName, original.ProjectName)
	}
	if parsed.Overview != original.Overview {
		t.Errorf("overview: got %q, want %q", parsed.Overview, original.Overview)
	}
	if len(parsed.Requirements) != len(original.Requirements) {
		t.Fatalf("got %d requisitos, want %d", len(parsed.Requirements), len(original.Requirements))
	}
	for i, want := range original.Requirements {
		got := parsed.Requirements[i]
		if got.ID != want.ID || got.Type != want.Type || got.Title != want.Title ||
			got.Priority != want.Priority || got.Status != want.Status ||
			got.Description != want.Description || strings.Join(got.Components, ",") != strings.Join(want.Components, ",") {
			t.Errorf("requisito %d divergiu:\n got %+v\nwant %+v", i, got, want)
		}
	}

	// A segunda volta precisa ser byte a byte idêntica: é isso que mantém os
	// diffs de Git limpos quando humanos e IAs escrevem no mesmo arquivo.
	first := RenderRequirements(original)
	second := RenderRequirements(ParseRequirements(first))
	if first != second {
		t.Error("renderização não é idempotente")
	}
}

func TestUseCaseRoundTrip(t *testing.T) {
	original := &UseCase{
		Code: "CDU001", Name: "Autenticar Usuário",
		Actors: []string{"Usuário final", "Provedor OAuth"}, Components: []string{"node-web", "node-api"},
		Complexity: "high", EstimatedHours: 24, Priority: "Alta", Status: StatusInProgress,
		PreConditions:  []string{"Usuário cadastrado", "Conta ativa"},
		MainFlow:       []string{"Informa credenciais", "API valida", "Emite JWT"},
		AlternateFlows: []string{"Login social via OAuth2"},
		Exceptions:     []string{"Credenciais inválidas retorna 401"},
		BusinessRules:  []string{"Senha sempre com hash"},
		Acceptance:     []string{"**Given** credenciais válidas **When** POST /login **Then** 200."},
	}

	parsed := ParseUseCase(RenderUseCase(original))

	if parsed.Code != original.Code || parsed.Name != original.Name {
		t.Errorf("cabeçalho: got %q/%q", parsed.Code, parsed.Name)
	}
	if parsed.Complexity != "high" || parsed.EstimatedHours != 24 || parsed.Status != StatusInProgress {
		t.Errorf("metadados: %+v", parsed)
	}
	for name, pair := range map[string][2][]string{
		"atores":      {parsed.Actors, original.Actors},
		"componentes": {parsed.Components, original.Components},
		"pré":         {parsed.PreConditions, original.PreConditions},
		"fluxo":       {parsed.MainFlow, original.MainFlow},
		"alternativo": {parsed.AlternateFlows, original.AlternateFlows},
		"exceções":    {parsed.Exceptions, original.Exceptions},
		"regras":      {parsed.BusinessRules, original.BusinessRules},
		"aceite":      {parsed.Acceptance, original.Acceptance},
	} {
		if strings.Join(pair[0], "|") != strings.Join(pair[1], "|") {
			t.Errorf("%s: got %v, want %v", name, pair[0], pair[1])
		}
	}
}

// Um documento editado à mão, com formatação irregular, ainda precisa ser lido.
func TestParseRequirementsTolerantesAEdicaoHumana(t *testing.T) {
	src := `# Requisitos — Projeto Manual

## Visão Geral

Texto livre escrito à mão.

## Requisitos Funcionais

### RF7 - Título com hífen simples
* **Prioridade:** Baixa
* **Componentes:** node-a

Descrição em parágrafo.

### RNF002 — Sem metadados
`
	doc := ParseRequirements(src)
	if len(doc.Requirements) != 2 {
		t.Fatalf("got %d requisitos, want 2", len(doc.Requirements))
	}
	if doc.Requirements[0].ID != "RF007" {
		t.Errorf("normalização de id: got %q, want RF007", doc.Requirements[0].ID)
	}
	if doc.Requirements[0].Priority != "Baixa" || len(doc.Requirements[0].Components) != 1 {
		t.Errorf("metadados com bullet '*': %+v", doc.Requirements[0])
	}
	if doc.Requirements[1].Type != "RNF" {
		t.Errorf("tipo derivado do id: got %q", doc.Requirements[1].Type)
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Auth Service":            "auth-service",
		"Autenticação de Usuário": "autenticacao-de-usuario",
		"API  Gateway / Proxy":    "api-gateway-proxy",
		"  ---  ":                 "",
		"Serviço nº 1":            "servico-n-1",
	}
	for in, want := range cases {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNextRequirementID(t *testing.T) {
	doc := &RequirementsDoc{Requirements: []Requirement{
		{ID: "RF001", Type: "RF"}, {ID: "RF009", Type: "RF"}, {ID: "RNF001", Type: "RNF"},
	}}
	if got := doc.NextRequirementID("RF"); got != "RF010" {
		t.Errorf("próximo RF: got %q, want RF010", got)
	}
	if got := doc.NextRequirementID("RNF"); got != "RNF002" {
		t.Errorf("próximo RNF: got %q, want RNF002", got)
	}
}

// Seções vazias são renderizadas com um texto de preenchimento em itálico.
// Ele não pode voltar como dado na leitura seguinte, ou o AI-PRD passaria a
// exibir "_A definir_" como se fosse um critério de aceite real.
func TestPlaceholdersNaoViramDados(t *testing.T) {
	original := &UseCase{
		Code: "CDU010", Name: "Caso Sem Detalhes",
		MainFlow: []string{"passo único"},
	}
	parsed := ParseUseCase(RenderUseCase(original))

	for name, items := range map[string][]string{
		"pré-condições": parsed.PreConditions,
		"alternativos":  parsed.AlternateFlows,
		"exceções":      parsed.Exceptions,
		"regras":        parsed.BusinessRules,
		"aceite":        parsed.Acceptance,
	} {
		if len(items) != 0 {
			t.Errorf("%s deveria continuar vazio, got %v", name, items)
		}
	}
	if len(parsed.MainFlow) != 1 || parsed.MainFlow[0] != "passo único" {
		t.Errorf("fluxo principal: got %v", parsed.MainFlow)
	}

	// Um caso de uso totalmente vazio também não pode inventar conteúdo.
	empty := ParseUseCase(RenderUseCase(&UseCase{Code: "CDU011", Name: "Vazio"}))
	if len(empty.MainFlow) != 0 {
		t.Errorf("fluxo principal deveria ficar vazio, got %v", empty.MainFlow)
	}
}
