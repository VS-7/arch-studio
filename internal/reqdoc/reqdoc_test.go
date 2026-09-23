package reqdoc

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/archcode/studio/internal/model"
)

// fixture monta um projeto pequeno mas completo, com todas as seções.
func fixture() Input {
	manifest := model.DefaultManifest("Estetify")
	manifest.Version = "1.0"
	manifest.Authors = []model.Author{{Name: "Rafael"}, {Name: "Vitor"}}

	diagram := model.NewDiagram()
	diagram.Nodes = []model.Node{
		{ID: "node-api", Type: "compute", Data: model.NodeData{Label: "Core API", Technology: "Go", Description: "Regras de negócio",
			Requirements: []string{"RF002"}}},
		{ID: "node-db", Type: "database", Data: model.NodeData{Label: "MySQL"}},
		{ID: "node-grupo", Type: "group", Data: model.NodeData{Label: "VPC"}},
	}

	ucDiagram := model.NewUMLDiagram("casos-de-uso", model.UMLKindUseCase, "Casos de Uso")
	ucDiagram.Elements = []model.UMLElement{
		{ID: "el-medico", Type: "actor", Name: "Médico"},
		{ID: "el-cdu001", Type: "usecase", Name: "Visualizar feedback", UseCase: "CDU001"},
	}
	ucDiagram.Relations = []model.UMLRelation{{ID: "rel-1", Type: "association", Source: "el-medico", Target: "el-cdu001"}}
	classes := model.NewUMLDiagram("modelo-de-dominio", model.UMLKindClass, "Modelo de Domínio")
	classes.Description = "Entidades principais."
	states := model.NewUMLDiagram("ciclo", model.UMLKindState, "Ciclo da consulta")

	return Input{
		Manifest: manifest,
		Meta: model.DocumentMeta{
			Date:       "2025-01-05",
			Client:     "Clínicas de estética.",
			Users:      "Funcionários e pacientes.",
			History:    []model.DocRevision{{Date: "2024-12-14", Version: "0.1", Description: "Draft inicial do documento.", Author: "Rafael"}},
			References: []string{"IEEE Std 830-1998."},
			Glossary:   []model.GlossaryTerm{{Term: "CDU", Definition: "Caso de uso"}},
		},
		Diagram: diagram,
		Requirements: &model.RequirementsDoc{
			ProjectName: "Estetify",
			Overview:    "O sistema automatiza atendimentos.",
			Requirements: []model.Requirement{
				{ID: "RF001", Type: "RF", Title: "Visualizar feedback", Priority: "Alta", Description: "O usuário deve poder visualizar os feedbacks."},
				{ID: "RF002", Type: "RF", Title: "Realizar login", Priority: "Média", Components: []string{"node-db"}},
				{ID: "RNF001", Type: "RNF", Title: "Interface amigável", Priority: "Média", Category: "Usabilidade", Related: []string{"Todos"}},
				{ID: "RNF002", Type: "RNF", Title: "Banco MySQL", Priority: "Baixa", Category: "Software", Related: []string{"RF001", "rf002"}},
				{ID: "RNF003", Type: "RNF", Title: "Navegação simples", Category: "usabilidade"},
				{ID: "RNF004", Type: "RNF", Title: "Sem categoria"},
			},
		},
		UseCases: []model.UseCase{
			{Code: "CDU001", Name: "Visualizar feedback", Description: "Permite ver os feedbacks.",
				Actors: []string{"Usuário"}, Priority: "Alta", Requirements: []string{"RF001"},
				PreConditions: []string{"Usuário autenticado"}, PostConditions: []string{"Lista exibida"},
				MainFlow: []string{"Faz login", "Seleciona Feedbacks"}},
			{Code: "CDU002", Name: "Gerenciar pacientes", Actors: []string{"Recepcionista", "usuário"},
				Requirements: []string{"RF001", "RF002"},
				MainFlow: []string{
					"# Acesso à funcionalidade", "Acessa o menu", "Sistema lista",
					"# Cadastro de paciente", "Escolhe Adicionar", "Preenche", "Salva",
				},
				AlternateFlows: []string{"# Erro de validação", "Exibe erro"},
				Exceptions:     []string{"Falha no banco"},
				BusinessRules:  []string{"CPF único"},
			},
		},
		ADRs:        []model.ADR{{ID: "ADR-001", Title: "Usar JWT", Status: "Aceito", Context: "Precisamos autenticar.", Decision: "JWT.", Consequences: "Stateless."}},
		Endpoints:   &model.EndpointsSpec{Endpoints: []model.Endpoint{{Method: "post", Path: "/api/login", Summary: "Autentica", Auth: "none"}}},
		UMLDiagrams: []model.UMLDiagram{*ucDiagram, *classes, *states},
		Now:         time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC),
	}
}

