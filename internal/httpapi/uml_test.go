package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

func TestDiagramasUMLViaHTTP(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	// O projeto de exemplo traz um diagrama de cada tipo, na ordem canônica.
	res, body := do(t, ts, "GET", "/api/snapshot", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("snapshot: status %d", res.StatusCode)
	}
	var snap store.Snapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		t.Fatal(err)
	}
	kinds := []string{}
	for _, d := range snap.UMLDiagrams {
		kinds = append(kinds, d.Kind)
	}
	if strings.Join(kinds, ",") != "usecase,class,sequence,state" {
		t.Errorf("uml_diagrams do exemplo: %v", kinds)
	}

	res, body = do(t, ts, "POST", "/api/uml", map[string]any{"kind": "class", "name": "Pagamentos"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("criação: status %d — %s", res.StatusCode, body)
	}
	var d model.UMLDiagram
	_ = json.Unmarshal(body, &d)
	if d.ID != "pagamentos" || d.File != ".arch/diagrams/class/pagamentos.json" || d.Elements == nil {
		t.Errorf("diagrama criado: %+v", d)
	}

	res, body = do(t, ts, "POST", "/api/uml/pagamentos/elements", map[string]any{
		"type": "class", "name": "Invoice", "attributes": []any{map[string]any{"name": "total", "type": "Money", "visibility": "+"}},
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("elemento: status %d — %s", res.StatusCode, body)
	}
	var el model.UMLElement
	_ = json.Unmarshal(body, &el)
	if el.ID != "el-invoice" || el.Position.X == 0 {
		t.Errorf("elemento criado: %+v", el)
	}
	res, _ = do(t, ts, "POST", "/api/uml/pagamentos/elements", map[string]any{"type": "interface", "name": "Gateway"})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("segundo elemento: status %d", res.StatusCode)
	}

	res, body = do(t, ts, "POST", "/api/uml/pagamentos/relations", map[string]any{
		"type": "dependency", "source": "el-invoice", "target": "Gateway",
	})
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("relação: status %d — %s", res.StatusCode, body)
	}
	var rel model.UMLRelation
	_ = json.Unmarshal(body, &rel)
	if rel.ID != "rel-1" || rel.Target != "el-gateway" {
		t.Errorf("relação criada: %+v", rel)
	}

	res, body = do(t, ts, "PATCH", "/api/uml/pagamentos/elements/el-invoice", map[string]any{"abstract": true})
	if res.StatusCode != http.StatusOK || !strings.Contains(string(body), `"abstract":true`) {
		t.Errorf("patch de elemento: status %d — %s", res.StatusCode, body)
	}

	res, body = do(t, ts, "GET", "/api/uml/pagamentos/mermaid", nil)
	var mmd struct {
		Mermaid string `json:"mermaid"`
	}
	_ = json.Unmarshal(body, &mmd)
	if res.StatusCode != http.StatusOK || !strings.Contains(mmd.Mermaid, "invoice ..> gateway") {
		t.Errorf("mermaid: status %d — %s", res.StatusCode, body)
	}

	// Geração a partir das fichas não colide com /api/uml/{id}/… e é idempotente.
	res, body = do(t, ts, "POST", "/api/uml/generate/use-cases", nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("generate: status %d — %s", res.StatusCode, body)
	}
	var gen model.UMLDiagram
	_ = json.Unmarshal(body, &gen)
	if gen.ID != "casos-de-uso" || len(gen.Elements) != 3 {
		t.Errorf("diagrama gerado: %+v", gen)
	}

	res, body = do(t, ts, "GET", "/api/uml", nil)
	var list []model.UMLDiagram
	_ = json.Unmarshal(body, &list)
	if res.StatusCode != http.StatusOK || len(list) != 5 {
		t.Errorf("lista: status %d, %d diagramas", res.StatusCode, len(list))
	}

	// 404 para ids inexistentes, 400 para validação.
	if res, _ = do(t, ts, "GET", "/api/uml/nao-existe", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("diagrama inexistente: status %d", res.StatusCode)
	}
	if res, _ = do(t, ts, "DELETE", "/api/uml/pagamentos/elements/el-fantasma", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("elemento inexistente: status %d", res.StatusCode)
	}
	if res, _ = do(t, ts, "POST", "/api/uml/pagamentos/elements", map[string]any{"type": "lifeline", "name": "X"}); res.StatusCode != http.StatusBadRequest {
		t.Errorf("tipo inválido: status %d", res.StatusCode)
	}

	res, _ = do(t, ts, "DELETE", "/api/uml/pagamentos", nil)
	if res.StatusCode != http.StatusOK {
		t.Errorf("remoção: status %d", res.StatusCode)
	}
}
