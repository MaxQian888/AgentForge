package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/agentforge/marketplace/internal/repository"
	"github.com/agentforge/marketplace/internal/service"
	"github.com/google/uuid"
)

type memoryItemRepo struct {
	items    map[uuid.UUID]*model.MarketplaceItem
	versions map[uuid.UUID]map[string]*model.MarketplaceItemVersion
}

func newMemoryItemRepo() *memoryItemRepo {
	return &memoryItemRepo{
		items:    map[uuid.UUID]*model.MarketplaceItem{},
		versions: map[uuid.UUID]map[string]*model.MarketplaceItemVersion{},
	}
}

func (r *memoryItemRepo) List(_ context.Context, _ model.ListItemsQuery) ([]*model.MarketplaceItem, int64, error) {
	items := make([]*model.MarketplaceItem, 0, len(r.items))
	for _, item := range r.items {
		if item.IsDeleted {
			continue
		}
		cloned := *item
		items = append(items, &cloned)
	}
	return items, int64(len(items)), nil
}

func (r *memoryItemRepo) ListFeatured(_ context.Context) ([]*model.MarketplaceItem, error) {
	items := make([]*model.MarketplaceItem, 0)
	for _, item := range r.items {
		if item.IsDeleted || !item.IsFeatured {
			continue
		}
		cloned := *item
		items = append(items, &cloned)
	}
	return items, nil
}

func (r *memoryItemRepo) Search(_ context.Context, query string) ([]*model.MarketplaceItem, error) {
	items := make([]*model.MarketplaceItem, 0)
	for _, item := range r.items {
		if item.IsDeleted {
			continue
		}
		if query == "" || item.Name == query {
			cloned := *item
			items = append(items, &cloned)
		}
	}
	return items, nil
}

func (r *memoryItemRepo) GetByID(_ context.Context, id uuid.UUID) (*model.MarketplaceItem, error) {
	item, ok := r.items[id]
	if !ok || item.IsDeleted {
		return nil, repository.ErrNotFound
	}
	cloned := *item
	return &cloned, nil
}

