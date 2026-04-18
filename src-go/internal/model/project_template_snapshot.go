// Package model — project_template_snapshot.go declares the typed envelope
// that lives inside a project template's snapshot_json column.
//
// Rule: every top-level field here is fixed. Adding a new sub-resource to a
// template requires a snapshot version bump and an entry in the service-
// layer upgrade registry. This keeps clones deterministic across releases.
package model

import (
	"encoding/json"
)

// ProjectTemplateSnapshot is the versioned envelope for a template payload.
// Omitted sub-resources use zero-length slices rather than nil so that
// Build→Marshal→Parse round-trips are stable.
type ProjectTemplateSnapshot struct {
	Version                int                                     `json:"version"`
	Settings               ProjectTemplateSettingsSnapshot         `json:"settings"`
	CustomFields           []ProjectTemplateCustomFieldSnapshot    `json:"customFields"`
	SavedViews             []ProjectTemplateSavedViewSnapshot      `json:"savedViews"`
	Dashboards             []ProjectTemplateDashboardSnapshot      `json:"dashboards"`
	Automations            []ProjectTemplateAutomationSnapshot     `json:"automations"`
	WorkflowDefinitions    []ProjectTemplateWorkflowSnapshot       `json:"workflowDefinitions"`
	TaskStatuses           []ProjectTemplateTaskStatusSnapshot     `json:"taskStatuses"`
	MemberRolePlaceholders []ProjectTemplateMemberPlaceholder      `json:"memberRolePlaceholders"`
}

// NewEmptyProjectTemplateSnapshot returns a zero-value snapshot with version
// set and all slices initialized. Useful for builders and tests.
func NewEmptyProjectTemplateSnapshot() ProjectTemplateSnapshot {
	return ProjectTemplateSnapshot{
		Version:                CurrentProjectTemplateSnapshotVersion,
		CustomFields:           []ProjectTemplateCustomFieldSnapshot{},
		SavedViews:             []ProjectTemplateSavedViewSnapshot{},
		Dashboards:             []ProjectTemplateDashboardSnapshot{},
		Automations:            []ProjectTemplateAutomationSnapshot{},
		WorkflowDefinitions:    []ProjectTemplateWorkflowSnapshot{},
		TaskStatuses:           []ProjectTemplateTaskStatusSnapshot{},
		MemberRolePlaceholders: []ProjectTemplateMemberPlaceholder{},
	}
}

// ProjectTemplateSettingsSnapshot captures the whitelisted subset of project
// settings that is safe to copy across projects. Tokens, secrets, webhook
// credentials, and anything user-specific are rejected by the sanitizer.
type ProjectTemplateSettingsSnapshot struct {
	ReviewPolicy       *ReviewPolicy         `json:"reviewPolicy,omitempty"`
	DefaultCodingAgent *CodingAgentSelection `json:"defaultCodingAgent,omitempty"`
	BudgetGovernance   *BudgetGovernance     `json:"budgetGovernance,omitempty"`
}

// ProjectTemplateCustomFieldSnapshot mirrors a custom field definition with
// identifiers stripped. Order is semantic (display order of fields).
type ProjectTemplateCustomFieldSnapshot struct {
	Key         string          `json:"key"`
	Label       string          `json:"label"`
	Type        string          `json:"type"`
	Required    bool            `json:"required,omitempty"`
	Options     json.RawMessage `json:"options,omitempty"`
	Description string          `json:"description,omitempty"`
}

// ProjectTemplateSavedViewSnapshot captures a saved view without its owner.
type ProjectTemplateSavedViewSnapshot struct {
	Name        string          `json:"name"`
	Kind        string          `json:"kind"`
	Config      json.RawMessage `json:"config"`
	Shared      bool            `json:"shared,omitempty"`
	Description string          `json:"description,omitempty"`
}

