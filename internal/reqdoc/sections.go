package reqdoc

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// Textos padrão do modelo de documento de requisitos, adaptados ao projeto.

// ---------------------------------------------------------------------------
// 1 Introdução
// ---------------------------------------------------------------------------

func (g *gen) intro(b *builder) {
	b.section(secIntro)
	if intro := strings.TrimSpace(g.meta.Introduction); intro != "" {
		b.text(intro, "")
	} else {
		b.para(fmt.Sprintf("Este documento especifica os requisitos do sistema %s, fornecendo as informações "+
			"necessárias para o projeto e implementação, assim como para a realização dos testes e homologação do sistema.", g.project))
	}

	b.heading(2, "Visão geral do documento", "Visão geral do documento", true)
	if len(g.overviewItems) > 0 {
		b.para("Além desta seção introdutória, as seções seguintes estão organizadas como descrito abaixo.")
		b.list(false, 0, g.overviewItems)
	} else {
		b.para("Este documento é composto apenas por esta seção introdutória.")
	}

	b.heading(2, "Convenções, termos e abreviações", "Convenções, termos e abreviações", true)
	b.para("A correta interpretação deste documento exige o conhecimento de algumas convenções e termos específicos, que são descritos a seguir.")
	if len(g.meta.Glossary) > 0 {
		rows := [][]string{}
		for _, t := range g.meta.Glossary {
			if strings.TrimSpace(t.Term) == "" {
				continue
			}
			rows = append(rows, []string{strings.TrimSpace(t.Term), orDash(t.Definition)})
		}
		if len(rows) > 0 {
			b.table([]string{"Termo", "Definição"}, rows)
		}
	}

	b.heading(3, "Identificação dos requisitos", "Identificação dos requisitos", true)
	b.para("Por convenção, a referência a requisitos é feita através do identificador do requisito, de acordo com a especificação a seguir:")
	b.para("[*identificador do requisito*]")
	b.para("Os requisitos devem ser identificados com um identificador único. A numeração inicia com o identificador " +
		"[RF001] para os requisitos funcionais, [RNF001] para os não funcionais e [CDU001] para os casos de uso, " +
		"e prossegue sendo incrementada à medida que forem surgindo novos requisitos.")

	b.heading(3, "Prioridades dos requisitos", "Prioridades dos requisitos", true)
	b.para("Para estabelecer a prioridade dos requisitos, foram adotadas as denominações “essencial”, “importante” e “desejável”.")
	b.list(false, 0, []string{
		"**Essencial** é o requisito sem o qual o sistema não entra em funcionamento. Requisitos essenciais são " +
			"requisitos imprescindíveis, que têm que ser implementados impreterivelmente.",
		"**Importante** é o requisito sem o qual o sistema entra em funcionamento, mas de forma não satisfatória. " +
			"Requisitos importantes devem ser implementados, mas, se não forem, o sistema poderá ser implantado e usado mesmo assim.",
		"**Desejável** é o requisito que não compromete as funcionalidades básicas do sistema, isto é, o sistema pode " +
			"funcionar de forma satisfatória sem ele. Requisitos desejáveis podem ser deixados para versões posteriores " +
			"do sistema, caso não haja tempo hábil para implementá-los na versão que está sendo especificada.",
	})
}

// ---------------------------------------------------------------------------
// 2 Descrição geral do sistema
// ---------------------------------------------------------------------------

func (g *gen) general(b *builder) {
	b.section(secGeneral)
	b.para(fmt.Sprintf("Esta seção descreve superficialmente o cliente, os futuros usuários e fornece uma visão geral do %s.", g.project))

	if client := strings.TrimSpace(g.meta.Client); client != "" {
		b.heading(2, "Cliente", "Cliente", true)
		b.text(client, "")
	}

	if users := strings.TrimSpace(g.meta.Users); users != "" || len(g.actors) > 0 {
		b.heading(2, "Usuário", "Usuário", true)
		if users != "" {
			b.text(users, "")
		}
		if len(g.actors) > 0 {
			if users == "" {
				b.para("Os atores que interagem com o sistema e os casos de uso de que participam são listados a seguir.")
			}
			rows := [][]string{}
			for _, a := range g.actors {
				rows = append(rows, []string{a.name, orDash(refs(a.codes))})
			}
			b.table([]string{"Ator", "Casos de uso"}, rows)
		}
	}

	if g.overview != "" {
		b.heading(2, "Visão Geral do Sistema", "Visão Geral do Sistema", true)
		b.text(g.overview, "")
	}
}

