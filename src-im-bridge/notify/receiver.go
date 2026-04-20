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
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"strings"
	"sync"

	"github.com/agentforge/im-bridge/audit"
	"github.com/agentforge/im-bridge/core"
)

// DefaultSignatureSkew is the fallback window for timestamp skew checks
// when the bridge operator hasn't configured IM_SIGNATURE_SKEW_SECONDS.
const DefaultSignatureSkew = 5 * time.Minute

// DedupeStore is the subset of the durable state API the receiver needs
// for idempotency. core/state.Store satisfies this interface.
type DedupeStore interface {
	Seen(id, surface string, ttl time.Duration) (bool, error)
}

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
	reactionSink  ReactionSink
	sharedSecret  string
	mu            sync.Mutex
	processed     map[string]struct{} // L1 fallback when no DedupeStore is wired; authoritative is the store.
	dedupe        DedupeStore
	skew          time.Duration
	now           func() time.Time
	audit         audit.Writer
	auditSalt     string
	bridgeID      string
	staging       *StagingStore
}

// ReactionSink persists inbound reaction events (typically shipping them to
// the Go backend via POST /api/v1/im/reactions).
type ReactionSink interface {
	RecordReaction(ctx context.Context, event ReactionEvent) error
}

// ReactionEvent is the payload the bridge persists for an inbound reaction.
type ReactionEvent struct {
	Platform    string            `json:"platform"`
	ChatID      string            `json:"chat_id,omitempty"`
	MessageID   string            `json:"message_id,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	EmojiCode   string            `json:"emoji_code,omitempty"`
	RawEmoji    string            `json:"raw_emoji,omitempty"`
	ReactedAt   time.Time         `json:"reacted_at"`
	Removed     bool              `json:"removed,omitempty"`
	ReplyTarget *core.ReplyTarget `json:"reply_target,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type callbackHTTPProvider interface {
	HTTPCallbackHandler() http.Handler
}

type callbackPathProvider interface {
	CallbackPaths() []string
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
		skew:      DefaultSignatureSkew,
		now:       time.Now,
	}
}

// SetDedupeStore wires a durable dedupe backend. When set, the in-memory
// processed map is bypassed and the store is the source of truth.
func (r *Receiver) SetDedupeStore(store DedupeStore) {
	r.dedupe = store
}

// SetSignatureSkew configures the timestamp window around
// X-AgentForge-Delivery-Timestamp. Values <= 0 fall back to DefaultSignatureSkew.
func (r *Receiver) SetSignatureSkew(skew time.Duration) {
	if skew <= 0 {
		skew = DefaultSignatureSkew
	}
	r.skew = skew
}

// setNow lets tests inject a deterministic clock.
func (r *Receiver) setNow(fn func() time.Time) { r.now = fn }

// SetAuditWriter wires a structured audit log writer. Pass nil (or omit
// this setter) to disable audit emission entirely.
func (r *Receiver) SetAuditWriter(writer audit.Writer, salt string) {
	r.audit = writer
	r.auditSalt = salt
}

// SetBridgeID attaches the stable bridge_id emitted in audit events.
func (r *Receiver) SetBridgeID(id string) { r.bridgeID = strings.TrimSpace(id) }

func (r *Receiver) emitAudit(ev audit.Event) {
	if r.audit == nil {
		return
	}
	if ev.V == 0 {
		ev.V = audit.SchemaVersion
	}
	if ev.Ts.IsZero() {
		ev.Ts = r.now().UTC()
	}
	if ev.Platform == "" {
		ev.Platform = r.metadata.Source
	}
	if ev.BridgeID == "" {
		ev.BridgeID = r.bridgeID
	}
	if err := r.audit.Emit(ev); err != nil {
		log.WithField("component", "notify").WithError(err).Warn("audit emit failed")
	}
}

func (r *Receiver) emitDelivered(surface, deliveryID, action, chatID, userID string, receipt core.DeliveryReceipt, start time.Time) {
	r.emitDeliveredTenant(surface, deliveryID, action, "", chatID, userID, receipt, start)
}

