// Package reqdoc gera o Documento de Requisitos formal do projeto (formato do
// modelo IEEE 830 adotado pela equipe: capa, histórico, sumário, introdução,
// descrição geral, RFs, RNFs por categoria, casos de uso detalhados…).
//
// Nada é digitado duas vezes: requisitos, fichas de casos de uso, diagramas
// UML, arquitetura, ADRs e contratos vêm dos próprios arquivos do projeto. Só
// os textos que não existem em nenhum outro lugar (cliente, usuários,
// histórico, referências, glossário) ficam em .arch/document.yaml.
//
// O resultado é um documento estruturado em blocos (consumido pela UI para a
// pré-visualização, o PDF e o DOCX) e o Markdown equivalente (Markdown).
package reqdoc

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/archcode/studio/internal/model"
)

// Tipos de bloco do documento estruturado.
const (
	BlockHeading   = "heading"
	BlockParagraph = "paragraph"
	BlockList      = "list"
	BlockTable     = "table"
	BlockPriority  = "priority"
	BlockImage     = "image"
	BlockPageBreak = "pagebreak"
)

// Níveis de prioridade (valor do bloco "priority").
const (
	PriorityEssential = "essencial"
	PriorityImportant = "importante"
	PriorityDesirable = "desejavel"
)

// MacroDiagramID identifica, no campo `diagram` das figuras, o diagrama macro
// de arquitetura (os demais usam o id do diagrama UML).
const MacroDiagramID = "macro"

// MacroImageSrc é a rota que renderiza o diagrama macro para o documento, no
// estilo de figura de documento (style=document): fundo branco, traço preto,
// sem ícones nem sombras, como os diagramas UML.
const MacroImageSrc = "/api/export/svg?mode=engineering&theme=light&title=0&style=document"

// UMLImageSrc devolve a rota do SVG de um diagrama UML.
func UMLImageSrc(id string) string { return "/api/export/uml/" + url.PathEscape(id) + ".svg" }

// Input reúne tudo de que o gerador precisa; normalmente vem do snapshot.
type Input struct {
	Manifest     *model.Manifest
	Meta         model.DocumentMeta
	Diagram      *model.Diagram
	Requirements *model.RequirementsDoc
	UseCases     []model.UseCase
	ADRs         []model.ADR
	Endpoints    *model.EndpointsSpec
	UMLDiagrams  []model.UMLDiagram
	// Now é a data usada quando meta.date está vazio (zero = time.Now()).
	Now time.Time
}

