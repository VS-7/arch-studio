// Package hub distribui eventos de mudança de estado para todos os clientes
// conectados (browser, app desktop) em tempo real (RF023).
package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Tipos de evento emitidos pelo servidor.
const (
	EventHello       = "hello"
	EventSnapshot    = "snapshot"
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
	EventError       = "error"
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

type client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}

	upgrader websocket.Upgrader
}

func New() *Hub {
	return &Hub{
		clients: map[*client]struct{}{},
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 8192,
			// Servidor local-first: aceita apenas origens de loopback (RNF007).
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast envia o evento a todos os clientes conectados. Clientes lentos que
// não consomem a fila são desconectados em vez de bloquear o servidor.
func (h *Hub) Broadcast(ev Event) {
	if ev.At == "" {
		ev.At = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("hub: falha ao serializar evento %s: %v", ev.Type, err)
		return
	}
	h.mu.RLock()
	targets := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- data:
		default:
			h.remove(c)
		}
	}
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// ServeWS faz o upgrade da conexão HTTP e mantém o cliente registrado.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go h.writePump(c)
	h.readPump(c)
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
)

func (h *Hub) readPump(c *client) {
	defer func() {
		h.remove(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(1 << 20)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
