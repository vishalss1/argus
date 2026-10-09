package websocket
 
import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"
 
	gorilla "github.com/gorilla/websocket"
	"github.com/vishalss1/argus/core/internal/infrastructure/redis"
	"github.com/vishalss1/argus/shared/common"
)
 
type Message struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}
 
const (
	clientSendBuffer = 64
	clientWriteWait  = 10 * time.Second
)

// client owns a per-connection outbound queue so one slow peer cannot stall
// the hub's broadcast loop.
type client struct {
	conn *gorilla.Conn
	send chan []byte
}

func (c *client) writePump() {
	defer c.conn.Close()
	for payload := range c.send {
		_ = c.conn.SetWriteDeadline(time.Now().Add(clientWriteWait))
		if err := c.conn.WriteMessage(gorilla.TextMessage, payload); err != nil {
			log.Printf("websocket write failed: %v", err)
			return
		}
	}
}

type Hub struct {
	clients     map[*gorilla.Conn]*client
	register    chan *gorilla.Conn
	unregister  chan *gorilla.Conn
	broadcast   chan []byte
	redisClient *redis.Client
	mu          sync.Mutex
	closed      bool
}
 
func NewHub(redisClient *redis.Client) *Hub {
	return &Hub{
		clients:     make(map[*gorilla.Conn]*client),
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
			c := &client{conn: conn, send: make(chan []byte, clientSendBuffer)}
			h.clients[conn] = c
			common.WSConnections.Inc()
			go c.writePump()
		case conn := <-h.unregister:
			h.remove(conn)
		case payload := <-h.broadcast:
			for conn, c := range h.clients {
				select {
				case c.send <- payload:
				default:
					log.Printf("websocket client send buffer full, dropping connection")
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
	if c, ok := h.clients[conn]; ok {
		delete(h.clients, conn)
		common.WSConnections.Dec()
		close(c.send) // stops writePump
	}
	_ = conn.Close()
}