func headings(doc *Document, level int) []string {
	out := []string{}
	for _, b := range doc.Blocks {
		if b.Type == BlockHeading && b.Level == level {
			out = append(out, strings.TrimSpace(b.Number+" "+b.Text))
		}
	}
	return out
}

func TestOrdemDasSecoes(t *testing.T) {
	doc := Build(fixture())
	want := []string{
		"1 Introdução", "2 Descrição geral do sistema", "3 Requisitos funcionais",
		"4 Requisitos não funcionais", "5 Diagramas de casos de uso", "6 Detalhamento dos casos de uso",
		"7 Modelagem do sistema", "8 Arquitetura do sistema", "9 Matriz de rastreabilidade", "10 Referências",
	}
	if got := headings(doc, 1); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("seções:\n got %v\nwant %v", got, want)
	}
	wantSub := []string{
		"1.1 Visão geral do documento", "1.2 Convenções, termos e abreviações",
		"2.1 Cliente", "2.2 Usuário", "2.3 Visão Geral do Sistema",
		"4.1 Usabilidade", "4.2 Software", "4.3 Gerais",
		"7.1 Diagramas de classes", "7.2 Diagramas de estados",
		"8.1 Decisões arquiteturais", "8.2 Contratos de API",
	}
	if got := headings(doc, 2); strings.Join(got, "|") != strings.Join(wantSub, "|") {
		t.Errorf("subseções:\n got %v\nwant %v", got, wantSub)
	}

	// Página nova antes de cada seção de nível 1.
	for i, b := range doc.Blocks {
		if b.Type == BlockHeading && b.Level == 1 && (i == 0 || doc.Blocks[i-1].Type != BlockPageBreak) {
			t.Errorf("seção %q sem pagebreak antes", b.Text)
		}
	}
	if doc.Date != "05/01/2025" || doc.Version != "1.0" || strings.Join(doc.Authors, ",") != "Rafael,Vitor" {
		t.Errorf("capa: %q %q %v", doc.Date, doc.Version, doc.Authors)
	}
	if len(doc.History) != 1 || doc.History[0].Date != "14/12/2024" {
		t.Errorf("histórico: %+v", doc.History)
	}
	// A visão geral do documento lista as seções efetivamente presentes.
	var overview []string
	for _, b := range doc.Blocks {
		if b.Type == BlockList && len(b.Items) > 0 && strings.HasPrefix(b.Items[0], "**Seção 2 –") {
			overview = b.Items
		}
	}
	if len(overview) != 9 || !strings.HasPrefix(overview[8], "**Seção 10 – Referências**:") {
		t.Errorf("visão geral do documento: %v", overview)
	}
	if !strings.Contains(overview[2], "divididos em requisitos de usabilidade e software.") {
		t.Errorf("resumo dos RNFs: %q", overview[2])
	}
}

// Seções sem conteúdo somem e as seguintes são renumeradas, sem buracos.
func TestSecoesVaziasSaoOmitidasERenumeradas(t *testing.T) {
	in := fixture()
	in.Meta.Client, in.Meta.References = "", nil
	in.UMLDiagrams = nil
	in.ADRs = nil
	in.Endpoints = nil
	in.Diagram = model.NewDiagram()
	in.Requirements.Requirements = in.Requirements.Requirements[:2] // só RFs
	doc := Build(in)

	want := []string{
		"1 Introdução", "2 Descrição geral do sistema", "3 Requisitos funcionais",
		"4 Detalhamento dos casos de uso", "5 Matriz de rastreabilidade",
	}
	if got := headings(doc, 1); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("seções:\n got %v\nwant %v", got, want)
	}
	for _, h := range headings(doc, 2) {
		if strings.Contains(h, "Cliente") {
			t.Errorf("subseção vazia presente: %q", h)
		}
	}
	if got := headings(doc, 2); got[2] != "2.1 Usuário" {
		t.Errorf("subseções renumeradas: %v", got)
	}
	// Sem figuras, nenhuma imagem nem legenda.
	if len(doc.Figures()) != 0 {
		t.Errorf("figuras inesperadas: %+v", doc.Figures())
	}
	// O sumário acompanha a numeração.
	for _, e := range doc.TOC {
		if e.Title == "Matriz de rastreabilidade" && e.Number != "5" {
			t.Errorf("sumário: %+v", e)
		}
	}
}

