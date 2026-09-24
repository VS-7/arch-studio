package httpapi

import (
	"errors"
	"io"
	"net/http"

	"github.com/archcode/studio/internal/app"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Handlers — diagramas UML
// ---------------------------------------------------------------------------

func (s *Server) listUML(w http.ResponseWriter, r *http.Request) {
	list, err := s.app.ListUMLDiagrams()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createUML(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind        string `json:"kind"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	d, err := s.app.CreateUMLDiagram(body.Kind, body.Name, body.Description, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) generateUseCaseUML(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	// Corpo opcional: aceita requisição vazia (sem nenhum JSON).
	if r.ContentLength != 0 {
		if err := decode(r, &body); err != nil && !errors.Is(err, io.EOF) {
			fail(w, http.StatusBadRequest, err)
			return
		}
	}
	d, err := s.app.GenerateUseCaseDiagram(body.Name, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) getUML(w http.ResponseWriter, r *http.Request) {
	d, err := s.app.GetUMLDiagram(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) getUMLMermaid(w http.ResponseWriter, r *http.Request) {
	src, err := s.app.UMLMermaid(r.PathValue("id"))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"mermaid": src})
}

func (s *Server) autoLayoutUML(w http.ResponseWriter, r *http.Request) {
	d, err := s.app.AutoLayoutUML(r.PathValue("id"), hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) putUML(w http.ResponseWriter, r *http.Request) {
	var d model.UMLDiagram
	if err := decode(r, &d); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.app.ReplaceUMLDiagram(r.PathValue("id"), &d, hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) patchUML(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	d, err := s.app.RenameUMLDiagram(r.PathValue("id"), body.Name, body.Description, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) deleteUML(w http.ResponseWriter, r *http.Request) {
	if err := s.app.DeleteUMLDiagram(r.PathValue("id"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) addUMLElement(w http.ResponseWriter, r *http.Request) {
	var in app.UMLElementInput
	if err := decode(r, &in); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	el, err := s.app.AddUMLElement(r.PathValue("id"), in, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, el)
}

func (s *Server) updateUMLElement(w http.ResponseWriter, r *http.Request) {
	patch := map[string]any{}
	if err := decode(r, &patch); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	el, err := s.app.UpdateUMLElement(r.PathValue("id"), r.PathValue("eid"), patch, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, el)
}

func (s *Server) deleteUMLElement(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RemoveUMLElement(r.PathValue("id"), r.PathValue("eid"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) addUMLRelation(w http.ResponseWriter, r *http.Request) {
	var rel model.UMLRelation
	if err := decode(r, &rel); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.app.AddUMLRelation(r.PathValue("id"), rel, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) updateUMLRelation(w http.ResponseWriter, r *http.Request) {
	patch := map[string]any{}
	if err := decode(r, &patch); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.app.UpdateUMLRelation(r.PathValue("id"), r.PathValue("rid"), patch, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deleteUMLRelation(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RemoveUMLRelation(r.PathValue("id"), r.PathValue("rid"), hub.SourceUI); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
