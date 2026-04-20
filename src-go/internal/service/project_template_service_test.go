package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// --- fakes ---

type fakeTemplateStore struct {
	rows map[uuid.UUID]*model.ProjectTemplate
}

func newFakeTemplateStore() *fakeTemplateStore {
	return &fakeTemplateStore{rows: map[uuid.UUID]*model.ProjectTemplate{}}
}

func (s *fakeTemplateStore) Insert(_ context.Context, t *model.ProjectTemplate) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	cp := *t
	s.rows[t.ID] = &cp
	return nil
}
func (s *fakeTemplateStore) Upsert(ctx context.Context, t *model.ProjectTemplate) error {
	return s.Insert(ctx, t)
}
func (s *fakeTemplateStore) GetByID(_ context.Context, id uuid.UUID) (*model.ProjectTemplate, error) {
	r, ok := s.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *r
	return &cp, nil
}
func (s *fakeTemplateStore) ListVisible(_ context.Context, userID uuid.UUID) ([]*model.ProjectTemplate, error) {
	out := make([]*model.ProjectTemplate, 0)
	for _, r := range s.rows {
		if r.Source == model.ProjectTemplateSourceSystem {
			cp := *r
			out = append(out, &cp)
			continue
		}
		if r.OwnerUserID != nil && *r.OwnerUserID == userID {
			cp := *r
			out = append(out, &cp)
		}
	}
	return out, nil
}
func (s *fakeTemplateStore) UpdateMetadata(_ context.Context, id uuid.UUID, name, description *string) error {
	r, ok := s.rows[id]
	if !ok {
		return errors.New("not found")
	}
	if name != nil {
		r.Name = *name
	}
	if description != nil {
		r.Description = *description
	}
	return nil
}
func (s *fakeTemplateStore) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := s.rows[id]; !ok {
		return errors.New("not found")
	}
	delete(s.rows, id)
	return nil
}

type fakeProjectReader struct {
	project *model.Project
}

func (f *fakeProjectReader) GetByID(_ context.Context, _ uuid.UUID) (*model.Project, error) {
	return f.project, nil
}

// --- sub-resource fake adapters ---

type fakeCustomFields struct {
	exported  []model.ProjectTemplateCustomFieldSnapshot
	imported  []model.ProjectTemplateCustomFieldSnapshot
	importErr error
}

func (a *fakeCustomFields) Export(context.Context, uuid.UUID) ([]model.ProjectTemplateCustomFieldSnapshot, error) {
	return a.exported, nil
}
func (a *fakeCustomFields) Import(_ context.Context, _ uuid.UUID, v []model.ProjectTemplateCustomFieldSnapshot) error {
	if a.importErr != nil {
		return a.importErr
	}
	a.imported = append(a.imported, v...)
	return nil
}

type fakeAutomations struct {
	exported  []model.ProjectTemplateAutomationSnapshot
	imported  []model.ProjectTemplateAutomationSnapshot
	importErr error
}

func (a *fakeAutomations) Export(context.Context, uuid.UUID) ([]model.ProjectTemplateAutomationSnapshot, error) {
	return a.exported, nil
}
func (a *fakeAutomations) ImportInactive(_ context.Context, _ uuid.UUID, v []model.ProjectTemplateAutomationSnapshot) error {
	if a.importErr != nil {
		return a.importErr
	}
	a.imported = append(a.imported, v...)
	return nil
}

// --- tests ---

