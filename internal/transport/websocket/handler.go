package websocket

import (
	"log"
	"net/http"
	"time"

	gorilla "github.com/gorilla/websocket"
	"github.com/vishalss1/argus/internal/domain/auth"
)

type Handler struct {
	hub         *Hub
	authService *auth.Service
	upgrader    gorilla.Upgrader
}

func NewHandler(hub *Hub, authService *auth.Service) *Handler {
	return &Handler{
		hub:         hub,
		authService: authService,
		upgrader: gorilla.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

type AuthPayload struct {
	Token       string `json:"token"`
	WorkspaceID string `json:"workspace_id"`
}

type AuthMessage struct {
	Type    string      `json:"type"`
	Payload AuthPayload `json:"payload"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS DEBUG] upgrade failed: %v", err)
		return
	}

	// 1. Enforce 5-second deadline for authentication message
	authDeadline := time.Now().Add(5 * time.Second)
	_ = conn.SetReadDeadline(authDeadline)

	var authMsg AuthMessage
	err = conn.ReadJSON(&authMsg)
	if err != nil {
		log.Printf("[WS DEBUG] auth read failed: %v", err)
		_ = conn.WriteJSON(map[string]any{"type": "error", "payload": "authentication required within 5 seconds"})
		_ = conn.Close()
		return
	}

	if authMsg.Type != "auth" {
		log.Printf("[WS DEBUG] invalid auth message type: %s", authMsg.Type)
		_ = conn.WriteJSON(map[string]any{"type": "error", "payload": "invalid message type, auth required"})
		_ = conn.Close()
		return
	}

	// 2. Validate token
	claims, err := h.authService.ValidateAccessToken(authMsg.Payload.Token)
	if err != nil {
		log.Printf("[WS DEBUG] invalid token: %v", err)
		_ = conn.WriteJSON(map[string]any{"type": "error", "payload": "invalid or expired token"})
		_ = conn.Close()
		return
	}

	// 3. Validate workspace membership
	isMember, err := h.authService.CheckWorkspaceMembership(r.Context(), claims.UserID, authMsg.Payload.WorkspaceID)
	if err != nil || !isMember {
		log.Printf("[WS DEBUG] workspace access denied user=%s workspace=%s", claims.UserID, authMsg.Payload.WorkspaceID)
		_ = conn.WriteJSON(map[string]any{"type": "error", "payload": "workspace access denied"})
		_ = conn.Close()
		return
	}

	// 4. Authentication success, clear read deadline
	_ = conn.SetReadDeadline(time.Time{})
	_ = conn.WriteJSON(map[string]any{"type": "authenticated"})

	h.hub.Register(conn)
	defer h.hub.Unregister(conn)

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}
