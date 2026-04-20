package vcs

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/model"
)

// ReviewServiceForRouter is the narrow interface the webhook router uses.
type ReviewServiceForRouter interface {
	Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
	TriggerIncremental(ctx context.Context, req *model.TriggerIncrementalReviewRequest) (*model.Review, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
}

// ReviewRepo is the narrow persistence interface the webhook router uses
// to look up prior reviews by integration + PR number.
type ReviewRepo interface {
	GetLatestByIntegrationAndPR(ctx context.Context, integrationID uuid.UUID, prNumber int) (*model.Review, error)
}

// WebhookRouter routes incoming VCS webhook events to the appropriate
// review service method. Created by Spec 2B, extended here for
// pull_request:synchronize handling (Spec 2C Trace C).
type WebhookRouter struct {
	reviews    ReviewServiceForRouter
	reviewRepo ReviewRepo
	vcs        Provider
	registry   *Registry
}

// NewWebhookRouter constructs a webhook router.
func NewWebhookRouter(reviews ReviewServiceForRouter, repo ReviewRepo, registry *Registry) *WebhookRouter {
	return &WebhookRouter{
		reviews:    reviews,
		reviewRepo: repo,
		registry:   registry,
	}
}

// RouteEvent dispatches a webhook event to the correct review path.
func (r *WebhookRouter) RouteEvent(ctx context.Context, integration *model.VCSIntegration, eventType, action string, payload map[string]any) error {
	if eventType != "pull_request" {
		return nil // Only PR events handled currently.
	}

	switch action {
	case "opened", "reopened":
		return r.routeOpened(ctx, integration, payload)
	case "synchronize":
		return r.routeSynchronize(ctx, integration, payload)
	default:
		return nil
	}
}

func (r *WebhookRouter) routeOpened(ctx context.Context, integration *model.VCSIntegration, payload map[string]any) error {
	prURL := extractString(payload, "pull_request.html_url")
	prNumber := extractInt(payload, "pull_request.number")
	headSHA := extractString(payload, "pull_request.head.sha")
	baseSHA := extractString(payload, "pull_request.base.sha")

	_, err := r.reviews.Trigger(ctx, &model.TriggerReviewRequest{
		PRURL:         prURL,
		PRNumber:      prNumber,
		Trigger:       "vcs_webhook",
		Event:         "pull_request.opened",
		IntegrationID: integration.ID.String(),
		HeadSHA:       headSHA,
		BaseSHA:       baseSHA,
	})
	return err
}

func (r *WebhookRouter) routeSynchronize(ctx context.Context, integration *model.VCSIntegration, payload map[string]any) error {
	prNumber := extractInt(payload, "pull_request.number")
	headSHA := extractString(payload, "pull_request.head.sha")

	if prNumber == 0 || headSHA == "" {
		return fmt.Errorf("synchronize event missing pr_number or head_sha")
	}

	// Find the latest reviewed review for this (integration, PR).
	parent, err := r.reviewRepo.GetLatestByIntegrationAndPR(ctx, integration.ID, prNumber)
	if err != nil || parent == nil || parent.LastReviewedSHA == "" {
		// No prior review — fall back to full trigger.
		log.WithFields(log.Fields{
			"integrationId": integration.ID.String(),
			"prNumber":      prNumber,
			"headSha":       headSHA,
		}).Info("vcs:webhook:synchronize: no prior review, falling back to full trigger")
		return r.routeOpened(ctx, integration, payload)
	}

	if parent.LastReviewedSHA == headSHA {
		// Already reviewed at this SHA — no-op.
		return nil
	}

	// Resolve the provider to get changed files.
	provider, err := r.resolveProvider(ctx, integration)
	if err != nil {
		return fmt.Errorf("resolve provider for compare: %w", err)
	}

	repo := repoRefFromIntegration(integration)
	diff, err := provider.ComparePullRequest(ctx, repo, parent.LastReviewedSHA, headSHA)
	if err != nil {
		return fmt.Errorf("compare PR %s..%s: %w", parent.LastReviewedSHA, headSHA, err)
	}

	changedFiles := extractFilePaths(diff)
	if len(changedFiles) == 0 {
		log.WithFields(log.Fields{
			"integrationId": integration.ID.String(),
			"prNumber":      prNumber,
		}).Info("vcs:webhook:noop_no_diff")
		return nil
	}

	// Skip heuristic: if all changed files had no findings in parent.
	if r.skipHeuristic(parent, changedFiles) {
		log.WithFields(log.Fields{
			"integrationId": integration.ID.String(),
			"prNumber":      prNumber,
		}).Info("vcs:webhook:noop_no_findings_cache")
		return nil
	}

	actingEmployee := ""
	if integration.ActingEmployeeID != nil {
		actingEmployee = integration.ActingEmployeeID.String()
	}
	_, err = r.reviews.TriggerIncremental(ctx, &model.TriggerIncrementalReviewRequest{
		ParentReviewID:   parent.ID.String(),
		IntegrationID:    integration.ID.String(),
		PRURL:            parent.PRURL,
		HeadSHA:          headSHA,
		BaseSHA:          parent.LastReviewedSHA,
		ChangedFiles:     changedFiles,
		Event:            "pull_request.synchronize",
		ActingEmployeeID: actingEmployee,
	})
	return err
}

// skipHeuristic returns true when ALL changed files are in the parent review's
// no-findings set (files that were reviewed and produced zero findings). This
// avoids re-running LLM reviewers on files that were already clean.
func (r *WebhookRouter) skipHeuristic(parent *model.Review, changedFiles []string) bool {
	if parent.ExecutionMetadata == nil {
		return false
	}
	noFindingsSet := make(map[string]struct{}, len(parent.ExecutionMetadata.NoFindingsFiles))
	for _, f := range parent.ExecutionMetadata.NoFindingsFiles {
		noFindingsSet[f] = struct{}{}
	}
	if len(noFindingsSet) == 0 {
		return false
	}
	for _, f := range changedFiles {
		if _, ok := noFindingsSet[f]; !ok {
			return false
		}
	}
	return true
}

func (r *WebhookRouter) resolveProvider(_ context.Context, integration *model.VCSIntegration) (Provider, error) {
	if r.registry == nil {
		return nil, fmt.Errorf("no registry configured")
	}
	// Token is resolved at call time via the token secret ref; for now we
	// pass empty and let the provider resolve it from context/config.
	return r.registry.Resolve(integration.Provider, integration.Host, "")
}

func repoRefFromIntegration(integration *model.VCSIntegration) RepoRef {
	return RepoRef{
		Host:  integration.Host,
		Owner: integration.Owner,
		Repo:  integration.Repo,
	}
}

func extractFilePaths(diff *Diff) []string {
	if diff == nil {
		return nil
	}
	paths := make([]string, 0, len(diff.ChangedFiles))
	for _, f := range diff.ChangedFiles {
		if f.Path != "" {
			paths = append(paths, f.Path)
		}
	}
	return paths
}

func extractString(payload map[string]any, path string) string {
	parts := strings.Split(path, ".")
	current := payload
	for i, part := range parts {
		if i == len(parts)-1 {
			v, _ := current[part].(string)
			return v
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return ""
}

func extractInt(payload map[string]any, path string) int {
	parts := strings.Split(path, ".")
	current := payload
	for i, part := range parts {
		if i == len(parts)-1 {
			switch v := current[part].(type) {
			case float64:
				return int(v)
			case int:
				return v
			}
			return 0
		}
		next, ok := current[part].(map[string]any)
		if !ok {
			return 0
		}
		current = next
	}
	return 0
}
