package app

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/memory"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/skills"
)

// ---------------------------------------------------------------------------
// Execução de uma tarefa: reservar, registrar checkpoint, concluir
// (RF049, RF056, RF057, RF063)
// ---------------------------------------------------------------------------

// ClaimOptions controla a reserva de uma tarefa.
type ClaimOptions struct {
	// Person é quem reserva ("" = identidade do Git).
	Person string
	// Agent é a ferramenta de IA que trabalha em nome da pessoa ("" = humano).
	Agent string
	// Switch passa a trabalhar na branch da tarefa.
	Switch bool
	// Push publica a reserva no remoto (commit vazio de reserva + push).
	Push bool
	// Force assume a tarefa mesmo reservada por outra pessoa ou bloqueada.
	Force bool
}

// ClaimResult descreve a reserva.
type ClaimResult struct {
	Task              *model.WorkItem `json:"task"`
	Branch            string          `json:"branch"`
	BranchCreated     bool            `json:"branch_created"`
	Switched          bool            `json:"switched"`
	Pushed            bool            `json:"pushed"`
	ReservationCommit string          `json:"reservation_commit,omitempty"`
	// Remote: confirmed (consultado e livre), unconfirmed (sem acesso ao
	// remoto) ou none (repositório sem remoto / sem Git).
	Remote      string   `json:"remote"`
	Warnings    []string `json:"warnings,omitempty"`
	NextCommand string   `json:"next_command,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}

// ClaimTask reserva a tarefa para a pessoa. A reserva é a própria branch da
// convenção: antes de reservar, o Studio consulta o remoto; se outra pessoa
// já publicou a branch da tarefa, a reserva é recusada.
func (a *App) ClaimTask(id string, opts ClaimOptions, source string) (*ClaimResult, error) {
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
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	if !item.Actionable() {
		return nil, model.Invalid("%s é um %s: reserve uma das tarefas dele", item.ID, strings.ToLower(model.ItemTypeLabels[item.Type]))
	}
	if item.Archived || item.Status == model.StatusCompleted {
		return nil, model.Invalid("%s já está %s", item.ID, map[bool]string{true: "arquivada", false: "concluída"}[item.Archived])
	}
	person := strings.TrimSpace(opts.Person)
	if person == "" {
		person = a.Person()
	}
	if person == "" {
		return nil, model.Invalid("não sei quem está reservando: configure git config user.name e user.email")
	}
	if item.Assignee != "" && !plan.SamePerson(item.Assignee, person) && !opts.Force &&
		!plan.Stale(item, a.now(), conv.Planning.StaleDays) {
		return nil, model.Invalid("%s está reservada por %s desde %s; escolha outra tarefa (ou use force para assumir)",
			item.ID, plan.DisplayName(item.Assignee), dateOf(item.ClaimedAt))
	}
	if blocked := plan.BlockedBy(item, p); len(blocked) > 0 && !opts.Force {
		return nil, model.Invalid("%s aguarda %s; termine as dependências antes (ou use force)", item.ID, strings.Join(blocked, ", "))
	}

	sprint := item.Sprint
	res := &ClaimResult{Remote: "none"}
	if sprint == 0 {
		if act := p.ActiveSprint(); act != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("%s não está na %s ativa", item.ID, act.Name))
		}
	}
	branch := item.Branch
	if branch == "" {
		branch = conventions.BranchName(conv, item, sprint)
	}
	res.Branch = branch

	if a.git.Available() {
		if err := a.checkRemoteClaim(conv, item, branch, person, opts.Force, res); err != nil {
			return nil, err
		}
		base := conv.Git.MainBranch
		if !a.git.RevExists(base) {
			base = ""
		}
		current := a.git.CurrentBranch()
		switch {
		case !a.git.BranchExists(branch):
			if err := a.git.CreateBranch(branch, base, opts.Switch); err != nil {
				return nil, err
			}
			res.BranchCreated, res.Switched = true, opts.Switch
		case opts.Switch && current != branch:
			if err := a.git.Switch(branch); err != nil {
				return nil, err
			}
			res.Switched = true
		}
		onBranch := a.git.CurrentBranch() == branch
		if opts.Push {
			switch {
			case !onBranch:
				res.Warnings = append(res.Warnings, "reserva não publicada: é preciso estar na branch da tarefa (switch)")
			default:
				if err := a.publishClaim(conv, item, sprint, base, branch, res); err != nil {
					res.Warnings = append(res.Warnings, "não foi possível publicar a reserva: "+err.Error())
				}
			}
		}
		if !res.Pushed && a.git.RemoteExists(conv.Git.Remote) {
			res.NextCommand = fmt.Sprintf("git push -u %s %s", conv.Git.Remote, branch)
		}
	} else {
		res.Warnings = append(res.Warnings, "Git indisponível: a reserva vale só neste backlog")
	}

	now := a.timestamp()
	if item.Status == model.StatusPending || item.Status == model.StatusBlocked {
		item.Status = model.StatusInProgress
	}
	if !plan.SamePerson(item.Assignee, person) {
		item.ClaimedAt = now
	}
	if item.ClaimedAt == "" {
		item.ClaimedAt = now
	}
	item.Assignee, item.Agent, item.Branch, item.UpdatedAt = person, strings.TrimSpace(opts.Agent), branch, now
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source,
		fmt.Sprintf("%s reservada por %s", item.ID, plan.DisplayName(person))); err != nil {
		return nil, err
	}
	res.Task = item
	for _, s := range a.skillsFor(item) {
		res.Skills = append(res.Skills, s.Name)
	}
	return res, nil
}

// checkRemoteClaim recusa a reserva se outra pessoa já publicou a branch da
// tarefa. Sem acesso ao remoto, a reserva segue como não confirmada.
func (a *App) checkRemoteClaim(conv *model.Conventions, item *model.WorkItem, branch, person string, force bool, res *ClaimResult) error {
	remote := conv.Git.Remote
	if !a.git.RemoteExists(remote) {
		return nil
	}
	refs, err := a.git.RemoteBranches(remote, "")
	if err != nil {
		res.Remote = "unconfirmed"
		res.Warnings = append(res.Warnings, fmt.Sprintf("não foi possível consultar %s (%v): reserva não confirmada até o próximo push", remote, err))
		return nil
	}
	res.Remote = "confirmed"
	for _, ref := range refs {
		if !gitx.MatchGlob("*/"+item.ID+"-*", ref.Name) && !gitx.MatchGlob("*/"+item.ID, ref.Name) {
			continue
		}
		tip, err := a.git.FetchTip(remote, ref.Name)
		if err != nil {
			if force {
				continue
			}
			return model.Invalid("%s já tem a branch %s no %s; não consegui ver de quem é (%v)", item.ID, ref.Name, remote, err)
		}
		owner := tip.Author
		if tip.Email != "" {
			owner += " <" + tip.Email + ">"
		}
		if plan.SamePerson(owner, person) || force {
			continue
		}
		return model.Invalid("%s já está reservada no %s por %s desde %s (branch %s); escolha outra tarefa",
			item.ID, remote, tip.Author, dateOf(tip.Date), ref.Name)
	}
	return nil
}

// publishClaim grava o commit vazio de reserva (se a branch ainda não tem
// commits próprios) e publica a branch.
func (a *App) publishClaim(conv *model.Conventions, item *model.WorkItem, sprint int, base, branch string, res *ClaimResult) error {
	own := []gitx.Commit{}
	if base != "" {
		own, _ = a.git.Log(base+".."+branch, 1)
	}
	if len(own) == 0 {
		staged, _ := a.git.StagedFiles()
		if len(staged) > 0 {
			return fmt.Errorf("há arquivos preparados (git add); o commit de reserva os levaria junto")
		}
		subject := conventions.CommitSubject(conv, conventions.CommitInput{Item: item, Sprint: sprint, Summary: "Reserva " + item.ID})
		hash, err := a.git.Commit(subject+"\n\nReserva feita pelo ArchCode Studio.\n\nTask: "+item.ID+"\n", true)
		if err != nil {
			return err
		}
		res.ReservationCommit = hash
	}
	if err := a.git.Push(conv.Git.Remote, branch, true); err != nil {
		return err
	}
	res.Pushed = true
	return nil
}

// ReleaseTask libera a reserva. A tarefa volta a pendente e o checkpoint
// fica, para quem assumir continuar dali. Liberar a reserva de outra pessoa
// só vale para reservas paradas (ou com force).
func (a *App) ReleaseTask(id, reason string, force bool, source string) (*model.WorkItem, error) {
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
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	person := a.Person()
	if item.Assignee == "" {
		return nil, model.Invalid("%s não está reservada", item.ID)
	}
	if !plan.SamePerson(item.Assignee, person) && !force && !plan.Stale(item, a.now(), conv.Planning.StaleDays) {
		return nil, model.Invalid("%s está com %s e não está parada; peça para a pessoa liberar (ou use force)",
			item.ID, plan.DisplayName(item.Assignee))
	}
	note := fmt.Sprintf("Liberada por %s em %s", orName(plan.DisplayName(person)), a.timestamp()[:10])
	if reason = strings.TrimSpace(reason); reason != "" {
		note += ": " + reason
	}
	if item.Handoff == nil {
		item.Handoff = &model.Handoff{}
	}
	item.Handoff.Notes = note
	item.Handoff.UpdatedAt, item.Handoff.By = a.timestamp(), plan.DisplayName(person)
	if item.Status == model.StatusInProgress {
		item.Status = model.StatusPending
	}
	item.Assignee, item.Agent, item.ClaimedAt, item.UpdatedAt = "", "", "", a.timestamp()
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source, item.ID+" liberada"); err != nil {
		return nil, err
	}
	return item, nil
}

// HandoffInput atualiza o checkpoint. Campos nil ficam como estão.
type HandoffInput struct {
	LastStep     *string   `json:"last_step,omitempty"`
	NextStep     *string   `json:"next_step,omitempty"`
	Files        *[]string `json:"files,omitempty"`
	FailingTests *[]string `json:"failing_tests,omitempty"`
	Notes        *string   `json:"notes,omitempty"`
}

// SaveCheckpoint grava o checkpoint de retomada da tarefa.
func (a *App) SaveCheckpoint(id string, in HandoffInput, source string) (*model.WorkItem, error) {
	texts := []string{}
	for _, s := range []*string{in.LastStep, in.NextStep, in.Notes} {
		if s != nil {
			texts = append(texts, *s)
		}
	}
	if found := memory.FindSecrets(texts...); len(found) > 0 {
		return nil, model.Invalid("o checkpoint parece conter um segredo (%s); ele vai para o Git — remova antes de gravar", strings.Join(found, ", "))
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	h := item.Handoff
	if h == nil {
		h = &model.Handoff{}
	}
	if in.LastStep != nil {
		h.LastStep = strings.TrimSpace(*in.LastStep)
	}
	if in.NextStep != nil {
		h.NextStep = strings.TrimSpace(*in.NextStep)
	}
	if in.Files != nil {
		h.Files = cleanList(*in.Files)
	}
	if in.FailingTests != nil {
		h.FailingTests = cleanList(*in.FailingTests)
	}
	if in.Notes != nil {
		h.Notes = strings.TrimSpace(*in.Notes)
	}
	h.UpdatedAt, h.By = a.timestamp(), plan.DisplayName(a.Person())
	item.Handoff, item.UpdatedAt = h, a.timestamp()
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source, "Checkpoint de "+item.ID); err != nil {
		return nil, err
	}
	return item, nil
}

// AutoCheckpoint grava, sem intervenção, os arquivos alterados na tarefa em
// andamento da pessoa (a da branch atual, ou a única). Usado pelos hooks de
// compactação e fim de sessão do agente. nil = nada a registrar.
func (a *App) AutoCheckpoint(source string) (*model.WorkItem, error) {
	if !a.git.Available() {
		return nil, nil
	}
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	mine := plan.MyWork(p, a.Person())
	current := a.git.CurrentBranch()
	var target *model.WorkItem
	for _, it := range mine {
		if it.Status == model.StatusInProgress && it.Branch == current {
			target = it
		}
	}
	if target == nil && len(mine) == 1 && mine[0].Status == model.StatusInProgress {
		target = mine[0]
	}
	if target == nil {
		return nil, nil
	}
	st, err := a.git.Status()
	if err != nil {
		return nil, nil
	}
	files := []string{}
	for _, c := range st.Changed {
		if !memory.StudioFile(c.Path) {
			files = append(files, c.Path)
		}
		if len(files) >= 30 {
			break
		}
	}
	if len(files) == 0 || (target.Handoff != nil && strings.Join(target.Handoff.Files, ",") == strings.Join(files, ",")) {
		return nil, nil
	}
	notes := "Checkpoint automático: arquivos alterados sem commit."
	return a.SaveCheckpoint(target.ID, HandoffInput{Files: &files, Notes: &notes}, source)
}

// CompleteInput conclui uma tarefa.
type CompleteInput struct {
	Checks []model.CheckResult `json:"checks"`
	Notes  string              `json:"notes,omitempty"`
	// Status: review ou completed ("" = review quando há branch da tarefa,
	// completed quando não há).
	Status string `json:"status,omitempty"`
	// Force conclui mesmo com checks obrigatórios faltando (fica registrado).
	Force bool `json:"force,omitempty"`
}

// CompleteResult descreve a conclusão (ou a recusa).
type CompleteResult struct {
	Task      *model.WorkItem `json:"task,omitempty"`
	Completed bool            `json:"completed"`
	Status    string          `json:"status,omitempty"`
	Missing   []string        `json:"missing_checks,omitempty"`
	Failed    []string        `json:"failed_checks,omitempty"`
	Required  []RequiredCheck `json:"required_checks,omitempty"`
	Warnings  []string        `json:"warnings,omitempty"`
	Message   string          `json:"message"`
	NextSteps []string        `json:"next_steps,omitempty"`
}

// RequiredCheck é um check obrigatório que a conclusão espera.
type RequiredCheck struct {
	Skill   string `json:"skill"`
	Check   string `json:"check"`
	Text    string `json:"text"`
	Command string `json:"command,omitempty"`
}

// CompleteTask conclui a tarefa com os resultados dos checks das skills que
// valem para ela. Faltando check obrigatório (ou com check falho), a
// conclusão é recusada e a resposta diz o que falta.
func (a *App) CompleteTask(id string, in CompleteInput, source string) (*CompleteResult, error) {
	for i := range in.Checks {
		c := &in.Checks[i]
		c.Skill, c.Check = strings.TrimSpace(c.Skill), strings.TrimSpace(c.Check)
		c.Result = model.NormalizeCheckResult(c.Result)
	}
	evidence := []string{in.Notes}
	for _, c := range in.Checks {
		evidence = append(evidence, c.Evidence)
	}
	if found := memory.FindSecrets(evidence...); len(found) > 0 {
		return nil, model.Invalid("as evidências parecem conter um segredo (%s); remova antes de concluir", strings.Join(found, ", "))
	}
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
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	if !item.Actionable() {
		return nil, model.Invalid("%s não é uma tarefa executável", item.ID)
	}
	applicable := a.skillsFor(item)
	missing, failed := skills.MissingChecks(applicable, in.Checks)
	res := &CompleteResult{Missing: missing, Failed: failed}
	if (len(missing) > 0 || len(failed) > 0) && !in.Force {
		for _, s := range applicable {
			if !s.GatesTask() {
				continue
			}
			for _, c := range s.Checks {
				if c.Required {
					res.Required = append(res.Required, RequiredCheck{Skill: s.Name, Check: c.ID, Text: c.Text, Command: c.Command})
				}
			}
		}
		parts := []string{}
		if len(missing) > 0 {
			parts = append(parts, "faltam os checks "+strings.Join(missing, ", "))
		}
		if len(failed) > 0 {
			parts = append(parts, "falharam "+strings.Join(failed, ", "))
		}
		res.Message = fmt.Sprintf("%s não foi concluída: %s. Informe cada check obrigatório (ok, fail ou na) com a evidência, ou corrija o que falhou.",
			item.ID, strings.Join(parts, "; "))
		return res, nil
	}

	// Registra os checks (o mais recente de cada skill/check vale).
	merged := append([]model.CheckResult{}, item.Checks...)
	for _, c := range in.Checks {
		replaced := false
		for i := range merged {
			if merged[i].Skill == c.Skill && merged[i].Check == c.Check {
				merged[i], replaced = c, true
			}
		}
		if !replaced {
			merged = append(merged, c)
		}
	}
	item.Checks = merged
	for i := range item.Acceptance {
		item.Acceptance[i].Done = true
	}
	if notes := strings.TrimSpace(in.Notes); notes != "" {
		item.Notes = strings.TrimSpace(item.Notes + "\n" + fmt.Sprintf("- %s (conclusão): %s", a.timestamp()[:10], notes))
	}
	if in.Force && (len(missing) > 0 || len(failed) > 0) {
		item.Notes = strings.TrimSpace(item.Notes + "\n" + fmt.Sprintf("- %s: concluída sem os checks %s", a.timestamp()[:10],
			strings.Join(append(missing, failed...), ", ")))
	}

	status := model.NormalizeStatus(in.Status)
	if in.Status == "" {
		status = model.StatusCompleted
		if a.git.Available() && item.Branch != "" && item.Branch != conv.Git.MainBranch {
			status = model.StatusReview
		}
	}
	if status != model.StatusReview && status != model.StatusCompleted {
		return nil, model.Invalid("status de conclusão inválido: %q (use review ou completed)", in.Status)
	}
	a.setStatus(item, status)
	item.UpdatedAt = a.timestamp()
	res.Warnings = a.scopeWarnings(p, item, conv)
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source,
		fmt.Sprintf("%s → %s", item.ID, plan.StatusLabels[item.Status])); err != nil {
		return nil, err
	}
	res.Task, res.Completed, res.Status = item, true, item.Status
	if item.Status == model.StatusReview {
		res.Message = fmt.Sprintf("%s pronta para revisão.", item.ID)
		res.NextSteps = []string{"propose_commit para o commit na convenção (se ainda houver mudanças sem commit)",
			"prepare_pull_request para o título e o corpo do PR"}
	} else {
		res.Message = fmt.Sprintf("%s concluída.", item.ID)
	}
	return res, nil
}

// RequiredChecksFor lista os checks obrigatórios que a conclusão da tarefa
// vai exigir (sem alterar nada).
func (a *App) RequiredChecksFor(id string) ([]RequiredCheck, error) {
	item, err := a.Item(id)
	if err != nil {
		return nil, err
	}
	out := []RequiredCheck{}
	for _, s := range a.skillsFor(item) {
		if !s.GatesTask() {
			continue
		}
		for _, c := range s.Checks {
			if c.Required {
				out = append(out, RequiredCheck{Skill: s.Name, Check: c.ID, Text: c.Text, Command: c.Command})
			}
		}
	}
	return out, nil
}

// scopeWarnings aponta arquivos alterados que também estão no checkpoint de
// outra tarefa em andamento: sinal de que o trabalho vazou de escopo (RF063).
func (a *App) scopeWarnings(p *model.Plan, item *model.WorkItem, conv *model.Conventions) []string {
	if !a.git.Available() {
		return nil
	}
	changed, err := a.git.ChangedSince(conv.Git.MainBranch)
	if err != nil {
		return nil
	}
	owners := map[string]string{}
	for _, other := range p.Items {
		if other.ID == item.ID || other.Status != model.StatusInProgress || other.Handoff == nil {
			continue
		}
		for _, f := range other.Handoff.Files {
			owners[f] = other.ID
		}
	}
	out := []string{}
	for _, f := range changed {
		if owner, ok := owners[f]; ok {
			out = append(out, fmt.Sprintf("%s também está no checkpoint de %s (em andamento): confira se a mudança é do escopo de %s", f, owner, item.ID))
		}
	}
	return out
}

func cleanList(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func dateOf(ts string) string {
	if len(ts) >= 10 {
		return ts[:10]
	}
	if ts == "" {
		return "data desconhecida"
	}
	return ts
}

func orName(s string) string {
	if s == "" {
		return "alguém"
	}
	return s
}
