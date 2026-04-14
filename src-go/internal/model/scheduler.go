package model

import "time"

type ScheduledJobScope string

const (
	ScheduledJobScopeSystem  ScheduledJobScope = "system"
	ScheduledJobScopeProject ScheduledJobScope = "project"
)

type ScheduledJobExecutionMode string

const (
	ScheduledJobExecutionModeInProcess    ScheduledJobExecutionMode = "in_process"
	ScheduledJobExecutionModeOSRegistered ScheduledJobExecutionMode = "os_registered"
)

type ScheduledJobOverlapPolicy string

const (
	ScheduledJobOverlapSkip  ScheduledJobOverlapPolicy = "skip"
	ScheduledJobOverlapAllow ScheduledJobOverlapPolicy = "allow"
)

type ScheduledJobRunStatus string

const (
	ScheduledJobRunStatusPending         ScheduledJobRunStatus = "pending"
	ScheduledJobRunStatusRunning         ScheduledJobRunStatus = "running"
	ScheduledJobRunStatusCancelRequested ScheduledJobRunStatus = "cancel_requested"
	ScheduledJobRunStatusCancelled       ScheduledJobRunStatus = "cancelled"
	ScheduledJobRunStatusSucceeded       ScheduledJobRunStatus = "succeeded"
	ScheduledJobRunStatusFailed          ScheduledJobRunStatus = "failed"
	ScheduledJobRunStatusSkipped         ScheduledJobRunStatus = "skipped"
)

type ScheduledJobControlState string

const (
	ScheduledJobControlStateActive ScheduledJobControlState = "active"
	ScheduledJobControlStatePaused ScheduledJobControlState = "paused"
)

type ScheduledJobAction string

const (
	ScheduledJobActionPause   ScheduledJobAction = "pause"
	ScheduledJobActionResume  ScheduledJobAction = "resume"
	ScheduledJobActionTrigger ScheduledJobAction = "trigger"
	ScheduledJobActionCancel  ScheduledJobAction = "cancel"
	ScheduledJobActionCleanup ScheduledJobAction = "cleanup"
	ScheduledJobActionUpdate  ScheduledJobAction = "update"
)

type ScheduledJobTriggerSource string

const (
	ScheduledJobTriggerCron      ScheduledJobTriggerSource = "cron"
	ScheduledJobTriggerManual    ScheduledJobTriggerSource = "manual"
	ScheduledJobTriggerStartup   ScheduledJobTriggerSource = "startup"
	ScheduledJobTriggerReconcile ScheduledJobTriggerSource = "reconcile"
)