// ---------------------------------------------------------------------------
// 3 Requisitos funcionais
// ---------------------------------------------------------------------------

// useCasesFor devolve os códigos das fichas cujo campo requirements inclui o id.
func (g *gen) useCasesFor(reqID string) []string {
	out := []string{}
	for _, uc := range g.in.UseCases {
		if containsFold(uc.Requirements, reqID) {
			out = append(out, uc.Code)
		}
	}
	return out
}

func reqHeading(r model.Requirement) (string, string) {
	title := strings.TrimSpace(r.Title)
	return fmt.Sprintf("[%s] %s", r.ID, strings.ToUpper(title)), r.ID + " " + title
}

func (g *gen) functional(b *builder) {
	b.section(secFunctional)
	for _, r := range g.rf {
		text, seed := reqHeading(r)
		b.heading(3, text, seed, false)
		b.text(orDash(r.Description), "**Descrição:** ")
		b.priority(r.Priority)
		if ucs := g.useCasesFor(r.ID); len(ucs) > 0 {
			b.para("**Casos de uso associados:** " + refs(ucs) + ".")
		}
	}
}

// ---------------------------------------------------------------------------
// 4 Requisitos não funcionais
// ---------------------------------------------------------------------------

// categoryIntros são as frases introdutórias das categorias conhecidas.
var categoryIntros = map[string]string{
	"usabilidade":     "Esta seção descreve os requisitos não funcionais associados à facilidade de uso da interface com o usuário e o *help on-line*.",
	"software":        "Esta seção descreve os requisitos não funcionais associados aos softwares que devem ser utilizados para o desenvolvimento do sistema.",
	"desempenho":      "Esta seção descreve os requisitos não funcionais associados à eficiência, uso de recursos e tempo de resposta do sistema.",
	"confiabilidade":  "Esta seção descreve os requisitos não funcionais associados à frequência, severidade de falhas do sistema e habilidade de recuperação das mesmas, bem como à corretude do sistema.",
	"seguranca":       "Esta seção descreve os requisitos não funcionais associados à integridade, privacidade e autenticidade dos dados e ao controle de acesso ao sistema.",
	"suportabilidade": "Esta seção descreve os requisitos não funcionais associados à facilidade de manutenção, configuração e evolução do sistema.",
	"portabilidade":   "Esta seção descreve os requisitos não funcionais associados à capacidade do sistema de operar em diferentes plataformas e ambientes.",
	"interface":       "Esta seção descreve os requisitos não funcionais associados às interfaces do sistema com usuários, hardware e outros sistemas.",
	"legal":           "Esta seção descreve os requisitos não funcionais associados a leis, normas, regulamentações e licenças que o sistema deve respeitar.",
	"gerais":          "Esta seção descreve os requisitos não funcionais que não se enquadram em uma categoria específica.",
}

func categoryIntro(name string) string {
	if intro, ok := categoryIntros[normalize(name)]; ok {
		return intro
	}
	return fmt.Sprintf("Esta seção descreve os requisitos não funcionais associados a %s.", strings.ToLower(strings.TrimSpace(name)))
}

