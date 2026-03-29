package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type assignmentTaskRepoStub struct {
	task *model.Task
}

func (s *assignmentTaskRepoStub) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if s.task == nil || s.task.ID != id {
		return nil, service.ErrAgentTaskNotFound
	}
	cloned := *s.task
	return &cloned, nil
}

func (s *assignmentTaskRepoStub) Create(context.Context, *model.Task) error {
	panic("unexpected Create call")
}

func (s *assignmentTaskRepoStub) Update(context.Context, uuid.UUID, *model.UpdateTaskRequest) error {
	panic("unexpected Update call")
}

func (s *assignmentTaskRepoStub) Delete(context.Context, uuid.UUID) error {
	panic("unexpected Delete call")
}

func (s *assignmentTaskRepoStub) List(context.Context, uuid.UUID, model.TaskListQuery) ([]*model.Task, int, error) {
	panic("unexpected ListByProject call")
}

func (s *assignmentTaskRepoStub) TransitionStatus(context.Context, uuid.UUID, string) error {
	panic("unexpected TransitionStatus call")
}

func (s *assignmentTaskRepoStub) UpdateAssignee(context.Context, uuid.UUID, uuid.UUID, string) error {
	panic("unexpected UpdateAssignee call")
}

type assignmentMemberRepoStub struct {
	members []*model.Member
}

func (s *assignmentMemberRepoStub) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.Member, error) {
	out := make([]*model.Member, 0, len(s.members))
	for _, member := range s.members {
		cloned := *member
		out = append(out, &cloned)
	}
	return out, nil
}

type assignmentRunRepoStub struct {
	runs []*model.AgentRun
}

func (s *assignmentRunRepoStub) ListActive(_ context.Context) ([]*model.AgentRun, error) {
	out := make([]*model.AgentRun, 0, len(s.runs))
	for _, run := range s.runs {
		cloned := *run
		out = append(out, &cloned)
	}
	return out, nil
}

func TestAssignmentRecommender_RecommendScoresAndLimitsCandidates(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	alphaID := uuid.New()
	bravoID := uuid.New()
	charlieID := uuid.New()
	deltaID := uuid.New()

	recommender := service.NewAssignmentRecommender(
		&assignmentTaskRepoStub{task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Labels:    []string{"frontend", "react", "execution:agent"},
		}},
		&assignmentMemberRepoStub{members: []*model.Member{
			{ID: alphaID, ProjectID: projectID, Name: "Alpha", Type: model.MemberTypeAgent, Role: "frontend", Skills: []string{"react"}, IsActive: true},
			{ID: bravoID, ProjectID: projectID, Name: "Bravo", Type: model.MemberTypeHuman, Role: "frontend", Skills: []string{"react"}, IsActive: true},
			{ID: charlieID, ProjectID: projectID, Name: "Charlie", Type: model.MemberTypeAgent, Skills: []string{"react"}, IsActive: true},
			{ID: deltaID, ProjectID: projectID, Name: "Delta", Type: model.MemberTypeAgent, Skills: []string{"react"}, IsActive: true},
		}},
		&assignmentRunRepoStub{runs: []*model.AgentRun{
			{ID: uuid.New(), TaskID: uuid.New(), MemberID: charlieID, Status: model.AgentRunStatusRunning},
			{ID: uuid.New(), TaskID: uuid.New(), MemberID: charlieID, Status: model.AgentRunStatusRunning},
			{ID: uuid.New(), TaskID: uuid.New(), MemberID: deltaID, Status: model.AgentRunStatusRunning},
		}},
	)

	candidates, err := recommender.Recommend(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("len(candidates) = %d, want 3", len(candidates))
	}

	if candidates[0].MemberID != alphaID || candidates[0].Score != 1.5 {
		t.Fatalf("top candidate = %+v, want Alpha score 1.5", candidates[0])
	}
	if candidates[1].MemberID != bravoID || candidates[1].Score != 1.4 {
		t.Fatalf("second candidate = %+v, want Bravo score 1.4", candidates[1])
	}
	if candidates[2].MemberID != deltaID || candidates[2].CurrentLoad != 1 {
		t.Fatalf("third candidate = %+v, want Delta with load 1", candidates[2])
	}
}

func TestAssignmentRecommender_RecommendHandlesEdgeCases(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()

	recommender := service.NewAssignmentRecommender(
		&assignmentTaskRepoStub{task: &model.Task{
			ID:        taskID,
			ProjectID: projectID,
			Labels:    []string{"backend"},
		}},
		&assignmentMemberRepoStub{members: []*model.Member{
			{ID: uuid.New(), ProjectID: projectID, Name: "Inactive", Type: model.MemberTypeAgent, Skills: []string{"backend"}, IsActive: false},
			{ID: uuid.New(), ProjectID: projectID, Name: "Mismatch", Type: model.MemberTypeHuman, Skills: []string{"design"}, IsActive: true},
		}},
		&assignmentRunRepoStub{},
	)

	candidates, err := recommender.Recommend(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("len(candidates) = %d, want 1 active candidate", len(candidates))
	}
	if candidates[0].MemberName != "Mismatch" || candidates[0].Score != 1.0 {
		t.Fatalf("edge-case candidate = %+v, want active fallback score 1.0", candidates[0])
	}

	emptyRecommender := service.NewAssignmentRecommender(
		&assignmentTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID}},
		&assignmentMemberRepoStub{},
		&assignmentRunRepoStub{},
	)
	emptyCandidates, err := emptyRecommender.Recommend(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Recommend() empty error = %v", err)
	}
	if len(emptyCandidates) != 0 {
		t.Fatalf("len(emptyCandidates) = %d, want 0", len(emptyCandidates))
	}
}

func TestAssignmentRecommender_RecommendUsesStableTieBreak(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()

	recommender := service.NewAssignmentRecommender(
		&assignmentTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID}},
		&assignmentMemberRepoStub{members: []*model.Member{
			{ID: uuid.New(), ProjectID: projectID, Name: "Zulu", Type: model.MemberTypeAgent, IsActive: true},
			{ID: uuid.New(), ProjectID: projectID, Name: "Alpha", Type: model.MemberTypeAgent, IsActive: true},
		}},
		&assignmentRunRepoStub{},
	)

	candidates, err := recommender.Recommend(context.Background(), taskID)
	if err != nil {
		t.Fatalf("Recommend() error = %v", err)
	}
	if len(candidates) != 2 {
		t.Fatalf("len(candidates) = %d, want 2", len(candidates))
	}
	if candidates[0].MemberName != "Alpha" || candidates[1].MemberName != "Zulu" {
		t.Fatalf("tie-break order = [%s %s], want [Alpha Zulu]", candidates[0].MemberName, candidates[1].MemberName)
	}
}
