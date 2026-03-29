package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type MemberHandler struct {
	repo memberRepository
}

type memberRepository interface {
	Create(ctx context.Context, member *model.Member) error
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateMemberRequest) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

func NewMemberHandler(repo memberRepository) *MemberHandler {
	return &MemberHandler{repo: repo}
}

func (h *MemberHandler) Create(c echo.Context) error {
	req := new(model.CreateMemberRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
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
