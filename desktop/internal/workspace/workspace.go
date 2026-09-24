// Package workspace é a sessão de projeto do app desktop: qual pasta está
// aberta, os projetos recentes e o http.Handler que a janela usa. Não depende
// do Wails, para ser testado sem interface gráfica.
package workspace

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/archcode/studio/internal/httpapi"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/project"
	"github.com/archcode/studio/internal/studio"
)

// Workspace é o projeto aberto na janela. Ele é o http.Handler do asset server
// do Wails: delega para o handler do projeto atual (a mesma API e o mesmo
// frontend do `archcode-studio serve`) ou, sem projeto, para a tela inicial.
// Trocar de projeto só troca o handler; a janela e o barramento continuam.
type Workspace struct {
	version string
	bus     *hub.Hub
	assets  fs.FS
	recent  *RecentList

	mu      sync.RWMutex
	current *studio.Studio
	info    *ProjectInfo
	handler http.Handler
}

// ProjectInfo identifica o projeto aberto.
type ProjectInfo struct {
	Root string `json:"root"`
	Name string `json:"name"`
}

// New cria a sessão sem projeto aberto. assets é o frontend compilado.
func New(version string, bus *hub.Hub, assets fs.FS, recent *RecentList) *Workspace {
	w := &Workspace{version: version, bus: bus, assets: assets, recent: recent}
	w.handler = w.emptyHandler()
	return w
}

// Recent devolve a lista de projetos recentes.
func (w *Workspace) Recent() *RecentList { return w.recent }

func (w *Workspace) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.mu.RLock()
	h := w.handler
	w.mu.RUnlock()
	h.ServeHTTP(rw, r)
}

// Current devolve o projeto aberto (nil = nenhum).
func (w *Workspace) Current() *ProjectInfo {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.info
}

// Open abre o projeto do diretório, fechando o anterior.
func (w *Workspace) Open(dir string) (*ProjectInfo, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	s, err := studio.Open(studio.Config{Dir: abs, Version: w.version, RequireProject: true, Watch: true}, w.bus)
	if err != nil {
		return nil, err
	}
	info := &ProjectInfo{Root: s.Root(), Name: filepath.Base(s.Root())}
	if m, err := s.App.Manifest(); err == nil && m.ProjectName != "" {
		info.Name = m.ProjectName
	}
	handler := s.Handler(studio.HandlerOptions{Assets: w.assets, Host: "desktop", MCPCommand: studio.Executable()})
	w.swap(s, info, handler)
	w.recent.Add(info.Root, info.Name)
	return info, nil
}

// Create cria a estrutura de um projeto novo no diretório e o abre.
func (w *Workspace) Create(dir, name string) (*ProjectInfo, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("escolha a pasta do projeto")
	}
	if _, err := studio.Init(dir, project.Options{ProjectName: strings.TrimSpace(name)}); err != nil {
		return nil, err
	}
	return w.Open(dir)
}

// Close fecha o projeto atual e volta para a tela inicial.
func (w *Workspace) Close() error {
	return w.swap(nil, nil, w.emptyHandler())
}

func (w *Workspace) swap(s *studio.Studio, info *ProjectInfo, handler http.Handler) error {
	w.mu.Lock()
	old := w.current
	w.current, w.info, w.handler = s, info, handler
	w.mu.Unlock()
	if old != nil {
		return old.Close()
	}
	return nil
}

// emptyHandler serve o frontend sem projeto: /api/health avisa que não há
// projeto (o frontend mostra a tela inicial) e o resto da API responde 409.
func (w *Workspace) emptyHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, http.StatusOK, map[string]any{
			"status": "ok", "version": w.version, "host": "desktop", "is_project": false,
			"frontend": w.assets != nil, "mcp_command": studio.Executable(), "mcp_sse_url": "",
		})
	})
	mux.HandleFunc("/api/", func(rw http.ResponseWriter, r *http.Request) {
		writeJSON(rw, http.StatusConflict, map[string]string{
			"error": "Nenhum projeto aberto",
			"hint":  "Abra ou crie um projeto (Arquivo → Abrir projeto…).",
		})
	})
	mux.Handle("/", httpapi.Frontend(w.assets, "desktop"))
	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
