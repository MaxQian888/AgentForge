package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/sirupsen/logrus"
)

// BridgeIntentClassifier classifies natural language into structured intents.
type BridgeIntentClassifier interface {
	ClassifyIntent(ctx context.Context, req ClassifyIntentRequest) (*ClassifyIntentResponse, error)
}

// IMTaskCreator creates tasks from IM command input.
type IMTaskCreator interface {
	Create(ctx context.Context, task *model.Task) error
}

// IMAgentSpawner spawns agent runs from IM command input.
type IMAgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
}

// IMReviewTrigger triggers code reviews from IM command input.
type IMReviewTrigger interface {
	Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
}

// ClassifyIntentRequest is sent to the TS Bridge for NLU.
type ClassifyIntentRequest struct {
	Text      string `json:"text"`
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
}

// ClassifyIntentResponse is the NLU result from the TS Bridge.
type ClassifyIntentResponse struct {
	Intent     string  `json:"intent"`
	Command    string  `json:"command"`
	Args       string  `json:"args"`
	Confidence float64 `json:"confidence"`
	Reply      string  `json:"reply,omitempty"`
}

// IMService handles IM Bridge message processing, command dispatch,
// and outbound message delivery.
type IMService struct {
	notifyURL      string
	platform       string
	httpClient     *http.Client
	classifier     BridgeIntentClassifier
	logger         *logrus.Logger
	controlPlane   *IMControlPlane
	deliverySecret string
	taskCreator    IMTaskCreator
	agentSpawner   IMAgentSpawner
	reviewTrigger  IMReviewTrigger
	actionExecutor IMActionExecutor
}

// NewIMService creates an IM service with the given notify URL and platform.
func NewIMService(notifyURL, platform string, controlPlane ...*IMControlPlane) *IMService {
	service := &IMService{
		notifyURL: notifyURL,
		platform:  platform,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logrus.StandardLogger(),
	}
	if len(controlPlane) > 0 {
		service.controlPlane = controlPlane[0]
	}
	return service
}

// SetClassifier sets the NLU intent classifier (TS Bridge client).
func (s *IMService) SetClassifier(c BridgeIntentClassifier) {
	s.classifier = c
}

func (s *IMService) SetDeliverySecret(secret string) {
	s.deliverySecret = strings.TrimSpace(secret)
}

// SetTaskCreator sets the optional task creation dependency for IM commands.
func (s *IMService) SetTaskCreator(tc IMTaskCreator) {
	s.taskCreator = tc
}

// SetAgentSpawner sets the optional agent spawner dependency for IM commands.
func (s *IMService) SetAgentSpawner(sp IMAgentSpawner) {
	s.agentSpawner = sp
}

// SetReviewTrigger sets the optional review trigger dependency for IM commands.
func (s *IMService) SetReviewTrigger(rt IMReviewTrigger) {
	s.reviewTrigger = rt
}

// SetActionExecutor sets the optional backend action execution seam for
// interactive IM callbacks.
func (s *IMService) SetActionExecutor(executor IMActionExecutor) {
	s.actionExecutor = executor
}

// HandleIncoming processes a natural language IM message.
// Slash commands are acknowledged and routed back to the IM Bridge engine.
// Non-command messages are sent to the NLU classifier if available.
func (s *IMService) HandleIncoming(ctx context.Context, req *model.IMMessageRequest) (*model.IMMessageResponse, error) {
	text := strings.TrimSpace(req.Text)
	entry := s.logger.WithFields(logrus.Fields{
		"platform":  req.Platform,
		"channelId": req.ChannelID,
		"userId":    req.UserID,
		"threadId":  req.ThreadID,
	})

	// Slash commands are handled by the IM Bridge engine directly;
	// this endpoint only receives non-command messages forwarded by the bridge.
	if strings.HasPrefix(text, "/") {
		parts := strings.SplitN(text, " ", 2)
		cmd := strings.TrimPrefix(parts[0], "/")
		entry.WithField("command", cmd).Info("IM slash command forwarded to bridge")
		return &model.IMMessageResponse{
			Reply: fmt.Sprintf("Command '%s' received. Processing via AgentForge.", cmd),
		}, nil
	}

	// Try NLU classification via the TS Bridge.
	if s.classifier != nil {
		intentResp, err := s.classifier.ClassifyIntent(ctx, ClassifyIntentRequest{
			Text:   text,
			UserID: req.UserID,
		})
		if err != nil {
			entry.WithError(err).Warn("IM intent classification failed, returning fallback")
			return &model.IMMessageResponse{
				Reply: "理解失败，请使用 /help 查看可用命令。",
			}, nil
		}
		entry.WithFields(logrus.Fields{
			"intent":     intentResp.Intent,
			"command":    intentResp.Command,
			"confidence": intentResp.Confidence,
			"hasReply":   intentResp.Reply != "",
		}).Info("IM intent classified")
		if intentResp.Reply != "" {
			return &model.IMMessageResponse{Reply: intentResp.Reply}, nil
		}
		return &model.IMMessageResponse{
			Reply: fmt.Sprintf("识别到意图: %s → %s %s", intentResp.Intent, intentResp.Command, intentResp.Args),
		}, nil
	}

	// No classifier available — return a helpful fallback.
	entry.Warn("IM intent classification skipped: classifier unavailable")
	return &model.IMMessageResponse{
		Reply: "自然语言理解功能尚未就绪。请使用 /help 查看可用命令。",
	}, nil
}

