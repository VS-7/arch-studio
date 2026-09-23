package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

// ---------------------------------------------------------------------------
// Diagramas UML (casos de uso, classes, sequência e estados)
// ---------------------------------------------------------------------------
//
// Mesma disciplina do diagrama macro: toda mutação é uma transação
// "ler → mutar → validar → gravar" sob a.tx, grava o espelho Mermaid quando
// settings.sync_mermaid está ativo e emite `uml_changed` para a interface.

// DefaultUseCaseDiagramID é o id do diagrama gerado a partir das fichas.
const DefaultUseCaseDiagramID = "casos-de-uso"

// saveUML valida, persiste o diagrama e, se habilitado, regenera o espelho Mermaid.
func (a *App) saveUML(d *model.UMLDiagram) error {
	d.Normalize()
	if err := d.Validate(); err != nil {
		return err
	}
	if err := a.st.SaveUMLDiagram(d); err != nil {
		return err
	}
	manifest, err := a.st.LoadManifest()
	if err == nil && manifest.Settings.SyncMermaid {
		rel := store.UMLMermaidPath(d.Kind, d.ID)
		if err := a.st.WriteFile(rel, []byte(mermaid.ExportUML(d))); err != nil {
			return fmt.Errorf("falha ao sincronizar %s: %w", rel, err)
		}
	}
	return nil
}

func (a *App) emitUML(d *model.UMLDiagram, source, message string) {
	a.emit(hub.Event{
		Type: hub.EventUML, Source: source, Path: store.UMLPath(d.Kind, d.ID),
		Message: message,
		Payload: map[string]any{"diagram_id": d.ID},
	})
}

// ListUMLDiagrams devolve todos os diagramas UML (nunca nulo).
func (a *App) ListUMLDiagrams() ([]model.UMLDiagram, error) { return a.st.ListUMLDiagrams() }

// GetUMLDiagram carrega um diagrama pelo id.
func (a *App) GetUMLDiagram(id string) (*model.UMLDiagram, error) { return a.st.LoadUMLDiagram(id) }

// UMLMermaid exporta o diagrama em Mermaid, independentemente de sync_mermaid.
func (a *App) UMLMermaid(id string) (string, error) {
	d, err := a.st.LoadUMLDiagram(id)
	if err != nil {
		return "", err
	}
	return mermaid.ExportUML(d), nil
}

// CreateUMLDiagram cria um diagrama vazio. O id é o slug do nome, único entre
// todos os diagramas UML, e nunca muda depois (renomear não altera o id).
func (a *App) CreateUMLDiagram(kind, name, description, source string) (*model.UMLDiagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	kind = strings.ToLower(strings.TrimSpace(kind))
	if !model.ValidUMLKind(kind) {
		return nil, fmt.Errorf("tipo de diagrama inválido: %q (válidos: %s)", kind, strings.Join(model.UMLKinds, ", "))
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("o diagrama precisa de um nome (name)")
	}
	d := model.NewUMLDiagram(a.st.UniqueUMLID(name), kind, name)
	d.Description = strings.TrimSpace(description)
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	a.emitUML(d, source, fmt.Sprintf("Diagrama %q criado", name))
	return d, nil
}

// RenameUMLDiagram altera nome e/ou descrição (campos nil são mantidos).
func (a *App) RenameUMLDiagram(id string, name, description *string, source string) (*model.UMLDiagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return nil, errors.New("o diagrama precisa de um nome (name)")
		}
		d.Name = n
	}
	if description != nil {
		d.Description = strings.TrimSpace(*description)
	}
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	a.emitUML(d, source, fmt.Sprintf("Diagrama %q atualizado", d.Name))
	return d, nil
}

