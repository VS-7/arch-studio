package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/reqdoc"
	"github.com/archcode/studio/internal/store"
	"github.com/archcode/studio/internal/svgexport"
)

// ---------------------------------------------------------------------------
// Documento de requisitos (docs/documento-de-requisitos.md)
// ---------------------------------------------------------------------------

// RequirementsDocument monta o documento estruturado (com o Markdown) a partir
// do estado atual do projeto. Não grava nada.
func (a *App) RequirementsDocument() (*reqdoc.Document, error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	return buildRequirementsDocument(snap), nil
}

func buildRequirementsDocument(snap *store.Snapshot) *reqdoc.Document {
	return reqdoc.Build(reqdoc.Input{
		Manifest:     snap.Manifest,
		Meta:         snap.Document,
		Diagram:      snap.Diagram,
		Requirements: snap.Requirements,
		UseCases:     snap.UseCases,
		ADRs:         snap.ADRs,
		Endpoints:    snap.Endpoints,
		UMLDiagrams:  snap.UMLDiagrams,
	})
}

// DocumentMeta devolve os metadados do documento com os padrões preenchidos.
func (a *App) DocumentMeta() (*model.DocumentMeta, error) { return a.st.LoadDocumentMeta() }

// SaveDocumentMeta substitui .arch/document.yaml.
func (a *App) SaveDocumentMeta(meta model.DocumentMeta, source string) (*model.DocumentMeta, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	if err := a.st.SaveDocumentMeta(&meta); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileDocument,
		Message: "Metadados do documento de requisitos salvos"})
	return a.st.LoadDocumentMeta()
}

// DocumentMetaPatch é uma atualização parcial dos metadados: campos nil são
// mantidos (merge), usado pela ferramenta MCP update_document_metadata.
type DocumentMetaPatch struct {
	Title        *string               `json:"title,omitempty"`
	Version      *string               `json:"version,omitempty"`
	Date         *string               `json:"date,omitempty"`
	Authors      *[]string             `json:"authors,omitempty"`
	Client       *string               `json:"client,omitempty"`
	Users        *string               `json:"users,omitempty"`
	Introduction *string               `json:"introduction,omitempty"`
	History      *[]model.DocRevision  `json:"history,omitempty"`
	References   *[]string             `json:"references,omitempty"`
	Glossary     *[]model.GlossaryTerm `json:"glossary,omitempty"`
}

// UpdateDocumentMeta aplica o patch sobre os metadados atuais e grava.
func (a *App) UpdateDocumentMeta(p DocumentMetaPatch, source string) (*model.DocumentMeta, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	meta, err := a.st.LoadDocumentMetaRaw()
	if err != nil {
		return nil, err
	}
	setStr := func(dst *string, v *string) {
		if v != nil {
			*dst = strings.TrimSpace(*v)
		}
	}
	setStr(&meta.Title, p.Title)
	setStr(&meta.Version, p.Version)
	setStr(&meta.Date, p.Date)
	setStr(&meta.Client, p.Client)
	setStr(&meta.Users, p.Users)
	setStr(&meta.Introduction, p.Introduction)
	if p.Authors != nil {
		meta.Authors = *p.Authors
	}
	if p.History != nil {
		meta.History = *p.History
	}
	if p.References != nil {
		meta.References = *p.References
	}
	if p.Glossary != nil {
		meta.Glossary = *p.Glossary
	}
	if err := a.st.SaveDocumentMeta(meta); err != nil {
		return nil, err
	}
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: store.FileDocument,
		Message: "Metadados do documento de requisitos atualizados"})
	return a.st.LoadDocumentMeta()
}

// ReqDocResult descreve o documento gravado em disco.
type ReqDocResult struct {
	File          string   `json:"file"`
	Images        []string `json:"images"`
	Functional    int      `json:"functional"`
	NonFunctional int      `json:"non_functional"`
	UseCases      int      `json:"use_cases"`
	Figures       int      `json:"figures"`
}

// Summary resume o documento gerado em uma linha.
func (r *ReqDocResult) Summary() string {
	return fmt.Sprintf("%d RF, %d RNF, %d CDU, %d figura(s)", r.Functional, r.NonFunctional, r.UseCases, r.Figures)
}

