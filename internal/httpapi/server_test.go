package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
)

func newTestServer(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := project.Init(st, project.Options{ProjectName: "HTTP Teste"}); err != nil {
		t.Fatal(err)
	}
	hb := hub.New()
	srv := New(app.New(st, hb), hb, Options{Version: "test", Assets: testAssets})
	return httptest.NewServer(srv.Handler()), st
}

func do(t *testing.T, ts *httptest.Server, method, path string, body any) (*http.Response, []byte) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, ts.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(res.Body)
	return res, buf.Bytes()
}

func TestHealthESnapshot(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "GET", "/api/health", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("health: status %d", res.StatusCode)
	}
	var health map[string]any
	if err := json.Unmarshal(body, &health); err != nil {
		t.Fatal(err)
	}
	if health["is_project"] != true {
		t.Errorf("is_project: %v", health["is_project"])
	}

	res, body = do(t, ts, "GET", "/api/snapshot", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("snapshot: status %d — %s", res.StatusCode, body)
	}
	var snap store.Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		t.Fatal(err)
	}
	if snap.Manifest.ProjectName != "HTTP Teste" {
		t.Errorf("nome do projeto: %q", snap.Manifest.ProjectName)
	}
	if len(snap.Diagram.Nodes) == 0 {
		t.Error("o projeto inicial deveria trazer a arquitetura de exemplo")
	}
}

func TestCicloDeVidaDoNoViaHTTP(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "POST", "/api/diagram/nodes", map[string]any{
		"label": "Payments Service", "type": "compute", "technology": "Go 1.23",
		"connect_to": "Core API", "protocol": "REST",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("criação: status %d — %s", res.StatusCode, body)
	}
	var node struct {
		ID   string `json:"id"`
		Data struct {
			Label string `json:"label"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &node); err != nil {
		t.Fatal(err)
	}

	res, body = do(t, ts, "PATCH", "/api/diagram/nodes/"+node.ID, map[string]any{"technology": "Go 1.24"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("atualização: status %d — %s", res.StatusCode, body)
	}

	res, _ = do(t, ts, "DELETE", "/api/diagram/nodes/"+node.ID, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("remoção: status %d", res.StatusCode)
	}

	res, body = do(t, ts, "DELETE", "/api/diagram/nodes/inexistente", nil)
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("remover inexistente: status %d — %s", res.StatusCode, body)
	}
}

func TestErrosRetornamMensagemUtil(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "POST", "/api/diagram/nodes", map[string]any{"type": "compute"})
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: %d", res.StatusCode)
	}
	var apiErr struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &apiErr)
	if !strings.Contains(apiErr.Error, "rótulo") {
		t.Errorf("mensagem pouco informativa: %q", apiErr.Error)
	}
}

func TestExportSVGRespeitaModo(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "GET", "/api/export/svg?mode=engineering&theme=dark", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
		t.Errorf("content-type: %q", ct)
	}
	engineering := string(body)
	if !strings.Contains(engineering, "<svg") || !strings.Contains(engineering, "PostgreSQL") {
		t.Error("SVG de engenharia incompleto")
	}

	// No modo executivo, os nós marcados como internos somem.
	_, body = do(t, ts, "GET", "/api/export/svg?mode=executive", nil)
	if strings.Contains(string(body), "PostgreSQL") {
		t.Error("modo executivo não deveria expor o banco marcado como interno")
	}
}

func TestPRDETarefasViaHTTP(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "POST", "/api/ai-prd", map[string]any{
		"target_stack": "Go / React", "include_test_scenarios": true,
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("geração do PRD: status %d — %s", res.StatusCode, body)
	}
	var prdRes struct {
		TotalTasks int    `json:"total_tasks"`
		Hash       string `json:"hash"`
	}
	_ = json.Unmarshal(body, &prdRes)
	if prdRes.TotalTasks == 0 || prdRes.Hash == "" {
		t.Fatalf("resultado inesperado: %s", body)
	}

	_, body = do(t, ts, "GET", "/api/tasks?status=pending", nil)
	var list struct {
		Tasks []struct {
			ID    string `json:"id"`
			Ready bool   `json:"ready"`
		} `json:"tasks"`
	}
	_ = json.Unmarshal(body, &list)
	if len(list.Tasks) == 0 {
		t.Fatal("nenhuma tarefa pendente listada")
	}

	first := list.Tasks[0].ID
	res, body = do(t, ts, "POST", "/api/tasks/"+first+"/status", map[string]any{
		"status": "completed", "notes": "feito",
	})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("marcação de status: %d — %s", res.StatusCode, body)
	}
}

func TestLeituraDeArquivoRespeitaSandbox(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, _ := do(t, ts, "GET", "/api/file?path=docs/requisitos.md", nil)
	if res.StatusCode != http.StatusOK {
		t.Errorf("leitura permitida falhou: %d", res.StatusCode)
	}

	for _, path := range []string{"../../etc/passwd", "/etc/passwd", "go.mod"} {
		res, _ = do(t, ts, "GET", "/api/file?path="+path, nil)
		if res.StatusCode != http.StatusForbidden {
			t.Errorf("caminho %q deveria ser bloqueado, got %d", path, res.StatusCode)
		}
	}
}

// testAssets imita o frontend compilado (internal/webui/dist).
var testAssets = fstest.MapFS{
	"index.html":    {Data: []byte("<!doctype html><html><head><title>t</title></head><body></body></html>")},
	"assets/app.js": {Data: []byte("console.log(1)")},
}

func TestSPAFallback(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	for _, path := range []string{"/", "/rota/inexistente/do/spa"} {
		res, body := do(t, ts, "GET", path, nil)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, res.StatusCode)
		}
		if !strings.Contains(string(body), `<meta name="archcode-host" content="web">`) {
			t.Errorf("%s deveria devolver o index.html com o host injetado: %s", path, body)
		}
	}
	res, body := do(t, ts, "GET", "/assets/app.js", nil)
	if res.StatusCode != http.StatusOK || string(body) != "console.log(1)" {
		t.Errorf("asset estático: status %d — %s", res.StatusCode, body)
	}
}

func TestRequisicoesDeOutroSiteSaoRecusadas(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	send := func(origin string) int {
		req, _ := http.NewRequest("PUT", ts.URL+"/api/file", strings.NewReader(`{"path":"docs/x.md","content":"x"}`))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
		return res.StatusCode
	}
	if code := send("https://site-malicioso.example"); code != http.StatusForbidden {
		t.Errorf("origem externa: status %d, quer 403", code)
	}
	for _, origin := range []string{"", ts.URL, "http://localhost:5173", "wails://wails", "http://wails.localhost"} {
		if code := send(origin); code != http.StatusOK {
			t.Errorf("origem %q: status %d, quer 200", origin, code)
		}
	}
}
