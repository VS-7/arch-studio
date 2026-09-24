// Package model contém as estruturas canônicas persistidas na árvore .arch/ e docs/.
// Todo estado do ArchCode Studio é serializável para texto plano legível por humanos,
// versionável em Git e reconstruível sem banco de dados (RNF001).
package model

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const SchemaVersion = "1.0.0"

// ---------------------------------------------------------------------------
// Manifest (.arch/manifest.yaml)
// ---------------------------------------------------------------------------

type Author struct {
	Name string `yaml:"name" json:"name"`
	Role string `yaml:"role,omitempty" json:"role,omitempty"`
}

type Settings struct {
	DiagramEngine      string `yaml:"diagram_engine" json:"diagram_engine"`
	SyncMermaid        bool   `yaml:"sync_mermaid" json:"sync_mermaid"`
	AutoLayoutOnImport bool   `yaml:"auto_layout_on_import" json:"auto_layout_on_import"`
	PricingCurrency    string `yaml:"pricing_currency" json:"pricing_currency"`
}

type Manifest struct {
	SchemaVersion string   `yaml:"schema_version" json:"schema_version"`
	ProjectName   string   `yaml:"project_name" json:"project_name"`
	Description   string   `yaml:"description" json:"description"`
	Version       string   `yaml:"version" json:"version"`
	Authors       []Author `yaml:"authors" json:"authors"`
	Settings      Settings `yaml:"settings" json:"settings"`
}

func DefaultManifest(name string) *Manifest {
	return &Manifest{
		SchemaVersion: SchemaVersion,
		ProjectName:   name,
		Description:   "Arquitetura gerenciada pelo ArchCode Studio",
		Version:       "0.1.0",
		Authors:       []Author{},
		Settings: Settings{
			DiagramEngine:      "react-flow",
			SyncMermaid:        true,
			AutoLayoutOnImport: false,
			PricingCurrency:    "BRL",
		},
	}
}

// ---------------------------------------------------------------------------
// Diagrama (.arch/diagrams/macro.json)
// ---------------------------------------------------------------------------

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Viewport struct {
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Zoom float64 `json:"zoom"`
}

// NodeTypes reconhecidos pelo canvas e pelo motor de precificação.
var NodeTypes = []string{
	"compute", "database", "cache", "queue", "gateway",
	"storage", "client", "external_service", "group",
}

// Tiers de arquitetura, usados na ordenação topológica do ai-prd.md.
var Tiers = []string{"data", "domain", "backend", "integration", "frontend", "devops"}

type NodePricing struct {
	Complexity     string  `json:"complexity,omitempty"`
	EstimatedHours float64 `json:"estimated_hours,omitempty"`
	CloudTier      string  `json:"cloud_tier,omitempty"`
	Role           string  `json:"role,omitempty"`
	MonthlyCost    float64 `json:"monthly_cost,omitempty"`
}

type NodeData struct {
	Label        string       `json:"label"`
	Technology   string       `json:"technology,omitempty"`
	Description  string       `json:"description,omitempty"`
	Tier         string       `json:"tier,omitempty"`
	Tags         []string     `json:"tags,omitempty"`
	Executive    *bool        `json:"executive,omitempty"`
	Status       string       `json:"status,omitempty"`
	Requirements []string     `json:"requirements,omitempty"`
	UseCases     []string     `json:"use_cases,omitempty"`
	Pricing      *NodePricing `json:"pricing,omitempty"`
}

// ShowInExecutive indica se o nó aparece no Modo Executivo do Pitch (RF009).
func (d NodeData) ShowInExecutive() bool {
	if d.Executive != nil {
		return *d.Executive
	}
	return true
}

type Node struct {
	ID       string         `json:"id"`
	Type     string         `json:"type"`
	Position Position       `json:"position"`
	Data     NodeData       `json:"data"`
	ParentID string         `json:"parentId,omitempty"`
	Width    float64        `json:"width,omitempty"`
	Height   float64        `json:"height,omitempty"`
	Style    map[string]any `json:"style,omitempty"`
}

