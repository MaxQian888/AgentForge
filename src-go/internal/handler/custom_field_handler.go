package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type customFieldService interface {
	CreateField(ctx context.Context, definition *model.CustomFieldDefinition) error
	GetField(ctx context.Context, id uuid.UUID) (*model.CustomFieldDefinition, error)
	ListFields(ctx context.Context, projectID uuid.UUID) ([]*model.CustomFieldDefinition, error)
	UpdateField(ctx context.Context, definition *model.CustomFieldDefinition) error
	DeleteField(ctx context.Context, id uuid.UUID) error
	ReorderFields(ctx context.Context, projectID uuid.UUID, orderedIDs []uuid.UUID) error
	SetValue(ctx context.Context, value *model.CustomFieldValue) error
	ClearValue(ctx context.Context, taskID uuid.UUID, fieldDefID uuid.UUID) error
	GetValuesForTask(ctx context.Context, taskID uuid.UUID) ([]*model.CustomFieldValue, error)
}

type CustomFieldHandler struct{ service customFieldService }

func NewCustomFieldHandler(service customFieldService) *CustomFieldHandler {
	return &CustomFieldHandler{service: service}
}

func (h *CustomFieldHandler) ListDefinitions(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	definitions, err := h.service.ListFields(c.Request().Context(), projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list custom fields"})
	}
	dtos := make([]model.CustomFieldDefinitionDTO, 0, len(definitions))
	for _, definition := range definitions {
		dtos = append(dtos, definition.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *CustomFieldHandler) CreateDefinition(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(model.CreateCustomFieldRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	definition := &model.CustomFieldDefinition{
		ProjectID: projectID,
		Name:      req.Name,
		FieldType: req.FieldType,
		Options:   string(req.Options),
		Required:  req.Required,
	}
	if err := h.service.CreateField(c.Request().Context(), definition); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create custom field"})
	}
	return c.JSON(http.StatusCreated, definition.ToDTO())
}

func (h *CustomFieldHandler) UpdateDefinition(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid field ID"})
	}
	definition, err := h.service.GetField(c.Request().Context(), fieldID)
	if err != nil || definition == nil || definition.ProjectID != projectID {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "custom field not found"})
	}
	req := new(model.UpdateCustomFieldRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if req.Name != nil {
		definition.Name = *req.Name
	}
	if req.FieldType != nil {
		definition.FieldType = *req.FieldType
	}
	if len(req.Options) > 0 {
		definition.Options = string(req.Options)
	}
	if req.Required != nil {
		definition.Required = *req.Required
	}
	if req.SortOrder != nil {
		definition.SortOrder = *req.SortOrder
	}
	if err := h.service.UpdateField(c.Request().Context(), definition); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to update custom field"})
	}
	return c.JSON(http.StatusOK, definition.ToDTO())
}

func (h *CustomFieldHandler) DeleteDefinition(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid field ID"})
	}
	definition, err := h.service.GetField(c.Request().Context(), fieldID)
	if err != nil || definition == nil || definition.ProjectID != projectID {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "custom field not found"})
	}
	if err := h.service.DeleteField(c.Request().Context(), fieldID); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to delete custom field"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "custom field deleted"})
}

func (h *CustomFieldHandler) ReorderDefinitions(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(model.ReorderCustomFieldsRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	orderedIDs := make([]uuid.UUID, 0, len(req.FieldIDs))
	for _, rawID := range req.FieldIDs {
		parsed, err := uuid.Parse(rawID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid field ID in reorder payload"})
		}
		orderedIDs = append(orderedIDs, parsed)
	}
	if err := h.service.ReorderFields(c.Request().Context(), projectID, orderedIDs); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to reorder custom fields"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "custom fields reordered"})
}

func (h *CustomFieldHandler) ListTaskValues(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	values, err := h.service.GetValuesForTask(c.Request().Context(), taskID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list custom field values"})
	}
	dtos := make([]model.CustomFieldValueDTO, 0, len(values))
	for _, value := range values {
		dtos = append(dtos, value.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *CustomFieldHandler) SetTaskValue(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid field ID"})
	}
	definition, err := h.service.GetField(c.Request().Context(), fieldID)
	if err != nil || definition == nil || definition.ProjectID != projectID {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "custom field not found"})
	}
	req := new(model.SetCustomFieldValueRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	value := &model.CustomFieldValue{
		TaskID:     taskID,
		FieldDefID: fieldID,
		Value:      string(req.Value),
	}
	if err := h.service.SetValue(c.Request().Context(), value); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to set custom field value"})
	}
	return c.JSON(http.StatusOK, value.ToDTO())
}

func (h *CustomFieldHandler) ClearTaskValue(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid field ID"})
	}
	if err := h.service.ClearValue(c.Request().Context(), taskID, fieldID); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to clear custom field value"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "custom field value cleared"})
}
