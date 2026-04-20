package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/imcards"
)

// ── stubs ────────────────────────────────────────────────────────────────

type handlerStubCorrelations struct {
	c       *imcards.Correlation
	lookErr error
	marked  []uuid.UUID
}

func (s *handlerStubCorrelations) Lookup(_ context.Context, _ uuid.UUID) (*imcards.Correlation, error) {
	if s.lookErr != nil {
		return nil, s.lookErr
	}
	return s.c, nil
}
func (s *handlerStubCorrelations) MarkConsumed(_ context.Context, t uuid.UUID) error {
	s.marked = append(s.marked, t)
	return nil
}

type handlerStubResumer struct{ retErr error }

func (s *handlerStubResumer) Resume(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) error {
	return s.retErr
}

type handlerStubFallback struct{}

func (s *handlerStubFallback) RouteAsIMEvent(_ context.Context, _ map[string]any) error { return nil }

type handlerStubAudit struct{}

func (s *handlerStubAudit) Record(_ context.Context, _ string, _ map[string]any) error { return nil }

// ── helpers ──────────────────────────────────────────────────────────────

func newCardActionsTestContext(body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/im/card-actions", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

// ── tests ────────────────────────────────────────────────────────────────

func TestIMCardActionsHandler_200Resumed(t *testing.T) {
	execID := uuid.New()
	token := uuid.New()
	router := &imcards.Router{
		Correlations: &handlerStubCorrelations{c: &imcards.Correlation{
			Token: token, ExecutionID: execID, NodeID: "wait-1", ActionID: "approve",
			ExpiresAt: time.Now().Add(time.Hour),
		}},
		Resumer: &handlerStubResumer{}, Fallback: &handlerStubFallback{}, Audit: &handlerStubAudit{},
	}
	h := NewIMCardActionsHandler(router)
	c, rec := newCardActionsTestContext(`{"correlation_token":"` + token.String() + `","action_id":"approve"}`)
	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestIMCardActionsHandler_200Fallback(t *testing.T) {
	router := &imcards.Router{
		Correlations: &handlerStubCorrelations{lookErr: imcards.ErrCorrelationNotFound},
		Resumer:      &handlerStubResumer{}, Fallback: &handlerStubFallback{}, Audit: &handlerStubAudit{},
	}
	h := NewIMCardActionsHandler(router)
	c, rec := newCardActionsTestContext(`{"correlation_token":"` + uuid.New().String() + `","action_id":"x"}`)
	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestIMCardActionsHandler_409Consumed(t *testing.T) {
	now := time.Now()
	token := uuid.New()
	router := &imcards.Router{
		Correlations: &handlerStubCorrelations{c: &imcards.Correlation{
			Token: token, ExpiresAt: time.Now().Add(time.Hour), ConsumedAt: &now,
		}},
		Resumer: &handlerStubResumer{}, Fallback: &handlerStubFallback{}, Audit: &handlerStubAudit{},
	}
	h := NewIMCardActionsHandler(router)
	c, rec := newCardActionsTestContext(`{"correlation_token":"` + token.String() + `"}`)
	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestIMCardActionsHandler_410Expired(t *testing.T) {
	token := uuid.New()
	router := &imcards.Router{
		Correlations: &handlerStubCorrelations{c: &imcards.Correlation{
			Token: token, ExpiresAt: time.Now().Add(-time.Minute),
		}},
		Resumer: &handlerStubResumer{}, Fallback: &handlerStubFallback{}, Audit: &handlerStubAudit{},
	}
	h := NewIMCardActionsHandler(router)
	c, rec := newCardActionsTestContext(`{"correlation_token":"` + token.String() + `"}`)
	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusGone {
		t.Errorf("status = %d, want 410", rec.Code)
	}
}

func TestIMCardActionsHandler_409NotWaiting(t *testing.T) {
	token := uuid.New()
	router := &imcards.Router{
		Correlations: &handlerStubCorrelations{c: &imcards.Correlation{
			Token: token, ExecutionID: uuid.New(), NodeID: "w", ActionID: "x",
			ExpiresAt: time.Now().Add(time.Hour),
		}},
		Resumer:  &handlerStubResumer{retErr: errors.New("wait_event: target node is not waiting")},
		Fallback: &handlerStubFallback{}, Audit: &handlerStubAudit{},
	}
	h := NewIMCardActionsHandler(router)
	c, rec := newCardActionsTestContext(`{"correlation_token":"` + token.String() + `"}`)
	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestIMCardActionsHandler_400BadUUID(t *testing.T) {
	router := &imcards.Router{
		Correlations: &handlerStubCorrelations{}, Resumer: &handlerStubResumer{},
		Fallback: &handlerStubFallback{}, Audit: &handlerStubAudit{},
	}
	h := NewIMCardActionsHandler(router)
	c, rec := newCardActionsTestContext(`{"correlation_token":"not-a-uuid"}`)
	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
