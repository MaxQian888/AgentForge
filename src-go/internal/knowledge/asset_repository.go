package knowledge

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

// pgKnowledgeAssetRepository is the Postgres (GORM) implementation.
type pgKnowledgeAssetRepository struct {
	db *gorm.DB
}

// NewPgKnowledgeAssetRepository creates a repository backed by the given *gorm.DB.
func NewPgKnowledgeAssetRepository(db *gorm.DB) KnowledgeAssetRepository {
	return &pgKnowledgeAssetRepository{db: db}
}

func (r *pgKnowledgeAssetRepository) Create(ctx context.Context, a *model.KnowledgeAsset) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	rec := newKnowledgeAssetRecord(a)
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("knowledge_asset create: %w", err)
	}
	return nil
}

func (r *pgKnowledgeAssetRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.KnowledgeAsset, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var rec knowledgeAssetRecord
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		Take(&rec).Error; err != nil {
		return nil, normalizeErr(err)
	}
	return rec.toModel(), nil
}

func (r *pgKnowledgeAssetRepository) Update(ctx context.Context, a *model.KnowledgeAsset) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"title":              a.Title,
		"content_json":       newJSONText(a.ContentJSON, ""),
		"content_text":       a.ContentText,
		"template_category":  a.TemplateCategory,
		"is_system_template": a.IsSystemTemplate,
		"is_pinned":          a.IsPinned,
		"updated_by":         a.UpdatedBy,
		"updated_at":         a.UpdatedAt,
		"version":            a.Version,
	}
	result := r.db.WithContext(ctx).
		Model(&knowledgeAssetRecord{}).
		Where("id = ? AND deleted_at IS NULL", a.ID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("knowledge_asset update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *pgKnowledgeAssetRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&knowledgeAssetRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("knowledge_asset soft delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *pgKnowledgeAssetRepository) Restore(ctx context.Context, id uuid.UUID) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&knowledgeAssetRecord{}).
		Where("id = ? AND deleted_at IS NOT NULL", id).
		Updates(map[string]any{"deleted_at": nil, "updated_at": now})
	if result.Error != nil {
		return fmt.Errorf("knowledge_asset restore: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *pgKnowledgeAssetRepository) ListByProject(ctx context.Context, projectID uuid.UUID, kind *model.KnowledgeAssetKind) ([]*model.KnowledgeAsset, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Order("sort_order ASC, created_at ASC")
	if kind != nil {
		q = q.Where("kind = ?", string(*kind))
	}
	var records []knowledgeAssetRecord
	if err := q.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("knowledge_asset list by project: %w", err)
	}
	out := make([]*model.KnowledgeAsset, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

func (r *pgKnowledgeAssetRepository) ListTree(ctx context.Context, spaceID uuid.UUID) ([]*model.KnowledgeAsset, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	var records []knowledgeAssetRecord
	if err := r.db.WithContext(ctx).
		Where("wiki_space_id = ? AND deleted_at IS NULL", spaceID).
		Where("kind IN ?", []string{string(model.KindWikiPage)}).
		Order("sort_order ASC, created_at ASC").
		Find(&records).Error; err != nil {
		return nil, fmt.Errorf("knowledge_asset list tree: %w", err)
	}
	out := make([]*model.KnowledgeAsset, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

func (r *pgKnowledgeAssetRepository) ListByParent(ctx context.Context, spaceID uuid.UUID, parentID *uuid.UUID) ([]*model.KnowledgeAsset, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	q := r.db.WithContext(ctx).
		Where("wiki_space_id = ? AND deleted_at IS NULL", spaceID).
		Order("sort_order ASC, created_at ASC")
	if parentID == nil {
		q = q.Where("parent_id IS NULL")
	} else {
		q = q.Where("parent_id = ?", parentID)
	}
	var records []knowledgeAssetRecord
	if err := q.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("knowledge_asset list by parent: %w", err)
	}
	out := make([]*model.KnowledgeAsset, 0, len(records))
	for i := range records {
		out = append(out, records[i].toModel())
	}
	return out, nil
}

func (r *pgKnowledgeAssetRepository) Move(ctx context.Context, id uuid.UUID, parentID *uuid.UUID, path string, sortOrder int) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	updates := map[string]any{
		"parent_id":  parentID,
		"path":       path,
		"sort_order": sortOrder,
		"updated_at": time.Now().UTC(),
	}
	result := r.db.WithContext(ctx).
		Model(&knowledgeAssetRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("knowledge_asset move: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *pgKnowledgeAssetRepository) UpdateIngestStatus(ctx context.Context, id uuid.UUID, status model.IngestStatus, chunkCount int) error {
	if r.db == nil {
		return ErrDatabaseUnavailable
	}
	s := string(status)
	result := r.db.WithContext(ctx).
		Model(&knowledgeAssetRecord{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"ingest_status":      s,
			"ingest_chunk_count": chunkCount,
			"updated_at":         time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("knowledge_asset update ingest status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrAssetNotFound
	}
	return nil
}

func (r *pgKnowledgeAssetRepository) Descendants(ctx context.Context, id uuid.UUID) ([]uuid.UUID, error) {
	if r.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	// Iterative BFS to collect all descendant IDs.
	result := []uuid.UUID{}
	queue := []uuid.UUID{id}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		var children []knowledgeAssetRecord
		if err := r.db.WithContext(ctx).
			Select("id").
			Where("parent_id = ? AND deleted_at IS NULL", current).
			Find(&children).Error; err != nil {
			return nil, fmt.Errorf("knowledge_asset descendants: %w", err)
		}
		for _, child := range children {
			result = append(result, child.ID)
			queue = append(queue, child.ID)
		}
	}
	return result, nil
}

// --- package-level errors ---

var ErrDatabaseUnavailable = errors.New("database unavailable")

func normalizeErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrAssetNotFound
	}
	return err
}
