package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	rolepkg "github.com/agentforge/server/internal/role"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type MemberHandler struct {
	repo      memberRepository
	roleStore memberRoleStore
}

type memberRepository interface {
	Create(ctx context.Context, member *model.Member) error
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateMemberRequest) error
	BulkUpdateStatus(ctx context.Context, projectID uuid.UUID, memberIDs []uuid.UUID, status string) ([]model.BulkUpdateMemberResult, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error)
	Delete(ctx context.Context, id uuid.UUID) error
	CountOwners(ctx context.Context, projectID uuid.UUID) (int64, error)
	UpdateProjectRole(ctx context.Context, id uuid.UUID, role string) error
}

type memberRoleStore interface {
	Get(id string) (*rolepkg.Manifest, error)
}

func NewMemberHandler(repo memberRepository) *MemberHandler {
	return &MemberHandler{repo: repo}
}

func (h *MemberHandler) WithRoleStore(store memberRoleStore) *MemberHandler {
	h.roleStore = store
	return h
}

func (h *MemberHandler) Create(c echo.Context) error {
	req := new(model.CreateMemberRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	// Human members must be onboarded via the invitation flow. The
	// create-member endpoint is retained for agent creation only — see
	// openspec/specs/member-invitation-flow/spec.md.
	if req.Type != model.MemberTypeAgent {
		return localizedError(c, http.StatusGone, i18n.MsgHumanMemberCreationMoved)
	}
	if validationErr := h.validateAgentRoleBinding(c.Request().Context(), req.Type, req.AgentConfig); validationErr != nil {
		return c.JSON(http.StatusUnprocessableEntity, validationErr)
	}

	projectID := appMiddleware.GetProjectID(c)
	status := model.NormalizeMemberStatus(req.Status, true)
	projectRole := model.NormalizeProjectRole(req.ProjectRole)

	var userID *uuid.UUID
	if req.UserID != "" {
		parsed, err := uuid.Parse(req.UserID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		userID = &parsed
	}

	member := &model.Member{
		ID:          uuid.New(),
		ProjectID:   projectID,
		UserID:      userID,
		Name:        req.Name,
		Type:        req.Type,
		Role:        req.Role,
		ProjectRole: projectRole,
		Status:      status,
		Email:       req.Email,
		IMPlatform:  req.IMPlatform,
		IMUserID:    req.IMUserID,
		AgentConfig: req.AgentConfig,
		Skills:      req.Skills,
		IsActive:    model.IsMemberStatusActive(status),
	}
	if err := h.repo.Create(c.Request().Context(), member); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateMember)
	}
	return c.JSON(http.StatusCreated, member.ToDTO())
}

func (h *MemberHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	members, err := h.repo.ListByProject(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListMembers)
	}
	roleFilter := c.QueryParam("role")
	dtos := make([]model.MemberDTO, 0, len(members))
	for _, m := range members {
		if roleFilter != "" && model.NormalizeProjectRole(m.ProjectRole) != roleFilter {
			continue
		}
		dtos = append(dtos, m.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *MemberHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
	}
	req := new(model.UpdateMemberRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if validator, ok := c.Echo().Validator.(interface{ Validate(any) error }); ok && validator != nil {
		if err := validator.Validate(req); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
		}
	}
	if req.AgentConfig != nil {
		if validationErr := h.validateAgentRoleBinding(c.Request().Context(), model.MemberTypeAgent, *req.AgentConfig); validationErr != nil {
			return c.JSON(http.StatusUnprocessableEntity, validationErr)
		}
	}
	if req.Status == nil && req.IsActive != nil {
		status := model.NormalizeMemberStatus("", *req.IsActive)
		req.Status = &status
	}

	// Project-role transition guards. These run before any persistence so a
	// rejected role change does not partially apply other field updates.
	if req.ProjectRole != nil {
		target, err := h.repo.GetByID(c.Request().Context(), id)
		if err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchUpdatedMember)
		}
		newRole := model.NormalizeProjectRole(*req.ProjectRole)
		oldRole := model.NormalizeProjectRole(target.ProjectRole)

		// Admin cannot modify owners (only owner→owner-or-promote chain may).
		callerRole := appMiddleware.GetCallerProjectRole(c)
		if callerRole == model.ProjectRoleAdmin && oldRole == model.ProjectRoleOwner && newRole != oldRole {
			return c.JSON(http.StatusForbidden, model.ErrorResponse{
				Message: i18nForbidden(c, i18n.MsgCannotModifyOwnerAsAdmin),
			})
		}

		// Last-owner protection: block downgrade of the only remaining owner.
		if oldRole == model.ProjectRoleOwner && newRole != model.ProjectRoleOwner {
			count, err := h.repo.CountOwners(c.Request().Context(), target.ProjectID)
			if err != nil {
				return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateMember)
			}
			if count <= 1 {
				return c.JSON(http.StatusConflict, model.ErrorResponse{
					Message: i18nForbidden(c, i18n.MsgLastOwnerProtected),
				})
			}
		}

		if err := h.repo.UpdateProjectRole(c.Request().Context(), id, newRole); err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateMember)
		}
		// Strip from req so the generic Update below does not also try to
		// touch the column with a stale value.
		req.ProjectRole = nil
	}

	if err := h.repo.Update(c.Request().Context(), id, req); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateMember)
	}
	member, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchUpdatedMember)
	}
	return c.JSON(http.StatusOK, member.ToDTO())
}

