package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/knowledge"
	"github.com/agentforge/server/internal/knowledge/liveartifact"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// --- stub service (local to this file) ---

type liveArtifactsStubSvc struct {
	assets        map[uuid.UUID]*model.KnowledgeAsset
	versions      map[uuid.UUID]*model.AssetVersion
	updateCalls   int
	versionCalls  int
	lastVersion   *model.AssetVersion
	lastUpdateCJS string
	// capturedAtVersion records the ContentJSON present on the asset at the
	// moment CreateVersion is invoked, so tests can assert the snapshot was
	// taken before Update mutated the asset.
	capturedAtVersion string
}

func newLiveArtifactsStubSvc(assets ...*model.KnowledgeAsset) *liveArtifactsStubSvc {
	s := &liveArtifactsStubSvc{
		assets:   make(map[uuid.UUID]*model.KnowledgeAsset),
		versions: make(map[uuid.UUID]*model.AssetVersion),
	}
	for _, a := range assets {
		s.assets[a.ID] = a
	}
	return s
}

func (s *liveArtifactsStubSvc) Create(_ context.Context, _ model.PrincipalContext, a *model.KnowledgeAsset) (*model.KnowledgeAsset, error) {
	a.ID = uuid.New()
	s.assets[a.ID] = a
	return a, nil
}
func (s *liveArtifactsStubSvc) Get(_ context.Context, pc model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error) {
	if !pc.CanRead() {
		return nil, knowledge.ErrAssetForbidden
	}
	a, ok := s.assets[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	return a, nil
}
func (s *liveArtifactsStubSvc) Update(_ context.Context, _ model.PrincipalContext, id uuid.UUID, req model.UpdateKnowledgeAssetRequest) (*model.KnowledgeAsset, error) {
	a, ok := s.assets[id]
	if !ok {
		return nil, knowledge.ErrAssetNotFound
	}
	s.updateCalls++
	if req.ContentJSON != "" {
		a.ContentJSON = req.ContentJSON
		s.lastUpdateCJS = req.ContentJSON
	}
	if req.Title != "" {
		a.Title = req.Title
	}
	return a, nil
}
func (s *liveArtifactsStubSvc) Delete(_ context.Context, _ model.PrincipalContext, _ uuid.UUID) error {
	return nil
}
func (s *liveArtifactsStubSvc) Restore(_ context.Context, _ model.PrincipalContext, id uuid.UUID) (*model.KnowledgeAsset, error) {
	return s.assets[id], nil
}
func (s *liveArtifactsStubSvc) List(_ context.Context, _ model.PrincipalContext, _ uuid.UUID, _ *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) ListTree(_ context.Context, _ model.PrincipalContext, _ uuid.UUID) ([]*model.KnowledgeAsset, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) Move(_ context.Context, _ model.PrincipalContext, id uuid.UUID, _ model.MoveKnowledgeAssetRequest) (*model.KnowledgeAsset, error) {
	return s.assets[id], nil
}
func (s *liveArtifactsStubSvc) ListVersions(_ context.Context, _ model.PrincipalContext, _ uuid.UUID) ([]*model.AssetVersion, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) CreateVersion(_ context.Context, _ model.PrincipalContext, assetID uuid.UUID, name string) (*model.AssetVersion, error) {
	s.versionCalls++
	// capture the asset's content at the instant the version is created.
	if a, ok := s.assets[assetID]; ok {
		s.capturedAtVersion = a.ContentJSON
	}
	v := &model.AssetVersion{ID: uuid.New(), AssetID: assetID, Name: name, ContentJSON: s.capturedAtVersion}
	s.versions[v.ID] = v
	s.lastVersion = v
	return v, nil
}
func (s *liveArtifactsStubSvc) GetVersion(_ context.Context, _ model.PrincipalContext, vid uuid.UUID) (*model.AssetVersion, error) {
	return s.versions[vid], nil
}
func (s *liveArtifactsStubSvc) RestoreVersion(_ context.Context, _ model.PrincipalContext, assetID, versionID uuid.UUID) (*model.KnowledgeAsset, *model.AssetVersion, error) {
	return s.assets[assetID], s.versions[versionID], nil
}
func (s *liveArtifactsStubSvc) ListComments(_ context.Context, _ model.PrincipalContext, _ uuid.UUID) ([]*model.AssetComment, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) CreateComment(_ context.Context, _ model.PrincipalContext, _ uuid.UUID, _ model.CreateAssetCommentRequest) (*model.AssetComment, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) UpdateComment(_ context.Context, _ model.PrincipalContext, _, _ uuid.UUID, _ model.UpdateAssetCommentRequest) (*model.AssetComment, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) DeleteComment(_ context.Context, _ model.PrincipalContext, _, _ uuid.UUID) error {
	return nil
}
func (s *liveArtifactsStubSvc) Search(_ context.Context, _ model.PrincipalContext, _ uuid.UUID, _ string, _ *model.KnowledgeAssetKind, _ int) ([]*model.KnowledgeSearchResult, error) {
	return nil, nil
}
func (s *liveArtifactsStubSvc) MaterializeAsWiki(_ context.Context, _ model.PrincipalContext, _ uuid.UUID, _ model.MaterializeAsWikiRequest) (*model.KnowledgeAsset, error) {
	return nil, nil
}

// --- stub projectors ---

type fakeProjector struct {
	kind     liveartifact.LiveArtifactKind
	result   liveartifact.ProjectionResult
	err      error
	sleep    time.Duration
	callBack func()
}

func (f *fakeProjector) Kind() liveartifact.LiveArtifactKind { return f.kind }
func (f *fakeProjector) RequiredRole() liveartifact.Role     { return liveartifact.RoleViewer }
func (f *fakeProjector) Project(_ context.Context, _ model.PrincipalContext, _ uuid.UUID, _ json.RawMessage, _ json.RawMessage) (liveartifact.ProjectionResult, error) {
	if f.sleep > 0 {
		time.Sleep(f.sleep)
	}
	if f.callBack != nil {
		f.callBack()
	}
	return f.result, f.err
}
func (f *fakeProjector) Subscribe(_ json.RawMessage) []liveartifact.EventTopic { return nil }

// --- helpers ---

func newHandlerWithLiveArtifacts(svc *liveArtifactsStubSvc, reg *liveartifact.Registry) *KnowledgeAssetHandler {
	return (&KnowledgeAssetHandler{svc: svc}).WithLiveArtifactRegistry(reg)
}

func setRouteParams(c echo.Context, names []string, values []string) {
	c.SetParamNames(names...)
	c.SetParamValues(values...)
}

func seedAsset(projectID uuid.UUID, content string) *model.KnowledgeAsset {
	spaceID := uuid.New()
	return &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: content,
		Title:       "Hello",
		Version:     1,
	}
}

