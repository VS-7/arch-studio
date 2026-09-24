package app

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

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
		return nil, model.NotFound("componente não encontrado: %q", ref)
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
		return model.NotFound("componente não encontrado: %q", ref)
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
		return nil, nil, model.NotFound("componente de origem não encontrado: %q", in.SourceID)
	}
	dst := d.ResolveNode(in.TargetID)
	if dst == nil {
		return nil, nil, model.NotFound("componente de destino não encontrado: %q", in.TargetID)
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
		return model.NotFound("conexão não encontrada: %q", id)
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
	a.pruneOrphanEndpoints(d, source)
	a.emit(hub.Event{Type: hub.EventDiagram, Source: source, Path: store.FileMacroJSON})
	return nil
}

// pruneOrphanEndpoints remove de api/endpoints.yaml os contratos cujo componente
// de origem ou destino saiu do diagrama — o que RemoveNode já faz para um nó,
// estendido às exclusões em lote e ao desfazer da UI (PUT do diagrama inteiro).
func (a *App) pruneOrphanEndpoints(d *model.Diagram, source string) {
	spec, err := a.st.LoadEndpoints()
	if err != nil {
		return
	}
	kept := spec.Endpoints[:0]
	removed := false
	for _, e := range spec.Endpoints {
		if (e.Source != "" && d.NodeByID(e.Source) == nil) || (e.Target != "" && d.NodeByID(e.Target) == nil) {
			removed = true
			continue
		}
		kept = append(kept, e)
	}
	if !removed {
		return
	}
	spec.Endpoints = kept
	if err := a.st.SaveEndpoints(spec); err == nil {
		a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileEndpoints})
	}
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
