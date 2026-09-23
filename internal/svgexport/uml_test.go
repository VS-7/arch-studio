package svgexport

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

// wellFormed percorre o documento inteiro com o decodificador XML.
func wellFormed(t *testing.T, svg []byte) {
	t.Helper()
	dec := xml.NewDecoder(bytes.NewReader(svg))
	root := ""
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("SVG inválido: %v\n%s", err, svg)
		}
		if se, ok := tok.(xml.StartElement); ok && root == "" {
			root = se.Name.Local
		}
	}
	if root != "svg" {
		t.Fatalf("raiz do documento: %q", root)
	}
}

func member(s string) model.UMLMember {
	m, _ := model.ParseUMLMember(s)
	return m
}

func pos(x, y float64) model.Position { return model.Position{X: x, Y: y} }

func TestRenderUMLCasosDeUso(t *testing.T) {
	d := model.NewUMLDiagram("casos", model.UMLKindUseCase, "Casos & Usos")
	d.Elements = []model.UMLElement{
		{ID: "el-sistema", Type: "boundary", Name: "Estetify", Position: pos(240, 40)},
		{ID: "el-paciente", Type: "actor", Name: "Paciente", Position: pos(40, 80)},
		{ID: "el-login", Type: "usecase", Name: "Realizar login", UseCase: "CDU002", ParentID: "el-sistema", Position: pos(300, 100)},
		{ID: "el-senha", Type: "usecase", Name: "Validar <senha>", ParentID: "el-sistema", Position: pos(300, 260)},
		{ID: "el-nota", Type: "note", Name: "Obs", Documentation: "Nota livre", Position: pos(700, 40)},
	}
	d.Relations = []model.UMLRelation{
		{ID: "rel-1", Type: "association", Source: "el-paciente", Target: "el-login"},
		{ID: "rel-2", Type: "include", Source: "el-login", Target: "el-senha"},
		{ID: "rel-3", Type: "note_link", Source: "el-nota", Target: "el-login"},
	}
	svg := RenderUML(d, UMLOptions{})
	wellFormed(t, svg)
	s := string(svg)
	for _, want := range []string{"Paciente", "Realizar login", "CDU002", "Validar &lt;senha&gt;", "Estetify", "«include»", "<ellipse", "Casos &amp; Usos"} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG sem %q", want)
		}
	}
	if !strings.Contains(s, `stroke-dasharray="6 4" marker-end="url(#m-arrow)"`) {
		t.Error("include deveria ser tracejado com seta aberta")
	}
	// Tema claro por padrão: fundo branco, traço preto.
	if !strings.Contains(s, `fill="#ffffff"`) || !strings.Contains(s, `stroke="#000000"`) {
		t.Error("tema claro esperado")
	}
	if dark := string(RenderUML(d, UMLOptions{Dark: true})); strings.Contains(dark, `stroke="#000000"`) {
		t.Error("tema escuro não deveria usar traço preto")
	}
}