// HandleCommand dispatches a structured slash command to the appropriate handler.
func (s *IMService) HandleCommand(ctx context.Context, req *model.IMCommandRequest) (*model.IMCommandResponse, error) {
	entry := s.logger.WithFields(logrus.Fields{
		"platform":  req.Platform,
		"channelId": req.ChannelID,
		"userId":    req.UserID,
		"command":   req.Command,
	})

	// The IM Bridge handles command execution directly via its own command engine
	// and API client. This backend endpoint exists as a secondary route for
	// webhook-based IM platforms that POST commands here directly.
	switch req.Command {
	case "task":
		return s.handleTaskCommand(ctx, entry, req)
	case "agent":
		return s.handleAgentCommand(ctx, entry, req)
	case "review":
		return s.handleReviewCommand(ctx, entry, req)
	case "sprint":
		entry.Info("IM command acknowledged")
		return &model.IMCommandResponse{Result: "Sprint command received. Use the IM Bridge for full execution.", Success: true}, nil
	case "cost":
		entry.Info("IM command acknowledged")
		return &model.IMCommandResponse{Result: "Cost command received. Use the IM Bridge for full execution.", Success: true}, nil
	default:
		entry.Warn("IM command rejected: unknown command")
		return &model.IMCommandResponse{Result: fmt.Sprintf("Unknown command: %s", req.Command), Success: false}, nil
	}
}

// HandleIntent processes a natural language intent request from the IM Bridge.
// This endpoint is called by the IM Bridge's NLU fallback handler.
func (s *IMService) HandleIntent(ctx context.Context, req *model.IMIntentRequest) (*model.IMIntentResponse, error) {
	entry := s.logger.WithFields(logrus.Fields{
		"userId":    req.UserID,
		"projectId": req.ProjectID,
	})

	if s.classifier == nil {
		entry.Warn("IM intent request skipped: classifier unavailable")
		return fallbackIMIntentResponse(req.Text), nil
	}

	intentResp, err := s.classifier.ClassifyIntent(ctx, ClassifyIntentRequest{
		Text:      req.Text,
		UserID:    req.UserID,
		ProjectID: req.ProjectID,
	})
	if err != nil {
		entry.WithError(err).Warn("IM intent classification failed")
		return fallbackIMIntentResponse(req.Text), nil
	}
	entry.WithFields(logrus.Fields{
		"intent":     intentResp.Intent,
		"command":    intentResp.Command,
		"confidence": intentResp.Confidence,
		"hasReply":   intentResp.Reply != "",
	}).Info("IM intent request classified")

	reply := intentResp.Reply
	if reply == "" {
		reply = fmt.Sprintf("识别到意图: %s → %s %s", intentResp.Intent, intentResp.Command, intentResp.Args)
	}
	return &model.IMIntentResponse{
		Reply:  reply,
		Intent: intentResp.Intent,
	}, nil
}

