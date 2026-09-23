package httpapi

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

type reqDocResponse struct {
	Title    string            `json:"title"`
	Project  string            `json:"project"`
	Version  string            `json:"version"`
	Date     string            `json:"date"`
	Authors  []string          `json:"authors"`
	History  []map[string]any  `json:"history"`
	TOC      []map[string]any  `json:"toc"`
	Blocks   []json.RawMessage `json:"blocks"`
	Markdown string            `json:"markdown"`
}

func TestDocumentoDeRequisitosViaHTTP(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "GET", "/api/reqdoc", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("reqdoc: status %d — %s", res.StatusCode, body)
	}
	var doc reqDocResponse
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Title != "Documento de Requisitos" || doc.Project != "HTTP Teste" || len(doc.History) == 0 ||
		len(doc.TOC) == 0 || len(doc.Blocks) == 0 || doc.Authors == nil {
		t.Fatalf("documento incompleto: %+v", doc)
	}
	raw := string(body)
	for _, want := range []string{
		`{"type":"pagebreak"}`, `"type":"priority"`,
		`"src":"/api/export/uml/casos-de-uso.svg"`, `"diagram":"macro"`,
		`[CDU001] AUTENTICAR USUÁRIO`,
	} {
		if !strings.Contains(raw, want) {
			t.Errorf("documento sem %s", want)
		}
	}
	if !strings.Contains(doc.Markdown, "| Prioridade: |") || !strings.Contains(doc.Markdown, "# 1 Introdução {#introducao}") {
		t.Errorf("markdown incompleto:\n%s", doc.Markdown)
	}

	// Download com as figuras embutidas.
	res, body = do(t, ts, "GET", "/api/reqdoc/markdown?embed=1", nil)
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/markdown") {
		t.Fatalf("markdown: status %d, %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if !strings.Contains(string(body), "](data:image/svg+xml;base64,") || strings.Contains(string(body), "](/api/export/") {
		t.Error("com embed=1 as imagens deveriam ser data URIs")
	}
}

func TestMetadadosDoDocumentoViaHTTP(t *testing.T) {
	ts, st := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "GET", "/api/reqdoc/meta", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("meta: status %d — %s", res.StatusCode, body)
	}
	var meta model.DocumentMeta
	_ = json.Unmarshal(body, &meta)
	// O init grava um exemplo com cliente e uma entrada de histórico.
	if meta.Title == "" || meta.Version != "0.1.0" || meta.Client == "" || len(meta.History) != 1 {
		t.Errorf("meta do exemplo: %+v", meta)
	}

	meta.Client = "Clínicas de estética."
	meta.References = []string{"IEEE Std 830-1998."}
	meta.Glossary = []model.GlossaryTerm{{Term: "CDU", Definition: "Caso de uso"}}
	res, body = do(t, ts, "PUT", "/api/reqdoc/meta", meta)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("put meta: status %d — %s", res.StatusCode, body)
	}
	var saved model.DocumentMeta
	_ = json.Unmarshal(body, &saved)
	if saved.Client != "Clínicas de estética." || len(saved.References) != 1 {
		t.Errorf("meta salva: %+v", saved)
	}
	if data, err := st.ReadFile(".arch/document.yaml"); err != nil || !strings.Contains(string(data), "Clínicas de estética.") {
		t.Errorf("document.yaml não gravado: %v\n%s", err, data)
	}

	// Referências passam a gerar a seção final do documento.
	_, body = do(t, ts, "GET", "/api/reqdoc", nil)
	if !strings.Contains(string(body), `"text":"Referências"`) {
		t.Error("seção de referências ausente após PUT")
	}

	// O snapshot expõe os metadados, nunca nulos.
	_, body = do(t, ts, "GET", "/api/snapshot", nil)
	if !strings.Contains(string(body), `"document":{"title":"Documento de Requisitos"`) {
		t.Errorf("snapshot sem document")
	}
}

func TestSalvarDocumentoDeRequisitosViaHTTP(t *testing.T) {
	ts, st := newTestServer(t)
	defer ts.Close()

	res, body := do(t, ts, "POST", "/api/reqdoc/save", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("save: status %d — %s", res.StatusCode, body)
	}
	var out struct {
		File   string   `json:"file"`
		Images []string `json:"images"`
	}
	_ = json.Unmarshal(body, &out)
	if out.File != "docs/documento-de-requisitos.md" || len(out.Images) != 5 {
		t.Fatalf("resposta: %s", body)
	}
	for _, img := range out.Images {
		data, err := os.ReadFile(filepath.Join(st.Root(), img))
		if err != nil || !strings.HasPrefix(string(data), "<svg") {
			t.Errorf("figura %s: %v", img, err)
		}
	}
	md, err := st.ReadFile(out.File)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"](diagramas/casos-de-uso.svg)", "](diagramas/arquitetura.svg)"} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown salvo sem %q", want)
		}
	}
	if strings.Contains(string(md), "](/api/") {
		t.Error("markdown salvo não deveria apontar para a API")
	}
}

func TestExportUMLSVGViaHTTP(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	for _, id := range []string{"casos-de-uso", "modelo-de-dominio", "cdu001-autenticar-usuario", "ciclo-de-vida-da-sessao"} {
		res, body := do(t, ts, "GET", "/api/export/uml/"+id+".svg", nil)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d — %s", id, res.StatusCode, body)
		}
		if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
			t.Errorf("%s: content-type %q", id, ct)
		}
		if !strings.HasPrefix(string(body), "<svg") {
			t.Errorf("%s: corpo não é SVG", id)
		}
	}
	_, body := do(t, ts, "GET", "/api/export/uml/modelo-de-dominio.svg?theme=dark", nil)
	if !strings.Contains(string(body), "#1e1f22") {
		t.Error("tema escuro ignorado")
	}
	if res, _ := do(t, ts, "GET", "/api/export/uml/nao-existe.svg", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("diagrama inexistente: status %d", res.StatusCode)
	}
}
