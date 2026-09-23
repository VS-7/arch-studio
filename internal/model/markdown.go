package model

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Este arquivo implementa a serialização bidirecional determinística entre as
// estruturas em memória e os arquivos Markdown de docs/. O contrato é:
// Render(Parse(x)) == x para qualquer documento gerado pela ferramenta, de modo
// que edições humanas em VS Code e edições de IA convirjam sem conflito.

var (
	reMetaBullet = regexp.MustCompile(`^\s*[-*]\s+\*\*([^*:]+):?\*\*:?\s*(.*)$`)
	reHeading    = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	reReqHeading = regexp.MustCompile(`^(RF|RNF)\s*[-]?\s*(\d+)\s*(?:[—:–-]\s*)?(.*)$`)
	reCduHeading = regexp.MustCompile(`(?i)^(CDU\s*-?\s*\d+)\s*(?:[—:–-]\s*)?(.*)$`)
	reAdrHeading = regexp.MustCompile(`(?i)^(ADR\s*-?\s*\d+)\s*(?:[—:–-]\s*)?(.*)$`)
	reListItem   = regexp.MustCompile(`^\s*(?:[-*+]\s+|\d+[.)]\s+)(.*)$`)
)

// titleAfterSeparator extrai o texto após o primeiro separador de título
// ("—", "–", "-" ou ":"), tolerando separadores multibyte.
func titleAfterSeparator(text string) string {
	for _, sep := range []string{"—", "–", " - ", ":"} {
		if idx := strings.Index(text, sep); idx >= 0 {
			return strings.TrimSpace(text[idx+len(sep):])
		}
	}
	return strings.TrimSpace(text)
}

var reLeadingFloat = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)

// parseLeadingFloat aceita "16", "16h", "16,5 horas" e devolve 0 se nada casar.
func parseLeadingFloat(s string) float64 {
	m := reLeadingFloat.FindString(s)
	if m == "" {
		return 0
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	if err != nil {
		return 0
	}
	return f
}

func splitCSV(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		part = strings.Trim(part, "`")
		if part != "" && !strings.EqualFold(part, "n/a") && part != "-" {
			out = append(out, part)
		}
	}
	return out
}

func joinCSV(v []string) string {
	if len(v) == 0 {
		return "-"
	}
	return strings.Join(v, ", ")
}

func normalizeKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	k = strings.NewReplacer("á", "a", "ã", "a", "â", "a", "é", "e", "ê", "e",
		"í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c").Replace(k)
	k = strings.TrimSuffix(k, ":")
	return strings.TrimSpace(k)
}

// ---------------------------------------------------------------------------
// docs/requisitos.md
// ---------------------------------------------------------------------------

// ParseRequirements interpreta o documento de requisitos preservando a ordem e
// tolerando edições manuais (bullets fora de ordem, seções extras, etc.).
func ParseRequirements(src string) *RequirementsDoc {
	doc := &RequirementsDoc{Requirements: []Requirement{}}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	var (
		overview []string
		inOview  bool
		cur      *Requirement
		curBody  []string
	)

	flush := func() {
		if cur != nil {
			cur.Description = strings.TrimSpace(strings.Join(curBody, "\n"))
			doc.Requirements = append(doc.Requirements, *cur)
		}
		cur, curBody = nil, nil
	}

	for _, line := range lines {
		if m := reHeading.FindStringSubmatch(line); m != nil {
			level, text := len(m[1]), strings.TrimSpace(m[2])
			switch {
			case level == 1:
				flush()
				inOview = false
				doc.ProjectName = titleAfterSeparator(text)
			case level == 2:
				flush()
				inOview = strings.Contains(normalizeKey(text), "visao geral")
			case level >= 3:
				flush()
				inOview = false
				if rm := reReqHeading.FindStringSubmatch(text); rm != nil {
					n, _ := strconv.Atoi(rm[2])
					cur = &Requirement{
						ID:     fmt.Sprintf("%s%03d", rm[1], n),
						Type:   rm[1],
						Title:  strings.TrimSpace(rm[3]),
						Status: StatusPending,
					}
					curBody = nil
				}
			}
			continue
		}

		if inOview {
			overview = append(overview, line)
			continue
		}
		if cur == nil {
			continue
		}
		if m := reMetaBullet.FindStringSubmatch(line); m != nil {
			val := strings.TrimSpace(m[2])
			switch normalizeKey(m[1]) {
			case "prioridade", "priority":
				cur.Priority = val
			case "status":
				cur.Status = NormalizeStatus(val)
			case "componentes", "components", "nos", "nodes":
				cur.Components = splitCSV(val)
			case "tipo", "type":
				if t := strings.ToUpper(val); t == "RF" || t == "RNF" {
					cur.Type = t
				}
			case "categoria", "category":
				cur.Category = val
			case "requisitos associados", "related":
				cur.Related = splitCSV(val)
			default:
				curBody = append(curBody, line)
			}
			continue
		}
		curBody = append(curBody, line)
	}
	flush()

	doc.Overview = strings.TrimSpace(strings.Join(overview, "\n"))
	return doc
}

