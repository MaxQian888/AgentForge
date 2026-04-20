package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// --- stubs ---

type stubSpawner struct {
	calls []uuid.UUID // findingIDs
}

func (s *stubSpawner) Spawn(_ echo.Context, _, findingID uuid.UUID, _ string) (uuid.UUID, error) {
	s.calls = append(s.calls, findingID)
	return uuid.New(), nil
}

type stubDecisionRepo struct {
	decisions map[uuid.UUID]string
}

func newStubDecisionRepo() *stubDecisionRepo {
	return &stubDecisionRepo{decisions: make(map[uuid.UUID]string)}
}

func (s *stubDecisionRepo) UpdateFindingDecision(_ echo.Context, id uuid.UUID, decision string, _ bool) error {
	s.decisions[id] = decision
	return nil
}

type stubEventPub struct {
	dismissed []uuid.UUID
}

func (s *stubEventPub) PublishFindingDismissed(_ echo.Context, id uuid.UUID, _ string) error {
	s.dismissed = append(s.dismissed, id)
	return nil
}

type stubAuditWriter struct {
	entries []map[string]any
}

func (s *stubAuditWriter) Append(_ echo.Context, _ string, _ string, payload map[string]any) error {
	s.entries = append(s.entries, payload)
	return nil
}

type stubRBAC struct {
	failForRole string
}

func (s *stubRBAC) RequireRole(_ echo.Context, role string) error {
	if s.failForRole == role {
		return echo.ErrForbidden
	}
	return nil
}

// --- echoValidator ---

type echoValidator struct{}

func (v *echoValidator) Validate(i interface{}) error {
	// Minimal validation: check that required fields are present
	if req, ok := i.(DecisionRequest); ok {
		switch req.Action {
		case "approve", "dismiss", "defer":
			return nil
		default:
			return echo.NewHTTPError(http.StatusBadRequest, "invalid action")
		}
	}
	return nil
}

func setupDecisionHandler() (*FindingsDecisionHandler, *stubSpawner, *stubDecisionRepo, *stubEventPub, *stubAuditWriter) {
	sp := &stubSpawner{}
	repo := newStubDecisionRepo()
	ev := &stubEventPub{}
	aw := &stubAuditWriter{}
	h := NewFindingsDecisionHandler(sp, aw, repo, ev, nil)
	return h, sp, repo, ev, aw
}

func decisionEcho(method, path, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = &echoValidator{}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func TestDecide_ApproveSpawnsCodeFixer(t *testing.T) {
	h, sp, repo, _, aw := setupDecisionHandler()
	fid := uuid.New()
	c, rec := decisionEcho("POST", "/api/v1/findings/"+fid.String()+"/decision", `{"action":"approve"}`)
	c.SetParamNames("id")
	c.SetParamValues(fid.String())

	if err := h.Decide(c); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", rec.Code)
	}
	if len(sp.calls) != 1 {
		t.Fatalf("spawner calls = %d, want 1", len(sp.calls))
	}
	if repo.decisions[fid] != "approved" {
		t.Errorf("decision = %q, want approved", repo.decisions[fid])
	}
	if len(aw.entries) != 1 {
		t.Errorf("audit entries = %d, want 1", len(aw.entries))
	}
}

func TestDecide_DismissSetsFlagAndEmits(t *testing.T) {
	h, _, repo, ev, _ := setupDecisionHandler()
	fid := uuid.New()
	c, rec := decisionEcho("POST", "/", `{"action":"dismiss"}`)
	c.SetParamNames("id")
	c.SetParamValues(fid.String())

	if err := h.Decide(c); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if repo.decisions[fid] != "dismissed" {
		t.Errorf("decision = %q, want dismissed", repo.decisions[fid])
	}
	if len(ev.dismissed) != 1 {
		t.Errorf("dismissed events = %d, want 1", len(ev.dismissed))
	}
}

func TestDecide_DeferSetsDecisionOnly(t *testing.T) {
	h, sp, repo, ev, _ := setupDecisionHandler()
	fid := uuid.New()
	c, rec := decisionEcho("POST", "/", `{"action":"defer"}`)
	c.SetParamNames("id")
	c.SetParamValues(fid.String())

	if err := h.Decide(c); err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if repo.decisions[fid] != "deferred" {
		t.Errorf("decision = %q, want deferred", repo.decisions[fid])
	}
	if len(sp.calls) != 0 {
		t.Error("spawner should not be called on defer")
	}
	if len(ev.dismissed) != 0 {
		t.Error("no dismiss event should fire on defer")
	}
}

func TestDecide_RejectsUnknownAction(t *testing.T) {
	h, _, _, _, _ := setupDecisionHandler()
	fid := uuid.New()
	c, rec := decisionEcho("POST", "/", `{"action":"unknown"}`)
	c.SetParamNames("id")
	c.SetParamValues(fid.String())

	if err := h.Decide(c); err != nil {
		// echo may return error for validation
		return
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestDecide_AuditLogPerCall(t *testing.T) {
	h, _, _, _, aw := setupDecisionHandler()
	for _, action := range []string{"approve", "dismiss", "defer"} {
		fid := uuid.New()
		c, _ := decisionEcho("POST", "/", `{"action":"`+action+`"}`)
		c.SetParamNames("id")
		c.SetParamValues(fid.String())
		_ = h.Decide(c)
	}
	if len(aw.entries) != 3 {
		t.Errorf("audit entries = %d, want 3", len(aw.entries))
	}
}

func TestDecide_RBAC_ApproveDismissRequireEditor(t *testing.T) {
	sp := &stubSpawner{}
	repo := newStubDecisionRepo()
	ev := &stubEventPub{}
	aw := &stubAuditWriter{}
	rbac := &stubRBAC{failForRole: "editor"}
	h := NewFindingsDecisionHandler(sp, aw, repo, ev, rbac)

	// approve should be forbidden
	fid := uuid.New()
	c, rec := decisionEcho("POST", "/", `{"action":"approve"}`)
	c.SetParamNames("id")
	c.SetParamValues(fid.String())
	_ = h.Decide(c)
	if rec.Code != http.StatusForbidden {
		t.Errorf("approve status = %d, want 403", rec.Code)
	}

	// dismiss should be forbidden
	c2, rec2 := decisionEcho("POST", "/", `{"action":"dismiss"}`)
	c2.SetParamNames("id")
	c2.SetParamValues(fid.String())
	_ = h.Decide(c2)
	if rec2.Code != http.StatusForbidden {
		t.Errorf("dismiss status = %d, want 403", rec2.Code)
	}

	// defer should be allowed (viewer level)
	c3, rec3 := decisionEcho("POST", "/", `{"action":"defer"}`)
	c3.SetParamNames("id")
	c3.SetParamValues(fid.String())
	_ = h.Decide(c3)
	if rec3.Code != http.StatusOK {
		t.Errorf("defer status = %d, want 200", rec3.Code)
	}
}
