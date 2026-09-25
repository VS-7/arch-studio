package plan

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/pricing"
)

// ---------------------------------------------------------------------------
// Geração do backlog a partir da arquitetura (RF030)
// ---------------------------------------------------------------------------
//
//	caso de uso (CDU001)        → épico     EP-CDU001
//	requisito funcional (RF003) → história  ST-RF003
//	componente do canvas        → tarefa    TASK-CORE-01 (as mesmas do AI-PRD)
//	RF × componente (híbrido)   → fatia     TASK-RF003-CORE
//
// A origem de cada item fica em `source`, que é a chave da sincronização.

// Origens de itens gerados.
const (
	SourceUseCase    = "usecase:"
	SourceRequirment = "requirement:"
	SourceComponent  = "component:"
	SourceSlice      = "slice:"
	SourceE2E        = "e2e"
)

// defaultSliceHours é a estimativa de uma fatia vertical sem casos de uso
// estimados; defaultE2EHours, a de cada caso de uso na tarefa ponta a ponta.
const (
	defaultSliceHours = 4
	defaultE2EHours   = 4
)

// Input reúne o que a geração lê do projeto.
type Input struct {
	Diagram      *model.Diagram
	Requirements *model.RequirementsDoc
	UseCases     []model.UseCase
	// Board são as tarefas de componente e a E2E compiladas pelo AI-PRD.
	Board *model.TaskBoard
	// Estimate, quando presente, dá as horas de cada tarefa.
	Estimate *pricing.Estimate
	// Slicing: model.SlicingComponent ou model.SlicingHybrid.
	Slicing string
	// Existing é o backlog atual, para manter ids de fatias já criadas.
	Existing *model.Plan
}

