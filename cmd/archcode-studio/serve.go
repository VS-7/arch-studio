package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/studio"
	"github.com/archcode/studio/internal/webui"
)

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
	addr := net.JoinHostPort(*host, fmt.Sprintf("%d", *port))
	url := "http://" + addr
	if *baseURL == "" {
		*baseURL = url
	}

	start := time.Now()
	s, err := studio.Open(studio.Config{Dir: *dir, Version: version, RequireProject: true, Watch: true}, hub.New())
	if err != nil {
		return err
	}
	defer func() { _ = s.Close() }()

	opts := studio.HandlerOptions{Assets: frontendAssets(), Host: "web"}
	if !*noMCP {
		opts.MCPBaseURL = *baseURL
	}
	httpServer := &http.Server{Addr: addr, Handler: s.Handler(opts), ReadHeaderTimeout: 10 * time.Second}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("não foi possível escutar em %s: %w\nTente outra porta com --port", addr, err)
	}

	projectName := "projeto"
	if manifest, err := s.App.Manifest(); err == nil {
		projectName = manifest.ProjectName
	}
	fmt.Printf("\n  ArchCode Studio %s\n", version)
	fmt.Printf("  ─────────────────────────────────────────────\n")
	fmt.Printf("  Projeto:    %s\n", projectName)
	fmt.Printf("  Diretório:  %s\n", s.Root())
	fmt.Printf("  Interface:  %s\n", url)
	if !*noMCP {
		fmt.Printf("  MCP (SSE):  %s/mcp/sse\n", url)
	}
	fmt.Printf("  MCP (stdio): archcode-studio mcp --dir %s\n", s.Root())
	if opts.Assets == nil {
		fmt.Printf("\n  ⚠ Frontend não embutido neste binário.\n")
		fmt.Printf("    Rode: npm --prefix web install && npm --prefix web run build && go build ./cmd/archcode-studio\n")
	}
	fmt.Printf("\n  Pronto em %s. Ctrl+C para encerrar.\n\n", time.Since(start).Round(time.Millisecond))

	if *open && opts.Assets != nil {
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

// frontendAssets devolve o frontend embutido, ou nil se o binário foi
// compilado sem ele.
func frontendAssets() fs.FS {
	if !webui.Built() {
		return nil
	}
	assets, err := webui.FS()
	if err != nil {
		return nil
	}
	return assets
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
