package qchandler

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"

	"github.com/agentforge/server/internal/adsplatform"
	"github.com/agentforge/server/internal/secrets"
	qianchuanbinding "github.com/agentforge/server/plugins/qianchuan-ads/binding"
	qianchuan "github.com/agentforge/server/plugins/qianchuan-ads/oauth"
)

//go:embed templates/qianchuan_oauth_error.html
var oauthErrorTemplateFS embed.FS

var oauthErrorTmpl = template.Must(template.ParseFS(oauthErrorTemplateFS, "templates/qianchuan_oauth_error.html"))

// QianchuanOAuthSecretsService is the narrow contract for creating/rotating secrets.
type QianchuanOAuthSecretsService interface {
	CreateSecret(ctx context.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error)
	RotateSecret(ctx context.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error
}

// QianchuanOAuthBindingService creates a binding row after successful callback.
type QianchuanOAuthBindingService interface {
	Create(ctx context.Context, in qianchuanbinding.CreateInput) (*qianchuanbinding.Record, error)
}

// QianchuanOAuthAuditEmitter emits audit events for OAuth operations.
type QianchuanOAuthAuditEmitter interface {
	Emit(ctx context.Context, projectID, actorUserID, bindingID uuid.UUID, action, payload string)
}

// QianchuanOAuthHandler manages the OAuth bind flow endpoints.
type QianchuanOAuthHandler struct {
	States       *qianchuan.OAuthStateRepo
	Registry     *adsplatform.Registry
	Secrets      QianchuanOAuthSecretsService
	Bindings     QianchuanOAuthBindingService
	Audit        QianchuanOAuthAuditEmitter
	PublicBase   string // backend public URL for callbacks
	FEPublicBase string // frontend URL for redirect after callback
}

// --- Initiate ---

type initiateReq struct {
	DisplayName      *string    `json:"display_name,omitempty"`
	ActingEmployeeID *uuid.UUID `json:"acting_employee_id,omitempty"`
}

type initiateResp struct {
	AuthorizeURL string    `json:"authorize_url"`
	StateToken   uuid.UUID `json:"state_token"`
}

// Initiate handles POST /api/v1/projects/:pid/qianchuan/oauth/bind/initiate
func (h *QianchuanOAuthHandler) Initiate(c echo.Context) error {
	pid, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid project_id"})
	}
	actorID := actorUserID(c)

	var req initiateReq
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	state := &qianchuan.OAuthState{
		StateToken:       uuid.New(),
		ProjectID:        pid,
		RedirectURI:      strings.TrimRight(h.PublicBase, "/") + "/api/v1/qianchuan/oauth/callback",
		InitiatedBy:      actorID,
		DisplayName:      req.DisplayName,
		ActingEmployeeID: req.ActingEmployeeID,
	}
	if err := h.States.Create(c.Request().Context(), state); err != nil {
		log.WithError(err).Error("qianchuan oauth: create state")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal"})
	}

	provider, err := h.Registry.Resolve("qianchuan")
	if err != nil {
		log.WithError(err).Error("qianchuan oauth: resolve provider")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "provider_unavailable"})
	}

	authorizeURL, err := provider.OAuthAuthorizeURL(c.Request().Context(), state.StateToken.String(), state.RedirectURI)
	if err != nil {
		log.WithError(err).Error("qianchuan oauth: build authorize URL")
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal"})
	}

	return c.JSON(http.StatusOK, initiateResp{
		AuthorizeURL: authorizeURL,
		StateToken:   state.StateToken,
	})
}

// --- Callback ---

