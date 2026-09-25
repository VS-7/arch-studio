package conventions

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/model"
)

func archcode() *model.Conventions { return model.DefaultConventions("") }

func TestThirdPerson(t *testing.T) {
	cases := map[string]string{
		"Implementar": "Implementa", "Corrigir": "Corrige", "Emitir": "Emite", "Distribuir": "Distribui",
		"Fazer": "Faz", "remover": "remove", "Construir": "Constrói",
	}
	for in, want := range cases {
		got, ok := ThirdPerson(in)
		if !ok || got != want {
			t.Errorf("ThirdPerson(%q) = %q,%v; quer %q", in, got, ok, want)
		}
	}
	if _, ok := ThirdPerson("Login"); ok {
		t.Error("Login não é verbo")
	}
}

func TestSummarize(t *testing.T) {
	c := archcode()
	cases := []struct{ title, typ, want string }{
		{"Implementar Core API (Go)", model.ItemTask, "Implementa Core API (Go)"},
		{"Login com e-mail", model.ItemTask, "Implementa login com e-mail"},
		{"JWT expira cedo", model.ItemBug, "Corrige JWT expira cedo"},
		{"Corrige vazamento", model.ItemBug, "Corrige vazamento"},
	}
	for _, cs := range cases {
		if got := Summarize(cs.title, cs.typ, c); got != cs.want {
			t.Errorf("Summarize(%q) = %q; quer %q", cs.title, got, cs.want)
		}
	}
	cc := model.DefaultConventions(model.PresetConventional)
	if got := Summarize("Implementar login", model.ItemTask, cc); got != "implementa login" {
		t.Errorf("conventional: %q", got)
	}
}

func TestBranchAndCommit(t *testing.T) {
	c := archcode()
	item := &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emitir JWT no Auth API",
		Parent: "ST-RF003", Requirements: []string{"RF003"}, UseCases: []string{"CDU001"}, Sprint: 1}
	if got := BranchName(c, item, 1); got != "sprint-01/TASK-API-01-emitir-jwt-no-auth-api" {
		t.Fatalf("branch = %q", got)
	}
	bug := &model.WorkItem{ID: "BUG-007", Type: model.ItemBug, Title: "Sessão vaza"}
	if got := BranchName(c, bug, 0); got != "hotfix/BUG-007-sessao-vaza" {
		t.Fatalf("branch fora de sprint = %q", got)
	}
	subject := CommitSubject(c, CommitInput{Item: item, Sprint: 1})
	if subject != "Sprint 01 - Emite JWT no Auth API [TASK-API-01]" {
		t.Fatalf("assunto = %q", subject)
	}
	if got := CommitSubject(c, CommitInput{Item: bug}); got != "Hotfix - Corrige sessão vaza [BUG-007]" {
		t.Fatalf("assunto fora de sprint = %q", got)
	}
	msg := CommitMessage(c, CommitInput{Item: item, Sprint: 1, Body: "Assina com RS256.", CoAuthor: "Claude <noreply@anthropic.com>"})
	for _, want := range []string{"\n\nAssina com RS256.\n\n", "Task: TASK-API-01\n", "Story: ST-RF003\n", "Refs: RF003, CDU001\n", "Co-Authored-By: Claude"} {
		if !strings.Contains(msg, want) {
			t.Errorf("mensagem sem %q:\n%s", want, msg)
		}
	}
	long := &model.WorkItem{ID: "TASK-VERYLONG-01", Type: model.ItemTask,
		Title: "Implementar integração completa com o gateway de pagamentos e conciliação diária de transações"}
	s := CommitSubject(c, CommitInput{Item: long, Sprint: 12})
	if len([]rune(s)) > 72 || !strings.HasSuffix(s, "… [TASK-VERYLONG-01]") {
		t.Fatalf("assunto longo mal encurtado (%d): %q", len([]rune(s)), s)
	}
	if issues := ValidateSubject(c, s, nil); len(issues) != 0 {
		t.Fatalf("assunto gerado deveria ser válido: %+v", issues)
	}
}

