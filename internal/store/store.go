// Package store implementa o acesso à "fonte única da verdade" em disco.
// Todas as escritas são atômicas (arquivo temporário + rename) e registradas
// num índice de eco, para que o file watcher não retransmita as próprias
// gravações do servidor como se fossem edições externas.
package store

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/archcode/studio/internal/model"
)

// Caminhos canônicos relativos à raiz do projeto.
const (
	DirArch       = ".arch"
	DirDiagrams   = ".arch/diagrams"
	DirSequence   = ".arch/diagrams/sequence"
	DirER         = ".arch/diagrams/er"
	DirDocs       = "docs"
	DirUseCases   = "docs/casos-de-uso"
	DirADR        = "docs/architecture-decisions"
	DirAPI        = "api"
	FileManifest  = ".arch/manifest.yaml"
	FilePricing   = ".arch/pricing.yaml"
	FileTasks     = ".arch/tasks.json"
	FileMacroJSON = ".arch/diagrams/macro.json"
	FileMacroMmd  = ".arch/diagrams/macro.mermaid"
	FileRequisit  = "docs/requisitos.md"
	FileAIPRD     = "docs/ai-prd.md"
	FileEndpoints = "api/endpoints.yaml"
	FileProposal  = "docs/proposta-comercial.md"
)

var ErrNotAProject = errors.New("diretório não é um projeto ArchCode Studio (.arch/manifest.yaml ausente)")

// Store serializa o acesso concorrente (HTTP, WebSocket, MCP e watcher) ao disco.
type Store struct {
	root string

	mu sync.RWMutex

	echoMu sync.Mutex
	echo   map[string]echoEntry
}

type echoEntry struct {
	hash string
	at   time.Time
}

func New(root string) (*Store, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return &Store{root: abs, echo: map[string]echoEntry{}}, nil
}

func (s *Store) Root() string { return s.root }

// Path resolve um caminho relativo dentro da raiz do projeto, bloqueando
// traversal para fora da árvore (`..`).
func (s *Store) Path(rel string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("caminho absoluto não permitido: %s", rel)
	}
	full := filepath.Join(s.root, clean)
	if full != s.root && !strings.HasPrefix(full, s.root+string(os.PathSeparator)) {
		return "", fmt.Errorf("caminho fora do projeto: %s", rel)
	}
	return full, nil
}

// Rel converte um caminho absoluto em caminho relativo com barras normais.
func (s *Store) Rel(abs string) string {
	rel, err := filepath.Rel(s.root, abs)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// IsProject informa se a raiz já contém um manifest válido.
func (s *Store) IsProject() bool {
	p, err := s.Path(FileManifest)
	if err != nil {
		return false
	}
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ---------------------------------------------------------------------------
// Primitivas de leitura e escrita
// ---------------------------------------------------------------------------

func (s *Store) ReadFile(rel string) ([]byte, error) {
	p, err := s.Path(rel)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return os.ReadFile(p)
}

func (s *Store) Exists(rel string) bool {
	p, err := s.Path(rel)
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// WriteFile grava de forma atômica e registra o hash para supressão de eco.
func (s *Store) WriteFile(rel string, data []byte) error {
	p, err := s.Path(rel)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeLocked(p, data)
}

func (s *Store) writeLocked(abs string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".archcode-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		return err
	}
	s.rememberEcho(abs, data)
	return os.Rename(tmpName, abs)
}

func (s *Store) Remove(rel string) error {
	p, err := s.Path(rel)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.Remove(p)
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *Store) rememberEcho(abs string, data []byte) {
	s.echoMu.Lock()
	defer s.echoMu.Unlock()
	s.echo[abs] = echoEntry{hash: hashBytes(data), at: time.Now()}
	// Poda entradas antigas para não crescer indefinidamente em sessões longas.
	if len(s.echo) > 512 {
		cutoff := time.Now().Add(-1 * time.Minute)
		for k, v := range s.echo {
			if v.at.Before(cutoff) {
				delete(s.echo, k)
			}
		}
	}
}

// IsEcho informa se o conteúdo atual do arquivo é idêntico à última gravação
// feita pelo próprio servidor, permitindo ignorar o evento do fsnotify.
func (s *Store) IsEcho(abs string) bool {
	s.echoMu.Lock()
	entry, ok := s.echo[abs]
	s.echoMu.Unlock()
	if !ok || time.Since(entry.at) > 10*time.Second {
		return false
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return false
	}
	return hashBytes(data) == entry.hash
}

// ---------------------------------------------------------------------------
// Manifest
// ---------------------------------------------------------------------------

func (s *Store) LoadManifest() (*model.Manifest, error) {
	data, err := s.ReadFile(FileManifest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotAProject
		}
		return nil, err
	}
	var m model.Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", FileManifest, err)
	}
	if m.SchemaVersion == "" {
		m.SchemaVersion = model.SchemaVersion
	}
	if m.Settings.DiagramEngine == "" {
		m.Settings.DiagramEngine = "react-flow"
	}
	if m.Settings.PricingCurrency == "" {
		m.Settings.PricingCurrency = "BRL"
	}
	return &m, nil
}

