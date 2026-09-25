// Package mcpserver expõe o ArchCode Studio como servidor Model Context
// Protocol, permitindo que agentes de IA leiam e modifiquem a arquitetura de
// forma atômica e determinística (RF015–RF017).
//
// Todas as ferramentas delegam para o pacote `app`, de modo que uma escrita
// feita por uma IA passa exatamente pelas mesmas invariantes de uma ação humana
// no canvas — inclusive a preservação de coordenadas dos nós existentes.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/pricing"
)

const instructions = `ArchCode Studio — arquitetura de software como código, com backlog, sprints e memória.

Este servidor dá acesso de leitura e escrita à arquitetura viva de um projeto
(diagrama de componentes, UML, contratos de API, requisitos, casos de uso,
ADRs, estimativas) e ao trabalho em cima dela: backlog, sprints, reservas de
tarefa, checkpoints, sessões, memórias do projeto e skills.

Ao começar QUALQUER sessão:
 1. resume_work                   — onde parou: sprint, suas tarefas com checkpoint, estado do Git
                                    (o Git vence a memória), próxima tarefa, skills e memórias.

Para implementar:
 2. claim_task                    — reserva a tarefa e cria a branch da convenção.
 3. list_skills / get_skill       — regras de segurança, organização e padrão que valem para ela.
 4. save_checkpoint               — depois de cada passo significativo.
 5. create_backlog_item           — o que estiver fora do escopo da tarefa.
 6. log_session → propose_commit → complete_task (com os checks das skills) → prepare_pull_request.

Para modelar (antes de codar algo que não está no diagrama):
 get_system_context → upsert_requirement / upsert_use_case → add_architecture_node / connect_nodes
 → validate_architecture_rules → sync_backlog.

Regras:
 - NUNCA edite .arch/diagrams/*.json, os JSON UML nem .arch/plan/* diretamente: use as ferramentas.
 - Respeite o nível de autonomia do projeto (resume_work informa): assistido (humano confirma
   commits), supervisionado (agente commita, humano publica) ou autônomo (agente publica e abre
   PR em rascunho). Merge, tag e release são SEMPRE humanos.
 - Nada de segredos em checkpoints, sessões ou memórias: tudo vai para o Git.
 - Todo componente novo deve ser justificado por um requisito ou caso de uso; toda aresta HTTP
   declara seus endpoints (api/endpoints.yaml).

Documento de requisitos formal: preencha category/related nos RNFs, description,
requirements e post_conditions nos casos de uso, registre cliente/usuários/histórico
com update_document_metadata e gere com generate_requirements_document.
Ferramentas legadas: get_implementation_tasks e mark_task_status continuam válidas.`

// Deps reúne o que o servidor MCP precisa para operar.
type Deps struct {
	App     *app.App
	Version string
}

// New monta o servidor MCP com todas as ferramentas e prompts.
func New(d Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"archcode-studio",
		d.Version,
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(false),
		server.WithRecovery(),
		server.WithInstructions(instructions),
	)
	registerTools(s, d.App)
	registerUMLTools(s, d.App)
	registerReqDocTools(s, d.App)
	registerImplementationTools(s, d.App)
	registerPrompts(s)
	registerImplementationPrompts(s)
	return s
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// jsonResult serializa a saída de forma compacta, omitindo campos vazios,
// para economizar tokens na janela de contexto do modelo (RNF004).
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError("falha ao serializar resposta: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func errResult(err error) (*mcp.CallToolResult, error) {
	return mcp.NewToolResultError(err.Error()), nil
}

