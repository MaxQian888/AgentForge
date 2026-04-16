package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

var ErrTaskProgressSnapshotNotFound = errors.New("task progress snapshot not found")

type TaskProgressTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	ListOpenForProgress(ctx context.Context) ([]*model.Task, error)
}

type TaskProgressSnapshotRepository interface {
	GetByTaskID(ctx context.Context, taskID uuid.UUID) (*model.TaskProgressSnapshot, error)
	Upsert(ctx context.Context, snapshot *model.TaskProgressSnapshot) error
}

type TaskProgressNotificationCreator interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type TaskProgressIMNotifier interface {
	NotifyTaskProgress(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot, title, body string) error
}

type TaskActivityInput struct {
	Source         string
	OccurredAt     time.Time
	UpdateHealth   bool
	MarkTransition bool
}

type TaskProgressConfig struct {
	WarningAfter     time.Duration
	StalledAfter     time.Duration
	AlertCooldown    time.Duration
	DetectorInterval time.Duration
	ExemptStatuses   []string
}

type TaskProgressService struct {
	taskRepo       TaskProgressTaskRepository
	snapshotRepo   TaskProgressSnapshotRepository
	notifications  TaskProgressNotificationCreator
	hub            *ws.Hub
	bus            eventbus.Publisher
	cfg            TaskProgressConfig
	now            func() time.Time
	imNotifier     TaskProgressIMNotifier
	exemptStatuses map[string]struct{}
}

func taskProgressLogFields(task *model.Task, snapshot *model.TaskProgressSnapshot) log.Fields {
	fields := log.Fields{}
	if task != nil {
		fields["taskId"] = task.ID.String()
		fields["projectId"] = task.ProjectID.String()
		fields["taskStatus"] = task.Status
	}
	if snapshot != nil {
		fields["healthStatus"] = snapshot.HealthStatus
		fields["riskReason"] = snapshot.RiskReason
		fields["lastActivitySource"] = snapshot.LastActivitySource
	}
	return fields
}

func NewTaskProgressService(
	taskRepo TaskProgressTaskRepository,
	snapshotRepo TaskProgressSnapshotRepository,
	notifications TaskProgressNotificationCreator,
	hub *ws.Hub,
	bus eventbus.Publisher,
	cfg TaskProgressConfig,
	now func() time.Time,
) *TaskProgressService {
	cfg = normalizeTaskProgressConfig(cfg)
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}

	exemptStatuses := make(map[string]struct{}, len(cfg.ExemptStatuses))
	for _, status := range cfg.ExemptStatuses {
		exemptStatuses[strings.TrimSpace(status)] = struct{}{}
	}

	return &TaskProgressService{
		taskRepo:       taskRepo,
		snapshotRepo:   snapshotRepo,
		notifications:  notifications,
		hub:            hub,
		bus:            bus,
		cfg:            cfg,
		now:            now,
		exemptStatuses: exemptStatuses,
	}
}

func (s *TaskProgressService) SetIMNotifier(notifier TaskProgressIMNotifier) {
	s.imNotifier = notifier
}

func (s *TaskProgressService) RecordActivity(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) (*model.TaskProgressSnapshot, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}

	snapshot, created, err := s.loadSnapshot(ctx, task)
	if err != nil {
		return nil, err
	}

	previous := cloneTaskProgressSnapshot(snapshot)
	now := resolveTaskActivityTime(input.OccurredAt, s.now)

	if snapshot.LastActivityAt.IsZero() || now.After(snapshot.LastActivityAt) || snapshot.LastActivitySource != input.Source {
		snapshot.LastActivityAt = now
		snapshot.LastActivitySource = input.Source
	}
	if input.MarkTransition && (snapshot.LastTransitionAt.IsZero() || now.After(snapshot.LastTransitionAt)) {
		snapshot.LastTransitionAt = now
	}
	if input.UpdateHealth {
		s.applyEvaluation(task, snapshot, now)
	}

	if err := s.afterSnapshotMutation(ctx, task, snapshot, previous, created); err != nil {
		return nil, err
	}
	log.WithFields(log.Fields{
		"taskId":             taskID.String(),
		"projectId":          task.ProjectID.String(),
		"taskStatus":         task.Status,
		"source":             input.Source,
		"occurredAt":         now.Format(time.RFC3339),
		"updateHealth":       input.UpdateHealth,
		"markTransition":     input.MarkTransition,
		"snapshotCreated":    created,
		"healthStatus":       snapshot.HealthStatus,
		"riskReason":         snapshot.RiskReason,
		"lastActivitySource": snapshot.LastActivitySource,
	}).Debug("task progress activity recorded")
	return snapshot, nil
}

