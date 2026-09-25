package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Módulo de Planejamento: backlog (RF030–RF032, RF036)
// ---------------------------------------------------------------------------
//
// O backlog em .arch/plan/ é a fonte da verdade do trabalho. Toda mutação
// passa por aqui, sob a mesma trava das demais escritas, e termina em
// commitPlan: grava os itens alterados, regenera o tasks.json derivado,
// reflete o progresso no canvas e anuncia a mudança.

// Person devolve a identidade de quem usa o Studio ("Nome <email>" do Git).
func (a *App) Person() string { return gitx.Identity(a.git) }

// Git expõe o adaptador do Git para os adaptadores (CLI e hooks).
func (a *App) Git() gitx.Git { return a.git }

// Plan devolve o backlog. Um projeto que ainda só tem o tasks.json de uma
// versão anterior vê as tarefas dele (a migração em disco acontece na
// primeira escrita).
func (a *App) Plan() (*model.Plan, error) {
	p, err := a.st.LoadPlan()
	if err != nil {
		return nil, err
	}
	if len(p.Items) == 0 {
		if board, err := a.st.LoadTasks(); err == nil && len(board.Tasks) > 0 {
			p.Items = plan.FromBoard(board, a.timestamp())
		}
	}
	return p, nil
}

// loadPlan lê o backlog para uma escrita, migrando o tasks.json de uma versão
// anterior na primeira vez (RNF012). Chamado com a.tx travado.
func (a *App) loadPlan() (*model.Plan, error) {
	p, err := a.st.LoadPlan()
	if err != nil {
		return nil, err
	}
	if len(p.Items) > 0 {
		return p, nil
	}
	board, err := a.st.LoadTasks()
	if err != nil || len(board.Tasks) == 0 {
		return p, nil
	}
	for _, it := range plan.FromBoard(board, a.timestamp()) {
		it := it
		if err := a.st.SaveItem(&it); err != nil {
			return nil, fmt.Errorf("migração do tasks.json: %w", err)
		}
		p.Items = append(p.Items, it)
	}
	p.SortItems()
	return p, nil
}

// EnsurePlanMigrated faz a migração do tasks.json na abertura do projeto.
func (a *App) EnsurePlanMigrated() error {
	a.tx.Lock()
	defer a.tx.Unlock()
	_, err := a.loadPlan()
	return err
}

// commitPlan grava os itens e sprints alterados e atualiza o que deriva do
// backlog. Chamado com a.tx travado.
func (a *App) commitPlan(p *model.Plan, items []*model.WorkItem, sprints []*model.Sprint, source, message string) error {
	for _, it := range items {
		if err := a.st.SaveItem(it); err != nil {
			return err
		}
	}
	for _, sp := range sprints {
		if err := a.st.SaveSprint(sp); err != nil {
			return err
		}
	}
	// Ordena uma cópia: os ponteiros que o chamador guarda continuam válidos.
	sorted := *p
	sorted.Items = append([]model.WorkItem(nil), p.Items...)
	sorted.SortItems()
	a.writeDerived(&sorted, source)
	ids := []string{}
	for _, it := range items {
		ids = append(ids, it.ID)
	}
	a.emit(hub.Event{Type: hub.EventPlan, Source: source, Path: store.DirPlan, Message: message,
		Payload: map[string]any{"items": ids}})
	return nil
}

// writeDerived regenera o tasks.json a partir do backlog, reflete o status no
// canvas e nas caixas do ai-prd.md. Falhas aqui não desfazem a escrita do
// backlog, que é a fonte da verdade: o doctor regenera depois.
func (a *App) writeDerived(p *model.Plan, source string) {
	prev, _ := a.st.LoadTasks()
	board := plan.ToBoard(p, prev)
	if prev == nil || !sameBoard(prev, board) {
		if err := a.st.SaveTasks(board); err == nil {
			a.emit(hub.Event{Type: hub.EventTasks, Source: source, Path: store.FileTasks,
				Payload: map[string]any{"progress": board.ProgressPercentage()}})
		}
	}
	if d, err := a.st.LoadDiagram(); err == nil && prd.SyncNodeStatus(d, board) {
		_ = a.saveDiagram(d)
	}
	a.refreshPRDCheckboxes(board)
}

func sameBoard(a, b *model.TaskBoard) bool {
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	return string(ja) == string(jb)
}

