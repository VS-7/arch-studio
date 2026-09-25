package skills

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func TestCatalogIsValid(t *testing.T) {
	c := model.DefaultConventions("")
	cat := Catalog(c)
	if len(cat) < 14 {
		t.Fatalf("catálogo com %d skills; esperava as 13 embutidas + convencoes-git", len(cat))
	}
	names := map[string]bool{}
	for _, s := range cat {
		if names[s.Name] {
			t.Errorf("skill repetida: %s", s.Name)
		}
		names[s.Name] = true
		if s.Body == "" || len(s.Description) > 1024 {
			t.Errorf("%s: corpo vazio ou descrição longa demais", s.Name)
		}
		if _, ok := CategoryLabels[s.Category]; !ok {
			t.Errorf("%s: categoria %q", s.Name, s.Category)
		}
		// Ida e volta pelo formato de arquivo.
		md, err := Render(&s)
		if err != nil {
			t.Fatal(err)
		}
		back, err := Parse(md)
		if err != nil || back.Name != s.Name || len(back.Checks) != len(s.Checks) {
			t.Errorf("%s: ida e volta falhou: %v", s.Name, err)
		}
	}
	for _, n := range DefaultSet {
		if !names[n] {
			t.Errorf("skill padrão ausente do catálogo: %s", n)
		}
	}
}

func TestParseRejectsBadSkill(t *testing.T) {
	for _, src := range []string{
		"sem frontmatter",
		"---\nname: Nome Ruim\ndescription: x\n---\n",
		"---\nname: ok-name\n---\n",
		"---\nname: ok-name\ndescription: x\nchecks:\n  - id: a\n    text: a\n  - id: a\n    text: b\n---\n",
	} {
		if _, err := Parse(src); err == nil {
			t.Errorf("deveria rejeitar:\n%s", src)
		}
	}
}

func TestDetectStacksAndApplies(t *testing.T) {
	d := model.NewDiagram()
	d.Nodes = []model.Node{
		{ID: "a", Data: model.NodeData{Technology: "Go 1.25 / net/http"}},
		{ID: "b", Data: model.NodeData{Technology: "React 19 + TypeScript"}},
		{ID: "c", Data: model.NodeData{Technology: "MongoDB"}},
	}
	stacks := DetectStacks(d, "")
	if strings.Join(stacks, ",") != "go,react,typescript" {
		t.Fatalf("stacks = %v (MongoDB não é Go)", stacks)
	}
	goStyle := &Skill{Name: "estilo-go", AppliesTo: &AppliesTo{Stacks: []string{"go"}}}
	backendOnly := &Skill{Name: "x", AppliesTo: &AppliesTo{Tiers: []string{"backend"}}}
	if !Applies(goStyle, "frontend", stacks) || Applies(goStyle, "", []string{"python"}) {
		t.Fatal("filtro por stack incorreto")
	}
	if Applies(backendOnly, "frontend", stacks) || !Applies(backendOnly, "", stacks) {
		t.Fatal("filtro por tier incorreto")
	}
	cat := Catalog(model.DefaultConventions(""))
	sug := strings.Join(Suggest(cat, stacks), ",")
	if !strings.Contains(sug, "estilo-go") || !strings.Contains(sug, "estilo-typescript") {
		t.Fatalf("sugestão sem estilos: %s", sug)
	}
}

func TestMissingChecks(t *testing.T) {
	list := []Skill{{Name: "s", Checks: []Check{
		{ID: "a", Text: "Testes passam", Required: true},
		{ID: "b", Text: "Sem segredo", Required: true},
		{ID: "c", Text: "Opcional"},
	}}}
	missing, failed := MissingChecks(list, []model.CheckResult{
		{Skill: "s", Check: "testes passam", Result: model.CheckOK},
		{Skill: "s", Check: "c", Result: model.CheckFail},
	})
	if strings.Join(missing, ",") != "s/b" || len(failed) != 0 {
		t.Fatalf("missing=%v failed=%v", missing, failed)
	}
	_, failed = MissingChecks(list, []model.CheckResult{{Skill: "s", Check: "a", Result: model.CheckFail}})
	if strings.Join(failed, ",") != "s/a" {
		t.Fatalf("failed=%v", failed)
	}
}

func TestExports(t *testing.T) {
	c := model.DefaultConventions("")
	s := CatalogSkill("segredos-e-config", c)
	if s == nil {
		t.Fatal("skill do catálogo ausente")
	}
	claude := ClaudeSkill(s)
	if !strings.HasPrefix(claude, "---\nname: segredos-e-config\ndescription: ") || !strings.Contains(claude, GeneratedMarker) ||
		!strings.Contains(claude, "## Checks") || strings.Contains(claude, "category:") {
		t.Fatalf("SKILL.md do Claude inesperado:\n%s", claude[:300])
	}
	cursor := CursorRule(s)
	if !strings.Contains(cursor, "alwaysApply: true") {
		t.Fatal("regra do Cursor sem alwaysApply")
	}
	block := AgentsBlock([]Skill{*s}, c)
	for _, want := range []string{"resume_work", "`Sprint 01 - Implementa emissão de JWT [TASK-API-01]`", "| `segredos-e-config` | sempre |"} {
		if !strings.Contains(block, want) {
			t.Errorf("AGENTS.md sem %q:\n%s", want, block)
		}
	}
	g := ConventionsSkill(model.DefaultConventions(model.PresetConventional))
	if !strings.Contains(g.Body, "`feat: implementa emissão de JWT [TASK-API-01]`") {
		t.Fatalf("convencoes-git não reflete o preset:\n%s", g.Body)
	}
}
