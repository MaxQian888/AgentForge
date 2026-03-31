package role

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type skillInterfaceMetadata struct {
	DisplayName      string `yaml:"display_name"`
	ShortDescription string `yaml:"short_description"`
	DefaultPrompt    string `yaml:"default_prompt"`
}

type skillAgentConfig struct {
	Interface skillInterfaceMetadata `yaml:"interface"`
}

type skillPackageDocument struct {
	Path           string
	RelativeDir    string
	Frontmatter    skillFrontmatter
	Body           string
	Interface      skillInterfaceMetadata
	AvailableParts []string
	ReferenceCount int
	ScriptCount    int
	AssetCount     int
}

func readSkillPackageDocument(root, canonicalPath string) (*skillPackageDocument, error) {
	canonicalPath = normalizeSkillReferencePath(canonicalPath)
	if canonicalPath == "" {
		return nil, os.ErrNotExist
	}

	relative := strings.TrimPrefix(canonicalPath, "skills/")
	skillDir := filepath.Join(root, filepath.FromSlash(relative))
	content, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return nil, err
	}

	document := parseSkillDocument(string(content))
	interfaceMetadata, hasAgentMetadata := readSkillInterfaceMetadata(skillDir)
	referenceCount := countSkillFiles(filepath.Join(skillDir, "references"))
	scriptCount := countSkillFiles(filepath.Join(skillDir, "scripts"))
	assetCount := countSkillFiles(filepath.Join(skillDir, "assets"))

	parts := make([]string, 0, 4)
	if hasAgentMetadata {
		parts = append(parts, "agents")
	}
	if referenceCount > 0 {
		parts = append(parts, "references")
	}
	if scriptCount > 0 {
		parts = append(parts, "scripts")
	}
	if assetCount > 0 {
		parts = append(parts, "assets")
	}

	return &skillPackageDocument{
		Path:           canonicalPath,
		RelativeDir:    relative,
		Frontmatter:    document.Frontmatter,
		Body:           document.Body,
		Interface:      interfaceMetadata,
		AvailableParts: parts,
		ReferenceCount: referenceCount,
		ScriptCount:    scriptCount,
		AssetCount:     assetCount,
	}, nil
}

func readSkillInterfaceMetadata(skillDir string) (skillInterfaceMetadata, bool) {
	configPath, ok := findSkillInterfaceConfig(skillDir)
	if !ok {
		return skillInterfaceMetadata{}, false
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return skillInterfaceMetadata{}, true
	}

	var config skillAgentConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return skillInterfaceMetadata{}, true
	}

	return skillInterfaceMetadata{
		DisplayName:      strings.TrimSpace(config.Interface.DisplayName),
		ShortDescription: strings.TrimSpace(config.Interface.ShortDescription),
		DefaultPrompt:    strings.TrimSpace(config.Interface.DefaultPrompt),
	}, true
}

func findSkillInterfaceConfig(skillDir string) (string, bool) {
	candidates := []string{
		filepath.Join(skillDir, "agents", "openai.yaml"),
		filepath.Join(skillDir, "agents", "openai.yml"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func countSkillFiles(root string) int {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return 0
	}

	count := 0
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return nil
		}
		count++
		return nil
	})
	return count
}

func skillLabel(document *skillPackageDocument) string {
	if document == nil {
		return "Skill"
	}
	if label := strings.TrimSpace(document.Frontmatter.Name); label != "" {
		return label
	}
	if label := strings.TrimSpace(document.Interface.DisplayName); label != "" {
		return label
	}
	return humanizeSkillLabel(document.RelativeDir)
}

func skillDescription(document *skillPackageDocument) string {
	if document == nil {
		return ""
	}
	if description := strings.TrimSpace(document.Frontmatter.Description); description != "" {
		return description
	}
	return strings.TrimSpace(document.Interface.ShortDescription)
}
