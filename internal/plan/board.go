package plan

import (
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Compatibilidade com .arch/tasks.json (RNF012)
// ---------------------------------------------------------------------------
//
// O backlog em .arch/plan/ é a fonte da verdade. O tasks.json continua
// existindo como índice derivado, no formato de sempre, para agentes e
// integrações que já o leem.

// ToBoard deriva o quadro de tarefas do backlog: itens executáveis e não
// arquivados, na ordem do rank. Metadados (stack, hash) vêm do quadro anterior.
func ToBoard(p *model.Plan, prev *model.TaskBoard) *model.TaskBoard {
	b := model.NewTaskBoard()
	if prev != nil {
		b.GeneratedAt, b.SourceHash, b.TargetStack = prev.GeneratedAt, prev.SourceHash, prev.TargetStack
	}
	order := 0
	for _, it := range p.Items {
		if !it.Actionable() || it.Archived {
			continue
		}
		order++
		acceptance := make([]string, 0, len(it.Acceptance))
		for _, c := range it.Acceptance {
			acceptance = append(acceptance, c.Text)
		}
		b.Tasks = append(b.Tasks, model.Task{
			ID:           it.ID,
			Title:        it.Title,
			Tier:         firstNonEmpty(it.Tier, tierOfType(it.Type)),
			Component:    it.Component,
			ComponentID:  it.ComponentID,
			Dependencies: it.Dependencies,
			Acceptance:   acceptance,
			Requirements: it.Requirements,
			UseCases:     it.UseCases,
			Endpoints:    it.Endpoints,
			Status:       it.Status,
			Notes:        boardNotes(&it),
			UpdatedAt:    it.UpdatedAt,
			Order:        order,
		})
	}
	return b
}

func tierOfType(t string) string {
	switch t {
	case model.ItemBug, model.ItemDebt, model.ItemSecurity:
		return "backend"
	default:
		return "devops"
	}
}

// boardNotes resume o checkpoint e as notas na coluna notes do tasks.json.
func boardNotes(it *model.WorkItem) string {
	parts := []string{}
	if h := it.Handoff; !h.Empty() && h.NextStep != "" {
		parts = append(parts, "Próximo passo: "+h.NextStep)
	}
	if n := strings.TrimSpace(it.Notes); n != "" {
		parts = append(parts, n)
	}
	return strings.Join(parts, "\n")
}

// FromBoard converte um tasks.json de uma versão anterior em itens do
// backlog, preservando ids, status e notas (migração da primeira abertura).
func FromBoard(b *model.TaskBoard, now string) []model.WorkItem {
	ranks := Spread(len(b.Tasks))
	out := make([]model.WorkItem, 0, len(b.Tasks))
	for i, t := range b.Tasks {
		item := model.WorkItem{
			ID:           strings.ToUpper(t.ID),
			Type:         model.ItemTask,
			Title:        t.Title,
			Status:       model.NormalizeStatus(t.Status),
			Rank:         ranks[i],
			Tier:         t.Tier,
			Component:    t.Component,
			ComponentID:  t.ComponentID,
			Dependencies: t.Dependencies,
			Requirements: upperAll(t.Requirements),
			UseCases:     t.UseCases,
			Endpoints:    t.Endpoints,
			Acceptance:   criteria(t.Acceptance),
			Notes:        strings.TrimSpace(t.Notes),
			Order:        t.Order,
			CreatedAt:    now,
			UpdatedAt:    firstNonEmpty(t.UpdatedAt, now),
		}
		switch {
		case t.ComponentID != "":
			item.Source = SourceComponent + t.ComponentID
		case strings.HasPrefix(item.ID, "TASK-E2E"):
			item.Source = SourceE2E
		}
		if item.Status == model.StatusCompleted {
			item.CompletedAt = item.UpdatedAt
			for j := range item.Acceptance {
				item.Acceptance[j].Done = true
			}
		}
		if !model.ValidItemID(item.ID) {
			item.ID = fmt.Sprintf("TASK-%03d", i+1)
		}
		out = append(out, item)
	}
	return out
}

// ---------------------------------------------------------------------------
// Diagnóstico (doctor)
// ---------------------------------------------------------------------------

// Problems devolve inconsistências do backlog: dependências e pais
// inexistentes, sprints inexistentes e mais de uma sprint ativa.
func Problems(p *model.Plan) []string {
	out := append([]string{}, p.Warnings...)
	active := 0
	for _, s := range p.Sprints {
		if s.Status == model.SprintActive {
			active++
		}
	}
	if active > 1 {
		out = append(out, fmt.Sprintf("%d sprints ativas ao mesmo tempo (só uma é permitida)", active))
	}
	for _, it := range p.Items {
		for _, dep := range it.Dependencies {
			if p.Item(dep) == nil {
				out = append(out, fmt.Sprintf("%s depende de %s, que não existe", it.ID, dep))
			}
		}
		if it.Parent != "" && p.Item(it.Parent) == nil {
			out = append(out, fmt.Sprintf("%s aponta para o pai %s, que não existe", it.ID, it.Parent))
		}
		if it.Sprint > 0 && p.Sprint(it.Sprint) == nil {
			out = append(out, fmt.Sprintf("%s está na %s, que não existe", it.ID, model.SprintName(it.Sprint)))
		}
	}
	return out
}
