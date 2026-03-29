package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type userRecord struct {
	ID        uuid.UUID `gorm:"column:id;primaryKey"`
	Email     string    `gorm:"column:email"`
	Password  string    `gorm:"column:password"`
	Name      string    `gorm:"column:name"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (userRecord) TableName() string { return "users" }

func newUserRecord(user *model.User) *userRecord {
	if user == nil {
		return nil
	}
	return &userRecord{
		ID:        user.ID,
		Email:     user.Email,
		Password:  user.Password,
		Name:      user.Name,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func (r *userRecord) toModel() *model.User {
	if r == nil {
		return nil
	}
	return &model.User{
		ID:        r.ID,
		Email:     r.Email,
		Password:  r.Password,
		Name:      r.Name,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

type projectRecord struct {
	ID            uuid.UUID `gorm:"column:id;primaryKey"`
	Name          string    `gorm:"column:name"`
	Slug          string    `gorm:"column:slug"`
	Description   string    `gorm:"column:description"`
	RepoURL       string    `gorm:"column:repo_url"`
	DefaultBranch string    `gorm:"column:default_branch"`
	Settings      jsonText  `gorm:"column:settings;type:jsonb"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (projectRecord) TableName() string { return "projects" }

func newProjectRecord(project *model.Project) *projectRecord {
	if project == nil {
		return nil
	}
	return &projectRecord{
		ID:            project.ID,
		Name:          project.Name,
		Slug:          project.Slug,
		Description:   project.Description,
		RepoURL:       project.RepoURL,
		DefaultBranch: project.DefaultBranch,
		Settings:      newJSONText(project.Settings, "{}"),
		CreatedAt:     project.CreatedAt,
		UpdatedAt:     project.UpdatedAt,
	}
}

func (r *projectRecord) toModel() *model.Project {
	if r == nil {
		return nil
	}
	return &model.Project{
		ID:            r.ID,
		Name:          r.Name,
		Slug:          r.Slug,
		Description:   r.Description,
		RepoURL:       r.RepoURL,
		DefaultBranch: r.DefaultBranch,
		Settings:      r.Settings.String("{}"),
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

type memberRecord struct {
	ID          uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID   uuid.UUID  `gorm:"column:project_id"`
	UserID      *uuid.UUID `gorm:"column:user_id"`
	Name        string     `gorm:"column:name"`
	Type        string     `gorm:"column:type"`
	Role        string     `gorm:"column:role"`
	Status      string     `gorm:"column:status"`
	Email       string     `gorm:"column:email"`
	IMPlatform  string     `gorm:"column:im_platform"`
	IMUserID    string     `gorm:"column:im_user_id"`
	AvatarURL   string     `gorm:"column:avatar_url"`
	AgentConfig jsonText   `gorm:"column:agent_config;type:jsonb"`
	Skills      stringList `gorm:"column:skills;type:text[]"`
	IsActive    bool       `gorm:"column:is_active"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (memberRecord) TableName() string { return "members" }

func newMemberRecord(member *model.Member) *memberRecord {
	if member == nil {
		return nil
	}
	status := model.NormalizeMemberStatus(member.Status, member.IsActive)
	return &memberRecord{
		ID:          member.ID,
		ProjectID:   member.ProjectID,
		UserID:      member.UserID,
		Name:        member.Name,
		Type:        member.Type,
		Role:        member.Role,
		Status:      status,
		Email:       member.Email,
		IMPlatform:  member.IMPlatform,
		IMUserID:    member.IMUserID,
		AvatarURL:   member.AvatarURL,
		AgentConfig: newJSONText(member.AgentConfig, "{}"),
		Skills:      newStringList(member.Skills),
		IsActive:    model.IsMemberStatusActive(status),
		CreatedAt:   member.CreatedAt,
		UpdatedAt:   member.UpdatedAt,
	}
}

func (r *memberRecord) toModel() *model.Member {
	if r == nil {
		return nil
	}
	status := model.NormalizeMemberStatus(r.Status, r.IsActive)
	return &model.Member{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		UserID:      r.UserID,
		Name:        r.Name,
		Type:        r.Type,
		Role:        r.Role,
		Status:      status,
		Email:       r.Email,
		IMPlatform:  r.IMPlatform,
		IMUserID:    r.IMUserID,
		AvatarURL:   r.AvatarURL,
		AgentConfig: r.AgentConfig.String("{}"),
		Skills:      r.Skills.Slice(),
		IsActive:    model.IsMemberStatusActive(status),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type sprintRecord struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id"`
	Name           string     `gorm:"column:name"`
	StartDate      time.Time  `gorm:"column:start_date"`
	EndDate        time.Time  `gorm:"column:end_date"`
	MilestoneID    *uuid.UUID `gorm:"column:milestone_id"`
	Status         string     `gorm:"column:status"`
	TotalBudgetUsd float64    `gorm:"column:total_budget_usd"`
	SpentUsd       float64    `gorm:"column:spent_usd"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (sprintRecord) TableName() string { return "sprints" }

func newSprintRecord(sprint *model.Sprint) *sprintRecord {
	if sprint == nil {
		return nil
	}
	return &sprintRecord{
		ID:             sprint.ID,
		ProjectID:      sprint.ProjectID,
		Name:           sprint.Name,
		StartDate:      sprint.StartDate,
		EndDate:        sprint.EndDate,
		MilestoneID:    cloneUUIDPointer(sprint.MilestoneID),
		Status:         sprint.Status,
		TotalBudgetUsd: sprint.TotalBudgetUsd,
		SpentUsd:       sprint.SpentUsd,
		CreatedAt:      sprint.CreatedAt,
		UpdatedAt:      sprint.UpdatedAt,
	}
}

func (r *sprintRecord) toModel() *model.Sprint {
	if r == nil {
		return nil
	}
	return &model.Sprint{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		Name:           r.Name,
		StartDate:      r.StartDate,
		EndDate:        r.EndDate,
		MilestoneID:    cloneUUIDPointer(r.MilestoneID),
		Status:         r.Status,
		TotalBudgetUsd: r.TotalBudgetUsd,
		SpentUsd:       r.SpentUsd,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

type notificationRecord struct {
	ID        uuid.UUID `gorm:"column:id;primaryKey"`
	TargetID  uuid.UUID `gorm:"column:target_id"`
	Type      string    `gorm:"column:type"`
	Title     string    `gorm:"column:title"`
	Body      string    `gorm:"column:body"`
	Data      jsonText  `gorm:"column:data;type:jsonb"`
	IsRead    bool      `gorm:"column:is_read"`
	Channel   string    `gorm:"column:channel"`
	Sent      bool      `gorm:"column:sent"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (notificationRecord) TableName() string { return "notifications" }

func newNotificationRecord(notification *model.Notification) *notificationRecord {
	if notification == nil {
		return nil
	}
	return &notificationRecord{
		ID:        notification.ID,
		TargetID:  notification.TargetID,
		Type:      notification.Type,
		Title:     notification.Title,
		Body:      notification.Body,
		Data:      newJSONText(notification.Data, "{}"),
		IsRead:    notification.IsRead,
		Channel:   notification.Channel,
		Sent:      notification.Sent,
		CreatedAt: notification.CreatedAt,
	}
}

func (r *notificationRecord) toModel() *model.Notification {
	if r == nil {
		return nil
	}
	return &model.Notification{
		ID:        r.ID,
		TargetID:  r.TargetID,
		Type:      r.Type,
		Title:     r.Title,
		Body:      r.Body,
		Data:      r.Data.String("{}"),
		IsRead:    r.IsRead,
		Channel:   r.Channel,
		Sent:      r.Sent,
		CreatedAt: r.CreatedAt,
	}
}

type workflowConfigRecord struct {
	ID          uuid.UUID `gorm:"column:id;primaryKey"`
	ProjectID   uuid.UUID `gorm:"column:project_id"`
	Transitions rawJSON   `gorm:"column:transitions;type:jsonb"`
	Triggers    rawJSON   `gorm:"column:triggers;type:jsonb"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (workflowConfigRecord) TableName() string { return "workflow_configs" }

func newWorkflowConfigRecord(projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) *workflowConfigRecord {
	return &workflowConfigRecord{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Transitions: newRawJSON(transitions, "{}"),
		Triggers:    newRawJSON(triggers, "[]"),
	}
}

func (r *workflowConfigRecord) toModel() *model.WorkflowConfig {
	if r == nil {
		return nil
	}
	return &model.WorkflowConfig{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Transitions: r.Transitions.Bytes("{}"),
		Triggers:    r.Triggers.Bytes("[]"),
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type falsePositiveRecord struct {
	ID          uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID   uuid.UUID  `gorm:"column:project_id"`
	Pattern     string     `gorm:"column:pattern"`
	Category    string     `gorm:"column:category"`
	FilePattern string     `gorm:"column:file_pattern"`
	Reason      string     `gorm:"column:reason"`
	ReporterID  *uuid.UUID `gorm:"column:reporter_id"`
	Occurrences int        `gorm:"column:occurrences"`
	IsStrong    bool       `gorm:"column:is_strong"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (falsePositiveRecord) TableName() string { return "false_positives" }

func newFalsePositiveRecord(fp *model.FalsePositive) *falsePositiveRecord {
	if fp == nil {
		return nil
	}
	return &falsePositiveRecord{
		ID:          fp.ID,
		ProjectID:   fp.ProjectID,
		Pattern:     fp.Pattern,
		Category:    fp.Category,
		FilePattern: fp.FilePattern,
		Reason:      fp.Reason,
		ReporterID:  fp.ReporterID,
		Occurrences: fp.Occurrences,
		IsStrong:    fp.IsStrong,
		CreatedAt:   fp.CreatedAt,
		UpdatedAt:   fp.UpdatedAt,
	}
}

func (r *falsePositiveRecord) toModel() *model.FalsePositive {
	if r == nil {
		return nil
	}
	return &model.FalsePositive{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Pattern:     r.Pattern,
		Category:    r.Category,
		FilePattern: r.FilePattern,
		Reason:      r.Reason,
		ReporterID:  r.ReporterID,
		Occurrences: r.Occurrences,
		IsStrong:    r.IsStrong,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type reviewAggregationRecord struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey"`
	PRURL          string     `gorm:"column:pr_url"`
	TaskID         uuid.UUID  `gorm:"column:task_id"`
	ReviewIDs      uuidList   `gorm:"column:review_ids;type:uuid[]"`
	OverallRisk    string     `gorm:"column:overall_risk"`
	Recommendation string     `gorm:"column:recommendation"`
	Findings       jsonText   `gorm:"column:findings;type:jsonb"`
	Summary        string     `gorm:"column:summary"`
	Metrics        jsonText   `gorm:"column:metrics;type:jsonb"`
	HumanDecision  *string    `gorm:"column:human_decision"`
	HumanReviewer  *uuid.UUID `gorm:"column:human_reviewer"`
	HumanComment   *string    `gorm:"column:human_comment"`
	DecidedAt      *time.Time `gorm:"column:decided_at"`
	TotalCostUsd   float64    `gorm:"column:total_cost_usd"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (reviewAggregationRecord) TableName() string { return "review_aggregations" }

func newReviewAggregationRecord(agg *model.ReviewAggregation) *reviewAggregationRecord {
	if agg == nil {
		return nil
	}
	return &reviewAggregationRecord{
		ID:             agg.ID,
		PRURL:          agg.PRURL,
		TaskID:         agg.TaskID,
		ReviewIDs:      newUUIDList(agg.ReviewIDs),
		OverallRisk:    agg.OverallRisk,
		Recommendation: agg.Recommendation,
		Findings:       newJSONText(agg.Findings, "[]"),
		Summary:        agg.Summary,
		Metrics:        newJSONText(agg.Metrics, "{}"),
		HumanDecision:  cloneStringPointer(agg.HumanDecision),
		HumanReviewer:  cloneUUIDPointer(agg.HumanReviewer),
		HumanComment:   cloneStringPointer(agg.HumanComment),
		DecidedAt:      cloneTimePointer(agg.DecidedAt),
		TotalCostUsd:   agg.TotalCostUsd,
		CreatedAt:      agg.CreatedAt,
		UpdatedAt:      agg.UpdatedAt,
	}
}

func (r *reviewAggregationRecord) toModel() *model.ReviewAggregation {
	if r == nil {
		return nil
	}
	return &model.ReviewAggregation{
		ID:             r.ID,
		PRURL:          r.PRURL,
		TaskID:         r.TaskID,
		ReviewIDs:      r.ReviewIDs.Slice(),
		OverallRisk:    r.OverallRisk,
		Recommendation: r.Recommendation,
		Findings:       r.Findings.String("[]"),
		Summary:        r.Summary,
		Metrics:        r.Metrics.String("{}"),
		HumanDecision:  cloneStringPointer(r.HumanDecision),
		HumanReviewer:  cloneUUIDPointer(r.HumanReviewer),
		HumanComment:   cloneStringPointer(r.HumanComment),
		DecidedAt:      cloneTimePointer(r.DecidedAt),
		TotalCostUsd:   r.TotalCostUsd,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

type pluginRecordModel struct {
	PluginID           string     `gorm:"column:plugin_id;primaryKey"`
	Kind               string     `gorm:"column:kind"`
	Name               string     `gorm:"column:name"`
	Version            string     `gorm:"column:version"`
	Description        *string    `gorm:"column:description"`
	Tags               stringList `gorm:"column:tags;type:text[]"`
	Manifest           rawJSON    `gorm:"column:manifest;type:jsonb"`
	SourceType         string     `gorm:"column:source_type"`
	SourcePath         *string    `gorm:"column:source_path"`
	Runtime            string     `gorm:"column:runtime"`
	LifecycleState     string     `gorm:"column:lifecycle_state"`
	RuntimeHost        string     `gorm:"column:runtime_host"`
	LastHealthAt       *time.Time `gorm:"column:last_health_at"`
	LastError          *string    `gorm:"column:last_error"`
	RestartCount       int        `gorm:"column:restart_count"`
	ResolvedSourcePath *string    `gorm:"column:resolved_source_path"`
	RuntimeMetadata    rawJSON    `gorm:"column:runtime_metadata;type:jsonb"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (pluginRecordModel) TableName() string { return "plugins" }

type pluginInstanceRecordModel struct {
	PluginID           string     `gorm:"column:plugin_id;primaryKey"`
	ProjectID          *string    `gorm:"column:project_id"`
	RuntimeHost        string     `gorm:"column:runtime_host"`
	LifecycleState     string     `gorm:"column:lifecycle_state"`
	ResolvedSourcePath *string    `gorm:"column:resolved_source_path"`
	RuntimeMetadata    rawJSON    `gorm:"column:runtime_metadata;type:jsonb"`
	RestartCount       int        `gorm:"column:restart_count"`
	LastHealthAt       *time.Time `gorm:"column:last_health_at"`
	LastError          *string    `gorm:"column:last_error"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (pluginInstanceRecordModel) TableName() string { return "plugin_instances" }

type pluginEventRecordModel struct {
	ID             string    `gorm:"column:id;primaryKey"`
	PluginID       string    `gorm:"column:plugin_id"`
	EventType      string    `gorm:"column:event_type"`
	EventSource    string    `gorm:"column:event_source"`
	LifecycleState *string   `gorm:"column:lifecycle_state"`
	Summary        *string   `gorm:"column:summary"`
	Payload        rawJSON   `gorm:"column:payload;type:jsonb"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (pluginEventRecordModel) TableName() string { return "plugin_events" }

type scheduledJobRecord struct {
	JobKey         string     `gorm:"column:job_key;primaryKey"`
	Name           string     `gorm:"column:name"`
	Scope          string     `gorm:"column:scope"`
	Schedule       string     `gorm:"column:schedule"`
	Enabled        bool       `gorm:"column:enabled"`
	ExecutionMode  string     `gorm:"column:execution_mode"`
	OverlapPolicy  string     `gorm:"column:overlap_policy"`
	LastRunStatus  string     `gorm:"column:last_run_status"`
	LastRunAt      *time.Time `gorm:"column:last_run_at"`
	NextRunAt      *time.Time `gorm:"column:next_run_at"`
	LastRunSummary *string    `gorm:"column:last_run_summary"`
	LastError      *string    `gorm:"column:last_error"`
	Config         jsonText   `gorm:"column:config;type:jsonb"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (scheduledJobRecord) TableName() string { return "scheduled_jobs" }

func newScheduledJobRecord(job *model.ScheduledJob) *scheduledJobRecord {
	if job == nil {
		return nil
	}
	return &scheduledJobRecord{
		JobKey:         job.JobKey,
		Name:           job.Name,
		Scope:          string(job.Scope),
		Schedule:       job.Schedule,
		Enabled:        job.Enabled,
		ExecutionMode:  string(job.ExecutionMode),
		OverlapPolicy:  string(job.OverlapPolicy),
		LastRunStatus:  string(job.LastRunStatus),
		LastRunAt:      cloneTimePointer(job.LastRunAt),
		NextRunAt:      cloneTimePointer(job.NextRunAt),
		LastRunSummary: cloneStringPointer(optionalStringPointer(job.LastRunSummary)),
		LastError:      cloneStringPointer(optionalStringPointer(job.LastError)),
		Config:         newJSONText(job.Config, "{}"),
		CreatedAt:      job.CreatedAt,
		UpdatedAt:      job.UpdatedAt,
	}
}

func (r *scheduledJobRecord) toModel() *model.ScheduledJob {
	if r == nil {
		return nil
	}
	return &model.ScheduledJob{
		JobKey:         r.JobKey,
		Name:           r.Name,
		Scope:          model.ScheduledJobScope(r.Scope),
		Schedule:       r.Schedule,
		Enabled:        r.Enabled,
		ExecutionMode:  model.ScheduledJobExecutionMode(r.ExecutionMode),
		OverlapPolicy:  model.ScheduledJobOverlapPolicy(r.OverlapPolicy),
		LastRunStatus:  model.ScheduledJobRunStatus(r.LastRunStatus),
		LastRunAt:      cloneTimePointer(r.LastRunAt),
		NextRunAt:      cloneTimePointer(r.NextRunAt),
		LastRunSummary: valueOrEmpty(r.LastRunSummary),
		LastError:      valueOrEmpty(r.LastError),
		Config:         r.Config.String("{}"),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

type scheduledJobRunRecord struct {
	RunID         string     `gorm:"column:run_id;primaryKey"`
	JobKey        string     `gorm:"column:job_key"`
	TriggerSource string     `gorm:"column:trigger_source"`
	Status        string     `gorm:"column:status"`
	StartedAt     time.Time  `gorm:"column:started_at"`
	FinishedAt    *time.Time `gorm:"column:finished_at"`
	Summary       *string    `gorm:"column:summary"`
	ErrorMessage  *string    `gorm:"column:error_message"`
	Metrics       jsonText   `gorm:"column:metrics;type:jsonb"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

func (scheduledJobRunRecord) TableName() string { return "scheduled_job_runs" }

func newScheduledJobRunRecord(run *model.ScheduledJobRun) *scheduledJobRunRecord {
	if run == nil {
		return nil
	}
	return &scheduledJobRunRecord{
		RunID:         run.RunID,
		JobKey:        run.JobKey,
		TriggerSource: string(run.TriggerSource),
		Status:        string(run.Status),
		StartedAt:     run.StartedAt,
		FinishedAt:    cloneTimePointer(run.FinishedAt),
		Summary:       cloneStringPointer(optionalStringPointer(run.Summary)),
		ErrorMessage:  cloneStringPointer(optionalStringPointer(run.ErrorMessage)),
		Metrics:       newJSONText(run.Metrics, "{}"),
		CreatedAt:     run.CreatedAt,
		UpdatedAt:     run.UpdatedAt,
	}
}

func (r *scheduledJobRunRecord) toModel() *model.ScheduledJobRun {
	if r == nil {
		return nil
	}
	run := &model.ScheduledJobRun{
		RunID:         r.RunID,
		JobKey:        r.JobKey,
		TriggerSource: model.ScheduledJobTriggerSource(r.TriggerSource),
		Status:        model.ScheduledJobRunStatus(r.Status),
		StartedAt:     r.StartedAt,
		FinishedAt:    cloneTimePointer(r.FinishedAt),
		Summary:       valueOrEmpty(r.Summary),
		ErrorMessage:  valueOrEmpty(r.ErrorMessage),
		Metrics:       r.Metrics.String("{}"),
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	run.ComputeDuration()
	return run
}

type taskRecord struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id"`
	ParentID       *uuid.UUID `gorm:"column:parent_id"`
	SprintID       *uuid.UUID `gorm:"column:sprint_id"`
	MilestoneID    *uuid.UUID `gorm:"column:milestone_id"`
	Title          string     `gorm:"column:title"`
	Description    string     `gorm:"column:description"`
	Status         string     `gorm:"column:status"`
	Priority       string     `gorm:"column:priority"`
	AssigneeID     *uuid.UUID `gorm:"column:assignee_id"`
	AssigneeType   *string    `gorm:"column:assignee_type"`
	ReporterID     *uuid.UUID `gorm:"column:reporter_id"`
	Labels         stringList `gorm:"column:labels;type:text[]"`
	BudgetUsd      float64    `gorm:"column:budget_usd"`
	SpentUsd       float64    `gorm:"column:spent_usd"`
	AgentBranch    string     `gorm:"column:agent_branch"`
	AgentWorktree  string     `gorm:"column:agent_worktree"`
	AgentSessionID string     `gorm:"column:agent_session_id"`
	PRUrl          string     `gorm:"column:pr_url"`
	PRNumber       int        `gorm:"column:pr_number"`
	BlockedBy      uuidList   `gorm:"column:blocked_by;type:uuid[]"`
	PlannedStartAt *time.Time `gorm:"column:planned_start_at"`
	PlannedEndAt   *time.Time `gorm:"column:planned_end_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	CompletedAt    *time.Time `gorm:"column:completed_at"`
}

func (taskRecord) TableName() string { return "tasks" }

func newTaskRecord(task *model.Task) *taskRecord {
	if task == nil {
		return nil
	}
	blockedByIDs := make([]uuid.UUID, 0, len(task.BlockedBy))
	for _, raw := range task.BlockedBy {
		if id, err := uuid.Parse(raw); err == nil {
			blockedByIDs = append(blockedByIDs, id)
		}
	}
	var assigneeType *string
	if task.AssigneeType != "" {
		assigneeType = &task.AssigneeType
	}
	return &taskRecord{
		ID:             task.ID,
		ProjectID:      task.ProjectID,
		ParentID:       task.ParentID,
		SprintID:       task.SprintID,
		MilestoneID:    cloneUUIDPointer(task.MilestoneID),
		Title:          task.Title,
		Description:    task.Description,
		Status:         task.Status,
		Priority:       task.Priority,
		AssigneeID:     task.AssigneeID,
		AssigneeType:   assigneeType,
		ReporterID:     task.ReporterID,
		Labels:         newStringList(task.Labels),
		BudgetUsd:      task.BudgetUsd,
		SpentUsd:       task.SpentUsd,
		AgentBranch:    task.AgentBranch,
		AgentWorktree:  task.AgentWorktree,
		AgentSessionID: task.AgentSessionID,
		PRUrl:          task.PRUrl,
		PRNumber:       task.PRNumber,
		BlockedBy:      newUUIDList(blockedByIDs),
		PlannedStartAt: task.PlannedStartAt,
		PlannedEndAt:   task.PlannedEndAt,
		CreatedAt:      task.CreatedAt,
		UpdatedAt:      task.UpdatedAt,
		CompletedAt:    task.CompletedAt,
	}
}

func (r *taskRecord) toModel() *model.Task {
	if r == nil {
		return nil
	}
	var assigneeType string
	if r.AssigneeType != nil {
		assigneeType = *r.AssigneeType
	}
	blockedByIDs := r.BlockedBy.Slice()
	blockedBy := make([]string, 0, len(blockedByIDs))
	for _, id := range blockedByIDs {
		blockedBy = append(blockedBy, id.String())
	}
	return &model.Task{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		ParentID:       r.ParentID,
		SprintID:       r.SprintID,
		MilestoneID:    cloneUUIDPointer(r.MilestoneID),
		Title:          r.Title,
		Description:    r.Description,
		Status:         r.Status,
		Priority:       r.Priority,
		AssigneeID:     r.AssigneeID,
		AssigneeType:   assigneeType,
		ReporterID:     r.ReporterID,
		Labels:         r.Labels.Slice(),
		BudgetUsd:      r.BudgetUsd,
		SpentUsd:       r.SpentUsd,
		AgentBranch:    r.AgentBranch,
		AgentWorktree:  r.AgentWorktree,
		AgentSessionID: r.AgentSessionID,
		PRUrl:          r.PRUrl,
		PRNumber:       r.PRNumber,
		BlockedBy:      blockedBy,
		PlannedStartAt: r.PlannedStartAt,
		PlannedEndAt:   r.PlannedEndAt,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		CompletedAt:    r.CompletedAt,
	}
}

type taskProgressSnapshotRecord struct {
	TaskID             uuid.UUID  `gorm:"column:task_id;primaryKey"`
	LastActivityAt     time.Time  `gorm:"column:last_activity_at"`
	LastActivitySource string     `gorm:"column:last_activity_source"`
	LastTransitionAt   time.Time  `gorm:"column:last_transition_at"`
	HealthStatus       string     `gorm:"column:health_status"`
	RiskReason         string     `gorm:"column:risk_reason"`
	RiskSinceAt        *time.Time `gorm:"column:risk_since_at"`
	LastAlertState     string     `gorm:"column:last_alert_state"`
	LastAlertAt        *time.Time `gorm:"column:last_alert_at"`
	LastRecoveredAt    *time.Time `gorm:"column:last_recovered_at"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
}

func (taskProgressSnapshotRecord) TableName() string { return "task_progress_snapshots" }

func newTaskProgressSnapshotRecord(s *model.TaskProgressSnapshot) *taskProgressSnapshotRecord {
	if s == nil {
		return nil
	}
	return &taskProgressSnapshotRecord{
		TaskID:             s.TaskID,
		LastActivityAt:     s.LastActivityAt,
		LastActivitySource: s.LastActivitySource,
		LastTransitionAt:   s.LastTransitionAt,
		HealthStatus:       s.HealthStatus,
		RiskReason:         s.RiskReason,
		RiskSinceAt:        cloneTimePointer(s.RiskSinceAt),
		LastAlertState:     s.LastAlertState,
		LastAlertAt:        cloneTimePointer(s.LastAlertAt),
		LastRecoveredAt:    cloneTimePointer(s.LastRecoveredAt),
		CreatedAt:          s.CreatedAt,
		UpdatedAt:          s.UpdatedAt,
	}
}

func (r *taskProgressSnapshotRecord) toModel() *model.TaskProgressSnapshot {
	if r == nil {
		return nil
	}
	return &model.TaskProgressSnapshot{
		TaskID:             r.TaskID,
		LastActivityAt:     r.LastActivityAt,
		LastActivitySource: r.LastActivitySource,
		LastTransitionAt:   r.LastTransitionAt,
		HealthStatus:       r.HealthStatus,
		RiskReason:         r.RiskReason,
		RiskSinceAt:        cloneTimePointer(r.RiskSinceAt),
		LastAlertState:     r.LastAlertState,
		LastAlertAt:        cloneTimePointer(r.LastAlertAt),
		LastRecoveredAt:    cloneTimePointer(r.LastRecoveredAt),
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

type agentRunRecord struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	TaskID          uuid.UUID  `gorm:"column:task_id"`
	MemberID        uuid.UUID  `gorm:"column:member_id"`
	RoleID          string     `gorm:"column:role_id"`
	Status          string     `gorm:"column:status"`
	Runtime         string     `gorm:"column:runtime"`
	Provider        string     `gorm:"column:provider"`
	Model           string     `gorm:"column:model"`
	InputTokens     int64      `gorm:"column:input_tokens"`
	OutputTokens    int64      `gorm:"column:output_tokens"`
	CacheReadTokens int64      `gorm:"column:cache_read_tokens"`
	CostUsd         float64    `gorm:"column:cost_usd"`
	TurnCount       int        `gorm:"column:turn_count"`
	ErrorMessage    string     `gorm:"column:error_message"`
	StartedAt       time.Time  `gorm:"column:started_at"`
	CompletedAt     *time.Time `gorm:"column:completed_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	TeamID          *uuid.UUID `gorm:"column:team_id"`
	TeamRole        string     `gorm:"column:team_role"`
}

func (agentRunRecord) TableName() string { return "agent_runs" }

func newAgentRunRecord(run *model.AgentRun) *agentRunRecord {
	if run == nil {
		return nil
	}
	return &agentRunRecord{
		ID:              run.ID,
		TaskID:          run.TaskID,
		MemberID:        run.MemberID,
		RoleID:          run.RoleID,
		Status:          run.Status,
		Runtime:         run.Runtime,
		Provider:        run.Provider,
		Model:           run.Model,
		InputTokens:     run.InputTokens,
		OutputTokens:    run.OutputTokens,
		CacheReadTokens: run.CacheReadTokens,
		CostUsd:         run.CostUsd,
		TurnCount:       run.TurnCount,
		ErrorMessage:    run.ErrorMessage,
		StartedAt:       run.StartedAt,
		CompletedAt:     cloneTimePointer(run.CompletedAt),
		CreatedAt:       run.CreatedAt,
		UpdatedAt:       run.UpdatedAt,
		TeamID:          run.TeamID,
		TeamRole:        run.TeamRole,
	}
}

func (r *agentRunRecord) toModel() *model.AgentRun {
	if r == nil {
		return nil
	}
	return &model.AgentRun{
		ID:              r.ID,
		TaskID:          r.TaskID,
		MemberID:        r.MemberID,
		RoleID:          r.RoleID,
		Status:          r.Status,
		Runtime:         r.Runtime,
		Provider:        r.Provider,
		Model:           r.Model,
		InputTokens:     r.InputTokens,
		OutputTokens:    r.OutputTokens,
		CacheReadTokens: r.CacheReadTokens,
		CostUsd:         r.CostUsd,
		TurnCount:       r.TurnCount,
		ErrorMessage:    r.ErrorMessage,
		StartedAt:       r.StartedAt,
		CompletedAt:     cloneTimePointer(r.CompletedAt),
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
		TeamID:          r.TeamID,
		TeamRole:        r.TeamRole,
	}
}

type agentTeamRecord struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id"`
	TaskID         uuid.UUID  `gorm:"column:task_id"`
	Name           string     `gorm:"column:name"`
	Status         string     `gorm:"column:status"`
	Strategy       string     `gorm:"column:strategy"`
	PlannerRunID   *uuid.UUID `gorm:"column:planner_run_id"`
	ReviewerRunID  *uuid.UUID `gorm:"column:reviewer_run_id"`
	TotalBudgetUsd float64    `gorm:"column:total_budget_usd"`
	TotalSpentUsd  float64    `gorm:"column:total_spent_usd"`
	Config         jsonText   `gorm:"column:config;type:jsonb"`
	ErrorMessage   string     `gorm:"column:error_message"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (agentTeamRecord) TableName() string { return "agent_teams" }

func newAgentTeamRecord(team *model.AgentTeam) *agentTeamRecord {
	if team == nil {
		return nil
	}
	return &agentTeamRecord{
		ID:             team.ID,
		ProjectID:      team.ProjectID,
		TaskID:         team.TaskID,
		Name:           team.Name,
		Status:         team.Status,
		Strategy:       team.Strategy,
		PlannerRunID:   team.PlannerRunID,
		ReviewerRunID:  team.ReviewerRunID,
		TotalBudgetUsd: team.TotalBudgetUsd,
		TotalSpentUsd:  team.TotalSpentUsd,
		Config:         newJSONText(team.Config, "{}"),
		ErrorMessage:   team.ErrorMessage,
		CreatedAt:      team.CreatedAt,
		UpdatedAt:      team.UpdatedAt,
	}
}

func (r *agentTeamRecord) toModel() *model.AgentTeam {
	if r == nil {
		return nil
	}
	return &model.AgentTeam{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		TaskID:         r.TaskID,
		Name:           r.Name,
		Status:         r.Status,
		Strategy:       r.Strategy,
		PlannerRunID:   r.PlannerRunID,
		ReviewerRunID:  r.ReviewerRunID,
		TotalBudgetUsd: r.TotalBudgetUsd,
		TotalSpentUsd:  r.TotalSpentUsd,
		Config:         r.Config.String("{}"),
		ErrorMessage:   r.ErrorMessage,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

type agentMemoryRecord struct {
	ID             uuid.UUID  `gorm:"column:id;primaryKey"`
	ProjectID      uuid.UUID  `gorm:"column:project_id"`
	Scope          string     `gorm:"column:scope"`
	RoleID         string     `gorm:"column:role_id"`
	Category       string     `gorm:"column:category"`
	Key            string     `gorm:"column:key"`
	Content        string     `gorm:"column:content"`
	Metadata       jsonText   `gorm:"column:metadata;type:jsonb"`
	RelevanceScore float64    `gorm:"column:relevance_score"`
	AccessCount    int        `gorm:"column:access_count"`
	LastAccessedAt *time.Time `gorm:"column:last_accessed_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (agentMemoryRecord) TableName() string { return "agent_memory" }

func newAgentMemoryRecord(mem *model.AgentMemory) *agentMemoryRecord {
	if mem == nil {
		return nil
	}
	return &agentMemoryRecord{
		ID:             mem.ID,
		ProjectID:      mem.ProjectID,
		Scope:          mem.Scope,
		RoleID:         mem.RoleID,
		Category:       mem.Category,
		Key:            mem.Key,
		Content:        mem.Content,
		Metadata:       newJSONText(mem.Metadata, "{}"),
		RelevanceScore: mem.RelevanceScore,
		AccessCount:    mem.AccessCount,
		LastAccessedAt: cloneTimePointer(mem.LastAccessedAt),
		CreatedAt:      mem.CreatedAt,
		UpdatedAt:      mem.UpdatedAt,
	}
}

func (r *agentMemoryRecord) toModel() *model.AgentMemory {
	if r == nil {
		return nil
	}
	return &model.AgentMemory{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		Scope:          r.Scope,
		RoleID:         r.RoleID,
		Category:       r.Category,
		Key:            r.Key,
		Content:        r.Content,
		Metadata:       r.Metadata.String("{}"),
		RelevanceScore: r.RelevanceScore,
		AccessCount:    r.AccessCount,
		LastAccessedAt: cloneTimePointer(r.LastAccessedAt),
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
}

type reviewRecord struct {
	ID                uuid.UUID `gorm:"column:id;primaryKey"`
	TaskID            uuid.UUID `gorm:"column:task_id"`
	PRURL             string    `gorm:"column:pr_url"`
	PRNumber          int       `gorm:"column:pr_number"`
	Layer             int       `gorm:"column:layer"`
	Status            string    `gorm:"column:status"`
	RiskLevel         string    `gorm:"column:risk_level"`
	Findings          rawJSON   `gorm:"column:findings;type:jsonb"`
	ExecutionMetadata rawJSON   `gorm:"column:execution_metadata;type:jsonb"`
	Summary           string    `gorm:"column:summary"`
	Recommendation    string    `gorm:"column:recommendation"`
	CostUSD           float64   `gorm:"column:cost_usd"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (reviewRecord) TableName() string { return "reviews" }

func newReviewRecord(review *model.Review) (*reviewRecord, error) {
	if review == nil {
		return nil, nil
	}
	findingsJSON, err := json.Marshal(review.Findings)
	if err != nil {
		return nil, fmt.Errorf("marshal findings: %w", err)
	}
	if len(review.Findings) == 0 {
		findingsJSON = []byte("[]")
	}
	executionMetadataJSON := []byte("{}")
	if review.ExecutionMetadata != nil {
		executionMetadataJSON, err = json.Marshal(review.ExecutionMetadata)
		if err != nil {
			return nil, fmt.Errorf("marshal execution metadata: %w", err)
		}
	}
	return &reviewRecord{
		ID:                review.ID,
		TaskID:            review.TaskID,
		PRURL:             review.PRURL,
		PRNumber:          review.PRNumber,
		Layer:             review.Layer,
		Status:            review.Status,
		RiskLevel:         review.RiskLevel,
		Findings:          newRawJSON(findingsJSON, "[]"),
		ExecutionMetadata: newRawJSON(executionMetadataJSON, "{}"),
		Summary:           review.Summary,
		Recommendation:    review.Recommendation,
		CostUSD:           review.CostUSD,
		CreatedAt:         review.CreatedAt,
		UpdatedAt:         review.UpdatedAt,
	}, nil
}

func (r *reviewRecord) toModel() (*model.Review, error) {
	if r == nil {
		return nil, nil
	}
	review := &model.Review{
		ID:             r.ID,
		TaskID:         r.TaskID,
		PRURL:          r.PRURL,
		PRNumber:       r.PRNumber,
		Layer:          r.Layer,
		Status:         r.Status,
		RiskLevel:      r.RiskLevel,
		Summary:        r.Summary,
		Recommendation: r.Recommendation,
		CostUSD:        r.CostUSD,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
	}
	if findingsRaw := r.Findings.Bytes("[]"); len(findingsRaw) > 0 {
		if err := json.Unmarshal(findingsRaw, &review.Findings); err != nil {
			return nil, fmt.Errorf("unmarshal findings: %w", err)
		}
	}
	if metaRaw := r.ExecutionMetadata.Bytes("{}"); len(metaRaw) > 0 && string(metaRaw) != "null" && string(metaRaw) != "{}" {
		var metadata model.ReviewExecutionMetadata
		if err := json.Unmarshal(metaRaw, &metadata); err != nil {
			return nil, fmt.Errorf("unmarshal execution metadata: %w", err)
		}
		review.ExecutionMetadata = &metadata
	}
	return review, nil
}

type agentPoolQueueEntryRecord struct {
	EntryID    string    `gorm:"column:entry_id;primaryKey"`
	ProjectID  string    `gorm:"column:project_id"`
	TaskID     string    `gorm:"column:task_id"`
	MemberID   string    `gorm:"column:member_id"`
	Status     string    `gorm:"column:status"`
	Reason     string    `gorm:"column:reason"`
	Runtime    string    `gorm:"column:runtime"`
	Provider   string    `gorm:"column:provider"`
	Model      string    `gorm:"column:model"`
	RoleID     *string   `gorm:"column:role_id"`
	Priority   int       `gorm:"column:priority"`
	BudgetUSD  float64   `gorm:"column:budget_usd"`
	AgentRunID *string   `gorm:"column:agent_run_id"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (agentPoolQueueEntryRecord) TableName() string { return "agent_pool_queue_entries" }

func newAgentPoolQueueEntryRecord(entry *model.AgentPoolQueueEntry) *agentPoolQueueEntryRecord {
	if entry == nil {
		return nil
	}
	return &agentPoolQueueEntryRecord{
		EntryID:    entry.EntryID,
		ProjectID:  entry.ProjectID,
		TaskID:     entry.TaskID,
		MemberID:   entry.MemberID,
		Status:     string(entry.Status),
		Reason:     entry.Reason,
		Runtime:    entry.Runtime,
		Provider:   entry.Provider,
		Model:      entry.Model,
		RoleID:     cloneStringPointer(optionalStringPointer(entry.RoleID)),
		Priority:   entry.Priority,
		BudgetUSD:  entry.BudgetUSD,
		AgentRunID: cloneStringPointer(entry.AgentRunID),
		CreatedAt:  entry.CreatedAt,
		UpdatedAt:  entry.UpdatedAt,
	}
}

func (r *agentPoolQueueEntryRecord) toModel() *model.AgentPoolQueueEntry {
	if r == nil {
		return nil
	}
	return &model.AgentPoolQueueEntry{
		EntryID:    r.EntryID,
		ProjectID:  r.ProjectID,
		TaskID:     r.TaskID,
		MemberID:   r.MemberID,
		Status:     model.AgentPoolQueueStatus(r.Status),
		Reason:     r.Reason,
		Runtime:    r.Runtime,
		Provider:   r.Provider,
		Model:      r.Model,
		RoleID:     valueOrEmpty(r.RoleID),
		Priority:   r.Priority,
		BudgetUSD:  r.BudgetUSD,
		AgentRunID: cloneStringPointer(r.AgentRunID),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}
