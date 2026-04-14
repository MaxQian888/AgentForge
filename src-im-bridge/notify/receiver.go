package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
	"strings"
	"sync"

	"github.com/agentforge/im-bridge/core"
)

// Notification is the payload sent from the Go backend.
type Notification struct {
	Type           string                  `json:"type"` // e.g., "task_completed", "agent_finished", "cost_alert"
	TargetIMUserID string                  `json:"target_im_user_id"`
	TargetChatID   string                  `json:"target_chat_id"`
	Platform       string                  `json:"platform"` // "feishu", "slack", etc.
	Content        string                  `json:"content"`  // plain text fallback
	Card           *core.Card              `json:"card"`     // optional rich card
	Structured     *core.StructuredMessage `json:"structured,omitempty"`
	Native         *core.NativeMessage     `json:"native,omitempty"`
	Metadata       map[string]string       `json:"metadata,omitempty"`
	ReplyTarget    *core.ReplyTarget       `json:"replyTarget,omitempty"`
}

// ActionHandler processes button click actions from IM cards.
type ActionHandler interface {
	HandleAction(ctx context.Context, req *ActionRequest) (*ActionResponse, error)
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
	return NewReceiverWithMetadata(platform, core.MetadataForPlatform(platform), port)
}

// NewReceiverWithMetadata creates a notification receiver bound to a platform
// and explicit provider metadata.
func NewReceiverWithMetadata(platform core.Platform, metadata core.PlatformMetadata, port string) *Receiver {
	if metadata.Source == "" {
		metadata = core.MetadataForPlatform(platform)
	}
	return &Receiver{
		platform:  platform,
		metadata:  metadata,
		port:      port,
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
			"readiness_tier":         r.metadata.Capabilities.ReadinessTier,
			"supports_rich_messages": r.metadata.Capabilities.SupportsRichMessages,
			"capability_matrix":      r.metadata.Capabilities.Matrix(),
		})
	})

	r.server = &http.Server{
		Addr:    ":" + r.port,
		Handler: mux,
	}

	log.WithFields(log.Fields{"component": "notify", "port": r.port}).Info("Notification receiver starting")
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
	if n.Native != nil && strings.TrimSpace(n.Native.Platform) == "" {
		n.Native.Platform = activePlatform
	}
	if n.Native != nil || n.Structured != nil {
		receipt, err := core.DeliverEnvelope(ctx, r.platform, r.metadata, chatID, &core.DeliveryEnvelope{
			Content:     n.Content,
			Structured:  n.Structured,
			Native:      n.Native,
			ReplyTarget: n.ReplyTarget,
			Metadata:    n.Metadata,
		})
		if err != nil {
			log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send typed payload")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeDeliveryReceipt(w, receipt)
		return
	}

	// Try card first if available.
	if n.Card != nil && r.metadata.Capabilities.SupportsRichMessages {
		if _, ok := r.platform.(core.CardSender); ok {
			if _, err := core.DeliverCard(ctx, r.platform, r.metadata, n.ReplyTarget, chatID, n.Card); err != nil {
				log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send card")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "sent", "type": "card"})
			return
		}
	}

	// Fallback to plain text.
	if _, err := core.DeliverText(ctx, r.platform, r.metadata, n.ReplyTarget, chatID, n.Content); err != nil {
		log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send message")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent", "type": "text"})
}

