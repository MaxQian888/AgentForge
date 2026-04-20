// Package repository — audit_event_repo.go persists project_audit_events.
//
// The repo intentionally has a narrow surface (Insert + List). It does NOT
// know about the RBAC ActionID enum or sanitization — those are upstream
// service-layer concerns. The repo accepts whatever the service hands it.
package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditEventRepository struct {
	db *gorm.DB
}

func NewAuditEventRepository(db *gorm.DB) *AuditEventRepository {
	return &AuditEventRepository{db: db}
}

type auditEventRecord struct {
	ID                     uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID              uuid.UUID  `gorm:"column:project_id"`
	OccurredAt             time.Time  `gorm:"column:occurred_at"`
	ActorUserID            *uuid.UUID `gorm:"column:actor_user_id"`
	ActorProjectRoleAtTime string     `gorm:"column:actor_project_role_at_time"`
	ActionID               string     `gorm:"column:action_id"`
	ResourceType           string     `gorm:"column:resource_type"`
	ResourceID             string     `gorm:"column:resource_id"`
	PayloadSnapshotJSON    jsonText   `gorm:"column:payload_snapshot_json;type:jsonb"`
	SystemInitiated        bool       `gorm:"column:system_initiated"`
	ConfiguredByUserID     *uuid.UUID `gorm:"column:configured_by_user_id"`
	RequestID              string     `gorm:"column:request_id"`
	IP                     string     `gorm:"column:ip"`
	UserAgent              string     `gorm:"column:user_agent"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
}

func (auditEventRecord) TableName() string { return "project_audit_events" }

func newAuditEventRecord(event *model.AuditEvent) *auditEventRecord {
	if event == nil {
		return nil
	}
	if event.ID == uuid.Nil {
		event.ID = uuid.New()
	}
	if event.OccurredAt.IsZero() {
		event.OccurredAt = time.Now().UTC()
	}
	return &auditEventRecord{
		ID:                     event.ID,
		ProjectID:              event.ProjectID,
		OccurredAt:             event.OccurredAt.UTC(),
		ActorUserID:            event.ActorUserID,
		ActorProjectRoleAtTime: event.ActorProjectRoleAtTime,
		ActionID:               event.ActionID,
		ResourceType:           event.ResourceType,
		ResourceID:             event.ResourceID,
		PayloadSnapshotJSON:    newJSONText(event.PayloadSnapshotJSON, "{}"),
		SystemInitiated:        event.SystemInitiated,
		ConfiguredByUserID:     event.ConfiguredByUserID,
		RequestID:              event.RequestID,
		IP:                     event.IP,
		UserAgent:              event.UserAgent,
		CreatedAt:              event.CreatedAt,
	}
}

func (r *auditEventRecord) toModel() *model.AuditEvent {
	if r == nil {
		return nil
	}
	return &model.AuditEvent{
		ID:                     r.ID,
		ProjectID:              r.ProjectID,
		OccurredAt:             r.OccurredAt,
		ActorUserID:            r.ActorUserID,
		ActorProjectRoleAtTime: r.ActorProjectRoleAtTime,
		ActionID:               r.ActionID,
		ResourceType:           r.ResourceType,
		ResourceID:             r.ResourceID,
		PayloadSnapshotJSON:    r.PayloadSnapshotJSON.String("{}"),
		SystemInitiated:        r.SystemInitiated,
		ConfiguredByUserID:     r.ConfiguredByUserID,
		RequestID:              r.RequestID,
		IP:                     r.IP,
		UserAgent:              r.UserAgent,
		CreatedAt:              r.CreatedAt,
	}
}

// Insert persists a single audit event. Returns ErrDatabaseUnavailable when
// no DB is wired, allowing the AuditSink to escalate to its retry queue.
func (r *AuditEventRepository) Insert(ctx context.Context, event *model.AuditEvent) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if event == nil {
		return fmt.Errorf("insert audit event: event is nil")
	}
	if err := r.db.WithContext(ctx).Create(newAuditEventRecord(event)).Error; err != nil {
		return fmt.Errorf("insert audit event: %w", err)
	}
	return nil
}

// GetByID fetches a single audit event for the detail endpoint.
func (r *AuditEventRepository) GetByID(ctx context.Context, projectID, eventID uuid.UUID) (*model.AuditEvent, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var record auditEventRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND id = ?", projectID, eventID).
		Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get audit event: %w", normalizeRepositoryError(err))
	}
	return record.toModel(), nil
}

// List returns events matching filters in reverse-chronological order with
// cursor pagination. Cursor encodes (occurred_at, id) so consecutive pages
// don't drop rows that share the same occurred_at.
func (r *AuditEventRepository) List(
	ctx context.Context,
	projectID uuid.UUID,
	filters model.AuditEventQueryFilters,
	cursor string,
	limit int,
) ([]*model.AuditEvent, string, error) {
	if r.db == nil {
		return nil, "", ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	q := r.db.WithContext(ctx).
		Model(&auditEventRecord{}).
		Where("project_id = ?", projectID)

	if filters.ActionID != "" {
		q = q.Where("action_id = ?", filters.ActionID)
	}
	if filters.ActorUserID != "" {
		uid, err := uuid.Parse(filters.ActorUserID)
		if err != nil {
			return nil, "", fmt.Errorf("invalid actorUserId filter: %w", err)
		}
		q = q.Where("actor_user_id = ?", uid)
	}
	if filters.ResourceType != "" {
		q = q.Where("resource_type = ?", filters.ResourceType)
	}
	if filters.ResourceID != "" {
		q = q.Where("resource_id = ?", filters.ResourceID)
	}
	if filters.From != nil {
		q = q.Where("occurred_at >= ?", filters.From.UTC())
	}
	if filters.To != nil {
		q = q.Where("occurred_at <= ?", filters.To.UTC())
	}
	if cursor != "" {
		t, id, err := decodeAuditCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		// Reverse-chronological pagination: rows before the cursor.
		q = q.Where("(occurred_at, id) < (?, ?)", t.UTC(), id)
	}

	var records []auditEventRecord
	if err := q.Order("occurred_at DESC").Order("id DESC").
		Limit(limit + 1).
		Find(&records).Error; err != nil {
		return nil, "", fmt.Errorf("list audit events: %w", err)
	}

	nextCursor := ""
	if len(records) > limit {
		last := records[limit-1]
		nextCursor = encodeAuditCursor(last.OccurredAt, last.ID)
		records = records[:limit]
	}
	out := make([]*model.AuditEvent, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nextCursor, nil
}

func encodeAuditCursor(t time.Time, id uuid.UUID) string {
	payload := map[string]string{
		"t":  t.UTC().Format(time.RFC3339Nano),
		"id": id.String(),
	}
	b, _ := json.Marshal(payload)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeAuditCursor(cursor string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(cursor))
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	var payload struct {
		T  string `json:"t"`
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return time.Time{}, uuid.Nil, err
	}
	t, err := time.Parse(time.RFC3339Nano, payload.T)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	id, err := uuid.Parse(payload.ID)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	return t, id, nil
}
