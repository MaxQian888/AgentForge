package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/agentforge/im-bridge/core"
)

// Notification is the payload sent from the Go backend.
type Notification struct {
	Type           string     `json:"type"` // e.g., "task_completed", "agent_finished", "cost_alert"
	TargetIMUserID string     `json:"target_im_user_id"`
	TargetChatID   string     `json:"target_chat_id"`
	Platform       string     `json:"platform"` // "feishu", "slack", etc.
	Content        string     `json:"content"`  // plain text fallback
	Card           *core.Card `json:"card"`     // optional rich card
}

// ActionHandler processes button click actions from IM cards.
type ActionHandler interface {
	HandleAction(ctx context.Context, action, entityID, chatID string) (string, error)
}

// Receiver listens for notifications from the AgentForge backend and pushes them to IM.
type Receiver struct {
	platform      core.Platform
	metadata      core.PlatformMetadata
	port          string
	server        *http.Server
	actionHandler ActionHandler
	sharedSecret  string
	mu            sync.Mutex
	processed     map[string]struct{}
}

// NewReceiver creates a notification receiver bound to a platform.
func NewReceiver(platform core.Platform, port string) *Receiver {
	return &Receiver{
		platform: platform,
		metadata: core.MetadataForPlatform(platform),
		port:     port,
		processed: make(map[string]struct{}),
	}
}

// SetActionHandler sets the handler for card button actions.
func (r *Receiver) SetActionHandler(h ActionHandler) {
	r.actionHandler = h
}

func (r *Receiver) SetSharedSecret(secret string) {
	r.sharedSecret = strings.TrimSpace(secret)
}

