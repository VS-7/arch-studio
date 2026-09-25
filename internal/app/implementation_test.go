package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
)

// newModuleApp cria o projeto de exemplo com Git e relógio falsos.
func newModuleApp(t *testing.T) (*App, *store.Store, *gitx.Fake, *gitx.FakeForge) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := project.Init(st, project.Options{ProjectName: "Loja"}); err != nil {
		t.Fatal(err)
	}
	g := gitx.NewFake("Ana Souza", "ana@exemplo.com")
	forge := &gitx.FakeForge{}
	clock := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC) // segunda-feira
	a := New(st, nil, WithGit(g), WithForge(forge), WithClock(func() time.Time { return clock }))
	return a, st, g, forge
}

func TestModuloDeImplementacaoPontaAPonta(t *testing.T) {
	a, st, g, forge := newModuleApp(t)
	if _, err := a.InitConventions("", false, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallSkills([]string{"archcode-fluxo", "testes-e-aceite"}, false, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}

	// 1. Backlog gerado da arquitetura (fatiamento híbrido: fundação + fatias).
	diff, err := a.SyncBacklog(true, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	if diff.Counts[plan.ChangeAdded] == 0 {
		t.Fatalf("prévia sem itens: %+v", diff)
	}
	if p, _ := st.LoadPlan(); len(p.Items) != 0 {
		t.Fatal("dry run não pode gravar")
	}
	sync, err := a.SyncBacklog(false, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	p, _ := st.LoadPlan()
	if len(p.Items) != sync.Total || sync.Slicing != model.SlicingHybrid {
		t.Fatalf("backlog gravado (%d) diferente do diff (%+v)", len(p.Items), sync)
	}
	hasSlice, hasEpic := false, false
	for _, it := range p.Items {
		hasSlice = hasSlice || strings.HasPrefix(it.Source, plan.SourceSlice)
		hasEpic = hasEpic || it.Type == model.ItemEpic
	}
	if !hasSlice || !hasEpic {
		t.Fatal("backlog sem épicos ou fatias verticais")
	}
	board, _ := st.LoadTasks()
	if len(board.Tasks) == 0 {
		t.Fatal("tasks.json derivado não foi gerado")
	}
	// Sincronizar de novo não muda nada.
	if again, _ := a.SyncBacklog(true, hub.SourceUI); len(again.Changes) != 0 {
		t.Fatalf("segunda sincronização deveria ser vazia: %+v", again.Changes)
	}

	// 2. Sprint 01 planejada pela capacidade e iniciada.
	res, err := a.PlanSprint(0, "Base e autenticação", 0, nil, true, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	if res.Sprint.Number != 1 || res.Sprint.Start != "2026-09-28" || res.Sprint.End != "2026-10-09" || len(res.Proposal.Items) == 0 {
		t.Fatalf("sprint planejada: %+v %+v", res.Sprint, res.Proposal)
	}
	if _, err := a.StartSprint(1, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.CreateSprint(SprintInput{}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.StartSprint(2, hub.SourceUI); err == nil {
		t.Fatal("duas sprints ativas não pode")
	}

	// 3. Retomada aponta a próxima tarefa pronta.
	r, err := a.Resume("brief", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Next == nil || r.Sprint == nil || r.Sprint.Number != 1 {
		t.Fatalf("retomada sem próxima tarefa/sprint: %+v", r)
	}
	taskID := r.Next.ID

	// 4. Reserva: cria a branch da convenção, publica com commit de reserva.
	claim, err := a.ClaimTask(taskID, ClaimOptions{Agent: "claude-code", Switch: true, Push: true}, hub.SourceAI)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "sprint-01/" + taskID + "-"
	if !strings.HasPrefix(claim.Branch, wantPrefix) || !claim.Switched || !claim.Pushed || claim.ReservationCommit == "" {
		t.Fatalf("reserva: %+v", claim)
	}
	if g.CurrentBranch() != claim.Branch || len(g.Remote[claim.Branch]) == 0 {
		t.Fatal("branch não criada/publicada")
	}
	if claim.Task.Status != model.StatusInProgress || claim.Task.Assignee != "Ana Souza <ana@exemplo.com>" {
		t.Fatalf("tarefa não reservada: %+v", claim.Task)
	}
	if len(claim.Skills) == 0 {
		t.Fatal("a reserva deveria listar as skills da tarefa")
	}

	// Outra pessoa não consegue reservar a mesma tarefa: a branch está no remoto.
	bruno := gitx.NewFake("Bruno Lima", "bruno@exemplo.com")
	bruno.Remote = g.Remote
	b := New(st, nil, WithGit(bruno), WithForge(forge), WithClock(a.now))
	if _, err := b.ClaimTask(taskID, ClaimOptions{Switch: true}, hub.SourceUI); err == nil ||
		!strings.Contains(err.Error(), "Ana Souza") {
		t.Fatalf("reserva duplicada deveria falhar citando a dona: %v", err)
	}

	// 5. Checkpoint recusa segredo e grava o passo.
	secret := "senha: sup3rs3cr3t0!"
	if _, err := a.SaveCheckpoint(taskID, HandoffInput{Notes: &secret}, hub.SourceAI); err == nil {
		t.Fatal("checkpoint com segredo deveria ser recusado")
	}
	last, next := "estrutura criada", "escrever testes de aceite"
	files := []string{"internal/auth/jwt.go"}
	if _, err := a.SaveCheckpoint(taskID, HandoffInput{LastStep: &last, NextStep: &next, Files: &files}, hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	r, _ = a.Resume("brief", "")
	if len(r.MyWork) != 1 || r.MyWork[0].Handoff == nil || !strings.Contains(r.Hints[0], next) {
		t.Fatalf("retomada não trouxe o checkpoint: %+v", r.MyWork)
	}

	// 6. Conclusão recusada sem os checks obrigatórios; aceita com eles.
	done, err := a.CompleteTask(taskID, CompleteInput{}, hub.SourceAI)
	if err != nil {
		t.Fatal(err)
	}
	if done.Completed || len(done.Missing) == 0 || len(done.Required) == 0 {
		t.Fatalf("conclusão sem checks deveria ser recusada: %+v", done)
	}
	checks := []model.CheckResult{}
	for _, rc := range done.Required {
		checks = append(checks, model.CheckResult{Skill: rc.Skill, Check: rc.Check, Result: "ok", Evidence: "go test ./... ok"})
	}
	done, err = a.CompleteTask(taskID, CompleteInput{Checks: checks, Notes: "tudo verde"}, hub.SourceAI)
	if err != nil || !done.Completed || done.Status != model.StatusReview {
		t.Fatalf("conclusão: %v %+v", err, done)
	}

	// 7. Commit na convenção, com os arquivos preparados.
	g.Staged = []string{"internal/auth/jwt.go"}
	prop, err := a.ProposeCommit(CommitRequest{TaskID: taskID, Commit: true, CoAuthor: "Claude <noreply@anthropic.com>"})
	if err != nil || !prop.Committed || !strings.HasPrefix(prop.Subject, "Sprint 01 - ") || !strings.HasSuffix(prop.Subject, "["+taskID+"]") {
		t.Fatalf("commit: %v %+v", err, prop)
	}
	lint, err := a.GitLint(LintRequest{})
	if err != nil || !lint.OK() || lint.Checked < 2 {
		t.Fatalf("git lint deveria passar: %v %+v", err, lint)
	}
	if issues, _ := a.CheckCommitMessage("# comentário\nImplementa qualquer coisa\n"); len(issues) == 0 {
		t.Fatal("hook commit-msg deveria recusar mensagem fora da convenção")
	}

	// 8. PR com título, critérios e checks; aberto no servidor (gh falso).
	pr, err := a.OpenPullRequest(nil, true)
	if err != nil || !pr.Opened || !strings.HasPrefix(pr.Title, "Sprint 01 - ") || !strings.Contains(pr.Body, "Fecha: "+taskID) {
		t.Fatalf("PR: %v %+v", err, pr)
	}
	if len(forge.Created) != 1 || !forge.Created[0].Draft {
		t.Fatal("PR em rascunho não foi criado")
	}

	// 9. Sessão e memória.
	if _, err := a.LogSession(SessionInput{Summary: "Autenticação implementada", Decisions: []string{"RS256"}, Agent: "claude-code"}, hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Remember(NoteInput{Title: "Tokens assinados com RS256", Body: "Chaves ficam no cofre.", Type: "decisao", Components: []string{"Core API"}}, hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	if notes, _ := a.Recall("rs256", "", "", 5); len(notes) != 1 {
		t.Fatalf("recall: %+v", notes)
	}
	if idx, _ := st.ReadFile(store.FileMemoryIndex); !strings.Contains(string(idx), "Tokens assinados com RS256") {
		t.Fatal("MEMORY.md não indexou a memória")
	}

	// 10. Merge (simulado): o commit chega na main e a reconciliação conclui.
	g.AddCommit("main", prop.Subject+" (#1)")
	changes, err := a.Reconcile(true, hub.SourceCLI)
	if err != nil || len(changes) != 1 || changes[0].To != model.StatusCompleted {
		t.Fatalf("reconciliação: %v %+v", err, changes)
	}

	// 11. Encerramento: relatório, transferência e tag sugerida.
	closed, err := a.CloseSprint(1, -1, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	if closed.Stats.Completed != 1 || closed.CarriedTo != 2 || len(closed.Carried) == 0 || closed.SuggestedTag != "sprint-01" {
		t.Fatalf("encerramento: %+v", closed)
	}
	report, _ := st.ReadFile(closed.Report)
	if !strings.Contains(string(report), "Relatório da Sprint 01") || !strings.Contains(string(report), taskID) {
		t.Fatalf("relatório:\n%s", report)
	}
	changelog, _ := st.ReadFile(store.FileChangelog)
	if !strings.Contains(string(changelog), "## Sprint 01") {
		t.Fatalf("changelog:\n%s", changelog)
	}
	p, _ = st.LoadPlan()
	if p.Sprint(1).Status != model.SprintClosed || p.Item(closed.Carried[0]).Sprint != 2 {
		t.Fatal("sprint não encerrada ou itens não transferidos")
	}
}

func TestMigracaoDoTasksJSONPreservaStatus(t *testing.T) {
	a, st, _, _ := newModuleApp(t)
	// Projeto de versão anterior: só o AI-PRD gerado, com uma tarefa concluída.
	if _, err := a.GenerateAIPRD(prd.Options{IncludeTestScenarios: true}, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	board, _ := st.LoadTasks()
	first := board.Tasks[0].ID
	// Simula o legado apagando o backlog e marcando a tarefa no tasks.json.
	if err := os.RemoveAll(filepath.Join(st.Root(), store.DirPlan)); err != nil {
		t.Fatal(err)
	}
	board.Tasks[0].Status = model.StatusCompleted
	if err := st.SaveTasks(board); err != nil {
		t.Fatal(err)
	}
	// Leitura não grava; a primeira escrita migra.
	if p, _ := a.Plan(); len(p.Items) != len(board.Tasks) {
		t.Fatalf("leitura deveria ver as tarefas legadas: %d", len(p.Items))
	}
	if _, err := a.MarkTaskStatus(board.Tasks[1].ID, "in_progress", "começando", hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	p, _ := st.LoadPlan()
	if it := p.Item(first); it == nil || it.Status != model.StatusCompleted {
		t.Fatalf("status legado perdido na migração: %+v", it)
	}
	// Recompilar o AI-PRD mantém os ids e os status.
	if _, err := a.GenerateAIPRD(prd.Options{IncludeTestScenarios: true}, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	p, _ = st.LoadPlan()
	if p.Item(first).Status != model.StatusCompleted || p.Item(board.Tasks[1].ID).Status != model.StatusInProgress {
		t.Fatal("recompilar perdeu o progresso")
	}
}

func TestRenomearComponenteNaoPerdeATarefa(t *testing.T) {
	a, st, _, _ := newModuleApp(t)
	if _, err := a.SyncBacklog(false, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	p, _ := st.LoadPlan()
	var target *model.WorkItem
	for i := range p.Items {
		if strings.HasPrefix(p.Items[i].Source, plan.SourceComponent) {
			target = &p.Items[i]
			break
		}
	}
	if _, err := a.MarkTaskStatus(target.ID, "completed", "", hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	nodeID := strings.TrimPrefix(target.Source, plan.SourceComponent)
	name := "Nome Totalmente Novo"
	if _, err := a.UpdateNode(nodeID, NodeInput{Label: name}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GenerateAIPRD(prd.Options{IncludeTestScenarios: true}, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	p, _ = st.LoadPlan()
	it := p.Item(target.ID)
	if it == nil || it.Status != model.StatusCompleted || it.Component != name {
		t.Fatalf("tarefa perdeu id ou status ao renomear o componente: %+v", it)
	}
	md, _ := st.ReadFile(store.FileAIPRD)
	if !strings.Contains(string(md), target.ID) {
		t.Fatal("ai-prd.md deveria usar o id estável")
	}
}

func TestItensManuaisEOverrides(t *testing.T) {
	a, st, _, _ := newModuleApp(t)
	if _, err := a.SyncBacklog(false, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	title, typ := "Sessão expira antes do prazo", "bug"
	bug, err := a.CreateItem(ItemInput{Type: &typ, Title: &title}, hub.SourceUI)
	if err != nil || bug.ID != "BUG-001" || bug.Source != "" {
		t.Fatalf("bug: %v %+v", err, bug)
	}
	p, _ := st.LoadPlan()
	var gen *model.WorkItem
	for i := range p.Items {
		if strings.HasPrefix(p.Items[i].Source, plan.SourceComponent) {
			gen = &p.Items[i]
			break
		}
	}
	newTitle := "Título escolhido pelo time"
	if _, err := a.UpdateItem(gen.ID, ItemInput{Title: &newTitle}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.SyncBacklog(false, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	p, _ = st.LoadPlan()
	if p.Item(gen.ID).Title != newTitle || !p.Item(gen.ID).Overridden("title") {
		t.Fatal("sincronização desfez a edição humana")
	}
	// Mover para o topo do backlog.
	first := p.Items[0].ID
	moved, err := a.MoveItem(bug.ID, MoveInput{Before: first}, hub.SourceUI)
	if err != nil || moved.Rank >= p.Items[0].Rank {
		t.Fatalf("mover: %v %+v", err, moved)
	}
	// Item gerado é arquivado, não apagado; manual é apagado.
	if err := a.DeleteItem(gen.ID, hub.SourceUI); err == nil || !strings.Contains(err.Error(), "dependência") {
		// Pode não ter dependentes; nesse caso é arquivado.
		p, _ = st.LoadPlan()
		if it := p.Item(gen.ID); it != nil && !it.Archived && err == nil {
			t.Fatal("item gerado deveria ser arquivado")
		}
	}
	if err := a.DeleteItem(bug.ID, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if st.Exists(store.ItemPath(model.ItemBug, "BUG-001")) {
		t.Fatal("bug manual deveria ter sido apagado")
	}
}

func TestSkillsSyncEAgentSetup(t *testing.T) {
	a, st, _, _ := newModuleApp(t)
	if _, err := a.InitConventions("", false, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallSkills([]string{"segredos-e-config", "convencoes-git"}, false, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	// Um hook já existente do usuário é preservado.
	if err := st.WriteFile(".claude/settings.json", []byte(`{"permissions":{"allow":["Bash(ls)"]}}`)); err != nil {
		t.Fatal(err)
	}
	res, err := a.AgentSetup("claude", hub.SourceCLI)
	if err != nil {
		t.Fatal(err)
	}
	settings, _ := st.ReadFile(".claude/settings.json")
	for _, want := range []string{`"permissions"`, `"SessionStart"`, "resume --brief --hook", `"PreCompact"`, `"SessionEnd"`} {
		if !strings.Contains(string(settings), want) {
			t.Errorf("settings.json sem %s:\n%s", want, settings)
		}
	}
	if _, err := a.AgentSetup("claude", hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	again, _ := st.ReadFile(".claude/settings.json")
	if strings.Count(string(again), "resume --brief") != 1 {
		t.Fatal("rodar de novo duplicou o hook")
	}
	mcp, _ := st.ReadFile(".mcp.json")
	if !strings.Contains(string(mcp), `"type": "stdio"`) {
		t.Fatalf(".mcp.json: %s", mcp)
	}
	if !st.Exists(".claude/skills/segredos-e-config/SKILL.md") || len(res.Written) < 3 {
		t.Fatalf("skills não exportadas: %+v", res)
	}
	agents, _ := st.ReadFile("AGENTS.md")
	if !strings.Contains(string(agents), "resume_work") {
		t.Fatal("AGENTS.md sem o protocolo")
	}
	// Desinstalar e sincronizar remove só o que foi gerado.
	if err := st.WriteFile(".claude/skills/minha/SKILL.md", []byte("---\nname: minha\ndescription: x\n---\nminha")); err != nil {
		t.Fatal(err)
	}
	if err := a.RemoveSkill("segredos-e-config", hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	sync, err := a.SyncSkills([]string{TargetClaude}, hub.SourceCLI)
	if err != nil {
		t.Fatal(err)
	}
	if len(sync.Removed) != 1 || st.Exists(".claude/skills/segredos-e-config/SKILL.md") || !st.Exists(".claude/skills/minha/SKILL.md") {
		t.Fatalf("limpeza errada: %+v", sync)
	}
	// Mudar a convenção regenera a skill gerada.
	c, _ := a.Conventions()
	c.AI.Autonomy = model.AutonomySupervised
	if _, err := a.SaveConventions(c, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	s, _ := a.Skill("convencoes-git")
	if !strings.Contains(s.Body, "Supervisionado") {
		t.Fatal("convencoes-git não acompanhou a autonomia")
	}
}

func TestDoctorResolveConflitos(t *testing.T) {
	a, st, _, _ := newModuleApp(t)
	if _, err := a.SyncBacklog(false, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	p, _ := st.LoadPlan()
	it := p.Items[len(p.Items)-1]
	data, _ := st.ReadFile(it.File)
	broken := strings.Replace(string(data), "status: pending", "<<<<<<< HEAD\nstatus: in_progress\n=======\nstatus: completed\n>>>>>>> outra", 1)
	if err := st.WriteFile(it.File, []byte(broken)); err != nil {
		t.Fatal(err)
	}
	rep, err := a.Doctor("", false)
	if err != nil || len(rep.Conflicts) != 1 {
		t.Fatalf("doctor não achou o conflito: %v %+v", err, rep)
	}
	rep, err = a.Doctor("theirs", false)
	if err != nil || len(rep.Resolved) != 1 {
		t.Fatalf("doctor não resolveu: %v %+v", err, rep)
	}
	p, _ = st.LoadPlan()
	if p.Item(it.ID).Status != model.StatusCompleted {
		t.Fatal("lado theirs deveria vencer")
	}
}
