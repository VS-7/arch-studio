package model

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ---------------------------------------------------------------------------
// Backlog e sprints (.arch/plan/)
// ---------------------------------------------------------------------------
//
// Cada item do backlog é um arquivo Markdown com frontmatter YAML em
// .arch/plan/{epics,stories,tasks}/<ID>.md, e cada sprint um YAML em
// .arch/plan/sprints/sprint-NN.yaml. Um arquivo por item mantém os diffs
// legíveis em PR e faz duas pessoas só conflitarem quando editam o mesmo item
// (RNF009).

// Tipos de item do backlog.
const (
	ItemEpic     = "epic"
	ItemStory    = "story"
	ItemTask     = "task"
	ItemBug      = "bug"
	ItemDebt     = "debt"
	ItemSpike    = "spike"
	ItemSecurity = "security"
)

// ItemTypes lista os tipos na ordem de exibição.
var ItemTypes = []string{ItemEpic, ItemStory, ItemTask, ItemBug, ItemDebt, ItemSpike, ItemSecurity}

// ItemPrefixes são os prefixos dos ids de cada tipo (EP-CDU001, BUG-007…).
var ItemPrefixes = map[string]string{
	ItemEpic: "EP", ItemStory: "ST", ItemTask: "TASK", ItemBug: "BUG",
	ItemDebt: "DEBT", ItemSpike: "SPIKE", ItemSecurity: "SEC",
}

// ItemTypeLabels são os nomes em português dos tipos.
var ItemTypeLabels = map[string]string{
	ItemEpic: "Épico", ItemStory: "História", ItemTask: "Tarefa", ItemBug: "Bug",
	ItemDebt: "Débito técnico", ItemSpike: "Spike", ItemSecurity: "Segurança",
}

// ValidItemType informa se o tipo é conhecido.
func ValidItemType(t string) bool {
	_, ok := ItemPrefixes[t]
	return ok
}

// NormalizeItemType aceita sinônimos em português e inglês.
func NormalizeItemType(t string) string {
	switch normalizeKey(t) {
	case "epic", "epico":
		return ItemEpic
	case "story", "historia", "user story":
		return ItemStory
	case "bug", "defeito":
		return ItemBug
	case "debt", "debito", "debito tecnico", "tech debt", "refactor":
		return ItemDebt
	case "spike", "investigacao", "pesquisa":
		return ItemSpike
	case "security", "seguranca", "sec":
		return ItemSecurity
	default:
		return ItemTask
	}
}

// StatusReview é o estado "Em revisão": o trabalho está num pull request.
const StatusReview = "review"

// ItemStatuses são os estados do ciclo de vida de um item, na ordem do quadro.
var ItemStatuses = []string{StatusPending, StatusInProgress, StatusReview, StatusCompleted, StatusBlocked}

// Prioridades MoSCoW.
const (
	PriorityMust   = "must"
	PriorityShould = "should"
	PriorityCould  = "could"
	PriorityWont   = "wont"
)

// NormalizePriority traduz as prioridades usadas nos requisitos (Alta,
// Média, Baixa, Essencial…) para MoSCoW. Vazio continua vazio.
func NormalizePriority(p string) string {
	switch normalizeKey(p) {
	case "":
		return ""
	case "must", "alta", "essencial", "critica", "obrigatorio", "high":
		return PriorityMust
	case "should", "media", "importante", "medium":
		return PriorityShould
	case "could", "baixa", "desejavel", "low":
		return PriorityCould
	case "wont", "won't", "nao", "fora":
		return PriorityWont
	default:
		return PriorityShould
	}
}

// Criterion é um critério de aceite com a marcação de atendido.
type Criterion struct {
	Text string `json:"text"`
	Done bool   `json:"done"`
}

