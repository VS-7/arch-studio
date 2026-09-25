package conventions

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/model"
)

// ---------------------------------------------------------------------------
// CHANGELOG por sprint (RF042)
// ---------------------------------------------------------------------------

// rePRSuffix é o " (#12)" que o GitHub acrescenta ao título num squash merge.
var rePRSuffix = regexp.MustCompile(`\s+\(#(\d+)\)$`)

// StripPRSuffix separa o número do PR do assunto de um squash merge.
func StripPRSuffix(subject string) (string, int) {
	m := rePRSuffix.FindStringSubmatchIndex(subject)
	if m == nil {
		return subject, 0
	}
	n, _ := strconv.Atoi(subject[m[2]:m[3]])
	return subject[:m[0]], n
}

// ChangelogBlock gera as seções do CHANGELOG, uma por sprint (a mais recente
// primeiro), a partir dos commits da branch principal. Commits fora da
// convenção (anteriores à adoção) ficam de fora.
func ChangelogBlock(c *model.Conventions, commits []gitx.Commit, sprints []model.Sprint) string {
	type entry struct {
		text string
	}
	bySprint := map[int][]entry{}
	offSprint := []entry{}
	for _, cm := range commits {
		subject, pr := StripPRSuffix(cm.Subject)
		p, ok := ParseSubject(c, subject)
		if !ok {
			continue
		}
		// Planejamento e reservas não são entregas.
		if _, isSprint := SprintRef(p.ID); isSprint || p.ID == PlanRef || IsReservation(c, subject) {
			continue
		}
		text := p.Summary
		if text == "" {
			text = subject
		}
		if p.ID != "" {
			text += " [" + p.ID + "]"
		}
		if pr > 0 {
			text += fmt.Sprintf(" (#%d)", pr)
		}
		if p.Sprint > 0 {
			bySprint[p.Sprint] = append(bySprint[p.Sprint], entry{text})
		} else {
			if p.Kind != "" {
				text = p.Kind + ": " + text
			}
			offSprint = append(offSprint, entry{text})
		}
	}
	numbers := []int{}
	for n := range bySprint {
		numbers = append(numbers, n)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(numbers)))

	var b strings.Builder
	b.WriteString("<!-- Gerado pelo ArchCode Studio a partir do histórico da branch principal (`archcode-studio changelog`). -->\n")
	if len(numbers) == 0 && len(offSprint) == 0 {
		b.WriteString("\nNenhum commit no padrão da convenção ainda.\n")
		return b.String()
	}
	for _, n := range numbers {
		heading := model.SprintName(n)
		for _, sp := range sprints {
			if sp.Number == n {
				if sp.Goal != "" {
					heading += " — " + sp.Goal
				}
				if sp.Start != "" && sp.End != "" {
					heading += fmt.Sprintf(" (%s a %s)", sp.Start, sp.End)
				}
			}
		}
		fmt.Fprintf(&b, "\n## %s\n\n", heading)
		for _, e := range bySprint[n] {
			fmt.Fprintf(&b, "- %s\n", e.text)
		}
	}
	if len(offSprint) > 0 {
		b.WriteString("\n## Fora de sprint\n\n")
		for _, e := range offSprint {
			fmt.Fprintf(&b, "- %s\n", e.text)
		}
	}
	return b.String()
}

// Changelog aplica o bloco num CHANGELOG.md existente (ou cria um).
func Changelog(existing, block string) string {
	if strings.TrimSpace(existing) == "" {
		existing = "# Changelog\n"
	}
	return model.ReplaceManagedBlock(existing, "changelog", block, true)
}

// ---------------------------------------------------------------------------
// Hooks do Git
// ---------------------------------------------------------------------------

// HookMarker identifica, dentro do script, um hook gerado pelo Studio.
const HookMarker = "archcode-studio: hook gerado pelo ArchCode Studio"

// HookNames são os hooks que o Studio instala.
var HookNames = []string{"commit-msg", "pre-push"}

