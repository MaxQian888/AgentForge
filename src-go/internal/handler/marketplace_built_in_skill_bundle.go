package handler

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"gopkg.in/yaml.v3"
)

const builtInSkillBundleFileName = "builtin-bundle.yaml"

type builtInSkillBundleFile struct {
	Skills []builtInSkillBundleEntry `yaml:"skills"`
}

type builtInSkillBundleEntry struct {
	ID            string   `yaml:"id"`
	Root          string   `yaml:"root"`
	Category      string   `yaml:"category"`
	Tags          []string `yaml:"tags"`
	DocsRef       string   `yaml:"docsRef"`
	RepositoryURL string   `yaml:"repositoryUrl"`
	Featured      bool     `yaml:"featured"`
}

type marketplaceBuiltInSkillItem struct {
	ID            string                   `json:"id"`
	Type          string                   `json:"type"`
	Slug          string                   `json:"slug"`
	Name          string                   `json:"name"`
	AuthorID      string                   `json:"author_id"`
	AuthorName    string                   `json:"author_name"`
	Description   string                   `json:"description"`
	Category      string                   `json:"category"`
	Tags          []string                 `json:"tags"`
	RepositoryURL string                   `json:"repository_url,omitempty"`
	License       string                   `json:"license"`
	ExtraMetadata map[string]any           `json:"extra_metadata"`
	DownloadCount int64                    `json:"download_count"`
	AvgRating     float64                  `json:"avg_rating"`
	RatingCount   int                      `json:"rating_count"`
	IsVerified    bool                     `json:"is_verified"`
	IsFeatured    bool                     `json:"is_featured"`
	CreatedAt     string                   `json:"created_at"`
	UpdatedAt     string                   `json:"updated_at"`
	SourceType    string                   `json:"sourceType"`
	LocalPath     string                   `json:"localPath"`
	SkillPreview  *rolepkg.SkillPackagePreview `json:"skillPreview,omitempty"`
	PreviewError  string                   `json:"previewError,omitempty"`
}

func loadBuiltInSkillBundle(skillsRoot string) ([]builtInSkillBundleEntry, error) {
	if strings.TrimSpace(skillsRoot) == "" {
		return nil, nil
	}

	bundlePath := filepath.Join(skillsRoot, builtInSkillBundleFileName)
	data, err := os.ReadFile(bundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read built-in skill bundle: %w", err)
	}

	var file builtInSkillBundleFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse built-in skill bundle: %w", err)
	}

	return file.Skills, nil
}

func (h *MarketplaceHandler) listBuiltInSkills() ([]marketplaceBuiltInSkillItem, error) {
	skillsRoot := h.skillsRootDir()
	if strings.TrimSpace(skillsRoot) == "" {
		return nil, nil
	}

	entries, err := loadBuiltInSkillBundle(skillsRoot)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}

	items := make([]marketplaceBuiltInSkillItem, 0, len(entries))
	for _, entry := range entries {
		item, err := h.resolveBuiltInSkillItem(skillsRoot, entry)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsFeatured != items[j].IsFeatured {
			return items[i].IsFeatured
		}
		return items[i].Slug < items[j].Slug
	})

	return items, nil
}

func (h *MarketplaceHandler) resolveBuiltInSkillItem(skillsRoot string, entry builtInSkillBundleEntry) (*marketplaceBuiltInSkillItem, error) {
	root := normalizeBuiltInSkillRoot(entry.Root)
	if root == "" {
		return nil, fmt.Errorf("invalid built-in skill bundle entry %q: missing root", entry.ID)
	}

	slug := filepath.Base(root)
	if strings.TrimSpace(entry.ID) == "" {
		entry.ID = slug
	}

	canonicalPath := "skills/" + filepath.ToSlash(root)
	preview, err := rolepkg.ReadSkillPackagePreview(skillsRoot, canonicalPath)
	if err != nil {
		return nil, fmt.Errorf("load built-in skill %q preview: %w", entry.ID, err)
	}

	localPath := filepath.Join(skillsRoot, filepath.FromSlash(root))
	now := time.Now().UTC().Format(time.RFC3339)
	return &marketplaceBuiltInSkillItem{
		ID:            entry.ID,
		Type:          string(model.MarketplaceItemTypeSkill),
		Slug:          slug,
		Name:          firstNonEmptyBuiltInSkill(preview.Label, preview.DisplayName, slug),
		AuthorID:      "agentforge",
		AuthorName:    "AgentForge",
		Description:   firstNonEmptyBuiltInSkill(preview.Description, preview.MarkdownBody),
		Category:      strings.TrimSpace(entry.Category),
		Tags:          append([]string(nil), entry.Tags...),
		RepositoryURL: strings.TrimSpace(entry.RepositoryURL),
		License:       "MIT",
		ExtraMetadata: map[string]any{
			"docsRef":       strings.TrimSpace(entry.DocsRef),
			"canonicalPath": preview.CanonicalPath,
			"sourceType":    string(model.PluginSourceBuiltin),
		},
		DownloadCount: 0,
		AvgRating:     0,
		RatingCount:   0,
		IsVerified:    true,
		IsFeatured:    entry.Featured,
		CreatedAt:     now,
		UpdatedAt:     now,
		SourceType:    string(model.PluginSourceBuiltin),
		LocalPath:     localPath,
		SkillPreview:  preview,
	}, nil
}

func normalizeBuiltInSkillRoot(value string) string {
	value = strings.ReplaceAll(strings.TrimSpace(value), "\\", "/")
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimPrefix(value, "skills/")
	value = strings.TrimPrefix(value, "/")
	cleaned := filepath.ToSlash(filepath.Clean(value))
	if cleaned == "." || cleaned == "" {
		return ""
	}
	return cleaned
}

func firstNonEmptyBuiltInSkill(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
