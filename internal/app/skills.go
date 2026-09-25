package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/skills"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Skills (RF043–RF047)
// ---------------------------------------------------------------------------

// installedSkills lê as skills do projeto; arquivos inválidos vão para os
// avisos, sem derrubar a leitura.
func (a *App) installedSkills() ([]skills.Skill, []string) {
	files, err := a.st.ListSkillFiles()
	if err != nil {
		return nil, []string{err.Error()}
	}
	out := []skills.Skill{}
	warnings := []string{}
	for dir, content := range files {
		s, err := skills.Parse(content)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("%s: %v", store.SkillPath(dir), err))
			continue
		}
		if s.Name != dir {
			warnings = append(warnings, fmt.Sprintf("%s: o nome %q não bate com a pasta", store.SkillPath(dir), s.Name))
			continue
		}
		s.Installed, s.File = true, store.SkillPath(dir)
		out = append(out, *s)
	}
	skills.SortSkills(out)
	return out, warnings
}

// projectStacks detecta os stacks do projeto pelas tecnologias do canvas.
func (a *App) projectStacks() []string {
	d, _ := a.st.LoadDiagram()
	target := ""
	if b, err := a.st.LoadTasks(); err == nil {
		target = b.TargetStack
	}
	return skills.DetectStacks(d, target)
}

// skillsFor devolve as skills ativas que valem para o item. Os stacks vêm da
// tecnologia do componente da tarefa (a tarefa do banco não recebe a skill de
// estilo do frontend); sem componente, valem os do projeto.
func (a *App) skillsFor(item *model.WorkItem) []skills.Skill {
	list, _ := a.installedSkills()
	stacks := a.projectStacks()
	if item != nil && item.ComponentID != "" {
		if d, err := a.st.LoadDiagram(); err == nil {
			if node := d.NodeByID(item.ComponentID); node != nil {
				one := model.NewDiagram()
				one.Nodes = []model.Node{*node}
				stacks = skills.DetectStacks(one, "")
			}
		}
	}
	return skills.ForTask(list, item, stacks)
}

// SkillsOverview é o que a tela de Skills mostra.
type SkillsOverview struct {
	Installed []skills.Skill `json:"installed"`
	Catalog   []skills.Skill `json:"catalog"`
	Stacks    []string       `json:"stacks"`
	Suggested []string       `json:"suggested"`
	Warnings  []string       `json:"warnings,omitempty"`
	Synced    SyncTargets    `json:"synced"`
}

// SyncTargets informa para quais agentes as skills já foram exportadas.
type SyncTargets struct {
	Claude bool `json:"claude"`
	Agents bool `json:"agents"`
	Cursor bool `json:"cursor"`
}

// Skills devolve as skills instaladas, o catálogo e as sugestões.
func (a *App) Skills() (*SkillsOverview, error) {
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	installed, warnings := a.installedSkills()
	catalog := skills.Catalog(conv)
	byName := map[string]*skills.Skill{}
	for i := range installed {
		byName[installed[i].Name] = &installed[i]
	}
	for i := range catalog {
		c := &catalog[i]
		if inst, ok := byName[c.Name]; ok {
			c.Installed = true
			v := skills.BuiltinVersion(inst.Source)
			if v != "" && v != c.Version {
				inst.UpdateAvailable = true
			}
			if inst.Source == skills.SourceGenerated {
				// A gerada acompanha as convenções: desatualizada se o corpo mudou.
				inst.UpdateAvailable = strings.TrimSpace(inst.Body) != strings.TrimSpace(c.Body)
			}
			inst.Builtin = true
		}
	}
	stacks := a.projectStacks()
	agents, _ := a.st.ReadFile("AGENTS.md")
	return &SkillsOverview{
		Installed: installed, Catalog: catalog, Stacks: stacks, Suggested: skills.Suggest(catalog, stacks),
		Warnings: warnings,
		Synced: SyncTargets{
			Claude: a.st.Exists(".claude/skills"),
			Agents: model.HasManagedBlock(string(agents), skills.AgentsBlockName),
			Cursor: a.st.Exists(".cursor/rules"),
		},
	}, nil
}

