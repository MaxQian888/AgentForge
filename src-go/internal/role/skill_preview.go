package role

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type SkillAgentConfigPreview struct {
	Path             string `json:"path"`
	Yaml             string `json:"yaml"`
	DisplayName      string `json:"displayName,omitempty"`
	ShortDescription string `json:"shortDescription,omitempty"`
	DefaultPrompt    string `json:"defaultPrompt,omitempty"`
}

type SkillPackagePreview struct {
	CanonicalPath   string                   `json:"canonicalPath"`
	Label           string                   `json:"label"`
	DisplayName     string                   `json:"displayName,omitempty"`
	Description     string                   `json:"description,omitempty"`
	DefaultPrompt   string                   `json:"defaultPrompt,omitempty"`
	MarkdownBody    string                   `json:"markdownBody"`
	FrontmatterYAML string                   `json:"frontmatterYaml"`
	Requires        []string                 `json:"requires,omitempty"`
	Tools           []string                 `json:"tools,omitempty"`
	AvailableParts  []string                 `json:"availableParts,omitempty"`
	ReferenceCount  int                      `json:"referenceCount,omitempty"`
	ScriptCount     int                      `json:"scriptCount,omitempty"`
	AssetCount      int                      `json:"assetCount,omitempty"`
	AgentConfigs    []SkillAgentConfigPreview `json:"agentConfigs,omitempty"`
}

func ReadSkillPackagePreview(root, canonicalPath string) (*SkillPackagePreview, error) {
	document, err := readSkillPackageDocument(root, canonicalPath)
	if err != nil {
		return nil, err
	}

	skillDir := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(document.Path, "skills/")))
	agentConfigs, err := readSkillAgentConfigPreviews(skillDir)
	if err != nil {
		return nil, err
	}

	return &SkillPackagePreview{
		CanonicalPath:   document.Path,
		Label:           skillLabel(document),
		DisplayName:     document.Interface.DisplayName,
		Description:     skillDescription(document),
		DefaultPrompt:   document.Interface.DefaultPrompt,
		MarkdownBody:    strings.TrimSpace(document.Body),
		FrontmatterYAML: renderSkillFrontmatterYAML(document),
		Requires:        append([]string(nil), document.Requires...),
		Tools:           append([]string(nil), document.Tools...),
		AvailableParts:  append([]string(nil), document.AvailableParts...),
		ReferenceCount:  document.ReferenceCount,
		ScriptCount:     document.ScriptCount,
		AssetCount:      document.AssetCount,
		AgentConfigs:    agentConfigs,
	}, nil
}

func renderSkillFrontmatterYAML(document *skillPackageDocument) string {
	if document == nil {
		return ""
	}

	data := make(map[string]any)
	if value := strings.TrimSpace(document.Frontmatter.Name); value != "" {
		data["name"] = value
	}
	if value := strings.TrimSpace(document.Frontmatter.Description); value != "" {
		data["description"] = value
	}
	if len(document.Requires) > 0 {
		data["requires"] = append([]string(nil), document.Requires...)
	}
	if len(document.Tools) > 0 {
		data["tools"] = append([]string(nil), document.Tools...)
	}
	if len(data) == 0 {
		return ""
	}

	return normalizeYAMLDocument(data)
}

func readSkillAgentConfigPreviews(skillDir string) ([]SkillAgentConfigPreview, error) {
	agentsDir := filepath.Join(skillDir, "agents")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	previews := make([]SkillAgentConfigPreview, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		absPath := filepath.Join(agentsDir, name)
		raw, readErr := os.ReadFile(absPath)
		if readErr != nil {
			return nil, readErr
		}

		var parsed map[string]any
		if err := yaml.Unmarshal(raw, &parsed); err != nil {
			return nil, err
		}

		var config skillAgentConfig
		if err := yaml.Unmarshal(raw, &config); err != nil {
			return nil, err
		}

		previews = append(previews, SkillAgentConfigPreview{
			Path:             filepath.ToSlash(filepath.Join("agents", name)),
			Yaml:             normalizeYAMLDocument(parsed),
			DisplayName:      strings.TrimSpace(config.Interface.DisplayName),
			ShortDescription: strings.TrimSpace(config.Interface.ShortDescription),
			DefaultPrompt:    strings.TrimSpace(config.Interface.DefaultPrompt),
		})
	}

	sort.Slice(previews, func(i, j int) bool {
		return previews[i].Path < previews[j].Path
	})

	return previews, nil
}

func normalizeYAMLDocument(value any) string {
	if value == nil {
		return ""
	}

	data, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
