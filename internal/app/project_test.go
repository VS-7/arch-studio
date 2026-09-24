package app

import (
	"errors"
	"testing"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

func TestEventoPorArquivo(t *testing.T) {
	cases := map[string]string{
		".arch/diagrams/class/modelo.json":         hub.EventUML,
		".arch/diagrams/usecase/casos-de-uso.json": hub.EventUML,
		".arch/diagrams/sequence/login.json":       hub.EventUML,
		".arch/diagrams/state/sessao.json":         hub.EventUML,
		".arch/diagrams/sequence/legado.mermaid":   hub.EventDiagram,
		".arch/diagrams/macro.json":                hub.EventDiagram,
		".arch/pricing.yaml":                       hub.EventPricing,
		"api/endpoints.yaml":                       hub.EventEndpoints,
		"docs/requisitos.md":                       hub.EventDocs,
		".arch/document.yaml":                      hub.EventDocs,
		"go.mod":                                   "",
	}
	for rel, want := range cases {
		if got := eventForPath(rel); got != want {
			t.Errorf("eventForPath(%q) = %q, quer %q", rel, got, want)
		}
	}
}

func TestArquivoBrutoNaoEscapaDasPastasDoProjeto(t *testing.T) {
	a, _ := newTestApp(t)
	for _, p := range []string{"docs/../go.mod", "docs/../../etc/passwd", "go.mod", "/etc/passwd", `docs\..\Makefile`} {
		if err := a.WriteProjectFile(p, "x", hub.SourceUI); !errors.Is(err, ErrPathNotAllowed) {
			t.Errorf("WriteProjectFile(%q) = %v, quer ErrPathNotAllowed", p, err)
		}
		if _, err := a.ReadProjectFile(p); !errors.Is(err, ErrPathNotAllowed) {
			t.Errorf("ReadProjectFile(%q) = %v, quer ErrPathNotAllowed", p, err)
		}
	}
	if err := a.WriteProjectFile("docs/./notas.md", "# ok", hub.SourceUI); err != nil {
		t.Fatalf("caminho válido recusado: %v", err)
	}
	if got, err := a.ReadProjectFile("docs/notas.md"); err != nil || got != "# ok" {
		t.Fatalf("leitura = %q, %v", got, err)
	}
}

func TestGravarArquivoAnunciaOEventoDoArquivo(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	bus := hub.New()
	events, cancel := bus.Subscribe(4)
	defer cancel()
	a := New(st, bus)

	if err := a.WriteProjectFile(store.FilePricing, "currency: BRL\n", hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	if ev := <-events; ev.Type != hub.EventPricing || ev.Path != store.FilePricing {
		t.Fatalf("evento = %+v, quer pricing_changed em %s", ev, store.FilePricing)
	}
}

// Remover uma decisão não pode fazer a próxima reaproveitar o número de outra
// ainda gravada (antes: ADR-%03d de len+1 sobrescrevia o arquivo existente).
func TestNovoADRNaoSobrescreveOutroDepoisDeRemocao(t *testing.T) {
	a, _ := newTestApp(t)
	existing, _ := a.ADRs()
	first, _, err := a.UpsertADR(model.ADR{Title: "Primeira"}, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := a.UpsertADR(model.ADR{Title: "Segunda"}, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteADR(first.ID, hub.SourceUI); err != nil {
		t.Fatal(err)
	}
	third, _, err := a.UpsertADR(model.ADR{Title: "Terceira"}, hub.SourceUI)
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == second.ID {
		t.Fatalf("nova decisão reaproveitou %s", second.ID)
	}
	list, _ := a.ADRs()
	if len(list) != len(existing)+2 {
		t.Fatalf("decisões gravadas = %d, quer %d (alguma foi sobrescrita)", len(list), len(existing)+2)
	}
	if third.Date == "" {
		t.Error("data da decisão deveria ser preenchida")
	}
}