// Skill devolve uma skill instalada (ou do catálogo, se não instalada).
func (a *App) Skill(name string) (*skills.Skill, error) {
	installed, _ := a.installedSkills()
	for _, s := range installed {
		if s.Name == name {
			s := s
			return &s, nil
		}
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	if s := skills.CatalogSkill(name, conv); s != nil {
		return s, nil
	}
	return nil, model.NotFound("skill %q não encontrada", name)
}

// SkillsForTask devolve as skills ativas que valem para a tarefa (todas as
// ativas, quando id é vazio).
func (a *App) SkillsForTask(id string) ([]skills.Skill, error) {
	if id == "" {
		list, _ := a.installedSkills()
		return skills.ForTask(list, nil, a.projectStacks()), nil
	}
	item, err := a.Item(id)
	if err != nil {
		return nil, err
	}
	return a.skillsFor(item), nil
}

// InstallSkills instala skills do catálogo no projeto. Com update, reinstala
// as que já existem (perdendo edições locais).
func (a *App) InstallSkills(names []string, update bool, source string) ([]string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	installed := []string{}
	for _, name := range names {
		s := skills.CatalogSkill(name, conv)
		if s == nil {
			return installed, model.NotFound("a skill %q não está no catálogo", name)
		}
		if a.st.Exists(store.SkillPath(name)) && !update {
			continue
		}
		inst := skills.ForInstall(*s)
		content, err := skills.Render(&inst)
		if err != nil {
			return installed, err
		}
		if err := a.st.WriteFile(store.SkillPath(name), []byte(content)); err != nil {
			return installed, err
		}
		installed = append(installed, name)
	}
	if len(installed) > 0 {
		a.emit(hub.Event{Type: hub.EventSkills, Source: source, Path: store.DirSkills,
			Message: fmt.Sprintf("%d skill(s) instalada(s)", len(installed))})
	}
	return installed, nil
}

// SaveSkill grava o conteúdo de uma skill do projeto (cria, se não existir).
func (a *App) SaveSkill(name, content, source string) (*skills.Skill, error) {
	s, err := skills.Parse(content)
	if err != nil {
		return nil, model.Invalid("%v", err)
	}
	if s.Name != name {
		return nil, model.Invalid("o name do frontmatter (%q) precisa ser igual ao nome da skill (%q)", s.Name, name)
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	if err := a.st.WriteFile(store.SkillPath(name), []byte(content)); err != nil {
		return nil, err
	}
	s.Installed, s.File = true, store.SkillPath(name)
	a.emit(hub.Event{Type: hub.EventSkills, Source: source, Path: s.File, Message: "Skill " + name + " salva"})
	return s, nil
}

// CreateSkill cria uma skill própria do projeto a partir de um esqueleto.
func (a *App) CreateSkill(name, description, category, trigger, source string) (*skills.Skill, error) {
	if !skills.ValidName(name) {
		return nil, model.Invalid("nome inválido: %q (minúsculas, dígitos e hífens, até 50 caracteres)", name)
	}
	if a.st.Exists(store.SkillPath(name)) {
		return nil, model.Invalid("a skill %s já existe", name)
	}
	if strings.TrimSpace(description) == "" {
		return nil, model.Invalid("descreva o que a skill garante e quando usar")
	}
	s := &skills.Skill{Name: name, Description: strings.TrimSpace(description), Category: category, Trigger: trigger,
		Version: "1.0.0", Source: skills.SourceProject,
		Body: "# " + name + "\n\n## Quando usar\n\n" + strings.TrimSpace(description) + "\n\n## Regras\n\n1. \n\n## Como verificar\n\n- \n"}
	if _, ok := skills.CategoryLabels[s.Category]; !ok {
		s.Category = skills.CategoryStandards
	}
	if _, ok := skills.TriggerLabels[s.Trigger]; !ok {
		s.Trigger = skills.TriggerAlways
	}
	content, err := skills.Render(s)
	if err != nil {
		return nil, err
	}
	return a.SaveSkill(name, content, source)
}

// SetSkillEnabled liga ou desliga uma skill instalada.
func (a *App) SetSkillEnabled(name string, enabled bool, source string) (*skills.Skill, error) {
	data, err := a.st.ReadFile(store.SkillPath(name))
	if err != nil {
		return nil, model.NotFound("skill %q não instalada", name)
	}
	s, err := skills.Parse(string(data))
	if err != nil {
		return nil, model.Invalid("%v", err)
	}
	s.Disabled = !enabled
	content, err := skills.Render(s)
	if err != nil {
		return nil, err
	}
	return a.SaveSkill(name, content, source)
}

// RemoveSkill desinstala uma skill do projeto.
func (a *App) RemoveSkill(name, source string) error {
	if !skills.ValidName(name) || !a.st.Exists(store.SkillPath(name)) {
		return model.NotFound("skill %q não instalada", name)
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	if err := a.st.RemoveDir(store.DirSkills + "/" + name); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventSkills, Source: source, Path: store.DirSkills, Message: "Skill " + name + " removida"})
	return nil
}

// refreshGeneratedSkill regenera a convencoes-git (se instalada como gerada)
// depois de uma mudança nas convenções. Chamado com a.tx travado.
func (a *App) refreshGeneratedSkill(conv *model.Conventions) {
	data, err := a.st.ReadFile(store.SkillPath("convencoes-git"))
	if err != nil {
		return
	}
	cur, err := skills.Parse(string(data))
	if err != nil || cur.Source != skills.SourceGenerated {
		return
	}
	gen := skills.ForInstall(*skills.ConventionsSkill(conv))
	gen.Disabled = cur.Disabled
	if content, err := skills.Render(&gen); err == nil {
		_ = a.st.WriteFile(store.SkillPath("convencoes-git"), []byte(content))
	}
}

// SyncSkillsResult lista o que a sincronização escreveu e apagou.
type SyncSkillsResult struct {
	Written []string `json:"written"`
	Removed []string `json:"removed"`
}

// Destinos de sincronização das skills.
const (
	TargetClaude = "claude"
	TargetAgents = "agents"
	TargetCursor = "cursor"
)

// SyncSkills exporta as skills ativas para o formato de cada agente, apagando
// arquivos gerados de skills que saíram (RF046). Só toca em arquivos que ele
// mesmo gerou e no bloco gerenciado do AGENTS.md.
func (a *App) SyncSkills(targets []string, source string) (*SyncSkillsResult, error) {
	if len(targets) == 0 {
		targets = []string{TargetClaude, TargetAgents}
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	a.refreshGeneratedSkill(conv)
	installed, _ := a.installedSkills()
	active := []skills.Skill{}
	for _, s := range installed {
		if s.Enabled() {
			active = append(active, s)
		}
	}
	res := &SyncSkillsResult{Written: []string{}, Removed: []string{}}
	write := func(rel, content string) error {
		if old, err := a.st.ReadFile(rel); err == nil && string(old) == content {
			return nil
		}
		if err := a.st.WriteFile(rel, []byte(content)); err != nil {
			return err
		}
		res.Written = append(res.Written, rel)
		return nil
	}
	for _, target := range targets {
		switch target {
		case TargetClaude:
			keep := map[string]bool{}
			for i := range active {
				rel := skills.ClaudePath(active[i].Name)
				keep[rel] = true
				if err := write(rel, skills.ClaudeSkill(&active[i])); err != nil {
					return nil, err
				}
			}
			res.Removed = append(res.Removed, a.removeStaleGenerated(".claude/skills", "SKILL.md", keep)...)
		case TargetCursor:
			keep := map[string]bool{}
			for i := range active {
				rel := skills.CursorPath(active[i].Name)
				keep[rel] = true
				if err := write(rel, skills.CursorRule(&active[i])); err != nil {
					return nil, err
				}
			}
			res.Removed = append(res.Removed, a.removeStaleGenerated(".cursor/rules", "", keep)...)
		case TargetAgents:
			old, _ := a.st.ReadFile("AGENTS.md")
			content := string(old)
			if strings.TrimSpace(content) == "" {
				content = "# AGENTS.md\n\nInstruções para agentes de IA neste repositório.\n"
			}
			if err := write("AGENTS.md", model.ReplaceManagedBlock(content, skills.AgentsBlockName,
				skills.AgentsBlock(active, conv), false)); err != nil {
				return nil, err
			}
		default:
			return nil, model.Invalid("destino desconhecido: %q (use claude, agents ou cursor)", target)
		}
	}
	a.emit(hub.Event{Type: hub.EventSkills, Source: source, Message: fmt.Sprintf("Skills sincronizadas (%d arquivo(s))", len(res.Written))})
	return res, nil
}

// removeStaleGenerated apaga, em dir, os arquivos gerados pelo Studio que
// não estão em keep. fileName filtra o nome (ex.: SKILL.md) dentro de
// subpastas; vazio = arquivos diretos da pasta.
func (a *App) removeStaleGenerated(dir, fileName string, keep map[string]bool) []string {
	abs, err := a.st.Path(dir)
	if err != nil {
		return nil
	}
	removed := []string{}
	_ = filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if fileName != "" && d.Name() != fileName {
			return nil
		}
		rel := a.st.Rel(path)
		if keep[rel] {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(data), skills.GeneratedMarker) {
			return nil
		}
		if a.st.Remove(rel) == nil {
			removed = append(removed, rel)
			if fileName != "" {
				_ = os.Remove(filepath.Dir(path)) // pasta vazia da skill
			}
		}
		return nil
	})
	sort.Strings(removed)
	return removed
}

