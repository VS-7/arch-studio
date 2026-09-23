package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Ferramentas — diagramas UML
// ---------------------------------------------------------------------------

// allUMLElementTypes e allUMLRelationTypes alimentam os enums dos schemas; a
// validação por tipo de diagrama acontece no pacote app.
var (
	allUMLElementTypes  = uniqueUnion(model.UMLElementTypes)
	allUMLRelationTypes = uniqueUnion(model.UMLRelationTypes)
)

func uniqueUnion(m map[string][]string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, kind := range model.UMLKinds {
		for _, t := range m[kind] {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	return out
}

// memberItems aceita tanto a notação textual UML quanto o objeto canônico.
var memberItems = map[string]any{
	"anyOf": []any{
		map[string]any{"type": "string"},
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name":       map[string]any{"type": "string"},
				"type":       map[string]any{"type": "string"},
				"visibility": map[string]any{"type": "string", "enum": model.UMLVisibilities},
				"static":     map[string]any{"type": "boolean"},
				"abstract":   map[string]any{"type": "boolean"},
				"default":    map[string]any{"type": "string"},
				"params":     map[string]any{"type": "string"},
			},
			"required": []string{"name"},
		},
	},
}

// elementFieldOptions são os campos opcionais comuns a add/update de elementos.
func elementFieldOptions() []mcp.ToolOption {
	return []mcp.ToolOption{
		mcp.WithString("stereotype", mcp.Description("Estereótipo sem « », ex.: 'service', 'entity'.")),
		mcp.WithString("documentation", mcp.Description("Texto livre; em notas, é o conteúdo exibido.")),
		mcp.WithBoolean("abstract", mcp.Description("Classe abstrata.")),
		mcp.WithArray("attributes",
			mcp.Description(`Atributos (class/interface). Aceita notação UML ("+ email: string", "- tentativas: int = 0") ou objetos {name,type,visibility,...}.`),
			mcp.Items(memberItems)),
		mcp.WithArray("operations",
			mcp.Description(`Operações (class/interface). Aceita notação UML ("+ login(email: string, senha: string): Token") ou objetos {name,params,type,visibility,...}.`),
			mcp.Items(memberItems)),
		mcp.WithArray("literals", mcp.Description("Literais de um enum, ex.: ['ACTIVE','BLOCKED']."), mcp.WithStringItems()),
		mcp.WithString("entry", mcp.Description("Ação de entrada (state).")),
		mcp.WithString("do", mcp.Description("Atividade contínua (state).")),
		mcp.WithString("exit", mcp.Description("Ação de saída (state).")),
		mcp.WithString("lifeline_kind", mcp.Description("Tipo de lifeline (padrão participant)."), mcp.Enum(model.UMLLifelineKinds...)),
		mcp.WithString("operator", mcp.Description("Operador do fragmento combinado (padrão alt)."), mcp.Enum(model.UMLOperators...)),
		mcp.WithString("guard", mcp.Description("Condição de guarda do fragmento.")),
		mcp.WithString("use_case", mcp.Description("Código do caso de uso vinculado, ex.: 'CDU001' (usecase).")),
		mcp.WithString("component_id", mcp.Description("Id do componente do diagrama macro que realiza este elemento, ex.: 'node-core-api'.")),
		mcp.WithString("parent_id", mcp.Description("Id ou nome do contêiner: boundary (usecase), package (class) ou estado composto (state).")),
	}
}

// decodeArgs converte os argumentos da ferramenta (sem as chaves de roteamento)
// no tipo de destino, passando pelo JSON para reaproveitar o UnmarshalJSON.
func decodeArgs(args map[string]any, out any, skip ...string) (map[string]any, error) {
	clean := map[string]any{}
	for k, v := range args {
		clean[k] = v
	}
	for _, k := range skip {
		delete(clean, k)
	}
	data, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			return nil, fmt.Errorf("argumentos inválidos: %w", err)
		}
	}
	return clean, nil
}

// compactElement remove posição e tamanho e escreve membros em notação UML,
// economizando tokens (RNF004).
func compactElement(e model.UMLElement) map[string]any {
	data, _ := json.Marshal(e)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	delete(out, "position")
	delete(out, "width")
	delete(out, "height")
	if len(e.Attributes) > 0 {
		attrs := make([]string, 0, len(e.Attributes))
		for _, m := range e.Attributes {
			attrs = append(attrs, model.FormatUMLMember(m, false))
		}
		out["attributes"] = attrs
	}
	if len(e.Operations) > 0 {
		ops := make([]string, 0, len(e.Operations))
		for _, m := range e.Operations {
			ops = append(ops, model.FormatUMLMember(m, true))
		}
		out["operations"] = ops
	}
	return out
}

