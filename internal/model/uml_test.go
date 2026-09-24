package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestParseUMLMember(t *testing.T) {
	cases := []struct {
		in   string
		want UMLMember
		op   bool
	}{
		{"+ email: string", UMLMember{Name: "email", Type: "string", Visibility: "+"}, false},
		{"- tentativas: int = 0", UMLMember{Name: "tentativas", Type: "int", Visibility: "-", Default: "0"}, false},
		{"+ login(email: string, senha: string): Token",
			UMLMember{Name: "login", Params: "email: string, senha: string", Type: "Token", Visibility: "+"}, true},
		{"#{static} novo(): User", UMLMember{Name: "novo", Type: "User", Visibility: "#", Static: true}, true},
		{"~ calcular()*", UMLMember{Name: "calcular", Visibility: "~", Abstract: true}, true},
		{"id", UMLMember{Name: "id"}, false},
	}
	for _, c := range cases {
		got, op := ParseUMLMember(c.in)
		if got != c.want || op != c.op {
			t.Errorf("ParseUMLMember(%q) = %+v (op=%v), want %+v (op=%v)", c.in, got, op, c.want, c.op)
		}
	}

	// Formatar e reinterpretar devolve o mesmo membro.
	m := UMLMember{Name: "login", Params: "email: string", Type: "Token", Visibility: "+", Static: true}
	back, op := ParseUMLMember(FormatUMLMember(m, true))
	if back != m || !op {
		t.Errorf("round-trip: %+v → %q → %+v", m, FormatUMLMember(m, true), back)
	}
}

func TestUMLMemberAceitaStringNoJSON(t *testing.T) {
	var el UMLElement
	data := `{"id":"el-a","type":"class","name":"A","position":{"x":0,"y":0},
		"attributes":["+ email: string",{"name":"id","type":"UUID"}],
		"operations":["+ login(email: string): Token"]}`
	if err := json.Unmarshal([]byte(data), &el); err != nil {
		t.Fatal(err)
	}
	if len(el.Attributes) != 2 || el.Attributes[0].Type != "string" || el.Attributes[1].Name != "id" {
		t.Errorf("atributos: %+v", el.Attributes)
	}
	if el.Operations[0].Params != "email: string" || el.Operations[0].Type != "Token" {
		t.Errorf("operação: %+v", el.Operations[0])
	}
	// A gravação é sempre no formato de objeto.
	out, _ := json.Marshal(el)
	var raw map[string]any
	_ = json.Unmarshal(out, &raw)
	if _, ok := raw["attributes"].([]any)[0].(map[string]any); !ok {
		t.Errorf("membro deveria ser serializado como objeto: %s", out)
	}
}

func seqDiagram() *UMLDiagram {
	d := NewUMLDiagram("seq", UMLKindSequence, "Login")
	d.Elements = []UMLElement{
		{ID: "a", Type: "lifeline", Name: "A"},
		{ID: "b", Type: "lifeline", Name: "B"},
		{ID: "c", Type: "lifeline", Name: "C"},
	}
	d.Relations = []UMLRelation{
		{ID: "m1", Type: "message", Source: "a", Target: "b", Order: 1},
		{ID: "m2", Type: "message", Source: "b", Target: "c", Order: 2},
		{ID: "m3", Type: "message", Source: "c", Target: "a", Order: 3},
		{ID: "m4", Type: "message", Source: "a", Target: "a", Order: 4},
	}
	d.Normalize()
	return d
}

func TestValidacaoDeTiposPorDiagrama(t *testing.T) {
	d := NewUMLDiagram("c", UMLKindClass, "Classes")
	d.Elements = []UMLElement{{ID: "x", Type: "lifeline", Name: "X"}}
	if err := d.Validate(); err == nil {
		t.Error("lifeline não deveria ser aceita num diagrama de classes")
	}

	d.Elements = []UMLElement{{ID: "x", Type: "class"}}
	if err := d.Validate(); err == nil {
		t.Error("classe sem nome deveria ser rejeitada")
	}

	d.Elements = []UMLElement{{ID: "x", Type: "class", Name: "X"}, {ID: "n", Type: "note"}}
	d.Relations = []UMLRelation{{ID: "r", Type: "generalization", Source: "x", Target: "x"}}
	if err := d.Validate(); err == nil {
		t.Error("generalização de um elemento para ele mesmo deveria ser rejeitada")
	}
	d.Relations = []UMLRelation{{ID: "r", Type: "association", Source: "x", Target: "n"}}
	if err := d.Validate(); err == nil {
		t.Error("nota só pode ser ligada por note_link")
	}
	d.Relations = []UMLRelation{{ID: "r", Type: "note_link", Source: "n", Target: "x"}}
	if err := d.Validate(); err != nil {
		t.Errorf("note_link válido rejeitado: %v", err)
	}
	d.Relations = []UMLRelation{{ID: "r", Type: "association", Source: "x", Target: "fantasma"}}
	if err := d.Validate(); !errors.Is(err, ErrNotFound) {
		t.Errorf("extremidade inexistente deveria ser ErrNotFound, got %v", err)
	}

	// Auto-mensagem e auto-transição são permitidas.
	if err := seqDiagram().Validate(); err != nil {
		t.Errorf("auto-mensagem rejeitada: %v", err)
	}
	st := NewUMLDiagram("s", UMLKindState, "Estados")
	st.Elements = []UMLElement{{ID: "i", Type: "initial"}, {ID: "s", Type: "state", Name: "Ativa"}}
	st.Relations = []UMLRelation{{ID: "t", Type: "transition", Source: "s", Target: "s", Trigger: "ping"}}
	if err := st.Validate(); err != nil {
		t.Errorf("auto-transição rejeitada: %v", err)
	}
}

