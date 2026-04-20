package knowledge

import (
	"context"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// MaterializeAsWiki converts an ingested_file asset into a wiki_page asset
// by copying its text content as ContentJSON (a simple paragraph block).
// The original ingested_file is deleted after materialization.
func (s *KnowledgeAssetService) MaterializeAsWiki(
	ctx context.Context,
	pc model.PrincipalContext,
	assetID uuid.UUID,
	req model.MaterializeAsWikiRequest,
) (*model.KnowledgeAsset, error) {
	if !pc.CanWrite() {
		return nil, ErrAssetForbidden
	}

	src, err := s.assets.GetByID(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if src.Kind != model.KindIngestedFile {
		return nil, fmt.Errorf("%w: materialize only works on ingested_file assets", ErrUnsupportedKind)
	}
	if src.IngestStatus == nil || *src.IngestStatus != model.IngestStatusReady {
		return nil, ErrIngestNotReady
	}

	spaceID, err := uuid.Parse(req.WikiSpaceID)
	if err != nil {
		return nil, fmt.Errorf("invalid wikiSpaceId: %w", err)
	}

	// Collect text from chunks.
	chunks, err := s.chunks.ListByAssetID(ctx, assetID)
	if err != nil {
		return nil, fmt.Errorf("materialize: load chunks: %w", err)
	}
	contentJSON := buildWikiBlocksFromChunks(chunks)
	title := req.Title
	if title == "" {
		title = src.Title
	}

	parentID, err := parseOptionalUUIDStr(req.ParentID)
	if err != nil {
		return nil, fmt.Errorf("invalid parentId: %w", err)
	}

	now := time.Now().UTC()
	newAsset := &model.KnowledgeAsset{
		ID:          uuid.New(),
		ProjectID:   src.ProjectID,
		WikiSpaceID: &spaceID,
		ParentID:    parentID,
		Kind:        model.KindWikiPage,
		Title:       title,
		ContentJSON: contentJSON,
		ContentText: extractText(chunks),
		CreatedBy:   &pc.UserID,
		UpdatedBy:   &pc.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
	}

	if err := ValidateKnowledgeAsset(newAsset); err != nil {
		return nil, err
	}
	if err := s.assets.Create(ctx, newAsset); err != nil {
		return nil, fmt.Errorf("materialize: create wiki page: %w", err)
	}

	// Soft-delete the source file.
	_ = s.assets.SoftDelete(ctx, assetID)

	s.publishEvent(ctx, "knowledge.asset.created", newAsset.ProjectID.String(), map[string]any{
		"assetId":          newAsset.ID.String(),
		"kind":             string(model.KindWikiPage),
		"materializedFrom": assetID.String(),
	})

	return newAsset, nil
}

func buildWikiBlocksFromChunks(chunks []*model.AssetIngestChunk) string {
	if len(chunks) == 0 {
		return `[{"type":"paragraph","content":[{"type":"text","text":""}]}]`
	}
	var result []byte
	result = append(result, '[')
	for i, c := range chunks {
		if i > 0 {
			result = append(result, ',')
		}
		escaped := escapeJSONString(c.Content)
		block := fmt.Sprintf(`{"type":"paragraph","content":[{"type":"text","text":%s}]}`, escaped)
		result = append(result, block...)
	}
	result = append(result, ']')
	return string(result)
}

func extractText(chunks []*model.AssetIngestChunk) string {
	var out []byte
	for i, c := range chunks {
		if i > 0 {
			out = append(out, ' ')
		}
		out = append(out, c.Content...)
	}
	return string(out)
}

func escapeJSONString(s string) string {
	// Use fmt.Sprintf with %q to get a Go-quoted string, then convert to JSON double-quotes.
	return fmt.Sprintf("%q", s)
}
