package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Diagramas UML (.arch/diagrams/{usecase,class,sequence,state}/<id>.json)
// ---------------------------------------------------------------------------
//
// Os diagramas UML coexistem com o diagrama macro de arquitetura. Cada um é um
// arquivo JSON independente, legível e versionável, com espelho Mermaid
// opcional. A semântica de direção das relações segue a convenção "seta ou
// losango sempre na ponta TARGET".

const (
	UMLKindUseCase  = "usecase"
	UMLKindClass    = "class"
	UMLKindSequence = "sequence"
	UMLKindState    = "state"
)

// UMLKinds lista os tipos de diagrama na ordem canônica de exibição.
var UMLKinds = []string{UMLKindUseCase, UMLKindClass, UMLKindSequence, UMLKindState}

// UMLKindLabel é o rótulo humano de cada tipo de diagrama.
var UMLKindLabel = map[string]string{
	UMLKindUseCase:  "Casos de uso",
	UMLKindClass:    "Classes",
	UMLKindSequence: "Sequência",
	UMLKindState:    "Estados",
}

// UMLElementTypes define os elementos válidos em cada tipo de diagrama.
var UMLElementTypes = map[string][]string{
	UMLKindUseCase:  {"actor", "usecase", "boundary", "note"},
	UMLKindClass:    {"class", "interface", "enum", "package", "note"},
	UMLKindSequence: {"lifeline", "fragment", "note"},
	UMLKindState:    {"state", "initial", "final", "choice", "fork", "join", "history", "note"},
}

// UMLRelationTypes define as relações válidas em cada tipo de diagrama.
var UMLRelationTypes = map[string][]string{
	UMLKindUseCase: {"association", "include", "extend", "generalization", "dependency", "note_link"},
	UMLKindClass: {"association", "directed_association", "aggregation", "composition",
		"generalization", "realization", "dependency", "note_link"},
	UMLKindSequence: {"message", "note_link"},
	UMLKindState:    {"transition", "note_link"},
}

// Valores enumerados aceitos nos campos opcionais.
var (
	UMLVisibilities  = []string{"+", "-", "#", "~"}
	UMLLifelineKinds = []string{"participant", "actor", "boundary", "control", "entity", "database"}
	UMLOperators     = []string{"alt", "opt", "loop", "par", "break", "critical", "ref"}
	UMLMessageKinds  = []string{"sync", "async", "reply", "create", "destroy"}
)

// umlUnnamedTypes são os elementos que dispensam nome (pseudo-estados e notas).
var umlUnnamedTypes = map[string]bool{
	"initial": true, "final": true, "choice": true, "fork": true,
	"join": true, "history": true, "note": true,
}

// UMLDefaultSizes são os tamanhos padrão (largura, altura) usados apenas pela
// colocação automática; a renderização real é decidida pelo frontend.
var UMLDefaultSizes = map[string][2]float64{
	"actor": {60, 100}, "usecase": {160, 70}, "boundary": {420, 400}, "note": {180, 80},
	"class": {200, 120}, "interface": {200, 120}, "enum": {200, 120}, "package": {240, 160},
	"lifeline": {140, 44}, "fragment": {420, 180},
	"state": {160, 70}, "initial": {24, 24}, "final": {28, 28}, "choice": {36, 36},
	"fork": {120, 8}, "join": {120, 8}, "history": {32, 32},
}

// ErrUMLNotFound é a causa raiz de todo "diagrama/elemento/relação inexistente",
// permitindo à camada HTTP responder 404 sem inspecionar mensagens.
var ErrUMLNotFound = errors.New("item UML não encontrado")

type umlNotFound struct{ msg string }

func (e *umlNotFound) Error() string { return e.msg }
func (e *umlNotFound) Unwrap() error { return ErrUMLNotFound }

// UMLNotFound cria um erro legível que satisfaz errors.Is(err, ErrUMLNotFound).
func UMLNotFound(format string, args ...any) error {
	return &umlNotFound{msg: fmt.Sprintf(format, args...)}
}

