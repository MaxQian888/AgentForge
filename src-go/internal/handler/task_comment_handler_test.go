package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type taskCommentTestValidator struct {
	validator *validator.Validate
}

func (v *taskCommentTestValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type mockTaskCommentHandlerService struct {
	comments map[uuid.UUID]*model.TaskComment
}

func (m *mockTaskCommentHandlerService) CreateComment(_ context.Context, input *service.CreateTaskCommentInput) (*model.TaskComment, error) {
	comment := &model.TaskComment{
		ID:        uuid.New(),
		TaskID:    input.TaskID,
		Body:      input.Body,
		CreatedBy: input.CreatedBy,
	}
	if m.comments == nil {
		m.comments = map[uuid.UUID]*model.TaskComment{}
	}
	m.comments[comment.ID] = comment
	return comment, nil
}

func (m *mockTaskCommentHandlerService) ReplyToComment(_ context.Context, parentCommentID uuid.UUID, input *service.CreateTaskCommentInput) (*model.TaskComment, error) {
	comment, _ := m.CreateComment(context.Background(), input)
	comment.ParentCommentID = &parentCommentID
	m.comments[comment.ID] = comment
	return comment, nil
}

func (m *mockTaskCommentHandlerService) ResolveComment(_ context.Context, commentID uuid.UUID) (*model.TaskComment, error) {
	comment := m.comments[commentID]
	now := time.Now().UTC()
	comment.ResolvedAt = &now
	return comment, nil
}

func (m *mockTaskCommentHandlerService) ReopenComment(_ context.Context, commentID uuid.UUID) (*model.TaskComment, error) {
	comment := m.comments[commentID]
	comment.ResolvedAt = nil
	return comment, nil
}

func (m *mockTaskCommentHandlerService) DeleteComment(_ context.Context, commentID uuid.UUID) error {
	delete(m.comments, commentID)
	return nil
}

func (m *mockTaskCommentHandlerService) ListComments(_ context.Context, taskID uuid.UUID) ([]*model.TaskComment, error) {
	result := make([]*model.TaskComment, 0)
	for _, comment := range m.comments {
		if comment.TaskID == taskID {
			cloned := *comment
			result = append(result, &cloned)
		}
	}
	return result, nil
}

func TestTaskCommentHandlerCRUD(t *testing.T) {
	e := echo.New()
	e.Validator = &taskCommentTestValidator{validator: validator.New()}
	projectID := uuid.New()
	userID := uuid.New()
	taskID := uuid.New()
	svc := &mockTaskCommentHandlerService{comments: map[uuid.UUID]*model.TaskComment{}}
	h := NewTaskCommentHandler(svc)

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/id/tasks/id/comments", strings.NewReader(`{"body":"Need @alice"}`))
	createReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.SetParamNames("tid")
	createCtx.SetParamValues(taskID.String())
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	createCtx.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: userID.String()})
	if err := h.Create(createCtx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("Create() status = %d", createRec.Code)
	}

	var created model.TaskCommentDTO
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal created comment: %v", err)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/id/tasks/id/comments", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.SetParamNames("tid")
	listCtx.SetParamValues(taskID.String())
	if err := h.List(listCtx); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List() status = %d", listRec.Code)
	}

	updateReq := httptest.NewRequest(http.MethodPatch, "/api/v1/projects/id/tasks/id/comments/id", strings.NewReader(`{"resolved":true}`))
	updateReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetParamNames("tid", "cid")
	updateCtx.SetParamValues(taskID.String(), created.ID)
	if err := h.Update(updateCtx); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("Update() status = %d", updateRec.Code)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/id/tasks/id/comments/id", nil)
	deleteRec := httptest.NewRecorder()
	deleteCtx := e.NewContext(deleteReq, deleteRec)
	deleteCtx.SetParamNames("tid", "cid")
	deleteCtx.SetParamValues(taskID.String(), created.ID)
	if err := h.Delete(deleteCtx); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("Delete() status = %d", deleteRec.Code)
	}
}
