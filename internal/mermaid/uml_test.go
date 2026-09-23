package mermaid

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func member(s string) model.UMLMember {
	m, _ := model.ParseUMLMember(s)
	return m
}

func sampleClass() *model.UMLDiagram {
	d := model.NewUMLDiagram("dominio", model.UMLKindClass, "Domínio")
	d.Elements = []model.UMLElement{
		{ID: "el-pkg", Type: "package", Name: "auth"},
		{ID: "el-user", Type: "class", Name: "User", ParentID: "el-pkg",
			Attributes: []model.UMLMember{member("+ email: string")},
			Operations: []model.UMLMember{member("+ login(email: string): Token")}},
		{ID: "el-admin", Type: "class", Name: "Admin"},
		{ID: "el-base", Type: "class", Name: "Entity", Abstract: true},
		{ID: "el-issuer", Type: "interface", Name: "TokenIssuer"},
		{ID: "el-jwt", Type: "class", Name: "JWTIssuer"},
		{ID: "el-status", Type: "enum", Name: "UserStatus", Literals: []string{"ACTIVE", "BLOCKED"}},
		{ID: "el-session", Type: "class", Name: "Session"},
		{ID: "el-note", Type: "note", Documentation: "Senha só como hash"},
	}
	d.Relations = []model.UMLRelation{
		{ID: "r1", Type: "generalization", Source: "el-admin", Target: "el-user"},
		{ID: "r2", Type: "realization", Source: "el-jwt", Target: "el-issuer"},
		{ID: "r3", Type: "composition", Source: "el-session", Target: "el-user", SourceMultiplicity: "*", TargetMultiplicity: "1"},
		{ID: "r4", Type: "directed_association", Source: "el-user", Target: "el-status", Name: "status"},
		{ID: "r5", Type: "dependency", Source: "el-user", Target: "el-issuer"},
		{ID: "r6", Type: "note_link", Source: "el-note", Target: "el-user"},
	}
	return d
}

