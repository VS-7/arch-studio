package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/skills"
)

// ---------------------------------------------------------------------------
// Módulo de Implementação: backlog, sprints, execução, memória e skills
// (RF045, RF049–RF052, RF056, RF060, RF061)
// ---------------------------------------------------------------------------
//
// O nível de autonomia do projeto (.arch/conventions.yaml) é aplicado aqui,
// no adaptador do agente: commit, push, abertura de PR e aplicação do
// planejamento só acontecem quando a convenção permite.

// agentName identifica o agente pelo clientInfo da sessão MCP (ex.:
// "claude-code"); o argumento `agent` tem precedência.
func agentName(ctx context.Context, req mcp.CallToolRequest) string {
	if a := strings.TrimSpace(req.GetString("agent", "")); a != "" {
		return a
	}
	if s, ok := server.ClientSessionFromContext(ctx).(server.SessionWithClientInfo); ok {
		if name := s.GetClientInfo().Name; name != "" {
			return name
		}
	}
	return "agente-mcp"
}

var checkItems = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"skill":    map[string]any{"type": "string", "description": "Nome da skill (ex.: segredos-e-config)."},
		"check":    map[string]any{"type": "string", "description": "Id do check (ex.: sem-segredo-no-repositorio)."},
		"result":   map[string]any{"type": "string", "enum": []string{"ok", "fail", "na"}},
		"evidence": map[string]any{"type": "string", "description": "Comando executado e resultado, ou por que não se aplica."},
	},
	"required": []string{"skill", "check", "result"},
}

func stringsArg(req mcp.CallToolRequest, key string) *[]string {
	if _, ok := req.GetArguments()[key]; !ok {
		return nil
	}
	v := stringSlice(req, key)
	if v == nil {
		v = []string{}
	}
	return &v
}

func optString(req mcp.CallToolRequest, key string) *string {
	if v, ok := req.GetArguments()[key].(string); ok {
		return &v
	}
	return nil
}

