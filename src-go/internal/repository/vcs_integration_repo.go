package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/agentforge/server/internal/model"
)

// VCSIntegrationRepo persists model.VCSIntegration rows in
// vcs_integrations. Plaintext PAT / webhook secret never travel through
// this layer; only the secrets.name refs do.
type VCSIntegrationRepo struct{ db *gorm.DB }

// NewVCSIntegrationRepo wires a repo on the supplied DB handle.
func NewVCSIntegrationRepo(db *gorm.DB) *VCSIntegrationRepo {
	return &VCSIntegrationRepo{db: db}
}

type vcsIntegrationRecord struct {
	ID               uuid.UUID  `gorm:"column:id;type:uuid;primaryKey"`
	ProjectID        uuid.UUID  `gorm:"column:project_id;type:uuid;not null"`
	Provider         string     `gorm:"column:provider;not null"`
	Host             string     `gorm:"column:host;not null"`
	Owner            string     `gorm:"column:owner;not null"`
	Repo             string     `gorm:"column:repo;not null"`
	DefaultBranch    string     `gorm:"column:default_branch;not null;default:main"`
	WebhookID        *string    `gorm:"column:webhook_id"`
	WebhookSecretRef string     `gorm:"column:webhook_secret_ref;not null"`
	TokenSecretRef   string     `gorm:"column:token_secret_ref;not null"`
	Status           string     `gorm:"column:status;not null;default:active"`
	ActingEmployeeID *uuid.UUID `gorm:"column:acting_employee_id;type:uuid"`
	LastSyncedAt     *time.Time `gorm:"column:last_synced_at"`
	CreatedAt        time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;not null"`
}

func (vcsIntegrationRecord) TableName() string { return "vcs_integrations" }

func newVCSIntegrationRecord(m *model.VCSIntegration) *vcsIntegrationRecord {
	if m == nil {
		return nil
	}
	return &vcsIntegrationRecord{
		ID:               m.ID,
		ProjectID:        m.ProjectID,
		Provider:         m.Provider,
		Host:             m.Host,
		Owner:            m.Owner,
		Repo:             m.Repo,
		DefaultBranch:    m.DefaultBranch,
		WebhookID:        m.WebhookID,
		WebhookSecretRef: m.WebhookSecretRef,
		TokenSecretRef:   m.TokenSecretRef,
		Status:           m.Status,
		ActingEmployeeID: m.ActingEmployeeID,
		LastSyncedAt:     m.LastSyncedAt,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func (r *vcsIntegrationRecord) toModel() *model.VCSIntegration {
	if r == nil {
		return nil
	}
	return &model.VCSIntegration{
		ID:               r.ID,
		ProjectID:        r.ProjectID,
		Provider:         r.Provider,
		Host:             r.Host,
		Owner:            r.Owner,
		Repo:             r.Repo,
		DefaultBranch:    r.DefaultBranch,
		WebhookID:        r.WebhookID,
		WebhookSecretRef: r.WebhookSecretRef,
		TokenSecretRef:   r.TokenSecretRef,
		Status:           r.Status,
		ActingEmployeeID: r.ActingEmployeeID,
		LastSyncedAt:     r.LastSyncedAt,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

// Create inserts a new integration row. ID/CreatedAt default if zero.
func (r *VCSIntegrationRepo) Create(ctx context.Context, rec *model.VCSIntegration) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if rec == nil {
		return errors.New("nil integration")
	}
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	row := newVCSIntegrationRecord(rec)
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create vcs integration: %w", err)
	}
	return nil
}

// Get returns a single integration by id.
func (r *VCSIntegrationRepo) Get(ctx context.Context, id uuid.UUID) (*model.VCSIntegration, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec vcsIntegrationRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&rec).Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return rec.toModel(), nil
}

// ListByProject returns integrations scoped to projectID, newest first.
func (r *VCSIntegrationRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rows []vcsIntegrationRecord
	if err := r.db.WithContext(ctx).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list vcs integrations: %w", err)
	}
	out := make([]*model.VCSIntegration, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toModel())
	}
	return out, nil
}

// FindByRepo returns every integration row matching the (host, owner, repo)
// triple. Used by the inbound webhook handler in 2B to fan out by project.
func (r *VCSIntegrationRepo) FindByRepo(ctx context.Context, host, owner, repo string) ([]*model.VCSIntegration, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rows []vcsIntegrationRecord
	if err := r.db.WithContext(ctx).
		Where("host = ? AND owner = ? AND repo = ?", host, owner, repo).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("find vcs integrations by repo: %w", err)
	}
	out := make([]*model.VCSIntegration, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].toModel())
	}
	return out, nil
}

// Update writes the mutable fields of rec back to the row. Returns
// ErrNotFound if the row is missing.
func (r *VCSIntegrationRepo) Update(ctx context.Context, rec *model.VCSIntegration) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if rec == nil {
		return errors.New("nil integration")
	}
	rec.UpdatedAt = time.Now().UTC()
	res := r.db.WithContext(ctx).
		Model(&vcsIntegrationRecord{}).
		Where("id = ?", rec.ID).
		Updates(map[string]any{
			"status":             rec.Status,
			"token_secret_ref":   rec.TokenSecretRef,
			"webhook_secret_ref": rec.WebhookSecretRef,
			"webhook_id":         rec.WebhookID,
			"acting_employee_id": rec.ActingEmployeeID,
			"default_branch":     rec.DefaultBranch,
			"last_synced_at":     rec.LastSyncedAt,
			"updated_at":         rec.UpdatedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("update vcs integration: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete hard-deletes the row.
func (r *VCSIntegrationRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	res := r.db.WithContext(ctx).Where("id = ?", id).Delete(&vcsIntegrationRecord{})
	if res.Error != nil {
		return fmt.Errorf("delete vcs integration: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
