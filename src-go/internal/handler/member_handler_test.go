package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type fakeMemberRepo struct {
	members         []*model.Member
	createdMembers  []*model.Member
	updatedRequests map[uuid.UUID]*model.UpdateMemberRequest
}

type fakeMemberRoleStore struct {
	roles map[string]*rolepkg.Manifest
}

func (f *fakeMemberRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if role, ok := f.roles[id]; ok {
		return role, nil
	}
	return nil, os.ErrNotExist
}

func (f *fakeMemberRepo) Create(_ context.Context, member *model.Member) error {
	f.createdMembers = append(f.createdMembers, member)
	f.members = append(f.members, member)
	return nil
}

func (f *fakeMemberRepo) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.Member, error) {
	results := make([]*model.Member, 0, len(f.members))
	for _, member := range f.members {
		if member != nil && member.ProjectID == projectID {
			results = append(results, member)
		}
	}
	return results, nil
}

func (f *fakeMemberRepo) Update(_ context.Context, id uuid.UUID, req *model.UpdateMemberRequest) error {
	if f.updatedRequests == nil {
		f.updatedRequests = map[uuid.UUID]*model.UpdateMemberRequest{}
	}
	f.updatedRequests[id] = req

	for _, member := range f.members {
		if member == nil || member.ID != id {
			continue
		}
		if req.Name != nil {
			member.Name = *req.Name
		}
		if req.Role != nil {
			member.Role = *req.Role
		}
		if req.Email != nil {
			member.Email = *req.Email
		}
		if req.AgentConfig != nil {
			member.AgentConfig = *req.AgentConfig
		}
		if req.Skills != nil {
			member.Skills = append([]string(nil), (*req.Skills)...)
		}
		if req.IsActive != nil {
			member.IsActive = *req.IsActive
		}
		setStringFieldIfPresent(member, "Status", getOptionalStringField(req, "Status"))
		setStringFieldIfPresent(member, "IMPlatform", getOptionalStringField(req, "IMPlatform"))
		setStringFieldIfPresent(member, "IMUserID", getOptionalStringField(req, "IMUserID"))
		return nil
	}
	return nil
}

func (f *fakeMemberRepo) BulkUpdateStatus(
	_ context.Context,
	projectID uuid.UUID,
	memberIDs []uuid.UUID,
	status string,
) ([]model.BulkUpdateMemberResult, error) {
	results := make([]model.BulkUpdateMemberResult, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		var matched *model.Member
		for _, member := range f.members {
			if member != nil && member.ID == memberID && member.ProjectID == projectID {
				matched = member
				break
			}
		}
		if matched == nil {
			results = append(results, model.BulkUpdateMemberResult{
				MemberID: memberID.String(),
				Success:  false,
				Error:    "member not found in project",
			})
			continue
		}

		matched.Status = model.NormalizeMemberStatus(status, status == model.MemberStatusActive)
		matched.IsActive = model.IsMemberStatusActive(matched.Status)
		results = append(results, model.BulkUpdateMemberResult{
			MemberID: memberID.String(),
			Success:  true,
			Status:   matched.Status,
		})
	}

	return results, nil
}