// ProjectTemplateDashboardSnapshot captures a dashboard + its widgets.
type ProjectTemplateDashboardSnapshot struct {
	Name        string                                `json:"name"`
	Description string                                `json:"description,omitempty"`
	Layout      json.RawMessage                       `json:"layout,omitempty"`
	Widgets     []ProjectTemplateDashboardWidgetSnap  `json:"widgets,omitempty"`
}

type ProjectTemplateDashboardWidgetSnap struct {
	Type     string          `json:"type"`
	Title    string          `json:"title,omitempty"`
	Config   json.RawMessage `json:"config,omitempty"`
	Position json.RawMessage `json:"position,omitempty"`
}

// ProjectTemplateAutomationSnapshot carries rule definition only; the
// `configuredByUserID` (who authorized the automation) is intentionally
// stripped — the clone initiator rebinds it on activation. New automations
// materialize as inactive until the initiator re-confirms them.
type ProjectTemplateAutomationSnapshot struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Trigger     json.RawMessage `json:"trigger"`
	Conditions  json.RawMessage `json:"conditions,omitempty"`
	Actions     json.RawMessage `json:"actions"`
}

// ProjectTemplateWorkflowSnapshot captures a project-owned workflow definition.
// TemplateRef preserves the link to a workflow-template-library entry so
// clone can delegate to that library's clone path rather than reimplementing.
type ProjectTemplateWorkflowSnapshot struct {
	Name           string          `json:"name"`
	Description    string          `json:"description,omitempty"`
	TemplateRef    string          `json:"templateRef,omitempty"`
	DefinitionJSON json.RawMessage `json:"definitionJson"`
}

// ProjectTemplateTaskStatusSnapshot captures a custom task status. Projects
// that use the fixed core status set yield an empty list.
type ProjectTemplateTaskStatusSnapshot struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Category    string `json:"category,omitempty"`
	Order       int    `json:"order,omitempty"`
	Description string `json:"description,omitempty"`
}

// ProjectTemplateMemberPlaceholder is advisory only — it does NOT
// create pending invitations. UI surfaces these as a bootstrap checklist.
type ProjectTemplateMemberPlaceholder struct {
	Label         string `json:"label"`
	SuggestedRole string `json:"suggestedRole,omitempty"`
	Description   string `json:"description,omitempty"`
}

// MarshalProjectTemplateSnapshot serializes a snapshot to canonical JSON.
func MarshalProjectTemplateSnapshot(s ProjectTemplateSnapshot) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

// ParseProjectTemplateSnapshot round-trips a JSON payload into the typed
// envelope. An empty payload parses to an empty current-version snapshot so
// callers do not branch on "never saved before".
func ParseProjectTemplateSnapshot(raw string) (ProjectTemplateSnapshot, error) {
	if raw == "" {
		return NewEmptyProjectTemplateSnapshot(), nil
	}
	var s ProjectTemplateSnapshot
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return ProjectTemplateSnapshot{}, err
	}
	if s.Version == 0 {
		s.Version = CurrentProjectTemplateSnapshotVersion
	}
	if s.CustomFields == nil {
		s.CustomFields = []ProjectTemplateCustomFieldSnapshot{}
	}
	if s.SavedViews == nil {
		s.SavedViews = []ProjectTemplateSavedViewSnapshot{}
	}
	if s.Dashboards == nil {
		s.Dashboards = []ProjectTemplateDashboardSnapshot{}
	}
	if s.Automations == nil {
		s.Automations = []ProjectTemplateAutomationSnapshot{}
	}
	if s.WorkflowDefinitions == nil {
		s.WorkflowDefinitions = []ProjectTemplateWorkflowSnapshot{}
	}
	if s.TaskStatuses == nil {
		s.TaskStatuses = []ProjectTemplateTaskStatusSnapshot{}
	}
	if s.MemberRolePlaceholders == nil {
		s.MemberRolePlaceholders = []ProjectTemplateMemberPlaceholder{}
	}
	return s, nil
}
