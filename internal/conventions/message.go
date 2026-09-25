package conventions

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Resumo do commit a partir do título do item
// ---------------------------------------------------------------------------

// irregulares mais comuns no presente da 3ª pessoa.
var irregularVerbs = map[string]string{
	"fazer": "faz", "ver": "vê", "ter": "tem", "pôr": "põe", "trazer": "traz", "dizer": "diz",
	"ler": "lê", "ir": "vai", "vir": "vem", "dar": "dá", "estar": "está", "ser": "é", "poder": "pode",
	"construir": "constrói", "destruir": "destrói", "refazer": "refaz", "desfazer": "desfaz",
}

// nonVerbs são palavras terminadas em -ar/-er/-ir que não são verbos e
// aparecem no começo de títulos.
var nonVerbs = map[string]bool{"par": true, "lar": true, "bar": true, "mar": true, "ar": true, "altar": true, "radar": true}

// ThirdPerson conjuga um verbo no infinitivo no presente da 3ª pessoa
// ("Implementar" → "Implementa", "Corrigir" → "Corrige"). ok = false quando a
// palavra não parece um infinitivo.
func ThirdPerson(word string) (string, bool) {
	lower := strings.ToLower(word)
	if nonVerbs[lower] || utf8.RuneCountInString(lower) < 3 {
		return word, false
	}
	conj := ""
	if irr, ok := irregularVerbs[lower]; ok {
		conj = irr
	} else {
		switch {
		case strings.HasSuffix(lower, "uir"):
			conj = strings.TrimSuffix(lower, "r")
		case strings.HasSuffix(lower, "ar"):
			conj = strings.TrimSuffix(lower, "r")
		case strings.HasSuffix(lower, "er"), strings.HasSuffix(lower, "ir"):
			conj = lower[:len(lower)-2] + "e"
		default:
			return word, false
		}
	}
	return matchCase(word, conj), true
}

// matchCase aplica a caixa da primeira letra de ref em s.
func matchCase(ref, s string) string {
	r, _ := utf8.DecodeRuneInString(ref)
	if unicode.IsUpper(r) {
		return capitalize(s)
	}
	return s
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}

