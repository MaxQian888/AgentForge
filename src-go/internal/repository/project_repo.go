package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

type ProjectRepository struct {
	db *gorm.DB
}

type projectTaskCountRow struct {
	ProjectID uuid.UUID `gorm:"column:project_id"`
	TaskCount int       `gorm:"column:task_count"`
}

type projectAgentCountRow struct {
	ProjectID  uuid.UUID `gorm:"column:project_id"`
	AgentCount int       `gorm:"column:agent_count"`
}

func NewProjectRepository(db *gorm.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) Create(ctx context.Context, project *model.Project) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if err := r.db.WithContext(ctx).Create(newProjectRecord(project)).Error; err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

// CreateWithOwner inserts a project and an owner-role member row for the
// creating user inside a single transaction. Either both rows persist or
// neither does — preventing the "project exists with no owner" state that
// would lock the project out of all admin-gated writes.
//
// `ownerMember` carries the human-facing identity for the new member row.
// The caller MUST set OwnerMember.UserID to the creating user. This method
// stamps ProjectID, ProjectRole=owner, Type=human, and IDs/timestamps that
// were left zero.
func (r *ProjectRepository) CreateWithOwner(
	ctx context.Context,
	project *model.Project,
	ownerMember *model.Member,
) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	if project == nil || ownerMember == nil {
		return fmt.Errorf("create project with owner: project and ownerMember are required")
	}
	if ownerMember.UserID == nil {
		return fmt.Errorf("create project with owner: ownerMember.UserID is required")
	}
	ownerMember.ProjectID = project.ID
	ownerMember.Type = model.MemberTypeHuman
	ownerMember.ProjectRole = model.ProjectRoleOwner
	ownerMember.Status = model.NormalizeMemberStatus(ownerMember.Status, true)
	ownerMember.IsActive = model.IsMemberStatusActive(ownerMember.Status)
	if ownerMember.ID == uuid.Nil {
		ownerMember.ID = uuid.New()
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(newProjectRecord(project)).Error; err != nil {
			return fmt.Errorf("create project: %w", err)
		}
		if err := tx.Create(newMemberRecord(ownerMember)).Error; err != nil {
			return fmt.Errorf("create owner member: %w", err)
		}
		return nil
	})
}

func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record projectRecord
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get project by id: %w", normalizeRepositoryError(err))
	}
	summaries, err := r.loadProjectSummaries(ctx, []uuid.UUID{id})
	if err != nil {
		return nil, err
	}
	return applyProjectManagementSummary(record.toModel(), summaries[id]), nil
}

func (r *ProjectRepository) GetBySlug(ctx context.Context, slug string) (*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	var record projectRecord
	if err := r.db.WithContext(ctx).Where("slug = ?", slug).Take(&record).Error; err != nil {
		return nil, fmt.Errorf("get project by slug: %w", normalizeRepositoryError(err))
	}
	summaries, err := r.loadProjectSummaries(ctx, []uuid.UUID{record.ID})
	if err != nil {
		return nil, err
	}
	return applyProjectManagementSummary(record.toModel(), summaries[record.ID]), nil
}

// ProjectListFilter narrows what ProjectRepository.List returns. The zero
// value (no statuses) defaults to "non-archived only" — the canonical API
// contract for `GET /projects`.
type ProjectListFilter struct {
	// Statuses, when non-empty, restricts the result to the given statuses.
	// When empty, the list returns only active + paused (archived hidden).
	Statuses []string
}

// List returns non-archived projects (active + paused). For archive-aware
// queries callers should use ListWithFilter instead.
func (r *ProjectRepository) List(ctx context.Context) ([]*model.Project, error) {
	return r.ListWithFilter(ctx, ProjectListFilter{})
}

// ListWithFilter returns projects matching the supplied filter. The zero
// filter defaults to "non-archived only" — same behavior as List.
func (r *ProjectRepository) ListWithFilter(ctx context.Context, filter ProjectListFilter) ([]*model.Project, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}

	statuses := filter.Statuses
	if len(statuses) == 0 {
		statuses = []string{model.ProjectStatusActive, model.ProjectStatusPaused}
	}

	var records []projectRecord
	if err := r.db.WithContext(ctx).
		Where("status IN ?", statuses).
		Order("created_at DESC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	projectIDs := make([]uuid.UUID, 0, len(records))
	for _, record := range records {
		projectIDs = append(projectIDs, record.ID)
	}
	summaries, err := r.loadProjectSummaries(ctx, projectIDs)
	if err != nil {
		return nil, err
	}

	projects := make([]*model.Project, 0, len(records))
	for i := range records {
		projects = append(projects, applyProjectManagementSummary(records[i].toModel(), summaries[records[i].ID]))
	}
	return projects, nil
}

