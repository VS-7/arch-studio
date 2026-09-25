package app

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Sprints (RF033, RF035)
// ---------------------------------------------------------------------------

// SprintInput cria ou altera uma sprint. Campos vazios recebem os padrões.
type SprintInput struct {
	Number        int      `json:"number,omitempty"`
	Goal          *string  `json:"goal,omitempty"`
	Start         *string  `json:"start,omitempty"`
	End           *string  `json:"end,omitempty"`
	CapacityHours *float64 `json:"capacity_h,omitempty"`
}

// CreateSprint cria uma sprint planejada. Sem datas, ela começa no próximo
// dia útil depois da última sprint (ou hoje) e dura os dias úteis da convenção;
// sem capacidade, vale pessoas × horas por dia × dias úteis da precificação.
func (a *App) CreateSprint(in SprintInput, source string) (*model.Sprint, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	n := in.Number
	if n <= 0 {
		n = p.NextSprintNumber()
	}
	if p.Sprint(n) != nil {
		return nil, model.Invalid("a %s já existe", model.SprintName(n))
	}
	start := plan.NextWorkingDay(a.now())
	for _, s := range p.Sprints {
		if end, ok := plan.ParseDate(s.End); ok && !end.Before(start) {
			start = plan.NextWorkingDay(end.AddDate(0, 0, 1))
		}
	}
	sp := &model.Sprint{Number: n, Name: model.SprintName(n), Status: model.SprintPlanned,
		Start: plan.FormatDate(start), End: plan.FormatDate(plan.EndAfter(start, conv.Planning.SprintDays))}
	if err := a.applySprintInput(sp, in, true); err != nil {
		return nil, err
	}
	p.Sprints = append(p.Sprints, *sp)
	if err := a.commitPlan(p, nil, []*model.Sprint{sp}, source, sp.Name+" criada"); err != nil {
		return nil, err
	}
	return sp, nil
}

func (a *App) applySprintInput(sp *model.Sprint, in SprintInput, fresh bool) error {
	if in.Goal != nil {
		sp.Goal = strings.TrimSpace(*in.Goal)
	}
	if in.Start != nil {
		if _, ok := plan.ParseDate(*in.Start); !ok {
			return model.Invalid("data de início inválida: %q (use AAAA-MM-DD)", *in.Start)
		}
		sp.Start = *in.Start
	}
	if in.End != nil {
		if _, ok := plan.ParseDate(*in.End); !ok {
			return model.Invalid("data de fim inválida: %q (use AAAA-MM-DD)", *in.End)
		}
		sp.End = *in.End
	}
	if s, ok1 := plan.ParseDate(sp.Start); ok1 {
		if e, ok2 := plan.ParseDate(sp.End); ok2 && e.Before(s) {
			return model.Invalid("a sprint termina (%s) antes de começar (%s)", sp.End, sp.Start)
		}
	}
	switch {
	case in.CapacityHours != nil:
		if *in.CapacityHours < 0 {
			return model.Invalid("capacidade negativa")
		}
		sp.CapacityHours = *in.CapacityHours
	case fresh || in.Start != nil || in.End != nil:
		if pr, err := a.st.LoadPricing(); err == nil {
			sp.CapacityHours = plan.Capacity(sp.Start, sp.End, pr.TeamSize, pr.HoursPerDay)
		}
	}
	return nil
}

// UpdateSprint altera meta, datas ou capacidade.
func (a *App) UpdateSprint(n int, in SprintInput, source string) (*model.Sprint, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	sp := p.Sprint(n)
	if sp == nil {
		return nil, model.NotFound("a %s não existe", model.SprintName(n))
	}
	if err := a.applySprintInput(sp, in, false); err != nil {
		return nil, err
	}
	if err := a.commitPlan(p, nil, []*model.Sprint{sp}, source, sp.Name+" atualizada"); err != nil {
		return nil, err
	}
	return sp, nil
}

