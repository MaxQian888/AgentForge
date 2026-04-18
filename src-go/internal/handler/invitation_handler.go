package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

// InvitationHandler fronts the `project_invitations` lifecycle:
//   * project-scoped: Create, List, Revoke, Resend (admin+)
//   * top-level:       GetByToken (public), Accept (auth), Decline (open)
type InvitationHandler struct {
	svc *service.InvitationService
}

func NewInvitationHandler(svc *service.InvitationService) *InvitationHandler {
	return &InvitationHandler{svc: svc}
}

// Create handles POST /projects/:pid/invitations.
func (h *InvitationHandler) Create(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateInvitation)
	}
	req := new(model.CreateInvitationRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if err := req.InvitedIdentity.Validate(); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvitationInvalidIdentity)
	}

	actorID, err := claimsUserID(c)
	if err != nil || actorID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	projectID := appMiddleware.GetProjectID(c)

	var expiresAt time.Time
	if trimmed := strings.TrimSpace(req.ExpiresAt); trimmed != "" {
		parsed, err := time.Parse(time.RFC3339, trimmed)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		expiresAt = parsed
	}

	result, err := h.svc.Create(c.Request().Context(), service.CreateInput{
		ProjectID:       projectID,
		InviterUserID:   *actorID,
		InvitedIdentity: req.InvitedIdentity,
		ProjectRole:     req.ProjectRole,
		Message:         req.Message,
		ExpiresAt:       expiresAt,
		RequestID:       c.Response().Header().Get(echo.HeaderXRequestID),
		IP:              c.RealIP(),
		UserAgent:       c.Request().UserAgent(),
	})
	if err != nil {
		return h.mapError(c, err, i18n.MsgFailedToCreateInvitation)
	}
	return c.JSON(http.StatusCreated, model.InvitationCreateResponse{
		Invitation:  result.Invitation.ToDTO(),
		AcceptToken: result.PlaintextToken,
		AcceptURL:   result.AcceptURL,
	})
}

// List handles GET /projects/:pid/invitations?status=<filter>.
func (h *InvitationHandler) List(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListInvitations)
	}
	projectID := appMiddleware.GetProjectID(c)
	invitations, err := h.svc.List(c.Request().Context(), projectID, c.QueryParam("status"))
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListInvitations)
	}
	out := make([]model.InvitationDTO, 0, len(invitations))
	for _, inv := range invitations {
		out = append(out, inv.ToDTO())
	}
	return c.JSON(http.StatusOK, out)
}

// Revoke handles POST /projects/:pid/invitations/:id/revoke.
func (h *InvitationHandler) Revoke(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToRevokeInvitation)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidInvitationID)
	}
	req := new(model.RevokeInvitationRequest)
	_ = c.Bind(req) // body is optional; binding errors ignored intentionally
	actorID, err := claimsUserID(c)
	if err != nil || actorID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	invitation, err := h.svc.Revoke(c.Request().Context(), id, *actorID, req.Reason,
		c.Response().Header().Get(echo.HeaderXRequestID), c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return h.mapError(c, err, i18n.MsgFailedToRevokeInvitation)
	}
	return c.JSON(http.StatusOK, invitation.ToDTO())
}

// Resend handles POST /projects/:pid/invitations/:id/resend.
func (h *InvitationHandler) Resend(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToResendInvitation)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidInvitationID)
	}
	actorID, err := claimsUserID(c)
	if err != nil || actorID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	invitation, err := h.svc.Resend(c.Request().Context(), id, *actorID,
		c.Response().Header().Get(echo.HeaderXRequestID), c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return h.mapError(c, err, i18n.MsgFailedToResendInvitation)
	}
	return c.JSON(http.StatusOK, invitation.ToDTO())
}

// GetByToken handles GET /invitations/by-token/:token (public, no auth).
// Returns a narrow preview payload suitable for a pre-login landing page.
func (h *InvitationHandler) GetByToken(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInvitationNotFound)
	}
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidInvitationToken)
	}
	preview, _, err := h.svc.PreviewByToken(c.Request().Context(), token)
	if err != nil {
		return h.mapError(c, err, i18n.MsgInvitationNotFound)
	}
	return c.JSON(http.StatusOK, preview)
}

// Accept handles POST /invitations/accept (requires auth).
func (h *InvitationHandler) Accept(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToAcceptInvitation)
	}
	req := new(model.AcceptInvitationRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	actorID, err := claimsUserID(c)
	if err != nil || actorID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	invitation, member, err := h.svc.Accept(c.Request().Context(), service.AcceptInput{
		PlaintextToken: req.Token,
		CallerUserID:   *actorID,
		RequestID:      c.Response().Header().Get(echo.HeaderXRequestID),
		IP:             c.RealIP(),
		UserAgent:      c.Request().UserAgent(),
	})
	if err != nil {
		return h.mapError(c, err, i18n.MsgFailedToAcceptInvitation)
	}
	resp := map[string]any{"invitation": invitation.ToDTO()}
	if member != nil {
		resp["member"] = member.ToDTO()
	}
	return c.JSON(http.StatusOK, resp)
}

// Decline handles POST /invitations/decline (auth optional).
func (h *InvitationHandler) Decline(c echo.Context) error {
	if h == nil || h.svc == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeclineInvitation)
	}
	req := new(model.DeclineInvitationRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	var callerID *uuid.UUID
	if actorID, err := claimsUserID(c); err == nil {
		callerID = actorID
	}
	invitation, err := h.svc.Decline(c.Request().Context(), service.DeclineInput{
		PlaintextToken: req.Token,
		CallerUserID:   callerID,
		Reason:         req.Reason,
		RequestID:      c.Response().Header().Get(echo.HeaderXRequestID),
		IP:             c.RealIP(),
		UserAgent:      c.Request().UserAgent(),
	})
	if err != nil {
		return h.mapError(c, err, i18n.MsgFailedToDeclineInvitation)
	}
	return c.JSON(http.StatusOK, invitation.ToDTO())
}

// mapError translates service-layer sentinel errors into localized HTTP
// responses. fallbackMsgID is used for unrecognised internal errors.
func (h *InvitationHandler) mapError(c echo.Context, err error, fallbackMsgID string) error {
	switch {
	case errors.Is(err, service.ErrInvitationNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgInvitationNotFound)
	case errors.Is(err, service.ErrInvitationAlreadyPendingForIdent):
		return localizedError(c, http.StatusConflict, i18n.MsgInvitationAlreadyPendingForIdent)
	case errors.Is(err, service.ErrInvitationAlreadyProcessed):
		return localizedError(c, http.StatusConflict, i18n.MsgInvitationAlreadyProcessed)
	case errors.Is(err, service.ErrInvitationExpired):
		return localizedError(c, http.StatusGone, i18n.MsgInvitationExpired)
	case errors.Is(err, service.ErrInvitationIdentityMismatch):
		return localizedError(c, http.StatusForbidden, i18n.MsgInvitationIdentityMismatch)
	case errors.Is(err, service.ErrInvitationInvalidIdentity):
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvitationInvalidIdentity)
	case errors.Is(err, service.ErrInvitationInvalidRole):
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvitationInvalidRole)
	}
	return localizedError(c, http.StatusInternalServerError, fallbackMsgID)
}