func TestRemoverElementoCascataERenumera(t *testing.T) {
	d := seqDiagram()
	d.Elements = append(d.Elements, UMLElement{ID: "f", Type: "fragment", Name: "loop", ParentID: "b"})

	if !d.RemoveElement("b") {
		t.Fatal("elemento não removido")
	}
	for _, r := range d.Relations {
		if r.Source == "b" || r.Target == "b" {
			t.Errorf("relação órfã sobreviveu: %+v", r)
		}
	}
	if f := d.ElementByID("f"); f == nil || f.ParentID != "" {
		t.Errorf("parent_id do filho deveria ser zerado: %+v", f)
	}
	// Sobraram m3 (c→a) e m4 (a→a), agora numeradas 1 e 2.
	if d.RelationByID("m3").Order != 1 || d.RelationByID("m4").Order != 2 {
		t.Errorf("ordens não renumeradas: m3=%d m4=%d", d.RelationByID("m3").Order, d.RelationByID("m4").Order)
	}

	d = seqDiagram()
	d.RemoveRelation("m2")
	if d.RelationByID("m3").Order != 2 || d.RelationByID("m4").Order != 3 {
		t.Error("remover mensagem deveria renumerar as seguintes")
	}

	d = seqDiagram()
	d.MoveMessage("m4", 1)
	want := map[string]int{"m4": 1, "m1": 2, "m2": 3, "m3": 4}
	for id, o := range want {
		if d.RelationByID(id).Order != o {
			t.Errorf("MoveMessage: %s=%d, want %d", id, d.RelationByID(id).Order, o)
		}
	}
}

func TestAutoPositionNaoSobrepoe(t *testing.T) {
	d := NewUMLDiagram("c", UMLKindClass, "Classes")
	for i := 0; i < 8; i++ {
		el := UMLElement{ID: d.UniqueElementID("Classe", "class"), Type: "class", Name: "Classe"}
		el.Position = d.AutoPosition(el)
		for _, other := range d.Elements {
			if rectOf(el).overlaps(rectOf(other)) {
				t.Fatalf("%s em %+v sobrepõe %s em %+v", el.ID, el.Position, other.ID, other.Position)
			}
		}
		d.Elements = append(d.Elements, el)
	}
	if d.Elements[1].ID != "el-classe-2" {
		t.Errorf("id único esperado el-classe-2, got %s", d.Elements[1].ID)
	}

	s := seqDiagram()
	p := s.AutoPosition(UMLElement{Type: "lifeline", Name: "D"})
	if p.X != 40+3*200 || p.Y != 40 {
		t.Errorf("lifeline deveria ir para x=640,y=40, got %+v", p)
	}
}

func TestSyncUseCaseDiagramEhIdempotente(t *testing.T) {
	ucs := []UseCase{
		{Code: "CDU001", Name: "Autenticar", Actors: []string{"Usuário", "Admin"}},
		{Code: "CDU002", Name: "Bloquear conta", Actors: []string{"admin"}},
	}
	d := NewUMLDiagram("casos-de-uso", UMLKindUseCase, "Casos de Uso")
	if !SyncUseCaseDiagram(d, "Loja", ucs) {
		t.Fatal("primeira sincronização deveria alterar o diagrama")
	}
	// 1 boundary + 2 usecases + 2 atores (admin é o mesmo ator, case-insensitive).
	if len(d.Elements) != 5 || len(d.Relations) != 3 {
		t.Fatalf("got %d elementos e %d relações", len(d.Elements), len(d.Relations))
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("diagrama gerado inválido: %v", err)
	}

	moved := d.ResolveElement("CDU001")
	if moved == nil {
		moved = d.ElementByID("el-cdu001")
	}
	moved.Position = Position{X: 999, Y: 999}
	if SyncUseCaseDiagram(d, "Loja", ucs) {
		t.Error("segunda sincronização não deveria mudar nada")
	}
	if d.ElementByID("el-cdu001").Position.X != 999 {
		t.Error("posição definida pelo usuário foi perdida")
	}
	for _, e := range d.Elements {
		if e.Type == "usecase" && e.ParentID == "" {
			t.Errorf("usecase %s fora da fronteira", e.Name)
		}
	}
}
