package rest

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/websocket"
)

// wsHub manages WebSocket connections for event streaming.
type wsHub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func newWSHub() *wsHub {
	return &wsHub{
		clients: make(map[*websocket.Conn]bool),
	}
}

func (h *wsHub) add(conn *websocket.Conn) {
	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()
}

func (h *wsHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	delete(h.clients, conn)
	h.mu.Unlock()
}

func (h *wsHub) broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.clients {
		if _, err := conn.Write(data); err != nil {
			log.Printf("[ws] write error: %v", err)
		}
	}
}

// handleWebSocketEvents streams real-time events to connected WebSocket clients.
func (s *Server) handleWebSocketEvents(ws *websocket.Conn) {
	s.wsHub.add(ws)
	defer s.wsHub.remove(ws)
	defer func() { _ = ws.Close() }()

	log.Printf("[ws] client connected (%s)", ws.Request().RemoteAddr)

	// Keep connection alive by reading (client may send pings).
	buf := make([]byte, 512)
	for {
		if _, err := ws.Read(buf); err != nil {
			log.Printf("[ws] client disconnected (%s): %v", ws.Request().RemoteAddr, err)
			return
		}
	}
}

// startEventBroadcaster polls engine events and broadcasts to all WS clients.
func (s *Server) startEventBroadcaster() {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			if s.engine == nil {
				continue
			}

			status := s.engine.Status()
			msg := map[string]interface{}{
				"type":      "status_update",
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"data":      status,
			}

			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}

			s.wsHub.mu.RLock()
			clientCount := len(s.wsHub.clients)
			s.wsHub.mu.RUnlock()

			if clientCount > 0 {
				s.wsHub.broadcast(data)
			}
		}
	}()
}

// wsUpgradeHandler wraps the WebSocket handler for use with http.ServeMux.
func (s *Server) wsUpgradeHandler() http.Handler {
	return websocket.Handler(s.handleWebSocketEvents)
}
