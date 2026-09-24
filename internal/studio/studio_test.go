package studio

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
)

func TestAbreProjetoEServeAPI(t *testing.T) {
	dir := t.TempDir()
	if _, err := Open(Config{Dir: dir, RequireProject: true}, nil); !errors.Is(err, store.ErrNotAProject) {
		t.Fatalf("diretório vazio deveria ser recusado: %v", err)
	}
	if _, err := Init(dir, project.Options{ProjectName: "Studio Teste"}); err != nil {
		t.Fatal(err)
	}

	bus := hub.New()
	s, err := Open(Config{Dir: dir, Version: "t", RequireProject: true, Watch: true}, bus)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()

	ts := httptest.NewServer(s.Handler(HandlerOptions{Host: "desktop", MCPBaseURL: "http://127.0.0.1:9999/"}))
	defer ts.Close()

	res, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	var health map[string]any
	_ = json.NewDecoder(res.Body).Decode(&health)
	_ = res.Body.Close()
	if health["is_project"] != true || health["host"] != "desktop" || health["mcp_sse_url"] != "http://127.0.0.1:9999/mcp/sse" {
		t.Fatalf("health inesperado: %v", health)
	}

	// O watcher liga as edições externas ao barramento.
	events, cancel := bus.Subscribe(8)
	defer cancel()
	if err := os.WriteFile(filepath.Join(dir, "docs", "externo.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case ev := <-events:
		if ev.Type != hub.EventDocs || ev.Source != hub.SourceDisk || ev.Path != "docs/externo.md" {
			t.Fatalf("evento inesperado: %+v", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("edição externa não chegou ao barramento")
	}
}
