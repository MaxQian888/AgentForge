package ws

import (
	"encoding/json"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

// SubscriptionRouter receives connection-lifecycle, client-message, and
// broadcast-event callbacks. The live-artifact router is the only
// implementation today; the interface is kept minimal so future routers
// can plug in without widening Hub's surface.
type SubscriptionRouter interface {
	OnClientRegister(clientID string)
	OnClientUnregister(clientID string)
	OnClientMessage(clientID string, payload []byte) error
	OnEvent(eventType string, payload []byte)
}

// Client represents a connected WebSocket client.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte
	projectID  string
	userID     string
	remoteAddr string
	id         string

	subMu         sync.Mutex
	subscriptions map[string]struct{}
}

// ID returns the opaque per-connection id used by the subscription router
// to target a single client. Populated on register; never empty once the
// client is live.
func (c *Client) ID() string { return c.id }

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

	routerMu sync.RWMutex
	router   SubscriptionRouter
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

// SetSubscriptionRouter installs the per-asset subscription router.
// Passing nil removes the router. Safe to call after Run has started.
func (h *Hub) SetSubscriptionRouter(r SubscriptionRouter) {
	h.routerMu.Lock()
	h.router = r
	h.routerMu.Unlock()
}

func (h *Hub) getRouter() SubscriptionRouter {
	h.routerMu.RLock()
	defer h.routerMu.RUnlock()
	return h.router
}

// Run starts the hub event loop. Should be run in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			if client.id == "" {
				client.id = fmt.Sprintf("client-%p", client)
			}
			h.mu.Lock()
			h.clients[client] = struct{}{}
			clientCount := len(h.clients)
			h.mu.Unlock()
			fields := client.logFields()
			fields["clientCount"] = clientCount
			fields["clientId"] = client.id
			log.WithFields(fields).Info("ws client registered")
			if r := h.getRouter(); r != nil {
				r.OnClientRegister(client.id)
			}

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
			fields["clientId"] = client.id
			log.WithFields(fields).Info("ws client unregistered")
			if r := h.getRouter(); r != nil && client.id != "" {
				r.OnClientUnregister(client.id)
			}

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

// SendToClient delivers a pre-serialized frame to a single client by id.
// Returns true when the target was found and the frame was enqueued.
// A slow consumer whose send buffer is full is treated as a miss (false);
// it is NOT dropped here — the broadcast path retains that policy.
func (h *Hub) SendToClient(clientID string, data []byte) bool {
	if clientID == "" {
		return false
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.id != clientID {
			continue
		}
		select {
		case client.send <- data:
			return true
		default:
			return false
		}
	}
	return false
}

// BroadcastEvent builds the canonical `{type,payload}` envelope, enqueues
// it onto the broadcast channel for subscription-less fan-out, and calls
// the subscription router synchronously so it can match the event against
// per-asset filters. eventType is the hub event name (see events.go);
// payload is the already-JSON-encoded event body.
//
// TODO: migrate existing FanoutBytes/BroadcastAllBytes callers that build
// envelopes by hand to this method so the router sees every event.
func (h *Hub) BroadcastEvent(eventType string, payload []byte) {
	if r := h.getRouter(); r != nil {
		r.OnEvent(eventType, payload)
	}
	var raw json.RawMessage
	if len(payload) == 0 {
		raw = json.RawMessage("null")
	} else {
		raw = json.RawMessage(payload)
	}
	frame := struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}{Type: eventType, Payload: raw}
	data, err := json.Marshal(frame)
	if err != nil {
		log.WithError(err).WithField("eventType", eventType).Warn("ws broadcast event marshal failed")
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.WithField("eventType", eventType).Warn("ws broadcast channel full, frame dropped")
	}
}

// DeliverClientMessage forwards an inbound client control message to the
// subscription router, if any. This is a thin pass-through so the read
// pump can stay router-agnostic; the coordinator wires real calls to
// this from handler.go once the router lands. Returns the router's error
// verbatim so callers can log it.
func (h *Hub) DeliverClientMessage(clientID string, data []byte) error {
	r := h.getRouter()
	if r == nil {
		return nil
	}
	return r.OnClientMessage(clientID, data)
}
