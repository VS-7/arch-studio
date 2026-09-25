package plan

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/archcode/studio/internal/model"
)

func TestBetweenKeepsOrder(t *testing.T) {
	cases := []struct{ a, b string }{
		{"", ""}, {"V", ""}, {"", "V"}, {"z", ""}, {"", "1"}, {"a", "a1"}, {"a0V", "a1"}, {"V", "W"}, {"Vz", "W"},
	}
	for _, c := range cases {
		got := Between(c.a, c.b)
		if !validRank(got) {
			t.Errorf("Between(%q,%q) = %q, chave inválida", c.a, c.b, got)
		}
		if c.a != "" && got <= c.a {
			t.Errorf("Between(%q,%q) = %q, não é maior que a", c.a, c.b, got)
		}
		if c.b != "" && got >= c.b {
			t.Errorf("Between(%q,%q) = %q, não é menor que b", c.a, c.b, got)
		}
	}
}

func TestBetweenRepeatedInsertions(t *testing.T) {
	// Inserir sempre logo depois do primeiro item nunca colide nem inverte.
	keys := []string{After("")}
	for i := 0; i < 200; i++ {
		next := ""
		if len(keys) > 1 {
			next = keys[1]
		}
		k := Between(keys[0], next)
		keys = append([]string{keys[0], k}, keys[1:]...)
	}
	if !sort.StringsAreSorted(keys) {
		t.Fatal("chaves fora de ordem")
	}
	seen := map[string]bool{}
	for _, k := range keys {
		if seen[k] {
			t.Fatalf("chave repetida %q", k)
		}
		seen[k] = true
	}
}

func TestBetweenToleratesInvalidInput(t *testing.T) {
	if got := Between("zz", "a"); got <= "zz" {
		t.Fatalf("fora de ordem deveria cair para depois de a: %q", got)
	}
	if got := Between("a0", ""); !validRank(got) {
		t.Fatalf("chave inválida: %q", got)
	}
}

func TestSpread(t *testing.T) {
	for _, n := range []int{1, 5, 60, 500, 2500} {
		keys := Spread(n)
		if len(keys) != n {
			t.Fatalf("Spread(%d) devolveu %d chaves", n, len(keys))
		}
		if !sort.StringsAreSorted(keys) {
			t.Fatalf("Spread(%d) fora de ordem", n)
		}
		for i, k := range keys {
			if !validRank(k) {
				t.Fatalf("Spread(%d)[%d] = %q inválida", n, i, k)
			}
			if i > 0 && keys[i-1] == k {
				t.Fatalf("Spread(%d) repetiu %q", n, k)
			}
		}
	}
}

// ---------------------------------------------------------------------------

func sampleInput() Input {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "node-web", Type: "client", Data: model.NodeData{Label: "Web App", UseCases: []string{"CDU001"}}},
		{ID: "node-api", Type: "compute", Data: model.NodeData{Label: "Core API", Requirements: []string{"RF001"}, UseCases: []string{"CDU001"}}},
		{ID: "node-db", Type: "database", Data: model.NodeData{Label: "Postgres"}},
	}
	d.Edges = []model.Edge{{ID: "e1", Source: "node-web", Target: "node-api"}, {ID: "e2", Source: "node-api", Target: "node-db"}}
	reqs := &model.RequirementsDoc{Requirements: []model.Requirement{
		{ID: "RF001", Type: "RF", Title: "Autenticar", Priority: "Alta"},
		{ID: "RF002", Type: "RF", Title: "Ver perfil", Priority: "Média", Components: []string{"Web App"}},
		{ID: "RNF001", Type: "RNF", Title: "JWT"},
	}}
	ucs := []model.UseCase{{Code: "CDU001", Name: "Autenticar usuário", Requirements: []string{"RF001"},
		MainFlow: []string{"Usuário informa credenciais", "Sistema valida"}, Acceptance: []string{"Login válido entra"}}}
	board := &model.TaskBoard{Tasks: []model.Task{
		{ID: "TASK-POSTGRES-01", Title: "Implementar Postgres", ComponentID: "node-db", Component: "Postgres", Tier: "data", Order: 1},
		{ID: "TASK-CORE-01", Title: "Implementar Core API", ComponentID: "node-api", Component: "Core API", Tier: "backend",
			Dependencies: []string{"TASK-POSTGRES-01"}, Requirements: []string{"RF001"}, Order: 2},
		{ID: "TASK-WEB-01", Title: "Implementar Web App", ComponentID: "node-web", Component: "Web App", Tier: "frontend",
			Dependencies: []string{"TASK-CORE-01"}, Requirements: []string{"RNF001"}, Order: 3},
		{ID: "TASK-E2E-01", Title: "Testes ponta a ponta", Tier: "devops",
			Dependencies: []string{"TASK-CORE-01", "TASK-POSTGRES-01", "TASK-WEB-01"}, Order: 4},
	}}
	return Input{Diagram: d, Requirements: reqs, UseCases: ucs, Board: board, Slicing: model.SlicingComponent}
}

