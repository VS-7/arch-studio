package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Módulo de Implementação: rotas da interface
// ---------------------------------------------------------------------------

func (s *Server) planRoutes() {
	m := s.mux

	// Backlog
	m.HandleFunc("GET /api/plan", s.getPlan)
	m.HandleFunc("POST /api/plan/sync", s.syncPlan)
	m.HandleFunc("GET /api/plan/items", s.listItems)
	m.HandleFunc("POST /api/plan/items", s.createItem)
	m.HandleFunc("GET /api/plan/items/{id}", s.getItem)
	m.HandleFunc("PATCH /api/plan/items/{id}", s.updateItem)
	m.HandleFunc("DELETE /api/plan/items/{id}", s.deleteItem)
	m.HandleFunc("POST /api/plan/items/{id}/move", s.moveItem)
	m.HandleFunc("POST /api/plan/items/{id}/claim", s.claimItem)
	m.HandleFunc("POST /api/plan/items/{id}/release", s.releaseItem)
	m.HandleFunc("POST /api/plan/items/{id}/checkpoint", s.checkpointItem)
	m.HandleFunc("POST /api/plan/items/{id}/complete", s.completeItem)
	m.HandleFunc("GET /api/plan/items/{id}/prompt", s.itemPrompt)
	m.HandleFunc("GET /api/plan/items/{id}/checks", s.itemChecks)

	// Sprints
	m.HandleFunc("POST /api/sprints", s.createSprint)
	m.HandleFunc("POST /api/sprints/plan", s.planSprint)
	m.HandleFunc("GET /api/sprints/{n}", s.getSprint)
	m.HandleFunc("PATCH /api/sprints/{n}", s.updateSprint)
	m.HandleFunc("POST /api/sprints/{n}/start", s.startSprint)
	m.HandleFunc("POST /api/sprints/{n}/close", s.closeSprint)

	// Memória
	m.HandleFunc("GET /api/resume", s.resume)
	m.HandleFunc("GET /api/sessions", s.listSessions)
	m.HandleFunc("POST /api/sessions", s.logSession)
	m.HandleFunc("GET /api/memory", s.listMemory)
	m.HandleFunc("POST /api/memory", s.remember)
	m.HandleFunc("DELETE /api/memory/{slug}", s.forget)

	// Convenções e Git
	m.HandleFunc("GET /api/conventions", s.getConventions)
	m.HandleFunc("PUT /api/conventions", s.putConventions)
	m.HandleFunc("POST /api/conventions/init", s.initConventions)
	m.HandleFunc("POST /api/conventions/preview", s.previewConventions)
	m.HandleFunc("POST /api/conventions/apply", s.applyConventions)
	m.HandleFunc("GET /api/git", s.gitOverview)
	m.HandleFunc("POST /api/git/lint", s.gitLint)
	m.HandleFunc("POST /api/git/hooks", s.installHooks)
	m.HandleFunc("DELETE /api/git/hooks", s.uninstallHooks)
	m.HandleFunc("POST /api/git/commit", s.gitCommit)
	m.HandleFunc("POST /api/git/pr", s.gitPR)
	m.HandleFunc("POST /api/git/reconcile", s.gitReconcile)
	m.HandleFunc("POST /api/git/changelog", s.gitChangelog)

	// Skills e agentes
	m.HandleFunc("GET /api/skills", s.listSkills)
	m.HandleFunc("POST /api/skills", s.createSkill)
	m.HandleFunc("POST /api/skills/install", s.installSkills)
	m.HandleFunc("POST /api/skills/sync", s.syncSkills)
	m.HandleFunc("GET /api/skills/{name}", s.getSkill)
	m.HandleFunc("PUT /api/skills/{name}", s.putSkill)
	m.HandleFunc("PATCH /api/skills/{name}", s.patchSkill)
	m.HandleFunc("DELETE /api/skills/{name}", s.deleteSkill)
	m.HandleFunc("POST /api/agents/setup", s.agentSetup)
	m.HandleFunc("POST /api/doctor", s.doctor)
}

// optionalBody decodifica o corpo quando houver (POST sem corpo é válido).
func optionalBody[T any](r *http.Request, dst *T) error {
	if r.ContentLength == 0 {
		return nil
	}
	return decode(r, dst)
}

// ---------------------------------------------------------------------------
// Backlog
// ---------------------------------------------------------------------------

