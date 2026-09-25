package studio

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/archcode/studio/internal/conventions"
)

// RunHook executa um hook do Git gerado por `archcode-studio hooks install`
// (commit-msg ou pre-push) e devolve o código de saída: 0 deixa o Git seguir,
// 1 bloqueia. É compartilhado pelo CLI e pelo app desktop, porque o hook
// chama o executável que o instalou quando o CLI não está no PATH.
//
// Fora de um projeto ArchCode Studio (ou sem conventions.yaml), o hook não
// bloqueia nada: o repositório ainda não adotou a convenção.
func RunHook(name string, args []string, stdin io.Reader, stdout, stderr io.Writer, version string) int {
	s, err := Open(Config{Version: version, RequireProject: true}, nil)
	if err != nil || !s.App.HasConventions() {
		return 0
	}
	defer s.Close()

	var issues []conventions.Issue
	switch name {
	case "commit-msg":
		if len(args) < 1 {
			fmt.Fprintln(stderr, "archcode-studio hook commit-msg: informe o arquivo da mensagem")
			return 1
		}
		data, err := os.ReadFile(args[0])
		if err != nil {
			fmt.Fprintf(stderr, "archcode-studio: %v\n", err)
			return 1
		}
		issues, err = s.App.CheckCommitMessage(string(data))
		if err != nil {
			fmt.Fprintf(stderr, "archcode-studio: %v\n", err)
			return 0
		}
	case "pre-push":
		lines := []string{}
		sc := bufio.NewScanner(stdin)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				lines = append(lines, line)
			}
		}
		issues, err = s.App.CheckPush(lines)
		if err != nil {
			fmt.Fprintf(stderr, "archcode-studio: %v\n", err)
			return 0
		}
	default:
		fmt.Fprintf(stderr, "archcode-studio: hook desconhecido %q (use commit-msg ou pre-push)\n", name)
		return 1
	}

	errors := 0
	for _, is := range issues {
		mark := "aviso"
		if is.Level == conventions.LevelError {
			mark = "erro"
			errors++
		}
		ref := ""
		if is.Ref != "" {
			ref = is.Ref + " "
		}
		fmt.Fprintf(stderr, "archcode-studio [%s] %s%q: %s\n", mark, ref, is.Subject, is.Message)
	}
	if errors > 0 {
		fmt.Fprintf(stderr, "\nConvenção em .arch/conventions.yaml. Para gerar a mensagem certa: archcode-studio commit.\n"+
			"Para pular esta verificação uma vez: git %s --no-verify.\n", map[string]string{"commit-msg": "commit", "pre-push": "push"}[name])
		return 1
	}
	return 0
}
