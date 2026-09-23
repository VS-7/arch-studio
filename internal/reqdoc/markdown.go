package reqdoc

import (
	"fmt"
	"regexp"
	"strings"
)

// MarkdownOptions ajusta a saída de Markdown.
type MarkdownOptions struct {
	// ImageSrc reescreve o endereço de cada figura (arquivo relativo ao salvar,
	// data URI no arquivo único…). Nil mantém o src do bloco (rota da API).
	ImageSrc func(b Block) string
}

// PageBreak é a quebra de página usada no Markdown (respeitada na impressão
// e pelo Pandoc com HTML habilitado).
const PageBreak = `<div style="page-break-after: always;"></div>`

// PriorityTable devolve o quadro de prioridade do modelo, com ■ no nível
// marcado e ◻ nos demais.
func PriorityTable(level string) string {
	mark := func(l string) string {
		if l == level {
			return "■"
		}
		return "◻"
	}
	return fmt.Sprintf("| Prioridade: | %s | Essencial | %s | Importante | %s | Desejável |\n",
		mark(PriorityEssential), mark(PriorityImportant), mark(PriorityDesirable)) +
		"| :---- | ----: | :---- | ----: | :---- | ----: | :---- |"
}

// Markdown gera o documento completo: capa, histórico, sumário e corpo.
func Markdown(doc *Document, opts MarkdownOptions) string {
	var b strings.Builder
	para := func(s string) {
		b.WriteString(s)
		b.WriteString("\n\n")
	}

	// Capa.
	para("**" + escapeInline(doc.Title) + "**")
	para(escapeInline(doc.Project))
	para(doc.Date)
	para("Versão " + escapeInline(doc.Version))
	for _, a := range doc.Authors {
		para(escapeInline(a))
	}
	para(PageBreak)

	// Histórico de alterações.
	para("**Histórico de Alterações**")
	rows := make([][]string, 0, len(doc.History))
	for _, h := range doc.History {
		rows = append(rows, []string{h.Date, h.Version, h.Description, h.Author})
	}
	para(table([]string{"Data", "Versão", "Descrição", "Autor"}, rows))

	// Sumário.
	para("**Sumário**")
	if len(doc.TOC) > 0 {
		var toc strings.Builder
		for _, e := range doc.TOC {
			label := e.Title
			if e.Number != "" {
				label = e.Number + " " + e.Title
			}
			link := fmt.Sprintf("[%s](#%s)", escapeBrackets(label), e.ID)
			if e.Level <= 1 {
				link = "**" + link + "**"
			}
			indent := 0
			if e.Level > 1 {
				indent = 2 * (e.Level - 1)
			}
			fmt.Fprintf(&toc, "%s- %s\n", strings.Repeat(" ", indent), link)
		}
		para(strings.TrimRight(toc.String(), "\n"))
	}

	for _, bl := range doc.Blocks {
		para(blockMarkdown(bl, opts))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

func blockMarkdown(bl Block, opts MarkdownOptions) string {
	switch bl.Type {
	case BlockHeading:
		level := bl.Level
		if level < 1 {
			level = 1
		}
		if level > 6 {
			level = 6
		}
		text := escapeInline(bl.Text)
		if bl.Number != "" {
			text = bl.Number + " " + text
		}
		return fmt.Sprintf("%s %s {#%s}", strings.Repeat("#", level), text, bl.ID)
	case BlockParagraph:
		return escapeInline(bl.Text)
	case BlockList:
		lines := make([]string, 0, len(bl.Items))
		for i, it := range bl.Items {
			item := escapeInline(strings.ReplaceAll(it, "\n", " "))
			if bl.Ordered {
				start := bl.Start
				if start < 1 {
					start = 1
				}
				lines = append(lines, fmt.Sprintf("%d. %s", start+i, item))
			} else {
				lines = append(lines, "- "+item)
			}
		}
		return strings.Join(lines, "\n")
	case BlockTable:
		return table(bl.Header, bl.Rows)
	case BlockPriority:
		return PriorityTable(bl.Value)
	case BlockImage:
		src := bl.Src
		if opts.ImageSrc != nil {
			src = opts.ImageSrc(bl)
		}
		caption := escapeInline(bl.Caption)
		return fmt.Sprintf("![%s](%s)\n\n*%s*", escapeBrackets(bl.Caption), src, caption)
	case BlockPageBreak:
		return PageBreak
	default:
		return ""
	}
}

// table monta uma tabela GFM; "|" e quebras de linha nas células são escapados.
func table(header []string, rows [][]string) string {
	cell := func(s string) string {
		s = escapeInline(strings.TrimSpace(s))
		s = strings.ReplaceAll(s, "|", `\|`)
		return strings.ReplaceAll(s, "\n", "<br>")
	}
	var b strings.Builder
	cells := make([]string, len(header))
	seps := make([]string, len(header))
	for i, h := range header {
		cells[i] = cell(h)
		seps[i] = "-----"
	}
	fmt.Fprintf(&b, "| %s |\n| %s |", strings.Join(cells, " | "), strings.Join(seps, " | "))
	for _, r := range rows {
		out := make([]string, len(header))
		for i := range header {
			if i < len(r) {
				out[i] = cell(r[i])
			}
		}
		fmt.Fprintf(&b, "\n| %s |", strings.Join(out, " | "))
	}
	return b.String()
}

// reLink casa links Markdown "[texto](destino)", que devem ser preservados.
var reLink = regexp.MustCompile(`\[[^\[\]]*\]\([^()\s]*\)`)

// escapeInline escapa colchetes que não fazem parte de links, para que
// "[RF001]" apareça literalmente (como no modelo: \[RF001\]).
func escapeInline(s string) string {
	var b strings.Builder
	last := 0
	for _, loc := range reLink.FindAllStringIndex(s, -1) {
		b.WriteString(escapeBrackets(s[last:loc[0]]))
		b.WriteString(s[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(escapeBrackets(s[last:]))
	return b.String()
}

var bracketReplacer = strings.NewReplacer(`[`, `\[`, `]`, `\]`)

func escapeBrackets(s string) string { return bracketReplacer.Replace(s) }