// Handoff é o checkpoint de retomada de uma tarefa: o que foi feito por
// último, o que vem a seguir e onde mexer (RF049).
type Handoff struct {
	LastStep     string   `json:"last_step,omitempty"`
	NextStep     string   `json:"next_step,omitempty"`
	Files        []string `json:"files,omitempty"`
	FailingTests []string `json:"failing_tests,omitempty"`
	Notes        string   `json:"notes,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
	By           string   `json:"by,omitempty"`
}

// Empty informa se o checkpoint não tem conteúdo.
func (h *Handoff) Empty() bool {
	return h == nil || (h.LastStep == "" && h.NextStep == "" && len(h.Files) == 0 &&
		len(h.FailingTests) == 0 && h.Notes == "")
}

// Resultados de um check de skill.
const (
	CheckOK   = "ok"
	CheckFail = "fail"
	CheckNA   = "na"
)

// CheckResult é o resultado informado para uma verificação de skill.
type CheckResult struct {
	Skill    string `json:"skill"`
	Check    string `json:"check"`
	Result   string `json:"result"`
	Evidence string `json:"evidence,omitempty"`
}

// NormalizeCheckResult aceita sinônimos (passou, falhou, n/a…).
func NormalizeCheckResult(r string) string {
	switch normalizeKey(r) {
	case "ok", "pass", "passou", "sim", "yes", "true", "aprovado":
		return CheckOK
	case "fail", "failed", "falhou", "nao", "no", "false", "erro":
		return CheckFail
	default:
		return CheckNA
	}
}

// WorkItem é um item do backlog: épico, história, tarefa ou item avulso.
//
// Os campos com tag yaml vão para o frontmatter; os demais formam o corpo
// Markdown (descrição, critérios, handoff, checks e notas).
type WorkItem struct {
	ID            string   `yaml:"id" json:"id"`
	Type          string   `yaml:"type" json:"type"`
	Title         string   `yaml:"title" json:"title"`
	Status        string   `yaml:"status" json:"status"`
	Priority      string   `yaml:"priority,omitempty" json:"priority,omitempty"`
	Sprint        int      `yaml:"sprint,omitempty" json:"sprint,omitempty"`
	Rank          string   `yaml:"rank" json:"rank"`
	Parent        string   `yaml:"parent,omitempty" json:"parent,omitempty"`
	Assignee      string   `yaml:"assignee,omitempty" json:"assignee,omitempty"`
	Agent         string   `yaml:"agent,omitempty" json:"agent,omitempty"`
	EstimateHours float64  `yaml:"estimate_h,omitempty" json:"estimate_h,omitempty"`
	Tier          string   `yaml:"tier,omitempty" json:"tier,omitempty"`
	Component     string   `yaml:"component,omitempty" json:"component,omitempty"`
	ComponentID   string   `yaml:"component_id,omitempty" json:"component_id,omitempty"`
	Dependencies  []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Requirements  []string `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	UseCases      []string `yaml:"use_cases,omitempty" json:"use_cases,omitempty"`
	Endpoints     []string `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`
	Branch        string   `yaml:"branch,omitempty" json:"branch,omitempty"`
	// Source identifica o que gerou o item (component:<nó>, requirement:RF003,
	// usecase:CDU001, slice:RF003:<nó>, e2e). Vazio = item criado à mão. É a
	// chave da sincronização com a arquitetura: o id nunca muda, mesmo que o
	// componente seja renomeado.
	Source string `yaml:"source,omitempty" json:"source,omitempty"`
	// Overrides lista os campos editados à mão que a sincronização preserva.
	Overrides     []string `yaml:"overrides,omitempty" json:"overrides,omitempty"`
	Aliases       []string `yaml:"aliases,omitempty" json:"aliases,omitempty"`
	Order         int      `yaml:"order,omitempty" json:"order,omitempty"`
	Archived      bool     `yaml:"archived,omitempty" json:"archived,omitempty"`
	ArchiveReason string   `yaml:"archive_reason,omitempty" json:"archive_reason,omitempty"`
	CreatedAt     string   `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt     string   `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	ClaimedAt     string   `yaml:"claimed_at,omitempty" json:"claimed_at,omitempty"`
	CompletedAt   string   `yaml:"completed_at,omitempty" json:"completed_at,omitempty"`

	Description string        `yaml:"-" json:"description,omitempty"`
	Acceptance  []Criterion   `yaml:"-" json:"acceptance,omitempty"`
	Handoff     *Handoff      `yaml:"-" json:"handoff,omitempty"`
	Checks      []CheckResult `yaml:"-" json:"checks,omitempty"`
	Notes       string        `yaml:"-" json:"notes,omitempty"`
	// File é o caminho relativo do arquivo (preenchido na leitura, nunca gravado).
	File string `yaml:"-" json:"file,omitempty"`
}

