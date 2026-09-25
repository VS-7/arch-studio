package model

import "strings"

// ---------------------------------------------------------------------------
// Convenções do projeto (.arch/conventions.yaml)
// ---------------------------------------------------------------------------
//
// Planejamento, Git, pull requests, tags e autonomia da IA num só arquivo,
// lido pelo Studio, pelos hooks do Git e pela CI (RF038). Os padrões usam
// marcadores entre chaves: {sprint} (dois dígitos), {id}, {slug}, {summary},
// {title}, {kind} (Hotfix, Chore…), {type} (feat, fix…) e {version}.

// Presets de convenção.
const (
	PresetArchCode     = "archcode-sprint"
	PresetConventional = "conventional-commits"
)

// Níveis de autonomia da IA (RF061).
const (
	AutonomyAssisted   = "assistido"
	AutonomySupervised = "supervisionado"
	AutonomyAutonomous = "autonomo"
)

// Modos de fatiamento do backlog.
const (
	SlicingComponent = "component"
	SlicingHybrid    = "hybrid"
)

type PlanningConventions struct {
	// SprintDays é a duração padrão de uma sprint, em dias úteis.
	SprintDays int `yaml:"sprint_days" json:"sprint_days"`
	// Slicing: component (uma tarefa de fundação por componente) ou hybrid
	// (fundação + fatias verticais por requisito funcional).
	Slicing string `yaml:"slicing" json:"slicing"`
	// StaleDays: reserva sem atividade há mais dias úteis que isso é "parada".
	StaleDays int `yaml:"stale_days" json:"stale_days"`
}

type GitConventions struct {
	MainBranch      string   `yaml:"main_branch" json:"main_branch"`
	Remote          string   `yaml:"remote" json:"remote"`
	Branch          string   `yaml:"branch" json:"branch"`
	BranchOffSprint string   `yaml:"branch_off_sprint" json:"branch_off_sprint"`
	Commit          string   `yaml:"commit" json:"commit"`
	CommitOffSprint string   `yaml:"commit_off_sprint" json:"commit_off_sprint"`
	OffSprintKinds  []string `yaml:"off_sprint_kinds" json:"off_sprint_kinds"`
	MaxSubject      int      `yaml:"max_subject" json:"max_subject"`
	// Verbs são os verbos aceitos no início do resumo (vazio = qualquer um).
	Verbs     []string `yaml:"verbs,omitempty" json:"verbs,omitempty"`
	RequireID bool     `yaml:"require_id" json:"require_id"`
	// BranchExempt são padrões glob de branches que não seguem a convenção
	// (bots de dependências, por exemplo). A branch principal é sempre isenta.
	BranchExempt []string `yaml:"branch_exempt,omitempty" json:"branch_exempt,omitempty"`
}

type PullRequestConventions struct {
	Title string `yaml:"title" json:"title"`
	// Granularity: task (um PR por tarefa) ou story (um PR por história).
	Granularity   string `yaml:"granularity" json:"granularity"`
	MergeStrategy string `yaml:"merge_strategy" json:"merge_strategy"`
}

type TagConventions struct {
	Sprint  string `yaml:"sprint" json:"sprint"`
	Release string `yaml:"release" json:"release"`
}

type AIConventions struct {
	Autonomy string `yaml:"autonomy" json:"autonomy"`
}

// Conventions é o conteúdo de .arch/conventions.yaml.
type Conventions struct {
	Preset      string                 `yaml:"preset" json:"preset"`
	Language    string                 `yaml:"language" json:"language"`
	Planning    PlanningConventions    `yaml:"planning" json:"planning"`
	Git         GitConventions         `yaml:"git" json:"git"`
	PullRequest PullRequestConventions `yaml:"pull_request" json:"pull_request"`
	Tags        TagConventions         `yaml:"tags" json:"tags"`
	AI          AIConventions          `yaml:"ai" json:"ai"`
}

// DefaultVerbs são os verbos do preset ArchCode Sprint (presente, 3ª pessoa,
// como o histórico do próprio ArchCode Studio).
var DefaultVerbs = []string{
	"Implementa", "Corrige", "Refatora", "Testa", "Documenta", "Configura", "Remove",
	"Adiciona", "Atualiza", "Cria", "Ajusta", "Melhora", "Reserva", "Investiga",
	"Protege", "Migra", "Otimiza", "Integra", "Publica", "Prepara",
}

