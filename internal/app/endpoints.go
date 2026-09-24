package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

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
		return model.NotFound("endpoint não encontrado: %s", id)
	}
	spec.Endpoints = kept
	if err := a.st.SaveEndpoints(spec); err != nil {
		return err
	}
	a.emit(hub.Event{Type: hub.EventEndpoints, Source: source, Path: store.FileEndpoints})
	return nil
}