// HookScript devolve o script de um hook. fallback é o caminho absoluto do
// executável que instalou o hook, usado se o archcode-studio não estiver no
// PATH. Sem nenhum dos dois, o hook não bloqueia nada.
func HookScript(name, fallback string) string {
	desc := map[string]string{
		"commit-msg": "Valida a mensagem de commit contra .arch/conventions.yaml.",
		"pre-push":   "Valida o nome das branches enviadas contra .arch/conventions.yaml.",
	}[name]
	args := `"$@"`
	if name == "commit-msg" {
		args = `"$1"`
	}
	return fmt.Sprintf(`#!/bin/sh
# %s (archcode-studio hooks install).
# %s
# Para pular uma vez: git commit --no-verify / git push --no-verify.
BIN=archcode-studio
if ! command -v "$BIN" >/dev/null 2>&1; then
  BIN=%s
  [ -x "$BIN" ] || exit 0
fi
exec "$BIN" hook %s %s
`, HookMarker, desc, shellQuote(fallback), name, args)
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// GitAttributes é o trecho do .gitattributes que marca os arquivos derivados:
// o merge driver do Studio resolve conflitos neles mantendo a versão local,
// que é regenerada a partir do backlog (RF058).
const GitAttributes = `.arch/tasks.json merge=archcode linguist-generated=true
docs/ai-prd.md merge=archcode linguist-generated=true
.arch/memory/MEMORY.md merge=archcode linguist-generated=true`

// ---------------------------------------------------------------------------
// Template de pull request
// ---------------------------------------------------------------------------

// PRTemplate devolve o .github/pull_request_template.md da convenção.
func PRTemplate(c *model.Conventions, checks []SkillCheck) string {
	example := PRTitle(c, PRInput{Items: []*model.WorkItem{{ID: "TASK-API-01", Type: model.ItemTask, Title: "Emissão de JWT", Sprint: 1}}})
	var b strings.Builder
	fmt.Fprintf(&b, "<!-- Título no padrão: %s -->\n<!-- Gerado pelo ArchCode Studio (archcode-studio conventions apply). -->\n\n", example)
	b.WriteString("## Sprint e meta\n\n<!-- Sprint NN — meta da sprint -->\n\n")
	b.WriteString("Fecha: <!-- ids dos itens, ex.: TASK-API-01 -->\n\n")
	b.WriteString("### Critérios de aceite\n\n- [ ] \n\n")
	b.WriteString("### Checks das skills\n\n")
	if len(checks) == 0 {
		b.WriteString("- [ ] Testes cobrindo cada critério de aceite\n- [ ] Nenhum segredo no diff\n")
	}
	for _, ch := range checks {
		fmt.Fprintf(&b, "- [ ] %s · %s\n", ch.Skill, ch.Check)
	}
	b.WriteString("\n### Como testar\n\n<!-- comandos e passos -->\n\n")
	b.WriteString("### Impacto na arquitetura\n\n- [ ] `archcode-studio validate` sem erros\n- Componentes: \n")
	fmt.Fprintf(&b, "\n---\nMerge: %s.\n", mergeLabel(c.PullRequest.MergeStrategy))
	return b.String()
}

// CIWorkflow devolve um workflow do GitHub Actions que valida commits,
// branch e título do PR com `archcode-studio git lint`.
func CIWorkflow() string {
	return `# Gerado pelo ArchCode Studio (archcode-studio conventions ci).
# Valida commits, nome da branch e título do PR contra .arch/conventions.yaml.
name: Convenções

on:
  pull_request:
    types: [opened, edited, synchronize, reopened]

jobs:
  git-lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - name: Instalar o ArchCode Studio
        run: |
          curl -fsSL https://raw.githubusercontent.com/VS-7/arch-studio/main/scripts/install.sh | sh
          echo "$HOME/.local/bin" >> "$GITHUB_PATH"
      - name: Validar convenções
        env:
          PR_TITLE: ${{ github.event.pull_request.title }}
        run: |
          archcode-studio git lint \
            --range "origin/${{ github.base_ref }}..HEAD" \
            --branch "${{ github.head_ref }}" \
            --pr-title "$PR_TITLE"
`
}
