// Package app concentra as operações de domínio do ArchCode Studio.
//
// É a única camada autorizada a mutar o estado em disco. Tanto a API HTTP
// (usada pelo browser e, futuramente, pelos bindings IPC do Wails) quanto o
// servidor MCP (usado por agentes de IA) chamam exatamente estes métodos, o que
// garante que humanos e IAs operem sobre a mesma semântica e as mesmas
// invariantes — em especial a preservação de coordenadas do canvas (RF017).
package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/lint"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/prd"
	"github.com/archcode/studio/internal/pricing"
	"github.com/archcode/studio/internal/store"
)

type App struct {
	st *store.Store
	hb *hub.Hub

	// tx serializa as transações "ler → mutar → gravar". Sem ela, duas escritas
	// concorrentes (por exemplo, duas chamadas MCP em paralelo) poderiam carregar
	// o mesmo diagrama e uma sobrescrever a outra silenciosamente.
	tx sync.Mutex
}

func New(st *store.Store, hb *hub.Hub) *App { return &App{st: st, hb: hb} }

func (a *App) Store() *store.Store { return a.st }

func (a *App) emit(ev hub.Event) {
	if a.hb != nil {
		a.hb.Broadcast(ev)
	}
}

func (a *App) Snapshot() (*store.Snapshot, error) { return a.st.Snapshot() }

// ---------------------------------------------------------------------------
// Diagrama
// ---------------------------------------------------------------------------

// saveDiagram persiste o diagrama e, se habilitado, regenera o Mermaid espelho.
func (a *App) saveDiagram(d *model.Diagram) error {
	if err := a.st.SaveDiagram(d); err != nil {
		return err
	}
	manifest, err := a.st.LoadManifest()
	if err == nil && manifest.Settings.SyncMermaid {
		if err := a.st.WriteFile(store.FileMacroMmd, []byte(mermaid.Export(d))); err != nil {
			return fmt.Errorf("falha ao sincronizar macro.mermaid: %w", err)
		}
	}
	return nil
}

