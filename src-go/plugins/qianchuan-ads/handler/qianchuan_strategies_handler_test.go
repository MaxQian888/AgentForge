package qchandler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	handler "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/handler"
	"github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/strategy"
	"github.com/react-go-quick-starter/server/internal/repository"
	service "github.com/react-go-quick-starter/server/internal/service"
	qcservice "github.com/react-go-quick-starter/server/plugins/qianchuan-ads/service"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

const validQianchuanYAML = `
name: q-test
triggers:
  schedule: 1m
inputs:
  - metric: cost
    dimensions: [ad_id]
    window: 1m
rules:
  - name: hb
    condition: "true"
    actions:
      - type: notify_im
        target: {}
        params:
          channel: default
          template: "tick"
`

type fakeQianchuanService struct {
	strategies map[uuid.UUID]*strategy.QianchuanStrategy
	listErr    error
	createErr  error
	updateErr  error
}

func newFakeSvc() *fakeQianchuanService {
	return &fakeQianchuanService{strategies: map[uuid.UUID]*strategy.QianchuanStrategy{}}
}

func (f *fakeQianchuanService) Create(_ context.Context, in qcservice.QianchuanStrategyCreateInput) (*strategy.QianchuanStrategy, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	row := &strategy.QianchuanStrategy{
		ID:         uuid.New(),
		ProjectID:  in.ProjectID,
		Name:       "q-test",
		YAMLSource: in.YAMLSource,
		ParsedSpec: `{"schema_version":1}`,
		Version:    1,
		Status:     strategy.StatusDraft,
		CreatedBy:  in.CreatedBy,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	f.strategies[row.ID] = row
	return row, nil
}

func (f *fakeQianchuanService) Update(_ context.Context, id uuid.UUID, yamlSource string) (*strategy.QianchuanStrategy, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	row, ok := f.strategies[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if row.Status != strategy.StatusDraft {
		return nil, qcservice.ErrStrategyImmutable
	}
	row.YAMLSource = yamlSource
	return row, nil
}

func (f *fakeQianchuanService) Publish(_ context.Context, id uuid.UUID) error {
	row, ok := f.strategies[id]
	if !ok {
		return repository.ErrNotFound
	}
	if row.IsSystem() {
		return qcservice.ErrStrategySystemReadOnly
	}
	if row.Status != strategy.StatusDraft {
		return qcservice.ErrStrategyInvalidTransition
	}
	row.Status = strategy.StatusPublished
	return nil
}

func (f *fakeQianchuanService) Archive(_ context.Context, id uuid.UUID) error {
	row, ok := f.strategies[id]
	if !ok {
		return repository.ErrNotFound
	}
	if row.IsSystem() {
		return qcservice.ErrStrategySystemReadOnly
	}
	if row.Status != strategy.StatusPublished {
		return qcservice.ErrStrategyInvalidTransition
	}
	row.Status = strategy.StatusArchived
	return nil
}

func (f *fakeQianchuanService) Delete(_ context.Context, id uuid.UUID) error {
	row, ok := f.strategies[id]
	if !ok {
		return repository.ErrNotFound
	}
	if row.IsSystem() {
		return qcservice.ErrStrategySystemReadOnly
	}
	if row.Status != strategy.StatusDraft {
		return qcservice.ErrStrategyImmutable
	}
	delete(f.strategies, id)
	return nil
}

func (f *fakeQianchuanService) Get(_ context.Context, id uuid.UUID) (*strategy.QianchuanStrategy, error) {
	row, ok := f.strategies[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return row, nil
}

func (f *fakeQianchuanService) List(_ context.Context, _ uuid.UUID) ([]*strategy.QianchuanStrategy, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]*strategy.QianchuanStrategy, 0, len(f.strategies))
	for _, r := range f.strategies {
		out = append(out, r)
	}
	return out, nil
}

func (f *fakeQianchuanService) TestRun(_ context.Context, id uuid.UUID, snapshot map[string]any) (*qcservice.TestRunResult, error) {
	if _, ok := f.strategies[id]; !ok {
		return nil, repository.ErrNotFound
	}
	return &qcservice.TestRunResult{
		FiredRules: []string{"hb"},
		Actions:    []qcservice.TestRunResolvedAction{{Rule: "hb", Type: "notify_im", Params: snapshot}},
	}, nil
}

func newTestEchoWithUser(userID uuid.UUID) *echo.Echo {
	e := echo.New()
	// Inject claims so claimsUserID succeeds. We bypass JWT entirely.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(middleware.JWTContextKey, &service.Claims{
				UserID:           userID.String(),
				RegisteredClaims: jwtv5.RegisteredClaims{},
			})
			return next(c)
		}
	})
	return e
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(into); err != nil {
		t.Fatalf("decode body: %v\nbody=%s", err, rec.Body.String())
	}
}