// HistoryRow é uma linha da tabela "Histórico de Alterações" (data dd/mm/aaaa).
type HistoryRow struct {
	Date        string `json:"date"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Author      string `json:"author"`
}

// TOCEntry é uma entrada do sumário, apontando para a âncora de um título.
type TOCEntry struct {
	ID     string `json:"id"`
	Number string `json:"number"`
	Title  string `json:"title"`
	Level  int    `json:"level"`
}

// Block é uma unidade do corpo do documento. Só os campos do tipo são
// serializados (ver MarshalJSON). Textos, itens e células aceitam apenas o
// inline **negrito**, *itálico*, `código` e [texto](#ancora).
type Block struct {
	Type    string     `json:"type"`
	Level   int        `json:"level,omitempty"`
	Number  string     `json:"number,omitempty"`
	Text    string     `json:"text,omitempty"`
	ID      string     `json:"id,omitempty"`
	Ordered bool       `json:"ordered,omitempty"`
	Start   int        `json:"start,omitempty"`
	Items   []string   `json:"items,omitempty"`
	Header  []string   `json:"header,omitempty"`
	Rows    [][]string `json:"rows,omitempty"`
	Value   string     `json:"value,omitempty"`
	Src     string     `json:"src,omitempty"`
	Caption string     `json:"caption,omitempty"`
	Diagram string     `json:"diagram,omitempty"`
}

// MarshalJSON emite exatamente os campos de cada tipo, com listas nunca nulas
// e `ordered` sempre presente nas listas.
func (b Block) MarshalJSON() ([]byte, error) {
	switch b.Type {
	case BlockHeading:
		return json.Marshal(struct {
			Type   string `json:"type"`
			Level  int    `json:"level"`
			Number string `json:"number"`
			Text   string `json:"text"`
			ID     string `json:"id"`
		}{b.Type, b.Level, b.Number, b.Text, b.ID})
	case BlockParagraph:
		return json.Marshal(struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{b.Type, b.Text})
	case BlockList:
		items := b.Items
		if items == nil {
			items = []string{}
		}
		if b.Ordered {
			return json.Marshal(struct {
				Type    string   `json:"type"`
				Ordered bool     `json:"ordered"`
				Start   int      `json:"start"`
				Items   []string `json:"items"`
			}{b.Type, true, b.Start, items})
		}
		return json.Marshal(struct {
			Type    string   `json:"type"`
			Ordered bool     `json:"ordered"`
			Items   []string `json:"items"`
		}{b.Type, false, items})
	case BlockTable:
		header, rows := b.Header, b.Rows
		if header == nil {
			header = []string{}
		}
		if rows == nil {
			rows = [][]string{}
		}
		return json.Marshal(struct {
			Type   string     `json:"type"`
			Header []string   `json:"header"`
			Rows   [][]string `json:"rows"`
		}{b.Type, header, rows})
	case BlockPriority:
		return json.Marshal(struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		}{b.Type, b.Value})
	case BlockImage:
		return json.Marshal(struct {
			Type    string `json:"type"`
			Src     string `json:"src"`
			Caption string `json:"caption"`
			Diagram string `json:"diagram"`
		}{b.Type, b.Src, b.Caption, b.Diagram})
	default:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{b.Type})
	}
}

// Document é o documento de requisitos estruturado.
type Document struct {
	Title    string       `json:"title"`
	Project  string       `json:"project"`
	Version  string       `json:"version"`
	Date     string       `json:"date"`
	Authors  []string     `json:"authors"`
	History  []HistoryRow `json:"history"`
	TOC      []TOCEntry   `json:"toc"`
	Blocks   []Block      `json:"blocks"`
	Markdown string       `json:"markdown"`
}

// Figures devolve os blocos de imagem, na ordem em que aparecem.
func (d *Document) Figures() []Block {
	out := []Block{}
	for _, b := range d.Blocks {
		if b.Type == BlockImage {
			out = append(out, b)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Prioridade
// ---------------------------------------------------------------------------

// PriorityLevel converte a prioridade livre dos requisitos e fichas em um dos
// três níveis do documento: "essencial", "importante" ou "desejavel".
func PriorityLevel(p string) string {
	k := normalize(p)
	switch {
	case k == "":
		return PriorityImportant
	case hasAnyPrefix(k, "alta", "alto", "essencial", "critica", "critico", "high", "must"):
		return PriorityEssential
	case hasAnyPrefix(k, "baixa", "baixo", "desejavel", "low", "could", "won"):
		return PriorityDesirable
	default:
		// Média, Importante, Medium, Should e valores desconhecidos.
		return PriorityImportant
	}
}

func hasAnyPrefix(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

var accentReplacer = strings.NewReplacer(
	"á", "a", "à", "a", "â", "a", "ã", "a", "ä", "a",
	"é", "e", "ê", "e", "ë", "e", "í", "i", "î", "i", "ï", "i",
	"ó", "o", "ô", "o", "õ", "o", "ö", "o", "ú", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// normalize reduz um texto a minúsculas sem acentos, para comparações.
func normalize(s string) string {
	return accentReplacer.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// ---------------------------------------------------------------------------
// Construção
// ---------------------------------------------------------------------------

type builder struct {
	doc      *Document
	counters [3]int
	ids      map[string]int
	figures  int
}

func (b *builder) add(bl Block) { b.doc.Blocks = append(b.doc.Blocks, bl) }

// anchor gera a âncora estável de um título: slug em minúsculas, sem o número
// da seção (que muda quando seções vazias são omitidas), único no documento.
func (b *builder) anchor(seed string) string {
	id := model.Slugify(seed)
	if id == "" {
		id = "secao"
	}
	b.ids[id]++
	if n := b.ids[id]; n > 1 {
		id = fmt.Sprintf("%s-%d", id, n)
	}
	return id
}

// heading adiciona um título (e a entrada do sumário). Títulos numerados
// seguem a numeração hierárquica contínua 1, 1.1, 1.1.1.
func (b *builder) heading(level int, text, seed string, numbered bool) {
	number := ""
	if numbered && level >= 1 && level <= 3 {
		b.counters[level-1]++
		for i := level; i < 3; i++ {
			b.counters[i] = 0
		}
		parts := []string{}
		for i := 0; i < level; i++ {
			parts = append(parts, strconv.Itoa(b.counters[i]))
		}
		number = strings.Join(parts, ".")
	}
	id := b.anchor(seed)
	b.add(Block{Type: BlockHeading, Level: level, Number: number, Text: text, ID: id})
	b.doc.TOC = append(b.doc.TOC, TOCEntry{ID: id, Number: number, Title: text, Level: level})
}

// section abre uma seção de nível 1, sempre precedida de quebra de página.
func (b *builder) section(title string) {
	b.add(Block{Type: BlockPageBreak})
	b.heading(1, title, title, true)
}

func (b *builder) para(text string) { b.add(Block{Type: BlockParagraph, Text: text}) }

func (b *builder) list(ordered bool, start int, items []string) {
	bl := Block{Type: BlockList, Ordered: ordered, Items: append([]string{}, items...)}
	if ordered {
		bl.Start = start
	}
	b.add(bl)
}

func (b *builder) table(header []string, rows [][]string) {
	if rows == nil {
		rows = [][]string{}
	}
	b.add(Block{Type: BlockTable, Header: header, Rows: rows})
}

func (b *builder) priority(p string) { b.add(Block{Type: BlockPriority, Value: PriorityLevel(p)}) }

// figure adiciona uma imagem com legenda "Figura N – Nome" (numeração contínua).
func (b *builder) figure(src, name, diagram string) {
	b.figures++
	b.add(Block{Type: BlockImage, Src: src, Diagram: diagram,
		Caption: fmt.Sprintf("Figura %d – %s", b.figures, strings.TrimSpace(name))})
}

var (
	reListLine    = regexp.MustCompile(`^\s*(?:[-*+]|(\d+)[.)])\s+(.*)$`)
	reHeadingLine = regexp.MustCompile(`^#{1,6}\s+(.*)$`)
)

