package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/archcode/studio/internal/conventions"
	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/skills"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Convenções de Git orientadas a sprint (RF038–RF042, RF058, RF061)
// ---------------------------------------------------------------------------

// Conventions devolve as convenções do projeto.
func (a *App) Conventions() (*model.Conventions, error) { return a.st.LoadConventions() }

// HasConventions informa se o projeto já tem .arch/conventions.yaml.
func (a *App) HasConventions() bool { return a.st.HasConventions() }

// SaveConventions grava as convenções e regenera a skill convencoes-git.
func (a *App) SaveConventions(c *model.Conventions, source string) (*model.Conventions, error) {
	c.Normalize()
	for _, pattern := range []string{c.Git.Branch, c.Git.BranchOffSprint, c.Git.Commit, c.Git.CommitOffSprint, c.PullRequest.Title} {
		if _, err := conventions.Compile(pattern, c); err != nil {
			return nil, model.Invalid("padrão inválido %q: %v", pattern, err)
		}
	}
	if !conventions.Has(c.Git.Commit, "id") && c.Git.RequireID {
		return nil, model.Invalid("o padrão de commit precisa ter {id} quando require_id está ligado")
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	if err := a.st.SaveConventions(c); err != nil {
		return nil, err
	}
	a.refreshGeneratedSkill(c)
	a.emit(hub.Event{Type: hub.EventConventions, Source: source, Path: store.FileConventions, Message: "Convenções atualizadas"})
	return c, nil
}

// InitConventions cria o .arch/conventions.yaml com o preset informado.
func (a *App) InitConventions(preset string, force bool, source string) (*model.Conventions, error) {
	if a.st.HasConventions() && !force {
		return nil, model.Invalid("%s já existe (use force para recriar)", store.FileConventions)
	}
	return a.SaveConventions(model.DefaultConventions(preset), source)
}

// ---------------------------------------------------------------------------
// Commit
// ---------------------------------------------------------------------------

// CommitProposal é a mensagem de commit da convenção para a tarefa.
type CommitProposal struct {
	TaskID        string   `json:"task_id"`
	Subject       string   `json:"subject"`
	Message       string   `json:"message"`
	Branch        string   `json:"branch,omitempty"`
	CurrentBranch string   `json:"current_branch,omitempty"`
	Staged        []string `json:"staged"`
	Warnings      []string `json:"warnings,omitempty"`
	Committed     bool     `json:"committed"`
	Hash          string   `json:"hash,omitempty"`
	Command       string   `json:"command,omitempty"`
}

// CommitInput pede uma mensagem (ou um commit) para a tarefa.
type CommitRequest struct {
	TaskID   string `json:"task_id,omitempty"`
	Summary  string `json:"summary,omitempty"`
	Body     string `json:"body,omitempty"`
	CoAuthor string `json:"co_author,omitempty"`
	// Commit grava de fato (só os arquivos preparados com git add).
	Commit bool `json:"commit,omitempty"`
	// Plan faz o commit de planejamento: prepara .arch/ e os arquivos
	// gerados pelo Studio e usa o id reservado da sprint ([SPRINT-01]).
	Plan bool `json:"plan,omitempty"`
}

// studioPaths são os caminhos que o Studio escreve no planejamento.
var studioPaths = []string{store.DirArch, ".gitattributes", store.DirSprintReports, store.FileChangelog}

// currentTask descobre a tarefa em andamento da pessoa na branch atual.
func (a *App) currentTask(p *model.Plan) *model.WorkItem {
	branch := ""
	if a.git.Available() {
		branch = a.git.CurrentBranch()
	}
	mine := plan.MyWork(p, a.Person())
	for _, it := range mine {
		if branch != "" && it.Branch == branch {
			return it
		}
	}
	if len(mine) == 1 {
		return mine[0]
	}
	return nil
}

// ProposeCommit monta a mensagem da convenção a partir da tarefa (a da branch
// atual, se nenhuma for informada) e, com Commit, grava o commit.
func (a *App) ProposeCommit(req CommitRequest) (*CommitProposal, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	var item *model.WorkItem
	switch {
	case req.Plan:
		// Commit de planejamento: id reservado da sprint ativa (ou da próxima
		// planejada); sem sprint, o id de manutenção do backlog.
		item = &model.WorkItem{ID: conventions.PlanRef, Type: model.ItemSpike, Title: "Atualiza o planejamento"}
		sp := p.ActiveSprint()
		if sp == nil {
			for i := range p.Sprints {
				if p.Sprints[i].Status == model.SprintPlanned {
					sp = &p.Sprints[i]
					break
				}
			}
		}
		if sp != nil {
			item.ID, item.Sprint = conventions.SprintRefID(sp.Number), sp.Number
			item.Title = "Atualiza o planejamento da " + sp.Name
		}
		if req.Summary == "" {
			req.Summary = item.Title
		}
		if req.Commit && a.git.Available() {
			existing := []string{}
			for _, p := range studioPaths {
				if a.st.Exists(p) {
					existing = append(existing, p)
				}
			}
			if err := a.git.Add(existing); err != nil {
				return nil, err
			}
		}
	case req.TaskID != "":
		if item = p.Item(req.TaskID); item == nil {
			return nil, itemNotFound(p, req.TaskID)
		}
	default:
		if item = a.currentTask(p); item == nil {
			return nil, model.Invalid("informe a tarefa (task_id): nenhuma tarefa sua em andamento nesta branch (para planejamento, use plan)")
		}
	}
	sprint := item.Sprint
	msg := conventions.CommitMessage(conv, conventions.CommitInput{Item: item, Sprint: sprint, Summary: req.Summary,
		Body: req.Body, CoAuthor: req.CoAuthor})
	subject, _, _ := strings.Cut(msg, "\n")
	out := &CommitProposal{TaskID: item.ID, Subject: subject, Message: msg, Branch: item.Branch, Staged: []string{}}
	for _, is := range conventions.ValidateSubject(conv, subject, p) {
		out.Warnings = append(out.Warnings, is.Message)
	}
	if !a.git.Available() {
		out.Warnings = append(out.Warnings, "Git indisponível")
		return out, nil
	}
	out.CurrentBranch = a.git.CurrentBranch()
	if !req.Plan && item.Branch != "" && out.CurrentBranch != item.Branch {
		out.Warnings = append(out.Warnings, fmt.Sprintf("você está em %s, mas a branch da tarefa é %s", out.CurrentBranch, item.Branch))
	}
	if staged, err := a.git.StagedFiles(); err == nil {
		out.Staged = staged
	}
	out.Command = "git commit -F - <<'MSG'\n" + msg + "MSG"
	if !req.Commit {
		if len(out.Staged) == 0 {
			out.Warnings = append(out.Warnings, "nenhum arquivo preparado: rode git add antes do commit")
		}
		return out, nil
	}
	if len(out.Staged) == 0 {
		return nil, model.Invalid("nada preparado para commit: rode git add nos arquivos da tarefa")
	}
	if !req.Plan && item.File != "" {
		// O arquivo da tarefa (status, checkpoint, checks) e o índice derivado
		// vão junto: o PR leva o quadro atualizado.
		paths := []string{item.File}
		if a.st.Exists(store.FileTasks) {
			paths = append(paths, store.FileTasks)
		}
		_ = a.git.Add(paths)
	}
	hash, err := a.git.Commit(msg, false)
	if err != nil {
		return nil, err
	}
	out.Committed, out.Hash, out.Command = true, hash, ""
	a.emit(hub.Event{Type: hub.EventGit, Source: hub.SourceCLI, Message: "Commit " + shortHash(hash) + ": " + subject})
	return out, nil
}

// ---------------------------------------------------------------------------
// Pull request
// ---------------------------------------------------------------------------

// PRProposal é o pull request da convenção para as tarefas.
type PRProposal struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Base     string   `json:"base"`
	Head     string   `json:"head"`
	Items    []string `json:"items"`
	Warnings []string `json:"warnings,omitempty"`
	Opened   bool     `json:"opened"`
	URL      string   `json:"url,omitempty"`
	Command  string   `json:"command,omitempty"`
}