// UMLMember é um atributo ou operação de classe/interface.
type UMLMember struct {
	Name       string `json:"name"`
	Type       string `json:"type,omitempty"`
	Visibility string `json:"visibility,omitempty"`
	Static     bool   `json:"static,omitempty"`
	Abstract   bool   `json:"abstract,omitempty"`
	Default    string `json:"default,omitempty"`
	Params     string `json:"params,omitempty"`
}

// UnmarshalJSON aceita tanto o objeto canônico quanto a notação textual UML
// ("+ login(email: string): Token"), o que simplifica a vida de IAs e humanos
// que editam o JSON à mão. A gravação é sempre no formato de objeto.
func (m *UMLMember) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, `"`) {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*m, _ = ParseUMLMember(s)
		return nil
	}
	type plain UMLMember
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		return err
	}
	*m = UMLMember(p)
	return nil
}

// ParseUMLMember interpreta a notação textual de um membro:
//
//	"+ email: string"                 atributo
//	"- tentativas: int = 0"           atributo com valor padrão
//	"+ login(email: string): Token"   operação
//	"+ {static} novo(): User"         modificadores {static}/{abstract} (ou sufixos $ e *)
//
// O segundo retorno indica se o texto descreve uma operação (tem parênteses).
func ParseUMLMember(s string) (UMLMember, bool) {
	m := UMLMember{}
	s = strings.TrimSpace(s)
	if s != "" && strings.ContainsRune("+-#~", rune(s[0])) {
		m.Visibility = s[:1]
		s = strings.TrimSpace(s[1:])
	}
	for {
		lower := strings.ToLower(s)
		switch {
		case strings.HasPrefix(lower, "{static}"):
			m.Static, s = true, strings.TrimSpace(s[len("{static}"):])
			continue
		case strings.HasPrefix(lower, "static "):
			m.Static, s = true, strings.TrimSpace(s[len("static "):])
			continue
		case strings.HasPrefix(lower, "{abstract}"):
			m.Abstract, s = true, strings.TrimSpace(s[len("{abstract}"):])
			continue
		case strings.HasPrefix(lower, "abstract "):
			m.Abstract, s = true, strings.TrimSpace(s[len("abstract "):])
			continue
		}
		break
	}
	for strings.HasSuffix(s, "$") || strings.HasSuffix(s, "*") {
		if strings.HasSuffix(s, "$") {
			m.Static = true
		} else {
			m.Abstract = true
		}
		s = strings.TrimSpace(s[:len(s)-1])
	}

	open := strings.IndexByte(s, '(')
	closeIdx := strings.LastIndexByte(s, ')')
	if open >= 0 && closeIdx > open {
		m.Name = strings.TrimSpace(s[:open])
		m.Params = strings.TrimSpace(s[open+1 : closeIdx])
		rest := strings.TrimSpace(s[closeIdx+1:])
		m.Type = strings.TrimSpace(strings.TrimPrefix(rest, ":"))
		return m, true
	}

	if eq := strings.Index(s, "="); eq >= 0 {
		m.Default = strings.TrimSpace(s[eq+1:])
		s = strings.TrimSpace(s[:eq])
	}
	if colon := strings.Index(s, ":"); colon >= 0 {
		m.Name = strings.TrimSpace(s[:colon])
		m.Type = strings.TrimSpace(s[colon+1:])
	} else {
		m.Name = s
	}
	return m, false
}

// FormatUMLMember devolve a notação textual UML do membro, inversa de
// ParseUMLMember. `operation` indica se deve ser formatado com parênteses.
func FormatUMLMember(m UMLMember, operation bool) string {
	var b strings.Builder
	if m.Visibility != "" {
		b.WriteString(m.Visibility + " ")
	}
	if m.Static {
		b.WriteString("{static} ")
	}
	if m.Abstract {
		b.WriteString("{abstract} ")
	}
	b.WriteString(m.Name)
	if operation {
		b.WriteString("(" + m.Params + ")")
	}
	if m.Type != "" {
		b.WriteString(": " + m.Type)
	}
	if !operation && m.Default != "" {
		b.WriteString(" = " + m.Default)
	}
	return b.String()
}

