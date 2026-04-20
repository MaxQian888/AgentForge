package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"

	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

// --- Dependency interfaces for VCS webhook handler (all injected) ---

// IntegrationResolver finds an integration row by (host, owner, repo).
type IntegrationResolver interface {
	ResolveByRepo(ctx context.Context, host, owner, repo string) (*model.VCSIntegration, error)
}

// WebhookSecretsResolver resolves a secret by project + name.
type WebhookSecretsResolver interface {
	Resolve(ctx context.Context, projectID uuid.UUID, name string) (string, error)
}

// WebhookRouter dispatches a verified webhook event to the appropriate handler.
type WebhookRouter interface {
	RouteEvent(ctx context.Context, integ *model.VCSIntegration, eventType, deliveryID string, body []byte) error
}

// WebhookEventsWriter persists inbound webhook events for dedup/audit.
type WebhookEventsWriter interface {
	Insert(ctx context.Context, e *model.VCSWebhookEvent) error
	MarkProcessed(ctx context.Context, id uuid.UUID, procErr string) error
}

// WebhookAuditRecorder is an optional audit emitter.
type WebhookAuditRecorder interface {
	RecordEvent(ctx context.Context, e *model.AuditEvent) error
}

// VCSWebhookHandler handles inbound GitHub webhook deliveries.
type VCSWebhookHandler struct {
	integrations IntegrationResolver
	secrets      WebhookSecretsResolver
	router       WebhookRouter
	events       WebhookEventsWriter
	audit        WebhookAuditRecorder
}

func NewVCSWebhookHandler(
	i IntegrationResolver,
	s WebhookSecretsResolver,
	r WebhookRouter,
	ev WebhookEventsWriter,
	a WebhookAuditRecorder,
) *VCSWebhookHandler {
	return &VCSWebhookHandler{integrations: i, secrets: s, router: r, events: ev, audit: a}
}

type repoRefPayload struct {
	Repository struct {
		Owner struct{ Login string `json:"login"` } `json:"owner"`
		Name  string `json:"name"`
	} `json:"repository"`
}

// HandleGitHubWebhook is the echo handler for POST /api/v1/vcs/github/webhook.
func (h *VCSWebhookHandler) HandleGitHubWebhook(c echo.Context) error {
	ctx := c.Request().Context()

	raw, _ := c.Get(appMiddleware.RawBodyKey).([]byte)
	if len(raw) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "vcs:webhook_empty_body"})
	}
	sig := strings.TrimSpace(c.Request().Header.Get("X-Hub-Signature-256"))
	eventType := strings.TrimSpace(c.Request().Header.Get("X-GitHub-Event"))
	deliveryID := strings.TrimSpace(c.Request().Header.Get("X-GitHub-Delivery"))
	if sig == "" || deliveryID == "" || eventType == "" {
		h.recordSignatureInvalid(ctx, "", "missing_headers")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:webhook_signature_invalid"})
	}

	// Parse repo coordinates from payload.
	var ref repoRefPayload
	_ = json.Unmarshal(raw, &ref)
	host := "github.com"
	owner := ref.Repository.Owner.Login
	repo := ref.Repository.Name
	if owner == "" || repo == "" {
		// ping / installation / etc. — accept silently.
		log.WithField("event", eventType).Debug("vcs_webhook: no repo in payload, accepting noop")
		return c.NoContent(http.StatusAccepted)
	}

	integ, err := h.integrations.ResolveByRepo(ctx, host, owner, repo)
	if err != nil {
		if errors.Is(err, repository.ErrVCSIntegrationNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "vcs:integration_not_found"})
		}
		log.WithError(err).Warn("vcs_webhook: resolve integration")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "vcs:integration_lookup_failed"})
	}

	secret, err := h.secrets.Resolve(ctx, integ.ProjectID, integ.WebhookSecretRef)
	if err != nil || secret == "" {
		log.WithError(err).Warn("vcs_webhook: resolve webhook secret")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:webhook_signature_invalid"})
	}

	if !verifyGitHubSignature([]byte(secret), raw, sig) {
		h.recordSignatureInvalid(ctx, integ.ID.String(), "hmac_mismatch")
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:webhook_signature_invalid"})
	}

	// Dedup: attempt insert; UNIQUE constraint rejects replays.
	sum := sha256.Sum256(raw)
	ev := &model.VCSWebhookEvent{
		ID:            uuid.New(),
		IntegrationID: integ.ID,
		EventID:       deliveryID,
		EventType:     eventType,
		PayloadHash:   sum[:],
	}
	if err := h.events.Insert(ctx, ev); err != nil {
		if errors.Is(err, repository.ErrVCSWebhookEventDuplicate) {
			return c.JSON(http.StatusOK, map[string]string{"status": "duplicate"})
		}
		log.WithError(err).Warn("vcs_webhook: persist event")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "vcs:event_persist_failed"})
	}
	h.recordReceived(ctx, integ.ID.String(), eventType, deliveryID)

	// Route to downstream handler.
	if err := h.router.RouteEvent(ctx, integ, eventType, deliveryID, raw); err != nil {
		if errors.Is(err, service.ErrPushHandlerNotImplemented) {
			// Plan 2C will plug in. Accept now so GitHub doesn't retry.
			_ = h.events.MarkProcessed(ctx, ev.ID, "push_handler_pending_plan_2c")
			return c.NoContent(http.StatusAccepted)
		}
		log.WithError(err).WithField("event", eventType).Warn("vcs_webhook: route")
		_ = h.events.MarkProcessed(ctx, ev.ID, err.Error())
		return c.JSON(http.StatusAccepted, map[string]string{"status": "routed_with_error"})
	}
	_ = h.events.MarkProcessed(ctx, ev.ID, "")
	return c.NoContent(http.StatusAccepted)
}

// verifyGitHubSignature performs constant-time HMAC-SHA256 comparison.
func verifyGitHubSignature(secret, body []byte, headerVal string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(headerVal, prefix) {
		return false
	}
	expected, err := hex.DecodeString(strings.TrimPrefix(headerVal, prefix))
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return hmac.Equal(expected, m.Sum(nil))
}

func (h *VCSWebhookHandler) recordSignatureInvalid(ctx context.Context, integID, reason string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.RecordEvent(ctx, &model.AuditEvent{
		ActionID:     "vcs.webhook_signature_invalid",
		ResourceType: model.AuditResourceTypeVCSIntegration,
		ResourceID:   integID,
		PayloadSnapshotJSON: mustJSON(map[string]any{"reason": reason}),
		SystemInitiated: true,
	})
}

func (h *VCSWebhookHandler) recordReceived(ctx context.Context, integID, eventType, deliveryID string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.RecordEvent(ctx, &model.AuditEvent{
		ActionID:     "vcs.webhook_received",
		ResourceType: model.AuditResourceTypeVCSIntegration,
		ResourceID:   integID,
		PayloadSnapshotJSON: mustJSON(map[string]any{"event_type": eventType, "delivery_id": deliveryID}),
		SystemInitiated: true,
	})
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