func TestPriorityLevel(t *testing.T) {
	cases := map[string]string{
		"Alta": "essencial", "Essencial": "essencial", "Crítica": "essencial", "High": "essencial", "Must": "essencial",
		"Média": "importante", "Media": "importante", "Importante": "importante", "Medium": "importante", "Should": "importante",
		"Baixa": "desejavel", "Desejável": "desejavel", "Low": "desejavel", "Could": "desejavel", "Won't": "desejavel",
		"": "importante",
	}
	for in, want := range cases {
		if got := PriorityLevel(in); got != want {
			t.Errorf("PriorityLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQuadroDePrioridadeNoMarkdown(t *testing.T) {
	want := "| Prioridade: | ■ | Essencial | ◻ | Importante | ◻ | Desejável |\n" +
		"| :---- | ----: | :---- | ----: | :---- | ----: | :---- |"
	if got := PriorityTable(PriorityEssential); got != want {
		t.Errorf("quadro:\n%s", got)
	}
	doc := Build(fixture())
	if !strings.Contains(doc.Markdown, "### \\[RF001\\] VISUALIZAR FEEDBACK {#rf001-visualizar-feedback}\n\n"+
		"**Descrição:** O usuário deve poder visualizar os feedbacks.\n\n"+want+"\n\n") {
		t.Errorf("RF001 fora do formato do modelo:\n%s", doc.Markdown)
	}
	if !strings.Contains(doc.Markdown, "| Prioridade: | ◻ | Essencial | ◻ | Importante | ■ | Desejável |") {
		t.Error("RNF002 (Baixa) deveria marcar Desejável")
	}
}

// findUseCase devolve os blocos de uma ficha, do título até o próximo título.
func findUseCase(doc *Document, code string) []Block {
	for i, b := range doc.Blocks {
		if b.Type == BlockHeading && strings.HasPrefix(b.Text, "["+code+"]") {
			j := i + 1
			for j < len(doc.Blocks) && doc.Blocks[j].Type != BlockHeading && doc.Blocks[j].Type != BlockPageBreak {
				j++
			}
			return doc.Blocks[i:j]
		}
	}
	return nil
}

func TestFluxoDoCasoDeUsoContinuaNumeracaoAposSubtitulos(t *testing.T) {
	doc := Build(fixture())
	blocks := findUseCase(doc, "CDU002")
	if blocks == nil {
		t.Fatal("CDU002 ausente")
	}
	var trail []string
	for _, b := range blocks {
		switch b.Type {
		case BlockList:
			if b.Ordered {
				trail = append(trail, strings.Join(append([]string{strconv.Itoa(b.Start)}, b.Items...), ","))
			}
		case BlockParagraph:
			if strings.HasSuffix(b.Text, ":**") {
				trail = append(trail, b.Text)
			}
		}
	}
	want := []string{
		"**Acesso à funcionalidade:**", "1,Acessa o menu,Sistema lista",
		"**Cadastro de paciente:**", "3,Escolhe Adicionar,Preenche,Salva",
		// Fluxos secundários: numeração própria, alternativos e exceções juntos.
		"**Erro de validação:**", "1,Exibe erro,Falha no banco",
	}
	if strings.Join(trail, "|") != strings.Join(want, "|") {
		t.Errorf("fluxos:\n got %v\nwant %v", trail, want)
	}
	if !strings.Contains(doc.Markdown, "**Cadastro de paciente:**\n\n3. Escolhe Adicionar\n4. Preenche\n5. Salva") {
		t.Errorf("markdown do fluxo não continua a numeração:\n%s", doc.Markdown)
	}
}

func TestSemFluxosSecundariosEscreveNaoHa(t *testing.T) {
	doc := Build(fixture())
	blocks := findUseCase(doc, "CDU001")
	for i, b := range blocks {
		if b.Type == BlockParagraph && b.Text == "**Fluxos secundários/exceção**" {
			if i+1 >= len(blocks) || blocks[i+1].Text != "Não há." {
				t.Errorf("esperava 'Não há.' após fluxos secundários, got %+v", blocks[i+1:])
			}
			return
		}
	}
	t.Error("seção de fluxos secundários ausente")
}

func TestRastreabilidade(t *testing.T) {
	doc := Build(fixture())
	texts := []string{}
	for _, b := range doc.Blocks {
		if b.Type == BlockParagraph {
			texts = append(texts, b.Text)
		}
	}
	all := strings.Join(texts, "\n")
	for _, want := range []string{
		"**Casos de uso associados:** [CDU001], [CDU002].", // RF001
		"**Requisitos associados:** Todos.",                // RNF001
		"**Requisitos associados:** [RF001], [RF002].",     // RNF002, ids normalizados
		"**Requisitos Associados:** [RF001], [RF002]",      // CDU002
		"**Ator**: Recepcionista, usuário",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("rastreabilidade sem %q", want)
		}
	}

	var matrix, actors *Block
	for i := range doc.Blocks {
		b := &doc.Blocks[i]
		if b.Type == BlockTable && b.Header[0] == "Requisito" {
			matrix = b
		}
		if b.Type == BlockTable && b.Header[0] == "Ator" {
			actors = b
		}
	}
	if matrix == nil || len(matrix.Rows) != 6 {
		t.Fatalf("matriz: %+v", matrix)
	}
	// RF002: casos de uso pela ficha; componentes declarados + nós que apontam para ele.
	if row := matrix.Rows[1]; row[1] != "[CDU002]" || row[2] != "MySQL, Core API" {
		t.Errorf("linha RF002: %v", row)
	}
	if row := matrix.Rows[2]; row[1] != "—" || row[2] != "—" {
		t.Errorf("linha RNF001: %v", row)
	}
	// Atores das fichas (sem duplicar "Usuário"/"usuário") e dos diagramas.
	if actors == nil || len(actors.Rows) != 3 {
		t.Fatalf("atores: %+v", actors)
	}
	if actors.Rows[0][0] != "Usuário" || actors.Rows[0][1] != "[CDU001], [CDU002]" ||
		actors.Rows[2][0] != "Médico" || actors.Rows[2][1] != "[CDU001]" {
		t.Errorf("tabela de atores: %v", actors.Rows)
	}
}

func TestFigurasNumeradasEmOrdem(t *testing.T) {
	doc := Build(fixture())
	figs := doc.Figures()
	want := []struct{ caption, src, diagram string }{
		{"Figura 1 – Casos de Uso", "/api/export/uml/casos-de-uso.svg", "casos-de-uso"},
		{"Figura 2 – Modelo de Domínio", "/api/export/uml/modelo-de-dominio.svg", "modelo-de-dominio"},
		{"Figura 3 – Ciclo da consulta", "/api/export/uml/ciclo.svg", "ciclo"},
		{"Figura 4 – Arquitetura de componentes", MacroImageSrc, MacroDiagramID},
	}
	if len(figs) != len(want) {
		t.Fatalf("figuras: %+v", figs)
	}
	for i, w := range want {
		if figs[i].Caption != w.caption || figs[i].Src != w.src || figs[i].Diagram != w.diagram {
			t.Errorf("figura %d: %+v", i+1, figs[i])
		}
	}
	md := Markdown(doc, MarkdownOptions{ImageSrc: func(b Block) string { return "diagramas/" + b.Diagram + ".svg" }})
	if !strings.Contains(md, "![Figura 2 – Modelo de Domínio](diagramas/modelo-de-dominio.svg)\n\n*Figura 2 – Modelo de Domínio*") {
		t.Errorf("imagem com src reescrito ausente:\n%s", md)
	}
}

func TestAncorasEstaveisESumario(t *testing.T) {
	doc := Build(fixture())
	ids := map[string]bool{}
	for _, e := range doc.TOC {
		if ids[e.ID] {
			t.Errorf("âncora duplicada: %q", e.ID)
		}
		ids[e.ID] = true
		if e.ID != strings.ToLower(e.ID) || strings.ContainsAny(e.ID, " []") {
			t.Errorf("âncora fora do padrão: %q", e.ID)
		}
	}
	for _, id := range []string{"introducao", "rf001-visualizar-feedback", "cdu002-gerenciar-pacientes", "usabilidade"} {
		if !ids[id] {
			t.Errorf("âncora %q ausente", id)
		}
	}
	if !strings.Contains(doc.Markdown, "- **[1 Introdução](#introducao)**\n  - [1.1 Visão geral do documento](#visao-geral-do-documento)") {
		t.Errorf("sumário do markdown:\n%s", doc.Markdown[:800])
	}
}

// Sem histórico no document.yaml, uma linha é gerada a partir da capa.
func TestHistoricoPadraoEJSON(t *testing.T) {
	in := fixture()
	in.Meta.History = nil
	in.Meta.Date = ""
	doc := Build(in)
	if len(doc.History) != 1 || doc.History[0].Date != "23/09/2026" ||
		doc.History[0].Description != "Versão gerada pelo ArchCode Studio." || doc.History[0].Author != "Rafael, Vitor" {
		t.Errorf("histórico padrão: %+v", doc.History)
	}

	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	for _, want := range []string{
		`{"type":"pagebreak"}`,
		`{"type":"heading","level":1,"number":"1","text":"Introdução","id":"introducao"}`,
		`{"type":"priority","value":"essencial"}`,
		`{"type":"list","ordered":true,"start":3,"items":["Escolhe Adicionar","Preenche","Salva"]}`,
		`{"type":"image","src":"/api/export/uml/casos-de-uso.svg","caption":"Figura 1 – Casos de Uso","diagram":"casos-de-uso"}`,
		`"toc":[{"id":"introducao","number":"1","title":"Introdução","level":1}`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("JSON sem %s", want)
		}
	}
}
