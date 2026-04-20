package qcrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/agentforge/server/plugins/qianchuan-ads/strategy"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// QianchuanStrategyRepository persists strategy.QianchuanStrategy rows.
// Project-scoped rows have a non-nil ProjectID; system seeds use NULL.
type QianchuanStrategyRepository struct {
	db *gorm.DB
}

func NewQianchuanStrategyRepository(db *gorm.DB) *QianchuanStrategyRepository {
	return &QianchuanStrategyRepository{db: db}
}

type qianchuanStrategyRecord struct {
	ID          uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID   *uuid.UUID `gorm:"column:project_id;type:uuid"`
	Name        string     `gorm:"column:name;not null"`
	Description string     `gorm:"column:description"`
	YAMLSource  string     `gorm:"column:yaml_source;not null"`
	ParsedSpec  jsonText   `gorm:"column:parsed_spec;type:jsonb;not null"`
	Version     int        `gorm:"column:version;not null;default:1"`
	Status      string     `gorm:"column:status;not null;default:draft"`
	CreatedBy   uuid.UUID  `gorm:"column:created_by;type:uuid;not null"`
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
}

func (qianchuanStrategyRecord) TableName() string { return "qianchuan_strategies" }

func newQianchuanStrategyRecord(s *strategy.QianchuanStrategy) *qianchuanStrategyRecord {
	if s == nil {
		return nil
	}
	return &qianchuanStrategyRecord{
		ID:          s.ID,
		ProjectID:   cloneUUIDPointer(s.ProjectID),
		Name:        s.Name,
		Description: s.Description,
		YAMLSource:  s.YAMLSource,
		ParsedSpec:  newJSONText(s.ParsedSpec, "{}"),
		Version:     s.Version,
		Status:      s.Status,
		CreatedBy:   s.CreatedBy,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (r *qianchuanStrategyRecord) toModel() *strategy.QianchuanStrategy {
	if r == nil {
		return nil
	}
	return &strategy.QianchuanStrategy{
		ID:          r.ID,
		ProjectID:   cloneUUIDPointer(r.ProjectID),
		Name:        r.Name,
		Description: r.Description,
		YAMLSource:  r.YAMLSource,
		ParsedSpec:  r.ParsedSpec.String("{}"),
		Version:     r.Version,
		Status:      r.Status,
		CreatedBy:   r.CreatedBy,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// Insert creates a new row. CreatedAt/UpdatedAt default to now if zero. ID
// is filled if zero.
func (r *QianchuanStrategyRepository) Insert(ctx context.Context, s *strategy.QianchuanStrategy) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if s == nil {
		return errors.New("nil strategy")
	}
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	rec := newQianchuanStrategyRecord(s)
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("insert qianchuan strategy: %w", err)
	}
	return nil
}

// GetByID returns a single row, system or project-scoped. ErrNotFound is
// returned when no match exists.
func (r *QianchuanStrategyRepository) GetByID(ctx context.Context, id uuid.UUID) (*strategy.QianchuanStrategy, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec qianchuanStrategyRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error; err != nil {
		return nil, fmt.Errorf("get qianchuan strategy: %w", normalizeRepositoryError(err))
	}
	return rec.toModel(), nil
}

// ListByProject returns rows scoped to projectID. When includeSystem is true
// the result also includes system rows (project_id IS NULL).
func (r *QianchuanStrategyRepository) ListByProject(ctx context.Context, projectID uuid.UUID, includeSystem bool) ([]*strategy.QianchuanStrategy, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []qianchuanStrategyRecord
	q := r.db.WithContext(ctx)
	if includeSystem {
		q = q.Where("project_id = ? OR project_id IS NULL", projectID)
	} else {
		q = q.Where("project_id = ?", projectID)
	}
	if err := q.Order("created_at ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list qianchuan strategies: %w", err)
	}
	out := make([]*strategy.QianchuanStrategy, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

// MaxVersion returns the largest version number stored for (projectID, name),
// treating projectID == nil as the system seed scope. Returns 0 when no rows
// exist.
func (r *QianchuanStrategyRepository) MaxVersion(ctx context.Context, projectID *uuid.UUID, name string) (int, error) {
	if r.db == nil {
		return 0, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).Model(&qianchuanStrategyRecord{}).Where("name = ?", name)
	if projectID == nil {
		q = q.Where("project_id IS NULL")
	} else {
		q = q.Where("project_id = ?", *projectID)
	}
	var max *int
	if err := q.Select("MAX(version)").Scan(&max).Error; err != nil {
		return 0, fmt.Errorf("max version: %w", err)
	}
	if max == nil {
		return 0, nil
	}
	return *max, nil
}

// UpdateDraft updates yaml_source / parsed_spec / description on a row that
// is still in draft status. Returns ErrNotFound when no row matches.
func (r *QianchuanStrategyRepository) UpdateDraft(ctx context.Context, id uuid.UUID, description, yamlSource, parsedSpec string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Model(&qianchuanStrategyRecord{}).
		Where("id = ? AND status = ?", id, strategy.StatusDraft).
		Updates(map[string]any{
			"description": description,
			"yaml_source": yamlSource,
			"parsed_spec": newJSONText(parsedSpec, "{}"),
			"updated_at":  time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("update draft: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetStatus moves status forward. The caller is responsible for verifying
// the transition is legal (draft->published or published->archived); the
// repo only enforces the row exists.
func (r *QianchuanStrategyRepository) SetStatus(ctx context.Context, id uuid.UUID, status string) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Model(&qianchuanStrategyRecord{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":     status,
			"updated_at": time.Now().UTC(),
		})
	if res.Error != nil {
		return fmt.Errorf("set status: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDraft hard-deletes a draft row. Returns ErrNotFound when no draft
// row matches.
func (r *QianchuanStrategyRepository) DeleteDraft(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, strategy.StatusDraft).
		Delete(&qianchuanStrategyRecord{})
	if res.Error != nil {
		return fmt.Errorf("delete draft: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindByProjectAndName returns all rows (any version, any status) for the
// given (projectID, name) tuple. Useful for upsert + version-bump flows.
// projectID == nil targets the system seed scope.
func (r *QianchuanStrategyRepository) FindByProjectAndName(ctx context.Context, projectID *uuid.UUID, name string) ([]*strategy.QianchuanStrategy, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []qianchuanStrategyRecord
	q := r.db.WithContext(ctx).Where("name = ?", name)
	if projectID == nil {
		q = q.Where("project_id IS NULL")
	} else {
		q = q.Where("project_id = ?", *projectID)
	}
	if err := q.Order("version ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("find by project and name: %w", err)
	}
	out := make([]*strategy.QianchuanStrategy, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}
