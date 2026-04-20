package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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

type CustomFieldHandler struct {
	service    customFieldService
	automation service.AutomationEventEvaluator
}

func NewCustomFieldHandler(service customFieldService) *CustomFieldHandler {
	return &CustomFieldHandler{service: service}
}

func (h *CustomFieldHandler) WithAutomation(evaluator service.AutomationEventEvaluator) *CustomFieldHandler {
	h.automation = evaluator
	return h
}

func (h *CustomFieldHandler) ListDefinitions(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	definitions, err := h.service.ListFields(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListCustomFields)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateCustomField)
	}
	return c.JSON(http.StatusCreated, definition.ToDTO())
}

func (h *CustomFieldHandler) UpdateDefinition(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFieldID)
	}
	definition, err := h.service.GetField(c.Request().Context(), fieldID)
	if err != nil || definition == nil || definition.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgCustomFieldNotFound)
	}
	req := new(model.UpdateCustomFieldRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
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
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateCustomField)
	}
	return c.JSON(http.StatusOK, definition.ToDTO())
}

func (h *CustomFieldHandler) DeleteDefinition(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFieldID)
	}
	definition, err := h.service.GetField(c.Request().Context(), fieldID)
	if err != nil || definition == nil || definition.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgCustomFieldNotFound)
	}
	if err := h.service.DeleteField(c.Request().Context(), fieldID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteCustomField)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "custom field deleted"})
}

func (h *CustomFieldHandler) ReorderDefinitions(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(model.ReorderCustomFieldsRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	orderedIDs := make([]uuid.UUID, 0, len(req.FieldIDs))
	for _, rawID := range req.FieldIDs {
		parsed, err := uuid.Parse(rawID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFieldIDInReorder)
		}
		orderedIDs = append(orderedIDs, parsed)
	}
	if err := h.service.ReorderFields(c.Request().Context(), projectID, orderedIDs); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToReorderCustomFields)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "custom fields reordered"})
}

func (h *CustomFieldHandler) ListTaskValues(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	values, err := h.service.GetValuesForTask(c.Request().Context(), taskID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListCustomFieldValues)
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
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFieldID)
	}
	definition, err := h.service.GetField(c.Request().Context(), fieldID)
	if err != nil || definition == nil || definition.ProjectID != projectID {
		return localizedError(c, http.StatusNotFound, i18n.MsgCustomFieldNotFound)
	}
	req := new(model.SetCustomFieldValueRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	value := &model.CustomFieldValue{
		TaskID:     taskID,
		FieldDefID: fieldID,
		Value:      string(req.Value),
	}
	if err := h.service.SetValue(c.Request().Context(), value); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSetCustomFieldValue)
	}
	if h.automation != nil {
		decodedValue := decodeAutomationJSONValue(req.Value)
		_ = h.automation.EvaluateRules(c.Request().Context(), service.AutomationEvent{
			EventType: model.AutomationEventTaskFieldChanged,
			ProjectID: projectID,
			TaskID:    &taskID,
			Data: map[string]any{
				"field":         "cf:" + fieldID.String(),
				"fieldDefId":    fieldID.String(),
				"value":         decodedValue,
				"current_value": decodedValue,
			},
		})
	}
	return c.JSON(http.StatusOK, value.ToDTO())
}

func (h *CustomFieldHandler) ClearTaskValue(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	fieldID, err := uuid.Parse(c.Param("fid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidFieldID)
	}
	if err := h.service.ClearValue(c.Request().Context(), taskID, fieldID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToClearCustomFieldValue)
	}
	if h.automation != nil {
		projectID := appMiddleware.GetProjectID(c)
		_ = h.automation.EvaluateRules(c.Request().Context(), service.AutomationEvent{
			EventType: model.AutomationEventTaskFieldChanged,
			ProjectID: projectID,
			TaskID:    &taskID,
			Data: map[string]any{
				"field":         "cf:" + fieldID.String(),
				"fieldDefId":    fieldID.String(),
				"value":         nil,
				"current_value": nil,
			},
		})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "custom field value cleared"})
}

func decodeAutomationJSONValue(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return string(raw)
	}
	return decoded
}
