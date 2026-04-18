// Package service — project_template_service.go implements snapshot build,
// sanitization, apply (clone), and metadata management for project templates.
//
// Design notes:
//   - The service is deliberately NOT coupled to a single concrete subresource
//     service. Each sub-resource (custom fields, saved views, dashboards,
//     automations, workflow definitions, task statuses) is reached through a
//     narrow adapter interface. Adapters may be nil in early stages — the
//     snapshot will simply omit that sub-resource rather than fail.
//   - Apply runs inside a caller-owned transaction. The service does not
//     open or commit transactions; callers (e.g. project lifecycle service)
//     compose it into their own atomic unit.
//   - A sanitizer checks the settings whitelist before build is allowed to
//     succeed. Unknown fields cause a fail-closed error — the reason is that
//     silently dropping an unknown field may later surprise a user who
//     "saved everything" into a template.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

// ProjectTemplateService builds and applies project-configuration snapshots.
type ProjectTemplateService struct {
	templates       ProjectTemplateStore
	projects        ProjectTemplateProjectReader
	customFields    ProjectTemplateCustomFieldAdapter
	savedViews     ProjectTemplateSavedViewAdapter
	dashboards     ProjectTemplateDashboardAdapter
	automations    ProjectTemplateAutomationAdapter
	workflows      ProjectTemplateWorkflowAdapter
	taskStatuses   ProjectTemplateTaskStatusAdapter
}