type EdgeData struct {
	Protocol       string   `json:"protocol,omitempty"`
	Port           int      `json:"port,omitempty"`
	Description    string   `json:"description,omitempty"`
	Security       string   `json:"security,omitempty"`
	Endpoints      []string `json:"endpoints,omitempty"`
	Complexity     string   `json:"complexity,omitempty"`
	EstimatedHours float64  `json:"estimated_hours,omitempty"`
	Executive      *bool    `json:"executive,omitempty"`
}

func (d EdgeData) ShowInExecutive() bool {
	if d.Executive != nil {
		return *d.Executive
	}
	return true
}

type Edge struct {
	ID       string   `json:"id"`
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Type     string   `json:"type,omitempty"`
	Animated bool     `json:"animated"`
	Label    string   `json:"label,omitempty"`
	Data     EdgeData `json:"data"`
}

type Diagram struct {
	Version      string   `json:"version"`
	LastModified string   `json:"last_modified"`
	Viewport     Viewport `json:"viewport"`
	Nodes        []Node   `json:"nodes"`
	Edges        []Edge   `json:"edges"`
}

func NewDiagram() *Diagram {
	return &Diagram{
		Version:      SchemaVersion,
		LastModified: time.Now().UTC().Format(time.RFC3339),
		Viewport:     Viewport{X: 0, Y: 0, Zoom: 1},
		Nodes:        []Node{},
		Edges:        []Edge{},
	}
}

func (d *Diagram) Touch() {
	d.Version = SchemaVersion
	d.LastModified = time.Now().UTC().Format(time.RFC3339)
	if d.Nodes == nil {
		d.Nodes = []Node{}
	}
	if d.Edges == nil {
		d.Edges = []Edge{}
	}
	if d.Viewport.Zoom == 0 {
		d.Viewport.Zoom = 1
	}
}

func (d *Diagram) NodeByID(id string) *Node {
	for i := range d.Nodes {
		if d.Nodes[i].ID == id {
			return &d.Nodes[i]
		}
	}
	return nil
}

// ResolveNode aceita id exato, label exato (case-insensitive) ou slug do label.
// Facilita o uso por LLMs, que frequentemente referenciam nós pelo nome visível.
func (d *Diagram) ResolveNode(ref string) *Node {
	if ref == "" {
		return nil
	}
	if n := d.NodeByID(ref); n != nil {
		return n
	}
	target := strings.ToLower(strings.TrimSpace(ref))
	for i := range d.Nodes {
		if strings.ToLower(d.Nodes[i].Data.Label) == target {
			return &d.Nodes[i]
		}
	}
	slug := Slugify(ref)
	for i := range d.Nodes {
		if Slugify(d.Nodes[i].Data.Label) == slug || d.Nodes[i].ID == "node-"+slug {
			return &d.Nodes[i]
		}
	}
	return nil
}

func (d *Diagram) EdgeByID(id string) *Edge {
	for i := range d.Edges {
		if d.Edges[i].ID == id {
			return &d.Edges[i]
		}
	}
	return nil
}

func (d *Diagram) HasEdge(source, target string) bool {
	for _, e := range d.Edges {
		if e.Source == source && e.Target == target {
			return true
		}
	}
	return false
}

func (d *Diagram) RemoveNode(id string) bool {
	found := false
	nodes := d.Nodes[:0]
	for _, n := range d.Nodes {
		if n.ID == id {
			found = true
			continue
		}
		nodes = append(nodes, n)
	}
	d.Nodes = nodes
	edges := make([]Edge, 0, len(d.Edges))
	for _, e := range d.Edges {
		if e.Source == id || e.Target == id {
			continue
		}
		edges = append(edges, e)
	}
	d.Edges = edges
	return found
}

// ---------------------------------------------------------------------------
// Contratos de API (api/endpoints.yaml)
// ---------------------------------------------------------------------------