func lowerFirst(s string) string {
	runes := []rune(s)
	if len(runes) < 2 || !unicode.IsUpper(runes[0]) || unicode.IsUpper(runes[1]) {
		return s
	}
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

// DefaultVerb é o verbo usado quando o título não começa com um.
func DefaultVerb(itemType string) string {
	switch itemType {
	case model.ItemBug:
		return "Corrige"
	case model.ItemDebt:
		return "Refatora"
	case model.ItemSpike:
		return "Investiga"
	case model.ItemSecurity:
		return "Protege"
	default:
		return "Implementa"
	}
}

// Summarize transforma o título do item no resumo do commit: verbo no
// presente da 3ª pessoa no início ("Implementar Core API" → "Implementa Core
// API"; "Login com e-mail" → "Implementa login com e-mail"). No preset
// Conventional Commits, o resumo começa em minúscula.
func Summarize(title, itemType string, c *model.Conventions) string {
	title = strings.Join(strings.Fields(title), " ")
	if title == "" {
		return DefaultVerb(itemType)
	}
	first, rest, _ := strings.Cut(title, " ")
	summary := ""
	switch {
	case isAllowedVerb(first, c):
		summary = title
	default:
		if conj, ok := ThirdPerson(first); ok {
			summary = strings.TrimSpace(conj + " " + rest)
		} else {
			summary = DefaultVerb(itemType) + " " + lowerFirst(title)
		}
	}
	if c.Preset == model.PresetConventional {
		summary = lowerFirst(summary)
	}
	return summary
}

func isAllowedVerb(word string, c *model.Conventions) bool {
	for _, v := range c.Git.Verbs {
		if strings.EqualFold(v, word) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Branch, commit e pull request
// ---------------------------------------------------------------------------

// maxSlug limita o trecho descritivo do nome da branch.
const maxSlug = 40

// Slug encurta o título para nomes de branch.
func Slug(title string) string {
	s := model.Slugify(title)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if len(s) > maxSlug {
		cut := strings.LastIndex(s[:maxSlug], "-")
		if cut < 10 {
			cut = maxSlug
		}
		s = strings.Trim(s[:cut], "-")
	}
	if s == "" {
		s = "trabalho"
	}
	return s
}

// OffSprintKind escolhe o tipo de commit fora de sprint pelo tipo do item
// (Hotfix para bug e segurança, Chore para o resto), dentro dos permitidos.
func OffSprintKind(itemType string, c *model.Conventions) string {
	want := "Chore"
	if itemType == model.ItemBug || itemType == model.ItemSecurity {
		want = "Hotfix"
	}
	for _, k := range c.Git.OffSprintKinds {
		if strings.EqualFold(k, want) {
			return k
		}
	}
	if len(c.Git.OffSprintKinds) > 0 {
		return c.Git.OffSprintKinds[0]
	}
	return want
}

// usesSprint informa se o item entra no padrão de sprint: tem sprint e o
// padrão pede uma.
func usesSprint(pattern string, sprint int) bool { return !Has(pattern, "sprint") || sprint > 0 }

// BranchName devolve o nome da branch do item.
func BranchName(c *model.Conventions, item *model.WorkItem, sprint int) string {
	v := Vars{Sprint: sprint, ID: item.ID, Slug: Slug(item.Title), Type: ConventionalType(item.Type),
		Kind: strings.ToLower(OffSprintKind(item.Type, c))}
	pattern := c.Git.Branch
	if !usesSprint(pattern, sprint) {
		pattern = c.Git.BranchOffSprint
	}
	return Render(pattern, v)
}

// CommitInput descreve o commit a gerar.
type CommitInput struct {
	Item    *model.WorkItem
	Sprint  int
	Summary string // "" = gerado a partir do título
	Body    string
	// Story e Refs entram como trailers.
	Story    string
	CoAuthor string
}

// CommitSubject devolve a primeira linha da mensagem, respeitando o limite
// de caracteres (o resumo é encurtado com "…" se preciso).
func CommitSubject(c *model.Conventions, in CommitInput) string {
	summary := strings.TrimSpace(in.Summary)
	if summary == "" {
		summary = Summarize(in.Item.Title, in.Item.Type, c)
	}
	pattern := c.Git.Commit
	if !usesSprint(pattern, in.Sprint) {
		pattern = c.Git.CommitOffSprint
	}
	v := Vars{Sprint: in.Sprint, ID: in.Item.ID, Title: in.Item.Title, Type: ConventionalType(in.Item.Type),
		Kind: OffSprintKind(in.Item.Type, c)}
	v.Summary = summary
	subject := Render(pattern, v)
	if max := c.Git.MaxSubject; max > 0 && utf8.RuneCountInString(subject) > max {
		over := utf8.RuneCountInString(subject) - max
		runes := []rune(summary)
		keep := len(runes) - over - 1
		if keep < 8 {
			keep = 8
		}
		if keep < len(runes) {
			short := strings.TrimSpace(string(runes[:keep]))
			if i := strings.LastIndex(short, " "); i > len(short)/2 {
				short = short[:i]
			}
			v.Summary = short + "…"
			subject = Render(pattern, v)
		}
	}
	return subject
}

// CommitMessage devolve a mensagem completa: assunto, corpo opcional e
// trailers (Task, Story, Refs, Co-Authored-By).
func CommitMessage(c *model.Conventions, in CommitInput) string {
	var b strings.Builder
	b.WriteString(CommitSubject(c, in))
	if body := strings.TrimSpace(in.Body); body != "" {
		b.WriteString("\n\n" + body)
	}
	trailers := []string{"Task: " + in.Item.ID}
	story := in.Story
	if story == "" && strings.HasPrefix(in.Item.Parent, "ST-") {
		story = in.Item.Parent
	}
	if story != "" {
		trailers = append(trailers, "Story: "+story)
	}
	refs := append(append([]string{}, in.Item.Requirements...), in.Item.UseCases...)
	if len(refs) > 0 {
		trailers = append(trailers, "Refs: "+strings.Join(refs, ", "))
	}
	if in.CoAuthor != "" {
		trailers = append(trailers, "Co-Authored-By: "+in.CoAuthor)
	}
	b.WriteString("\n\n" + strings.Join(trailers, "\n") + "\n")
	return b.String()
}

// PRInput descreve o pull request a gerar.
type PRInput struct {
	Items  []*model.WorkItem
	Sprint *model.Sprint
	// Title substitui o título do item principal (ex.: o da história).
	Title string
	// Validation é o resumo do `validate` (opcional).
	Validation string
	// Skills lista as skills ativas que entram no checklist.
	Skills []SkillCheck
}

// SkillCheck é uma verificação obrigatória de uma skill para o checklist.
type SkillCheck struct {
	Skill string
	Check string
}

// PRTitle devolve o título do pull request.
func PRTitle(c *model.Conventions, in PRInput) string {
	if len(in.Items) == 0 {
		return ""
	}
	main := in.Items[0]
	sprint := 0
	if in.Sprint != nil {
		sprint = in.Sprint.Number
	} else {
		sprint = main.Sprint
	}
	title := in.Title
	if title == "" {
		title = main.Title
	}
	id := main.ID
	if in.Title != "" && strings.HasPrefix(main.Parent, "ST-") && c.PullRequest.Granularity == "story" {
		id = main.Parent
	}
	pattern := c.PullRequest.Title
	if !usesSprint(pattern, sprint) {
		pattern = strings.Replace(c.Git.CommitOffSprint, "{summary}", "{title}", 1)
	}
	return Render(pattern, Vars{Sprint: sprint, ID: id, Title: title, Type: ConventionalType(main.Type),
		Kind: OffSprintKind(main.Type, c)})
}

// PRBody devolve o corpo do pull request em Markdown.
func PRBody(c *model.Conventions, in PRInput) string {
	var b strings.Builder
	if sp := in.Sprint; sp != nil {
		fmt.Fprintf(&b, "## %s", sp.Name)
		if sp.Goal != "" {
			fmt.Fprintf(&b, " — %s", sp.Goal)
		}
		b.WriteString("\n\n")
	}
	ids := []string{}
	for _, it := range in.Items {
		ids = append(ids, it.ID)
	}
	fmt.Fprintf(&b, "Fecha: %s\n\n", strings.Join(ids, ", "))

	b.WriteString("### Itens\n\n")
	components, endpoints := []string{}, []string{}
	for _, it := range in.Items {
		refs := append(append([]string{}, it.Requirements...), it.UseCases...)
		line := fmt.Sprintf("- **%s** %s", it.ID, it.Title)
		if len(refs) > 0 {
			line += " (" + strings.Join(refs, " · ") + ")"
		}
		b.WriteString(line + "\n")
		if it.Component != "" {
			components = appendUnique(components, it.Component)
		}
		for _, e := range it.Endpoints {
			endpoints = appendUnique(endpoints, e)
		}
	}

	criteria := 0
	for _, it := range in.Items {
		criteria += len(it.Acceptance)
	}
	if criteria > 0 {
		b.WriteString("\n### Critérios de aceite\n\n")
		for _, it := range in.Items {
			for _, cr := range it.Acceptance {
				mark := " "
				if cr.Done {
					mark = "x"
				}
				prefix := ""
				if len(in.Items) > 1 {
					prefix = it.ID + ": "
				}
				fmt.Fprintf(&b, "- [%s] %s%s\n", mark, prefix, cr.Text)
			}
		}
	}

	reported := []model.CheckResult{}
	for _, it := range in.Items {
		reported = append(reported, it.Checks...)
	}
	if len(reported) > 0 || len(in.Skills) > 0 {
		b.WriteString("\n### Checks das skills\n\n")
		seen := map[string]bool{}
		for _, ch := range reported {
			key := ch.Skill + "|" + ch.Check
			seen[key] = true
			mark := map[string]string{model.CheckOK: "x", model.CheckNA: "x", model.CheckFail: " "}[ch.Result]
			line := fmt.Sprintf("- [%s] %s · %s", mark, ch.Skill, ch.Check)
			if ch.Result == model.CheckNA {
				line += " (não se aplica)"
			}
			if ch.Evidence != "" {
				line += " — " + ch.Evidence
			}
			b.WriteString(line + "\n")
		}
		for _, sc := range in.Skills {
			if !seen[sc.Skill+"|"+sc.Check] {
				fmt.Fprintf(&b, "- [ ] %s · %s\n", sc.Skill, sc.Check)
			}
		}
	}

	b.WriteString("\n### Como testar\n\n")
	wrote := false
	for _, it := range in.Items {
		for _, ch := range it.Checks {
			if ch.Result == model.CheckOK && strings.Contains(strings.ToLower(ch.Check+" "+ch.Skill), "test") && ch.Evidence != "" {
				fmt.Fprintf(&b, "- %s\n", ch.Evidence)
				wrote = true
			}
		}
	}
	if !wrote {
		b.WriteString("<!-- Comandos e passos para validar a mudança. -->\n")
	}

	b.WriteString("\n### Impacto na arquitetura\n\n")
	if len(components) > 0 {
		fmt.Fprintf(&b, "- Componentes: %s\n", strings.Join(components, ", "))
	}
	if len(endpoints) > 0 {
		fmt.Fprintf(&b, "- Endpoints: %s\n", strings.Join(endpoints, ", "))
	}
	if in.Validation != "" {
		fmt.Fprintf(&b, "- `archcode-studio validate`: %s\n", in.Validation)
	}
	if len(components) == 0 && len(endpoints) == 0 && in.Validation == "" {
		b.WriteString("- Nenhum componente do diagrama alterado.\n")
	}
	fmt.Fprintf(&b, "\n---\nMerge: %s. Gerado pelo ArchCode Studio.\n", mergeLabel(c.PullRequest.MergeStrategy))
	return b.String()
}

func mergeLabel(strategy string) string {
	switch strategy {
	case "squash":
		return "squash (uma linha por item na branch principal)"
	case "rebase":
		return "rebase"
	default:
		return strategy
	}
}

func appendUnique(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}