// UMLElement é um nó de qualquer diagrama UML. Os campos específicos de cada
// tipo são opcionais e omitidos na serialização quando vazios.
type UMLElement struct {
	ID            string      `json:"id"`
	Type          string      `json:"type"`
	Name          string      `json:"name"`
	Position      Position    `json:"position"`
	Width         float64     `json:"width,omitempty"`
	Height        float64     `json:"height,omitempty"`
	ParentID      string      `json:"parent_id,omitempty"`
	Stereotype    string      `json:"stereotype,omitempty"`
	Documentation string      `json:"documentation,omitempty"`
	Abstract      bool        `json:"abstract,omitempty"`
	Attributes    []UMLMember `json:"attributes,omitempty"`
	Operations    []UMLMember `json:"operations,omitempty"`
	Literals      []string    `json:"literals,omitempty"`
	Entry         string      `json:"entry,omitempty"`
	Do            string      `json:"do,omitempty"`
	Exit          string      `json:"exit,omitempty"`
	LifelineKind  string      `json:"lifeline_kind,omitempty"`
	Operator      string      `json:"operator,omitempty"`
	Guard         string      `json:"guard,omitempty"`
	UseCase       string      `json:"use_case,omitempty"`
	ComponentID   string      `json:"component_id,omitempty"`
}

// Size devolve a largura e a altura efetivas (explícitas ou padrão do tipo).
func (e UMLElement) Size() (float64, float64) {
	w, h := e.Width, e.Height
	def, ok := UMLDefaultSizes[e.Type]
	if !ok {
		def = [2]float64{160, 70}
	}
	if w <= 0 {
		w = def[0]
	}
	if h <= 0 {
		h = def[1]
	}
	return w, h
}

// UMLRelation liga dois elementos do mesmo diagrama.
type UMLRelation struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Source             string `json:"source"`
	Target             string `json:"target"`
	Name               string `json:"name,omitempty"`
	SourceMultiplicity string `json:"source_multiplicity,omitempty"`
	TargetMultiplicity string `json:"target_multiplicity,omitempty"`
	SourceRole         string `json:"source_role,omitempty"`
	TargetRole         string `json:"target_role,omitempty"`
	MessageKind        string `json:"message_kind,omitempty"`
	Order              int    `json:"order,omitempty"`
	Trigger            string `json:"trigger,omitempty"`
	Guard              string `json:"guard,omitempty"`
	Effect             string `json:"effect,omitempty"`
	Documentation      string `json:"documentation,omitempty"`
}

// UMLDiagram é o conteúdo de um arquivo .arch/diagrams/<kind>/<id>.json.
// `File` só existe nas respostas da API: o store o preenche na leitura e o
// remove antes de gravar.
type UMLDiagram struct {
	ID           string        `json:"id"`
	Kind         string        `json:"kind"`
	Name         string        `json:"name"`
	Description  string        `json:"description,omitempty"`
	Version      string        `json:"version"`
	LastModified string        `json:"last_modified"`
	Viewport     Viewport      `json:"viewport"`
	Elements     []UMLElement  `json:"elements"`
	Relations    []UMLRelation `json:"relations"`
	File         string        `json:"file,omitempty"`
}

// NewUMLDiagram cria um diagrama vazio já normalizado.
func NewUMLDiagram(id, kind, name string) *UMLDiagram {
	d := &UMLDiagram{ID: id, Kind: kind, Name: name}
	d.Touch()
	return d
}

