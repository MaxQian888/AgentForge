// Package service — builtin_project_template_bundle.go registers the
// system-source project templates shipped with every release. Parallels
// builtin_bundle.go (plugins): a small, curated list of stable "starter
// projects" that users can clone the moment they sign in.
//
// Design:
//   - IDs are deterministic (hardcoded UUIDs) so repeated bundle
//     registration upserts the same rows instead of creating duplicates.
//   - Adding a new system template is an explicit code change with a new
//     UUID. No runtime config expands this list (see design.md Decision 2).
//   - Snapshot payloads live inline rather than loading from YAML/JSON on
//     disk. They are small, auditable, and participate in compile-time
//     checks against the typed snapshot schema.
package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// builtInProjectTemplateRegistrar is the narrow contract used to upsert the
// system bundle. Any implementation with Upsert(ctx, template) satisfies it
// — in production the project template repository.
type builtInProjectTemplateRegistrar interface {
	Upsert(ctx context.Context, t *model.ProjectTemplate) error
}

// RegisterBuiltInProjectTemplates idempotently upserts every system-source
// template in the bundle. Safe to call on every server start; repeated
// invocations produce the same DB state.
func RegisterBuiltInProjectTemplates(ctx context.Context, reg builtInProjectTemplateRegistrar) error {
	if reg == nil {
		return fmt.Errorf("register built-in project templates: registrar is nil")
	}
	for _, tpl := range builtInProjectTemplates() {
		if err := reg.Upsert(ctx, tpl); err != nil {
			return fmt.Errorf("upsert built-in project template %q: %w", tpl.Name, err)
		}
	}
	return nil
}

// builtInProjectTemplates returns the curated list. Exposed (unexported) so
// tests can count/inspect without hitting the registrar.
func builtInProjectTemplates() []*model.ProjectTemplate {
	return []*model.ProjectTemplate{
		starterAgileProjectTemplate(),
	}
}

// Deterministic UUIDs for system templates. These never change across
// releases — renaming or repurposing a template requires a new UUID.
var starterAgileProjectTemplateID = uuid.MustParse("00000000-0000-4000-a000-000000000001")

// starterAgileProjectTemplate seeds a lightweight agile project shape:
//   - priority + sprint + status custom fields
//   - one "Active sprint" saved view
//   - one default dashboard with a burndown widget
//   - two inactive automation starter rules
//   - one workflow definition reference to the basic-agile starter
//   - member role placeholders for PM / Lead Engineer / Reviewer
//
// Why "Starter Agile Project" as the single default: it exercises every
// sub-resource in the snapshot schema so clone regressions surface fast,
// and it mirrors what most new teams actually configure by hand on day 1.
func starterAgileProjectTemplate() *model.ProjectTemplate {
	snapshot := model.ProjectTemplateSnapshot{
		Version: model.CurrentProjectTemplateSnapshotVersion,
		Settings: model.ProjectTemplateSettingsSnapshot{
			ReviewPolicy: &model.ReviewPolicy{
				RequiredLayers:          []string{},
				RequireManualApproval:   false,
				MinRiskLevelForBlock:    "high",
				AutoTriggerOnPR:         true,
				EnabledPluginDimensions: []string{},
			},
		},
		CustomFields: []model.ProjectTemplateCustomFieldSnapshot{
			{
				Key:         "priority",
				Label:       "Priority",
				Type:        "select",
				Required:    false,
				Options:     json.RawMessage(`{"choices":["low","medium","high","critical"]}`),
				Description: "Task priority; drives sort order in default views.",
			},
			{
				Key:         "sprint",
				Label:       "Sprint",
				Type:        "select",
				Required:    false,
				Options:     json.RawMessage(`{"choices":[]}`),
				Description: "The sprint this task belongs to. Options fill as sprints are created.",
			},
			{
				Key:         "status",
				Label:       "Status",
				Type:        "select",
				Required:    true,
				Options:     json.RawMessage(`{"choices":["todo","in_progress","in_review","done"]}`),
				Description: "Workflow state. Independent from the project's core task status enum so that UI filters can layer on top.",
			},
		},
		SavedViews: []model.ProjectTemplateSavedViewSnapshot{
			{
				Name:        "Active sprint",
				Kind:        "board",
				Config:      json.RawMessage(`{"filters":{"sprint":"current"},"groupBy":"status"}`),
				Description: "Default sprint board view.",
			},
		},
		Dashboards: []model.ProjectTemplateDashboardSnapshot{
			{
				Name:        "Sprint overview",
				Description: "At-a-glance burndown + velocity for the current sprint.",
				Layout:      json.RawMessage(`{"cols":2,"rows":2}`),
				Widgets: []model.ProjectTemplateDashboardWidgetSnap{
					{
						Type:     "burndown",
						Title:    "Burndown",
						Config:   json.RawMessage(`{"window":"sprint"}`),
						Position: json.RawMessage(`{"x":0,"y":0,"w":2,"h":1}`),
					},
					{
						Type:     "velocity",
						Title:    "Velocity",
						Config:   json.RawMessage(`{"window":"6-sprints"}`),
						Position: json.RawMessage(`{"x":0,"y":1,"w":2,"h":1}`),
					},
				},
			},
		},
		Automations: []model.ProjectTemplateAutomationSnapshot{
			{
				Name:        "Stale task escalation",
				Description: "Flags tasks untouched for 7 days in the active sprint.",
				Trigger:     json.RawMessage(`{"type":"schedule","cron":"0 9 * * *"}`),
				Conditions:  json.RawMessage(`{"idleDays":7,"sprintField":"sprint","sprintValue":"current"}`),
				Actions:     json.RawMessage(`[{"type":"notify","channel":"project-notifications","template":"task_stale"}]`),
			},
			{
				Name:        "PR-linked status update",
				Description: "Moves a task to \"in_review\" when a linked PR opens.",
				Trigger:     json.RawMessage(`{"type":"event","event":"pr.opened"}`),
				Conditions:  json.RawMessage(`{"linkedTask":true}`),
				Actions:     json.RawMessage(`[{"type":"set_field","field":"status","value":"in_review"}]`),
			},
		},
		WorkflowDefinitions: []model.ProjectTemplateWorkflowSnapshot{
			{
				Name:           "Basic agile loop",
				Description:    "Plan → In progress → Review → Done.",
				TemplateRef:    "basic-agile-starter",
				DefinitionJSON: json.RawMessage(`{"nodes":[],"edges":[]}`),
			},
		},
		TaskStatuses: []model.ProjectTemplateTaskStatusSnapshot{},
		MemberRolePlaceholders: []model.ProjectTemplateMemberPlaceholder{
			{Label: "Project Manager", SuggestedRole: model.ProjectRoleAdmin, Description: "Owns sprint planning and status."},
			{Label: "Lead Engineer", SuggestedRole: model.ProjectRoleEditor, Description: "Owns the technical direction."},
			{Label: "Reviewer", SuggestedRole: model.ProjectRoleEditor, Description: "Primary code reviewer."},
		},
	}
	raw, _ := model.MarshalProjectTemplateSnapshot(snapshot)
	return &model.ProjectTemplate{
		ID:              starterAgileProjectTemplateID,
		Source:          model.ProjectTemplateSourceSystem,
		OwnerUserID:     nil,
		Name:            "Starter Agile Project",
		Description:     "An opinionated baseline covering custom fields, a sprint dashboard, two starter automation rules, and an agile workflow. A reasonable starting point for most product teams.",
		SnapshotJSON:    raw,
		SnapshotVersion: snapshot.Version,
	}
}
