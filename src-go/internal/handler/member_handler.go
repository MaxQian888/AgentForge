package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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
}

func NewMemberHandler(repo memberRepository) *MemberHandler {
	return &MemberHandler{repo: repo}
}

func (h *MemberHandler) Create(c echo.Context) error {
	req := new(model.CreateMemberRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	projectID := appMiddleware.GetProjectID(c)
	member := &model.Member{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        req.Name,
		Type:        req.Type,
		Role:        req.Role,
		Email:       req.Email,
		AgentConfig: req.AgentConfig,
		Skills:      req.Skills,
		IsActive:    true,
	}
	if err := h.repo.Create(c.Request().Context(), member); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create member"})
	}
	return c.JSON(http.StatusCreated, member.ToDTO())
}

func (h *MemberHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	members, err := h.repo.ListByProject(c.Request().Context(), projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list members"})
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid member ID"})
	}
	req := new(model.UpdateMemberRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.repo.Update(c.Request().Context(), id, req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to update member"})
	}
	member, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch updated member"})
	}
	return c.JSON(http.StatusOK, member.ToDTO())
}