// 7.1 BuildSnapshot round-trip equivalence (modulo identities).
func TestBuildAndApplySnapshot_RoundTrip(t *testing.T) {
	proj := &model.Project{
		ID:       uuid.New(),
		Settings: `{"review_policy":{"requireManualApproval":true,"minRiskLevelForBlock":"high","autoTriggerOnPR":true}}`,
	}
	cfAdapter := &fakeCustomFields{}
	cfAdapter.exported = []model.ProjectTemplateCustomFieldSnapshot{
		{Key: "priority", Label: "Priority", Type: "select"},
		{Key: "sprint", Label: "Sprint", Type: "select"},
	}
	autoAdapter := &fakeAutomations{}
	autoAdapter.exported = []model.ProjectTemplateAutomationSnapshot{
		{Name: "escalate", Trigger: json.RawMessage(`{"type":"schedule"}`), Actions: json.RawMessage(`[{"type":"notify"}]`)},
	}

	svc := NewProjectTemplateService(newFakeTemplateStore(), &fakeProjectReader{project: proj}).
		WithCustomFieldAdapter(cfAdapter).
		WithAutomationAdapter(autoAdapter)

	snap, err := svc.BuildSnapshot(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if snap.Version != model.CurrentProjectTemplateSnapshotVersion {
		t.Fatalf("version: want %d got %d", model.CurrentProjectTemplateSnapshotVersion, snap.Version)
	}
	if snap.Settings.ReviewPolicy == nil || !snap.Settings.ReviewPolicy.RequireManualApproval {
		t.Fatalf("settings review policy not captured: %+v", snap.Settings)
	}
	if len(snap.CustomFields) != 2 {
		t.Fatalf("custom fields count: want 2 got %d", len(snap.CustomFields))
	}
	// Round-trip through marshal/parse.
	raw, err := model.MarshalProjectTemplateSnapshot(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := model.ParseProjectTemplateSnapshot(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.CustomFields) != len(snap.CustomFields) {
		t.Fatalf("parsed custom fields differ")
	}
	// Apply onto a fresh project (adapters capture what would be written).
	if err := svc.ApplySnapshot(context.Background(), uuid.New(), parsed); err != nil {
		t.Fatalf("ApplySnapshot: %v", err)
	}
	if len(cfAdapter.imported) != 2 {
		t.Fatalf("custom fields not imported: %d", len(cfAdapter.imported))
	}
	if len(autoAdapter.imported) != 1 {
		t.Fatalf("automations not imported: %d", len(autoAdapter.imported))
	}
}

// 7.2 Sanitizer: automation identity fields are stripped on build.
func TestBuildSnapshot_StripsAutomationIdentity(t *testing.T) {
	proj := &model.Project{ID: uuid.New(), Settings: "{}"}
	autoAdapter := &fakeAutomations{}
	autoAdapter.exported = []model.ProjectTemplateAutomationSnapshot{
		{
			Name:    "bad",
			Trigger: json.RawMessage(`{"type":"schedule","configuredByUserID":"abc"}`),
			Actions: json.RawMessage(`[{"type":"notify","actorUserId":"def"}]`),
		},
	}
	svc := NewProjectTemplateService(newFakeTemplateStore(), &fakeProjectReader{project: proj}).
		WithAutomationAdapter(autoAdapter)
	snap, err := svc.BuildSnapshot(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if got := string(snap.Automations[0].Trigger); errors.Is(json.Unmarshal([]byte(got), &map[string]any{}), nil) {
		if containsSubstr(got, "configuredByUserID") {
			t.Fatalf("trigger still contains configuredByUserID: %s", got)
		}
	}
	if containsSubstr(string(snap.Automations[0].Actions), "actorUserId") {
		t.Fatalf("actions still contain actorUserId: %s", snap.Automations[0].Actions)
	}
}

// 7.2 (size guard) Snapshot size cap rejects absurdly large payloads.
func TestBuildSnapshot_SizeGuard(t *testing.T) {
	proj := &model.Project{ID: uuid.New(), Settings: "{}"}
	svc := NewProjectTemplateService(newFakeTemplateStore(), &fakeProjectReader{project: proj})
	big := make([]byte, ProjectTemplateSnapshotMaxBytes+1)
	for i := range big {
		big[i] = 'a'
	}
	// Force a huge description by injecting into an automations adapter with a
	// large JSON-escaped string.
	autoAdapter := &fakeAutomations{}
	autoAdapter.exported = []model.ProjectTemplateAutomationSnapshot{
		{
			Name:    string(big),
			Trigger: json.RawMessage(`{}`),
			Actions: json.RawMessage(`[]`),
		},
	}
	svc = svc.WithAutomationAdapter(autoAdapter)
	_, err := svc.BuildSnapshot(context.Background(), proj.ID)
	if !errors.Is(err, ErrProjectTemplateSnapshotTooLarge) {
		t.Fatalf("expected size-guard error, got %v", err)
	}
}

// 7.3 ApplySnapshot: if a sub-resource import fails, the error propagates
// (transactionality is enforced by the caller's wrapper; the service reports
// the first failure rather than attempting subsequent sub-resources).
func TestApplySnapshot_SubresourceFailurePropagates(t *testing.T) {
	proj := &model.Project{ID: uuid.New(), Settings: "{}"}
	cfAdapter := &fakeCustomFields{importErr: errors.New("boom")}
	cfAdapter.exported = []model.ProjectTemplateCustomFieldSnapshot{{Key: "priority", Label: "Priority", Type: "select"}}
	autoAdapter := &fakeAutomations{}
	autoAdapter.exported = []model.ProjectTemplateAutomationSnapshot{
		{Name: "x", Trigger: json.RawMessage(`{}`), Actions: json.RawMessage(`[]`)},
	}

	svc := NewProjectTemplateService(newFakeTemplateStore(), &fakeProjectReader{project: proj}).
		WithCustomFieldAdapter(cfAdapter).
		WithAutomationAdapter(autoAdapter)

	snap, err := svc.BuildSnapshot(context.Background(), proj.ID)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if err := svc.ApplySnapshot(context.Background(), uuid.New(), snap); err == nil {
		t.Fatalf("expected error from failing adapter")
	}
	if len(autoAdapter.imported) != 0 {
		t.Fatalf("later adapters should not have run: %d", len(autoAdapter.imported))
	}
}

// 7.4 Owner / system / immutable rules on UpdateMetadata and Delete.
func TestOwnershipRules(t *testing.T) {
	ctx := context.Background()
	store := newFakeTemplateStore()
	svc := NewProjectTemplateService(store, nil)

	owner := uuid.New()
	stranger := uuid.New()

	// Seed: one system, one owner-private, one stranger-owned.
	_ = store.Insert(ctx, &model.ProjectTemplate{
		Source: model.ProjectTemplateSourceSystem, Name: "sys", SnapshotJSON: "{}",
	})
	var ownerTpl *model.ProjectTemplate
	_ = store.Insert(ctx, &model.ProjectTemplate{
		Source: model.ProjectTemplateSourceUser, OwnerUserID: &owner, Name: "mine", SnapshotJSON: "{}",
	})
	for _, r := range store.rows {
		if r.Name == "mine" {
			ownerTpl = r
		}
	}
	if ownerTpl == nil {
		t.Fatalf("seed failed")
	}

	var sysID uuid.UUID
	for _, r := range store.rows {
		if r.Source == model.ProjectTemplateSourceSystem {
			sysID = r.ID
		}
	}

	// Stranger cannot see owner's template.
	if _, err := svc.Get(ctx, ownerTpl.ID, stranger); !errors.Is(err, ErrProjectTemplateNotFound) {
		t.Fatalf("stranger should get not-found, got %v", err)
	}
	// Stranger cannot update owner's template.
	newName := "x"
	if _, err := svc.UpdateMetadata(ctx, ownerTpl.ID, stranger, model.UpdateProjectTemplateRequest{Name: &newName}); !errors.Is(err, ErrProjectTemplateOwnerMismatch) {
		t.Fatalf("expected owner mismatch, got %v", err)
	}
	// Owner can update.
	if _, err := svc.UpdateMetadata(ctx, ownerTpl.ID, owner, model.UpdateProjectTemplateRequest{Name: &newName}); err != nil {
		t.Fatalf("owner update failed: %v", err)
	}
	// System template is read-only.
	if _, err := svc.UpdateMetadata(ctx, sysID, owner, model.UpdateProjectTemplateRequest{Name: &newName}); !errors.Is(err, ErrProjectTemplateImmutableSystem) {
		t.Fatalf("expected system immutable, got %v", err)
	}
	if err := svc.Delete(ctx, sysID, owner); !errors.Is(err, ErrProjectTemplateImmutableSystem) {
		t.Fatalf("expected system immutable on delete, got %v", err)
	}
	// Owner can delete own template.
	if err := svc.Delete(ctx, ownerTpl.ID, owner); err != nil {
		t.Fatalf("owner delete failed: %v", err)
	}
}

// 7.5 Marketplace install seam converts a marketplace payload into a
// user-visible template with source=marketplace.
func TestMaterializeMarketplaceInstall(t *testing.T) {
	ctx := context.Background()
	store := newFakeTemplateStore()
	svc := NewProjectTemplateService(store, nil)

	installer := uuid.New()
	tpl, err := svc.MaterializeMarketplaceInstall(ctx, installer, "From mkt", "", `{"version":1}`, 1)
	if err != nil {
		t.Fatalf("materialize: %v", err)
	}
	if tpl.Source != model.ProjectTemplateSourceMarketplace {
		t.Fatalf("source: want marketplace got %q", tpl.Source)
	}
	if tpl.OwnerUserID == nil || *tpl.OwnerUserID != installer {
		t.Fatalf("owner should be installer")
	}
	list, _ := svc.ListVisible(ctx, installer)
	if len(list) == 0 {
		t.Fatalf("installer should see the materialized template")
	}
}

// 7.6 Built-in bundle registration yields at least one system template.
func TestBuiltInProjectTemplatesRegistered(t *testing.T) {
	ctx := context.Background()
	store := newFakeTemplateStore()
	if err := RegisterBuiltInProjectTemplates(ctx, store); err != nil {
		t.Fatalf("register: %v", err)
	}
	sysCount := 0
	for _, r := range store.rows {
		if r.Source == model.ProjectTemplateSourceSystem {
			sysCount++
		}
	}
	if sysCount < 1 {
		t.Fatalf("expected at least one system template, got %d", sysCount)
	}
}

// --- helpers ---

func containsSubstr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