func registerUMLTools(s *server.MCPServer, a *app.App) {
	// --- list_uml_diagrams ----------------------------------------------------
	s.AddTool(mcp.NewTool("list_uml_diagrams",
		mcp.WithDescription("Lista os diagramas UML do projeto (casos de uso, classes, sequência, estados) com contadores de elementos e relações."),
		mcp.WithTitleAnnotation("Listar diagramas UML"),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		list, err := a.ListUMLDiagrams()
		if err != nil {
			return errResult(err)
		}
		type brief struct {
			ID        string `json:"id"`
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Elements  int    `json:"elements"`
			Relations int    `json:"relations"`
		}
		out := make([]brief, 0, len(list))
		for _, d := range list {
			out = append(out, brief{d.ID, d.Kind, d.Name, len(d.Elements), len(d.Relations)})
		}
		return jsonResult(out)
	})

	// --- get_uml_diagram ------------------------------------------------------
	s.AddTool(mcp.NewTool("get_uml_diagram",
		mcp.WithDescription("Retorna um diagrama UML em JSON compacto (sem coordenadas) mais sua representação Mermaid."),
		mcp.WithTitleAnnotation("Ler diagrama UML"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("id", mcp.Required(), mcp.Description("Id do diagrama, ex.: 'modelo-de-dominio'.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		d, err := a.GetUMLDiagram(id)
		if err != nil {
			return errResult(err)
		}
		els := make([]map[string]any, 0, len(d.Elements))
		for _, e := range d.Elements {
			els = append(els, compactElement(e))
		}
		out := map[string]any{
			"id": d.ID, "kind": d.Kind, "name": d.Name, "file": d.File,
			"elements": els, "relations": d.Relations,
			"mermaid": mermaid.ExportUML(d),
		}
		if d.Description != "" {
			out["description"] = d.Description
		}
		return jsonResult(out)
	})

	// --- create_uml_diagram ---------------------------------------------------
	s.AddTool(mcp.NewTool("create_uml_diagram",
		mcp.WithDescription("Cria um diagrama UML vazio. O id é o slug do nome e não muda ao renomear."),
		mcp.WithTitleAnnotation("Criar diagrama UML"),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Tipo do diagrama."), mcp.Enum(model.UMLKinds...)),
		mcp.WithString("name", mcp.Required(), mcp.Description("Nome do diagrama, ex.: 'Modelo de Domínio'.")),
		mcp.WithString("description", mcp.Description("Objetivo do diagrama em uma frase.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		kind, err := req.RequireString("kind")
		if err != nil {
			return errResult(err)
		}
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		d, err := a.CreateUMLDiagram(kind, name, req.GetString("description", ""), hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "created", "id": d.ID, "kind": d.Kind, "file": d.File,
			"valid_element_types":  model.UMLElementTypes[d.Kind],
			"valid_relation_types": model.UMLRelationTypes[d.Kind],
		})
	})

	// --- add_uml_element ------------------------------------------------------
	addOpts := []mcp.ToolOption{
		mcp.WithDescription("Adiciona um elemento a um diagrama UML numa posição livre (elementos existentes nunca são movidos). Tipos por diagrama: usecase → actor, usecase, boundary, note; class → class, interface, enum, package, note; sequence → lifeline, fragment, note; state → state, initial, final, choice, fork, join, history, note."),
		mcp.WithTitleAnnotation("Adicionar elemento UML"),
		mcp.WithString("diagram_id", mcp.Required(), mcp.Description("Id do diagrama.")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Tipo do elemento."), mcp.Enum(allUMLElementTypes...)),
		mcp.WithString("name", mcp.Description("Nome visível. Obrigatório, exceto para initial, final, choice, fork, join, history e note.")),
	}
	s.AddTool(mcp.NewTool("add_uml_element", append(addOpts, elementFieldOptions()...)...),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			diagramID, err := req.RequireString("diagram_id")
			if err != nil {
				return errResult(err)
			}
			var in app.UMLElementInput
			if _, err := decodeArgs(req.GetArguments(), &in, "diagram_id", "id", "position"); err != nil {
				return errResult(err)
			}
			el, err := a.AddUMLElement(diagramID, in, hub.SourceAI)
			if err != nil {
				return errResult(err)
			}
			return jsonResult(map[string]any{
				"status": "created", "element_id": el.ID, "type": el.Type, "name": el.Name,
				"position": el.Position,
			})
		})

	// --- update_uml_element ---------------------------------------------------
	updOpts := []mcp.ToolOption{
		mcp.WithDescription("Atualiza campos de um elemento UML (merge: só os campos informados mudam; listas como attributes/operations são substituídas por inteiro). A posição é preservada."),
		mcp.WithTitleAnnotation("Atualizar elemento UML"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("diagram_id", mcp.Required(), mcp.Description("Id do diagrama.")),
		mcp.WithString("element_id", mcp.Required(), mcp.Description("Id ou nome do elemento.")),
		mcp.WithString("name", mcp.Description("Novo nome.")),
	}
	s.AddTool(mcp.NewTool("update_uml_element", append(updOpts, elementFieldOptions()...)...),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			diagramID, err := req.RequireString("diagram_id")
			if err != nil {
				return errResult(err)
			}
			elementID, err := req.RequireString("element_id")
			if err != nil {
				return errResult(err)
			}
			patch, err := decodeArgs(req.GetArguments(), nil, "diagram_id", "element_id", "id", "position", "width", "height")
			if err != nil {
				return errResult(err)
			}
			el, err := a.UpdateUMLElement(diagramID, elementID, patch, hub.SourceAI)
			if err != nil {
				return errResult(err)
			}
			return jsonResult(map[string]any{"status": "updated", "element_id": el.ID, "name": el.Name})
		})

	// --- add_uml_relation -----------------------------------------------------
	s.AddTool(mcp.NewTool("add_uml_relation",
		mcp.WithDescription("Liga dois elementos de um diagrama UML. A seta/losango fica sempre no TARGET: generalization/realization (filho → pai), aggregation/composition (parte → todo), include (base → incluído), extend (extensão → base), message/transition/dependency (origem → destino). Mensagens sem 'order' vão para o fim da sequência."),
		mcp.WithTitleAnnotation("Adicionar relação UML"),
		mcp.WithString("diagram_id", mcp.Required(), mcp.Description("Id do diagrama.")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Tipo da relação (válido para o tipo do diagrama)."), mcp.Enum(allUMLRelationTypes...)),
		mcp.WithString("source", mcp.Required(), mcp.Description("Id ou nome (case-insensitive) do elemento de origem.")),
		mcp.WithString("target", mcp.Required(), mcp.Description("Id ou nome (case-insensitive) do elemento de destino.")),
		mcp.WithString("name", mcp.Description("Rótulo; em mensagens, a assinatura, ex.: 'login(email, senha)'.")),
		mcp.WithString("source_multiplicity", mcp.Description("Multiplicidade na origem, ex.: '1'.")),
		mcp.WithString("target_multiplicity", mcp.Description("Multiplicidade no destino, ex.: '0..*'.")),
		mcp.WithString("source_role", mcp.Description("Papel na origem.")),
		mcp.WithString("target_role", mcp.Description("Papel no destino.")),
		mcp.WithString("message_kind", mcp.Description("Tipo da mensagem (padrão sync)."), mcp.Enum(model.UMLMessageKinds...)),
		mcp.WithNumber("order", mcp.Description("Posição da mensagem na sequência (1..n); as demais são deslocadas.")),
		mcp.WithString("trigger", mcp.Description("Evento disparador da transição.")),
		mcp.WithString("guard", mcp.Description("Condição de guarda da transição.")),
		mcp.WithString("effect", mcp.Description("Efeito executado na transição.")),
		mcp.WithString("documentation", mcp.Description("Observações.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramID, err := req.RequireString("diagram_id")
		if err != nil {
			return errResult(err)
		}
		var rel model.UMLRelation
		if _, err := decodeArgs(req.GetArguments(), &rel, "diagram_id", "id"); err != nil {
			return errResult(err)
		}
		saved, err := a.AddUMLRelation(diagramID, rel, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		out := map[string]any{
			"status": "created", "relation_id": saved.ID, "type": saved.Type,
			"source": saved.Source, "target": saved.Target,
		}
		if saved.Order > 0 {
			out["order"] = saved.Order
		}
		return jsonResult(out)
	})

	// --- remove_uml_item ------------------------------------------------------
	s.AddTool(mcp.NewTool("remove_uml_item",
		mcp.WithDescription("Remove um elemento (com todas as relações ligadas a ele) ou uma relação de um diagrama UML. Mensagens restantes são renumeradas."),
		mcp.WithTitleAnnotation("Remover item UML"),
		mcp.WithDestructiveHintAnnotation(true),
		mcp.WithString("diagram_id", mcp.Required(), mcp.Description("Id do diagrama.")),
		mcp.WithString("id", mcp.Required(), mcp.Description("Id do elemento ou da relação (elementos também aceitam o nome).")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		diagramID, err := req.RequireString("diagram_id")
		if err != nil {
			return errResult(err)
		}
		id, err := req.RequireString("id")
		if err != nil {
			return errResult(err)
		}
		item, err := a.RemoveUMLItem(diagramID, id, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"status": "removed", "id": id, "item": item})
	})

	// --- generate_use_case_diagram --------------------------------------------
	s.AddTool(mcp.NewTool("generate_use_case_diagram",
		mcp.WithDescription("Cria ou completa o diagrama de casos de uso (id 'casos-de-uso') a partir das fichas em docs/casos-de-uso: atores, um usecase por CDU dentro da fronteira do sistema e associações ator–caso de uso. Idempotente: preserva posições e só adiciona o que falta."),
		mcp.WithTitleAnnotation("Gerar diagrama de casos de uso"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("name", mcp.Description("Nome do diagrama (padrão 'Casos de Uso').")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		d, err := a.GenerateUseCaseDiagram(req.GetString("name", ""), hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "generated", "id": d.ID, "file": d.File,
			"elements": len(d.Elements), "relations": len(d.Relations),
		})
	})
}
