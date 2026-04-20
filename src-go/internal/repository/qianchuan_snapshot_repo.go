// Package repository provides data-access implementations for the Qianchuan
// strategy execution loop (Spec 3D).
package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/react-go-quick-starter/server/internal/model"
)

// QianchuanSnapshotRepo is the persistence contract for metric snapshots.
type QianchuanSnapshotRepo interface {
	Upsert(ctx context.Context, bindingID uuid.UUID, minuteBucket time.Time, payload json.RawMessage) error
	ListByBinding(ctx context.Context, bindingID uuid.UUID, limit int) ([]*model.QianchuanMetricSnapshot, error)
	Latest(ctx context.Context, bindingID uuid.UUID) (*model.QianchuanMetricSnapshot, error)
}

// snapshotRow is the GORM model for qianchuan_metric_snapshots.
type snapshotRow struct {
	ID           int64           `gorm:"column:id;primaryKey"`
	BindingID    uuid.UUID       `gorm:"column:binding_id"`
	MinuteBucket time.Time       `gorm:"column:minute_bucket"`
	Payload      json.RawMessage `gorm:"column:payload;type:jsonb"`
	CreatedAt    time.Time       `gorm:"column:created_at"`
}

func (snapshotRow) TableName() string { return "qianchuan_metric_snapshots" }

func snapshotRowToModel(r *snapshotRow) *model.QianchuanMetricSnapshot {
	return &model.QianchuanMetricSnapshot{
		ID:           r.ID,
		BindingID:    r.BindingID,
		MinuteBucket: r.MinuteBucket,
		Payload:      r.Payload,
		CreatedAt:    r.CreatedAt,
	}
}

// GormSnapshotRepo is the GORM-backed implementation.
type GormSnapshotRepo struct{ db *gorm.DB }

// NewGormSnapshotRepo constructs a snapshot repo.
func NewGormSnapshotRepo(db *gorm.DB) *GormSnapshotRepo {
	return &GormSnapshotRepo{db: db}
}

// Upsert inserts or updates a snapshot row. On conflict (binding_id, minute_bucket)
// it overwrites the payload.
func (r *GormSnapshotRepo) Upsert(ctx context.Context, bindingID uuid.UUID, minuteBucket time.Time, payload json.RawMessage) error {
	sql := `INSERT INTO qianchuan_metric_snapshots (binding_id, minute_bucket, payload, created_at)
			VALUES (?, ?, ?, now())
			ON CONFLICT (binding_id, minute_bucket) DO UPDATE SET payload = EXCLUDED.payload`
	return r.db.WithContext(ctx).Exec(sql, bindingID, minuteBucket, payload).Error
}

// ListByBinding returns the most recent snapshots for a binding.
func (r *GormSnapshotRepo) ListByBinding(ctx context.Context, bindingID uuid.UUID, limit int) ([]*model.QianchuanMetricSnapshot, error) {
	var rows []snapshotRow
	if err := r.db.WithContext(ctx).
		Where("binding_id = ?", bindingID).
		Order("minute_bucket DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.QianchuanMetricSnapshot, len(rows))
	for i := range rows {
		out[i] = snapshotRowToModel(&rows[i])
	}
	return out, nil
}

// Latest returns the most recent snapshot for a binding, or nil if none exists.
func (r *GormSnapshotRepo) Latest(ctx context.Context, bindingID uuid.UUID) (*model.QianchuanMetricSnapshot, error) {
	var row snapshotRow
	if err := r.db.WithContext(ctx).
		Where("binding_id = ?", bindingID).
		Order("minute_bucket DESC").
		First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return snapshotRowToModel(&row), nil
}
