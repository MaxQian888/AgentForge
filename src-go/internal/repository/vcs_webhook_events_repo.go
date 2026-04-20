package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ErrVCSWebhookEventDuplicate is returned when an Insert violates the
// UNIQUE(integration_id, event_id) constraint — the event was already
// processed (or is being processed). Callers treat this as a safe no-op.
var ErrVCSWebhookEventDuplicate = errors.New("vcs_webhook_event: duplicate (integration_id, event_id)")

// ErrVCSIntegrationNotFound is returned when no integration matches the
// host/owner/repo triple during webhook resolution.
var ErrVCSIntegrationNotFound = errors.New("vcs_integration: not found for repo")

// VCSWebhookEventsRepo persists inbound webhook delivery events for
// dedup + audit purposes.
type VCSWebhookEventsRepo struct{ db *gorm.DB }

func NewVCSWebhookEventsRepo(db *gorm.DB) *VCSWebhookEventsRepo {
	return &VCSWebhookEventsRepo{db: db}
}

type vcsWebhookEventRecord struct {
	ID              uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	IntegrationID   uuid.UUID  `gorm:"column:integration_id;type:uuid;not null"`
	EventID         string     `gorm:"column:event_id;not null"`
	EventType       string     `gorm:"column:event_type;not null"`
	PayloadHash     []byte     `gorm:"column:payload_hash;not null"`
	ReceivedAt      time.Time  `gorm:"column:received_at;not null"`
	ProcessedAt     *time.Time `gorm:"column:processed_at"`
	ProcessingError *string    `gorm:"column:processing_error"`
}

func (vcsWebhookEventRecord) TableName() string { return "vcs_webhook_events" }

// Insert persists a new webhook event. Returns ErrVCSWebhookEventDuplicate
// if the (integration_id, event_id) pair already exists.
func (r *VCSWebhookEventsRepo) Insert(ctx context.Context, e *model.VCSWebhookEvent) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.ReceivedAt.IsZero() {
		e.ReceivedAt = time.Now().UTC()
	}
	rec := &vcsWebhookEventRecord{
		ID:            e.ID,
		IntegrationID: e.IntegrationID,
		EventID:       e.EventID,
		EventType:     e.EventType,
		PayloadHash:   e.PayloadHash,
		ReceivedAt:    e.ReceivedAt,
	}
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		if isDuplicateKeyError(err) {
			return ErrVCSWebhookEventDuplicate
		}
		return err
	}
	return nil
}

// MarkProcessed updates the processed_at timestamp and optional error.
func (r *VCSWebhookEventsRepo) MarkProcessed(ctx context.Context, id uuid.UUID, procErr string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	updates := map[string]any{"processed_at": now}
	if procErr != "" {
		updates["processing_error"] = procErr
	}
	return r.db.WithContext(ctx).
		Model(&vcsWebhookEventRecord{}).
		Where("id = ?", id).
		Updates(updates).Error
}

// isDuplicateKeyError checks for PostgreSQL unique-violation (23505).
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// GORM wraps pgconn.PgError; check the string for the code.
	return errors.Is(err, gorm.ErrDuplicatedKey) ||
		containsDuplicateKeyCode(err)
}

func containsDuplicateKeyCode(err error) bool {
	// pgconn.PgError surfaces Code = "23505" for unique violations.
	return err != nil && (errors.As(err, new(interface{ SQLState() string })) ||
		// Fallback: string match for GORM drivers that don't expose typed errors.
		len(err.Error()) > 0 && (contains(err.Error(), "23505") || contains(err.Error(), "duplicate key")))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
