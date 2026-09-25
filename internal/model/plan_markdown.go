package model

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ---------------------------------------------------------------------------
// Frontmatter YAML + corpo Markdown
// ---------------------------------------------------------------------------
//
// Itens do backlog, sessões e memórias usam o mesmo formato: um bloco
// "---\n<yaml>\n---" seguido de seções "## Título". O frontmatter guarda os
// campos estruturados; as seções guardam o texto que as pessoas leem no diff.

// SplitFrontmatter separa o frontmatter do corpo. ok = false quando o texto
// não começa com "---".
func SplitFrontmatter(src string) (fm, body string, ok bool) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = strings.TrimPrefix(src, "\ufeff")
	if !strings.HasPrefix(src, "---\n") {
		return "", src, false
	}
	rest := src[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", src, false
	}
	fm = rest[:end]
	body = rest[end+4:]
	// Descarta o restante da linha de fechamento ("---" pode vir seguido de \n).
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		body = body[i+1:]
	} else {
		body = ""
	}
	return fm, body, true
}

// HasConflictMarkers informa se o texto tem marcadores de conflito do Git.
func HasConflictMarkers(src string) bool {
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(line, "<<<<<<< ") || strings.HasPrefix(line, ">>>>>>> ") || line == "=======" {
			return true
		}
	}
	return false
}

// RenderFrontmatter serializa v como frontmatter YAML (com os delimitadores).
func RenderFrontmatter(v any) (string, error) {
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return "---\n" + sb.String() + "---\n", nil
}

// mdSection é uma seção "## Título" do corpo.
type mdSection struct {
	Key     string // título normalizado (sem acento, minúsculo)
	Heading string // título como escrito
	Lines   []string
}

func (s mdSection) text() string { return strings.TrimSpace(strings.Join(s.Lines, "\n")) }

// parseSections divide o corpo em seções de nível 2. O que vem antes da
// primeira seção (normalmente o "# título") é ignorado.
func parseSections(body string) []mdSection {
	out := []mdSection{}
	var cur *mdSection
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(line, "## ") {
			heading := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			out = append(out, mdSection{Key: normalizeKey(heading), Heading: heading})
			cur = &out[len(out)-1]
			continue
		}
		if cur != nil {
			cur.Lines = append(cur.Lines, line)
		}
	}
	return out
}

// bulletItems devolve os itens de uma lista com marcadores ("- " ou "* ").
// Linhas que não são itens continuam o item anterior.
func bulletItems(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		t := strings.TrimSpace(line)
		switch {
		case t == "":
			continue
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			out = append(out, strings.TrimSpace(t[2:]))
		case len(out) > 0:
			out[len(out)-1] += " " + t
		default:
			out = append(out, t)
		}
	}
	return out
}

// parseCheckbox interpreta "[x] texto" / "[ ] texto".
func parseCheckbox(item string) (Criterion, bool) {
	if len(item) >= 3 && item[0] == '[' && item[2] == ']' {
		done := item[1] == 'x' || item[1] == 'X'
		return Criterion{Text: strings.TrimSpace(item[3:]), Done: done}, true
	}
	return Criterion{Text: item}, false
}

// keyValue interpreta "Chave: valor".
func keyValue(item string) (string, string) {
	k, v, ok := strings.Cut(item, ":")
	if !ok {
		return "", strings.TrimSpace(item)
	}
	return normalizeKey(k), strings.TrimSpace(v)
}

func writeBullets(b *strings.Builder, items []string) {
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", oneLineText(it))
	}
}

// oneLineText colapsa quebras de linha para caber num item de lista.
func oneLineText(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
}

// ---------------------------------------------------------------------------
// Itens do backlog
// ---------------------------------------------------------------------------

