package store

import (
	"errors"
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func TestUMLRoundTrip(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	d := model.NewUMLDiagram("modelo", model.UMLKindClass, "Modelo")
	d.Elements = []model.UMLElement{{ID: "el-user", Type: "class", Name: "User",
		Attributes: []model.UMLMember{{Name: "email", Type: "string", Visibility: "+"}}}}
	d.File = "lixo-que-nao-deve-ser-gravado"
	if err := st.SaveUMLDiagram(d); err != nil {
		t.Fatal(err)
	}
	if d.File != ".arch/diagrams/class/modelo.json" {
		t.Errorf("file no retorno: %q", d.File)
	}
	raw, _ := st.ReadFile(d.File)
	if strings.Contains(string(raw), `"file"`) {
		t.Errorf("campo file não pode ser gravado:\n%s", raw)
	}

	got, err := st.LoadUMLDiagram("modelo")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != model.UMLKindClass || got.File != d.File || got.Elements[0].Attributes[0].Name != "email" {
		t.Errorf("leitura divergente: %+v", got)
	}

	// Ids são únicos entre TODOS os tipos de diagrama.
	if id := st.UniqueUMLID("Modelo"); id != "modelo-2" {
		t.Errorf("UniqueUMLID: %q", id)
	}

	// Ordenação por tipo (usecase antes de class) e depois por nome; .mermaid
	// avulso em sequence/ é ignorado.
	uc := model.NewUMLDiagram("z", model.UMLKindUseCase, "Zeta")
	if err := st.SaveUMLDiagram(uc); err != nil {
		t.Fatal(err)
	}
	_ = st.WriteFile(DirSequence+"/legado.mermaid", []byte("sequenceDiagram\n"))
	list, err := st.ListUMLDiagrams()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != "z" || list[1].ID != "modelo" {
		t.Errorf("lista inesperada: %+v", list)
	}

	if err := st.DeleteUMLDiagram(got); err != nil {
		t.Fatal(err)
	}
	if _, err := st.LoadUMLDiagram("modelo"); !errors.Is(err, model.ErrUMLNotFound) {
		t.Errorf("esperado ErrUMLNotFound, got %v", err)
	}
	if _, err := st.LoadUMLDiagram("../manifest"); !errors.Is(err, model.ErrUMLNotFound) {
		t.Errorf("id com traversal deveria ser recusado, got %v", err)
	}
}