// RenderRequirements produz o Markdown canônico do documento de requisitos.
func RenderRequirements(doc *RequirementsDoc) string {
	var b strings.Builder
	name := doc.ProjectName
	if name == "" {
		name = "Projeto"
	}
	fmt.Fprintf(&b, "# Requisitos — %s\n\n", name)
	b.WriteString("> Documento gerenciado pelo ArchCode Studio. Editável por humanos e por agentes de IA via MCP.\n\n")

	b.WriteString("## Visão Geral\n\n")
	overview := strings.TrimSpace(doc.Overview)
	if overview == "" {
		overview = "_Descreva aqui o objetivo do sistema, o problema de negócio e o público-alvo._"
	}
	b.WriteString(overview)
	b.WriteString("\n\n")

	writeSection := func(title string, reqs []Requirement, empty string) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(reqs) == 0 {
			b.WriteString("_" + empty + "_\n\n")
			return
		}
		for _, r := range reqs {
			fmt.Fprintf(&b, "### %s — %s\n\n", r.ID, r.Title)
			priority := r.Priority
			if priority == "" {
				priority = "Média"
			}
			status := r.Status
			if status == "" {
				status = StatusPending
			}
			fmt.Fprintf(&b, "- **Prioridade:** %s\n", priority)
			fmt.Fprintf(&b, "- **Status:** %s\n", status)
			fmt.Fprintf(&b, "- **Componentes:** %s\n", joinCSV(r.Components))
			// Bullets opcionais só aparecem quando preenchidos, para que arquivos
			// antigos não mudem de forma ao serem regravados.
			if c := strings.TrimSpace(r.Category); c != "" {
				fmt.Fprintf(&b, "- **Categoria:** %s\n", c)
			}
			if len(r.Related) > 0 {
				fmt.Fprintf(&b, "- **Requisitos associados:** %s\n", strings.Join(r.Related, ", "))
			}
			b.WriteString("\n")
			if d := strings.TrimSpace(r.Description); d != "" {
				b.WriteString(d)
				b.WriteString("\n\n")
			}
		}
	}

	writeSection("Requisitos Funcionais", doc.Functional(), "Nenhum requisito funcional cadastrado.")
	writeSection("Requisitos Não Funcionais", doc.NonFunctional(), "Nenhum requisito não funcional cadastrado.")
	return b.String()
}

// ---------------------------------------------------------------------------
// docs/casos-de-uso/*.md
// ---------------------------------------------------------------------------

func normalizeCode(raw, prefix string) string {
	digits := regexp.MustCompile(`\d+`).FindString(raw)
	if digits == "" {
		return strings.ToUpper(strings.ReplaceAll(raw, " ", ""))
	}
	n, _ := strconv.Atoi(digits)
	return fmt.Sprintf("%s%03d", prefix, n)
}