func TestRenderUMLClassesComMarcadores(t *testing.T) {
	d := model.NewUMLDiagram("dominio", model.UMLKindClass, "Domínio")
	d.Elements = []model.UMLElement{
		{ID: "el-pessoa", Type: "class", Name: "Pessoa", Abstract: true, Position: pos(40, 40),
			Attributes: []model.UMLMember{member("+ nome: string"), member("- {static} total: int")},
			Operations: []model.UMLMember{member("+ {abstract} falar(): void")}},
		{ID: "el-paciente", Type: "class", Name: "Paciente", Position: pos(40, 300)},
		{ID: "el-consulta", Type: "class", Name: "Consulta", Position: pos(400, 300)},
		{ID: "el-agenda", Type: "class", Name: "Agenda", Position: pos(400, 40)},
		{ID: "el-repo", Type: "interface", Name: "Repositorio", Position: pos(700, 40),
			Operations: []model.UMLMember{member("+ salvar(p: Paciente)")}},
		{ID: "el-status", Type: "enum", Name: "Status", Literals: []string{"ATIVO", "INATIVO"}, Position: pos(700, 300)},
		{ID: "el-pkg", Type: "package", Name: "clinica", Position: pos(1000, 40)},
	}
	d.Relations = []model.UMLRelation{
		{ID: "rel-1", Type: "generalization", Source: "el-paciente", Target: "el-pessoa"},
		{ID: "rel-2", Type: "composition", Source: "el-consulta", Target: "el-agenda", SourceMultiplicity: "0..*", TargetMultiplicity: "1"},
		{ID: "rel-3", Type: "aggregation", Source: "el-paciente", Target: "el-consulta"},
		{ID: "rel-4", Type: "realization", Source: "el-agenda", Target: "el-repo"},
	}
	svg := RenderUML(d, UMLOptions{})
	wellFormed(t, svg)
	s := string(svg)
	for _, want := range []string{
		"Pessoa", "Paciente", "Consulta", "«interface»", "«enumeration»", "ATIVO", "clinica",
		"+ nome: string", "- total: int", "+ falar(): void", "0..*",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG sem %q", want)
		}
	}
	for _, want := range []string{
		`marker-end="url(#m-triangle)"`,       // generalização / realização
		`marker-end="url(#m-diamond-filled)"`, // composição
		`marker-end="url(#m-diamond)"`,        // agregação
		`<marker id="m-triangle"`, `<marker id="m-diamond-filled"`,
		`font-style="italic"`,         // classe abstrata
		`text-decoration="underline"`, // membro estático
	} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG sem %s", want)
		}
	}
	// A realização é tracejada; a generalização, contínua.
	if !strings.Contains(s, `stroke-dasharray="6 4" marker-end="url(#m-triangle)"`) {
		t.Error("realização deveria ser tracejada")
	}
}

