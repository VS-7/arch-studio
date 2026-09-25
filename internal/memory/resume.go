// Package memory monta a memória de trabalho do projeto: a resposta de
// "onde parou" (resume_work), a partir do backlog, dos checkpoints, das
// sessões, das memórias e do estado real do Git. É puro: recebe tudo pronto
// e devolve a visão consolidada, que o app entrega ao agente, ao CLI, ao
// hook de início de sessão e à interface (RF051).
package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/skills"
)

// Níveis de detalhe.
const (
	DetailBrief = "brief"
	DetailFull  = "full"
)

// GitState é o retrato do repositório no momento da retomada.
type GitState struct {
	Available    bool     `json:"available"`
	Branch       string   `json:"branch,omitempty"`
	Upstream     string   `json:"upstream,omitempty"`
	Ahead        int      `json:"ahead,omitempty"`
	Behind       int      `json:"behind,omitempty"`
	Changed      []string `json:"changed,omitempty"`
	ChangedCount int      `json:"changed_count"`
	LastCommit   string   `json:"last_commit,omitempty"`
}

// maxChangedFiles limita a lista de arquivos alterados na resposta.
const maxChangedFiles = 25

// NewGitState resume o status e o último commit do Git.
func NewGitState(st *gitx.Status, last *gitx.Commit) *GitState {
	if st == nil {
		return &GitState{Available: false}
	}
	g := &GitState{Available: true, Branch: st.Branch, Upstream: st.Upstream, Ahead: st.Ahead, Behind: st.Behind,
		ChangedCount: len(st.Changed)}
	for i, c := range st.Changed {
		if i >= maxChangedFiles {
			break
		}
		g.Changed = append(g.Changed, c.Path)
	}
	if last != nil {
		g.LastCommit = fmt.Sprintf("%s %s", shortHash(last.Hash), last.Subject)
	}
	return g
}

// TaskBrief é uma tarefa na resposta de retomada.
type TaskBrief struct {
	ID            string            `json:"id"`
	Title         string            `json:"title"`
	Type          string            `json:"type"`
	Status        string            `json:"status"`
	Sprint        int               `json:"sprint,omitempty"`
	Assignee      string            `json:"assignee,omitempty"`
	Component     string            `json:"component,omitempty"`
	Branch        string            `json:"branch,omitempty"`
	EstimateHours float64           `json:"estimate_h,omitempty"`
	Stale         bool              `json:"stale,omitempty"`
	BlockedBy     []string          `json:"blocked_by,omitempty"`
	Handoff       *model.Handoff    `json:"handoff,omitempty"`
	Acceptance    []model.Criterion `json:"acceptance,omitempty"`
	Dependencies  []string          `json:"dependencies,omitempty"`
	Requirements  []string          `json:"requirements,omitempty"`
	Endpoints     []string          `json:"endpoints,omitempty"`
	// SuggestedBranch é a branch da convenção para a tarefa.
	SuggestedBranch string `json:"suggested_branch,omitempty"`
}

// SprintBrief resume a sprint ativa.
type SprintBrief struct {
	Number   int        `json:"number"`
	Name     string     `json:"name"`
	Goal     string     `json:"goal,omitempty"`
	Start    string     `json:"start,omitempty"`
	End      string     `json:"end,omitempty"`
	DaysLeft int        `json:"days_left"`
	Stats    plan.Stats `json:"stats"`
	Capacity float64    `json:"capacity_h,omitempty"`
}

