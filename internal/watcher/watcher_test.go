package watcher

import (
	"testing"

	"github.com/archcode/studio/internal/hub"
)

func TestClassifyDiagramasUML(t *testing.T) {
	cases := map[string]string{
		".arch/diagrams/class/modelo.json":         hub.EventUML,
		".arch/diagrams/usecase/casos-de-uso.json": hub.EventUML,
		".arch/diagrams/sequence/login.json":       hub.EventUML,
		".arch/diagrams/state/sessao.json":         hub.EventUML,
		".arch/diagrams/sequence/legado.mermaid":   hub.EventDiagram,
		".arch/diagrams/macro.json":                hub.EventDiagram,
		"docs/requisitos.md":                       hub.EventDocs,
		".arch/document.yaml":                      hub.EventDocs,
	}
	for rel, want := range cases {
		if got := classify(rel); got != want {
			t.Errorf("classify(%q) = %q, want %q", rel, got, want)
		}
	}
}
