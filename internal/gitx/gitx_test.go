package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// newRepo cria um repositório real num diretório temporário, com um remoto
// "origin" (bare) — os testes são pulados sem o git instalado.
func newRepo(t *testing.T) (*CLI, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não instalado")
	}
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	work := filepath.Join(base, "work")
	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	run(base, "init", "--quiet", "--bare", "-b", "main", remote)
	run(work, "init", "--quiet", "-b", "main")
	run(work, "config", "user.name", "Ana Souza")
	run(work, "config", "user.email", "ana@exemplo.com")
	run(work, "config", "commit.gpgsign", "false")
	run(work, "remote", "add", "origin", remote)
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte("# x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "add", ".")
	run(work, "commit", "--quiet", "-m", "Commit inicial")
	run(work, "push", "--quiet", "-u", "origin", "main")
	return New(work), work
}

func TestCLIBasics(t *testing.T) {
	g, dir := newRepo(t)
	if !g.Available() {
		t.Fatal("repositório deveria estar disponível")
	}
	if Identity(g) != "Ana Souza <ana@exemplo.com>" {
		t.Fatalf("identidade = %q", Identity(g))
	}
	if g.CurrentBranch() != "main" {
		t.Fatalf("branch = %q", g.CurrentBranch())
	}

	// Árvore suja: um arquivo novo, um modificado e um preparado.
	_ = os.WriteFile(filepath.Join(dir, "novo.txt"), []byte("a"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("# y\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "staged.go"), []byte("package x\n"), 0o644)
	cmd := exec.Command("git", "-C", dir, "add", "staged.go")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v %s", err, out)
	}
	st, err := g.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.Branch != "main" || st.Upstream != "origin/main" || st.Untracked != 1 || st.Staged != 1 || st.Unstaged != 1 {
		t.Fatalf("status inesperado: %+v", st)
	}
	staged, _ := g.StagedFiles()
	if len(staged) != 1 || staged[0] != "staged.go" {
		t.Fatalf("staged = %v", staged)
	}

	hash, err := g.Commit("Sprint 01 - Implementa x [TASK-001]\n\nCorpo.\n\nTask: TASK-001", false)
	if err != nil || len(hash) != 40 {
		t.Fatalf("commit: %v %q", err, hash)
	}
	log, err := g.Log("origin/main..HEAD", 10)
	if err != nil || len(log) != 1 {
		t.Fatalf("log: %v %+v", err, log)
	}
	if log[0].Subject != "Sprint 01 - Implementa x [TASK-001]" || log[0].Body != "Corpo.\n\nTask: TASK-001" || log[0].Author != "Ana Souza" {
		t.Fatalf("commit lido errado: %+v", log[0])
	}
}

func TestCLIBranchesAndRemote(t *testing.T) {
	g, _ := newRepo(t)
	branch := "sprint-01/TASK-API-01-emitir-jwt"
	if err := g.CreateBranch(branch, "main", true); err != nil {
		t.Fatal(err)
	}
	if g.CurrentBranch() != branch || !g.BranchExists(branch) {
		t.Fatal("branch não criada")
	}
	if _, err := g.Commit("Sprint 01 - Reserva TASK-API-01 [TASK-API-01]", true); err != nil {
		t.Fatal(err)
	}
	if err := g.Push("origin", branch, true); err != nil {
		t.Fatal(err)
	}
	refs, err := g.RemoteBranches("origin", "sprint-*/TASK-API-01-*")
	if err != nil || len(refs) != 1 || refs[0].Name != branch {
		t.Fatalf("ls-remote: %v %+v", err, refs)
	}
	tip, err := g.FetchTip("origin", branch)
	if err != nil || tip.Author != "Ana Souza" {
		t.Fatalf("fetch: %v %+v", err, tip)
	}
	hooks, err := g.GitPath("hooks")
	if err != nil || filepath.Base(hooks) != "hooks" {
		t.Fatalf("hooks: %v %q", err, hooks)
	}
	if err := g.Switch("main"); err != nil {
		t.Fatal(err)
	}
	files, err := g.ChangedSince("main")
	if err != nil || len(files) != 0 {
		t.Fatalf("changed: %v %v", err, files)
	}
}

func TestNotARepo(t *testing.T) {
	g := New(t.TempDir())
	if g.Available() {
		t.Fatal("diretório vazio não é repositório")
	}
}

func TestParseStatusRename(t *testing.T) {
	out := "# branch.oid abc\x00# branch.head feat/x\x00# branch.ab +2 -1\x00" +
		"2 R. N... 100644 100644 100644 a b R100 novo.go\x00velho.go\x00" +
		"u UU N... 1 2 3 4 h1 h2 h3 conflito.go\x00? solto.txt\x00"
	st := parseStatus(out)
	if st.Branch != "feat/x" || st.Ahead != 2 || st.Behind != 1 {
		t.Fatalf("cabeçalho: %+v", st)
	}
	if len(st.Changed) != 3 || st.Changed[0].Path != "novo.go" || st.Conflicts != 1 || st.Untracked != 1 {
		t.Fatalf("arquivos: %+v", st.Changed)
	}
}

func TestMatchGlob(t *testing.T) {
	cases := []struct {
		p, s string
		ok   bool
	}{
		{"sprint-*/TASK-1-*", "sprint-01/TASK-1-login", true},
		{"sprint-*/TASK-1-*", "sprint-01/TASK-12-login", false},
		{"*TASK-1-*", "feat/TASK-1-x", true},
		{"main", "main", true},
		{"main", "mainx", false},
	}
	for _, c := range cases {
		if MatchGlob(c.p, c.s) != c.ok {
			t.Errorf("MatchGlob(%q,%q) != %v", c.p, c.s, c.ok)
		}
	}
}