// ---------------------------------------------------------------------------
// Sincronização com a arquitetura
// ---------------------------------------------------------------------------

// compileBoard compila as tarefas de componente como o AI-PRD faria, com os
// ids que o backlog já deu a cada componente.
func (a *App) compileBoard(snap *store.Snapshot, p *model.Plan, conv *model.Conventions, opts prd.Options) *prd.Result {
	ids := map[string]string{}
	for _, it := range p.Items {
		if strings.HasPrefix(it.Source, plan.SourceComponent) {
			ids[strings.TrimPrefix(it.Source, plan.SourceComponent)] = it.ID
		}
	}
	opts.SliceUseCases = conv.Planning.Slicing == model.SlicingHybrid
	return prd.Compile(prd.Input{
		Manifest: snap.Manifest, Diagram: snap.Diagram, Requirements: snap.Requirements,
		UseCases: snap.UseCases, ADRs: snap.ADRs, Endpoints: snap.Endpoints, UMLDiagrams: snap.UMLDiagrams,
		Previous: snap.Tasks, TaskIDs: ids,
	}, opts)
}

// desiredItems gera os itens que a arquitetura pede agora.
func (a *App) desiredItems(snap *store.Snapshot, p *model.Plan, conv *model.Conventions, board *model.TaskBoard) []model.WorkItem {
	est := pricing.Calculate(snap.Diagram, snap.UseCases, snap.Pricing, -1)
	return plan.Generate(plan.Input{
		Diagram: snap.Diagram, Requirements: snap.Requirements, UseCases: snap.UseCases,
		Board: board, Estimate: est, Slicing: conv.Planning.Slicing, Existing: p,
	})
}

// SyncResult é o diff da sincronização do backlog.
type SyncResult struct {
	DryRun  bool           `json:"dry_run"`
	Changes []plan.Change  `json:"changes"`
	Counts  map[string]int `json:"counts"`
	Total   int            `json:"total_items"`
	Slicing string         `json:"slicing"`
}

// SyncBacklog reconcilia o backlog com a arquitetura. Com dryRun, só devolve
// o diff (a interface mostra antes de aplicar).
func (a *App) SyncBacklog(dryRun bool, source string) (*SyncResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	return a.syncBacklogLocked(dryRun, source, nil)
}

func (a *App) syncBacklogLocked(dryRun bool, source string, compiled *prd.Result) (*SyncResult, error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	var p *model.Plan
	if dryRun {
		p, err = a.Plan()
	} else {
		p, err = a.loadPlan()
	}
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	if compiled == nil {
		compiled = a.compileBoard(snap, p, conv, prd.Options{IncludeTestScenarios: true}.Normalize())
	}
	res := plan.Sync(p, a.desiredItems(snap, p, conv, compiled.Board), a.timestamp())
	out := &SyncResult{DryRun: dryRun, Changes: res.Changes, Counts: res.Counts(), Slicing: conv.Planning.Slicing}
	if dryRun {
		out.Total = len(p.Items) + out.Counts[plan.ChangeAdded]
		return out, nil
	}
	for _, it := range res.Items {
		if cur := p.Item(it.ID); cur != nil {
			*cur = it
		} else {
			p.Items = append(p.Items, it)
		}
	}
	// Ponteiros só depois de todos os appends (o slice pode ter mudado).
	changed := make([]*model.WorkItem, 0, len(res.Items))
	for _, it := range res.Items {
		changed = append(changed, p.Item(it.ID))
	}
	// Mantém os metadados do AI-PRD compilado (stack, hash) no tasks.json.
	if prev, err := a.st.LoadTasks(); err == nil {
		prev.TargetStack, prev.SourceHash, prev.GeneratedAt = compiled.Board.TargetStack, compiled.Board.SourceHash, compiled.Board.GeneratedAt
		_ = a.st.SaveTasks(prev)
	}
	msg := fmt.Sprintf("Backlog sincronizado: %d novo(s), %d atualizado(s), %d arquivado(s)",
		out.Counts[plan.ChangeAdded], out.Counts[plan.ChangeUpdated], out.Counts[plan.ChangeArchived])
	if err := a.commitPlan(p, changed, nil, source, msg); err != nil {
		return nil, err
	}
	out.Total = len(p.Items)
	return out, nil
}