// text converte um texto livre em Markdown simples (parágrafos separados por
// linha em branco, listas e títulos) em blocos. `prefix` (ex.: "**Descrição:** ")
// é colocado no início do primeiro parágrafo.
func (b *builder) text(s, prefix string) {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\r\n", "\n"))
	if s == "" {
		if prefix != "" {
			b.para(strings.TrimSpace(prefix))
		}
		return
	}
	first := true
	for _, chunk := range regexp.MustCompile(`\n\s*\n`).Split(s, -1) {
		lines := []string{}
		for _, l := range strings.Split(chunk, "\n") {
			if strings.TrimSpace(l) != "" {
				lines = append(lines, l)
			}
		}
		if len(lines) == 0 {
			continue
		}
		items, ordered, start := []string{}, false, 1
		allList := true
		for i, l := range lines {
			m := reListLine.FindStringSubmatch(l)
			if m == nil {
				allList = false
				break
			}
			if i == 0 && m[1] != "" {
				ordered = true
				start, _ = strconv.Atoi(m[1])
			}
			items = append(items, strings.TrimSpace(m[2]))
		}
		if allList {
			if first && prefix != "" {
				b.para(strings.TrimSpace(prefix))
			}
			b.list(ordered, start, items)
			first = false
			continue
		}
		joined := []string{}
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if m := reHeadingLine.FindStringSubmatch(l); m != nil {
				l = "**" + strings.TrimSpace(m[1]) + "**"
			}
			joined = append(joined, l)
		}
		text := strings.Join(joined, " ")
		if first {
			text = prefix + text
		}
		b.para(text)
		first = false
	}
}