// ---------------------------------------------------------------------------
// Configuração dos agentes (RF053)
// ---------------------------------------------------------------------------

// AgentSetupResult lista os arquivos configurados.
type AgentSetupResult struct {
	Agent   string   `json:"agent"`
	Written []string `json:"written"`
	Notes   []string `json:"notes,omitempty"`
}

// hookCommand envolve o comando para não falhar onde o CLI não está instalado.
func hookCommand(args string) string {
	return "command -v archcode-studio >/dev/null 2>&1 && archcode-studio " + args + " || true"
}

// AgentSetup configura o agente para trabalhar com o Studio: MCP do
// projeto, hooks de retomada (Claude Code) e skills exportadas.
func (a *App) AgentSetup(agent, source string) (*AgentSetupResult, error) {
	agent = strings.ToLower(strings.TrimSpace(agent))
	res := &AgentSetupResult{Agent: agent, Written: []string{}}
	switch agent {
	case "claude", "claude-code":
		res.Agent = "claude"
		if err := a.mergeJSON(".mcp.json", func(root map[string]any) {
			servers := childMap(root, "mcpServers")
			servers["archcode-studio"] = map[string]any{"type": "stdio", "command": "archcode-studio", "args": []any{"mcp"}}
		}); err != nil {
			return nil, err
		}
		res.Written = append(res.Written, ".mcp.json")
		if err := a.mergeJSON(".claude/settings.json", func(root map[string]any) {
			hooks := childMap(root, "hooks")
			ensureHook(hooks, "SessionStart", "startup|resume|clear|compact", hookCommand("resume --brief --hook"))
			ensureHook(hooks, "PreCompact", "", hookCommand("checkpoint --auto --quiet"))
			ensureHook(hooks, "SessionEnd", "", hookCommand("checkpoint --auto --quiet"))
		}); err != nil {
			return nil, err
		}
		res.Written = append(res.Written, ".claude/settings.json")
		sync, err := a.SyncSkills([]string{TargetClaude, TargetAgents}, source)
		if err != nil {
			return nil, err
		}
		res.Written = append(res.Written, sync.Written...)
		res.Notes = append(res.Notes,
			"Ao abrir o Claude Code no projeto, o hook de início injeta o `archcode-studio resume --brief`.",
			"O CLI archcode-studio precisa estar no PATH (os hooks são ignorados sem ele).")
	case "cursor":
		if err := a.mergeJSON(".cursor/mcp.json", func(root map[string]any) {
			servers := childMap(root, "mcpServers")
			servers["archcode-studio"] = map[string]any{"command": "archcode-studio", "args": []any{"mcp"}}
		}); err != nil {
			return nil, err
		}
		res.Written = append(res.Written, ".cursor/mcp.json")
		sync, err := a.SyncSkills([]string{TargetCursor, TargetAgents}, source)
		if err != nil {
			return nil, err
		}
		res.Written = append(res.Written, sync.Written...)
	default:
		return nil, model.Invalid("agente desconhecido: %q (use claude ou cursor)", agent)
	}
	return res, nil
}