func TestQianchuanHandler_ListAndStatusFilter(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	svc.strategies[uuid.New()] = &strategy.QianchuanStrategy{
		ID: uuid.New(), ProjectID: &pid, Name: "a", Status: strategy.StatusDraft,
		Version: 1, CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	svc.strategies[uuid.New()] = &strategy.QianchuanStrategy{
		ID: uuid.New(), ProjectID: &pid, Name: "b", Status: strategy.StatusPublished,
		Version: 1, CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	// List all.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid.String()+"/qianchuan/strategies", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var all []handler.QianchuanStrategyDTO
	decodeBody(t, rec, &all)
	if len(all) != 2 {
		t.Errorf("len(all): got %d want 2", len(all))
	}

	// Status filter narrows.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid.String()+"/qianchuan/strategies?status=published", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status filter: got %d body=%s", rec.Code, rec.Body.String())
	}
	var filtered []handler.QianchuanStrategyDTO
	decodeBody(t, rec, &filtered)
	if len(filtered) != 1 || filtered[0].Status != "published" {
		t.Errorf("filter result: %+v", filtered)
	}
}

func TestQianchuanHandler_CreateValidYAML(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	body, _ := json.Marshal(map[string]any{"yamlSource": validQianchuanYAML})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid.String()+"/qianchuan/strategies", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var dto handler.QianchuanStrategyDTO
	decodeBody(t, rec, &dto)
	if dto.Name != "q-test" {
		t.Errorf("name: got %q", dto.Name)
	}
}

func TestQianchuanHandler_CreateInvalidYAMLReturnsStructuredError(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	svc.createErr = &strategy.StrategyParseError{Line: 3, Col: 5, Field: "rules[0].condition", Msg: "must be non-empty"}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	body, _ := json.Marshal(map[string]any{"yamlSource": "anything"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid.String()+"/qianchuan/strategies", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d", rec.Code)
	}
	var resp struct {
		Error strategy.StrategyParseError `json:"error"`
	}
	decodeBody(t, rec, &resp)
	if resp.Error.Line != 3 || resp.Error.Col != 5 || resp.Error.Field != "rules[0].condition" {
		t.Errorf("structured error: %+v", resp.Error)
	}
}

func TestQianchuanHandler_UpdatePublishedReturns409(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	id := uuid.New()
	svc.strategies[id] = &strategy.QianchuanStrategy{
		ID: id, ProjectID: &pid, Name: "x", Status: strategy.StatusPublished, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	body, _ := json.Marshal(map[string]any{"yamlSource": validQianchuanYAML})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/qianchuan/strategies/"+id.String(), strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestQianchuanHandler_PublishLifecycle(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	id := uuid.New()
	svc.strategies[id] = &strategy.QianchuanStrategy{
		ID: id, ProjectID: &pid, Name: "x", Status: strategy.StatusDraft, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	// First publish OK.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qianchuan/strategies/"+id.String()+"/publish", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish: got %d body=%s", rec.Code, rec.Body.String())
	}
	// Second publish 409.
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, httptest.NewRequest(http.MethodPost, "/api/v1/qianchuan/strategies/"+id.String()+"/publish", nil))
	if rec2.Code != http.StatusConflict {
		t.Fatalf("second publish: got %d", rec2.Code)
	}
}

func TestQianchuanHandler_ArchiveOnlyAfterPublish(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	id := uuid.New()
	svc.strategies[id] = &strategy.QianchuanStrategy{
		ID: id, ProjectID: &pid, Name: "x", Status: strategy.StatusDraft, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	// Archive on draft -> 409.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/qianchuan/strategies/"+id.String()+"/archive", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("archive draft: got %d", rec.Code)
	}
}

func TestQianchuanHandler_DeleteDraftAndPublishedRules(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	draftID := uuid.New()
	publishedID := uuid.New()
	svc.strategies[draftID] = &strategy.QianchuanStrategy{
		ID: draftID, ProjectID: &pid, Name: "d", Status: strategy.StatusDraft, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	svc.strategies[publishedID] = &strategy.QianchuanStrategy{
		ID: publishedID, ProjectID: &pid, Name: "p", Status: strategy.StatusPublished, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	// Delete draft -> 204.
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/qianchuan/strategies/"+draftID.String(), nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete draft: got %d body=%s", rec.Code, rec.Body.String())
	}
	// Delete published -> 409.
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, httptest.NewRequest(http.MethodDelete, "/api/v1/qianchuan/strategies/"+publishedID.String(), nil))
	if rec2.Code != http.StatusConflict {
		t.Fatalf("delete published: got %d", rec2.Code)
	}
}

func TestQianchuanHandler_TestRunReturnsActions(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	id := uuid.New()
	svc.strategies[id] = &strategy.QianchuanStrategy{
		ID: id, ProjectID: &pid, Name: "x", Status: strategy.StatusDraft, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	body, _ := json.Marshal(map[string]any{"snapshot": map[string]any{"metrics": map[string]any{"cost": 12.5}}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/qianchuan/strategies/"+id.String()+"/test", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d body=%s", rec.Code, rec.Body.String())
	}
	var res qcservice.TestRunResult
	decodeBody(t, rec, &res)
	if len(res.FiredRules) != 1 || res.FiredRules[0] != "hb" {
		t.Errorf("fired rules: %+v", res.FiredRules)
	}
	if len(res.Actions) != 1 || res.Actions[0].Type != "notify_im" {
		t.Errorf("actions: %+v", res.Actions)
	}
}

func TestQianchuanHandler_RejectsSystemRowWrites(t *testing.T) {
	user := uuid.New()
	svc := newFakeSvc()
	id := uuid.New()
	svc.strategies[id] = &strategy.QianchuanStrategy{
		ID: id, ProjectID: nil, Name: "system:x", Status: strategy.StatusPublished, Version: 1,
		CreatedBy: user, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	for _, target := range []string{"/publish", "/archive"} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/qianchuan/strategies/"+id.String()+target, nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s on system: got %d want 403", target, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/api/v1/qianchuan/strategies/"+id.String(), nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("delete system: got %d want 403", rec.Code)
	}
}

func TestQianchuanHandler_GetReturnsNotFound(t *testing.T) {
	user := uuid.New()
	svc := newFakeSvc()
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/qianchuan/strategies/"+uuid.New().String(), nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: got %d", rec.Code)
	}
}

// Smoke test that ErrNotFound mapping doesn't trigger on nil errors.
func TestQianchuanHandler_ListErrorPropagates(t *testing.T) {
	pid := uuid.New()
	user := uuid.New()
	svc := newFakeSvc()
	svc.listErr = errors.New("db down")
	h := handler.NewQianchuanStrategiesHandler(svc)
	e := newTestEchoWithUser(user)
	handler.RegisterQianchuanStrategyRoutes(e.Group("/api/v1"), h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid.String()+"/qianchuan/strategies", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d", rec.Code)
	}
}
