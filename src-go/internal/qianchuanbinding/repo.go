// Package qianchuanbinding owns persistence + business operations for
// qianchuan_bindings rows. Token plaintext NEVER lives here; the
// *_secret_ref columns hold secrets.name strings owned by Plan 1B.
//
// Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §6.1
package qianchuanbinding

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrNotFound is returned when no row matches the lookup.
var ErrNotFound = errors.New("qianchuanbinding: not found")

// ErrAdvertiserAlreadyBound is returned on (project, advertiser, aweme) UNIQUE conflict.
var ErrAdvertiserAlreadyBound = errors.New("qianchuanbinding: advertiser_already_bound")

// Status enum.
const (
	StatusActive      = "active"
	StatusAuthExpired = "auth_expired"
	StatusPaused      = "paused"
)

// Record is the in-memory representation of one qianchuan_bindings row.
type Record struct {
	ID                    uuid.UUID
	ProjectID             uuid.UUID
	AdvertiserID          string
	AwemeID               string
	DisplayName           string
	Status                string
	ActingEmployeeID      *uuid.UUID
	AccessTokenSecretRef  string
	RefreshTokenSecretRef string
	TokenExpiresAt        *time.Time
	LastSyncedAt          *time.Time
	CreatedBy             uuid.UUID
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

// Repository is the persistence contract used by the service layer.
type Repository interface {
	Create(ctx context.Context, r *Record) error
	Get(ctx context.Context, id uuid.UUID) (*Record, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*Record, error)
	Update(ctx context.Context, r *Record) error
	Delete(ctx context.Context, id uuid.UUID) error
	TouchSync(ctx context.Context, id uuid.UUID, when time.Time) error
}

// ---------------- GORM impl ----------------

type bindingRow struct {
	ID                    uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID             uuid.UUID  `gorm:"column:project_id"`
	AdvertiserID          string     `gorm:"column:advertiser_id"`
	AwemeID               string     `gorm:"column:aweme_id"`
	DisplayName           string     `gorm:"column:display_name"`
	Status                string     `gorm:"column:status"`
	ActingEmployeeID      *uuid.UUID `gorm:"column:acting_employee_id"`
	AccessTokenSecretRef  string     `gorm:"column:access_token_secret_ref"`
	RefreshTokenSecretRef string     `gorm:"column:refresh_token_secret_ref"`
	TokenExpiresAt        *time.Time `gorm:"column:token_expires_at"`
	LastSyncedAt          *time.Time `gorm:"column:last_synced_at"`
	CreatedBy             uuid.UUID  `gorm:"column:created_by"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (bindingRow) TableName() string { return "qianchuan_bindings" }

func toRecord(r *bindingRow) *Record {
	return &Record{
		ID: r.ID, ProjectID: r.ProjectID, AdvertiserID: r.AdvertiserID, AwemeID: r.AwemeID,
		DisplayName: r.DisplayName, Status: r.Status, ActingEmployeeID: r.ActingEmployeeID,
		AccessTokenSecretRef: r.AccessTokenSecretRef, RefreshTokenSecretRef: r.RefreshTokenSecretRef,
		TokenExpiresAt: r.TokenExpiresAt, LastSyncedAt: r.LastSyncedAt,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromRecord(r *Record) *bindingRow {
	return &bindingRow{
		ID: r.ID, ProjectID: r.ProjectID, AdvertiserID: r.AdvertiserID, AwemeID: r.AwemeID,
		DisplayName: r.DisplayName, Status: r.Status, ActingEmployeeID: r.ActingEmployeeID,
		AccessTokenSecretRef: r.AccessTokenSecretRef, RefreshTokenSecretRef: r.RefreshTokenSecretRef,
		TokenExpiresAt: r.TokenExpiresAt, LastSyncedAt: r.LastSyncedAt,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

// GormRepo is the production Repository implementation.
type GormRepo struct{ db *gorm.DB }

// NewGormRepo wires a Repository on top of the shared GORM DB.
func NewGormRepo(db *gorm.DB) *GormRepo { return &GormRepo{db: db} }

func (r *GormRepo) Create(ctx context.Context, rec *Record) error {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	if rec.Status == "" {
		rec.Status = StatusActive
	}
	if err := r.db.WithContext(ctx).Create(fromRecord(rec)).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrAdvertiserAlreadyBound
		}
		return err
	}
	return nil
}

func (r *GormRepo) Get(ctx context.Context, id uuid.UUID) (*Record, error) {
	var row bindingRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return toRecord(&row), nil
}

func (r *GormRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
	var rows []bindingRow
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*Record, 0, len(rows))
	for i := range rows {
		out = append(out, toRecord(&rows[i]))
	}
	return out, nil
}

func (r *GormRepo) Update(ctx context.Context, rec *Record) error {
	rec.UpdatedAt = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&bindingRow{}).
		Where("id = ?", rec.ID).
		Updates(map[string]any{
			"display_name":       rec.DisplayName,
			"status":             rec.Status,
			"acting_employee_id": rec.ActingEmployeeID,
			"token_expires_at":   rec.TokenExpiresAt,
			"last_synced_at":     rec.LastSyncedAt,
			"updated_at":         rec.UpdatedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepo) Delete(ctx context.Context, id uuid.UUID) error {
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&bindingRow{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepo) TouchSync(ctx context.Context, id uuid.UUID, when time.Time) error {
	return r.db.WithContext(ctx).Model(&bindingRow{}).
		Where("id = ?", id).
		Update("last_synced_at", when).Error
}

// FindDueForRefresh returns active bindings whose token_expires_at is before
// the supplied threshold. Caller passes now()+earlyWindow.
func (r *GormRepo) FindDueForRefresh(ctx context.Context, before time.Time) ([]*Record, error) {
	var rows []bindingRow
	if err := r.db.WithContext(ctx).
		Where("status = ? AND token_expires_at < ?", StatusActive, before).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*Record, 0, len(rows))
	for i := range rows {
		out = append(out, toRecord(&rows[i]))
	}
	return out, nil
}

// UpdateExpiry updates the token_expires_at timestamp on a binding after
// a successful token refresh.
func (r *GormRepo) UpdateExpiry(ctx context.Context, id uuid.UUID, expiresAt time.Time) error {
	return r.db.WithContext(ctx).Model(&bindingRow{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"token_expires_at": expiresAt,
			"updated_at":       time.Now().UTC(),
		}).Error
}

// MarkAuthExpired transitions a binding to auth_expired status.
func (r *GormRepo) MarkAuthExpired(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&bindingRow{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     StatusAuthExpired,
			"updated_at": time.Now().UTC(),
		}).Error
}
