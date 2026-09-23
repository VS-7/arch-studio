// Package prd implementa o Compilador de PRD para Agentes de IA (RF019–RF022).
//
// Ele consolida diagrama, contratos de API, requisitos, casos de uso e ADRs em
// dois artefatos complementares:
//
//	docs/ai-prd.md   — documento legível, hiper-estruturado, consumido por LLMs
//	.arch/tasks.json — espelho legível por máquina, usado para rastrear progresso
//
// A ordem das tarefas respeita o grafo de dependências entre componentes: um nó
// só é implementado depois de tudo aquilo de que ele depende.
package prd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
)

// tierRank define a ordem canônica de implementação entre camadas.
var tierRank = map[string]int{
	"data":        0,
	"domain":      1,
	"backend":     2,
	"integration": 3,
	"devops":      4,
	"frontend":    5,
}

func rankOf(n model.Node) int {
	if r, ok := tierRank[layout.ResolveTier(n)]; ok {
		return r
	}
	return 2
}

// tierLabel é o rótulo humano exibido no ai-prd.md.
var tierLabel = map[string]string{
	"data":        "Data Tier",
	"domain":      "Domain Tier",
	"backend":     "Service Tier",
	"integration": "Integration Tier",
	"devops":      "DevOps Tier",
	"frontend":    "Presentation Tier",
}

// Options controla a granularidade da compilação.
type Options struct {
	TargetStack          string
	IncludeTestScenarios bool
	Granularity          string // summary | detailed
}

func (o Options) detailed() bool { return !strings.EqualFold(o.Granularity, "summary") }

// Input reúne tudo que o compilador precisa ler.
type Input struct {
	Manifest     *model.Manifest
	Diagram      *model.Diagram
	Requirements *model.RequirementsDoc
	UseCases     []model.UseCase
	ADRs         []model.ADR
	Endpoints    *model.EndpointsSpec
	Previous     *model.TaskBoard
}

// Result é a saída da compilação.
type Result struct {
	Markdown string
	Board    *model.TaskBoard
	Hash     string
}

// ---------------------------------------------------------------------------
// Ordenação topológica
// ---------------------------------------------------------------------------

