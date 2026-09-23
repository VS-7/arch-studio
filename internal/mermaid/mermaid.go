// Package mermaid implementa a sincronização bidirecional entre o diagrama
// canônico (.arch/diagrams/macro.json) e sua representação textual Mermaid
// (.arch/diagrams/macro.mermaid), conforme RF007.
//
// Exportação é determinística: o mesmo diagrama sempre gera exatamente o mesmo
// texto, o que mantém os diffs de Git limpos.
package mermaid

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/archcode/studio/internal/layout"
	"github.com/archcode/studio/internal/model"
)

// shapes define a sintaxe Mermaid de cada tipo de nó do ArchCode Studio.
var shapes = map[string][2]string{
	"compute":          {`["`, `"]`},
	"database":         {`[("`, `")]`},
	"cache":            {`(("`, `"))`},
	"storage":          {`[/"`, `"/]`},
	"queue":            {`>"`, `"]`},
	"gateway":          {`{{"`, `"}}`},
	"client":           {`(["`, `"])`},
	"external_service": {`[["`, `"]]`},
}

// shapeType é o mapeamento inverso, usado na importação.
var shapeType = map[string]string{
	`["`: "compute", `[("`: "database", `(("`: "cache", `[/"`: "storage",
	`>"`: "queue", `{{"`: "gateway", `(["`: "client", `[["`: "external_service",
}

var reSafeID = regexp.MustCompile(`[^A-Za-z0-9_]`)

func safeID(id string) string {
	s := reSafeID.ReplaceAllString(id, "_")
	if s == "" {
		return "n"
	}
	if s[0] >= '0' && s[0] <= '9' {
		s = "n" + s
	}
	return s
}

func escapeLabel(s string) string {
	r := strings.NewReplacer(`"`, `'`, "\n", " ", "|", "/")
	return strings.TrimSpace(r.Replace(s))
}

// groupChildren devolve, para cada nó de grupo, os ids contidos geometricamente.
func groupChildren(d *model.Diagram) map[string][]string {
	out := map[string][]string{}
	for _, g := range d.Nodes {
		if g.Type != "group" {
			continue
		}
		w, h := g.Width, g.Height
		if w <= 0 {
			w = 480
		}
		if h <= 0 {
			h = 320
		}
		for _, n := range d.Nodes {
			if n.ID == g.ID || n.Type == "group" {
				continue
			}
			cx := n.Position.X + layout.NodeWidth/2
			cy := n.Position.Y + layout.NodeHeight/2
			if n.ParentID == g.ID ||
				(n.ParentID == "" && cx >= g.Position.X && cx <= g.Position.X+w &&
					cy >= g.Position.Y && cy <= g.Position.Y+h) {
				out[g.ID] = append(out[g.ID], n.ID)
			}
		}
	}
	return out
}

