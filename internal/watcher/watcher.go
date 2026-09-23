// Package watcher observa a árvore do projeto e converte eventos de disco em
// eventos de aplicação, com debounce para evitar tempestades de re-render.
//
// Gravações feitas pelo próprio servidor são filtradas pelo índice de eco do
// store, de modo que apenas edições externas (VS Code, git checkout, CLI de IA)
// chegam ao browser.
package watcher

import (
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/store"
)

const debounce = 300 * time.Millisecond

type Watcher struct {
	st  *store.Store
	hb  *hub.Hub
	fsw *fsnotify.Watcher

	mu      sync.Mutex
	pending map[string]time.Time
	timer   *time.Timer
}

func New(st *store.Store, hb *hub.Hub) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{st: st, hb: hb, fsw: fsw, pending: map[string]time.Time{}}, nil
}

// watchedDirs são os diretórios monitorados recursivamente.
var watchedDirs = []string{
	store.DirArch, store.DirDiagrams, store.DirSequence, store.DirER,
	store.DirUseCaseUML, store.DirClass, store.DirState,
	store.DirDocs, store.DirUseCases, store.DirADR, store.DirAPI,
}

func (w *Watcher) Start() error {
	for _, dir := range watchedDirs {
		abs, err := w.st.Path(dir)
		if err != nil {
			continue
		}
		if err := os.MkdirAll(abs, 0o755); err != nil {
			continue
		}
		if err := w.fsw.Add(abs); err != nil {
			log.Printf("watcher: não foi possível observar %s: %v", dir, err)
		}
	}
	go w.loop()
	return nil
}

func (w *Watcher) Close() error { return w.fsw.Close() }

func (w *Watcher) loop() {
	for {
		select {
		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: %v", err)
		}
	}
}

func relevant(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") && !strings.HasSuffix(base, ".yaml") && !strings.HasSuffix(base, ".json") {
		return false
	}
	if strings.HasSuffix(base, "~") || strings.Contains(base, ".archcode-") || strings.HasSuffix(base, ".swp") {
		return false
	}
	switch filepath.Ext(base) {
	case ".json", ".yaml", ".yml", ".md", ".mermaid", ".mmd":
		return true
	}
	return false
}

func (w *Watcher) handle(ev fsnotify.Event) {
	if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}
	// Diretório novo dentro da árvore observada passa a ser monitorado também
	// (ex.: .arch/diagrams/class criado depois do start). A checagem vem antes
	// do filtro de extensão, que descartaria nomes de diretório.
	if ev.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			_ = w.fsw.Add(ev.Name)
			return
		}
	}
	if !relevant(ev.Name) {
		return
	}

	w.mu.Lock()
	w.pending[ev.Name] = time.Now()
	if w.timer == nil {
		w.timer = time.AfterFunc(debounce, w.flush)
	} else {
		w.timer.Reset(debounce)
	}
	w.mu.Unlock()
}

func (w *Watcher) flush() {
	w.mu.Lock()
	paths := make([]string, 0, len(w.pending))
	for p := range w.pending {
		paths = append(paths, p)
	}
	w.pending = map[string]time.Time{}
	w.timer = nil
	w.mu.Unlock()

	seen := map[string]bool{}
	for _, abs := range paths {
		// Ignora o eco das próprias gravações do servidor.
		if w.st.IsEcho(abs) {
			continue
		}
		rel := w.st.Rel(abs)
		evType := classify(rel)
		if evType == "" || seen[evType+rel] {
			continue
		}
		seen[evType+rel] = true
		ev := hub.Event{
			Type:    evType,
			Source:  hub.SourceDisk,
			Path:    rel,
			Message: "Arquivo alterado fora do ArchCode Studio",
		}
		if evType == hub.EventUML {
			ev.Payload = map[string]any{"diagram_id": strings.TrimSuffix(path.Base(rel), ".json")}
		}
		w.hb.Broadcast(ev)
	}
}

// umlDiagramFile informa se o caminho é o JSON de um diagrama UML.
func umlDiagramFile(rel string) bool {
	if !strings.HasSuffix(rel, ".json") {
		return false
	}
	dir := path.Dir(rel)
	for _, d := range store.UMLDirs {
		if dir == d {
			return true
		}
	}
	return false
}

func classify(rel string) string {
	switch {
	case umlDiagramFile(rel):
		return hub.EventUML
	case rel == store.FileMacroJSON, rel == store.FileMacroMmd:
		return hub.EventDiagram
	case rel == store.FileEndpoints:
		return hub.EventEndpoints
	case rel == store.FilePricing:
		return hub.EventPricing
	case rel == store.FileTasks:
		return hub.EventTasks
	case rel == store.FileManifest:
		return hub.EventManifest
	case rel == store.FileDocument:
		return hub.EventDocs
	case strings.HasPrefix(rel, store.DirDocs+"/"):
		return hub.EventDocs
	case strings.HasPrefix(rel, store.DirDiagrams+"/"):
		return hub.EventDiagram
	default:
		return ""
	}
}