func (s *Store) SaveManifest(m *model.Manifest) error {
	data, err := marshalYAML(m)
	if err != nil {
		return err
	}
	return s.WriteFile(FileManifest, data)
}

// ---------------------------------------------------------------------------
// Diagrama macro
// ---------------------------------------------------------------------------

func (s *Store) LoadDiagram() (*model.Diagram, error) {
	data, err := s.ReadFile(FileMacroJSON)
	if err != nil {
		if os.IsNotExist(err) {
			return model.NewDiagram(), nil
		}
		return nil, err
	}
	var d model.Diagram
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", FileMacroJSON, err)
	}
	d.Touch()
	return &d, nil
}

func (s *Store) SaveDiagram(d *model.Diagram) error {
	d.Touch()
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return s.WriteFile(FileMacroJSON, append(data, '\n'))
}

// ---------------------------------------------------------------------------
// Precificação
// ---------------------------------------------------------------------------

func (s *Store) LoadPricing() (*model.PricingConfig, error) {
	data, err := s.ReadFile(FilePricing)
	if err != nil {
		if os.IsNotExist(err) {
			p := model.DefaultPricing()
			return p, nil
		}
		return nil, err
	}
	var p model.PricingConfig
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", FilePricing, err)
	}
	p.Normalize()
	return &p, nil
}

func (s *Store) SavePricing(p *model.PricingConfig) error {
	p.Normalize()
	data, err := marshalYAML(p)
	if err != nil {
		return err
	}
	return s.WriteFile(FilePricing, data)
}

// ---------------------------------------------------------------------------
// Contratos de API
// ---------------------------------------------------------------------------

func (s *Store) LoadEndpoints() (*model.EndpointsSpec, error) {
	data, err := s.ReadFile(FileEndpoints)
	if err != nil {
		if os.IsNotExist(err) {
			return model.NewEndpointsSpec(), nil
		}
		return nil, err
	}
	var spec model.EndpointsSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", FileEndpoints, err)
	}
	if spec.Version == "" {
		spec.Version = model.SchemaVersion
	}
	if spec.Endpoints == nil {
		spec.Endpoints = []model.Endpoint{}
	}
	return &spec, nil
}

func (s *Store) SaveEndpoints(spec *model.EndpointsSpec) error {
	spec.Sort()
	data, err := marshalYAML(spec)
	if err != nil {
		return err
	}
	header := "# Contratos de API derivados das arestas do diagrama macro.\n" +
		"# Gerado e mantido pelo ArchCode Studio — edições manuais são preservadas.\n"
	return s.WriteFile(FileEndpoints, append([]byte(header), data...))
}

// ---------------------------------------------------------------------------
// Requisitos
// ---------------------------------------------------------------------------

func (s *Store) LoadRequirements() (*model.RequirementsDoc, error) {
	data, err := s.ReadFile(FileRequisit)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.RequirementsDoc{Requirements: []model.Requirement{}}, nil
		}
		return nil, err
	}
	return model.ParseRequirements(string(data)), nil
}

func (s *Store) SaveRequirements(doc *model.RequirementsDoc) error {
	doc.Sort()
	return s.WriteFile(FileRequisit, []byte(model.RenderRequirements(doc)))
}

// ---------------------------------------------------------------------------
// Casos de uso
// ---------------------------------------------------------------------------