func fallbackIMIntentResponse(text string) *model.IMIntentResponse {
	trimmed := strings.TrimSpace(text)
	runID := regexp.MustCompile(`run-[A-Za-z0-9_-]+`).FindString(trimmed)
	lower := strings.ToLower(trimmed)

	switch {
	case strings.Contains(trimmed, "暂停") || strings.Contains(lower, "pause"):
		command := "/agent pause"
		if runID != "" {
			command += " " + runID
		}
		return &model.IMIntentResponse{Reply: fmt.Sprintf("我建议先使用 %s", command), Intent: "agent_pause"}
	case strings.Contains(trimmed, "恢复") || strings.Contains(lower, "resume"):
		command := "/agent resume"
		if runID != "" {
			command += " " + runID
		}
		return &model.IMIntentResponse{Reply: fmt.Sprintf("我建议先使用 %s", command), Intent: "agent_resume"}
	case strings.Contains(trimmed, "终止") || strings.Contains(trimmed, "停止") || strings.Contains(lower, "kill"):
		command := "/agent kill"
		if runID != "" {
			command += " " + runID
		}
		return &model.IMIntentResponse{Reply: fmt.Sprintf("我建议先使用 %s", command), Intent: "agent_kill"}
	case strings.Contains(trimmed, "队列"):
		return &model.IMIntentResponse{Reply: "我建议先使用 /queue list", Intent: "queue_list"}
	case strings.Contains(trimmed, "成员") || strings.Contains(trimmed, "团队"):
		return &model.IMIntentResponse{Reply: "我建议先使用 /team list", Intent: "team_list"}
	default:
		return &model.IMIntentResponse{Reply: "自然语言理解功能尚未就绪。请使用 /help 查看可用命令。"}
	}
}

// HandleAction processes a button click action from an IM card.
func (s *IMService) HandleAction(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
	entry := s.logger.WithFields(logrus.Fields{
		"platform":  req.Platform,
		"channelId": req.ChannelID,
		"userId":    req.UserID,
		"bridgeId":  req.BridgeID,
		"action":    req.Action,
		"entityId":  req.EntityID,
	})

	if s.actionExecutor != nil {
		resp, err := s.actionExecutor.Execute(ctx, req)
		if err != nil {
			entry.WithError(err).Error("IM action execution failed")
			return nil, err
		}
		if resp != nil {
			entry.WithField("status", resp.Status).Info("IM action executed")
			return resp, nil
		}
	}

	// Executor was wired but returned nil — action not recognized.
	entry.Warn("IM action not handled by executor")
	return &model.IMActionResponse{
		Result:      fmt.Sprintf("Action %q not handled by executor.", req.Action),
		Success:     false,
		Status:      model.IMActionStatusFailed,
		ReplyTarget: req.ReplyTarget,
		Metadata:    cloneStringMap(req.Metadata),
	}, nil
}