type Endpoint struct {
	ID          string   `yaml:"id" json:"id"`
	Method      string   `yaml:"method" json:"method"`
	Path        string   `yaml:"path" json:"path"`
	Summary     string   `yaml:"summary,omitempty" json:"summary,omitempty"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Source      string   `yaml:"source,omitempty" json:"source,omitempty"`
	Target      string   `yaml:"target,omitempty" json:"target,omitempty"`
	EdgeID      string   `yaml:"edge_id,omitempty" json:"edge_id,omitempty"`
	Auth        string   `yaml:"auth,omitempty" json:"auth,omitempty"`
	Request     string   `yaml:"request,omitempty" json:"request,omitempty"`
	Response    string   `yaml:"response,omitempty" json:"response,omitempty"`
	StatusCodes []int    `yaml:"status_codes,omitempty" json:"status_codes,omitempty"`
	UseCases    []string `yaml:"use_cases,omitempty" json:"use_cases,omitempty"`
}

type EndpointsSpec struct {
	Version   string     `yaml:"version" json:"version"`
	BaseURL   string     `yaml:"base_url,omitempty" json:"base_url,omitempty"`
	Endpoints []Endpoint `yaml:"endpoints" json:"endpoints"`
}

func NewEndpointsSpec() *EndpointsSpec {
	return &EndpointsSpec{Version: SchemaVersion, BaseURL: "/api/v1", Endpoints: []Endpoint{}}
}

func (s *EndpointsSpec) Upsert(e Endpoint) bool {
	for i := range s.Endpoints {
		if s.Endpoints[i].ID == e.ID {
			s.Endpoints[i] = e
			return false
		}
	}
	s.Endpoints = append(s.Endpoints, e)
	return true
}

func (s *EndpointsSpec) Sort() {
	sort.SliceStable(s.Endpoints, func(i, j int) bool {
		if s.Endpoints[i].Path == s.Endpoints[j].Path {
			return s.Endpoints[i].Method < s.Endpoints[j].Method
		}
		return s.Endpoints[i].Path < s.Endpoints[j].Path
	})
}

// ---------------------------------------------------------------------------
// Requisitos (docs/requisitos.md)
// ---------------------------------------------------------------------------

type Requirement struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"` // RF | RNF
	Title       string   `json:"title"`
	Priority    string   `json:"priority,omitempty"`
	Status      string   `json:"status,omitempty"`
	Components  []string `json:"components,omitempty"`
	Description string   `json:"description,omitempty"`
	// Category agrupa os RNFs no documento de requisitos (Usabilidade,
	// Desempenho, Segurança…). Texto livre.
	Category string `json:"category,omitempty"`
	// Related lista os "requisitos associados" (ids de RF/RNF). O valor
	// especial ["Todos"] significa todos os requisitos.
	Related []string `json:"related,omitempty"`
}

type RequirementsDoc struct {
	ProjectName  string        `json:"project_name"`
	Overview     string        `json:"overview"`
	Requirements []Requirement `json:"requirements"`
}

func (r *RequirementsDoc) Functional() []Requirement    { return r.filter("RF") }
func (r *RequirementsDoc) NonFunctional() []Requirement { return r.filter("RNF") }

func (r *RequirementsDoc) filter(kind string) []Requirement {
	out := []Requirement{}
	for _, req := range r.Requirements {
		if strings.EqualFold(req.Type, kind) {
			out = append(out, req)
		}
	}
	return out
}

func (r *RequirementsDoc) Upsert(req Requirement) bool {
	for i := range r.Requirements {
		if strings.EqualFold(r.Requirements[i].ID, req.ID) {
			r.Requirements[i] = req
			return false
		}
	}
	r.Requirements = append(r.Requirements, req)
	r.Sort()
	return true
}

