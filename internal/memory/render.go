package memory

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
)

// Text devolve a retomada em Markdown compacto, para o terminal e para o
// hook de início de sessão do Claude Code (o texto entra no contexto do
// agente, então cada linha precisa valer o espaço que ocupa).
func Text(r *Resume) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Onde parou — %s\n\n", r.Project)
	if r.Person != "" {
		fmt.Fprintf(&b, "Você: %s · autonomia da IA: %s\n", plan.DisplayName(r.Person), model.AutonomyLabel(r.Autonomy))
	}
	if sp := r.Sprint; sp != nil {
		fmt.Fprintf(&b, "%s", sp.Name)
		if sp.Goal != "" {
			fmt.Fprintf(&b, " — %s", sp.Goal)
		}
		fmt.Fprintf(&b, " · %d/%d concluídas (%d%%) · %d dia(s) útil(eis) restante(s)\n",
			sp.Stats.Completed, sp.Stats.Total, sp.Stats.Progress, sp.DaysLeft)
	} else {
		fmt.Fprintf(&b, "Sem sprint ativa · %d item(ns) no backlog\n", r.BacklogSize)
	}

	if len(r.MyWork) > 0 {
		b.WriteString("\n## Seu trabalho\n\n")
		for _, t := range r.MyWork {
			writeTask(&b, t, true)
		}
	}
	if r.Next != nil {
		b.WriteString("\n## Próxima tarefa pronta\n\n")
		writeTask(&b, *r.Next, r.Detail == DetailFull)
		if r.Next.SuggestedBranch != "" {
			fmt.Fprintf(&b, "  - Branch: `%s`\n", r.Next.SuggestedBranch)
		}
	}

	if g := r.Git; g != nil && g.Available {
		b.WriteString("\n## Git\n\n")
		fmt.Fprintf(&b, "- Branch `%s`", orDash(g.Branch))
		if g.Upstream != "" {
			fmt.Fprintf(&b, " → `%s` (+%d/-%d)", g.Upstream, g.Ahead, g.Behind)
		}
		b.WriteString("\n")
		if g.ChangedCount > 0 {
			fmt.Fprintf(&b, "- %d arquivo(s) alterado(s): %s", g.ChangedCount, strings.Join(g.Changed, ", "))
			if g.ChangedCount > len(g.Changed) {
				b.WriteString(", …")
			}
			b.WriteString("\n")
		}
		if g.LastCommit != "" {
			fmt.Fprintf(&b, "- Último commit: %s\n", g.LastCommit)
		}
	}

	if len(r.Divergences) > 0 {
		b.WriteString("\n## Atenção (o Git vence a memória)\n\n")
		for _, d := range r.Divergences {
			fmt.Fprintf(&b, "- %s\n", d)
		}
	}

	if s := r.LastSession; s != nil {
		b.WriteString("\n## Sua última sessão\n\n")
		writeSession(&b, *s)
	}
	if len(r.TeamSessions) > 0 {
		b.WriteString("\n## Sessões recentes do time\n\n")
		for _, s := range r.TeamSessions {
			writeSession(&b, s)
		}
	}
	if len(r.StaleClaims) > 0 {
		b.WriteString("\n## Reservas paradas\n\n")
		for _, t := range r.StaleClaims {
			fmt.Fprintf(&b, "- %s %s — %s\n", t.ID, t.Title, plan.DisplayName(t.Assignee))
		}
	}
	if len(r.Skills) > 0 {
		b.WriteString("\n## Skills que valem (get_skill ou .arch/skills/<nome>/SKILL.md)\n\n")
		for _, s := range r.Skills {
			fmt.Fprintf(&b, "- `%s` (%s", s.Name, triggerLabel(s.Trigger))
			if s.RequiredChecks > 0 {
				fmt.Fprintf(&b, ", %d check(s) obrigatório(s)", s.RequiredChecks)
			}
			b.WriteString(")")
			if s.Description != "" {
				fmt.Fprintf(&b, ": %s", s.Description)
			}
			b.WriteString("\n")
		}
	}
	if len(r.Memories) > 0 {
		b.WriteString("\n## Memórias do projeto (.arch/memory/)\n\n")
		for _, n := range r.Memories {
			fmt.Fprintf(&b, "- [%s] %s", n.Type, n.Title)
			if n.Summary != "" && n.Summary != n.Title {
				fmt.Fprintf(&b, " — %s", n.Summary)
			}
			b.WriteString("\n")
		}
	}
	if len(r.Hints) > 0 {
		b.WriteString("\n## Próximos passos\n\n")
		for i, h := range r.Hints {
			fmt.Fprintf(&b, "%d. %s\n", i+1, h)
		}
	}
	return b.String()
}

func writeTask(b *strings.Builder, t TaskBrief, detailed bool) {
	status := plan.StatusLabels[t.Status]
	if t.Stale {
		status += ", parada"
	}
	fmt.Fprintf(b, "- **%s** %s (%s", t.ID, t.Title, status)
	if t.Sprint > 0 {
		fmt.Fprintf(b, ", %s", model.SprintName(t.Sprint))
	}
	if t.EstimateHours > 0 {
		fmt.Fprintf(b, ", %.4gh", t.EstimateHours)
	}
	b.WriteString(")\n")
	if t.Branch != "" {
		fmt.Fprintf(b, "  - Branch: `%s`\n", t.Branch)
	}
	if len(t.BlockedBy) > 0 {
		fmt.Fprintf(b, "  - Aguarda: %s\n", strings.Join(t.BlockedBy, ", "))
	}
	if h := t.Handoff; h != nil {
		if h.LastStep != "" {
			fmt.Fprintf(b, "  - Último passo: %s\n", h.LastStep)
		}
		if h.NextStep != "" {
			fmt.Fprintf(b, "  - Próximo passo: %s\n", h.NextStep)
		}
		if len(h.FailingTests) > 0 {
			fmt.Fprintf(b, "  - Testes falhando: %s\n", strings.Join(h.FailingTests, ", "))
		}
		if len(h.Files) > 0 {
			fmt.Fprintf(b, "  - Arquivos: %s\n", strings.Join(h.Files, ", "))
		}
	}
	if detailed && len(t.Acceptance) > 0 {
		b.WriteString("  - Critérios:\n")
		for _, c := range t.Acceptance {
			mark := " "
			if c.Done {
				mark = "x"
			}
			fmt.Fprintf(b, "    - [%s] %s\n", mark, c.Text)
		}
	}
}

func writeSession(b *strings.Builder, s SessionBrief) {
	who := s.Author
	if s.Agent != "" {
		who += " (" + s.Agent + ")"
	}
	date := s.Started
	if len(date) > 16 {
		date = strings.Replace(date[:16], "T", " ", 1)
	}
	fmt.Fprintf(b, "- %s · %s: %s\n", orDash(date), orDash(who), orDash(s.Summary))
	for _, n := range s.NextSteps {
		fmt.Fprintf(b, "  - Próximo: %s\n", n)
	}
	for _, bl := range s.Blockers {
		fmt.Fprintf(b, "  - Bloqueio: %s\n", bl)
	}
}

func triggerLabel(t string) string {
	switch t {
	case "ao_iniciar_tarefa":
		return "ao iniciar a tarefa"
	case "antes_do_pr":
		return "antes do PR"
	default:
		return "sempre"
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