// ValidUMLKind informa se o tipo de diagrama é suportado.
func ValidUMLKind(kind string) bool {
	_, ok := UMLElementTypes[kind]
	return ok
}

func (d *UMLDiagram) Touch() {
	d.Version = SchemaVersion
	d.LastModified = time.Now().UTC().Format(time.RFC3339)
	if d.Elements == nil {
		d.Elements = []UMLElement{}
	}
	if d.Relations == nil {
		d.Relations = []UMLRelation{}
	}
	if d.Viewport.Zoom == 0 {
		d.Viewport.Zoom = 1
	}
}

func (d *UMLDiagram) ElementByID(id string) *UMLElement {
	for i := range d.Elements {
		if d.Elements[i].ID == id {
			return &d.Elements[i]
		}
	}
	return nil
}

func (d *UMLDiagram) RelationByID(id string) *UMLRelation {
	for i := range d.Relations {
		if d.Relations[i].ID == id {
			return &d.Relations[i]
		}
	}
	return nil
}

// ResolveElement aceita id exato, nome exato (case-insensitive) ou slug do nome,
// na mesma linha de Diagram.ResolveNode — IAs referenciam elementos pelo nome.
func (d *UMLDiagram) ResolveElement(ref string) *UMLElement {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	if e := d.ElementByID(ref); e != nil {
		return e
	}
	for i := range d.Elements {
		if d.Elements[i].Name != "" && strings.EqualFold(d.Elements[i].Name, ref) {
			return &d.Elements[i]
		}
	}
	slug := Slugify(ref)
	for i := range d.Elements {
		if d.Elements[i].ID == "el-"+slug || (d.Elements[i].Name != "" && Slugify(d.Elements[i].Name) == slug) {
			return &d.Elements[i]
		}
	}
	return nil
}

// UniqueElementID gera `el-<slug>` livre no diagrama (sufixo -2, -3… em colisão).
func (d *UMLDiagram) UniqueElementID(name, typ string) string {
	base := Slugify(name)
	if base == "" {
		base = Slugify(typ)
	}
	if base == "" {
		base = "elemento"
	}
	base = "el-" + base
	id, i := base, 2
	for d.ElementByID(id) != nil || d.RelationByID(id) != nil {
		id = fmt.Sprintf("%s-%d", base, i)
		i++
	}
	return id
}

// UniqueRelationID gera `rel-<n>` livre no diagrama.
func (d *UMLDiagram) UniqueRelationID() string {
	n := len(d.Relations) + 1
	for {
		id := fmt.Sprintf("rel-%d", n)
		if d.RelationByID(id) == nil && d.ElementByID(id) == nil {
			return id
		}
		n++
	}
}

// RemoveElement apaga o elemento, todas as relações ligadas a ele e zera o
// parent_id dos filhos. Mensagens restantes são renumeradas.
func (d *UMLDiagram) RemoveElement(id string) bool {
	found := false
	kept := make([]UMLElement, 0, len(d.Elements))
	for _, e := range d.Elements {
		if e.ID == id {
			found = true
			continue
		}
		kept = append(kept, e)
	}
	if !found {
		return false
	}
	for i := range kept {
		if kept[i].ParentID == id {
			kept[i].ParentID = ""
		}
	}
	d.Elements = kept
	rels := make([]UMLRelation, 0, len(d.Relations))
	for _, r := range d.Relations {
		if r.Source == id || r.Target == id {
			continue
		}
		rels = append(rels, r)
	}
	d.Relations = rels
	d.RenumberMessages()
	return true
}