func TestExportUMLClasses(t *testing.T) {
	out := ExportUML(sampleClass())
	for _, want := range []string{
		"classDiagram",
		"namespace auth {",
		`class user["User"] {`,
		"+email: string",
		"+login(email: string) Token",
		"<<interface>>",
		"<<enumeration>>",
		"<<abstract>>",
		"user <|-- admin",
		"tokenissuer <|.. jwtissuer",
		`user "1" *-- "*" session`,
		"user --> userstatus : status",
		"user ..> tokenissuer",
		`note for user "Senha só como hash"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("classDiagram sem %q:\n%s", want, out)
		}
	}
	if ExportUML(sampleClass()) != out {
		t.Error("exportação não é determinística")
	}
}

func TestExportUMLSequencia(t *testing.T) {
	d := model.NewUMLDiagram("login", model.UMLKindSequence, "Login")
	d.Elements = []model.UMLElement{
		{ID: "el-web", Type: "lifeline", Name: "Web App", Position: model.Position{X: 240}},
		{ID: "el-user", Type: "lifeline", Name: "Usuário", LifelineKind: "actor", Position: model.Position{X: 40}},
		{ID: "el-db", Type: "lifeline", Name: "Auth DB", LifelineKind: "database", Position: model.Position{X: 440}},
	}
	d.Relations = []model.UMLRelation{
		{ID: "m3", Type: "message", Source: "el-db", Target: "el-web", Name: "linhas", MessageKind: "reply", Order: 3},
		{ID: "m1", Type: "message", Source: "el-user", Target: "el-web", Name: "login", Order: 1},
		{ID: "m2", Type: "message", Source: "el-web", Target: "el-db", Name: "SELECT", MessageKind: "async", Order: 2},
		{ID: "m4", Type: "message", Source: "el-web", Target: "el-web", Name: "sessão", MessageKind: "create", Order: 4},
	}
	out := ExportUML(d)
	for _, want := range []string{
		"sequenceDiagram",
		"actor usuario as Usuário",
		"participant web_app as Web App",
		"participant auth_db as «database» Auth DB",
		"usuario->>web_app: login",
		"web_app-)auth_db: SELECT",
		"auth_db-->>web_app: linhas",
		"web_app->>web_app: «create» sessão",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sequenceDiagram sem %q:\n%s", want, out)
		}
	}
	// Participantes na ordem horizontal e mensagens na ordem de `order`.
	if strings.Index(out, "actor usuario") > strings.Index(out, "participant web_app") {
		t.Error("participantes fora da ordem horizontal")
	}
	if strings.Index(out, ": login") > strings.Index(out, ": SELECT") || strings.Index(out, ": SELECT") > strings.Index(out, ": linhas") {
		t.Error("mensagens fora da ordem")
	}
}

func TestExportUMLEstados(t *testing.T) {
	d := model.NewUMLDiagram("sessao", model.UMLKindState, "Sessão")
	d.Elements = []model.UMLElement{
		{ID: "i", Type: "initial"},
		{ID: "ativa", Type: "state", Name: "Ativa", Entry: "emite JWT"},
		{ID: "ci", Type: "initial", ParentID: "ativa"},
		{ID: "ociosa", Type: "state", Name: "Ociosa", ParentID: "ativa"},
		{ID: "ch", Type: "choice"},
		{ID: "fim", Type: "final"},
	}
	d.Relations = []model.UMLRelation{
		{ID: "t1", Type: "transition", Source: "i", Target: "ativa"},
		{ID: "t2", Type: "transition", Source: "ativa", Target: "ch", Trigger: "tick", Guard: "expirou", Effect: "log"},
		{ID: "t3", Type: "transition", Source: "ch", Target: "fim"},
		{ID: "t4", Type: "transition", Source: "ci", Target: "ociosa"},
	}
	out := ExportUML(d)
	for _, want := range []string{
		"stateDiagram-v2",
		`state "Ativa" as ativa`,
		"state ativa {",
		"        [*] --> ociosa",
		"note left of ativa : entry / emite JWT", // composto: ações viram nota
		"state ch <<choice>>",
		"    [*] --> ativa",
		"ativa --> ch : tick [expirou] / log",
		"ch --> [*]",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stateDiagram sem %q:\n%s", want, out)
		}
	}
}

func TestExportUMLCasosDeUso(t *testing.T) {
	d := model.NewUMLDiagram("casos-de-uso", model.UMLKindUseCase, "Casos de Uso")
	d.Elements = []model.UMLElement{
		{ID: "el-sis", Type: "boundary", Name: "Loja"},
		{ID: "el-cliente", Type: "actor", Name: "Cliente"},
		{ID: "el-comprar", Type: "usecase", Name: "Comprar", ParentID: "el-sis"},
		{ID: "el-pagar", Type: "usecase", Name: "Pagar", ParentID: "el-sis"},
		{ID: "el-cupom", Type: "usecase", Name: "Aplicar cupom", ParentID: "el-sis"},
		{ID: "el-end", Type: "usecase", Name: "End", ParentID: "el-sis"},
	}
	d.Relations = []model.UMLRelation{
		{ID: "r1", Type: "association", Source: "el-cliente", Target: "el-comprar"},
		{ID: "r2", Type: "include", Source: "el-comprar", Target: "el-pagar"},
		{ID: "r3", Type: "extend", Source: "el-cupom", Target: "el-comprar"},
	}
	out := ExportUML(d)
	for _, want := range []string{
		"flowchart LR",
		`cliente["👤 Cliente"]`,
		`subgraph loja["Loja"]`,
		`comprar(["Comprar"])`,
		"cliente --- comprar",
		"comprar -. «include» .-> pagar",
		"aplicar_cupom -. «extend» .-> comprar",
		`end_(["End"])`, // palavra reservada do Mermaid ganha sufixo
	} {
		if !strings.Contains(out, want) {
			t.Errorf("flowchart sem %q:\n%s", want, out)
		}
	}
}

// edgeUML monta, para cada tipo, um diagrama com textos que já quebraram o
// parser do Mermaid 11: nomes vazios, palavras reservadas como id, chaves e
// parênteses em membros, ":" em rótulos e notas vazias.
func edgeUML() []*model.UMLDiagram {
	class := model.NewUMLDiagram("classes", model.UMLKindClass, "Classes")
	class.Elements = []model.UMLElement{
		{ID: "el-o", Type: "class", Name: "O"},
		{ID: "el-note-cls", Type: "class", Name: "Note",
			Stereotype: "entity {x}",
			Attributes: []model.UMLMember{{Name: "preco", Type: "Decimal(10,2)", Visibility: "#"}, {Name: "z)", Type: "q"}, {Name: "mapa", Type: "Map{}", Visibility: "{"}},
			Operations: []model.UMLMember{{Name: "run", Params: "cb: () => void", Type: "Promise{T}"}}},
		{ID: "el-enum", Type: "enum", Name: "Status", Literals: []string{"A { }", "B)"}},
		{ID: "el-callback", Type: "interface", Name: "callback"},
		{ID: "el-n1", Type: "note"},
		{ID: "el-n2", Type: "note", Documentation: "x : y"},
	}
	class.Relations = []model.UMLRelation{
		{ID: "r1", Type: "association", Source: "el-o", Target: "el-note-cls", Name: "tem : muitos", SourceMultiplicity: " ", TargetMultiplicity: "0..*"},
		{ID: "r2", Type: "note_link", Source: "el-n1", Target: "el-o"},
		{ID: "r3", Type: "note_link", Source: "el-n2", Target: "el-callback"},
	}

	seq := model.NewUMLDiagram("sequencia", model.UMLKindSequence, "Sequência")
	seq.Elements = []model.UMLElement{
		{ID: "el-box", Type: "lifeline", Name: "Box", Position: model.Position{X: 0}},
		{ID: "el-sem-nome", Type: "lifeline", Position: model.Position{X: 100}},
		{ID: "el-db", Type: "lifeline", LifelineKind: "database", Position: model.Position{X: 200}},
		{ID: "el-frag", Type: "fragment", Operator: "alt;#", Guard: "x: 1"},
		{ID: "el-nota", Type: "note"},
	}
	seq.Relations = []model.UMLRelation{
		{ID: "m1", Type: "message", Source: "el-box", Target: "el-sem-nome", Name: "a : b ; c", Order: 1},
		{ID: "m2", Type: "message", Source: "el-sem-nome", Target: "el-db", Order: 2},
		{ID: "n1", Type: "note_link", Source: "el-nota", Target: "el-box"},
	}

	state := model.NewUMLDiagram("estados", model.UMLKindState, "Estados")
	state.Elements = []model.UMLElement{
		{ID: "i", Type: "initial"},
		{ID: "el-note", Type: "state", Name: "", Entry: "x : y"},
		{ID: "el-scale", Type: "state", Name: "Scale", Do: "a : b"},
		{ID: "el-inner", Type: "state", Name: "Dentro", ParentID: "el-scale"},
		{ID: "el-ch", Type: "choice", Name: "state"},
		{ID: "el-nota", Type: "note", Documentation: "hora: 10:00"},
		{ID: "el-nota-vazia", Type: "note"},
	}
	state.Relations = []model.UMLRelation{
		{ID: "t1", Type: "transition", Source: "i", Target: "el-note", Trigger: "go : now"},
		{ID: "t2", Type: "transition", Source: "el-note", Target: "el-ch"},
		{ID: "t3", Type: "transition", Source: "el-ch", Target: "el-scale", Guard: "ok"},
		{ID: "n1", Type: "note_link", Source: "el-nota", Target: "el-note"},
		{ID: "n2", Type: "note_link", Source: "el-nota-vazia", Target: "el-scale"},
	}

	uc := model.NewUMLDiagram("casos", model.UMLKindUseCase, "Casos")
	uc.Elements = []model.UMLElement{
		{ID: "el-sis", Type: "boundary"},
		{ID: "el-ator", Type: "actor", Name: "Click"},
		{ID: "el-uc", Type: "usecase", ParentID: "el-sis"},
		{ID: "el-uc2", Type: "usecase", Name: "Pagar (cartão)", ParentID: "el-sis"},
		{ID: "el-o", Type: "usecase", Name: "o", ParentID: "el-sis"},
	}
	uc.Relations = []model.UMLRelation{
		{ID: "r1", Type: "association", Source: "el-ator", Target: "el-uc", Name: "usa (sempre) | às vezes"},
		{ID: "r2", Type: "dependency", Source: "el-uc", Target: "el-uc2", Name: "depende [x] {y}"},
		{ID: "r3", Type: "association", Source: "el-ator", Target: "el-o"},
	}

	return []*model.UMLDiagram{class, seq, state, uc, model.NewUMLDiagram("vazio", model.UMLKindClass, "Vazio")}
}

func TestExportUMLTextosProblematicos(t *testing.T) {
	var out strings.Builder
	for _, d := range edgeUML() {
		out.WriteString(ExportUML(d))
	}
	s := out.String()
	for _, bad := range []string{
		`[""]`, `([""])`, `state "" as`, `note ""`, "note for o_ \"\"", "Note over box_: \n",
		"class o[", "class note[", "participant box as", "state scale {", "---|usa", "-. depende",
		": : ", "hora: 10", "Map{}", "entity {x}", "z)",
	} {
		if strings.Contains(s, bad) {
			t.Errorf("saída contém %q, que o Mermaid 11 recusa:\n%s", bad, s)
		}
	}
	for _, want := range []string{
		// classes
		`class o_["O"]`, `class note_["Note"] {`, "<<entity #123;x#125;>>",
		"#preco: Decimal#40;10,2#41;", "z#41;: q", "mapa: Map#123;#125;", "run(cb: #40;#41; =#62; void) Promise#123;T#125;",
		"A #123; #125;", "B#41;", "callback_", `o_ -- "0..*" note_ : tem #58; muitos`,
		`note for callback_ "x : y"`, "classDiagram\n    direction TB\n",
		// sequência
		"participant box_ as Box", "participant sem_nome as sem_nome", "participant db as «database» db",
		"box_->>sem_nome: a : b , c", "Note over box_,db: alt, [x: 1]",
		// estados
		`state "note_" as note_`, "note_ : entry / x #58; y", `state "Scale" as scale_`, "state scale_ {",
		"note left of scale_ : do / a #58; b", "state state_ <<choice>>", "note right of note_ : hora#58; 10#58;00",
		"[*] --> note_ : go #58; now",
		// casos de uso
		`subgraph sis["sis"]`, `uc(["uc"])`, `click_["👤 Click"]`, `click_ ---|"usa (sempre) | às vezes"| uc`,
		`uc -. "depende [x] {y}" .-> pagar_cartao`, `o_(["o"])`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("saída sem %q:\n%s", want, s)
		}
	}
}
