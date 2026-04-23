package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/secrets"
	"github.com/agentforge/server/internal/service"
)

type fakeSecretsSvc struct{ stored map[string]string }

func (f *fakeSecretsSvc) CreateSecret(_ echo.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error) {
	_ = actor
	f.stored[name] = plaintext
	return &secrets.Record{ProjectID: projectID, Name: name, Description: description, CreatedBy: actor}, nil
}
func (f *fakeSecretsSvc) RotateSecret(_ echo.Context, _ uuid.UUID, name, plaintext string, _ uuid.UUID) error {
	if _, ok := f.stored[name]; !ok {
		return secrets.ErrSecretNotFound
	}
	f.stored[name] = plaintext
	return nil
}
func (f *fakeSecretsSvc) DeleteSecret(_ echo.Context, _ uuid.UUID, name string, _ uuid.UUID) error {
	delete(f.stored, name)
	return nil
}
func (f *fakeSecretsSvc) ListSecrets(_ echo.Context, projectID uuid.UUID) ([]*secrets.Record, error) {
	out := []*secrets.Record{}
	for n := range f.stored {
		out = append(out, &secrets.Record{ProjectID: projectID, Name: n})
	}
	return out, nil
}

func setupSecretsCtx(t *testing.T, method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, uuid.New())
	c.Set(appMiddleware.JWTContextKey, &service.Claims{UserID: uuid.New().String()})
	return c, rec
}

func TestSecretsHandler_CreateDoesNotReturnPlaintextValue(t *testing.T) {
	svc := &fakeSecretsSvc{stored: map[string]string{}}
	h := handler.NewSecretsHandler(svc)

	body := `{"name":"GITHUB_TOKEN","value":"ghp_xyz","description":"review token"}`
	c, rec := setupSecretsCtx(t, http.MethodPost, "/api/v1/projects/123/secrets", body)

	if err := h.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status: %d body=%s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if _, ok := resp["value"]; ok {
		t.Fatalf("create response leaked plaintext value: %+v", resp)
	}
}

func TestSecretsHandler_ListNeverReturnsValues(t *testing.T) {
	svc := &fakeSecretsSvc{stored: map[string]string{"TOKEN": "ghp_xyz"}}
	h := handler.NewSecretsHandler(svc)

	c, rec := setupSecretsCtx(t, http.MethodGet, "/api/v1/projects/123/secrets", "")

	if err := h.List(c); err != nil {
		t.Fatalf("List: %v", err)
	}
	if strings.Contains(rec.Body.String(), "ghp_xyz") {
		t.Fatalf("list response leaked value: %s", rec.Body.String())
	}
}

func TestSecretsHandler_RotateNotFound(t *testing.T) {
	svc := &fakeSecretsSvc{stored: map[string]string{}}
	h := handler.NewSecretsHandler(svc)

	body := `{"value":"new"}`
	c, rec := setupSecretsCtx(t, http.MethodPatch, "/api/v1/projects/123/secrets/MISSING", body)
	c.SetParamNames("name")
	c.SetParamValues("MISSING")

	if err := h.Rotate(c); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSecretsHandler_RotateDoesNotEchoPlaintextValue(t *testing.T) {
	svc := &fakeSecretsSvc{stored: map[string]string{"TOKEN": "old"}}
	h := handler.NewSecretsHandler(svc)

	body := `{"value":"new"}`
	c, rec := setupSecretsCtx(t, http.MethodPatch, "/api/v1/projects/123/secrets/TOKEN", body)
	c.SetParamNames("name")
	c.SetParamValues("TOKEN")

	if err := h.Rotate(c); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "new") {
		t.Fatalf("rotate response leaked plaintext value: %s", rec.Body.String())
	}
}

func TestSecretsHandler_DeleteNoContent(t *testing.T) {
	svc := &fakeSecretsSvc{stored: map[string]string{"TOKEN": "x"}}
	h := handler.NewSecretsHandler(svc)

	c, rec := setupSecretsCtx(t, http.MethodDelete, "/api/v1/projects/123/secrets/TOKEN", "")
	c.SetParamNames("name")
	c.SetParamValues("TOKEN")

	if err := h.Delete(c); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := svc.stored["TOKEN"]; ok {
		t.Errorf("expected delete to remove from stored")
	}
}