// ProjectTemplateStore is the narrow CRUD contract the service needs from
// the repository layer.
type ProjectTemplateStore interface {
	Insert(ctx context.Context, t *model.ProjectTemplate) error
	Upsert(ctx context.Context, t *model.ProjectTemplate) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.ProjectTemplate, error)
	ListVisible(ctx context.Context, userID uuid.UUID) ([]*model.ProjectTemplate, error)
	UpdateMetadata(ctx context.Context, id uuid.UUID, name, description *string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProjectTemplateProjectReader is what the snapshot builder needs to read
// settings off the source project.
type ProjectTemplateProjectReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

// --- sub-resource adapter interfaces ---
// Each adapter is optional. A nil adapter means the sub-resource is not yet
// wired; snapshot Build/Apply will simply skip it without error.

type ProjectTemplateCustomFieldAdapter interface {
	Export(ctx context.Context, projectID uuid.UUID) ([]model.ProjectTemplateCustomFieldSnapshot, error)
	Import(ctx context.Context, projectID uuid.UUID, fields []model.ProjectTemplateCustomFieldSnapshot) error
}

type ProjectTemplateSavedViewAdapter interface {
	Export(ctx context.Context, projectID uuid.UUID) ([]model.ProjectTemplateSavedViewSnapshot, error)
	Import(ctx context.Context, projectID uuid.UUID, views []model.ProjectTemplateSavedViewSnapshot) error
}

type ProjectTemplateDashboardAdapter interface {
	Export(ctx context.Context, projectID uuid.UUID) ([]model.ProjectTemplateDashboardSnapshot, error)
	Import(ctx context.Context, projectID uuid.UUID, dashboards []model.ProjectTemplateDashboardSnapshot) error
}

type ProjectTemplateAutomationAdapter interface {
	Export(ctx context.Context, projectID uuid.UUID) ([]model.ProjectTemplateAutomationSnapshot, error)
	// ImportInactive creates automation rules in their inactive state; the
	// clone initiator must explicitly activate them before they run.
	ImportInactive(ctx context.Context, projectID uuid.UUID, rules []model.ProjectTemplateAutomationSnapshot) error
}

type ProjectTemplateWorkflowAdapter interface {
	Export(ctx context.Context, projectID uuid.UUID) ([]model.ProjectTemplateWorkflowSnapshot, error)
	Import(ctx context.Context, projectID uuid.UUID, defs []model.ProjectTemplateWorkflowSnapshot) error
}

type ProjectTemplateTaskStatusAdapter interface {
	Export(ctx context.Context, projectID uuid.UUID) ([]model.ProjectTemplateTaskStatusSnapshot, error)
	Import(ctx context.Context, projectID uuid.UUID, statuses []model.ProjectTemplateTaskStatusSnapshot) error
}

// NewProjectTemplateService constructs a service with the minimum required
// dependencies. Optional sub-resource adapters attach via the With* methods.
func NewProjectTemplateService(
	templates ProjectTemplateStore,
	projects ProjectTemplateProjectReader,
) *ProjectTemplateService {
	return &ProjectTemplateService{
		templates: templates,
		projects:  projects,
	}
}

func (s *ProjectTemplateService) WithCustomFieldAdapter(a ProjectTemplateCustomFieldAdapter) *ProjectTemplateService {
	s.customFields = a
	return s
}
func (s *ProjectTemplateService) WithSavedViewAdapter(a ProjectTemplateSavedViewAdapter) *ProjectTemplateService {
	s.savedViews = a
	return s
}
func (s *ProjectTemplateService) WithDashboardAdapter(a ProjectTemplateDashboardAdapter) *ProjectTemplateService {
	s.dashboards = a
	return s
}
func (s *ProjectTemplateService) WithAutomationAdapter(a ProjectTemplateAutomationAdapter) *ProjectTemplateService {
	s.automations = a
	return s
}
func (s *ProjectTemplateService) WithWorkflowAdapter(a ProjectTemplateWorkflowAdapter) *ProjectTemplateService {
	s.workflows = a
	return s
}
func (s *ProjectTemplateService) WithTaskStatusAdapter(a ProjectTemplateTaskStatusAdapter) *ProjectTemplateService {
	s.taskStatuses = a
	return s
}

// Errors.
var (
	ErrProjectTemplateSnapshotInvalid = errors.New("project template: snapshot invalid")
	ErrProjectTemplateSnapshotTooLarge = errors.New("project template: snapshot exceeds size limit")
	ErrProjectTemplateUnknownSource    = errors.New("project template: unknown source")
	ErrProjectTemplateNotFound         = errors.New("project template: not found")
	ErrProjectTemplateOwnerMismatch    = errors.New("project template: caller is not the owner")
	ErrProjectTemplateImmutableSystem  = errors.New("project template: system templates are read-only")
)

// ProjectTemplateSnapshotMaxBytes caps snapshot JSON size. Snapshots that
// exceed the cap fail build — a template that large probably contains data
// that should not be in a configuration snapshot.
const ProjectTemplateSnapshotMaxBytes = 1024 * 1024 // 1 MiB

// BuildSnapshot assembles a ProjectTemplateSnapshot from the named project.
// Omits any sub-resource whose adapter is nil. Fails fast if the settings
// sanitizer rejects an unknown/unsafe field.
func (s *ProjectTemplateService) BuildSnapshot(ctx context.Context, projectID uuid.UUID) (model.ProjectTemplateSnapshot, error) {
	snap := model.NewEmptyProjectTemplateSnapshot()

	if s.projects != nil {
		project, err := s.projects.GetByID(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: read project: %w", err)
		}
		settings, err := sanitizeProjectTemplateSettings(project.StoredSettings())
		if err != nil {
			return snap, fmt.Errorf("build snapshot: %w", err)
		}
		snap.Settings = settings
	}

	if s.customFields != nil {
		out, err := s.customFields.Export(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: custom fields: %w", err)
		}
		snap.CustomFields = out
	}
	if s.savedViews != nil {
		out, err := s.savedViews.Export(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: saved views: %w", err)
		}
		snap.SavedViews = out
	}
	if s.dashboards != nil {
		out, err := s.dashboards.Export(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: dashboards: %w", err)
		}
		snap.Dashboards = out
	}
	if s.automations != nil {
		out, err := s.automations.Export(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: automations: %w", err)
		}
		for i := range out {
			// Strip any identity fields that might have leaked through.
			out[i] = stripAutomationIdentity(out[i])
		}
		snap.Automations = out
	}
	if s.workflows != nil {
		out, err := s.workflows.Export(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: workflow definitions: %w", err)
		}
		snap.WorkflowDefinitions = out
	}
	if s.taskStatuses != nil {
		out, err := s.taskStatuses.Export(ctx, projectID)
		if err != nil {
			return snap, fmt.Errorf("build snapshot: task statuses: %w", err)
		}
		snap.TaskStatuses = out
	}

	// Deterministic ordering where sub-resource order has no semantic meaning.
	sort.SliceStable(snap.CustomFields, func(i, j int) bool {
		return snap.CustomFields[i].Key < snap.CustomFields[j].Key
	})
	sort.SliceStable(snap.TaskStatuses, func(i, j int) bool {
		if snap.TaskStatuses[i].Order == snap.TaskStatuses[j].Order {
			return snap.TaskStatuses[i].Key < snap.TaskStatuses[j].Key
		}
		return snap.TaskStatuses[i].Order < snap.TaskStatuses[j].Order
	})

	// Size guard. Serialize once just to check; the real persist path
	// serializes again — the double cost is cheap and keeps the guard
	// independent of later serialization choices.
	raw, err := json.Marshal(snap)
	if err != nil {
		return snap, fmt.Errorf("build snapshot: marshal: %w", err)
	}
	if len(raw) > ProjectTemplateSnapshotMaxBytes {
		return snap, fmt.Errorf("%w: %d bytes (max %d)", ErrProjectTemplateSnapshotTooLarge, len(raw), ProjectTemplateSnapshotMaxBytes)
	}
	return snap, nil
}

// ApplySnapshot writes a snapshot onto an already-created project. Order is
// topological: settings → customFields → savedViews → taskStatuses →
// workflowDefinitions → dashboards → automations-inactive. Automations land
// in an inactive state; the caller must activate them explicitly afterwards.
//
// The caller MUST provide a running transaction context; this function does
// NOT open its own transaction.
func (s *ProjectTemplateService) ApplySnapshot(
	ctx context.Context,
	projectID uuid.UUID,
	snap model.ProjectTemplateSnapshot,
) error {
	// 1. Upgrade across versions if needed.
	upgraded, err := upgradeProjectTemplateSnapshot(snap)
	if err != nil {
		return fmt.Errorf("apply snapshot: %w", err)
	}

	// 2. Settings go onto the project row.
	//    The caller wires this via an optional settings applier so the
	//    template service does not need to know about settings persistence.
	//    For now, ApplySnapshot delegates settings to a caller-supplied func
	//    via the dedicated ApplySettings method — this function focuses on
	//    sub-resources. Callers that want settings applied should call
	//    ApplySettings first.

	// 3. Sub-resources, each only if its adapter is wired.
	if s.customFields != nil && len(upgraded.CustomFields) > 0 {
		if err := s.customFields.Import(ctx, projectID, upgraded.CustomFields); err != nil {
			return fmt.Errorf("apply snapshot: custom fields: %w", err)
		}
	}
	if s.savedViews != nil && len(upgraded.SavedViews) > 0 {
		if err := s.savedViews.Import(ctx, projectID, upgraded.SavedViews); err != nil {
			return fmt.Errorf("apply snapshot: saved views: %w", err)
		}
	}
	if s.taskStatuses != nil && len(upgraded.TaskStatuses) > 0 {
		if err := s.taskStatuses.Import(ctx, projectID, upgraded.TaskStatuses); err != nil {
			return fmt.Errorf("apply snapshot: task statuses: %w", err)
		}
	}
	if s.workflows != nil && len(upgraded.WorkflowDefinitions) > 0 {
		if err := s.workflows.Import(ctx, projectID, upgraded.WorkflowDefinitions); err != nil {
			return fmt.Errorf("apply snapshot: workflow definitions: %w", err)
		}
	}
	if s.dashboards != nil && len(upgraded.Dashboards) > 0 {
		if err := s.dashboards.Import(ctx, projectID, upgraded.Dashboards); err != nil {
			return fmt.Errorf("apply snapshot: dashboards: %w", err)
		}
	}
	if s.automations != nil && len(upgraded.Automations) > 0 {
		if err := s.automations.ImportInactive(ctx, projectID, upgraded.Automations); err != nil {
			return fmt.Errorf("apply snapshot: automations: %w", err)
		}
	}
	return nil
}

// ApplySnapshotSettings returns a project settings merge patch derived from
// the snapshot's settings subtree. Callers compose this into their own
// project Update call. Returning a patch (rather than writing directly) keeps
// transaction ownership in the caller's lifecycle service.
func (s *ProjectTemplateService) ApplySnapshotSettings(snap model.ProjectTemplateSnapshot) *model.ProjectSettingsPatch {
	if snap.Settings.ReviewPolicy == nil && snap.Settings.DefaultCodingAgent == nil && snap.Settings.BudgetGovernance == nil {
		return nil
	}
	patch := &model.ProjectSettingsPatch{}
	if snap.Settings.ReviewPolicy != nil {
		cp := *snap.Settings.ReviewPolicy
		patch.ReviewPolicy = &cp
	}
	if snap.Settings.DefaultCodingAgent != nil {
		cp := *snap.Settings.DefaultCodingAgent
		patch.CodingAgent = &cp
	}
	if snap.Settings.BudgetGovernance != nil {
		cp := *snap.Settings.BudgetGovernance
		patch.BudgetGovernance = &cp
	}
	return patch
}

// SaveAsTemplate builds a snapshot of the project and persists it as a
// user-source template owned by ownerID. Returns the newly created row.
func (s *ProjectTemplateService) SaveAsTemplate(
	ctx context.Context,
	projectID uuid.UUID,
	ownerID uuid.UUID,
	req model.CreateProjectTemplateRequest,
) (*model.ProjectTemplate, error) {
	snap, err := s.BuildSnapshot(ctx, projectID)
	if err != nil {
		return nil, err
	}
	raw, err := model.MarshalProjectTemplateSnapshot(snap)
	if err != nil {
		return nil, fmt.Errorf("save as template: marshal snapshot: %w", err)
	}
	owner := ownerID
	tpl := &model.ProjectTemplate{
		Source:          model.ProjectTemplateSourceUser,
		OwnerUserID:     &owner,
		Name:            req.Name,
		Description:     req.Description,
		SnapshotJSON:    raw,
		SnapshotVersion: snap.Version,
	}
	if err := s.templates.Insert(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

// ListVisible delegates to the repo. Exposed here so the handler calls a
// single service entry point.
func (s *ProjectTemplateService) ListVisible(ctx context.Context, userID uuid.UUID) ([]*model.ProjectTemplate, error) {
	return s.templates.ListVisible(ctx, userID)
}

// Get returns a single template by id and verifies it is visible to userID.
func (s *ProjectTemplateService) Get(ctx context.Context, id, userID uuid.UUID) (*model.ProjectTemplate, error) {
	tpl, err := s.templates.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProjectTemplateNotFound
		}
		return nil, err
	}
	if !templateVisibleTo(tpl, userID) {
		return nil, ErrProjectTemplateNotFound
	}
	return tpl, nil
}

// UpdateMetadata patches Name/Description on a user-source template the
// caller owns. System and marketplace-origin templates cannot be renamed.
func (s *ProjectTemplateService) UpdateMetadata(
	ctx context.Context,
	id, userID uuid.UUID,
	req model.UpdateProjectTemplateRequest,
) (*model.ProjectTemplate, error) {
	tpl, err := s.templates.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProjectTemplateNotFound
		}
		return nil, err
	}
	if tpl.Source == model.ProjectTemplateSourceSystem {
		return nil, ErrProjectTemplateImmutableSystem
	}
	if tpl.OwnerUserID == nil || *tpl.OwnerUserID != userID {
		return nil, ErrProjectTemplateOwnerMismatch
	}
	if err := s.templates.UpdateMetadata(ctx, id, req.Name, req.Description); err != nil {
		return nil, err
	}
	return s.templates.GetByID(ctx, id)
}