func (s *Server) getPlan(w http.ResponseWriter, r *http.Request) {
	p, err := s.app.Plan()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) syncPlan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DryRun bool `json:"dry_run"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.SyncBacklog(body.DryRun, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listItems(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := app.BacklogQuery{Status: q.Get("status"), Type: q.Get("type"), Assignee: q.Get("assignee"),
		OnlyReady: q.Get("ready") == "1", Archived: q.Get("archived") == "1"}
	if v := q.Get("sprint"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			fail(w, http.StatusBadRequest, model.Invalid("sprint inválida: %q", v))
			return
		}
		query.Sprint = &n
	}
	if query.Assignee == "me" {
		query.Assignee = s.app.Person()
	}
	items, total, err := s.app.Backlog(query)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (s *Server) createItem(w http.ResponseWriter, r *http.Request) {
	var in app.ItemInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	it, err := s.app.CreateItem(in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, it)
}

func (s *Server) getItem(w http.ResponseWriter, r *http.Request) {
	it, err := s.app.Item(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) updateItem(w http.ResponseWriter, r *http.Request) {
	var in app.ItemInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	it, err := s.app.UpdateItem(r.PathValue("id"), in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) deleteItem(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteItem(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) moveItem(w http.ResponseWriter, r *http.Request) {
	var in app.MoveInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	it, err := s.app.MoveItem(r.PathValue("id"), in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) claimItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Switch bool `json:"switch"`
		Push   bool `json:"push"`
		Force  bool `json:"force"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.ClaimTask(r.PathValue("id"), app.ClaimOptions{Switch: body.Switch, Push: body.Push, Force: body.Force}, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) releaseItem(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
		Force  bool   `json:"force"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	it, err := s.app.ReleaseTask(r.PathValue("id"), body.Reason, body.Force, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) checkpointItem(w http.ResponseWriter, r *http.Request) {
	var in app.HandoffInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	it, err := s.app.SaveCheckpoint(r.PathValue("id"), in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (s *Server) completeItem(w http.ResponseWriter, r *http.Request) {
	var in app.CompleteInput
	if err := optionalBody(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.CompleteTask(r.PathValue("id"), in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) itemChecks(w http.ResponseWriter, r *http.Request) {
	list, err := s.app.RequiredChecksFor(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) itemPrompt(w http.ResponseWriter, r *http.Request) {
	text, err := s.app.AgentPrompt(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"prompt": text})
}

// ---------------------------------------------------------------------------
// Sprints
// ---------------------------------------------------------------------------

func sprintNumber(r *http.Request) (int, error) {
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 {
		return 0, model.Invalid("número de sprint inválido: %q", r.PathValue("n"))
	}
	return n, nil
}

func (s *Server) createSprint(w http.ResponseWriter, r *http.Request) {
	var in app.SprintInput
	if err := optionalBody(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sp, err := s.app.CreateSprint(in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, sp)
}

func (s *Server) planSprint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Number   int      `json:"number"`
		Goal     string   `json:"goal"`
		Capacity float64  `json:"capacity_h"`
		IDs      []string `json:"ids"`
		Apply    bool     `json:"apply"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.PlanSprint(body.Number, body.Goal, body.Capacity, body.IDs, body.Apply, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) getSprint(w http.ResponseWriter, r *http.Request) {
	n, err := sprintNumber(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	st, err := s.app.Sprint(n)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Server) updateSprint(w http.ResponseWriter, r *http.Request) {
	n, err := sprintNumber(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	var in app.SprintInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sp, err := s.app.UpdateSprint(n, in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, sp)
}

func (s *Server) startSprint(w http.ResponseWriter, r *http.Request) {
	n, err := sprintNumber(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sp, err := s.app.StartSprint(n, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, sp)
}

func (s *Server) closeSprint(w http.ResponseWriter, r *http.Request) {
	n, err := sprintNumber(r)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	var body struct {
		CarryTo int `json:"carry_to"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.CloseSprint(n, body.CarryTo, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// Memória
// ---------------------------------------------------------------------------

func (s *Server) resume(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.Resume(r.URL.Query().Get("detail"), "")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 30
	}
	list, err := s.app.Sessions(limit)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) logSession(w http.ResponseWriter, r *http.Request) {
	var in app.SessionInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sess, err := s.app.LogSession(in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) listMemory(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	list, err := s.app.Recall(q.Get("q"), q.Get("type"), q.Get("component"), 0)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) remember(w http.ResponseWriter, r *http.Request) {
	var in app.NoteInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	n, err := s.app.Remember(in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func (s *Server) forget(w http.ResponseWriter, r *http.Request) {
	if err := s.app.ForgetNote(r.PathValue("slug"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ---------------------------------------------------------------------------
// Convenções e Git
// ---------------------------------------------------------------------------

func (s *Server) getConventions(w http.ResponseWriter, r *http.Request) {
	c, err := s.app.Conventions()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"conventions": c, "exists": s.app.HasConventions(),
		"preview": s.app.PreviewConventions(c)})
}

func (s *Server) putConventions(w http.ResponseWriter, r *http.Request) {
	var c model.Conventions
	if err := decode(r, &c); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.app.SaveConventions(&c, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) initConventions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Preset string `json:"preset"`
		Force  bool   `json:"force"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	c, err := s.app.InitConventions(body.Preset, body.Force, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) previewConventions(w http.ResponseWriter, r *http.Request) {
	var c model.Conventions
	if err := decode(r, &c); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, s.app.PreviewConventions(&c))
}

func (s *Server) applyConventions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CI bool `json:"ci"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	files, err := s.app.ApplyConventions(body.CI, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"written": files})
}

func (s *Server) gitOverview(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.GitOverview()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) gitLint(w http.ResponseWriter, r *http.Request) {
	var req app.LintRequest
	if err := optionalBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.GitLint(req)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"checked": res.Checked, "issues": res.Issues, "errors": res.Errors(), "ok": res.OK()})
}

func (s *Server) installHooks(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Force bool `json:"force"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.InstallHooks(s.opts.MCPCommand, body.Force)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) uninstallHooks(w http.ResponseWriter, r *http.Request) {
	removed, err := s.app.UninstallHooks()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"removed": removed})
}

func (s *Server) gitCommit(w http.ResponseWriter, r *http.Request) {
	var req app.CommitRequest
	if err := optionalBody(r, &req); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.ProposeCommit(req)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) gitPR(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs   []string `json:"ids"`
		Open  bool     `json:"open"`
		Draft bool     `json:"draft"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	var (
		res *app.PRProposal
		err error
	)
	if body.Open {
		res, err = s.app.OpenPullRequest(body.IDs, body.Draft)
	} else {
		res, err = s.app.PreparePullRequest(body.IDs)
	}
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) gitReconcile(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Apply bool `json:"apply"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.Reconcile(body.Apply, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"changes": res, "applied": body.Apply})
}

func (s *Server) gitChangelog(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Write bool `json:"write"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	content, err := s.app.Changelog(body.Write)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": content, "file": store.FileChangelog, "written": body.Write})
}

// ---------------------------------------------------------------------------
// Skills e agentes
// ---------------------------------------------------------------------------

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.Skills()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) getSkill(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	sk, err := s.app.Skill(name)
	if err != nil {
		fail(w, http.StatusNotFound, err)
		return
	}
	content, _ := s.app.ReadProjectFile(store.SkillPath(name))
	writeJSON(w, http.StatusOK, map[string]any{"skill": sk, "content": content})
}

func (s *Server) createSkill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Category    string `json:"category"`
		Trigger     string `json:"trigger"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sk, err := s.app.CreateSkill(strings.TrimSpace(body.Name), body.Description, body.Category, body.Trigger, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, sk)
}

func (s *Server) installSkills(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Names  []string `json:"names"`
		Update bool     `json:"update"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	done, err := s.app.InstallSkills(body.Names, body.Update, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"installed": done})
}

func (s *Server) putSkill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sk, err := s.app.SaveSkill(r.PathValue("name"), body.Content, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, sk)
}

func (s *Server) patchSkill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sk, err := s.app.SetSkillEnabled(r.PathValue("name"), body.Enabled, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, sk)
}

func (s *Server) deleteSkill(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RemoveSkill(r.PathValue("name"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) syncSkills(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Targets []string `json:"targets"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.SyncSkills(body.Targets, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) agentSetup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Agent string `json:"agent"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.AgentSetup(body.Agent, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) doctor(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prefer string `json:"prefer"`
		Fix    bool   `json:"fix"`
	}
	if err := optionalBody(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.app.Doctor(body.Prefer, body.Fix)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}
