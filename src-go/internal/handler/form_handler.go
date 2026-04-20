package handler

import (
	"context"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type formService interface {
	CreateForm(ctx context.Context, form *model.FormDefinition) error
	GetForm(ctx context.Context, id uuid.UUID) (*model.FormDefinition, error)
	ListForms(ctx context.Context, projectID uuid.UUID) ([]*model.FormDefinition, error)
	UpdateForm(ctx context.Context, form *model.FormDefinition) error
	DeleteForm(ctx context.Context, id uuid.UUID) error
	GetFormBySlug(ctx context.Context, slug string) (*model.FormDefinition, error)
	SubmitForm(ctx context.Context, slug string, input service.FormSubmissionInput) (*model.Task, error)
}

type FormHandler struct{ service formService }

func NewFormHandler(service formService) *FormHandler { return &FormHandler{service: service} }

func (h *FormHandler) GetBySlug(c echo.Context) error {
	slug := c.Param("slug")
	form, err := h.service.GetFormBySlug(c.Request().Context(), slug)
	if err != nil || form == nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgFormNotFound)
	}
	if !form.IsPublic {
		if _, claimsErr := claimsUserID(c); claimsErr != nil {
			return localizedError(c, http.StatusUnauthorized, i18n.MsgAuthRequired)
		}
	}
	return c.JSON(http.StatusOK, form.ToDTO())
}

func (h *FormHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	forms, err := h.service.ListForms(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListForms)
	}
	dtos := make([]model.FormDefinitionDTO, 0, len(forms))
	for _, form := range forms {
		dtos = append(dtos, form.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *FormHandler) Create(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(model.CreateFormRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	targetAssignee, err := parseOptionalUUIDString(req.TargetAssignee)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTargetAssignee)
	}
	form := &model.FormDefinition{
		ProjectID:      projectID,
		Name:           req.Name,
		Slug:           req.Slug,
		Fields:         string(req.Fields),
		TargetStatus:   req.TargetStatus,
		TargetAssignee: targetAssignee,
		IsPublic:       req.IsPublic,
	}
	if err := h.service.CreateForm(c.Request().Context(), form); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateForm)
	}
	return c.JSON(http.StatusCreated, form.ToDTO())
}

func (h *FormHandler) Update(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	formID, err := uuid.Parse(c.Param("formId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFormID)
	}
	form, err := h.service.GetForm(c.Request().Context(), formID)
	if err != nil || form == nil || form.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgFormNotFound)
	}
	req := new(model.UpdateFormRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Name != nil {
		form.Name = *req.Name
	}
	if req.Slug != nil {
		form.Slug = *req.Slug
	}
	if len(req.Fields) > 0 {
		form.Fields = string(req.Fields)
	}
	if req.TargetStatus != nil {
		form.TargetStatus = *req.TargetStatus
	}
	if req.TargetAssignee != nil {
		targetAssignee, parseErr := parseOptionalUUIDString(req.TargetAssignee)
		if parseErr != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTargetAssignee)
		}
		form.TargetAssignee = targetAssignee
	}
	if req.IsPublic != nil {
		form.IsPublic = *req.IsPublic
	}
	if err := h.service.UpdateForm(c.Request().Context(), form); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateForm)
	}
	return c.JSON(http.StatusOK, form.ToDTO())
}

func (h *FormHandler) Delete(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	formID, err := uuid.Parse(c.Param("formId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFormID)
	}
	form, err := h.service.GetForm(c.Request().Context(), formID)
	if err != nil || form == nil || form.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgFormNotFound)
	}
	if err := h.service.DeleteForm(c.Request().Context(), formID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteForm)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "form deleted"})
}

func (h *FormHandler) Submit(c echo.Context) error {
	slug := c.Param("slug")
	form, err := h.service.GetFormBySlug(c.Request().Context(), slug)
	if err != nil || form == nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgFormNotFound)
	}
	if !form.IsPublic {
		if _, claimsErr := claimsUserID(c); claimsErr != nil {
			return localizedError(c, http.StatusUnauthorized, i18n.MsgAuthRequired)
		}
	}
	req := new(model.SubmitFormRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	submittedBy := req.SubmittedBy
	if userID, claimsErr := claimsUserID(c); claimsErr == nil && userID != nil {
		submittedBy = userID.String()
	}
	task, err := h.service.SubmitForm(c.Request().Context(), slug, service.FormSubmissionInput{
		SubmittedBy: submittedBy,
		IPAddress:   c.RealIP(),
		Values:      req.Values,
	})
	if err != nil {
		switch {
		case err == service.ErrFormRateLimited:
			return localizedError(c, http.StatusTooManyRequests, i18n.MsgTooManySubmissions)
		default:
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSubmitForm)
		}
	}
	return c.JSON(http.StatusCreated, task.ToDTO())
}