func (r *Receiver) emitDeliveredTenant(surface, deliveryID, action, tenantID, chatID, userID string, receipt core.DeliveryReceipt, start time.Time) {
	r.emitAudit(audit.Event{
		Direction:      audit.DirectionEgress,
		Surface:        surface,
		DeliveryID:     deliveryID,
		Action:         action,
		TenantID:       tenantID,
		Status:         audit.StatusDelivered,
		DeliveryMethod: string(receipt.Method),
		FallbackReason: receipt.FallbackReason,
		ChatIDHash:     audit.HashID(r.auditSalt, chatID),
		UserIDHash:     audit.HashID(r.auditSalt, userID),
		LatencyMs:      r.now().Sub(start).Milliseconds(),
	})
}

func (r *Receiver) emitFailed(surface, deliveryID, action, chatID, userID, reason string, start time.Time) {
	r.emitFailedTenant(surface, deliveryID, action, "", chatID, userID, reason, start)
}

func (r *Receiver) emitFailedTenant(surface, deliveryID, action, tenantID, chatID, userID, reason string, start time.Time) {
	r.emitAudit(audit.Event{
		Direction:      audit.DirectionEgress,
		Surface:        surface,
		DeliveryID:     deliveryID,
		Action:         action,
		TenantID:       tenantID,
		Status:         audit.StatusFailed,
		FallbackReason: reason,
		ChatIDHash:     audit.HashID(r.auditSalt, chatID),
		UserIDHash:     audit.HashID(r.auditSalt, userID),
		LatencyMs:      r.now().Sub(start).Milliseconds(),
	})
}

// SetActionHandler sets the handler for card button actions.
func (r *Receiver) SetActionHandler(h ActionHandler) {
	r.actionHandler = h
}

// SetStagingStore wires the staging dir used for inbound attachments. Pass nil
// to treat attachments as unsupported (receiver rejects attachment payloads).
func (r *Receiver) SetStagingStore(store *StagingStore) {
	r.staging = store
}

// SetReactionSink wires the sink that records inbound reaction events.
func (r *Receiver) SetReactionSink(sink ReactionSink) {
	r.reactionSink = sink
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
	mux.HandleFunc("POST /im/attachments", r.handleUploadAttachment)
	mux.HandleFunc("POST /im/reactions", r.handleReactionIngress)
	mux.HandleFunc("GET /im/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stagingDir := ""
		if r.staging != nil {
			stagingDir = r.staging.Dir()
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":                 "ok",
			"platform":               r.platform.Name(),
			"source":                 r.metadata.Source,
			"readiness_tier":         r.metadata.Capabilities.ReadinessTier,
			"supports_rich_messages": r.metadata.Capabilities.SupportsRichMessages,
			"supports_attachments":   r.metadata.Capabilities.SupportsAttachments,
			"supports_reactions":     r.metadata.Capabilities.SupportsReactions,
			"supports_threads":       r.metadata.Capabilities.SupportsThreads,
			"staging_dir":            stagingDir,
			"capability_matrix":      r.metadata.Capabilities.Matrix(),
		})
	})
	r.mountPlatformCallbacks(mux)

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

func (r *Receiver) mountPlatformCallbacks(mux *http.ServeMux) {
	if mux == nil || r == nil || r.platform == nil {
		return
	}
	handlerProvider, ok := r.platform.(callbackHTTPProvider)
	if !ok {
		return
	}
	handler := handlerProvider.HTTPCallbackHandler()
	if handler == nil {
		return
	}
	pathProvider, ok := r.platform.(callbackPathProvider)
	if !ok {
		return
	}
	for _, rawPath := range pathProvider.CallbackPaths() {
		path := strings.TrimSpace(rawPath)
		if path == "" {
			continue
		}
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		mux.Handle(path, handler)
	}
}

// Stop gracefully shuts down the notification receiver.
func (r *Receiver) Stop() error {
	if r.server != nil {
		return r.server.Shutdown(context.Background())
	}
	return nil
}

