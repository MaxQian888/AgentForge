package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

var ErrFormRateLimited = errors.New("form submission rate limited")

type formRepository interface {
	CreateDefinition(ctx context.Context, form *model.FormDefinition) error
	GetDefinition(ctx context.Context, id uuid.UUID) (*model.FormDefinition, error)
	GetBySlug(ctx context.Context, slug string) (*model.FormDefinition, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.FormDefinition, error)
	UpdateDefinition(ctx context.Context, form *model.FormDefinition) error
	DeleteDefinition(ctx context.Context, id uuid.UUID) error
	CreateSubmission(ctx context.Context, submission *model.FormSubmission) error
}

type formTaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
}

type formCustomFieldValueWriter interface {
	SetValue(ctx context.Context, value *model.CustomFieldValue) error
}

type FormSubmissionInput struct {
	SubmittedBy string
	IPAddress   string
	Values      map[string]string
}

type formFieldMapping struct {
	Key    string `json:"key"`
	Target string `json:"target"`
}

type FormService struct {
	repo            formRepository
	tasks           formTaskRepository
	customFieldRepo formCustomFieldValueWriter
	now             func() time.Time
	mu              sync.Mutex
	submissionsByIP map[string][]time.Time
}

func NewFormService(repo formRepository, tasks formTaskRepository, customFieldRepo formCustomFieldValueWriter) *FormService {
	return &FormService{
		repo:            repo,
		tasks:           tasks,
		customFieldRepo: customFieldRepo,
		now:             func() time.Time { return time.Now().UTC() },
		submissionsByIP: map[string][]time.Time{},
	}
}

func (s *FormService) CreateForm(ctx context.Context, form *model.FormDefinition) error {
	now := s.now()
	if form.ID == uuid.Nil {
		form.ID = uuid.New()
	}
	if form.CreatedAt.IsZero() {
		form.CreatedAt = now
	}
	form.UpdatedAt = now
	return s.repo.CreateDefinition(ctx, form)
}

func (s *FormService) GetForm(ctx context.Context, id uuid.UUID) (*model.FormDefinition, error) {
	return s.repo.GetDefinition(ctx, id)
}

func (s *FormService) ListForms(ctx context.Context, projectID uuid.UUID) ([]*model.FormDefinition, error) {
	return s.repo.ListByProject(ctx, projectID)
}

func (s *FormService) UpdateForm(ctx context.Context, form *model.FormDefinition) error {
	form.UpdatedAt = s.now()
	return s.repo.UpdateDefinition(ctx, form)
}

func (s *FormService) DeleteForm(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteDefinition(ctx, id)
}

func (s *FormService) GetFormBySlug(ctx context.Context, slug string) (*model.FormDefinition, error) {
	return s.repo.GetBySlug(ctx, slug)
}

func (s *FormService) SubmitForm(ctx context.Context, slug string, input FormSubmissionInput) (*model.Task, error) {
	form, err := s.repo.GetBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("get form by slug: %w", err)
	}
	if form == nil {
		return nil, fmt.Errorf("form %q not found", slug)
	}
	if form.IsPublic {
		if err := s.checkRateLimit(input.IPAddress); err != nil {
			return nil, err
		}
	}

	mappings, err := parseFormMappings(form.Fields)
	if err != nil {
		return nil, fmt.Errorf("parse form mappings: %w", err)
	}

	task := &model.Task{
		ID:          uuid.New(),
		ProjectID:   form.ProjectID,
		Status:      defaultString(form.TargetStatus, model.TaskStatusInbox),
		Priority:    "medium",
		Description: "",
		Labels:      []string{"origin:form"},
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}
	if form.TargetAssignee != nil {
		task.AssigneeID = form.TargetAssignee
		task.AssigneeType = model.MemberTypeHuman
	}

	customFieldValues := make([]*model.CustomFieldValue, 0)
	for _, mapping := range mappings {
		value := input.Values[mapping.Key]
		switch {
		case mapping.Target == "title":
			task.Title = value
		case mapping.Target == "description":
			task.Description = value
		case mapping.Target == "priority":
			task.Priority = defaultString(value, "medium")
		case strings.HasPrefix(mapping.Target, "cf:"):
			fieldID, parseErr := uuid.Parse(strings.TrimPrefix(mapping.Target, "cf:"))
			if parseErr != nil {
				return nil, fmt.Errorf("parse custom field target: %w", parseErr)
			}
			customFieldValues = append(customFieldValues, &model.CustomFieldValue{
				ID:         uuid.New(),
				TaskID:     task.ID,
				FieldDefID: fieldID,
				Value:      fmt.Sprintf("%q", value),
				CreatedAt:  s.now(),
				UpdatedAt:  s.now(),
			})
		}
	}
	if strings.TrimSpace(task.Title) == "" {
		return nil, errors.New("form submission missing required title mapping")
	}
	if err := s.tasks.Create(ctx, task); err != nil {
		return nil, fmt.Errorf("create task from form: %w", err)
	}
	for _, value := range customFieldValues {
		if err := s.customFieldRepo.SetValue(ctx, value); err != nil {
			return nil, fmt.Errorf("persist custom field value: %w", err)
		}
	}
	if err := s.repo.CreateSubmission(ctx, &model.FormSubmission{
		ID:          uuid.New(),
		FormID:      form.ID,
		TaskID:      task.ID,
		SubmittedBy: input.SubmittedBy,
		SubmittedAt: s.now(),
		IPAddress:   input.IPAddress,
	}); err != nil {
		return nil, fmt.Errorf("create form submission: %w", err)
	}
	return task, nil
}

func (s *FormService) checkRateLimit(ipAddress string) error {
	ip := strings.TrimSpace(ipAddress)
	if ip == "" {
		ip = "unknown"
	}
	now := s.now()
	windowStart := now.Add(-time.Minute)

	s.mu.Lock()
	defer s.mu.Unlock()

	timestamps := s.submissionsByIP[ip][:0]
	for _, timestamp := range s.submissionsByIP[ip] {
		if timestamp.After(windowStart) {
			timestamps = append(timestamps, timestamp)
		}
	}
	if len(timestamps) >= 10 {
		s.submissionsByIP[ip] = timestamps
		return ErrFormRateLimited
	}
	s.submissionsByIP[ip] = append(timestamps, now)
	return nil
}

func parseFormMappings(raw string) ([]formFieldMapping, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var mappings []formFieldMapping
	if err := json.Unmarshal([]byte(raw), &mappings); err != nil {
		return nil, err
	}
	return mappings, nil
}

func defaultString(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
