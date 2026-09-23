package mermaid

import (
	"fmt"
	"sort"
	"strings"

	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// Exportação dos diagramas UML (.arch/diagrams/<kind>/<id>.mermaid)
// ---------------------------------------------------------------------------
//
// Assim como o diagrama macro, a exportação é determinística. A ida é apenas
// JSON → Mermaid: o JSON continua sendo a fonte da verdade (posições, tamanhos
// e metadados que o Mermaid não representa).

// mermaidReserved são palavras que quebram o parser quando usadas como id.
var mermaidReserved = map[string]bool{
	"end": true, "graph": true, "flowchart": true, "subgraph": true, "class": true,
	"state": true, "note": true, "style": true, "click": true, "default": true,
	"participant": true, "actor": true, "loop": true, "alt": true, "opt": true,
	"par": true, "and": true, "else": true, "rect": true, "critical": true,
	"break": true, "direction": true, "namespace": true, "link": true, "classdef": true,
}

// umlIDs atribui a cada elemento um id Mermaid seguro e legível: o slug do
// nome com `_` (ou do id, para elementos sem nome), único no diagrama.
func umlIDs(d *model.UMLDiagram) map[string]string {
	out := map[string]string{}
	used := map[string]bool{}
	for _, e := range d.Elements {
		base := model.Slugify(e.Name)
		if base == "" {
			base = model.Slugify(strings.TrimPrefix(e.ID, "el-"))
		}
		base = strings.ReplaceAll(base, "-", "_")
		if base == "" {
			base = "el"
		}
		if base[0] >= '0' && base[0] <= '9' {
			base = "n" + base
		}
		if mermaidReserved[base] {
			base += "_"
		}
		id, i := base, 2
		for used[id] {
			id = fmt.Sprintf("%s_%d", base, i)
			i++
		}
		used[id] = true
		out[e.ID] = id
	}
	return out
}

// umlText sanitiza texto livre para rótulos Mermaid.
func umlText(s string) string {
	r := strings.NewReplacer(`"`, `'`, "\r", "", "\n", " ", ";", ",", "#", "")
	return strings.TrimSpace(r.Replace(s))
}

// sortedElements devolve os elementos em ordem estável (y, x, id).
func sortedElements(els []model.UMLElement) []model.UMLElement {
	out := append([]model.UMLElement(nil), els...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Position, out[j].Position
		if a.Y != b.Y {
			return a.Y < b.Y
		}
		if a.X != b.X {
			return a.X < b.X
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// effectiveParent devolve o parent_id apenas quando ele aponta para um
// contêiner válido do tipo informado; caso contrário o elemento é raiz.
func effectiveParent(d *model.UMLDiagram, e model.UMLElement, container string) string {
	if e.ParentID == "" {
		return ""
	}
	if p := d.ElementByID(e.ParentID); p != nil && p.Type == container {
		return p.ID
	}
	return ""
}

// childrenOf agrupa os elementos pelo contêiner efetivo ("" = raiz).
func childrenOf(d *model.UMLDiagram, container string) map[string][]model.UMLElement {
	out := map[string][]model.UMLElement{}
	for _, e := range sortedElements(d.Elements) {
		p := effectiveParent(d, e, container)
		out[p] = append(out[p], e)
	}
	return out
}

// ExportUML converte um diagrama UML na sintaxe Mermaid correspondente ao tipo.
func ExportUML(d *model.UMLDiagram) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%%%% Gerado automaticamente pelo ArchCode Studio a partir de %s\n", umlSourceFile(d))
	fmt.Fprintf(&b, "%%%% %s — %s\n", umlText(d.Name), model.UMLKindLabel[d.Kind])
	switch d.Kind {
	case model.UMLKindClass:
		exportClass(&b, d)
	case model.UMLKindSequence:
		exportSequence(&b, d)
	case model.UMLKindState:
		exportState(&b, d)
	default:
		exportUseCase(&b, d)
	}
	return b.String()
}

func umlSourceFile(d *model.UMLDiagram) string {
	dir := map[string]string{
		model.UMLKindUseCase: "usecase", model.UMLKindClass: "class",
		model.UMLKindSequence: "sequence", model.UMLKindState: "state",
	}[d.Kind]
	return fmt.Sprintf(".arch/diagrams/%s/%s.json", dir, d.ID)
}

// noteText devolve o texto de uma nota (documentation, com fallback no nome).
func noteText(e model.UMLElement) string {
	if t := umlText(e.Documentation); t != "" {
		return t
	}
	return umlText(e.Name)
}

// noteTargets devolve, para cada nota, os elementos ligados por note_link.
func noteTargets(d *model.UMLDiagram) map[string][]string {
	out := map[string][]string{}
	for _, r := range d.Relations {
		if r.Type != "note_link" {
			continue
		}
		src, dst := d.ElementByID(r.Source), d.ElementByID(r.Target)
		if src == nil || dst == nil {
			continue
		}
		if src.Type == "note" && dst.Type != "note" {
			out[src.ID] = append(out[src.ID], dst.ID)
		} else if dst.Type == "note" && src.Type != "note" {
			out[dst.ID] = append(out[dst.ID], src.ID)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Classes → classDiagram
// ---------------------------------------------------------------------------

func mermaidMember(m model.UMLMember, operation bool) string {
	var s strings.Builder
	s.WriteString(m.Visibility)
	s.WriteString(umlText(m.Name))
	if operation {
		s.WriteString("(" + umlText(m.Params) + ")")
		if m.Type != "" {
			s.WriteString(" " + umlText(m.Type))
		}
	} else if m.Type != "" {
		s.WriteString(": " + umlText(m.Type))
	}
	if m.Static {
		s.WriteString("$")
	} else if m.Abstract {
		s.WriteString("*")
	}
	return s.String()
}

func exportClass(b *strings.Builder, d *model.UMLDiagram) {
	b.WriteString("classDiagram\n")
	ids := umlIDs(d)
	children := childrenOf(d, "package")

	writeClass := func(e model.UMLElement, indent string) {
		label := umlText(e.Name)
		head := ids[e.ID]
		if label != "" && label != head {
			head += "[\"" + label + "\"]"
		}
		annotation := ""
		switch {
		case e.Type == "interface":
			annotation = "interface"
		case e.Type == "enum":
			annotation = "enumeration"
		case e.Stereotype != "":
			annotation = umlText(e.Stereotype)
		case e.Abstract:
			annotation = "abstract"
		}
		lines := []string{}
		if annotation != "" {
			lines = append(lines, "<<"+annotation+">>")
		}
		for _, lit := range e.Literals {
			if lit = umlText(lit); lit != "" {
				lines = append(lines, lit)
			}
		}
		for _, m := range e.Attributes {
			lines = append(lines, mermaidMember(m, false))
		}
		for _, m := range e.Operations {
			lines = append(lines, mermaidMember(m, true))
		}
		if len(lines) == 0 {
			fmt.Fprintf(b, "%sclass %s\n", indent, head)
			return
		}
		fmt.Fprintf(b, "%sclass %s {\n", indent, head)
		for _, l := range lines {
			fmt.Fprintf(b, "%s    %s\n", indent, l)
		}
		fmt.Fprintf(b, "%s}\n", indent)
	}
	isClassifier := func(t string) bool { return t == "class" || t == "interface" || t == "enum" }

	// Pacotes viram namespaces (um nível: o Mermaid não aninha namespaces).
	placed := map[string]bool{}
	for _, e := range sortedElements(d.Elements) {
		if e.Type != "package" {
			continue
		}
		members := []model.UMLElement{}
		visited := map[string]bool{}
		var collect func(parent string)
		collect = func(parent string) {
			if visited[parent] {
				return
			}
			visited[parent] = true
			for _, c := range children[parent] {
				if isClassifier(c.Type) && !placed[c.ID] {
					members = append(members, c)
					placed[c.ID] = true
				} else if c.Type == "package" {
					collect(c.ID)
				}
			}
		}
		collect(e.ID)
		if len(members) == 0 {
			continue
		}
		fmt.Fprintf(b, "    namespace %s {\n", ids[e.ID])
		for _, c := range members {
			writeClass(c, "        ")
		}
		b.WriteString("    }\n")
	}
	for _, e := range sortedElements(d.Elements) {
		if isClassifier(e.Type) && !placed[e.ID] {
			writeClass(e, "    ")
		}
	}

	rels := append([]model.UMLRelation(nil), d.Relations...)
	sort.SliceStable(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	for _, r := range rels {
		src, dst := ids[r.Source], ids[r.Target]
		if src == "" || dst == "" || r.Type == "note_link" {
			continue
		}
		// left/right: quem aparece à esquerda da seta no texto Mermaid.
		left, right := src, dst
		leftMul, rightMul := r.SourceMultiplicity, r.TargetMultiplicity
		arrow := "--"
		switch r.Type {
		case "generalization":
			arrow = "<|--"
		case "realization":
			arrow = "<|.."
		case "composition":
			arrow = "*--"
		case "aggregation":
			arrow = "o--"
		case "directed_association":
			arrow = "-->"
		case "dependency":
			arrow = "..>"
		}
		switch r.Type {
		case "generalization", "realization", "composition", "aggregation":
			// Ponta (triângulo/losango) no TARGET, que o Mermaid escreve à esquerda.
			left, right = dst, src
			leftMul, rightMul = r.TargetMultiplicity, r.SourceMultiplicity
		}
		line := "    " + left
		if leftMul != "" {
			line += fmt.Sprintf(" \"%s\"", umlText(leftMul))
		}
		line += " " + arrow
		if rightMul != "" {
			line += fmt.Sprintf(" \"%s\"", umlText(rightMul))
		}
		line += " " + right
		if name := umlText(r.Name); name != "" {
			line += " : " + name
		}
		b.WriteString(line + "\n")
	}

	targets := noteTargets(d)
	for _, e := range sortedElements(d.Elements) {
		if e.Type != "note" {
			continue
		}
		text := noteText(e)
		if len(targets[e.ID]) == 0 {
			fmt.Fprintf(b, "    note \"%s\"\n", text)
			continue
		}
		for _, t := range targets[e.ID] {
			if el := d.ElementByID(t); el != nil && isClassifier(el.Type) {
				fmt.Fprintf(b, "    note for %s \"%s\"\n", ids[t], text)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Sequência → sequenceDiagram
// ---------------------------------------------------------------------------

func exportSequence(b *strings.Builder, d *model.UMLDiagram) {
	b.WriteString("sequenceDiagram\n")
	ids := umlIDs(d)

	lifelines := []model.UMLElement{}
	for _, e := range d.Elements {
		if e.Type == "lifeline" {
			lifelines = append(lifelines, e)
		}
	}
	// Participantes na ordem horizontal do canvas.
	sort.SliceStable(lifelines, func(i, j int) bool {
		if lifelines[i].Position.X != lifelines[j].Position.X {
			return lifelines[i].Position.X < lifelines[j].Position.X
		}
		return lifelines[i].ID < lifelines[j].ID
	})
	for _, l := range lifelines {
		keyword := "participant"
		if l.LifelineKind == "actor" {
			keyword = "actor"
		}
		label := umlText(l.Name)
		if l.Stereotype != "" {
			label = "«" + umlText(l.Stereotype) + "» " + label
		} else if l.LifelineKind != "" && l.LifelineKind != "participant" && l.LifelineKind != "actor" {
			label = "«" + l.LifelineKind + "» " + label
		}
		fmt.Fprintf(b, "    %s %s as %s\n", keyword, ids[l.ID], label)
	}

	// Fragmentos combinados não guardam quais mensagens envolvem (só a ordem
	// vertical é persistida), então viram notas que cobrem todas as lifelines.
	if len(lifelines) > 0 {
		span := ids[lifelines[0].ID]
		if len(lifelines) > 1 {
			span += "," + ids[lifelines[len(lifelines)-1].ID]
		}
		for _, e := range sortedElements(d.Elements) {
			if e.Type != "fragment" {
				continue
			}
			text := e.Operator
			if text == "" {
				text = "alt"
			}
			if e.Guard != "" {
				text += " [" + umlText(e.Guard) + "]"
			}
			if e.Name != "" {
				text += " " + umlText(e.Name)
			}
			fmt.Fprintf(b, "    Note over %s: %s\n", span, text)
		}
	}

	msgs := []model.UMLRelation{}
	for _, r := range d.Relations {
		if r.Type == "message" {
			msgs = append(msgs, r)
		}
	}
	sort.SliceStable(msgs, func(i, j int) bool { return msgs[i].Order < msgs[j].Order })
	for _, m := range msgs {
		src, dst := ids[m.Source], ids[m.Target]
		if src == "" || dst == "" {
			continue
		}
		arrow := "->>"
		text := umlText(m.Name)
		switch m.MessageKind {
		case "async":
			arrow = "-)"
		case "reply":
			arrow = "-->>"
		case "create":
			text = strings.TrimSpace("«create» " + text)
		case "destroy":
			text = strings.TrimSpace("«destroy» " + text)
		}
		if text == "" {
			text = " "
		}
		fmt.Fprintf(b, "    %s%s%s: %s\n", src, arrow, dst, text)
	}

	targets := noteTargets(d)
	for _, e := range sortedElements(d.Elements) {
		if e.Type != "note" {
			continue
		}
		over := []string{}
		for _, t := range targets[e.ID] {
			if el := d.ElementByID(t); el != nil && el.Type == "lifeline" {
				over = append(over, ids[t])
			}
		}
		if len(over) == 0 {
			if len(lifelines) == 0 {
				continue
			}
			over = []string{ids[lifelines[0].ID]}
		}
		if len(over) > 2 {
			over = over[:2]
		}
		fmt.Fprintf(b, "    Note over %s: %s\n", strings.Join(over, ","), noteText(e))
	}
}

// ---------------------------------------------------------------------------
// Estados → stateDiagram-v2
// ---------------------------------------------------------------------------

func exportState(b *strings.Builder, d *model.UMLDiagram) {
	b.WriteString("stateDiagram-v2\n")
	ids := umlIDs(d)
	children := childrenOf(d, "state")

	pseudo := func(t string) bool { return t == "initial" || t == "final" }
	ref := func(e *model.UMLElement) string {
		if pseudo(e.Type) {
			return "[*]"
		}
		return ids[e.ID]
	}

	// Transições são escritas no escopo do pai comum (necessário para que [*]
	// dentro de um estado composto se refira ao escopo correto).
	byScope := map[string][]model.UMLRelation{}
	rels := append([]model.UMLRelation(nil), d.Relations...)
	sort.SliceStable(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	for _, r := range rels {
		if r.Type != "transition" {
			continue
		}
		src, dst := d.ElementByID(r.Source), d.ElementByID(r.Target)
		if src == nil || dst == nil {
			continue
		}
		sp, dp := effectiveParent(d, *src, "state"), effectiveParent(d, *dst, "state")
		scope := ""
		switch {
		case sp == dp:
			scope = sp
		case pseudo(src.Type):
			scope = sp
		case pseudo(dst.Type):
			scope = dp
		}
		byScope[scope] = append(byScope[scope], r)
	}

	var writeScope func(parent, indent string)
	writeScope = func(parent, indent string) {
		for _, e := range children[parent] {
			id := ids[e.ID]
			switch e.Type {
			case "state":
				fmt.Fprintf(b, "%sstate \"%s\" as %s\n", indent, umlText(e.Name), id)
				composite := len(children[e.ID]) > 0 || len(byScope[e.ID]) > 0
				if composite {
					fmt.Fprintf(b, "%sstate %s {\n", indent, id)
					writeScope(e.ID, indent+"    ")
					fmt.Fprintf(b, "%s}\n", indent)
				}
				actions := []string{}
				for _, act := range [][2]string{{"entry", e.Entry}, {"do", e.Do}, {"exit", e.Exit}} {
					if t := umlText(act[1]); t != "" {
						actions = append(actions, act[0]+" / "+t)
					}
				}
				if composite && len(actions) > 0 {
					// Estados compostos não aceitam descrição no Mermaid: vira nota.
					fmt.Fprintf(b, "%snote left of %s : %s\n", indent, id, strings.Join(actions, "<br/>"))
				} else {
					for _, a := range actions {
						fmt.Fprintf(b, "%s%s : %s\n", indent, id, a)
					}
				}
			case "choice", "fork", "join":
				fmt.Fprintf(b, "%sstate %s <<%s>>\n", indent, id, e.Type)
			case "history":
				label := "H"
				if e.Name != "" {
					label = "H " + umlText(e.Name)
				}
				fmt.Fprintf(b, "%sstate \"%s\" as %s\n", indent, label, id)
			}
		}
		for _, r := range byScope[parent] {
			src, dst := d.ElementByID(r.Source), d.ElementByID(r.Target)
			line := fmt.Sprintf("%s%s --> %s", indent, ref(src), ref(dst))
			label := umlText(r.Trigger)
			if g := umlText(r.Guard); g != "" {
				label = strings.TrimSpace(label + " [" + g + "]")
			}
			if ef := umlText(r.Effect); ef != "" {
				label = strings.TrimSpace(label + " / " + ef)
			}
			if label == "" {
				label = umlText(r.Name)
			}
			if label != "" {
				line += " : " + label
			}
			b.WriteString(line + "\n")
		}
	}
	writeScope("", "    ")

	targets := noteTargets(d)
	for _, e := range sortedElements(d.Elements) {
		if e.Type != "note" {
			continue
		}
		for _, t := range targets[e.ID] {
			if el := d.ElementByID(t); el != nil && el.Type == "state" {
				fmt.Fprintf(b, "    note right of %s : %s\n", ids[t], noteText(e))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Casos de uso → flowchart LR
// ---------------------------------------------------------------------------

func exportUseCase(b *strings.Builder, d *model.UMLDiagram) {
	b.WriteString("flowchart LR\n")
	ids := umlIDs(d)
	children := childrenOf(d, "boundary")

	var writeScope func(parent, indent string)
	writeScope = func(parent, indent string) {
		for _, e := range children[parent] {
			id := ids[e.ID]
			name := umlText(e.Name)
			switch e.Type {
			case "actor":
				fmt.Fprintf(b, "%s%s[\"👤 %s\"]\n", indent, id, name)
			case "usecase":
				fmt.Fprintf(b, "%s%s([\"%s\"])\n", indent, id, name)
			case "note":
				fmt.Fprintf(b, "%s%s>\"📝 %s\"]\n", indent, id, noteText(e))
			case "boundary":
				fmt.Fprintf(b, "%ssubgraph %s[\"%s\"]\n", indent, id, name)
				writeScope(e.ID, indent+"    ")
				if len(children[e.ID]) == 0 {
					fmt.Fprintf(b, "%s    %s_empty[\" \"]\n", indent, id)
				}
				fmt.Fprintf(b, "%send\n", indent)
			}
		}
	}
	writeScope("", "    ")

	rels := append([]model.UMLRelation(nil), d.Relations...)
	sort.SliceStable(rels, func(i, j int) bool { return rels[i].ID < rels[j].ID })
	for _, r := range rels {
		src, dst := ids[r.Source], ids[r.Target]
		if src == "" || dst == "" {
			continue
		}
		name := umlText(r.Name)
		switch r.Type {
		case "include":
			fmt.Fprintf(b, "    %s -. «include» .-> %s\n", src, dst)
		case "extend":
			fmt.Fprintf(b, "    %s -. «extend» .-> %s\n", src, dst)
		case "generalization":
			fmt.Fprintf(b, "    %s --> %s\n", src, dst)
		case "dependency":
			if name != "" {
				fmt.Fprintf(b, "    %s -. %s .-> %s\n", src, name, dst)
			} else {
				fmt.Fprintf(b, "    %s -.-> %s\n", src, dst)
			}
		case "note_link":
			fmt.Fprintf(b, "    %s -.- %s\n", src, dst)
		default:
			if name != "" {
				fmt.Fprintf(b, "    %s ---|%s| %s\n", src, name, dst)
			} else {
				fmt.Fprintf(b, "    %s --- %s\n", src, dst)
			}
		}
	}
}