// StartSprint ativa a sprint. Só uma sprint fica ativa por vez.
func (a *App) StartSprint(n int, source string) (*model.Sprint, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	sp := p.Sprint(n)
	if sp == nil {
		return nil, model.NotFound("a %s não existe", model.SprintName(n))
	}
	if sp.Status == model.SprintClosed {
		return nil, model.Invalid("a %s já foi encerrada", sp.Name)
	}
	if act := p.ActiveSprint(); act != nil && act.Number != n {
		return nil, model.Invalid("a %s ainda está ativa: encerre-a antes (archcode-studio sprint close %d)", act.Name, act.Number)
	}
	sp.Status = model.SprintActive
	sp.StartedAt = a.timestamp()
	today := plan.NextWorkingDay(a.now())
	if start, ok := plan.ParseDate(sp.Start); !ok || start.After(today) {
		// Começar antes do planejado traz a sprint para hoje, com a mesma duração.
		days := 0
		if end, ok := plan.ParseDate(sp.End); ok && !start.IsZero() {
			days = plan.WorkingDays(start, end)
		}
		if days <= 0 {
			if conv, err := a.st.LoadConventions(); err == nil {
				days = conv.Planning.SprintDays
			}
		}
		sp.Start = plan.FormatDate(today)
		sp.End = plan.FormatDate(plan.EndAfter(today, days))
	}
	if err := a.commitPlan(p, nil, []*model.Sprint{sp}, source, sp.Name+" iniciada"); err != nil {
		return nil, err
	}
	return sp, nil
}

// SprintPlanResult é a proposta (ou o plano aplicado) de uma sprint.
type SprintPlanResult struct {
	Sprint   *model.Sprint    `json:"sprint"`
	Proposal *plan.Proposal   `json:"proposal"`
	Items    []model.WorkItem `json:"items"`
	Applied  bool             `json:"applied"`
	Warnings []string         `json:"warnings,omitempty"`
}

// PlanSprint propõe os itens que cabem na sprint (na ordem do backlog,
// respeitando dependências e capacidade). Com apply, move os itens para ela;
// ids, quando informados, substituem a proposta. Sprint inexistente é criada.
func (a *App) PlanSprint(n int, goal string, capacity float64, ids []string, apply bool, source string) (*SprintPlanResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	var p *model.Plan
	var err error
	if apply {
		p, err = a.loadPlan()
	} else {
		p, err = a.Plan()
	}
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		// Sem número: a sprint planejada mais próxima, ou uma nova.
		for _, s := range p.Sprints {
			if s.Status == model.SprintPlanned {
				n = s.Number
				break
			}
		}
		if n <= 0 {
			n = p.NextSprintNumber()
		}
	}
	sp := p.Sprint(n)
	created := false
	if sp == nil {
		start := plan.NextWorkingDay(a.now())
		for _, s := range p.Sprints {
			if end, ok := plan.ParseDate(s.End); ok && !end.Before(start) {
				start = plan.NextWorkingDay(end.AddDate(0, 0, 1))
			}
		}
		sp = &model.Sprint{Number: n, Name: model.SprintName(n), Status: model.SprintPlanned,
			Start: plan.FormatDate(start), End: plan.FormatDate(plan.EndAfter(start, conv.Planning.SprintDays))}
		if pr, err := a.st.LoadPricing(); err == nil {
			sp.CapacityHours = plan.Capacity(sp.Start, sp.End, pr.TeamSize, pr.HoursPerDay)
		}
		created = true
	}
	if sp.Status == model.SprintClosed {
		return nil, model.Invalid("a %s já foi encerrada", sp.Name)
	}
	if goal = strings.TrimSpace(goal); goal != "" {
		sp.Goal = goal
	}
	if capacity > 0 {
		sp.CapacityHours = capacity
	}
	res := &SprintPlanResult{Sprint: sp, Items: []model.WorkItem{}}
	if len(ids) > 0 {
		res.Proposal = &plan.Proposal{Items: []string{}, CapacityHours: sp.CapacityHours}
		for _, id := range ids {
			it := p.Item(id)
			if it == nil {
				return nil, itemNotFound(p, id)
			}
			res.Proposal.Items = append(res.Proposal.Items, it.ID)
			res.Proposal.Hours += it.EstimateHours
		}
	} else {
		res.Proposal = plan.Propose(p, sp.Number, sp.CapacityHours)
	}
	if sp.CapacityHours > 0 && res.Proposal.Hours > sp.CapacityHours {
		res.Warnings = append(res.Warnings, fmt.Sprintf("o planejado (%.0f h) passa da capacidade (%.0f h) em %.0f h",
			res.Proposal.Hours, sp.CapacityHours, res.Proposal.Hours-sp.CapacityHours))
	}
	for _, id := range res.Proposal.Items {
		res.Items = append(res.Items, *p.Item(id))
	}
	if !apply {
		return res, nil
	}
	changed := []*model.WorkItem{}
	for _, id := range res.Proposal.Items {
		it := p.Item(id)
		if it.Sprint != sp.Number {
			it.Sprint = sp.Number
			it.UpdatedAt = a.timestamp()
			changed = append(changed, it)
		}
	}
	if created {
		p.Sprints = append(p.Sprints, *sp)
		sp = p.Sprint(sp.Number)
		res.Sprint = sp
	} else {
		sp = p.Sprint(sp.Number)
	}
	if err := a.commitPlan(p, changed, []*model.Sprint{sp}, source,
		fmt.Sprintf("%s planejada com %d item(ns)", sp.Name, len(res.Proposal.Items))); err != nil {
		return nil, err
	}
	res.Applied = true
	return res, nil
}