func (s *TaskProgressService) EvaluateTask(ctx context.Context, taskID uuid.UUID) (*model.TaskProgressSnapshot, error) {
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return s.evaluateLoadedTask(ctx, task, s.now())
}

func (s *TaskProgressService) EvaluateOpenTasks(ctx context.Context) (int, error) {
	tasks, err := s.taskRepo.ListOpenForProgress(ctx)
	if err != nil {
		return 0, err
	}

	changed := 0
	now := s.now()
	for _, task := range tasks {
		snapshot, err := s.evaluateLoadedTask(ctx, task, now)
		if err != nil {
			return changed, err
		}
		if snapshot != nil {
			changed++
		}
	}
	return changed, nil
}

func (s *TaskProgressService) evaluateLoadedTask(ctx context.Context, task *model.Task, now time.Time) (*model.TaskProgressSnapshot, error) {
	snapshot, created, err := s.loadSnapshot(ctx, task)
	if err != nil {
		return nil, err
	}

	previous := cloneTaskProgressSnapshot(snapshot)
	s.applyEvaluation(task, snapshot, now)

	if err := s.afterSnapshotMutation(ctx, task, snapshot, previous, created); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func (s *TaskProgressService) afterSnapshotMutation(
	ctx context.Context,
	task *model.Task,
	snapshot *model.TaskProgressSnapshot,
	previous *model.TaskProgressSnapshot,
	created bool,
) error {
	alertEvent, alertSent, recovered := s.applyAlerting(ctx, task, snapshot, previous)
	task.Progress = snapshot

	if !created && taskProgressSnapshotsEqual(previous, snapshot) {
		log.WithFields(taskProgressLogFields(task, snapshot)).Debug("task progress unchanged after mutation")
		return nil
	}
	if err := s.snapshotRepo.Upsert(ctx, snapshot); err != nil {
		return fmt.Errorf("upsert task progress snapshot: %w", err)
	}
	fields := taskProgressLogFields(task, snapshot)
	fields["transition"] = alertEvent
	fields["snapshotCreated"] = created
	fields["alertSent"] = alertSent
	fields["recovered"] = recovered
	log.WithFields(fields).Info("task progress snapshot updated")

	s.broadcastProgressUpdate(ctx, task, snapshot, alertEvent)
	if alertSent {
		s.broadcastProgressAlert(ctx, task, snapshot)
	}
	if recovered {
		s.broadcastProgressRecovered(ctx, task, snapshot)
	}
	return nil
}

func (s *TaskProgressService) applyEvaluation(task *model.Task, snapshot *model.TaskProgressSnapshot, now time.Time) {
	health := model.TaskProgressHealthHealthy
	reason := model.TaskProgressReasonNone
	riskSince := (*time.Time)(nil)

	switch {
	case s.isExemptStatus(task.Status):
		health = model.TaskProgressHealthHealthy
	case task.AssigneeID == nil:
		health = model.TaskProgressHealthWarning
		reason = model.TaskProgressReasonNoAssignee
	default:
		inactiveFor := now.Sub(snapshot.LastActivityAt)
		switch {
		case inactiveFor >= s.cfg.StalledAfter:
			health = model.TaskProgressHealthStalled
			reason = s.resolveRiskReason(task)
		case inactiveFor >= s.cfg.WarningAfter:
			health = model.TaskProgressHealthWarning
			reason = s.resolveRiskReason(task)
		}
	}

	if health == model.TaskProgressHealthHealthy {
		if snapshot.HealthStatus != model.TaskProgressHealthHealthy && snapshot.LastAlertState != "" {
			recoveredAt := now
			snapshot.LastRecoveredAt = &recoveredAt
		}
		snapshot.HealthStatus = model.TaskProgressHealthHealthy
		snapshot.RiskReason = model.TaskProgressReasonNone
		snapshot.RiskSinceAt = nil
		return
	}

	if snapshot.HealthStatus == health && snapshot.RiskReason == reason && snapshot.RiskSinceAt != nil {
		riskSince = snapshot.RiskSinceAt
	} else {
		riskStart := now
		riskSince = &riskStart
	}

	snapshot.HealthStatus = health
	snapshot.RiskReason = reason
	snapshot.RiskSinceAt = riskSince
}

func (s *TaskProgressService) applyAlerting(
	ctx context.Context,
	task *model.Task,
	snapshot *model.TaskProgressSnapshot,
	previous *model.TaskProgressSnapshot,
) (string, bool, bool) {
	now := s.now()

	if snapshot.HealthStatus == model.TaskProgressHealthHealthy {
		if previous.LastAlertState == "" {
			snapshot.LastAlertState = ""
			return "activity", false, false
		}

		snapshot.LastAlertState = ""
		payload := mustJSONMarshal(taskProgressNotificationPayload(task, snapshot))
		title := fmt.Sprintf("Task recovered: %s", task.Title)
		body := fmt.Sprintf("Task %s is active again and no longer marked at risk.", task.Title)
		log.WithFields(taskProgressLogFields(task, snapshot)).Info("task progress recovered")
		s.notifyRecipients(ctx, task, model.NotificationTypeTaskProgressRecovered, title, body, payload)
		s.notifyIM(ctx, task, snapshot, title, body)
		return "recovered", false, true
	}

	alertState := fmt.Sprintf("%s:%s", snapshot.HealthStatus, snapshot.RiskReason)
	snapshot.LastAlertState = alertState

	shouldNotify := previous.LastAlertState != alertState
	if !shouldNotify && previous.LastAlertAt != nil && s.cfg.AlertCooldown > 0 && now.Sub(*previous.LastAlertAt) >= s.cfg.AlertCooldown {
		shouldNotify = true
	}
	if !shouldNotify && previous.LastAlertAt == nil {
		shouldNotify = true
	}

	if !shouldNotify {
		snapshot.LastAlertAt = previous.LastAlertAt
		return "activity", false, false
	}

	alertAt := now
	snapshot.LastAlertAt = &alertAt
	payload := mustJSONMarshal(taskProgressNotificationPayload(task, snapshot))
	title, body, notificationType := buildTaskProgressAlertMessage(task, snapshot)
	log.WithFields(taskProgressLogFields(task, snapshot)).WithField("notificationType", notificationType).Warn("task progress alert triggered")
	s.notifyRecipients(ctx, task, notificationType, title, body, payload)
	s.notifyIM(ctx, task, snapshot, title, body)
	return "alerted", true, false
}

func (s *TaskProgressService) notifyRecipients(ctx context.Context, task *model.Task, ntype, title, body, payload string) {
	if s.notifications == nil {
		return
	}

	seen := make(map[uuid.UUID]struct{}, 2)
	recipients := make([]uuid.UUID, 0, 2)
	for _, candidate := range []*uuid.UUID{task.AssigneeID, task.ReporterID} {
		if candidate == nil {
			continue
		}
		if _, exists := seen[*candidate]; exists {
			continue
		}
		seen[*candidate] = struct{}{}
		recipients = append(recipients, *candidate)
	}

	for _, recipient := range recipients {
		if _, err := s.notifications.Create(ctx, recipient, ntype, title, body, payload); err != nil {
			log.WithFields(log.Fields{
				"taskId":           task.ID.String(),
				"projectId":        task.ProjectID.String(),
				"recipientId":      recipient.String(),
				"notificationType": ntype,
			}).WithError(err).Warn("task progress notification creation failed")
		}
	}
}

func (s *TaskProgressService) notifyIM(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot, title, body string) {
	if s.imNotifier == nil {
		return
	}
	if err := s.imNotifier.NotifyTaskProgress(ctx, task, snapshot, title, body); err != nil {
		log.WithFields(taskProgressLogFields(task, snapshot)).WithError(err).Warn("task progress IM notify failed")
	}
}

func (s *TaskProgressService) loadSnapshot(ctx context.Context, task *model.Task) (*model.TaskProgressSnapshot, bool, error) {
	if task.Progress != nil {
		return cloneTaskProgressSnapshot(task.Progress), false, nil
	}

	snapshot, err := s.snapshotRepo.GetByTaskID(ctx, task.ID)
	if err == nil {
		return snapshot, false, nil
	}
	if errors.Is(err, ErrTaskProgressSnapshotNotFound) || errors.Is(err, pgx.ErrNoRows) {
		created := bootstrapTaskProgressSnapshot(task)
		return created, true, nil
	}
	return nil, false, err
}

func (s *TaskProgressService) isExemptStatus(status string) bool {
	_, ok := s.exemptStatuses[status]
	return ok
}

func (s *TaskProgressService) resolveRiskReason(task *model.Task) string {
	if task.Status == model.TaskStatusInReview {
		return model.TaskProgressReasonAwaitingReview
	}
	return model.TaskProgressReasonNoRecentUpdate
}

func (s *TaskProgressService) broadcastProgressUpdate(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot, transition string) {
	log.WithFields(taskProgressLogFields(task, snapshot)).WithField("transition", transition).Debug("task progress update broadcast")
	task.Progress = snapshot
	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskProgressUpdated, task.ProjectID.String(), map[string]any{
		"taskId":       task.ID.String(),
		"task":         task.ToDTO(),
		"progress":     snapshot.ToDTO(),
		"transition":   transition,
		"healthStatus": snapshot.HealthStatus,
		"riskReason":   snapshot.RiskReason,
	})
}

