package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

// AssignmentCandidate represents a scored member recommendation for task assignment.
type AssignmentCandidate struct {
	MemberID    uuid.UUID `json:"memberId"`
	MemberName  string    `json:"memberName"`
	MemberType  string    `json:"memberType"` // agent/human
	Score       float64   `json:"score"`
	Reasons     []string  `json:"reasons"`
	CurrentLoad int       `json:"currentLoad"`
}

// MemberLister lists project members.
type MemberLister interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error)
}

// ActiveRunCounter counts active agent runs per member.
type ActiveRunCounter interface {
	ListActive(ctx context.Context) ([]*model.AgentRun, error)
}

// AssignmentRecommender scores project members for a given task.
type AssignmentRecommender struct {
	tasks   TaskRepository
	members MemberLister
	runs    ActiveRunCounter
}

// NewAssignmentRecommender creates a new recommender.
func NewAssignmentRecommender(tasks TaskRepository, members MemberLister, runs ActiveRunCounter) *AssignmentRecommender {
	return &AssignmentRecommender{
		tasks:   tasks,
		members: members,
		runs:    runs,
	}
}

// Recommend returns the top 3 assignment candidates for the given task.
func (r *AssignmentRecommender) Recommend(ctx context.Context, taskID uuid.UUID) ([]AssignmentCandidate, error) {
	task, err := r.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("fetch task: %w", err)
	}

	members, err := r.members.ListByProject(ctx, task.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}

	// Build a map of memberID -> active run count.
	activeRuns, err := r.runs.ListActive(ctx)
	if err != nil {
		// Non-fatal: proceed with zero load if unavailable.
		activeRuns = nil
	}
	loadMap := make(map[uuid.UUID]int, len(members))
	for _, run := range activeRuns {
		loadMap[run.MemberID]++
	}

	// Build label set for quick lookup.
	labelSet := make(map[string]struct{}, len(task.Labels))
	for _, l := range task.Labels {
		labelSet[l] = struct{}{}
	}

	candidates := make([]AssignmentCandidate, 0, len(members))
	for _, m := range members {
		if !m.IsActive {
			continue
		}

		score := 1.0
		var reasons []string

		// Load penalty: -0.1 per active run.
		load := loadMap[m.ID]
		if load > 0 {
			penalty := float64(load) * 0.1
			score -= penalty
			reasons = append(reasons, fmt.Sprintf("-%.1f load (%d active runs)", penalty, load))
		} else {
			reasons = append(reasons, "no active runs (idle)")
		}

		// Role match: check if member role appears in task labels.
		if m.Role != "" {
			if _, ok := labelSet[m.Role]; ok {
				score += 0.2
				reasons = append(reasons, fmt.Sprintf("+0.2 role match (%s)", m.Role))
			}
		}

		// Skill match: check if any member skill appears in task labels.
		for _, skill := range m.Skills {
			if _, ok := labelSet[skill]; ok {
				score += 0.2
				reasons = append(reasons, fmt.Sprintf("+0.2 skill match (%s)", skill))
				break // count skill match once
			}
		}

		// Agent type bonus when task has "execution:agent" label.
		if _, ok := labelSet["execution:agent"]; ok && m.Type == model.MemberTypeAgent {
			score += 0.1
			reasons = append(reasons, "+0.1 agent type preferred")
		}

		candidates = append(candidates, AssignmentCandidate{
			MemberID:    m.ID,
			MemberName:  m.Name,
			MemberType:  m.Type,
			Score:       score,
			Reasons:     reasons,
			CurrentLoad: load,
		})
	}

	// Sort descending by score, then by name for stability.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].MemberName < candidates[j].MemberName
	})

	// Return top 3.
	if len(candidates) > 3 {
		candidates = candidates[:3]
	}
	return candidates, nil
}
