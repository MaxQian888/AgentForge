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
	ScheduledJobRunStatusPending   ScheduledJobRunStatus = "pending"
	ScheduledJobRunStatusRunning   ScheduledJobRunStatus = "running"
	ScheduledJobRunStatusSucceeded ScheduledJobRunStatus = "succeeded"
	ScheduledJobRunStatusFailed    ScheduledJobRunStatus = "failed"
	ScheduledJobRunStatusSkipped   ScheduledJobRunStatus = "skipped"
)

type ScheduledJobTriggerSource string

const (
	ScheduledJobTriggerCron      ScheduledJobTriggerSource = "cron"
	ScheduledJobTriggerManual    ScheduledJobTriggerSource = "manual"
	ScheduledJobTriggerStartup   ScheduledJobTriggerSource = "startup"
	ScheduledJobTriggerReconcile ScheduledJobTriggerSource = "reconcile"
)

type ScheduledJob struct {
	JobKey         string                    `db:"job_key" json:"jobKey"`
	Name           string                    `db:"name" json:"name"`
	Scope          ScheduledJobScope         `db:"scope" json:"scope"`
	Schedule       string                    `db:"schedule" json:"schedule"`
	Enabled        bool                      `db:"enabled" json:"enabled"`
	ExecutionMode  ScheduledJobExecutionMode `db:"execution_mode" json:"executionMode"`
	OverlapPolicy  ScheduledJobOverlapPolicy `db:"overlap_policy" json:"overlapPolicy"`
	LastRunStatus  ScheduledJobRunStatus     `db:"last_run_status" json:"lastRunStatus"`
	LastRunAt      *time.Time                `db:"last_run_at" json:"lastRunAt,omitempty"`
	NextRunAt      *time.Time                `db:"next_run_at" json:"nextRunAt,omitempty"`
	LastRunSummary string                    `db:"last_run_summary" json:"lastRunSummary"`
	LastError      string                    `db:"last_error" json:"lastError"`
	Config         string                    `db:"config" json:"config"`
	CreatedAt      time.Time                 `db:"created_at" json:"createdAt"`
	UpdatedAt      time.Time                 `db:"updated_at" json:"updatedAt"`
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
	TotalJobs     int `json:"totalJobs"`
	EnabledJobs   int `json:"enabledJobs"`
	DisabledJobs  int `json:"disabledJobs"`
	FailedJobs    int `json:"failedJobs"`
	ActiveRuns    int `json:"activeRuns"`
	TotalRuns24h  int `json:"totalRuns24h"`
	FailedRuns24h int `json:"failedRuns24h"`
}

func (status ScheduledJobRunStatus) IsTerminal() bool {
	switch status {
	case ScheduledJobRunStatusSucceeded, ScheduledJobRunStatusFailed, ScheduledJobRunStatusSkipped:
		return true
	default:
		return false
	}
}