func (s *TaskProgressService) broadcastProgressAlert(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot) {
	log.WithFields(taskProgressLogFields(task, snapshot)).Warn("task progress alert broadcast")
	task.Progress = snapshot
	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskProgressAlerted, task.ProjectID.String(), map[string]any{
		"taskId":   task.ID.String(),
		"task":     task.ToDTO(),
		"progress": snapshot.ToDTO(),
	})
}

func (s *TaskProgressService) broadcastProgressRecovered(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot) {
	log.WithFields(taskProgressLogFields(task, snapshot)).Info("task progress recovery broadcast")
	task.Progress = snapshot
	_ = eventbus.PublishLegacy(ctx, s.bus, ws.EventTaskProgressRecovered, task.ProjectID.String(), map[string]any{
		"taskId":   task.ID.String(),
		"task":     task.ToDTO(),
		"progress": snapshot.ToDTO(),
	})
}

func normalizeTaskProgressConfig(cfg TaskProgressConfig) TaskProgressConfig {
	if cfg.WarningAfter <= 0 {
		cfg.WarningAfter = 2 * time.Hour
	}
	if cfg.StalledAfter <= 0 {
		cfg.StalledAfter = 4 * time.Hour
	}
	if cfg.AlertCooldown <= 0 {
		cfg.AlertCooldown = 30 * time.Minute
	}
	if cfg.DetectorInterval <= 0 {
		cfg.DetectorInterval = time.Minute
	}
	if len(cfg.ExemptStatuses) == 0 {
		cfg.ExemptStatuses = []string{
			model.TaskStatusBlocked,
			model.TaskStatusDone,
			model.TaskStatusCancelled,
		}
	}
	return cfg
}

