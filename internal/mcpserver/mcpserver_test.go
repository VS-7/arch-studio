package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
)

// newTestClient sobe o servidor MCP em processo sobre um projeto de exemplo.
func newTestClient(t *testing.T) (*client.Client, *app.App) {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := project.Init(st, project.Options{ProjectName: "MCP Teste"}); err != nil {
		t.Fatal(err)
	}
	a := app.New(st, nil)
	c, err := client.NewInProcessClient(New(Deps{App: a, Version: "test"}))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := c.Start(ctx); err != nil {
		t.Fatal(err)
	}
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	init.Params.ClientInfo = mcp.Implementation{Name: "teste", Version: "1"}
	if _, err := c.Initialize(ctx, init); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c, a
}

func call(t *testing.T, c *client.Client, tool string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = tool
	req.Params.Arguments = args
	res, err := c.CallTool(context.Background(), req)
	if err != nil {
		t.Fatalf("%s: %v", tool, err)
	}
	if res.IsError {
		t.Fatalf("%s devolveu erro: %+v", tool, res.Content)
	}
	return res
}

// Chamar connect_nodes numa conexão existente sem informar o protocolo (ex.:
// só para registrar endpoints) não pode transformá-la em REST.
func TestConnectNodesPreservaProtocoloExistente(t *testing.T) {
	c, a := newTestClient(t)

	call(t, c, "connect_nodes", map[string]any{
		"source_id": "node-core-api", "target_id": "node-postgres", "description": "consultas de leitura",
	})

	d, err := a.Diagram()
	if err != nil {
		t.Fatal(err)
	}
	edge := d.EdgeByID("edge-core-api-to-postgres")
	if edge == nil {
		t.Fatal("conexão de exemplo sumiu")
	}
	if edge.Data.Protocol != "SQL" {
		t.Fatalf("protocolo = %q, quer SQL preservado", edge.Data.Protocol)
	}
}

// auto_layout_diagram aceita um diagrama UML, "macro" ou "all"; com "all" a
// segunda chamada não muda nada (o layout é estável).
func TestAutoLayoutDiagram(t *testing.T) {
	c, a := newTestClient(t)

	before, err := a.GetUMLDiagram("modelo-de-dominio")
	if err != nil {
		t.Fatal(err)
	}
	call(t, c, "auto_layout_diagram", map[string]any{"diagram_id": "modelo-de-dominio"})
	after, _ := a.GetUMLDiagram("modelo-de-dominio")
	if before.Elements[0].Position == after.Elements[0].Position && before.Elements[1].Position == after.Elements[1].Position {
		t.Error("o diagrama de classes deveria ter sido reorganizado")
	}

	call(t, c, "auto_layout_diagram", map[string]any{"diagram_id": "macro"})
	res := call(t, c, "auto_layout_diagram", map[string]any{"diagram_id": "all"})
	text := res.Content[0].(mcp.TextContent).Text
	res = call(t, c, "auto_layout_diagram", map[string]any{"diagram_id": "all"})
	if again := res.Content[0].(mcp.TextContent).Text; !strings.Contains(again, `"diagrams":[]`) || !strings.Contains(again, `"architecture":false`) {
		t.Errorf("segunda reorganização mudou algo: %s (primeira: %s)", again, text)
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "auto_layout_diagram"
	req.Params.Arguments = map[string]any{"diagram_id": "nao-existe"}
	if res, err := c.CallTool(context.Background(), req); err != nil || !res.IsError {
		t.Errorf("diagrama inexistente deveria devolver erro: %v %+v", err, res)
	}
}