// SprintStatus resume uma sprint (ativa, quando n = 0).
type SprintStatus struct {
	Sprint   *model.Sprint  `json:"sprint"`
	Stats    plan.Stats     `json:"stats"`
	DaysLeft int            `json:"days_left"`
	Items    []BacklogView  `json:"items"`
	Load     float64        `json:"load_h"`
	Sprints  []model.Sprint `json:"sprints"`
	Backlog  plan.Stats     `json:"backlog"`
}

// Sprint devolve o estado de uma sprint; n = 0 é a ativa (ou a próxima
// planejada).
func (a *App) Sprint(n int) (*SprintStatus, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	var sp *model.Sprint
	if n > 0 {
		sp = p.Sprint(n)
		if sp == nil {
			return nil, model.NotFound("a %s não existe", model.SprintName(n))
		}
	} else if sp = p.ActiveSprint(); sp == nil {
		for i := range p.Sprints {
			if p.Sprints[i].Status == model.SprintPlanned {
				sp = &p.Sprints[i]
				break
			}
		}
	}
	out := &SprintStatus{Sprints: p.Sprints, Items: []BacklogView{}, Backlog: plan.SprintStats(p, 0)}
	if sp == nil {
		return out, nil
	}
	out.Sprint = sp
	out.Stats = plan.SprintStats(p, sp.Number)
	out.Load = out.Stats.PlannedHours
	out.DaysLeft = plan.DaysLeft(sp, a.now())
	num := sp.Number
	items, _, err := a.Backlog(BacklogQuery{Sprint: &num})
	if err != nil {
		return nil, err
	}
	out.Items = items
	return out, nil
}

// CloseResult é o resultado do encerramento de uma sprint.
type CloseResult struct {
	Sprint       *model.Sprint `json:"sprint"`
	Report       string        `json:"report_file"`
	Carried      []string      `json:"carried"`
	CarriedTo    int           `json:"carried_to"`
	Stats        plan.Stats    `json:"stats"`
	Changelog    string        `json:"changelog_file,omitempty"`
	SuggestedTag string        `json:"suggested_tag"`
	TagCommand   string        `json:"tag_command"`
}

