package knowledge

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"gorm.io/gorm"
)

// SearchProvider performs full-text searches over knowledge assets.
type SearchProvider interface {
	Search(ctx context.Context, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error)
}

// PgFTSProvider uses Postgres tsvector / tsquery for full-text search.
type PgFTSProvider struct {
	db *gorm.DB
}

func NewPgFTSProvider(db *gorm.DB) *PgFTSProvider {
	return &PgFTSProvider{db: db}
}

type ftsRow struct {
	knowledgeAssetRecord
	Rank    float64 `gorm:"column:rank"`
	Snippet string  `gorm:"column:snippet"`
}

func (PgFTSProvider) tableName() string { return "knowledge_assets" }

func (p *PgFTSProvider) Search(ctx context.Context, projectID uuid.UUID, query string, kind *model.KnowledgeAssetKind, limit int) ([]*model.KnowledgeSearchResult, error) {
	if p.db == nil {
		return nil, ErrDatabaseUnavailable
	}
	if limit <= 0 {
		limit = 20
	}

	q := p.db.WithContext(ctx).
		Table("knowledge_assets").
		Select(`*, ts_rank(search_vector, plainto_tsquery('english', ?)) AS rank,
			ts_headline('english', COALESCE(content_text,''), plainto_tsquery('english', ?),
			  'MaxFragments=1,MaxWords=30,MinWords=10') AS snippet`, query, query).
		Where("project_id = ? AND deleted_at IS NULL", projectID).
		Where("search_vector @@ plainto_tsquery('english', ?)", query).
		Order("rank DESC").
		Limit(limit)

	if kind != nil {
		q = q.Where("kind = ?", string(*kind))
	}

	var rows []ftsRow
	if err := q.Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("knowledge fts search: %w", err)
	}

	results := make([]*model.KnowledgeSearchResult, 0, len(rows))
	for _, row := range rows {
		results = append(results, &model.KnowledgeSearchResult{
			Asset:   row.toModel().ToDTO(),
			Rank:    row.Rank,
			Snippet: row.Snippet,
		})
	}
	return results, nil
}