// ReplaceUMLDiagram substitui elementos, relações e viewport com o conteúdo
// vindo da UI (arrastar, redimensionar, reordenar mensagens). id, tipo, nome e
// descrição do arquivo existente prevalecem.
func (a *App) ReplaceUMLDiagram(id string, in *model.UMLDiagram, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	if in == nil {
		return errors.New("diagrama vazio")
	}
	d, err := a.st.LoadUMLDiagram(id)
	if err != nil {
		return err
	}
	d.Elements = in.Elements
	d.Relations = in.Relations
	d.Viewport = in.Viewport
	if err := a.saveUML(d); err != nil {
		return err
	}
	a.emitUML(d, source, "")
	return nil
}

// DeleteUMLDiagram remove o JSON e o espelho Mermaid.
func (a *App) DeleteUMLDiagram(id, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(id)
	if err != nil {
		return err
	}
	if err := a.st.DeleteUMLDiagram(d); err != nil {
		return err
	}
	a.emitUML(d, source, fmt.Sprintf("Diagrama %q removido", d.Name))
	return nil
}

// UMLElementInput é um elemento parcial. `position` é ponteiro para distinguir
// "omitido" (posição automática) de (0,0) explícito; o campo externo sombreia
// o Position do elemento embutido na decodificação JSON.
type UMLElementInput struct {
	model.UMLElement
	Position *model.Position `json:"position,omitempty"`
}

// AddUMLElement cria um elemento, gerando id `el-<slug>` e posição livre quando
// omitidos. Elementos existentes nunca são deslocados.
func (a *App) AddUMLElement(diagramID string, in UMLElementInput, source string) (*model.UMLElement, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return nil, err
	}
	el := in.UMLElement
	el.Type = strings.ToLower(strings.TrimSpace(el.Type))
	el.Name = strings.TrimSpace(el.Name)
	if err := model.ValidateUMLElement(d.Kind, el); err != nil {
		return nil, err
	}
	if el.ParentID != "" {
		parent := d.ResolveElement(el.ParentID)
		if parent == nil {
			return nil, model.UMLNotFound("elemento pai não encontrado: %q", el.ParentID)
		}
		el.ParentID = parent.ID
	}

	el.ID = strings.TrimSpace(el.ID)
	if el.ID == "" {
		el.ID = d.UniqueElementID(el.Name, el.Type)
	} else if d.ElementByID(el.ID) != nil || d.RelationByID(el.ID) != nil {
		return nil, fmt.Errorf("já existe um item com id %q neste diagrama", el.ID)
	}
	if in.Position != nil {
		el.Position = *in.Position
	} else {
		el.Position = d.AutoPosition(el)
	}

	d.Elements = append(d.Elements, el)
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	saved := *d.ElementByID(el.ID)
	a.emitUML(d, source, fmt.Sprintf("%s %q adicionado a %q", saved.Type, displayName(saved), d.Name))
	return &saved, nil
}

// UpdateUMLElement aplica um merge parcial: apenas as chaves presentes em
// `patch` sobrescrevem o elemento (null ou "" limpam o campo). O id não muda.
func (a *App) UpdateUMLElement(diagramID, elementID string, patch map[string]any, source string) (*model.UMLElement, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return nil, err
	}
	el := d.ResolveElement(elementID)
	if el == nil {
		return nil, model.UMLNotFound("elemento não encontrado: %q", elementID)
	}
	id := el.ID
	var merged model.UMLElement
	if err := mergeJSON(el, patch, &merged); err != nil {
		return nil, err
	}
	merged.ID = id
	merged.Type = strings.ToLower(strings.TrimSpace(merged.Type))
	if merged.ParentID != "" {
		parent := d.ResolveElement(merged.ParentID)
		if parent == nil {
			return nil, model.UMLNotFound("elemento pai não encontrado: %q", merged.ParentID)
		}
		if parent.ID == id {
			return nil, errors.New("um elemento não pode conter a si mesmo")
		}
		merged.ParentID = parent.ID
	}
	if err := model.ValidateUMLElement(d.Kind, merged); err != nil {
		return nil, err
	}
	*el = merged
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	saved := *d.ElementByID(id)
	a.emitUML(d, source, fmt.Sprintf("%s %q atualizado", saved.Type, displayName(saved)))
	return &saved, nil
}

