package app

import (
	"fmt"
	"reflect"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
	"github.com/archcode/studio/internal/svgexport"
)

// ---------------------------------------------------------------------------
// Reorganizar diagramas
// ---------------------------------------------------------------------------
//
// Única exceção à preservação de coordenadas (RF017): o usuário pede para
// reorganizar e o diagrama inteiro é redisposto para caber na página do
// Documento de Requisitos. Cada diagrama alterado emite o seu evento, então o
// desfazer por diagrama da interface funciona como em qualquer outra edição.

// LayoutResult lista o que uma reorganização em lote de fato alterou.
type LayoutResult struct {
	Architecture bool     `json:"architecture"`
	Diagrams     []string `json:"diagrams"`
}

// AutoLayout reorganiza o diagrama de arquitetura, a pedido do usuário.
func (a *App) AutoLayout(source string) (*model.Diagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	if _, err := a.relayoutArchitecture(d, source); err != nil {
		return nil, err
	}
	return d, nil
}

// AutoLayoutUML reorganiza um diagrama UML inteiro, a pedido do usuário.
func (a *App) AutoLayoutUML(id, source string) (*model.UMLDiagram, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	d, err := a.st.LoadUMLDiagram(id)
	if err != nil {
		return nil, err
	}
	if _, err := a.relayoutUML(d, source); err != nil {
		return nil, err
	}
	return d, nil
}

// AutoLayoutAll reorganiza a arquitetura e todos os diagramas UML numa única
// transação — o atalho para deixar todas as figuras do documento legíveis.
func (a *App) AutoLayoutAll(source string) (*LayoutResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	res := &LayoutResult{Diagrams: []string{}}
	d, err := a.st.LoadDiagram()
	if err != nil {
		return nil, err
	}
	if res.Architecture, err = a.relayoutArchitecture(d, source); err != nil {
		return nil, err
	}
	list, err := a.st.ListUMLDiagrams()
	if err != nil {
		return nil, err
	}
	for i := range list {
		changed, err := a.relayoutUML(&list[i], source)
		if err != nil {
			return nil, fmt.Errorf("diagrama %q: %w", list[i].Name, err)
		}
		if changed {
			res.Diagrams = append(res.Diagrams, list[i].ID)
		}
	}
	return res, nil
}

// relayoutArchitecture aplica o layout e grava só se alguma posição mudou.
func (a *App) relayoutArchitecture(d *model.Diagram, source string) (bool, error) {
	before := append([]model.Node(nil), d.Nodes...)
	layout.AutoLayout(d)
	if reflect.DeepEqual(before, d.Nodes) {
		return false, nil
	}
	if err := a.saveDiagram(d); err != nil {
		return false, err
	}
	a.emit(hub.Event{Type: hub.EventDiagram, Source: source, Path: store.FileMacroJSON,
		Message: "Arquitetura reorganizada"})
	return true, nil
}

// relayoutUML aplica o layout do tipo do diagrama e grava só se algo mudou.
func (a *App) relayoutUML(d *model.UMLDiagram, source string) (bool, error) {
	before := append([]model.UMLElement(nil), d.Elements...)
	layout.UML(d, svgexport.Metrics{})
	if reflect.DeepEqual(before, d.Elements) {
		return false, nil
	}
	if err := a.saveUML(d); err != nil {
		return false, err
	}
	a.emitUML(d, source, fmt.Sprintf("Diagrama %q reorganizado", d.Name))
	return true, nil
}
