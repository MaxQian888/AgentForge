package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// ReviewAggregationReviewLister lists reviews for a given task.
type ReviewAggregationReviewLister interface {
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.Review, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Review, error)
}

// ReviewAggregationRepository persists aggregation records.
type ReviewAggregationRepository interface {
	Create(ctx context.Context, agg *model.ReviewAggregation) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.ReviewAggregation, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.ReviewAggregation, error)
	Update(ctx context.Context, agg *model.ReviewAggregation) error
}

// FalsePositiveRepository manages false positive records for learning.
type FalsePositiveRepository interface {
	Create(ctx context.Context, fp *model.FalsePositive) error
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.FalsePositive, error)
	IncrementOccurrences(ctx context.Context, id uuid.UUID) error
}

// ReviewAggregationTaskLookup resolves task metadata from a task ID.
type ReviewAggregationTaskLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
}

// ReviewAggregationService wires existing aggregation and false positive repositories
// to deduplicate findings, suppress false positives, and produce an overall assessment.
type ReviewAggregationService struct {
	reviews      ReviewAggregationReviewLister
	aggregations ReviewAggregationRepository
	falsePosRepo FalsePositiveRepository
	tasks        ReviewAggregationTaskLookup
}

// NewReviewAggregationService creates a new aggregation service.
func NewReviewAggregationService(
	reviews ReviewAggregationReviewLister,
	aggregations ReviewAggregationRepository,
	falsePosRepo FalsePositiveRepository,
	tasks ReviewAggregationTaskLookup,
) *ReviewAggregationService {
	return &ReviewAggregationService{
		reviews:      reviews,
		aggregations: aggregations,
		falsePosRepo: falsePosRepo,
		tasks:        tasks,
	}
}

// findingKey produces a deduplication key for a review finding.
func findingKey(f model.ReviewFinding) string {
	return fmt.Sprintf("%s:%d:%s:%s", f.File, f.Line, f.Category, f.Message)
}

// riskRank returns a numeric rank for risk levels (higher = more severe).
func riskRank(level string) int {
	switch level {
	case model.ReviewRiskLevelCritical:
		return 4
	case model.ReviewRiskLevelHigh:
		return 3
	case model.ReviewRiskLevelMedium:
		return 2
	case model.ReviewRiskLevelLow:
		return 1
	default:
		return 0
	}
}

// recommendationRank returns a numeric rank (higher = more conservative).
func recommendationRank(rec string) int {
	switch rec {
	case model.ReviewRecommendationReject:
		return 3
	case model.ReviewRecommendationRequestChanges:
		return 2
	case model.ReviewRecommendationApprove:
		return 1
	default:
		return 0
	}
}

// matchesFalsePositive checks if a finding matches a known false positive pattern.
func matchesFalsePositive(f model.ReviewFinding, fps []*model.FalsePositive) *model.FalsePositive {
	for _, fp := range fps {
		if fp.Category != "" && fp.Category != f.Category {
			continue
		}
		if fp.Pattern != "" && !strings.Contains(f.Message, fp.Pattern) {
			continue
		}
		if fp.FilePattern != "" && !strings.Contains(f.File, fp.FilePattern) {
			continue
		}
		return fp
	}
	return nil
}