// Delete removes an owner-owned user or marketplace-origin template. System
// templates can never be deleted at runtime.
func (s *ProjectTemplateService) Delete(ctx context.Context, id, userID uuid.UUID) error {
	tpl, err := s.templates.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrProjectTemplateNotFound
		}
		return err
	}
	if tpl.Source == model.ProjectTemplateSourceSystem {
		return ErrProjectTemplateImmutableSystem
	}
	if tpl.OwnerUserID == nil || *tpl.OwnerUserID != userID {
		return ErrProjectTemplateOwnerMismatch
	}
	return s.templates.Delete(ctx, id)
}

// MaterializeMarketplaceInstall persists a marketplace-sourced project
// template for a specific installer. Used by the marketplace install seam.
func (s *ProjectTemplateService) MaterializeMarketplaceInstall(
	ctx context.Context,
	installer uuid.UUID,
	name, description, snapshotJSON string,
	snapshotVersion int,
) (*model.ProjectTemplate, error) {
	// Validate the snapshot round-trips so we reject invalid marketplace payloads.
	if _, err := model.ParseProjectTemplateSnapshot(snapshotJSON); err != nil {
		return nil, fmt.Errorf("marketplace install: %w", errors.Join(ErrProjectTemplateSnapshotInvalid, err))
	}
	if snapshotVersion <= 0 {
		snapshotVersion = model.CurrentProjectTemplateSnapshotVersion
	}
	ownerCopy := installer
	tpl := &model.ProjectTemplate{
		Source:          model.ProjectTemplateSourceMarketplace,
		OwnerUserID:     &ownerCopy,
		Name:            name,
		Description:     description,
		SnapshotJSON:    snapshotJSON,
		SnapshotVersion: snapshotVersion,
	}
	if err := s.templates.Insert(ctx, tpl); err != nil {
		return nil, err
	}
	return tpl, nil
}

