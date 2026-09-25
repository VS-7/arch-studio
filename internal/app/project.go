package app

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/mermaid"
	"github.com/archcode/studio/internal/model"
	"github.com/archcode/studio/internal/store"
	"github.com/archcode/studio/internal/svgexport"
)

// ---------------------------------------------------------------------------
// Projeto e leituras
// ---------------------------------------------------------------------------
//
// Os adaptadores (HTTP, MCP, CLI, desktop) só leem o projeto por aqui: nenhum
// deles conhece o store.

// Root devolve o diretório raiz do projeto.
func (a *App) Root() string { return a.st.Root() }

// IsProject informa se a raiz contém um projeto ArchCode Studio.
func (a *App) IsProject() bool { return a.st.IsProject() }

func (a *App) Manifest() (*model.Manifest, error) { return a.st.LoadManifest() }

func (a *App) Diagram() (*model.Diagram, error) { return a.st.LoadDiagram() }

// DiagramMermaid gera o espelho Mermaid do diagrama atual (mesmo texto gravado
// em macro.mermaid quando a sincronização está ligada).
func (a *App) DiagramMermaid() (string, error) {
	d, err := a.st.LoadDiagram()
	if err != nil {
		return "", err
	}
	return mermaid.Export(d), nil
}

// DiagramSVGOptions controla a exportação do diagrama macro em SVG.
type DiagramSVGOptions struct {
	Executive   bool
	Dark        bool
	Transparent bool
	NoTitle     bool
	// Document usa o estilo de figura de documento (ver svgexport.Options).
	Document bool
}

// DiagramSVG desenha o diagrama macro e sugere um nome de arquivo.
func (a *App) DiagramSVG(o DiagramSVGOptions) (svg, filename string, err error) {
	snap, err := a.st.Snapshot()
	if err != nil {
		return "", "", err
	}
	opts := svgexport.Options{
		Executive: o.Executive, Dark: o.Dark, Transparent: o.Transparent, Document: o.Document,
		Title: snap.Manifest.ProjectName, Subtitle: snap.Manifest.Description,
	}
	if o.NoTitle {
		opts.Title, opts.Subtitle = "", ""
	}
	return svgexport.Render(snap.Diagram, opts), model.Slugify(snap.Manifest.ProjectName) + "-arquitetura.svg", nil
}

func (a *App) Requirements() (*model.RequirementsDoc, error) { return a.st.LoadRequirements() }

// RequirementsRaw devolve docs/requisitos.md como está no disco ("" se ausente).
func (a *App) RequirementsRaw() string {
	data, _ := a.st.ReadFile(store.FileRequisit)
	return string(data)
}

func (a *App) UseCases() ([]model.UseCase, error)           { return a.st.ListUseCases() }
func (a *App) ADRs() ([]model.ADR, error)                   { return a.st.ListADRs() }
func (a *App) Endpoints() (*model.EndpointsSpec, error)     { return a.st.LoadEndpoints() }
func (a *App) PricingConfig() (*model.PricingConfig, error) { return a.st.LoadPricing() }

// ---------------------------------------------------------------------------
// Arquivos brutos (editor de documentos)
// ---------------------------------------------------------------------------

// editableDirs limita a leitura/escrita direta às pastas do projeto.
var editableDirs = []string{store.DirDocs, store.DirArch, store.DirAPI}

// ErrPathNotAllowed indica um caminho fora das pastas editáveis do projeto.
var ErrPathNotAllowed = errors.New("caminho não permitido")

// editablePath normaliza o caminho *antes* de validá-lo: "docs/../go.mod" não
// pode passar pelo prefixo "docs/".
func editablePath(p string) (string, error) {
	clean := path.Clean(strings.ReplaceAll(p, `\`, "/"))
	for _, dir := range editableDirs {
		if strings.HasPrefix(clean, dir+"/") {
			return clean, nil
		}
	}
	return "", fmt.Errorf("%w: %q", ErrPathNotAllowed, p)
}

// ReadProjectFile lê um arquivo de docs/, .arch/ ou api/.
func (a *App) ReadProjectFile(p string) (string, error) {
	rel, err := editablePath(p)
	if err != nil {
		return "", err
	}
	data, err := a.st.ReadFile(rel)
	if err != nil {
		return "", model.NotFound("arquivo %s não encontrado", rel)
	}
	return string(data), nil
}

// WriteProjectFile grava um arquivo de docs/, .arch/ ou api/ e anuncia a
// mudança com o evento correspondente ao arquivo (a UI recarrega a tela certa).
func (a *App) WriteProjectFile(p, content, source string) error {
	rel, err := editablePath(p)
	if err != nil {
		return err
	}
	a.tx.Lock()
	defer a.tx.Unlock()
	if err := a.st.WriteFile(rel, []byte(content)); err != nil {
		return err
	}
	evType := eventForPath(rel)
	if evType == "" {
		evType = hub.EventDocs
	}
	a.emit(hub.Event{Type: evType, Source: source, Path: rel})
	return nil
}

// ---------------------------------------------------------------------------
// Mudanças externas (watcher)
// ---------------------------------------------------------------------------

// ExternalChange anuncia um arquivo alterado fora do ArchCode Studio (editor,
// git checkout, agente de IA em outro processo).
func (a *App) ExternalChange(rel string) {
	evType := eventForPath(rel)
	if evType == "" {
		return
	}
	ev := hub.Event{
		Type:    evType,
		Source:  hub.SourceDisk,
		Path:    rel,
		Message: "Arquivo alterado fora do ArchCode Studio",
	}
	if evType == hub.EventUML {
		ev.Payload = map[string]any{"diagram_id": strings.TrimSuffix(path.Base(rel), ".json")}
	}
	a.emit(ev)
}

// eventForPath decide qual evento uma mudança no arquivo produz ("" = nenhum).
func eventForPath(rel string) string {
	switch {
	case umlDiagramFile(rel):
		return hub.EventUML
	case rel == store.FileMacroJSON, rel == store.FileMacroMmd:
		return hub.EventDiagram
	case rel == store.FileEndpoints:
		return hub.EventEndpoints
	case rel == store.FilePricing:
		return hub.EventPricing
	case rel == store.FileTasks:
		return hub.EventTasks
	case rel == store.FileManifest:
		return hub.EventManifest
	case rel == store.FileConventions:
		return hub.EventConventions
	case strings.HasPrefix(rel, store.DirPlan+"/"):
		return hub.EventPlan
	case strings.HasPrefix(rel, store.DirSessions+"/"):
		return hub.EventSession
	case strings.HasPrefix(rel, store.DirMemory+"/"):
		return hub.EventMemory
	case strings.HasPrefix(rel, store.DirSkills+"/"):
		return hub.EventSkills
	case rel == ".git/HEAD", strings.HasPrefix(rel, ".git/refs/"), rel == ".git/packed-refs":
		return hub.EventGit
	case rel == store.FileDocument, strings.HasPrefix(rel, store.DirDocs+"/"):
		return hub.EventDocs
	case strings.HasPrefix(rel, store.DirDiagrams+"/"):
		return hub.EventDiagram
	default:
		return ""
	}
}

// umlDiagramFile informa se o caminho é o JSON de um diagrama UML.
func umlDiagramFile(rel string) bool {
	if !strings.HasSuffix(rel, ".json") {
		return false
	}
	dir := path.Dir(rel)
	for _, d := range store.UMLDirs {
		if dir == d {
			return true
		}
	}
	return false
}
