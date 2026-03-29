package role

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
)

type ExecutionProfileOption func(*executionProfileOptions)

type executionProfileOptions struct {
	skillRoot string
}

func WithSkillRoot(root string) ExecutionProfileOption {
	return func(options *executionProfileOptions) {
		options.skillRoot = root
	}
}

type runtimeSkillDocument struct {
	Path        string
	Label       string
	Description string
	Instructions string
	Requires    []string
	Tools       []string
	Source      string
	SourceRoot  string
}

func resolveRuntimeSkills(manifest *Manifest, skillRoot string) ([]model.RoleExecutionSkill, []model.RoleExecutionSkill, []model.RoleExecutionSkillDiagnostic) {
	if manifest == nil || len(manifest.Capabilities.Skills) == 0 || strings.TrimSpace(skillRoot) == "" {
		return nil, nil, nil
	}

	loaded := make([]model.RoleExecutionSkill, 0)
	available := make([]model.RoleExecutionSkill, 0)
	diagnostics := make([]model.RoleExecutionSkillDiagnostic, 0)
	loadedSeen := make(map[string]struct{})
	availableSeen := make(map[string]struct{})
	visiting := make(map[string]struct{})

	var appendLoaded func(path string, blocking bool, origin string, parentPath string)
	appendLoaded = func(path string, blocking bool, origin string, parentPath string) {
		path = normalizeSkillReferencePath(path)
		if path == "" {
			return
		}
		if _, ok := loadedSeen[path]; ok {
			return
		}
		if _, ok := visiting[path]; ok {
			diagnostics = append(diagnostics, model.RoleExecutionSkillDiagnostic{
				Code:     "role_skill_cycle",
				Path:     path,
				Message:  fmt.Sprintf("Skill dependency cycle detected at %s", path),
				Blocking: blocking,
				AutoLoad: blocking,
			})
			return
		}
		visiting[path] = struct{}{}
		doc, err := readRuntimeSkillDocument(skillRoot, path)
		if err != nil {
			message := fmt.Sprintf("Skill %s could not be resolved from repo-local skills", path)
			code := "role_skill_not_found"
			if parentPath != "" {
				message = fmt.Sprintf("Skill %s required by %s could not be resolved from repo-local skills", path, parentPath)
				code = "role_skill_dependency_not_found"
			}
			diagnostics = append(diagnostics, model.RoleExecutionSkillDiagnostic{
				Code:     code,
				Path:     path,
				Message:  message,
				Blocking: blocking,
				AutoLoad: blocking,
			})
			delete(visiting, path)
			return
		}

		loadedSeen[path] = struct{}{}
		loaded = append(loaded, model.RoleExecutionSkill{
			Path:         doc.Path,
			Label:        doc.Label,
			Description:  doc.Description,
			Instructions: doc.Instructions,
			Source:       doc.Source,
			SourceRoot:   doc.SourceRoot,
			Origin:       origin,
			Requires:     append([]string(nil), doc.Requires...),
			Tools:        append([]string(nil), doc.Tools...),
		})
		for _, requirePath := range doc.Requires {
			appendLoaded(requirePath, blocking, "dependency", path)
		}
		delete(visiting, path)
	}

	for _, skill := range manifest.Capabilities.Skills {
		path := normalizeSkillReferencePath(skill.Path)
		if path == "" {
			continue
		}
		if skill.AutoLoad {
			appendLoaded(path, true, "direct", "")
			continue
		}
		if _, ok := availableSeen[path]; ok {
			continue
		}
		doc, err := readRuntimeSkillDocument(skillRoot, path)
		if err != nil {
			diagnostics = append(diagnostics, model.RoleExecutionSkillDiagnostic{
				Code:     "role_skill_not_found",
				Path:     path,
				Message:  fmt.Sprintf("Skill %s is not currently available in repo-local skills", path),
				Blocking: false,
				AutoLoad: false,
			})
			continue
		}
		availableSeen[path] = struct{}{}
		available = append(available, model.RoleExecutionSkill{
			Path:        doc.Path,
			Label:       doc.Label,
			Description: doc.Description,
			Source:      doc.Source,
			SourceRoot:  doc.SourceRoot,
			Origin:      "direct",
			Requires:    append([]string(nil), doc.Requires...),
			Tools:       append([]string(nil), doc.Tools...),
		})
	}

	slices.SortFunc(diagnostics, func(a, b model.RoleExecutionSkillDiagnostic) int {
		if compare := strings.Compare(a.Path, b.Path); compare != 0 {
			return compare
		}
		return strings.Compare(a.Code, b.Code)
	})

	return loaded, available, diagnostics
}

func HasBlockingSkillDiagnostics(profile *ExecutionProfile) bool {
	if profile == nil {
		return false
	}
	for _, diagnostic := range profile.SkillDiagnostics {
		if diagnostic.Blocking {
			return true
		}
	}
	return false
}

func readRuntimeSkillDocument(root, canonicalPath string) (*runtimeSkillDocument, error) {
	canonicalPath = normalizeSkillReferencePath(canonicalPath)
	if canonicalPath == "" {
		return nil, fmt.Errorf("skill path is required")
	}
	relative := strings.TrimPrefix(canonicalPath, "skills/")
	skillFile := filepath.Join(root, filepath.FromSlash(relative), "SKILL.md")
	content, err := os.ReadFile(skillFile)
	if err != nil {
		return nil, err
	}

	document := parseSkillDocument(string(content))
	label := strings.TrimSpace(document.Frontmatter.Name)
	if label == "" {
		label = humanizeSkillLabel(relative)
	}

	return &runtimeSkillDocument{
		Path:         canonicalPath,
		Label:        label,
		Description:  strings.TrimSpace(document.Frontmatter.Description),
		Instructions: strings.TrimSpace(document.Body),
		Requires:     normalizeRequiredSkillPaths(document.Frontmatter.Requires),
		Tools:        trimNonEmpty(document.Frontmatter.Tools),
		Source:       "repo-local",
		SourceRoot:   "skills",
	}, nil
}

func normalizeRequiredSkillPaths(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		path := normalizeSkillReferencePath(value)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		normalized = append(normalized, path)
	}
	return normalized
}

func normalizeSkillReferencePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "skills/") {
		return value
	}
	if !strings.Contains(value, "/") {
		return "skills/" + value
	}
	return value
}

func trimNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