// Generate devolve os itens que a arquitetura pede, na ordem sugerida
// (épicos, histórias, tarefas de componente, fatias e E2E).
func Generate(in Input) []model.WorkItem {
	if in.Requirements == nil {
		in.Requirements = &model.RequirementsDoc{}
	}
	if in.Diagram == nil {
		in.Diagram = model.NewDiagram()
	}
	if in.Board == nil {
		in.Board = model.NewTaskBoard()
	}
	hybrid := in.Slicing == model.SlicingHybrid

	hours := estimateIndex(in.Estimate)
	nodes := map[string]model.Node{}
	for _, n := range in.Diagram.Nodes {
		nodes[n.ID] = n
	}

	// Casos de uso que citam cada RF, e a prioridade de cada RF.
	ucsByReq := map[string][]model.UseCase{}
	for _, uc := range in.UseCases {
		for _, r := range uc.Requirements {
			key := strings.ToUpper(strings.TrimSpace(r))
			ucsByReq[key] = append(ucsByReq[key], uc)
		}
	}
	reqPriority := map[string]string{}
	functional := in.Requirements.Functional()
	for _, r := range functional {
		reqPriority[strings.ToUpper(r.ID)] = model.NormalizePriority(r.Priority)
	}

	out := []model.WorkItem{}

	// Épicos: um por caso de uso.
	for _, uc := range in.UseCases {
		if uc.Code == "" {
			continue
		}
		out = append(out, model.WorkItem{
			ID:           "EP-" + strings.ToUpper(uc.Code),
			Type:         model.ItemEpic,
			Title:        firstNonEmpty(uc.Name, uc.Code),
			Description:  strings.TrimSpace(uc.Description),
			Priority:     model.NormalizePriority(uc.Priority),
			Requirements: upperAll(uc.Requirements),
			UseCases:     []string{uc.Code},
			Acceptance:   criteria(uc.Acceptance),
			Source:       SourceUseCase + uc.Code,
		})
	}

	// Histórias: uma por requisito funcional.
	for _, r := range functional {
		id := strings.ToUpper(r.ID)
		linked := ucsByReq[id]
		story := model.WorkItem{
			ID:           "ST-" + id,
			Type:         model.ItemStory,
			Title:        firstNonEmpty(r.Title, id),
			Description:  strings.TrimSpace(r.Description),
			Priority:     reqPriority[id],
			Requirements: []string{id},
			Source:       SourceRequirment + id,
		}
		for _, uc := range linked {
			story.UseCases = append(story.UseCases, uc.Code)
			story.Acceptance = append(story.Acceptance, criteria(prd.GivenWhenThen(uc))...)
		}
		if len(linked) > 0 {
			story.Parent = "EP-" + strings.ToUpper(linked[0].Code)
		}
		if len(story.Acceptance) == 0 {
			story.Acceptance = []model.Criterion{{Text: fmt.Sprintf("%s atendido: %s", id, story.Title)}}
		}
		out = append(out, story)
	}

	// Tarefas de componente e a tarefa E2E, como o AI-PRD compilou.
	taskByNode := map[string]string{}
	e2e := -1 // índice da tarefa E2E em out (ponteiros não sobrevivem ao append)
	for _, t := range in.Board.Tasks {
		item := model.WorkItem{
			ID:           t.ID,
			Type:         model.ItemTask,
			Title:        t.Title,
			Tier:         t.Tier,
			Component:    t.Component,
			ComponentID:  t.ComponentID,
			Dependencies: append([]string(nil), t.Dependencies...),
			Requirements: upperAll(t.Requirements),
			UseCases:     append([]string(nil), t.UseCases...),
			Endpoints:    append([]string(nil), t.Endpoints...),
			Acceptance:   criteria(t.Acceptance),
			Order:        t.Order,
			Priority:     highestPriority(t.Requirements, reqPriority),
		}
		if t.ComponentID != "" {
			item.Source = SourceComponent + t.ComponentID
			item.EstimateHours = hours.component(in.Diagram, t.ComponentID)
			if !hybrid {
				for _, code := range t.UseCases {
					item.EstimateHours += hours.useCaseShare(in.UseCases, code, nodes)
				}
			}
			taskByNode[t.ComponentID] = t.ID
		} else {
			item.Source = SourceE2E
			item.EstimateHours = float64(defaultE2EHours * max(1, len(in.UseCases)))
		}
		// Só requisitos funcionais viram história: uma tarefa ligada a um RNF
		// não tem pai.
		if len(item.Requirements) == 1 {
			if _, ok := reqPriority[item.Requirements[0]]; ok {
				item.Parent = "ST-" + item.Requirements[0]
			}
		}
		item.EstimateHours = roundHours(item.EstimateHours)
		out = append(out, item)
		if item.Source == SourceE2E {
			e2e = len(out) - 1
		}
	}

	// Fatias verticais: RF × componente que o atende.
	if hybrid {
		used := map[string]bool{}
		for _, it := range out {
			used[it.ID] = true
		}
		sliceIDs := []string{}
		order := len(in.Board.Tasks)
		for _, r := range functional {
			rid := strings.ToUpper(r.ID)
			comps := componentsForRequirement(r, in.Diagram, ucsByReq[rid])
			for _, n := range comps {
				source := SourceSlice + rid + ":" + n.ID
				id := ""
				if in.Existing != nil {
					if prev := in.Existing.BySource(source); prev != nil {
						id = prev.ID
					}
				}
				if id == "" {
					base := fmt.Sprintf("TASK-%s-%s", rid, prd.TaskAbbrev(n.Data.Label))
					id = base
					for seq := 2; used[id]; seq++ {
						id = fmt.Sprintf("%s-%d", base, seq)
					}
				}
				used[id] = true
				order++
				slice := model.WorkItem{
					ID:           id,
					Type:         model.ItemTask,
					Title:        fmt.Sprintf("Implementar %s em %s (%s)", lowerFirst(firstNonEmpty(r.Title, rid)), n.Data.Label, rid),
					Tier:         layout.ResolveTier(n),
					Component:    n.Data.Label,
					ComponentID:  n.ID,
					Parent:       "ST-" + rid,
					Requirements: []string{rid},
					Priority:     reqPriority[rid],
					Source:       source,
					Order:        order,
				}
				if dep := taskByNode[n.ID]; dep != "" {
					slice.Dependencies = []string{dep}
				}
				slice.Acceptance = append(slice.Acceptance, model.Criterion{
					Text: fmt.Sprintf("%s atendido em %s: %s", rid, n.Data.Label, firstNonEmpty(r.Title, rid)),
				})
				est := 0.0
				for _, uc := range ucsByReq[rid] {
					if !useCaseTouches(uc, n) {
						continue
					}
					slice.UseCases = append(slice.UseCases, uc.Code)
					slice.Acceptance = append(slice.Acceptance, criteria(prd.GivenWhenThen(uc))...)
					est += hours.useCase(uc.Code) / float64(max(1, len(uc.Requirements))*max(1, len(comps)))
				}
				if est == 0 {
					est = defaultSliceHours
				}
				slice.EstimateHours = roundHours(est)
				out = append(out, slice)
				sliceIDs = append(sliceIDs, id)
			}
		}
		// A verificação ponta a ponta vem depois das fatias.
		if e2e >= 0 {
			item := out[e2e]
			item.Dependencies = sortedUnique(append(item.Dependencies, sliceIDs...))
			item.Order = order + 1
			out = append(append(out[:e2e:e2e], out[e2e+1:]...), item)
		}
	}
	return out
}