func ParseUseCase(src string) *UseCase {
	uc := &UseCase{Status: StatusPending}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")

	section := ""
	inPreamble := true
	var description []string

	appendTo := func(target *[]string, item string) {
		item = strings.TrimSpace(item)
		// Itens inteiramente em itálico são os placeholders que o próprio
		// renderizador escreve para seções vazias ("_Nenhuma_", "_A definir._").
		// Sem este filtro, eles voltariam como dados na próxima leitura.
		if item == "" || isPlaceholder(item) {
			return
		}
		*target = append(*target, item)
	}

	for _, line := range lines {
		if m := reHeading.FindStringSubmatch(line); m != nil {
			level, text := len(m[1]), strings.TrimSpace(m[2])
			if level == 1 {
				if cm := reCduHeading.FindStringSubmatch(text); cm != nil {
					uc.Code = normalizeCode(cm[1], "CDU")
					uc.Name = strings.TrimSpace(cm[2])
				} else {
					uc.Name = text
				}
				inPreamble = true
				section = ""
				continue
			}
			inPreamble = false
			section = normalizeKey(text)
			continue
		}

		if inPreamble {
			if m := reMetaBullet.FindStringSubmatch(line); m != nil {
				val := strings.TrimSpace(m[2])
				switch normalizeKey(m[1]) {
				case "ator", "atores", "ator(es)", "actors":
					uc.Actors = splitCSV(val)
				case "componentes", "components":
					uc.Components = splitCSV(val)
				case "complexidade", "complexity":
					uc.Complexity = NormalizeComplexity(val)
				case "horas estimadas", "estimated hours", "horas":
					uc.EstimatedHours = parseLeadingFloat(val)
				case "prioridade", "priority":
					uc.Priority = val
				case "status":
					uc.Status = NormalizeStatus(val)
				case "requisitos", "requisitos associados", "requirements":
					uc.Requirements = splitCSV(val)
				}
			}
			continue
		}

		// A descrição é um parágrafo livre: as linhas são preservadas como estão.
		if section == "descricao" || section == "description" {
			description = append(description, line)
			continue
		}

		item := line
		isListItem := false
		if m := reListItem.FindStringSubmatch(line); m != nil {
			item, isListItem = m[1], true
		} else if strings.TrimSpace(line) == "" {
			continue
		}

		// Nos fluxos, uma linha inteira em negrito fora da lista é um subtítulo
		// de grupo ("**Cadastro de paciente:**" → "# Cadastro de paciente").
		flow := func(target *[]string) {
			if !isListItem {
				if m := reBoldLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
					if text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m[1]), ":")); text != "" {
						*target = append(*target, FlowSubtitlePrefix+text)
						return
					}
				}
			}
			appendTo(target, item)
		}

		switch {
		case strings.Contains(section, "pos-condi"), strings.Contains(section, "pos condi"),
			strings.Contains(section, "poscondi"), strings.Contains(section, "saidas"), strings.Contains(section, "post-cond"),
			strings.Contains(section, "postcond"), strings.Contains(section, "post cond"):
			appendTo(&uc.PostConditions, item)
		case strings.Contains(section, "pre-condi"), strings.Contains(section, "pre condi"), strings.Contains(section, "precondi"):
			appendTo(&uc.PreConditions, item)
		case strings.Contains(section, "fluxo principal"), strings.Contains(section, "main flow"):
			flow(&uc.MainFlow)
		case strings.Contains(section, "fluxos alternativos"), strings.Contains(section, "alternate"):
			flow(&uc.AlternateFlows)
		case strings.Contains(section, "excec"), strings.Contains(section, "exception"):
			flow(&uc.Exceptions)
		case strings.Contains(section, "regras"), strings.Contains(section, "business rules"):
			appendTo(&uc.BusinessRules, item)
		case strings.Contains(section, "aceite"), strings.Contains(section, "acceptance"):
			appendTo(&uc.Acceptance, item)
		}
	}
	uc.Description = strings.TrimSpace(strings.Join(description, "\n"))
	return uc
}

// reBoldLine casa uma linha inteira em negrito, com ou sem dois-pontos finais.
var reBoldLine = regexp.MustCompile(`^\*\*([^*]+)\*\*:?$`)

func RenderUseCase(uc *UseCase) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — %s\n\n", uc.Code, uc.Name)

	complexity := uc.Complexity
	if complexity == "" {
		complexity = "medium"
	}
	priority := uc.Priority
	if priority == "" {
		priority = "Média"
	}
	status := uc.Status
	if status == "" {
		status = StatusPending
	}
	fmt.Fprintf(&b, "- **Atores:** %s\n", joinCSV(uc.Actors))
	fmt.Fprintf(&b, "- **Componentes:** %s\n", joinCSV(uc.Components))
	fmt.Fprintf(&b, "- **Complexidade:** %s\n", complexity)
	fmt.Fprintf(&b, "- **Horas Estimadas:** %s\n", trimFloat(uc.EstimatedHours))
	fmt.Fprintf(&b, "- **Prioridade:** %s\n", priority)
	fmt.Fprintf(&b, "- **Status:** %s\n", status)
	if len(uc.Requirements) > 0 {
		fmt.Fprintf(&b, "- **Requisitos:** %s\n", strings.Join(uc.Requirements, ", "))
	}
	b.WriteString("\n")

	if d := strings.TrimSpace(uc.Description); d != "" {
		fmt.Fprintf(&b, "## Descrição\n\n%s\n\n", d)
	}

	// writeFlow escreve os itens de um fluxo. Subtítulos ("# X") viram uma linha
	// em negrito fora da lista, cercada de linhas em branco para não virar
	// continuação do item anterior; a numeração dos passos continua através deles.
	writeFlow := func(items []string, ordered bool) {
		step, inList := 0, false
		for _, it := range items {
			if text, ok := IsFlowSubtitle(it); ok {
				if inList {
					b.WriteString("\n")
				}
				fmt.Fprintf(&b, "**%s**\n\n", text)
				inList = false
				continue
			}
			step++
			if ordered {
				fmt.Fprintf(&b, "%d. %s\n", step, stripLeadingIndex(it))
			} else {
				fmt.Fprintf(&b, "- %s\n", it)
			}
			inList = true
		}
		if inList {
			b.WriteString("\n")
		}
	}
	numbered := func(title string, items []string) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(items) == 0 {
			b.WriteString("1. _A definir._\n\n")
			return
		}
		writeFlow(items, true)
	}
	bulleted := func(title string, items []string, empty string) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(items) == 0 {
			fmt.Fprintf(&b, "- _%s_\n\n", empty)
			return
		}
		for _, it := range items {
			fmt.Fprintf(&b, "- %s\n", it)
		}
		b.WriteString("\n")
	}
	flowSection := func(title string, items []string, empty string) {
		fmt.Fprintf(&b, "## %s\n\n", title)
		if len(items) == 0 {
			fmt.Fprintf(&b, "- _%s_\n\n", empty)
			return
		}
		writeFlow(items, false)
	}

	bulleted("Pré-condições", uc.PreConditions, "Nenhuma")
	if len(uc.PostConditions) > 0 {
		bulleted("Pós-condições", uc.PostConditions, "Nenhuma")
	}
	numbered("Fluxo Principal", uc.MainFlow)
	flowSection("Fluxos Alternativos", uc.AlternateFlows, "Nenhum")
	flowSection("Exceções", uc.Exceptions, "Nenhuma")
	bulleted("Regras de Negócio", uc.BusinessRules, "Nenhuma")
	bulleted("Critérios de Aceite", uc.Acceptance, "A definir (formato Given-When-Then)")
	return b.String()
}