func stringSlice(req mcp.CallToolRequest, key string) []string {
	v := req.GetStringSlice(key, nil)
	out := make([]string, 0, len(v))
	for _, s := range v {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ---------------------------------------------------------------------------
// Ferramentas
// ---------------------------------------------------------------------------

func registerTools(s *server.MCPServer, a *app.App) {
	nodeTypes := model.NodeTypes

	// --- 1. get_system_context ----------------------------------------------
	s.AddTool(mcp.NewTool("get_system_context",
		mcp.WithDescription("Retorna a visão macro da arquitetura: componentes, conexões, endpoints, requisitos, casos de uso, diagramas UML e progresso. Chame isto ANTES de qualquer outra coisa para não alucinar contexto."),
		mcp.WithTitleAnnotation("Contexto do sistema"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithBoolean("include_pricing",
			mcp.Description("Inclui o resumo de horas e custo estimados do projeto."),
			mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sum, err := a.ContextSummary(req.GetBool("include_pricing", false))
		if err != nil {
			return errResult(err)
		}
		return jsonResult(sum)
	})

	// --- 2. get_architecture_summary ----------------------------------------
	s.AddTool(mcp.NewTool("get_architecture_summary",
		mcp.WithDescription("Resumo ultracompacto: nome do projeto, contadores e lista de componentes. Use quando só precisar saber o que existe."),
		mcp.WithTitleAnnotation("Resumo da arquitetura"),
		mcp.WithReadOnlyHintAnnotation(true),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sum, err := a.ContextSummary(false)
		if err != nil {
			return errResult(err)
		}
		lines := []string{fmt.Sprintf("# %s (v%s)", sum.Project, sum.Version)}
		if sum.Description != "" {
			lines = append(lines, sum.Description)
		}
		lines = append(lines, fmt.Sprintf("Componentes: %d | Conexões: %d | Endpoints: %d | Requisitos: %d | Casos de uso: %d",
			sum.Counts["components"], sum.Counts["connections"], sum.Counts["endpoints"],
			sum.Counts["requirements"], sum.Counts["use_cases"]))
		for _, c := range sum.Components {
			tech := c.Tech
			if tech == "" {
				tech = "tecnologia não definida"
			}
			lines = append(lines, fmt.Sprintf("- %s [%s/%s] %s — %s", c.Label, c.Type, c.Tier, c.ID, tech))
		}
		if len(sum.UMLDiagrams) > 0 {
			lines = append(lines, "Diagramas UML:")
			for _, u := range sum.UMLDiagrams {
				lines = append(lines, "- "+u)
			}
		}
		if sum.Progress != nil {
			lines = append(lines, fmt.Sprintf("Progresso de implementação: %d%% (%d/%d tarefas)",
				sum.Progress.Percentage, sum.Progress.Completed, sum.Progress.TotalTasks))
		}
		return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
	})

	// --- 3. get_full_context -------------------------------------------------
	s.AddTool(mcp.NewTool("get_full_context",
		mcp.WithDescription("Retorna o estado consolidado do projeto: grafo completo, requisitos, casos de uso, ADRs e contratos de API. Resposta grande — prefira get_system_context quando possível."),
		mcp.WithTitleAnnotation("Contexto completo"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithBoolean("include_mermaid",
			mcp.Description("Inclui a representação Mermaid do diagrama."),
			mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		snap, err := a.Snapshot()
		if err != nil {
			return errResult(err)
		}
		if !req.GetBool("include_mermaid", false) {
			snap.Mermaid = ""
		}
		return jsonResult(snap)
	})

	// --- 4. add_architecture_node -------------------------------------------
	s.AddTool(mcp.NewTool("add_architecture_node",
		mcp.WithDescription("Cria um componente no diagrama, calculando automaticamente uma posição livre que não sobrepõe nós existentes. As coordenadas dos demais nós NUNCA são alteradas."),
		mcp.WithTitleAnnotation("Adicionar componente"),
		mcp.WithString("label", mcp.Required(),
			mcp.Description("Nome visível do componente, ex.: 'Payments Service'.")),
		mcp.WithString("type", mcp.Required(),
			mcp.Description("Tipo do componente."),
			mcp.Enum(nodeTypes...)),
		mcp.WithString("technology",
			mcp.Description("Stack concreto, ex.: 'Go 1.23 / Gin' ou 'PostgreSQL 16'.")),
		mcp.WithString("tier",
			mcp.Description("Camada arquitetural; inferida do tipo quando omitida."),
			mcp.Enum(model.Tiers...)),
		mcp.WithString("description",
			mcp.Description("Responsabilidade do componente em uma frase.")),
		mcp.WithString("complexity",
			mcp.Description("Complexidade de implementação, usada na precificação."),
			mcp.Enum("low", "medium", "high"), mcp.DefaultString("medium")),
		mcp.WithNumber("estimated_hours",
			mcp.Description("Horas estimadas; se omitido, deriva do tipo e da complexidade.")),
		mcp.WithString("cloud_tier",
			mcp.Description("Instância de nuvem associada, ex.: 'db.t4g.small' (catálogo em .arch/pricing.yaml).")),
		mcp.WithArray("tags", mcp.Description("Etiquetas como 'critical', 'pci-dss', 'lgpd'."), mcp.WithStringItems()),
		mcp.WithString("connect_to",
			mcp.Description("Id ou rótulo de um componente existente ao qual conectar imediatamente. Também serve de âncora para o posicionamento.")),
		mcp.WithString("protocol",
			mcp.Description("Protocolo da conexão criada por connect_to, ex.: 'REST', 'SQL', 'AMQP'.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		label, err := req.RequireString("label")
		if err != nil {
			return errResult(err)
		}
		in := app.NodeInput{
			Label:          label,
			Type:           req.GetString("type", "compute"),
			Technology:     req.GetString("technology", ""),
			Tier:           req.GetString("tier", ""),
			Description:    req.GetString("description", ""),
			Complexity:     req.GetString("complexity", "medium"),
			EstimatedHours: req.GetFloat("estimated_hours", 0),
			CloudTier:      req.GetString("cloud_tier", ""),
			Tags:           stringSlice(req, "tags"),
			ConnectTo:      req.GetString("connect_to", ""),
			Protocol:       req.GetString("protocol", ""),
		}
		node, err := a.AddNode(in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status":    "created",
			"node_id":   node.ID,
			"label":     node.Data.Label,
			"position":  node.Position,
			"next_step": "Garanta que exista um requisito ou caso de uso justificando este componente (upsert_requirement / upsert_use_case).",
		})
	})

	// --- 5. connect_nodes ----------------------------------------------------
	s.AddTool(mcp.NewTool("connect_nodes",
		mcp.WithDescription("Conecta dois componentes definindo protocolo, porta e segurança. Endpoints HTTP informados são gravados em api/endpoints.yaml."),
		mcp.WithTitleAnnotation("Conectar componentes"),
		mcp.WithString("source_id", mcp.Required(),
			mcp.Description("Id ou rótulo do componente de origem (quem chama).")),
		mcp.WithString("target_id", mcp.Required(),
			mcp.Description("Id ou rótulo do componente de destino (quem é chamado).")),
		mcp.WithString("protocol",
			mcp.Description("REST, gRPC, GraphQL, WebSocket, SQL, AMQP, Kafka, Redis, S3, Webhook…"),
			mcp.DefaultString("REST")),
		mcp.WithNumber("port", mcp.Description("Porta de rede, ex.: 5432.")),
		mcp.WithString("security", mcp.Description("Mecanismo de segurança, ex.: 'mTLS', 'JWT', 'API Key'.")),
		mcp.WithString("description", mcp.Description("O que trafega nesta conexão.")),
		mcp.WithString("complexity",
			mcp.Description("Complexidade da integração, usada na precificação."),
			mcp.Enum("low", "medium", "high")),
		mcp.WithArray("endpoints",
			mcp.Description(`Contratos HTTP desta conexão. Cada item: {"method":"POST","path":"/api/v1/auth/login","summary":"...","auth":"bearer","request":"{...}","response":"{...}","status_codes":[200,401]}`),
			mcp.Items(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"method":       map[string]any{"type": "string"},
					"path":         map[string]any{"type": "string"},
					"summary":      map[string]any{"type": "string"},
					"auth":         map[string]any{"type": "string"},
					"request":      map[string]any{"type": "string"},
					"response":     map[string]any{"type": "string"},
					"status_codes": map[string]any{"type": "array", "items": map[string]any{"type": "integer"}},
				},
				"required": []string{"method", "path"},
			})),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sourceID, err := req.RequireString("source_id")
		if err != nil {
			return errResult(err)
		}
		targetID, err := req.RequireString("target_id")
		if err != nil {
			return errResult(err)
		}
		in := app.EdgeInput{
			SourceID: sourceID,
			TargetID: targetID,
			// Vazio preserva o protocolo de uma conexão existente; conexões
			// novas nascem REST (padrão do app).
			Protocol:    req.GetString("protocol", ""),
			Port:        req.GetInt("port", 0),
			Security:    req.GetString("security", ""),
			Description: req.GetString("description", ""),
			Complexity:  req.GetString("complexity", ""),
		}
		if raw, ok := req.GetArguments()["endpoints"]; ok {
			data, _ := json.Marshal(raw)
			var eps []app.EndpointInput
			if err := json.Unmarshal(data, &eps); err != nil {
				return errResult(fmt.Errorf("campo 'endpoints' inválido: %w", err))
			}
			in.Endpoints = eps
		}
		edge, endpoints, err := a.ConnectNodes(in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		paths := []string{}
		for _, e := range endpoints {
			paths = append(paths, e.Method+" "+e.Path)
		}
		return jsonResult(map[string]any{
			"status": "connected", "edge_id": edge.ID,
			"protocol": edge.Data.Protocol, "endpoints_registered": paths,
		})
	})

	// --- 6. update_node_metadata ---------------------------------------------
	s.AddTool(mcp.NewTool("update_node_metadata",
		mcp.WithDescription("Altera tecnologia, descrição, tier, tags ou atributos de precificação de um componente existente. A posição no canvas é preservada."),
		mcp.WithTitleAnnotation("Atualizar componente"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("node_id", mcp.Required(), mcp.Description("Id ou rótulo do componente.")),
		mcp.WithString("label", mcp.Description("Novo rótulo.")),
		mcp.WithString("technology", mcp.Description("Nova tecnologia.")),
		mcp.WithString("description", mcp.Description("Nova descrição.")),
		mcp.WithString("tier", mcp.Description("Nova camada."), mcp.Enum(model.Tiers...)),
		mcp.WithString("complexity", mcp.Description("Nova complexidade."), mcp.Enum("low", "medium", "high")),
		mcp.WithNumber("estimated_hours", mcp.Description("Novas horas estimadas.")),
		mcp.WithString("cloud_tier", mcp.Description("Nova instância de nuvem.")),
		mcp.WithNumber("monthly_cost", mcp.Description("Custo mensal explícito, sobrepõe o catálogo.")),
		mcp.WithArray("tags", mcp.Description("Substitui a lista de etiquetas."), mcp.WithStringItems()),
		mcp.WithArray("requirements", mcp.Description("Ids de requisitos ligados a este componente."), mcp.WithStringItems()),
		mcp.WithArray("use_cases", mcp.Description("Códigos de casos de uso ligados a este componente."), mcp.WithStringItems()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		nodeID, err := req.RequireString("node_id")
		if err != nil {
			return errResult(err)
		}
		in := app.NodeInput{
			Label:          req.GetString("label", ""),
			Technology:     req.GetString("technology", ""),
			Description:    req.GetString("description", ""),
			Tier:           req.GetString("tier", ""),
			Complexity:     req.GetString("complexity", ""),
			EstimatedHours: req.GetFloat("estimated_hours", 0),
			CloudTier:      req.GetString("cloud_tier", ""),
			MonthlyCost:    req.GetFloat("monthly_cost", 0),
			Tags:           stringSlice(req, "tags"),
			Requirements:   stringSlice(req, "requirements"),
			UseCases:       stringSlice(req, "use_cases"),
		}
		node, err := a.UpdateNode(nodeID, in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"status": "updated", "node_id": node.ID, "label": node.Data.Label})
	})

	// --- 7. remove_architecture_node -----------------------------------------
	s.AddTool(mcp.NewTool("remove_architecture_node",
		mcp.WithDescription("Remove um componente e todas as suas conexões e endpoints. Use com parcimônia: a operação é destrutiva e reflete imediatamente no canvas do usuário."),
		mcp.WithTitleAnnotation("Remover componente"),
		mcp.WithString("node_id", mcp.Required(), mcp.Description("Id ou rótulo do componente a remover.")),
		mcp.WithString("reason", mcp.Description("Justificativa da remoção, registrada no evento enviado à interface.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		nodeID, err := req.RequireString("node_id")
		if err != nil {
			return errResult(err)
		}
		if err := a.RemoveNode(nodeID, hub.SourceAI); err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"status": "removed", "node_id": nodeID})
	})

	// --- 8. upsert_requirement -----------------------------------------------
	s.AddTool(mcp.NewTool("upsert_requirement",
		mcp.WithDescription("Cria ou atualiza um requisito funcional (RF) ou não funcional (RNF) em docs/requisitos.md. RNFs devem ter 'category' (agrupamento no documento de requisitos) e 'related' (requisitos associados)."),
		mcp.WithTitleAnnotation("Registrar requisito"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("id", mcp.Description("Identificador, ex.: 'RF004'. Omitido, o próximo livre é gerado.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Título curto do requisito.")),
		mcp.WithString("type", mcp.Description("RF (funcional) ou RNF (não funcional)."), mcp.Enum("RF", "RNF"), mcp.DefaultString("RF")),
		mcp.WithString("description", mcp.Description("Descrição completa, no formato 'O sistema deve…'.")),
		mcp.WithString("priority", mcp.Description("Prioridade de negócio."), mcp.Enum("Alta", "Média", "Baixa")),
		mcp.WithArray("components", mcp.Description("Ids dos componentes que realizam este requisito."), mcp.WithStringItems()),
		mcp.WithString("category", mcp.Description("Categoria do RNF no documento de requisitos: Usabilidade, Confiabilidade, Desempenho, Segurança, Software, Suportabilidade, Portabilidade, Interface, Legal… (texto livre).")),
		mcp.WithArray("related", mcp.Description("Requisitos associados (ids de RF/RNF), ex.: ['RF001','RF002']. Use ['Todos'] quando o RNF vale para todos."), mcp.WithStringItems()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		saved, created, err := a.UpsertRequirement(model.Requirement{
			ID:          req.GetString("id", ""),
			Title:       title,
			Type:        req.GetString("type", "RF"),
			Description: req.GetString("description", ""),
			Priority:    req.GetString("priority", ""),
			Components:  stringSlice(req, "components"),
			Category:    req.GetString("category", ""),
			Related:     stringSlice(req, "related"),
		}, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		status := "updated"
		if created {
			status = "created"
		}
		return jsonResult(map[string]any{"status": status, "id": saved.ID, "file": "docs/requisitos.md"})
	})

	// --- 9. upsert_use_case ---------------------------------------------------
	s.AddTool(mcp.NewTool("upsert_use_case",
		mcp.WithDescription("Cria ou atualiza a ficha estruturada de um caso de uso em docs/casos-de-uso/. Nos fluxos (main_flow, alternate_flows, exceptions), um item que começa com '# ' é um subtítulo de grupo (ex.: '# Cadastro de paciente'), não um passo: a numeração dos passos continua através dele."),
		mcp.WithTitleAnnotation("Registrar caso de uso"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("code", mcp.Description("Código, ex.: 'CDU003'. Omitido, o próximo livre é gerado.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Nome do caso de uso, ex.: 'Processar Assinatura Recorrente'.")),
		mcp.WithString("description", mcp.Description("Descrição do caso de uso: objetivo e resultado esperado, em um parágrafo.")),
		mcp.WithArray("actors", mcp.Description("Atores envolvidos."), mcp.WithStringItems()),
		mcp.WithArray("requirements", mcp.Description("Ids dos requisitos que o caso de uso realiza, ex.: ['RF001']."), mcp.WithStringItems()),
		mcp.WithArray("components", mcp.Description("Ids dos componentes envolvidos."), mcp.WithStringItems()),
		mcp.WithString("priority", mcp.Description("Prioridade de negócio."), mcp.Enum("Alta", "Média", "Baixa")),
		mcp.WithArray("pre_conditions", mcp.Description("Entradas e pré-condições."), mcp.WithStringItems()),
		mcp.WithArray("post_conditions", mcp.Description("Saídas e pós-condições."), mcp.WithStringItems()),
		mcp.WithArray("main_flow", mcp.Description("Passos do fluxo principal, em ordem. Itens '# Título' agrupam passos."), mcp.WithStringItems()),
		mcp.WithArray("alternate_flows", mcp.Description("Fluxos alternativos. Itens '# Título' agrupam passos."), mcp.WithStringItems()),
		mcp.WithArray("exceptions", mcp.Description("Fluxos de exceção. Itens '# Título' agrupam passos."), mcp.WithStringItems()),
		mcp.WithArray("business_rules", mcp.Description("Regras de negócio aplicáveis."), mcp.WithStringItems()),
		mcp.WithArray("acceptance", mcp.Description("Critérios de aceite no formato Given-When-Then."), mcp.WithStringItems()),
		mcp.WithString("complexity", mcp.Description("Complexidade do caso de uso."), mcp.Enum("low", "medium", "high")),
		mcp.WithNumber("estimated_hours", mcp.Description("Horas estimadas para implementar o caso de uso.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		uc := model.UseCase{
			Code:           req.GetString("code", ""),
			Name:           name,
			Description:    req.GetString("description", ""),
			Actors:         stringSlice(req, "actors"),
			Requirements:   stringSlice(req, "requirements"),
			Components:     stringSlice(req, "components"),
			Priority:       req.GetString("priority", ""),
			PreConditions:  stringSlice(req, "pre_conditions"),
			PostConditions: stringSlice(req, "post_conditions"),
			MainFlow:       stringSlice(req, "main_flow"),
			AlternateFlows: stringSlice(req, "alternate_flows"),
			Exceptions:     stringSlice(req, "exceptions"),
			BusinessRules:  stringSlice(req, "business_rules"),
			Acceptance:     stringSlice(req, "acceptance"),
			Complexity:     req.GetString("complexity", ""),
			EstimatedHours: req.GetFloat("estimated_hours", 0),
		}
		saved, path, err := a.UpsertUseCase(uc, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"status": "saved", "code": saved.Code, "file": path})
	})

	// --- 10. upsert_adr -------------------------------------------------------
	s.AddTool(mcp.NewTool("upsert_adr",
		mcp.WithDescription("Registra uma decisão arquitetural (ADR) em docs/architecture-decisions/, no formato Nygard."),
		mcp.WithTitleAnnotation("Registrar ADR"),
		mcp.WithString("id", mcp.Description("Identificador, ex.: 'ADR-002'. Omitido, o próximo livre é gerado.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Título da decisão.")),
		mcp.WithString("status", mcp.Description("Status da decisão."), mcp.Enum("Proposto", "Aceito", "Rejeitado", "Substituído", "Depreciado")),
		mcp.WithString("context", mcp.Description("Forças e restrições que motivaram a decisão.")),
		mcp.WithString("decision", mcp.Description("O que foi decidido.")),
		mcp.WithString("consequences", mcp.Description("Consequências positivas, negativas e trade-offs.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		title, err := req.RequireString("title")
		if err != nil {
			return errResult(err)
		}
		saved, path, err := a.UpsertADR(model.ADR{
			ID:           req.GetString("id", ""),
			Title:        title,
			Status:       req.GetString("status", "Proposto"),
			Context:      req.GetString("context", ""),
			Decision:     req.GetString("decision", ""),
			Consequences: req.GetString("consequences", ""),
		}, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"status": "saved", "id": saved.ID, "file": path})
	})

	// --- 11. calculate_project_estimate --------------------------------------
	s.AddTool(mcp.NewTool("calculate_project_estimate",
		mcp.WithDescription("Calcula horas de desenvolvimento, distribuição por perfil, custo total, custo mensal de nuvem e prazo estimado, com base em .arch/pricing.yaml."),
		mcp.WithTitleAnnotation("Estimar projeto"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithNumber("contingency_margin",
			mcp.Description("Margem de contingência; aceita 0.20 ou 20. Omitido, usa o valor de pricing.yaml.")),
		mcp.WithBoolean("detailed",
			mcp.Description("Inclui a lista item a item de onde vem cada hora."),
			mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		est, err := a.Estimate(req.GetFloat("contingency_margin", -1))
		if err != nil {
			return errResult(err)
		}
		if !req.GetBool("detailed", false) {
			est.Items = nil
		}
		var b strings.Builder
		cur := est.Currency
		fmt.Fprintf(&b, "## Estimativa do projeto\n\n")
		fmt.Fprintf(&b, "- Componentes: %s | Integrações: %s | Casos de uso: %s\n",
			pricing.Hours(est.NodeHours), pricing.Hours(est.EdgeHours), pricing.Hours(est.UseCaseHours))
		fmt.Fprintf(&b, "- Subtotal: %s + margem %.0f%% (%s) = **%s**\n",
			pricing.Hours(est.BaseHours), est.RiskMarginPercentage,
			pricing.Hours(est.MarginHours), pricing.Hours(est.TotalHours))
		b.WriteString("\n| Perfil | Horas | Valor/h | Subtotal |\n| :-- | --: | --: | --: |\n")
		for _, r := range est.Roles {
			fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", r.Role, pricing.Hours(r.Hours),
				pricing.Money(cur, r.Rate), pricing.Money(cur, r.Subtotal))
		}
		fmt.Fprintf(&b, "\n- Desenvolvimento: %s\n- Impostos (%.0f%%): %s\n- **TOTAL: %s**\n",
			pricing.Money(cur, est.PersonnelCost), est.TaxPercentage,
			pricing.Money(cur, est.TaxAmount), pricing.Money(cur, est.TotalCost))
		if est.CloudMonthlyCost > 0 {
			fmt.Fprintf(&b, "- Infraestrutura: %s/mês (%s/ano)\n",
				pricing.Money(cur, est.CloudMonthlyCost), pricing.Money(cur, est.CloudYearlyCost))
		}
		fmt.Fprintf(&b, "- Prazo: %.0f dias úteis (~%.1f meses) com equipe de %.0f pessoa(s)\n",
			est.WorkingDays, est.CalendarMonths, est.TeamSize)
		for _, warn := range est.Warnings {
			fmt.Fprintf(&b, "\n> ⚠️ %s", warn)
		}
		data, _ := json.Marshal(est)
		return mcp.NewToolResultText(b.String() + "\n\n<!-- dados estruturados -->\n" + string(data)), nil
	})

	// --- 12. generate_commercial_proposal ------------------------------------
	s.AddTool(mcp.NewTool("generate_commercial_proposal",
		mcp.WithDescription("Gera docs/proposta-comercial.md: escopo visual, componentes, esforço, investimento e prazo, pronto para enviar ao cliente."),
		mcp.WithTitleAnnotation("Gerar proposta comercial"),
		mcp.WithString("client_name", mcp.Description("Nome do cliente que receberá a proposta.")),
		mcp.WithNumber("validity_days", mcp.Description("Validade da proposta em dias."), mcp.DefaultNumber(15)),
		mcp.WithNumber("margin", mcp.Description("Margem de contingência a aplicar; aceita 0.20 ou 20.")),
		mcp.WithBoolean("include_diagram", mcp.Description("Inclui o diagrama Mermaid no documento."), mcp.DefaultBool(true)),
		mcp.WithBoolean("include_cloud", mcp.Description("Inclui a seção de custo de infraestrutura."), mcp.DefaultBool(true)),
		mcp.WithString("notes", mcp.Description("Observações adicionais, premissas ou exclusões de escopo.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := a.GenerateProposal(app.ProposalOptions{
			ClientName:     req.GetString("client_name", ""),
			ValidityDays:   req.GetInt("validity_days", 15),
			Margin:         req.GetFloat("margin", 0),
			IncludeDiagram: req.GetBool("include_diagram", true),
			IncludeCloud:   req.GetBool("include_cloud", true),
			Notes:          req.GetString("notes", ""),
		}, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "generated", "file_path": res.FilePath,
			"total_cost": res.TotalCost, "currency": res.Currency,
		})
	})

	// --- 13. generate_ai_prd --------------------------------------------------
	s.AddTool(mcp.NewTool("generate_ai_prd",
		mcp.WithDescription("Compila diagrama, endpoints, requisitos, casos de uso e ADRs no blueprint docs/ai-prd.md, com ordem topológica de implementação e critérios de aceite. Regenerar preserva o status já registrado das tarefas."),
		mcp.WithTitleAnnotation("Compilar PRD para IA"),
		mcp.WithString("target_stack",
			mcp.Description("Stack alvo, ex.: 'Go / React / PostgreSQL'. Omitido, é inferido das tecnologias dos nós.")),
		mcp.WithBoolean("include_test_scenarios",
			mcp.Description("Converte casos de uso em critérios Given-When-Then e cria a tarefa de testes E2E."),
			mcp.DefaultBool(true)),
		mcp.WithString("granularity",
			mcp.Description("'detailed' inclui tabelas de componentes, fluxos e contratos; 'summary' produz um documento enxuto."),
			mcp.Enum("detailed", "summary"), mcp.DefaultString("detailed")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := a.GenerateAIPRD(prd.Options{
			TargetStack:          req.GetString("target_stack", ""),
			IncludeTestScenarios: req.GetBool("include_test_scenarios", true),
			Granularity:          req.GetString("granularity", ""),
		}, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "generated", "file_path": res.FilePath, "total_tasks": res.TotalTasks,
			"integrity_hash": res.Hash, "overall_progress_percentage": res.Progress,
			"summary":   res.Summary,
			"next_step": "Chame get_implementation_tasks para pegar a primeira tarefa pronta.",
		})
	})

	// --- 14. get_implementation_tasks ----------------------------------------
	s.AddTool(mcp.NewTool("get_implementation_tasks",
		mcp.WithDescription("Devolve a fila ordenada de tarefas do ai-prd.md. Cada tarefa traz dependências, critérios de aceite e o campo 'ready' indicando se todas as dependências já estão concluídas."),
		mcp.WithTitleAnnotation("Listar tarefas"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("status",
			mcp.Description("Filtro de status."),
			mcp.Enum("pending", "in_progress", "completed", "blocked", "all"),
			mcp.DefaultString("pending")),
		mcp.WithBoolean("only_ready",
			mcp.Description("Retorna apenas tarefas cujas dependências já estão concluídas."),
			mcp.DefaultBool(false)),
		mcp.WithNumber("limit", mcp.Description("Número máximo de tarefas retornadas."), mcp.DefaultNumber(20)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		list, err := a.ImplementationTasks(app.TaskQuery{
			Status:    req.GetString("status", "pending"),
			OnlyReady: req.GetBool("only_ready", false),
			Limit:     req.GetInt("limit", 20),
		})
		if err != nil {
			return errResult(err)
		}
		tasks, board := list.Tasks, list.Board
		if len(board.Tasks) == 0 {
			return jsonResult(map[string]any{
				"tasks": []any{}, "total": 0,
				"hint": "Nenhum AI-PRD gerado ainda. Chame generate_ai_prd primeiro.",
			})
		}
		return jsonResult(map[string]any{
			"tasks": tasks, "total_in_board": len(board.Tasks),
			"overall_progress_percentage": board.ProgressPercentage(),
			"target_stack":                board.TargetStack,
			"truncated":                   list.Truncated,
		})
	})

	// --- 15. mark_task_status -------------------------------------------------
	s.AddTool(mcp.NewTool("mark_task_status",
		mcp.WithDescription("Atualiza o status de uma tarefa após implementar e validar. O progresso é refletido no canvas do usuário em tempo real."),
		mcp.WithTitleAnnotation("Atualizar status da tarefa"),
		mcp.WithIdempotentHintAnnotation(true),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Identificador da tarefa, ex.: 'TASK-AUTH-01'.")),
		mcp.WithString("status", mcp.Required(),
			mcp.Description("Novo status."),
			mcp.Enum("pending", "in_progress", "completed", "blocked")),
		mcp.WithString("notes",
			mcp.Description("Evidência objetiva: comando de teste executado e resultado, ou motivo do bloqueio.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		taskID, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		status, err := req.RequireString("status")
		if err != nil {
			return errResult(err)
		}
		res, err := a.MarkTaskStatus(taskID, status, req.GetString("notes", ""), hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(res)
	})

	// --- 16. validate_architecture_rules -------------------------------------
	s.AddTool(mcp.NewTool("validate_architecture_rules",
		mcp.WithDescription("Executa o linter de arquitetura: componentes isolados, bancos expostos a clientes, ausência de autenticação, conexões sem protocolo, componentes sem requisito que os justifique, entre outras regras."),
		mcp.WithTitleAnnotation("Validar arquitetura"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("min_severity",
			mcp.Description("Filtra o relatório pela severidade mínima."),
			mcp.Enum("error", "warning", "info"), mcp.DefaultString("info")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rep, err := a.Validate()
		if err != nil {
			return errResult(err)
		}
		rep.Filter(req.GetString("min_severity", "info"))
		return jsonResult(rep)
	})

	// --- 17. import_mermaid_diagram ------------------------------------------
	s.AddTool(mcp.NewTool("import_mermaid_diagram",
		mcp.WithDescription("Substitui o diagrama macro a partir de um snippet Mermaid (flowchart/graph). Nós com o mesmo rótulo mantêm suas coordenadas atuais; os demais recebem layout automático por camadas."),
		mcp.WithTitleAnnotation("Importar Mermaid"),
		mcp.WithString("source", mcp.Required(),
			mcp.Description("Código Mermaid completo, ex.: 'flowchart LR\\n  A[\"Web\"] --> B[\"API\"]'.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		src, err := req.RequireString("source")
		if err != nil {
			return errResult(err)
		}
		d, err := a.ImportMermaid(src, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{
			"status": "imported", "nodes": len(d.Nodes), "edges": len(d.Edges),
		})
	})

	// --- 18. export_openapi ---------------------------------------------------
	s.AddTool(mcp.NewTool("export_openapi",
		mcp.WithDescription("Converte api/endpoints.yaml em uma especificação OpenAPI 3.1 gravada em api/openapi.yaml."),
		mcp.WithTitleAnnotation("Exportar OpenAPI"),
		mcp.WithIdempotentHintAnnotation(true),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		path, err := a.ExportOpenAPI(hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]string{"status": "exported", "file_path": path})
	})
}

// ---------------------------------------------------------------------------
// Prompts
// ---------------------------------------------------------------------------

func registerPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("implement_next_task",
		mcp.WithPromptDescription("Instrui o agente a pegar a próxima tarefa pronta do AI-PRD e implementá-la seguindo o protocolo do ArchCode Studio."),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult("Implementar a próxima tarefa do AI-PRD", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(strings.TrimSpace(`
Siga exatamente este protocolo:

1. Chame get_system_context para carregar a arquitetura vigente.
2. Chame get_implementation_tasks com status="pending" e only_ready=true.
3. Pegue a primeira tarefa da lista. Se a lista vier vazia, pare e explique por quê.
4. Chame mark_task_status com status="in_progress" para essa tarefa.
5. Implemente APENAS o escopo daquela tarefa, respeitando todos os invariantes
   da seção 1 do docs/ai-prd.md.
6. Escreva e execute os testes que provam cada critério de aceite da tarefa.
7. Chame mark_task_status com status="completed" e, em notes, o comando de teste
   executado e o resultado. Se estiver bloqueado, use status="blocked" e explique.

Nunca edite .arch/diagrams/macro.json diretamente.`))),
		}), nil
	})

	s.AddPrompt(mcp.NewPrompt("model_new_feature",
		mcp.WithPromptDescription("Guia o agente para modelar uma nova funcionalidade na arquitetura antes de escrever qualquer código."),
		mcp.WithArgument("feature", mcp.ArgumentDescription("Descrição da funcionalidade a modelar.")),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		feature := "a funcionalidade solicitada pelo usuário"
		if v, ok := req.Params.Arguments["feature"]; ok && v != "" {
			feature = v
		}
		text := fmt.Sprintf(strings.TrimSpace(`
Modele %s na arquitetura, nesta ordem:

1. get_system_context — entenda o que já existe e reaproveite componentes.
2. upsert_requirement — registre o requisito funcional que justifica a mudança.
3. upsert_use_case — descreva o fluxo principal, exceções e critérios de aceite
   em Given-When-Then.
4. add_architecture_node — crie apenas os componentes realmente necessários,
   usando connect_to para ancorar o posicionamento junto de quem os consome.
5. connect_nodes — declare protocolo, porta, segurança e os endpoints HTTP.
6. validate_architecture_rules — corrija tudo que vier como "error".
7. generate_ai_prd — recompile o blueprint de implementação.

Ao final, resuma em até cinco linhas o que mudou na arquitetura e qual é a
primeira tarefa de implementação.`), feature)
		return mcp.NewGetPromptResult("Modelar nova funcionalidade", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
		}), nil
	})
}