// PreparePullRequest gera título e corpo do PR. Sem ids, valem as tarefas da
// pessoa na branch atual (em andamento ou em revisão).
func (a *App) PreparePullRequest(ids []string) (*PRProposal, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	items := []*model.WorkItem{}
	for _, id := range ids {
		it := p.Item(id)
		if it == nil {
			return nil, itemNotFound(p, id)
		}
		items = append(items, it)
	}
	if len(items) == 0 {
		branch := ""
		if a.git.Available() {
			branch = a.git.CurrentBranch()
		}
		for _, it := range plan.MyWork(p, a.Person()) {
			if branch == "" || it.Branch == branch {
				items = append(items, it)
			}
		}
	}
	if len(items) == 0 {
		return nil, model.Invalid("informe as tarefas do PR: nenhuma tarefa sua nesta branch")
	}
	var sp *model.Sprint
	if n := items[0].Sprint; n > 0 {
		sp = p.Sprint(n)
	}
	checks := []conventions.SkillCheck{}
	seen := map[string]bool{}
	for _, it := range items {
		for _, s := range a.skillsFor(it) {
			if !s.GatesTask() {
				continue
			}
			for _, c := range s.Checks {
				if c.Required && !seen[s.Name+c.ID] {
					seen[s.Name+c.ID] = true
					checks = append(checks, conventions.SkillCheck{Skill: s.Name, Check: c.ID})
				}
			}
		}
	}
	validation := ""
	if rep, err := a.Validate(); err == nil {
		if rep.Errors == 0 {
			validation = fmt.Sprintf("sem erros (%d aviso(s))", rep.Warnings)
		} else {
			validation = fmt.Sprintf("%d erro(s), %d aviso(s)", rep.Errors, rep.Warnings)
		}
	}
	in := conventions.PRInput{Items: items, Sprint: sp, Validation: validation, Skills: checks}
	if conv.PullRequest.Granularity == "story" && strings.HasPrefix(items[0].Parent, "ST-") {
		if story := p.Item(items[0].Parent); story != nil {
			in.Title = story.Title
		}
	}
	out := &PRProposal{Title: conventions.PRTitle(conv, in), Body: conventions.PRBody(conv, in), Base: conv.Git.MainBranch}
	for _, it := range items {
		out.Items = append(out.Items, it.ID)
		if it.Status != model.StatusReview && it.Status != model.StatusCompleted {
			out.Warnings = append(out.Warnings, fmt.Sprintf("%s ainda está %s: conclua com complete_task antes do PR", it.ID, plan.StatusLabels[it.Status]))
		}
	}
	out.Head = items[0].Branch
	if out.Head == "" && a.git.Available() {
		out.Head = a.git.CurrentBranch()
	}
	for _, is := range conventions.ValidateBranch(conv, out.Head) {
		out.Warnings = append(out.Warnings, is.Message)
	}
	out.Command = fmt.Sprintf("gh pr create --base %s --head %s --title %q --body-file pr.md", out.Base, out.Head, out.Title)
	return out, nil
}

