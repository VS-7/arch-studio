package conventions

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Validação (hooks e `archcode-studio git lint`) — RF040
// ---------------------------------------------------------------------------

// Níveis de problema.
const (
	LevelError   = "error"
	LevelWarning = "warning"
)

// Issue é um problema encontrado na validação.
type Issue struct {
	Level   string `json:"level"`
	Subject string `json:"subject"` // o que foi validado (assunto, branch, título)
	Ref     string `json:"ref,omitempty"`
	Message string `json:"message"`
}

// Result agrupa os problemas de uma validação.
type Result struct {
	Checked int     `json:"checked"`
	Issues  []Issue `json:"issues"`
}

// Errors conta os problemas de nível erro.
func (r *Result) Errors() int {
	n := 0
	for _, i := range r.Issues {
		if i.Level == LevelError {
			n++
		}
	}
	return n
}

// OK informa se não há erros.
func (r *Result) OK() bool { return r.Errors() == 0 }

// Parsed é o que foi extraído de uma mensagem válida.
type Parsed struct {
	Sprint  int
	ID      string
	Summary string
	Kind    string
	Type    string
}

// exemptSubject informa se o assunto é gerado pelo Git e não segue a
// convenção (merge, revert, fixup).
func exemptSubject(subject string) bool {
	for _, p := range []string{"Merge ", "Revert \"", "fixup! ", "squash! ", "amend! "} {
		if strings.HasPrefix(subject, p) {
			return true
		}
	}
	return false
}

// ParseSubject aplica os padrões de commit (de sprint e fora de sprint).
func ParseSubject(c *model.Conventions, subject string) (*Parsed, bool) {
	for _, pattern := range []string{c.Git.Commit, c.Git.CommitOffSprint} {
		if pattern == "" {
			continue
		}
		if m := Match(pattern, subject, c); m != nil {
			p := &Parsed{ID: m["id"], Summary: m["summary"], Kind: m["kind"], Type: m["type"]}
			if s := m["sprint"]; s != "" {
				p.Sprint, _ = strconv.Atoi(s)
			}
			if p.ID == "" {
				if refs := model.ItemRefs(subject); len(refs) > 0 {
					p.ID = refs[0]
				}
			}
			return p, true
		}
	}
	return nil, false
}

// CommitExample devolve um exemplo de assunto válido, para mensagens de erro.
func CommitExample(c *model.Conventions) string {
	item := &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Implementar emissão de JWT"}
	return CommitSubject(c, CommitInput{Item: item, Sprint: 1})
}

// ValidateSubject valida o assunto de um commit. plan (opcional) confere se o
// item citado existe e se a sprint confere.
func ValidateSubject(c *model.Conventions, subject string, plan *model.Plan) []Issue {
	subject = strings.TrimSpace(subject)
	issues := []Issue{}
	add := func(level, msg string) {
		issues = append(issues, Issue{Level: level, Subject: subject, Message: msg})
	}
	if subject == "" {
		add(LevelError, "mensagem de commit vazia")
		return issues
	}
	if exemptSubject(subject) {
		return issues
	}
	// O " (#12)" do squash merge do GitHub não conta.
	core, _ := StripPRSuffix(subject)
	parsed, ok := ParseSubject(c, core)
	if !ok {
		add(LevelError, fmt.Sprintf("assunto fora da convenção. Exemplo: %q", CommitExample(c)))
		return issues
	}
	if max := c.Git.MaxSubject; max > 0 && utf8.RuneCountInString(core) > max {
		add(LevelError, fmt.Sprintf("assunto com %d caracteres (máximo %d)", utf8.RuneCountInString(core), max))
	}
	if c.Git.RequireID && parsed.ID == "" {
		add(LevelError, "assunto sem o id do item entre colchetes, ex.: [TASK-API-01]")
	}
	if len(c.Git.Verbs) > 0 && parsed.Summary != "" {
		verb, _, _ := strings.Cut(parsed.Summary, " ")
		if !isAllowedVerb(verb, c) {
			add(LevelWarning, fmt.Sprintf("o resumo começa com %q, que não está na lista de verbos (%s…)", verb,
				strings.Join(c.Git.Verbs[:min(4, len(c.Git.Verbs))], ", ")))
		}
	}
	if n, ok := SprintRef(parsed.ID); ok {
		// Commit de planejamento da sprint ([SPRINT-01]).
		if plan != nil && len(plan.Sprints) > 0 && plan.Sprint(n) == nil {
			add(LevelError, fmt.Sprintf("a %s não existe no backlog", model.SprintName(n)))
		}
		return issues
	}
	if parsed.ID == PlanRef {
		return issues
	}
	if plan != nil && parsed.ID != "" && (len(plan.Items) > 0) {
		it := plan.Item(parsed.ID)
		switch {
		case it == nil:
			add(LevelError, fmt.Sprintf("o item %s não existe no backlog (.arch/plan/)", parsed.ID))
		case parsed.Sprint > 0 && it.Sprint > 0 && it.Sprint != parsed.Sprint:
			add(LevelWarning, fmt.Sprintf("%s está na %s, mas o commit diz %s", it.ID,
				model.SprintName(it.Sprint), model.SprintName(parsed.Sprint)))
		}
	}
	return issues
}

