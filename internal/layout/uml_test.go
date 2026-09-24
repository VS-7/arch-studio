package layout_test

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/svgexport"
)

// Os testes usam a medição real do SVG, como a aplicação.
var metrics = svgexport.Metrics{}

func cloneUML(d *model.UMLDiagram) *model.UMLDiagram {
	data, _ := json.Marshal(d)
	var out model.UMLDiagram
	_ = json.Unmarshal(data, &out)
	return &out
}

type box struct{ x, y, w, h float64 }

func boxOf(e model.UMLElement) box {
	w, h := metrics.Size(e)
	return box{e.Position.X, e.Position.Y, w, h}
}

func (a box) overlaps(b box) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

func (a box) contains(b box) bool {
	return b.x >= a.x && b.y >= a.y && b.x+b.w <= a.x+a.w && b.y+b.h <= a.y+a.h
}

// checkNoOverlap falha se dois elementos se sobrepõem, exceto um contêiner e
// quem ele contém (direta ou indiretamente), e se algum filho sai do pai.
func checkNoOverlap(t *testing.T, d *model.UMLDiagram) {
	t.Helper()
	ancestor := func(anc, id string) bool {
		for e := d.ElementByID(id); e != nil && e.ParentID != ""; e = d.ElementByID(e.ParentID) {
			if e.ParentID == anc {
				return true
			}
		}
		return false
	}
	for i, a := range d.Elements {
		if a.ParentID != "" {
			if p := d.ElementByID(a.ParentID); p != nil && !boxOf(*p).contains(boxOf(a)) {
				t.Errorf("%s sai do contêiner %s: %+v ⊄ %+v", a.ID, p.ID, boxOf(a), boxOf(*p))
			}
		}
		for _, b := range d.Elements[i+1:] {
			if ancestor(a.ID, b.ID) || ancestor(b.ID, a.ID) {
				continue
			}
			if boxOf(a).overlaps(boxOf(b)) {
				t.Errorf("%s %+v sobrepõe %s %+v", a.ID, boxOf(a), b.ID, boxOf(b))
			}
		}
	}
}

// pageScale é a escala com que o SVG do diagrama entra na página do documento.
func pageScale(d *model.UMLDiagram) float64 {
	var w, h float64
	if _, err := fmt.Sscanf(string(svgexport.RenderUML(d, svgexport.UMLOptions{})),
		`<svg xmlns="http://www.w3.org/2000/svg" width="%g" height="%g"`, &w, &h); err != nil {
		return 0
	}
	return math.Min(1, math.Min(layout.PageWidth/w, layout.PageHeight/h))
}

func useCaseDiagram(n int) *model.UMLDiagram {
	actors := [][]string{{"Usuário"}, {"Médico", "Enfermeiro"}, {"Administrador"}, {"Recepcionista", "Administrador"}, {"Paciente"}, {"Sistema"}}
	ucs := []model.UseCase{}
	for i := 1; i <= n; i++ {
		ucs = append(ucs, model.UseCase{
			Code: fmt.Sprintf("CDU%03d", i), Name: fmt.Sprintf("Gerenciar cadastro número %d do sistema", i),
			Actors: actors[(i-1)*len(actors)/n],
		})
	}
	d := model.NewUMLDiagram("casos-de-uso", model.UMLKindUseCase, "Casos de Uso")
	model.SyncUseCaseDiagram(d, "Clínica", ucs)
	return d
}

// O caso que motivou a reorganização: o diagrama gerado das fichas vira uma
// coluna única enorme que, na página, fica ilegível.
func TestUMLCasosDeUsoCabemNaPagina(t *testing.T) {
	d := useCaseDiagram(24)
	before := pageScale(d)
	layout.UML(d, metrics)
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	checkNoOverlap(t, d)
	after := pageScale(d)
	if after < 0.5 || after < before*1.5 {
		t.Errorf("escala na página: antes %.2f, depois %.2f", before, after)
	}
	// Atores ficam fora da fronteira, nas laterais.
	var boundary box
	for _, e := range d.Elements {
		if e.Type == "boundary" {
			boundary = boxOf(e)
		}
	}
	for _, e := range d.Elements {
		if e.Type == "actor" && boxOf(e).overlaps(boundary) {
			t.Errorf("ator %s dentro da fronteira", e.Name)
		}
	}
}

