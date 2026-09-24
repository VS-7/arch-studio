package openapi

import (
	"strings"
	"testing"

	"github.com/archcode/studio/internal/model"
)

func TestBuildGeraOperacoesESeguranca(t *testing.T) {
	m := &model.Manifest{ProjectName: "Loja", Version: "1.2.0"}
	d := &model.Diagram{Nodes: []model.Node{{ID: "node-api", Data: model.NodeData{Label: "Core API"}}}}
	spec := &model.EndpointsSpec{Endpoints: []model.Endpoint{
		{Method: "POST", Path: "/api/v1/orders", Summary: "Cria pedido", Target: "node-api", Auth: "JWT", StatusCodes: []int{201, 422}},
		{Method: "GET", Path: "/api/v1/health"},
	}}

	out, err := Build(m, d, spec)
	if err != nil {
		t.Fatal(err)
	}
	yaml := string(out)
	for _, want := range []string{
		"openapi: 3.1.0", "title: Loja API", "url: /api/v1",
		"/api/v1/orders:", "post:", "operationId: post-api-v1-orders", "- Core API",
		`"201":`, "description: Criado", `"422":`, "bearerAuth", "/api/v1/health:", `"200":`,
	} {
		if !strings.Contains(yaml, want) {
			t.Errorf("OpenAPI sem %q:\n%s", want, yaml)
		}
	}
}
