package repository

import (
	"encoding/json"
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
	Email       string     `gorm:"column:email"`
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
	return &memberRecord{
		ID:          member.ID,
		ProjectID:   member.ProjectID,
		UserID:      member.UserID,
		Name:        member.Name,
		Type:        member.Type,
		Role:        member.Role,
		Email:       member.Email,
		AvatarURL:   member.AvatarURL,
		AgentConfig: newJSONText(member.AgentConfig, "{}"),
		Skills:      newStringList(member.Skills),
		IsActive:    member.IsActive,
		CreatedAt:   member.CreatedAt,
		UpdatedAt:   member.UpdatedAt,
	}
}

func (r *memberRecord) toModel() *model.Member {
	if r == nil {
		return nil
	}
	return &model.Member{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		UserID:      r.UserID,
		Name:        r.Name,
		Type:        r.Type,
		Role:        r.Role,
		Email:       r.Email,
		AvatarURL:   r.AvatarURL,
		AgentConfig: r.AgentConfig.String("{}"),
		Skills:      r.Skills.Slice(),
		IsActive:    r.IsActive,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type sprintRecord struct {
	ID             uuid.UUID `gorm:"column:id;primaryKey"`
	ProjectID      uuid.UUID `gorm:"column:project_id"`
	Name           string    `gorm:"column:name"`
	StartDate      time.Time `gorm:"column:start_date"`
	EndDate        time.Time `gorm:"column:end_date"`
	Status         string    `gorm:"column:status"`
	TotalBudgetUsd float64   `gorm:"column:total_budget_usd"`
	SpentUsd       float64   `gorm:"column:spent_usd"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
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
	return &model.ScheduledJobRun{
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
		BudgetUSD:  r.BudgetUSD,
		AgentRunID: cloneStringPointer(r.AgentRunID),
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}