// DefaultConventions devolve o preset informado ("" = ArchCode Sprint).
func DefaultConventions(preset string) *Conventions {
	c := &Conventions{
		Preset:   PresetArchCode,
		Language: "pt-BR",
		Planning: PlanningConventions{SprintDays: 10, Slicing: SlicingHybrid, StaleDays: 3},
		Git: GitConventions{
			MainBranch:      "main",
			Remote:          "origin",
			Branch:          "sprint-{sprint}/{id}-{slug}",
			BranchOffSprint: "{kind}/{id}-{slug}",
			Commit:          "Sprint {sprint} - {summary} [{id}]",
			CommitOffSprint: "{kind} - {summary} [{id}]",
			OffSprintKinds:  []string{"Hotfix", "Chore"},
			MaxSubject:      72,
			Verbs:           append([]string(nil), DefaultVerbs...),
			RequireID:       true,
			BranchExempt:    []string{"dependabot/*", "renovate/*"},
		},
		PullRequest: PullRequestConventions{
			Title:         "Sprint {sprint} - {title} [{id}]",
			Granularity:   "task",
			MergeStrategy: "squash",
		},
		Tags: TagConventions{Sprint: "sprint-{sprint}", Release: "v{version}"},
		AI:   AIConventions{Autonomy: AutonomyAssisted},
	}
	if preset == PresetConventional {
		c.Preset = PresetConventional
		c.Git.Branch = "{type}/{id}-{slug}"
		c.Git.BranchOffSprint = "{type}/{id}-{slug}"
		c.Git.Commit = "{type}: {summary} [{id}]"
		c.Git.CommitOffSprint = "{type}: {summary} [{id}]"
		c.Git.OffSprintKinds = nil
		c.Git.Verbs = nil
		c.PullRequest.Title = "{type}: {title} [{id}]"
	}
	return c
}

// LegacyConventions são os padrões de projetos que ainda não têm
// conventions.yaml: iguais ao preset, mas com o fatiamento por componente do
// AI-PRD original, para que nada mude sem o time escolher (RNF012).
func LegacyConventions() *Conventions {
	c := DefaultConventions("")
	c.Planning.Slicing = SlicingComponent
	return c
}

// Normalize preenche campos vazios com os padrões do preset.
func (c *Conventions) Normalize() {
	d := DefaultConventions(c.Preset)
	if c.Preset != PresetConventional {
		c.Preset = PresetArchCode
	}
	if c.Language == "" {
		c.Language = d.Language
	}
	if c.Planning.SprintDays <= 0 {
		c.Planning.SprintDays = d.Planning.SprintDays
	}
	if c.Planning.Slicing != SlicingComponent && c.Planning.Slicing != SlicingHybrid {
		c.Planning.Slicing = d.Planning.Slicing
	}
	if c.Planning.StaleDays <= 0 {
		c.Planning.StaleDays = d.Planning.StaleDays
	}
	g, dg := &c.Git, d.Git
	if g.MainBranch == "" {
		g.MainBranch = dg.MainBranch
	}
	if g.Remote == "" {
		g.Remote = dg.Remote
	}
	if g.Branch == "" {
		g.Branch = dg.Branch
	}
	if g.BranchOffSprint == "" {
		g.BranchOffSprint = dg.BranchOffSprint
	}
	if g.Commit == "" {
		g.Commit = dg.Commit
	}
	if g.CommitOffSprint == "" {
		g.CommitOffSprint = dg.CommitOffSprint
	}
	if g.MaxSubject <= 0 {
		g.MaxSubject = dg.MaxSubject
	}
	if c.PullRequest.Title == "" {
		c.PullRequest.Title = d.PullRequest.Title
	}
	if c.PullRequest.Granularity != "story" {
		c.PullRequest.Granularity = "task"
	}
	if c.PullRequest.MergeStrategy == "" {
		c.PullRequest.MergeStrategy = d.PullRequest.MergeStrategy
	}
	if c.Tags.Sprint == "" {
		c.Tags.Sprint = d.Tags.Sprint
	}
	if c.Tags.Release == "" {
		c.Tags.Release = d.Tags.Release
	}
	c.AI.Autonomy = NormalizeAutonomy(c.AI.Autonomy)
}

// NormalizeAutonomy aceita os níveis em português ou inglês.
func NormalizeAutonomy(a string) string {
	switch normalizeKey(a) {
	case "supervisionado", "supervised":
		return AutonomySupervised
	case "autonomo", "autonomous", "auto":
		return AutonomyAutonomous
	default:
		return AutonomyAssisted
	}
}

// AutonomyLabel devolve o nome legível do nível de autonomia.
func AutonomyLabel(a string) string {
	switch a {
	case AutonomySupervised:
		return "Supervisionado"
	case AutonomyAutonomous:
		return "Autônomo"
	default:
		return "Assistido"
	}
}

// AgentMayCommit informa se o agente pode fazer commit local sozinho.
func (c *Conventions) AgentMayCommit() bool { return c.AI.Autonomy != AutonomyAssisted }

// AgentMayPush informa se o agente pode publicar branches e abrir PR em rascunho.
func (c *Conventions) AgentMayPush() bool { return c.AI.Autonomy == AutonomyAutonomous }

// IsOffSprintKind informa se a palavra é um tipo de commit fora de sprint.
func (c *Conventions) IsOffSprintKind(kind string) bool {
	for _, k := range c.Git.OffSprintKinds {
		if strings.EqualFold(k, kind) {
			return true
		}
	}
	return false
}
