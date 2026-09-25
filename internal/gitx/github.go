package gitx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// PullRequest é um PR aberto no servidor.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Head   string `json:"headRefName"`
	URL    string `json:"url"`
	Draft  bool   `json:"isDraft"`
}

// Forge é o servidor de código (GitHub via `gh`). É opcional: sem ele, o
// Studio gera título e corpo do PR para a pessoa abrir à mão.
type Forge interface {
	Available() bool
	// CreatePR abre o PR e devolve a URL.
	CreatePR(title, body, base, head string, draft bool) (string, error)
	// OpenPRs lista os PRs abertos.
	OpenPRs() ([]PullRequest, error)
}

// GitHubCLI implementa Forge com o `gh` autenticado de quem usa.
type GitHubCLI struct{ Dir string }

// NewGitHub cria o adaptador do `gh` para o diretório do projeto.
func NewGitHub(dir string) *GitHubCLI { return &GitHubCLI{Dir: dir} }

func (g *GitHubCLI) Available() bool {
	if _, err := exec.LookPath("gh"); err != nil {
		return false
	}
	cmd := exec.Command("gh", "auth", "status")
	cmd.Dir = g.Dir
	return cmd.Run() == nil
}

func (g *GitHubCLI) run(stdin string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = g.Dir
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "NO_COLOR=1")
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gh %s: %s", args[0], msg)
	}
	return strings.TrimSpace(out.String()), nil
}

func (g *GitHubCLI) CreatePR(title, body, base, head string, draft bool) (string, error) {
	args := []string{"pr", "create", "--title", title, "--body-file", "-", "--base", base, "--head", head}
	if draft {
		args = append(args, "--draft")
	}
	out, err := g.run(body, args...)
	if err != nil {
		return "", err
	}
	lines := strings.Split(out, "\n")
	return strings.TrimSpace(lines[len(lines)-1]), nil
}

func (g *GitHubCLI) OpenPRs() ([]PullRequest, error) {
	out, err := g.run("", "pr", "list", "--state", "open", "--limit", "100", "--json", "number,title,headRefName,url,isDraft")
	if err != nil {
		return nil, err
	}
	prs := []PullRequest{}
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("gh pr list: resposta inesperada: %w", err)
	}
	return prs, nil
}

// FakeForge é um servidor em memória para testes.
type FakeForge struct {
	PRs     []PullRequest
	Created []PullRequest
	Off     bool
}

func (f *FakeForge) Available() bool { return !f.Off }

func (f *FakeForge) CreatePR(title, body, base, head string, draft bool) (string, error) {
	pr := PullRequest{Number: len(f.PRs) + 1, Title: title, Head: head, Draft: draft,
		URL: fmt.Sprintf("https://example.test/pr/%d", len(f.PRs)+1)}
	f.PRs = append(f.PRs, pr)
	f.Created = append(f.Created, pr)
	return pr.URL, nil
}

func (f *FakeForge) OpenPRs() ([]PullRequest, error) { return f.PRs, nil }

var _ Forge = (*GitHubCLI)(nil)
var _ Forge = (*FakeForge)(nil)
