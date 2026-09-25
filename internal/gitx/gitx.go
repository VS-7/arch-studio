// Package gitx é o adaptador do ArchCode Studio para o Git.
//
// Usa o executável `git` do sistema, e não uma biblioteca, para respeitar as
// credenciais, os hooks e a configuração de quem usa. Sem `git` instalado, o
// planejamento continua funcionando e as funções de Git ficam desabilitadas.
// O resto do sistema depende só da interface Git; os testes usam o Fake.
package gitx

import (
	"errors"
	"strings"
)

// ErrUnavailable indica que o Git não está instalado ou o diretório não é um
// repositório.
var ErrUnavailable = errors.New("git indisponível: instale o Git e rode dentro de um repositório")

// Commit é um commit do histórico.
type Commit struct {
	Hash    string `json:"hash"`
	Parents int    `json:"parents"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`
	Subject string `json:"subject"`
	Body    string `json:"body,omitempty"`
}

// Merge informa se o commit é um merge (mais de um pai).
func (c Commit) Merge() bool { return c.Parents > 1 }

// FileChange é um arquivo alterado na árvore de trabalho.
type FileChange struct {
	Path string `json:"path"`
	// Code é o código XY do `git status` (ex.: "M.", ".M", "??", "A.").
	Code string `json:"code"`
}

// Status é o estado da árvore de trabalho e da branch atual.
type Status struct {
	Branch    string       `json:"branch"`
	Detached  bool         `json:"detached,omitempty"`
	Upstream  string       `json:"upstream,omitempty"`
	Ahead     int          `json:"ahead"`
	Behind    int          `json:"behind"`
	Changed   []FileChange `json:"changed"`
	Staged    int          `json:"staged"`
	Unstaged  int          `json:"unstaged"`
	Untracked int          `json:"untracked"`
	Conflicts int          `json:"conflicts"`
}

// Clean informa se não há nada alterado.
func (s *Status) Clean() bool { return len(s.Changed) == 0 }

// RemoteRef é uma branch no servidor.
type RemoteRef struct {
	Name string `json:"name"`
	Hash string `json:"hash"`
}

// Git é o que o Studio precisa do controle de versão.
type Git interface {
	// Available informa se o git existe e o diretório é um repositório.
	Available() bool
	// Identity devolve user.name e user.email configurados.
	Identity() (name, email string)
	CurrentBranch() string
	Status() (*Status, error)
	// Log lista até max commits de rev ("main", "origin/main..HEAD"…).
	Log(rev string, max int) ([]Commit, error)
	RevExists(rev string) bool
	BranchExists(name string) bool
	// CreateBranch cria a branch a partir de from ("" = HEAD) e, com
	// switchTo, passa a trabalhar nela.
	CreateBranch(name, from string, switchTo bool) error
	Switch(name string) error
	RemoteExists(remote string) bool
	// RemoteBranches lista as branches do remoto que casam com o padrão
	// glob (ex.: "sprint-*/TASK-API-01-*").
	RemoteBranches(remote, pattern string) ([]RemoteRef, error)
	// FetchTip busca a branch do remoto e devolve o último commit dela.
	FetchTip(remote, branch string) (*Commit, error)
	// Add prepara (git add) os caminhos informados.
	Add(paths []string) error
	// Commit grava os arquivos preparados (staged) e devolve o hash.
	Commit(message string, allowEmpty bool) (string, error)
	Push(remote, branch string, setUpstream bool) error
	StagedFiles() ([]string, error)
	// ChangedSince lista os arquivos alterados desde base (commits e árvore).
	ChangedSince(base string) ([]string, error)
	// GitPath resolve um caminho dentro do diretório do Git (ex.: "hooks").
	GitPath(rel string) (string, error)
	ConfigSet(key, value string) error
}

// Identity formata "Nome <email>" (ou só o que existir).
func Identity(g Git) string {
	if g == nil {
		return ""
	}
	name, email := g.Identity()
	switch {
	case name != "" && email != "":
		return name + " <" + email + ">"
	case name != "":
		return name
	default:
		return email
	}
}

// MatchGlob compara um nome com um padrão glob simples (só '*').
func MatchGlob(pattern, name string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == name
	}
	if !strings.HasPrefix(name, parts[0]) {
		return false
	}
	name = name[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(name, parts[i])
		if idx < 0 {
			return false
		}
		name = name[idx+len(parts[i]):]
	}
	return strings.HasSuffix(name, parts[len(parts)-1])
}
