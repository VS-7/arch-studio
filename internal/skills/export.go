package skills

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// convencoes-git: gerada a partir de .arch/conventions.yaml
// ---------------------------------------------------------------------------

// ConventionsSkill gera a skill convencoes-git com os padrões do projeto.
func ConventionsSkill(c *model.Conventions) *Skill {
	item := &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Implementar emissão de JWT",
		Parent: "ST-RF003", Requirements: []string{"RF003"}, Sprint: 1}
	bug := &model.WorkItem{ID: "BUG-007", Type: model.ItemBug, Title: "Expiração antecipada da sessão"}
	commit := conventions.CommitSubject(c, conventions.CommitInput{Item: item, Sprint: 1})
	offSprint := conventions.CommitSubject(c, conventions.CommitInput{Item: bug})
	branch := conventions.BranchName(c, item, 1)
	prTitle := conventions.PRTitle(c, conventions.PRInput{Items: []*model.WorkItem{{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emissão de JWT", Sprint: 1}}})

	var b strings.Builder
	b.WriteString("# Convenções de Git do projeto\n\n")
	b.WriteString("## Quando usar\n\nSempre que criar branch, escrever commit ou abrir pull request. ")
	b.WriteString("Os padrões vêm de `.arch/conventions.yaml` e são validados pelos hooks locais e por `archcode-studio git lint` na CI.\n\n")
	b.WriteString("## Regras\n\n")
	fmt.Fprintf(&b, "1. **Branch:** `%s` — padrão `%s`. Fora de sprint: `%s`.\n", branch, c.Git.Branch, c.Git.BranchOffSprint)
	fmt.Fprintf(&b, "2. **Commit:** `%s` — padrão `%s`. Fora de sprint: `%s`.\n", commit, c.Git.Commit, offSprint)
	fmt.Fprintf(&b, "3. Assunto com até %d caracteres", c.Git.MaxSubject)
	if len(c.Git.Verbs) > 0 {
		fmt.Fprintf(&b, ", começando por um verbo no presente da 3ª pessoa (%s…)", strings.Join(c.Git.Verbs[:min(6, len(c.Git.Verbs))], ", "))
	}
	b.WriteString(".\n")
	if c.Git.RequireID {
		b.WriteString("4. O id do item vai entre colchetes no fim do assunto e precisa existir em `.arch/plan/`. Commits só de planejamento usam o id da sprint (`[SPRINT-01]`) ou `[ARCH-PLAN]` — gere com `archcode-studio commit --plan`.\n")
	} else {
		b.WriteString("4. Cite o id do item entre colchetes sempre que houver um.\n")
	}
	b.WriteString("5. Corpo opcional explica o porquê; no fim, os trailers `Task:`, `Story:`, `Refs:` e `Co-Authored-By:` quando um agente participou. Use `propose_commit` (MCP) ou `archcode-studio commit` para gerar a mensagem.\n")
	fmt.Fprintf(&b, "6. **Pull request:** título `%s`; um PR por %s; merge por %s.\n", prTitle,
		map[string]string{"task": "tarefa", "story": "história"}[c.PullRequest.Granularity], c.PullRequest.MergeStrategy)
	fmt.Fprintf(&b, "7. **Tags:** sprint `%s`, release `%s` — criadas só por humanos.\n", conventions.Render(c.Tags.Sprint, conventions.Vars{Sprint: 1}),
		conventions.Render(c.Tags.Release, conventions.Vars{Version: "1.0.0"}))
	fmt.Fprintf(&b, "8. **Autonomia da IA:** %s — %s Merge, tag e release são sempre humanos.\n", model.AutonomyLabel(c.AI.Autonomy), autonomyRule(c.AI.Autonomy))
	b.WriteString("\n## Como verificar\n\n")
	b.WriteString("- `commits-na-convencao`: `archcode-studio git lint` sem erros.\n")
	b.WriteString("- `branch-na-convencao`: o nome da branch atual passa em `archcode-studio git lint --branch \"$(git branch --show-current)\"`.\n")

	return &Skill{
		Name:        "convencoes-git",
		Description: "Padrões de branch, commit, pull request e tags do projeto, gerados de .arch/conventions.yaml. Use ao criar branch, escrever commit ou abrir PR.",
		Category:    CategoryStandards,
		Version:     "1.0.0",
		Trigger:     TriggerAlways,
		Checks: []Check{
			{ID: "commits-na-convencao", Text: "Todos os commits da branch seguem a convenção do projeto.", Command: "archcode-studio git lint", Required: true},
			{ID: "branch-na-convencao", Text: "A branch segue o padrão de nomes do projeto.", Required: false},
		},
		Source: SourceGenerated,
		Body:   b.String(),
	}
}

func autonomyRule(a string) string {
	switch a {
	case model.AutonomySupervised:
		return "o agente faz commits locais; push e PR ficam com um humano."
	case model.AutonomyAutonomous:
		return "o agente faz commits, publica a branch e abre PR em rascunho."
	default:
		return "o agente propõe cada commit e um humano confirma; push e PR são humanos."
	}
}

// ---------------------------------------------------------------------------
// Exportação para os agentes (RF046)
// ---------------------------------------------------------------------------

// GeneratedMarker identifica arquivos gerados pela sincronização.
const GeneratedMarker = "Gerado pelo ArchCode Studio a partir de .arch/skills/"

func generatedNote(name string) string {
	return fmt.Sprintf("<!-- %s%s/SKILL.md — edite lá e rode `archcode-studio skills sync`. -->", GeneratedMarker, name)
}

// checksSection descreve os checks no fim do corpo exportado.
func checksSection(s *Skill) string {
	if len(s.Checks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n## Checks (informe cada um em `complete_task`)\n\n")
	for _, c := range s.Checks {
		req := "opcional"
		if c.Required {
			req = "obrigatório"
		}
		fmt.Fprintf(&b, "- `%s` (%s): %s", c.ID, req, c.Text)
		if c.Command != "" {
			fmt.Fprintf(&b, " — `%s`", c.Command)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// ClaudeSkill devolve o .claude/skills/<nome>/SKILL.md (só name e
// description no frontmatter, que é o que o Claude Code lê).
func ClaudeSkill(s *Skill) string {
	desc := strings.Join(strings.Fields(s.Description), " ")
	fm, _ := model.RenderFrontmatter(struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}{s.Name, desc})
	return fm + "\n" + generatedNote(s.Name) + "\n\n" + strings.TrimSpace(s.Body) + checksSection(s) + "\n"
}

// ClaudePath devolve o caminho da skill no formato do Claude Code.
func ClaudePath(name string) string { return ".claude/skills/" + name + "/SKILL.md" }

// CursorRule devolve o .cursor/rules/archcode-<nome>.mdc.
func CursorRule(s *Skill) string {
	fm, _ := model.RenderFrontmatter(struct {
		Description string `yaml:"description"`
		AlwaysApply bool   `yaml:"alwaysApply"`
	}{strings.Join(strings.Fields(s.Description), " "), s.Trigger == TriggerAlways})
	return fm + "\n" + generatedNote(s.Name) + "\n\n" + strings.TrimSpace(s.Body) + checksSection(s) + "\n"
}

// CursorPath devolve o caminho da regra no formato do Cursor.
func CursorPath(name string) string { return ".cursor/rules/archcode-" + name + ".mdc" }

// AgentsBlockName é o nome do bloco gerenciado no AGENTS.md.
const AgentsBlockName = "skills"

// AgentsBlock devolve o bloco do AGENTS.md: protocolo do Studio, convenção e
// a tabela de skills ativas (qualquer agente lê).
func AgentsBlock(list []Skill, c *model.Conventions) string {
	item := &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Implementar emissão de JWT"}
	var b strings.Builder
	b.WriteString("## ArchCode Studio\n\n")
	b.WriteString("Este projeto é gerenciado pelo ArchCode Studio (arquitetura, backlog, sprints e memória em `.arch/`).\n\n")
	b.WriteString("- **Ao começar:** chame `resume_work` no MCP `archcode-studio` (sem MCP: `archcode-studio resume --brief`). Ele diz onde o trabalho parou, a próxima tarefa e as skills que valem.\n")
	b.WriteString("- **Durante:** `claim_task` antes de mexer, `save_checkpoint` a cada passo, `create_backlog_item` para o que estiver fora do escopo.\n")
	b.WriteString("- **Ao parar:** `log_session`; ao concluir, `complete_task` com os checks das skills.\n")
	fmt.Fprintf(&b, "- **Commit:** `%s` · **branch:** `%s`.\n", conventions.CommitSubject(c, conventions.CommitInput{Item: item, Sprint: 1}), conventions.BranchName(c, item, 1))
	fmt.Fprintf(&b, "- **Autonomia da IA:** %s — %s Merge, tag e release são sempre humanos.\n", model.AutonomyLabel(c.AI.Autonomy), autonomyRule(c.AI.Autonomy))
	b.WriteString("- Nunca edite `.arch/diagrams/*.json` nem `.arch/plan/*` à mão quando existe ferramenta.\n")
	active := []Skill{}
	for _, s := range list {
		if s.Enabled() {
			active = append(active, s)
		}
	}
	if len(active) > 0 {
		b.WriteString("\n### Skills do projeto\n\nLeia a skill inteira em `.arch/skills/<nome>/SKILL.md` (ou com `get_skill`) quando ela valer para a tarefa.\n\n")
		b.WriteString("| Skill | Quando | O que garante |\n| --- | --- | --- |\n")
		for _, s := range active {
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", s.Name, TriggerLabels[s.Trigger], strings.ReplaceAll(strings.Join(strings.Fields(s.Description), " "), "|", "/"))
		}
	}
	return b.String()
}
