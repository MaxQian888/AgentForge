package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
)

type reviewServiceMock struct {
	triggered             *model.TriggerReviewRequest
	completed             *model.CompleteReviewRequest
	completeID            uuid.UUID
	getID                 uuid.UUID
	getTaskID             uuid.UUID
	listStatus            string
	listRiskLevel         string
	listLimit             int
	approveID             uuid.UUID
	approveActor          string
	approveComment        string
	requestChangesID      uuid.UUID
	requestChangesActor   string
	requestChangesComment string
	rejectID              uuid.UUID
	rejectActor           string
	rejectReason          string
	rejectComment         string
	falsePositiveID       uuid.UUID
	falsePositiveActor    string
	falsePositiveIDs      []string
	falsePositiveReason   string
	ciRequest             *model.CIReviewRequest
	routeFixID            uuid.UUID
	routeFixErr           error
	review                *model.Review
	reviews               []*model.Review
}

func (m *reviewServiceMock) Trigger(_ context.Context, req *model.TriggerReviewRequest) (*model.Review, error) {
	m.triggered = req
	return m.review, nil
}

func (m *reviewServiceMock) Complete(_ context.Context, id uuid.UUID, req *model.CompleteReviewRequest) (*model.Review, error) {
	m.completeID = id
	m.completed = req
	return m.review, nil
}

func (m *reviewServiceMock) GetByID(_ context.Context, id uuid.UUID) (*model.Review, error) {
	m.getID = id
	return m.review, nil
}

func (m *reviewServiceMock) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.Review, error) {
	m.getTaskID = taskID
	return m.reviews, nil
}

func (m *reviewServiceMock) ListAll(_ context.Context, status, riskLevel string, limit int) ([]*model.Review, error) {
	m.listStatus = status
	m.listRiskLevel = riskLevel
	m.listLimit = limit
	return m.reviews, nil
}

func (m *reviewServiceMock) Approve(_ context.Context, _ uuid.UUID, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) ApproveReview(_ context.Context, id uuid.UUID, actor, comment string) (*model.Review, error) {
	m.approveID = id
	m.approveActor = actor
	m.approveComment = comment
	return m.review, nil
}

func (m *reviewServiceMock) RequestChangesReview(_ context.Context, id uuid.UUID, actor, comment string) (*model.Review, error) {
	m.requestChangesID = id
	m.requestChangesActor = actor
	m.requestChangesComment = comment
	return m.review, nil
}

func (m *reviewServiceMock) Reject(_ context.Context, _ uuid.UUID, _, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) RejectReview(_ context.Context, id uuid.UUID, actor, reason, comment string) (*model.Review, error) {
	m.rejectID = id
	m.rejectActor = actor
	m.rejectReason = reason
	m.rejectComment = comment
	return m.review, nil
}

func (m *reviewServiceMock) MarkFalsePositive(_ context.Context, reviewID uuid.UUID, actor string, findingIDs []string, reason string) (*model.Review, error) {
	m.falsePositiveID = reviewID
	m.falsePositiveActor = actor
	m.falsePositiveIDs = append([]string(nil), findingIDs...)
	m.falsePositiveReason = reason
	return m.review, nil
}

func (m *reviewServiceMock) IngestCIResult(_ context.Context, req *model.CIReviewRequest) (*model.Review, error) {
	m.ciRequest = req
	return m.review, nil
}

func (m *reviewServiceMock) RequestHumanApproval(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *reviewServiceMock) RouteFixRequest(_ context.Context, id uuid.UUID) error {
	m.routeFixID = id
	return m.routeFixErr
}

type reviewValidator struct {
	validator *validator.Validate
}

