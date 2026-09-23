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