// refs formata ids como "[RF001], [RF002]".
func refs(ids []string) string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id = strings.TrimSpace(id); id != "" {
			out = append(out, "["+strings.ToUpper(id)+"]")
		}
	}
	return strings.Join(out, ", ")
}

// joinPT junta itens no estilo "a, b e c".
func joinPT(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		return strings.Join(items[:len(items)-1], ", ") + " e " + items[len(items)-1]
	}
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return strings.TrimSpace(s)
}

func containsFold(list []string, v string) bool {
	for _, s := range list {
		if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(v)) {
			return true
		}
	}
	return false
}

// formatDate converte uma data ISO (aaaa-mm-dd) em dd/mm/aaaa; outros
// formatos são mantidos como vieram.
func formatDate(s string) string {
	s = strings.TrimSpace(s)
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("02/01/2006")
	}
	return s
}

// isPlaceholder reconhece textos inteiramente em itálico ("_Descreva aqui…_"),
// que o próprio ArchCode escreve em seções vazias.
func isPlaceholder(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) >= 2 && strings.HasPrefix(s, "_") && strings.HasSuffix(s, "_") && !strings.Contains(s, "\n\n")
}

// ---------------------------------------------------------------------------
// Build
// ---------------------------------------------------------------------------

// Seções de nível 1, na ordem do documento.
const (
	secIntro        = "Introdução"
	secGeneral      = "Descrição geral do sistema"
	secFunctional   = "Requisitos funcionais"
	secNonFunc      = "Requisitos não funcionais"
	secUCDiagrams   = "Diagramas de casos de uso"
	secUCDetail     = "Detalhamento dos casos de uso"
	secModeling     = "Modelagem do sistema"
	secArchitecture = "Arquitetura do sistema"
	secMatrix       = "Matriz de rastreabilidade"
	secReferences   = "Referências"
)

// Build monta o documento estruturado (com o Markdown já preenchido).
func Build(in Input) *Document {
	g := newGen(in)
	doc := g.build()
	doc.Markdown = Markdown(doc, MarkdownOptions{})
	return doc
}

type actorRow struct {
	name  string
	codes []string
}

type rnfGroup struct {
	name string
	reqs []model.Requirement
}

// gen guarda os dados já derivados da entrada.
type gen struct {
	in       Input
	project  string
	meta     model.DocumentMeta
	rf, rnf  []model.Requirement
	groups   []rnfGroup
	actors   []actorRow
	overview string
	ucDiag   []model.UMLDiagram
	modeling map[string][]model.UMLDiagram
	nodes    []model.Node
	adrs     []model.ADR
	eps      []model.Endpoint

	// overviewItems é a lista "Seção N – Nome" da visão geral do documento,
	// calculada depois do plano de seções.
	overviewItems []string
}

