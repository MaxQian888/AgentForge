package liveartifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// reviewReader is the narrow slice of the review repository the
// projector needs.
type reviewReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
}

// taskReader is the narrow slice of the task repository the projector
// needs to resolve the linked task for cross-project checks and title
// rendering. It may be nil at construction; when nil the cross-project
// guard is skipped and the heading falls back to the task id.
type taskReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
}

// ReviewProjector renders a review as a BlockNote JSON fragment.
type ReviewProjector struct {
	reviews reviewReader
	tasks   taskReader
}

// NewReviewProjector constructs the projector with its dependencies.
// The tasks dep may be nil; see taskReader.
func NewReviewProjector(reviews reviewReader, tasks taskReader) *ReviewProjector {
	return &ReviewProjector{reviews: reviews, tasks: tasks}
}

// Kind reports the discriminator this projector handles.
func (p *ReviewProjector) Kind() LiveArtifactKind { return KindReview }

// RequiredRole reports the minimum role tier for a successful projection.
func (p *ReviewProjector) RequiredRole() Role { return RoleViewer }

// reviewTargetRef is the target_ref shape this projector accepts.
type reviewTargetRef struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// reviewViewOpts is the view_opts shape this projector accepts.
type reviewViewOpts struct {
	ShowFindingsPreview *bool `json:"show_findings_preview,omitempty"`
}

const (
	reviewFindingsPreviewMax = 3
	reviewSummaryMaxLen      = 400
)

// Project runs the projection. See LiveArtifactProjector for contract.
func (p *ReviewProjector) Project(
	ctx context.Context,
	principal model.PrincipalContext,
	projectID uuid.UUID,
	targetRef json.RawMessage,
	viewOpts json.RawMessage,
) (ProjectionResult, error) {
	now := time.Now().UTC()

	if !PrincipalHasRole(principal, p.RequiredRole()) {
		return ProjectionResult{Status: StatusForbidden, ProjectedAt: now}, nil
	}

	ref, err := parseReviewTargetRef(targetRef)
	if err != nil {
		return ProjectionResult{
			Status:      StatusNotFound,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	showPreview := parseReviewViewOpts(viewOpts)

	review, err := p.reviews.GetByID(ctx, ref)
	if review == nil {
		// Missing review is not-found regardless of the repo's inner error.
		diag := ""
		if err != nil {
			diag = err.Error()
		}
		return ProjectionResult{
			Status:      StatusNotFound,
			ProjectedAt: now,
			Diagnostics: diag,
		}, nil
	}
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ProjectionResult{
				Status:      StatusNotFound,
				ProjectedAt: now,
				Diagnostics: err.Error(),
			}, nil
		}
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	// Cross-project check + task title resolution. If tasks dep is nil,
	// skip the guard with a short diagnostic and render with the id-based
	// heading; callers must have validated project membership upstream.
	var taskTitle string
	if p.tasks != nil {
		task, terr := p.tasks.GetByID(ctx, review.TaskID)
		if task == nil || errors.Is(terr, gorm.ErrRecordNotFound) {
			// Linked task missing -> still render, heading uses id form.
		} else if terr != nil {
			return ProjectionResult{
				Status:      StatusDegraded,
				ProjectedAt: now,
				Diagnostics: terr.Error(),
			}, nil
		} else {
			if task.ProjectID != projectID {
				// Do not leak cross-project existence.
				return ProjectionResult{Status: StatusNotFound, ProjectedAt: now}, nil
			}
			taskTitle = task.Title
		}
	} else {
		log.Printf("liveartifact.review: task reader nil; skipping cross-project guard for review %s", review.ID)
	}

	fragment, err := renderReviewBlocks(review, taskTitle, showPreview)
	if err != nil {
		return ProjectionResult{
			Status:      StatusDegraded,
			ProjectedAt: now,
			Diagnostics: err.Error(),
		}, nil
	}

	ttl := 30 * time.Second
	return ProjectionResult{
		Status:      StatusOK,
		Projection:  fragment,
		ProjectedAt: now,
		TTLHint:     &ttl,
	}, nil
}

// Subscribe lists hub event topics that trigger a re-projection.
// Event names mirror constants in internal/ws/events.go; we inline the
// strings here to avoid coupling the projector package to the hub.
func (p *ReviewProjector) Subscribe(targetRef json.RawMessage) []EventTopic {
	empty := []EventTopic{}
	ref, err := parseReviewTargetRef(targetRef)
	if err != nil {
		return empty
	}
	scope := map[string]string{"review_id": ref.String()}
	names := []string{
		"review.updated",
		"review.completed",
		"review.pending_human",
		"review.fix_requested",
	}
	out := make([]EventTopic, 0, len(names))
	for _, name := range names {
		out = append(out, EventTopic{Event: name, Scope: copyScope(scope)})
	}
	return out
}

// --- helpers ---

func parseReviewTargetRef(raw json.RawMessage) (uuid.UUID, error) {
	if len(raw) == 0 {
		return uuid.Nil, fmt.Errorf("target_ref missing")
	}
	var ref reviewTargetRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return uuid.Nil, fmt.Errorf("target_ref invalid: %w", err)
	}
	if ref.Kind != "" && ref.Kind != string(KindReview) {
		return uuid.Nil, fmt.Errorf("target_ref kind %q not review", ref.Kind)
	}
	id, err := uuid.Parse(ref.ID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("target_ref id invalid: %w", err)
	}
	return id, nil
}

