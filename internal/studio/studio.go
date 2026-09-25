// Package studio monta o ArchCode Studio para um diretório de projeto: store,
// app, watcher e os adaptadores HTTP e MCP.
//
// É a única composição do sistema. O CLI (`serve`, `mcp`, relatórios) e o app
// desktop só escolhem o transporte — nenhum deles repete a montagem.
package studio

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/server"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/httpapi"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/mcpserver"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/store"
	"github.com/archcode/studio/internal/watcher"
)

// Config descreve como abrir um projeto.
type Config struct {
	// Dir é a raiz do projeto ("" = diretório atual).
	Dir     string
	Version string
	// RequireProject recusa diretórios sem .arch/manifest.yaml.
	RequireProject bool
	// Watch anuncia no barramento as edições feitas fora do Studio.
	Watch bool
}

// Studio é um projeto aberto, pronto para ser servido.
type Studio struct {
	App *app.App

	bus     *hub.Hub
	watcher *watcher.Watcher
	version string
}

// Open abre o projeto. O barramento fica do lado de fora para que o app
// desktop troque de projeto sem perder as assinaturas da janela; nil cria um
// barramento próprio (CLI e MCP stdio, que não têm a quem avisar).
func Open(cfg Config, bus *hub.Hub) (*Studio, error) {
	if bus == nil {
		bus = hub.New()
	}
	st, err := openStore(cfg.Dir)
	if err != nil {
		return nil, err
	}
	if cfg.RequireProject && !st.IsProject() {
		return nil, fmt.Errorf("%w\nDiretório: %s\nRode `archcode-studio init` para criar a estrutura", store.ErrNotAProject, st.Root())
	}
	s := &Studio{App: app.New(st, bus), bus: bus, version: cfg.Version}
	if cfg.Watch {
		w, err := watcher.New(st, s.App.ExternalChange)
		if err != nil {
			return nil, fmt.Errorf("falha ao iniciar o file watcher: %w", err)
		}
		if err := w.Start(); err != nil {
			_ = w.Close()
			return nil, err
		}
		s.watcher = w
	}
	return s, nil
}

// Init cria a estrutura canônica de um projeto no diretório e prepara o
// Módulo de Implementação: convenções, as skills sugeridas para o stack e o
// backlog gerado da arquitetura de exemplo.
func Init(dir string, opts project.Options) (*project.Result, error) {
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
	res, err := project.Init(st, opts)
	if err != nil {
		return nil, err
	}
	a := app.New(st, nil)
	if !a.HasConventions() {
		if _, err := a.InitConventions("", false, hub.SourceCLI); err == nil {
			res.Created = append(res.Created, store.FileConventions)
		}
	}
	if ov, err := a.Skills(); err == nil {
		if names, err := a.InstallSkills(ov.Suggested, false, hub.SourceCLI); err == nil {
			for _, n := range names {
				res.Created = append(res.Created, store.SkillPath(n))
			}
		}
	}
	if !opts.Empty {
		if sync, err := a.SyncBacklog(false, hub.SourceCLI); err == nil && sync.Counts["added"] > 0 {
			res.Created = append(res.Created, fmt.Sprintf("%s/ (%d itens no backlog)", store.DirPlan, sync.Counts["added"]))
		}
	}
	return res, nil
}

// openStore abre o projeto do diretório. Sem diretório, vale o atual ou,
// como no Git, o primeiro diretório acima dele com .arch/manifest.yaml — os
// hooks e o MCP podem rodar de uma subpasta do repositório.
func openStore(dir string) (*store.Store, error) {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		dir = FindRoot(cwd)
	}
	return store.New(dir)
}

// FindRoot devolve o primeiro diretório, de start para cima, que contém um
// projeto ArchCode Studio; sem nenhum, devolve o próprio start.
func FindRoot(start string) string {
	abs, err := filepath.Abs(start)
	if err != nil {
		return start
	}
	for d := abs; ; {
		if info, err := os.Stat(filepath.Join(d, store.FileManifest)); err == nil && !info.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return abs
		}
		d = parent
	}
}

// Root devolve o diretório do projeto.
func (s *Studio) Root() string { return s.App.Root() }

// Close encerra o watcher.
func (s *Studio) Close() error {
	if s.watcher != nil {
		return s.watcher.Close()
	}
	return nil
}

// MCPServer devolve o servidor MCP (ferramentas para agentes de IA).
func (s *Studio) MCPServer() *server.MCPServer {
	return mcpserver.New(mcpserver.Deps{App: s.App, Version: s.version})
}

// HandlerOptions configura a interface servida por HTTP.
type HandlerOptions struct {
	// Assets é o frontend compilado (nil = sem interface).
	Assets fs.FS
	// Host identifica quem abre a interface: "web" (navegador) ou "desktop".
	Host string
	// MCPBaseURL, quando preenchido, expõe o MCP via SSE em /mcp/ anunciando
	// este endereço público (ex.: http://127.0.0.1:8765).
	MCPBaseURL string
	// MCPCommand é o executável anunciado para o MCP em stdio ("" = o binário
	// em execução).
	MCPCommand string
}

// Handler monta a API HTTP, o WebSocket, o frontend e, opcionalmente, o MCP.
func (s *Studio) Handler(o HandlerOptions) http.Handler {
	opts := httpapi.Options{Version: s.version, Assets: o.Assets, Host: o.Host, MCPCommand: o.MCPCommand}
	if opts.MCPCommand == "" {
		opts.MCPCommand = Executable()
	}
	base := strings.TrimRight(o.MCPBaseURL, "/")
	if base != "" {
		opts.MCPSSEURL = base + "/mcp/sse"
	}
	srv := httpapi.New(s.App, s.bus, opts)
	if base != "" {
		srv.Mount("/mcp/", server.NewSSEServer(s.MCPServer(),
			server.WithStaticBasePath("/mcp"),
			server.WithSSEEndpoint("/sse"),
			server.WithMessageEndpoint("/message"),
			server.WithBaseURL(base),
		))
	}
	return srv.Handler()
}

// Executable devolve o caminho do binário em execução, para ser usado na
// configuração MCP dos agentes ("archcode-studio" quando rodando via go run).
func Executable() string {
	bin, err := os.Executable()
	if err != nil || strings.Contains(bin, "go-build") {
		return "archcode-studio"
	}
	return bin
}
