// Command archcode-studio é o executável único do ArchCode Studio.
//
// Subcomandos:
//
//	init        cria a estrutura .arch/, docs/ e api/ no diretório atual
//	serve       sobe a interface web, o watcher e o MCP via SSE
//	mcp         sobe o servidor MCP em stdio (Claude Code, Cursor, Antigravity)
//	prd         compila docs/ai-prd.md
//	estimate    imprime a estimativa de esforço e custo
//	validate    roda o linter de arquitetura
//	export      exporta svg, mermaid, openapi, proposta comercial ou documento de requisitos
//	mcp-config  imprime o trecho de configuração para clientes MCP
//	backlog, sprint, claim, release, resume, checkpoint, session, remember, recall
//	            Módulo de Implementação: planejamento, execução e memória
//	commit, pr, conventions, hooks, hook, git lint, changelog, reconcile, doctor, merge-driver
//	            convenções de Git orientadas a sprint
//	skills, agent, check
//	            skills do projeto e configuração dos agentes de IA
//	version     mostra a versão
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime"

	"github.com/archcode/studio/internal/studio"
)

// version é sobrescrita no build via -ldflags "-X main.version=…".
var version = "1.3.0"

const usage = `ArchCode Studio %s — arquitetura de software como código, local-first e nativa para IAs.

Uso:
  archcode-studio <comando> [opções]

Comandos:
  init          Cria a estrutura canônica do projeto (.arch/, docs/, api/)
  serve         Sobe a interface web + WebSocket + MCP via SSE
  mcp           Sobe o servidor MCP em stdio (para Claude Code, Cursor, Antigravity)
  prd           Compila docs/ai-prd.md com a ordem topológica de implementação
  estimate      Calcula esforço, custo e prazo do projeto
  validate      Executa o linter de regras arquiteturais
  export        Exporta svg | mermaid | openapi | proposal | requirements
  mcp-config    Imprime a configuração pronta para clientes MCP
  version       Mostra a versão

Planejamento e execução:
  backlog       sync | list | show | add | set | archive — backlog gerado da arquitetura
  sprint        status | list | new | plan | start | close
  claim         Reserva uma tarefa (cria a branch da convenção)
  release       Libera a reserva de uma tarefa
  resume        Onde parou: sprint, suas tarefas, Git, próxima tarefa, skills
  checkpoint    Grava o checkpoint de retomada da tarefa
  complete      Conclui a tarefa com os checks das skills
  session       log | list — diário das sessões de trabalho
  remember      Grava uma memória do projeto; recall busca

Git e convenções:
  commit        Commit na convenção a partir da tarefa (git add antes)
  pr            Título e corpo do PR; abre com o gh após confirmação
  conventions   init | show | apply — .arch/conventions.yaml
  hooks         install | uninstall — commit-msg e pre-push
  git lint      Valida commits, branch e título de PR (CI)
  changelog     CHANGELOG.md por sprint
  reconcile     Move as tarefas conforme o histórico do Git
  doctor        Conflitos e inconsistências do backlog

Skills e agentes:
  skills        list | add | update | remove | enable | disable | sync
  agent setup   claude | cursor — MCP, hooks de retomada e skills
  check         Roda os comandos dos checks das skills ativas

Exemplos:
  archcode-studio init --name "E-Commerce Enterprise"
  archcode-studio sprint plan --goal "Autenticação" --apply && archcode-studio sprint start 1
  archcode-studio claim && archcode-studio commit
  archcode-studio serve --port 8765
  archcode-studio prd --stack "Go / React / PostgreSQL"
  archcode-studio export svg --mode executive --out arquitetura.svg
  archcode-studio export requirements    # docs/documento-de-requisitos.md + figuras

Use "archcode-studio <comando> -h" para ver as opções de cada comando.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Printf(usage, version)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "serve", "start":
		err = cmdServe(args)
	case "mcp":
		err = cmdMCP(args)
	case "prd", "ai-prd":
		err = cmdPRD(args)
	case "estimate", "pricing":
		err = cmdEstimate(args)
	case "validate", "lint":
		err = cmdValidate(args)
	case "export":
		err = cmdExport(args)
	case "mcp-config":
		err = cmdMCPConfig(args)
	case "backlog":
		err = cmdBacklog(args)
	case "sprint":
		err = cmdSprint(args)
	case "claim":
		err = cmdClaim(args)
	case "release":
		err = cmdRelease(args)
	case "resume":
		err = cmdResume(args)
	case "checkpoint":
		err = cmdCheckpoint(args)
	case "complete", "done":
		err = cmdComplete(args)
	case "session":
		err = cmdSession(args)
	case "remember":
		err = cmdRemember(args)
	case "recall":
		err = cmdRecall(args)
	case "commit":
		err = cmdCommit(args)
	case "pr":
		err = cmdPR(args)
	case "conventions":
		err = cmdConventions(args)
	case "hooks":
		err = cmdHooks(args)
	case "hook":
		err = cmdHook(args)
	case "git":
		err = cmdGit(args)
	case "changelog":
		err = cmdChangelog(args)
	case "reconcile":
		err = cmdReconcile(args)
	case "doctor":
		err = cmdDoctor(args)
	case "merge-driver":
		err = cmdMergeDriver(args)
	case "skills":
		err = cmdSkills(args)
	case "agent":
		err = cmdAgent(args)
	case "check":
		err = cmdCheck(args)
	case "version", "-v", "--version":
		fmt.Printf("archcode-studio %s (%s/%s, %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, "comando desconhecido: %q\n\n", cmd)
		fmt.Printf(usage, version)
		os.Exit(1)
	}

	var code exitCode
	if errors.As(err, &code) {
		os.Exit(int(code))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

// exitCode encerra o processo com o código informado, sem mensagem de erro
// (ex.: `validate` com problemas encontrados). Devolvido em vez de os.Exit
// para que os defers dos comandos rodem.
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("código de saída %d", int(c)) }

// openProject abre o projeto do diretório (ou do diretório atual) para os
// comandos de linha de comando, que não publicam eventos.
func openProject(dir string) (*studio.Studio, error) {
	return studio.Open(studio.Config{Dir: dir, Version: version, RequireProject: true}, nil)
}