func resolveTaskActivityTime(value time.Time, now func() time.Time) time.Time {
	if !value.IsZero() {
		return value.UTC()
	}
	return now().UTC()
}

func bootstrapTaskProgressSnapshot(task *model.Task) *model.TaskProgressSnapshot {
	lastTransition := task.CreatedAt
	if task.CompletedAt != nil && task.CompletedAt.After(lastTransition) {
		lastTransition = *task.CompletedAt
	} else if task.UpdatedAt.After(lastTransition) {
		lastTransition = task.UpdatedAt
	}

	lastActivityAt := task.UpdatedAt
	if lastActivityAt.IsZero() {
		lastActivityAt = task.CreatedAt
	}

	return &model.TaskProgressSnapshot{
		TaskID:             task.ID,
		LastActivityAt:     lastActivityAt,
		LastActivitySource: model.TaskProgressSourceTaskCreated,
		LastTransitionAt:   lastTransition,
		HealthStatus:       model.TaskProgressHealthHealthy,
		RiskReason:         model.TaskProgressReasonNone,
		LastAlertState:     "",
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
}

func cloneTaskProgressSnapshot(snapshot *model.TaskProgressSnapshot) *model.TaskProgressSnapshot {
	if snapshot == nil {
		return nil
	}
	clone := *snapshot
	if snapshot.RiskSinceAt != nil {
		value := *snapshot.RiskSinceAt
		clone.RiskSinceAt = &value
	}
	if snapshot.LastAlertAt != nil {
		value := *snapshot.LastAlertAt
		clone.LastAlertAt = &value
	}
	if snapshot.LastRecoveredAt != nil {
		value := *snapshot.LastRecoveredAt
		clone.LastRecoveredAt = &value
	}
	return &clone
}

func taskProgressSnapshotsEqual(a, b *model.TaskProgressSnapshot) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.TaskID == b.TaskID &&
		a.LastActivityAt.Equal(b.LastActivityAt) &&
		a.LastActivitySource == b.LastActivitySource &&
		a.LastTransitionAt.Equal(b.LastTransitionAt) &&
		a.HealthStatus == b.HealthStatus &&
		a.RiskReason == b.RiskReason &&
		timePointersEqual(a.RiskSinceAt, b.RiskSinceAt) &&
		a.LastAlertState == b.LastAlertState &&
		timePointersEqual(a.LastAlertAt, b.LastAlertAt) &&
		timePointersEqual(a.LastRecoveredAt, b.LastRecoveredAt)
}