var rePlaceholderItem = regexp.MustCompile(`^_[^_]*_$`)

// isPlaceholder reconhece o texto de preenchimento de uma seção vazia.
func isPlaceholder(item string) bool {
	return rePlaceholderItem.MatchString(strings.TrimSpace(item))
}

var reLeadingIndex = regexp.MustCompile(`^\s*\d+[.)]\s*`)

func stripLeadingIndex(s string) string { return reLeadingIndex.ReplaceAllString(s, "") }

func trimFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', 1, 64)
}

// UseCaseFileName gera o nome de arquivo canônico de um caso de uso.
func UseCaseFileName(uc *UseCase) string {
	slug := Slugify(uc.Name)
	if slug == "" {
		slug = "caso-de-uso"
	}
	return strings.ToLower(uc.Code) + "-" + slug + ".md"
}

// ---------------------------------------------------------------------------
// docs/architecture-decisions/*.md
// ---------------------------------------------------------------------------

func ParseADR(src string) *ADR {
	adr := &ADR{}
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	section := ""
	body := map[string][]string{}
	inPreamble := true

	for _, line := range lines {
		if m := reHeading.FindStringSubmatch(line); m != nil {
			level, text := len(m[1]), strings.TrimSpace(m[2])
			if level == 1 {
				if am := reAdrHeading.FindStringSubmatch(text); am != nil {
					adr.ID = normalizeCode(am[1], "ADR-")
					adr.Title = strings.TrimSpace(am[2])
				} else {
					adr.Title = text
				}
				inPreamble = true
				continue
			}
			inPreamble = false
			section = normalizeKey(text)
			continue
		}
		if inPreamble {
			if m := reMetaBullet.FindStringSubmatch(line); m != nil {
				val := strings.TrimSpace(m[2])
				switch normalizeKey(m[1]) {
				case "status":
					adr.Status = val
				case "data", "date":
					adr.Date = val
				}
			}
			continue
		}
		body[section] = append(body[section], line)
	}

	get := func(keys ...string) string {
		for k, v := range body {
			for _, want := range keys {
				if strings.Contains(k, want) {
					return strings.TrimSpace(strings.Join(v, "\n"))
				}
			}
		}
		return ""
	}
	adr.Context = get("contexto", "context")
	adr.Decision = get("decis", "decision")
	adr.Consequences = get("consequ")
	return adr
}

func RenderADR(adr *ADR) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s — %s\n\n", adr.ID, adr.Title)
	status := adr.Status
	if status == "" {
		status = "Proposto"
	}
	fmt.Fprintf(&b, "- **Status:** %s\n", status)
	fmt.Fprintf(&b, "- **Data:** %s\n\n", adr.Date)
	fmt.Fprintf(&b, "## Contexto\n\n%s\n\n", orPlaceholder(adr.Context, "_Qual problema ou força motivou esta decisão?_"))
	fmt.Fprintf(&b, "## Decisão\n\n%s\n\n", orPlaceholder(adr.Decision, "_O que foi decidido e por quê?_"))
	fmt.Fprintf(&b, "## Consequências\n\n%s\n", orPlaceholder(adr.Consequences, "_Impactos positivos, negativos e trade-offs aceitos._"))
	return b.String()
}

func orPlaceholder(s, placeholder string) string {
	if strings.TrimSpace(s) == "" {
		return placeholder
	}
	return strings.TrimSpace(s)
}

// ADRFileName gera o nome de arquivo canônico de um ADR.
func ADRFileName(adr *ADR) string {
	slug := Slugify(adr.Title)
	if slug == "" {
		slug = "decisao"
	}
	return strings.ToLower(adr.ID) + "-" + slug + ".md"
}
