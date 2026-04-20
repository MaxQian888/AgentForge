package vcs

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/react-go-quick-starter/server/internal/model"
)

// Dispatcher posts review results back to the VCS host as PR comments.
// Initial reviews get a fresh summary + inline comments; incremental
// reviews follow the stale-findings annotation policy (Spec 2 §10).
type Dispatcher struct {
	reviews  DispatcherReviewRepo
	vcs      Provider
	registry *Registry
}

// DispatcherReviewRepo is the narrow persistence contract the dispatcher needs.
type DispatcherReviewRepo interface {
	GetByID(ctx context.Context, id interface{}) (*model.Review, error)
}

// NewDispatcher constructs an outbound dispatcher.
func NewDispatcher(reviews DispatcherReviewRepo, registry *Registry) *Dispatcher {
	return &Dispatcher{reviews: reviews, registry: registry}
}

// OnReviewCompleted is the entry point called when a review reaches "completed".
func (d *Dispatcher) OnReviewCompleted(ctx context.Context, review *model.Review, provider Provider) error {
	if review.ParentReviewID != nil {
		return d.handleIncremental(ctx, review, provider)
	}
	return d.handleInitial(ctx, review, provider)
}

func (d *Dispatcher) handleInitial(ctx context.Context, review *model.Review, provider Provider) error {
	if review.PRURL == "" {
		return nil
	}
	pr := prFromReview(review)
	repo := RepoRef{} // Would be resolved from integration in production.

	// Post summary comment.
	body := renderSummary(review)
	commentID, err := provider.PostSummaryComment(ctx, pr, body)
	if err != nil {
		return fmt.Errorf("post summary comment: %w", err)
	}
	review.SummaryCommentID = commentID

	// Post inline comments for findings.
	comments := toInlineComments(review.Findings)
	if len(comments) > 0 {
		ids, err := provider.PostReviewComments(ctx, pr, comments)
		if err != nil {
			return fmt.Errorf("post review comments: %w", err)
		}
		for i, id := range ids {
			if i < len(review.Findings) {
				review.Findings[i].InlineCommentID = id
			}
		}
	}

	_ = repo // suppress unused
	return nil
}

func (d *Dispatcher) handleIncremental(ctx context.Context, review *model.Review, provider Provider) error {
	parent, err := d.reviews.GetByID(ctx, *review.ParentReviewID)
	if err != nil {
		return fmt.Errorf("load parent review: %w", err)
	}

	pr := prFromReview(review)
	changedSet := newStringSet(changedFilesFromReview(review))

	parentByID := indexFindingsByID(parent.Findings)
	currentByID := indexFindingsByID(review.Findings)

	// 1. New findings → post inline comments.
	var fresh []model.ReviewFinding
	for _, f := range review.Findings {
		if f.ID == "" {
			fresh = append(fresh, f)
			continue
		}
		if _, existed := parentByID[f.ID]; !existed {
			fresh = append(fresh, f)
		}
	}
	if len(fresh) > 0 {
		comments := toInlineComments(fresh)
		if _, err := provider.PostReviewComments(ctx, pr, comments); err != nil {
			return fmt.Errorf("post incremental comments: %w", err)
		}
	}

	// 2. Stale findings → annotate as resolved (never delete).
	for _, f := range parent.Findings {
		if f.ID == "" {
			continue
		}
		if _, stillThere := currentByID[f.ID]; stillThere {
			continue
		}
		if !changedSet.Has(f.File) {
			continue // File unchanged — leave comment as-is.
		}
		if f.InlineCommentID == "" {
			continue
		}
		body := f.Message + fmt.Sprintf("\n\n_(已修复或被 superseded — review #%s)_", shortUUID(review.ID))
		if err := provider.EditReviewComment(ctx, pr, f.InlineCommentID, body); err != nil {
			log.WithError(err).WithField("findingId", f.ID).Warn("vcs:edit_review_comment_failed")
		}
	}

	// 3. Update summary comment.
	if parent.SummaryCommentID != "" {
		if err := provider.EditSummaryComment(ctx, pr, parent.SummaryCommentID, renderSummary(review)); err != nil {
			return fmt.Errorf("edit summary comment: %w", err)
		}
	}

	return nil
}

func prFromReview(review *model.Review) *PullRequest {
	return &PullRequest{
		Number: review.PRNumber,
		URL:    review.PRURL,
	}
}

func renderSummary(review *model.Review) string {
	return fmt.Sprintf("## AgentForge Review #%s\n\n**Risk:** %s | **Recommendation:** %s\n\n%s\n\nFindings: %d",
		shortUUID(review.ID), review.RiskLevel, review.Recommendation, review.Summary, len(review.Findings))
}

func toInlineComments(findings []model.ReviewFinding) []InlineComment {
	var comments []InlineComment
	for _, f := range findings {
		if f.File == "" {
			continue
		}
		comments = append(comments, InlineComment{
			Path: f.File,
			Line: f.Line,
			Body: fmt.Sprintf("**[%s]** %s\n\n%s", f.Severity, f.Message, f.Suggestion),
		})
	}
	return comments
}

func changedFilesFromReview(review *model.Review) []string {
	if review.ExecutionMetadata != nil && len(review.ExecutionMetadata.ChangedFiles) > 0 {
		return review.ExecutionMetadata.ChangedFiles
	}
	return nil
}

func indexFindingsByID(findings []model.ReviewFinding) map[string]model.ReviewFinding {
	m := make(map[string]model.ReviewFinding, len(findings))
	for _, f := range findings {
		if f.ID != "" {
			m[f.ID] = f
		}
	}
	return m
}

type stringSet map[string]struct{}

func newStringSet(items []string) stringSet {
	s := make(stringSet, len(items))
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}

func (s stringSet) Has(item string) bool {
	_, ok := s[item]
	return ok
}

func shortUUID(id interface{}) string {
	s := fmt.Sprint(id)
	if len(s) > 8 {
		return s[:8]
	}
	return s
}
