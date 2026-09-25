package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
)

func newModuleClient(t *testing.T) (*client.Client, *app.App, *gitx.Fake) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := project.Init(st, project.Options{ProjectName: "MCP Módulo"}); err != nil {
		t.Fatal(err)
	}
	g := gitx.NewFake("Ana Souza", "ana@exemplo.com")
	clock := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	a := app.New(st, nil, app.WithGit(g), app.WithForge(&gitx.FakeForge{}), app.WithClock(func() time.Time { return clock }))
	if _, err := a.InitConventions("", false, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	if _, err := a.InstallSkills([]string{"testes-e-aceite"}, false, hub.SourceCLI); err != nil {
		t.Fatal(err)
	}
	c, err := client.NewInProcessClient(New(Deps{App: a, Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "claude-code", Version: "1"}
	if _, err := c.Initialize(ctx, init); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, a, g
}

func text(res *mcp.CallToolResult) string { return res.Content[0].(mcp.TextContent).Text }

func decode(t *testing.T, res *mcp.CallToolResult, out any) {
	t.Helper()
	if err := json.Unmarshal([]byte(text(res)), out); err != nil {
		t.Fatalf("resposta não é JSON: %v\n%s", err, text(res))
	}
}

func TestFluxoDoAgenteNoModuloDeImplementacao(t *testing.T) {
	c, a, g := newModuleClient(t)

	var sync struct {
		Counts map[string]int `json:"counts"`
	}
	decode(t, call(t, c, "sync_backlog", map[string]any{}), &sync)
	if sync.Counts["added"] == 0 {
		t.Fatal("sync_backlog não criou itens")
	}

	// Planejar sem autonomia não aplica.
	var planned struct {
		Applied bool   `json:"applied"`
		Note    string `json:"note"`
	}
	decode(t, call(t, c, "plan_sprint", map[string]any{"goal": "Base", "apply": true}), &planned)
	if planned.Applied || !strings.Contains(planned.Note, "Assistido") {
		t.Fatalf("plan_sprint aplicou sem autonomia: %+v", planned)
	}

	var resume struct {
		Next *struct {
			ID string `json:"id"`
		} `json:"next"`
		Autonomy string `json:"autonomy"`
	}
	decode(t, call(t, c, "resume_work", map[string]any{}), &resume)
	if resume.Next == nil || resume.Autonomy != "assistido" {
		t.Fatalf("resume_work: %s", text(call(t, c, "resume_work", nil)))
	}
	id := resume.Next.ID

	// Reserva: o agente vem do clientInfo; o push é bloqueado pela autonomia.
	var claim map[string]any
	decode(t, call(t, c, "claim_task", map[string]any{"task_id": id, "push": true}), &claim)
	if claim["pushed"] == true || claim["publish_command"] == nil {
		t.Fatalf("push deveria ficar com um humano: %+v", claim)
	}
	it, _ := a.Item(id)
	if it.Agent != "claude-code" || it.Assignee == "" {
		t.Fatalf("agente/dono não registrados: %+v", it)
	}

	call(t, c, "save_checkpoint", map[string]any{"task_id": id, "last_step": "a", "next_step": "b", "files": []any{"x.go"}})

	// Conclusão sem checks: resposta estruturada com o que falta.
	var done struct {
		Completed bool     `json:"completed"`
		Missing   []string `json:"missing_checks"`
		Required  []struct {
			Skill, Check string
		} `json:"required_checks"`
	}
	decode(t, call(t, c, "complete_task", map[string]any{"task_id": id}), &done)
	if done.Completed || len(done.Missing) == 0 {
		t.Fatalf("complete_task sem checks deveria recusar: %+v", done)
	}
	checks := []any{}
	for _, r := range done.Required {
		checks = append(checks, map[string]any{"skill": r.Skill, "check": r.Check, "result": "ok", "evidence": "go test ./... ok"})
	}
	decode(t, call(t, c, "complete_task", map[string]any{"task_id": id, "checks": checks, "notes": "ok"}), &done)
	if !done.Completed {
		t.Fatalf("complete_task com checks: %+v", done)
	}

	// Commit: no nível assistido o agente só recebe a mensagem.
	g.Staged = []string{"x.go"}
	var prop struct {
		Proposal struct {
			Subject   string `json:"subject"`
			Committed bool   `json:"committed"`
		} `json:"proposal"`
		Note string `json:"note"`
	}
	decode(t, call(t, c, "propose_commit", map[string]any{"task_id": id, "commit": true}), &prop)
	if prop.Proposal.Committed || !strings.HasPrefix(prop.Proposal.Subject, "Sprint") && !strings.HasPrefix(prop.Proposal.Subject, "Chore") {
		t.Fatalf("propose_commit: %+v", prop)
	}

	call(t, c, "log_session", map[string]any{"summary": "Fundação pronta", "decisions": []any{"usar pgx"}})
	call(t, c, "remember", map[string]any{"title": "Usar pgx", "body": "Driver pgx, não database/sql.", "type": "decisao"})
	if !strings.Contains(text(call(t, c, "recall", map[string]any{"query": "pgx"})), "Usar pgx") {
		t.Fatal("recall não achou a memória")
	}
	skill := text(call(t, c, "get_skill", map[string]any{"name": "testes-e-aceite"}))
	if !strings.Contains(skill, `check=`) {
		t.Fatalf("get_skill sem checks:\n%s", skill)
	}
	var created map[string]any
	decode(t, call(t, c, "create_backlog_item", map[string]any{"type": "bug", "title": "Token não expira", "acceptance": []any{"expira em 15 min"}}), &created)
	if created["id"] != "BUG-001" {
		t.Fatalf("create_backlog_item: %+v", created)
	}
	pr := text(call(t, c, "prepare_pull_request", map[string]any{"task_ids": []any{id}}))
	if !strings.Contains(pr, "Fecha: "+id) {
		t.Fatalf("prepare_pull_request:\n%s", pr)
	}
}