func TestValidateSubject(t *testing.T) {
	c := archcode()
	plan := &model.Plan{Items: []model.WorkItem{{ID: "TASK-API-01", Sprint: 1}}}
	ok := []string{
		"Sprint 01 - Implementa emissão de JWT [TASK-API-01]",
		"Sprint 01 - Implementa emissão de JWT [TASK-API-01] (#12)",
		"Merge branch 'main' into x",
		"Revert \"Sprint 01 - Implementa x [TASK-API-01]\"",
	}
	for _, s := range ok {
		for _, is := range ValidateSubject(c, s, plan) {
			if is.Level == LevelError {
				t.Errorf("%q deveria passar: %s", s, is.Message)
			}
		}
	}
	bad := map[string]string{
		"Implementa JWT":                                            "fora da convenção",
		"Sprint 01 - Implementa JWT":                                "fora da convenção",
		"Sprint 01 - Implementa JWT [TASK-NOPE-01]":                 "não existe",
		"Sprint 1 - Implementa JWT [TASK-API-01]":                   "fora da convenção",
		"Hotfix - Corrige sessão [BUG-9]":                           "não existe",
		"Sprint 01 - " + strings.Repeat("x", 70) + " [TASK-API-01]": "caracteres",
	}
	for s, want := range bad {
		issues := ValidateSubject(c, s, plan)
		found := false
		for _, is := range issues {
			if is.Level == LevelError && strings.Contains(is.Message, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%q: esperava erro com %q, veio %+v", s, want, issues)
		}
	}
	warn := ValidateSubject(c, "Sprint 02 - Faz coisas [TASK-API-01]", plan)
	if len(warn) != 2 || warn[0].Level != LevelWarning || warn[1].Level != LevelWarning {
		t.Fatalf("esperava 2 avisos (verbo e sprint): %+v", warn)
	}
}

func TestValidateBranchAndPR(t *testing.T) {
	c := archcode()
	for _, b := range []string{"main", "sprint-01/TASK-API-01-emitir-jwt", "hotfix/BUG-007-sessao", "Chore/TASK-001-deps", "dependabot/npm/vite-5"} {
		if issues := ValidateBranch(c, b); len(issues) != 0 {
			t.Errorf("branch %q deveria passar: %+v", b, issues)
		}
	}
	for _, b := range []string{"feature/login", "sprint-01/login", "sprint-1/TASK-1-x"} {
		if issues := ValidateBranch(c, b); len(issues) == 0 {
			t.Errorf("branch %q deveria falhar", b)
		}
	}
	if issues := ValidatePRTitle(c, "Sprint 01 - Autenticação com JWT [ST-RF003] (#4)", nil); len(issues) != 0 {
		t.Fatalf("título válido rejeitado: %+v", issues)
	}
	if issues := ValidatePRTitle(c, "Adiciona login", nil); len(issues) != 1 {
		t.Fatal("título inválido aceito")
	}
}

func TestConventionalPreset(t *testing.T) {
	c := model.DefaultConventions(model.PresetConventional)
	item := &model.WorkItem{ID: "BUG-007", Type: model.ItemBug, Title: "Sessão vaza"}
	subject := CommitSubject(c, CommitInput{Item: item, Sprint: 3})
	if subject != "fix: corrige sessão vaza [BUG-007]" {
		t.Fatalf("assunto = %q", subject)
	}
	if issues := ValidateSubject(c, "feat(auth)!: implementa login [TASK-001]", nil); len(issues) != 0 {
		t.Fatalf("conventional com escopo rejeitado: %+v", issues)
	}
	if got := BranchName(c, item, 3); got != "fix/BUG-007-sessao-vaza" {
		t.Fatalf("branch = %q", got)
	}
}

func TestChangelogAndPR(t *testing.T) {
	c := archcode()
	commits := []gitx.Commit{
		{Hash: "3", Parents: 1, Subject: "Sprint 02 - Implementa perfil [TASK-WEB-01] (#7)"},
		{Hash: "2", Parents: 1, Subject: "Hotfix - Corrige sessão [BUG-001]"},
		{Hash: "1", Parents: 1, Subject: "Sprint 01 - Implementa banco [TASK-DB-01]"},
		{Hash: "p", Parents: 1, Subject: "Sprint 01 - Atualiza o planejamento da Sprint 01 [SPRINT-01]"},
		{Hash: "r", Parents: 1, Subject: "Sprint 01 - Reserva TASK-DB-01 [TASK-DB-01]"},
		{Hash: "0", Parents: 1, Subject: "Commit inicial"},
	}
	block := ChangelogBlock(c, commits, []model.Sprint{{Number: 2, Goal: "Perfil", Start: "2026-10-12", End: "2026-10-23"}})
	iS2, iS1 := strings.Index(block, "## Sprint 02 — Perfil (2026-10-12 a 2026-10-23)"), strings.Index(block, "## Sprint 01")
	if iS2 < 0 || iS1 < 0 || iS2 > iS1 {
		t.Fatalf("seções fora de ordem:\n%s", block)
	}
	for _, want := range []string{"- Implementa perfil [TASK-WEB-01] (#7)", "- Hotfix: Corrige sessão [BUG-001]"} {
		if !strings.Contains(block, want) {
			t.Errorf("changelog sem %q:\n%s", want, block)
		}
	}
	if strings.Contains(block, "Commit inicial") || strings.Contains(block, "planejamento") || strings.Contains(block, "Reserva") {
		t.Errorf("commit fora da convenção, de planejamento ou de reserva não deveria entrar:\n%s", block)
	}
	doc := Changelog("# Changelog\n\nTexto meu.\n", block)
	if !strings.Contains(doc, "Texto meu.") || !strings.Contains(Changelog(doc, "novo"), "novo") {
		t.Fatal("bloco gerenciado não preservou o arquivo")
	}

	sp := &model.Sprint{Number: 1, Name: "Sprint 01", Goal: "Autenticação"}
	item := &model.WorkItem{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emissão de JWT", Component: "Core API",
		Endpoints: []string{"POST /auth/login"}, Acceptance: []model.Criterion{{Text: "login 200", Done: true}},
		Checks: []model.CheckResult{{Skill: "testes-e-aceite", Check: "testes passam", Result: model.CheckOK, Evidence: "go test ./... ok"}}}
	in := PRInput{Items: []*model.WorkItem{item}, Sprint: sp, Skills: []SkillCheck{{Skill: "segredos-e-config", Check: "Nenhum segredo"}}}
	if title := PRTitle(c, in); title != "Sprint 01 - Emissão de JWT [TASK-API-01]" {
		t.Fatalf("título = %q", title)
	}
	body := PRBody(c, in)
	for _, want := range []string{"## Sprint 01 — Autenticação", "Fecha: TASK-API-01", "- [x] login 200",
		"- [x] testes-e-aceite · testes passam — go test ./... ok", "- [ ] segredos-e-config · Nenhum segredo",
		"- go test ./... ok", "Endpoints: POST /auth/login"} {
		if !strings.Contains(body, want) {
			t.Errorf("corpo sem %q:\n%s", want, body)
		}
	}
}

func TestIsReservation(t *testing.T) {
	c := archcode()
	if !IsReservation(c, "Sprint 01 - Reserva TASK-API-01 [TASK-API-01]") {
		t.Fatal("commit de reserva não reconhecido")
	}
	if IsReservation(c, "Sprint 01 - Implementa reserva de hotel [TASK-API-01]") {
		t.Fatal("entrega com a palavra reserva foi tratada como reserva")
	}
}

func TestHookScript(t *testing.T) {
	s := HookScript("commit-msg", "/opt/arch's/bin")
	for _, want := range []string{HookMarker, `exec "$BIN" hook commit-msg "$1"`, `BIN='/opt/arch'\''s/bin'`} {
		if !strings.Contains(s, want) {
			t.Errorf("hook sem %q:\n%s", want, s)
		}
	}
}