// RenderWorkItem gera o arquivo Markdown do item.
func RenderWorkItem(w *WorkItem) (string, error) {
	fm, err := RenderFrontmatter(w)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(fm)
	fmt.Fprintf(&b, "\n# %s · %s\n", w.ID, oneLineText(w.Title))

	if d := strings.TrimSpace(w.Description); d != "" {
		fmt.Fprintf(&b, "\n## Descrição\n\n%s\n", d)
	}
	if len(w.Acceptance) > 0 {
		b.WriteString("\n## Critérios de aceite\n\n")
		for _, c := range w.Acceptance {
			mark := " "
			if c.Done {
				mark = "x"
			}
			fmt.Fprintf(&b, "- [%s] %s\n", mark, oneLineText(c.Text))
		}
	}
	if h := w.Handoff; !h.Empty() {
		b.WriteString("\n## Handoff\n\n")
		if h.LastStep != "" {
			fmt.Fprintf(&b, "- Último passo: %s\n", oneLineText(h.LastStep))
		}
		if h.NextStep != "" {
			fmt.Fprintf(&b, "- Próximo passo: %s\n", oneLineText(h.NextStep))
		}
		if len(h.Files) > 0 {
			fmt.Fprintf(&b, "- Arquivos: %s\n", strings.Join(h.Files, ", "))
		}
		if len(h.FailingTests) > 0 {
			fmt.Fprintf(&b, "- Testes falhando: %s\n", strings.Join(h.FailingTests, ", "))
		}
		if h.Notes != "" {
			fmt.Fprintf(&b, "- Observações: %s\n", oneLineText(h.Notes))
		}
		if h.UpdatedAt != "" || h.By != "" {
			line := "- Atualizado:"
			if h.UpdatedAt != "" {
				line += " " + h.UpdatedAt
			}
			if h.By != "" {
				line += " por " + h.By
			}
			b.WriteString(line + "\n")
		}
	}
	if len(w.Checks) > 0 {
		b.WriteString("\n## Checks\n\n")
		for _, c := range w.Checks {
			line := fmt.Sprintf("- [%s] %s · %s", c.Result, c.Skill, oneLineText(c.Check))
			if c.Evidence != "" {
				line += " — " + oneLineText(c.Evidence)
			}
			b.WriteString(line + "\n")
		}
	}
	if n := strings.TrimSpace(w.Notes); n != "" {
		fmt.Fprintf(&b, "\n## Notas\n\n%s\n", n)
	}
	return b.String(), nil
}

// ParseWorkItem lê o arquivo Markdown de um item. Seções desconhecidas vão
// para as notas, para que nada escrito à mão se perca.
func ParseWorkItem(src string) (*WorkItem, error) {
	if HasConflictMarkers(src) {
		return nil, fmt.Errorf("arquivo com marcadores de conflito do Git")
	}
	fm, body, ok := SplitFrontmatter(src)
	if !ok {
		return nil, fmt.Errorf("frontmatter YAML ausente")
	}
	w := &WorkItem{}
	if err := yaml.Unmarshal([]byte(fm), w); err != nil {
		return nil, fmt.Errorf("frontmatter inválido: %w", err)
	}
	w.ID = strings.ToUpper(strings.TrimSpace(w.ID))
	if w.ID == "" {
		return nil, fmt.Errorf("item sem id")
	}
	if !ValidItemType(w.Type) {
		w.Type = NormalizeItemType(w.Type)
	}
	w.Status = NormalizeStatus(w.Status)

	extra := []string{}
	for _, sec := range parseSections(body) {
		switch sec.Key {
		case "descricao":
			w.Description = sec.text()
		case "criterios de aceite", "criterios":
			for _, it := range bulletItems(sec.Lines) {
				c, _ := parseCheckbox(it)
				if c.Text != "" {
					w.Acceptance = append(w.Acceptance, c)
				}
			}
		case "handoff", "checkpoint":
			h := &Handoff{}
			for _, it := range bulletItems(sec.Lines) {
				k, v := keyValue(it)
				switch k {
				case "ultimo passo":
					h.LastStep = v
				case "proximo passo":
					h.NextStep = v
				case "arquivos":
					h.Files = splitCSV(v)
				case "testes falhando":
					h.FailingTests = splitCSV(v)
				case "observacoes", "notas":
					h.Notes = v
				case "atualizado":
					at, by, _ := strings.Cut(v, " por ")
					h.UpdatedAt, h.By = strings.TrimSpace(at), strings.TrimSpace(by)
				}
			}
			if !h.Empty() || h.UpdatedAt != "" {
				w.Handoff = h
			}
		case "checks":
			for _, it := range bulletItems(sec.Lines) {
				if c, ok := parseCheckLine(it); ok {
					w.Checks = append(w.Checks, c)
				}
			}
		case "notas":
			extra = append(extra, sec.text())
		default:
			if t := sec.text(); t != "" {
				extra = append(extra, "### "+sec.Heading+"\n\n"+t)
			}
		}
	}
	w.Notes = strings.TrimSpace(strings.Join(extra, "\n\n"))
	return w, nil
}

