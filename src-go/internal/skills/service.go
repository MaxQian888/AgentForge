package skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	rolepkg "github.com/agentforge/server/internal/role"
	"gopkg.in/yaml.v3"
)

type Family string

const (
	FamilyBuiltInRuntime Family = "built-in-runtime"
	FamilyRepoAssistant  Family = "repo-assistant"
	FamilyWorkflowMirror Family = "workflow-mirror"
)

type HealthStatus string

const (
	HealthHealthy HealthStatus = "healthy"
	HealthWarning HealthStatus = "warning"
	HealthBlocked HealthStatus = "blocked"
	HealthDrifted HealthStatus = "drifted"
)

type ListOptions struct {
	Families []Family
}

type VerifyOptions struct {
	Families []Family
}

type SyncMirrorsOptions struct {
	SkillIDs []string
}

type Issue struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	TargetPath string `json:"targetPath,omitempty"`
	Family     Family `json:"family,omitempty"`
	SourceType string `json:"sourceType,omitempty"`
}

type HealthSummary struct {
	Status HealthStatus `json:"status"`
	Issues []Issue      `json:"issues,omitempty"`
}

type BundleInfo struct {
	Member   bool     `json:"member"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	DocsRef  string   `json:"docsRef,omitempty"`
	Featured bool     `json:"featured,omitempty"`
}

type LockInfo struct {
	Key          string `json:"key"`
	Source       string `json:"source,omitempty"`
	SourceType   string `json:"sourceType,omitempty"`
	ComputedHash string `json:"computedHash,omitempty"`
}

type ConsumerSurface struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Label   string `json:"label"`
	Href    string `json:"href,omitempty"`
	Message string `json:"message,omitempty"`
}

type BlockedAction struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type InventoryItem struct {
	ID                  string                       `json:"id"`
	Family              Family                       `json:"family"`
	VerificationProfile string                       `json:"verificationProfile,omitempty"`
	CanonicalRoot       string                       `json:"canonicalRoot"`
	SourceType          string                       `json:"sourceType"`
	DocsRef             string                       `json:"docsRef,omitempty"`
	Lock                *LockInfo                    `json:"lock,omitempty"`
	Bundle              BundleInfo                   `json:"bundle"`
	MirrorTargets       []string                     `json:"mirrorTargets,omitempty"`
	PreviewAvailable    bool                         `json:"previewAvailable"`
	PreviewError        string                       `json:"previewError,omitempty"`
	Health              HealthSummary                `json:"health"`
	ConsumerSurfaces    []ConsumerSurface            `json:"consumerSurfaces,omitempty"`
	SupportedActions    []string                     `json:"supportedActions,omitempty"`
	BlockedActions      []BlockedAction              `json:"blockedActions,omitempty"`
	Preview             *rolepkg.SkillPackagePreview `json:"preview,omitempty"`
}

type VerificationResult struct {
	SkillID string       `json:"skillId"`
	Family  Family       `json:"family"`
	Status  HealthStatus `json:"status"`
	Issues  []Issue      `json:"issues,omitempty"`
}

type VerifyResult struct {
	OK      bool                 `json:"ok"`
	Results []VerificationResult `json:"results"`
}

type SyncMirrorsResult struct {
	UpdatedTargets []string             `json:"updatedTargets,omitempty"`
	Results        []VerificationResult `json:"results,omitempty"`
}

type Service struct {
	repoRoot string
}

func NewService(repoRoot string) *Service {
	return &Service{repoRoot: repoRoot}
}

func (s *Service) List(opts ListOptions) ([]InventoryItem, error) {
	registry, err := loadRegistry(s.repoRoot)
	if err != nil {
		return nil, err
	}
	lock, err := loadSkillsLock(s.repoRoot)
	if err != nil {
		return nil, err
	}
	bundle, err := loadBuiltInBundle(s.repoRoot)
	if err != nil {
		return nil, err
	}

	allowedFamilies := make(map[Family]struct{}, len(opts.Families))
	for _, family := range opts.Families {
		allowedFamilies[family] = struct{}{}
	}

	items := make([]InventoryItem, 0, len(registry.Skills))
	for _, entry := range registry.Skills {
		if len(allowedFamilies) > 0 {
			if _, ok := allowedFamilies[entry.Family]; !ok {
				continue
			}
		}
		item := InventoryItem{
			ID:                  entry.ID,
			Family:              entry.Family,
			VerificationProfile: entry.VerificationProfile,
			CanonicalRoot:       entry.CanonicalRoot,
			SourceType:          entry.SourceType,
			DocsRef:             entry.DocsRef,
			MirrorTargets:       append([]string(nil), entry.MirrorTargets...),
			SupportedActions:    supportedActions(entry.Family),
			BlockedActions:      blockedActions(entry.Family),
		}

		item.Preview, item.PreviewError = loadPreview(s.repoRoot, entry.CanonicalRoot)
		item.PreviewAvailable = item.Preview != nil && item.PreviewError == ""
		item.Health.Status = HealthHealthy

		if item.PreviewError != "" {
			appendIssue(&item, Issue{
				Code:       "preview_unavailable",
				Message:    item.PreviewError,
				TargetPath: entry.CanonicalRoot,
				Family:     entry.Family,
				SourceType: entry.SourceType,
			}, HealthBlocked)
		}

		if entry.Family == FamilyBuiltInRuntime {
			if bundleEntry, ok := bundle.ByID[entry.ID]; ok && normalizePath(bundleEntry.Root) == normalizePath(entry.CanonicalRoot) {
				item.Bundle = BundleInfo{
					Member:   true,
					Category: bundleEntry.Category,
					Tags:     append([]string(nil), bundleEntry.Tags...),
					DocsRef:  firstNonEmpty(strings.TrimSpace(bundleEntry.DocsRef), entry.DocsRef),
					Featured: bundleEntry.Featured,
				}
			} else {
				appendIssue(&item, Issue{
					Code:       "built_in_bundle_mismatch",
					Message:    fmt.Sprintf("missing or mismatched built-in bundle entry for %s", entry.ID),
					TargetPath: entry.CanonicalRoot,
					Family:     entry.Family,
					SourceType: entry.SourceType,
				}, HealthBlocked)
			}
		}

		if entry.LockKey != "" {
			if lockEntry, ok := lock.Skills[entry.LockKey]; ok {
				item.Lock = &LockInfo{
					Key:          entry.LockKey,
					Source:       lockEntry.Source,
					SourceType:   lockEntry.SourceType,
					ComputedHash: lockEntry.ComputedHash,
				}
			} else {
				appendIssue(&item, Issue{
					Code:       "missing_lock_entry",
					Message:    fmt.Sprintf("missing skills-lock.json entry for %s", entry.LockKey),
					TargetPath: "skills-lock.json",
					Family:     entry.Family,
					SourceType: entry.SourceType,
				}, HealthBlocked)
			}
		}

		if entry.Family == FamilyWorkflowMirror {
			if err := validateMirrorTargets(s.repoRoot, entry, &item); err != nil {
				return nil, err
			}
		}

		item.ConsumerSurfaces = resolveConsumerSurfaces(item)
		items = append(items, item)
	}

	slices.SortFunc(items, func(a, b InventoryItem) int {
		return strings.Compare(a.ID, b.ID)
	})
	return items, nil
}

func (s *Service) Get(id string) (*InventoryItem, error) {
	items, err := s.List(ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range items {
		if items[i].ID == id {
			return &items[i], nil
		}
	}
	return nil, os.ErrNotExist
}

func (s *Service) Verify(opts VerifyOptions) (*VerifyResult, error) {
	items, err := s.List(ListOptions{Families: opts.Families})
	if err != nil {
		return nil, err
	}
	results := make([]VerificationResult, 0, len(items))
	ok := true
	for _, item := range items {
		results = append(results, VerificationResult{
			SkillID: item.ID,
			Family:  item.Family,
			Status:  item.Health.Status,
			Issues:  append([]Issue(nil), item.Health.Issues...),
		})
		if item.Health.Status == HealthBlocked || item.Health.Status == HealthDrifted {
			ok = false
		}
	}
	return &VerifyResult{OK: ok, Results: results}, nil
}

func (s *Service) SyncMirrors(opts SyncMirrorsOptions) (*SyncMirrorsResult, error) {
	registry, err := loadRegistry(s.repoRoot)
	if err != nil {
		return nil, err
	}
	requestedIDs := make(map[string]struct{}, len(opts.SkillIDs))
	for _, id := range opts.SkillIDs {
		requestedIDs[id] = struct{}{}
	}

	updatedTargets := make([]string, 0)
	for _, entry := range registry.Skills {
		if entry.Family != FamilyWorkflowMirror {
			continue
		}
		if len(requestedIDs) > 0 {
			if _, ok := requestedIDs[entry.ID]; !ok {
				continue
			}
		}
		canonicalBytes, err := os.ReadFile(filepath.Join(s.repoRoot, filepath.FromSlash(entry.CanonicalRoot), "SKILL.md"))
		if err != nil {
			return nil, err
		}
		for _, target := range entry.MirrorTargets {
			targetPath := filepath.Join(s.repoRoot, filepath.FromSlash(target))
			current, err := os.ReadFile(targetPath)
			if err == nil && string(current) == string(canonicalBytes) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(targetPath, canonicalBytes, 0o644); err != nil {
				return nil, err
			}
			updatedTargets = append(updatedTargets, target)
		}
	}

	verifyResult, err := s.Verify(VerifyOptions{Families: []Family{FamilyWorkflowMirror}})
	if err != nil {
		return nil, err
	}
	return &SyncMirrorsResult{
		UpdatedTargets: updatedTargets,
		Results:        verifyResult.Results,
	}, nil
}

type registryFile struct {
	Skills []registryEntry `yaml:"skills"`
}

type registryEntry struct {
	ID                  string   `yaml:"id"`
	Family              Family   `yaml:"family"`
	VerificationProfile string   `yaml:"verificationProfile"`
	CanonicalRoot       string   `yaml:"canonicalRoot"`
	SourceType          string   `yaml:"sourceType"`
	DocsRef             string   `yaml:"docsRef,omitempty"`
	LockKey             string   `yaml:"lockKey,omitempty"`
	MirrorTargets       []string `yaml:"mirrorTargets,omitempty"`
}

type skillsLockFile struct {
	Skills map[string]skillsLockEntry `json:"skills"`
}

type skillsLockEntry struct {
	Source       string `json:"source"`
	SourceType   string `json:"sourceType"`
	ComputedHash string `json:"computedHash"`
}

type builtInBundleFile struct {
	Skills []builtInBundleEntry `yaml:"skills"`
}

type builtInBundle struct {
	ByID map[string]builtInBundleEntry
}

type builtInBundleEntry struct {
	ID       string   `yaml:"id"`
	Root     string   `yaml:"root"`
	Category string   `yaml:"category"`
	Tags     []string `yaml:"tags"`
	DocsRef  string   `yaml:"docsRef,omitempty"`
	Featured bool     `yaml:"featured,omitempty"`
}

func loadRegistry(repoRoot string) (*registryFile, error) {
	raw, err := os.ReadFile(filepath.Join(repoRoot, "internal-skills.yaml"))
	if err != nil {
		return nil, err
	}
	var registry registryFile
	if err := yaml.Unmarshal(raw, &registry); err != nil {
		return nil, err
	}
	for i := range registry.Skills {
		registry.Skills[i].CanonicalRoot = normalizePath(registry.Skills[i].CanonicalRoot)
		for j, target := range registry.Skills[i].MirrorTargets {
			registry.Skills[i].MirrorTargets[j] = normalizePath(target)
		}
	}
	return &registry, nil
}

func loadSkillsLock(repoRoot string) (*skillsLockFile, error) {
	lockPath := filepath.Join(repoRoot, "skills-lock.json")
	raw, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &skillsLockFile{Skills: map[string]skillsLockEntry{}}, nil
		}
		return nil, err
	}
	var lock skillsLockFile
	if err := json.Unmarshal(raw, &lock); err != nil {
		return nil, err
	}
	if lock.Skills == nil {
		lock.Skills = map[string]skillsLockEntry{}
	}
	return &lock, nil
}

func loadBuiltInBundle(repoRoot string) (*builtInBundle, error) {
	bundlePath := filepath.Join(repoRoot, "skills", "builtin-bundle.yaml")
	raw, err := os.ReadFile(bundlePath)
	if err != nil {
		if os.IsNotExist(err) {
			return &builtInBundle{ByID: map[string]builtInBundleEntry{}}, nil
		}
		return nil, err
	}
	var file builtInBundleFile
	if err := yaml.Unmarshal(raw, &file); err != nil {
		return nil, err
	}
	result := &builtInBundle{ByID: make(map[string]builtInBundleEntry, len(file.Skills))}
	for _, entry := range file.Skills {
		entry.Root = normalizePath(filepath.ToSlash(filepath.Join("skills", strings.TrimPrefix(strings.TrimSpace(entry.Root), "skills/"))))
		result.ByID[entry.ID] = entry
	}
	return result, nil
}

func validateMirrorTargets(repoRoot string, entry registryEntry, item *InventoryItem) error {
	canonicalBytes, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(entry.CanonicalRoot), "SKILL.md"))
	if err != nil {
		appendIssue(item, Issue{
			Code:       "canonical_skill_missing",
			Message:    fmt.Sprintf("missing canonical skill document for %s", entry.CanonicalRoot),
			TargetPath: entry.CanonicalRoot,
			Family:     entry.Family,
			SourceType: entry.SourceType,
		}, HealthBlocked)
		return nil
	}

	for _, target := range entry.MirrorTargets {
		targetPath := filepath.Join(repoRoot, filepath.FromSlash(target))
		targetBytes, err := os.ReadFile(targetPath)
		if err != nil || string(targetBytes) != string(canonicalBytes) {
			appendIssue(item, Issue{
				Code:       "mirror_drift",
				Message:    fmt.Sprintf("mirror target %s does not match canonical skill content", target),
				TargetPath: target,
				Family:     entry.Family,
				SourceType: entry.SourceType,
			}, HealthDrifted)
		}
	}
	return nil
}

func resolveConsumerSurfaces(item InventoryItem) []ConsumerSurface {
	switch item.Family {
	case FamilyBuiltInRuntime:
		surfaces := []ConsumerSurface{{
			ID:     "role-skill-catalog",
			Status: "available",
			Label:  "Role Skill Catalog",
			Href:   "/roles",
		}}
		if item.Bundle.Member {
			surfaces = append(surfaces, ConsumerSurface{
				ID:     "marketplace-built-ins",
				Status: "available",
				Label:  "Marketplace Built-ins",
				Href:   "/marketplace",
			})
		}
		return surfaces
	case FamilyRepoAssistant:
		return []ConsumerSurface{{
			ID:      "repo-workflow",
			Status:  "available",
			Label:   "Repository Workflow",
			Message: "Repo-assistant skills are consumed by Codex/Claude repository workflows.",
		}}
	case FamilyWorkflowMirror:
		return []ConsumerSurface{{
			ID:      "workflow-mirrors",
			Status:  "available",
			Label:   "Workflow Mirrors",
			Message: "Canonical workflow skills sync to mirror targets for external consumers.",
		}}
	default:
		return nil
	}
}

func supportedActions(family Family) []string {
	switch family {
	case FamilyBuiltInRuntime:
		return []string{"verify-internal", "verify-builtins", "open-roles", "open-marketplace"}
	case FamilyRepoAssistant:
		return []string{"verify-internal"}
	case FamilyWorkflowMirror:
		return []string{"verify-internal", "sync-mirrors"}
	default:
		return nil
	}
}

func blockedActions(family Family) []BlockedAction {
	switch family {
	case FamilyBuiltInRuntime:
		return []BlockedAction{{ID: "sync-mirrors", Reason: "only workflow-mirror skills can sync mirrors"}}
	case FamilyRepoAssistant:
		return []BlockedAction{
			{ID: "sync-mirrors", Reason: "only workflow-mirror skills can sync mirrors"},
			{ID: "refresh-upstream", Reason: "upstream refresh remains a maintainer workflow outside the operator UI"},
		}
	case FamilyWorkflowMirror:
		return []BlockedAction{{ID: "refresh-upstream", Reason: "workflow mirror skills are repo-authored and do not support upstream refresh"}}
	default:
		return nil
	}
}

func loadPreview(repoRoot, canonicalRoot string) (*rolepkg.SkillPackagePreview, string) {
	preview, err := rolepkg.ReadManagedSkillPackagePreview(repoRoot, canonicalRoot)
	if err != nil {
		return nil, err.Error()
	}
	return preview, ""
}

func appendIssue(item *InventoryItem, issue Issue, status HealthStatus) {
	item.Health.Issues = append(item.Health.Issues, issue)
	if severity(status) > severity(item.Health.Status) {
		item.Health.Status = status
	}
}

func severity(status HealthStatus) int {
	switch status {
	case HealthBlocked:
		return 4
	case HealthDrifted:
		return 3
	case HealthWarning:
		return 2
	case HealthHealthy:
		return 1
	default:
		return 0
	}
}

func normalizePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	value = strings.TrimPrefix(value, "./")
	value = strings.TrimPrefix(value, "/")
	if value == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