// PlanRef é o id reservado dos commits de manutenção do backlog sem sprint
// (ex.: "Chore - Atualiza o backlog [ARCH-PLAN]").
const PlanRef = "ARCH-PLAN"

var reSprintRef = regexp.MustCompile(`^SPRINT-(\d+)$`)

// SprintRef reconhece o id reservado de uma sprint ("SPRINT-01"), usado nos
// commits de planejamento.
func SprintRef(id string) (int, bool) {
	m := reSprintRef.FindStringSubmatch(id)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil && n > 0
}

// IsReservation informa se o assunto é o commit vazio de reserva de uma
// tarefa ("Sprint 01 - Reserva TASK-API-01 [TASK-API-01]"), que não é entrega.
func IsReservation(c *model.Conventions, subject string) bool {
	core, _ := StripPRSuffix(subject)
	p, ok := ParseSubject(c, core)
	if !ok || p.ID == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(p.Summary), "Reserva "+p.ID)
}

// SprintRefID devolve o id reservado da sprint ("SPRINT-01").
func SprintRefID(n int) string { return "SPRINT-" + model.SprintLabel(n) }

// ValidateCommits valida uma lista de commits (merges são ignorados).
func ValidateCommits(c *model.Conventions, commits []gitx.Commit, plan *model.Plan) *Result {
	res := &Result{Issues: []Issue{}}
	for _, cm := range commits {
		if cm.Merge() {
			continue
		}
		res.Checked++
		for _, is := range ValidateSubject(c, cm.Subject, plan) {
			is.Ref = shortHash(cm.Hash)
			res.Issues = append(res.Issues, is)
		}
	}
	return res
}

// ValidateBranch valida o nome de uma branch de trabalho.
func ValidateBranch(c *model.Conventions, branch string) []Issue {
	branch = strings.TrimSpace(branch)
	if branch == "" || branch == "HEAD" || branch == c.Git.MainBranch {
		return nil
	}
	for _, ex := range c.Git.BranchExempt {
		if gitx.MatchGlob(ex, branch) {
			return nil
		}
	}
	for _, pattern := range []string{c.Git.Branch, c.Git.BranchOffSprint} {
		if pattern != "" && matchBranch(pattern, branch, c) {
			return nil
		}
	}
	example := BranchName(c, &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emitir JWT"}, 1)
	return []Issue{{Level: LevelError, Subject: branch,
		Message: fmt.Sprintf("branch fora da convenção. Exemplo: %q", example)}}
}

// matchBranch aceita {kind} em qualquer caixa (hotfix/…, Hotfix/…).
func matchBranch(pattern, branch string, c *model.Conventions) bool {
	return Match(pattern, branch, c) != nil
}

// ValidatePRTitle valida o título de um pull request.
func ValidatePRTitle(c *model.Conventions, title string, plan *model.Plan) []Issue {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	title, _ = StripPRSuffix(title)
	for _, pattern := range []string{c.PullRequest.Title, strings.Replace(c.Git.CommitOffSprint, "{summary}", "{title}", 1)} {
		if m := Match(pattern, title, c); m != nil {
			if plan != nil && m["id"] != "" && len(plan.Items) > 0 && plan.Item(m["id"]) == nil {
				return []Issue{{Level: LevelError, Subject: title,
					Message: fmt.Sprintf("o item %s não existe no backlog", m["id"])}}
			}
			return nil
		}
	}
	example := PRTitle(c, PRInput{Items: []*model.WorkItem{{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emissão de JWT", Sprint: 1}}})
	return []Issue{{Level: LevelError, Subject: title,
		Message: fmt.Sprintf("título de PR fora da convenção. Exemplo: %q", example)}}
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}