// Send delivers a message to an IM channel via the IM Bridge notification receiver.
func (s *IMService) Send(ctx context.Context, req *model.IMSendRequest) error {
	entry := s.logger.WithFields(logrus.Fields{
		"platform":       req.Platform,
		"channelId":      req.ChannelID,
		"projectId":      req.ProjectID,
		"bridgeId":       req.BridgeID,
		"deliveryId":     req.DeliveryID,
		"hasReplyTarget": req.ReplyTarget != nil,
		"hasThreadId":    strings.TrimSpace(req.ThreadID) != "",
	})

	if s.controlPlane != nil {
		delivery, err := s.controlPlane.QueueDelivery(ctx, IMQueueDeliveryRequest{
			DeliveryID:     strings.TrimSpace(req.DeliveryID),
			TargetBridgeID: strings.TrimSpace(req.BridgeID),
			Platform:       req.Platform,
			ProjectID:      req.ProjectID,
			Kind:           IMDeliveryKindSend,
			Content:        req.Text,
			Structured:     req.Structured,
			Native:         req.Native,
			Metadata:       req.Metadata,
			TargetChatID:   req.ChannelID,
			ReplyTarget:    req.ReplyTarget,
		})
		if err == nil {
			record := buildSendDeliveryRecord(req, model.IMDeliveryStatusPending, "")
			record.ID = strings.TrimSpace(delivery.DeliveryID)
			s.controlPlane.RecordDeliveryResult(record)
			entry.Info("IM send queued via control plane")
			return nil
		}
		entry.WithError(err).Warn("IM control-plane send failed, falling back to compatibility HTTP")
	}

	if s.notifyURL == "" {
		if s.controlPlane != nil {
			s.controlPlane.RecordDeliveryResult(buildSendDeliveryRecord(req, model.IMDeliveryStatusFailed, "notify URL not configured"))
		}
		entry.Warn("IM send skipped: no notify URL configured")
		return nil
	}

	compatPayload := map[string]any{
		"platform":    req.Platform,
		"chat_id":     req.ChannelID,
		"content":     req.Text,
		"structured":  req.Structured,
		"native":      req.Native,
		"metadata":    req.Metadata,
		"thread_id":   req.ThreadID,
		"replyTarget": req.ReplyTarget,
	}
	body, err := json.Marshal(compatPayload)
	if err != nil {
		return fmt.Errorf("marshal IM send payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.notifyURL+"/im/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create IM send request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyCompatibilityDeliveryHeaders(httpReq, "/im/send", body, s.deliverySecret)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		if s.controlPlane != nil {
			s.controlPlane.RecordDeliveryResult(buildSendDeliveryRecord(req, model.IMDeliveryStatusFailed, err.Error()))
		}
		entry.WithError(err).Error("IM send compatibility HTTP failed")
		return fmt.Errorf("IM send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if s.controlPlane != nil {
			s.controlPlane.RecordDeliveryResult(buildSendDeliveryRecord(req, model.IMDeliveryStatusFailed, fmt.Sprintf("HTTP %d", resp.StatusCode)))
		}
		entry.WithField("status", resp.StatusCode).Warn("IM send compatibility HTTP returned non-OK status")
		return fmt.Errorf("IM send returned %d", resp.StatusCode)
	}
	if s.controlPlane != nil {
		s.controlPlane.RecordDeliveryResult(buildSendDeliveryRecord(req, model.IMDeliveryStatusDelivered, ""))
	}
	entry.WithField("status", resp.StatusCode).Info("IM send delivered via compatibility HTTP")
	return nil
}

// Notify delivers a notification event to an IM channel via the IM Bridge.
func (s *IMService) Notify(ctx context.Context, req *model.IMNotifyRequest) error {
	entry := s.logger.WithFields(logrus.Fields{
		"platform":       req.Platform,
		"channelId":      req.ChannelID,
		"projectId":      req.ProjectID,
		"bridgeId":       req.BridgeID,
		"deliveryId":     req.DeliveryID,
		"event":          req.Event,
		"hasReplyTarget": req.ReplyTarget != nil,
	})

	if s.controlPlane != nil {
		content := buildNotifyContent(req)
		delivery, err := s.controlPlane.QueueDelivery(ctx, IMQueueDeliveryRequest{
			DeliveryID:     strings.TrimSpace(req.DeliveryID),
			TargetBridgeID: strings.TrimSpace(req.BridgeID),
			Platform:       req.Platform,
			ProjectID:      req.ProjectID,
			Kind:           IMDeliveryKindNotify,
			Content:        content,
			Structured:     req.Structured,
			Native:         req.Native,
			Metadata:       req.Metadata,
			TargetChatID:   req.ChannelID,
			ReplyTarget:    req.ReplyTarget,
		})
		if err == nil {
			record := buildNotifyDeliveryRecord(req, model.IMDeliveryStatusPending, "")
			record.ID = strings.TrimSpace(delivery.DeliveryID)
			s.controlPlane.RecordDeliveryResult(record)
			entry.Info("IM notify queued via control plane")
			return nil
		}
		entry.WithError(err).Warn("IM control-plane notify failed, falling back to compatibility HTTP")
	}

	if s.notifyURL == "" {
		if s.controlPlane != nil {
			s.controlPlane.RecordDeliveryResult(buildNotifyDeliveryRecord(req, model.IMDeliveryStatusFailed, "notify URL not configured"))
		}
		entry.Warn("IM notify skipped: no notify URL configured")
		return nil
	}

	payload := map[string]any{
		"type":           req.Event,
		"platform":       req.Platform,
		"target_chat_id": req.ChannelID,
		"content":        buildNotifyContent(req),
		"structured":     req.Structured,
		"native":         req.Native,
		"metadata":       req.Metadata,
		"replyTarget":    req.ReplyTarget,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal IM notify payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.notifyURL+"/im/notify", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create IM notify request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	applyCompatibilityDeliveryHeaders(httpReq, "/im/notify", body, s.deliverySecret)

	resp, err := s.httpClient.Do(httpReq)
	if err != nil {
		if s.controlPlane != nil {
			s.controlPlane.RecordDeliveryResult(buildNotifyDeliveryRecord(req, model.IMDeliveryStatusFailed, err.Error()))
		}
		entry.WithError(err).Error("IM notify compatibility HTTP failed")
		return fmt.Errorf("IM notify failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if s.controlPlane != nil {
			s.controlPlane.RecordDeliveryResult(buildNotifyDeliveryRecord(req, model.IMDeliveryStatusFailed, fmt.Sprintf("HTTP %d", resp.StatusCode)))
		}
		entry.WithField("status", resp.StatusCode).Warn("IM notify compatibility HTTP returned non-OK status")
		return fmt.Errorf("IM notify returned %d", resp.StatusCode)
	}
	if s.controlPlane != nil {
		s.controlPlane.RecordDeliveryResult(buildNotifyDeliveryRecord(req, model.IMDeliveryStatusDelivered, ""))
	}
	entry.WithField("status", resp.StatusCode).Info("IM notify delivered via compatibility HTTP")
	return nil
}

func (s *IMService) handleTaskCommand(ctx context.Context, entry *logrus.Entry, req *model.IMCommandRequest) (*model.IMCommandResponse, error) {
	if s.taskCreator == nil {
		entry.Info("IM command acknowledged (task creator unavailable)")
		return &model.IMCommandResponse{Result: "Task command received. Use the IM Bridge for full execution.", Success: true}, nil
	}

	title, _ := req.Args["title"].(string)
	if title == "" {
		title = "Task from IM"
	}
	description, _ := req.Args["description"].(string)

	task := &model.Task{
		ID:          uuid.New(),
		Title:       title,
		Description: description,
		Status:      "inbox",
	}

	if projectIDStr, ok := req.Args["projectId"].(string); ok {
		if pid, err := uuid.Parse(projectIDStr); err == nil {
			task.ProjectID = pid
		}
	}

	if err := s.taskCreator.Create(ctx, task); err != nil {
		entry.WithError(err).Error("IM task creation failed")
		return &model.IMCommandResponse{Result: fmt.Sprintf("Task creation failed: %s", err.Error()), Success: false}, nil
	}

	entry.WithField("taskId", task.ID).Info("IM task created")
	return &model.IMCommandResponse{
		Result:  fmt.Sprintf("Task created: %s (ID: %s)", task.Title, task.ID),
		Success: true,
	}, nil
}

func (s *IMService) handleAgentCommand(ctx context.Context, entry *logrus.Entry, req *model.IMCommandRequest) (*model.IMCommandResponse, error) {
	if s.agentSpawner == nil {
		entry.Info("IM command acknowledged (agent spawner unavailable)")
		return &model.IMCommandResponse{Result: "Agent command received. Use the IM Bridge for full execution.", Success: true}, nil
	}

	taskIDStr, _ := req.Args["taskId"].(string)
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		return &model.IMCommandResponse{Result: "Invalid or missing taskId argument.", Success: false}, nil
	}

	memberIDStr, _ := req.Args["memberId"].(string)
	memberID, err := uuid.Parse(memberIDStr)
	if err != nil {
		return &model.IMCommandResponse{Result: "Invalid or missing memberId argument.", Success: false}, nil
	}

	runtime, _ := req.Args["runtime"].(string)
	provider, _ := req.Args["provider"].(string)
	modelName, _ := req.Args["model"].(string)
	roleID, _ := req.Args["roleId"].(string)
	budgetUsd := 0.0
	if b, ok := req.Args["budgetUsd"].(float64); ok {
		budgetUsd = b
	}

	run, err := s.agentSpawner.Spawn(ctx, taskID, memberID, runtime, provider, modelName, budgetUsd, roleID)
	if err != nil {
		entry.WithError(err).Error("IM agent spawn failed")
		return &model.IMCommandResponse{Result: fmt.Sprintf("Agent spawn failed: %s", err.Error()), Success: false}, nil
	}

	entry.WithField("runId", run.ID).Info("IM agent spawned")
	return &model.IMCommandResponse{
		Result:  fmt.Sprintf("Agent spawned: run %s (status: %s)", run.ID, run.Status),
		Success: true,
	}, nil
}

func (s *IMService) handleReviewCommand(ctx context.Context, entry *logrus.Entry, req *model.IMCommandRequest) (*model.IMCommandResponse, error) {
	if s.reviewTrigger == nil {
		entry.Info("IM command acknowledged (review trigger unavailable)")
		return &model.IMCommandResponse{Result: "Review command received. Use the IM Bridge for full execution.", Success: true}, nil
	}

	triggerReq := &model.TriggerReviewRequest{
		Trigger: "manual",
	}
	if taskIDStr, ok := req.Args["taskId"].(string); ok {
		triggerReq.TaskID = taskIDStr
	}
	if prURL, ok := req.Args["prUrl"].(string); ok {
		triggerReq.PRURL = prURL
	}

	review, err := s.reviewTrigger.Trigger(ctx, triggerReq)
	if err != nil {
		entry.WithError(err).Error("IM review trigger failed")
		return &model.IMCommandResponse{Result: fmt.Sprintf("Review trigger failed: %s", err.Error()), Success: false}, nil
	}

	entry.WithField("reviewId", review.ID).Info("IM review triggered")
	return &model.IMCommandResponse{
		Result:  fmt.Sprintf("Review triggered: %s (recommendation: %s)", review.ID, review.Recommendation),
		Success: true,
	}, nil
}

func applyCompatibilityDeliveryHeaders(req *http.Request, path string, body []byte, secret string) {
	if req == nil {
		return
	}
	deliveryID := uuid.NewString()
	timestamp := time.Now().UTC().Format(time.RFC3339)
	req.Header.Set("X-AgentForge-Delivery-Id", deliveryID)
	req.Header.Set("X-AgentForge-Delivery-Timestamp", timestamp)
	if strings.TrimSpace(secret) == "" {
		return
	}
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(strings.Join([]string{
		req.Method,
		path,
		deliveryID,
		timestamp,
		string(body),
	}, "|")))
	req.Header.Set("X-AgentForge-Signature", hex.EncodeToString(mac.Sum(nil)))
}