// Callback handles GET /api/v1/qianchuan/oauth/callback?code=...&state=...
func (h *QianchuanOAuthHandler) Callback(c echo.Context) error {
	ctx := c.Request().Context()
	code := c.QueryParam("code")
	stateStr := c.QueryParam("state")

	if code == "" || stateStr == "" {
		return h.renderError(c, http.StatusBadRequest, "missing_params", "Missing code or state parameter")
	}

	stateUUID, err := uuid.Parse(stateStr)
	if err != nil {
		return h.renderError(c, http.StatusBadRequest, "invalid_state", "OAuth state invalid or expired")
	}

	row, err := h.States.Lookup(ctx, stateUUID)
	if err != nil {
		switch err {
		case qianchuan.ErrStateNotFound, qianchuan.ErrStateExpired:
			return h.renderError(c, http.StatusBadRequest, "state_invalid", "OAuth state invalid or expired")
		case qianchuan.ErrStateConsumed:
			return h.renderError(c, http.StatusBadRequest, "state_consumed", "Bind already completed; please re-initiate")
		default:
			log.WithError(err).Error("qianchuan oauth callback: lookup state")
			return h.renderError(c, http.StatusInternalServerError, "internal", "Internal error")
		}
	}

	provider, err := h.Registry.Resolve("qianchuan")
	if err != nil {
		return h.renderError(c, http.StatusInternalServerError, "provider_unavailable", "Provider unavailable")
	}

	tokens, err := provider.OAuthExchange(ctx, code, row.RedirectURI)
	if err != nil {
		log.WithError(err).Warn("qianchuan oauth callback: exchange failed")
		return h.renderError(c, http.StatusBadGateway, "exchange_failed", "Qianchuan exchange failed; please retry")
	}

	// Extract advertiser ID
	if len(tokens.AdvertiserIDs) == 0 {
		return h.renderError(c, http.StatusBadRequest, "no_advertiser", "No advertiser granted in OAuth response")
	}
	if len(tokens.AdvertiserIDs) > 1 {
		return h.renderError(c, http.StatusBadRequest, "advertiser_ambiguous", "Multiple advertisers granted; bind one at a time")
	}
	advertiserID := tokens.AdvertiserIDs[0]

	// Persist secrets
	accessSecretName := fmt.Sprintf("qianchuan.%s.access_token", advertiserID)
	refreshSecretName := fmt.Sprintf("qianchuan.%s.refresh_token", advertiserID)

	if _, err := h.Secrets.CreateSecret(ctx, row.ProjectID, accessSecretName, tokens.AccessToken, "Qianchuan OAuth access token (auto-managed)", row.InitiatedBy); err != nil {
		// Fall back to rotate if already exists
		if rotErr := h.Secrets.RotateSecret(ctx, row.ProjectID, accessSecretName, tokens.AccessToken, row.InitiatedBy); rotErr != nil {
			log.WithError(rotErr).Error("qianchuan oauth callback: create/rotate access secret")
			return h.renderError(c, http.StatusInternalServerError, "secret_error", "Failed to persist tokens")
		}
	}
	if _, err := h.Secrets.CreateSecret(ctx, row.ProjectID, refreshSecretName, tokens.RefreshToken, "Qianchuan OAuth refresh token (auto-managed)", row.InitiatedBy); err != nil {
		if rotErr := h.Secrets.RotateSecret(ctx, row.ProjectID, refreshSecretName, tokens.RefreshToken, row.InitiatedBy); rotErr != nil {
			log.WithError(rotErr).Error("qianchuan oauth callback: create/rotate refresh secret")
			return h.renderError(c, http.StatusInternalServerError, "secret_error", "Failed to persist tokens")
		}
	}

	// Create binding
	displayName := fmt.Sprintf("Qianchuan %s", advertiserID)
	if row.DisplayName != nil && *row.DisplayName != "" {
		displayName = *row.DisplayName
	}

	expiresAt := tokens.ExpiresAt
	binding, err := h.Bindings.Create(ctx, qianchuanbinding.CreateInput{
		ProjectID:             row.ProjectID,
		AdvertiserID:          advertiserID,
		DisplayName:           displayName,
		ActingEmployeeID:      row.ActingEmployeeID,
		AccessTokenSecretRef:  accessSecretName,
		RefreshTokenSecretRef: refreshSecretName,
		CreatedBy:             row.InitiatedBy,
	})
	if err != nil {
		log.WithError(err).Error("qianchuan oauth callback: create binding")
		return h.renderError(c, http.StatusInternalServerError, "binding_error", "Failed to create binding")
	}
	// Update token_expires_at on the binding
	_ = expiresAt // Will be used once UpdateExpiry is wired; for now binding starts active

	// Mark state consumed
	if err := h.States.MarkConsumed(ctx, stateUUID); err != nil {
		log.WithError(err).Warn("qianchuan oauth callback: mark consumed")
	}

	// Audit
	if h.Audit != nil {
		h.Audit.Emit(ctx, row.ProjectID, row.InitiatedBy, binding.ID,
			"qianchuan.binding.oauth_completed",
			fmt.Sprintf(`{"advertiser_id":%q,"display_name":%q}`, advertiserID, displayName))
	}

	// Redirect to FE
	redirectTarget := fmt.Sprintf("%s/projects/%s/qianchuan/bindings?bind=success&advertiser=%s",
		strings.TrimRight(h.FEPublicBase, "/"), row.ProjectID.String(), advertiserID)
	return c.Redirect(http.StatusFound, redirectTarget)
}

// renderError renders the error HTML template.
func (h *QianchuanOAuthHandler) renderError(c echo.Context, status int, code, message string) error {
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(status)
	data := map[string]string{
		"Code":    code,
		"Message": message,
		"HomeURL": h.FEPublicBase,
	}
	return oauthErrorTmpl.Execute(c.Response().Writer, data)
}
