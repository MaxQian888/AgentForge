package service

import (
	"archive/zip"
	"bufio"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type skillFrontmatter struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Requires    []string `yaml:"requires"`
	Tools       []string `yaml:"tools"`
}

type skillDocument struct {
	Frontmatter skillFrontmatter
	Body        string
}

type skillAgentConfig struct {
	Interface struct {
		DisplayName      string `yaml:"display_name"`
		ShortDescription string `yaml:"short_description"`
		DefaultPrompt    string `yaml:"default_prompt"`
	} `yaml:"interface"`
}

func (s *MarketplaceService) loadSkillPreviewForVersion(ctx context.Context, itemID uuid.UUID, version string) (*model.SkillPackagePreview, error) {
	storedVersion, err := s.itemRepo.GetVersion(ctx, itemID, version)
	if err != nil {
		return nil, err
	}
	return buildSkillPackagePreviewFromArtifact(storedVersion.ArtifactPath)
}

func buildSkillPackagePreviewFromArtifact(artifactPath string) (*model.SkillPackagePreview, error) {
	reader, err := zip.OpenReader(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("invalid skill artifact: expected zip package: %w", err)
	}
	defer reader.Close()

	fileMap := make(map[string]*zip.File, len(reader.File))
	names := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		safeName, safeErr := sanitizeArtifactZipPath(file.Name)
		if safeErr != nil {
			return nil, safeErr
		}
		if safeName == "" || file.FileInfo().IsDir() {
			continue
		}
		fileMap[safeName] = file
		names = append(names, safeName)
	}

	root := detectArtifactZipRoot(names)
	skillPath := rootPrefixedPath(root, "SKILL.md")
	skillFile, ok := fileMap[skillPath]
	if !ok {
		return nil, fmt.Errorf("invalid skill artifact: SKILL.md is required at the package root")
	}

	skillSource, err := readZipFile(skillFile)
	if err != nil {
		return nil, err
	}
	document := parseMarketplaceSkillDocument(skillSource)

	requires := normalizeMarketplaceSkillList(document.Frontmatter.Requires)
	tools := normalizeMarketplaceSkillList(document.Frontmatter.Tools)
	agentConfigs, err := loadSkillAgentConfigPreviews(root, fileMap)
	if err != nil {
		return nil, err
	}

	preview := &model.SkillPackagePreview{
		CanonicalPath:   canonicalSkillPathFromRoot(root),
		Label:           marketplaceSkillLabel(root, document),
		DisplayName:     firstNonEmptyMarketplaceSkill(document.Frontmatter.Name, firstAgentDisplayName(agentConfigs)),
		Description:     firstNonEmptyMarketplaceSkill(document.Frontmatter.Description, firstAgentDescription(agentConfigs)),
		DefaultPrompt:   firstAgentPrompt(agentConfigs),
		MarkdownBody:    strings.TrimSpace(document.Body),
		FrontmatterYAML: normalizeMarketplaceSkillFrontmatter(document, requires, tools),
		Requires:        requires,
		Tools:           tools,
		ReferenceCount:  countArtifactFiles(root, fileMap, "references"),
		ScriptCount:     countArtifactFiles(root, fileMap, "scripts"),
		AssetCount:      countArtifactFiles(root, fileMap, "assets"),
		AgentConfigs:    agentConfigs,
	}
	if len(agentConfigs) > 0 {
		preview.AvailableParts = append(preview.AvailableParts, "agents")
	}
	if preview.ReferenceCount > 0 {
		preview.AvailableParts = append(preview.AvailableParts, "references")
	}
	if preview.ScriptCount > 0 {
		preview.AvailableParts = append(preview.AvailableParts, "scripts")
	}
	if preview.AssetCount > 0 {
		preview.AvailableParts = append(preview.AvailableParts, "assets")
	}

	return preview, nil
}

func parseMarketplaceSkillDocument(content string) skillDocument {
	scanner := bufio.NewScanner(strings.NewReader(content))
	if !scanner.Scan() {
		return skillDocument{}
	}
	if strings.TrimSpace(scanner.Text()) != "---" {
		return skillDocument{Body: strings.TrimSpace(content)}
	}

	var frontmatterLines []string
	var bodyLines []string
	frontmatterClosed := false
	for scanner.Scan() {
		line := scanner.Text()
		if !frontmatterClosed {
			if strings.TrimSpace(line) == "---" {
				frontmatterClosed = true
				continue
			}
			frontmatterLines = append(frontmatterLines, line)
			continue
		}
		bodyLines = append(bodyLines, line)
	}

	document := skillDocument{
		Body: strings.TrimSpace(strings.Join(bodyLines, "\n")),
	}
	if len(frontmatterLines) == 0 {
		return document
	}
	if err := yaml.Unmarshal([]byte(strings.Join(frontmatterLines, "\n")), &document.Frontmatter); err != nil {
		return skillDocument{Body: document.Body}
	}
	return document
}

