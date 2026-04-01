package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/agentforge/marketplace/internal/repository"
	"github.com/google/uuid"
)

// Sentinel errors
var (
	ErrNotItemOwner    = errors.New("not item owner")
	ErrSlugTaken       = errors.New("slug already taken")
	ErrInvalidSemver   = errors.New("invalid semver")
	ErrVersionYanked   = errors.New("version is yanked")
	ErrVersionNotFound = errors.New("version not found")
)

var semverRe = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+`)

// MarketplaceService orchestrates marketplace business logic.
type MarketplaceService struct {
	itemRepo     marketplaceItemRepository
	reviewRepo   marketplaceReviewRepository
	artifactsDir string
}

type marketplaceItemRepository interface {
	List(ctx context.Context, q model.ListItemsQuery) ([]*model.MarketplaceItem, int64, error)
	ListFeatured(ctx context.Context) ([]*model.MarketplaceItem, error)
	Search(ctx context.Context, query string) ([]*model.MarketplaceItem, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.MarketplaceItem, error)
	GetBySlugAndType(ctx context.Context, slug, itemType string) (*model.MarketplaceItem, error)
	Create(ctx context.Context, item *model.MarketplaceItem) error
	Update(ctx context.Context, id uuid.UUID, req model.UpdateItemRequest) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	SetFeatured(ctx context.Context, id uuid.UUID, featured bool) error
	SetVerified(ctx context.Context, id uuid.UUID, verified bool) error
	CreateVersion(ctx context.Context, version *model.MarketplaceItemVersion) error
	YankVersion(ctx context.Context, itemID uuid.UUID, version string) error
	ListVersions(ctx context.Context, itemID uuid.UUID) ([]*model.MarketplaceItemVersion, error)
	GetVersion(ctx context.Context, itemID uuid.UUID, version string) (*model.MarketplaceItemVersion, error)
	SetLatestVersion(ctx context.Context, itemID uuid.UUID, version string) error
	UpdateLatestVersion(ctx context.Context, id uuid.UUID, version string) error
	IncrementDownloadCount(ctx context.Context, id uuid.UUID) error
	UpdateRatingStats(ctx context.Context, id uuid.UUID, avg float64, count int) error
}

type marketplaceReviewRepository interface {
	ListByItem(ctx context.Context, itemID uuid.UUID, limit, offset int) ([]*model.MarketplaceReview, error)
	UpsertReview(ctx context.Context, review *model.MarketplaceReview) error
	ComputeRatingStats(ctx context.Context, itemID uuid.UUID) (float64, int, error)
	DeleteByItemAndUser(ctx context.Context, itemID, userID uuid.UUID) error
}

func NewMarketplaceService(
	itemRepo marketplaceItemRepository,
	reviewRepo marketplaceReviewRepository,
	artifactsDir string,
) *MarketplaceService {
	return &MarketplaceService{itemRepo, reviewRepo, artifactsDir}
}

// --------------------------------------------------------------------------
// Item methods
// --------------------------------------------------------------------------

func (s *MarketplaceService) ListItems(ctx context.Context, q model.ListItemsQuery) (*model.ItemListResponse, error) {
	items, total, err := s.itemRepo.List(ctx, q)
	if err != nil {
		return nil, err
	}
	page := q.Page
	if page < 1 {
		page = 1
	}
	pageSize := q.PageSize
	if pageSize < 1 {
		pageSize = model.DefaultPageSize
	}
	if pageSize > model.MaxPageSize {
		pageSize = model.MaxPageSize
	}

	// Convert []*model.MarketplaceItem to []model.MarketplaceItem
	flat := make([]model.MarketplaceItem, 0, len(items))
	for _, it := range items {
		flat = append(flat, *it)
	}
	return &model.ItemListResponse{
		Items:    flat,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *MarketplaceService) GetFeatured(ctx context.Context) ([]*model.MarketplaceItem, error) {
	return s.itemRepo.ListFeatured(ctx)
}

func (s *MarketplaceService) Search(ctx context.Context, query string) ([]*model.MarketplaceItem, error) {
	return s.itemRepo.Search(ctx, query)
}

func (s *MarketplaceService) GetItem(ctx context.Context, id uuid.UUID) (*model.MarketplaceItem, error) {
	item, err := s.itemRepo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	item.SourceType = "marketplace"
	if item.Type == model.ItemTypeSkill && item.LatestVersion != nil && strings.TrimSpace(*item.LatestVersion) != "" {
		preview, previewErr := s.loadSkillPreviewForVersion(ctx, item.ID, *item.LatestVersion)
		if previewErr != nil {
			item.PreviewError = previewErr.Error()
		} else {
			item.SkillPreview = preview
		}
	}
	return item, nil
}

func (s *MarketplaceService) PublishItem(
	ctx context.Context,
	authorID uuid.UUID,
	authorName string,
	req model.CreateItemRequest,
) (*model.MarketplaceItem, error) {
	// Check slug uniqueness within same type.
	_, err := s.itemRepo.GetBySlugAndType(ctx, req.Slug, req.Type)
	if err == nil {
		// Found an existing item — slug is taken.
		return nil, ErrSlugTaken
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	license := req.License
	if license == "" {
		license = "MIT"
	}
	extraMeta := req.ExtraMetadata
	if extraMeta == nil {
		extraMeta = []byte("{}")
	}

	item := &model.MarketplaceItem{
		ID:            uuid.New(),
		Type:          req.Type,
		Slug:          req.Slug,
		Name:          req.Name,
		AuthorID:      authorID,
		AuthorName:    authorName,
		Description:   req.Description,
		Category:      req.Category,
		Tags:          model.StringArray(req.Tags),
		IconURL:       req.IconURL,
		RepositoryURL: req.RepositoryURL,
		License:       license,
		ExtraMetadata: extraMeta,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := s.itemRepo.Create(ctx, item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *MarketplaceService) UpdateItem(
	ctx context.Context,
	itemID, requesterID uuid.UUID,
	req model.UpdateItemRequest,
) (*model.MarketplaceItem, error) {
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.AuthorID != requesterID {
		return nil, ErrNotItemOwner
	}
	if err := s.itemRepo.Update(ctx, itemID, req); err != nil {
		return nil, err
	}
	return s.itemRepo.GetByID(ctx, itemID)
}

func (s *MarketplaceService) DeleteItem(ctx context.Context, itemID, requesterID uuid.UUID) error {
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return err
	}
	if item.AuthorID != requesterID {
		return ErrNotItemOwner
	}
	return s.itemRepo.SoftDelete(ctx, itemID)
}

func (s *MarketplaceService) AdminFeature(ctx context.Context, itemID uuid.UUID, featured bool) error {
	return s.itemRepo.SetFeatured(ctx, itemID, featured)
}

func (s *MarketplaceService) AdminVerify(ctx context.Context, itemID uuid.UUID, verified bool) error {
	return s.itemRepo.SetVerified(ctx, itemID, verified)
}

// --------------------------------------------------------------------------
// Version methods
// --------------------------------------------------------------------------

func (s *MarketplaceService) PublishVersion(
	ctx context.Context,
	itemID, uploaderID uuid.UUID,
	req model.CreateVersionRequest,
	artifactReader io.Reader,
	size int64,
) (*model.MarketplaceItemVersion, error) {
	if !semverRe.MatchString(req.Version) {
		return nil, ErrInvalidSemver
	}

	// Verify uploader owns the item.
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if item.AuthorID != uploaderID {
		return nil, ErrNotItemOwner
	}

	destPath := filepath.Join(s.artifactsDir, itemID.String(), req.Version+".artifact")
	digest, written, err := streamToFile(destPath, artifactReader)
	if err != nil {
		return nil, fmt.Errorf("stream artifact: %w", err)
	}
	if err := validateMarketplaceArtifact(item.Type, destPath); err != nil {
		_ = os.Remove(destPath)
		return nil, err
	}

	v := &model.MarketplaceItemVersion{
		ID:                uuid.New(),
		ItemID:            itemID,
		Version:           req.Version,
		Changelog:         req.Changelog,
		ArtifactPath:      destPath,
		ArtifactSizeBytes: written,
		ArtifactDigest:    digest,
		IsLatest:          false,
		IsYanked:          false,
		CreatedAt:         time.Now(),
	}
	if err := s.itemRepo.CreateVersion(ctx, v); err != nil {
		return nil, err
	}
	if err := s.itemRepo.SetLatestVersion(ctx, itemID, req.Version); err != nil {
		return nil, err
	}
	// Best-effort: persist the latest_version string on the item row.
	_ = s.itemRepo.UpdateLatestVersion(ctx, itemID, req.Version)

	v.IsLatest = true
	return v, nil
}

func (s *MarketplaceService) YankVersion(ctx context.Context, itemID, requesterID uuid.UUID, version string) error {
	item, err := s.itemRepo.GetByID(ctx, itemID)
	if err != nil {
		return err
	}
	if item.AuthorID != requesterID {
		return ErrNotItemOwner
	}
	return s.itemRepo.YankVersion(ctx, itemID, version)
}

func (s *MarketplaceService) GetVersions(ctx context.Context, itemID uuid.UUID) ([]*model.MarketplaceItemVersion, error) {
	return s.itemRepo.ListVersions(ctx, itemID)
}

func (s *MarketplaceService) GetVersionDownloadPath(ctx context.Context, itemID uuid.UUID, version string) (path string, digest string, err error) {
	v, err := s.itemRepo.GetVersion(ctx, itemID, version)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", "", ErrVersionNotFound
		}
		return "", "", err
	}
	if v.IsYanked {
		return "", "", ErrVersionYanked
	}
	return v.ArtifactPath, v.ArtifactDigest, nil
}

func (s *MarketplaceService) IncrementDownload(ctx context.Context, itemID uuid.UUID) error {
	return s.itemRepo.IncrementDownloadCount(ctx, itemID)
}

// --------------------------------------------------------------------------
// Review methods
// --------------------------------------------------------------------------

func (s *MarketplaceService) GetReviews(ctx context.Context, itemID uuid.UUID, limit, offset int) ([]*model.MarketplaceReview, error) {
	return s.reviewRepo.ListByItem(ctx, itemID, limit, offset)
}

func (s *MarketplaceService) UpsertReview(
	ctx context.Context,
	itemID, userID uuid.UUID,
	userName string,
	req model.CreateReviewRequest,
) (*model.MarketplaceReview, error) {
	rev := &model.MarketplaceReview{
		ID:       uuid.New(),
		ItemID:   itemID,
		UserID:   userID,
		UserName: userName,
		Rating:   req.Rating,
		Comment:  req.Comment,
	}
	if err := s.reviewRepo.UpsertReview(ctx, rev); err != nil {
		return nil, err
	}
	// Recompute and persist rating stats.
	avg, count, err := s.reviewRepo.ComputeRatingStats(ctx, itemID)
	if err == nil {
		_ = s.itemRepo.UpdateRatingStats(ctx, itemID, avg, count)
	}
	return rev, nil
}

func (s *MarketplaceService) DeleteReview(ctx context.Context, itemID, userID uuid.UUID) error {
	if err := s.reviewRepo.DeleteByItemAndUser(ctx, itemID, userID); err != nil {
		return err
	}
	// Recompute and persist rating stats.
	avg, count, err := s.reviewRepo.ComputeRatingStats(ctx, itemID)
	if err == nil {
		_ = s.itemRepo.UpdateRatingStats(ctx, itemID, avg, count)
	}
	return nil
}

// --------------------------------------------------------------------------
// Private helpers
// --------------------------------------------------------------------------

// streamToFile writes src to dst atomically via a temp file, returning the
// sha256 hex digest and total bytes written.
func streamToFile(dst string, src io.Reader) (digest string, size int64, err error) {
	if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", 0, fmt.Errorf("mkdir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".upload-*")
	if err != nil {
		return "", 0, fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		// Clean up temp file on error.
		if err != nil {
			tmp.Close()
			os.Remove(tmpPath)
		}
	}()

	hasher := sha256.New()
	mw := io.MultiWriter(tmp, hasher)
	size, err = io.Copy(mw, src)
	if err != nil {
		return "", 0, fmt.Errorf("copy: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return "", 0, fmt.Errorf("close temp: %w", err)
	}
	if err = os.Rename(tmpPath, dst); err != nil {
		return "", 0, fmt.Errorf("rename: %w", err)
	}
	digest = hex.EncodeToString(hasher.Sum(nil))
	return digest, size, nil
}
