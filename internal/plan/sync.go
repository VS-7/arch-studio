package plan

import (
	"reflect"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Sincronização com a arquitetura (RF032)
// ---------------------------------------------------------------------------
//
// Regras:
//   - item novo na arquitetura entra no fim do backlog, como pendente;
//   - item existente recebe os campos gerados, exceto os marcados em
//     `overrides` (editados à mão);
//   - item cuja origem sumiu é arquivado, nunca apagado; se a origem voltar,
//     ele é restaurado com o status que tinha;
//   - status, sprint, rank, dono e checkpoint nunca são tocados.

// Tipos de mudança da sincronização.
const (
	ChangeAdded    = "added"
	ChangeUpdated  = "updated"
	ChangeArchived = "archived"
	ChangeRestored = "restored"
)

// Change descreve o que a sincronização fez (ou faria) com um item.
type Change struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Type   string   `json:"type"`
	Kind   string   `json:"kind"`
	Fields []string `json:"fields,omitempty"`
}

// SyncResult é o diff da sincronização e os itens a gravar.
type SyncResult struct {
	Changes []Change `json:"changes"`
	// Items são os itens novos ou alterados, prontos para gravar.
	Items []model.WorkItem `json:"-"`
	// Unchanged conta os itens gerados que já estavam em dia.
	Unchanged int `json:"unchanged"`
}

// Counts resume o diff por tipo de mudança.
func (r *SyncResult) Counts() map[string]int {
	out := map[string]int{}
	for _, c := range r.Changes {
		out[c.Kind]++
	}
	return out
}

// archivedBySync marca os itens arquivados pela sincronização (e só esses
// são restaurados automaticamente).
const archivedBySync = "origem removida da arquitetura"

// Sync compara o backlog atual com os itens gerados. now é o carimbo de
// tempo gravado nos itens criados ou alterados.
func Sync(existing *model.Plan, desired []model.WorkItem, now string) *SyncResult {
	if existing == nil {
		existing = model.NewPlan()
	}
	res := &SyncResult{Changes: []Change{}}
	wanted := map[string]bool{}

	lastRank := ""
	for _, it := range existing.Items {
		if it.Rank > lastRank && validRank(it.Rank) {
			lastRank = it.Rank
		}
	}
	fresh := len(existing.Items) == 0
	var spread []string
	if fresh {
		spread = Spread(len(desired))
	}

	for i, d := range desired {
		wanted[d.Source] = true
		cur := existing.BySource(d.Source)
		if cur == nil {
			// Item migrado de um tasks.json antigo ainda sem origem: casa pelo id.
			if byID := existing.Item(d.ID); byID != nil && byID.Source == "" && byID.Type == d.Type {
				cur = byID
			}
		}
		if cur == nil {
			item := d
			item.Status = model.StatusPending
			item.CreatedAt, item.UpdatedAt = now, now
			if fresh {
				item.Rank = spread[i]
			} else {
				item.Rank = After(lastRank)
				lastRank = item.Rank
			}
			res.Items = append(res.Items, item)
			res.Changes = append(res.Changes, Change{ID: item.ID, Title: item.Title, Type: item.Type, Kind: ChangeAdded})
			continue
		}

		next := *cur
		fields := applyGenerated(&next, d)
		kind := ChangeUpdated
		if next.Archived && strings.HasPrefix(next.ArchiveReason, archivedBySync) {
			next.Archived, next.ArchiveReason = false, ""
			kind = ChangeRestored
		}
		if len(fields) == 0 && kind != ChangeRestored {
			res.Unchanged++
			continue
		}
		next.UpdatedAt = now
		res.Items = append(res.Items, next)
		res.Changes = append(res.Changes, Change{ID: next.ID, Title: next.Title, Type: next.Type, Kind: kind, Fields: fields})
	}

	for _, it := range existing.Items {
		if it.Source == "" || it.Archived || wanted[it.Source] {
			continue
		}
		next := it
		next.Archived = true
		next.ArchiveReason = archivedBySync + " (" + it.Source + ")"
		next.UpdatedAt = now
		res.Items = append(res.Items, next)
		res.Changes = append(res.Changes, Change{ID: next.ID, Title: next.Title, Type: next.Type, Kind: ChangeArchived})
	}
	return res
}

// applyGenerated copia para cur os campos gerados de d que não foram editados
// à mão, e devolve os nomes dos campos que mudaram.
func applyGenerated(cur *model.WorkItem, d model.WorkItem) []string {
	changed := []string{}
	set := func(field string, dst, src any) {
		if cur.Overridden(field) {
			return
		}
		dv := reflect.ValueOf(dst).Elem()
		sv := reflect.ValueOf(src)
		if !reflect.DeepEqual(normalizeEmpty(dv.Interface()), normalizeEmpty(sv.Interface())) {
			dv.Set(sv)
			changed = append(changed, field)
		}
	}
	set("title", &cur.Title, d.Title)
	set("description", &cur.Description, d.Description)
	set("priority", &cur.Priority, d.Priority)
	set("parent", &cur.Parent, d.Parent)
	set("tier", &cur.Tier, d.Tier)
	set("component", &cur.Component, d.Component)
	set("component_id", &cur.ComponentID, d.ComponentID)
	set("dependencies", &cur.Dependencies, d.Dependencies)
	set("requirements", &cur.Requirements, d.Requirements)
	set("use_cases", &cur.UseCases, d.UseCases)
	set("endpoints", &cur.Endpoints, d.Endpoints)
	set("estimate_h", &cur.EstimateHours, d.EstimateHours)
	set("order", &cur.Order, d.Order)
	if cur.Source == "" && d.Source != "" {
		cur.Source = d.Source
		changed = append(changed, "source")
	}
	if !cur.Overridden("acceptance") {
		merged := mergeCriteria(cur.Acceptance, d.Acceptance)
		if !reflect.DeepEqual(normalizeEmpty(merged), normalizeEmpty(cur.Acceptance)) {
			cur.Acceptance = merged
			changed = append(changed, "acceptance")
		}
	}
	return changed
}

// mergeCriteria adota os critérios gerados mantendo a marcação de atendido
// dos que já existiam com o mesmo texto.
func mergeCriteria(cur, gen []model.Criterion) []model.Criterion {
	done := map[string]bool{}
	for _, c := range cur {
		if c.Done {
			done[normalizeText(c.Text)] = true
		}
	}
	out := make([]model.Criterion, 0, len(gen))
	for _, c := range gen {
		out = append(out, model.Criterion{Text: c.Text, Done: done[normalizeText(c.Text)]})
	}
	return out
}

func normalizeText(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

// normalizeEmpty trata slice nil e vazio como iguais na comparação.
func normalizeEmpty(v any) any {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.Len() == 0 {
		return nil
	}
	return v
}