func (s *Store) ListUseCases() ([]model.UseCase, error) {
	dir, err := s.Path(DirUseCases)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.UseCase{}, nil
		}
		return nil, err
	}
	out := []model.UseCase{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		uc := model.ParseUseCase(string(data))
		if uc.Code == "" {
			continue
		}
		uc.File = DirUseCases + "/" + name
		out = append(out, *uc)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

// FindUseCase localiza o arquivo de um caso de uso pelo código (ex.: CDU001).
func (s *Store) FindUseCase(code string) (*model.UseCase, error) {
	list, err := s.ListUseCases()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if strings.EqualFold(list[i].Code, code) {
			return &list[i], nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *Store) SaveUseCase(uc *model.UseCase) (string, error) {
	if uc.Code == "" {
		return "", errors.New("caso de uso sem código (ex.: CDU001)")
	}
	existing, err := s.FindUseCase(uc.Code)
	target := DirUseCases + "/" + model.UseCaseFileName(uc)
	if err == nil && existing.File != "" {
		// Renomeia se o nome do caso de uso mudou, evitando arquivos órfãos.
		if existing.File != target {
			_ = s.Remove(existing.File)
		}
	}
	uc.File = target
	return target, s.WriteFile(target, []byte(model.RenderUseCase(uc)))
}

// NextUseCaseCode devolve o próximo código livre da série CDU.
func (s *Store) NextUseCaseCode() string {
	list, _ := s.ListUseCases()
	max := 0
	for _, uc := range list {
		var n int
		if _, err := fmt.Sscanf(strings.ToUpper(uc.Code), "CDU%d", &n); err == nil && n > max {
			max = n
		}
	}
	return fmt.Sprintf("CDU%03d", max+1)
}

// ---------------------------------------------------------------------------
// ADRs
// ---------------------------------------------------------------------------

func (s *Store) ListADRs() ([]model.ADR, error) {
	dir, err := s.Path(DirADR)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.ADR{}, nil
		}
		return nil, err
	}
	out := []model.ADR{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "_") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		adr := model.ParseADR(string(data))
		if adr.ID == "" {
			continue
		}
		adr.File = DirADR + "/" + name
		out = append(out, *adr)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (s *Store) SaveADR(adr *model.ADR) (string, error) {
	if adr.ID == "" {
		list, _ := s.ListADRs()
		adr.ID = fmt.Sprintf("ADR-%03d", len(list)+1)
	}
	if adr.Date == "" {
		adr.Date = time.Now().Format("2006-01-02")
	}
	existing, _ := s.ListADRs()
	target := DirADR + "/" + model.ADRFileName(adr)
	for _, e := range existing {
		if strings.EqualFold(e.ID, adr.ID) && e.File != target {
			_ = s.Remove(e.File)
		}
	}
	adr.File = target
	return target, s.WriteFile(target, []byte(model.RenderADR(adr)))
}

// ---------------------------------------------------------------------------
// Board de tarefas do AI-PRD
// ---------------------------------------------------------------------------

func (s *Store) LoadTasks() (*model.TaskBoard, error) {
	data, err := s.ReadFile(FileTasks)
	if err != nil {
		if os.IsNotExist(err) {
			return model.NewTaskBoard(), nil
		}
		return nil, err
	}
	var b model.TaskBoard
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("%s inválido: %w", FileTasks, err)
	}
	if b.Tasks == nil {
		b.Tasks = []model.Task{}
	}
	return &b, nil
}

func (s *Store) SaveTasks(b *model.TaskBoard) error {
	b.Version = model.SchemaVersion
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return s.WriteFile(FileTasks, append(data, '\n'))
}

// ---------------------------------------------------------------------------
// Snapshot agregado
// ---------------------------------------------------------------------------

// Snapshot é a visão consolidada do projeto entregue à UI e às ferramentas MCP.
type Snapshot struct {
	Manifest     *model.Manifest        `json:"manifest"`
	Diagram      *model.Diagram         `json:"diagram"`
	Requirements *model.RequirementsDoc `json:"requirements"`
	UseCases     []model.UseCase        `json:"use_cases"`
	ADRs         []model.ADR            `json:"adrs"`
	Endpoints    *model.EndpointsSpec   `json:"endpoints"`
	Pricing      *model.PricingConfig   `json:"pricing"`
	Tasks        *model.TaskBoard       `json:"tasks"`
	Mermaid      string                 `json:"mermaid"`
	AIPRDExists  bool                   `json:"ai_prd_exists"`
}

func (s *Store) Snapshot() (*Snapshot, error) {
	manifest, err := s.LoadManifest()
	if err != nil {
		return nil, err
	}
	diagram, err := s.LoadDiagram()
	if err != nil {
		return nil, err
	}
	reqs, err := s.LoadRequirements()
	if err != nil {
		return nil, err
	}
	if reqs.ProjectName == "" {
		reqs.ProjectName = manifest.ProjectName
	}
	useCases, err := s.ListUseCases()
	if err != nil {
		return nil, err
	}
	adrs, err := s.ListADRs()
	if err != nil {
		return nil, err
	}
	endpoints, err := s.LoadEndpoints()
	if err != nil {
		return nil, err
	}
	pricing, err := s.LoadPricing()
	if err != nil {
		return nil, err
	}
	tasks, err := s.LoadTasks()
	if err != nil {
		return nil, err
	}
	mermaid, _ := s.ReadFile(FileMacroMmd)
	return &Snapshot{
		Manifest:     manifest,
		Diagram:      diagram,
		Requirements: reqs,
		UseCases:     useCases,
		ADRs:         adrs,
		Endpoints:    endpoints,
		Pricing:      pricing,
		Tasks:        tasks,
		Mermaid:      string(mermaid),
		AIPRDExists:  s.Exists(FileAIPRD),
	}, nil
}

func marshalYAML(v any) ([]byte, error) {
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return []byte(sb.String()), nil
}
