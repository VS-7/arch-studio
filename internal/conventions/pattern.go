// Package conventions gera e valida nomes de branch, mensagens de commit,
// títulos e corpos de pull request, changelog e hooks a partir das
// convenções do projeto (.arch/conventions.yaml). É puro: recebe a
// configuração e o backlog, devolve texto.
package conventions

import (
	"regexp"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Padrões com marcadores
// ---------------------------------------------------------------------------

// Vars são os valores dos marcadores de um padrão.
type Vars struct {
	Sprint  int    // {sprint}, com dois dígitos
	ID      string // {id}
	Slug    string // {slug}
	Summary string // {summary}
	Title   string // {title}
	Kind    string // {kind}: Hotfix, Chore…
	Type    string // {type}: feat, fix…
	Version string // {version}
}

var reMarker = regexp.MustCompile(`\{(sprint|id|slug|summary|title|kind|type|version)\}`)

// Render substitui os marcadores do padrão.
func Render(pattern string, v Vars) string {
	return reMarker.ReplaceAllStringFunc(pattern, func(m string) string {
		switch m {
		case "{sprint}":
			return model.SprintLabel(v.Sprint)
		case "{id}":
			return v.ID
		case "{slug}":
			return v.Slug
		case "{summary}":
			return v.Summary
		case "{title}":
			return v.Title
		case "{kind}":
			return v.Kind
		case "{type}":
			return v.Type
		case "{version}":
			return v.Version
		}
		return m
	})
}

// Has informa se o padrão usa o marcador (ex.: "sprint").
func Has(pattern, marker string) bool { return strings.Contains(pattern, "{"+marker+"}") }

// ConventionalTypes são os tipos aceitos pelo preset Conventional Commits.
var ConventionalTypes = []string{"feat", "fix", "refactor", "test", "docs", "chore", "perf", "build", "ci", "style", "revert"}

// Compile transforma o padrão numa expressão regular ancorada, com grupos
// nomeados para os marcadores (só a primeira ocorrência de cada um captura).
func Compile(pattern string, c *model.Conventions) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	used := map[string]bool{}
	last := 0
	for _, loc := range reMarker.FindAllStringSubmatchIndex(pattern, -1) {
		b.WriteString(regexp.QuoteMeta(pattern[last:loc[0]]))
		name := pattern[loc[2]:loc[3]]
		expr := markerExpr(name, c)
		if used[name] {
			b.WriteString("(?:" + expr + ")")
		} else {
			b.WriteString("(?P<" + name + ">" + expr + ")")
			used[name] = true
		}
		last = loc[1]
	}
	b.WriteString(regexp.QuoteMeta(pattern[last:]))
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func markerExpr(name string, c *model.Conventions) string {
	switch name {
	case "sprint":
		return `\d{2,}`
	case "id":
		return `[A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+`
	case "slug":
		return `[a-z0-9][a-z0-9._-]*`
	case "summary", "title":
		return `\S.*?`
	case "kind":
		kinds := []string{}
		for _, k := range c.Git.OffSprintKinds {
			kinds = append(kinds, regexp.QuoteMeta(k))
		}
		if len(kinds) == 0 {
			return `[A-Z][a-z]+`
		}
		return "(?i:" + strings.Join(kinds, "|") + ")"
	case "type":
		return `(?:` + strings.Join(ConventionalTypes, "|") + `)(?:\([^)\s]+\))?!?`
	case "version":
		return `\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?`
	}
	return `.+?`
}

// Match aplica o padrão e devolve os valores capturados (nil se não casar).
func Match(pattern, text string, c *model.Conventions) map[string]string {
	re, err := Compile(pattern, c)
	if err != nil {
		return nil
	}
	m := re.FindStringSubmatch(text)
	if m == nil {
		return nil
	}
	out := map[string]string{}
	for i, name := range re.SubexpNames() {
		if name != "" {
			out[name] = m[i]
		}
	}
	return out
}

// ConventionalType traduz o tipo do item para o tipo do Conventional Commits.
func ConventionalType(itemType string) string {
	switch itemType {
	case model.ItemBug, model.ItemSecurity:
		return "fix"
	case model.ItemDebt:
		return "refactor"
	case model.ItemSpike:
		return "chore"
	default:
		return "feat"
	}
}