// ---------------------------------------------------------------------------
// Itens
// ---------------------------------------------------------------------------

// ItemInput cria ou altera um item. Campos nil ficam como estão.
type ItemInput struct {
	Type          *string            `json:"type,omitempty"`
	Title         *string            `json:"title,omitempty"`
	Description   *string            `json:"description,omitempty"`
	Status        *string            `json:"status,omitempty"`
	Priority      *string            `json:"priority,omitempty"`
	Sprint        *int               `json:"sprint,omitempty"`
	Parent        *string            `json:"parent,omitempty"`
	Assignee      *string            `json:"assignee,omitempty"`
	EstimateHours *float64           `json:"estimate_h,omitempty"`
	ComponentID   *string            `json:"component_id,omitempty"`
	Dependencies  *[]string          `json:"dependencies,omitempty"`
	Requirements  *[]string          `json:"requirements,omitempty"`
	Acceptance    *[]model.Criterion `json:"acceptance,omitempty"`
	Notes         *string            `json:"notes,omitempty"`
	Archived      *bool              `json:"archived,omitempty"`
}

// generatedFields são os campos que a sincronização preenche; editá-los à
// mão marca o override.
var generatedFields = map[string]bool{
	"title": true, "description": true, "priority": true, "parent": true, "tier": true, "component": true,
	"component_id": true, "dependencies": true, "requirements": true, "estimate_h": true, "acceptance": true,
}

// CreateItem cria um item à mão (bug, débito, spike, tarefa…).
func (a *App) CreateItem(in ItemInput, source string) (*model.WorkItem, error) {
	if in.Title == nil || strings.TrimSpace(*in.Title) == "" {
		return nil, model.Invalid("informe o título do item")
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	itemType := model.ItemTask
	if in.Type != nil {
		itemType = model.NormalizeItemType(*in.Type)
	}
	now := a.timestamp()
	last := ""
	for _, it := range p.Items {
		if it.Rank > last {
			last = it.Rank
		}
	}
	item := &model.WorkItem{ID: p.NextItemID(itemType), Type: itemType, Status: model.StatusPending,
		Rank: plan.After(last), CreatedAt: now, UpdatedAt: now}
	if err := a.applyItemInput(p, item, in, false); err != nil {
		return nil, err
	}
	p.Items = append(p.Items, *item)
	saved := p.Item(item.ID)
	if err := a.commitPlan(p, []*model.WorkItem{saved}, nil, source,
		fmt.Sprintf("%s criado: %s", saved.ID, saved.Title)); err != nil {
		return nil, err
	}
	return saved, nil
}

// UpdateItem altera um item. Em itens gerados, mudar um campo que vem da
// arquitetura marca o override para a sincronização não desfazer.
func (a *App) UpdateItem(id string, in ItemInput, source string) (*model.WorkItem, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	if err := a.applyItemInput(p, item, in, item.Source != ""); err != nil {
		return nil, err
	}
	item.UpdatedAt = a.timestamp()
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source,
		fmt.Sprintf("%s atualizado", item.ID)); err != nil {
		return nil, err
	}
	return item, nil
}

