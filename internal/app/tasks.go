package app

import (
	"fmt"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/plan"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Compilador de PRD para IAs
// ---------------------------------------------------------------------------

type PRDResult struct {
	FilePath   string `json:"file_path"`
	TotalTasks int    `json:"total_tasks"`
	Hash       string `json:"hash"`
	Progress   int    `json:"overall_progress_percentage"`
	Summary    string `json:"summary"`
	// Backlog resume o que a compilação mudou no backlog.
	Backlog map[string]int `json:"backlog,omitempty"`
}

// GenerateAIPRD compila o blueprint docs/ai-prd.md e sincroniza o backlog com
// as mesmas tarefas: os ids vêm do backlog, então renomear um componente não
// perde o status da tarefa dele.
func (a *App) GenerateAIPRD(opts prd.Options, source string) (*PRDResult, error) {
	opts = opts.Normalize()
	a.tx.Lock()
	defer a.tx.Unlock()

	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	conv, err := a.st.LoadConventions()
	if err != nil {
		return nil, err
	}
	res := a.compileBoard(snap, p, conv, opts)
	if err := a.st.WriteFile(store.FileAIPRD, []byte(res.Markdown)); err != nil {
		return nil, err
	}
	sync, err := a.syncBacklogLocked(false, source, res)
	if err != nil {
		return nil, err
	}
	board, _ := a.st.LoadTasks()

	a.emit(hub.Event{
		Type: hub.EventPRD, Source: source, Path: store.FileAIPRD,
		Message: fmt.Sprintf("AI-PRD gerado com %d tarefas", len(board.Tasks)),
		Payload: map[string]any{"total_tasks": len(board.Tasks), "hash": res.Hash},
	})
	return &PRDResult{
		FilePath:   store.FileAIPRD,
		TotalTasks: len(board.Tasks),
		Hash:       res.Hash,
		Progress:   board.ProgressPercentage(),
		Summary:    fmt.Sprintf("PRD para IA gerado com %d tarefas em ordem topológica.", len(board.Tasks)),
		Backlog:    sync.Counts,
	}, nil
}

// ImplementationTasks devolve a fila de tarefas, opcionalmente filtrada, já
// anotada com o campo `ready` (todas as dependências concluídas).
type TaskView struct {
	model.Task
	Ready     bool     `json:"ready"`
	BlockedBy []string `json:"blocked_by,omitempty"`
}

// TaskQuery filtra a fila de implementação.
type TaskQuery struct {
	// Status: pending | in_progress | review | completed | blocked | all ("" = todas).
	Status string
	// OnlyReady mantém só tarefas com todas as dependências concluídas.
	OnlyReady bool
	// Limit corta a lista (0 = sem limite).
	Limit int
}

// TaskList é a fila filtrada, com o quadro completo para progresso e metadados.
type TaskList struct {
	Tasks     []TaskView
	Board     *model.TaskBoard
	Truncated bool
}

// ImplementationTasks lê a fila do backlog, no formato do tasks.json.
func (a *App) ImplementationTasks(q TaskQuery) (*TaskList, error) {
	p, err := a.Plan()
	if err != nil {
		return nil, err
	}
	prev, _ := a.st.LoadTasks()
	board := plan.ToBoard(p, prev)
	byID := map[string]model.Task{}
	for _, t := range board.Tasks {
		byID[t.ID] = t
	}
	out := []TaskView{}
	filter := strings.ToLower(strings.TrimSpace(q.Status))
	for _, t := range board.Tasks {
		if filter != "" && filter != "all" && t.Status != model.NormalizeStatus(filter) {
			continue
		}
		blocked := []string{}
		for _, dep := range t.Dependencies {
			if d, ok := byID[dep]; ok && d.Status != model.StatusCompleted {
				blocked = append(blocked, dep)
			}
		}
		if q.OnlyReady && len(blocked) > 0 {
			continue
		}
		out = append(out, TaskView{Task: t, Ready: len(blocked) == 0, BlockedBy: blocked})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	list := &TaskList{Tasks: out, Board: board}
	if q.Limit > 0 && len(out) > q.Limit {
		list.Tasks, list.Truncated = out[:q.Limit], true
	}
	return list, nil
}

type TaskUpdateResult struct {
	TaskID   string `json:"task_id"`
	Updated  bool   `json:"updated"`
	Status   string `json:"status"`
	Progress int    `json:"overall_progress_percentage"`
}

// MarkTaskStatus muda o status de uma tarefa (ferramenta legada, mantida para
// agentes e integrações que usam o tasks.json). As notas entram datadas nas
// notas do item.
func (a *App) MarkTaskStatus(id, status, notes, source string) (*TaskUpdateResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	p, err := a.loadPlan()
	if err != nil {
		return nil, err
	}
	item := p.Item(id)
	if item == nil || !item.Actionable() {
		if len(p.Items) == 0 {
			return nil, model.NotFound("tarefa %q não encontrada; rode generate_ai_prd primeiro", id)
		}
		return nil, itemNotFound(p, id)
	}
	a.setStatus(item, status)
	if notes = strings.TrimSpace(notes); notes != "" {
		line := fmt.Sprintf("- %s (%s): %s", a.timestamp()[:10], plan.StatusLabels[item.Status], notes)
		item.Notes = strings.TrimSpace(item.Notes + "\n" + line)
	}
	item.UpdatedAt = a.timestamp()
	if err := a.commitPlan(p, []*model.WorkItem{item}, nil, source,
		fmt.Sprintf("%s → %s", item.ID, item.Status)); err != nil {
		return nil, err
	}
	board, _ := a.st.LoadTasks()
	return &TaskUpdateResult{
		TaskID: item.ID, Updated: true, Status: item.Status,
		Progress: board.ProgressPercentage(),
	}, nil
}

// refreshPRDCheckboxes atualiza as caixas de seleção do ai-prd.md com o
// status das tarefas.
func (a *App) refreshPRDCheckboxes(board *model.TaskBoard) {
	data, err := a.st.ReadFile(store.FileAIPRD)
	if err != nil {
		return
	}
	if md, changed := prd.RefreshCheckboxes(string(data), board); changed {
		_ = a.st.WriteFile(store.FileAIPRD, []byte(md))
	}
}