// componentsForRequirement devolve os componentes (não-grupo) que atendem o
// requisito: citados no RF, que citam o RF, ou dos casos de uso que o citam.
func componentsForRequirement(r model.Requirement, d *model.Diagram, ucs []model.UseCase) []model.Node {
	rid := strings.ToUpper(r.ID)
	match := map[string]bool{}
	byRef := func(ref string) {
		ref = strings.TrimSpace(ref)
		for _, n := range d.Nodes {
			if n.ID == ref || strings.EqualFold(n.Data.Label, ref) {
				match[n.ID] = true
			}
		}
	}
	for _, c := range r.Components {
		byRef(c)
	}
	for _, n := range d.Nodes {
		for _, req := range n.Data.Requirements {
			if strings.EqualFold(strings.TrimSpace(req), rid) {
				match[n.ID] = true
			}
		}
	}
	if len(match) == 0 {
		for _, uc := range ucs {
			for _, c := range uc.Components {
				byRef(c)
			}
			for _, n := range d.Nodes {
				for _, code := range n.Data.UseCases {
					if strings.EqualFold(code, uc.Code) {
						match[n.ID] = true
					}
				}
			}
		}
	}
	out := []model.Node{}
	for _, n := range d.Nodes {
		if match[n.ID] && n.Type != "group" {
			out = append(out, n)
		}
	}
	return out
}

// useCaseTouches informa se o caso de uso envolve o componente (ou não
// declara componentes, caso em que vale para todos).
func useCaseTouches(uc model.UseCase, n model.Node) bool {
	for _, code := range n.Data.UseCases {
		if strings.EqualFold(code, uc.Code) {
			return true
		}
	}
	if len(uc.Components) == 0 {
		return true
	}
	for _, c := range uc.Components {
		if c == n.ID || strings.EqualFold(c, n.Data.Label) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Estimativas
// ---------------------------------------------------------------------------

type hoursIndex struct {
	nodes, edges, useCases map[string]float64
}

func estimateIndex(est *pricing.Estimate) hoursIndex {
	h := hoursIndex{nodes: map[string]float64{}, edges: map[string]float64{}, useCases: map[string]float64{}}
	if est == nil {
		return h
	}
	for _, it := range est.Items {
		switch it.Kind {
		case "node":
			h.nodes[it.ID] = it.Hours
		case "edge":
			h.edges[it.ID] = it.Hours
		case "use_case":
			h.useCases[it.ID] = it.Hours
		}
	}
	return h
}

// component soma as horas do nó e das integrações que ele consome (arestas
// que saem dele): quem chama é quem implementa a integração.
func (h hoursIndex) component(d *model.Diagram, nodeID string) float64 {
	total := h.nodes[nodeID]
	for _, e := range d.Edges {
		if e.Source == nodeID {
			total += h.edges[e.ID]
		}
	}
	return total
}

func (h hoursIndex) useCase(code string) float64 { return h.useCases[code] }

// useCaseShare divide as horas do caso de uso entre os componentes que ele
// toca.
func (h hoursIndex) useCaseShare(ucs []model.UseCase, code string, nodes map[string]model.Node) float64 {
	total := h.useCases[code]
	if total == 0 {
		return 0
	}
	count := 0
	for _, n := range nodes {
		for _, c := range n.Data.UseCases {
			if strings.EqualFold(c, code) {
				count++
			}
		}
	}
	if count == 0 {
		for _, uc := range ucs {
			if uc.Code == code {
				count = len(uc.Components)
			}
		}
	}
	return total / float64(max(1, count))
}

func roundHours(h float64) float64 { return math.Round(h*2) / 2 }

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------

func criteria(list []string) []model.Criterion {
	out := []model.Criterion{}
	for _, s := range list {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, model.Criterion{Text: s})
		}
	}
	return out
}

// lowerFirst põe a primeira letra em minúscula, exceto em siglas ("JWT").
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) < 2 || !unicode.IsUpper(r[0]) || unicode.IsUpper(r[1]) {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func upperAll(list []string) []string {
	out := []string{}
	for _, s := range list {
		if s = strings.ToUpper(strings.TrimSpace(s)); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

var priorityRank = map[string]int{model.PriorityMust: 3, model.PriorityShould: 2, model.PriorityCould: 1, model.PriorityWont: 0}

// highestPriority devolve a maior prioridade entre os requisitos citados.
func highestPriority(reqs []string, byReq map[string]string) string {
	best, bestRank := "", -1
	for _, r := range reqs {
		p := byReq[strings.ToUpper(r)]
		if p == "" {
			continue
		}
		if priorityRank[p] > bestRank {
			best, bestRank = p, priorityRank[p]
		}
	}
	return best
}

func sortedUnique(list []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range list {
		if s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
