package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/store"
)

func TestDiagramaUMLCicloCompleto(t *testing.T) {
	a, st := newTestApp(t)

	d, err := a.CreateUMLDiagram("class", "Modelo de Domínio", "", hub.SourceUI)
	if err != nil {
		t.Fatalf("CreateUMLDiagram: %v", err)
	}
	if d.ID != "modelo-de-dominio" {
		t.Errorf("id deveria ser o slug do nome, got %q", d.ID)
	}
	if _, err := a.CreateUMLDiagram("class", "Modelo de Domínio", "", hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if !st.Exists(".arch/diagrams/class/modelo-de-dominio-2.json") {
		t.Error("colisão de nome deveria gerar sufixo -2")
	}
	if _, err := a.CreateUMLDiagram("er", "X", "", hub.SourceUI); err == nil {
		t.Error("tipo inválido deveria ser rejeitado")
	}

	user, err := a.AddUMLElement(d.ID, UMLElementInput{UMLElement: model.UMLElement{
		Type: "class", Name: "User",
		Attributes: []model.UMLMember{{Name: "email", Type: "string", Visibility: "+"}},
	}}, hub.SourceAI)
	if err != nil {
		t.Fatalf("AddUMLElement: %v", err)
	}
	if user.ID != "el-user" {
		t.Errorf("id do elemento: %q", user.ID)
	}
	role, err := a.AddUMLElement(d.ID, UMLElementInput{UMLElement: model.UMLElement{Type: "class", Name: "Role"}}, hub.SourceAI)
	if err != nil {
		t.Fatal(err)
	}
	if role.Position == user.Position {
		t.Error("posição automática sobrepôs o elemento existente")
	}
	fixed := model.Position{X: 0, Y: 0}
	if el, _ := a.AddUMLElement(d.ID, UMLElementInput{UMLElement: model.UMLElement{Type: "class", Name: "Origem"}, Position: &fixed}, hub.SourceUI); el.Position != fixed {
		t.Errorf("posição explícita (0,0) foi ignorada: %+v", el.Position)
	}

	// Relação por nome, case-insensitive.
	rel, err := a.AddUMLRelation(d.ID, model.UMLRelation{Type: "association", Source: "user", Target: "ROLE",
		SourceMultiplicity: "*", TargetMultiplicity: "1..*"}, hub.SourceAI)
	if err != nil {
		t.Fatalf("AddUMLRelation: %v", err)
	}
	if rel.Source != "el-user" || rel.Target != "el-role" || rel.ID != "rel-1" {
		t.Errorf("relação resolvida incorretamente: %+v", rel)
	}
	if _, err := a.AddUMLRelation(d.ID, model.UMLRelation{Type: "message", Source: "User", Target: "Role"}, hub.SourceAI); err == nil {
		t.Error("message não pertence a diagrama de classes")
	}

	// Merge: só as chaves enviadas mudam; a posição é preservada.
	before, _ := st.LoadUMLDiagram(d.ID)
	pos := before.ElementByID("el-user").Position
	upd, err := a.UpdateUMLElement(d.ID, "el-user", map[string]any{
		"stereotype": "entity", "operations": []any{"+ login(email: string): Token"},
	}, hub.SourceUI)
	if err != nil {
		t.Fatalf("UpdateUMLElement: %v", err)
	}
	if upd.Stereotype != "entity" || upd.Name != "User" || len(upd.Attributes) != 1 || upd.Position != pos {
		t.Errorf("merge incorreto: %+v", upd)
	}
	if upd.Operations[0].Type != "Token" {
		t.Errorf("operação em notação UML não interpretada: %+v", upd.Operations)
	}
	if _, err := a.UpdateUMLElement(d.ID, "el-user", map[string]any{"name": ""}, hub.SourceUI); err == nil {
		t.Error("remover o nome de uma classe deveria falhar")
	}

	// Cascata: remover User leva a associação junto.
	if err := a.RemoveUMLElement(d.ID, "el-user", hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	after, _ := st.LoadUMLDiagram(d.ID)
	if len(after.Relations) != 0 {
		t.Errorf("relações ligadas ao elemento removido sobreviveram: %+v", after.Relations)
	}
	if err := a.RemoveUMLElement(d.ID, "el-user", hub.SourceUI); !errors.Is(err, model.ErrUMLNotFound) {
		t.Errorf("remover de novo deveria ser ErrUMLNotFound, got %v", err)
	}

	// Espelho Mermaid acompanha cada gravação (sync_mermaid ativo por padrão).
	mmd, err := st.ReadFile(store.UMLMermaidPath("class", d.ID))
	if err != nil || !strings.Contains(string(mmd), "classDiagram") || strings.Contains(string(mmd), "user") {
		t.Errorf("espelho Mermaid desatualizado: %v\n%s", err, mmd)
	}

	// Renomear não muda o id.
	name := "Domínio"
	renamed, err := a.RenameUMLDiagram(d.ID, &name, nil, hub.SourceUI)
	if err != nil || renamed.ID != d.ID || renamed.Name != "Domínio" {
		t.Errorf("rename: %+v %v", renamed, err)
	}

	if err := a.DeleteUMLDiagram(d.ID, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if st.Exists(store.UMLPath("class", d.ID)) || st.Exists(store.UMLMermaidPath("class", d.ID)) {
		t.Error("delete deveria remover .json e .mermaid")
	}
}

func TestMensagensSaoNumeradas(t *testing.T) {
	a, st := newTestApp(t)
	d, _ := a.CreateUMLDiagram("sequence", "Login", "", hub.SourceUI)
	for _, n := range []string{"Web", "API", "DB"} {
		el, err := a.AddUMLElement(d.ID, UMLElementInput{UMLElement: model.UMLElement{Type: "lifeline", Name: n}}, hub.SourceAI)
		if err != nil {
			t.Fatal(err)
		}
		if el.LifelineKind != "participant" {
			t.Errorf("lifeline_kind padrão deveria ser participant, got %q", el.LifelineKind)
		}
	}
	add := func(src, dst string, order int) *model.UMLRelation {
		r, err := a.AddUMLRelation(d.ID, model.UMLRelation{Type: "message", Source: src, Target: dst, Order: order}, hub.SourceAI)
		if err != nil {
			t.Fatalf("mensagem %s→%s: %v", src, dst, err)
		}
		return r
	}
	m1 := add("Web", "API", 0)
	m2 := add("API", "DB", 0)
	m3 := add("API", "API", 0) // auto-mensagem
	if m1.Order != 1 || m2.Order != 2 || m3.Order != 3 || m1.MessageKind != "sync" {
		t.Fatalf("ordens/kind: %d %d %d %q", m1.Order, m2.Order, m3.Order, m1.MessageKind)
	}
	// Inserir com ordem explícita desloca as seguintes.
	m0 := add("DB", "API", 1)
	cur, _ := st.LoadUMLDiagram(d.ID)
	if m0.Order != 1 || cur.RelationByID(m1.ID).Order != 2 || cur.RelationByID(m3.ID).Order != 4 {
		t.Errorf("inserção com order não deslocou: %+v", cur.Relations)
	}

	// Remover uma mensagem renumera 1..n.
	if _, err := a.RemoveUMLItem(d.ID, m1.ID, hub.SourceAI); err != nil {
		t.Fatal(err)
	}
	cur, _ = st.LoadUMLDiagram(d.ID)
	got := map[string]int{}
	for _, r := range cur.Relations {
		got[r.ID] = r.Order
	}
	if got[m0.ID] != 1 || got[m2.ID] != 2 || got[m3.ID] != 3 {
		t.Errorf("renumeração após remoção: %v", got)
	}

	// PATCH de order move a mensagem.
	if _, err := a.UpdateUMLRelation(d.ID, m3.ID, map[string]any{"order": 1}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	cur, _ = st.LoadUMLDiagram(d.ID)
	if cur.RelationByID(m3.ID).Order != 1 || cur.RelationByID(m0.ID).Order != 2 || cur.RelationByID(m2.ID).Order != 3 {
		t.Errorf("reordenação: %+v", cur.Relations)
	}

	// Remover a lifeline leva as mensagens dela e renumera o resto.
	if err := a.RemoveUMLElement(d.ID, "DB", hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	cur, _ = st.LoadUMLDiagram(d.ID)
	if len(cur.Relations) != 1 || cur.Relations[0].Order != 1 {
		t.Errorf("cascata na sequência: %+v", cur.Relations)
	}
}

func TestGerarDiagramaDeCasosDeUso(t *testing.T) {
	a, st := newTestApp(t)
	if _, _, err := a.UpsertUseCase(model.UseCase{Name: "Autenticar", Actors: []string{"Usuário"}}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	d, err := a.GenerateUseCaseDiagram("", hub.SourceAI)
	if err != nil {
		t.Fatalf("GenerateUseCaseDiagram: %v", err)
	}
	if d.ID != DefaultUseCaseDiagramID || len(d.Elements) != 3 || len(d.Relations) != 1 {
		t.Fatalf("diagrama gerado: %+v", d)
	}

	// O usuário arrasta o caso de uso; regenerar preserva a posição e só
	// acrescenta o novo caso de uso.
	moved := model.Position{X: 777, Y: 555}
	if _, err := a.UpdateUMLElement(d.ID, "el-cdu001", map[string]any{"position": moved}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.UpsertUseCase(model.UseCase{Name: "Sair", Actors: []string{"usuário"}}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	d, err = a.GenerateUseCaseDiagram("", hub.SourceAI)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Elements) != 4 || len(d.Relations) != 2 {
		t.Errorf("regeneração deveria adicionar só CDU002 e uma associação: %d elementos, %d relações", len(d.Elements), len(d.Relations))
	}
	if d.ElementByID("el-cdu001").Position != moved {
		t.Error("posição do usuário foi perdida na regeneração")
	}

	// AI-PRD ganha a seção UML e o hash muda quando o modelo muda.
	res1, err := a.GenerateAIPRD(prd.Options{Granularity: "detailed"}, hub.SourceAI)
	if err != nil {
		t.Fatal(err)
	}
	md, _ := st.ReadFile(store.FileAIPRD)
	if !strings.Contains(string(md), "## 7. UML MODELS") || !strings.Contains(string(md), "### Casos de Uso (usecase)") {
		t.Errorf("ai-prd.md sem seção UML")
	}
	if _, err := a.AddUMLElement(d.ID, UMLElementInput{UMLElement: model.UMLElement{Type: "actor", Name: "Admin"}}, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	res2, _ := a.GenerateAIPRD(prd.Options{Granularity: "detailed"}, hub.SourceAI)
	if res1.Hash == res2.Hash {
		t.Error("hash de integridade deveria mudar com o diagrama UML")
	}

	sum, _ := a.ContextSummary(false)
	if sum.Counts["uml_diagrams"] != 1 || len(sum.UMLDiagrams) != 1 {
		t.Errorf("get_system_context sem resumo UML: %+v", sum.UMLDiagrams)
	}
}