func (r *ProjectRepository) Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	project, err := r.GetByID(ctx, id)
	if err != nil {
		return err
	}
	settingsJSON, err := model.MergeProjectSettings(project.Settings, req.Settings)
	if err != nil {
		return fmt.Errorf("merge project settings: %w", err)
	}

	updates := map[string]any{
		"settings": newJSONText(settingsJSON, "{}"),
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.RepoURL != nil {
		updates["repo_url"] = *req.RepoURL
	}
	if req.DefaultBranch != nil {
		updates["default_branch"] = *req.DefaultBranch
	}

	if err := r.db.WithContext(ctx).
		Model(&projectRecord{}).
		Where("id = ?", id).
		Updates(updates).
		Error; err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	return nil
}

func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}

	result := r.db.WithContext(ctx).Delete(&projectRecord{}, "id = ?", id)
	if result.Error != nil {
		return fmt.Errorf("delete project: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("delete project: %w", ErrNotFound)
	}
	return nil
}

// SetArchived flips status to archived and stamps archived_at + archived_by_user_id.
// Returns ErrNotFound if no row matched.
func (r *ProjectRepository) SetArchived(ctx context.Context, id, archivedByUserID uuid.UUID, archivedAt time.Time) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"status":              model.ProjectStatusArchived,
		"archived_at":         archivedAt,
		"archived_by_user_id": archivedByUserID,
	}
	result := r.db.WithContext(ctx).
		Model(&projectRecord{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("set project archived: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("set project archived: %w", ErrNotFound)
	}
	return nil
}

// SetUnarchived flips status back to active and clears archival bookkeeping.
// Returns ErrNotFound if no row matched.
func (r *ProjectRepository) SetUnarchived(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"status":              model.ProjectStatusActive,
		"archived_at":         gorm.Expr("NULL"),
		"archived_by_user_id": gorm.Expr("NULL"),
	}
	result := r.db.WithContext(ctx).
		Model(&projectRecord{}).
		Where("id = ?", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("set project unarchived: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("set project unarchived: %w", ErrNotFound)
	}
	return nil
}

func applyProjectManagementSummary(project *model.Project, summary projectManagementSummary) *model.Project {
	if project == nil {
		return nil
	}
	project.Status = model.NormalizeProjectStatus(project.Status)
	project.TaskCount = summary.TaskCount
	project.AgentCount = summary.AgentCount
	return project
}

type projectManagementSummary struct {
	TaskCount  int
	AgentCount int
}

func defaultProjectManagementSummary() projectManagementSummary {
	return projectManagementSummary{}
}

func (r *ProjectRepository) loadProjectSummaries(ctx context.Context, projectIDs []uuid.UUID) (map[uuid.UUID]projectManagementSummary, error) {
	summaries := make(map[uuid.UUID]projectManagementSummary, len(projectIDs))
	if len(projectIDs) == 0 {
		return summaries, nil
	}
	for _, projectID := range projectIDs {
		summaries[projectID] = defaultProjectManagementSummary()
	}

	var taskRows []projectTaskCountRow
	if err := r.db.WithContext(ctx).
		Model(&taskRecord{}).
		Select("project_id, COUNT(*) AS task_count").
		Where("project_id IN ?", projectIDs).
		Group("project_id").
		Scan(&taskRows).Error; err != nil {
		return nil, fmt.Errorf("load project task counts: %w", err)
	}
	for _, row := range taskRows {
		summary := summaries[row.ProjectID]
		summary.TaskCount = row.TaskCount
		summaries[row.ProjectID] = summary
	}

	var agentRows []projectAgentCountRow
	if err := r.db.WithContext(ctx).
		Model(&memberRecord{}).
		Select("project_id, COUNT(*) AS agent_count").
		Where("project_id IN ? AND type = ?", projectIDs, model.MemberTypeAgent).
		Group("project_id").
		Scan(&agentRows).Error; err != nil {
		return nil, fmt.Errorf("load project agent counts: %w", err)
	}
	for _, row := range agentRows {
		summary := summaries[row.ProjectID]
		summary.AgentCount = row.AgentCount
		summaries[row.ProjectID] = summary
	}

	return summaries, nil
}