func (h *MemberHandler) BulkUpdate(c echo.Context) error {
	req := new(model.BulkUpdateMembersRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if validator, ok := c.Echo().Validator.(interface{ Validate(any) error }); ok && validator != nil {
		if err := validator.Validate(req); err != nil {
			return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
		}
	}

	projectID := appMiddleware.GetProjectID(c)
	memberIDs := make([]uuid.UUID, 0, len(req.MemberIDs))
	for _, rawID := range req.MemberIDs {
		memberID, err := uuid.Parse(rawID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
		}
		memberIDs = append(memberIDs, memberID)
	}

	results, err := h.repo.BulkUpdateStatus(c.Request().Context(), projectID, memberIDs, req.Status)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateMember)
	}

	return c.JSON(http.StatusOK, model.BulkUpdateMembersResponse{Results: results})
}

func (h *MemberHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemberID)
	}

	target, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteMember)
	}
	if target != nil {
		// Admin cannot delete owners — same boundary as role-change.
		callerRole := appMiddleware.GetCallerProjectRole(c)
		if callerRole == model.ProjectRoleAdmin && model.NormalizeProjectRole(target.ProjectRole) == model.ProjectRoleOwner {
			return c.JSON(http.StatusForbidden, model.ErrorResponse{
				Message: i18nForbidden(c, i18n.MsgCannotModifyOwnerAsAdmin),
			})
		}
		// Last-owner protection: block deleting the only remaining owner.
		if model.NormalizeProjectRole(target.ProjectRole) == model.ProjectRoleOwner {
			count, err := h.repo.CountOwners(c.Request().Context(), target.ProjectID)
			if err != nil {
				return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteMember)
			}
			if count <= 1 {
				return c.JSON(http.StatusConflict, model.ErrorResponse{
					Message: i18nForbidden(c, i18n.MsgLastOwnerProtected),
				})
			}
		}
	}

	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteMember)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *MemberHandler) validateAgentRoleBinding(ctx context.Context, memberType string, agentConfig string) *model.ErrorResponse {
	if memberType != model.MemberTypeAgent {
		return nil
	}
	roleID := service.ExtractRoleIDFromAgentConfig(agentConfig)
	if roleID == "" {
		return nil
	}
	err := service.NewRoleReferenceGovernanceService(nil, nil, nil, nil).
		WithRoleStore(h.roleStore).
		ValidateRoleBinding(ctx, roleID)
	if err == nil {
		return nil
	}
	if errors.Is(err, service.ErrRoleBindingNotFound) {
		return &model.ErrorResponse{
			Message: err.Error(),
			Field:   "agentConfig.roleId",
		}
	}
	return &model.ErrorResponse{Message: err.Error()}
}

// i18nForbidden returns the localized message string for a 403/409 response.
func i18nForbidden(c echo.Context, messageID string) string {
	loc := appMiddleware.GetLocalizer(c)
	return i18n.Localize(loc, messageID)
}
