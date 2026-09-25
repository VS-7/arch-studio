package store

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/archcode/studio/internal/model"
)

// Caminhos do Módulo de Implementação.
const (
	DirPlan          = ".arch/plan"
	DirPlanEpics     = ".arch/plan/epics"
	DirPlanStories   = ".arch/plan/stories"
	DirPlanTasks     = ".arch/plan/tasks"
	DirSprints       = ".arch/plan/sprints"
	DirSessions      = ".arch/sessions"
	DirMemory        = ".arch/memory"
	DirSkills        = ".arch/skills"
	DirLocal         = ".arch/.archcode-local"
	DirSprintReports = "docs/sprints"
	FileConventions  = ".arch/conventions.yaml"
	FileMemoryIndex  = ".arch/memory/MEMORY.md"
	FileChangelog    = "CHANGELOG.md"
)

// PlanDirs são as pastas do módulo, observadas pelo watcher.
var PlanDirs = []string{
	DirPlan, DirPlanEpics, DirPlanStories, DirPlanTasks, DirSprints,
	DirSessions, DirMemory, DirSkills,
}

// ItemPath devolve o caminho relativo do arquivo de um item.
func ItemPath(itemType, id string) string {
	return DirPlan + "/" + model.ItemDir(itemType) + "/" + model.WorkItemFileName(id)
}

// SprintPath devolve o caminho relativo do arquivo de uma sprint.
func SprintPath(n int) string { return DirSprints + "/" + model.SprintFileName(n) }

// ---------------------------------------------------------------------------
// Backlog
// ---------------------------------------------------------------------------

// LoadPlan lê todos os itens e sprints. Arquivos ilegíveis ou com marcadores
// de conflito não derrubam a leitura: viram avisos em Plan.Warnings.
func (s *Store) LoadPlan() (*model.Plan, error) {
	p := model.NewPlan()
	seen := map[string]string{}
	for _, dir := range []string{DirPlanEpics, DirPlanStories, DirPlanTasks} {
		files, err := s.listFiles(dir, ".md")
		if err != nil {
			return nil, err
		}
		for _, rel := range files {
			data, err := s.ReadFile(rel)
			if err != nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", rel, err))
				continue
			}
			item, err := model.ParseWorkItem(string(data))
			if err != nil {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s: %v", rel, err))
				continue
			}
			if prev, dup := seen[item.ID]; dup {
				p.Warnings = append(p.Warnings, fmt.Sprintf("%s: id %s repetido (também em %s)", rel, item.ID, prev))
				continue
			}
			seen[item.ID] = rel
			item.File = rel
			p.Items = append(p.Items, *item)
		}
	}
	files, err := s.listFiles(DirSprints, ".yaml")
	if err != nil {
		return nil, err
	}
	for _, rel := range files {
		data, err := s.ReadFile(rel)
		if err != nil {
			continue
		}
		if model.HasConflictMarkers(string(data)) {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s: arquivo com marcadores de conflito do Git", rel))
			continue
		}
		var sp model.Sprint
		if err := yaml.Unmarshal(data, &sp); err != nil || sp.Number <= 0 {
			p.Warnings = append(p.Warnings, fmt.Sprintf("%s: sprint inválida", rel))
			continue
		}
		if sp.Name == "" {
			sp.Name = model.SprintName(sp.Number)
		}
		if sp.Status == "" {
			sp.Status = model.SprintPlanned
		}
		sp.File = rel
		p.Sprints = append(p.Sprints, sp)
	}
	p.SortItems()
	p.SortSprints()
	return p, nil
}

// SaveItem grava o item. Se o arquivo antigo estiver em outra pasta (o tipo
// mudou), ele é removido.
func (s *Store) SaveItem(item *model.WorkItem) error {
	if !model.ValidItemID(item.ID) {
		return model.Invalid("id de item inválido: %q (use letras maiúsculas, dígitos e hífens, ex.: TASK-001)", item.ID)
	}
	rel := ItemPath(item.Type, item.ID)
	data, err := model.RenderWorkItem(item)
	if err != nil {
		return err
	}
	if item.File != "" && item.File != rel {
		_ = s.Remove(item.File)
	}
	if err := s.WriteFile(rel, []byte(data)); err != nil {
		return err
	}
	item.File = rel
	return nil
}

// RemoveItem apaga o arquivo do item.
func (s *Store) RemoveItem(item *model.WorkItem) error {
	rel := item.File
	if rel == "" {
		rel = ItemPath(item.Type, item.ID)
	}
	if err := s.Remove(rel); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SaveSprint grava a sprint.
func (s *Store) SaveSprint(sp *model.Sprint) error {
	if sp.Number <= 0 {
		return model.Invalid("número de sprint inválido: %d", sp.Number)
	}
	data, err := marshalYAML(sp)
	if err != nil {
		return err
	}
	rel := SprintPath(sp.Number)
	header := "# " + sp.Name + " — mantido pelo ArchCode Studio.\n"
	if err := s.WriteFile(rel, append([]byte(header), data...)); err != nil {
		return err
	}
	sp.File = rel
	return nil
}

// ---------------------------------------------------------------------------
// Convenções
// ---------------------------------------------------------------------------

// HasConventions informa se o projeto já tem .arch/conventions.yaml.
func (s *Store) HasConventions() bool { return s.Exists(FileConventions) }

// LoadConventions lê .arch/conventions.yaml. Sem o arquivo, valem os padrões
// de compatibilidade (fatiamento por componente, como o AI-PRD original).
func (s *Store) LoadConventions() (*model.Conventions, error) {
	data, err := s.ReadFile(FileConventions)
	if err != nil {
		if os.IsNotExist(err) {
			return model.LegacyConventions(), nil
		}
		return nil, err
	}
	c := &model.Conventions{}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", FileConventions, err)
	}
	c.Normalize()
	return c, nil
}