// OpenPullRequest publica a branch e abre o PR com o `gh`. Só deve ser chamado
// com confirmação humana ou no nível de autonomia autônomo (e aí em rascunho).
func (a *App) OpenPullRequest(ids []string, draft bool) (*PRProposal, error) {
	pr, err := a.PreparePullRequest(ids)
	if err != nil {
		return nil, err
	}
	if !a.forge.Available() {
		return pr, model.Invalid("GitHub CLI (gh) indisponível ou sem login: abra o PR à mão com o título e o corpo gerados")
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	if pr.Head == "" || pr.Head == pr.Base {
		return nil, model.Invalid("o PR precisa sair de uma branch de trabalho (atual: %q)", pr.Head)
	}
	if err := a.git.Push(conv.Git.Remote, pr.Head, true); err != nil {
		return nil, err
	}
	url, err := a.forge.CreatePR(pr.Title, pr.Body, pr.Base, pr.Head, draft)
	if err != nil {
		return nil, err
	}
	pr.Opened, pr.URL, pr.Command = true, url, ""
	a.emit(hub.Event{Type: hub.EventGit, Source: hub.SourceCLI, Message: "Pull request aberto: " + url})
	return pr, nil
}

// ---------------------------------------------------------------------------
// Validação (git lint e hooks)
// ---------------------------------------------------------------------------

// LintRequest escolhe o que validar.
type LintRequest struct {
	Range   string `json:"range,omitempty"`
	Branch  string `json:"branch,omitempty"`
	PRTitle string `json:"pr_title,omitempty"`
}

// GitLint valida commits, branch e título de PR contra a convenção. Sem
// intervalo, vale <principal>..HEAD; sem branch, a atual.
func (a *App) GitLint(req LintRequest) (*conventions.Result, error) {
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	p, _ := a.Plan()
	res := &conventions.Result{Issues: []conventions.Issue{}}
	if a.git.Available() {
		rng := req.Range
		if rng == "" && a.git.RevExists(conv.Git.MainBranch) {
			rng = conv.Git.MainBranch + "..HEAD"
		}
		if rng != "" {
			commits, err := a.git.Log(rng, 500)
			if err != nil {
				return nil, err
			}
			res = conventions.ValidateCommits(conv, commits, p)
		}
		if req.Branch == "" {
			req.Branch = a.git.CurrentBranch()
		}
	}
	if req.Branch != "" {
		res.Checked++
		res.Issues = append(res.Issues, conventions.ValidateBranch(conv, req.Branch)...)
	}
	if req.PRTitle != "" {
		res.Checked++
		res.Issues = append(res.Issues, conventions.ValidatePRTitle(conv, req.PRTitle, p)...)
	}
	return res, nil
}

// CheckCommitMessage valida o texto de uma mensagem de commit (hook
// commit-msg): linhas de comentário são ignoradas, como o Git faz.
func (a *App) CheckCommitMessage(message string) ([]conventions.Issue, error) {
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	p, _ := a.Plan()
	subject := ""
	for _, line := range strings.Split(message, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "# ------------------------ >8") {
			break
		}
		if t := strings.TrimSpace(line); t != "" {
			subject = t
			break
		}
	}
	return conventions.ValidateSubject(conv, subject, p), nil
}

