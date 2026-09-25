package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestWorkItemRoundTrip(t *testing.T) {
	in := &WorkItem{
		ID: "TASK-API-01", Type: ItemTask, Title: "Emitir JWT no Auth API", Status: StatusInProgress,
		Priority: PriorityMust, Sprint: 1, Rank: "12", Assignee: "Ana Souza <ana@exemplo.com>", Agent: "claude-code",
		EstimateHours: 6, Dependencies: []string{"TASK-DB-01"}, Requirements: []string{"RF003"},
		Branch: "sprint-01/TASK-API-01-emitir-jwt", Source: "component:node-auth", Overrides: []string{"title"},
		Description: "Emite tokens.\n\nCom refresh.",
		Acceptance:  []Criterion{{Text: "POST /auth/login devolve 200", Done: true}, {Text: "Token expirado devolve 401"}},
		Handoff: &Handoff{LastStep: "emissão RS256 pronta", NextStep: "refresh token",
			Files: []string{"internal/auth/jwt.go", "internal/auth/jwt_test.go"}, FailingTests: []string{"TestRefresh"},
			UpdatedAt: "2026-09-25T14:30:00Z", By: "Ana Souza"},
		Checks: []CheckResult{{Skill: "segredos-e-config", Check: "Nenhum segredo no diff", Result: CheckOK, Evidence: "gitleaks: 0 achados"}},
		Notes:  "Ver ADR-002.",
	}
	md, err := RenderWorkItem(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"id: TASK-API-01", "rank: \"12\"", "# TASK-API-01 · Emitir JWT", "- [x] POST /auth/login", "- Próximo passo: refresh token", "- [ok] segredos-e-config · Nenhum segredo no diff — gitleaks: 0 achados"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown sem %q:\n%s", want, md)
		}
	}
	out, err := ParseWorkItem(md)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("ida e volta diferente:\n in=%+v\nout=%+v", in, out)
	}
}

func TestParseWorkItemKeepsUnknownSections(t *testing.T) {
	src := "---\nid: bug-007\ntype: defeito\ntitle: Falha\nstatus: em revisão\nrank: V\n---\n\n## Descrição\n\nQuebra.\n\n## Como reproduzir\n\n1. abrir\n"
	w, err := ParseWorkItem(src)
	if err != nil {
		t.Fatal(err)
	}
	if w.ID != "BUG-007" || w.Type != ItemBug || w.Status != StatusReview {
		t.Fatalf("normalização falhou: %+v", w)
	}
	if !strings.Contains(w.Notes, "### Como reproduzir") {
		t.Fatalf("seção desconhecida perdida: %q", w.Notes)
	}
}

func TestParseWorkItemRejectsConflicts(t *testing.T) {
	src := "---\nid: TASK-1\n<<<<<<< HEAD\nstatus: pending\n=======\nstatus: completed\n>>>>>>> outra\n---\n"
	if _, err := ParseWorkItem(src); err == nil {
		t.Fatal("arquivo com conflito deveria falhar")
	}
}

func TestSessionAndNoteRoundTrip(t *testing.T) {
	s := &Session{ID: "2026-09-25-1430-ana-auth", Author: "Ana", Agent: "claude-code", Started: "2026-09-25T14:30:00Z",
		Sprint: 1, Tasks: []string{"TASK-API-01"}, Summary: "Implementou JWT.", Done: []string{"emissão"},
		Decisions: []string{"RS256"}, NextSteps: []string{"refresh"}, Commands: []string{"go test ./... → ok"},
		Checks: []CheckResult{{Skill: "testes-e-aceite", Check: "testes passam", Result: CheckOK}}}
	md, err := RenderSession(s)
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseSession(md)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s, back) {
		t.Fatalf("sessão diferente:\n%+v\n%+v", s, back)
	}

	n := &Note{Title: "Usar pgx", Type: "decisao", Tags: []string{"db"}, Body: "Usamos pgx, não database/sql."}
	md, err = RenderNote(n)
	if err != nil {
		t.Fatal(err)
	}
	nb, err := ParseNote(md)
	if err != nil {
		t.Fatal(err)
	}
	if nb.Title != n.Title || nb.Body != n.Body || NoteSummary(nb) != "Usamos pgx, não database/sql." {
		t.Fatalf("memória diferente: %+v", nb)
	}
}

func TestPlanHelpers(t *testing.T) {
	p := &Plan{Items: []WorkItem{{ID: "TASK-001", Type: ItemTask}, {ID: "TASK-CORE-01", Type: ItemTask, Aliases: []string{"TASK-009"}}, {ID: "BUG-002", Type: ItemBug}}}
	if got := p.NextItemID(ItemTask); got != "TASK-010" {
		t.Fatalf("NextItemID = %s", got)
	}
	if got := p.NextItemID(ItemBug); got != "BUG-003" {
		t.Fatalf("NextItemID bug = %s", got)
	}
	if p.Item("task-009") == nil {
		t.Fatal("alias não encontrado")
	}
	if refs := ItemRefs("Sprint 01 - Implementa [TASK-API-01] e [BUG-7]"); strings.Join(refs, ",") != "TASK-API-01,BUG-7" {
		t.Fatalf("refs = %v", refs)
	}
	if NormalizePriority("Média") != PriorityShould || NormalizePriority("") != "" {
		t.Fatal("prioridade")
	}
}

func TestConventionsNormalize(t *testing.T) {
	c := &Conventions{AI: AIConventions{Autonomy: "supervised"}}
	c.Normalize()
	if c.Git.Commit != "Sprint {sprint} - {summary} [{id}]" || c.AI.Autonomy != AutonomySupervised || c.Planning.Slicing != SlicingHybrid {
		t.Fatalf("normalização: %+v", c)
	}
	cc := DefaultConventions(PresetConventional)
	if !strings.HasPrefix(cc.Git.Commit, "{type}") || len(cc.Git.Verbs) != 0 {
		t.Fatal("preset conventional")
	}
	if LegacyConventions().Planning.Slicing != SlicingComponent {
		t.Fatal("legado deveria manter fatiamento por componente")
	}
}
