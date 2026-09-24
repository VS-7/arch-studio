package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
)

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
		return model.NotFound("requisito não encontrado: %s", id)
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
		list, err := a.st.ListUseCases()
		if err != nil {
			return nil, "", err
		}
		uc.Code = model.NextUseCaseCode(list)
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
		return model.NotFound("caso de uso não encontrado: %s", code)
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
	adr.ID = strings.ToUpper(strings.TrimSpace(adr.ID))
	if adr.ID == "" {
		list, err := a.st.ListADRs()
		if err != nil {
			return nil, "", err
		}
		adr.ID = model.NextADRID(list)
	}
	if adr.Date == "" {
		adr.Date = time.Now().Format("2006-01-02")
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
	return model.NotFound("ADR não encontrado: %s", id)
}
