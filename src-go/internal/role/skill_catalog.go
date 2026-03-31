package role

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type SkillCatalogEntry struct {
	Path             string   `json:"path"`
	Label            string   `json:"label"`
	Description      string   `json:"description,omitempty"`
	DisplayName      string   `json:"displayName,omitempty"`
	ShortDescription string   `json:"shortDescription,omitempty"`
	DefaultPrompt    string   `json:"defaultPrompt,omitempty"`
	AvailableParts   []string `json:"availableParts,omitempty"`
	ReferenceCount   int      `json:"referenceCount,omitempty"`
	ScriptCount      int      `json:"scriptCount,omitempty"`
	AssetCount       int      `json:"assetCount,omitempty"`
	Source           string   `json:"source"`
	SourceRoot       string   `json:"sourceRoot"`
}

func DiscoverSkillCatalog(root string) ([]SkillCatalogEntry, error) {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []SkillCatalogEntry{}, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return []SkillCatalogEntry{}, nil
	}

	entries := make([]SkillCatalogEntry, 0)
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.EqualFold(d.Name(), "SKILL.md") {
			return nil
		}

		entry, parseErr := buildSkillCatalogEntry(root, path)
		if parseErr != nil {
			return parseErr
		}
		entries = append(entries, entry)
		return nil
	})
	if err != nil {
		return nil, err
	}

	slices.SortFunc(entries, func(a, b SkillCatalogEntry) int {
		return strings.Compare(a.Path, b.Path)
	})
	return entries, nil
}

func buildSkillCatalogEntry(root, skillFile string) (SkillCatalogEntry, error) {
	relDir, err := filepath.Rel(root, filepath.Dir(skillFile))
	if err != nil {
		return SkillCatalogEntry{}, err
	}
	normalizedRelDir := filepath.ToSlash(relDir)
	path := "skills"
	if normalizedRelDir != "." {
		path += "/" + normalizedRelDir
	}

	document, err := readSkillPackageDocument(root, path)
	if err != nil {
		return SkillCatalogEntry{}, err
	}

	return SkillCatalogEntry{
		Path:             path,
		Label:            skillLabel(document),
		Description:      skillDescription(document),
		DisplayName:      document.Interface.DisplayName,
		ShortDescription: document.Interface.ShortDescription,
		DefaultPrompt:    document.Interface.DefaultPrompt,
		AvailableParts:   append([]string(nil), document.AvailableParts...),
		ReferenceCount:   document.ReferenceCount,
		ScriptCount:      document.ScriptCount,
		AssetCount:       document.AssetCount,
		Source:           "repo-local",
		SourceRoot:       "skills",
	}, nil
}

func humanizeSkillLabel(relDir string) string {
	if relDir == "" || relDir == "." {
		return "Skill"
	}
	parts := strings.Split(relDir, "/")
	segment := parts[len(parts)-1]
	words := strings.Fields(strings.NewReplacer("-", " ", "_", " ").Replace(segment))
	for index, word := range words {
		lower := strings.ToLower(word)
		words[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(words, " ")
}