func (r *memoryItemRepo) GetBySlugAndType(_ context.Context, slug, itemType string) (*model.MarketplaceItem, error) {
	for _, item := range r.items {
		if item.IsDeleted {
			continue
		}
		if item.Slug == slug && item.Type == itemType {
			cloned := *item
			return &cloned, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *memoryItemRepo) Create(_ context.Context, item *model.MarketplaceItem) error {
	cloned := *item
	r.items[item.ID] = &cloned
	return nil
}

func (r *memoryItemRepo) Update(_ context.Context, id uuid.UUID, req model.UpdateItemRequest) error {
	item, ok := r.items[id]
	if !ok || item.IsDeleted {
		return repository.ErrNotFound
	}
	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Description != nil {
		item.Description = *req.Description
	}
	if req.Category != nil {
		item.Category = *req.Category
	}
	if req.Tags != nil {
		item.Tags = model.StringArray(req.Tags)
	}
	if req.License != nil {
		item.License = *req.License
	}
	if req.ExtraMetadata != nil {
		item.ExtraMetadata = req.ExtraMetadata
	}
	item.UpdatedAt = time.Now()
	return nil
}

func (r *memoryItemRepo) SoftDelete(_ context.Context, id uuid.UUID) error {
	item, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.IsDeleted = true
	return nil
}

func (r *memoryItemRepo) SetFeatured(_ context.Context, id uuid.UUID, featured bool) error {
	item, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.IsFeatured = featured
	return nil
}

func (r *memoryItemRepo) SetVerified(_ context.Context, id uuid.UUID, verified bool) error {
	item, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.IsVerified = verified
	return nil
}

func (r *memoryItemRepo) CreateVersion(_ context.Context, version *model.MarketplaceItemVersion) error {
	if _, ok := r.items[version.ItemID]; !ok {
		return repository.ErrNotFound
	}
	if _, ok := r.versions[version.ItemID]; !ok {
		r.versions[version.ItemID] = map[string]*model.MarketplaceItemVersion{}
	}
	cloned := *version
	r.versions[version.ItemID][version.Version] = &cloned
	return nil
}

func (r *memoryItemRepo) YankVersion(_ context.Context, itemID uuid.UUID, version string) error {
	versions, ok := r.versions[itemID]
	if !ok {
		return repository.ErrNotFound
	}
	entry, ok := versions[version]
	if !ok {
		return repository.ErrNotFound
	}
	entry.IsYanked = true
	return nil
}

func (r *memoryItemRepo) ListVersions(_ context.Context, itemID uuid.UUID) ([]*model.MarketplaceItemVersion, error) {
	versions := r.versions[itemID]
	result := make([]*model.MarketplaceItemVersion, 0, len(versions))
	for _, version := range versions {
		cloned := *version
		result = append(result, &cloned)
	}
	return result, nil
}

func (r *memoryItemRepo) GetVersion(_ context.Context, itemID uuid.UUID, version string) (*model.MarketplaceItemVersion, error) {
	versions, ok := r.versions[itemID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	entry, ok := versions[version]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cloned := *entry
	return &cloned, nil
}

func (r *memoryItemRepo) SetLatestVersion(_ context.Context, itemID uuid.UUID, version string) error {
	versions, ok := r.versions[itemID]
	if !ok {
		return repository.ErrNotFound
	}
	found := false
	for currentVersion, entry := range versions {
		entry.IsLatest = currentVersion == version
		if currentVersion == version {
			found = true
		}
	}
	if !found {
		return repository.ErrNotFound
	}
	return nil
}

func (r *memoryItemRepo) UpdateLatestVersion(_ context.Context, id uuid.UUID, version string) error {
	item, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.LatestVersion = &version
	return nil
}

func (r *memoryItemRepo) IncrementDownloadCount(_ context.Context, id uuid.UUID) error {
	item, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.DownloadCount++
	return nil
}

func (r *memoryItemRepo) UpdateRatingStats(_ context.Context, id uuid.UUID, avg float64, count int) error {
	item, ok := r.items[id]
	if !ok {
		return repository.ErrNotFound
	}
	item.AvgRating = avg
	item.RatingCount = count
	return nil
}

type memoryReviewRepo struct{}

func (memoryReviewRepo) ListByItem(_ context.Context, _ uuid.UUID, _, _ int) ([]*model.MarketplaceReview, error) {
	return nil, nil
}

func (memoryReviewRepo) UpsertReview(_ context.Context, _ *model.MarketplaceReview) error {
	return nil
}

func (memoryReviewRepo) ComputeRatingStats(_ context.Context, _ uuid.UUID) (float64, int, error) {
	return 0, 0, nil
}

func (memoryReviewRepo) DeleteByItemAndUser(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

func TestMarketplaceService_PublishVersionPersistsValidatedArtifactsAcrossItemTypes(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	for _, tc := range []struct {
		name         string
		itemType     string
		requiredFile string
		files        map[string]string
	}{
		{
			name:         "plugin package",
			itemType:     model.ItemTypePlugin,
			requiredFile: "manifest.yaml",
			files: map[string]string{
				"manifest.yaml": "apiVersion: agentforge/v1\nkind: ToolPlugin\nmetadata:\n  id: repo-search\n  name: Repo Search\n  version: 1.0.0\nspec:\n  runtime: mcp\n",
				"README.md":     "# Repo Search\n",
			},
		},
		{
			name:         "role package",
			itemType:     model.ItemTypeRole,
			requiredFile: "role.yaml",
			files: map[string]string{
				"bundle/role.yaml": "apiVersion: agentforge/v1\nkind: Role\nmetadata:\n  id: release-manager\n  name: Release Manager\nidentity:\n  role: Release Manager\nknowledge:\n  repositories: []\n  documents: []\n  patterns: []\nsecurity:\n  allowed_paths: []\n  denied_paths: []\n  max_budget_usd: 5\n  require_review: true\n",
			},
		},
		{
			name:         "skill package",
			itemType:     model.ItemTypeSkill,
			requiredFile: "SKILL.md",
			files: map[string]string{
				"skill-item/SKILL.md": "---\nname: skill-item\ndescription: Marketplace skill\n---\n\n# Skill\n",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			itemRepo := newMemoryItemRepo()
			svc := service.NewMarketplaceService(itemRepo, memoryReviewRepo{}, t.TempDir())
			item := &model.MarketplaceItem{
				ID:            uuid.New(),
				Type:          tc.itemType,
				Slug:          strings.ReplaceAll(tc.name, " ", "-"),
				Name:          tc.name,
				AuthorID:      authorID,
				AuthorName:    "AgentForge",
				License:       "MIT",
				ExtraMetadata: json.RawMessage("{}"),
			}
			if err := itemRepo.Create(context.Background(), item); err != nil {
				t.Fatalf("seed item: %v", err)
			}
			artifact := buildZipArchive(t, tc.files)

			version, err := svc.PublishVersion(
				context.Background(),
				item.ID,
				authorID,
				model.CreateVersionRequest{Version: "1.0.0"},
				bytes.NewReader(artifact),
				int64(len(artifact)),
			)
			if err != nil {
				t.Fatalf("PublishVersion() error = %v", err)
			}

			if version.Version != "1.0.0" {
				t.Fatalf("version = %q, want 1.0.0", version.Version)
			}
			if version.ArtifactDigest == "" {
				t.Fatal("expected artifact digest to be recorded")
			}
			if version.ArtifactSizeBytes == 0 {
				t.Fatal("expected artifact size to be recorded")
			}
			if _, err := os.Stat(version.ArtifactPath); err != nil {
				t.Fatalf("expected artifact to persist on disk: %v", err)
			}

			storedItem, err := itemRepo.GetByID(context.Background(), item.ID)
			if err != nil {
				t.Fatalf("GetByID() error = %v", err)
			}
			if storedItem.LatestVersion == nil || *storedItem.LatestVersion != "1.0.0" {
				t.Fatalf("LatestVersion = %#v, want 1.0.0", storedItem.LatestVersion)
			}
		})
	}
}

func TestMarketplaceService_PublishVersionRejectsInvalidArtifactShapes(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	for _, tc := range []struct {
		name     string
		itemType string
		files    map[string]string
		want     string
	}{
		{
			name:     "plugin missing manifest",
			itemType: model.ItemTypePlugin,
			files:    map[string]string{"README.md": "# missing manifest\n"},
			want:     "invalid plugin artifact: manifest.yaml is required at the package root",
		},
		{
			name:     "role missing manifest",
			itemType: model.ItemTypeRole,
			files:    map[string]string{"README.md": "# missing role manifest\n"},
			want:     "invalid role artifact: role.yaml is required at the package root",
		},
		{
			name:     "skill missing skill doc",
			itemType: model.ItemTypeSkill,
			files:    map[string]string{"README.md": "# missing skill manifest\n"},
			want:     "invalid skill artifact: SKILL.md is required at the package root",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			itemRepo := newMemoryItemRepo()
			artifactsDir := t.TempDir()
			svc := service.NewMarketplaceService(itemRepo, memoryReviewRepo{}, artifactsDir)
			item := &model.MarketplaceItem{
				ID:            uuid.New(),
				Type:          tc.itemType,
				Slug:          tc.name,
				Name:          tc.name,
				AuthorID:      authorID,
				AuthorName:    "AgentForge",
				License:       "MIT",
				ExtraMetadata: json.RawMessage("{}"),
			}
			if err := itemRepo.Create(context.Background(), item); err != nil {
				t.Fatalf("seed item: %v", err)
			}
			artifact := buildZipArchive(t, tc.files)

			_, err := svc.PublishVersion(
				context.Background(),
				item.ID,
				authorID,
				model.CreateVersionRequest{Version: "1.0.0"},
				bytes.NewReader(artifact),
				0,
			)
			var artifactErr *service.ArtifactValidationError
			if !errors.As(err, &artifactErr) {
				t.Fatalf("expected ArtifactValidationError, got %v", err)
			}
			if artifactErr.Error() != tc.want {
				t.Fatalf("error = %q, want %q", artifactErr.Error(), tc.want)
			}

			artifactPath := filepath.Join(artifactsDir, item.ID.String(), "1.0.0.artifact")
			if _, statErr := os.Stat(artifactPath); !os.IsNotExist(statErr) {
				t.Fatalf("expected invalid artifact to be removed, stat err = %v", statErr)
			}
		})
	}
}

func TestMarketplaceService_ModerationAndVersionLifecycle(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	itemRepo := newMemoryItemRepo()
	svc := service.NewMarketplaceService(itemRepo, memoryReviewRepo{}, t.TempDir())

	item := &model.MarketplaceItem{
		ID:            uuid.New(),
		Type:          model.ItemTypePlugin,
		Slug:          "release-train",
		Name:          "Release Train",
		AuthorID:      authorID,
		AuthorName:    "AgentForge",
		License:       "MIT",
		ExtraMetadata: json.RawMessage("{}"),
	}
	if err := itemRepo.Create(context.Background(), item); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	version, err := svc.PublishVersion(
		context.Background(),
		item.ID,
		authorID,
		model.CreateVersionRequest{Version: "1.0.0"},
		bytes.NewReader(buildZipArchive(t, map[string]string{
			"manifest.yaml": "apiVersion: agentforge/v1\nkind: ToolPlugin\nmetadata:\n  id: release-train\n  name: Release Train\n  version: 1.0.0\nspec:\n  runtime: mcp\n",
		})),
		0,
	)
	if err != nil {
		t.Fatalf("PublishVersion() error = %v", err)
	}

	if err := svc.AdminFeature(context.Background(), item.ID, true); err != nil {
		t.Fatalf("AdminFeature() error = %v", err)
	}
	if err := svc.AdminVerify(context.Background(), item.ID, true); err != nil {
		t.Fatalf("AdminVerify() error = %v", err)
	}
	if err := svc.YankVersion(context.Background(), item.ID, authorID, version.Version); err != nil {
		t.Fatalf("YankVersion() error = %v", err)
	}

	storedItem, err := itemRepo.GetByID(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if !storedItem.IsFeatured {
		t.Fatal("expected item to be featured")
	}
	if !storedItem.IsVerified {
		t.Fatal("expected item to be verified")
	}

	storedVersion, err := itemRepo.GetVersion(context.Background(), item.ID, version.Version)
	if err != nil {
		t.Fatalf("GetVersion() error = %v", err)
	}
	if !storedVersion.IsYanked {
		t.Fatal("expected version to be yanked")
	}
}

func TestMarketplaceService_GetItemIncludesSkillPreviewForSkillItems(t *testing.T) {
	t.Parallel()

	authorID := uuid.New()
	itemRepo := newMemoryItemRepo()
	svc := service.NewMarketplaceService(itemRepo, memoryReviewRepo{}, t.TempDir())
	item := &model.MarketplaceItem{
		ID:            uuid.New(),
		Type:          model.ItemTypeSkill,
		Slug:          "react-skill",
		Name:          "React Skill",
		AuthorID:      authorID,
		AuthorName:    "AgentForge",
		License:       "MIT",
		ExtraMetadata: json.RawMessage("{}"),
	}
	if err := itemRepo.Create(context.Background(), item); err != nil {
		t.Fatalf("seed item: %v", err)
	}

	artifact := buildZipArchive(t, map[string]string{
		"react-skill/SKILL.md": `---
name: React
description: Build React surfaces.
requires:
  - skills/typescript
tools:
  - code_editor
  - browser_preview
---

# React

Build product-facing React surfaces.
`,
		"react-skill/agents/openai.yaml": `interface:
  display_name: "AgentForge React"
  short_description: "Build React safely"
  default_prompt: "Use React skill"
`,
	})

	if _, err := svc.PublishVersion(
		context.Background(),
		item.ID,
		authorID,
		model.CreateVersionRequest{Version: "1.0.0"},
		bytes.NewReader(artifact),
		int64(len(artifact)),
	); err != nil {
		t.Fatalf("PublishVersion() error = %v", err)
	}

	stored, err := svc.GetItem(context.Background(), item.ID)
	if err != nil {
		t.Fatalf("GetItem() error = %v", err)
	}

	raw, err := json.Marshal(stored)
	if err != nil {
		t.Fatalf("marshal stored item: %v", err)
	}

	var payload struct {
		ID           string `json:"id"`
		SkillPreview *struct {
			CanonicalPath   string `json:"canonicalPath"`
			MarkdownBody    string `json:"markdownBody"`
			FrontmatterYAML string `json:"frontmatterYaml"`
			Requires        []string `json:"requires"`
			Tools           []string `json:"tools"`
			AgentConfigs    []struct {
				Path string `json:"path"`
				Yaml string `json:"yaml"`
			} `json:"agentConfigs"`
		} `json:"skillPreview"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode stored skill item: %v", err)
	}

	if payload.SkillPreview == nil {
		t.Fatalf("expected skillPreview in item payload, got %s", string(raw))
	}
	if payload.SkillPreview.CanonicalPath != "skills/react-skill" {
		t.Fatalf("canonicalPath = %q, want skills/react-skill", payload.SkillPreview.CanonicalPath)
	}
	if payload.SkillPreview.MarkdownBody == "" {
		t.Fatal("expected markdown body to be populated")
	}
	if !strings.Contains(payload.SkillPreview.FrontmatterYAML, "name: React") {
		t.Fatalf("frontmatterYaml = %q, want normalized skill frontmatter", payload.SkillPreview.FrontmatterYAML)
	}
	if len(payload.SkillPreview.AgentConfigs) != 1 {
		t.Fatalf("agentConfigs len = %d, want 1", len(payload.SkillPreview.AgentConfigs))
	}
	if payload.SkillPreview.AgentConfigs[0].Path != "agents/openai.yaml" {
		t.Fatalf("agent config path = %q, want agents/openai.yaml", payload.SkillPreview.AgentConfigs[0].Path)
	}
	if !strings.Contains(payload.SkillPreview.AgentConfigs[0].Yaml, "display_name: AgentForge React") {
		t.Fatalf("agent config yaml = %q, want normalized yaml", payload.SkillPreview.AgentConfigs[0].Yaml)
	}
}

func buildZipArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("Create(%s) error = %v", name, err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatalf("WriteString(%s) error = %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	return buf.Bytes()
}
