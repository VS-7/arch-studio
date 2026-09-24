package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/archcode/studio/internal/studio"
	"github.com/mark3labs/mcp-go/server"
)

// cmdMCP sobe o servidor MCP em stdio (Claude Code, Cursor, Antigravity).
func cmdMCP(args []string) error {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto (padrão: diretório atual)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := openProject(*dir)
	if err != nil {
		return err
	}

	// Em stdio, stdout pertence ao protocolo: logs vão para stderr.
	fmt.Fprintf(os.Stderr, "archcode-studio mcp %s — projeto: %s\n", version, s.Root())
	return server.ServeStdio(s.MCPServer())
}

func cmdMCPConfig(args []string) error {
	fs := flag.NewFlagSet("mcp-config", flag.ExitOnError)
	dir := fs.String("dir", "", "diretório do projeto")
	if err := fs.Parse(args); err != nil {
		return err
	}
	s, err := studio.Open(studio.Config{Dir: *dir, Version: version}, nil)
	if err != nil {
		return err
	}
	bin := studio.Executable()

	cfg := map[string]any{
		"mcpServers": map[string]any{
			"archcode-studio": map[string]any{
				"command": bin,
				"args":    []string{"mcp", "--dir", s.Root()},
			},
		},
	}
	data, _ := json.MarshalIndent(cfg, "", "  ")

	fmt.Printf("\nConfiguração MCP (stdio) — projeto %s\n\n", s.Root())
	fmt.Printf("Claude Code:\n  claude mcp add archcode-studio -- %s mcp --dir %s\n\n", bin, s.Root())
	fmt.Printf("Cursor / Antigravity / Windsurf (~/.cursor/mcp.json ou .mcp.json do projeto):\n%s\n\n", data)
	fmt.Printf("Transporte SSE (com `archcode-studio serve` rodando):\n  http://127.0.0.1:8765/mcp/sse\n\n")
	return nil
}
