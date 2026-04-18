// Package model — audit_event.go declares the canonical audit event type
// persisted in `project_audit_events`. Both human-initiated writes and
// system-initiated automation writes feed the same row shape; readers
// distinguish them via SystemInitiated + ConfiguredByUserID.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Audit event ResourceType enum. Matches the CHECK constraint on
// project_audit_events.resource_type. Must stay in sync with the SQL.
const (
	AuditResourceTypeProject    = "project"
	AuditResourceTypeMember     = "member"
	AuditResourceTypeTask       = "task"
	AuditResourceTypeTeamRun    = "team_run"
	AuditResourceTypeWorkflow   = "workflow"
	AuditResourceTypeWiki       = "wiki"
	AuditResourceTypeSettings   = "settings"
	AuditResourceTypeAutomation = "automation"
	AuditResourceTypeDashboard  = "dashboard"
	AuditResourceTypeAuth       = "auth"
	AuditResourceTypeInvitation = "invitation"
)

// IsValidAuditResourceType reports whether v is one of the allowed types.
func IsValidAuditResourceType(v string) bool {
	switch v {
	case AuditResourceTypeProject, AuditResourceTypeMember, AuditResourceTypeTask,
		AuditResourceTypeTeamRun, AuditResourceTypeWorkflow, AuditResourceTypeWiki,
		AuditResourceTypeSettings, AuditResourceTypeAutomation, AuditResourceTypeDashboard,
		AuditResourceTypeAuth, AuditResourceTypeInvitation:
		return true
	}
	return false
}

// AuditEvent is the canonical record persisted in `project_audit_events`.
//
// Field semantics:
//   - ActorUserID may be nil only when SystemInitiated=true (e.g. scheduler
//     auto-trigger). For human-initiated and rbac_denied events ActorUserID
//     is required.
//   - ActorProjectRoleAtTime captures the caller's role at the moment of the
//     event so post-hoc reads do not need to reconstruct historical state.
//     Empty string allowed for system-initiated rows or for rbac_denied
//     events where the caller has no membership.
//   - ActionID MUST come from the canonical RBAC ActionID enum. The audit
//     service rejects writes that reference an undeclared ActionID.
//   - ConfiguredByUserID is set when SystemInitiated=true and refers to the
//     human who last authorized the automation/scheduler binding.
//   - PayloadSnapshotJSON is the sanitized, size-bounded JSON of the change.
//     The sanitizer redacts known sensitive field names and truncates at
//     64 KB with a `_truncated:true` marker.
type AuditEvent struct {
	ID                       uuid.UUID  `db:"id"`
	ProjectID                uuid.UUID  `db:"project_id"`
	OccurredAt               time.Time  `db:"occurred_at"`
	ActorUserID              *uuid.UUID `db:"actor_user_id"`
	ActorProjectRoleAtTime   string     `db:"actor_project_role_at_time"`
	ActionID                 string     `db:"action_id"`
	ResourceType             string     `db:"resource_type"`
	ResourceID               string     `db:"resource_id"`
	PayloadSnapshotJSON      string     `db:"payload_snapshot_json"`
	SystemInitiated          bool       `db:"system_initiated"`
	ConfiguredByUserID       *uuid.UUID `db:"configured_by_user_id"`
	RequestID                string     `db:"request_id"`
	IP                       string     `db:"ip"`
	UserAgent                string     `db:"user_agent"`
	CreatedAt                time.Time  `db:"created_at"`
}

// AuditEventDTO is the JSON shape returned to API consumers. Field names
// match the documented audit query API.
type AuditEventDTO struct {
	ID                       string  `json:"id"`
	ProjectID                string  `json:"projectId"`
	OccurredAt               string  `json:"occurredAt"`
	ActorUserID              *string `json:"actorUserId,omitempty"`
	ActorProjectRoleAtTime   string  `json:"actorProjectRoleAtTime,omitempty"`
	ActionID                 string  `json:"actionId"`
	ResourceType             string  `json:"resourceType"`
	ResourceID               string  `json:"resourceId,omitempty"`
	PayloadSnapshot          string  `json:"payloadSnapshotJson"`
	SystemInitiated          bool    `json:"systemInitiated"`
	ConfiguredByUserID       *string `json:"configuredByUserId,omitempty"`
	RequestID                string  `json:"requestId,omitempty"`
	IP                       string  `json:"ip,omitempty"`
	UserAgent                string  `json:"userAgent,omitempty"`
}

func (e *AuditEvent) ToDTO() AuditEventDTO {
	dto := AuditEventDTO{
		ID:                     e.ID.String(),
		ProjectID:              e.ProjectID.String(),
		OccurredAt:             e.OccurredAt.UTC().Format(time.RFC3339Nano),
		ActorProjectRoleAtTime: e.ActorProjectRoleAtTime,
		ActionID:               e.ActionID,
		ResourceType:           e.ResourceType,
		ResourceID:             e.ResourceID,
		PayloadSnapshot:        e.PayloadSnapshotJSON,
		SystemInitiated:        e.SystemInitiated,
		RequestID:              e.RequestID,
		IP:                     e.IP,
		UserAgent:              e.UserAgent,
	}
	if e.ActorUserID != nil {
		s := e.ActorUserID.String()
		dto.ActorUserID = &s
	}
	if e.ConfiguredByUserID != nil {
		s := e.ConfiguredByUserID.String()
		dto.ConfiguredByUserID = &s
	}
	return dto
}

// AuditEventListResponse is the paginated list-endpoint response.
type AuditEventListResponse struct {
	Events     []AuditEventDTO `json:"events"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

// AuditEventQueryFilters captures the supported list filters.
type AuditEventQueryFilters struct {
	ActionID     string
	ActorUserID  string
	ResourceType string
	ResourceID   string
	From         *time.Time
	To           *time.Time
}