type ScheduledJob struct {
	JobKey           string                      `db:"job_key" json:"jobKey"`
	Name             string                      `db:"name" json:"name"`
	Scope            ScheduledJobScope           `db:"scope" json:"scope"`
	Schedule         string                      `db:"schedule" json:"schedule"`
	Enabled          bool                        `db:"enabled" json:"enabled"`
	ExecutionMode    ScheduledJobExecutionMode   `db:"execution_mode" json:"executionMode"`
	OverlapPolicy    ScheduledJobOverlapPolicy   `db:"overlap_policy" json:"overlapPolicy"`
	LastRunStatus    ScheduledJobRunStatus       `db:"last_run_status" json:"lastRunStatus"`
	LastRunAt        *time.Time                  `db:"last_run_at" json:"lastRunAt,omitempty"`
	NextRunAt        *time.Time                  `db:"next_run_at" json:"nextRunAt,omitempty"`
	LastRunSummary   string                      `db:"last_run_summary" json:"lastRunSummary"`
	LastError        string                      `db:"last_error" json:"lastError"`
	Config           string                      `db:"config" json:"config"`
	ControlState     ScheduledJobControlState    `db:"-" json:"controlState,omitempty"`
	ActiveRun        *ScheduledJobRunSummary     `db:"-" json:"activeRun,omitempty"`
	SupportedActions []ScheduledJobActionSupport `db:"-" json:"supportedActions,omitempty"`
	ConfigMetadata   *ScheduledJobConfigMetadata `db:"-" json:"configMetadata,omitempty"`
	UpcomingRuns     []ScheduledJobOccurrence    `db:"-" json:"upcomingRuns,omitempty"`
	CreatedAt        time.Time                   `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time                   `db:"updated_at" json:"updatedAt"`
}

type ScheduledJobRun struct {
	RunID         string                    `db:"run_id" json:"runId"`
	JobKey        string                    `db:"job_key" json:"jobKey"`
	TriggerSource ScheduledJobTriggerSource `db:"trigger_source" json:"triggerSource"`
	Status        ScheduledJobRunStatus     `db:"status" json:"status"`
	StartedAt     time.Time                 `db:"started_at" json:"startedAt"`
	FinishedAt    *time.Time                `db:"finished_at" json:"finishedAt,omitempty"`
	DurationMs    *int64                    `db:"-" json:"durationMs,omitempty"`
	Summary       string                    `db:"summary" json:"summary"`
	ErrorMessage  string                    `db:"error_message" json:"errorMessage"`
	Metrics       string                    `db:"metrics" json:"metrics"`
	CreatedAt     time.Time                 `db:"created_at" json:"createdAt"`
	UpdatedAt     time.Time                 `db:"updated_at" json:"updatedAt"`
}

type ScheduledJobRunFilters struct {
	JobKey         string                      `json:"jobKey,omitempty"`
	Statuses       []ScheduledJobRunStatus     `json:"statuses,omitempty"`
	TriggerSources []ScheduledJobTriggerSource `json:"triggerSources,omitempty"`
	StartedAfter   *time.Time                  `json:"startedAfter,omitempty"`
	StartedBefore  *time.Time                  `json:"startedBefore,omitempty"`
	Limit          int                         `json:"limit,omitempty"`
}

type ScheduledJobRunCleanupPolicy struct {
	JobKey        string     `json:"jobKey,omitempty"`
	StartedBefore *time.Time `json:"startedBefore,omitempty"`
	RetainRecent  int        `json:"retainRecent,omitempty"`
}

type ScheduledJobRunSummary struct {
	RunID         string                    `json:"runId"`
	TriggerSource ScheduledJobTriggerSource `json:"triggerSource"`
	Status        ScheduledJobRunStatus     `json:"status"`
	StartedAt     time.Time                 `json:"startedAt"`
	FinishedAt    *time.Time                `json:"finishedAt,omitempty"`
	DurationMs    *int64                    `json:"durationMs,omitempty"`
	Summary       string                    `json:"summary"`
	ErrorMessage  string                    `json:"errorMessage"`
}

type ScheduledJobActionSupport struct {
	Action  ScheduledJobAction `json:"action"`
	Enabled bool               `json:"enabled"`
	Reason  string             `json:"reason,omitempty"`
}

type ScheduledJobOccurrence struct {
	RunAt time.Time `json:"runAt"`
}

type ScheduledJobConfigFieldType string

const (
	ScheduledJobConfigFieldTypeBoolean ScheduledJobConfigFieldType = "boolean"
	ScheduledJobConfigFieldTypeInteger ScheduledJobConfigFieldType = "integer"
	ScheduledJobConfigFieldTypeNumber  ScheduledJobConfigFieldType = "number"
	ScheduledJobConfigFieldTypeSelect  ScheduledJobConfigFieldType = "select"
	ScheduledJobConfigFieldTypeString  ScheduledJobConfigFieldType = "string"
)

type ScheduledJobConfigFieldOption struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type ScheduledJobConfigFieldDescriptor struct {
	Key          string                          `json:"key"`
	Label        string                          `json:"label"`
	Type         ScheduledJobConfigFieldType     `json:"type"`
	Required     bool                            `json:"required,omitempty"`
	DefaultValue any                             `json:"defaultValue,omitempty"`
	HelpText     string                          `json:"helpText,omitempty"`
	Placeholder  string                          `json:"placeholder,omitempty"`
	Options      []ScheduledJobConfigFieldOption `json:"options,omitempty"`
}

type ScheduledJobConfigMetadata struct {
	SchemaVersion string                              `json:"schemaVersion,omitempty"`
	Editable      bool                                `json:"editable"`
	Reason        string                              `json:"reason,omitempty"`
	Fields        []ScheduledJobConfigFieldDescriptor `json:"fields,omitempty"`
}

func (r *ScheduledJobRun) ComputeDuration() {
	if r == nil || r.FinishedAt == nil {
		return
	}
	ms := r.FinishedAt.Sub(r.StartedAt).Milliseconds()
	r.DurationMs = &ms
}

type UpdateScheduledJobRequest struct {
	Enabled  *bool   `json:"enabled,omitempty"`
	Schedule *string `json:"schedule,omitempty"`
}

type SchedulerStats struct {
	TotalJobs         int     `json:"totalJobs"`
	EnabledJobs       int     `json:"enabledJobs"`
	DisabledJobs      int     `json:"disabledJobs"`
	PausedJobs        int     `json:"pausedJobs"`
	FailedJobs        int     `json:"failedJobs"`
	ActiveRuns        int     `json:"activeRuns"`
	QueueDepth        int     `json:"queueDepth"`
	TotalRuns24h      int     `json:"totalRuns24h"`
	SuccessfulRuns24h int     `json:"successfulRuns24h"`
	FailedRuns24h     int     `json:"failedRuns24h"`
	AverageDurationMs int64   `json:"averageDurationMs"`
	SuccessRate24h    float64 `json:"successRate24h"`
}

func (status ScheduledJobRunStatus) IsTerminal() bool {
	switch status {
	case ScheduledJobRunStatusCancelled, ScheduledJobRunStatusSucceeded, ScheduledJobRunStatusFailed, ScheduledJobRunStatusSkipped:
		return true
	default:
		return false
	}
}