// RemoveUMLElement remove o elemento, as relações ligadas a ele e zera o
// parent_id dos filhos; mensagens restantes são renumeradas.
func (a *App) RemoveUMLElement(diagramID, elementID, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return err
	}
	el := d.ResolveElement(elementID)
	if el == nil {
		return model.UMLNotFound("elemento não encontrado: %q", elementID)
	}
	name := displayName(*el)
	d.RemoveElement(el.ID)
	if err := a.saveUML(d); err != nil {
		return err
	}
	a.emitUML(d, source, fmt.Sprintf("Elemento %q removido", name))
	return nil
}

// AddUMLRelation cria uma relação. source/target aceitam id ou nome do
// elemento; o id padrão é `rel-<n>`. Mensagens sem ordem vão para o fim
// (max+1); com ordem, são inseridas nessa posição e as demais deslocadas.
func (a *App) AddUMLRelation(diagramID string, rel model.UMLRelation, source string) (*model.UMLRelation, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return nil, err
	}
	rel.Type = strings.ToLower(strings.TrimSpace(rel.Type))
	src := d.ResolveElement(rel.Source)
	if src == nil {
		return nil, model.UMLNotFound("elemento de origem não encontrado: %q", rel.Source)
	}
	dst := d.ResolveElement(rel.Target)
	if dst == nil {
		return nil, model.UMLNotFound("elemento de destino não encontrado: %q", rel.Target)
	}
	rel.Source, rel.Target = src.ID, dst.ID
	if rel.Type == "message" && rel.MessageKind == "" {
		rel.MessageKind = "sync"
	}
	if err := d.ValidateUMLRelation(rel); err != nil {
		return nil, err
	}

	rel.ID = strings.TrimSpace(rel.ID)
	if rel.ID == "" {
		rel.ID = d.UniqueRelationID()
	} else if d.RelationByID(rel.ID) != nil || d.ElementByID(rel.ID) != nil {
		return nil, fmt.Errorf("já existe um item com id %q neste diagrama", rel.ID)
	}

	order := 0
	if rel.Type == "message" {
		order = rel.Order
		rel.Order = d.MaxMessageOrder() + 1
	} else {
		rel.Order = 0
	}
	d.Relations = append(d.Relations, rel)
	if order > 0 {
		d.MoveMessage(rel.ID, order)
	}
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	saved := *d.RelationByID(rel.ID)
	a.emitUML(d, source, fmt.Sprintf("%s %s → %s adicionada", saved.Type, displayName(*src), displayName(*dst)))
	return &saved, nil
}

// UpdateUMLRelation aplica merge parcial numa relação. Alterar `order` de uma
// mensagem a reposiciona na sequência, deslocando as demais.
func (a *App) UpdateUMLRelation(diagramID, relationID string, patch map[string]any, source string) (*model.UMLRelation, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return nil, err
	}
	rel := d.RelationByID(relationID)
	if rel == nil {
		return nil, model.UMLNotFound("relação não encontrada: %q", relationID)
	}
	id, oldOrder := rel.ID, rel.Order
	var merged model.UMLRelation
	if err := mergeJSON(rel, patch, &merged); err != nil {
		return nil, err
	}
	merged.ID = id
	merged.Type = strings.ToLower(strings.TrimSpace(merged.Type))
	if _, ok := patch["source"]; ok {
		src := d.ResolveElement(merged.Source)
		if src == nil {
			return nil, model.UMLNotFound("elemento de origem não encontrado: %q", merged.Source)
		}
		merged.Source = src.ID
	}
	if _, ok := patch["target"]; ok {
		dst := d.ResolveElement(merged.Target)
		if dst == nil {
			return nil, model.UMLNotFound("elemento de destino não encontrado: %q", merged.Target)
		}
		merged.Target = dst.ID
	}
	if merged.Type == "message" && merged.MessageKind == "" {
		merged.MessageKind = "sync"
	}
	if err := d.ValidateUMLRelation(merged); err != nil {
		return nil, err
	}
	newOrder := merged.Order
	merged.Order = oldOrder
	*rel = merged
	if merged.Type == "message" {
		if oldOrder == 0 {
			rel.Order = d.MaxMessageOrder() + 1
		}
		if _, ok := patch["order"]; ok && newOrder > 0 {
			d.MoveMessage(id, newOrder)
		}
	}
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	saved := *d.RelationByID(id)
	a.emitUML(d, source, fmt.Sprintf("Relação %s atualizada", id))
	return &saved, nil
}

