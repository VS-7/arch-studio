package workspace

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/archcode/studio/internal/hub"
)

var assets = fstest.MapFS{
	"index.html": {Data: []byte("<!doctype html><html><head><title>t</title></head><body></body></html>")},
}

func get(t *testing.T, h http.Handler, path string) (int, string) {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

func TestSemProjetoServeTelaInicial(t *testing.T) {
	w := New("t", hub.New(), assets, NewRecentList(filepath.Join(t.TempDir(), "recent.json")))

	code, body := get(t, w, "/api/health")
	var health map[string]any
	_ = json.Unmarshal([]byte(body), &health)
	if code != http.StatusOK || health["is_project"] != false || health["host"] != "desktop" {
		t.Fatalf("health sem projeto: %d %s", code, body)
	}
	if code, _ := get(t, w, "/api/snapshot"); code != http.StatusConflict {
		t.Fatalf("snapshot sem projeto: status %d, quer 409", code)
	}
	if _, body := get(t, w, "/"); !strings.Contains(body, `<meta name="archcode-host" content="desktop">`) {
		t.Fatalf("index sem o host desktop: %s", body)
	}
	if w.Current() != nil {
		t.Fatal("não deveria haver projeto aberto")
	}
}

func TestCriarAbrirEFecharProjeto(t *testing.T) {
	recent := NewRecentList(filepath.Join(t.TempDir(), "recent.json"))
	w := New("t", hub.New(), assets, recent)
	dir := t.TempDir()

	if _, err := w.Open(dir); err == nil {
		t.Fatal("pasta vazia não é projeto e deveria ser recusada")
	}
	info, err := w.Create(dir, "Loja Virtual")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Loja Virtual" || w.Current() == nil {
		t.Fatalf("projeto aberto inesperado: %+v", info)
	}
	if code, body := get(t, w, "/api/snapshot"); code != http.StatusOK || !strings.Contains(body, "Loja Virtual") {
		t.Fatalf("snapshot do projeto: %d", code)
	}

	// Recentes persistem em disco e sobrevivem a um novo carregamento.
	reloaded := NewRecentList(recent.path)
	if list := reloaded.List(); len(list) != 1 || list[0].Root != info.Root {
		t.Fatalf("recentes = %+v", list)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, w, "/api/snapshot"); code != http.StatusConflict || w.Current() != nil {
		t.Fatalf("depois de fechar: status %d", code)
	}

	// Reabrir o mesmo projeto não duplica a entrada dos recentes.
	if _, err := w.Open(dir); err != nil {
		t.Fatal(err)
	}
	if list := recent.List(); len(list) != 1 {
		t.Fatalf("recentes duplicados: %+v", list)
	}
	recent.Remove(info.Root)
	if list := recent.List(); len(list) != 0 {
		t.Fatalf("remover dos recentes falhou: %+v", list)
	}
	_ = w.Close()
}