// Actionable informa se o item é trabalho executável (não épico nem história).
func (w *WorkItem) Actionable() bool { return w.Type != ItemEpic && w.Type != ItemStory }

// Open informa se o item ainda pode receber trabalho.
func (w *WorkItem) Open() bool { return !w.Archived && w.Status != StatusCompleted }

// Overridden informa se o campo foi editado à mão.
func (w *WorkItem) Overridden(field string) bool {
	for _, f := range w.Overrides {
		if f == field {
			return true
		}
	}
	return false
}

// MarkOverride registra que o campo foi editado à mão (idempotente).
func (w *WorkItem) MarkOverride(field string) {
	if !w.Overridden(field) {
		w.Overrides = append(w.Overrides, field)
		sort.Strings(w.Overrides)
	}
}

// HasID informa se o id (ou um alias) identifica o item.
func (w *WorkItem) HasID(id string) bool {
	if strings.EqualFold(w.ID, id) {
		return true
	}
	for _, a := range w.Aliases {
		if strings.EqualFold(a, id) {
			return true
		}
	}
	return false
}

// CriteriaDone conta os critérios marcados como atendidos.
func (w *WorkItem) CriteriaDone() int {
	n := 0
	for _, c := range w.Acceptance {
		if c.Done {
			n++
		}
	}
	return n
}

// Estados de uma sprint.
const (
	SprintPlanned = "planned"
	SprintActive  = "active"
	SprintClosed  = "closed"
)

// Sprint é uma iteração com número, meta, datas e capacidade.
type Sprint struct {
	Number        int     `yaml:"number" json:"number"`
	Name          string  `yaml:"name" json:"name"`
	Goal          string  `yaml:"goal,omitempty" json:"goal,omitempty"`
	Start         string  `yaml:"start,omitempty" json:"start,omitempty"`
	End           string  `yaml:"end,omitempty" json:"end,omitempty"`
	Status        string  `yaml:"status" json:"status"`
	CapacityHours float64 `yaml:"capacity_h,omitempty" json:"capacity_h,omitempty"`
	StartedAt     string  `yaml:"started_at,omitempty" json:"started_at,omitempty"`
	ClosedAt      string  `yaml:"closed_at,omitempty" json:"closed_at,omitempty"`
	File          string  `yaml:"-" json:"file,omitempty"`
}

// SprintName devolve o nome padrão da sprint ("Sprint 01").
func SprintName(n int) string { return fmt.Sprintf("Sprint %02d", n) }

// SprintLabel devolve o número com dois dígitos ("01").
func SprintLabel(n int) string { return fmt.Sprintf("%02d", n) }

// SprintFileName devolve o nome do arquivo da sprint.
func SprintFileName(n int) string { return fmt.Sprintf("sprint-%02d.yaml", n) }

// Plan é o backlog completo: itens e sprints.
type Plan struct {
	Items   []WorkItem `json:"items"`
	Sprints []Sprint   `json:"sprints"`
	// Warnings lista arquivos ilegíveis ou em conflito (vistos pelo doctor).
	Warnings []string `json:"warnings,omitempty"`
}

