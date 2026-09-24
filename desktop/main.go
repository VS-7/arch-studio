// Command archcode-desktop é o ArchCode Studio como aplicativo desktop (Wails v3).
//
// A janela usa o mesmo frontend e a mesma API HTTP do `archcode-studio serve`,
// entregues pelo asset server do Wails em vez de uma porta TCP. Só mudam o
// transporte dos eventos em tempo real (eventos do Wails no lugar do WebSocket)
// e os recursos nativos: abrir e criar projetos, salvar arquivos e imprimir.
//
// Uso:
//
//	archcode-desktop [pasta-do-projeto]     abre a janela (padrão: último projeto)
//	archcode-desktop mcp --dir <pasta>      servidor MCP em stdio, sem janela
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/mark3labs/mcp-go/server"
	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/archcode/studio/desktop/internal/workspace"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/studio"
	"github.com/archcode/studio/internal/webui"
)

// version é sobrescrita no build via -ldflags "-X main.version=…".
var version = "1.2.0"

//go:embed appicon.png
var appIcon []byte

const (
	// eventName leva ao frontend os eventos do barramento (mesmo formato do WebSocket).
	eventName = "archcode:event"
	// windowName identifica a janela principal.
	windowName = "main"
	// appID identifica o aplicativo no sistema (instância única, .desktop no Linux).
	appID = "io.archcode.studio"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "mcp":
			if err := runMCP(os.Args[2:]); err != nil {
				fmt.Fprintf(os.Stderr, "erro: %v\n", err)
				os.Exit(1)
			}
			return
		case "version", "--version", "-v":
			fmt.Printf("archcode-desktop %s (%s/%s)\n", version, runtime.GOOS, runtime.GOARCH)
			return
		}
	}
	if err := run(projectArg(os.Args[1:], "")); err != nil {
		log.Fatal(err)
	}
}

// runMCP expõe o projeto a agentes de IA em stdio, como `archcode-studio mcp`.
// As gravações chegam à janela aberta pelo watcher, como qualquer edição externa.
func runMCP(args []string) error {
	flags := flag.NewFlagSet("mcp", flag.ExitOnError)
	dir := flags.String("dir", "", "diretório do projeto (padrão: diretório atual)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	s, err := studio.Open(studio.Config{Dir: *dir, Version: version, RequireProject: true}, nil)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "archcode-desktop mcp %s — projeto: %s\n", version, s.Root())
	return server.ServeStdio(s.MCPServer())
}

func run(initialDir string) error {
	bus := hub.New()
	recent := workspace.LoadRecent()
	ws := workspace.New(version, bus, frontendAssets(), recent)
	prints := newPrintJobs()
	desktop := &Desktop{ws: ws, prints: prints}

	// A janela recebe o frontend e a API do projeto aberto; /__print/ serve as
	// páginas de impressão (ver print.go).
	assets := http.NewServeMux()
	assets.Handle(printPrefix, prints)
	assets.Handle("/", ws)

	app := application.New(application.Options{
		Name:        "ArchCode Studio",
		Description: "Arquitetura de software como código, local-first e nativa para IAs",
		Icon:        appIcon,
		// O application id identifica a janela no Linux (Wayland/X11) e dá nome ao
		// .desktop instalado pelos pacotes (build/linux/io.archcode.studio.desktop).
		Linux:    application.LinuxOptions{ApplicationID: appID},
		Services: []application.Service{application.NewService(desktop)},
		Assets:   application.AssetOptions{Handler: assets},
		Mac:      application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: appID,
			// Abrir o app de novo (ou abrir uma pasta por ele) foca a janela existente.
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if dir := projectArg(data.Args, data.WorkingDir); dir != "" {
					openAndReload(ws, dir)
				}
				if w, ok := application.Get().Window.GetByName(windowName); ok {
					w.Focus()
				}
			},
		},
		OnShutdown: func() { _ = ws.Close() },
	})

	// Projeto inicial: a pasta passada na linha de comando ou o último aberto.
	candidates := []string{initialDir}
	for _, p := range recent.List() {
		candidates = append(candidates, p.Root)
	}
	for _, dir := range candidates {
		if dir == "" {
			continue
		}
		if _, err := ws.Open(dir); err == nil {
			break
		}
	}

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      windowName,
		Title:     windowTitle(ws.Current()),
		Width:     1440,
		Height:    900,
		MinWidth:  960,
		MinHeight: 600,
		URL:       "/",
		Linux:     application.LinuxWindow{Icon: appIcon},
	})
	if runtime.GOOS == "darwin" {
		app.Menu.Set(macMenu(ws))
	}

	go forwardEvents(app, bus)
	return app.Run()
}

// forwardEvents entrega à janela cada evento do barramento. Se o hub
// desconectar o assinante por lentidão, assina de novo.
func forwardEvents(app *application.App, bus *hub.Hub) {
	for {
		events, cancel := bus.Subscribe(256)
		for ev := range events {
			app.Event.Emit(eventName, ev)
		}
		cancel()
	}
}

// macMenu é o menu nativo do macOS: sem o menu Editar, Cmd+C/Cmd+V não
// funcionam em campos de texto. Em Linux e Windows o menu da própria interface
// (Arquivo, Editar, Exibir…) basta.
func macMenu(ws *workspace.Workspace) *application.Menu {
	menu := application.NewMenu()
	menu.AddRole(application.AppMenu)
	file := menu.AddSubmenu("Arquivo")
	file.Add("Abrir projeto…").SetAccelerator("CmdOrCtrl+O").OnClick(func(*application.Context) {
		dir, err := application.Get().Dialog.OpenFile().SetTitle("Abrir projeto").
			CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
		if err == nil && dir != "" {
			openAndReload(ws, dir)
		}
	})
	file.Add("Fechar projeto").OnClick(func(*application.Context) {
		_ = ws.Close()
		setWindowTitle(nil)
		reloadWindow()
	})
	menu.AddRole(application.EditMenu)
	menu.AddRole(application.WindowMenu)
	return menu
}

// openAndReload abre o projeto por fora da interface (menu nativo, segunda
// instância) e recarrega a janela para refletir a troca.
func openAndReload(ws *workspace.Workspace, dir string) {
	info, err := ws.Open(dir)
	if err != nil {
		application.Get().Dialog.Error().SetTitle("Não foi possível abrir o projeto").SetMessage(err.Error()).Show()
		return
	}
	setWindowTitle(info)
	reloadWindow()
}

func reloadWindow() {
	if w, ok := application.Get().Window.GetByName(windowName); ok {
		w.Reload()
	}
}

func windowTitle(info *workspace.ProjectInfo) string {
	if info == nil {
		return "ArchCode Studio"
	}
	return info.Name + " — ArchCode Studio"
}

func setWindowTitle(info *workspace.ProjectInfo) {
	if w, ok := application.Get().Window.GetByName(windowName); ok {
		w.SetTitle(windowTitle(info))
	}
}

// projectArg devolve a primeira pasta existente entre os argumentos; caminhos
// relativos partem de base (o diretório de quem abriu o app).
func projectArg(args []string, base string) string {
	for _, a := range args {
		if !filepath.IsAbs(a) && base != "" {
			a = filepath.Join(base, a)
		}
		if info, err := os.Stat(a); err == nil && info.IsDir() {
			return a
		}
	}
	return ""
}

// frontendAssets devolve o frontend embutido no pacote webui (o mesmo do CLI).
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