func buildNotifyContent(req *model.IMNotifyRequest) string {
	if req == nil {
		return ""
	}
	if req.Structured != nil {
		if fallback := strings.TrimSpace(req.Structured.FallbackText()); fallback != "" {
			return fallback
		}
	}
	return strings.TrimSpace(strings.TrimSpace(req.Title) + "\n" + strings.TrimSpace(req.Body))
}

func buildSendDeliveryRecord(req *model.IMSendRequest, status model.IMDeliveryStatus, failureReason string) model.IMDelivery {
	if req == nil {
		return model.IMDelivery{}
	}
	return model.IMDelivery{
		ID:            strings.TrimSpace(req.DeliveryID),
		BridgeID:      strings.TrimSpace(req.BridgeID),
		ProjectID:     strings.TrimSpace(req.ProjectID),
		ChannelID:     strings.TrimSpace(req.ChannelID),
		TargetChatID:  strings.TrimSpace(req.ChannelID),
		Platform:      req.Platform,
		EventType:     "message.send",
		Kind:          IMDeliveryKindSend,
		Status:        status,
		FailureReason: strings.TrimSpace(failureReason),
		Content:       strings.TrimSpace(req.Text),
		Structured:    req.Structured,
		Native:        req.Native,
		Metadata:      cloneStringMap(req.Metadata),
		ReplyTarget:   req.ReplyTarget,
	}
}

func buildNotifyDeliveryRecord(req *model.IMNotifyRequest, status model.IMDeliveryStatus, failureReason string) model.IMDelivery {
	if req == nil {
		return model.IMDelivery{}
	}
	return model.IMDelivery{
		ID:            strings.TrimSpace(req.DeliveryID),
		BridgeID:      strings.TrimSpace(req.BridgeID),
		ProjectID:     strings.TrimSpace(req.ProjectID),
		ChannelID:     strings.TrimSpace(req.ChannelID),
		TargetChatID:  strings.TrimSpace(req.ChannelID),
		Platform:      req.Platform,
		EventType:     strings.TrimSpace(req.Event),
		Kind:          IMDeliveryKindNotify,
		Status:        status,
		FailureReason: strings.TrimSpace(failureReason),
		Content:       buildNotifyContent(req),
		Structured:    req.Structured,
		Native:        req.Native,
		Metadata:      cloneStringMap(req.Metadata),
		ReplyTarget:   req.ReplyTarget,
	}
}
