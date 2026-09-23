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
//	export      exporta svg, mermaid, openapi ou proposta comercial
//	mcp-config  imprime o trecho de configuração para clientes MCP
//	version     mostra a versão
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/httpapi"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/lint"
	"github.com/archcode/studio/internal/mcpserver"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
	"github.com/archcode/studio/internal/svgexport"
	"github.com/archcode/studio/internal/watcher"
	"github.com/archcode/studio/internal/webui"
)

// version é sobrescrita no build via -ldflags "-X main.version=…".
var version = "1.0.0"

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
  export        Exporta svg | mermaid | openapi | proposal
  mcp-config    Imprime a configuração pronta para clientes MCP
  version       Mostra a versão

Exemplos:
  archcode-studio init --name "E-Commerce Enterprise"
  archcode-studio serve --port 8765
  archcode-studio prd --stack "Go / React / PostgreSQL"
  archcode-studio export svg --mode executive --out arquitetura.svg

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
	case "version", "-v", "--version":
		fmt.Printf("archcode-studio %s (%s/%s, %s)\n", version, runtime.GOOS, runtime.GOARCH, runtime.Version())
	case "help", "-h", "--help":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, "comando desconhecido: %q\n\n", cmd)
		fmt.Printf(usage, version)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "erro: %v\n", err)
		os.Exit(1)
	}
}

// openStore resolve o diretório do projeto e valida a existência do manifest.
func openStore(dir string, requireProject bool) (*store.Store, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = cwd
	}
	st, err := store.New(dir)
	if err != nil {
		return nil, err
	}
	if requireProject && !st.IsProject() {
		return nil, fmt.Errorf("%w\nDiretório: %s\nRode `archcode-studio init` para criar a estrutura", store.ErrNotAProject, st.Root())
	}
	return st, nil
}

// ---------------------------------------------------------------------------
// init
// ---------------------------------------------------------------------------

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto (padrão: diretório atual)")
	name := fs.String("name", "", "nome do projeto")
	desc := fs.String("description", "", "descrição curta do projeto")
	author := fs.String("author", "", "autor principal")
	currency := fs.String("currency", "BRL", "moeda da precificação (BRL, USD, EUR)")
	empty := fs.Bool("empty", false, "não criar a arquitetura de exemplo")
	force := fs.Bool("force", false, "sobrescrever o manifest de um projeto existente")
	if err := fs.Parse(args); err != nil {
		return err
	}

	st, err := openStore(*dir, false)
	if err != nil {
		return err
	}
	res, err := project.Init(st, project.Options{
		ProjectName: *name, Description: *desc, Author: *author,
		Currency: *currency, Empty: *empty, Force: *force,
	})
	if err != nil {
		return err
	}

	fmt.Printf("✓ Projeto ArchCode Studio criado em %s\n\n", res.Root)
	for _, f := range res.Created {
		fmt.Printf("  criado   %s\n", f)
	}
	for _, f := range res.Skipped {
		fmt.Printf("  mantido  %s\n", f)
	}
	fmt.Printf("\nPróximos passos:\n")
	fmt.Printf("  1. archcode-studio serve          # abrir o canvas no browser\n")
	fmt.Printf("  2. archcode-studio mcp-config     # conectar seu agente de IA\n")
	fmt.Printf("  3. archcode-studio prd            # compilar o blueprint para IAs\n")
	return nil
}