// CloseSprint encerra a sprint: itens não concluídos vão para carryTo (0 =
// backlog, -1 = próxima sprint, criada se preciso), o relatório vai para
// docs/sprints/sprint-NN.md e o CHANGELOG é atualizado. A tag é só sugerida:
// criá-la é decisão humana.
func (a *App) CloseSprint(n, carryTo int, source string) (*CloseResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	if n <= 0 {
		act := p.ActiveSprint()
		if act == nil {
			return nil, model.Invalid("nenhuma sprint ativa para encerrar")
		}
		n = act.Number
	}
	sp := p.Sprint(n)
	if sp == nil {
		return nil, model.NotFound("a %s não existe", model.SprintName(n))
	}
	if sp.Status == model.SprintClosed {
		return nil, model.Invalid("a %s já foi encerrada", sp.Name)
	}
	stats := plan.SprintStats(p, sp.Number)
	sprints := []*model.Sprint{}
	if carryTo < 0 {
		carryTo = sp.Number + 1
		if p.Sprint(carryTo) == nil {
			start := plan.NextWorkingDay(a.now())
			if end, ok := plan.ParseDate(sp.End); ok {
				start = plan.NextWorkingDay(end.AddDate(0, 0, 1))
			}
			next := model.Sprint{Number: carryTo, Name: model.SprintName(carryTo), Status: model.SprintPlanned,
				Start: plan.FormatDate(start), End: plan.FormatDate(plan.EndAfter(start, conv.Planning.SprintDays))}
			if pr, err := a.st.LoadPricing(); err == nil {
				next.CapacityHours = plan.Capacity(next.Start, next.End, pr.TeamSize, pr.HoursPerDay)
			}
			p.Sprints = append(p.Sprints, next)
			sp = p.Sprint(n)
		}
	} else if carryTo > 0 && p.Sprint(carryTo) == nil {
		return nil, model.NotFound("a %s (destino dos itens não concluídos) não existe", model.SprintName(carryTo))
	}

	carried := []string{}
	changed := []*model.WorkItem{}
	for i := range p.Items {
		it := &p.Items[i]
		if it.Sprint != sp.Number || !it.Actionable() || it.Archived || it.Status == model.StatusCompleted {
			continue
		}
		it.Sprint = carryTo
		it.UpdatedAt = a.timestamp()
		carried = append(carried, it.ID)
		changed = append(changed, it)
	}
	sp.Status = model.SprintClosed
	sp.ClosedAt = a.timestamp()
	// Encerrada antes do fim planejado: o período registrado é o real.
	if end, ok := plan.ParseDate(sp.End); ok && a.now().Before(end) {
		sp.End = plan.FormatDate(a.now())
	}
	sprints = append(sprints, sp)
	if carryTo > 0 {
		sprints = append(sprints, p.Sprint(carryTo))
	}

	// Commits e sessões da sprint para o relatório.
	commits := []plan.CommitInfo{}
	if a.git.Available() {
		main := conv.Git.MainBranch
		if !a.git.RevExists(main) {
			main = "HEAD"
		}
		if log, err := a.git.Log(main, 500); err == nil {
			for _, c := range log {
				subject, _ := conventions.StripPRSuffix(c.Subject)
				if parsed, ok := conventions.ParseSubject(conv, subject); ok && parsed.Sprint == sp.Number && !c.Merge() {
					commits = append(commits, plan.CommitInfo{Hash: c.Hash, Subject: c.Subject, Author: c.Author, Date: c.Date})
				}
			}
		}
	}
	sessions := []model.Session{}
	if all, err := a.st.ListSessions(0); err == nil {
		for _, s := range all {
			if s.Sprint == sp.Number {
				sessions = append(sessions, s)
			}
		}
	}
	// Relatório com os itens ainda na sprint (antes da transferência).
	snapshot := *p
	snapshot.Items = append([]model.WorkItem(nil), p.Items...)
	for i := range snapshot.Items {
		if containsID(carried, snapshot.Items[i].ID) {
			snapshot.Items[i].Sprint = sp.Number
		}
	}
	report := plan.Report(plan.ReportInput{Plan: &snapshot, Sprint: sp, Commits: commits, Sessions: sessions,
		CarriedTo: carryTo, Carried: carried})
	reportPath := store.DirSprintReports + "/" + strings.TrimSuffix(model.SprintFileName(sp.Number), ".yaml") + ".md"
	if err := a.st.WriteFile(reportPath, []byte(report)); err != nil {
		return nil, err
	}
	if err := a.commitPlan(p, changed, sprints, source,
		fmt.Sprintf("%s encerrada: %d/%d concluídas", sp.Name, stats.Completed, stats.Total)); err != nil {
		return nil, err
	}
	res := &CloseResult{Sprint: sp, Report: reportPath, Carried: carried, CarriedTo: carryTo, Stats: stats}
	if path, err := a.writeChangelogLocked(conv, p); err == nil {
		res.Changelog = path
	}
	res.SuggestedTag = conventions.Render(conv.Tags.Sprint, conventions.Vars{Sprint: sp.Number})
	res.TagCommand = fmt.Sprintf("git tag -a %s -m %q && git push %s %s", res.SuggestedTag, sp.Name+" encerrada",
		conv.Git.Remote, res.SuggestedTag)
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: reportPath, Message: "Relatório da " + sp.Name + " gerado"})
	return res, nil
}
