package qcrepo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/agentforge/server/internal/model"
)

// QianchuanActionLogRepo is the persistence contract for action logs.
type QianchuanActionLogRepo interface {
	Create(ctx context.Context, log *model.QianchuanActionLog) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.QianchuanActionLog, error)
	MarkApplied(ctx context.Context, id uuid.UUID) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error
	MarkGated(ctx context.Context, id uuid.UUID, reason string) error
	ListByRun(ctx context.Context, runID uuid.UUID) ([]*model.QianchuanActionLog, error)
	ListByBinding(ctx context.Context, bindingID uuid.UUID, limit int) ([]*model.QianchuanActionLog, error)
}

// actionLogRow is the GORM model for qianchuan_action_logs.
type actionLogRow struct {
	ID            uuid.UUID       `gorm:"column:id;primaryKey"`
	BindingID     uuid.UUID       `gorm:"column:binding_id"`
	StrategyID    *uuid.UUID      `gorm:"column:strategy_id"`
	StrategyRunID uuid.UUID       `gorm:"column:strategy_run_id"`
	RuleName      string          `gorm:"column:rule_name"`
	ActionType    string          `gorm:"column:action_type"`
	TargetAdID    string          `gorm:"column:target_ad_id"`
	Params        json.RawMessage `gorm:"column:params;type:jsonb"`
	Status        string          `gorm:"column:status"`
	GateReason    string          `gorm:"column:gate_reason"`
	AppliedAt     *time.Time      `gorm:"column:applied_at"`
	ErrorMessage  string          `gorm:"column:error_message"`
	CreatedAt     time.Time       `gorm:"column:created_at"`
}

func (actionLogRow) TableName() string { return "qianchuan_action_logs" }

func actionLogRowToModel(r *actionLogRow) *model.QianchuanActionLog {
	return &model.QianchuanActionLog{
		ID: r.ID, BindingID: r.BindingID, StrategyID: r.StrategyID,
		StrategyRunID: r.StrategyRunID, RuleName: r.RuleName,
		ActionType: r.ActionType, TargetAdID: r.TargetAdID,
		Params: r.Params, Status: r.Status, GateReason: r.GateReason,
		AppliedAt: r.AppliedAt, ErrorMessage: r.ErrorMessage, CreatedAt: r.CreatedAt,
	}
}

func actionLogModelToRow(m *model.QianchuanActionLog) *actionLogRow {
	return &actionLogRow{
		ID: m.ID, BindingID: m.BindingID, StrategyID: m.StrategyID,
		StrategyRunID: m.StrategyRunID, RuleName: m.RuleName,
		ActionType: m.ActionType, TargetAdID: m.TargetAdID,
		Params: m.Params, Status: m.Status, GateReason: m.GateReason,
		AppliedAt: m.AppliedAt, ErrorMessage: m.ErrorMessage, CreatedAt: m.CreatedAt,
	}
}

// GormActionLogRepo is the GORM-backed implementation.
type GormActionLogRepo struct{ db *gorm.DB }

// NewGormActionLogRepo constructs an action log repo.
func NewGormActionLogRepo(db *gorm.DB) *GormActionLogRepo {
	return &GormActionLogRepo{db: db}
}

func (r *GormActionLogRepo) Create(ctx context.Context, log *model.QianchuanActionLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	if log.Status == "" {
		log.Status = model.ActionLogStatusPending
	}
	return r.db.WithContext(ctx).Create(actionLogModelToRow(log)).Error
}

func (r *GormActionLogRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.QianchuanActionLog, error) {
	var row actionLogRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return actionLogRowToModel(&row), nil
}

func (r *GormActionLogRepo) MarkApplied(ctx context.Context, id uuid.UUID) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&actionLogRow{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": model.ActionLogStatusApplied, "applied_at": now}).
		Error
}

func (r *GormActionLogRepo) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string) error {
	return r.db.WithContext(ctx).
		Model(&actionLogRow{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": model.ActionLogStatusFailed, "error_message": errMsg}).
		Error
}

func (r *GormActionLogRepo) MarkGated(ctx context.Context, id uuid.UUID, reason string) error {
	return r.db.WithContext(ctx).
		Model(&actionLogRow{}).
		Where("id = ?", id).
		Updates(map[string]any{"status": model.ActionLogStatusGated, "gate_reason": reason}).
		Error
}

func (r *GormActionLogRepo) ListByRun(ctx context.Context, runID uuid.UUID) ([]*model.QianchuanActionLog, error) {
	var rows []actionLogRow
	if err := r.db.WithContext(ctx).
		Where("strategy_run_id = ?", runID).
		Order("created_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.QianchuanActionLog, len(rows))
	for i := range rows {
		out[i] = actionLogRowToModel(&rows[i])
	}
	return out, nil
}

func (r *GormActionLogRepo) ListByBinding(ctx context.Context, bindingID uuid.UUID, limit int) ([]*model.QianchuanActionLog, error) {
	var rows []actionLogRow
	if err := r.db.WithContext(ctx).
		Where("binding_id = ?", bindingID).
		Order("created_at DESC").
		Limit(limit).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*model.QianchuanActionLog, len(rows))
	for i := range rows {
		out[i] = actionLogRowToModel(&rows[i])
	}
	return out, nil
}