// RemoveRelation apaga a relação; se era mensagem, renumera as demais 1..n.
func (d *UMLDiagram) RemoveRelation(id string) bool {
	found := false
	kept := make([]UMLRelation, 0, len(d.Relations))
	for _, r := range d.Relations {
		if r.ID == id {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	d.Relations = kept
	if found {
		d.RenumberMessages()
	}
	return found
}

// messageIndexes devolve os índices das mensagens na ordem vertical atual. Mensagens
// sem ordem (0) vão para o fim, preservando a ordem de inserção.
func (d *UMLDiagram) messageIndexes() []int {
	idx := []int{}
	for i, r := range d.Relations {
		if r.Type == "message" {
			idx = append(idx, i)
		}
	}
	sort.SliceStable(idx, func(a, b int) bool {
		oa, ob := d.Relations[idx[a]].Order, d.Relations[idx[b]].Order
		if oa == 0 {
			return false
		}
		if ob == 0 {
			return true
		}
		return oa < ob
	})
	return idx
}

// RenumberMessages garante ordens únicas e contíguas 1..n para as mensagens.
func (d *UMLDiagram) RenumberMessages() {
	for n, i := range d.messageIndexes() {
		d.Relations[i].Order = n + 1
	}
}

// MoveMessage posiciona a mensagem `id` na ordem `order` (1..n), deslocando as
// demais, e renumera tudo.
func (d *UMLDiagram) MoveMessage(id string, order int) {
	idx := d.messageIndexes()
	ids := make([]string, 0, len(idx))
	for _, i := range idx {
		if d.Relations[i].ID != id {
			ids = append(ids, d.Relations[i].ID)
		}
	}
	if order < 1 {
		order = 1
	}
	if order > len(ids)+1 {
		order = len(ids) + 1
	}
	ids = append(ids[:order-1], append([]string{id}, ids[order-1:]...)...)
	for n, rid := range ids {
		if r := d.RelationByID(rid); r != nil {
			r.Order = n + 1
		}
	}
}

// MaxMessageOrder devolve a maior ordem de mensagem existente.
func (d *UMLDiagram) MaxMessageOrder() int {
	max := 0
	for _, r := range d.Relations {
		if r.Type == "message" && r.Order > max {
			max = r.Order
		}
	}
	return max
}

// Normalize aplica os padrões do contrato: listas nunca nulas, lifeline como
// participant, fragmento como alt, mensagem síncrona, parent_id órfão removido
// e mensagens numeradas 1..n.
func (d *UMLDiagram) Normalize() {
	if d.Elements == nil {
		d.Elements = []UMLElement{}
	}
	if d.Relations == nil {
		d.Relations = []UMLRelation{}
	}
	if d.Viewport.Zoom == 0 {
		d.Viewport.Zoom = 1
	}
	ids := map[string]bool{}
	for _, e := range d.Elements {
		ids[e.ID] = true
	}
	for i := range d.Elements {
		e := &d.Elements[i]
		e.Type = strings.ToLower(strings.TrimSpace(e.Type))
		e.Name = strings.TrimSpace(e.Name)
		if e.ParentID == e.ID || (e.ParentID != "" && !ids[e.ParentID]) {
			e.ParentID = ""
		}
		// Contenção cíclica (a ⊂ b ⊂ a) é desfeita no elemento que a fecha.
		if e.ParentID != "" && d.isAncestor(e.ID, e.ParentID) {
			e.ParentID = ""
		}
		switch e.Type {
		case "lifeline":
			if e.LifelineKind == "" {
				e.LifelineKind = "participant"
			}
		case "fragment":
			if e.Operator == "" {
				e.Operator = "alt"
			}
		}
		e.Stereotype = strings.Trim(strings.TrimSpace(e.Stereotype), "«»<>")
	}
	for i := range d.Relations {
		r := &d.Relations[i]
		r.Type = strings.ToLower(strings.TrimSpace(r.Type))
		if r.Type == "message" && r.MessageKind == "" {
			r.MessageKind = "sync"
		}
		if r.Type != "message" {
			r.Order = 0
		}
	}
	d.RenumberMessages()
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// ValidateUMLElement confere tipo, nome obrigatório e campos enumerados.
func ValidateUMLElement(kind string, e UMLElement) error {
	types := UMLElementTypes[kind]
	if !contains(types, e.Type) {
		return fmt.Errorf("tipo de elemento inválido para diagrama %s: %q (válidos: %s)",
			kind, e.Type, strings.Join(types, ", "))
	}
	if strings.TrimSpace(e.Name) == "" && !umlUnnamedTypes[e.Type] {
		return fmt.Errorf("elemento do tipo %s precisa de um nome (name)", e.Type)
	}
	for _, m := range append(append([]UMLMember{}, e.Attributes...), e.Operations...) {
		if strings.TrimSpace(m.Name) == "" {
			return fmt.Errorf("membro sem nome em %q", e.Name)
		}
		if m.Visibility != "" && !contains(UMLVisibilities, m.Visibility) {
			return fmt.Errorf("visibilidade inválida %q em %s.%s (válidas: + - # ~)", m.Visibility, e.Name, m.Name)
		}
	}
	if e.LifelineKind != "" && !contains(UMLLifelineKinds, e.LifelineKind) {
		return fmt.Errorf("lifeline_kind inválido: %q (válidos: %s)", e.LifelineKind, strings.Join(UMLLifelineKinds, ", "))
	}
	if e.Operator != "" && !contains(UMLOperators, e.Operator) {
		return fmt.Errorf("operator inválido: %q (válidos: %s)", e.Operator, strings.Join(UMLOperators, ", "))
	}
	return nil
}

// ValidateUMLRelation confere tipo, extremidades e regras de direção.
func (d *UMLDiagram) ValidateUMLRelation(r UMLRelation) error {
	types := UMLRelationTypes[d.Kind]
	if !contains(types, r.Type) {
		return fmt.Errorf("tipo de relação inválido para diagrama %s: %q (válidos: %s)",
			d.Kind, r.Type, strings.Join(types, ", "))
	}
	src := d.ElementByID(r.Source)
	if src == nil {
		return UMLNotFound("elemento de origem não encontrado: %q", r.Source)
	}
	dst := d.ElementByID(r.Target)
	if dst == nil {
		return UMLNotFound("elemento de destino não encontrado: %q", r.Target)
	}
	if r.Source == r.Target && r.Type != "message" && r.Type != "transition" {
		return fmt.Errorf("relação %s não pode ligar um elemento a ele mesmo", r.Type)
	}
	switch r.Type {
	case "note_link":
		if src.Type != "note" && dst.Type != "note" {
			return errors.New("note_link precisa de uma nota (note) em uma das pontas")
		}
	case "message":
		if src.Type != "lifeline" || dst.Type != "lifeline" {
			return errors.New("mensagens só ligam lifelines")
		}
		if r.MessageKind != "" && !contains(UMLMessageKinds, r.MessageKind) {
			return fmt.Errorf("message_kind inválido: %q (válidos: %s)", r.MessageKind, strings.Join(UMLMessageKinds, ", "))
		}
	default:
		if src.Type == "note" || dst.Type == "note" {
			return fmt.Errorf("notas só se ligam por note_link, não por %s", r.Type)
		}
	}
	return nil
}

// Validate confere o diagrama inteiro: tipo, ids únicos, elementos e relações.
func (d *UMLDiagram) Validate() error {
	if !ValidUMLKind(d.Kind) {
		return fmt.Errorf("tipo de diagrama inválido: %q (válidos: %s)", d.Kind, strings.Join(UMLKinds, ", "))
	}
	seen := map[string]bool{}
	for _, e := range d.Elements {
		if strings.TrimSpace(e.ID) == "" {
			return fmt.Errorf("elemento %q sem id", e.Name)
		}
		if seen[e.ID] {
			return fmt.Errorf("id duplicado no diagrama: %q", e.ID)
		}
		seen[e.ID] = true
		if err := ValidateUMLElement(d.Kind, e); err != nil {
			return err
		}
	}
	for _, r := range d.Relations {
		if strings.TrimSpace(r.ID) == "" {
			return fmt.Errorf("relação %s sem id", r.Type)
		}
		if seen[r.ID] {
			return fmt.Errorf("id duplicado no diagrama: %q", r.ID)
		}
		seen[r.ID] = true
		if err := d.ValidateUMLRelation(r); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Posicionamento automático
// ---------------------------------------------------------------------------

type umlRect struct{ x, y, w, h float64 }

func (a umlRect) overlaps(b umlRect) bool {
	return a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
}

func rectOf(e UMLElement) umlRect {
	w, h := e.Size()
	return umlRect{e.Position.X, e.Position.Y, w, h}
}

// isAncestor informa se `anc` contém `id` direta ou indiretamente.
func (d *UMLDiagram) isAncestor(anc, id string) bool {
	for depth := 0; depth < 64; depth++ {
		e := d.ElementByID(id)
		if e == nil || e.ParentID == "" {
			return false
		}
		if e.ParentID == anc {
			return true
		}
		id = e.ParentID
	}
	return false
}

// fits informa se o retângulo candidato (com margem) não colide com nenhum
// elemento existente, ignorando os contêineres ancestrais do novo elemento.
func (d *UMLDiagram) fits(r umlRect, parentID string) bool {
	const margin = 20
	probe := umlRect{r.x - margin/2, r.y - margin/2, r.w + margin, r.h + margin}
	for _, e := range d.Elements {
		if parentID != "" && (e.ID == parentID || d.isAncestor(e.ID, parentID)) {
			continue
		}
		if probe.overlaps(rectOf(e)) {
			return false
		}
	}
	return true
}

// AutoPosition escolhe uma posição livre para um novo elemento, sem mover os
// existentes (mesma invariante do diagrama macro, RF017):
//   - lifeline: alinhadas no topo, x = 40 + i*200;
//   - com parent_id: primeira vaga livre dentro do contêiner;
//   - demais: primeira célula livre de uma grade 240x160 a partir de (40,40);
//     atores preferem a coluna x=40.
func (d *UMLDiagram) AutoPosition(e UMLElement) Position {
	w, h := e.Size()
	if e.Type == "lifeline" {
		n := 0
		for _, el := range d.Elements {
			if el.Type == "lifeline" {
				n++
			}
		}
		return Position{X: 40 + float64(n)*200, Y: 40}
	}

	if parent := d.ElementByID(e.ParentID); parent != nil {
		pw, _ := parent.Size()
		stepX, stepY := w+40, h+30
		cols := int((pw - 40) / stepX)
		if cols < 1 {
			cols = 1
		}
		// Centraliza a grade de vagas na largura do contêiner.
		startX := parent.Position.X + (pw-(float64(cols)*stepX-40))/2
		if startX < parent.Position.X+20 {
			startX = parent.Position.X + 20
		}
		for row := 0; row < 200; row++ {
			for col := 0; col < cols; col++ {
				pos := Position{X: startX + float64(col)*stepX, Y: parent.Position.Y + 60 + float64(row)*stepY}
				if d.fits(umlRect{pos.X, pos.Y, w, h}, parent.ID) {
					return pos
				}
			}
		}
	}

	if e.Type == "actor" {
		for row := 0; row < 60; row++ {
			pos := Position{X: 40, Y: 40 + float64(row)*160}
			if d.fits(umlRect{pos.X, pos.Y, w, h}, "") {
				return pos
			}
		}
	}
	const cols = 6
	for row := 0; ; row++ {
		for col := 0; col < cols; col++ {
			pos := Position{X: 40 + float64(col)*240, Y: 40 + float64(row)*160}
			if d.fits(umlRect{pos.X, pos.Y, w, h}, "") {
				return pos
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Diagrama de casos de uso derivado das fichas docs/casos-de-uso
// ---------------------------------------------------------------------------

// SyncUseCaseDiagram completa o diagrama de casos de uso a partir das fichas:
// um ator por nome em `actors`, um usecase por CDU (vinculado por use_case),
// uma fronteira com o nome do sistema contendo os usecases e associações
// ator–caso de uso. É idempotente: reaproveita o que já existe, preserva
// posições e só adiciona o que falta. Devolve true se algo mudou.
func SyncUseCaseDiagram(d *UMLDiagram, systemName string, ucs []UseCase) bool {
	changed := false
	if strings.TrimSpace(systemName) == "" {
		systemName = "Sistema"
	}

	// Fronteira do sistema: por nome, ou a primeira existente, ou uma nova.
	boundaryID := ""
	for _, e := range d.Elements {
		if e.Type == "boundary" && strings.EqualFold(e.Name, systemName) {
			boundaryID = e.ID
			break
		}
	}
	if boundaryID == "" {
		for _, e := range d.Elements {
			if e.Type == "boundary" {
				boundaryID = e.ID
				break
			}
		}
	}
	if boundaryID == "" {
		el := UMLElement{ID: d.UniqueElementID(systemName, "boundary"), Type: "boundary", Name: systemName}
		el.Position = Position{X: 240, Y: 40}
		if !d.fits(rectOf(el), "") {
			el.Position = d.AutoPosition(el)
		}
		d.Elements = append(d.Elements, el)
		boundaryID = el.ID
		changed = true
	}

	actorByName := func(name string) *UMLElement {
		for i := range d.Elements {
			if d.Elements[i].Type == "actor" && strings.EqualFold(strings.TrimSpace(d.Elements[i].Name), strings.TrimSpace(name)) {
				return &d.Elements[i]
			}
		}
		return nil
	}
	useCaseByCode := func(code string) *UMLElement {
		for i := range d.Elements {
			if d.Elements[i].Type == "usecase" && strings.EqualFold(d.Elements[i].UseCase, code) {
				return &d.Elements[i]
			}
		}
		return nil
	}
	hasAssociation := func(a, b string) bool {
		for _, r := range d.Relations {
			if r.Type == "association" && ((r.Source == a && r.Target == b) || (r.Source == b && r.Target == a)) {
				return true
			}
		}
		return false
	}

	for _, uc := range ucs {
		ucEl := useCaseByCode(uc.Code)
		if ucEl == nil {
			el := UMLElement{
				ID: d.UniqueElementID(uc.Code, "usecase"), Type: "usecase", Name: uc.Name,
				UseCase: uc.Code, ParentID: boundaryID,
			}
			el.Position = d.AutoPosition(el)
			d.Elements = append(d.Elements, el)
			changed = true
			growToContain(d, boundaryID, el)
		}
		ucID := useCaseByCode(uc.Code).ID

		for _, actor := range uc.Actors {
			actor = strings.TrimSpace(actor)
			if actor == "" || actor == "-" {
				continue
			}
			act := actorByName(actor)
			if act == nil {
				el := UMLElement{ID: d.UniqueElementID(actor, "actor"), Type: "actor", Name: actor}
				el.Position = d.AutoPosition(el)
				d.Elements = append(d.Elements, el)
				changed = true
				act = actorByName(actor)
			}
			if !hasAssociation(act.ID, ucID) {
				d.Relations = append(d.Relations, UMLRelation{
					ID: d.UniqueRelationID(), Type: "association", Source: act.ID, Target: ucID,
				})
				changed = true
			}
		}
	}
	return changed
}

// growToContain amplia o contêiner (sem movê-lo) para abrigar o filho.
func growToContain(d *UMLDiagram, parentID string, child UMLElement) {
	p := d.ElementByID(parentID)
	if p == nil {
		return
	}
	pw, ph := p.Size()
	cw, ch := child.Size()
	needW := child.Position.X + cw + 40 - p.Position.X
	needH := child.Position.Y + ch + 40 - p.Position.Y
	if needW > pw {
		p.Width = needW
	}
	if needH > ph {
		p.Height = needH
	}
}
