package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const maxRecent = 10

// RecentProject é um projeto aberto recentemente.
type RecentProject struct {
	Root     string `json:"root"`
	Name     string `json:"name"`
	OpenedAt string `json:"opened_at"`
}

// RecentList guarda os projetos abertos recentemente, do mais novo ao mais antigo.
type RecentList struct {
	path string

	mu    sync.Mutex
	items []RecentProject
}

// LoadRecent lê os recentes de <config do usuário>/archcode-studio/recent.json.
func LoadRecent() *RecentList {
	r := &RecentList{}
	if dir, err := os.UserConfigDir(); err == nil {
		r.path = filepath.Join(dir, "archcode-studio", "recent.json")
		r.load()
	}
	return r
}

// NewRecentList usa um arquivo específico (testes).
func NewRecentList(path string) *RecentList {
	r := &RecentList{path: path}
	r.load()
	return r
}

func (r *RecentList) load() {
	if data, err := os.ReadFile(r.path); err == nil {
		_ = json.Unmarshal(data, &r.items)
	}
}

// List devolve os recentes que ainda existem no disco, do mais novo ao mais antigo.
func (r *RecentList) List() []RecentProject {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []RecentProject{}
	for _, p := range r.items {
		if _, err := os.Stat(filepath.Join(p.Root, ".arch", "manifest.yaml")); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// Add move o projeto para o topo da lista e grava o arquivo.
func (r *RecentList) Add(root, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := []RecentProject{{Root: root, Name: name, OpenedAt: time.Now().Format(time.RFC3339)}}
	for _, p := range r.items {
		if p.Root != root && len(items) < maxRecent {
			items = append(items, p)
		}
	}
	r.items = items
	r.save()
}

// Remove tira o projeto da lista (ex.: o usuário apagou a pasta).
func (r *RecentList) Remove(root string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	kept := r.items[:0]
	for _, p := range r.items {
		if p.Root != root {
			kept = append(kept, p)
		}
	}
	r.items = kept
	r.save()
}

func (r *RecentList) save() {
	if r.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return
	}
	data, _ := json.MarshalIndent(r.items, "", "  ")
	_ = os.WriteFile(r.path, data, 0o644)
}
