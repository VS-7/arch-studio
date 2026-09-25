package gitx

import (
	"fmt"
	"strings"
	"sync"
)

// Fake é um Git em memória para testes: branches, commits e um remoto
// simulado. Só o que o Studio usa é modelado.
type Fake struct {
	mu sync.Mutex

	Name, Email string
	Branch      string
	Branches    map[string][]Commit // branch → commits (mais recente primeiro)
	Remote      map[string][]Commit // branches publicadas
	Staged      []string
	Working     []FileChange
	NoRepo      bool
	Pushed      []string
	Configs     map[string]string
	HooksPath   string
	seq         int
}

// NewFake cria um repositório com a branch main e um commit inicial.
func NewFake(name, email string) *Fake {
	f := &Fake{Name: name, Email: email, Branch: "main", Branches: map[string][]Commit{}, Remote: map[string][]Commit{},
		Configs: map[string]string{}}
	f.Branches["main"] = []Commit{f.newCommit("Commit inicial")}
	f.Remote["main"] = append([]Commit(nil), f.Branches["main"]...)
	return f
}

func (f *Fake) newCommit(msg string) Commit {
	f.seq++
	subject, body, _ := strings.Cut(msg, "\n")
	return Commit{Hash: fmt.Sprintf("%040d", f.seq), Parents: 1, Author: f.Name, Email: f.Email,
		Date: fmt.Sprintf("2026-09-25T10:%02d:00Z", f.seq%60), Subject: subject, Body: strings.TrimSpace(body)}
}

// AddCommit grava um commit na branch informada (atalho para testes).
func (f *Fake) AddCommit(branch, msg string) Commit {
	f.mu.Lock()
	defer f.mu.Unlock()
	c := f.newCommit(msg)
	f.Branches[branch] = append([]Commit{c}, f.Branches[branch]...)
	return c
}

func (f *Fake) Available() bool            { return !f.NoRepo }
func (f *Fake) Identity() (string, string) { return f.Name, f.Email }

func (f *Fake) CurrentBranch() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.Branch
}

func (f *Fake) Status() (*Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.NoRepo {
		return nil, ErrUnavailable
	}
	st := &Status{Branch: f.Branch, Changed: append([]FileChange{}, f.Working...)}
	for _, p := range f.Staged {
		st.Changed = append(st.Changed, FileChange{Path: p, Code: "A."})
		st.Staged++
	}
	st.Unstaged = len(f.Working)
	if _, ok := f.Remote[f.Branch]; ok {
		st.Upstream = "origin/" + f.Branch
		st.Ahead = len(f.Branches[f.Branch]) - len(f.Remote[f.Branch])
	}
	return st, nil
}

func (f *Fake) Log(rev string, max int) ([]Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	branch := rev
	var exclude map[string]bool
	if a, b, ok := strings.Cut(rev, ".."); ok {
		branch = b
		exclude = map[string]bool{}
		for _, c := range f.Branches[strings.TrimPrefix(a, "origin/")] {
			exclude[c.Hash] = true
		}
	}
	if branch == "" || branch == "HEAD" {
		branch = f.Branch
	}
	out := []Commit{}
	for _, c := range f.Branches[strings.TrimPrefix(branch, "origin/")] {
		if exclude[c.Hash] {
			continue
		}
		out = append(out, c)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out, nil
}

func (f *Fake) RevExists(rev string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.Branches[strings.TrimPrefix(rev, "origin/")]
	return ok
}

func (f *Fake) BranchExists(name string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.Branches[name]
	return ok
}

func (f *Fake) CreateBranch(name, from string, switchTo bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Branches[name]; ok {
		return fmt.Errorf("git branch: a branch '%s' já existe", name)
	}
	if from == "" {
		from = f.Branch
	}
	f.Branches[name] = append([]Commit(nil), f.Branches[from]...)
	if switchTo {
		f.Branch = name
	}
	return nil
}

func (f *Fake) Switch(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.Branches[name]; !ok {
		return fmt.Errorf("git switch: branch '%s' não existe", name)
	}
	f.Branch = name
	return nil
}

func (f *Fake) RemoteExists(string) bool { return !f.NoRepo }

func (f *Fake) RemoteBranches(remote, pattern string) ([]RemoteRef, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []RemoteRef{}
	for name, commits := range f.Remote {
		if pattern == "" || MatchGlob(pattern, name) {
			out = append(out, RemoteRef{Name: name, Hash: commits[0].Hash})
		}
	}
	return out, nil
}

func (f *Fake) FetchTip(remote, branch string) (*Commit, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	commits := f.Remote[branch]
	if len(commits) == 0 {
		return nil, fmt.Errorf("branch %s não existe no remoto", branch)
	}
	c := commits[0]
	return &c, nil
}

func (f *Fake) Add(paths []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	keep := []FileChange{}
	for _, w := range f.Working {
		matched := false
		for _, p := range paths {
			if w.Path == p || strings.HasPrefix(w.Path, strings.TrimSuffix(p, "/")+"/") {
				matched = true
			}
		}
		if matched {
			f.Staged = append(f.Staged, w.Path)
		} else {
			keep = append(keep, w)
		}
	}
	f.Working = keep
	return nil
}

func (f *Fake) Commit(message string, allowEmpty bool) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.Staged) == 0 && !allowEmpty {
		return "", fmt.Errorf("git commit: nada preparado para commit")
	}
	c := f.newCommit(message)
	f.Branches[f.Branch] = append([]Commit{c}, f.Branches[f.Branch]...)
	f.Staged = nil
	return c.Hash, nil
}

func (f *Fake) Push(remote, branch string, setUpstream bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Remote[branch] = append([]Commit(nil), f.Branches[branch]...)
	f.Pushed = append(f.Pushed, branch)
	return nil
}

func (f *Fake) StagedFiles() ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.Staged...), nil
}

func (f *Fake) ChangedSince(string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := append([]string(nil), f.Staged...)
	for _, w := range f.Working {
		out = append(out, w.Path)
	}
	return out, nil
}

func (f *Fake) GitPath(rel string) (string, error) {
	if f.HooksPath != "" && rel == "hooks" {
		return f.HooksPath, nil
	}
	return "", ErrUnavailable
}

func (f *Fake) ConfigSet(key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Configs[key] = value
	return nil
}

var _ Git = (*Fake)(nil)
var _ Git = (*CLI)(nil)
