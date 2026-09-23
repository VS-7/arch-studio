package store

import (
	"path/filepath"
	"testing"

	"github.com/archcode/studio/internal/model"
)

// Remoções feitas pelo próprio servidor não podem ser anunciadas pelo watcher
// como edições externas ("arquivo alterado fora do Studio").
func TestRemoveIsEcho(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.WriteFile("docs/a.md", []byte("x")); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(st.Root(), "docs", "a.md")
	if !st.IsEcho(abs) {
		t.Fatal("gravação do servidor deveria ser eco")
	}
	if err := st.Remove("docs/a.md"); err != nil {
		t.Fatal(err)
	}
	if !st.IsEcho(abs) {
		t.Fatal("remoção do servidor deveria ser eco")
	}
	// Se o arquivo reaparecer (recriado por outro processo), deixa de ser eco.
	if err := st.WriteFile("docs/b.md", []byte("y")); err != nil {
		t.Fatal(err)
	}
	st.rememberRemoval(filepath.Join(st.Root(), "docs", "b.md"))
	if st.IsEcho(filepath.Join(st.Root(), "docs", "b.md")) {
		t.Fatal("arquivo existente não pode ser tratado como eco de remoção")
	}
}

// Sem .arch/document.yaml, os metadados vêm dos padrões do manifest; depois de
// gravados, voltam idênticos.
func TestDocumentMetaPadroesEGravacao(t *testing.T) {
	st, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := model.DefaultManifest("Clínica")
	m.Authors = []model.Author{{Name: "Ana"}}
	if err := st.SaveManifest(m); err != nil {
		t.Fatal(err)
	}
	meta, err := st.LoadDocumentMeta()
	if err != nil {
		t.Fatal(err)
	}
	if meta.Title != model.DefaultDocumentTitle || meta.Version != "0.1.0" || len(meta.Authors) != 1 || meta.History == nil {
		t.Errorf("padrões: %+v", meta)
	}

	meta.Client = "Clínicas de estética."
	meta.History = []model.DocRevision{{Date: "2026-09-01", Version: "0.1", Description: "Draft inicial do documento.", Author: "Ana"}}
	meta.Glossary = []model.GlossaryTerm{{Term: "CDU", Definition: "Caso de uso"}}
	if err := st.SaveDocumentMeta(meta); err != nil {
		t.Fatal(err)
	}
	again, err := st.LoadDocumentMeta()
	if err != nil {
		t.Fatal(err)
	}
	if again.Client != meta.Client || len(again.History) != 1 || again.History[0].Author != "Ana" || again.Glossary[0].Term != "CDU" {
		t.Errorf("releitura: %+v", again)
	}
}
