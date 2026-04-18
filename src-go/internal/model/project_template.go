// Package model — project_template.go declares the project_templates entity
// and its snapshot envelope. A project template is a reusable snapshot of a
// project's *configuration* (not its business data): settings, custom fields,
// saved views, dashboards, automations, workflow definitions, task statuses,
// and advisory role placeholders. See:
//   - openspec/changes/2026-04-17-add-project-templates/proposal.md
//   - openspec/changes/2026-04-17-add-project-templates/design.md
package model

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Project template source categories. Matches the CHECK on
// project_templates.source and drives the visibility / edit-permission rules.
const (
	ProjectTemplateSourceSystem      = "system"
	ProjectTemplateSourceUser        = "user"
	ProjectTemplateSourceMarketplace = "marketplace"
)

// CurrentProjectTemplateSnapshotVersion is the schema version stamped into
// newly built snapshots. Bump when the snapshot top-level shape changes and
// register an upgrade in service.projectTemplateSnapshotUpgrades.
const CurrentProjectTemplateSnapshotVersion = 1

// IsValidProjectTemplateSource reports whether v is a recognized source value.
func IsValidProjectTemplateSource(v string) bool {
	switch v {
	case ProjectTemplateSourceSystem, ProjectTemplateSourceUser, ProjectTemplateSourceMarketplace:
		return true
	}
	return false
}

// NormalizeProjectTemplateSource lowercases/trims and returns "" for unknowns.
func NormalizeProjectTemplateSource(v string) string {
	s := strings.ToLower(strings.TrimSpace(v))
	if !IsValidProjectTemplateSource(s) {
		return ""
	}
	return s
}

// ProjectTemplate is the canonical in-memory representation of a row in
// project_templates. owner_user_id is nil for system templates.
type ProjectTemplate struct {
	ID              uuid.UUID  `db:"id"`
	Source          string     `db:"source"`
	OwnerUserID     *uuid.UUID `db:"owner_user_id"`
	Name            string     `db:"name"`
	Description     string     `db:"description"`
	SnapshotJSON    string     `db:"snapshot_json"`
	SnapshotVersion int        `db:"snapshot_version"`
	CreatedAt       time.Time  `db:"created_at"`
	UpdatedAt       time.Time  `db:"updated_at"`
}

// ProjectTemplateDTO is the JSON shape returned to API consumers.
type ProjectTemplateDTO struct {
	ID              string  `json:"id"`
	Source          string  `json:"source"`
	OwnerUserID     *string `json:"ownerUserId,omitempty"`
	Name            string  `json:"name"`
	Description     string  `json:"description,omitempty"`
	SnapshotVersion int     `json:"snapshotVersion"`
	Snapshot        any     `json:"snapshot,omitempty"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

// ToDTO converts a template row to its API representation. `snapshot` is
// opaque JSON — callers that do not want to ship the full payload back should
// zero it out after calling ToDTO.
func (t *ProjectTemplate) ToDTO() ProjectTemplateDTO {
	dto := ProjectTemplateDTO{
		ID:              t.ID.String(),
		Source:          t.Source,
		Name:            t.Name,
		Description:     t.Description,
		SnapshotVersion: t.SnapshotVersion,
		CreatedAt:       t.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:       t.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if t.OwnerUserID != nil {
		s := t.OwnerUserID.String()
		dto.OwnerUserID = &s
	}
	return dto
}

// CreateProjectTemplateRequest is the body accepted by
// `POST /projects/:pid/save-as-template`. Snapshot content is assembled by
// the service layer from the referenced project — callers cannot override it.
type CreateProjectTemplateRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=128"`
	Description string `json:"description" validate:"max=4096"`
}

// UpdateProjectTemplateRequest is the body accepted by
// `PUT /project-templates/:id`. Only metadata is editable; the snapshot is
// immutable after initial save (users recreate a template to refresh content).
type UpdateProjectTemplateRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=128"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=4096"`
}

// ProjectTemplateClonePayload extends CreateProjectRequest for the "clone
// from template" path. When TemplateID is empty the existing blank-project
// path applies.
type ProjectTemplateClonePayload struct {
	TemplateSource string `json:"templateSource,omitempty"`
	TemplateID     string `json:"templateId,omitempty"`
}

// ProjectTemplateListResponse is the list-endpoint body. Snapshots are omitted
// from list results; callers fetch the full snapshot via Get.
type ProjectTemplateListResponse struct {
	Templates []ProjectTemplateDTO `json:"templates"`
}
