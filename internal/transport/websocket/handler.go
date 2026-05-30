package websocket

import (
	"log"
	"net/http"

	gorilla "github.com/gorilla/websocket"
)

type Handler struct {
	hub      *Hub
	upgrader gorilla.Upgrader
}

func NewHandler(hub *Hub) *Handler {
	return &Handler{
		hub: hub,
		upgrader: gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS DEBUG] upgrade failed: %v", err)
		return
	}

	h.hub.Register(conn)
	defer h.hub.Unregister(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