// principalEditor returns a principal with editor role; used via context set
// in the tests. We don't set JWT claims, so resolvePrincipal returns a
// default editor principal. This mirrors how the wiring sees anonymous-dev
// requests.

// --- Project endpoint tests ---

func TestProjectLiveArtifacts_Mixed(t *testing.T) {
	projectID := uuid.New()
	asset := seedAsset(projectID, `[]`)
	svc := newLiveArtifactsStubSvc(asset)

	okFragment := json.RawMessage(`[{"id":"frag1","type":"paragraph","content":[{"type":"text","text":"x","styles":{}}]}]`)
	reg := liveartifact.NewRegistry()
	reg.Register(&fakeProjector{kind: liveartifact.KindAgentRun, result: liveartifact.ProjectionResult{Status: liveartifact.StatusOK, Projection: okFragment, ProjectedAt: time.Now().UTC()}})
	reg.Register(&fakeProjector{kind: liveartifact.KindReview, result: liveartifact.ProjectionResult{Status: liveartifact.StatusNotFound, ProjectedAt: time.Now().UTC()}})
	reg.Register(&fakeProjector{kind: liveartifact.KindCostSummary, result: liveartifact.ProjectionResult{Status: liveartifact.StatusForbidden, ProjectedAt: time.Now().UTC()}})

	body := `{
		"blocks": [
			{"block_id":"b-ok","live_kind":"agent_run","target_ref":{"id":"run-1"},"view_opts":{}},
			{"block_id":"b-nf","live_kind":"review","target_ref":{},"view_opts":{}},
			{"block_id":"b-fb","live_kind":"cost_summary","target_ref":{},"view_opts":{}},
			{"block_id":"b-unk","live_kind":"does_not_exist","target_ref":{},"view_opts":{}}
		]
	}`

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id"}, []string{asset.ID.String()})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.ProjectLiveArtifacts(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp projectLiveArtifactsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ok := resp.Results["b-ok"]
	if ok.Status != liveartifact.StatusOK {
		t.Fatalf("b-ok status: %s", ok.Status)
	}
	if len(ok.Projection) == 0 {
		t.Fatalf("b-ok should carry projection")
	}
	if nf := resp.Results["b-nf"]; nf.Status != liveartifact.StatusNotFound {
		t.Fatalf("b-nf status: %s", nf.Status)
	} else if len(nf.Projection) != 0 {
		t.Fatalf("b-nf should not carry projection")
	}
	if fb := resp.Results["b-fb"]; fb.Status != liveartifact.StatusForbidden {
		t.Fatalf("b-fb status: %s", fb.Status)
	} else if len(fb.Projection) != 0 {
		t.Fatalf("b-fb should not carry projection")
	}
	if unk := resp.Results["b-unk"]; unk.Status != liveartifact.StatusNotFound {
		t.Fatalf("b-unk status: %s", unk.Status)
	} else if unk.Diagnostics == "" {
		t.Fatalf("b-unk should carry diagnostics")
	}
}