// figureFile devolve o nome do SVG de uma figura: <id>.svg para diagramas UML
// e arquitetura.svg para o diagrama macro.
func figureFile(b reqdoc.Block) string {
	if b.Diagram == reqdoc.MacroDiagramID {
		return "arquitetura.svg"
	}
	return b.Diagram + ".svg"
}

// renderFigure produz o SVG de uma figura do documento (tema claro, estilo de
// figura de documento para a arquitetura).
func renderFigure(snap *store.Snapshot, b reqdoc.Block) ([]byte, error) {
	if b.Diagram == reqdoc.MacroDiagramID {
		return []byte(svgexport.Render(snap.Diagram, svgexport.Options{Document: true})), nil
	}
	for i := range snap.UMLDiagrams {
		if snap.UMLDiagrams[i].ID == b.Diagram {
			return svgexport.RenderUML(&snap.UMLDiagrams[i], svgexport.UMLOptions{}), nil
		}
	}
	return nil, model.NotFound("diagrama UML não encontrado: %q", b.Diagram)
}

// GenerateRequirementsDocument grava o documento de requisitos em Markdown
// (padrão docs/documento-de-requisitos.md) e o SVG de cada figura em
// <pasta do documento>/diagramas/, com as imagens referenciadas por caminho
// relativo. `out` é relativo à raiz do projeto.
func (a *App) GenerateRequirementsDocument(out, source string) (*ReqDocResult, error) {
	a.tx.Lock()
	defer a.tx.Unlock()

	out = strings.TrimSpace(filepath.ToSlash(out))
	if out == "" {
		out = store.FileReqDoc
	}
	if path.Ext(out) == "" {
		out += ".md"
	}
	if _, err := a.st.Path(out); err != nil {
		return nil, err
	}
	// Com o destino padrão, as figuras ficam em docs/diagramas (store.DirDocImages).
	imgDir := path.Join(path.Dir(out), "diagramas")

	snap, err := a.st.Snapshot()
	if err != nil {
		return nil, err
	}
	doc := buildRequirementsDocument(snap)

	res := &ReqDocResult{File: out, Images: []string{}}
	for _, fig := range doc.Figures() {
		data, err := renderFigure(snap, fig)
		if err != nil {
			return nil, err
		}
		rel := path.Join(imgDir, figureFile(fig))
		if err := a.st.WriteFile(rel, data); err != nil {
			return nil, err
		}
		res.Images = append(res.Images, rel)
	}
	md := reqdoc.Markdown(doc, reqdoc.MarkdownOptions{ImageSrc: func(b reqdoc.Block) string {
		return "diagramas/" + figureFile(b)
	}})
	if err := a.st.WriteFile(out, []byte(md)); err != nil {
		return nil, err
	}

	res.Functional = len(snap.Requirements.Functional())
	res.NonFunctional = len(snap.Requirements.NonFunctional())
	res.UseCases = len(snap.UseCases)
	res.Figures = len(res.Images)
	a.emit(hub.Event{Type: hub.EventDocs, Source: source, Path: out,
		Message: "Documento de requisitos gerado: " + res.Summary()})
	return res, nil
}

// RequirementsMarkdown devolve o Markdown do documento para download. Com
// embed, as figuras vão embutidas como data URI (arquivo único).
func (a *App) RequirementsMarkdown(embed bool) (string, error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return "", err
	}
	doc := buildRequirementsDocument(snap)
	if !embed {
		return doc.Markdown, nil
	}
	return reqdoc.Markdown(doc, reqdoc.MarkdownOptions{ImageSrc: func(b reqdoc.Block) string {
		data, err := renderFigure(snap, b)
		if err != nil {
			return b.Src
		}
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(data)
	}}), nil
}

// UMLSVG renderiza um diagrama UML em SVG.
func (a *App) UMLSVG(id string, dark bool) ([]byte, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("informe o id do diagrama")
	}
	d, err := a.st.LoadUMLDiagram(id)
	if err != nil {
		return nil, err
	}
	return svgexport.RenderUML(d, svgexport.UMLOptions{Dark: dark}), nil
}
