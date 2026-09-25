// Package skills cuida das skills do projeto: pacotes de instruções que dizem
// ao agente de IA como trabalhar ali (segurança, organização, padronização e
// uso do próprio Studio).
//
// A fonte é .arch/skills/<nome>/SKILL.md, no formato Agent Skills (o mesmo do
// Claude Code) com campos extras que só o Studio lê: categoria, quando vale,
// em que momento carregar e os checks que entram na definição de pronto. Este
// pacote é puro: lê e gera texto; quem grava é o pacote app.
package skills

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/archcode/studio/internal/model"
)

//go:embed catalog/*/SKILL.md
var catalogFS embed.FS

// Categorias e momentos de carga.
const (
	CategorySecurity     = "seguranca"
	CategoryOrganization = "organizacao"
	CategoryStandards    = "padronizacao"
	CategoryStudio       = "studio"

	// Escopos: os checks de skills de tarefa entram na conclusão da tarefa;
	// os de modelagem e de sprint valem para essas atividades.
	ScopeTask     = "tarefa"
	ScopeModeling = "modelagem"
	ScopeSprint   = "sprint"

	TriggerAlways     = "sempre"
	TriggerTaskStart  = "ao_iniciar_tarefa"
	TriggerBeforePR   = "antes_do_pr"
	SourceGenerated   = "gerado"
	SourceProject     = "projeto"
	sourceBuiltinPref = "builtin@"
)

// CategoryLabels são os nomes legíveis das categorias.
var CategoryLabels = map[string]string{
	CategorySecurity: "Segurança", CategoryOrganization: "Organização",
	CategoryStandards: "Padronização", CategoryStudio: "Uso do Studio",
}

// TriggerLabels são os nomes legíveis dos momentos de carga.
var TriggerLabels = map[string]string{
	TriggerAlways: "sempre", TriggerTaskStart: "ao iniciar a tarefa", TriggerBeforePR: "antes do PR",
}

// Check é uma verificação objetiva da skill.
type Check struct {
	ID       string `yaml:"id" json:"id"`
	Text     string `yaml:"text" json:"text"`
	Command  string `yaml:"command,omitempty" json:"command,omitempty"`
	Required bool   `yaml:"required" json:"required"`
}

// AppliesTo limita a skill a tiers da arquitetura e stacks do projeto.
type AppliesTo struct {
	Tiers  []string `yaml:"tiers,omitempty" json:"tiers,omitempty"`
	Stacks []string `yaml:"stacks,omitempty" json:"stacks,omitempty"`
}

// Skill é uma skill do catálogo ou do projeto.
type Skill struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Category    string `yaml:"category" json:"category"`
	Version     string `yaml:"version,omitempty" json:"version,omitempty"`
	Trigger     string `yaml:"trigger" json:"trigger"`
	// Scope: tarefa (padrão), modelagem ou sprint. Só os checks de skills de
	// tarefa são exigidos por complete_task.
	Scope     string     `yaml:"scope,omitempty" json:"scope,omitempty"`
	AppliesTo *AppliesTo `yaml:"applies_to,omitempty" json:"applies_to,omitempty"`
	Checks    []Check    `yaml:"checks,omitempty" json:"checks,omitempty"`
	Disabled  bool       `yaml:"disabled,omitempty" json:"disabled,omitempty"`
	// Source: builtin@<versão> (instalada do catálogo), gerado ou projeto.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`

	Body string `yaml:"-" json:"body"`
	// Campos de apresentação (nunca gravados).
	Installed       bool   `yaml:"-" json:"installed"`
	Builtin         bool   `yaml:"-" json:"builtin"`
	UpdateAvailable bool   `yaml:"-" json:"update_available,omitempty"`
	File            string `yaml:"-" json:"file,omitempty"`
}

// GatesTask informa se os checks da skill são exigidos na conclusão de tarefas.
func (s *Skill) GatesTask() bool { return s.Scope == "" || s.Scope == ScopeTask }

// Enabled informa se a skill está ativa.
func (s *Skill) Enabled() bool { return !s.Disabled }