func TestProjectLiveArtifacts_TooManyBlocks(t *testing.T) {
	projectID := uuid.New()
	asset := seedAsset(projectID, `[]`)
	svc := newLiveArtifactsStubSvc(asset)
	reg := liveartifact.NewRegistry()

	blocks := make([]map[string]any, 0, 51)
	for i := 0; i < 51; i++ {
		blocks = append(blocks, map[string]any{
			"block_id":   "b-" + uuid.NewString(),
			"live_kind":  "agent_run",
			"target_ref": map[string]any{},
			"view_opts":  map[string]any{},
		})
	}
	bodyBytes, _ := json.Marshal(map[string]any{"blocks": blocks})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(bodyBytes)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id"}, []string{asset.ID.String()})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.ProjectLiveArtifacts(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "too many blocks") {
		t.Fatalf("expected too-many-blocks message, got %s", rec.Body.String())
	}
}

func TestProjectLiveArtifacts_AssetNotOwnedByProject(t *testing.T) {
	otherProjectID := uuid.New()
	requestProjectID := uuid.New()
	asset := seedAsset(otherProjectID, `[]`)
	svc := newLiveArtifactsStubSvc(asset)
	reg := liveartifact.NewRegistry()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"blocks":[]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id"}, []string{asset.ID.String()})
	c.Set("project_id", requestProjectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.ProjectLiveArtifacts(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectLiveArtifacts_PerformanceBatchOf10(t *testing.T) {
	projectID := uuid.New()
	asset := seedAsset(projectID, `[]`)
	svc := newLiveArtifactsStubSvc(asset)

	reg := liveartifact.NewRegistry()
	reg.Register(&fakeProjector{
		kind:   liveartifact.KindAgentRun,
		result: liveartifact.ProjectionResult{Status: liveartifact.StatusOK, Projection: json.RawMessage(`[]`), ProjectedAt: time.Now().UTC()},
		sleep:  5 * time.Millisecond,
	})

	blocks := make([]map[string]any, 0, 10)
	for i := 0; i < 10; i++ {
		blocks = append(blocks, map[string]any{
			"block_id":   "b-" + uuid.NewString(),
			"live_kind":  "agent_run",
			"target_ref": map[string]any{},
			"view_opts":  map[string]any{},
		})
	}
	bodyBytes, _ := json.Marshal(map[string]any{"blocks": blocks})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(string(bodyBytes)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id"}, []string{asset.ID.String()})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	start := time.Now()
	if err := h.ProjectLiveArtifacts(c); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if elapsed > 200*time.Millisecond {
		t.Fatalf("batch of 10 took %v, expected < 200ms", elapsed)
	}
}

// --- Freeze endpoint tests ---

func liveArtifactAsset(projectID uuid.UUID, blockID string) *model.KnowledgeAsset {
	content := `[
		{"id":"para-1","type":"paragraph","content":[{"type":"text","text":"hi","styles":{}}]},
		{"id":"` + blockID + `","type":"live_artifact","props":{"live_kind":"agent_run","target_ref":{"id":"run-1"},"view_opts":{}}}
	]`
	return seedAssetWithContent(projectID, content)
}

func seedAssetWithContent(projectID uuid.UUID, content string) *model.KnowledgeAsset {
	spaceID := uuid.New()
	return &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Kind:        model.KindWikiPage,
		WikiSpaceID: &spaceID,
		ContentJSON: content,
		Title:       "Doc",
		Version:     1,
	}
}

func TestFreezeLiveArtifact_HappyPath(t *testing.T) {
	projectID := uuid.New()
	blockID := "live-block-1"
	asset := liveArtifactAsset(projectID, blockID)
	svc := newLiveArtifactsStubSvc(asset)

	fragment := json.RawMessage(`[
		{"id":"frag-a","type":"paragraph","content":[{"type":"text","text":"a","styles":{}}]},
		{"id":"frag-b","type":"paragraph","content":[{"type":"text","text":"b","styles":{}}]}
	]`)
	reg := liveartifact.NewRegistry()
	reg.Register(&fakeProjector{
		kind:   liveartifact.KindAgentRun,
		result: liveartifact.ProjectionResult{Status: liveartifact.StatusOK, Projection: fragment, ProjectedAt: time.Now().UTC()},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id", "blockId"}, []string{asset.ID.String(), blockID})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.FreezeLiveArtifact(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var newBlocks []map[string]any
	if err := json.Unmarshal([]byte(svc.lastUpdateCJS), &newBlocks); err != nil {
		t.Fatalf("decode updated content: %v", err)
	}
	if len(newBlocks) != 4 {
		t.Fatalf("expected 4 blocks (paragraph, callout, frag-a, frag-b), got %d: %s", len(newBlocks), svc.lastUpdateCJS)
	}
	if newBlocks[0]["id"] != "para-1" {
		t.Fatalf("block[0] should be original paragraph, got %v", newBlocks[0]["id"])
	}
	if newBlocks[1]["type"] != "callout" {
		t.Fatalf("block[1] should be callout, got %v", newBlocks[1]["type"])
	}
	if newBlocks[2]["id"] != "frag-a" {
		t.Fatalf("block[2] should be frag-a, got %v", newBlocks[2]["id"])
	}
	if newBlocks[3]["id"] != "frag-b" {
		t.Fatalf("block[3] should be frag-b, got %v", newBlocks[3]["id"])
	}
	if svc.versionCalls != 1 {
		t.Fatalf("expected CreateVersion to be called once, got %d", svc.versionCalls)
	}
	if svc.lastVersion == nil || svc.lastVersion.Name != "Frozen live artifact "+blockID {
		name := ""
		if svc.lastVersion != nil {
			name = svc.lastVersion.Name
		}
		t.Fatalf("version name mismatch: %q", name)
	}
}

func TestFreezeLiveArtifact_RejectOnNotFound(t *testing.T) {
	projectID := uuid.New()
	blockID := "live-block-1"
	asset := liveArtifactAsset(projectID, blockID)
	svc := newLiveArtifactsStubSvc(asset)

	reg := liveartifact.NewRegistry()
	reg.Register(&fakeProjector{
		kind:   liveartifact.KindAgentRun,
		result: liveartifact.ProjectionResult{Status: liveartifact.StatusNotFound, Diagnostics: "gone", ProjectedAt: time.Now().UTC()},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id", "blockId"}, []string{asset.ID.String(), blockID})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.FreezeLiveArtifact(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.updateCalls != 0 {
		t.Fatalf("expected no Update calls, got %d", svc.updateCalls)
	}
	if svc.versionCalls != 0 {
		t.Fatalf("expected no CreateVersion calls, got %d", svc.versionCalls)
	}
}

func TestFreezeLiveArtifact_BlockNotInAsset(t *testing.T) {
	projectID := uuid.New()
	asset := liveArtifactAsset(projectID, "some-block")
	svc := newLiveArtifactsStubSvc(asset)
	reg := liveartifact.NewRegistry()
	reg.Register(&fakeProjector{
		kind:   liveartifact.KindAgentRun,
		result: liveartifact.ProjectionResult{Status: liveartifact.StatusOK, Projection: json.RawMessage(`[]`), ProjectedAt: time.Now().UTC()},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id", "blockId"}, []string{asset.ID.String(), "does-not-exist"})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.FreezeLiveArtifact(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFreezeLiveArtifact_BlockNotLiveArtifact(t *testing.T) {
	projectID := uuid.New()
	content := `[
		{"id":"para-only","type":"paragraph","content":[{"type":"text","text":"x","styles":{}}]}
	]`
	asset := seedAssetWithContent(projectID, content)
	svc := newLiveArtifactsStubSvc(asset)
	reg := liveartifact.NewRegistry()

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id", "blockId"}, []string{asset.ID.String(), "para-only"})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.FreezeLiveArtifact(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestFreezeLiveArtifact_VersionSnapshot(t *testing.T) {
	projectID := uuid.New()
	blockID := "live-block-1"
	asset := liveArtifactAsset(projectID, blockID)
	originalContent := asset.ContentJSON
	svc := newLiveArtifactsStubSvc(asset)

	fragment := json.RawMessage(`[
		{"id":"frag-a","type":"paragraph","content":[{"type":"text","text":"a","styles":{}}]}
	]`)
	reg := liveartifact.NewRegistry()
	reg.Register(&fakeProjector{
		kind:   liveartifact.KindAgentRun,
		result: liveartifact.ProjectionResult{Status: liveartifact.StatusOK, Projection: fragment, ProjectedAt: time.Now().UTC()},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setRouteParams(c, []string{"id", "blockId"}, []string{asset.ID.String(), blockID})
	c.Set("project_id", projectID)

	h := newHandlerWithLiveArtifacts(svc, reg)
	if err := h.FreezeLiveArtifact(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.versionCalls != 1 {
		t.Fatalf("expected 1 CreateVersion call, got %d", svc.versionCalls)
	}
	if svc.capturedAtVersion != originalContent {
		t.Fatalf("version should capture pre-freeze content\nwant:\n%s\ngot:\n%s", originalContent, svc.capturedAtVersion)
	}
	if svc.lastVersion == nil || svc.lastVersion.ContentJSON != originalContent {
		t.Fatalf("lastVersion should carry pre-freeze content, got %+v", svc.lastVersion)
	}
}