// Start begins listening for notifications.
func (r *Receiver) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /im/notify", r.handleNotify)
	mux.HandleFunc("POST /im/send", r.handleSend)
	mux.HandleFunc("POST /im/action", r.handleAction)
	mux.HandleFunc("GET /im/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                 "ok",
			"platform":               r.platform.Name(),
			"source":                 r.metadata.Source,
			"supports_rich_messages": r.metadata.Capabilities.SupportsRichMessages,
		})
	})

	r.server = &http.Server{
		Addr:    ":" + r.port,
		Handler: mux,
	}

	log.Printf("[notify] Notification receiver starting on :%s", r.port)
	if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("notification server: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the notification receiver.
func (r *Receiver) Stop() error {
	if r.server != nil {
		return r.server.Shutdown(context.Background())
	}
	return nil
}

func (r *Receiver) handleNotify(w http.ResponseWriter, req *http.Request) {
	bodyBytes, ok := r.verifyAndRememberDelivery(w, req, "/im/notify")
	if !ok {
		return
	}
	var n Notification
	if err := json.Unmarshal(bodyBytes, &n); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	notificationPlatform := core.NormalizePlatformName(n.Platform)
	activePlatform := r.metadata.Source
	if notificationPlatform == "" {
		http.Error(w, "notification platform is required", http.StatusBadRequest)
		return
	}
	if notificationPlatform != activePlatform {
		http.Error(w, fmt.Sprintf("notification platform %q does not match active platform %q", notificationPlatform, activePlatform), http.StatusConflict)
		return
	}

	ctx := context.Background()
	chatID := n.TargetChatID
	if chatID == "" {
		chatID = n.TargetIMUserID // fallback to DM
	}

	// Try card first if available.
	if n.Card != nil && r.metadata.Capabilities.SupportsRichMessages {
		if cs, ok := r.platform.(core.CardSender); ok {
			if err := cs.SendCard(ctx, chatID, n.Card); err != nil {
				log.Printf("[notify] Failed to send card to %s: %v", chatID, err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "sent", "type": "card"})
			return
		}
	}

	// Fallback to plain text.
	if err := r.platform.Send(ctx, chatID, n.Content); err != nil {
		log.Printf("[notify] Failed to send message to %s: %v", chatID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent", "type": "text"})
}

// SendRequest is the payload for the /im/send endpoint.
type SendRequest struct {
	Platform string `json:"platform"`
	ChatID   string `json:"chat_id"`
	Content  string `json:"content"`
	ThreadID string `json:"thread_id,omitempty"`
}

func (r *Receiver) handleSend(w http.ResponseWriter, req *http.Request) {
	bodyBytes, ok := r.verifyAndRememberDelivery(w, req, "/im/send")
	if !ok {
		return
	}
	var s SendRequest
	if err := json.Unmarshal(bodyBytes, &s); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	sendPlatform := core.NormalizePlatformName(s.Platform)
	activePlatform := r.metadata.Source
	if sendPlatform != "" && sendPlatform != activePlatform {
		http.Error(w, fmt.Sprintf("send platform %q does not match active platform %q", sendPlatform, activePlatform), http.StatusConflict)
		return
	}

	chatID := s.ChatID
	if chatID == "" {
		http.Error(w, "chat_id is required", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	if err := r.platform.Send(ctx, chatID, s.Content); err != nil {
		log.Printf("[notify] Failed to send message to %s: %v", chatID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (r *Receiver) verifyAndRememberDelivery(w http.ResponseWriter, req *http.Request, path string) ([]byte, bool) {
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read request body: %v", err), http.StatusBadRequest)
		return nil, false
	}
	deliveryID := strings.TrimSpace(req.Header.Get("X-AgentForge-Delivery-Id"))
	timestamp := strings.TrimSpace(req.Header.Get("X-AgentForge-Delivery-Timestamp"))
	signature := strings.TrimSpace(req.Header.Get("X-AgentForge-Signature"))

	if r.sharedSecret != "" {
		if deliveryID == "" || timestamp == "" || signature == "" {
			http.Error(w, "missing signed delivery headers", http.StatusUnauthorized)
			return nil, false
		}
		if !verifyCompatibilitySignature(r.sharedSecret, req.Method, path, deliveryID, timestamp, bodyBytes, signature) {
			http.Error(w, "invalid delivery signature", http.StatusUnauthorized)
			return nil, false
		}
	}

	if deliveryID != "" {
		r.mu.Lock()
		if _, exists := r.processed[deliveryID]; exists {
			r.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "duplicate"})
			return nil, false
		}
		r.processed[deliveryID] = struct{}{}
		r.mu.Unlock()
	}

	return bodyBytes, true
}

func verifyCompatibilitySignature(secret, method, path, deliveryID, timestamp string, body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(strings.Join([]string{
		strings.TrimSpace(method),
		strings.TrimSpace(path),
		strings.TrimSpace(deliveryID),
		strings.TrimSpace(timestamp),
		string(body),
	}, "|")))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature)))
}

// ActionRequest is the payload for the /im/action endpoint.
type ActionRequest struct {
	Action   string `json:"action"`   // e.g. "assign-agent", "decompose"
	EntityID string `json:"entity_id"`
	ChatID   string `json:"chat_id"`
}

func (r *Receiver) handleAction(w http.ResponseWriter, req *http.Request) {
	var a ActionRequest
	if err := json.NewDecoder(req.Body).Decode(&a); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if a.Action == "" || a.EntityID == "" {
		http.Error(w, "action and entity_id are required", http.StatusBadRequest)
		return
	}

	if r.actionHandler == nil {
		http.Error(w, "action handler not configured", http.StatusServiceUnavailable)
		return
	}

	ctx := context.Background()
	result, err := r.actionHandler.HandleAction(ctx, a.Action, a.EntityID, a.ChatID)
	if err != nil {
		log.Printf("[notify] Action %s failed for %s: %v", a.Action, a.EntityID, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send the result back to the chat if chatID is provided.
	if a.ChatID != "" {
		_ = r.platform.Send(ctx, a.ChatID, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "result": result})
}