func (r *RequirementsDoc) Sort() {
	sort.SliceStable(r.Requirements, func(i, j int) bool {
		a, b := r.Requirements[i], r.Requirements[j]
		if a.Type != b.Type {
			return a.Type == "RF"
		}
		return a.ID < b.ID
	})
}

// NextRequirementID devolve o próximo identificador livre para o tipo informado.
// NextADRID devolve o próximo identificador livre da série ADR-NNN. Usa o
// maior número existente (e não a quantidade), para não reaproveitar o número
// de uma decisão ainda gravada depois que outra foi removida.
func NextADRID(list []ADR) string {
	ids := make([]string, len(list))
	for i, adr := range list {
		ids[i] = adr.ID
	}
	return "ADR-" + fmt.Sprintf("%03d", maxSeq(ids, `^ADR-(\d+)$`)+1)
}

// NextUseCaseCode devolve o próximo código livre da série CDU.
func NextUseCaseCode(list []UseCase) string {
	codes := make([]string, len(list))
	for i, uc := range list {
		codes[i] = uc.Code
	}
	return fmt.Sprintf("CDU%03d", maxSeq(codes, `^CDU(\d+)$`)+1)
}

// maxSeq devolve o maior número capturado por pattern entre os ids (0 se nenhum).
func maxSeq(ids []string, pattern string) int {
	re := regexp.MustCompile(pattern)
	max := 0
	for _, id := range ids {
		if m := re.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(id))); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil && n > max {
				max = n
			}
		}
	}
	return max
}

func (r *RequirementsDoc) NextRequirementID(kind string) string {
	kind = strings.ToUpper(kind)
	if kind != "RNF" {
		kind = "RF"
	}
	max := 0
	re := regexp.MustCompile(`^(RF|RNF)(\d+)$`)
	for _, req := range r.Requirements {
		m := re.FindStringSubmatch(strings.ToUpper(req.ID))
		if len(m) == 3 && m[1] == kind {
			n := 0
			fmt.Sscanf(m[2], "%d", &n)
			if n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("%s%03d", kind, max+1)
}

// ---------------------------------------------------------------------------
// Casos de Uso (docs/casos-de-uso/*.md)
// ---------------------------------------------------------------------------

// Nos fluxos (main_flow, alternate_flows, exceptions), um item que começa com
// "# " é um subtítulo de grupo, não um passo: a numeração dos passos continua
// através dele (ver IsFlowSubtitle).
type UseCase struct {
	Code           string   `json:"code"`
	Name           string   `json:"name"`
	File           string   `json:"file,omitempty"`
	Description    string   `json:"description,omitempty"`
	Actors         []string `json:"actors,omitempty"`
	Requirements   []string `json:"requirements,omitempty"`
	Components     []string `json:"components,omitempty"`
	Complexity     string   `json:"complexity,omitempty"`
	EstimatedHours float64  `json:"estimated_hours,omitempty"`
	Priority       string   `json:"priority,omitempty"`
	Status         string   `json:"status,omitempty"`
	PreConditions  []string `json:"pre_conditions,omitempty"`
	PostConditions []string `json:"post_conditions,omitempty"`
	MainFlow       []string `json:"main_flow,omitempty"`
	AlternateFlows []string `json:"alternate_flows,omitempty"`
	Exceptions     []string `json:"exceptions,omitempty"`
	BusinessRules  []string `json:"business_rules,omitempty"`
	Acceptance     []string `json:"acceptance,omitempty"`
}

// FlowSubtitlePrefix marca um subtítulo de grupo dentro de um fluxo.
const FlowSubtitlePrefix = "# "

// IsFlowSubtitle informa se o item de fluxo é um subtítulo ("# Cadastro") e
// devolve o texto sem o marcador nem os dois-pontos finais.
func IsFlowSubtitle(item string) (string, bool) {
	item = strings.TrimSpace(item)
	if !strings.HasPrefix(item, FlowSubtitlePrefix) {
		return "", false
	}
	text := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(item[len(FlowSubtitlePrefix):]), ":"))
	return text, text != ""
}

