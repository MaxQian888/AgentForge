package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	eventbus "github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/ws"
	"github.com/google/uuid"
)

type taskCommentRepository interface {
	Create(ctx context.Context, comment *model.TaskComment) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.TaskComment, error)
	Update(ctx context.Context, comment *model.TaskComment) error
	SoftDelete(ctx context.Context, id uuid.UUID) error
	ListByTaskID(ctx context.Context, taskID uuid.UUID) ([]*model.TaskComment, error)
}

type taskCommentMemberRepository interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Member, error)
}

type taskCommentNotificationCreator interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type CreateTaskCommentInput struct {
	ProjectID uuid.UUID
	TaskID    uuid.UUID
	Body      string
	CreatedBy uuid.UUID
}

type TaskCommentService struct {
	repo     taskCommentRepository
	members  taskCommentMemberRepository
	notifier taskCommentNotificationCreator
	tasks    entityLinkTaskReader
	hub      *ws.Hub
	bus      eventbus.Publisher
}

func NewTaskCommentService(repo taskCommentRepository, members taskCommentMemberRepository, notifier taskCommentNotificationCreator, tasks ...entityLinkTaskReader) *TaskCommentService {
	service := &TaskCommentService{repo: repo, members: members, notifier: notifier}
	if len(tasks) > 0 {
		service.tasks = tasks[0]
	}
	return service
}

func (s *TaskCommentService) WithHub(hub *ws.Hub) *TaskCommentService {
	s.hub = hub
	return s
}

func (s *TaskCommentService) WithBus(bus eventbus.Publisher) *TaskCommentService {
	s.bus = bus
	return s
}

func (s *TaskCommentService) CreateComment(ctx context.Context, input *CreateTaskCommentInput) (*model.TaskComment, error) {
	comment := &model.TaskComment{
		ID:        uuid.New(),
		TaskID:    input.TaskID,
		Body:      strings.TrimSpace(input.Body),
		Mentions:  ExtractTaskCommentMentions(input.Body),
		CreatedBy: input.CreatedBy,
	}
	if err := s.repo.Create(ctx, comment); err != nil {
		return nil, fmt.Errorf("create task comment: %w", err)
	}
	s.notifyMentions(ctx, input.ProjectID, input.TaskID, comment)
	s.broadcast(ctx, input.ProjectID, ws.EventTaskCommentCreated, comment.ToDTO())
	return comment, nil
}

func (s *TaskCommentService) ReplyToComment(ctx context.Context, parentCommentID uuid.UUID, input *CreateTaskCommentInput) (*model.TaskComment, error) {
	parent, err := s.repo.GetByID(ctx, parentCommentID)
	if err != nil {
		return nil, fmt.Errorf("get parent task comment: %w", err)
	}
	comment, err := s.CreateComment(ctx, input)
	if err != nil {
		return nil, err
	}
	comment.ParentCommentID = &parent.ID
	if err := s.repo.Update(ctx, comment); err != nil {
		return nil, fmt.Errorf("update replied task comment: %w", err)
	}
	return comment, nil
}

func (s *TaskCommentService) ResolveComment(ctx context.Context, commentID uuid.UUID) (*model.TaskComment, error) {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("get task comment: %w", err)
	}
	now := time.Now().UTC()
	comment.ResolvedAt = &now
	if err := s.repo.Update(ctx, comment); err != nil {
		return nil, fmt.Errorf("resolve task comment: %w", err)
	}
	s.broadcast(ctx, s.projectIDForTask(ctx, comment.TaskID), ws.EventTaskCommentResolved, map[string]any{
		"id":       comment.ID.String(),
		"taskId":   comment.TaskID.String(),
		"resolved": true,
	})
	return comment, nil
}

func (s *TaskCommentService) ReopenComment(ctx context.Context, commentID uuid.UUID) (*model.TaskComment, error) {
	comment, err := s.repo.GetByID(ctx, commentID)
	if err != nil {
		return nil, fmt.Errorf("get task comment: %w", err)
	}
	comment.ResolvedAt = nil
	if err := s.repo.Update(ctx, comment); err != nil {
		return nil, fmt.Errorf("reopen task comment: %w", err)
	}
	s.broadcast(ctx, s.projectIDForTask(ctx, comment.TaskID), ws.EventTaskCommentResolved, map[string]any{
		"id":       comment.ID.String(),
		"taskId":   comment.TaskID.String(),
		"resolved": false,
	})
	return comment, nil
}

func (s *TaskCommentService) DeleteComment(ctx context.Context, commentID uuid.UUID) error {
	if err := s.repo.SoftDelete(ctx, commentID); err != nil {
		return fmt.Errorf("delete task comment: %w", err)
	}
	return nil
}

func (s *TaskCommentService) ListComments(ctx context.Context, taskID uuid.UUID) ([]*model.TaskComment, error) {
	comments, err := s.repo.ListByTaskID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list task comments: %w", err)
	}
	return comments, nil
}

var taskCommentMentionPattern = regexp.MustCompile(`(?:^|[^A-Za-z0-9_@])@([A-Za-z0-9][A-Za-z0-9_-]*)`)

func ExtractTaskCommentMentions(body string) []string {
	matches := taskCommentMentionPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(matches))
	mentions := make([]string, 0, len(matches))
	for _, match := range matches {
		name := strings.ToLower(strings.TrimSpace(match[1]))
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		mentions = append(mentions, name)
	}
	return mentions
}

func (s *TaskCommentService) notifyMentions(ctx context.Context, projectID uuid.UUID, taskID uuid.UUID, comment *model.TaskComment) {
	if s.notifier == nil || s.members == nil || comment == nil || len(comment.Mentions) == 0 {
		return
	}
	members, err := s.members.ListByProject(ctx, projectID)
	if err != nil {
		return
	}
	for _, mention := range comment.Mentions {
		for _, member := range members {
			if member == nil || !strings.EqualFold(strings.TrimSpace(member.Name), mention) {
				continue
			}
			_, _ = s.notifier.Create(
				ctx,
				member.ID,
				model.NotificationTypeTaskCommentMention,
				"Task comment mention",
				comment.Body,
				fmt.Sprintf(`{"href":"/tasks/%s#comment-%s"}`, taskID.String(), comment.ID.String()),
			)
		}
	}
}

func (s *TaskCommentService) projectIDForTask(ctx context.Context, taskID uuid.UUID) uuid.UUID {
	if s.tasks == nil {
		return uuid.Nil
	}
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil || task == nil {
		return uuid.Nil
	}
	return task.ProjectID
}

func (s *TaskCommentService) broadcast(ctx context.Context, projectID uuid.UUID, eventType string, payload any) {
	projectIDStr := ""
	if projectID != uuid.Nil {
		projectIDStr = projectID.String()
	}
	_ = eventbus.PublishLegacy(ctx, s.bus, eventType, projectIDStr, payload)
}