// topoOrder devolve os nós ordenados de forma que todo nó apareça depois
// daqueles de que depende. Uma aresta A → B significa "A consome B", logo B é
// pré-requisito de A. Ciclos são desempatados pelo rank de tier.
func topoOrder(d *model.Diagram) ([]model.Node, map[string][]string) {
	nodes := []model.Node{}
	for _, n := range d.Nodes {
		if n.Type != "group" {
			nodes = append(nodes, n)
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if rankOf(nodes[i]) != rankOf(nodes[j]) {
			return rankOf(nodes[i]) < rankOf(nodes[j])
		}
		return nodes[i].Data.Label < nodes[j].Data.Label
	})

	valid := map[string]bool{}
	for _, n := range nodes {
		valid[n.ID] = true
	}

	deps := map[string][]string{}       // nó -> pré-requisitos
	dependents := map[string][]string{} // pré-requisito -> dependentes
	for _, e := range d.Edges {
		if !valid[e.Source] || !valid[e.Target] || e.Source == e.Target {
			continue
		}
		if containsStr(deps[e.Source], e.Target) {
			continue
		}
		deps[e.Source] = append(deps[e.Source], e.Target)
		dependents[e.Target] = append(dependents[e.Target], e.Source)
	}

	pending := map[string]int{}
	for _, n := range nodes {
		pending[n.ID] = len(deps[n.ID])
	}

	byID := map[string]model.Node{}
	for _, n := range nodes {
		byID[n.ID] = n
	}

	ready := []string{}
	for _, n := range nodes {
		if pending[n.ID] == 0 {
			ready = append(ready, n.ID)
		}
	}

	ordered := []model.Node{}
	emitted := map[string]bool{}
	sortReady := func() {
		sort.SliceStable(ready, func(i, j int) bool {
			a, b := byID[ready[i]], byID[ready[j]]
			if rankOf(a) != rankOf(b) {
				return rankOf(a) < rankOf(b)
			}
			return a.Data.Label < b.Data.Label
		})
	}

	for len(ordered) < len(nodes) {
		sortReady()
		if len(ready) == 0 {
			// Ciclo detectado: libera o nó de menor rank ainda pendente.
			best := ""
			for _, n := range nodes {
				if emitted[n.ID] {
					continue
				}
				if best == "" || rankOf(byID[n.ID]) < rankOf(byID[best]) {
					best = n.ID
				}
			}
			if best == "" {
				break
			}
			ready = append(ready, best)
			continue
		}
		id := ready[0]
		ready = ready[1:]
		if emitted[id] {
			continue
		}
		emitted[id] = true
		ordered = append(ordered, byID[id])
		for _, dep := range dependents[id] {
			if emitted[dep] {
				continue
			}
			pending[dep]--
			if pending[dep] <= 0 {
				ready = append(ready, dep)
			}
		}
	}
	return ordered, deps
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Geração de tarefas
// ---------------------------------------------------------------------------

func taskAbbrev(label string) string {
	slug := model.Slugify(label)
	parts := strings.Split(slug, "-")
	skip := map[string]bool{"service": true, "servico": true, "api": true, "app": true, "db": true}
	for _, p := range parts {
		if p != "" && !skip[p] {
			if len(p) > 8 {
				p = p[:8]
			}
			return strings.ToUpper(p)
		}
	}
	if len(slug) > 8 {
		slug = slug[:8]
	}
	if slug == "" {
		return "CORE"
	}
	return strings.ToUpper(slug)
}

func endpointsForNode(spec *model.EndpointsSpec, nodeID string) []model.Endpoint {
	out := []model.Endpoint{}
	if spec == nil {
		return out
	}
	for _, e := range spec.Endpoints {
		if e.Target == nodeID || e.Source == nodeID {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func useCasesForNode(list []model.UseCase, node model.Node) []model.UseCase {
	out := []model.UseCase{}
	for _, uc := range list {
		for _, c := range uc.Components {
			if c == node.ID || strings.EqualFold(c, node.Data.Label) {
				out = append(out, uc)
				break
			}
		}
	}
	return out
}

func requirementsForNode(doc *model.RequirementsDoc, node model.Node) []string {
	out := []string{}
	if doc == nil {
		return out
	}
	for _, r := range doc.Requirements {
		for _, c := range r.Components {
			if c == node.ID || strings.EqualFold(c, node.Data.Label) {
				out = append(out, r.ID)
				break
			}
		}
	}
	return out
}

// defaultAcceptance gera critérios verificáveis a partir do tipo/tier do nó.
func defaultAcceptance(n model.Node, stack string, eps []model.Endpoint) []string {
	tech := n.Data.Technology
	if tech == "" {
		tech = stack
	}
	out := []string{}
	switch n.Type {
	case "database":
		out = append(out,
			fmt.Sprintf("Migrações idempotentes criam o schema de `%s` e podem ser aplicadas duas vezes sem erro.", n.Data.Label),
			"Rollback da última migração restaura o schema anterior sem perda de dados de teste.")
	case "cache":
		out = append(out,
			fmt.Sprintf("Chaves de `%s` têm TTL explícito e a aplicação funciona (degradada) com o cache indisponível.", n.Data.Label))
	case "queue":
		out = append(out,
			fmt.Sprintf("Consumidores de `%s` são idempotentes: reprocessar a mesma mensagem não duplica efeitos.", n.Data.Label),
			"Mensagens com falha permanente vão para dead-letter queue e são observáveis.")
	case "gateway":
		out = append(out,
			"Rotas desconhecidas retornam 404 e erros upstream retornam 502 com corpo padronizado.",
			"Rate limiting e CORS configurados e cobertos por teste de integração.")
	case "storage":
		out = append(out, "Upload e download assinados funcionam e objetos privados não são acessíveis sem assinatura.")
	case "client":
		out = append(out,
			"A interface consome exclusivamente os contratos declarados em `api/endpoints.yaml`.",
			"Estados de carregamento, vazio e erro estão implementados em todas as telas do fluxo.")
	case "external_service":
		out = append(out,
			fmt.Sprintf("Integração com `%s` possui timeout, retry com backoff e client fake para testes.", n.Data.Label),
			"Credenciais vêm de variáveis de ambiente — nunca versionadas no repositório.")
	default:
		out = append(out,
			fmt.Sprintf("Testes unitários da lógica de `%s` cobrem cenários de sucesso e de falha.", n.Data.Label))
	}
	for _, e := range eps {
		out = append(out, fmt.Sprintf("Contrato `%s %s` implementado e validado contra `api/endpoints.yaml`.",
			strings.ToUpper(e.Method), e.Path))
	}
	return out
}

// givenWhenThen converte um caso de uso em critérios de aceite verificáveis.
func givenWhenThen(uc model.UseCase) []string {
	if len(uc.Acceptance) > 0 {
		return uc.Acceptance
	}
	given := "o sistema está no estado inicial"
	if len(uc.PreConditions) > 0 {
		given = strings.Join(uc.PreConditions, " e ")
	}
	when := "o ator executa o fluxo principal"
	then := "o resultado esperado do caso de uso é observável"
	if len(uc.MainFlow) > 0 {
		when = uc.MainFlow[0]
		then = uc.MainFlow[len(uc.MainFlow)-1]
	}
	out := []string{fmt.Sprintf("**Given** %s **When** %s **Then** %s.",
		strings.TrimSuffix(given, "."), strings.TrimSuffix(when, "."), strings.TrimSuffix(then, "."))}
	for _, ex := range uc.Exceptions {
		out = append(out, fmt.Sprintf("**Given** %s **When** %s **Then** o sistema trata a exceção sem estado inconsistente.",
			strings.TrimSuffix(given, "."), strings.TrimSuffix(ex, ".")))
	}
	return out
}

// buildInvariants deriva regras negativas e positivas a partir da arquitetura.
func buildInvariants(in Input, stack string) []string {
	inv := []string{
		"A camada de domínio NUNCA importa pacotes de infraestrutura, banco de dados ou transporte HTTP.",
		"Nenhum segredo, token ou credencial é versionado: toda configuração sensível vem de variáveis de ambiente.",
		"Toda alteração de contrato de API exige atualização correspondente em `api/endpoints.yaml` no mesmo commit.",
	}

	hasAuth := false
	privatePrefixes := map[string]bool{}
	for _, e := range in.Endpoints.Endpoints {
		if e.Auth != "" && !strings.EqualFold(e.Auth, "none") && !strings.EqualFold(e.Auth, "público") {
			hasAuth = true
			parts := strings.Split(strings.Trim(e.Path, "/"), "/")
			if len(parts) >= 2 {
				privatePrefixes["/"+parts[0]+"/"+parts[1]+"/*"] = true
			}
		}
	}
	if hasAuth {
		prefixes := []string{}
		for p := range privatePrefixes {
			prefixes = append(prefixes, "`"+p+"`")
		}
		sort.Strings(prefixes)
		inv = append(inv, fmt.Sprintf(
			"Toda rota autenticada (%s) passa obrigatoriamente pelo middleware de autenticação antes do handler.",
			strings.Join(prefixes, ", ")))
	}

	for _, n := range in.Diagram.Nodes {
		switch n.Type {
		case "database":
			inv = append(inv, fmt.Sprintf(
				"Acesso a `%s` acontece exclusivamente através da camada de repositório — nenhum SQL em handlers.", n.Data.Label))
		case "queue":
			inv = append(inv, fmt.Sprintf(
				"Todo consumidor de `%s` é idempotente e tolerante a entregas duplicadas.", n.Data.Label))
		case "external_service":
			inv = append(inv, fmt.Sprintf(
				"Chamadas a `%s` são encapsuladas atrás de uma interface própria, com timeout e retry — nunca chamadas diretas espalhadas pelo código.", n.Data.Label))
		}
		for _, tag := range n.Data.Tags {
			switch strings.ToLower(tag) {
			case "pci-dss":
				inv = append(inv, fmt.Sprintf("`%s` está em escopo PCI-DSS: dados de cartão jamais são logados ou persistidos em claro.", n.Data.Label))
			case "lgpd", "gdpr":
				inv = append(inv, fmt.Sprintf("`%s` trata dados pessoais: aplique minimização, retenção definida e trilha de auditoria.", n.Data.Label))
			}
		}
	}

	for _, r := range in.Requirements.NonFunctional() {
		inv = append(inv, fmt.Sprintf("[%s] %s", r.ID, strings.TrimSuffix(r.Title, ".")))
	}
	return inv
}

// ---------------------------------------------------------------------------
// Compilação
// ---------------------------------------------------------------------------

func Compile(in Input, opts Options) *Result {
	if in.Endpoints == nil {
		in.Endpoints = model.NewEndpointsSpec()
	}
	if in.Requirements == nil {
		in.Requirements = &model.RequirementsDoc{}
	}

	stack := opts.TargetStack
	if stack == "" {
		stack = inferStack(in.Diagram)
	}

	ordered, deps := topoOrder(in.Diagram)

	// Tarefas, na ordem topológica.
	board := model.NewTaskBoard()
	board.TargetStack = stack
	board.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	taskIDByNode := map[string]string{}
	usedIDs := map[string]bool{}
	for i, n := range ordered {
		abbrev := taskAbbrev(n.Data.Label)
		id := fmt.Sprintf("TASK-%s-%02d", abbrev, 1)
		for seq := 2; usedIDs[id]; seq++ {
			id = fmt.Sprintf("TASK-%s-%02d", abbrev, seq)
		}
		usedIDs[id] = true
		taskIDByNode[n.ID] = id

		tier := layout.ResolveTier(n)
		eps := endpointsForNode(in.Endpoints, n.ID)
		ucs := useCasesForNode(in.UseCases, n)

		acceptance := defaultAcceptance(n, stack, eps)
		if opts.IncludeTestScenarios {
			for _, uc := range ucs {
				acceptance = append(acceptance, givenWhenThen(uc)...)
			}
		}

		epIDs := []string{}
		for _, e := range eps {
			epIDs = append(epIDs, strings.ToUpper(e.Method)+" "+e.Path)
		}
		ucCodes := []string{}
		for _, uc := range ucs {
			ucCodes = append(ucCodes, uc.Code)
		}

		depIDs := []string{}
		for _, depNode := range deps[n.ID] {
			if tid, ok := taskIDByNode[depNode]; ok {
				depIDs = append(depIDs, tid)
			}
		}
		sort.Strings(depIDs)

		title := fmt.Sprintf("Implementar %s", n.Data.Label)
		if n.Data.Technology != "" {
			title += fmt.Sprintf(" (%s)", n.Data.Technology)
		}

		board.Tasks = append(board.Tasks, model.Task{
			ID:           id,
			Title:        title,
			Tier:         tier,
			Component:    n.Data.Label,
			ComponentID:  n.ID,
			Dependencies: depIDs,
			Acceptance:   acceptance,
			Requirements: requirementsForNode(in.Requirements, n),
			UseCases:     ucCodes,
			Endpoints:    epIDs,
			Status:       model.StatusPending,
			Order:        i + 1,
		})
	}

	// Tarefa final de verificação ponta a ponta.
	if opts.IncludeTestScenarios && len(in.UseCases) > 0 {
		allDeps := []string{}
		for _, t := range board.Tasks {
			allDeps = append(allDeps, t.ID)
		}
		acceptance := []string{}
		for _, uc := range in.UseCases {
			for _, gwt := range givenWhenThen(uc) {
				acceptance = append(acceptance, fmt.Sprintf("[%s] %s", uc.Code, gwt))
			}
		}
		board.Tasks = append(board.Tasks, model.Task{
			ID:           "TASK-E2E-01",
			Title:        "Testes ponta a ponta dos casos de uso",
			Tier:         "devops",
			Dependencies: allDeps,
			Acceptance:   acceptance,
			Status:       model.StatusPending,
			Order:        len(board.Tasks) + 1,
		})
	}

	// Preserva progresso já registrado pelos agentes (RF022).
	if in.Previous != nil {
		for i := range board.Tasks {
			if old := in.Previous.ByID(board.Tasks[i].ID); old != nil {
				board.Tasks[i].Status = old.Status
				board.Tasks[i].Notes = old.Notes
				board.Tasks[i].UpdatedAt = old.UpdatedAt
			}
		}
	}

	invariants := buildInvariants(in, stack)
	md := render(in, opts, stack, board, invariants)
	sum := sha256.Sum256([]byte(md))
	hash := hex.EncodeToString(sum[:])[:7]
	board.SourceHash = hash
	md = strings.Replace(md, "{{HASH}}", hash, 1)

	return &Result{Markdown: md, Board: board, Hash: hash}
}

func inferStack(d *model.Diagram) string {
	techs := []string{}
	seen := map[string]bool{}
	for _, n := range d.Nodes {
		t := strings.TrimSpace(n.Data.Technology)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		techs = append(techs, t)
	}
	sort.Strings(techs)
	if len(techs) == 0 {
		return "A definir"
	}
	if len(techs) > 8 {
		techs = techs[:8]
	}
	return strings.Join(techs, " / ")
}

// SyncNodeStatus reflete no canvas o progresso das tarefas (RF022).
// Devolve true se alguma posição de status mudou.
func SyncNodeStatus(d *model.Diagram, board *model.TaskBoard) bool {
	byNode := map[string][]model.Task{}
	for _, t := range board.Tasks {
		if t.ComponentID != "" {
			byNode[t.ComponentID] = append(byNode[t.ComponentID], t)
		}
	}
	changed := false
	for i := range d.Nodes {
		tasks := byNode[d.Nodes[i].ID]
		if len(tasks) == 0 {
			continue
		}
		status := model.StatusCompleted
		anyStarted := false
		for _, t := range tasks {
			switch t.Status {
			case model.StatusBlocked:
				status = model.StatusBlocked
			case model.StatusInProgress:
				anyStarted = true
				if status != model.StatusBlocked {
					status = model.StatusInProgress
				}
			case model.StatusPending:
				if status == model.StatusCompleted {
					status = model.StatusPending
				}
			case model.StatusCompleted:
				anyStarted = true
			}
		}
		if status == model.StatusPending && anyStarted {
			status = model.StatusInProgress
		}
		if d.Nodes[i].Data.Status != status {
			d.Nodes[i].Data.Status = status
			changed = true
		}
	}
	return changed
}

// ---------------------------------------------------------------------------
// Renderização do docs/ai-prd.md
// ---------------------------------------------------------------------------

func render(in Input, opts Options, stack string, board *model.TaskBoard, invariants []string) string {
	var b strings.Builder
	projectName := "Projeto"
	if in.Manifest != nil && in.Manifest.ProjectName != "" {
		projectName = in.Manifest.ProjectName
	}

	fmt.Fprintf(&b, "# AI MASTER IMPLEMENTATION SPECIFICATION — %s\n\n", projectName)
	fmt.Fprintf(&b, "> Auto-gerado pelo ArchCode Studio | Hash de Integridade: `{{HASH}}` | Total Tasks: %d | Gerado em: %s\n>\n",
		len(board.Tasks), time.Now().Format("2006-01-02 15:04"))
	b.WriteString("> **Este documento é o blueprint mestre de codificação.** Não reinterprete a arquitetura:\n")
	b.WriteString("> implemente exatamente o que está aqui, na ordem indicada, e marque cada tarefa concluída\n")
	b.WriteString("> com a ferramenta MCP `mark_task_status`. Se algo estiver ambíguo, use `get_system_context`\n")
	b.WriteString("> antes de assumir qualquer coisa.\n\n")

	if in.Manifest != nil && in.Manifest.Description != "" {
		fmt.Fprintf(&b, "%s\n\n", in.Manifest.Description)
	}

	// 1. Stack e invariantes
	b.WriteString("## 1. TECH STACK & SYSTEM INVARIANTS\n\n")
	fmt.Fprintf(&b, "- **Stack alvo:** %s\n", stack)
	fmt.Fprintf(&b, "- **Componentes:** %d | **Integrações:** %d | **Casos de uso:** %d | **Endpoints:** %d\n\n",
		countNonGroup(in.Diagram), len(in.Diagram.Edges), len(in.UseCases), len(in.Endpoints.Endpoints))
	for i, inv := range invariants {
		fmt.Fprintf(&b, "- **Invariant %d:** %s\n", i+1, inv)
	}
	b.WriteString("\n")

	// 2. Topologia
	b.WriteString("## 2. ARCHITECTURE TOPOLOGY\n\n")
	b.WriteString("```mermaid\n")
	b.WriteString(strings.TrimRight(mermaid.Export(in.Diagram), "\n"))
	b.WriteString("\n```\n\n")

	if opts.detailed() && countNonGroup(in.Diagram) > 0 {
		b.WriteString("| Componente | Tipo | Tier | Tecnologia | Responsabilidade |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
		nodes := sortedNodes(in.Diagram)
		for _, n := range nodes {
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s |\n",
				n.Data.Label, n.Type, layout.ResolveTier(n),
				orDash(n.Data.Technology), orDash(oneLine(n.Data.Description)))
		}
		b.WriteString("\n")
	}

	if len(in.Diagram.Edges) > 0 && opts.detailed() {
		b.WriteString("### 2.1 Fluxos de Comunicação\n\n")
		b.WriteString("| Origem | Destino | Protocolo | Porta | Segurança | Descrição |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- |\n")
		edges := append([]model.Edge(nil), in.Diagram.Edges...)
		sort.SliceStable(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
		for _, e := range edges {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
				labelOf(in.Diagram, e.Source), labelOf(in.Diagram, e.Target),
				orDash(e.Data.Protocol), portStr(e.Data.Port), orDash(e.Data.Security),
				orDash(oneLine(e.Data.Description)))
		}
		b.WriteString("\n")
	}

	// 3. Contratos de API
	b.WriteString("## 3. API CONTRACTS\n\n")
	if len(in.Endpoints.Endpoints) == 0 {
		b.WriteString("_Nenhum contrato declarado. Ao criar rotas HTTP, registre-as em `api/endpoints.yaml`._\n\n")
	} else {
		fmt.Fprintf(&b, "Base URL: `%s`\n\n", orDash(in.Endpoints.BaseURL))
		b.WriteString("| Método | Rota | Auth | Origem → Destino | Resumo |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- |\n")
		for _, e := range in.Endpoints.Endpoints {
			fmt.Fprintf(&b, "| `%s` | `%s` | %s | %s → %s | %s |\n",
				strings.ToUpper(e.Method), e.Path, orDash(e.Auth),
				labelOf(in.Diagram, e.Source), labelOf(in.Diagram, e.Target), orDash(e.Summary))
		}
		b.WriteString("\n")
		if opts.detailed() {
			for _, e := range in.Endpoints.Endpoints {
				if e.Request == "" && e.Response == "" {
					continue
				}
				fmt.Fprintf(&b, "#### `%s %s`\n\n", strings.ToUpper(e.Method), e.Path)
				if e.Request != "" {
					fmt.Fprintf(&b, "- **Request:** `%s`\n", oneLine(e.Request))
				}
				if e.Response != "" {
					fmt.Fprintf(&b, "- **Response:** `%s`\n", oneLine(e.Response))
				}
				if len(e.StatusCodes) > 0 {
					fmt.Fprintf(&b, "- **Status:** %s\n", intsToStr(e.StatusCodes))
				}
				b.WriteString("\n")
			}
		}
	}

	// 4. Sequência topológica
	b.WriteString("## 4. TOPOLOGICAL IMPLEMENTATION SEQUENCE\n\n")
	b.WriteString("Implemente estritamente nesta ordem. Uma tarefa só pode começar quando todas as suas dependências estiverem `completed`.\n\n")
	if len(board.Tasks) == 0 {
		b.WriteString("_Nenhum componente modelado ainda._\n\n")
	}
	for _, t := range board.Tasks {
		checkbox := " "
		switch t.Status {
		case model.StatusCompleted:
			checkbox = "x"
		case model.StatusInProgress:
			checkbox = "~"
		case model.StatusBlocked:
			checkbox = "!"
		}
		label := tierLabel[t.Tier]
		if label == "" {
			label = capitalize(t.Tier)
		}
		fmt.Fprintf(&b, "- [%s] **%s (%s):** %s\n", checkbox, t.ID, label, t.Title)
		if len(t.Dependencies) == 0 {
			b.WriteString("  - *Dependências:* Nenhuma\n")
		} else {
			fmt.Fprintf(&b, "  - *Dependências:* %s\n", strings.Join(t.Dependencies, ", "))
		}
		if len(t.Requirements) > 0 {
			fmt.Fprintf(&b, "  - *Requisitos:* %s\n", strings.Join(t.Requirements, ", "))
		}
		if len(t.UseCases) > 0 {
			fmt.Fprintf(&b, "  - *Casos de uso:* %s\n", strings.Join(t.UseCases, ", "))
		}
		if len(t.Endpoints) > 0 {
			fmt.Fprintf(&b, "  - *Contratos:* `%s`\n", strings.Join(t.Endpoints, "`, `"))
		}
		for _, a := range t.Acceptance {
			fmt.Fprintf(&b, "  - *Critério de Aceite:* %s\n", a)
		}
		if t.Notes != "" {
			fmt.Fprintf(&b, "  - *Notas do agente:* %s\n", oneLine(t.Notes))
		}
	}
	b.WriteString("\n")

	// 5. Casos de uso
	if len(in.UseCases) > 0 {
		b.WriteString("## 5. USE CASE ACCEPTANCE CRITERIA\n\n")
		for _, uc := range in.UseCases {
			fmt.Fprintf(&b, "### %s — %s\n\n", uc.Code, uc.Name)
			if len(uc.Actors) > 0 {
				fmt.Fprintf(&b, "- **Atores:** %s\n", strings.Join(uc.Actors, ", "))
			}
			if len(uc.Components) > 0 {
				fmt.Fprintf(&b, "- **Componentes:** %s\n", strings.Join(uc.Components, ", "))
			}
			b.WriteString("\n")
			if opts.detailed() && len(uc.MainFlow) > 0 {
				b.WriteString("**Fluxo Principal:**\n\n")
				for i, step := range uc.MainFlow {
					fmt.Fprintf(&b, "%d. %s\n", i+1, step)
				}
				b.WriteString("\n")
			}
			for _, gwt := range givenWhenThen(uc) {
				fmt.Fprintf(&b, "- %s\n", gwt)
			}
			if opts.detailed() && len(uc.BusinessRules) > 0 {
				b.WriteString("\n**Regras de Negócio:**\n\n")
				for _, r := range uc.BusinessRules {
					fmt.Fprintf(&b, "- %s\n", r)
				}
			}
			b.WriteString("\n")
		}
	}

	// 6. Requisitos
	b.WriteString("## 6. REQUIREMENTS TRACEABILITY\n\n")
	if len(in.Requirements.Requirements) == 0 {
		b.WriteString("_Nenhum requisito cadastrado em `docs/requisitos.md`._\n\n")
	} else {
		b.WriteString("| ID | Tipo | Título | Prioridade | Componentes | Tarefas |\n")
		b.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- |\n")
		for _, r := range in.Requirements.Requirements {
			tasks := []string{}
			for _, t := range board.Tasks {
				if containsStr(t.Requirements, r.ID) {
					tasks = append(tasks, t.ID)
				}
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s | %s | %s |\n",
				r.ID, r.Type, r.Title, orDash(r.Priority),
				orDash(strings.Join(r.Components, ", ")), orDash(strings.Join(tasks, ", ")))
		}
		b.WriteString("\n")
	}

	// 7. ADRs
	if len(in.ADRs) > 0 {
		b.WriteString("## 7. ARCHITECTURE DECISIONS (ADR)\n\n")
		for _, adr := range in.ADRs {
			fmt.Fprintf(&b, "- **%s — %s** (%s): %s\n", adr.ID, adr.Title, orDash(adr.Status), oneLine(firstSentence(adr.Decision)))
		}
		b.WriteString("\n")
	}

	// 8. Protocolo do agente
	b.WriteString("## 8. AGENT EXECUTION PROTOCOL\n\n")
	b.WriteString("1. Chame `get_implementation_tasks` com `status: \"pending\"` e pegue a primeira tarefa cujas dependências já estejam `completed`.\n")
	b.WriteString("2. Chame `mark_task_status` com `in_progress` antes de escrever código.\n")
	b.WriteString("3. Implemente somente o escopo daquela tarefa, respeitando todos os invariantes da seção 1.\n")
	b.WriteString("4. Execute os testes que provam cada *Critério de Aceite* da tarefa.\n")
	b.WriteString("5. Chame `mark_task_status` com `completed` e uma nota curta descrevendo a evidência (comando de teste e resultado).\n")
	b.WriteString("6. Se a arquitetura precisar mudar, use `add_architecture_node` / `connect_nodes` / `upsert_requirement` e regenere este documento com `generate_ai_prd`. Nunca edite `.arch/diagrams/macro.json` à mão.\n\n")
	b.WriteString("---\n\n")
	b.WriteString("> Gerado pelo ArchCode Studio. Regenerar este arquivo preserva o status já registrado das tarefas.\n")

	return b.String()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func countNonGroup(d *model.Diagram) int {
	c := 0
	for _, n := range d.Nodes {
		if n.Type != "group" {
			c++
		}
	}
	return c
}

func sortedNodes(d *model.Diagram) []model.Node {
	nodes := []model.Node{}
	for _, n := range d.Nodes {
		if n.Type != "group" {
			nodes = append(nodes, n)
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		if rankOf(nodes[i]) != rankOf(nodes[j]) {
			return rankOf(nodes[i]) < rankOf(nodes[j])
		}
		return nodes[i].Data.Label < nodes[j].Data.Label
	})
	return nodes
}

func labelOf(d *model.Diagram, id string) string {
	if id == "" {
		return "-"
	}
	if n := d.NodeByID(id); n != nil {
		return n.Data.Label
	}
	return id
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "|", "/")
	return strings.TrimSpace(s)
}

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, ". "); idx > 0 {
		return s[:idx+1]
	}
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}

func portStr(p int) string {
	if p <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", p)
}

func intsToStr(v []int) string {
	parts := make([]string, 0, len(v))
	for _, i := range v {
		parts = append(parts, fmt.Sprintf("`%d`", i))
	}
	return strings.Join(parts, ", ")
}
