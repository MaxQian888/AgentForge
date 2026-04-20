package service

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/react-go-quick-starter/server/internal/model"
)

var defaultDeepReviewDimensions = []string{"logic", "security", "performance", "compliance"}

type ReviewPluginCatalog interface {
	List(ctx context.Context, filter PluginListFilter) ([]*model.PluginRecord, error)
}

type ReviewExecutionPlanner struct {
	catalog ReviewPluginCatalog
}

func NewReviewExecutionPlanner(catalog ReviewPluginCatalog) *ReviewExecutionPlanner {
	return &ReviewExecutionPlanner{catalog: catalog}
}

func (p *ReviewExecutionPlanner) BuildPlan(ctx context.Context, req *model.TriggerReviewRequest) (*model.ReviewExecutionPlan, error) {
	var requestedDimensions []string
	if req != nil {
		requestedDimensions = req.Dimensions
	}
	plan := &model.ReviewExecutionPlan{
		TriggerEvent: deriveReviewTriggerEvent(req),
		ChangedFiles: normalizeChangedFiles(req),
		Dimensions:   normalizeReviewDimensions(requestedDimensions),
		Plugins:      []model.ReviewExecutionPlugin{},
	}

	if p == nil || p.catalog == nil {
		return plan, nil
	}

	records, err := p.catalog.List(ctx, PluginListFilter{Kind: model.PluginKindReview})
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if !matchesReviewExecutionCandidate(record, plan.TriggerEvent, plan.ChangedFiles) {
			continue
		}
		plan.Plugins = append(plan.Plugins, model.ReviewExecutionPlugin{
			ID:           record.Metadata.ID,
			Name:         record.Metadata.Name,
			Entrypoint:   record.Spec.Review.Entrypoint,
			SourceType:   record.Source.Type,
			Transport:    record.Spec.Transport,
			Command:      record.Spec.Command,
			Args:         append([]string(nil), record.Spec.Args...),
			URL:          record.Spec.URL,
			Events:       append([]string(nil), record.Spec.Review.Triggers.Events...),
			FilePatterns: append([]string(nil), record.Spec.Review.Triggers.FilePatterns...),
			OutputFormat: record.Spec.Review.Output.Format,
		})
	}

	return plan, nil
}

// BuildIncrementalPlan is the diff-of-diff variant: it forces ChangedFiles
// into the plan and returns the same shape as BuildPlan. Plugins with empty
// FilePatterns are still selected (they run on the diff and we filter their
// findings post-hoc in the dispatcher); plugins with FilePatterns must
// intersect ChangedFiles.
func (p *ReviewExecutionPlanner) BuildIncrementalPlan(ctx context.Context, req *model.TriggerIncrementalReviewRequest) (*model.ReviewExecutionPlan, error) {
	if req == nil || len(req.ChangedFiles) == 0 {
		return nil, fmt.Errorf("incremental plan requires ChangedFiles")
	}
	adapted := &model.TriggerReviewRequest{
		PRURL:        req.PRURL,
		ChangedFiles: append([]string(nil), req.ChangedFiles...),
		Event:        firstNonEmpty(req.Event, "pull_request.synchronize"),
		Trigger:      "vcs_webhook",
	}
	plan, err := p.BuildPlan(ctx, adapted)
	if err != nil {
		return nil, err
	}
	plan.TriggerEvent = adapted.Event
	return plan, nil
}

func normalizeReviewDimensions(dimensions []string) []string {
	if len(dimensions) == 0 {
		return append([]string(nil), defaultDeepReviewDimensions...)
	}

	seen := make(map[string]struct{}, len(dimensions))
	normalized := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		value := strings.TrimSpace(dimension)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	if len(normalized) == 0 {
		return append([]string(nil), defaultDeepReviewDimensions...)
	}
	return normalized
}

func deriveReviewTriggerEvent(req *model.TriggerReviewRequest) string {
	if req == nil {
		return "review.manual"
	}
	if value := strings.TrimSpace(req.Event); value != "" {
		return value
	}
	if strings.TrimSpace(req.PRURL) != "" {
		return "pull_request.updated"
	}
	switch req.Trigger {
	case model.ReviewTriggerLayer1:
		return "review.layer1_escalated"
	case model.ReviewTriggerAgent:
		return "review.agent_requested"
	default:
		return "review.manual"
	}
}

func normalizeChangedFiles(req *model.TriggerReviewRequest) []string {
	if req == nil {
		return nil
	}

	seen := map[string]struct{}{}
	files := make([]string, 0, len(req.ChangedFiles))
	appendFile := func(value string) {
		value = normalizeReviewPath(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		files = append(files, value)
	}

	for _, value := range req.ChangedFiles {
		appendFile(value)
	}
	if len(files) > 0 {
		return files
	}

	for _, value := range extractChangedFilesFromDiff(req.Diff) {
		appendFile(value)
	}
	return files
}

func matchesReviewExecutionCandidate(record *model.PluginRecord, triggerEvent string, changedFiles []string) bool {
	if record == nil || record.Kind != model.PluginKindReview || record.Spec.Review == nil {
		return false
	}
	if !isEnabledReviewPluginState(record.LifecycleState) {
		return false
	}

	configuredEvents := record.Spec.Review.Triggers.Events
	if len(configuredEvents) > 0 && !slices.Contains(configuredEvents, triggerEvent) {
		return false
	}

	patterns := record.Spec.Review.Triggers.FilePatterns
	if len(patterns) == 0 {
		return true
	}
	if len(changedFiles) == 0 {
		return false
	}

	for _, changedFile := range changedFiles {
		if matchesAnyReviewPattern(changedFile, patterns) {
			return true
		}
	}
	return false
}

func isEnabledReviewPluginState(state model.PluginLifecycleState) bool {
	return state == model.PluginStateEnabled || state == model.PluginStateActive
}

func matchesAnyReviewPattern(changedFile string, patterns []string) bool {
	for _, pattern := range patterns {
		if matchReviewPattern(changedFile, pattern) {
			return true
		}
	}
	return false
}

func matchReviewPattern(changedFile, pattern string) bool {
	changedFile = normalizeReviewPath(changedFile)
	pattern = normalizeReviewPath(pattern)
	if changedFile == "" || pattern == "" {
		return false
	}

	pattern = strings.ReplaceAll(pattern, "**/", "DOUBLESTAR_DIR_PLACEHOLDER")
	pattern = regexp.QuoteMeta(pattern)
	pattern = strings.ReplaceAll(pattern, "DOUBLESTAR_DIR_PLACEHOLDER", "(?:.*/)?")
	pattern = strings.ReplaceAll(pattern, "\\*\\*", ".*")
	pattern = strings.ReplaceAll(pattern, "\\*", "[^/]*")
	pattern = strings.ReplaceAll(pattern, "\\?", "[^/]")

	matched, err := regexp.MatchString("^"+pattern+"$", changedFile)
	return err == nil && matched
}

func extractChangedFilesFromDiff(diff string) []string {
	if strings.TrimSpace(diff) == "" {
		return nil
	}

	files := []string{}
	seen := map[string]struct{}{}
	lines := strings.Split(diff, "\n")
	re := regexp.MustCompile(`^diff --git a/(.+?) b/(.+)$`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if matches := re.FindStringSubmatch(line); len(matches) == 3 {
			value := normalizeReviewPath(matches[2])
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			files = append(files, value)
		}
	}
	return files
}

func normalizeReviewPath(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\\", "/")
	value = strings.TrimPrefix(value, "a/")
	value = strings.TrimPrefix(value, "b/")
	value = path.Clean(value)
	if value == "." {
		return ""
	}
	return value
}
