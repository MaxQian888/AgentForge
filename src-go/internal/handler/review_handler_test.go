package handler_test

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
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
)

type reviewServiceMock struct {
	triggered *model.TriggerReviewRequest
	completed *model.CompleteReviewRequest
	review    *model.Review
	reviews   []*model.Review
}

func (m *reviewServiceMock) Trigger(_ context.Context, req *model.TriggerReviewRequest) (*model.Review, error) {
	m.triggered = req
	return m.review, nil
}

func (m *reviewServiceMock) Complete(_ context.Context, _ uuid.UUID, req *model.CompleteReviewRequest) (*model.Review, error) {
	m.completed = req
	return m.review, nil
}

func (m *reviewServiceMock) GetByID(_ context.Context, _ uuid.UUID) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) GetByTask(_ context.Context, _ uuid.UUID) ([]*model.Review, error) {
	return m.reviews, nil
}

func (m *reviewServiceMock) ListAll(_ context.Context, _, _ string, _ int) ([]*model.Review, error) {
	return m.reviews, nil
}

func (m *reviewServiceMock) Approve(_ context.Context, _ uuid.UUID, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) ApproveReview(_ context.Context, _ uuid.UUID, _, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) RequestChangesReview(_ context.Context, _ uuid.UUID, _, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) Reject(_ context.Context, _ uuid.UUID, _, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) RejectReview(_ context.Context, _ uuid.UUID, _, _, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) MarkFalsePositive(_ context.Context, _ uuid.UUID, _ string, _ []string, _ string) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) IngestCIResult(_ context.Context, _ *model.CIReviewRequest) (*model.Review, error) {
	return m.review, nil
}

func (m *reviewServiceMock) RequestHumanApproval(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *reviewServiceMock) RouteFixRequest(_ context.Context, _ uuid.UUID) error {
	return nil
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
