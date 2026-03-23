package ws

import (
	"log/slog"
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents a connected WebSocket client.
type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	send      chan []byte
	projectID string // filter events by project, empty = all
	userID    string
}

// Hub maintains the set of active clients and broadcasts events.
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
			h.mu.Unlock()
			slog.Debug("ws client connected", "user", client.userID, "project", client.projectID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			slog.Debug("ws client disconnected", "user", client.userID)

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Slow client, drop
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients, client)
					close(client.send)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastEvent sends an event to all connected clients.
// If the event has a ProjectID, only clients subscribed to that project receive it.
func (h *Hub) BroadcastEvent(event *Event) {
	data := event.JSON()

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		// Filter by project if both event and client have projectID set.
		if event.ProjectID != "" && client.projectID != "" && client.projectID != event.ProjectID {
			continue
		}
		select {
		case client.send <- data:
		default:
			// Skip slow client.
		}
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