func newGen(in Input) *gen {
	g := &gen{in: in, meta: in.Meta, modeling: map[string][]model.UMLDiagram{}}
	if in.Manifest != nil {
		g.project = strings.TrimSpace(in.Manifest.ProjectName)
	}
	if in.Requirements != nil {
		if g.project == "" {
			g.project = strings.TrimSpace(in.Requirements.ProjectName)
		}
		g.rf = in.Requirements.Functional()
		g.rnf = in.Requirements.NonFunctional()
		if !isPlaceholder(in.Requirements.Overview) {
			g.overview = strings.TrimSpace(in.Requirements.Overview)
		}
	}
	if g.project == "" {
		g.project = "Projeto"
	}
	g.meta.Normalize(in.Manifest)

	// RNFs agrupados por categoria, na ordem da primeira aparição.
	index := map[string]int{}
	for _, r := range g.rnf {
		name := strings.TrimSpace(r.Category)
		if name == "" {
			name = "Gerais"
		}
		key := normalize(name)
		i, ok := index[key]
		if !ok {
			i = len(g.groups)
			index[key] = i
			g.groups = append(g.groups, rnfGroup{name: name})
		}
		g.groups[i].reqs = append(g.groups[i].reqs, r)
	}

	for _, d := range in.UMLDiagrams {
		if d.Kind == model.UMLKindUseCase {
			g.ucDiag = append(g.ucDiag, d)
		} else {
			g.modeling[d.Kind] = append(g.modeling[d.Kind], d)
		}
	}
	g.collectActors()

	if in.Diagram != nil {
		for _, n := range in.Diagram.Nodes {
			if n.Type != "group" {
				g.nodes = append(g.nodes, n)
			}
		}
	}
	g.adrs = in.ADRs
	if in.Endpoints != nil {
		g.eps = in.Endpoints.Endpoints
	}
	return g
}

// collectActors reúne os atores das fichas e dos diagramas de casos de uso,
// com os casos de uso de que participam.
func (g *gen) collectActors() {
	index := map[string]int{}
	add := func(name, code string) {
		name = strings.TrimSpace(name)
		if name == "" || name == "-" {
			return
		}
		key := normalize(name)
		i, ok := index[key]
		if !ok {
			i = len(g.actors)
			index[key] = i
			g.actors = append(g.actors, actorRow{name: name})
		}
		if code != "" && !containsFold(g.actors[i].codes, code) {
			g.actors[i].codes = append(g.actors[i].codes, strings.ToUpper(code))
		}
	}
	for _, uc := range g.in.UseCases {
		for _, a := range uc.Actors {
			add(a, uc.Code)
		}
	}
	for _, d := range g.ucDiag {
		for _, e := range d.Elements {
			if e.Type == "actor" {
				add(e.Name, "")
			}
		}
		for _, r := range d.Relations {
			src, dst := d.ElementByID(r.Source), d.ElementByID(r.Target)
			if src == nil || dst == nil {
				continue
			}
			if src.Type == "usecase" {
				src, dst = dst, src
			}
			if src.Type == "actor" && dst.Type == "usecase" && dst.UseCase != "" {
				add(src.Name, dst.UseCase)
			}
		}
	}
	for i := range g.actors {
		sort.Strings(g.actors[i].codes)
	}
}

func (g *gen) hasGeneral() bool {
	return strings.TrimSpace(g.meta.Client) != "" || strings.TrimSpace(g.meta.Users) != "" ||
		len(g.actors) > 0 || g.overview != ""
}

func (g *gen) modelingKinds() []string {
	out := []string{}
	for _, k := range []string{model.UMLKindClass, model.UMLKindSequence, model.UMLKindState} {
		if len(g.modeling[k]) > 0 {
			out = append(out, k)
		}
	}
	return out
}

// plannedSection descreve uma seção de nível 1 presente no documento.
type plannedSection struct {
	title   string
	summary string
	build   func(b *builder)
}

