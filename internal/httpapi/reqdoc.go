package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Handlers — documento de requisitos e SVG dos diagramas UML
// ---------------------------------------------------------------------------

func (s *Server) getReqDoc(w http.ResponseWriter, r *http.Request) {
	doc, err := s.app.RequirementsDocument()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) getReqDocMeta(w http.ResponseWriter, r *http.Request) {
	meta, err := s.app.DocumentMeta()
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *Server) putReqDocMeta(w http.ResponseWriter, r *http.Request) {
	var meta model.DocumentMeta
	if err := decode(r, &meta); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.app.SaveDocumentMeta(meta, hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) saveReqDoc(w http.ResponseWriter, r *http.Request) {
	res, err := s.app.GenerateRequirementsDocument("", hub.SourceUI)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"file": res.File, "images": res.Images})
}

func (s *Server) getReqDocMarkdown(w http.ResponseWriter, r *http.Request) {
	md, err := s.app.RequirementsMarkdown(r.URL.Query().Get("embed") == "1")
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="documento-de-requisitos.md"`)
	_, _ = w.Write([]byte(md))
}

// exportUMLSVG atende /api/export/uml/{file}: o ServeMux não aceita curinga
// com sufixo ({id}.svg), então o ".svg" é removido aqui.
func (s *Server) exportUMLSVG(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(r.PathValue("file"), ".svg")
	svg, err := s.app.UMLSVG(id, r.URL.Query().Get("theme") == "dark")
	if err != nil {
		failUML(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.svg"`, model.Slugify(id)))
	}
	_, _ = w.Write(svg)
}
