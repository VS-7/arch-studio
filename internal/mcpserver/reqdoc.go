package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Ferramentas — documento de requisitos
// ---------------------------------------------------------------------------

func registerReqDocTools(s *server.MCPServer, a *app.App) {
	// --- generate_requirements_document --------------------------------------
	s.AddTool(mcp.NewTool("generate_requirements_document",
		mcp.WithDescription("Gera o Documento de Requisitos formal (capa, histórico, sumário, introdução, descrição geral, RFs, RNFs por categoria com quadro de prioridade, diagramas de casos de uso, detalhamento dos CDUs, modelagem UML, arquitetura, matriz de rastreabilidade e referências) a partir dos dados do projeto. Grava docs/documento-de-requisitos.md e o SVG de cada figura em docs/diagramas/. Textos que não existem em outro lugar (cliente, usuários, histórico, referências, glossário) vêm de update_document_metadata."),
		mcp.WithTitleAnnotation("Gerar documento de requisitos"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("out",
			mcp.Description("Caminho do Markdown relativo à raiz do projeto. Padrão: docs/documento-de-requisitos.md (as figuras vão para a subpasta diagramas/ ao lado).")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := a.GenerateRequirementsDocument(req.GetString("out", ""), hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "generated", "file_path": res.File, "images": res.Images,
			"functional": res.Functional, "non_functional": res.NonFunctional,
			"use_cases": res.UseCases, "figures": res.Figures,
			"summary": "Documento de requisitos gerado: " + res.Summary() + ".",
		})
	})

	// --- update_document_metadata --------------------------------------------
	s.AddTool(mcp.NewTool("update_document_metadata",
		mcp.WithDescription("Atualiza (merge) os metadados do documento de requisitos em .arch/document.yaml: apenas os campos informados são alterados. Listas informadas substituem as atuais."),
		mcp.WithTitleAnnotation("Atualizar metadados do documento"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("title", mcp.Description("Título da capa. Padrão: 'Documento de Requisitos'.")),
		mcp.WithString("version", mcp.Description("Versão do documento. Vazio = versão do manifest.")),
		mcp.WithString("date", mcp.Description("Data do documento em ISO (aaaa-mm-dd). Vazio = data da geração.")),
		mcp.WithArray("authors", mcp.Description("Autores da capa, um por item. Vazio = autores do manifest."), mcp.WithStringItems()),
		mcp.WithString("client", mcp.Description("Texto da seção 'Cliente': quem contrata o sistema e por quê.")),
		mcp.WithString("users", mcp.Description("Texto da seção 'Usuário': perfis de usuário e responsabilidades. A tabela de atores é gerada automaticamente.")),
		mcp.WithString("introduction", mcp.Description("Substitui o primeiro parágrafo da Introdução (opcional).")),
		mcp.WithArray("history",
			mcp.Description(`Histórico de alterações, substitui o atual. Cada item: {"date":"2026-09-01","version":"0.1","description":"Draft inicial do documento.","author":"Nome"}`),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"date":        map[string]any{"type": "string"},
					"version":     map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"author":      map[string]any{"type": "string"},
				},
				"required": []string{"version", "description"},
			})),
		mcp.WithArray("references", mcp.Description("Referências bibliográficas, uma por item."), mcp.WithStringItems()),
		mcp.WithArray("glossary",
			mcp.Description(`Termos e abreviações, substitui o glossário atual. Cada item: {"term":"CDU","definition":"Caso de uso"}`),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"term":       map[string]any{"type": "string"},
					"definition": map[string]any{"type": "string"},
				},
				"required": []string{"term", "definition"},
			})),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		var p app.DocumentMetaPatch
		str := func(key string) *string {
			if _, ok := args[key]; !ok {
				return nil
			}
			v := req.GetString(key, "")
			return &v
		}
		list := func(key string) *[]string {
			if _, ok := args[key]; !ok {
				return nil
			}
			v := stringSlice(req, key)
			if v == nil {
				v = []string{}
			}
			return &v
		}
		p.Title, p.Version, p.Date = str("title"), str("version"), str("date")
		p.Client, p.Users, p.Introduction = str("client"), str("users"), str("introduction")
		p.Authors, p.References = list("authors"), list("references")
		if raw, ok := args["history"]; ok {
			var h []model.DocRevision
			if err := remarshal(raw, &h); err != nil {
				return errResult(fmt.Errorf("campo 'history' inválido: %w", err))
			}
			p.History = &h
		}
		if raw, ok := args["glossary"]; ok {
			var g []model.GlossaryTerm
			if err := remarshal(raw, &g); err != nil {
				return errResult(fmt.Errorf("campo 'glossary' inválido: %w", err))
			}
			p.Glossary = &g
		}
		meta, err := a.UpdateDocumentMeta(p, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "updated", "file": ".arch/document.yaml", "document": meta,
			"next_step": "Chame generate_requirements_document para regravar o documento.",
		})
	})
}

// remarshal converte um argumento genérico (map/slice decodificado do JSON)
// na estrutura tipada.
func remarshal(raw any, dst any) error {
	data, err := json.Marshal(raw)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}
