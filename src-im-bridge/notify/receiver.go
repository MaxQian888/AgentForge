package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

// Receiver listens for notifications from the AgentForge backend and pushes them to IM.
type Receiver struct {
	platform core.Platform
	port     string
	server   *http.Server
}

// NewReceiver creates a notification receiver bound to a platform.
func NewReceiver(platform core.Platform, port string) *Receiver {
	return &Receiver{platform: platform, port: port}
}

// Start begins listening for notifications.
func (r *Receiver) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /im/notify", r.handleNotify)
	mux.HandleFunc("GET /im/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "platform": r.platform.Name()})
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
	var n Notification
	if err := json.NewDecoder(req.Body).Decode(&n); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	notificationPlatform := core.NormalizePlatformName(n.Platform)
	activePlatform := core.NormalizePlatformName(r.platform.Name())
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
	if n.Card != nil {
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
