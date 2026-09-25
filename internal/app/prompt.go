package app

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
)

// ---------------------------------------------------------------------------
// Prévia das convenções e prompt para agentes sem MCP (RF062)
// ---------------------------------------------------------------------------

// ConventionsPreview mostra como ficam branch, commit e PR com as convenções
// informadas (ainda não gravadas), para a tela de edição.
type ConventionsPreview struct {
	Branch          string   `json:"branch"`
	BranchOffSprint string   `json:"branch_off_sprint"`
	Commit          string   `json:"commit"`
	CommitOffSprint string   `json:"commit_off_sprint"`
	PRTitle         string   `json:"pr_title"`
	SprintTag       string   `json:"sprint_tag"`
	Errors          []string `json:"errors,omitempty"`
}

// PreviewConventions gera os exemplos de uma configuração.
func (a *App) PreviewConventions(c *model.Conventions) *ConventionsPreview {
	c.Normalize()
	out := &ConventionsPreview{}
	for _, pattern := range []string{c.Git.Branch, c.Git.BranchOffSprint, c.Git.Commit, c.Git.CommitOffSprint, c.PullRequest.Title} {
		if _, err := conventions.Compile(pattern, c); err != nil {
			out.Errors = append(out.Errors, fmt.Sprintf("%q: %v", pattern, err))
		}
	}
	task := &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Implementar emissão de JWT", Sprint: 1, Parent: "ST-RF003"}
	bug := &model.WorkItem{ID: "BUG-007", Type: model.ItemBug, Title: "Expiração antecipada da sessão"}
	out.Branch = conventions.BranchName(c, task, 1)
	out.BranchOffSprint = conventions.BranchName(c, bug, 0)
	out.Commit = conventions.CommitSubject(c, conventions.CommitInput{Item: task, Sprint: 1})
	out.CommitOffSprint = conventions.CommitSubject(c, conventions.CommitInput{Item: bug})
	out.PRTitle = conventions.PRTitle(c, conventions.PRInput{Items: []*model.WorkItem{{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emissão de JWT", Sprint: 1}}})
	out.SprintTag = conventions.Render(c.Tags.Sprint, conventions.Vars{Sprint: 1})
	if !conventions.Has(c.Git.Commit, "id") && c.Git.RequireID {
		out.Errors = append(out.Errors, "o padrão de commit precisa ter {id} quando require_id está ligado")
	}
	return out
}

// AgentPrompt monta um prompt autocontido para qualquer agente de IA (mesmo
// sem MCP) implementar a tarefa seguindo as regras do projeto.
func (a *App) AgentPrompt(id string) (string, error) {
	p, err := a.Plan()
	if err != nil {
		return "", err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return "", err
	}
	item := p.Item(id)
	if item == nil {
		return "", itemNotFound(p, id)
	}
	manifest, _ := a.st.LoadManifest()
	sprint := item.Sprint
	var b strings.Builder
	project := "o projeto"
	if manifest != nil && manifest.ProjectName != "" {
		project = manifest.ProjectName
	}
	fmt.Fprintf(&b, "Implemente a tarefa %s de %s (gerenciado pelo ArchCode Studio).\n\n", item.ID, project)
	fmt.Fprintf(&b, "## %s · %s\n\n", item.ID, item.Title)
	if item.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", item.Description)
	}
	if item.Component != "" {
		fmt.Fprintf(&b, "- Componente: %s", item.Component)
		if item.Tier != "" {
			fmt.Fprintf(&b, " (%s)", item.Tier)
		}
		b.WriteString("\n")
	}
	if sprint > 0 {
		if sp := p.Sprint(sprint); sp != nil {
			fmt.Fprintf(&b, "- Sprint: %s", sp.Name)
			if sp.Goal != "" {
				fmt.Fprintf(&b, " — %s", sp.Goal)
			}
			b.WriteString("\n")
		}
	}
	if len(item.Requirements)+len(item.UseCases) > 0 {
		fmt.Fprintf(&b, "- Rastreabilidade: %s\n", strings.Join(append(append([]string{}, item.Requirements...), item.UseCases...), ", "))
	}
	if len(item.Endpoints) > 0 {
		fmt.Fprintf(&b, "- Endpoints: %s\n", strings.Join(item.Endpoints, ", "))
	}
	if blocked := plan.BlockedBy(item, p); len(blocked) > 0 {
		fmt.Fprintf(&b, "- ATENÇÃO: depende de %s, ainda não concluída(s)\n", strings.Join(blocked, ", "))
	}
	if len(item.Acceptance) > 0 {
		b.WriteString("\n### Critérios de aceite (cada um vira ao menos um teste)\n\n")
		for _, c := range item.Acceptance {
			fmt.Fprintf(&b, "- %s\n", c.Text)
		}
	}
	if h := item.Handoff; !h.Empty() {
		b.WriteString("\n### Onde parou\n\n")
		if h.LastStep != "" {
			fmt.Fprintf(&b, "- Último passo: %s\n", h.LastStep)
		}
		if h.NextStep != "" {
			fmt.Fprintf(&b, "- Próximo passo: %s\n", h.NextStep)
		}
		if len(h.Files) > 0 {
			fmt.Fprintf(&b, "- Arquivos: %s\n", strings.Join(h.Files, ", "))
		}
		if len(h.FailingTests) > 0 {
			fmt.Fprintf(&b, "- Testes falhando: %s\n", strings.Join(h.FailingTests, ", "))
		}
	}
	applicable := a.skillsFor(item)
	if len(applicable) > 0 {
		b.WriteString("\n### Regras do projeto (skills)\n\nLeia cada arquivo antes de codar e confira os checks obrigatórios no fim:\n\n")
		for _, s := range applicable {
			fmt.Fprintf(&b, "- `.arch/skills/%s/SKILL.md` — %s\n", s.Name, s.Description)
		}
	}
	branch := item.Branch
	if branch == "" {
		branch = conventions.BranchName(conv, item, sprint)
	}
	b.WriteString("\n### Convenções\n\n")
	fmt.Fprintf(&b, "- Branch: `%s`\n", branch)
	fmt.Fprintf(&b, "- Commit: `%s`\n", conventions.CommitSubject(conv, conventions.CommitInput{Item: item, Sprint: sprint}))
	fmt.Fprintf(&b, "- Autonomia: %s — merge, tag e release são sempre humanos.\n", model.AutonomyLabel(conv.AI.Autonomy))
	b.WriteString("\n### Como trabalhar\n\n")
	b.WriteString("1. Implemente só o escopo desta tarefa; o que estiver fora, anote como item novo do backlog.\n")
	b.WriteString("2. Não edite `.arch/diagrams/*.json` nem `.arch/plan/*` à mão.\n")
	b.WriteString("3. Rode os testes e `archcode-studio validate` antes de terminar.\n")
	b.WriteString("4. Ao terminar, resuma o que mudou, os comandos de teste com o resultado e o que ficou pendente.\n")
	b.WriteString("\nCom o MCP `archcode-studio` disponível, use `resume_work`, `claim_task`, `save_checkpoint` e `complete_task` em vez dos passos manuais.\n")
	return b.String(), nil
}
