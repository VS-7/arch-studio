package model

import "strings"

// ---------------------------------------------------------------------------
// Blocos gerenciados em arquivos de terceiros
// ---------------------------------------------------------------------------
//
// O Studio escreve em arquivos que também são editados à mão (AGENTS.md,
// CHANGELOG.md, CLAUDE.md). Só o trecho entre os marcadores é dele; o resto
// do arquivo fica intacto.

func managedMarkers(name string) (string, string) {
	return "<!-- archcode:" + name + ":start -->", "<!-- archcode:" + name + ":end -->"
}

// ReplaceManagedBlock troca (ou acrescenta no fim) o bloco gerenciado.
// Com atTop, um bloco novo entra no começo do arquivo, depois do primeiro
// título "# ", se houver.
func ReplaceManagedBlock(content, name, block string, atTop bool) string {
	start, end := managedMarkers(name)
	wrapped := start + "\n" + strings.TrimSpace(block) + "\n" + end
	i := strings.Index(content, start)
	j := strings.Index(content, end)
	if i >= 0 && j > i {
		return content[:i] + wrapped + content[j+len(end):]
	}
	if strings.TrimSpace(content) == "" {
		return wrapped + "\n"
	}
	if atTop {
		if strings.HasPrefix(content, "# ") {
			if nl := strings.Index(content, "\n"); nl >= 0 {
				return content[:nl+1] + "\n" + wrapped + "\n" + content[nl+1:]
			}
		}
		return wrapped + "\n\n" + content
	}
	return strings.TrimRight(content, "\n") + "\n\n" + wrapped + "\n"
}

// RemoveManagedBlock tira o bloco gerenciado (se existir).
func RemoveManagedBlock(content, name string) string {
	start, end := managedMarkers(name)
	i := strings.Index(content, start)
	j := strings.Index(content, end)
	if i < 0 || j < i {
		return content
	}
	out := content[:i] + content[j+len(end):]
	for strings.Contains(out, "\n\n\n") {
		out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	}
	return out
}

// HasManagedBlock informa se o conteúdo tem o bloco gerenciado.
func HasManagedBlock(content, name string) bool {
	start, _ := managedMarkers(name)
	return strings.Contains(content, start)
}

// ResolveConflicts resolve os blocos de conflito do Git (<<<<<<< / ======= /
// >>>>>>>) ficando com um dos lados: "ours" (o local, antes do =======) ou
// "theirs" (o que veio no merge). ok = false se não havia conflito.
func ResolveConflicts(content, prefer string) (string, bool) {
	if !HasConflictMarkers(content) {
		return content, false
	}
	var out []string
	state := 0 // 0 = fora; 1 = lado local; 2 = lado remoto; 3 = base (diff3)
	for _, line := range strings.Split(content, "\n") {
		switch {
		case strings.HasPrefix(line, "<<<<<<< "), line == "<<<<<<<":
			state = 1
			continue
		case strings.HasPrefix(line, "||||||| ") && state == 1:
			state = 3
			continue
		case line == "=======" && (state == 1 || state == 3):
			state = 2
			continue
		case (strings.HasPrefix(line, ">>>>>>> ") || line == ">>>>>>>") && state == 2:
			state = 0
			continue
		}
		switch {
		case state == 0,
			state == 1 && prefer != "theirs",
			state == 2 && prefer == "theirs":
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n"), true
}