// CheckPush valida o que um push envia (hook pre-push): o nome de cada branch
// e os commits novos. Cada linha é "<ref local> <sha local> <ref remota> <sha remoto>".
func (a *App) CheckPush(lines []string) ([]conventions.Issue, error) {
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	p, _ := a.Plan()
	issues := []conventions.Issue{}
	const zero = "0000000000000000000000000000000000000000"
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) != 4 || !strings.HasPrefix(f[2], "refs/heads/") || f[1] == zero {
			continue
		}
		branch := strings.TrimPrefix(f[2], "refs/heads/")
		issues = append(issues, conventions.ValidateBranch(conv, branch)...)
		rng := f[3] + ".." + f[1]
		if f[3] == zero {
			if !a.git.RevExists(conv.Git.Remote + "/" + conv.Git.MainBranch) {
				continue
			}
			rng = conv.Git.Remote + "/" + conv.Git.MainBranch + ".." + f[1]
		}
		if commits, err := a.git.Log(rng, 200); err == nil {
			issues = append(issues, conventions.ValidateCommits(conv, commits, p).Issues...)
		}
	}
	return issues, nil
}

// ---------------------------------------------------------------------------
// Hooks, template de PR, CI e changelog
// ---------------------------------------------------------------------------

// HooksResult descreve a instalação dos hooks.
type HooksResult struct {
	Dir       string   `json:"dir"`
	Installed []string `json:"installed"`
	Skipped   []string `json:"skipped,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

// InstallHooks instala os hooks commit-msg e pre-push, registra o merge driver
// dos arquivos derivados e marca-os no .gitattributes. Hooks de terceiros não
// são sobrescritos sem force. fallback é o executável usado se o
// archcode-studio não estiver no PATH.
func (a *App) InstallHooks(fallback string, force bool) (*HooksResult, error) {
	if !a.git.Available() {
		return nil, gitx.ErrUnavailable
	}
	dir, err := a.git.GitPath("hooks")
	if err != nil {
		return nil, err
	}
	res := &HooksResult{Dir: dir, Installed: []string{}}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	for _, name := range conventions.HookNames {
		path := filepath.Join(dir, name)
		if data, err := os.ReadFile(path); err == nil && !strings.Contains(string(data), conventions.HookMarker) && !force {
			res.Skipped = append(res.Skipped, name)
			res.Notes = append(res.Notes, fmt.Sprintf("%s já tem um hook %s de outra ferramenta; chame `archcode-studio hook %s` dentro dele ou use --force", dir, name, name))
			continue
		}
		if err := os.WriteFile(path, []byte(conventions.HookScript(name, fallback)), 0o755); err != nil {
			return nil, err
		}
		res.Installed = append(res.Installed, name)
	}
	driver := "archcode-studio merge-driver %O %A %B %P"
	_ = a.git.ConfigSet("merge.archcode.name", "ArchCode Studio: arquivos derivados do backlog")
	_ = a.git.ConfigSet("merge.archcode.driver", driver)
	attrs, _ := a.st.ReadFile(".gitattributes")
	content := replaceHashBlock(string(attrs), "archcode", conventions.GitAttributes)
	if content != string(attrs) {
		if err := a.st.WriteFile(".gitattributes", []byte(content)); err != nil {
			return nil, err
		}
		res.Notes = append(res.Notes, ".gitattributes atualizado: tasks.json, ai-prd.md e MEMORY.md são regenerados em vez de conflitar")
	}
	return res, nil
}

// UninstallHooks remove os hooks gerados pelo Studio.
func (a *App) UninstallHooks() ([]string, error) {
	if !a.git.Available() {
		return nil, gitx.ErrUnavailable
	}
	dir, err := a.git.GitPath("hooks")
	if err != nil {
		return nil, err
	}
	removed := []string{}
	for _, name := range conventions.HookNames {
		path := filepath.Join(dir, name)
		if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), conventions.HookMarker) {
			if os.Remove(path) == nil {
				removed = append(removed, name)
			}
		}
	}
	return removed, nil
}

// HooksInstalled informa quais hooks do Studio estão instalados.
func (a *App) HooksInstalled() []string {
	out := []string{}
	if !a.git.Available() {
		return out
	}
	dir, err := a.git.GitPath("hooks")
	if err != nil {
		return out
	}
	for _, name := range conventions.HookNames {
		if data, err := os.ReadFile(filepath.Join(dir, name)); err == nil && strings.Contains(string(data), conventions.HookMarker) {
			out = append(out, name)
		}
	}
	return out
}

// replaceHashBlock troca um bloco "# archcode:start … # archcode:end" em
// arquivos com comentário por "#" (o .gitattributes).
func replaceHashBlock(content, name, block string) string {
	start, end := "# "+name+":start", "# "+name+":end"
	wrapped := start + " (gerado pelo ArchCode Studio)\n" + strings.TrimSpace(block) + "\n" + end
	i := strings.Index(content, start)
	j := strings.Index(content, end)
	if i >= 0 && j > i {
		return content[:i] + wrapped + content[j+len(end):]
	}
	if strings.TrimSpace(content) == "" {
		return wrapped + "\n"
	}
	return strings.TrimRight(content, "\n") + "\n\n" + wrapped + "\n"
}

// ApplyConventions grava os arquivos que derivam das convenções: template de
// PR, a skill convencoes-git (se instalada) e, com ci, o workflow que roda o
// git lint nos PRs.
func (a *App) ApplyConventions(ci bool, source string) ([]string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	installed, _ := a.installedSkills()
	checks := []conventions.SkillCheck{}
	for _, s := range skills.ForTask(installed, nil, a.projectStacks()) {
		for _, c := range s.Checks {
			if c.Required {
				checks = append(checks, conventions.SkillCheck{Skill: s.Name, Check: c.Text})
			}
		}
	}
	written := []string{}
	if err := a.st.WriteFile(".github/pull_request_template.md", []byte(conventions.PRTemplate(conv, checks))); err != nil {
		return nil, err
	}
	written = append(written, ".github/pull_request_template.md")
	if ci {
		rel := ".github/workflows/archcode-conventions.yml"
		if err := a.st.WriteFile(rel, []byte(conventions.CIWorkflow())); err != nil {
			return nil, err
		}
		written = append(written, rel)
	}
	a.refreshGeneratedSkill(conv)
	a.emit(hub.Event{Type: hub.EventConventions, Source: source, Message: "Convenções aplicadas: " + strings.Join(written, ", ")})
	return written, nil
}

// Changelog gera (e, com write, grava) o CHANGELOG.md por sprint.
func (a *App) Changelog(write bool) (string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	conv, err := a.st.LoadConventions()
	if err != nil {
		return "", err
	}
	p, err := a.Plan()
	if err != nil {
		return "", err
	}
	if !a.git.Available() {
		return "", gitx.ErrUnavailable
	}
	content, err := a.changelogContent(conv, p)
	if err != nil {
		return "", err
	}
	if write {
		if err := a.st.WriteFile(store.FileChangelog, []byte(content)); err != nil {
			return "", err
		}
	}
	return content, nil
}

func (a *App) changelogContent(conv *model.Conventions, p *model.Plan) (string, error) {
	main := conv.Git.MainBranch
	if !a.git.RevExists(main) {
		main = "HEAD"
	}
	commits, err := a.git.Log(main, 2000)
	if err != nil {
		return "", err
	}
	old, _ := a.st.ReadFile(store.FileChangelog)
	return conventions.Changelog(string(old), conventions.ChangelogBlock(conv, commits, p.Sprints)), nil
}

// writeChangelogLocked atualiza o CHANGELOG.md (chamado com a.tx travado).
func (a *App) writeChangelogLocked(conv *model.Conventions, p *model.Plan) (string, error) {
	if !a.git.Available() {
		return "", gitx.ErrUnavailable
	}
	content, err := a.changelogContent(conv, p)
	if err != nil {
		return "", err
	}
	return store.FileChangelog, a.st.WriteFile(store.FileChangelog, []byte(content))
}

// ---------------------------------------------------------------------------
// Estado do Git e reconciliação do quadro (RF041)
// ---------------------------------------------------------------------------

// GitOverview é o painel de Git da interface.
type GitOverview struct {
	Available    bool                `json:"available"`
	Status       *gitx.Status        `json:"status,omitempty"`
	BranchIssues []conventions.Issue `json:"branch_issues,omitempty"`
	CurrentTask  *model.WorkItem     `json:"current_task,omitempty"`
	Person       string              `json:"person,omitempty"`
	Hooks        []string            `json:"hooks"`
	Forge        bool                `json:"forge"`
	Recent       []gitx.Commit       `json:"recent,omitempty"`
}

// GitOverview devolve o estado do repositório para a interface.
func (a *App) GitOverview() (*GitOverview, error) {
	out := &GitOverview{Available: a.git.Available(), Hooks: []string{}}
	if !out.Available {
		return out, nil
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	st, err := a.git.Status()
	if err != nil {
		return nil, err
	}
	out.Status = st
	out.Person = a.Person()
	out.BranchIssues = conventions.ValidateBranch(conv, st.Branch)
	if p, err := a.Plan(); err == nil {
		out.CurrentTask = a.currentTask(p)
	}
	out.Hooks = a.HooksInstalled()
	out.Recent, _ = a.git.Log("HEAD", 8)
	return out, nil
}

// ReconcileChange é uma mudança de status sugerida pelo histórico do Git.
type ReconcileChange struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// Reconcile move as tarefas conforme o Git: commit na branch principal que
// cita a tarefa → concluída; PR aberto da branch da tarefa (via gh) → em
// revisão; commit na branch atual que cita tarefa pendente → em andamento.
// Com apply, grava; sem, só descreve.
func (a *App) Reconcile(apply bool, source string) ([]ReconcileChange, error) {
	if !a.git.Available() {
		return nil, gitx.ErrUnavailable
	}
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
	changes := []ReconcileChange{}
	target := map[string]string{}
	propose := func(it *model.WorkItem, to, reason string) {
		if it == nil || it.Archived || !it.Actionable() || it.Status == to || target[it.ID] != "" {
			return
		}
		rank := map[string]int{model.StatusPending: 0, model.StatusBlocked: 0, model.StatusInProgress: 1, model.StatusReview: 2, model.StatusCompleted: 3}
		if rank[to] <= rank[it.Status] {
			return
		}
		target[it.ID] = to
		changes = append(changes, ReconcileChange{ID: it.ID, Title: it.Title, From: it.Status, To: to, Reason: reason})
	}
	main := conv.Git.MainBranch
	if a.git.RevExists(main) {
		if log, err := a.git.Log(main, 300); err == nil {
			for _, c := range log {
				subject, pr := conventions.StripPRSuffix(c.Subject)
				if c.Merge() || conventions.IsReservation(conv, subject) {
					continue
				}
				reason := fmt.Sprintf("commit %s na %s", shortHash(c.Hash), main)
				if pr > 0 {
					reason += fmt.Sprintf(" (PR #%d)", pr)
				}
				for _, id := range model.ItemRefs(subject) {
					propose(p.Item(id), model.StatusCompleted, reason)
				}
			}
		}
	}
	if a.forge.Available() {
		if prs, err := a.forge.OpenPRs(); err == nil {
			for _, pr := range prs {
				for i := range p.Items {
					if it := &p.Items[i]; it.Branch != "" && it.Branch == pr.Head {
						propose(it, model.StatusReview, fmt.Sprintf("PR #%d aberto", pr.Number))
					}
				}
			}
		}
	}
	if cur := a.git.CurrentBranch(); cur != "" && cur != main && a.git.RevExists(main) {
		if log, err := a.git.Log(main+".."+cur, 100); err == nil {
			for _, c := range log {
				if conventions.IsReservation(conv, c.Subject) {
					continue
				}
				for _, id := range model.ItemRefs(c.Subject) {
					propose(p.Item(id), model.StatusInProgress, fmt.Sprintf("commit %s na branch %s", shortHash(c.Hash), cur))
				}
			}
		}
	}
	if !apply || len(changes) == 0 {
		return changes, nil
	}
	changed := []*model.WorkItem{}
	for _, ch := range changes {
		it := p.Item(ch.ID)
		a.setStatus(it, ch.To)
		if ch.To == model.StatusCompleted {
			for i := range it.Acceptance {
				it.Acceptance[i].Done = true
			}
		}
		it.UpdatedAt = a.timestamp()
		changed = append(changed, it)
	}
	if err := a.commitPlan(p, changed, nil, source, fmt.Sprintf("Quadro reconciliado com o Git: %d tarefa(s)", len(changes))); err != nil {
		return nil, err
	}
	return changes, nil
}