// RemoveUMLRelation remove a relação; se era mensagem, renumera as demais 1..n.
func (a *App) RemoveUMLRelation(diagramID, relationID, source string) error {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return err
	}
	if !d.RemoveRelation(relationID) {
		return model.UMLNotFound("relação não encontrada: %q", relationID)
	}
	if err := a.saveUML(d); err != nil {
		return err
	}
	a.emitUML(d, source, "Relação removida")
	return nil
}

// RemoveUMLItem remove um elemento ou uma relação pelo id (ou nome, para
// elementos). Devolve "element" ou "relation".
func (a *App) RemoveUMLItem(diagramID, id, source string) (string, error) {
	d, err := a.st.LoadUMLDiagram(diagramID)
	if err != nil {
		return "", err
	}
	if d.RelationByID(id) != nil {
		return "relation", a.RemoveUMLRelation(diagramID, id, source)
	}
	if d.ResolveElement(id) != nil {
		return "element", a.RemoveUMLElement(diagramID, id, source)
	}
	return "", model.UMLNotFound("nenhum elemento ou relação com id %q no diagrama %q", id, diagramID)
}

// GenerateUseCaseDiagram cria (ou completa) o diagrama de casos de uso a partir
// das fichas em docs/casos-de-uso. É idempotente: reaproveita atores (pelo
// nome) e usecases (pelo código CDU), preserva posições e só adiciona o que falta.
func (a *App) GenerateUseCaseDiagram(name, source string) (*model.UMLDiagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	manifest, err := a.st.LoadManifest()
	if err != nil {
		return nil, err
	}
	ucs, err := a.st.ListUseCases()
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)

	d, err := a.st.LoadUMLDiagram(DefaultUseCaseDiagramID)
	created := false
	switch {
	case errors.Is(err, model.ErrUMLNotFound):
		if name == "" {
			name = "Casos de Uso"
		}
		d = model.NewUMLDiagram(DefaultUseCaseDiagramID, model.UMLKindUseCase, name)
		d.Description = "Gerado a partir das fichas em docs/casos-de-uso."
		created = true
	case err != nil:
		return nil, err
	case d.Kind != model.UMLKindUseCase:
		return nil, fmt.Errorf("o id %q já pertence a um diagrama do tipo %s", DefaultUseCaseDiagramID, d.Kind)
	}
	if !created && name != "" {
		d.Name = name
	}

	model.SyncUseCaseDiagram(d, manifest.ProjectName, ucs)
	if err := a.saveUML(d); err != nil {
		return nil, err
	}
	a.emitUML(d, source, fmt.Sprintf("Diagrama de casos de uso sincronizado com %d ficha(s)", len(ucs)))
	return d, nil
}

// mergeJSON sobrepõe as chaves de `patch` à representação JSON de `current`
// e decodifica o resultado em `out`.
func mergeJSON(current any, patch map[string]any, out any) error {
	data, err := json.Marshal(current)
	if err != nil {
		return err
	}
	base := map[string]any{}
	if err := json.Unmarshal(data, &base); err != nil {
		return err
	}
	for k, v := range patch {
		base[k] = v
	}
	merged, err := json.Marshal(base)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(merged, out); err != nil {
		return fmt.Errorf("campos inválidos: %w", err)
	}
	return nil
}

func displayName(e model.UMLElement) string {
	if e.Name != "" {
		return e.Name
	}
	return e.ID
}