func TestRenderUMLSequenciaNaOrdemDasMensagens(t *testing.T) {
	d := model.NewUMLDiagram("login", model.UMLKindSequence, "Login")
	d.Elements = []model.UMLElement{
		{ID: "el-usuario", Type: "lifeline", Name: "Usuário", LifelineKind: "actor", Position: pos(40, 40)},
		{ID: "el-web", Type: "lifeline", Name: "Web", LifelineKind: "boundary", Position: pos(240, 40)},
		{ID: "el-api", Type: "lifeline", Name: "API", Position: pos(440, 40)},
		{ID: "el-db", Type: "lifeline", Name: "Banco", LifelineKind: "database", Position: pos(640, 40)},
		{ID: "el-alt", Type: "fragment", Name: "Erro", Operator: "alt", Guard: "senha inválida", Position: pos(220, 400), Width: 460, Height: 120},
	}
	// Fora de ordem no arquivo: a ordem vertical vem de `order`.
	d.Relations = []model.UMLRelation{
		{ID: "rel-3", Type: "message", Source: "el-api", Target: "el-db", Name: "consulta", Order: 3},
		{ID: "rel-1", Type: "message", Source: "el-usuario", Target: "el-web", Name: "informa senha", Order: 1},
		{ID: "rel-4", Type: "message", Source: "el-db", Target: "el-api", Name: "hash", MessageKind: "reply", Order: 4},
		{ID: "rel-2", Type: "message", Source: "el-web", Target: "el-api", Name: "POST /login", MessageKind: "async", Order: 2},
		{ID: "rel-5", Type: "message", Source: "el-api", Target: "el-api", Name: "emite JWT", Order: 5},
	}
	svg := RenderUML(d, UMLOptions{})
	wellFormed(t, svg)
	s := string(svg)

	labels := []string{"1: informa senha", "2: POST /login", "3: consulta", "4: hash", "5: emite JWT"}
	last := -1
	for _, l := range labels {
		i := strings.Index(s, l)
		if i < 0 {
			t.Fatalf("mensagem %q ausente", l)
		}
		if i < last {
			t.Errorf("mensagem %q fora de ordem", l)
		}
		last = i
	}

	// y = 40 + 44 + 36 + i*44 para cada mensagem, na ordem.
	re := regexp.MustCompile(`data-type="message" data-order="(\d+)">\n<path d="M[\d.]+ ([\d.]+) `)
	matches := re.FindAllStringSubmatch(s, -1)
	if len(matches) != 5 {
		t.Fatalf("mensagens desenhadas: %d", len(matches))
	}
	for i, m := range matches {
		y, _ := strconv.ParseFloat(m[2], 64)
		if want := 120 + float64(i)*44; y != want || m[1] != strconv.Itoa(i+1) {
			t.Errorf("mensagem %s em y=%v, want ordem %d em y=%v", m[1], y, i+1, want)
		}
	}
	for _, want := range []string{
		`marker-end="url(#m-arrow-filled)"`,                 // síncrona
		`marker-end="url(#m-arrow)" stroke-dasharray="6 4"`, // resposta
		"[senha inválida]", ">alt<", "Banco", `stroke-dasharray="5 4"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG sem %s", want)
		}
	}
}

func TestRenderUMLEstados(t *testing.T) {
	d := model.NewUMLDiagram("sessao", model.UMLKindState, "Sessão")
	d.Elements = []model.UMLElement{
		{ID: "el-inicio", Type: "initial", Position: pos(40, 143)},
		{ID: "el-ativa", Type: "state", Name: "Ativa", Entry: "emite JWT", Do: "valida", Position: pos(240, 120)},
		{ID: "el-escolha", Type: "choice", Position: pos(500, 140)},
		{ID: "el-fork", Type: "fork", Position: pos(600, 40)},
		{ID: "el-h", Type: "history", Position: pos(600, 240)},
		{ID: "el-fim", Type: "final", Position: pos(800, 141)},
	}
	d.Relations = []model.UMLRelation{
		{ID: "rel-1", Type: "transition", Source: "el-inicio", Target: "el-ativa", Trigger: "login"},
		{ID: "rel-2", Type: "transition", Source: "el-ativa", Target: "el-escolha", Trigger: "tempo", Guard: "expirou", Effect: "revoga"},
		{ID: "rel-3", Type: "transition", Source: "el-escolha", Target: "el-fim"},
		{ID: "rel-4", Type: "transition", Source: "el-ativa", Target: "el-ativa", Trigger: "renova"},
	}
	svg := RenderUML(d, UMLOptions{})
	wellFormed(t, svg)
	s := string(svg)
	for _, want := range []string{"Ativa", "entry / emite JWT", "do / valida", "tempo [expirou] / revoga", "renova", `rx="12"`, ">H<"} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG sem %q", want)
		}
	}
	// Auto-transição desenhada como laço (curva), não como reta.
	if !strings.Contains(s, `data-id="rel-4" data-type="transition">`+"\n<path d=\"M") || !strings.Contains(s, " C") {
		t.Error("auto-transição deveria ser um laço")
	}
}

func TestRenderUMLVazioEViewBox(t *testing.T) {
	svg := RenderUML(model.NewUMLDiagram("vazio", model.UMLKindClass, "Vazio"), UMLOptions{})
	wellFormed(t, svg)
	if !strings.Contains(string(svg), "Diagrama vazio") {
		t.Error("diagrama vazio deveria ter um aviso")
	}

	// viewBox = caixa delimitadora + 30px de margem.
	d := model.NewUMLDiagram("um", model.UMLKindUseCase, "Um")
	d.Elements = []model.UMLElement{{ID: "el-a", Type: "usecase", Name: "A", Position: pos(100, 200)}}
	if s := string(RenderUML(d, UMLOptions{})); !strings.Contains(s, `viewBox="70 170 220 130"`) {
		t.Errorf("viewBox inesperado:\n%s", s)
	}
}