func (r *Receiver) handleNotify(w http.ResponseWriter, req *http.Request) {
	start := r.now()
	deliveryID := strings.TrimSpace(req.Header.Get("X-AgentForge-Delivery-Id"))
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
			r.emitFailed("/im/notify", deliveryID, n.Type, chatID, n.TargetIMUserID, err.Error(), start)
			return
		}
		writeDeliveryReceipt(w, receipt)
		r.emitDelivered("/im/notify", deliveryID, n.Type, chatID, n.TargetIMUserID, receipt, start)
		return
	}

	// Try card first if available.
	if n.Card != nil && r.metadata.Capabilities.SupportsRichMessages {
		if _, ok := r.platform.(core.CardSender); ok {
			if _, err := core.DeliverCard(ctx, r.platform, r.metadata, n.ReplyTarget, chatID, n.Card); err != nil {
				log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send card")
				http.Error(w, err.Error(), http.StatusInternalServerError)
				r.emitFailed("/im/notify", deliveryID, n.Type, chatID, n.TargetIMUserID, err.Error(), start)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "sent", "type": "card"})
			r.emitDelivered("/im/notify", deliveryID, n.Type, chatID, n.TargetIMUserID, core.DeliveryReceipt{Type: "card"}, start)
			return
		}
	}

	// Fallback to plain text.
	if _, err := core.DeliverText(ctx, r.platform, r.metadata, n.ReplyTarget, chatID, n.Content); err != nil {
		log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send message")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		r.emitFailed("/im/notify", deliveryID, n.Type, chatID, n.TargetIMUserID, err.Error(), start)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "sent", "type": "text"})
	r.emitDelivered("/im/notify", deliveryID, n.Type, chatID, n.TargetIMUserID, core.DeliveryReceipt{Type: "text"}, start)
}

// SendRequest is the payload for the /im/send endpoint. Exactly one of
// `content`, `structured`, `native`, or `card` may be set; mixing these
// fields (or combining `card` with attachments) returns 400.
type SendRequest struct {
	Platform    string                   `json:"platform"`
	ChatID      string                   `json:"chat_id"`
	Content     string                   `json:"content"`
	Structured  *core.StructuredMessage  `json:"structured,omitempty"`
	Native      *core.NativeMessage      `json:"native,omitempty"`
	Card        *core.ProviderNeutralCard `json:"card,omitempty"`
	Attachments []AttachmentRequest      `json:"attachments,omitempty"`
	Metadata    map[string]string        `json:"metadata,omitempty"`
	ThreadID    string                   `json:"thread_id,omitempty"`
	ReplyTarget *core.ReplyTarget        `json:"replyTarget,omitempty"`
}

