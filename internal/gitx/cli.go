package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Tempos máximos: operações locais são rápidas; as de rede esperam o servidor.
const (
	localTimeout  = 20 * time.Second
	remoteTimeout = 60 * time.Second
	// lsRemoteTimeout limita a consulta de reservas no remoto.
	lsRemoteTimeout = 15 * time.Second
)

// CLI implementa Git chamando o executável `git` no diretório Dir.
type CLI struct {
	Dir string
	// Bin é o executável ("" = "git" do PATH).
	Bin string
}

// New cria o adaptador para o diretório do projeto.
func New(dir string) *CLI { return &CLI{Dir: dir} }

func (g *CLI) bin() string {
	if g.Bin != "" {
		return g.Bin
	}
	return "git"
}

// run executa o git e devolve a saída padrão. Erros trazem a saída de erro,
// que é o que explica o problema para quem lê.
func (g *CLI) run(timeout time.Duration, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, g.bin(), append([]string{"-C", g.Dir}, args...)...)
	// Nunca pedir senha no terminal: o Studio pode estar rodando sem um.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("git %s: tempo esgotado", args[0])
	}
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return out.String(), fmt.Errorf("git %s: %s", args[0], msg)
	}
	return out.String(), nil
}

func (g *CLI) Available() bool {
	if _, err := exec.LookPath(g.bin()); err != nil {
		return false
	}
	out, err := g.run(localTimeout, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func (g *CLI) Identity() (string, string) {
	name, _ := g.run(localTimeout, "config", "user.name")
	email, _ := g.run(localTimeout, "config", "user.email")
	return strings.TrimSpace(name), strings.TrimSpace(email)
}

func (g *CLI) CurrentBranch() string {
	out, err := g.run(localTimeout, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func (g *CLI) Status() (*Status, error) {
	out, err := g.run(localTimeout, "status", "--porcelain=v2", "--branch", "-z", "--untracked-files=all")
	if err != nil {
		return nil, err
	}
	return parseStatus(out), nil
}

// parseStatus interpreta `git status --porcelain=v2 --branch -z`.
func parseStatus(out string) *Status {
	st := &Status{Changed: []FileChange{}}
	records := strings.Split(out, "\x00")
	for i := 0; i < len(records); i++ {
		rec := records[i]
		switch {
		case rec == "":
			continue
		case strings.HasPrefix(rec, "# branch.head "):
			st.Branch = strings.TrimPrefix(rec, "# branch.head ")
			if st.Branch == "(detached)" {
				st.Branch, st.Detached = "", true
			}
		case strings.HasPrefix(rec, "# branch.upstream "):
			st.Upstream = strings.TrimPrefix(rec, "# branch.upstream ")
		case strings.HasPrefix(rec, "# branch.ab "):
			fields := strings.Fields(strings.TrimPrefix(rec, "# branch.ab "))
			if len(fields) == 2 {
				st.Ahead, _ = strconv.Atoi(strings.TrimPrefix(fields[0], "+"))
				st.Behind, _ = strconv.Atoi(strings.TrimPrefix(fields[1], "-"))
			}
		case strings.HasPrefix(rec, "1 "), strings.HasPrefix(rec, "2 "), strings.HasPrefix(rec, "u "):
			fields := strings.SplitN(rec, " ", 9)
			if rec[0] == '2' {
				fields = strings.SplitN(rec, " ", 10)
				i++ // o caminho original vem no registro seguinte
			} else if rec[0] == 'u' {
				fields = strings.SplitN(rec, " ", 11)
			}
			if len(fields) < 2 {
				continue
			}
			code := fields[1]
			path := fields[len(fields)-1]
			st.Changed = append(st.Changed, FileChange{Path: path, Code: code})
			if rec[0] == 'u' {
				st.Conflicts++
				continue
			}
			if code[0] != '.' {
				st.Staged++
			}
			if len(code) > 1 && code[1] != '.' {
				st.Unstaged++
			}
		case strings.HasPrefix(rec, "? "):
			st.Changed = append(st.Changed, FileChange{Path: rec[2:], Code: "??"})
			st.Untracked++
		}
	}
	return st
}

const logSep, recSep = "\x1f", "\x1e"

func (g *CLI) Log(rev string, max int) ([]Commit, error) {
	args := []string{"log", "--format=%H" + logSep + "%P" + logSep + "%an" + logSep + "%ae" + logSep + "%aI" + logSep + "%s" + logSep + "%b" + recSep}
	if max > 0 {
		args = append(args, "-n", strconv.Itoa(max))
	}
	if rev != "" {
		args = append(args, rev)
	}
	args = append(args, "--")
	out, err := g.run(localTimeout, args...)
	if err != nil {
		return nil, err
	}
	return parseLog(out), nil
}

func parseLog(out string) []Commit {
	commits := []Commit{}
	for _, rec := range strings.Split(out, recSep) {
		rec = strings.TrimLeft(rec, "\n")
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, logSep, 7)
		if len(f) < 7 {
			continue
		}
		commits = append(commits, Commit{
			Hash: f[0], Parents: len(strings.Fields(f[1])), Author: f[2], Email: f[3],
			Date: f[4], Subject: f[5], Body: strings.TrimSpace(f[6]),
		})
	}
	return commits
}

func (g *CLI) RevExists(rev string) bool {
	_, err := g.run(localTimeout, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	return err == nil
}

func (g *CLI) BranchExists(name string) bool {
	_, err := g.run(localTimeout, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

func (g *CLI) CreateBranch(name, from string, switchTo bool) error {
	if switchTo {
		args := []string{"switch", "-c", name}
		if from != "" {
			args = append(args, from)
		}
		_, err := g.run(localTimeout, args...)
		return err
	}
	args := []string{"branch", name}
	if from != "" {
		args = append(args, from)
	}
	_, err := g.run(localTimeout, args...)
	return err
}

func (g *CLI) Switch(name string) error {
	_, err := g.run(localTimeout, "switch", name)
	return err
}

func (g *CLI) RemoteExists(remote string) bool {
	_, err := g.run(localTimeout, "remote", "get-url", remote)
	return err == nil
}

func (g *CLI) RemoteBranches(remote, pattern string) ([]RemoteRef, error) {
	// Consulta curta: sem rede, a reserva segue como "não confirmada".
	out, err := g.run(lsRemoteTimeout, "ls-remote", "--heads", remote)
	if err != nil {
		return nil, err
	}
	refs := []RemoteRef{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "refs/heads/")
		if pattern == "" || MatchGlob(pattern, name) {
			refs = append(refs, RemoteRef{Name: name, Hash: fields[0]})
		}
	}
	return refs, nil
}

func (g *CLI) FetchTip(remote, branch string) (*Commit, error) {
	if _, err := g.run(remoteTimeout, "fetch", "--quiet", remote, "refs/heads/"+branch); err != nil {
		return nil, err
	}
	commits, err := g.Log("FETCH_HEAD", 1)
	if err != nil {
		return nil, err
	}
	if len(commits) == 0 {
		return nil, errors.New("branch sem commits")
	}
	return &commits[0], nil
}

func (g *CLI) Add(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := g.run(localTimeout, append([]string{"add", "--all", "--"}, paths...)...)
	return err
}

func (g *CLI) Commit(message string, allowEmpty bool) (string, error) {
	args := []string{"commit", "--quiet", "-F", "-"}
	if allowEmpty {
		args = append(args, "--allow-empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), localTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, g.bin(), append([]string{"-C", g.Dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	cmd.Stdin = strings.NewReader(message)
	var errb, outb bytes.Buffer
	cmd.Stderr, cmd.Stdout = &errb, &outb
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errb.String() + "\n" + outb.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git commit: %s", msg)
	}
	out, err := g.run(localTimeout, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

func (g *CLI) Push(remote, branch string, setUpstream bool) error {
	args := []string{"push", "--quiet"}
	if setUpstream {
		args = append(args, "-u")
	}
	args = append(args, remote, branch)
	_, err := g.run(remoteTimeout, args...)
	return err
}

func (g *CLI) StagedFiles() ([]string, error) {
	out, err := g.run(localTimeout, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return nil, err
	}
	return splitNul(out), nil
}

func (g *CLI) ChangedSince(base string) ([]string, error) {
	seen := map[string]bool{}
	out := []string{}
	add := func(list []string) {
		for _, f := range list {
			if f != "" && !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	if base != "" && g.RevExists(base) {
		committed, err := g.run(localTimeout, "diff", "--name-only", "-z", base+"...HEAD")
		if err == nil {
			add(splitNul(committed))
		}
	}
	st, err := g.Status()
	if err != nil {
		return out, err
	}
	for _, c := range st.Changed {
		add([]string{c.Path})
	}
	return out, nil
}

func (g *CLI) GitPath(rel string) (string, error) {
	out, err := g.run(localTimeout, "rev-parse", "--git-path", rel)
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(out)
	if !filepath.IsAbs(p) {
		p = filepath.Join(g.Dir, p)
	}
	return p, nil
}

func (g *CLI) ConfigSet(key, value string) error {
	_, err := g.run(localTimeout, "config", key, value)
	return err
}

func splitNul(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, "\x00") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
