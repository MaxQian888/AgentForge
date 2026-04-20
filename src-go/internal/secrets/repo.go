package secrets

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// ErrNotFound is returned when a (project_id, name) lookup misses.
var ErrNotFound = errors.New("secrets: not found")

// ErrNameConflict is returned when Create violates the
// (project_id, name) uniqueness constraint.
var ErrNameConflict = errors.New("secrets: name already exists in project")

// Record is the persisted row. Plaintext is NEVER held here.
type Record struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Name        string
	Ciphertext  []byte
	Nonce       []byte
	KeyVersion  int
	Description string
	LastUsedAt  *time.Time
	CreatedBy   uuid.UUID
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Repository is the persistence contract used by Service.
type Repository interface {
	Create(ctx context.Context, r *Record) error
	Get(ctx context.Context, projectID uuid.UUID, name string) (*Record, error)
	List(ctx context.Context, projectID uuid.UUID) ([]*Record, error)
	Update(ctx context.Context, r *Record) error
	Delete(ctx context.Context, projectID uuid.UUID, name string) error
	TouchLastUsed(ctx context.Context, projectID uuid.UUID, name string, when time.Time) error
}

// ---------------- GORM-backed implementation ----------------

type secretRecord struct {
	ID          uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID   uuid.UUID  `gorm:"column:project_id"`
	Name        string     `gorm:"column:name"`
	Ciphertext  []byte     `gorm:"column:ciphertext"`
	Nonce       []byte     `gorm:"column:nonce"`
	KeyVersion  int        `gorm:"column:key_version"`
	Description string     `gorm:"column:description"`
	LastUsedAt  *time.Time `gorm:"column:last_used_at"`
	CreatedBy   uuid.UUID  `gorm:"column:created_by"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (secretRecord) TableName() string { return "secrets" }

func toRecord(r *secretRecord) *Record {
	return &Record{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name,
		Ciphertext: r.Ciphertext, Nonce: r.Nonce, KeyVersion: r.KeyVersion,
		Description: r.Description, LastUsedAt: r.LastUsedAt,
		CreatedBy: r.CreatedBy, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func fromRecord(r *Record) *secretRecord {
	return &secretRecord{
		ID: r.ID, ProjectID: r.ProjectID, Name: r.Name,
		Ciphertext: r.Ciphertext, Nonce: r.Nonce, KeyVersion: r.KeyVersion,
		Description: r.Description, LastUsedAt: r.LastUsedAt,
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
	if err := r.db.WithContext(ctx).Create(fromRecord(rec)).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrNameConflict
		}
		return err
	}
	return nil
}

func (r *GormRepo) Get(ctx context.Context, projectID uuid.UUID, name string) (*Record, error) {
	var row secretRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND name = ?", projectID, name).
		First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return toRecord(&row), nil
}

func (r *GormRepo) List(ctx context.Context, projectID uuid.UUID) ([]*Record, error) {
	var rows []secretRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("name ASC").
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
		Model(&secretRecord{}).
		Where("project_id = ? AND name = ?", rec.ProjectID, rec.Name).
		Updates(map[string]any{
			"ciphertext":  rec.Ciphertext,
			"nonce":       rec.Nonce,
			"key_version": rec.KeyVersion,
			"description": rec.Description,
			"updated_at":  rec.UpdatedAt,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *GormRepo) Delete(ctx context.Context, projectID uuid.UUID, name string) error {
	return r.db.WithContext(ctx).
		Where("project_id = ? AND name = ?", projectID, name).
		Delete(&secretRecord{}).Error
}

func (r *GormRepo) TouchLastUsed(ctx context.Context, projectID uuid.UUID, name string, when time.Time) error {
	res := r.db.WithContext(ctx).
		Model(&secretRecord{}).
		Where("project_id = ? AND name = ?", projectID, name).
		Update("last_used_at", when)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