// Diagramas pequenos mantêm o desenho clássico: atores à esquerda, uma coluna.
func TestUMLCasosDeUsoPequenoFicaClassico(t *testing.T) {
	d := useCaseDiagram(4)
	layout.UML(d, metrics)
	checkNoOverlap(t, d)
	var boundary box
	for _, e := range d.Elements {
		if e.Type == "boundary" {
			boundary = boxOf(e)
		}
	}
	xs := map[float64]bool{}
	for _, e := range d.Elements {
		switch e.Type {
		case "actor":
			if b := boxOf(e); b.x+b.w > boundary.x {
				t.Errorf("ator %s deveria estar à esquerda da fronteira", e.Name)
			}
		case "usecase":
			xs[e.Position.X+boxOf(e).w/2] = true
		}
	}
	if len(xs) != 1 {
		t.Errorf("esperava uma única coluna de casos de uso, got %d", len(xs))
	}
	if s := pageScale(d); s < 1 {
		t.Errorf("diagrama pequeno deveria caber em tamanho real, escala %.2f", s)
	}
}

func classDiagram() *model.UMLDiagram {
	d := model.NewUMLDiagram("dominio", model.UMLKindClass, "Domínio")
	add := func(id, typ, name, parent string, attrs ...string) {
		e := model.UMLElement{ID: id, Type: typ, Name: name, ParentID: parent}
		for _, a := range attrs {
			m, _ := model.ParseUMLMember(a)
			e.Attributes = append(e.Attributes, m)
		}
		e.Position = d.AutoPosition(e)
		d.Elements = append(d.Elements, e)
	}
	add("pkg", "package", "cadastro", "")
	add("pessoa", "class", "Pessoa", "", "- nome: string", "- cpf: string")
	add("paciente", "class", "Paciente", "pkg", "- convenio: string")
	add("medico", "class", "Medico", "pkg", "- crm: string")
	add("consulta", "class", "Consulta", "", "- data: DateTime", "- valor: Decimal")
	add("pagamento", "class", "Pagamento", "", "- valor: Decimal")
	add("nota", "note", "Regra", "")
	rel := func(typ, s, t string) {
		d.Relations = append(d.Relations, model.UMLRelation{ID: d.UniqueRelationID(), Type: typ, Source: s, Target: t})
	}
	rel("generalization", "paciente", "pessoa")
	rel("generalization", "medico", "pessoa")
	rel("association", "consulta", "paciente")
	rel("association", "consulta", "medico")
	rel("composition", "pagamento", "consulta")
	rel("note_link", "nota", "consulta")
	d.Normalize()
	return d
}

func TestUMLClassesHierarquiaDeCimaParaBaixo(t *testing.T) {
	d := classDiagram()
	layout.UML(d, metrics)
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	checkNoOverlap(t, d)
	y := func(id string) float64 { return d.ElementByID(id).Position.Y }
	if !(y("pessoa") < y("paciente") && y("pessoa") < y("medico")) {
		t.Errorf("superclasse deveria ficar acima das subclasses: pessoa=%v paciente=%v medico=%v",
			y("pessoa"), y("paciente"), y("medico"))
	}
	if y("consulta") >= y("pagamento") {
		t.Errorf("o todo (composição) deveria ficar acima da parte")
	}
	if y("nota") >= y("consulta") {
		t.Errorf("a nota deveria ficar logo acima do que anota")
	}
	// O pacote foi redimensionado para envolver as duas classes.
	if pkg := d.ElementByID("pkg"); pkg.Width == 0 || pkg.Height == 0 {
		t.Errorf("pacote sem tamanho explícito: %+v", pkg)
	}
}