func parseReviewViewOpts(raw json.RawMessage) bool {
	// Default: show findings preview.
	if len(raw) == 0 {
		return true
	}
	var opts reviewViewOpts
	if err := json.Unmarshal(raw, &opts); err != nil {
		return true
	}
	if opts.ShowFindingsPreview == nil {
		return true
	}
	return *opts.ShowFindingsPreview
}

func renderReviewBlocks(review *model.Review, taskTitle string, showPreview bool) (json.RawMessage, error) {
	blocks := make([]map[string]any, 0, 8)

	heading := taskTitle
	if heading == "" {
		heading = "Task " + review.TaskID.String()
	}
	blocks = append(blocks, headingBlock(3, "Review: "+heading))

	risk := review.RiskLevel
	if risk == "" {
		risk = "—"
	}
	recommendation := review.Recommendation
	if recommendation == "" {
		recommendation = "pending"
	}
	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Status: %s · Risk: %s · Layer: %d · Recommendation: %s",
		review.Status, risk, review.Layer, recommendation,
	)))

	if review.PRNumber != 0 || review.PRURL != "" {
		blocks = append(blocks, paragraphBlock(fmt.Sprintf(
			"PR: #%d — %s", review.PRNumber, review.PRURL,
		)))
	}

	total, crit, high, med, low := countReviewFindings(review.Findings)
	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Findings: %d total (%d critical / %d high / %d medium / %d low)",
		total, crit, high, med, low,
	)))

	if showPreview && total > 0 {
		limit := reviewFindingsPreviewMax
		if total < limit {
			limit = total
		}
		for i := 0; i < limit; i++ {
			f := review.Findings[i]
			blocks = append(blocks, paragraphBlock(fmt.Sprintf(
				"• [%s] %s: %s", f.Severity, f.Category, f.Message,
			)))
		}
		if total > reviewFindingsPreviewMax {
			blocks = append(blocks, paragraphBlock(fmt.Sprintf(
				"(%d more findings …)", total-reviewFindingsPreviewMax,
			)))
		}
	}

	if review.Summary != "" {
		summary := review.Summary
		if len(summary) > reviewSummaryMaxLen {
			summary = summary[:reviewSummaryMaxLen] + "…"
		}
		blocks = append(blocks, paragraphBlock("Summary: "+summary))
	}

	blocks = append(blocks, paragraphBlock(fmt.Sprintf(
		"Cost: $%.4f · Updated: %s",
		review.CostUSD, review.UpdatedAt.UTC().Format(time.RFC3339),
	)))

	return json.Marshal(blocks)
}

func countReviewFindings(findings []model.ReviewFinding) (total, critical, high, medium, low int) {
	total = len(findings)
	for _, f := range findings {
		switch f.Severity {
		case model.ReviewRiskLevelCritical:
			critical++
		case model.ReviewRiskLevelHigh:
			high++
		case model.ReviewRiskLevelMedium:
			medium++
		case model.ReviewRiskLevelLow:
			low++
		}
	}
	return
}