func (g *gen) nonFunctional(b *builder) {
	b.section(secNonFunc)
	for _, grp := range g.groups {
		b.heading(2, grp.name, grp.name, true)
		b.para(categoryIntro(grp.name))
		for _, r := range grp.reqs {
			text, seed := reqHeading(r)
			b.heading(3, text, seed, false)
			if d := strings.TrimSpace(r.Description); d != "" {
				b.text(d, "")
			}
			b.priority(r.Priority)
			if len(r.Related) > 0 {
				if containsFold(r.Related, "todos") {
					b.para("**Requisitos associados:** Todos.")
				} else {
					b.para("**Requisitos associados:** " + refs(r.Related) + ".")
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 5 Diagramas de casos de uso
// ---------------------------------------------------------------------------

func (g *gen) useCaseDiagrams(b *builder) {
	b.section(secUCDiagrams)
	for _, d := range g.ucDiag {
		b.figure(UMLImageSrc(d.ID), d.Name, d.ID)
	}
}

// ---------------------------------------------------------------------------
// 6 Detalhamento dos casos de uso
// ---------------------------------------------------------------------------

// flow escreve um fluxo como listas numeradas; subtítulos ("# X") viram uma
// linha em negrito "**X:**" e a numeração continua através deles.
func flow(b *builder, items []string) {
	step := 0
	var run []string
	runStart := 1
	flush := func() {
		if len(run) > 0 {
			b.list(true, runStart, run)
		}
		run = nil
	}
	any := false
	for _, it := range items {
		it = strings.TrimSpace(it)
		if it == "" {
			continue
		}
		any = true
		if text, ok := model.IsFlowSubtitle(it); ok {
			flush()
			b.para("**" + text + ":**")
			continue
		}
		step++
		if len(run) == 0 {
			runStart = step
		}
		run = append(run, it)
	}
	flush()
	if !any {
		b.para("Não há.")
	}
}

func paragraphsOrNone(b *builder, items []string) {
	n := 0
	for _, it := range items {
		if it = strings.TrimSpace(it); it != "" {
			b.para(it)
			n++
		}
	}
	if n == 0 {
		b.para("Não há.")
	}
}

func (g *gen) useCaseDetail(b *builder) {
	b.section(secUCDetail)
	for _, uc := range g.in.UseCases {
		name := strings.TrimSpace(uc.Name)
		b.heading(3, fmt.Sprintf("[%s] %s", uc.Code, strings.ToUpper(name)), uc.Code+" "+name, false)
		b.text(orDash(uc.Description), "**Descrição do caso de uso:** ")

		actors := []string{}
		for _, a := range uc.Actors {
			if a = strings.TrimSpace(a); a != "" && a != "-" {
				actors = append(actors, a)
			}
		}
		b.para("**Ator**: " + orDash(strings.Join(actors, ", ")))
		b.priority(uc.Priority)
		b.para("**Requisitos Associados:** " + orDash(refs(uc.Requirements)))

		b.para("**Entradas e pré-condições**:")
		paragraphsOrNone(b, uc.PreConditions)
		b.para("**Saídas e pós-condição**:")
		paragraphsOrNone(b, uc.PostConditions)

		b.para("**Fluxo de eventos principal**")
		flow(b, uc.MainFlow)
		b.para("**Fluxos secundários/exceção**")
		flow(b, append(append([]string{}, uc.AlternateFlows...), uc.Exceptions...))

		if len(uc.BusinessRules) > 0 {
			b.para("**Regras de negócio**")
			b.list(false, 0, uc.BusinessRules)
		}
		if len(uc.Acceptance) > 0 {
			b.para("**Critérios de aceite**")
			b.list(false, 0, uc.Acceptance)
		}
	}
}

// ---------------------------------------------------------------------------
// 7 Modelagem do sistema
// ---------------------------------------------------------------------------

func (g *gen) modelingSection(b *builder) {
	b.section(secModeling)
	for _, kind := range g.modelingKinds() {
		title := modelingTitles[kind]
		b.heading(2, title, title, true)
		for _, d := range g.modeling[kind] {
			b.figure(UMLImageSrc(d.ID), d.Name, d.ID)
			if desc := strings.TrimSpace(d.Description); desc != "" {
				b.text(desc, "")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// 8 Arquitetura do sistema
// ---------------------------------------------------------------------------

func (g *gen) architecture(b *builder) {
	b.section(secArchitecture)
	if len(g.nodes) > 0 {
		b.para(fmt.Sprintf("Esta seção apresenta os componentes que compõem a arquitetura do %s, "+
			"suas tecnologias e responsabilidades.", g.project))
		b.figure(MacroImageSrc, "Arquitetura de componentes", MacroDiagramID)
		rows := [][]string{}
		for _, n := range g.nodes {
			rows = append(rows, []string{orDash(n.Data.Label), model.NodeTypeLabel(n.Type), orDash(n.Data.Technology), orDash(n.Data.Description)})
		}
		b.table([]string{"Componente", "Tipo", "Tecnologia", "Responsabilidade"}, rows)
	}

	if len(g.adrs) > 0 {
		b.heading(2, "Decisões arquiteturais", "Decisões arquiteturais", true)
		for _, adr := range g.adrs {
			title := strings.TrimSpace(adr.Title)
			b.heading(3, fmt.Sprintf("%s — %s", adr.ID, title), adr.ID+" "+title, false)
			b.para("**Status:** " + orDash(adr.Status))
			for _, part := range []struct{ label, text string }{
				{"Contexto", adr.Context}, {"Decisão", adr.Decision}, {"Consequências", adr.Consequences},
			} {
				if strings.TrimSpace(part.text) != "" && !isPlaceholder(part.text) {
					b.text(part.text, "**"+part.label+":** ")
				}
			}
		}
	}

	if len(g.eps) > 0 {
		b.heading(2, "Contratos de API", "Contratos de API", true)
		rows := [][]string{}
		for _, e := range g.eps {
			desc := e.Summary
			if strings.TrimSpace(desc) == "" {
				desc = e.Description
			}
			auth := strings.TrimSpace(e.Auth)
			if strings.EqualFold(auth, "none") {
				auth = "Nenhuma"
			}
			rows = append(rows, []string{strings.ToUpper(e.Method), "`" + e.Path + "`", orDash(desc), orDash(auth)})
		}
		b.table([]string{"Método", "Rota", "Descrição", "Autenticação"}, rows)
	}
}

// ---------------------------------------------------------------------------
// 9 Matriz de rastreabilidade
// ---------------------------------------------------------------------------

// componentsFor une os componentes declarados no requisito e os nós do
// diagrama que apontam para ele, pelos rótulos.
func (g *gen) componentsFor(r model.Requirement) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(id string) {
		label := id
		if g.in.Diagram != nil {
			if n := g.in.Diagram.ResolveNode(id); n != nil {
				label, id = n.Data.Label, n.ID
			}
		}
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, label)
	}
	for _, c := range r.Components {
		add(strings.TrimSpace(c))
	}
	for _, n := range g.nodes {
		if containsFold(n.Data.Requirements, r.ID) {
			add(n.ID)
		}
	}
	return out
}

func (g *gen) matrix(b *builder) {
	b.section(secMatrix)
	b.para("A matriz a seguir relaciona cada requisito aos casos de uso e aos componentes que o realizam.")
	rows := [][]string{}
	for _, r := range append(append([]model.Requirement{}, g.rf...), g.rnf...) {
		rows = append(rows, []string{
			fmt.Sprintf("[%s] %s", r.ID, strings.TrimSpace(r.Title)),
			orDash(refs(g.useCasesFor(r.ID))),
			orDash(strings.Join(g.componentsFor(r), ", ")),
		})
	}
	b.table([]string{"Requisito", "Casos de uso", "Componentes"}, rows)
}

// ---------------------------------------------------------------------------
// 10 Referências
// ---------------------------------------------------------------------------

func (g *gen) references(b *builder) {
	b.section(secReferences)
	items := []string{}
	for _, r := range g.meta.References {
		if r = strings.TrimSpace(r); r != "" {
			items = append(items, r)
		}
	}
	b.list(false, 0, items)
}