// SendRequest is the payload for the /im/send endpoint.
type SendRequest struct {
	Platform    string                  `json:"platform"`
	ChatID      string                  `json:"chat_id"`
	Content     string                  `json:"content"`
	Structured  *core.StructuredMessage `json:"structured,omitempty"`
	Native      *core.NativeMessage     `json:"native,omitempty"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	ThreadID    string                  `json:"thread_id,omitempty"`
	ReplyTarget *core.ReplyTarget       `json:"replyTarget,omitempty"`
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
	receipt, err := core.DeliverEnvelope(ctx, r.platform, r.metadata, chatID, &core.DeliveryEnvelope{
		Content:     s.Content,
		Structured:  s.Structured,
		Native:      s.Native,
		ReplyTarget: s.ReplyTarget,
		Metadata:    s.Metadata,
	})
	if err != nil {
		log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send message")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeDeliveryReceipt(w, receipt)
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
	Platform    string            `json:"platform,omitempty"`
	Action      string            `json:"action"` // e.g. "assign-agent", "decompose"
	EntityID    string            `json:"entity_id"`
	ChatID      string            `json:"chat_id"`
	UserID      string            `json:"user_id,omitempty"`
	BridgeID    string            `json:"bridge_id,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"replyTarget,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ActionResponse is the normalized result of an interactive action callback.
type ActionResponse struct {
	Result      string                  `json:"result"`
	ReplyTarget *core.ReplyTarget       `json:"replyTarget,omitempty"`
	Metadata    map[string]string       `json:"metadata,omitempty"`
	Structured  *core.StructuredMessage `json:"structured,omitempty"`
	Native      *core.NativeMessage     `json:"native,omitempty"`
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
	result, err := r.actionHandler.HandleAction(ctx, &a)
	if err != nil {
		log.WithFields(log.Fields{"component": "notify", "action": a.Action, "entity_id": a.EntityID}).WithError(err).Error("Action failed")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if result == nil {
		result = &ActionResponse{}
	}

	// Send the result back to the chat if chatID is provided.
	if a.ChatID != "" && (strings.TrimSpace(result.Result) != "" || result.Native != nil || result.Structured != nil) {
		target := a.ReplyTarget
		if result.ReplyTarget != nil {
			target = result.ReplyTarget
		}
		actionMetadata := cloneMetadata(result.Metadata)
		nativeMessage, fallbackReason := r.resolveActionNativeMessage(result, target)
		if fallbackReason != "" {
			actionMetadata["fallback_reason"] = fallbackReason
		}

		receipt, err := core.DeliverEnvelope(ctx, r.platform, r.metadata, a.ChatID, &core.DeliveryEnvelope{
			Content:     result.Result,
			Structured:  result.Structured,
			Native:      nativeMessage,
			ReplyTarget: target,
			Metadata:    actionMetadata,
		})
		if receipt.FallbackReason != "" && strings.TrimSpace(actionMetadata["fallback_reason"]) == "" {
			actionMetadata["fallback_reason"] = receipt.FallbackReason
		}
		if err != nil {
			log.WithField("component", "notify").WithField("entity_id", a.EntityID).WithError(err).Error("Action response delivery failed")
		}
		result.Metadata = actionMetadata
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":      "ok",
		"result":      result.Result,
		"replyTarget": result.ReplyTarget,
		"metadata":    result.Metadata,
	})
}

func (r *Receiver) resolveActionNativeMessage(result *ActionResponse, target *core.ReplyTarget) (*core.NativeMessage, string) {
	if result == nil {
		return nil, ""
	}
	if result.Native != nil {
		if strings.TrimSpace(result.Native.Platform) == "" {
			result.Native.Platform = r.metadata.Source
		}
		return result.Native, ""
	}
	if r.metadata.Source != "feishu" || target == nil {
		return nil, ""
	}
	if strings.TrimSpace(target.ProgressMode) != string(core.AsyncUpdateDeferredCardUpdate) {
		return nil, ""
	}
	if strings.TrimSpace(target.CallbackToken) == "" {
		return nil, "missing_delayed_update_context"
	}
	if strings.TrimSpace(result.Result) == "" {
		return nil, ""
	}

	builder, ok := r.platform.(core.NativeTextMessageBuilder)
	if ok {
		message, err := builder.BuildNativeTextMessage("AgentForge Update", strings.TrimSpace(result.Result))
		if err != nil {
			return nil, "native_payload_encode_failed"
		}
		return message, ""
	}
	message, err := core.NewFeishuMarkdownCardMessage("AgentForge Update", strings.TrimSpace(result.Result))
	if err != nil {
		return nil, "native_payload_encode_failed"
	}
	return message, ""
}

func classifyNativeFallbackReason(err error, target *core.ReplyTarget) string {
	if target == nil || strings.TrimSpace(target.CallbackToken) == "" {
		return "missing_delayed_update_context"
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(message, "expired"):
		return "delayed_update_context_expired"
	case strings.Contains(message, "exhaust") || strings.Contains(message, "used"):
		return "delayed_update_context_exhausted"
	case strings.Contains(message, "invalid") || strings.Contains(message, "token"):
		return "invalid_delayed_update_context"
	default:
		return "native_update_failed"
	}
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return make(map[string]string)
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

func writeDeliveryReceipt(w http.ResponseWriter, receipt core.DeliveryReceipt) {
	w.Header().Set("Content-Type", "application/json")
	if fallbackReason := strings.TrimSpace(receipt.FallbackReason); fallbackReason != "" {
		w.Header().Set("X-IM-Downgrade-Reason", fallbackReason)
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":          "sent",
		"type":            receipt.Type,
		"delivery_method": receipt.Method,
		"fallback_reason": strings.TrimSpace(receipt.FallbackReason),
	})
}
