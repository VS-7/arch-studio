package memory

import (
	"strings"
	"testing"
	"time"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/skills"
)

func samplePlan() *model.Plan {
	return &model.Plan{
		Sprints: []model.Sprint{{Number: 1, Name: "Sprint 01", Goal: "Autenticação", Status: model.SprintActive,
			Start: "2026-09-21", End: "2026-10-02"}},
		Items: []model.WorkItem{
			{ID: "TASK-DB-01", Type: model.ItemTask, Title: "Implementar banco", Status: model.StatusCompleted, Sprint: 1, Rank: "1", Tier: "data"},
			{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emitir JWT", Status: model.StatusInProgress, Sprint: 1, Rank: "2",
				Assignee: "Ana <ana@x.dev>", Branch: "sprint-01/TASK-API-01-emitir-jwt", Tier: "backend", Component: "Core API",
				ClaimedAt: "2026-09-24T10:00:00Z", Dependencies: []string{"TASK-DB-01"},
				Handoff: &model.Handoff{LastStep: "emissão pronta", NextStep: "validar refresh token", FailingTests: []string{"TestRefresh"}}},
			{ID: "TASK-WEB-01", Type: model.ItemTask, Title: "Tela de login", Status: model.StatusPending, Sprint: 1, Rank: "3", Tier: "frontend",
				Dependencies: []string{"TASK-DB-01"}},
			{ID: "TASK-OLD-01", Type: model.ItemTask, Title: "Relatório", Status: model.StatusInProgress, Rank: "4",
				Assignee: "Bruno <bruno@x.dev>", ClaimedAt: "2026-09-10T10:00:00Z"},
		},
	}
}

func TestBuildResume(t *testing.T) {
	now := time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC)
	sk := []skills.Skill{
		{Name: "testes-e-aceite", Trigger: "ao_iniciar_tarefa", Checks: []skills.Check{{ID: "a", Required: true}}},
		{Name: "estilo-go", Trigger: "sempre", AppliesTo: &skills.AppliesTo{Stacks: []string{"go"}}},
		{Name: "so-frontend", Trigger: "sempre", AppliesTo: &skills.AppliesTo{Tiers: []string{"frontend"}}},
		{Name: "desligada", Trigger: "sempre", Disabled: true},
	}
	r := Build(Input{
		Now: now, Project: "Loja", Person: "Ana <ana@x.dev>", Plan: samplePlan(),
		Conventions: model.DefaultConventions(""), Skills: sk, Stacks: []string{"go"},
		Sessions: []model.Session{
			{ID: "s3", Author: "Bruno <bruno@x.dev>", Summary: "Relatório pela metade"},
			{ID: "s2", Author: "Ana <ana@x.dev>", Summary: "Emissão de JWT\nmais detalhes", NextSteps: []string{"refresh"}},
		},
		Notes: []model.Note{
			{Slug: "pgx", Title: "Usar pgx", Type: "decisao", Components: []string{"Core API"}, Body: "Nada de database/sql."},
			{Slug: "outra", Title: "Glossário", Type: "glossario", Body: "SKU"},
		},
		Git: NewGitState(&gitx.Status{Branch: "main", Upstream: "origin/main", Behind: 2,
			Changed: []gitx.FileChange{{Path: "a.go"}}}, &gitx.Commit{Hash: "abcdef1234", Subject: "Commit"}),
		MainCommits: []gitx.Commit{{Hash: "9999999aaa", Parents: 1, Subject: "Sprint 01 - Implementa tela [TASK-WEB-01] (#3)"}},
	})

	if r.Sprint == nil || r.Sprint.Stats.Total != 3 || r.Sprint.Stats.Completed != 1 || r.Sprint.DaysLeft != 6 {
		t.Fatalf("sprint: %+v", r.Sprint)
	}
	if len(r.MyWork) != 1 || r.MyWork[0].ID != "TASK-API-01" || r.MyWork[0].Handoff == nil {
		t.Fatalf("meu trabalho: %+v", r.MyWork)
	}
	if r.Next == nil || r.Next.ID != "TASK-WEB-01" || r.Next.SuggestedBranch != "sprint-01/TASK-WEB-01-tela-de-login" {
		t.Fatalf("próxima: %+v", r.Next)
	}
	if r.LastSession == nil || r.LastSession.ID != "s2" || r.LastSession.Summary != "Emissão de JWT" {
		t.Fatalf("última sessão: %+v", r.LastSession)
	}
	if len(r.TeamSessions) != 1 || r.TeamSessions[0].Author != "Bruno" {
		t.Fatalf("sessões do time: %+v", r.TeamSessions)
	}
	if len(r.StaleClaims) != 1 || r.StaleClaims[0].ID != "TASK-OLD-01" {
		t.Fatalf("reservas paradas: %+v", r.StaleClaims)
	}
	names := []string{}
	for _, s := range r.Skills {
		names = append(names, s.Name)
	}
	if strings.Join(names, ",") != "testes-e-aceite,estilo-go" {
		t.Fatalf("skills da tarefa backend: %v", names)
	}
	if len(r.Memories) == 0 || r.Memories[0].Slug != "pgx" {
		t.Fatalf("memória do componente deveria vir primeiro: %+v", r.Memories)
	}
	joined := strings.Join(r.Divergences, "\n")
	for _, want := range []string{"branch sprint-01/TASK-API-01-emitir-jwt, mas você está em main", "2 commit(s) atrás", "9999999 na branch principal cita TASK-WEB-01"} {
		if !strings.Contains(joined, want) {
			t.Errorf("divergências sem %q:\n%s", want, joined)
		}
	}
	if len(r.Hints) == 0 || !strings.Contains(r.Hints[0], "Continue TASK-API-01: validar refresh token") {
		t.Fatalf("dicas: %v", r.Hints)
	}

	text := Text(r)
	for _, want := range []string{"# Onde parou — Loja", "Sprint 01 — Autenticação · 1/3 concluídas", "**TASK-API-01** Emitir JWT (em andamento",
		"Próximo passo: validar refresh token", "## Atenção", "`testes-e-aceite` (ao iniciar a tarefa, 1 check(s) obrigatório(s))"} {
		if !strings.Contains(text, want) {
			t.Errorf("texto sem %q:\n%s", want, text)
		}
	}
	// O resumo breve precisa caber no contexto de início de sessão.
	if len(text) > 8000 {
		t.Fatalf("texto breve grande demais: %d bytes", len(text))
	}
}

func TestEmptyBacklogHint(t *testing.T) {
	r := Build(Input{Now: time.Now(), Project: "X", Person: "Ana"})
	if len(r.Hints) != 1 || !strings.Contains(r.Hints[0], "backlog está vazio") {
		t.Fatalf("dica: %v", r.Hints)
	}
}