// ---------------------------------------------------------------------------
// Metadados do documento de requisitos (.arch/document.yaml)
// ---------------------------------------------------------------------------

// DocRevision é uma linha do "Histórico de Alterações".
type DocRevision struct {
	Date        string `yaml:"date" json:"date"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
	Author      string `yaml:"author" json:"author"`
}

// GlossaryTerm é uma linha da tabela de convenções, termos e abreviações.
type GlossaryTerm struct {
	Term       string `yaml:"term" json:"term"`
	Definition string `yaml:"definition" json:"definition"`
}

// DocumentMeta guarda apenas os textos do documento de requisitos que não
// existem em nenhum outro arquivo do projeto (cliente, usuários, histórico,
// referências, glossário). Todo o resto é derivado dos dados do projeto.
type DocumentMeta struct {
	Title        string         `yaml:"title" json:"title"`
	Version      string         `yaml:"version" json:"version"`
	Date         string         `yaml:"date" json:"date"` // ISO; vazio = data da geração
	Authors      []string       `yaml:"authors" json:"authors"`
	Client       string         `yaml:"client" json:"client"`
	Users        string         `yaml:"users" json:"users"`
	Introduction string         `yaml:"introduction" json:"introduction"`
	History      []DocRevision  `yaml:"history" json:"history"`
	References   []string       `yaml:"references" json:"references"`
	Glossary     []GlossaryTerm `yaml:"glossary" json:"glossary"`
}

// DefaultDocumentTitle é o título usado quando document.yaml não define um.
const DefaultDocumentTitle = "Documento de Requisitos"

// Normalize preenche os padrões (título, versão e autores do manifest) e
// garante listas nunca nulas, para que a UI não precise tratar null.
func (m *DocumentMeta) Normalize(manifest *Manifest) {
	m.Title = strings.TrimSpace(m.Title)
	if m.Title == "" {
		m.Title = DefaultDocumentTitle
	}
	m.Version = strings.TrimSpace(m.Version)
	if m.Version == "" && manifest != nil {
		m.Version = manifest.Version
	}
	m.Date = strings.TrimSpace(m.Date)
	authors := []string{}
	for _, a := range m.Authors {
		if a = strings.TrimSpace(a); a != "" {
			authors = append(authors, a)
		}
	}
	if len(authors) == 0 && manifest != nil {
		for _, a := range manifest.Authors {
			if n := strings.TrimSpace(a.Name); n != "" {
				authors = append(authors, n)
			}
		}
	}
	m.Authors = authors
	if m.History == nil {
		m.History = []DocRevision{}
	}
	if m.References == nil {
		m.References = []string{}
	}
	if m.Glossary == nil {
		m.Glossary = []GlossaryTerm{}
	}
}

// ---------------------------------------------------------------------------
// ADRs (docs/architecture-decisions/*.md)
// ---------------------------------------------------------------------------

type ADR struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	File         string `json:"file,omitempty"`
	Status       string `json:"status,omitempty"`
	Date         string `json:"date,omitempty"`
	Context      string `json:"context,omitempty"`
	Decision     string `json:"decision,omitempty"`
	Consequences string `json:"consequences,omitempty"`
}

// ---------------------------------------------------------------------------
// Tarefas do AI-PRD (.arch/tasks.json)
// ---------------------------------------------------------------------------

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusBlocked    = "blocked"
)

type Task struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Tier         string   `json:"tier"`
	Component    string   `json:"component,omitempty"`
	ComponentID  string   `json:"component_id,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Acceptance   []string `json:"acceptance,omitempty"`
	Requirements []string `json:"requirements,omitempty"`
	UseCases     []string `json:"use_cases,omitempty"`
	Endpoints    []string `json:"endpoints,omitempty"`
	Status       string   `json:"status"`
	Notes        string   `json:"notes,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
	Order        int      `json:"order"`
}

type TaskBoard struct {
	Version     string `json:"version"`
	GeneratedAt string `json:"generated_at"`
	SourceHash  string `json:"source_hash"`
	TargetStack string `json:"target_stack,omitempty"`
	Tasks       []Task `json:"tasks"`
}

func NewTaskBoard() *TaskBoard {
	return &TaskBoard{Version: SchemaVersion, Tasks: []Task{}}
}

func (b *TaskBoard) ByID(id string) *Task {
	for i := range b.Tasks {
		if strings.EqualFold(b.Tasks[i].ID, id) {
			return &b.Tasks[i]
		}
	}
	return nil
}

func (b *TaskBoard) ProgressPercentage() int {
	if len(b.Tasks) == 0 {
		return 0
	}
	done := 0
	for _, t := range b.Tasks {
		if t.Status == StatusCompleted {
			done++
		}
	}
	return int(float64(done)/float64(len(b.Tasks))*100 + 0.5)
}

// ---------------------------------------------------------------------------
// Precificação (.arch/pricing.yaml)
// ---------------------------------------------------------------------------

type PricingConfig struct {
	Currency              string             `yaml:"currency" json:"currency"`
	HourlyRates           map[string]float64 `yaml:"hourly_rates" json:"hourly_rates"`
	ComplexityMultipliers map[string]float64 `yaml:"complexity_multipliers" json:"complexity_multipliers"`
	NodeTypeHours         map[string]float64 `yaml:"node_type_hours" json:"node_type_hours"`
	ComplexityFactors     map[string]float64 `yaml:"complexity_factors" json:"complexity_factors"`
	ProtocolHours         map[string]float64 `yaml:"protocol_hours" json:"protocol_hours"`
	UseCaseHours          map[string]float64 `yaml:"use_case_hours" json:"use_case_hours"`
	CloudCatalog          map[string]float64 `yaml:"cloud_catalog" json:"cloud_catalog"`
	RoleDistribution      map[string]float64 `yaml:"role_distribution" json:"role_distribution"`
	RiskMarginPercentage  float64            `yaml:"risk_margin_percentage" json:"risk_margin_percentage"`
	TaxPercentage         float64            `yaml:"tax_percentage" json:"tax_percentage"`
	TeamSize              float64            `yaml:"team_size" json:"team_size"`
	HoursPerDay           float64            `yaml:"hours_per_day" json:"hours_per_day"`
	WorkingDaysPerMonth   float64            `yaml:"working_days_per_month" json:"working_days_per_month"`
}

func DefaultPricing() *PricingConfig {
	return &PricingConfig{
		Currency: "BRL",
		HourlyRates: map[string]float64{
			"tech_lead":       250,
			"senior_engineer": 180,
			"pleno_engineer":  120,
			"cloud_architect": 260,
		},
		ComplexityMultipliers: map[string]float64{
			"frontend_screen_simple":   8,
			"frontend_screen_complex":  24,
			"backend_crud_endpoint":    6,
			"backend_complex_business": 20,
			"integration_external_api": 16,
			"database_modeling":        12,
			"ci_cd_pipeline":           16,
		},
		NodeTypeHours: map[string]float64{
			"compute":          32,
			"database":         12,
			"cache":            8,
			"queue":            16,
			"gateway":          16,
			"storage":          8,
			"client":           40,
			"external_service": 16,
			"group":            0,
		},
		ComplexityFactors: map[string]float64{"low": 0.6, "medium": 1, "high": 1.8},
		ProtocolHours: map[string]float64{
			"rest": 6, "grpc": 10, "graphql": 10, "websocket": 12,
			"sql": 4, "nosql": 4, "amqp": 10, "kafka": 12, "webhook": 8, "s3": 4,
		},
		UseCaseHours: map[string]float64{"low": 8, "medium": 16, "high": 32},
		CloudCatalog: map[string]float64{
			"t4g.small":       95,
			"t4g.medium":      190,
			"db.t4g.micro":    110,
			"db.t4g.small":    220,
			"ecs.fargate.0.5": 140,
			"lambda.low":      25,
			"elasticache.t4g": 130,
			"s3.standard":     30,
			"cloudfront":      45,
			"vercel.pro":      110,
			"supabase.pro":    135,
			"cloud_run.small": 80,
		},
		RoleDistribution: map[string]float64{
			"tech_lead":       0.15,
			"senior_engineer": 0.45,
			"pleno_engineer":  0.30,
			"cloud_architect": 0.10,
		},
		RiskMarginPercentage: 20,
		TaxPercentage:        15,
		TeamSize:             2,
		HoursPerDay:          6,
		WorkingDaysPerMonth:  21,
	}
}

// Normalize garante que mapas obrigatórios existam mesmo em arquivos parciais.
func (p *PricingConfig) Normalize() {
	def := DefaultPricing()
	if p.Currency == "" {
		p.Currency = def.Currency
	}
	mergeFloatMap(&p.HourlyRates, def.HourlyRates)
	mergeFloatMap(&p.ComplexityMultipliers, def.ComplexityMultipliers)
	mergeFloatMap(&p.NodeTypeHours, def.NodeTypeHours)
	mergeFloatMap(&p.ComplexityFactors, def.ComplexityFactors)
	mergeFloatMap(&p.ProtocolHours, def.ProtocolHours)
	mergeFloatMap(&p.UseCaseHours, def.UseCaseHours)
	mergeFloatMap(&p.CloudCatalog, def.CloudCatalog)
	if len(p.RoleDistribution) == 0 {
		p.RoleDistribution = def.RoleDistribution
	}
	if p.TeamSize <= 0 {
		p.TeamSize = def.TeamSize
	}
	if p.HoursPerDay <= 0 {
		p.HoursPerDay = def.HoursPerDay
	}
	if p.WorkingDaysPerMonth <= 0 {
		p.WorkingDaysPerMonth = def.WorkingDaysPerMonth
	}
}

func mergeFloatMap(dst *map[string]float64, src map[string]float64) {
	if *dst == nil {
		*dst = map[string]float64{}
	}
	for k, v := range src {
		if _, ok := (*dst)[k]; !ok {
			(*dst)[k] = v
		}
	}
}

// ---------------------------------------------------------------------------
// Utilidades compartilhadas
// ---------------------------------------------------------------------------

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify normaliza rótulos livres em identificadores estáveis e determinísticos.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case unicode.IsSpace(r), r == '-', r == '_', r == '.', r == '/':
			b.WriteRune('-')
		default:
			// transliteração simples de acentos comuns em PT-BR
			switch r {
			case 'á', 'à', 'â', 'ã', 'ä':
				b.WriteRune('a')
			case 'é', 'ê', 'ë':
				b.WriteRune('e')
			case 'í', 'î', 'ï':
				b.WriteRune('i')
			case 'ó', 'ô', 'õ', 'ö':
				b.WriteRune('o')
			case 'ú', 'û', 'ü':
				b.WriteRune('u')
			case 'ç':
				b.WriteRune('c')
			case 'ñ':
				b.WriteRune('n')
			default:
				b.WriteRune('-')
			}
		}
	}
	out := nonAlnum.ReplaceAllString(b.String(), "-")
	return strings.Trim(out, "-")
}

func NormalizeComplexity(c string) string {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "xs", "low", "baixa", "baixo", "simples":
		return "low"
	case "l", "xl", "high", "alta", "alto", "complexa", "complexo":
		return "high"
	case "":
		return ""
	default:
		return "medium"
	}
}

func NormalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "completed", "done", "concluido", "concluído", "feito":
		return StatusCompleted
	case "in_progress", "in-progress", "doing", "andamento", "em_andamento":
		return StatusInProgress
	case "blocked", "bloqueado":
		return StatusBlocked
	default:
		return StatusPending
	}
}
