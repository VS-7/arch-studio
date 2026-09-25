package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Diagnóstico do backlog e da memória (RF058)
// ---------------------------------------------------------------------------

// DoctorReport é o resultado do diagnóstico.
type DoctorReport struct {
	Problems    []string `json:"problems"`
	Conflicts   []string `json:"conflicts"`
	Resolved    []string `json:"resolved,omitempty"`
	Regenerated []string `json:"regenerated,omitempty"`
}

// Doctor procura arquivos do Studio com conflito de merge e inconsistências
// do backlog. Com prefer (ours|theirs), resolve os conflitos ficando com um
// lado; com fix, regenera os arquivos derivados (tasks.json, MEMORY.md).
func (a *App) Doctor(prefer string, fix bool) (*DoctorReport, error) {
	if prefer != "" && prefer != "ours" && prefer != "theirs" {
		return nil, model.Invalid("prefer deve ser ours ou theirs")
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	rep := &DoctorReport{Problems: []string{}, Conflicts: []string{}}
	dirs := append([]string{store.DirArch}, store.PlanDirs...)
	seen := map[string]bool{}
	for _, dir := range dirs {
		abs, err := a.st.Path(dir)
		if err != nil {
			continue
		}
		_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || seen[path] {
				return nil
			}
			seen[path] = true
			ext := filepath.Ext(path)
			if ext != ".md" && ext != ".yaml" && ext != ".json" {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil || !model.HasConflictMarkers(string(data)) {
				return nil
			}
			rel := a.st.Rel(path)
			rep.Conflicts = append(rep.Conflicts, rel)
			if prefer != "" {
				if fixed, ok := model.ResolveConflicts(string(data), prefer); ok {
					if a.st.WriteFile(rel, []byte(fixed)) == nil {
						rep.Resolved = append(rep.Resolved, rel)
					}
				}
			}
			return nil
		})
	}
	p, err := a.st.LoadPlan()
	if err != nil {
		return nil, err
	}
	for _, prob := range plan.Problems(p) {
		if !strings.Contains(prob, "marcadores de conflito") || prefer == "" {
			rep.Problems = append(rep.Problems, prob)
		}
	}
	if fix || prefer != "" {
		a.writeDerived(p, hub.SourceCLI)
		a.writeMemoryIndex()
		rep.Regenerated = append(rep.Regenerated, store.FileTasks, store.FileMemoryIndex)
	}
	if len(rep.Resolved) > 0 {
		a.emit(hub.Event{Type: hub.EventPlan, Source: hub.SourceCLI,
			Message: fmt.Sprintf("%d conflito(s) resolvido(s) pelo doctor", len(rep.Resolved))})
	}
	return rep, nil
}