var reName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,48}[a-z0-9]$`)

// ValidName informa se o nome serve como pasta e como skill do Claude Code
// (minúsculas, dígitos e hífens, até 50 caracteres).
func ValidName(name string) bool { return reName.MatchString(name) }

// Parse lê um SKILL.md.
func Parse(src string) (*Skill, error) {
	fm, body, ok := model.SplitFrontmatter(src)
	if !ok {
		return nil, fmt.Errorf("SKILL.md sem frontmatter YAML (--- no início)")
	}
	s := &Skill{}
	if err := yaml.Unmarshal([]byte(fm), s); err != nil {
		return nil, fmt.Errorf("frontmatter inválido: %w", err)
	}
	s.Name = strings.TrimSpace(s.Name)
	if !ValidName(s.Name) {
		return nil, fmt.Errorf("nome de skill inválido: %q (use minúsculas, dígitos e hífens)", s.Name)
	}
	if strings.TrimSpace(s.Description) == "" {
		return nil, fmt.Errorf("skill %s sem description", s.Name)
	}
	if _, ok := CategoryLabels[s.Category]; !ok {
		s.Category = CategoryStandards
	}
	if _, ok := TriggerLabels[s.Trigger]; !ok {
		s.Trigger = TriggerAlways
	}
	if s.Scope != ScopeModeling && s.Scope != ScopeSprint {
		s.Scope = ""
	}
	ids := map[string]bool{}
	for i := range s.Checks {
		c := &s.Checks[i]
		c.ID = strings.TrimSpace(c.ID)
		if c.ID == "" {
			c.ID = model.Slugify(c.Text)
		}
		if ids[c.ID] {
			return nil, fmt.Errorf("skill %s: check %q repetido", s.Name, c.ID)
		}
		ids[c.ID] = true
	}
	if s.AppliesTo != nil && len(s.AppliesTo.Tiers) == 0 && len(s.AppliesTo.Stacks) == 0 {
		s.AppliesTo = nil
	}
	s.Body = strings.TrimSpace(body)
	return s, nil
}

// Render gera o SKILL.md (frontmatter + corpo).
func Render(s *Skill) (string, error) {
	fm, err := model.RenderFrontmatter(s)
	if err != nil {
		return "", err
	}
	return fm + "\n" + strings.TrimSpace(s.Body) + "\n", nil
}

// ---------------------------------------------------------------------------
// Catálogo embutido
// ---------------------------------------------------------------------------

// Catalog devolve as skills embutidas no binário, em ordem de categoria e nome,
// mais a convencoes-git gerada das convenções informadas.
func Catalog(c *model.Conventions) []Skill {
	out := []Skill{}
	_ = fs.WalkDir(catalogFS, "catalog", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, "/SKILL.md") {
			return nil
		}
		data, err := catalogFS.ReadFile(path)
		if err != nil {
			return nil
		}
		s, err := Parse(string(data))
		if err != nil {
			return nil
		}
		s.Builtin = true
		s.Source = sourceBuiltinPref + s.Version
		out = append(out, *s)
		return nil
	})
	if c != nil {
		g := ConventionsSkill(c)
		g.Builtin = true
		out = append(out, *g)
	}
	SortSkills(out)
	return out
}

// CatalogSkill devolve uma skill do catálogo pelo nome.
func CatalogSkill(name string, c *model.Conventions) *Skill {
	for _, s := range Catalog(c) {
		if s.Name == name {
			s := s
			return &s
		}
	}
	return nil
}

var categoryOrder = map[string]int{CategoryStudio: 0, CategorySecurity: 1, CategoryOrganization: 2, CategoryStandards: 3}

// SortSkills ordena por categoria e nome.
func SortSkills(list []Skill) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if categoryOrder[a.Category] != categoryOrder[b.Category] {
			return categoryOrder[a.Category] < categoryOrder[b.Category]
		}
		return a.Name < b.Name
	})
}

// BuiltinVersion devolve a versão do catálogo registrada na origem
// ("builtin@1.0.0" → "1.0.0"; "" se não veio do catálogo).
func BuiltinVersion(source string) string {
	if strings.HasPrefix(source, sourceBuiltinPref) {
		return strings.TrimPrefix(source, sourceBuiltinPref)
	}
	return ""
}

// ForInstall prepara uma skill do catálogo para gravar no projeto.
func ForInstall(s Skill) Skill {
	s.Installed, s.Builtin, s.UpdateAvailable, s.File = false, false, false, ""
	if s.Source != SourceGenerated {
		s.Source = sourceBuiltinPref + s.Version
	}
	return s
}

// DefaultSet são as skills instaladas num projeto novo: o protocolo do
// Studio, a definição de pronto, segredos, testes e a convenção de Git.
var DefaultSet = []string{"archcode-fluxo", "convencoes-git", "definicao-de-pronto", "segredos-e-config", "testes-e-aceite"}

// Suggest devolve os nomes do conjunto padrão mais as skills de estilo dos
// stacks detectados no projeto.
func Suggest(catalog []Skill, stacks []string) []string {
	out := append([]string{}, DefaultSet...)
	for _, s := range catalog {
		if s.AppliesTo == nil || len(s.AppliesTo.Stacks) == 0 || s.Category != CategoryStandards {
			continue
		}
		if matchesStacks(s.AppliesTo.Stacks, stacks) {
			out = append(out, s.Name)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Seleção por tarefa
// ---------------------------------------------------------------------------

// stackAliases normaliza os nomes de tecnologia encontrados no canvas.
var stackAliases = map[string]string{
	"golang": "go", "go": "go", "ts": "typescript", "typescript": "typescript", "tsx": "typescript",
	"react": "react", "reactjs": "react", "nextjs": "react", "next": "react", "js": "javascript",
	"javascript": "javascript", "node": "node", "nodejs": "node", "express": "node", "nestjs": "node",
	"python": "python", "django": "python", "fastapi": "python", "flask": "python", "java": "java",
	"spring": "java", "kotlin": "kotlin", "rust": "rust", "php": "php", "laravel": "php", "ruby": "ruby",
	"rails": "ruby", "dotnet": "dotnet", "csharp": "dotnet", "vue": "vue", "angular": "angular", "svelte": "svelte",
}

var reToken = regexp.MustCompile(`[a-z0-9#+]+`)

// DetectStacks extrai os stacks do projeto das tecnologias dos componentes e
// do stack alvo (ex.: "Go / React / PostgreSQL" → go, react).
func DetectStacks(d *model.Diagram, target string) []string {
	texts := []string{target}
	if d != nil {
		for _, n := range d.Nodes {
			texts = append(texts, n.Data.Technology)
		}
	}
	set := map[string]bool{}
	for _, t := range texts {
		for _, tok := range reToken.FindAllString(strings.ToLower(strings.ReplaceAll(t, ".", "")), -1) {
			if s, ok := stackAliases[tok]; ok {
				set[s] = true
			}
		}
	}
	out := []string{}
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func matchesStacks(want, have []string) bool {
	for _, w := range want {
		w = strings.ToLower(w)
		if a, ok := stackAliases[w]; ok {
			w = a
		}
		for _, h := range have {
			if w == h {
				return true
			}
		}
	}
	return false
}

// Applies informa se a skill vale para o tier da tarefa e os stacks do
// projeto. Tier vazio (tarefa sem componente) aceita qualquer skill.
func Applies(s *Skill, tier string, stacks []string) bool {
	if s.AppliesTo == nil {
		return true
	}
	if len(s.AppliesTo.Tiers) > 0 && tier != "" {
		ok := false
		for _, t := range s.AppliesTo.Tiers {
			if strings.EqualFold(t, tier) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	if len(s.AppliesTo.Stacks) > 0 && !matchesStacks(s.AppliesTo.Stacks, stacks) {
		return false
	}
	return true
}

// ForTask devolve as skills ativas que valem para o item.
func ForTask(list []Skill, item *model.WorkItem, stacks []string) []Skill {
	out := []Skill{}
	tier := ""
	if item != nil {
		tier = item.Tier
	}
	for _, s := range list {
		if s.Enabled() && Applies(&s, tier, stacks) {
			out = append(out, s)
		}
	}
	return out
}

// RequiredChecks lista os checks obrigatórios das skills.
func RequiredChecks(list []Skill) []model.CheckResult {
	out := []model.CheckResult{}
	for _, s := range list {
		if !s.GatesTask() {
			continue
		}
		for _, c := range s.Checks {
			if c.Required {
				out = append(out, model.CheckResult{Skill: s.Name, Check: c.ID})
			}
		}
	}
	return out
}

// MissingChecks confere os resultados informados contra os checks
// obrigatórios: devolve os que faltam e os que falharam. Um resultado casa
// com o check pelo id ou pelo texto.
func MissingChecks(list []Skill, reported []model.CheckResult) (missing, failed []string) {
	for _, s := range list {
		if !s.GatesTask() {
			continue
		}
		for _, c := range s.Checks {
			if !c.Required {
				continue
			}
			var got *model.CheckResult
			for i := range reported {
				r := &reported[i]
				if r.Skill == s.Name && (r.Check == c.ID || strings.EqualFold(strings.TrimSpace(r.Check), c.Text)) {
					got = r
				}
			}
			key := s.Name + "/" + c.ID
			switch {
			case got == nil:
				missing = append(missing, key)
			case got.Result == model.CheckFail:
				failed = append(failed, key)
			}
		}
	}
	return missing, failed
}