func (a *App) applyItemInput(p *model.Plan, item *model.WorkItem, in ItemInput, trackOverrides bool) error {
	mark := func(field string) {
		if trackOverrides && generatedFields[field] {
			item.MarkOverride(field)
		}
	}
	if in.Type != nil {
		item.Type = model.NormalizeItemType(*in.Type)
	}
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if t == "" {
			return model.Invalid("o título não pode ficar vazio")
		}
		if t != item.Title {
			item.Title = t
			mark("title")
		}
	}
	if in.Description != nil && *in.Description != item.Description {
		item.Description = strings.TrimSpace(*in.Description)
		mark("description")
	}
	if in.Priority != nil {
		pr := model.NormalizePriority(*in.Priority)
		if pr != item.Priority {
			item.Priority = pr
			mark("priority")
		}
	}
	if in.Sprint != nil {
		if *in.Sprint > 0 && p.Sprint(*in.Sprint) == nil {
			return model.Invalid("a %s não existe (crie a sprint primeiro)", model.SprintName(*in.Sprint))
		}
		if *in.Sprint < 0 {
			return model.Invalid("número de sprint inválido")
		}
		item.Sprint = *in.Sprint
	}
	if in.Parent != nil {
		parent := strings.ToUpper(strings.TrimSpace(*in.Parent))
		if parent != "" && p.Item(parent) == nil {
			return model.Invalid("o item pai %s não existe", parent)
		}
		if parent != item.Parent {
			item.Parent = parent
			mark("parent")
		}
	}
	if in.Assignee != nil {
		item.Assignee = strings.TrimSpace(*in.Assignee)
	}
	if in.EstimateHours != nil && *in.EstimateHours != item.EstimateHours {
		if *in.EstimateHours < 0 {
			return model.Invalid("estimativa negativa")
		}
		item.EstimateHours = *in.EstimateHours
		mark("estimate_h")
	}
	if in.ComponentID != nil && *in.ComponentID != item.ComponentID {
		item.ComponentID = strings.TrimSpace(*in.ComponentID)
		item.Component, item.Tier = "", ""
		if item.ComponentID != "" {
			d, err := a.st.LoadDiagram()
			if err != nil {
				return err
			}
			node := d.NodeByID(item.ComponentID)
			if node == nil {
				return model.Invalid("componente %q não existe no diagrama", item.ComponentID)
			}
			item.Component = node.Data.Label
			item.Tier = layout.ResolveTier(*node)
		}
		mark("component_id")
		mark("component")
		mark("tier")
	}
	if in.Dependencies != nil {
		deps := []string{}
		for _, dep := range *in.Dependencies {
			dep = strings.ToUpper(strings.TrimSpace(dep))
			if dep == "" {
				continue
			}
			if dep == item.ID {
				return model.Invalid("um item não pode depender de si mesmo")
			}
			if p.Item(dep) == nil {
				return model.Invalid("a dependência %s não existe", dep)
			}
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		item.Dependencies = deps
		mark("dependencies")
	}
	if in.Requirements != nil {
		item.Requirements = upperTrim(*in.Requirements)
		mark("requirements")
	}
	if in.Acceptance != nil {
		list := []model.Criterion{}
		for _, c := range *in.Acceptance {
			if c.Text = strings.TrimSpace(c.Text); c.Text != "" {
				list = append(list, c)
			}
		}
		item.Acceptance = list
		mark("acceptance")
	}
	if in.Notes != nil {
		item.Notes = strings.TrimSpace(*in.Notes)
	}
	if in.Status != nil {
		a.setStatus(item, *in.Status)
	}
	if in.Archived != nil {
		item.Archived = *in.Archived
		if item.Archived && item.ArchiveReason == "" {
			item.ArchiveReason = "arquivado à mão"
		}
		if !item.Archived {
			item.ArchiveReason = ""
		}
	}
	return nil
}

// setStatus muda o estado e registra os carimbos de tempo do ciclo de vida.
func (a *App) setStatus(item *model.WorkItem, status string) {
	s := model.NormalizeStatus(status)
	if s == item.Status {
		return
	}
	item.Status = s
	switch s {
	case model.StatusCompleted:
		item.CompletedAt = a.timestamp()
	case model.StatusPending:
		item.CompletedAt = ""
	default:
		item.CompletedAt = ""
	}
}

// MoveInput reposiciona um item no backlog ou no quadro.
type MoveInput struct {
	// Before/After são os ids dos vizinhos na nova posição ("" = ponta).
	Before string  `json:"before,omitempty"`
	After  string  `json:"after,omitempty"`
	Sprint *int    `json:"sprint,omitempty"`
	Status *string `json:"status,omitempty"`
}