func (v *reviewValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

func TestReviewHandler_Trigger_ValidatesRequest(t *testing.T) {
	e := echo.New()
	e.Validator = &reviewValidator{validator: validator.New()}
	svc := &reviewServiceMock{}
	h := handler.NewReviewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/trigger", strings.NewReader(`{"taskId":"","prUrl":"","trigger":"manual"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Trigger(c); err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestReviewHandler_Trigger_ReturnsAcceptedReview(t *testing.T) {
	e := echo.New()
	e.Validator = &reviewValidator{validator: validator.New()}
	now := time.Now()
	review := &model.Review{
		ID:             uuid.New(),
		TaskID:         uuid.New(),
		PRURL:          "https://github.com/acme/project/pull/11",
		PRNumber:       11,
		Layer:          model.ReviewLayerDeep,
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      model.ReviewRiskLevelMedium,
		Summary:        "Deep review completed",
		Recommendation: model.ReviewRecommendationApprove,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	svc := &reviewServiceMock{review: review}
	h := handler.NewReviewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/trigger", strings.NewReader(`{"taskId":"`+review.TaskID.String()+`","prUrl":"https://github.com/acme/project/pull/11","prNumber":11,"trigger":"agent","dimensions":["logic","security"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Trigger(c); err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if svc.triggered == nil || svc.triggered.Trigger != model.ReviewTriggerAgent {
		t.Fatalf("expected trigger request to be passed to service")
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if _, ok := payload["executionMetadata"]; ok {
		t.Fatalf("did not expect executionMetadata for review without metadata, got %#v", payload["executionMetadata"])
	}
}

func TestReviewHandler_Trigger_ExposesExecutionMetadataInDTO(t *testing.T) {
	e := echo.New()
	e.Validator = &reviewValidator{validator: validator.New()}
	now := time.Now()
	review := &model.Review{
		ID:       uuid.New(),
		TaskID:   uuid.New(),
		PRURL:    "https://github.com/acme/project/pull/14",
		PRNumber: 14,
		Layer:    model.ReviewLayerDeep,
		Status:   model.ReviewStatusCompleted,
		ExecutionMetadata: &model.ReviewExecutionMetadata{
			TriggerEvent: "pull_request.updated",
			ChangedFiles: []string{"src/server/routes.go"},
			Results: []model.ReviewExecutionResult{
				{ID: "security", Kind: model.ReviewExecutionKindBuiltinDimension, Status: model.ReviewExecutionStatusCompleted, Summary: "security ok"},
				{ID: "review.architecture", Kind: model.ReviewExecutionKindPlugin, Status: model.ReviewExecutionStatusFailed, Summary: "plugin failed", Error: "timeout"},
			},
		},
		RiskLevel:      model.ReviewRiskLevelHigh,
		Summary:        "Deep review completed",
		Recommendation: model.ReviewRecommendationRequestChanges,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	svc := &reviewServiceMock{review: review}
	h := handler.NewReviewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/trigger", strings.NewReader(`{"taskId":"`+review.TaskID.String()+`","prUrl":"https://github.com/acme/project/pull/14","prNumber":14,"trigger":"manual"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Trigger(c); err != nil {
		t.Fatalf("Trigger() error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	executionMetadata, ok := payload["executionMetadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected executionMetadata object, got %#v", payload["executionMetadata"])
	}
	if executionMetadata["triggerEvent"] != "pull_request.updated" {
		t.Fatalf("triggerEvent = %#v, want pull_request.updated", executionMetadata["triggerEvent"])
	}
	results, ok := executionMetadata["results"].([]any)
	if !ok || len(results) != 2 {
		t.Fatalf("results = %#v, want 2 items", executionMetadata["results"])
	}
}

func sampleReviewForHandlerTests() *model.Review {
	now := time.Now().UTC()
	return &model.Review{
		ID:             uuid.New(),
		TaskID:         uuid.New(),
		PRURL:          "https://github.com/acme/project/pull/21",
		PRNumber:       21,
		Layer:          model.ReviewLayerDeep,
		Status:         model.ReviewStatusCompleted,
		RiskLevel:      model.ReviewRiskLevelMedium,
		Summary:        "review completed",
		Recommendation: model.ReviewRecommendationApprove,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func TestReviewHandlerCRUDAndWorkflowEndpoints(t *testing.T) {
	e := echo.New()
	e.Validator = &reviewValidator{validator: validator.New()}
	review := sampleReviewForHandlerTests()
	secondReview := sampleReviewForHandlerTests()
	svc := &reviewServiceMock{review: review, reviews: []*model.Review{review, secondReview}}
	h := handler.NewReviewHandler(svc)

	completeReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+review.ID.String()+"/complete", strings.NewReader(`{"riskLevel":"medium","findings":[],"summary":"done","recommendation":"approve","costUsd":1.5}`))
	completeReq.Header.Set("Content-Type", "application/json")
	completeRec := httptest.NewRecorder()
	completeCtx := e.NewContext(completeReq, completeRec)
	completeCtx.SetPath("/api/v1/reviews/:id/complete")
	completeCtx.SetParamNames("id")
	completeCtx.SetParamValues(review.ID.String())
	if err := h.Complete(completeCtx); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	if completeRec.Code != http.StatusOK || svc.completeID != review.ID || svc.completed == nil {
		t.Fatalf("Complete() status/input = %d / %s / %#v", completeRec.Code, svc.completeID, svc.completed)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews/"+review.ID.String(), nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPath("/api/v1/reviews/:id")
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(review.ID.String())
	if err := h.Get(getCtx); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if getRec.Code != http.StatusOK || svc.getID != review.ID {
		t.Fatalf("Get() status/id = %d / %s", getRec.Code, svc.getID)
	}

	listTaskReq := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+review.TaskID.String()+"/reviews", nil)
	listTaskRec := httptest.NewRecorder()
	listTaskCtx := e.NewContext(listTaskReq, listTaskRec)
	listTaskCtx.SetPath("/api/v1/tasks/:taskId/reviews")
	listTaskCtx.SetParamNames("taskId")
	listTaskCtx.SetParamValues(review.TaskID.String())
	if err := h.ListByTask(listTaskCtx); err != nil {
		t.Fatalf("ListByTask() error = %v", err)
	}
	if listTaskRec.Code != http.StatusOK || svc.getTaskID != review.TaskID {
		t.Fatalf("ListByTask() status/id = %d / %s", listTaskRec.Code, svc.getTaskID)
	}

	listAllReq := httptest.NewRequest(http.MethodGet, "/api/v1/reviews?status=completed&riskLevel=medium&limit=10", nil)
	listAllRec := httptest.NewRecorder()
	listAllCtx := e.NewContext(listAllReq, listAllRec)
	if err := h.ListAll(listAllCtx); err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}
	if listAllRec.Code != http.StatusOK || svc.listStatus != "completed" || svc.listRiskLevel != "medium" || svc.listLimit != 10 {
		t.Fatalf("ListAll() captured = %q / %q / %d", svc.listStatus, svc.listRiskLevel, svc.listLimit)
	}

	approveReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+review.ID.String()+"/approve", strings.NewReader(`{"comment":"ship it"}`))
	approveReq.Header.Set("Content-Type", "application/json")
	approveRec := httptest.NewRecorder()
	approveCtx := e.NewContext(approveReq, approveRec)
	approveCtx.SetPath("/api/v1/reviews/:id/approve")
	approveCtx.SetParamNames("id")
	approveCtx.SetParamValues(review.ID.String())
	approveCtx.Set("user_id", " reviewer ")
	if err := h.Approve(approveCtx); err != nil {
		t.Fatalf("Approve() error = %v", err)
	}
	if approveRec.Code != http.StatusOK || svc.approveActor != "reviewer" || svc.approveComment != "ship it" {
		t.Fatalf("Approve() actor/comment = %q / %q", svc.approveActor, svc.approveComment)
	}

	rejectReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+review.ID.String()+"/reject", strings.NewReader(`{"reason":"security issue","comment":"fix before merge"}`))
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectRec := httptest.NewRecorder()
	rejectCtx := e.NewContext(rejectReq, rejectRec)
	rejectCtx.SetPath("/api/v1/reviews/:id/reject")
	rejectCtx.SetParamNames("id")
	rejectCtx.SetParamValues(review.ID.String())
	rejectCtx.Set("sub", "review-api")
	if err := h.Reject(rejectCtx); err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejectRec.Code != http.StatusOK || svc.rejectReason != "security issue" || svc.rejectActor != "review-api" {
		t.Fatalf("Reject() captured = %q / %q", svc.rejectReason, svc.rejectActor)
	}

	ciReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/ci-result", strings.NewReader(`{"prUrl":"https://github.com/acme/project/pull/21","status":"completed","findings":[]}`))
	ciReq.Header.Set("Content-Type", "application/json")
	ciRec := httptest.NewRecorder()
	ciCtx := e.NewContext(ciReq, ciRec)
	if err := h.IngestCIResult(ciCtx); err != nil {
		t.Fatalf("IngestCIResult() error = %v", err)
	}
	if ciRec.Code != http.StatusCreated || svc.ciRequest == nil || svc.ciRequest.PRURL == "" {
		t.Fatalf("IngestCIResult() status/request = %d / %#v", ciRec.Code, svc.ciRequest)
	}

	requestChangesReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+review.ID.String()+"/request-changes", strings.NewReader(`{"comment":"needs tests"}`))
	requestChangesReq.Header.Set("Content-Type", "application/json")
	requestChangesRec := httptest.NewRecorder()
	requestChangesCtx := e.NewContext(requestChangesReq, requestChangesRec)
	requestChangesCtx.SetPath("/api/v1/reviews/:id/request-changes")
	requestChangesCtx.SetParamNames("id")
	requestChangesCtx.SetParamValues(review.ID.String())
	if err := h.RequestChanges(requestChangesCtx); err != nil {
		t.Fatalf("RequestChanges() error = %v", err)
	}
	if requestChangesRec.Code != http.StatusOK || svc.requestChangesComment != "needs tests" || svc.routeFixID != review.ID {
		t.Fatalf("RequestChanges() captured = %q / %s", svc.requestChangesComment, svc.routeFixID)
	}

	falsePositiveReq := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+review.ID.String()+"/false-positive", strings.NewReader(`{"findingIds":["f-1","f-2"],"reason":"expected behavior"}`))
	falsePositiveReq.Header.Set("Content-Type", "application/json")
	falsePositiveRec := httptest.NewRecorder()
	falsePositiveCtx := e.NewContext(falsePositiveReq, falsePositiveRec)
	falsePositiveCtx.SetPath("/api/v1/reviews/:id/false-positive")
	falsePositiveCtx.SetParamNames("id")
	falsePositiveCtx.SetParamValues(review.ID.String())
	falsePositiveCtx.Set("uid", "ops-user")
	if err := h.MarkFalsePositive(falsePositiveCtx); err != nil {
		t.Fatalf("MarkFalsePositive() error = %v", err)
	}
	if falsePositiveRec.Code != http.StatusOK || svc.falsePositiveActor != "ops-user" || len(svc.falsePositiveIDs) != 2 {
		t.Fatalf("MarkFalsePositive() captured = %q / %#v", svc.falsePositiveActor, svc.falsePositiveIDs)
	}
}

func TestReviewHandlerRequestChangesReturnsInternalErrorWhenRouteFixFails(t *testing.T) {
	e := echo.New()
	e.Validator = &reviewValidator{validator: validator.New()}
	review := sampleReviewForHandlerTests()
	svc := &reviewServiceMock{review: review, routeFixErr: errors.New("route fix failed")}
	h := handler.NewReviewHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/reviews/"+review.ID.String()+"/request-changes", strings.NewReader(`{"comment":"needs work"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/reviews/:id/request-changes")
	c.SetParamNames("id")
	c.SetParamValues(review.ID.String())
	if err := h.RequestChanges(c); err != nil {
		t.Fatalf("RequestChanges() error = %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}
