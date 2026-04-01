package role

import (
	"fmt"
	"slices"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
)

func appendSkillCompatibilityDiagnostics(
	manifest *Manifest,
	loaded []model.RoleExecutionSkill,
	available []model.RoleExecutionSkill,
	diagnostics []model.RoleExecutionSkillDiagnostic,
) []model.RoleExecutionSkillDiagnostic {
	capabilities := buildRoleCapabilitySet(manifest)
	augmented := append([]model.RoleExecutionSkillDiagnostic(nil), diagnostics...)
	augmented = append(augmented, buildSkillCompatibilityDiagnostics(loaded, capabilities, true)...)
	augmented = append(augmented, buildSkillCompatibilityDiagnostics(available, capabilities, false)...)
	slices.SortFunc(augmented, func(a, b model.RoleExecutionSkillDiagnostic) int {
		if compare := strings.Compare(a.Path, b.Path); compare != 0 {
			return compare
		}
		if compare := strings.Compare(a.Code, b.Code); compare != 0 {
			return compare
		}
		return strings.Compare(a.Message, b.Message)
	})
	return dedupeSkillDiagnostics(augmented)
}

func buildSkillCompatibilityDiagnostics(
	skills []model.RoleExecutionSkill,
	capabilities map[string]struct{},
	blocking bool,
) []model.RoleExecutionSkillDiagnostic {
	diagnostics := make([]model.RoleExecutionSkillDiagnostic, 0)
	for _, skill := range skills {
		missing := missingRequiredCapabilities(skill.Tools, capabilities)
		if len(missing) == 0 {
			continue
		}

		prefix := "Skill"
		if blocking {
			if strings.TrimSpace(skill.Origin) == "dependency" {
				prefix = "Auto-loaded dependency skill"
			} else {
				prefix = "Auto-loaded skill"
			}
		} else {
			prefix = "On-demand skill"
		}

		diagnostics = append(diagnostics, model.RoleExecutionSkillDiagnostic{
			Code:     "role_skill_tools_unavailable",
			Path:     skill.Path,
			Message:  fmt.Sprintf("%s %s requires tool capabilities %s that this role does not currently provide", prefix, skill.Path, strings.Join(missing, ", ")),
			Blocking: blocking,
			AutoLoad: blocking,
		})
	}
	return diagnostics
}

func dedupeSkillDiagnostics(values []model.RoleExecutionSkillDiagnostic) []model.RoleExecutionSkillDiagnostic {
	deduped := make([]model.RoleExecutionSkillDiagnostic, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, diagnostic := range values {
		key := strings.Join([]string{
			diagnostic.Code,
			diagnostic.Path,
			diagnostic.Message,
			fmt.Sprintf("%t", diagnostic.Blocking),
			fmt.Sprintf("%t", diagnostic.AutoLoad),
		}, "|")
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, diagnostic)
	}
	return deduped
}
