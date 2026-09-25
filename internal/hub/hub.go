// Package hub distribui eventos de mudança de estado para todos os
// interessados (WebSocket do browser, janela do app desktop) em tempo real
// (RF023).
//
// É um barramento em memória, sem dependência de transporte: cada adaptador
// assina o hub e entrega os eventos pelo seu próprio canal.
package hub

import (
	"sync"
	"time"
)

// Tipos de evento emitidos pelo servidor.
const (
	EventDiagram     = "diagram_changed"
	EventDocs        = "docs_changed"
	EventEndpoints   = "endpoints_changed"
	EventPricing     = "pricing_changed"
	EventTasks       = "tasks_changed"
	EventManifest    = "manifest_changed"
	EventNodeAdded   = "node_added"
	EventNodeUpdated = "node_updated"
	EventNodeRemoved = "node_removed"
	EventEdgeAdded   = "edge_added"
	EventPRD         = "ai_prd_generated"
	EventUML         = "uml_changed"

	// Módulo de Implementação.
	EventPlan        = "plan_changed"
	EventConventions = "conventions_changed"
	EventSession     = "session_logged"
	EventMemory      = "memory_changed"
	EventSkills      = "skills_changed"
	EventGit         = "git_changed"
)

// Origem da mudança, usada pela UI para decidir se deve animar ou recarregar.
const (
	SourceUI   = "ui"
	SourceAI   = "ai"
	SourceDisk = "disk"
	SourceCLI  = "cli"
)

type Event struct {
	Type    string `json:"type"`
	Source  string `json:"source"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
	Payload any    `json:"payload,omitempty"`
	At      string `json:"at"`
}

type Hub struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

func New() *Hub { return &Hub{subs: map[chan Event]struct{}{}} }

// Count devolve o número de assinantes ativos.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

// Subscribe registra um assinante com uma fila de `buf` eventos. O canal é
// fechado ao cancelar a assinatura ou quando o assinante fica para trás.
func (h *Hub) Subscribe(buf int) (<-chan Event, func()) {
	ch := make(chan Event, buf)
	h.mu.Lock()
	h.subs[ch] = struct{}{}
	h.mu.Unlock()
	return ch, func() { h.drop(ch) }
}

// Broadcast entrega o evento a todos os assinantes. Quem não consome a fila é
// desconectado em vez de bloquear quem publicou.
func (h *Hub) Broadcast(ev Event) {
	if ev.At == "" {
		ev.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	// Os envios acontecem sob a trava de leitura (são não bloqueantes), para
	// que nenhum canal seja fechado por drop no meio do caminho.
	var slow []chan Event
	h.mu.RLock()
	for ch := range h.subs {
		select {
		case ch <- ev:
		default:
			slow = append(slow, ch)
		}
	}
	h.mu.RUnlock()

	for _, ch := range slow {
		h.drop(ch)
	}
}

func (h *Hub) drop(ch chan Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[ch]; ok {
		delete(h.subs, ch)
		close(ch)
	}
}