// SessionBrief resume uma sessão.
type SessionBrief struct {
	ID        string   `json:"id"`
	Author    string   `json:"author,omitempty"`
	Agent     string   `json:"agent,omitempty"`
	Started   string   `json:"started,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Tasks     []string `json:"tasks,omitempty"`
	NextSteps []string `json:"next_steps,omitempty"`
	Blockers  []string `json:"blockers,omitempty"`
}

// SkillBrief resume uma skill que vale para a tarefa.
type SkillBrief struct {
	Name           string `json:"name"`
	Trigger        string `json:"trigger"`
	Description    string `json:"description,omitempty"`
	RequiredChecks int    `json:"required_checks,omitempty"`
}

// NoteBrief resume uma memória.
type NoteBrief struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Summary string `json:"summary,omitempty"`
}

// Resume é a resposta de "onde parou".
type Resume struct {
	Project      string         `json:"project"`
	Person       string         `json:"person,omitempty"`
	Detail       string         `json:"detail"`
	Autonomy     string         `json:"autonomy"`
	Sprint       *SprintBrief   `json:"sprint,omitempty"`
	MyWork       []TaskBrief    `json:"my_work"`
	Next         *TaskBrief     `json:"next,omitempty"`
	LastSession  *SessionBrief  `json:"last_session,omitempty"`
	TeamSessions []SessionBrief `json:"team_sessions,omitempty"`
	StaleClaims  []TaskBrief    `json:"stale_claims,omitempty"`
	Git          *GitState      `json:"git"`
	Divergences  []string       `json:"divergences,omitempty"`
	Skills       []SkillBrief   `json:"skills,omitempty"`
	Memories     []NoteBrief    `json:"memories,omitempty"`
	Hints        []string       `json:"hints"`
	BacklogSize  int            `json:"backlog_size"`
}

// Input reúne o que a retomada lê.
type Input struct {
	Now         time.Time
	Project     string
	Person      string
	Detail      string
	Plan        *model.Plan
	Conventions *model.Conventions
	Sessions    []model.Session // da mais recente para a mais antiga
	Notes       []model.Note
	Skills      []skills.Skill // instaladas (ativas ou não)
	Stacks      []string
	// SkillsFor, quando presente, escolhe as skills da tarefa em foco (o app
	// usa a tecnologia do componente); sem ele, valem Skills e Stacks.
	SkillsFor func(*model.WorkItem) []skills.Skill
	Git       *GitState
	// MainCommits são commits recentes da branch principal, para descobrir
	// tarefas já integradas que o backlog ainda não marcou.
	MainCommits []gitx.Commit
}

// Build monta a resposta.
func Build(in Input) *Resume {
	if in.Detail != DetailFull {
		in.Detail = DetailBrief
	}
	if in.Plan == nil {
		in.Plan = model.NewPlan()
	}
	if in.Conventions == nil {
		in.Conventions = model.LegacyConventions()
	}
	if in.Git == nil {
		in.Git = &GitState{}
	}
	full := in.Detail == DetailFull
	r := &Resume{Project: in.Project, Person: in.Person, Detail: in.Detail, Git: in.Git,
		Autonomy: in.Conventions.AI.Autonomy, MyWork: []TaskBrief{}, Hints: []string{}}
	p := in.Plan
	for _, it := range p.Items {
		if it.Actionable() && !it.Archived {
			r.BacklogSize++
		}
	}

	if sp := p.ActiveSprint(); sp != nil {
		r.Sprint = &SprintBrief{Number: sp.Number, Name: sp.Name, Goal: sp.Goal, Start: sp.Start, End: sp.End,
			DaysLeft: plan.DaysLeft(sp, in.Now), Stats: plan.SprintStats(p, sp.Number), Capacity: sp.CapacityHours}
	}

	stale := in.Conventions.Planning.StaleDays
	for _, it := range plan.MyWork(p, in.Person) {
		tb := brief(it, p, in, true)
		tb.Stale = plan.Stale(it, in.Now, stale)
		r.MyWork = append(r.MyWork, tb)
	}
	if next := plan.NextReady(p, in.Person); next != nil {
		tb := brief(next, p, in, true)
		tb.SuggestedBranch = conventions.BranchName(in.Conventions, next, sprintFor(next, p))
		r.Next = &tb
	}
	for i := range p.Items {
		it := &p.Items[i]
		if it.Assignee != "" && !plan.SamePerson(it.Assignee, in.Person) && plan.Stale(it, in.Now, stale) {
			tb := brief(it, p, in, false)
			tb.Stale = true
			r.StaleClaims = append(r.StaleClaims, tb)
		}
	}

	// Sessões: a minha mais recente e as últimas do time.
	teamMax := 2
	if full {
		teamMax = 5
	}
	for _, s := range in.Sessions {
		sb := sessionBrief(s)
		if r.LastSession == nil && plan.SamePerson(s.Author, in.Person) {
			r.LastSession = &sb
			continue
		}
		if !plan.SamePerson(s.Author, in.Person) && len(r.TeamSessions) < teamMax {
			r.TeamSessions = append(r.TeamSessions, sb)
		}
	}

	// Skills da tarefa em foco (a minha em andamento ou a próxima).
	var focus *model.WorkItem
	if len(r.MyWork) > 0 {
		focus = p.Item(r.MyWork[0].ID)
	} else if r.Next != nil {
		focus = p.Item(r.Next.ID)
	}
	chosen := skills.ForTask(in.Skills, focus, in.Stacks)
	if in.SkillsFor != nil {
		chosen = in.SkillsFor(focus)
	}
	for _, s := range chosen {
		sb := SkillBrief{Name: s.Name, Trigger: s.Trigger}
		for _, c := range s.Checks {
			if c.Required && s.GatesTask() {
				sb.RequiredChecks++
			}
		}
		if full {
			sb.Description = s.Description
		}
		r.Skills = append(r.Skills, sb)
	}

	r.Memories = relevantNotes(in.Notes, focus, full)
	r.Divergences = divergences(in, r)
	r.Hints = hints(in, r)
	return r
}

func sprintFor(it *model.WorkItem, p *model.Plan) int {
	if it.Sprint > 0 {
		return it.Sprint
	}
	if sp := p.ActiveSprint(); sp != nil {
		return sp.Number
	}
	return 0
}

func brief(it *model.WorkItem, p *model.Plan, in Input, withCriteria bool) TaskBrief {
	tb := TaskBrief{ID: it.ID, Title: it.Title, Type: it.Type, Status: it.Status, Sprint: it.Sprint,
		Assignee: it.Assignee, Component: it.Component, Branch: it.Branch, EstimateHours: it.EstimateHours,
		BlockedBy: plan.BlockedBy(it, p)}
	if !it.Handoff.Empty() {
		tb.Handoff = it.Handoff
	}
	if withCriteria {
		tb.Acceptance = it.Acceptance
		tb.Dependencies = it.Dependencies
		tb.Requirements = it.Requirements
		tb.Endpoints = it.Endpoints
	}
	return tb
}

func sessionBrief(s model.Session) SessionBrief {
	summary := strings.TrimSpace(s.Summary)
	if i := strings.Index(summary, "\n"); i >= 0 {
		summary = summary[:i]
	}
	return SessionBrief{ID: s.ID, Author: plan.DisplayName(s.Author), Agent: s.Agent, Started: s.Started,
		Summary: summary, Tasks: s.Tasks, NextSteps: s.NextSteps, Blockers: s.Blockers}
}

// relevantNotes escolhe as memórias ligadas ao componente em foco e, depois,
// as armadilhas e convenções mais recentes.
func relevantNotes(notes []model.Note, focus *model.WorkItem, full bool) []NoteBrief {
	type scored struct {
		n     model.Note
		score int
	}
	list := []scored{}
	for _, n := range notes {
		score := 0
		if focus != nil {
			for _, c := range n.Components {
				if strings.EqualFold(c, focus.Component) || c == focus.ComponentID {
					score += 3
				}
			}
			for _, t := range n.Tags {
				if strings.EqualFold(t, focus.Tier) || strings.EqualFold(t, focus.Component) {
					score += 2
				}
			}
		}
		if n.Type == "armadilha" || n.Type == "convencao" {
			score++
		}
		list = append(list, scored{n, score})
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].score != list[j].score {
			return list[i].score > list[j].score
		}
		return list[i].n.Updated > list[j].n.Updated
	})
	limit := 5
	if full {
		limit = 15
	}
	out := []NoteBrief{}
	for _, s := range list {
		if len(out) >= limit {
			break
		}
		out = append(out, NoteBrief{Slug: s.n.Slug, Title: s.n.Title, Type: s.n.Type, Summary: model.NoteSummary(&s.n)})
	}
	return out
}

// divergences compara o que a memória diz com o que o Git mostra. O Git vence.
func divergences(in Input, r *Resume) []string {
	out := []string{}
	g := in.Git
	if g.Available {
		for _, t := range r.MyWork {
			if t.Status == model.StatusInProgress && t.Branch != "" && g.Branch != "" && t.Branch != g.Branch {
				out = append(out, fmt.Sprintf("%s está em andamento na branch %s, mas você está em %s.", t.ID, t.Branch, g.Branch))
			}
		}
		work, studio := 0, 0
		for _, f := range g.Changed {
			if StudioFile(f) {
				studio++
			} else {
				work++
			}
		}
		if len(r.MyWork) == 0 && work > 0 {
			out = append(out, fmt.Sprintf("Há %d arquivo(s) de código alterado(s) sem tarefa em andamento no seu nome.", work))
		}
		if studio > 0 && work == 0 && len(r.MyWork) == 0 {
			out = append(out, fmt.Sprintf("Há %d alteração(ões) de planejamento em .arch/ sem commit: registre com `archcode-studio commit --plan`.", studio))
		}
		if g.Behind > 0 {
			out = append(out, fmt.Sprintf("A branch %s está %d commit(s) atrás de %s: rode git pull antes de continuar.", g.Branch, g.Behind, g.Upstream))
		}
	}
	// Commits integrados na branch principal que citam itens não concluídos.
	seen := map[string]bool{}
	for _, c := range in.MainCommits {
		if c.Merge() {
			continue
		}
		subject, _ := conventions.StripPRSuffix(c.Subject)
		for _, id := range model.ItemRefs(subject) {
			it := in.Plan.Item(id)
			if it == nil || seen[it.ID] || it.Archived || it.Status == model.StatusCompleted {
				continue
			}
			seen[it.ID] = true
			if conventions.IsReservation(in.Conventions, subject) {
				continue
			}
			out = append(out, fmt.Sprintf("O commit %s na branch principal cita %s, que ainda está %s no backlog: se o trabalho foi integrado, conclua a tarefa.",
				shortHash(c.Hash), it.ID, plan.StatusLabels[it.Status]))
		}
	}
	return out
}

// hints sugere o próximo passo, na ordem em que deve ser feito.
func hints(in Input, r *Resume) []string {
	out := []string{}
	if r.BacklogSize == 0 {
		out = append(out, "O backlog está vazio: gere-o a partir da arquitetura com sync_backlog (ou `archcode-studio backlog sync`).")
		return out
	}
	for _, t := range r.MyWork {
		switch {
		case t.Status == model.StatusInProgress && t.Handoff != nil && t.Handoff.NextStep != "":
			out = append(out, fmt.Sprintf("Continue %s: %s", t.ID, t.Handoff.NextStep))
		case t.Status == model.StatusInProgress:
			out = append(out, fmt.Sprintf("Continue %s (%s) e grave um checkpoint com save_checkpoint.", t.ID, t.Title))
		case t.Status == model.StatusReview:
			out = append(out, fmt.Sprintf("%s está em revisão: acompanhe o PR ou pegue a próxima tarefa.", t.ID))
		case t.Status == model.StatusBlocked:
			out = append(out, fmt.Sprintf("%s está bloqueada: resolva o impedimento ou libere a tarefa.", t.ID))
		}
		if len(out) >= 2 {
			break
		}
	}
	if len(r.MyWork) == 0 && r.Next != nil {
		out = append(out, fmt.Sprintf("Reserve %s (%s) com claim_task — branch sugerida: %s.", r.Next.ID, r.Next.Title, r.Next.SuggestedBranch))
	}
	if r.Sprint == nil {
		out = append(out, "Nenhuma sprint ativa: planeje uma com plan_sprint (ou `archcode-studio sprint new`).")
	} else if r.Sprint.DaysLeft == 0 {
		out = append(out, fmt.Sprintf("A %s terminou: encerre-a com `archcode-studio sprint close`.", r.Sprint.Name))
	}
	if len(r.MyWork) == 0 && r.Next == nil && r.Sprint != nil {
		out = append(out, "Nada pronto para você na sprint: veja itens bloqueados ou reservas paradas.")
	}
	if len(r.StaleClaims) > 0 {
		out = append(out, fmt.Sprintf("%d reserva(s) parada(s) de outras pessoas podem ser assumidas.", len(r.StaleClaims)))
	}
	return out
}

// studioPrefixes e studioFiles são os caminhos que o próprio Studio escreve
// (planejamento, memória, configuração dos agentes): não são o trabalho da
// tarefa.
var studioPrefixes = []string{".arch/", ".claude/", ".cursor/", "docs/sprints/"}
var studioFiles = map[string]bool{
	".gitattributes": true, ".mcp.json": true, "AGENTS.md": true, "CLAUDE.md": true, "CHANGELOG.md": true,
	".github/pull_request_template.md": true, ".github/workflows/archcode-conventions.yml": true,
}

// StudioFile informa se o caminho é escrito pelo Studio, e não código da tarefa.
func StudioFile(path string) bool {
	if studioFiles[path] {
		return true
	}
	for _, p := range studioPrefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