// MoveItem muda a posição (rank), a sprint e/ou o status de um item. Só o
// arquivo do item muda.
func (a *App) MoveItem(id string, in MoveInput, source string) (*model.WorkItem, error) {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	if in.Before != "" || in.After != "" {
		lo, hi := "", ""
		if in.After != "" {
			n := p.Item(in.After)
			if n == nil {
				return nil, itemNotFound(p, in.After)
			}
			lo = n.Rank
		}
		if in.Before != "" {
			n := p.Item(in.Before)
			if n == nil {
				return nil, itemNotFound(p, in.Before)
			}
			hi = n.Rank
		}
		item.Rank = plan.Between(lo, hi)
	}
	if err := a.applyItemInput(p, item, ItemInput{Sprint: in.Sprint, Status: in.Status}, false); err != nil {
		return nil, err
	}
	item.UpdatedAt = a.timestamp()
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source, fmt.Sprintf("%s movido", item.ID)); err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem apaga um item criado à mão. Itens gerados são arquivados (a
// sincronização os recriaria).
func (a *App) DeleteItem(id, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()
	p, err := a.loadPlan()
	if err != nil {
		return err
	}
	item := p.Item(id)
	if item == nil {
		return itemNotFound(p, id)
	}
	for _, other := range p.Items {
		if other.ID != item.ID && !other.Archived && containsID(other.Dependencies, item.ID) {
			return model.Invalid("%s é dependência de %s; remova a dependência antes", item.ID, other.ID)
		}
	}
	if item.Source != "" {
		item.Archived, item.ArchiveReason, item.UpdatedAt = true, "arquivado à mão", a.timestamp()
		return a.commitPlan(p, []*model.WorkItem{item}, nil, source, item.ID+" arquivado")
	}
	if err := a.st.RemoveItem(item); err != nil {
		return err
	}
	out := p.Items[:0]
	for _, it := range p.Items {
		if it.ID != item.ID {
			out = append(out, it)
		}
	}
	p.Items = out
	return a.commitPlan(p, nil, nil, source, id+" excluído")
}

// Item devolve um item pelo id.
func (a *App) Item(id string) (*model.WorkItem, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	item := p.Item(id)
	if item == nil {
		return nil, itemNotFound(p, id)
	}
	return item, nil
}

// BacklogQuery filtra o backlog.
type BacklogQuery struct {
	Status    string // pending | in_progress | review | completed | blocked | open | all
	Sprint    *int   // nil = qualquer; 0 = sem sprint
	Type      string
	Assignee  string
	OnlyReady bool
	Archived  bool
	Limit     int
}

// BacklogView é um item com a prontidão calculada.
type BacklogView struct {
	model.WorkItem
	Ready     bool     `json:"ready"`
	BlockedBy []string `json:"blocked_by,omitempty"`
	Stale     bool     `json:"stale,omitempty"`
}

// Backlog lista os itens filtrados, na ordem do rank.
func (a *App) Backlog(q BacklogQuery) ([]BacklogView, int, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, 0, err
	}
	conv, _ := a.st.LoadConventions()
	status := strings.ToLower(strings.TrimSpace(q.Status))
	out := []BacklogView{}
	total := 0
	for i := range p.Items {
		it := &p.Items[i]
		if it.Archived != q.Archived {
			continue
		}
		switch status {
		case "", "all":
		case "open":
			if it.Status == model.StatusCompleted {
				continue
			}
		default:
			if it.Status != model.NormalizeStatus(status) {
				continue
			}
		}
		if q.Sprint != nil && it.Sprint != *q.Sprint {
			continue
		}
		if q.Type != "" && it.Type != model.NormalizeItemType(q.Type) {
			continue
		}
		if q.Assignee != "" && !plan.SamePerson(it.Assignee, q.Assignee) {
			continue
		}
		blocked := plan.BlockedBy(it, p)
		if q.OnlyReady && (len(blocked) > 0 || !it.Actionable()) {
			continue
		}
		total++
		if q.Limit > 0 && len(out) >= q.Limit {
			continue
		}
		out = append(out, BacklogView{WorkItem: *it, Ready: len(blocked) == 0, BlockedBy: blocked,
			Stale: plan.Stale(it, a.now(), conv.Planning.StaleDays)})
	}
	return out, total, nil
}

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------

func itemNotFound(p *model.Plan, id string) error {
	sample := []string{}
	for _, it := range p.Items {
		if it.Actionable() && !it.Archived {
			sample = append(sample, it.ID)
		}
		if len(sample) == 8 {
			break
		}
	}
	if len(sample) == 0 {
		return model.NotFound("item %q não encontrado: o backlog está vazio (rode sync_backlog ou `archcode-studio backlog sync`)", id)
	}
	return model.NotFound("item %q não encontrado (existentes: %s…)", id, strings.Join(sample, ", "))
}

func containsID(list []string, id string) bool {
	for _, v := range list {
		if strings.EqualFold(v, id) {
			return true
		}
	}
	return false
}

func upperTrim(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
			out = append(out, s)
		}
	}
	return out
}
