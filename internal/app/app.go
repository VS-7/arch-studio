// Package app concentra as operações de domínio do ArchCode Studio.
//
// É a única camada autorizada a ler e mutar o estado em disco. A API HTTP (usada
// pelo navegador e pela janela do app desktop), o servidor MCP (agentes de IA)
// e o CLI chamam exatamente estes métodos, o que garante que humanos e IAs
// operem sobre a mesma semântica e as mesmas invariantes — em especial a
// preservação de coordenadas do canvas (RF017).
//
// Os arquivos se dividem por área: diagram (arquitetura macro), uml, docs
// (requisitos, casos de uso, ADRs), endpoints, pricing, tasks (AI-PRD), export
// (proposta e OpenAPI), reqdoc (Documento de Requisitos), layout (reorganizar
// diagramas), validate e project (leituras, arquivos brutos e mudanças externas). Os geradores em si são
// funções puras nos pacotes prd, proposal, openapi, reqdoc, lint e svgexport.
package app

import (
	"sync"
	"time"

	"github.com/archcode/studio/internal/gitx"
	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/store"
)

type App struct {
	st    *store.Store
	hb    *hub.Hub
	git   gitx.Git
	forge gitx.Forge
	now   func() time.Time

	// tx serializa as transações "ler → mutar → gravar". Sem ela, duas escritas
	// concorrentes (por exemplo, duas chamadas MCP em paralelo) poderiam carregar
	// o mesmo diagrama e uma sobrescrever a outra silenciosamente.
	tx sync.Mutex
}

// Option ajusta dependências do App (usado pelos testes).
type Option func(*App)

// WithGit troca o adaptador do Git (os testes usam gitx.Fake).
func WithGit(g gitx.Git) Option { return func(a *App) { a.git = g } }

// WithForge troca o servidor de código (GitHub via gh).
func WithForge(f gitx.Forge) Option { return func(a *App) { a.forge = f } }

// WithClock troca o relógio.
func WithClock(now func() time.Time) Option { return func(a *App) { a.now = now } }

func New(st *store.Store, hb *hub.Hub, opts ...Option) *App {
	a := &App{st: st, hb: hb, git: gitx.New(st.Root()), forge: gitx.NewGitHub(st.Root()), now: time.Now}
	for _, o := range opts {
		o(a)
	}
	return a
}

// timestamp devolve o instante atual no formato gravado nos arquivos.
func (a *App) timestamp() string { return a.now().UTC().Format(time.RFC3339) }

func (a *App) emit(ev hub.Event) {
	if a.hb != nil {
		a.hb.Broadcast(ev)
	}
}

func (a *App) Snapshot() (*store.Snapshot, error) { return a.st.Snapshot() }