// mergeJSON lê um JSON do projeto (ou começa vazio), aplica a mudança e grava
// com indentação, preservando as chaves que não são do Studio.
func (a *App) mergeJSON(rel string, change func(map[string]any)) error {
	root := map[string]any{}
	if data, err := a.st.ReadFile(rel); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &root); err != nil {
			return model.Invalid("%s não é um JSON válido (%v); corrija antes de configurar o agente", rel, err)
		}
	}
	change(root)
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	// Sem escapar <, > e &: o arquivo é lido e revisado por pessoas.
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return err
	}
	return a.st.WriteFile(rel, buf.Bytes())
}

func childMap(root map[string]any, key string) map[string]any {
	if m, ok := root[key].(map[string]any); ok {
		return m
	}
	m := map[string]any{}
	root[key] = m
	return m
}

// ensureHook garante um hook de comando no evento, sem duplicar os do Studio.
func ensureHook(hooks map[string]any, event, matcher, command string) {
	groups, _ := hooks[event].([]any)
	for _, g := range groups {
		gm, _ := g.(map[string]any)
		list, _ := gm["hooks"].([]any)
		for i, h := range list {
			hm, _ := h.(map[string]any)
			if cmd, _ := hm["command"].(string); strings.Contains(cmd, "archcode-studio") {
				hm["command"] = command
				list[i] = hm
				return
			}
		}
	}
	group := map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}}
	if matcher != "" {
		group["matcher"] = matcher
	}
	hooks[event] = append(groups, group)
}