func loadSkillAgentConfigPreviews(root string, fileMap map[string]*zip.File) ([]model.SkillAgentConfigPreview, error) {
	previews := make([]model.SkillAgentConfigPreview, 0)
	prefix := rootPrefixedPath(root, "agents") + "/"
	for name, file := range fileMap {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		ext := strings.ToLower(path.Ext(name))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		source, err := readZipFile(file)
		if err != nil {
			return nil, err
		}

		var raw map[string]any
		if err := yaml.Unmarshal([]byte(source), &raw); err != nil {
			return nil, err
		}
		var config skillAgentConfig
		if err := yaml.Unmarshal([]byte(source), &config); err != nil {
			return nil, err
		}

		previews = append(previews, model.SkillAgentConfigPreview{
			Path:             strings.TrimPrefix(name, rootPrefix(root)),
			Yaml:             normalizeMarketplaceYAML(raw),
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

func normalizeMarketplaceSkillFrontmatter(document skillDocument, requires []string, tools []string) string {
	data := make(map[string]any)
	if value := strings.TrimSpace(document.Frontmatter.Name); value != "" {
		data["name"] = value
	}
	if value := strings.TrimSpace(document.Frontmatter.Description); value != "" {
		data["description"] = value
	}
	if len(requires) > 0 {
		data["requires"] = requires
	}
	if len(tools) > 0 {
		data["tools"] = tools
	}
	if len(data) == 0 {
		return ""
	}
	return normalizeMarketplaceYAML(data)
}

func normalizeMarketplaceYAML(value any) string {
	if value == nil {
		return ""
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func normalizeMarketplaceSkillList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, strings.ReplaceAll(trimmed, "\\", "/"))
	}
	return result
}

func canonicalSkillPathFromRoot(root string) string {
	if strings.TrimSpace(root) == "" {
		return "skills"
	}
	return "skills/" + strings.TrimPrefix(root, "./")
}

func marketplaceSkillLabel(root string, document skillDocument) string {
	if value := strings.TrimSpace(document.Frontmatter.Name); value != "" {
		return value
	}
	base := path.Base(root)
	if base == "." || base == "/" || base == "" {
		return "Skill"
	}
	base = strings.NewReplacer("-", " ", "_", " ").Replace(base)
	parts := strings.Fields(base)
	for index, part := range parts {
		lower := strings.ToLower(part)
		parts[index] = strings.ToUpper(lower[:1]) + lower[1:]
	}
	return strings.Join(parts, " ")
}

func firstNonEmptyMarketplaceSkill(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstAgentDisplayName(configs []model.SkillAgentConfigPreview) string {
	for _, config := range configs {
		if strings.TrimSpace(config.DisplayName) != "" {
			return config.DisplayName
		}
	}
	return ""
}

func firstAgentDescription(configs []model.SkillAgentConfigPreview) string {
	for _, config := range configs {
		if strings.TrimSpace(config.ShortDescription) != "" {
			return config.ShortDescription
		}
	}
	return ""
}

func firstAgentPrompt(configs []model.SkillAgentConfigPreview) string {
	for _, config := range configs {
		if strings.TrimSpace(config.DefaultPrompt) != "" {
			return config.DefaultPrompt
		}
	}
	return ""
}

func countArtifactFiles(root string, fileMap map[string]*zip.File, dir string) int {
	prefix := rootPrefixedPath(root, dir) + "/"
	count := 0
	for name := range fileMap {
		if strings.HasPrefix(name, prefix) {
			count++
		}
	}
	return count
}

func rootPrefixedPath(root string, child string) string {
	if strings.TrimSpace(root) == "" {
		return child
	}
	return strings.TrimSuffix(root, "/") + "/" + strings.TrimPrefix(child, "/")
}

func rootPrefix(root string) string {
	if strings.TrimSpace(root) == "" {
		return ""
	}
	return strings.TrimSuffix(root, "/") + "/"
}

func readZipFile(file *zip.File) (string, error) {
	reader, err := file.Open()
	if err != nil {
		return "", err
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
