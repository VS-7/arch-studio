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
// (proposta e OpenAPI), reqdoc (Documento de Requisitos), validate e project
// (leituras, arquivos brutos e mudanças externas). Os geradores em si são
// funções puras nos pacotes prd, proposal, openapi, reqdoc, lint e svgexport.
package app

import (
	"sync"

	"github.com/archcode/studio/internal/hub"
	"github.com/archcode/studio/internal/store"
)

type App struct {
	st *store.Store
	hb *hub.Hub

	// tx serializa as transações "ler → mutar → gravar". Sem ela, duas escritas
	// concorrentes (por exemplo, duas chamadas MCP em paralelo) poderiam carregar
	// o mesmo diagrama e uma sobrescrever a outra silenciosamente.
	tx sync.Mutex
}

func New(st *store.Store, hb *hub.Hub) *App { return &App{st: st, hb: hb} }

func (a *App) emit(ev hub.Event) {
	if a.hb != nil {
		a.hb.Broadcast(ev)
	}
}

func (a *App) Snapshot() (*store.Snapshot, error) { return a.st.Snapshot() }