// AttachmentRequest is the wire shape for an egress attachment reference. The
// caller supplies one of: a staged_id (materialized via POST /im/attachments),
// a raw base64 blob, or a remote URL. The receiver resolves each to a
// ContentRef before delivery.
type AttachmentRequest struct {
	ID         string            `json:"id,omitempty"`
	StagedID   string            `json:"staged_id,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	MimeType   string            `json:"mime_type,omitempty"`
	Filename   string            `json:"filename,omitempty"`
	SizeBytes  int64             `json:"size_bytes,omitempty"`
	ContentRef string            `json:"content_ref,omitempty"`
	URL        string            `json:"url,omitempty"`
	DataBase64 string            `json:"data_base64,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

func (r *Receiver) handleSend(w http.ResponseWriter, req *http.Request) {
	start := r.now()
	deliveryID := strings.TrimSpace(req.Header.Get("X-AgentForge-Delivery-Id"))
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

	// Card path: spec §8 ProviderNeutralCard. Mutually exclusive with the
	// content/structured/native/attachments envelope.
	if s.Card != nil {
		if s.Content != "" || s.Structured != nil || s.Native != nil || len(s.Attachments) > 0 {
			http.Error(w, "card cannot be combined with content/structured/native/attachments", http.StatusBadRequest)
			r.emitFailed("/im/send", deliveryID, "send", chatID, "", "exclusive_card", start)
			return
		}
		target := s.ReplyTarget
		if target == nil {
			target = &core.ReplyTarget{Platform: r.metadata.Source, ChatID: chatID}
		} else if strings.TrimSpace(target.Platform) == "" {
			target.Platform = r.metadata.Source
		}
		rendered, err := core.DispatchCard(*s.Card, target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			r.emitFailed("/im/send", deliveryID, "send", chatID, "", err.Error(), start)
			return
		}
		sender, ok := r.platform.(core.RawCardSender)
		if !ok {
			http.Error(w, "platform does not support raw card delivery", http.StatusNotImplemented)
			r.emitFailed("/im/send", deliveryID, "send", chatID, "", "raw_card_unsupported", start)
			return
		}
		if err := sender.SendRawCard(ctx, chatID, rendered.ContentType, rendered.Body, target); err != nil {
			log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send card")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			r.emitFailed("/im/send", deliveryID, "send", chatID, "", err.Error(), start)
			return
		}
		receipt := core.DeliveryReceipt{Type: "card"}
		writeDeliveryReceipt(w, receipt)
		r.emitDelivered("/im/send", deliveryID, "send", chatID, "", receipt, start)
		return
	}

	attachments, attErr := r.resolveAttachments(s.Attachments)
	if attErr != nil {
		http.Error(w, attErr.Error(), http.StatusBadRequest)
		r.emitFailed("/im/send", deliveryID, "send", chatID, "", attErr.Error(), start)
		return
	}

	receipt, err := core.DeliverEnvelope(ctx, r.platform, r.metadata, chatID, &core.DeliveryEnvelope{
		Content:     s.Content,
		Structured:  s.Structured,
		Native:      s.Native,
		Attachments: attachments,
		ReplyTarget: s.ReplyTarget,
		Metadata:    s.Metadata,
	})
	if err != nil {
		log.WithField("component", "notify").WithField("chat_id", chatID).WithError(err).Error("Failed to send message")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		r.emitFailed("/im/send", deliveryID, "send", chatID, "", err.Error(), start)
		return
	}

	writeDeliveryReceipt(w, receipt)
	r.emitDelivered("/im/send", deliveryID, "send", chatID, "", receipt, start)
}

// rejectReason enumerates the classified reasons the receiver may refuse a
// signed compatibility delivery. These values are stable and appear in the
// error response body so the backend can branch retry policy on them.
type rejectReason string

const (
	reasonInvalidSignature     rejectReason = "invalid_signature"
	reasonTimestampOutOfWindow rejectReason = "timestamp_out_of_window"
	reasonDuplicateDelivery    rejectReason = "duplicate_delivery"
	reasonMissingHeaders       rejectReason = "missing_signed_delivery_headers"
)

func writeRejection(w http.ResponseWriter, status int, reason rejectReason, retryable bool) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":     string(reason),
		"retryable": retryable,
	})
}