// NewPlan cria um plano vazio.
func NewPlan() *Plan { return &Plan{Items: []WorkItem{}, Sprints: []Sprint{}} }

// Item localiza um item pelo id ou alias.
func (p *Plan) Item(id string) *WorkItem {
	id = strings.TrimSpace(id)
	for i := range p.Items {
		if p.Items[i].HasID(id) {
			return &p.Items[i]
		}
	}
	return nil
}

// BySource localiza o item gerado a partir da origem informada.
func (p *Plan) BySource(source string) *WorkItem {
	for i := range p.Items {
		if p.Items[i].Source == source {
			return &p.Items[i]
		}
	}
	return nil
}

// Sprint localiza a sprint pelo número.
func (p *Plan) Sprint(n int) *Sprint {
	for i := range p.Sprints {
		if p.Sprints[i].Number == n {
			return &p.Sprints[i]
		}
	}
	return nil
}

// ActiveSprint devolve a sprint ativa (nil se nenhuma).
func (p *Plan) ActiveSprint() *Sprint {
	for i := range p.Sprints {
		if p.Sprints[i].Status == SprintActive {
			return &p.Sprints[i]
		}
	}
	return nil
}

// NextSprintNumber devolve o próximo número de sprint livre.
func (p *Plan) NextSprintNumber() int {
	max := 0
	for _, s := range p.Sprints {
		if s.Number > max {
			max = s.Number
		}
	}
	return max + 1
}

// SortItems ordena os itens pelo rank (e pelo id, em empate).
func (p *Plan) SortItems() {
	sort.SliceStable(p.Items, func(i, j int) bool {
		a, b := p.Items[i], p.Items[j]
		if a.Rank != b.Rank {
			return a.Rank < b.Rank
		}
		return a.ID < b.ID
	})
}

// SortSprints ordena as sprints pelo número.
func (p *Plan) SortSprints() {
	sort.SliceStable(p.Sprints, func(i, j int) bool { return p.Sprints[i].Number < p.Sprints[j].Number })
}

// NextItemID devolve o próximo id livre para itens criados à mão
// (TASK-001, BUG-001…). Ids gerados (TASK-API-01) não entram na contagem.
func (p *Plan) NextItemID(itemType string) string {
	prefix := ItemPrefixes[itemType]
	if prefix == "" {
		prefix = "TASK"
	}
	re := regexp.MustCompile(`^` + prefix + `-(\d+)$`)
	max := 0
	for _, it := range p.Items {
		for _, id := range append([]string{it.ID}, it.Aliases...) {
			if m := re.FindStringSubmatch(strings.ToUpper(id)); m != nil {
				if n, err := strconv.Atoi(m[1]); err == nil && n > max {
					max = n
				}
			}
		}
	}
	return fmt.Sprintf("%s-%03d", prefix, max+1)
}

// ItemDir devolve a pasta (relativa a .arch/plan) de cada tipo de item.
func ItemDir(itemType string) string {
	switch itemType {
	case ItemEpic:
		return "epics"
	case ItemStory:
		return "stories"
	default:
		return "tasks"
	}
}

// reItemID aceita ids de item: letras maiúsculas, dígitos e hífens.
var reItemID = regexp.MustCompile(`^[A-Z][A-Z0-9]*(-[A-Z0-9]+)+$`)

// ValidItemID informa se o id pode virar nome de arquivo com segurança.
func ValidItemID(id string) bool { return reItemID.MatchString(id) }

// reItemRef encontra referências a itens entre colchetes numa mensagem de
// commit ("[TASK-API-01]").
var reItemRef = regexp.MustCompile(`\[([A-Z][A-Z0-9]*(?:-[A-Z0-9]+)+)\]`)

// ItemRefs extrai os ids citados entre colchetes no texto.
func ItemRefs(text string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, m := range reItemRef.FindAllStringSubmatch(text, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}