// Export converte o diagrama em um flowchart Mermaid legível.
func Export(d *model.Diagram) string {
	var b strings.Builder
	b.WriteString("%% Gerado automaticamente pelo ArchCode Studio a partir de .arch/diagrams/macro.json\n")
	b.WriteString("%% Não edite manualmente enquanto o servidor estiver rodando: as alterações serão sobrescritas.\n")
	b.WriteString("flowchart LR\n")

	children := groupChildren(d)
	claimed := map[string]bool{}
	for _, ids := range children {
		for _, id := range ids {
			claimed[id] = true
		}
	}

	nodes := append([]model.Node(nil), d.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Position.X == nodes[j].Position.X {
			return nodes[i].Position.Y < nodes[j].Position.Y
		}
		return nodes[i].Position.X < nodes[j].Position.X
	})

	writeNode := func(n model.Node, indent string) {
		shape, ok := shapes[n.Type]
		if !ok {
			shape = shapes["compute"]
		}
		label := escapeLabel(n.Data.Label)
		if n.Data.Technology != "" {
			label += "<br/><small>" + escapeLabel(n.Data.Technology) + "</small>"
		}
		fmt.Fprintf(&b, "%s%s%s%s%s\n", indent, safeID(n.ID), shape[0], label, shape[1])
	}

	// Primeiro os grupos (subgraphs) e seus filhos.
	groupIDs := []string{}
	for _, n := range nodes {
		if n.Type == "group" {
			groupIDs = append(groupIDs, n.ID)
		}
	}
	for _, gid := range groupIDs {
		g := d.NodeByID(gid)
		fmt.Fprintf(&b, "    subgraph %s[\"%s\"]\n", safeID(gid), escapeLabel(g.Data.Label))
		ids := children[gid]
		sort.Strings(ids)
		for _, id := range ids {
			if n := d.NodeByID(id); n != nil {
				writeNode(*n, "        ")
			}
		}
		if len(ids) == 0 {
			fmt.Fprintf(&b, "        %s_empty[\" \"]\n", safeID(gid))
		}
		b.WriteString("    end\n")
	}

	// Depois os nós soltos.
	for _, n := range nodes {
		if n.Type == "group" || claimed[n.ID] {
			continue
		}
		writeNode(n, "    ")
	}

	// Arestas em ordem estável.
	edges := append([]model.Edge(nil), d.Edges...)
	sort.SliceStable(edges, func(i, j int) bool { return edges[i].ID < edges[j].ID })
	for _, e := range edges {
		arrow := "-->"
		if e.Data.Protocol != "" && strings.EqualFold(e.Data.Protocol, "webhook") {
			arrow = "-.->"
		}
		label := e.Label
		if label == "" {
			label = e.Data.Protocol
			if e.Data.Port > 0 {
				label = strings.TrimSpace(label + " :" + strconv.Itoa(e.Data.Port))
			}
		}
		if label == "" {
			fmt.Fprintf(&b, "    %s %s %s\n", safeID(e.Source), arrow, safeID(e.Target))
		} else {
			fmt.Fprintf(&b, "    %s %s|\"%s\"| %s\n", safeID(e.Source), arrow, escapeLabel(label), safeID(e.Target))
		}
	}

	// Estilos por tipo, para leitura direta em READMEs do GitHub.
	b.WriteString("\n")
	classes := map[string]string{
		"compute":          "fill:#1e3a5f,stroke:#3b82f6,color:#e6f0ff",
		"database":         "fill:#3b1e5f,stroke:#a855f7,color:#f5e6ff",
		"cache":            "fill:#5f3b1e,stroke:#f59e0b,color:#fff4e6",
		"queue":            "fill:#1e5f4a,stroke:#10b981,color:#e6fff5",
		"gateway":          "fill:#5f1e2e,stroke:#f43f5e,color:#ffe6ec",
		"storage":          "fill:#1e4f5f,stroke:#06b6d4,color:#e6fbff",
		"client":           "fill:#2b2b3f,stroke:#94a3b8,color:#eef2f7",
		"external_service": "fill:#3f3f2b,stroke:#eab308,color:#fbfbe6",
	}
	used := map[string][]string{}
	for _, n := range d.Nodes {
		if _, ok := classes[n.Type]; ok {
			used[n.Type] = append(used[n.Type], safeID(n.ID))
		}
	}
	types := make([]string, 0, len(used))
	for t := range used {
		types = append(types, t)
	}
	sort.Strings(types)
	for _, t := range types {
		fmt.Fprintf(&b, "    classDef %s %s\n", t, classes[t])
		ids := used[t]
		sort.Strings(ids)
		fmt.Fprintf(&b, "    class %s %s\n", strings.Join(ids, ","), t)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Importação
// ---------------------------------------------------------------------------

var (
	reComment = regexp.MustCompile(`%%.*$`)
	// O corpo do rótulo aceita '>' porque a exportação embute HTML (`<br/>`,
	// `<small>`); os operadores de aresta já foram substituídos por marcadores
	// \x00 antes desta etapa, então não há ambiguidade com setas.
	reNodeDef  = regexp.MustCompile(`([A-Za-z0-9_\-.]+)\s*(\[\[|\[\(|\(\[|\{\{|\[/|\[\\|\[|\(\(|\(|\{|>)("?)([^\]\)\}\x00]*?)("?)(\]\]|\)\]|\]\)|\}\}|/\]|\\\]|\]|\)\)|\)|\})`)
	reArrow    = regexp.MustCompile(`(-\.->|-\.-|==>|===|-->|---|->|--)\s*(?:\|\s*"?([^|"]*)"?\s*\|)?`)
	reSubgraph = regexp.MustCompile(`^\s*subgraph\s+([A-Za-z0-9_\-.]+)\s*(?:\[\s*"?([^\]"]*)"?\s*\])?`)
	reClassDef = regexp.MustCompile(`^\s*(classDef|class|style|linkStyle|direction)\b`)
	reBRTag    = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHTMLTag  = regexp.MustCompile(`<[^>]+>`)
	// Marcador temporário que substitui os operadores de aresta durante o parsing.
	rePlaceholder = regexp.MustCompile("\x00\\d+\x00")
)

// lastToken/firstToken pegam o identificador adjacente ao operador de aresta,
// ignorando texto solto que porventura tenha sobrado na linha.
type rawEdge struct{ src, dst, label, arrow string }

func lastToken(segment string) string {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	return fields[len(fields)-1]
}

func firstToken(segment string) string {
	fields := strings.Fields(segment)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

type parsedNode struct {
	id, label, tech, typ, group string
	order                       int
}

// Import converte um snippet Mermaid (flowchart/graph) em um diagrama.
// Posições são calculadas por AutoLayout; se `base` for informado, nós já
// existentes com o mesmo rótulo preservam suas coordenadas originais (RF017).
// umlHeaders mapeia cabeçalhos Mermaid que não descrevem arquitetura para a
// orientação dada ao usuário. Importá-los como flowchart gerava componentes
// sem sentido ("<|", "motivo: String", nomes vazios).
var umlHeaders = map[string]string{
	"classdiagram":       "diagrama de classes",
	"sequencediagram":    "diagrama de sequência",
	"statediagram":       "diagrama de estados",
	"erdiagram":          "diagrama entidade-relacionamento",
	"journey":            "jornada de usuário",
	"gantt":              "gráfico de Gantt",
	"pie":                "gráfico de pizza",
	"mindmap":            "mapa mental",
	"timeline":           "linha do tempo",
	"gitgraph":           "grafo de git",
	"quadrantchart":      "gráfico de quadrantes",
	"requirementdiagram": "diagrama de requisitos",
}

// checkImportKind recusa snippets Mermaid que não são flowchart/graph/C4.
func checkImportKind(lines []string) error {
	for _, l := range lines {
		t := strings.ToLower(strings.TrimSpace(l))
		if t == "" || strings.HasPrefix(t, "%%") || t == "---" || strings.HasPrefix(t, "title:") || strings.HasPrefix(t, "config:") {
			continue
		}
		head := strings.Fields(t)[0]
		for prefix, name := range umlHeaders {
			if strings.HasPrefix(head, prefix) {
				hint := ""
				switch prefix {
				case "classdiagram", "sequencediagram", "statediagram":
					hint = " Crie um diagrama UML pelo Model Explorer (ou pelas ferramentas MCP create_uml_diagram/add_uml_element) para modelá-lo."
				}
				return fmt.Errorf("o snippet é um %s, não uma arquitetura: importe apenas flowchart, graph ou C4.%s", name, hint)
			}
		}
		return nil
	}
	return errors.New("snippet Mermaid vazio")
}

func Import(src string, base *model.Diagram) (*model.Diagram, error) {
	lines := strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n")
	// Remove fences de markdown.
	cleaned := make([]string, 0, len(lines))
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") {
			continue
		}
		cleaned = append(cleaned, reComment.ReplaceAllString(l, ""))
	}

	if err := checkImportKind(cleaned); err != nil {
		return nil, err
	}

	nodes := map[string]*parsedNode{}
	order := 0
	groupStack := []string{}
	groups := map[string]string{} // id -> título
	edges := []rawEdge{}

	ensure := func(id string) *parsedNode {
		if n, ok := nodes[id]; ok {
			return n
		}
		g := ""
		if len(groupStack) > 0 {
			g = groupStack[len(groupStack)-1]
		}
		n := &parsedNode{id: id, label: id, typ: "compute", group: g, order: order}
		order++
		nodes[id] = n
		return n
	}

	for _, raw := range cleaned {
		line := strings.TrimSpace(raw)
		if line == "" || reClassDef.MatchString(line) {
			continue
		}
		low := strings.ToLower(line)
		if strings.HasPrefix(low, "flowchart") || strings.HasPrefix(low, "graph") {
			continue
		}
		if m := reSubgraph.FindStringSubmatch(line); m != nil {
			id := m[1]
			title := strings.TrimSpace(m[2])
			if title == "" {
				title = id
			}
			groups[id] = title
			groupStack = append(groupStack, id)
			continue
		}
		if low == "end" {
			if len(groupStack) > 0 {
				groupStack = groupStack[:len(groupStack)-1]
			}
			continue
		}

		// Os operadores de aresta são extraídos ANTES das definições de nó.
		// O caractere '>' abre a forma de fila (`id>"texto"]`) e também fecha as
		// setas (`-->`); sem essa proteção, o regex de nó casaria com o próprio
		// operador e engoliria o restante da linha.
		arrows := []rawEdge{}
		protected := reArrow.ReplaceAllStringFunc(line, func(match string) string {
			sub := reArrow.FindStringSubmatch(match)
			arrows = append(arrows, rawEdge{arrow: sub[1], label: strings.TrimSpace(sub[2])})
			return fmt.Sprintf("\x00%d\x00", len(arrows)-1)
		})

		// Extrai as definições de nó e reduz a linha a "A \x00i\x00 B".
		simplified := protected
		for _, m := range reNodeDef.FindAllStringSubmatch(protected, -1) {
			id, open, body := m[1], m[2]+m[3], m[4]
			n := ensure(id)
			label := reBRTag.ReplaceAllString(body, "\n")
			label = reHTMLTag.ReplaceAllString(label, "")
			parts := strings.SplitN(label, "\n", 2)
			n.label = strings.TrimSpace(strings.Trim(parts[0], `"`))
			if len(parts) > 1 {
				n.tech = strings.TrimSpace(parts[1])
			}
			if t, ok := shapeType[open]; ok {
				n.typ = t
			} else if strings.HasPrefix(open, "((") {
				n.typ = "cache"
			} else if strings.HasPrefix(open, "(") {
				n.typ = "client"
			} else if strings.HasPrefix(open, "{") {
				n.typ = "gateway"
			}
			simplified = strings.Replace(simplified, m[0], id, 1)
		}

		if len(arrows) == 0 {
			continue
		}

		// Suporta cadeias: A --> B -->|rótulo| C
		segments := rePlaceholder.Split(simplified, -1)
		for i, a := range arrows {
			if i+1 >= len(segments) {
				break
			}
			src := lastToken(segments[i])
			dst := firstToken(segments[i+1])
			if src == "" || dst == "" {
				continue
			}
			ensure(src)
			ensure(dst)
			edges = append(edges, rawEdge{src: src, dst: dst, label: a.label, arrow: a.arrow})
		}
	}

	if len(nodes) == 0 {
		return nil, fmt.Errorf("nenhum nó encontrado no snippet Mermaid")
	}

	out := model.NewDiagram()
	if base != nil {
		out.Viewport = base.Viewport
	}

	ordered := make([]*parsedNode, 0, len(nodes))
	for _, n := range nodes {
		ordered = append(ordered, n)
	}
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].order < ordered[j].order })

	idMap := map[string]string{}
	for _, pn := range ordered {
		newID := "node-" + model.Slugify(pn.label)
		if newID == "node-" {
			newID = "node-" + model.Slugify(pn.id)
		}
		// Garante unicidade.
		candidate, i := newID, 2
		for out.NodeByID(candidate) != nil {
			candidate = fmt.Sprintf("%s-%d", newID, i)
			i++
		}
		idMap[pn.id] = candidate

		tier := layout.TypeTier[pn.typ]
		node := model.Node{
			ID:       candidate,
			Type:     pn.typ,
			Position: model.Position{},
			Data: model.NodeData{
				Label:      pn.label,
				Technology: pn.tech,
				Tier:       tier,
				Status:     model.StatusPending,
				Pricing:    &model.NodePricing{Complexity: "medium"},
			},
		}
		if pn.group != "" {
			node.ParentID = "group-" + model.Slugify(groups[pn.group])
		}
		out.Nodes = append(out.Nodes, node)
	}

	for _, e := range edges {
		src, sok := idMap[e.src]
		dst, dok := idMap[e.dst]
		if !sok || !dok || src == dst {
			continue
		}
		if out.HasEdge(src, dst) {
			continue
		}
		edge := model.Edge{
			ID:       fmt.Sprintf("edge-%s-to-%s", strings.TrimPrefix(src, "node-"), strings.TrimPrefix(dst, "node-")),
			Source:   src,
			Target:   dst,
			Type:     "smoothstep",
			Animated: strings.HasPrefix(e.arrow, "-."),
			Data:     model.EdgeData{Description: e.label},
		}
		if p, port := inferProtocol(e.label); p != "" {
			edge.Data.Protocol = p
			edge.Data.Port = port
			edge.Data.Description = e.label
		} else if e.label != "" {
			edge.Label = e.label
		}
		out.Edges = append(out.Edges, edge)
	}

	layout.AutoLayout(out)

	// Preserva coordenadas de nós homônimos já existentes (RF017).
	if base != nil {
		for i := range out.Nodes {
			if old := base.ResolveNode(out.Nodes[i].Data.Label); old != nil {
				out.Nodes[i].Position = old.Position
				out.Nodes[i].ID = old.ID
			}
		}
	}

	// Materializa subgraphs como nós de grupo envolvendo seus filhos.
	for gid, title := range groups {
		groupID := "group-" + model.Slugify(title)
		minX, minY, maxX, maxY := 0.0, 0.0, 0.0, 0.0
		found := false
		for _, n := range out.Nodes {
			if n.ParentID != groupID {
				continue
			}
			if !found {
				minX, minY = n.Position.X, n.Position.Y
				maxX, maxY = n.Position.X+layout.NodeWidth, n.Position.Y+layout.NodeHeight
				found = true
				continue
			}
			minX = min(minX, n.Position.X)
			minY = min(minY, n.Position.Y)
			maxX = max(maxX, n.Position.X+layout.NodeWidth)
			maxY = max(maxY, n.Position.Y+layout.NodeHeight)
		}
		if !found {
			continue
		}
		_ = gid
		out.Nodes = append(out.Nodes, model.Node{
			ID:       groupID,
			Type:     "group",
			Position: model.Position{X: minX - 40, Y: minY - 60},
			Width:    maxX - minX + 80,
			Height:   maxY - minY + 100,
			Data:     model.NodeData{Label: title, Tier: "backend"},
		})
	}

	return out, nil
}

var protocolHints = map[string]string{
	"rest": "REST", "http": "REST", "https": "REST", "api": "REST",
	"grpc": "gRPC", "graphql": "GraphQL", "ws": "WebSocket", "websocket": "WebSocket",
	"sql": "SQL", "postgres": "SQL", "mysql": "SQL", "jdbc": "SQL",
	"amqp": "AMQP", "rabbitmq": "AMQP", "kafka": "Kafka", "sqs": "AMQP",
	"redis": "Redis", "s3": "S3", "webhook": "Webhook", "tcp": "TCP",
}

func inferProtocol(label string) (string, int) {
	low := strings.ToLower(label)
	port := 0
	if m := regexp.MustCompile(`:(\d{2,5})`).FindStringSubmatch(low); m != nil {
		port, _ = strconv.Atoi(m[1])
	}
	for hint, proto := range protocolHints {
		if strings.Contains(low, hint) {
			return proto, port
		}
	}
	return "", port
}