// NodeInput descreve a criação ou atualização de um componente.
type NodeInput struct {
	ID             string          `json:"id,omitempty"`
	Label          string          `json:"label"`
	Type           string          `json:"type,omitempty"`
	Technology     string          `json:"technology,omitempty"`
	Description    string          `json:"description,omitempty"`
	Tier           string          `json:"tier,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
	Complexity     string          `json:"complexity,omitempty"`
	EstimatedHours float64         `json:"estimated_hours,omitempty"`
	CloudTier      string          `json:"cloud_tier,omitempty"`
	MonthlyCost    float64         `json:"monthly_cost,omitempty"`
	Requirements   []string        `json:"requirements,omitempty"`
	UseCases       []string        `json:"use_cases,omitempty"`
	Executive      *bool           `json:"executive,omitempty"`
	Status         string          `json:"status,omitempty"`
	ConnectTo      string          `json:"connect_to,omitempty"`
	Protocol       string          `json:"protocol,omitempty"`
	Position       *model.Position `json:"position,omitempty"`
}

var validTypes = func() map[string]bool {
	m := map[string]bool{}
	for _, t := range model.NodeTypes {
		m[t] = true
	}
	return m
}()

// AddNode insere um componente calculando uma posição livre e estética.
// Nós já existentes jamais são deslocados.
func (a *App) AddNode(in NodeInput, source string) (*model.Node, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	label := strings.TrimSpace(in.Label)
	if label == "" {
		return nil, errors.New("o componente precisa de um rótulo (label)")
	}
	typ := strings.ToLower(strings.TrimSpace(in.Type))
	if typ == "" {
		typ = "compute"
	}
	if !validTypes[typ] {
		return nil, fmt.Errorf("tipo de nó inválido: %q (válidos: %s)", typ, strings.Join(model.NodeTypes, ", "))
	}

	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	if existing := d.ResolveNode(label); existing != nil {
		return nil, fmt.Errorf("já existe um componente chamado %q (id %s); use update_node_metadata para alterá-lo", label, existing.ID)
	}

	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = "node-" + model.Slugify(label)
	}
	base, i := id, 2
	for d.NodeByID(id) != nil {
		id = fmt.Sprintf("%s-%d", base, i)
		i++
	}

	tier := strings.ToLower(strings.TrimSpace(in.Tier))
	if tier == "" {
		tier = layout.TypeTier[typ]
	}

	pos := layout.FindFreePosition(d, in.ConnectTo, tier)
	if in.Position != nil {
		pos = *in.Position
	}

	complexity := model.NormalizeComplexity(in.Complexity)
	if complexity == "" {
		complexity = "medium"
	}
	status := in.Status
	if status == "" {
		status = model.StatusPending
	}

	node := model.Node{
		ID:       id,
		Type:     typ,
		Position: pos,
		Data: model.NodeData{
			Label:        label,
			Technology:   strings.TrimSpace(in.Technology),
			Description:  strings.TrimSpace(in.Description),
			Tier:         tier,
			Tags:         in.Tags,
			Executive:    in.Executive,
			Status:       model.NormalizeStatus(status),
			Requirements: in.Requirements,
			UseCases:     in.UseCases,
			Pricing: &model.NodePricing{
				Complexity:     complexity,
				EstimatedHours: in.EstimatedHours,
				CloudTier:      in.CloudTier,
				MonthlyCost:    in.MonthlyCost,
			},
		},
	}
	if typ == "group" {
		node.Width, node.Height = 520, 360
	}
	d.Nodes = append(d.Nodes, node)

	var createdEdge *model.Edge
	if in.ConnectTo != "" {
		if target := d.ResolveNode(in.ConnectTo); target != nil && target.ID != node.ID {
			edge := buildEdge(node.ID, target.ID, in.Protocol, 0, "", "")
			if !d.HasEdge(node.ID, target.ID) {
				d.Edges = append(d.Edges, edge)
				createdEdge = &edge
			}
		}
	}

	if err := a.saveDiagram(d); err != nil {
		return nil, err
	}
	a.emit(hub.Event{
		Type: hub.EventNodeAdded, Source: source, Path: store.FileMacroJSON,
		Message: fmt.Sprintf("Componente %q adicionado", label),
		Payload: map[string]any{"node": node, "edge": createdEdge},
	})
	return &node, nil
}

// UpdateNode altera metadados sem nunca mexer na posição, salvo pedido explícito.
func (a *App) UpdateNode(ref string, in NodeInput, source string) (*model.Node, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	node := d.ResolveNode(ref)
	if node == nil {
		return nil, fmt.Errorf("componente não encontrado: %q", ref)
	}

	if in.Label != "" {
		node.Data.Label = strings.TrimSpace(in.Label)
	}
	if in.Type != "" {
		typ := strings.ToLower(in.Type)
		if !validTypes[typ] {
			return nil, fmt.Errorf("tipo de nó inválido: %q", in.Type)
		}
		node.Type = typ
	}
	if in.Technology != "" {
		node.Data.Technology = in.Technology
	}
	if in.Description != "" {
		node.Data.Description = in.Description
	}
	if in.Tier != "" {
		node.Data.Tier = strings.ToLower(in.Tier)
	}
	if in.Tags != nil {
		node.Data.Tags = in.Tags
	}
	if in.Requirements != nil {
		node.Data.Requirements = in.Requirements
	}
	if in.UseCases != nil {
		node.Data.UseCases = in.UseCases
	}
	if in.Executive != nil {
		node.Data.Executive = in.Executive
	}
	if in.Status != "" {
		node.Data.Status = model.NormalizeStatus(in.Status)
	}
	if in.Position != nil {
		node.Position = *in.Position
	}
	if in.Complexity != "" || in.EstimatedHours > 0 || in.CloudTier != "" || in.MonthlyCost > 0 {
		if node.Data.Pricing == nil {
			node.Data.Pricing = &model.NodePricing{}
		}
		if c := model.NormalizeComplexity(in.Complexity); c != "" {
			node.Data.Pricing.Complexity = c
		}
		if in.EstimatedHours > 0 {
			node.Data.Pricing.EstimatedHours = in.EstimatedHours
		}
		if in.CloudTier != "" {
			node.Data.Pricing.CloudTier = in.CloudTier
		}
		if in.MonthlyCost > 0 {
			node.Data.Pricing.MonthlyCost = in.MonthlyCost
		}
	}

	if err := a.saveDiagram(d); err != nil {
		return nil, err
	}
	updated := *node
	a.emit(hub.Event{
		Type: hub.EventNodeUpdated, Source: source, Path: store.FileMacroJSON,
		Message: fmt.Sprintf("Componente %q atualizado", updated.Data.Label),
		Payload: map[string]any{"node": updated},
	})
	return &updated, nil
}

func (a *App) RemoveNode(ref, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadDiagram()
	if err != nil {
		return err
	}
	node := d.ResolveNode(ref)
	if node == nil {
		return fmt.Errorf("componente não encontrado: %q", ref)
	}
	id, label := node.ID, node.Data.Label
	d.RemoveNode(id)
	if err := a.saveDiagram(d); err != nil {
		return err
	}

	// Remove endpoints órfãos vinculados ao componente.
	if spec, err := a.st.LoadEndpoints(); err == nil {
		kept := spec.Endpoints[:0]
		removed := false
		for _, e := range spec.Endpoints {
			if e.Source == id || e.Target == id {
				removed = true
				continue
			}
			kept = append(kept, e)
		}
		if removed {
			spec.Endpoints = kept
			_ = a.st.SaveEndpoints(spec)
			a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileEndpoints})
		}
	}

	a.emit(hub.Event{
		Type: hub.EventNodeRemoved, Source: source, Path: store.FileMacroJSON,
		Message: fmt.Sprintf("Componente %q removido", label),
		Payload: map[string]any{"node_id": id},
	})
	return nil
}

// EdgeInput descreve uma conexão entre dois componentes.
type EdgeInput struct {
	SourceID       string          `json:"source_id"`
	TargetID       string          `json:"target_id"`
	Protocol       string          `json:"protocol,omitempty"`
	Port           int             `json:"port,omitempty"`
	Description    string          `json:"description,omitempty"`
	Security       string          `json:"security,omitempty"`
	Label          string          `json:"label,omitempty"`
	Complexity     string          `json:"complexity,omitempty"`
	EstimatedHours float64         `json:"estimated_hours,omitempty"`
	Animated       bool            `json:"animated,omitempty"`
	Endpoints      []EndpointInput `json:"endpoints,omitempty"`
}

// EndpointInput é a rota HTTP declarada junto de uma conexão.
type EndpointInput struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Summary     string `json:"summary,omitempty"`
	Auth        string `json:"auth,omitempty"`
	Request     string `json:"request,omitempty"`
	Response    string `json:"response,omitempty"`
	StatusCodes []int  `json:"status_codes,omitempty"`
}

func buildEdge(source, target, protocol string, port int, description, security string) model.Edge {
	if protocol == "" {
		protocol = "REST"
	}
	return model.Edge{
		ID:     fmt.Sprintf("edge-%s-to-%s", strings.TrimPrefix(source, "node-"), strings.TrimPrefix(target, "node-")),
		Source: source, Target: target, Type: "smoothstep",
		Data: model.EdgeData{
			Protocol: protocol, Port: port, Description: description, Security: security,
			Complexity: "medium",
		},
	}
}

// ConnectNodes cria (ou atualiza) a aresta entre dois componentes e registra os
// contratos HTTP correspondentes em api/endpoints.yaml.
func (a *App) ConnectNodes(in EdgeInput, source string) (*model.Edge, []model.Endpoint, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, nil, err
	}
	src := d.ResolveNode(in.SourceID)
	if src == nil {
		return nil, nil, fmt.Errorf("componente de origem não encontrado: %q", in.SourceID)
	}
	dst := d.ResolveNode(in.TargetID)
	if dst == nil {
		return nil, nil, fmt.Errorf("componente de destino não encontrado: %q", in.TargetID)
	}
	if src.ID == dst.ID {
		return nil, nil, errors.New("origem e destino são o mesmo componente")
	}

	var edge *model.Edge
	for i := range d.Edges {
		if d.Edges[i].Source == src.ID && d.Edges[i].Target == dst.ID {
			edge = &d.Edges[i]
			break
		}
	}
	if edge == nil {
		e := buildEdge(src.ID, dst.ID, in.Protocol, in.Port, in.Description, in.Security)
		base, i := e.ID, 2
		for d.EdgeByID(e.ID) != nil {
			e.ID = fmt.Sprintf("%s-%d", base, i)
			i++
		}
		d.Edges = append(d.Edges, e)
		edge = &d.Edges[len(d.Edges)-1]
	}
	if in.Protocol != "" {
		edge.Data.Protocol = in.Protocol
	}
	if in.Port > 0 {
		edge.Data.Port = in.Port
	}
	if in.Description != "" {
		edge.Data.Description = in.Description
	}
	if in.Security != "" {
		edge.Data.Security = in.Security
	}
	if in.Label != "" {
		edge.Label = in.Label
	}
	if c := model.NormalizeComplexity(in.Complexity); c != "" {
		edge.Data.Complexity = c
	}
	if in.EstimatedHours > 0 {
		edge.Data.EstimatedHours = in.EstimatedHours
	}
	edge.Animated = in.Animated || strings.EqualFold(edge.Data.Protocol, "webhook")

	// Contratos de API derivados da aresta (RF006).
	created := []model.Endpoint{}
	if len(in.Endpoints) > 0 {
		spec, err := a.st.LoadEndpoints()
		if err != nil {
			return nil, nil, err
		}
		ids := []string{}
		for _, ep := range in.Endpoints {
			method := strings.ToUpper(strings.TrimSpace(ep.Method))
			if method == "" {
				method = "GET"
			}
			path := strings.TrimSpace(ep.Path)
			if path == "" {
				continue
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			id := strings.ToLower(method) + "-" + model.Slugify(path)
			endpoint := model.Endpoint{
				ID: id, Method: method, Path: path, Summary: ep.Summary,
				Source: src.ID, Target: dst.ID, EdgeID: edge.ID, Auth: ep.Auth,
				Request: ep.Request, Response: ep.Response, StatusCodes: ep.StatusCodes,
			}
			spec.Upsert(endpoint)
			created = append(created, endpoint)
			ids = append(ids, id)
		}
		if len(ids) > 0 {
			edge.Data.Endpoints = mergeUnique(edge.Data.Endpoints, ids)
			if err := a.st.SaveEndpoints(spec); err != nil {
				return nil, nil, err
			}
			a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileEndpoints,
				Message: fmt.Sprintf("%d contrato(s) de API atualizado(s)", len(ids))})
		}
	}

	result := *edge
	if err := a.saveDiagram(d); err != nil {
		return nil, nil, err
	}
	a.emit(hub.Event{
		Type: hub.EventEdgeAdded, Source: source, Path: store.FileMacroJSON,
		Message: fmt.Sprintf("%s → %s conectados via %s", src.Data.Label, dst.Data.Label, result.Data.Protocol),
		Payload: map[string]any{"edge": result},
	})
	return &result, created, nil
}

func (a *App) RemoveEdge(id, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadDiagram()
	if err != nil {
		return err
	}
	kept := d.Edges[:0]
	found := false
	for _, e := range d.Edges {
		if e.ID == id {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		return fmt.Errorf("conexão não encontrada: %q", id)
	}
	d.Edges = kept
	if err := a.saveDiagram(d); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventDiagram, Source: source, Path: store.FileMacroJSON,
		Message: "Conexão removida"})
	return nil
}

// ReplaceDiagram grava o diagrama completo vindo da UI (arrastar, redimensionar,
// editar em lote). É a única operação que move nós existentes — e ela vem de uma
// ação humana explícita no canvas.
func (a *App) ReplaceDiagram(d *model.Diagram, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	if d == nil {
		return errors.New("diagrama vazio")
	}
	d.Touch()
	if err := a.saveDiagram(d); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventDiagram, Source: source, Path: store.FileMacroJSON})
	return nil
}

// AutoLayout reorganiza o canvas por camadas topológicas, a pedido do usuário.
func (a *App) AutoLayout(source string) (*model.Diagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	layout.AutoLayout(d)
	if err := a.saveDiagram(d); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventDiagram, Source: source, Path: store.FileMacroJSON,
		Message: "Layout reorganizado automaticamente"})
	return d, nil
}

// ImportMermaid substitui o diagrama a partir de um snippet Mermaid (RF007).
func (a *App) ImportMermaid(src string, source string) (*model.Diagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	base, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	imported, err := mermaid.Import(src, base)
	if err != nil {
		return nil, err
	}
	if err := a.saveDiagram(imported); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventDiagram, Source: source, Path: store.FileMacroJSON,
		Message: fmt.Sprintf("Mermaid importado: %d componentes, %d conexões", len(imported.Nodes), len(imported.Edges))})
	return imported, nil
}

func mergeUnique(existing, added []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, v := range append(append([]string{}, existing...), added...) {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// Documentação
// ---------------------------------------------------------------------------

func (a *App) UpsertRequirement(req model.Requirement, source string) (*model.Requirement, bool, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	doc, err := a.st.LoadRequirements()
	if err != nil {
		return nil, false, err
	}
	if strings.TrimSpace(req.Title) == "" {
		return nil, false, errors.New("requisito precisa de um título")
	}
	req.Type = strings.ToUpper(strings.TrimSpace(req.Type))
	if req.Type != "RNF" {
		req.Type = "RF"
	}
	if strings.TrimSpace(req.ID) == "" {
		req.ID = doc.NextRequirementID(req.Type)
	}
	req.ID = strings.ToUpper(strings.TrimSpace(req.ID))
	if req.Status == "" {
		req.Status = model.StatusPending
	}
	if req.Priority == "" {
		req.Priority = "Média"
	}
	created := doc.Upsert(req)
	if doc.ProjectName == "" {
		if m, err := a.st.LoadManifest(); err == nil {
			doc.ProjectName = m.ProjectName
		}
	}
	if err := a.st.SaveRequirements(doc); err != nil {
		return nil, false, err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileRequisit,
		Message: fmt.Sprintf("Requisito %s salvo", req.ID)})
	return &req, created, nil
}

func (a *App) DeleteRequirement(id, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	doc, err := a.st.LoadRequirements()
	if err != nil {
		return err
	}
	kept := doc.Requirements[:0]
	found := false
	for _, r := range doc.Requirements {
		if strings.EqualFold(r.ID, id) {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		return fmt.Errorf("requisito não encontrado: %s", id)
	}
	doc.Requirements = kept
	if err := a.st.SaveRequirements(doc); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileRequisit})
	return nil
}

// SaveRequirementsMarkdown grava o documento bruto vindo do editor visual.
func (a *App) SaveRequirementsMarkdown(content, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	if err := a.st.WriteFile(store.FileRequisit, []byte(content)); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileRequisit})
	return nil
}

func (a *App) UpsertUseCase(uc model.UseCase, source string) (*model.UseCase, string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	if strings.TrimSpace(uc.Name) == "" {
		return nil, "", errors.New("caso de uso precisa de um nome")
	}
	uc.Code = strings.ToUpper(strings.TrimSpace(uc.Code))
	if uc.Code == "" {
		uc.Code = a.st.NextUseCaseCode()
	}
	if uc.Complexity == "" {
		uc.Complexity = "medium"
	} else {
		uc.Complexity = model.NormalizeComplexity(uc.Complexity)
	}
	if uc.Status == "" {
		uc.Status = model.StatusPending
	}
	path, err := a.st.SaveUseCase(&uc)
	if err != nil {
		return nil, "", err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: path,
		Message: fmt.Sprintf("Caso de uso %s salvo", uc.Code)})
	return &uc, path, nil
}

func (a *App) DeleteUseCase(code, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	uc, err := a.st.FindUseCase(code)
	if err != nil {
		return fmt.Errorf("caso de uso não encontrado: %s", code)
	}
	if err := a.st.Remove(uc.File); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: uc.File})
	return nil
}

func (a *App) UpsertADR(adr model.ADR, source string) (*model.ADR, string, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	if strings.TrimSpace(adr.Title) == "" {
		return nil, "", errors.New("ADR precisa de um título")
	}
	path, err := a.st.SaveADR(&adr)
	if err != nil {
		return nil, "", err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: path,
		Message: fmt.Sprintf("%s salvo", adr.ID)})
	return &adr, path, nil
}

func (a *App) DeleteADR(id, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	list, err := a.st.ListADRs()
	if err != nil {
		return err
	}
	for _, adr := range list {
		if strings.EqualFold(adr.ID, id) {
			if err := a.st.Remove(adr.File); err != nil {
				return err
			}
			a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: adr.File})
			return nil
		}
	}
	return fmt.Errorf("ADR não encontrado: %s", id)
}

// ---------------------------------------------------------------------------
// Contratos de API
// ---------------------------------------------------------------------------

func (a *App) UpsertEndpoint(ep model.Endpoint, source string) (*model.Endpoint, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	spec, err := a.st.LoadEndpoints()
	if err != nil {
		return nil, err
	}
	ep.Method = strings.ToUpper(strings.TrimSpace(ep.Method))
	if ep.Method == "" {
		ep.Method = "GET"
	}
	if strings.TrimSpace(ep.Path) == "" {
		return nil, errors.New("endpoint precisa de um path")
	}
	if !strings.HasPrefix(ep.Path, "/") {
		ep.Path = "/" + ep.Path
	}
	if ep.ID == "" {
		ep.ID = strings.ToLower(ep.Method) + "-" + model.Slugify(ep.Path)
	}
	spec.Upsert(ep)
	if err := a.st.SaveEndpoints(spec); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileEndpoints,
		Message: fmt.Sprintf("Contrato %s %s salvo", ep.Method, ep.Path)})
	return &ep, nil
}

func (a *App) DeleteEndpoint(id, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	spec, err := a.st.LoadEndpoints()
	if err != nil {
		return err
	}
	kept := spec.Endpoints[:0]
	found := false
	for _, e := range spec.Endpoints {
		if e.ID == id {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		return fmt.Errorf("endpoint não encontrado: %s", id)
	}
	spec.Endpoints = kept
	if err := a.st.SaveEndpoints(spec); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileEndpoints})
	return nil
}

// ---------------------------------------------------------------------------
// Precificação
// ---------------------------------------------------------------------------

func (a *App) Estimate(marginOverride float64) (*pricing.Estimate, error) {
	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	ucs, err := a.st.ListUseCases()
	if err != nil {
		return nil, err
	}
	cfg, err := a.st.LoadPricing()
	if err != nil {
		return nil, err
	}
	return pricing.Calculate(d, ucs, cfg, marginOverride), nil
}

func (a *App) SavePricingConfig(cfg *model.PricingConfig, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	if err := a.st.SavePricing(cfg); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventPricing, Source: source, Path: store.FilePricing})
	return nil
}

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

func (a *App) ImplementationTasks(status string) ([]TaskView, *model.TaskBoard, error) {
	board, err := a.st.LoadTasks()
	if err != nil {
		return nil, nil, err
	}
	byID := map[string]model.Task{}
	for _, t := range board.Tasks {
		byID[t.ID] = t
	}
	out := []TaskView{}
	filter := strings.ToLower(strings.TrimSpace(status))
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
		out = append(out, TaskView{Task: t, Ready: len(blocked) == 0, BlockedBy: blocked})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Order < out[j].Order })
	return out, board, nil
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
		return nil, fmt.Errorf("tarefa %q não encontrada (existentes: %s…); rode generate_ai_prd primeiro",
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

// refreshPRDCheckboxes reescreve apenas as caixas de seleção do ai-prd.md,
// mantendo o restante do documento intacto.
func (a *App) refreshPRDCheckboxes(board *model.TaskBoard) {
	data, err := a.st.ReadFile(store.FileAIPRD)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- [") || len(trimmed) < 6 {
			continue
		}
		for _, t := range board.Tasks {
			if !strings.Contains(line, "**"+t.ID+" ") {
				continue
			}
			mark := " "
			switch t.Status {
			case model.StatusCompleted:
				mark = "x"
			case model.StatusInProgress:
				mark = "~"
			case model.StatusBlocked:
				mark = "!"
			}
			idx := strings.Index(line, "- [")
			if idx >= 0 && len(line) > idx+4 {
				updated := line[:idx+3] + mark + line[idx+4:]
				if updated != line {
					lines[i] = updated
					changed = true
				}
			}
			break
		}
	}
	if changed {
		_ = a.st.WriteFile(store.FileAIPRD, []byte(strings.Join(lines, "\n")))
	}
}

// ---------------------------------------------------------------------------
// Validação arquitetural
// ---------------------------------------------------------------------------

func (a *App) Validate() (*lint.Report, error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	return lint.Run(lint.Input{
		Diagram:      snap.Diagram,
		Requirements: snap.Requirements,
		UseCases:     snap.UseCases,
		Endpoints:    snap.Endpoints,
	}), nil
}
