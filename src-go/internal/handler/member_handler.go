package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
)

type MemberHandler struct {
	repo memberRepository
	roleStore memberRoleStore
}

type memberRepository interface {
	Create(ctx context.Context, member *model.Member) error
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateMemberRequest) error
	BulkUpdateStatus(ctx context.Context, projectID uuid.UUID, memberIDs []uuid.UUID, status string) ([]model.BulkUpdateMemberResult, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error)
	Delete(ctx context.Context, id uuid.UUID) error
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
	if validationErr := h.validateAgentRoleBinding(c.Request().Context(), req.Type, req.AgentConfig); validationErr != nil {
		return c.JSON(http.StatusUnprocessableEntity, validationErr)
	}

	projectID := appMiddleware.GetProjectID(c)
	status := model.NormalizeMemberStatus(req.Status, true)
	member := &model.Member{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        req.Name,
		Type:        req.Type,
		Role:        req.Role,
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
	dtos := make([]model.MemberDTO, 0, len(members))
	for _, m := range members {
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
