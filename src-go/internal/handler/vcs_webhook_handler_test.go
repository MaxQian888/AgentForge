package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
)

// --- stubs ---

type stubIntegrations struct {
	integ *model.VCSIntegration
	err   error
}

func (s *stubIntegrations) ResolveByRepo(_ context.Context, _, _, _ string) (*model.VCSIntegration, error) {
	return s.integ, s.err
}

type stubSecrets struct {
	value string
	err   error
}

func (s *stubSecrets) Resolve(_ context.Context, _ uuid.UUID, _ string) (string, error) {
	return s.value, s.err
}

type stubRouter struct {
	called bool
	err    error
}

func (s *stubRouter) RouteEvent(_ context.Context, _ *model.VCSIntegration, _, _ string, _ []byte) error {
	s.called = true
	return s.err
}

type stubEventsRepo struct {
	insertErr     error
	insertedCount int
}

func (s *stubEventsRepo) Insert(_ context.Context, _ *model.VCSWebhookEvent) error {
	s.insertedCount++
	return s.insertErr
}
func (s *stubEventsRepo) MarkProcessed(_ context.Context, _ uuid.UUID, _ string) error { return nil }

// --- test helpers ---

func sign(secret, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func newCtx(t *testing.T, body []byte, sig, event, delivery string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vcs/github/webhook", bytes.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", event)
	req.Header.Set("X-GitHub-Delivery", delivery)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	// Mimic the CaptureRawBody middleware contract.
	c.Set(middleware.RawBodyKey, body)
	return c, rec
}

// --- tests ---

func TestWebhook_ValidSignature_RoutesAndAccepts(t *testing.T) {
	body := []byte(`{"action":"opened","pull_request":{"number":1,"head":{"sha":"a"},"base":{"sha":"b"},"html_url":"https://github.com/o/r/pull/1"},"repository":{"owner":{"login":"o"},"name":"r"}}`)
	secret := []byte("s3cr3t")
	integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Host: "github.com", Owner: "o", Repo: "r", WebhookSecretRef: "vcs.github.x.webhook"}
	router := &stubRouter{}
	repo := &stubEventsRepo{}
	h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: integ}, &stubSecrets{value: string(secret)}, router, repo, nil)

	c, rec := newCtx(t, body, sign(secret, body), "pull_request", "delivery-1")
	if err := h.HandleGitHubWebhook(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d, body: %s", rec.Code, rec.Body.String())
	}
	if !router.called {
		t.Fatal("router not invoked")
	}
	if repo.insertedCount != 1 {
		t.Fatal("event not persisted")
	}
}

func TestWebhook_BadSignature_401(t *testing.T) {
	body := []byte(`{"action":"opened","repository":{"owner":{"login":"o"},"name":"r"}}`)
	integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Host: "github.com", Owner: "o", Repo: "r"}
	h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: integ}, &stubSecrets{value: "right"}, &stubRouter{}, &stubEventsRepo{}, nil)
	c, rec := newCtx(t, body, sign([]byte("WRONG"), body), "pull_request", "d")
	_ = h.HandleGitHubWebhook(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWebhook_MissingSignature_401(t *testing.T) {
	body := []byte(`{"repository":{"owner":{"login":"o"},"name":"r"}}`)
	h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New()}}, &stubSecrets{value: "s"}, &stubRouter{}, &stubEventsRepo{}, nil)
	c, rec := newCtx(t, body, "", "pull_request", "d")
	_ = h.HandleGitHubWebhook(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWebhook_DuplicateEvent_200NoOp(t *testing.T) {
	body := []byte(`{"action":"opened","pull_request":{"number":1,"head":{"sha":"a"},"base":{"sha":"b"},"html_url":"x"},"repository":{"owner":{"login":"o"},"name":"r"}}`)
	secret := []byte("s")
	integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Host: "github.com", Owner: "o", Repo: "r"}
	router := &stubRouter{}
	repo := &stubEventsRepo{insertErr: repository.ErrVCSWebhookEventDuplicate}
	h := handler.NewVCSWebhookHandler(&stubIntegrations{integ: integ}, &stubSecrets{value: string(secret)}, router, repo, nil)

	c, rec := newCtx(t, body, sign(secret, body), "pull_request", "delivery-1")
	if err := h.HandleGitHubWebhook(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("dup expected 200 noop, got %d", rec.Code)
	}
	if router.called {
		t.Fatal("router must NOT be invoked on dup")
	}
}

func TestWebhook_IntegrationNotFound_404(t *testing.T) {
	body := []byte(`{"repository":{"owner":{"login":"o"},"name":"r"}}`)
	h := handler.NewVCSWebhookHandler(
		&stubIntegrations{err: repository.ErrVCSIntegrationNotFound},
		&stubSecrets{value: "s"},
		&stubRouter{},
		&stubEventsRepo{},
		nil,
	)
	c, rec := newCtx(t, body, sign([]byte("s"), body), "pull_request", "d")
	_ = h.HandleGitHubWebhook(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestWebhook_NoRepoInPayload_AcceptedNoop(t *testing.T) {
	body := []byte(`{"zen":"hello"}`)
	h := handler.NewVCSWebhookHandler(
		&stubIntegrations{err: errors.New("should not be called")},
		&stubSecrets{value: "s"},
		&stubRouter{},
		&stubEventsRepo{},
		nil,
	)
	c, rec := newCtx(t, body, sign([]byte("s"), body), "ping", "d")
	if err := h.HandleGitHubWebhook(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d", rec.Code)
	}
}
