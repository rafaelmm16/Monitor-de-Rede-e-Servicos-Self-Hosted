package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rafael/monitor-de-rede-backend/internal/models"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
	}
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	c := &client{conn: conn}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, c)
		h.mu.Unlock()
		_ = conn.Close()
	}()

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}
}

func (h *Hub) Broadcast(event string, data any) {
	payload, err := json.Marshal(models.WSMessage{
		Event: event,
		Data:  data,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.mu.Lock()
		err := c.conn.WriteMessage(websocket.TextMessage, payload)
		c.mu.Unlock()
		if err != nil {
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			_ = c.conn.Close()
		}
	}
}