// SaveConventions grava as convenções.
func (s *Store) SaveConventions(c *model.Conventions) error {
	c.Normalize()
	data, err := marshalYAML(c)
	if err != nil {
		return err
	}
	header := "# Convenções do projeto: planejamento, Git, pull requests e autonomia da IA.\n" +
		"# Lidas pelo ArchCode Studio, pelos hooks do Git e pela CI (`archcode-studio git lint`).\n" +
		"# Marcadores: {sprint} {id} {slug} {summary} {title} {kind} {type} {version}.\n"
	return s.WriteFile(FileConventions, append([]byte(header), data...))
}

// ---------------------------------------------------------------------------
// Sessões
// ---------------------------------------------------------------------------

// ListSessions lê as sessões, da mais recente para a mais antiga (o nome do
// arquivo começa com data e hora). limit <= 0 = todas.
func (s *Store) ListSessions(limit int) ([]model.Session, error) {
	files, err := s.listFiles(DirSessions, ".md")
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	out := []model.Session{}
	for _, rel := range files {
		if limit > 0 && len(out) >= limit {
			break
		}
		data, err := s.ReadFile(rel)
		if err != nil {
			continue
		}
		sess, err := model.ParseSession(string(data))
		if err != nil {
			continue
		}
		if sess.ID == "" {
			sess.ID = strings.TrimSuffix(path.Base(rel), ".md")
		}
		sess.File = rel
		out = append(out, *sess)
	}
	return out, nil
}

// SaveSession grava a sessão em .arch/sessions/<id>.md.
func (s *Store) SaveSession(sess *model.Session) error {
	if sess.ID == "" || strings.ContainsAny(sess.ID, `/\`) || strings.HasPrefix(sess.ID, ".") {
		return model.Invalid("id de sessão inválido: %q", sess.ID)
	}
	data, err := model.RenderSession(sess)
	if err != nil {
		return err
	}
	rel := DirSessions + "/" + sess.ID + ".md"
	if err := s.WriteFile(rel, []byte(data)); err != nil {
		return err
	}
	sess.File = rel
	return nil
}

// ---------------------------------------------------------------------------
// Memórias
// ---------------------------------------------------------------------------

// ListNotes lê as memórias do projeto (MEMORY.md é o índice, não uma memória).
func (s *Store) ListNotes() ([]model.Note, error) {
	files, err := s.listFiles(DirMemory, ".md")
	if err != nil {
		return nil, err
	}
	out := []model.Note{}
	for _, rel := range files {
		if rel == FileMemoryIndex {
			continue
		}
		data, err := s.ReadFile(rel)
		if err != nil {
			continue
		}
		n, err := model.ParseNote(string(data))
		if err != nil {
			continue
		}
		n.Slug = strings.TrimSuffix(path.Base(rel), ".md")
		n.File = rel
		out = append(out, *n)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// SaveNote grava a memória em .arch/memory/<slug>.md.
func (s *Store) SaveNote(n *model.Note) error {
	if n.Slug == "" || n.Slug != model.Slugify(n.Slug) {
		return model.Invalid("slug de memória inválido: %q", n.Slug)
	}
	data, err := model.RenderNote(n)
	if err != nil {
		return err
	}
	rel := DirMemory + "/" + n.Slug + ".md"
	if err := s.WriteFile(rel, []byte(data)); err != nil {
		return err
	}
	n.File = rel
	return nil
}

// RemoveNote apaga a memória.
func (s *Store) RemoveNote(slug string) error {
	if slug == "" || slug != model.Slugify(slug) {
		return model.NotFound("memória não encontrada: %q", slug)
	}
	err := s.Remove(DirMemory + "/" + slug + ".md")
	if os.IsNotExist(err) {
		return model.NotFound("memória não encontrada: %q", slug)
	}
	return err
}

// ---------------------------------------------------------------------------
// Skills
// ---------------------------------------------------------------------------

// ListSkillFiles devolve o conteúdo de cada .arch/skills/<nome>/SKILL.md,
// indexado pelo nome da pasta.
func (s *Store) ListSkillFiles() (map[string]string, error) {
	dir, err := s.Path(DirSkills)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		data, err := s.ReadFile(DirSkills + "/" + e.Name() + "/SKILL.md")
		if err != nil {
			continue
		}
		out[e.Name()] = string(data)
	}
	return out, nil
}

// SkillPath devolve o caminho do SKILL.md de uma skill do projeto.
func SkillPath(name string) string { return DirSkills + "/" + name + "/SKILL.md" }

// RemoveDir apaga uma pasta do projeto (relativa) e tudo dentro dela.
func (s *Store) RemoveDir(rel string) error {
	p, err := s.Path(rel)
	if err != nil {
		return err
	}
	if p == s.root {
		return fmt.Errorf("não é permitido apagar a raiz do projeto")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = filepath.WalkDir(p, func(abs string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			s.rememberRemoval(abs)
		}
		return nil
	})
	return os.RemoveAll(p)
}

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------

// listFiles lista (sem recursão) os arquivos de dir com a extensão informada,
// em ordem alfabética, como caminhos relativos.
func (s *Store) listFiles(dir, ext string) ([]string, error) {
	abs, err := s.Path(dir)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ext) || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
			continue
		}
		out = append(out, dir+"/"+name)
	}
	sort.Strings(out)
	return out, nil
}