// plan decide quais seções entram (as vazias são omitidas) e em que ordem; a
// numeração é consequência dessa lista.
func (g *gen) plan() []plannedSection {
	out := []plannedSection{{title: secIntro, build: g.intro}}
	if g.hasGeneral() {
		out = append(out, plannedSection{secGeneral,
			"apresenta uma visão geral do sistema, caracterizando qual é o seu escopo e descrevendo seus usuários.", g.general})
	}
	if len(g.rf) > 0 {
		out = append(out, plannedSection{secFunctional, "especifica todos os cenários funcionais do sistema.", g.functional})
	}
	if len(g.rnf) > 0 {
		cats := []string{}
		for _, grp := range g.groups {
			if normalize(grp.name) != "gerais" {
				cats = append(cats, strings.ToLower(grp.name))
			}
		}
		summary := "especifica todos os requisitos não funcionais do sistema."
		if len(cats) > 0 {
			summary = "especifica todos os requisitos não funcionais do sistema, divididos em requisitos de " + joinPT(cats) + "."
		}
		out = append(out, plannedSection{secNonFunc, summary, g.nonFunctional})
	}
	if len(g.ucDiag) > 0 {
		out = append(out, plannedSection{secUCDiagrams,
			"especifica os atores e cenários utilizando a notação de diagramas UML.", g.useCaseDiagrams})
	}
	if len(g.in.UseCases) > 0 {
		out = append(out, plannedSection{secUCDetail,
			"especifica a prioridade, o fluxo principal e os fluxos secundários de cada caso de uso e sua relação com os requisitos funcionais e não funcionais.", g.useCaseDetail})
	}
	if kinds := g.modelingKinds(); len(kinds) > 0 {
		names := []string{}
		for _, k := range kinds {
			names = append(names, modelingNames[k])
		}
		out = append(out, plannedSection{secModeling,
			"apresenta os diagramas UML de " + joinPT(names) + " que descrevem a estrutura e o comportamento do sistema.", g.modelingSection})
	}
	if len(g.nodes) > 0 || len(g.adrs) > 0 || len(g.eps) > 0 {
		parts := []string{}
		if len(g.nodes) > 0 {
			parts = append(parts, "a arquitetura de componentes do sistema")
		}
		if len(g.adrs) > 0 {
			parts = append(parts, "as decisões arquiteturais")
		}
		if len(g.eps) > 0 {
			parts = append(parts, "os contratos de API")
		}
		out = append(out, plannedSection{secArchitecture, "apresenta " + joinPT(parts) + ".", g.architecture})
	}
	if len(g.rf)+len(g.rnf) > 0 {
		out = append(out, plannedSection{secMatrix,
			"relaciona cada requisito aos casos de uso e aos componentes que o realizam.", g.matrix})
	}
	if len(g.meta.References) > 0 {
		out = append(out, plannedSection{secReferences,
			"apresenta referências para outros documentos utilizados para a confecção deste documento.", g.references})
	}
	return out
}

var modelingNames = map[string]string{
	model.UMLKindClass:    "classes",
	model.UMLKindSequence: "sequência",
	model.UMLKindState:    "estados",
}

var modelingTitles = map[string]string{
	model.UMLKindClass:    "Diagramas de classes",
	model.UMLKindSequence: "Diagramas de sequência",
	model.UMLKindState:    "Diagramas de estados",
}

func (g *gen) build() *Document {
	now := g.in.Now
	if now.IsZero() {
		now = time.Now()
	}
	doc := &Document{
		Title:   g.meta.Title,
		Project: g.project,
		Version: g.meta.Version,
		Date:    formatDate(g.meta.Date),
		Authors: append([]string{}, g.meta.Authors...),
		History: []HistoryRow{},
		TOC:     []TOCEntry{},
		Blocks:  []Block{},
	}
	if doc.Date == "" {
		doc.Date = now.Format("02/01/2006")
	}
	for _, h := range g.meta.History {
		doc.History = append(doc.History, HistoryRow{
			Date: formatDate(h.Date), Version: strings.TrimSpace(h.Version),
			Description: strings.TrimSpace(h.Description), Author: strings.TrimSpace(h.Author),
		})
	}
	if len(doc.History) == 0 {
		doc.History = append(doc.History, HistoryRow{
			Date: doc.Date, Version: doc.Version,
			Description: "Versão gerada pelo ArchCode Studio.",
			Author:      strings.Join(doc.Authors, ", "),
		})
	}

	b := &builder{doc: doc, ids: map[string]int{}}
	sections := g.plan()
	g.overviewItems = nil
	for i, s := range sections {
		if i == 0 {
			continue
		}
		g.overviewItems = append(g.overviewItems,
			fmt.Sprintf("**Seção %d – %s**: %s", i+1, s.title, s.summary))
	}
	for _, s := range sections {
		s.build(b)
	}
	return doc
}