// parseCheckLine interpreta "[ok] skill · check — evidência".
func parseCheckLine(item string) (CheckResult, bool) {
	if !strings.HasPrefix(item, "[") {
		return CheckResult{}, false
	}
	end := strings.Index(item, "]")
	if end < 0 {
		return CheckResult{}, false
	}
	res := CheckResult{Result: NormalizeCheckResult(item[1:end])}
	rest := strings.TrimSpace(item[end+1:])
	if body, evidence, ok := strings.Cut(rest, " — "); ok {
		rest, res.Evidence = body, strings.TrimSpace(evidence)
	}
	if skill, check, ok := strings.Cut(rest, " · "); ok {
		res.Skill, res.Check = strings.TrimSpace(skill), strings.TrimSpace(check)
	} else {
		res.Check = rest
	}
	return res, res.Check != ""
}

// WorkItemFileName devolve o nome do arquivo do item.
func WorkItemFileName(id string) string { return id + ".md" }

// ---------------------------------------------------------------------------
// Sessões de trabalho (.arch/sessions/)
// ---------------------------------------------------------------------------

// Session é o diário de uma sessão de trabalho, humana ou de um agente (RF050).
type Session struct {
	ID      string   `yaml:"id" json:"id"`
	Author  string   `yaml:"author,omitempty" json:"author,omitempty"`
	Agent   string   `yaml:"agent,omitempty" json:"agent,omitempty"`
	Started string   `yaml:"started,omitempty" json:"started,omitempty"`
	Ended   string   `yaml:"ended,omitempty" json:"ended,omitempty"`
	Sprint  int      `yaml:"sprint,omitempty" json:"sprint,omitempty"`
	Tasks   []string `yaml:"tasks,omitempty" json:"tasks,omitempty"`
	Branch  string   `yaml:"branch,omitempty" json:"branch,omitempty"`
	Commits []string `yaml:"commits,omitempty" json:"commits,omitempty"`

	Summary   string        `yaml:"-" json:"summary,omitempty"`
	Done      []string      `yaml:"-" json:"done,omitempty"`
	Decisions []string      `yaml:"-" json:"decisions,omitempty"`
	NextSteps []string      `yaml:"-" json:"next_steps,omitempty"`
	Blockers  []string      `yaml:"-" json:"blockers,omitempty"`
	Files     []string      `yaml:"-" json:"files,omitempty"`
	Commands  []string      `yaml:"-" json:"commands,omitempty"`
	Checks    []CheckResult `yaml:"-" json:"checks,omitempty"`
	File      string        `yaml:"-" json:"file,omitempty"`
}

// RenderSession gera o arquivo Markdown da sessão.
func RenderSession(s *Session) (string, error) {
	fm, err := RenderFrontmatter(s)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString(fm)
	title := "Sessão"
	if s.Author != "" {
		title += " de " + s.Author
	}
	if s.Agent != "" {
		title += " (" + s.Agent + ")"
	}
	fmt.Fprintf(&b, "\n# %s\n", title)
	if t := strings.TrimSpace(s.Summary); t != "" {
		fmt.Fprintf(&b, "\n## Resumo\n\n%s\n", t)
	}
	lists := []struct {
		heading string
		items   []string
	}{
		{"Feito", s.Done}, {"Decisões", s.Decisions}, {"Próximos passos", s.NextSteps},
		{"Bloqueios", s.Blockers}, {"Arquivos", s.Files}, {"Comandos", s.Commands},
	}
	for _, l := range lists {
		if len(l.items) > 0 {
			fmt.Fprintf(&b, "\n## %s\n\n", l.heading)
			writeBullets(&b, l.items)
		}
	}
	if len(s.Checks) > 0 {
		b.WriteString("\n## Checks\n\n")
		for _, c := range s.Checks {
			line := fmt.Sprintf("- [%s] %s · %s", c.Result, c.Skill, oneLineText(c.Check))
			if c.Evidence != "" {
				line += " — " + oneLineText(c.Evidence)
			}
			b.WriteString(line + "\n")
		}
	}
	return b.String(), nil
}