func timePointersEqual(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func buildTaskProgressAlertMessage(task *model.Task, snapshot *model.TaskProgressSnapshot) (string, string, string) {
	reasonLabel := snapshot.RiskReason
	if reasonLabel == "" {
		reasonLabel = "unknown"
	}
	switch snapshot.HealthStatus {
	case model.TaskProgressHealthStalled:
		return fmt.Sprintf("Task stalled: %s", task.Title),
			fmt.Sprintf("Task %s is stalled (%s) while in %s.", task.Title, reasonLabel, task.Status),
			model.NotificationTypeTaskProgressStalled
	default:
		return fmt.Sprintf("Task at risk: %s", task.Title),
			fmt.Sprintf("Task %s needs attention (%s) while in %s.", task.Title, reasonLabel, task.Status),
			model.NotificationTypeTaskProgressWarning
	}
}

func taskProgressNotificationPayload(task *model.Task, snapshot *model.TaskProgressSnapshot) map[string]any {
	return map[string]any{
		"taskId":       task.ID.String(),
		"projectId":    task.ProjectID.String(),
		"title":        task.Title,
		"status":       task.Status,
		"healthStatus": snapshot.HealthStatus,
		"riskReason":   snapshot.RiskReason,
		"alertAt":      snapshot.LastAlertAt,
		"href":         fmt.Sprintf("/project?id=%s#task-%s", task.ProjectID, task.ID),
	}
}

func mustJSONMarshal(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(data)
}

type HTTPTaskProgressIMNotifier struct {
	client         *http.Client
	endpoint       string
	platform       string
	targetChatID   string
	deliverySecret string
}

type IMBoundProgressNotifier interface {
	QueueBoundProgress(ctx context.Context, req IMBoundProgressRequest) (bool, error)
}

func NewHTTPTaskProgressIMNotifier(endpoint, platform, targetChatID string, deliverySecret ...string) *HTTPTaskProgressIMNotifier {
	if strings.TrimSpace(endpoint) == "" || strings.TrimSpace(platform) == "" || strings.TrimSpace(targetChatID) == "" {
		return nil
	}
	secret := ""
	if len(deliverySecret) > 0 {
		secret = strings.TrimSpace(deliverySecret[0])
	}
	return &HTTPTaskProgressIMNotifier{
		client:         &http.Client{Timeout: 10 * time.Second},
		endpoint:       strings.TrimRight(strings.TrimSpace(endpoint), "/"),
		platform:       strings.TrimSpace(platform),
		targetChatID:   strings.TrimSpace(targetChatID),
		deliverySecret: secret,
	}
}

func (n *HTTPTaskProgressIMNotifier) NotifyTaskProgress(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot, title, body string) error {
	if n == nil {
		return nil
	}
	fields := taskProgressLogFields(task, snapshot)
	fields["platform"] = n.platform
	fields["targetChatId"] = n.targetChatID

	payload := map[string]any{
		"type":           "task_progress",
		"target_chat_id": n.targetChatID,
		"platform":       n.platform,
		"content": fmt.Sprintf("%s\n%s\nTask: %s\nStatus: %s\nHealth: %s",
			title,
			body,
			task.Title,
			task.Status,
			snapshot.HealthStatus,
		),
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		log.WithFields(fields).WithError(err).Warn("task progress HTTP IM notify marshal failed")
		return fmt.Errorf("marshal im notify payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint+"/im/notify", bytes.NewReader(bodyBytes))
	if err != nil {
		log.WithFields(fields).WithError(err).Warn("task progress HTTP IM notify request creation failed")
		return fmt.Errorf("build im notify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	applyCompatibilityDeliveryHeaders(req, "/im/notify", bodyBytes, n.deliverySecret)

	resp, err := n.client.Do(req)
	if err != nil {
		log.WithFields(fields).WithError(err).Warn("task progress HTTP IM notify request failed")
		return fmt.Errorf("send im notify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		log.WithFields(fields).WithField("status", resp.StatusCode).Warn("task progress HTTP IM notify returned non-OK status")
		return fmt.Errorf("im notify returned status %d", resp.StatusCode)
	}
	log.WithFields(fields).WithField("status", resp.StatusCode).Debug("task progress HTTP IM notify delivered")
	return nil
}

type ControlPlaneTaskProgressIMNotifier struct {
	control IMBoundProgressNotifier
}

func NewControlPlaneTaskProgressIMNotifier(control IMBoundProgressNotifier) *ControlPlaneTaskProgressIMNotifier {
	if control == nil {
		return nil
	}
	return &ControlPlaneTaskProgressIMNotifier{control: control}
}

func (n *ControlPlaneTaskProgressIMNotifier) NotifyTaskProgress(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot, title, body string) error {
	if n == nil || n.control == nil || task == nil || snapshot == nil {
		return nil
	}
	queued, err := n.control.QueueBoundProgress(ctx, IMBoundProgressRequest{
		TaskID:  task.ID.String(),
		Kind:    IMDeliveryKindProgress,
		Content: fmt.Sprintf("%s\n%s\nTask: %s\nStatus: %s\nHealth: %s", title, body, task.Title, task.Status, snapshot.HealthStatus),
		Structured: &model.IMStructuredMessage{
			Title: title,
			Body:  body,
			Fields: []model.IMStructuredField{
				{Label: "Task", Value: task.Title},
				{Label: "Status", Value: task.Status},
				{Label: "Health", Value: snapshot.HealthStatus},
			},
		},
	})
	fields := taskProgressLogFields(task, snapshot)
	fields["queued"] = queued
	if err != nil {
		log.WithFields(fields).WithError(err).Warn("task progress control-plane IM notify failed")
		return err
	}
	if queued {
		log.WithFields(fields).Debug("task progress control-plane IM notify queued")
	}
	return err
}

type MultiTaskProgressIMNotifier struct {
	notifiers []TaskProgressIMNotifier
}

func NewMultiTaskProgressIMNotifier(notifiers ...TaskProgressIMNotifier) *MultiTaskProgressIMNotifier {
	filtered := make([]TaskProgressIMNotifier, 0, len(notifiers))
	for _, notifier := range notifiers {
		if notifier != nil {
			filtered = append(filtered, notifier)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return &MultiTaskProgressIMNotifier{notifiers: filtered}
}

func (n *MultiTaskProgressIMNotifier) NotifyTaskProgress(ctx context.Context, task *model.Task, snapshot *model.TaskProgressSnapshot, title, body string) error {
	if n == nil {
		return nil
	}
	var firstErr error
	for _, notifier := range n.notifiers {
		if err := notifier.NotifyTaskProgress(ctx, task, snapshot, title, body); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