func registerImplementationTools(s *server.MCPServer, a *app.App) {
	// --- resume_work ------------------------------------------------------------
	s.AddTool(mcp.NewTool("resume_work",
		mcp.WithDescription("ONDE PAROU. Chame primeiro em toda sessão: sprint ativa, suas tarefas em andamento com checkpoint (último e próximo passo), últimas sessões suas e do time, estado real do Git (branch, arquivos alterados, ahead/behind), divergências entre memória e Git (o Git vence), a próxima tarefa pronta, as skills que valem para ela e memórias relevantes."),
		mcp.WithTitleAnnotation("Retomar o trabalho"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("detail", mcp.Description("'brief' ao abrir a sessão; 'full' ao assumir trabalho de outra pessoa."),
			mcp.Enum("brief", "full"), mcp.DefaultString("brief")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := a.Resume(req.GetString("detail", "brief"), "")
		if err != nil {
			return errResult(err)
		}
		return jsonResult(r)
	})

	// --- get_sprint -------------------------------------------------------------
	s.AddTool(mcp.NewTool("get_sprint",
		mcp.WithDescription("Estado de uma sprint: meta, datas, dias restantes, capacidade, carga e os itens com prontidão. Sem número, a sprint ativa."),
		mcp.WithTitleAnnotation("Ver sprint"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithNumber("number", mcp.Description("Número da sprint (0 = ativa).")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		st, err := a.Sprint(req.GetInt("number", 0))
		if err != nil {
			return errResult(err)
		}
		return jsonResult(st)
	})

	// --- list_backlog -----------------------------------------------------------
	s.AddTool(mcp.NewTool("list_backlog",
		mcp.WithDescription("Lista itens do backlog (épicos, histórias, tarefas, bugs…) na ordem de prioridade, com dependências, critérios e o campo 'ready'."),
		mcp.WithTitleAnnotation("Listar backlog"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("status", mcp.Description("Filtro de status."),
			mcp.Enum("open", "pending", "in_progress", "review", "completed", "blocked", "all"), mcp.DefaultString("open")),
		mcp.WithNumber("sprint", mcp.Description("Só itens desta sprint (0 = sem sprint). Omitido = qualquer.")),
		mcp.WithString("type", mcp.Description("Tipo do item."), mcp.Enum(model.ItemTypes...)),
		mcp.WithString("assignee", mcp.Description("'me' para as suas, ou o nome/e-mail de alguém.")),
		mcp.WithBoolean("only_ready", mcp.Description("Só itens executáveis com dependências concluídas.")),
		mcp.WithNumber("limit", mcp.Description("Máximo de itens."), mcp.DefaultNumber(30)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := app.BacklogQuery{Status: req.GetString("status", "open"), Type: req.GetString("type", ""),
			OnlyReady: req.GetBool("only_ready", false), Limit: req.GetInt("limit", 30)}
		if _, ok := req.GetArguments()["sprint"]; ok {
			n := req.GetInt("sprint", 0)
			q.Sprint = &n
		}
		if as := req.GetString("assignee", ""); as == "me" {
			q.Assignee = a.Person()
		} else {
			q.Assignee = as
		}
		items, total, err := a.Backlog(q)
		if err != nil {
			return errResult(err)
		}
		out := map[string]any{"items": items, "total": total}
		if total == 0 {
			out["hint"] = "Backlog vazio para o filtro. Sem itens no projeto? Chame sync_backlog."
		}
		return jsonResult(out)
	})

	// --- get_work_item ----------------------------------------------------------
	s.AddTool(mcp.NewTool("get_work_item",
		mcp.WithDescription("Detalhes completos de um item: descrição, critérios de aceite, dependências, checkpoint, checks registrados e notas."),
		mcp.WithTitleAnnotation("Ver item"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Id do item, ex.: TASK-CORE-01.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		it, err := a.Item(id)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(it)
	})

	// --- plan_sprint ------------------------------------------------------------
	s.AddTool(mcp.NewTool("plan_sprint",
		mcp.WithDescription("Propõe os itens de uma sprint: na ordem do backlog, com dependências satisfeitas, até a capacidade (pessoas × horas/dia × dias úteis). Aplicar a proposta só é permitido no nível de autonomia 'autonomo'; nos demais, um humano confirma na interface ou com `archcode-studio sprint plan N --apply`."),
		mcp.WithTitleAnnotation("Planejar sprint"),
		mcp.WithNumber("number", mcp.Description("Número da sprint (0 = próxima planejada ou nova).")),
		mcp.WithString("goal", mcp.Description("Meta da sprint em uma frase.")),
		mcp.WithNumber("capacity_hours", mcp.Description("Capacidade em horas (omitido = calculada).")),
		mcp.WithBoolean("apply", mcp.Description("Aplica a proposta (só no nível autônomo)."), mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		conv, err := a.Conventions()
		if err != nil {
			return errResult(err)
		}
		apply := req.GetBool("apply", false)
		note := ""
		if apply && !conv.AgentMayPush() {
			apply = false
			note = fmt.Sprintf("Autonomia %s: a proposta não foi aplicada. Peça a confirmação de um humano.", model.AutonomyLabel(conv.AI.Autonomy))
		}
		res, err := a.PlanSprint(req.GetInt("number", 0), req.GetString("goal", ""), req.GetFloat("capacity_hours", 0), nil, apply, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		out := map[string]any{"sprint": res.Sprint, "proposal": res.Proposal, "applied": res.Applied, "warnings": res.Warnings}
		if note != "" {
			out["note"] = note
		}
		return jsonResult(out)
	})

	// --- claim_task -------------------------------------------------------------
	s.AddTool(mcp.NewTool("claim_task",
		mcp.WithDescription("Reserva a tarefa para você: marca o dono, cria a branch da convenção (sprint-NN/<ID>-<slug>) e passa a trabalhar nela. Recusa se outra pessoa já reservou (inclusive no remoto) ou se há dependências pendentes. Publicar a reserva (push) só acontece no nível de autonomia 'autonomo'."),
		mcp.WithTitleAnnotation("Reservar tarefa"),
		mcp.WithString("task_id", mcp.Required(), mcp.Description("Id da tarefa.")),
		mcp.WithBoolean("switch_branch", mcp.Description("Troca para a branch da tarefa."), mcp.DefaultBool(true)),
		mcp.WithBoolean("push", mcp.Description("Publica a reserva no remoto (só no nível autônomo)."), mcp.DefaultBool(false)),
		mcp.WithBoolean("force", mcp.Description("Assume mesmo reservada/bloqueada (só com pedido explícito do usuário)."), mcp.DefaultBool(false)),
		mcp.WithString("agent", mcp.Description("Nome do agente (padrão: o cliente MCP).")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		conv, err := a.Conventions()
		if err != nil {
			return errResult(err)
		}
		push := req.GetBool("push", false) && conv.AgentMayPush()
		res, err := a.ClaimTask(id, app.ClaimOptions{Agent: agentName(ctx, req), Switch: req.GetBool("switch_branch", true),
			Push: push, Force: req.GetBool("force", false)}, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		out := map[string]any{"task": compactItem(res.Task), "branch": res.Branch, "branch_created": res.BranchCreated,
			"switched": res.Switched, "pushed": res.Pushed, "remote": res.Remote, "skills": res.Skills,
			"warnings":  res.Warnings,
			"next_step": "Leia as skills listadas com get_skill e grave um checkpoint a cada passo com save_checkpoint."}
		if !res.Pushed && res.NextCommand != "" {
			out["publish_command"] = res.NextCommand
			out["publish_note"] = fmt.Sprintf("Autonomia %s: publicar a reserva é com um humano.", model.AutonomyLabel(conv.AI.Autonomy))
		}
		return jsonResult(out)
	})

	// --- release_task -----------------------------------------------------------
	s.AddTool(mcp.NewTool("release_task",
		mcp.WithDescription("Libera a reserva da tarefa (ela volta a pendente, o checkpoint fica para quem assumir)."),
		mcp.WithTitleAnnotation("Liberar tarefa"),
		mcp.WithString("task_id", mcp.Required()),
		mcp.WithString("reason", mcp.Description("Motivo, visível para o time.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		it, err := a.ReleaseTask(id, req.GetString("reason", ""), false, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(compactItem(it))
	})

	// --- save_checkpoint --------------------------------------------------------
	s.AddTool(mcp.NewTool("save_checkpoint",
		mcp.WithDescription("Grava o checkpoint de retomada da tarefa: último passo, próximo passo, arquivos e testes falhando. Chame depois de CADA passo significativo, não só no fim — outra sessão ou pessoa continua daqui. Nunca inclua segredos."),
		mcp.WithTitleAnnotation("Salvar checkpoint"),
		mcp.WithString("task_id", mcp.Required()),
		mcp.WithString("last_step", mcp.Description("O que acabou de ser feito.")),
		mcp.WithString("next_step", mcp.Description("O próximo passo concreto.")),
		mcp.WithArray("files", mcp.Description("Arquivos em que está mexendo."), mcp.WithStringItems()),
		mcp.WithArray("failing_tests", mcp.Description("Testes que ainda falham."), mcp.WithStringItems()),
		mcp.WithString("notes", mcp.Description("Observações para quem retomar.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		it, err := a.SaveCheckpoint(id, app.HandoffInput{
			LastStep: optString(req, "last_step"), NextStep: optString(req, "next_step"), Notes: optString(req, "notes"),
			Files: stringsArg(req, "files"), FailingTests: stringsArg(req, "failing_tests"),
		}, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"task_id": it.ID, "handoff": it.Handoff})
	})

	// --- complete_task ----------------------------------------------------------
	s.AddTool(mcp.NewTool("complete_task",
		mcp.WithDescription("Conclui a tarefa informando o resultado de CADA check obrigatório das skills que valem para ela (ok, fail ou na) com a evidência. Recusa, dizendo o que falta, se algum check obrigatório não foi informado ou falhou. Com branch de trabalho, a tarefa vai para 'review' (PR); sem, para 'completed'."),
		mcp.WithTitleAnnotation("Concluir tarefa"),
		mcp.WithString("task_id", mcp.Required()),
		mcp.WithArray("checks", mcp.Description("Resultados dos checks das skills."), mcp.Items(checkItems)),
		mcp.WithString("notes", mcp.Description("Resumo do que foi entregue e como foi testado.")),
		mcp.WithString("status", mcp.Description("Força o destino."), mcp.Enum("review", "completed")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("task_id")
		if err != nil {
			return errResult(err)
		}
		var in app.CompleteInput
		if _, err := decodeArgs(req.GetArguments(), &in, "task_id"); err != nil {
			return errResult(err)
		}
		in.Force = false
		res, err := a.CompleteTask(id, in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		if res.Task != nil {
			res.Task = compactItemPtr(res.Task)
		}
		return jsonResult(res)
	})

	// --- log_session ------------------------------------------------------------
	s.AddTool(mcp.NewTool("log_session",
		mcp.WithDescription("Registra o diário da sessão em .arch/sessions/ antes de parar: resumo, o que foi feito, decisões, próximos passos, bloqueios, arquivos e comandos com resultado. Branch, commits e sprint vêm do Git e do backlog. Nunca inclua segredos."),
		mcp.WithTitleAnnotation("Registrar sessão"),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Uma ou duas frases.")),
		mcp.WithArray("done", mcp.WithStringItems()),
		mcp.WithArray("decisions", mcp.WithStringItems()),
		mcp.WithArray("next_steps", mcp.WithStringItems()),
		mcp.WithArray("blockers", mcp.WithStringItems()),
		mcp.WithArray("files", mcp.WithStringItems()),
		mcp.WithArray("commands", mcp.Description("Comandos executados e resultado, ex.: 'go test ./... → ok'."), mcp.WithStringItems()),
		mcp.WithArray("tasks", mcp.Description("Tarefas da sessão (padrão: as suas em andamento)."), mcp.WithStringItems()),
		mcp.WithString("agent", mcp.Description("Nome do agente (padrão: o cliente MCP).")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in app.SessionInput
		if _, err := decodeArgs(req.GetArguments(), &in); err != nil {
			return errResult(err)
		}
		in.Agent = agentName(ctx, req)
		sess, err := a.LogSession(in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"session": sess.ID, "file": sess.File, "tasks": sess.Tasks, "branch": sess.Branch})
	})

	// --- remember / recall ------------------------------------------------------
	s.AddTool(mcp.NewTool("remember",
		mcp.WithDescription("Grava conhecimento durável do projeto em .arch/memory/ (decisão pequena, convenção, armadilha, contexto de negócio, termo do glossário). Mesmo título atualiza a memória. Decisões arquiteturais grandes viram ADR (upsert_adr)."),
		mcp.WithTitleAnnotation("Lembrar"),
		mcp.WithString("title", mcp.Required()),
		mcp.WithString("body", mcp.Required(), mcp.Description("O fato, curto e verificável.")),
		mcp.WithString("type", mcp.Enum(model.NoteTypes...), mcp.DefaultString("contexto")),
		mcp.WithArray("tags", mcp.WithStringItems()),
		mcp.WithArray("components", mcp.Description("Rótulos dos componentes a que se refere."), mcp.WithStringItems()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var in app.NoteInput
		if _, err := decodeArgs(req.GetArguments(), &in); err != nil {
			return errResult(err)
		}
		n, err := a.Remember(in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"slug": n.Slug, "file": n.File})
	})

	s.AddTool(mcp.NewTool("recall",
		mcp.WithDescription("Busca memórias do projeto por texto, tipo ou componente."),
		mcp.WithTitleAnnotation("Recordar"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("query", mcp.Description("Palavras a procurar (vazio = todas).")),
		mcp.WithString("type", mcp.Enum(model.NoteTypes...)),
		mcp.WithString("component"),
		mcp.WithNumber("limit", mcp.DefaultNumber(10)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		notes, err := a.Recall(req.GetString("query", ""), req.GetString("type", ""), req.GetString("component", ""), req.GetInt("limit", 10))
		if err != nil {
			return errResult(err)
		}
		return jsonResult(map[string]any{"memories": notes, "total": len(notes)})
	})

	// --- propose_commit ---------------------------------------------------------
	s.AddTool(mcp.NewTool("propose_commit",
		mcp.WithDescription("Monta a mensagem de commit na convenção do projeto (ex.: 'Sprint 01 - Implementa emissão de JWT [TASK-API-01]') com os trailers Task/Story/Refs. Com commit=true, grava o commit dos arquivos preparados (git add) — permitido só nos níveis 'supervisionado' e 'autonomo'."),
		mcp.WithTitleAnnotation("Propor commit"),
		mcp.WithString("task_id", mcp.Description("Tarefa (padrão: a sua na branch atual).")),
		mcp.WithString("summary", mcp.Description("Resumo com verbo no presente da 3ª pessoa (padrão: gerado do título).")),
		mcp.WithString("body", mcp.Description("Por que a mudança foi feita.")),
		mcp.WithString("co_author", mcp.Description("Trailer Co-Authored-By, ex.: 'Claude <noreply@anthropic.com>'.")),
		mcp.WithBoolean("commit", mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		conv, err := a.Conventions()
		if err != nil {
			return errResult(err)
		}
		creq := app.CommitRequest{TaskID: req.GetString("task_id", ""), Summary: req.GetString("summary", ""),
			Body: req.GetString("body", ""), CoAuthor: req.GetString("co_author", ""), Commit: req.GetBool("commit", false)}
		note := ""
		if creq.Commit && !conv.AgentMayCommit() {
			creq.Commit = false
			note = "Autonomia Assistido: mostre a mensagem ao usuário; o commit é feito por um humano (comando em 'command')."
		}
		res, err := a.ProposeCommit(creq)
		if err != nil {
			return errResult(err)
		}
		out := map[string]any{"proposal": res}
		if note != "" {
			out["note"] = note
		}
		return jsonResult(out)
	})

	// --- prepare_pull_request ---------------------------------------------------
	s.AddTool(mcp.NewTool("prepare_pull_request",
		mcp.WithDescription("Gera título e corpo do PR na convenção: sprint e meta, itens fechados, critérios de aceite, checks das skills, como testar e impacto na arquitetura. Com open=true, publica a branch e abre o PR em rascunho via gh — só no nível 'autonomo'. Merge é sempre humano."),
		mcp.WithTitleAnnotation("Preparar pull request"),
		mcp.WithArray("task_ids", mcp.Description("Tarefas do PR (padrão: as suas na branch atual)."), mcp.WithStringItems()),
		mcp.WithBoolean("open", mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		conv, err := a.Conventions()
		if err != nil {
			return errResult(err)
		}
		ids := stringSlice(req, "task_ids")
		if req.GetBool("open", false) && conv.AgentMayPush() {
			pr, err := a.OpenPullRequest(ids, true)
			if err != nil {
				return errResult(err)
			}
			return jsonResult(pr)
		}
		pr, err := a.PreparePullRequest(ids)
		if err != nil {
			return errResult(err)
		}
		out := map[string]any{"pull_request": pr}
		if req.GetBool("open", false) {
			out["note"] = fmt.Sprintf("Autonomia %s: quem abre o PR é um humano (título, corpo e comando acima).", model.AutonomyLabel(conv.AI.Autonomy))
		}
		return jsonResult(out)
	})

	// --- create_backlog_item ----------------------------------------------------
	s.AddTool(mcp.NewTool("create_backlog_item",
		mcp.WithDescription("Registra no backlog algo descoberto fora do escopo da tarefa atual (bug, débito técnico, spike, risco de segurança ou tarefa), em vez de fazer agora."),
		mcp.WithTitleAnnotation("Criar item no backlog"),
		mcp.WithString("type", mcp.Required(), mcp.Enum(model.ItemBug, model.ItemDebt, model.ItemSpike, model.ItemSecurity, model.ItemTask)),
		mcp.WithString("title", mcp.Required()),
		mcp.WithString("description", mcp.Description("Contexto, com arquivo e linha quando houver.")),
		mcp.WithArray("acceptance", mcp.Description("Critérios de aceite."), mcp.WithStringItems()),
		mcp.WithString("priority", mcp.Enum(model.PriorityMust, model.PriorityShould, model.PriorityCould, model.PriorityWont)),
		mcp.WithString("parent", mcp.Description("História ou épico pai.")),
		mcp.WithString("component_id", mcp.Description("Id do componente do diagrama.")),
		mcp.WithArray("dependencies", mcp.WithStringItems()),
		mcp.WithNumber("estimate_h"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		in := app.ItemInput{Type: optString(req, "type"), Title: optString(req, "title"), Description: optString(req, "description"),
			Priority: optString(req, "priority"), Parent: optString(req, "parent"), ComponentID: optString(req, "component_id"),
			Dependencies: stringsArg(req, "dependencies")}
		if acc := stringSlice(req, "acceptance"); acc != nil {
			list := []model.Criterion{}
			for _, t := range acc {
				list = append(list, model.Criterion{Text: t})
			}
			in.Acceptance = &list
		}
		if _, ok := req.GetArguments()["estimate_h"]; ok {
			h := req.GetFloat("estimate_h", 0)
			in.EstimateHours = &h
		}
		it, err := a.CreateItem(in, hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(compactItem(it))
	})

	// --- sync_backlog -----------------------------------------------------------
	s.AddTool(mcp.NewTool("sync_backlog",
		mcp.WithDescription("Reconcilia o backlog com a arquitetura (casos de uso → épicos, requisitos funcionais → histórias, componentes → tarefas). Itens novos entram no fim; itens de componentes removidos são arquivados; edições humanas e status são preservados. Chame depois de modelar uma funcionalidade."),
		mcp.WithTitleAnnotation("Sincronizar backlog"),
		mcp.WithBoolean("dry_run", mcp.Description("Só mostra o diff."), mcp.DefaultBool(false)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		res, err := a.SyncBacklog(req.GetBool("dry_run", false), hub.SourceAI)
		if err != nil {
			return errResult(err)
		}
		return jsonResult(res)
	})

	// --- list_skills / get_skill ------------------------------------------------
	s.AddTool(mcp.NewTool("list_skills",
		mcp.WithDescription("Skills ativas do projeto (segurança, organização, padronização, uso do Studio) que valem para a tarefa, com os checks obrigatórios que complete_task vai exigir."),
		mcp.WithTitleAnnotation("Listar skills"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("task_id", mcp.Description("Tarefa (vazio = todas as ativas).")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		list, err := a.SkillsForTask(req.GetString("task_id", ""))
		if err != nil {
			return errResult(err)
		}
		out := []map[string]any{}
		for _, s := range list {
			req := []string{}
			for _, c := range s.Checks {
				if c.Required {
					req = append(req, c.ID)
				}
			}
			out = append(out, map[string]any{"name": s.Name, "description": s.Description, "category": s.Category,
				"trigger": s.Trigger, "required_checks": req})
		}
		return jsonResult(map[string]any{"skills": out, "hint": "Leia cada uma com get_skill antes de codar."})
	})

	s.AddTool(mcp.NewTool("get_skill",
		mcp.WithDescription("Conteúdo completo de uma skill: regras, como verificar e os checks (id, texto, comando, obrigatório)."),
		mcp.WithTitleAnnotation("Ler skill"),
		mcp.WithReadOnlyHintAnnotation(true),
		mcp.WithString("name", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name, err := req.RequireString("name")
		if err != nil {
			return errResult(err)
		}
		sk, err := a.Skill(name)
		if err != nil {
			return errResult(err)
		}
		return mcp.NewToolResultText(skillText(sk)), nil
	})
}

// skillText devolve a skill em Markdown, com os checks no fim.
func skillText(s *skills.Skill) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n## Checks (informe em complete_task: skill=%q)\n\n", strings.TrimSpace(s.Body), s.Name)
	for _, c := range s.Checks {
		req := "opcional"
		if c.Required {
			req = "obrigatório"
		}
		fmt.Fprintf(&b, "- check=%q (%s): %s", c.ID, req, c.Text)
		if c.Command != "" {
			fmt.Fprintf(&b, " — `%s`", c.Command)
		}
		b.WriteString("\n")
	}
	return b.String()
}

// compactItem reduz o item ao essencial para a resposta (RNF004).
func compactItem(it *model.WorkItem) map[string]any {
	if it == nil {
		return nil
	}
	out := map[string]any{"id": it.ID, "type": it.Type, "title": it.Title, "status": it.Status}
	if it.Sprint > 0 {
		out["sprint"] = it.Sprint
	}
	if it.Assignee != "" {
		out["assignee"] = it.Assignee
	}
	if it.Branch != "" {
		out["branch"] = it.Branch
	}
	if len(it.Acceptance) > 0 {
		out["acceptance"] = it.Acceptance
	}
	if len(it.Dependencies) > 0 {
		out["dependencies"] = it.Dependencies
	}
	return out
}

func compactItemPtr(it *model.WorkItem) *model.WorkItem {
	c := *it
	c.Description, c.Notes = "", ""
	return &c
}

// ---------------------------------------------------------------------------
// Prompts
// ---------------------------------------------------------------------------

func registerImplementationPrompts(s *server.MCPServer) {
	s.AddPrompt(mcp.NewPrompt("continue_sprint",
		mcp.WithPromptDescription("Retoma de onde parou e continua a sprint seguindo o protocolo do ArchCode Studio (sucessor de implement_next_task)."),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult("Continuar a sprint", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(strings.TrimSpace(`
Continue o trabalho da sprint seguindo exatamente este protocolo:

1. resume_work (detail="brief"). Se houver divergência entre memória e Git, o Git vence: ajuste antes de seguir.
2. Tarefa sua em andamento? Continue do next_step do checkpoint. Senão, claim_task na próxima tarefa pronta.
3. list_skills(task_id) e get_skill para cada skill; recall para armadilhas do componente.
4. Implemente APENAS o escopo da tarefa. O que estiver fora vira create_backlog_item.
   Faltou componente ou conexão no diagrama? Pare, modele (prompt model_new_feature) e rode sync_backlog.
5. save_checkpoint depois de cada passo significativo.
6. Testes provando cada critério de aceite; lint e archcode-studio validate verdes.
7. log_session → propose_commit → complete_task (com cada check obrigatório e a evidência) → prepare_pull_request.
8. Respeite a autonomia do projeto informada por resume_work. Merge, tag e release são sempre humanos.

Nunca edite .arch/diagrams/*.json nem .arch/plan/* à mão. Nunca grave segredos em checkpoints, sessões ou memórias.`))),
		}), nil
	})

	s.AddPrompt(mcp.NewPrompt("plan_sprint",
		mcp.WithPromptDescription("Planeja a próxima sprint a partir do backlog e da capacidade do time."),
		mcp.WithArgument("goal", mcp.ArgumentDescription("Meta da sprint em uma frase (opcional).")),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		goal := req.Params.Arguments["goal"]
		text := strings.TrimSpace(`
Planeje a próxima sprint:

1. get_sprint para ver a sprint ativa e o que sobrou; list_backlog(status="pending", sprint=0) para o backlog.
2. Proponha uma meta em uma frase` + map[bool]string{true: " (sugestão do usuário: " + goal + ")", false: ""}[goal != ""] + `.
3. plan_sprint com a meta: confira a capacidade, os itens escolhidos e os que ficaram de fora.
4. Apresente a proposta em uma tabela (id, título, horas, dependências) e pergunte ao usuário se aplica.
   A aplicação é feita por um humano, exceto no nível de autonomia autônomo.`)
		return mcp.NewGetPromptResult("Planejar sprint", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(text)),
		}), nil
	})

	s.AddPrompt(mcp.NewPrompt("close_sprint",
		mcp.WithPromptDescription("Prepara o encerramento da sprint ativa: resultado, pendências e próximos passos."),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult("Encerrar sprint", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(strings.TrimSpace(`
Prepare o encerramento da sprint ativa:

1. get_sprint: planejado × entregue, itens em revisão, bloqueados e parados.
2. Para cada item não concluído, diga se volta ao backlog ou passa para a próxima sprint, e por quê.
3. Liste as decisões da sprint (sessões e memórias) que merecem virar ADR.
4. Peça ao usuário para encerrar com "archcode-studio sprint close" (gera o relatório em docs/sprints/,
   atualiza o CHANGELOG e sugere a tag). A tag e a release são criadas por um humano.`))),
		}), nil
	})

	s.AddPrompt(mcp.NewPrompt("review_pull_request",
		mcp.WithPromptDescription("Revisa o trabalho da branch atual contra critérios de aceite, skills e arquitetura antes do PR."),
	), func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return mcp.NewGetPromptResult("Revisar pull request", []mcp.PromptMessage{
			mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(strings.TrimSpace(`
Revise a branch atual como revisor do time:

1. resume_work e get_work_item para cada tarefa da branch: critérios de aceite e checks registrados.
2. Leia o diff inteiro (git diff da branch principal até HEAD).
3. Para cada skill de list_skills(task_id), confira os checks obrigatórios contra o diff — não confie só no que foi informado.
4. Confira as fronteiras: o código só chama componentes conectados no diagrama (get_system_context).
5. Rode os comandos dos checks que tiverem comando.
6. Responda com: aprovado ou mudanças pedidas, e uma lista de achados com arquivo:linha e severidade.
   Achados fora do escopo viram create_backlog_item.`))),
		}), nil
	})
}