// ---------------------------------------------------------------------------
// serve
// ---------------------------------------------------------------------------

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto (padrão: diretório atual)")
	host := fs.String("host", "127.0.0.1", "endereço de escuta (local-first por padrão)")
	port := fs.Int("port", 8765, "porta HTTP")
	open := fs.Bool("open", true, "abrir o navegador automaticamente")
	noMCP := fs.Bool("no-mcp", false, "não expor o transporte MCP via SSE")
	baseURL := fs.String("base-url", os.Getenv("ARCHCODE_BASE_URL"), "URL pública anunciada pelo MCP (ex.: atrás de proxy reverso)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *baseURL == "" {
		*baseURL = fmt.Sprintf("http://%s:%d", *host, *port)
	}

	start := time.Now()
	st, err := openStore(*dir, true)
	if err != nil {
		return err
	}

	hb := hub.New()
	application := app.New(st, hb)

	w, err := watcher.New(st, hb)
	if err != nil {
		return fmt.Errorf("falha ao iniciar o file watcher: %w", err)
	}
	if err := w.Start(); err != nil {
		return err
	}
	defer func() { _ = w.Close() }()

	srv := httpapi.New(application, hb, version)

	if !*noMCP {
		mcpSrv := mcpserver.New(mcpserver.Deps{App: application, Version: version})
		sse := server.NewSSEServer(mcpSrv,
			server.WithStaticBasePath("/mcp"),
			server.WithSSEEndpoint("/sse"),
			server.WithMessageEndpoint("/message"),
			server.WithBaseURL(strings.TrimRight(*baseURL, "/")),
		)
		srv.Mount("/mcp/", sse)
	}

	addr := net.JoinHostPort(*host, fmt.Sprintf("%d", *port))
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("não foi possível escutar em %s: %w\nTente outra porta com --port", addr, err)
	}

	url := fmt.Sprintf("http://%s", addr)
	manifest, _ := st.LoadManifest()
	projectName := "projeto"
	if manifest != nil {
		projectName = manifest.ProjectName
	}

	fmt.Printf("\n  ArchCode Studio %s\n", version)
	fmt.Printf("  ─────────────────────────────────────────────\n")
	fmt.Printf("  Projeto:    %s\n", projectName)
	fmt.Printf("  Diretório:  %s\n", st.Root())
	fmt.Printf("  Interface:  %s\n", url)
	if !*noMCP {
		fmt.Printf("  MCP (SSE):  %s/mcp/sse\n", url)
	}
	fmt.Printf("  MCP (stdio): archcode-studio mcp --dir %s\n", st.Root())
	if !webui.Built() {
		fmt.Printf("\n  ⚠ Frontend não embutido neste binário.\n")
		fmt.Printf("    Rode: npm --prefix web install && npm --prefix web run build && go build ./cmd/archcode-studio\n")
	}
	fmt.Printf("\n  Pronto em %s. Ctrl+C para encerrar.\n\n", time.Since(start).Round(time.Millisecond))

	if *open && webui.Built() {
		go openBrowser(url)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-sig:
		fmt.Printf("\n  Encerrando…\n")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(ctx)
	}
}

