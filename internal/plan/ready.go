package plan

import (
	"strings"
	"time"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Prontidão, próxima tarefa e reservas paradas
// ---------------------------------------------------------------------------

// BlockedBy devolve as dependências ainda não concluídas do item.
// Dependências arquivadas (componente removido) ou inexistentes não travam.
func BlockedBy(item *model.WorkItem, p *model.Plan) []string {
	out := []string{}
	for _, dep := range item.Dependencies {
		d := p.Item(dep)
		if d == nil || d.Archived || d.Status == model.StatusCompleted {
			continue
		}
		out = append(out, d.ID)
	}
	return out
}

// Ready informa se todas as dependências do item estão concluídas.
func Ready(item *model.WorkItem, p *model.Plan) bool { return len(BlockedBy(item, p)) == 0 }

// SamePerson compara identidades "Nome <email>" pelo e-mail (ou pelo nome,
// quando não há e-mail).
func SamePerson(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ea, eb := emailOf(a), emailOf(b)
	if ea != "" && eb != "" {
		return strings.EqualFold(ea, eb)
	}
	return strings.EqualFold(nameOf(a), nameOf(b))
}

func emailOf(id string) string {
	if i := strings.Index(id, "<"); i >= 0 {
		if j := strings.Index(id[i:], ">"); j > 0 {
			return strings.TrimSpace(id[i+1 : i+j])
		}
	}
	if strings.Contains(id, "@") && !strings.Contains(id, " ") {
		return id
	}
	return ""
}

func nameOf(id string) string {
	if i := strings.Index(id, "<"); i >= 0 {
		return strings.TrimSpace(id[:i])
	}
	return strings.TrimSpace(id)
}

// DisplayName devolve só o nome de uma identidade "Nome <email>".
func DisplayName(id string) string {
	if n := nameOf(id); n != "" {
		return n
	}
	return id
}

// NextReady escolhe a próxima tarefa para a pessoa: pendente, pronta, sem
// dono (ou dela), priorizando a sprint ativa e depois o rank do backlog.
func NextReady(p *model.Plan, person string) *model.WorkItem {
	active := p.ActiveSprint()
	pick := func(inSprint func(*model.WorkItem) bool) *model.WorkItem {
		for i := range p.Items {
			it := &p.Items[i]
			if !it.Actionable() || it.Archived || it.Status != model.StatusPending || !inSprint(it) {
				continue
			}
			if it.Assignee != "" && !SamePerson(it.Assignee, person) {
				continue
			}
			if Ready(it, p) {
				return it
			}
		}
		return nil
	}
	if active != nil {
		if it := pick(func(it *model.WorkItem) bool { return it.Sprint == active.Number }); it != nil {
			return it
		}
	}
	return pick(func(it *model.WorkItem) bool {
		// Sem sprint ativa (ou com ela esgotada), vale o backlog sem sprint.
		return it.Sprint == 0 || active == nil
	})
}

// MyWork devolve os itens em andamento, em revisão ou bloqueados da pessoa.
func MyWork(p *model.Plan, person string) []*model.WorkItem {
	out := []*model.WorkItem{}
	for i := range p.Items {
		it := &p.Items[i]
		if it.Archived || !SamePerson(it.Assignee, person) {
			continue
		}
		switch it.Status {
		case model.StatusInProgress, model.StatusReview, model.StatusBlocked:
			out = append(out, it)
		}
	}
	return out
}

// LastActivity devolve o momento da última atividade registrada no item.
func LastActivity(it *model.WorkItem) time.Time {
	var last time.Time
	for _, s := range []string{it.ClaimedAt, it.UpdatedAt, handoffTime(it)} {
		if t, ok := parseTime(s); ok && t.After(last) {
			last = t
		}
	}
	return last
}

func handoffTime(it *model.WorkItem) string {
	if it.Handoff == nil {
		return ""
	}
	return it.Handoff.UpdatedAt
}

// Stale informa se a reserva está parada: em andamento, com dono e sem
// atividade há mais de staleDays dias úteis (RF057).
func Stale(it *model.WorkItem, now time.Time, staleDays int) bool {
	if it.Status != model.StatusInProgress || it.Assignee == "" || staleDays <= 0 {
		return false
	}
	last := LastActivity(it)
	if last.IsZero() {
		return false
	}
	return WorkingDaysBetween(last, now) > staleDays
}

func parseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
