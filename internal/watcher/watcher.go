// Package watcher observa a árvore do projeto e avisa, com debounce, quais
// arquivos mudaram fora do ArchCode Studio.
//
// Gravações feitas pelo próprio servidor são filtradas pelo índice de eco do
// store, de modo que apenas edições externas (VS Code, git checkout, CLI de IA)
// são reportadas. O que fazer com a mudança (qual evento publicar) é decisão de
// quem recebe o aviso — o pacote app.
package watcher

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/archcode/studio/internal/store"
)

const debounce = 300 * time.Millisecond

type Watcher struct {
	st       *store.Store
	onChange func(rel string)
	fsw      *fsnotify.Watcher

	mu      sync.Mutex
	pending map[string]time.Time
	timer   *time.Timer
}

// New cria o watcher; onChange recebe o caminho relativo (com barras) de cada
// arquivo alterado externamente.
func New(st *store.Store, onChange func(rel string)) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{st: st, onChange: onChange, fsw: fsw, pending: map[string]time.Time{}}, nil
}

func (w *Watcher) Start() error {
	for _, dir := range store.ProjectDirs {
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
		if seen[rel] {
			continue
		}
		seen[rel] = true
		w.onChange(rel)
	}
}