func ids(items []model.WorkItem) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestGenerateComponentMode(t *testing.T) {
	items := Generate(sampleInput())
	got := strings.Join(ids(items), ",")
	want := "EP-CDU001,ST-RF001,ST-RF002,TASK-POSTGRES-01,TASK-CORE-01,TASK-WEB-01,TASK-E2E-01"
	if got != want {
		t.Fatalf("ids = %s\nwant %s", got, want)
	}
	story := items[1]
	if story.Parent != "EP-CDU001" || story.Priority != model.PriorityMust {
		t.Fatalf("história mal ligada: parent=%q priority=%q", story.Parent, story.Priority)
	}
	if len(story.Acceptance) == 0 {
		t.Fatal("história sem critérios")
	}
	core := items[4]
	if core.Source != "component:node-api" || core.Parent != "ST-RF001" || core.Priority != model.PriorityMust {
		t.Fatalf("tarefa de componente inesperada: %+v", core)
	}
	if items[5].Parent != "" {
		t.Fatalf("tarefa ligada só a RNF não pode ter história pai: %q", items[5].Parent)
	}
	if items[6].Source != SourceE2E {
		t.Fatalf("E2E sem origem: %q", items[6].Source)
	}
}

func TestGenerateHybridAddsSlices(t *testing.T) {
	in := sampleInput()
	in.Slicing = model.SlicingHybrid
	items := Generate(in)
	var slices []model.WorkItem
	var e2e model.WorkItem
	for _, it := range items {
		if strings.HasPrefix(it.Source, SourceSlice) {
			slices = append(slices, it)
		}
		if it.Source == SourceE2E {
			e2e = it
		}
	}
	if len(slices) != 2 {
		t.Fatalf("esperava 2 fatias (RF001×Core API, RF002×Web App), veio %v", ids(slices))
	}
	if slices[0].ID != "TASK-RF001-CORE" || slices[0].Dependencies[0] != "TASK-CORE-01" || slices[0].Parent != "ST-RF001" {
		t.Fatalf("fatia inesperada: %+v", slices[0])
	}
	if slices[1].ID != "TASK-RF002-WEB" {
		t.Fatalf("segunda fatia: %s", slices[1].ID)
	}
	for _, s := range slices {
		found := false
		for _, dep := range e2e.Dependencies {
			if dep == s.ID {
				found = true
			}
		}
		if !found {
			t.Fatalf("E2E não depende da fatia %s", s.ID)
		}
	}
}

func TestSyncCreatesThenPreservesHumanEdits(t *testing.T) {
	now := "2026-09-25T10:00:00Z"
	desired := Generate(sampleInput())
	res := Sync(model.NewPlan(), desired, now)
	if res.Counts()[ChangeAdded] != len(desired) {
		t.Fatalf("primeira sincronização deveria criar tudo: %+v", res.Counts())
	}
	p := &model.Plan{Items: res.Items}
	p.SortItems()
	if strings.Join(ids(p.Items), ",") != strings.Join(ids(desired), ",") {
		t.Fatal("ranks iniciais não seguem a ordem gerada")
	}

	// Humano renomeia a tarefa e avança o status; a arquitetura muda o título.
	core := p.Item("TASK-CORE-01")
	core.Title = "API principal (renomeada)"
	core.MarkOverride("title")
	core.Status = model.StatusInProgress
	core.Acceptance = []model.Criterion{{Text: "x", Done: true}}

	in := sampleInput()
	in.Board.Tasks[1].Title = "Implementar Core API v2"
	in.Board.Tasks[1].Acceptance = []string{"x", "y"}
	res = Sync(p, Generate(in), "2026-09-26T10:00:00Z")
	if len(res.Changes) != 1 || res.Changes[0].ID != "TASK-CORE-01" {
		t.Fatalf("esperava só TASK-CORE-01 alterada: %+v", res.Changes)
	}
	updated := res.Items[0]
	if updated.Title != "API principal (renomeada)" {
		t.Fatalf("título editado à mão foi sobrescrito: %q", updated.Title)
	}
	if updated.Status != model.StatusInProgress {
		t.Fatalf("status mudou: %q", updated.Status)
	}
	if len(updated.Acceptance) != 2 || !updated.Acceptance[0].Done || updated.Acceptance[1].Done {
		t.Fatalf("critérios não preservaram a marcação: %+v", updated.Acceptance)
	}
}

func TestSyncArchivesAndRestores(t *testing.T) {
	p := &model.Plan{Items: Sync(model.NewPlan(), Generate(sampleInput()), "t0").Items}
	p.SortItems()

	in := sampleInput()
	in.Board.Tasks = in.Board.Tasks[1:] // Postgres saiu da arquitetura
	res := Sync(p, Generate(in), "t1")
	var archived *model.WorkItem
	for i, c := range res.Changes {
		if c.Kind == ChangeArchived {
			archived = &res.Items[i]
		}
	}
	if archived == nil || archived.ID != "TASK-POSTGRES-01" || !archived.Archived {
		t.Fatalf("Postgres deveria ser arquivado: %+v", res.Changes)
	}
	// Aplica e traz o componente de volta.
	for _, it := range res.Items {
		*p.Item(it.ID) = it
	}
	res = Sync(p, Generate(sampleInput()), "t2")
	if len(res.Changes) != 1 || res.Changes[0].Kind != ChangeRestored {
		t.Fatalf("esperava restauração: %+v", res.Changes)
	}
}

