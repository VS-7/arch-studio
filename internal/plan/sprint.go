package plan

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Datas, capacidade e carga das sprints (RF033)
// ---------------------------------------------------------------------------

const dateLayout = "2006-01-02"

// ParseDate lê uma data AAAA-MM-DD.
func ParseDate(s string) (time.Time, bool) {
	t, err := time.Parse(dateLayout, strings.TrimSpace(s))
	return t, err == nil
}

// FormatDate formata uma data como AAAA-MM-DD.
func FormatDate(t time.Time) string { return t.Format(dateLayout) }

func weekday(t time.Time) bool { return t.Weekday() != time.Saturday && t.Weekday() != time.Sunday }

// WorkingDays conta os dias úteis (segunda a sexta) entre start e end,
// incluindo os dois extremos.
func WorkingDays(start, end time.Time) int {
	start, end = dayOf(start), dayOf(end)
	if end.Before(start) {
		return 0
	}
	n := 0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if weekday(d) {
			n++
		}
	}
	return n
}

// WorkingDaysBetween conta os dias úteis decorridos de from até to, sem
// contar o dia de from.
func WorkingDaysBetween(from, to time.Time) int {
	from, to = dayOf(from), dayOf(to)
	n := 0
	for d := from.AddDate(0, 0, 1); !d.After(to); d = d.AddDate(0, 0, 1) {
		if weekday(d) {
			n++
		}
	}
	return n
}

