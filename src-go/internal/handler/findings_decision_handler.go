package handler

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	log "github.com/sirupsen/logrus"
)

// DecisionRequest is the body for POST /api/v1/findings/:id/decision.
type DecisionRequest struct {
	Action  string `json:"action" validate:"required,oneof=approve dismiss defer"`
	Comment string `json:"comment,omitempty"`
}

// DecisionResponse carries the spawned execution ID on approve.
type DecisionResponse struct {
	ExecutionID string `json:"executionId,omitempty"`
}

// CodeFixerSpawner launches the system code_fixer DAG for a finding.
type CodeFixerSpawner interface {
	Spawn(ctx echo.Context, reviewID, findingID uuid.UUID, actorUserID string) (executionID uuid.UUID, err error)
}

// FindingsDecisionAuditWriter records decision audit entries.
type FindingsDecisionAuditWriter interface {
	Append(ctx echo.Context, action, actor string, payload map[string]any) error
}

// FindingsDecisionReviewRepo is the narrow repo interface consumed here.
type FindingsDecisionReviewRepo interface {
	UpdateFindingDecision(ctx echo.Context, findingID uuid.UUID, decision string, dismissed bool) error
}

// FindingsEventPublisher publishes finding-related domain events.
type FindingsEventPublisher interface {
	PublishFindingDismissed(ctx echo.Context, findingID uuid.UUID, actor string) error
}

// FindingsRBACChecker checks project-level permissions.
type FindingsRBACChecker interface {
	RequireRole(ctx echo.Context, requiredRole string) error
}

// FindingsDecisionHandler handles per-finding decision actions.
type FindingsDecisionHandler struct {
	spawner CodeFixerSpawner
	audit   FindingsDecisionAuditWriter
	repo    FindingsDecisionReviewRepo
	events  FindingsEventPublisher
	rbac    FindingsRBACChecker
}

// NewFindingsDecisionHandler creates a new handler.
func NewFindingsDecisionHandler(
	spawner CodeFixerSpawner,
	audit FindingsDecisionAuditWriter,
	repo FindingsDecisionReviewRepo,
	events FindingsEventPublisher,
	rbac FindingsRBACChecker,
) *FindingsDecisionHandler {
	return &FindingsDecisionHandler{
		spawner: spawner,
		audit:   audit,
		repo:    repo,
		events:  events,
		rbac:    rbac,
	}
}

// Decide handles POST /api/v1/findings/:id/decision.
func (h *FindingsDecisionHandler) Decide(c echo.Context) error {
	findingIDStr := c.Param("id")
	findingID, err := uuid.Parse(findingIDStr)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid finding id"})
	}

	var req DecisionRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// RBAC: approve/dismiss require editor, defer requires viewer
	if req.Action == "approve" || req.Action == "dismiss" {
		if h.rbac != nil {
			if err := h.rbac.RequireRole(c, "editor"); err != nil {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "insufficient permissions"})
			}
		}
	}

	// Extract actor from context (set by auth middleware)
	actor := ""
	if userID, ok := c.Get("userID").(string); ok {
		actor = userID
	}

	switch req.Action {
	case "approve":
		// Persist decision
		if h.repo != nil {
			if err := h.repo.UpdateFindingDecision(c, findingID, "approved", false); err != nil {
				log.WithError(err).Error("failed to update finding decision")
			}
		}
		// Spawn code_fixer DAG
		var execID uuid.UUID
		if h.spawner != nil {
			execID, err = h.spawner.Spawn(c, uuid.Nil, findingID, actor)
			if err != nil {
				log.WithError(err).Error("failed to spawn code_fixer")
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to spawn fixer"})
			}
		}
		// Audit
		h.auditDecision(c, findingID, req.Action, actor, req.Comment)
		return c.JSON(http.StatusAccepted, DecisionResponse{ExecutionID: execID.String()})

	case "dismiss":
		if h.repo != nil {
			if err := h.repo.UpdateFindingDecision(c, findingID, "dismissed", true); err != nil {
				log.WithError(err).Error("failed to update finding decision")
			}
		}
		if h.events != nil {
			_ = h.events.PublishFindingDismissed(c, findingID, actor)
		}
		h.auditDecision(c, findingID, req.Action, actor, req.Comment)
		return c.JSON(http.StatusOK, map[string]string{"status": "dismissed"})

	case "defer":
		if h.repo != nil {
			if err := h.repo.UpdateFindingDecision(c, findingID, "deferred", false); err != nil {
				log.WithError(err).Error("failed to update finding decision")
			}
		}
		h.auditDecision(c, findingID, req.Action, actor, req.Comment)
		return c.JSON(http.StatusOK, map[string]string{"status": "deferred"})

	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unknown action"})
	}
}

// DecideInternal is a non-HTTP entry point for the automation branch of
// card_action_router (no Echo context needed).
func (h *FindingsDecisionHandler) DecideInternal(ctx echo.Context, findingID uuid.UUID, action, actor string) error {
	switch action {
	case "apply", "approve":
		if h.repo != nil {
			_ = h.repo.UpdateFindingDecision(ctx, findingID, "approved", false)
		}
		if h.spawner != nil {
			_, _ = h.spawner.Spawn(ctx, uuid.Nil, findingID, actor)
		}
	case "dismiss":
		if h.repo != nil {
			_ = h.repo.UpdateFindingDecision(ctx, findingID, "dismissed", true)
		}
		if h.events != nil {
			_ = h.events.PublishFindingDismissed(ctx, findingID, actor)
		}
	}
	return nil
}

func (h *FindingsDecisionHandler) auditDecision(c echo.Context, findingID uuid.UUID, action, actor, comment string) {
	if h.audit == nil {
		return
	}
	_ = h.audit.Append(c, "finding.decision", actor, map[string]any{
		"findingId": findingID.String(),
		"action":    action,
		"comment":   comment,
	})
}