func TestUMLEstadosComEstadoComposto(t *testing.T) {
	d := model.NewUMLDiagram("ciclo", model.UMLKindState, "Ciclo")
	add := func(id, typ, name, parent string) {
		e := model.UMLElement{ID: id, Type: typ, Name: name, ParentID: parent}
		e.Position = d.AutoPosition(e)
		d.Elements = append(d.Elements, e)
	}
	add("ini", "initial", "", "")
	add("a", "state", "Agendada", "")
	add("b", "state", "Em atendimento", "")
	add("b1", "state", "Triagem", "b")
	add("b2", "state", "Consultório", "b")
	add("fim", "final", "", "")
	tr := func(s, t, trigger string) {
		d.Relations = append(d.Relations, model.UMLRelation{ID: d.UniqueRelationID(), Type: "transition", Source: s, Target: t, Trigger: trigger})
	}
	tr("ini", "a", "")
	tr("a", "b", "paciente chega no horário marcado")
	tr("b1", "b2", "triagem concluída")
	tr("b", "fim", "")
	d.Normalize()

	layout.UML(d, metrics)
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	checkNoOverlap(t, d)
	ini, fim := boxOf(*d.ElementByID("ini")), boxOf(*d.ElementByID("fim"))
	if !(ini.y < fim.y || ini.x < fim.x) {
		t.Errorf("o fluxo deveria ir do estado inicial ao final")
	}
}

func TestUMLSequenciaEspacaPelosRotulos(t *testing.T) {
	d := model.NewUMLDiagram("login", model.UMLKindSequence, "Login")
	for _, id := range []string{"a", "b", "c"} {
		e := model.UMLElement{ID: id, Type: "lifeline", Name: "Participante " + id}
		e.Position = d.AutoPosition(e)
		d.Elements = append(d.Elements, e)
	}
	long := "POST /api/v1/autenticacao/login {email, senha, dispositivo}"
	d.Relations = []model.UMLRelation{
		{ID: "m1", Type: "message", Source: "a", Target: "b", Name: long, Order: 1},
		{ID: "m2", Type: "message", Source: "b", Target: "c", Name: "ok", Order: 2},
	}
	d.Elements = append(d.Elements, model.UMLElement{ID: "f", Type: "fragment", Name: "alt", Operator: "opt",
		Position: model.Position{X: 250, Y: 150}, Width: 120, Height: 40})
	d.Normalize()
	order := []string{"a", "b", "c"}

	layout.UML(d, metrics)
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	center := func(id string) float64 { b := boxOf(*d.ElementByID(id)); return b.x + b.w/2 }
	for i := 1; i < len(order); i++ {
		if center(order[i]) <= center(order[i-1]) {
			t.Fatalf("a ordem dos participantes mudou")
		}
	}
	if gap := center("b") - center("a"); gap < metrics.TextWidth("1: "+long, 11) {
		t.Errorf("rótulo da mensagem 1 não cabe entre as linhas de vida: %.0f", gap)
	}
	// O fragmento cobre a mensagem 2 (y = 40+44+36+44 = 164): vai de b a c.
	f := boxOf(*d.ElementByID("f"))
	if f.x > center("b") || f.x+f.w < center("c") {
		t.Errorf("fragmento %+v não cobre as linhas de vida da mensagem 2 (%v..%v)", f, center("b"), center("c"))
	}
	for _, e := range d.Elements {
		if e.Type == "lifeline" && e.Position.Y != 40 {
			t.Errorf("linha de vida %s fora do topo: %v", e.ID, e.Position.Y)
		}
	}
}

// Reorganizar duas vezes não pode mexer em nada: o usuário clica de novo e o
// diagrama fica onde está.
func TestUMLReorganizarEIdempotente(t *testing.T) {
	for _, d := range []*model.UMLDiagram{useCaseDiagram(24), useCaseDiagram(4), classDiagram()} {
		layout.UML(d, metrics)
		first := cloneUML(d)
		layout.UML(d, metrics)
		if !reflect.DeepEqual(first.Elements, d.Elements) {
			t.Errorf("%s mudou na segunda reorganização", d.ID)
		}
	}
}

func TestUMLDiagramaVazioNaoFalha(t *testing.T) {
	for _, kind := range model.UMLKinds {
		d := model.NewUMLDiagram("vazio", kind, "Vazio")
		layout.UML(d, nil)
		if len(d.Elements) != 0 {
			t.Errorf("%s: elementos surgiram do nada", kind)
		}
	}
}