func (r *Receiver) verifyAndRememberDelivery(w http.ResponseWriter, req *http.Request, path string) ([]byte, bool) {
	start := r.now()
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read request body: %v", err), http.StatusBadRequest)
		return nil, false
	}
	deliveryID := strings.TrimSpace(req.Header.Get("X-AgentForge-Delivery-Id"))
	timestamp := strings.TrimSpace(req.Header.Get("X-AgentForge-Delivery-Timestamp"))
	signature := strings.TrimSpace(req.Header.Get("X-AgentForge-Signature"))
	signatureSource := "unsigned"
	if r.sharedSecret != "" {
		signatureSource = "shared_secret"
	}

	rejectAudit := func(reason rejectReason) {
		r.emitAudit(audit.Event{
			Direction:       audit.DirectionIngress,
			Surface:         path,
			DeliveryID:      deliveryID,
			Status:          audit.StatusRejected,
			LatencyMs:       r.now().Sub(start).Milliseconds(),
			SignatureSource: signatureSource,
			Metadata:        map[string]string{"reason": string(reason)},
		})
	}

	// 1. HMAC verification (when shared secret configured).
	if r.sharedSecret != "" {
		if deliveryID == "" || timestamp == "" || signature == "" {
			writeRejection(w, http.StatusUnauthorized, reasonMissingHeaders, false)
			rejectAudit(reasonMissingHeaders)
			return nil, false
		}
		if !verifyCompatibilitySignature(r.sharedSecret, req.Method, path, deliveryID, timestamp, bodyBytes, signature) {
			writeRejection(w, http.StatusUnauthorized, reasonInvalidSignature, false)
			rejectAudit(reasonInvalidSignature)
			return nil, false
		}

		// 2. Timestamp skew window. A captured-but-valid signature is only
		// honored within the configured window; retrying with the same
		// timestamp outside the window cannot succeed, hence retryable=false.
		if r.skew > 0 {
			ts, ok := parseDeliveryTimestamp(timestamp)
			if !ok {
				writeRejection(w, http.StatusUnauthorized, reasonInvalidSignature, false)
				rejectAudit(reasonInvalidSignature)
				return nil, false
			}
			delta := r.now().Sub(ts)
			if delta < 0 {
				delta = -delta
			}
			if delta > r.skew {
				writeRejection(w, http.StatusRequestTimeout, reasonTimestampOutOfWindow, false)
				rejectAudit(reasonTimestampOutOfWindow)
				return nil, false
			}
		}
	}

	// 3. Idempotency (durable store preferred, in-memory fallback).
	if deliveryID != "" {
		if r.dedupe != nil {
			ttl := r.skew + time.Minute // align with design: skew + 60s grace
			if ttl <= 0 {
				ttl = DefaultSignatureSkew + time.Minute
			}
			duplicate, err := r.dedupe.Seen(deliveryID, path, ttl)
			if err != nil {
				log.WithField("component", "notify").WithError(err).Error("durable dedupe failed")
				http.Error(w, "dedupe store unavailable", http.StatusServiceUnavailable)
				return nil, false
			}
			if duplicate {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status":    "duplicate",
					"error":     string(reasonDuplicateDelivery),
					"retryable": false,
				})
				r.emitAudit(audit.Event{
					Direction:       audit.DirectionIngress,
					Surface:         path,
					DeliveryID:      deliveryID,
					Status:          audit.StatusDuplicate,
					LatencyMs:       r.now().Sub(start).Milliseconds(),
					SignatureSource: signatureSource,
				})
				return nil, false
			}
		} else {
			r.mu.Lock()
			if _, exists := r.processed[deliveryID]; exists {
				r.mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]string{"status": "duplicate"})
				r.emitAudit(audit.Event{
					Direction:       audit.DirectionIngress,
					Surface:         path,
					DeliveryID:      deliveryID,
					Status:          audit.StatusDuplicate,
					LatencyMs:       r.now().Sub(start).Milliseconds(),
					SignatureSource: signatureSource,
				})
				return nil, false
			}
			r.processed[deliveryID] = struct{}{}
			r.mu.Unlock()
		}
	}

	return bodyBytes, true
}

// parseDeliveryTimestamp accepts either a Unix-seconds integer or an
// RFC3339 string and returns the parsed time.Time. Both forms are in use:
// Unix seconds is the canonical form for new signers, RFC3339 is the form
// the existing backend emitter and tests still use.
func parseDeliveryTimestamp(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(n, 0), true
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, true
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t, true
	}
	return time.Time{}, false
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
	start := r.now()
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
		r.emitAudit(audit.Event{
			Direction:  audit.DirectionAction,
			Surface:    "/im/action",
			Action:     a.Action,
			Status:     audit.StatusFailed,
			ChatIDHash: audit.HashID(r.auditSalt, a.ChatID),
			UserIDHash: audit.HashID(r.auditSalt, a.UserID),
			LatencyMs:  r.now().Sub(start).Milliseconds(),
			Metadata:   map[string]string{"entity_id": a.EntityID, "error": err.Error()},
		})
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

	r.emitAudit(audit.Event{
		Direction:  audit.DirectionAction,
		Surface:    "/im/action",
		Action:     a.Action,
		Status:     audit.StatusDelivered,
		ChatIDHash: audit.HashID(r.auditSalt, a.ChatID),
		UserIDHash: audit.HashID(r.auditSalt, a.UserID),
		LatencyMs:  r.now().Sub(start).Milliseconds(),
		Metadata:   map[string]string{"entity_id": a.EntityID},
	})
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
