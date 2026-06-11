package websocket
 
import (
	"context"
	"encoding/json"
	"log"
	"sync"
 
	gorilla "github.com/gorilla/websocket"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	"github.com/vishalss1/argus/shared/common"
)
 
type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}
 
type Hub struct {
	clients     map[*gorilla.Conn]struct{}
	register    chan *gorilla.Conn
	unregister  chan *gorilla.Conn
	broadcast   chan []byte
	redisClient *redis.Client
	mu          sync.Mutex
	closed      bool
}
 
func NewHub(redisClient *redis.Client) *Hub {
	return &Hub{
		clients:     make(map[*gorilla.Conn]struct{}),
		register:    make(chan *gorilla.Conn, 64),
		unregister:  make(chan *gorilla.Conn, 64),
		broadcast:   make(chan []byte, 64),
		redisClient: redisClient,
	}
}
 
func (h *Hub) Run(ctx context.Context) {
	if h.redisClient != nil {
		pubsub := h.redisClient.Client().Subscribe(ctx, "ws:broadcast")
		go func() {
			defer pubsub.Close()
			ch := pubsub.Channel()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-ch:
					if !ok {
						return
					}
					h.broadcastLocal([]byte(msg.Payload))
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			h.Close()
			return
		case conn := <-h.register:
			h.clients[conn] = struct{}{}
			common.WSConnections.Inc()
		case conn := <-h.unregister:
			h.remove(conn)
		case payload := <-h.broadcast:
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
 
	if h.redisClient != nil {
		err := h.redisClient.Client().Publish(context.Background(), "ws:broadcast", message).Err()
		if err != nil {
			log.Printf("websocket redis publish failed: %v", err)
			h.broadcastLocal(message)
		}
	} else {
		h.broadcastLocal(message)
	}
}

func (h *Hub) broadcastLocal(message []byte) {
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
		common.WSConnections.Dec()
	}
	_ = conn.Close()
}

