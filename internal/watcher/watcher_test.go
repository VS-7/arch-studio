package watcher

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/archcode/studio/internal/store"
)

func TestAvisaEdicaoExternaMasIgnoraEco(t *testing.T) {
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	changed := make(chan string, 8)
	w, err := New(st, func(rel string) { changed <- rel })
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()

	// Gravação do próprio Studio: é eco, não pode ser anunciada.
	if err := st.WriteFile(store.FileRequisit, []byte("# interno\n")); err != nil {
		t.Fatal(err)
	}
	// Edição externa (editor, git): deve ser anunciada com caminho relativo.
	abs, _ := st.Path("docs/externo.md")
	if err := os.WriteFile(abs, []byte("# externo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	select {
	case rel := <-changed:
		if rel != "docs/externo.md" {
			t.Fatalf("mudança anunciada = %q, quer docs/externo.md", rel)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("edição externa não foi anunciada")
	}
	select {
	case rel := <-changed:
		t.Fatalf("anúncio inesperado: %q (eco de gravação interna?)", filepath.ToSlash(rel))
	case <-time.After(debounce + 200*time.Millisecond):
	}
}
