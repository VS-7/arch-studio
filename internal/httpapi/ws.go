package httpapi

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/archcode/studio/internal/hub"
)

// Tempo real: cada conexão WebSocket é um assinante do hub.

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	wsQueue    = 64
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 8192,
	CheckOrigin:     allowedOrigin,
}

func (s *Server) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	events, cancel := s.hub.Subscribe(wsQueue)
	go writePump(conn, events)
	readPump(conn)
	cancel()
}

// readPump só mantém a conexão viva (pong) e detecta o fechamento pelo cliente.
func readPump(conn *websocket.Conn) {
	defer func() { _ = conn.Close() }()
	conn.SetReadLimit(1 << 20)
	_ = conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump encaminha os eventos do hub; o canal fecha quando o cliente sai
// ou fica para trás, e a conexão é encerrada.
func writePump(conn *websocket.Conn, events <-chan hub.Event) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = conn.Close()
	}()
	for {
		select {
		case ev, ok := <-events:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			data, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Origem das requisições (RNF007)
// ---------------------------------------------------------------------------

// allowedOrigin aceita requisições sem Origin (CLI, agentes MCP), da própria
// origem (inclusive atrás de proxy reverso), de loopback (Vite em dev) e da
// janela do app desktop. Qualquer outro site é recusado: sem isso, uma página
// aberta no navegador poderia ler os eventos ou alterar o projeto local.
func allowedOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if u.Scheme == "wails" || u.Hostname() == "wails.localhost" {
		return true
	}
	if strings.EqualFold(u.Host, r.Host) || strings.EqualFold(u.Host, r.Header.Get("X-Forwarded-Host")) {
		return true
	}
	return isLoopback(u.Hostname())
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// guardOrigin recusa requisições que alteram estado vindas de outros sites.
func guardOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
		default:
			if !allowedOrigin(r) {
				http.Error(w, "origem não permitida", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