func openBrowser(url string) {
	time.Sleep(250 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// ---------------------------------------------------------------------------
// mcp (stdio)
// ---------------------------------------------------------------------------

func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto (padrão: diretório atual)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore(*dir, true)
	if err != nil {
		return err
	}

	// Em stdio, stdout pertence ao protocolo: logs vão para stderr.
	fmt.Fprintf(os.Stderr, "archcode-studio mcp %s — projeto: %s\n", version, st.Root())

	// Sem hub: eventos de UI não se aplicam ao transporte stdio isolado.
	application := app.New(st, nil)
	mcpSrv := mcpserver.New(mcpserver.Deps{App: application, Version: version})
	return server.ServeStdio(mcpSrv)
}

// ---------------------------------------------------------------------------
// prd
// ---------------------------------------------------------------------------

func cmdPRD(args []string) error {
	fs := flag.NewFlagSet("prd", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	stack := fs.String("stack", "", "stack alvo, ex.: \"Go / React / PostgreSQL\"")
	granularity := fs.String("granularity", "detailed", "detailed | summary")
	noTests := fs.Bool("no-tests", false, "não gerar critérios Given-When-Then nem a tarefa E2E")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore(*dir, true)
	if err != nil {
		return err
	}
	application := app.New(st, nil)
	res, err := application.GenerateAIPRD(prd.Options{
		TargetStack:          *stack,
		IncludeTestScenarios: !*noTests,
		Granularity:          *granularity,
	}, hub.SourceCLI)
	if err != nil {
		return err
	}
	fmt.Printf("✓ %s gerado\n", res.FilePath)
	fmt.Printf("  Tarefas: %d | Hash de integridade: %s | Progresso: %d%%\n",
		res.TotalTasks, res.Hash, res.Progress)
	fmt.Printf("  Espelho legível por máquina: %s\n", store.FileTasks)
	return nil
}

// ---------------------------------------------------------------------------
// estimate
// ---------------------------------------------------------------------------

func cmdEstimate(args []string) error {
	fs := flag.NewFlagSet("estimate", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	margin := fs.Float64("margin", -1, "margem de contingência (0.20 ou 20); padrão: pricing.yaml")
	asJSON := fs.Bool("json", false, "saída em JSON")
	detailed := fs.Bool("detailed", false, "mostrar de onde vem cada hora")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore(*dir, true)
	if err != nil {
		return err
	}
	est, err := app.New(st, nil).Estimate(*margin)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(est)
	}

	cur := est.Currency
	fmt.Printf("\n  Estimativa do projeto\n")
	fmt.Printf("  ─────────────────────────────────────────────\n")
	fmt.Printf("  Componentes          %10s\n", pricing.Hours(est.NodeHours))
	fmt.Printf("  Integrações/APIs     %10s\n", pricing.Hours(est.EdgeHours))
	fmt.Printf("  Casos de uso         %10s\n", pricing.Hours(est.UseCaseHours))
	fmt.Printf("  Subtotal             %10s\n", pricing.Hours(est.BaseHours))
	fmt.Printf("  Margem (%3.0f%%)        %10s\n", est.RiskMarginPercentage, pricing.Hours(est.MarginHours))
	fmt.Printf("  TOTAL                %10s\n\n", pricing.Hours(est.TotalHours))

	for _, r := range est.Roles {
		fmt.Printf("  %-18s %8s × %12s = %14s\n",
			r.Role, pricing.Hours(r.Hours), pricing.Money(cur, r.Rate), pricing.Money(cur, r.Subtotal))
	}
	fmt.Printf("\n  Desenvolvimento      %18s\n", pricing.Money(cur, est.PersonnelCost))
	fmt.Printf("  Impostos (%3.0f%%)      %18s\n", est.TaxPercentage, pricing.Money(cur, est.TaxAmount))
	fmt.Printf("  TOTAL DO PROJETO     %18s\n", pricing.Money(cur, est.TotalCost))
	if est.CloudMonthlyCost > 0 {
		fmt.Printf("  Infraestrutura       %18s/mês\n", pricing.Money(cur, est.CloudMonthlyCost))
	}
	fmt.Printf("  Prazo                %.0f dias úteis (~%.1f meses, equipe de %.0f)\n",
		est.WorkingDays, est.CalendarMonths, est.TeamSize)

	if *detailed {
		fmt.Printf("\n  Detalhamento\n  ─────────────────────────────────────────────\n")
		for _, it := range est.Items {
			flag := " "
			if it.Explicit {
				flag = "*"
			}
			fmt.Printf("  %s %-30s %-12s %8s\n", flag, truncate(it.Label, 30), it.Kind, pricing.Hours(it.Hours))
		}
		fmt.Printf("\n  * horas definidas explicitamente no modelo\n")
	}
	for _, warn := range est.Warnings {
		fmt.Printf("\n  ⚠ %s", warn)
	}
	fmt.Println()
	return nil
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ---------------------------------------------------------------------------
// validate
// ---------------------------------------------------------------------------

func cmdValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	asJSON := fs.Bool("json", false, "saída em JSON")
	strict := fs.Bool("strict", false, "sair com código 1 também em avisos")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore(*dir, true)
	if err != nil {
		return err
	}
	rep, err := app.New(st, nil).Validate()
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		icons := map[string]string{lint.SeverityError: "✗", lint.SeverityWarning: "▲", lint.SeverityInfo: "·"}
		fmt.Printf("\n  Validação da arquitetura — score %d/100\n", rep.Score)
		fmt.Printf("  ─────────────────────────────────────────────\n")
		if len(rep.Findings) == 0 {
			fmt.Printf("  ✓ Nenhum problema encontrado.\n\n")
		}
		for _, f := range rep.Findings {
			target := f.Target
			if target == "" {
				target = "arquitetura"
			}
			fmt.Printf("  %s [%s] %s\n      %s\n", icons[f.Severity], f.Rule, target, f.Message)
			if f.Fix != "" {
				fmt.Printf("      → %s\n", f.Fix)
			}
		}
		fmt.Printf("\n  %d erro(s), %d aviso(s), %d informativo(s)\n\n", rep.Errors, rep.Warnings, rep.Infos)
	}
	if rep.Errors > 0 || (*strict && rep.Warnings > 0) {
		os.Exit(1)
	}
	return nil
}

// ---------------------------------------------------------------------------
// export
// ---------------------------------------------------------------------------

func cmdExport(args []string) error {
	if len(args) == 0 {
		return errors.New("informe o formato: svg | mermaid | openapi | proposal")
	}
	format := args[0]
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	out := fs.String("out", "", "arquivo de saída (padrão: stdout, quando aplicável)")
	mode := fs.String("mode", "engineering", "svg: executive | engineering")
	theme := fs.String("theme", "light", "svg: light | dark")
	transparent := fs.Bool("transparent", false, "svg: fundo transparente")
	client := fs.String("client", "", "proposal: nome do cliente")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	st, err := openStore(*dir, true)
	if err != nil {
		return err
	}
	application := app.New(st, nil)

	switch format {
	case "svg":
		snap, err := st.Snapshot()
		if err != nil {
			return err
		}
		svg := svgexport.Render(snap.Diagram, svgexport.Options{
			Executive:   *mode == "executive",
			Dark:        *theme == "dark",
			Transparent: *transparent,
			Title:       snap.Manifest.ProjectName,
			Subtitle:    snap.Manifest.Description,
		})
		return writeOut(*out, svg, fmt.Sprintf("%s-arquitetura.svg", model.Slugify(snap.Manifest.ProjectName)))

	case "mermaid", "mmd":
		data, err := st.ReadFile(store.FileMacroMmd)
		if err != nil {
			return err
		}
		return writeOut(*out, string(data), "")

	case "openapi":
		path, err := application.ExportOpenAPI(hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s gerado\n", path)
		return nil

	case "proposal", "proposta":
		res, err := application.GenerateProposal(app.ProposalOptions{
			ClientName: *client, IncludeDiagram: true, IncludeCloud: true,
		}, hub.SourceCLI)
		if err != nil {
			return err
		}
		fmt.Printf("✓ %s gerado — total: %s\n", res.FilePath, pricing.Money(res.Currency, res.TotalCost))
		return nil

	default:
		return fmt.Errorf("formato desconhecido: %q (use svg, mermaid, openapi ou proposal)", format)
	}
}

func writeOut(out, content, defaultName string) error {
	if out == "" {
		if defaultName == "" {
			fmt.Print(content)
			return nil
		}
		out = defaultName
	}
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return err
	}
	abs, _ := filepath.Abs(out)
	fmt.Printf("✓ %s\n", abs)
	return nil
}

// ---------------------------------------------------------------------------
// mcp-config
// ---------------------------------------------------------------------------

func cmdMCPConfig(args []string) error {
	fs := flag.NewFlagSet("mcp-config", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore(*dir, false)
	if err != nil {
		return err
	}
	bin, err := os.Executable()
	if err != nil || strings.Contains(bin, "go-build") {
		bin = "archcode-studio"
	}

	cfg := map[string]any{
		"mcpServers": map[string]any{
			"archcode-studio": map[string]any{
				"command": bin,
				"args":    []string{"mcp", "--dir", st.Root()},
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")

	fmt.Printf("\nConfiguração MCP (stdio) — projeto %s\n\n", st.Root())
	fmt.Printf("Claude Code:\n  claude mcp add archcode-studio -- %s mcp --dir %s\n\n", bin, st.Root())
	fmt.Printf("Cursor / Antigravity / Windsurf (~/.cursor/mcp.json ou .mcp.json do projeto):\n%s\n\n", data)
	fmt.Printf("Transporte SSE (com `archcode-studio serve` rodando):\n  http://127.0.0.1:8765/mcp/sse\n\n")
	return nil
}