// --- helpers ---

func templateVisibleTo(t *model.ProjectTemplate, userID uuid.UUID) bool {
	if t == nil {
		return false
	}
	switch t.Source {
	case model.ProjectTemplateSourceSystem:
		return true
	case model.ProjectTemplateSourceUser, model.ProjectTemplateSourceMarketplace:
		return t.OwnerUserID != nil && *t.OwnerUserID == userID
	default:
		return false
	}
}

// stripAutomationIdentity enforces the design rule that automations in a
// snapshot must not carry `configuredByUserID` or similar identity fields.
// The snapshot schema already omits those; this function exists as a defense
// against adapters that accidentally leak them into the JSON payload.
func stripAutomationIdentity(a model.ProjectTemplateAutomationSnapshot) model.ProjectTemplateAutomationSnapshot {
	// Trigger/Conditions/Actions are opaque JSON; scan for known bad keys and
	// drop them. We deliberately keep the implementation simple — a single
	// recursive walk over each payload.
	a.Trigger = stripIdentityKeys(a.Trigger)
	a.Conditions = stripIdentityKeys(a.Conditions)
	a.Actions = stripIdentityKeys(a.Actions)
	return a
}

var automationIdentityKeys = map[string]struct{}{
	"configuredByUserID":   {},
	"configured_by_user_id": {},
	"actorUserId":          {},
	"actor_user_id":        {},
	"createdBy":            {},
	"created_by":           {},
	"ownerId":              {},
	"owner_id":             {},
}

func stripIdentityKeys(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return raw
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	cleaned := stripIdentityValue(v)
	out, err := json.Marshal(cleaned)
	if err != nil {
		return raw
	}
	return out
}

func stripIdentityValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, child := range val {
			if _, banned := automationIdentityKeys[k]; banned {
				continue
			}
			out[k] = stripIdentityValue(child)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, child := range val {
			out[i] = stripIdentityValue(child)
		}
		return out
	default:
		return val
	}
}
