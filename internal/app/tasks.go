package app

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
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
}

func (a *App) GenerateAIPRD(opts prd.Options, source string) (*PRDResult, error) {
	opts = opts.Normalize()
	a.tx.Lock()
	defer a.tx.Unlock()

	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	res := prd.Compile(prd.Input{
		Manifest:     snap.Manifest,
		Diagram:      snap.Diagram,
		Requirements: snap.Requirements,
		UseCases:     snap.UseCases,
		ADRs:         snap.ADRs,
		Endpoints:    snap.Endpoints,
		UMLDiagrams:  snap.UMLDiagrams,
		Previous:     snap.Tasks,
	}, opts)

	if err := a.st.WriteFile(store.FileAIPRD, []byte(res.Markdown)); err != nil {
		return nil, err
	}
	if err := a.st.SaveTasks(res.Board); err != nil {
		return nil, err
	}
	if prd.SyncNodeStatus(snap.Diagram, res.Board) {
		_ = a.saveDiagram(snap.Diagram)
	}

	a.emit(hub.Event{
		Type: hub.EventPRD, Source: source, Path: store.FileAIPRD,
		Message: fmt.Sprintf("AI-PRD gerado com %d tarefas", len(res.Board.Tasks)),
		Payload: map[string]any{"total_tasks": len(res.Board.Tasks), "hash": res.Hash},
	})
	return &PRDResult{
		FilePath:   store.FileAIPRD,
		TotalTasks: len(res.Board.Tasks),
		Hash:       res.Hash,
		Progress:   res.Board.ProgressPercentage(),
		Summary:    fmt.Sprintf("PRD para IA gerado com %d tarefas em ordem topológica.", len(res.Board.Tasks)),
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
	// Status: pending | in_progress | completed | blocked | all ("" = todas).
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

func (a *App) ImplementationTasks(q TaskQuery) (*TaskList, error) {
	board, err := a.st.LoadTasks()
	if err != nil {
		return nil, err
	}
	byID := map[string]model.Task{}
	for _, t := range board.Tasks {
		byID[t.ID] = t
	}
	out := []TaskView{}
	filter := strings.ToLower(strings.TrimSpace(q.Status))
	for _, t := range board.Tasks {
		if filter != "" && filter != "all" && t.Status != filter {
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

func (a *App) MarkTaskStatus(id, status, notes, source string) (*TaskUpdateResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	board, err := a.st.LoadTasks()
	if err != nil {
		return nil, err
	}
	task := board.ByID(id)
	if task == nil {
		available := []string{}
		for _, t := range board.Tasks {
			available = append(available, t.ID)
		}
		if len(available) > 8 {
			available = available[:8]
		}
		return nil, model.NotFound("tarefa %q não encontrada (existentes: %s…); rode generate_ai_prd primeiro",
			id, strings.Join(available, ", "))
	}
	task.Status = model.NormalizeStatus(status)
	if notes != "" {
		task.Notes = notes
	}
	task.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := a.st.SaveTasks(board); err != nil {
		return nil, err
	}

	// Reflete o progresso no canvas (RF022).
	if d, err := a.st.LoadDiagram(); err == nil {
		if prd.SyncNodeStatus(d, board) {
			_ = a.saveDiagram(d)
		}
	}
	// Atualiza os checkboxes do ai-prd.md, se ele existir.
	a.refreshPRDCheckboxes(board)

	a.emit(hub.Event{
		Type: hub.EventTasks, Source: source, Path: store.FileTasks,
		Message: fmt.Sprintf("%s → %s", task.ID, task.Status),
		Payload: map[string]any{"task_id": task.ID, "status": task.Status,
			"progress": board.ProgressPercentage()},
	})
	return &TaskUpdateResult{
		TaskID: task.ID, Updated: true, Status: task.Status,
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