// NextWorkingDay devolve t, ou a próxima segunda-feira se t cair no fim de semana.
func NextWorkingDay(t time.Time) time.Time {
	t = dayOf(t)
	for !weekday(t) {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

// EndAfter devolve o dia em que termina uma sprint de days dias úteis que
// começa em start.
func EndAfter(start time.Time, days int) time.Time {
	d := NextWorkingDay(start)
	for counted := 1; counted < days; {
		d = d.AddDate(0, 0, 1)
		if weekday(d) {
			counted++
		}
	}
	return d
}

func dayOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Capacity calcula a capacidade em horas: pessoas × horas por dia × dias
// úteis entre o início e o fim da sprint.
func Capacity(start, end string, teamSize, hoursPerDay float64) float64 {
	s, ok1 := ParseDate(start)
	e, ok2 := ParseDate(end)
	if !ok1 || !ok2 || teamSize <= 0 || hoursPerDay <= 0 {
		return 0
	}
	return math.Round(teamSize * hoursPerDay * float64(WorkingDays(s, e)))
}

// DaysLeft conta os dias úteis que faltam até o fim da sprint (0 se acabou).
func DaysLeft(sp *model.Sprint, now time.Time) int {
	end, ok := ParseDate(sp.End)
	if !ok {
		return 0
	}
	if dayOf(now).After(end) {
		return 0
	}
	return WorkingDays(now, end)
}

// ---------------------------------------------------------------------------
// Estatísticas
// ---------------------------------------------------------------------------

// Stats resume uma sprint (ou o backlog, com número 0).
type Stats struct {
	Total        int     `json:"total"`
	Pending      int     `json:"pending"`
	InProgress   int     `json:"in_progress"`
	Review       int     `json:"review"`
	Completed    int     `json:"completed"`
	Blocked      int     `json:"blocked"`
	PlannedHours float64 `json:"planned_hours"`
	DoneHours    float64 `json:"done_hours"`
	Progress     int     `json:"progress"`
}

// SprintStats conta os itens executáveis e não arquivados da sprint.
func SprintStats(p *model.Plan, number int) Stats {
	s := Stats{}
	for _, it := range p.Items {
		if !it.Actionable() || it.Archived || it.Sprint != number {
			continue
		}
		s.Total++
		s.PlannedHours += it.EstimateHours
		switch it.Status {
		case model.StatusInProgress:
			s.InProgress++
		case model.StatusReview:
			s.Review++
		case model.StatusCompleted:
			s.Completed++
			s.DoneHours += it.EstimateHours
		case model.StatusBlocked:
			s.Blocked++
		default:
			s.Pending++
		}
	}
	if s.Total > 0 {
		s.Progress = int(float64(s.Completed)/float64(s.Total)*100 + 0.5)
	}
	return s
}

// ---------------------------------------------------------------------------
// Proposta de sprint
// ---------------------------------------------------------------------------

// unknownEstimate é a carga assumida para um item sem estimativa.
const unknownEstimate = 4

// Proposal é a sugestão de itens para uma sprint.
type Proposal struct {
	Items         []string `json:"items"`
	Hours         float64  `json:"hours"`
	CapacityHours float64  `json:"capacity_h"`
	// Skipped lista itens prontos que não couberam.
	Skipped []string `json:"skipped,omitempty"`
}

// Propose escolhe, na ordem do backlog, itens pendentes sem sprint que cabem
// na capacidade e cujas dependências estão concluídas, já na sprint ou
// escolhidas antes. Itens que já estão na sprint contam na carga.
func Propose(p *model.Plan, number int, capacity float64) *Proposal {
	prop := &Proposal{Items: []string{}, CapacityHours: capacity}
	chosen := map[string]bool{}
	for _, it := range p.Items {
		if it.Actionable() && !it.Archived && it.Sprint == number && number > 0 {
			chosen[it.ID] = true
			prop.Hours += estimateOf(&it)
		}
	}
	for i := range p.Items {
		it := &p.Items[i]
		if !it.Actionable() || it.Archived || it.Sprint != 0 || it.Status != model.StatusPending {
			continue
		}
		ok := true
		for _, dep := range BlockedBy(it, p) {
			if !chosen[dep] {
				ok = false
				break
			}
		}
		if !ok {
			continue
		}
		h := estimateOf(it)
		if capacity > 0 && prop.Hours+h > capacity {
			prop.Skipped = append(prop.Skipped, it.ID)
			continue
		}
		chosen[it.ID] = true
		prop.Items = append(prop.Items, it.ID)
		prop.Hours += h
	}
	return prop
}

func estimateOf(it *model.WorkItem) float64 {
	if it.EstimateHours > 0 {
		return it.EstimateHours
	}
	return unknownEstimate
}

// ---------------------------------------------------------------------------
// Relatório de encerramento (RF035)
// ---------------------------------------------------------------------------

// CommitInfo é um commit citado no relatório.
type CommitInfo struct {
	Hash    string
	Subject string
	Author  string
	Date    string
}

// ReportInput reúne o que o relatório da sprint mostra.
type ReportInput struct {
	Plan     *model.Plan
	Sprint   *model.Sprint
	Commits  []CommitInfo
	Sessions []model.Session
	// CarriedTo é a sprint que recebeu os itens não concluídos (0 = backlog).
	CarriedTo int
	Carried   []string
}

// Report gera docs/sprints/sprint-NN.md.
func Report(in ReportInput) string {
	sp := in.Sprint
	stats := SprintStats(in.Plan, sp.Number)
	var b strings.Builder
	fmt.Fprintf(&b, "# Relatório da %s\n\n", sp.Name)
	if sp.Goal != "" {
		fmt.Fprintf(&b, "**Meta:** %s\n\n", sp.Goal)
	}
	fmt.Fprintf(&b, "**Período:** %s a %s", orDash(sp.Start), orDash(sp.End))
	if sp.ClosedAt != "" {
		fmt.Fprintf(&b, " · encerrada em %s", sp.ClosedAt[:min(10, len(sp.ClosedAt))])
	}
	b.WriteString("\n\n## Resultado\n\n")
	b.WriteString("| Indicador | Valor |\n| --- | --- |\n")
	fmt.Fprintf(&b, "| Itens planejados | %d |\n", stats.Total)
	fmt.Fprintf(&b, "| Concluídos | %d (%d%%) |\n", stats.Completed, stats.Progress)
	fmt.Fprintf(&b, "| Horas planejadas | %s |\n", hoursStr(stats.PlannedHours))
	fmt.Fprintf(&b, "| Horas entregues | %s |\n", hoursStr(stats.DoneHours))
	if sp.CapacityHours > 0 {
		fmt.Fprintf(&b, "| Capacidade | %s |\n", hoursStr(sp.CapacityHours))
	}
	fmt.Fprintf(&b, "| Commits | %d |\n", len(in.Commits))
	fmt.Fprintf(&b, "| Sessões registradas | %d |\n", len(in.Sessions))

	items := []model.WorkItem{}
	for _, it := range in.Plan.Items {
		if it.Actionable() && !it.Archived && it.Sprint == sp.Number {
			items = append(items, it)
		}
	}
	// Itens transferidos já saíram da sprint no plano: entram pela lista.
	for _, id := range in.Carried {
		if it := in.Plan.Item(id); it != nil && it.Sprint != sp.Number {
			items = append(items, *it)
		}
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Rank < items[j].Rank })

	b.WriteString("\n## Entregue\n\n")
	delivered := 0
	for _, it := range items {
		if it.Status == model.StatusCompleted {
			fmt.Fprintf(&b, "- **%s** %s%s\n", it.ID, it.Title, assigneeSuffix(it.Assignee))
			delivered++
		}
	}
	if delivered == 0 {
		b.WriteString("Nenhum item concluído.\n")
	}

	if len(in.Carried) > 0 {
		dest := "o backlog"
		if in.CarriedTo > 0 {
			dest = "a " + model.SprintName(in.CarriedTo)
		}
		fmt.Fprintf(&b, "\n## Não concluído (transferido para %s)\n\n", dest)
		for _, id := range in.Carried {
			if it := in.Plan.Item(id); it != nil {
				fmt.Fprintf(&b, "- **%s** %s — %s\n", it.ID, it.Title, statusLabel(it.Status))
			}
		}
	}

	if len(in.Commits) > 0 {
		b.WriteString("\n## Commits\n\n")
		for _, c := range in.Commits {
			fmt.Fprintf(&b, "- `%s` %s", shortHash(c.Hash), c.Subject)
			if c.Author != "" {
				fmt.Fprintf(&b, " — %s", c.Author)
			}
			b.WriteString("\n")
		}
	}

	if len(in.Sessions) > 0 {
		b.WriteString("\n## Sessões de trabalho\n\n")
		for _, s := range in.Sessions {
			who := DisplayName(s.Author)
			if s.Agent != "" {
				who += " (" + s.Agent + ")"
			}
			summary := strings.TrimSpace(strings.Split(s.Summary, "\n")[0])
			fmt.Fprintf(&b, "- %s · %s: %s\n", dateOnly(s.Started), who, orDash(summary))
		}
		decisions := []string{}
		for _, s := range in.Sessions {
			decisions = append(decisions, s.Decisions...)
		}
		if len(decisions) > 0 {
			b.WriteString("\n### Decisões registradas\n\n")
			for _, d := range decisions {
				fmt.Fprintf(&b, "- %s\n", d)
			}
		}
	}
	return b.String()
}

// StatusLabels são os nomes em português dos estados.
var StatusLabels = map[string]string{
	model.StatusPending:    "pendente",
	model.StatusInProgress: "em andamento",
	model.StatusReview:     "em revisão",
	model.StatusCompleted:  "concluída",
	model.StatusBlocked:    "bloqueada",
}

func statusLabel(s string) string {
	if l, ok := StatusLabels[s]; ok {
		return l
	}
	return s
}

func assigneeSuffix(a string) string {
	if a == "" {
		return ""
	}
	return " — " + DisplayName(a)
}

func hoursStr(h float64) string {
	if h == math.Trunc(h) {
		return fmt.Sprintf("%.0f h", h)
	}
	return fmt.Sprintf("%.1f h", h)
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func shortHash(h string) string {
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func dateOnly(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return orDash(s)
}
