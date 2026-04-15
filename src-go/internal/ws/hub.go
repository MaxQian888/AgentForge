package ws

import (
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	projectID  string
	userID     string
	remoteAddr string

	subMu         sync.Mutex
	subscriptions map[string]struct{}
}

func (c *Client) logFields() log.Fields {
	return log.Fields{
		"userId":     c.userID,
		"projectId":  c.projectID,
		"remoteAddr": c.remoteAddr,
	}
}

func (c *Client) subscribe(channels []string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	if c.subscriptions == nil {
		c.subscriptions = map[string]struct{}{}
	}
	for _, ch := range channels {
		if ch == "" {
			continue
		}
		c.subscriptions[ch] = struct{}{}
	}
}

func (c *Client) unsubscribe(channels []string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for _, ch := range channels {
		delete(c.subscriptions, ch)
	}
}

func (c *Client) matchesAny(channels []string) bool {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	for _, ch := range channels {
		if _, ok := c.subscriptions[ch]; ok {
			return true
		}
	}
	return false
}

// Hub maintains the set of active clients and fans out pre-serialized frames.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]struct{}
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

// NewHub creates a new WebSocket hub.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub event loop. Should be run in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			clientCount := len(h.clients)
			h.mu.Unlock()
			fields := client.logFields()
			fields["clientCount"] = clientCount
			log.WithFields(fields).Info("ws client registered")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			clientCount := len(h.clients)
			h.mu.Unlock()
			fields := client.logFields()
			fields["clientCount"] = clientCount
			log.WithFields(fields).Info("ws client unregistered")

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					clientCount := len(h.clients)
					h.mu.Unlock()
					fields := client.logFields()
					fields["clientCount"] = clientCount
					log.WithFields(fields).Warn("ws client dropped: slow consumer")
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// FanoutBytes delivers a pre-serialized frame to every client subscribed to
// any of the given channels. Callers must supply at least one channel; an
// empty list short-circuits to a no-op (use BroadcastAllBytes for public
// events instead).
func (h *Hub) FanoutBytes(data []byte, channels []string) {
	if len(channels) == 0 {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if !client.matchesAny(channels) {
			continue
		}
		select {
		case client.send <- data:
		default:
		}
	}
}

// BroadcastAllBytes delivers a pre-serialized frame to every connected
// client regardless of subscriptions. Use only for Visibility=public events.
func (h *Hub) BroadcastAllBytes(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