// Aggregate collects all reviews for a task, deduplicates findings, filters
// false positives, computes overall risk and recommendation, then creates or
// updates an aggregation record.
func (s *ReviewAggregationService) Aggregate(ctx context.Context, taskID uuid.UUID) (*model.ReviewAggregation, error) {
	reviews, err := s.reviews.GetByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list reviews for aggregation: %w", err)
	}
	if len(reviews) == 0 {
		return nil, fmt.Errorf("no reviews found for task %s", taskID)
	}

	// Resolve task to get project ID for false positive lookup.
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("get task for aggregation: %w", err)
	}

	// Load known false positives for this project.
	falsePositives, err := s.falsePosRepo.ListByProject(ctx, task.ProjectID)
	if err != nil {
		log.WithField("projectId", task.ProjectID.String()).WithError(err).Warn("failed to load false positives, proceeding without suppression")
		falsePositives = nil
	}

	// Collect review IDs, deduplicate findings, compute totals.
	reviewIDs := make([]uuid.UUID, 0, len(reviews))
	seen := make(map[string]struct{})
	var dedupedFindings []model.ReviewFinding
	var totalCost float64
	highestRisk := ""
	mostConservativeRec := ""
	prURL := ""

	for _, r := range reviews {
		if r.Status != model.ReviewStatusCompleted {
			continue
		}
		reviewIDs = append(reviewIDs, r.ID)
		totalCost += r.CostUSD

		if prURL == "" {
			prURL = r.PRURL
		}

		// Track highest risk.
		if riskRank(r.RiskLevel) > riskRank(highestRisk) {
			highestRisk = r.RiskLevel
		}

		// Track most conservative recommendation.
		if recommendationRank(r.Recommendation) > recommendationRank(mostConservativeRec) {
			mostConservativeRec = r.Recommendation
		}

		for _, f := range r.Findings {
			key := findingKey(f)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}

			// Suppress false positives.
			if fp := matchesFalsePositive(f, falsePositives); fp != nil {
				_ = s.falsePosRepo.IncrementOccurrences(ctx, fp.ID)
				log.WithFields(log.Fields{
					"finding":         key,
					"falsePositiveId": fp.ID.String(),
				}).Debug("finding suppressed as false positive")
				continue
			}
			dedupedFindings = append(dedupedFindings, f)
		}
	}

	if len(reviewIDs) == 0 {
		return nil, fmt.Errorf("no completed reviews for task %s", taskID)
	}

	if highestRisk == "" {
		highestRisk = model.ReviewRiskLevelLow
	}
	if mostConservativeRec == "" {
		mostConservativeRec = model.ReviewRecommendationApprove
	}

	findingsJSON, _ := json.Marshal(dedupedFindings)
	metrics, _ := json.Marshal(map[string]any{
		"totalReviews":    len(reviewIDs),
		"totalFindings":   len(dedupedFindings),
		"suppressedCount": len(seen) - len(dedupedFindings),
	})

	summary := fmt.Sprintf(
		"Aggregated %d reviews: %d unique findings (%d suppressed). Overall risk: %s, recommendation: %s",
		len(reviewIDs), len(dedupedFindings), len(seen)-len(dedupedFindings), highestRisk, mostConservativeRec,
	)

	now := time.Now().UTC()

	// Check for existing aggregation to update.
	existing, _ := s.aggregations.GetByTask(ctx, taskID)
	if len(existing) > 0 {
		agg := existing[0]
		agg.ReviewIDs = reviewIDs
		agg.OverallRisk = highestRisk
		agg.Recommendation = mostConservativeRec
		agg.Findings = string(findingsJSON)
		agg.Summary = summary
		agg.Metrics = string(metrics)
		agg.TotalCostUsd = totalCost
		agg.UpdatedAt = now

		if err := s.aggregations.Update(ctx, agg); err != nil {
			return nil, fmt.Errorf("update review aggregation: %w", err)
		}
		log.WithFields(log.Fields{
			"aggregationId": agg.ID.String(),
			"taskId":        taskID.String(),
			"reviewCount":   len(reviewIDs),
			"findingCount":  len(dedupedFindings),
		}).Info("review aggregation updated")
		return agg, nil
	}

	agg := &model.ReviewAggregation{
		ID:             uuid.New(),
		PRURL:          prURL,
		TaskID:         taskID,
		ReviewIDs:      reviewIDs,
		OverallRisk:    highestRisk,
		Recommendation: mostConservativeRec,
		Findings:       string(findingsJSON),
		Summary:        summary,
		Metrics:        string(metrics),
		TotalCostUsd:   totalCost,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.aggregations.Create(ctx, agg); err != nil {
		return nil, fmt.Errorf("create review aggregation: %w", err)
	}

	log.WithFields(log.Fields{
		"aggregationId": agg.ID.String(),
		"taskId":        taskID.String(),
		"reviewCount":   len(reviewIDs),
		"findingCount":  len(dedupedFindings),
	}).Info("review aggregation created")
	return agg, nil
}

// MarkFalsePositive extracts a finding from a review and creates a false positive
// record so that future aggregations suppress it.
func (s *ReviewAggregationService) MarkFalsePositive(ctx context.Context, reviewID uuid.UUID, findingIndex int, reason string) error {
	review, err := s.reviews.GetByID(ctx, reviewID)
	if err != nil {
		return fmt.Errorf("get review: %w", err)
	}

	if findingIndex < 0 || findingIndex >= len(review.Findings) {
		return fmt.Errorf("finding index %d out of range (0..%d)", findingIndex, len(review.Findings)-1)
	}

	finding := review.Findings[findingIndex]

	task, err := s.tasks.GetByID(ctx, review.TaskID)
	if err != nil {
		return fmt.Errorf("get task for false positive: %w", err)
	}

	fp := &model.FalsePositive{
		ID:          uuid.New(),
		ProjectID:   task.ProjectID,
		Pattern:     finding.Message,
		Category:    finding.Category,
		FilePattern: finding.File,
		Reason:      reason,
		Occurrences: 1,
		IsStrong:    false,
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	if err := s.falsePosRepo.Create(ctx, fp); err != nil {
		return fmt.Errorf("create false positive: %w", err)
	}

	log.WithFields(log.Fields{
		"reviewId":        reviewID.String(),
		"findingIndex":    findingIndex,
		"falsePositiveId": fp.ID.String(),
		"category":        finding.Category,
		"pattern":         finding.Message,
	}).Info("false positive recorded")
	return nil
}