func TestReadyAndNext(t *testing.T) {
	p := &model.Plan{Items: Sync(model.NewPlan(), Generate(sampleInput()), "t0").Items}
	p.SortItems()
	me := "Ana <ana@x.dev>"
	if next := NextReady(p, me); next == nil || next.ID != "TASK-POSTGRES-01" {
		t.Fatalf("próxima deveria ser o banco: %+v", next)
	}
	p.Item("TASK-POSTGRES-01").Status = model.StatusCompleted
	if next := NextReady(p, me); next == nil || next.ID != "TASK-CORE-01" {
		t.Fatalf("próxima deveria ser a API: %+v", next)
	}
	p.Item("TASK-CORE-01").Assignee = "Bruno <bruno@x.dev>"
	if next := NextReady(p, me); next != nil {
		t.Fatalf("API reservada por outro e Web bloqueada: nada pronto, veio %s", next.ID)
	}
	if !SamePerson("Ana Souza <ANA@x.dev>", me) || SamePerson("Ana", "Bruno") {
		t.Fatal("comparação de identidade incorreta")
	}
}

func TestProposeRespectsCapacityAndDependencies(t *testing.T) {
	p := &model.Plan{Items: Sync(model.NewPlan(), Generate(sampleInput()), "t0").Items}
	p.SortItems()
	for i := range p.Items {
		p.Items[i].EstimateHours = 10
	}
	prop := Propose(p, 1, 25)
	if strings.Join(prop.Items, ",") != "TASK-POSTGRES-01,TASK-CORE-01" {
		t.Fatalf("proposta = %v", prop.Items)
	}
	if prop.Hours != 20 || len(prop.Skipped) != 1 || prop.Skipped[0] != "TASK-WEB-01" {
		t.Fatalf("carga/skipped = %v %v", prop.Hours, prop.Skipped)
	}
}

func TestWorkingDaysAndCapacity(t *testing.T) {
	mon := time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC)
	end := EndAfter(mon, 10)
	if FormatDate(end) != "2026-10-09" {
		t.Fatalf("fim de sprint de 10 dias = %s", FormatDate(end))
	}
	if WorkingDays(mon, end) != 10 {
		t.Fatal("dias úteis incorretos")
	}
	if c := Capacity("2026-09-28", "2026-10-09", 2, 6); c != 120 {
		t.Fatalf("capacidade = %v", c)
	}
	sat := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	if !NextWorkingDay(sat).Equal(mon) {
		t.Fatal("sábado deveria virar segunda")
	}
}

func TestStale(t *testing.T) {
	it := &model.WorkItem{Status: model.StatusInProgress, Assignee: "Ana", ClaimedAt: "2026-09-21T09:00:00Z"}
	if !Stale(it, time.Date(2026, 9, 25, 9, 0, 0, 0, time.UTC), 3) {
		t.Fatal("4 dias úteis parada deveria ser stale")
	}
	if Stale(it, time.Date(2026, 9, 23, 9, 0, 0, 0, time.UTC), 3) {
		t.Fatal("2 dias úteis não é stale")
	}
}

func TestBoardRoundTrip(t *testing.T) {
	p := &model.Plan{Items: Sync(model.NewPlan(), Generate(sampleInput()), "t0").Items}
	p.SortItems()
	p.Item("TASK-CORE-01").Status = model.StatusReview
	b := ToBoard(p, &model.TaskBoard{TargetStack: "Go"})
	if len(b.Tasks) != 4 || b.TargetStack != "Go" || b.Tasks[0].Order != 1 {
		t.Fatalf("quadro derivado inesperado: %+v", b)
	}
	back := FromBoard(b, "t1")
	if back[1].Status != model.StatusReview || back[1].Source != "component:node-api" || back[3].Source != SourceE2E {
		t.Fatalf("migração perdeu dados: %+v", back[1])
	}
}

func TestReportMentionsDeliveredAndCarried(t *testing.T) {
	p := &model.Plan{Items: Sync(model.NewPlan(), Generate(sampleInput()), "t0").Items}
	p.SortItems()
	sp := model.Sprint{Number: 1, Name: "Sprint 01", Goal: "Base", Start: "2026-09-28", End: "2026-10-09"}
	p.Sprints = []model.Sprint{sp}
	p.Item("TASK-POSTGRES-01").Sprint, p.Item("TASK-POSTGRES-01").Status = 1, model.StatusCompleted
	md := Report(ReportInput{Plan: p, Sprint: &sp, Carried: []string{"TASK-CORE-01"},
		Commits: []CommitInfo{{Hash: "abcdef123", Subject: "Sprint 01 - Implementa banco [TASK-POSTGRES-01]"}}})
	for _, want := range []string{"Relatório da Sprint 01", "**TASK-POSTGRES-01**", "transferido para o backlog", "`abcdef1`"} {
		if !strings.Contains(md, want) {
			t.Errorf("relatório sem %q:\n%s", want, md)
		}
	}
}
