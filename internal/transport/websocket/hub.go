package websocket

import (
	"context"
	"encoding/json"
	"log"
	"sync"

	gorilla "github.com/gorilla/websocket"
)

type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Hub struct {
	clients    map[*gorilla.Conn]struct{}
	register   chan *gorilla.Conn
	unregister chan *gorilla.Conn
	broadcast  chan []byte
	mu         sync.Mutex
	closed     bool
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*gorilla.Conn]struct{}),
		register:   make(chan *gorilla.Conn, 64),
		unregister: make(chan *gorilla.Conn, 64),
		broadcast:  make(chan []byte, 64),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.Close()
			return
		case conn := <-h.register:
			log.Printf("[WS HUB] client registered: %s (Total: %d)", conn.RemoteAddr(), len(h.clients)+1)
			h.clients[conn] = struct{}{}
		case conn := <-h.unregister:
			log.Printf("[WS HUB] client unregistered (Total: %d)", len(h.clients)-1)
			h.remove(conn)
		case payload := <-h.broadcast:
			log.Printf("[WS HUB] broadcasting message to %d clients: %s", len(h.clients), string(payload))
			for conn := range h.clients {
				if err := conn.WriteMessage(gorilla.TextMessage, payload); err != nil {
					log.Printf("websocket write failed: %v", err)
					h.remove(conn)
				}
			}
		}
	}
}

func (h *Hub) Broadcast(messageType string, payload any) {
	message, err := json.Marshal(Message{Type: messageType, Payload: payload})
	if err != nil {
		log.Printf("websocket marshal failed: %v", err)
		return
	}

	h.BroadcastJSON(message)
}

func (h *Hub) BroadcastPayload(payload any) {
	message, err := json.Marshal(payload)
	if err != nil {
		log.Printf("websocket marshal failed: %v", err)
		return
	}

	h.BroadcastJSON(message)
}

func (h *Hub) BroadcastJSON(message []byte) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		return
	}

	select {
	case h.broadcast <- message:
	default:
		log.Printf("websocket broadcast dropped: channel full")
	}
}

func (h *Hub) Register(conn *gorilla.Conn) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		_ = conn.Close()
		return
	}

	select {
	case h.register <- conn:
	default:
		_ = conn.Close()
	}
}

func (h *Hub) Unregister(conn *gorilla.Conn) {
	h.mu.Lock()
	closed := h.closed
	h.mu.Unlock()
	if closed {
		_ = conn.Close()
		return
	}

	select {
	case h.unregister <- conn:
	default:
		_ = conn.Close()
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	h.mu.Unlock()

	for conn := range h.clients {
		h.remove(conn)
	}
}

func (h *Hub) remove(conn *gorilla.Conn) {
	if _, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
	}
	_ = conn.Close()
}
