// Package openapi converte os contratos de api/endpoints.yaml numa
// especificação OpenAPI 3.1. É uma função pura: quem grava o arquivo é o
// pacote app.
package openapi

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/archcode/studio/internal/model"
)

// Build devolve o YAML de api/openapi.yaml.
func Build(m *model.Manifest, d *model.Diagram, spec *model.EndpointsSpec) ([]byte, error) {
	type oaResponse struct {
		Description string `yaml:"description"`
	}
	type oaOperation struct {
		Summary     string                `yaml:"summary,omitempty"`
		Description string                `yaml:"description,omitempty"`
		OperationID string                `yaml:"operationId"`
		Tags        []string              `yaml:"tags,omitempty"`
		Security    []map[string][]string `yaml:"security,omitempty"`
		Responses   map[string]oaResponse `yaml:"responses"`
	}

	paths := map[string]map[string]oaOperation{}
	needsAuth := false
	for _, e := range spec.Endpoints {
		method := strings.ToLower(e.Method)
		if _, ok := paths[e.Path]; !ok {
			paths[e.Path] = map[string]oaOperation{}
		}
		responses := map[string]oaResponse{}
		codes := e.StatusCodes
		if len(codes) == 0 {
			codes = []int{200}
		}
		for _, c := range codes {
			desc := e.Response
			if desc == "" {
				desc = statusText(c)
			}
			responses[fmt.Sprintf("%d", c)] = oaResponse{Description: desc}
		}
		op := oaOperation{
			Summary:     e.Summary,
			Description: e.Description,
			OperationID: model.Slugify(e.Method + "-" + e.Path),
			Responses:   responses,
		}
		if t := d.NodeByID(e.Target); t != nil {
			op.Tags = []string{t.Data.Label}
		}
		if e.Auth != "" && !strings.EqualFold(e.Auth, "none") {
			needsAuth = true
			op.Security = []map[string][]string{{"bearerAuth": {}}}
		}
		paths[e.Path][method] = op
	}

	doc := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       m.ProjectName + " API",
			"version":     m.Version,
			"description": m.Description,
		},
		"servers": []map[string]string{
			{"url": orDefault(spec.BaseURL, "/api/v1"), "description": "Servidor padrão"},
		},
		"paths": paths,
	}
	if needsAuth {
		doc["components"] = map[string]any{
			"securitySchemes": map[string]any{
				"bearerAuth": map[string]string{"type": "http", "scheme": "bearer", "bearerFormat": "JWT"},
			},
		}
	}

	var sb strings.Builder
	sb.WriteString("# Gerado pelo ArchCode Studio a partir de api/endpoints.yaml — não edite à mão.\n")
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	_ = enc.Close()

	return []byte(sb.String()), nil
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func statusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Criado"
	case 202:
		return "Aceito para processamento assíncrono"
	case 204:
		return "Sem conteúdo"
	case 400:
		return "Requisição inválida"
	case 401:
		return "Não autenticado"
	case 403:
		return "Não autorizado"
	case 404:
		return "Não encontrado"
	case 409:
		return "Conflito"
	case 422:
		return "Entidade não processável"
	case 429:
		return "Limite de requisições excedido"
	case 500:
		return "Erro interno"
	case 502:
		return "Erro no serviço upstream"
	default:
		return fmt.Sprintf("Status %d", code)
	}
}
