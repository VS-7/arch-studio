package store

import (
	"path/filepath"
	"testing"
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