// ParseSession lê o arquivo de uma sessão.
func ParseSession(src string) (*Session, error) {
	fm, body, ok := SplitFrontmatter(src)
	if !ok {
		return nil, fmt.Errorf("frontmatter YAML ausente")
	}
	s := &Session{}
	if err := yaml.Unmarshal([]byte(fm), s); err != nil {
		return nil, fmt.Errorf("frontmatter inválido: %w", err)
	}
	for _, sec := range parseSections(body) {
		switch sec.Key {
		case "resumo":
			s.Summary = sec.text()
		case "feito":
			s.Done = bulletItems(sec.Lines)
		case "decisoes":
			s.Decisions = bulletItems(sec.Lines)
		case "proximos passos", "pendencias":
			s.NextSteps = bulletItems(sec.Lines)
		case "bloqueios":
			s.Blockers = bulletItems(sec.Lines)
		case "arquivos":
			s.Files = bulletItems(sec.Lines)
		case "comandos":
			s.Commands = bulletItems(sec.Lines)
		case "checks":
			for _, it := range bulletItems(sec.Lines) {
				if c, ok := parseCheckLine(it); ok {
					s.Checks = append(s.Checks, c)
				}
			}
		}
	}
	return s, nil
}

// ---------------------------------------------------------------------------
// Memórias do projeto (.arch/memory/)
// ---------------------------------------------------------------------------

// Tipos de memória.
var NoteTypes = []string{"decisao", "convencao", "armadilha", "contexto", "glossario"}

// NormalizeNoteType aceita sinônimos; desconhecido vira "contexto".
func NormalizeNoteType(t string) string {
	switch normalizeKey(t) {
	case "decisao", "decision":
		return "decisao"
	case "convencao", "convention", "padrao":
		return "convencao"
	case "armadilha", "gotcha", "cuidado", "pitfall":
		return "armadilha"
	case "glossario", "glossary", "termo":
		return "glossario"
	default:
		return "contexto"
	}
}

// Note é um fato durável sobre o projeto (decisão pequena, convenção,
// armadilha, contexto de negócio ou termo do glossário) (RF052).
type Note struct {
	Slug       string   `yaml:"-" json:"slug"`
	Title      string   `yaml:"title" json:"title"`
	Type       string   `yaml:"type" json:"type"`
	Tags       []string `yaml:"tags,omitempty" json:"tags,omitempty"`
	Components []string `yaml:"components,omitempty" json:"components,omitempty"`
	Author     string   `yaml:"author,omitempty" json:"author,omitempty"`
	Created    string   `yaml:"created,omitempty" json:"created,omitempty"`
	Updated    string   `yaml:"updated,omitempty" json:"updated,omitempty"`
	Body       string   `yaml:"-" json:"body"`
	File       string   `yaml:"-" json:"file,omitempty"`
}

// RenderNote gera o arquivo Markdown da memória.
func RenderNote(n *Note) (string, error) {
	fm, err := RenderFrontmatter(n)
	if err != nil {
		return "", err
	}
	return fm + "\n" + strings.TrimSpace(n.Body) + "\n", nil
}

// ParseNote lê o arquivo de uma memória.
func ParseNote(src string) (*Note, error) {
	fm, body, ok := SplitFrontmatter(src)
	if !ok {
		return nil, fmt.Errorf("frontmatter YAML ausente")
	}
	n := &Note{}
	if err := yaml.Unmarshal([]byte(fm), n); err != nil {
		return nil, fmt.Errorf("frontmatter inválido: %w", err)
	}
	n.Type = NormalizeNoteType(n.Type)
	n.Body = strings.TrimSpace(body)
	return n, nil
}

// NoteSummary devolve a primeira linha útil do corpo, para o índice.
func NoteSummary(n *Note) string {
	for _, line := range strings.Split(n.Body, "\n") {
		t := strings.TrimSpace(strings.TrimLeft(line, "#>-* "))
		if t != "" {
			if len([]rune(t)) > 140 {
				t = string([]rune(t)[:139]) + "…"
			}
			return t
		}
	}
	return ""
}