func (f *fakeMemberRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Member, error) {
	for _, member := range f.members {
		if member != nil && member.ID == id {
			return member, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeMemberRepo) Delete(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakeMemberRepo) CountOwners(_ context.Context, projectID uuid.UUID) (int64, error) {
	var n int64
	for _, member := range f.members {
		if member != nil && member.ProjectID == projectID && member.ProjectRole == model.ProjectRoleOwner {
			n++
		}
	}
	return n, nil
}

func (f *fakeMemberRepo) UpdateProjectRole(_ context.Context, id uuid.UUID, role string) error {
	for _, member := range f.members {
		if member != nil && member.ID == id {
			member.ProjectRole = role
			return nil
		}
	}
	return nil
}

func TestMemberHandlerListIncludesAgentConfig(t *testing.T) {
	projectID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()

	repo := &fakeMemberRepo{
		members: []*model.Member{
			{
				ID:          memberID,
				ProjectID:   projectID,
				Name:        "Review Bot",
				Type:        "agent",
				Role:        "code-reviewer",
				AgentConfig: `{"roleId":"frontend-developer","runtime":"codex"}`,
				Skills:      []string{"review", "security"},
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/members", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo)
	if err := h.List(ctx); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("len(response) = %d, want 1", len(response))
	}

	if response[0]["agentConfig"] != `{"roleId":"frontend-developer","runtime":"codex"}` {
		t.Fatalf("agentConfig = %#v", response[0]["agentConfig"])
	}

	skills, ok := response[0]["skills"].([]any)
	if !ok || len(skills) != 2 {
		t.Fatalf("skills = %#v, want 2 entries", response[0]["skills"])
	}
}

func TestMemberHandlerCreateAcceptsDocumentedStatusAndIMIdentity(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeMemberRepo{}
	e := echo.New()
	e.Validator = validatorStub{}

	body := `{"name":"Ops Bot","type":"agent","role":"operator","status":"suspended","imPlatform":"feishu","imUserId":"ou_bot_123","agentConfig":"{}","skills":["ops"]}`
	req := httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo)
	if err := h.Create(ctx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if len(repo.createdMembers) != 1 {
		t.Fatalf("created members = %d, want 1", len(repo.createdMembers))
	}

	created := repo.createdMembers[0]
	assertStringField(t, created, "Status", "suspended")
	assertStringField(t, created, "IMPlatform", "feishu")
	assertStringField(t, created, "IMUserID", "ou_bot_123")

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["status"] != "suspended" {
		t.Fatalf("response status = %#v, want suspended", response["status"])
	}
	if response["imPlatform"] != "feishu" {
		t.Fatalf("response imPlatform = %#v, want feishu", response["imPlatform"])
	}
	if response["imUserId"] != "ou_bot_123" {
		t.Fatalf("response imUserId = %#v, want ou_bot_123", response["imUserId"])
	}
}

func TestMemberHandlerCreateRejectsUnknownBoundRole(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeMemberRepo{}
	e := echo.New()
	e.Validator = validatorStub{}

	body := `{"name":"Ops Bot","type":"agent","role":"operator","agentConfig":"{\"roleId\":\"missing-role\",\"runtime\":\"codex\"}","skills":["ops"]}`
	req := httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo).WithRoleStore(&fakeMemberRoleStore{
		roles: map[string]*rolepkg.Manifest{},
	})
	if err := h.Create(ctx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if len(repo.createdMembers) != 0 {
		t.Fatalf("created members = %d, want 0", len(repo.createdMembers))
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["field"] != "agentConfig.roleId" {
		t.Fatalf("field = %#v, want agentConfig.roleId", response["field"])
	}
}

func TestMemberHandlerUpdateRoundTripsCanonicalStatusAndIMIdentity(t *testing.T) {
	projectID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()
	repo := &fakeMemberRepo{
		members: []*model.Member{
			{
				ID:          memberID,
				ProjectID:   projectID,
				Name:        "Review Bot",
				Type:        model.MemberTypeAgent,
				Role:        "code-reviewer",
				AgentConfig: `{"runtime":"codex"}`,
				Skills:      []string{"review"},
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	setStringFieldIfPresent(repo.members[0], "Status", stringPtr("active"))
	setStringFieldIfPresent(repo.members[0], "IMPlatform", stringPtr("slack"))
	setStringFieldIfPresent(repo.members[0], "IMUserID", stringPtr("U123"))

	e := echo.New()
	body := `{"status":"inactive","imPlatform":"feishu","imUserId":"ou_456"}`
	req := httptest.NewRequest(http.MethodPut, "/members/"+memberID.String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/members/:id")
	ctx.SetParamNames("id")
	ctx.SetParamValues(memberID.String())

	h := handler.NewMemberHandler(repo)
	if err := h.Update(ctx); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	reqBody := repo.updatedRequests[memberID]
	if reqBody == nil {
		t.Fatal("update request not captured")
	}
	assertStringPointerField(t, reqBody, "Status", "inactive")
	assertStringPointerField(t, reqBody, "IMPlatform", "feishu")
	assertStringPointerField(t, reqBody, "IMUserID", "ou_456")

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["status"] != "inactive" {
		t.Fatalf("response status = %#v, want inactive", response["status"])
	}
	if response["imPlatform"] != "feishu" {
		t.Fatalf("response imPlatform = %#v, want feishu", response["imPlatform"])
	}
	if response["imUserId"] != "ou_456" {
		t.Fatalf("response imUserId = %#v, want ou_456", response["imUserId"])
	}
}

func TestMemberHandlerUpdateRejectsUnknownBoundRole(t *testing.T) {
	projectID := uuid.New()
	memberID := uuid.New()
	now := time.Now().UTC()
	repo := &fakeMemberRepo{
		members: []*model.Member{
			{
				ID:          memberID,
				ProjectID:   projectID,
				Name:        "Review Bot",
				Type:        model.MemberTypeAgent,
				Role:        "code-reviewer",
				AgentConfig: `{"roleId":"frontend-developer","runtime":"codex"}`,
				IsActive:    true,
				CreatedAt:   now,
				UpdatedAt:   now,
			},
		},
	}
	e := echo.New()
	body := `{"agentConfig":"{\"roleId\":\"missing-role\",\"runtime\":\"codex\"}"}`
	req := httptest.NewRequest(http.MethodPut, "/members/"+memberID.String(), strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/members/:id")
	ctx.SetParamNames("id")
	ctx.SetParamValues(memberID.String())

	h := handler.NewMemberHandler(repo).WithRoleStore(&fakeMemberRoleStore{
		roles: map[string]*rolepkg.Manifest{},
	})
	if err := h.Update(ctx); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	if len(repo.updatedRequests) != 0 {
		t.Fatalf("updated requests = %d, want 0", len(repo.updatedRequests))
	}
}

func TestMemberHandlerBulkUpdateReturnsPerMemberResults(t *testing.T) {
	projectID := uuid.New()
	memberID := uuid.New()
	otherProjectMemberID := uuid.New()
	now := time.Now().UTC()
	repo := &fakeMemberRepo{
		members: []*model.Member{
			{
				ID:        memberID,
				ProjectID: projectID,
				Name:      "Review Bot",
				Type:      model.MemberTypeAgent,
				Role:      "code-reviewer",
				IsActive:  true,
				CreatedAt: now,
				UpdatedAt: now,
			},
			{
				ID:        otherProjectMemberID,
				ProjectID: uuid.New(),
				Name:      "Other Bot",
				Type:      model.MemberTypeAgent,
				Role:      "observer",
				IsActive:  true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}
	setStringFieldIfPresent(repo.members[0], "Status", stringPtr("active"))
	setStringFieldIfPresent(repo.members[1], "Status", stringPtr("active"))

	e := echo.New()
	e.Validator = validatorStub{}
	body := `{"memberIds":["` + memberID.String() + `","` + otherProjectMemberID.String() + `"],"status":"suspended"}`
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/members/bulk-update", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.SetPath("/projects/:pid/members/bulk-update")
	ctx.SetParamNames("pid")
	ctx.SetParamValues(projectID.String())
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo)
	method := reflect.ValueOf(h).MethodByName("BulkUpdate")
	if !method.IsValid() {
		t.Fatalf("BulkUpdate method is missing")
	}

	results := method.Call([]reflect.Value{reflect.ValueOf(ctx)})
	if len(results) != 1 {
		t.Fatalf("BulkUpdate() returned %d values, want 1", len(results))
	}
	if errValue := results[0]; !errValue.IsNil() {
		t.Fatalf("BulkUpdate() error = %v", errValue.Interface())
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(response.Results))
	}
	if response.Results[0]["memberId"] != memberID.String() || response.Results[0]["success"] != true {
		t.Fatalf("first result = %#v, want successful update for project member", response.Results[0])
	}
	if response.Results[0]["status"] != "suspended" {
		t.Fatalf("first result status = %#v, want suspended", response.Results[0]["status"])
	}
	if response.Results[1]["memberId"] != otherProjectMemberID.String() || response.Results[1]["success"] != false {
		t.Fatalf("second result = %#v, want failed update for out-of-scope member", response.Results[1])
	}
	if !strings.Contains(strings.ToLower(asString(response.Results[1]["error"])), "project") {
		t.Fatalf("second result error = %#v, want project-scoped failure", response.Results[1]["error"])
	}
}

type validatorStub struct{}

func (validatorStub) Validate(any) error { return nil }

func assertStringField(t *testing.T, target any, fieldName string, want string) {
	t.Helper()
	value := reflect.ValueOf(target)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("missing field %s", fieldName)
	}
	if field.String() != want {
		t.Fatalf("%s = %q, want %q", fieldName, field.String(), want)
	}
}

func setStringFieldIfPresent(target any, fieldName string, value *string) {
	field := reflect.ValueOf(target).Elem().FieldByName(fieldName)
	if !field.IsValid() || value == nil {
		return
	}
	field.SetString(*value)
}

func assertStringPointerField(t *testing.T, target any, fieldName string, want string) {
	t.Helper()
	value := reflect.ValueOf(target)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("missing field %s", fieldName)
	}
	if field.IsNil() {
		t.Fatalf("%s = nil, want %q", fieldName, want)
	}
	if got := field.Elem().String(); got != want {
		t.Fatalf("%s = %q, want %q", fieldName, got, want)
	}
}

func getOptionalStringField(target any, fieldName string) *string {
	value := reflect.ValueOf(target)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}
	field := value.FieldByName(fieldName)
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	result := field.Elem().String()
	return &result
}

func stringPtr(value string) *string {
	return &value
}

func asString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

// TestMemberHandlerCreateRejectsHumanType asserts that the direct member
// create endpoint refuses to land a human member. Humans are now onboarded
// via the invitation flow — the endpoint only accepts agents.
func TestMemberHandlerCreateRejectsHumanType(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeMemberRepo{}
	e := echo.New()
	e.Validator = validatorStub{}

	body := `{"name":"Alice","type":"human","email":"alice@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo)
	if err := h.Create(ctx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410 Gone", rec.Code)
	}
	if len(repo.createdMembers) != 0 {
		t.Fatalf("created members = %d, want 0", len(repo.createdMembers))
	}
}

// TestMemberHandlerCreateAcceptsAgentType verifies the agent creation path
// is still valid after the human restriction is in place.
func TestMemberHandlerCreateAcceptsAgentType(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeMemberRepo{}
	e := echo.New()
	e.Validator = validatorStub{}

	body := `{"name":"Ops Agent","type":"agent","role":"operator","agentConfig":"{}"}`
	req := httptest.NewRequest(http.MethodPost, "/members", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	ctx.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewMemberHandler(repo)
	if err := h.Create(ctx); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 Created", rec.Code)
	}
	if len(repo.createdMembers) != 1 {
		t.Fatalf("created members = %d, want 1", len(repo.createdMembers))
	}
}
