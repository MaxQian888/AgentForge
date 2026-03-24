package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// BridgeIntentClassifier classifies natural language into structured intents.
type BridgeIntentClassifier interface {
	ClassifyIntent(ctx context.Context, req ClassifyIntentRequest) (*ClassifyIntentResponse, error)
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
	notifyURL  string
	platform   string
	httpClient *http.Client
	classifier BridgeIntentClassifier
	logger     *slog.Logger
	controlPlane *IMControlPlane
	deliverySecret string
}

// NewIMService creates an IM service with the given notify URL and platform.
func NewIMService(notifyURL, platform string, controlPlane ...*IMControlPlane) *IMService {
	service := &IMService{
		notifyURL: notifyURL,
		platform:  platform,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: slog.Default(),
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

// HandleIncoming processes a natural language IM message.
// Slash commands are acknowledged and routed back to the IM Bridge engine.
// Non-command messages are sent to the NLU classifier if available.
func (s *IMService) HandleIncoming(ctx context.Context, req *model.IMMessageRequest) (*model.IMMessageResponse, error) {
	text := strings.TrimSpace(req.Text)

	// Slash commands are handled by the IM Bridge engine directly;
	// this endpoint only receives non-command messages forwarded by the bridge.
	if strings.HasPrefix(text, "/") {
		parts := strings.SplitN(text, " ", 2)
		cmd := strings.TrimPrefix(parts[0], "/")
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
			s.logger.WarnContext(ctx, "classify intent failed, returning fallback", "error", err)
			return &model.IMMessageResponse{
				Reply: "理解失败，请使用 /help 查看可用命令。",
			}, nil
		}
		if intentResp.Reply != "" {
			return &model.IMMessageResponse{Reply: intentResp.Reply}, nil
		}
		return &model.IMMessageResponse{
			Reply: fmt.Sprintf("识别到意图: %s → %s %s", intentResp.Intent, intentResp.Command, intentResp.Args),
		}, nil
	}

	// No classifier available — return a helpful fallback.
	return &model.IMMessageResponse{
		Reply: "自然语言理解功能尚未就绪。请使用 /help 查看可用命令。",
	}, nil
}

// HandleCommand dispatches a structured slash command to the appropriate handler.
func (s *IMService) HandleCommand(_ context.Context, req *model.IMCommandRequest) (*model.IMCommandResponse, error) {
	// The IM Bridge handles command execution directly via its own command engine
	// and API client. This backend endpoint exists as a secondary route for
	// webhook-based IM platforms that POST commands here directly.
	switch req.Command {
	case "task":
		return &model.IMCommandResponse{Result: "Task command received. Use the IM Bridge for full execution.", Success: true}, nil
	case "agent":
		return &model.IMCommandResponse{Result: "Agent command received. Use the IM Bridge for full execution.", Success: true}, nil
	case "review":
		return &model.IMCommandResponse{Result: "Review command received. Use the IM Bridge for full execution.", Success: true}, nil
	case "sprint":
		return &model.IMCommandResponse{Result: "Sprint command received. Use the IM Bridge for full execution.", Success: true}, nil
	case "cost":
		return &model.IMCommandResponse{Result: "Cost command received. Use the IM Bridge for full execution.", Success: true}, nil
	default:
		return &model.IMCommandResponse{Result: fmt.Sprintf("Unknown command: %s", req.Command), Success: false}, nil
	}
}

// HandleIntent processes a natural language intent request from the IM Bridge.
// This endpoint is called by the IM Bridge's NLU fallback handler.
func (s *IMService) HandleIntent(ctx context.Context, req *model.IMIntentRequest) (*model.IMIntentResponse, error) {
	if s.classifier == nil {
		return &model.IMIntentResponse{
			Reply: "自然语言理解功能尚未就绪。请使用 /help 查看可用命令。",
		}, nil
	}

	intentResp, err := s.classifier.ClassifyIntent(ctx, ClassifyIntentRequest{
		Text:      req.Text,
		UserID:    req.UserID,
		ProjectID: req.ProjectID,
	})
	if err != nil {
		s.logger.WarnContext(ctx, "classify intent failed", "error", err)
		return &model.IMIntentResponse{
			Reply: "理解失败，请使用 /help 查看可用命令。",
		}, nil
	}

	reply := intentResp.Reply
	if reply == "" {
		reply = fmt.Sprintf("识别到意图: %s → %s %s", intentResp.Intent, intentResp.Command, intentResp.Args)
	}
	return &model.IMIntentResponse{
		Reply:  reply,
		Intent: intentResp.Intent,
	}, nil
}

// HandleAction processes a button click action from an IM card.
func (s *IMService) HandleAction(_ context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
	// Actions follow the format "assign-agent", "decompose", "approve", etc.
	// The IM Bridge handles the actual API calls; this endpoint provides a
	// backend-side hook for direct webhook callbacks from IM platforms.
	switch req.Action {
	case "assign-agent":
		return &model.IMActionResponse{
			Result:  fmt.Sprintf("Agent assignment requested for entity %s", req.EntityID),
			Success: true,
		}, nil
	case "decompose":
		return &model.IMActionResponse{
			Result:  fmt.Sprintf("Task decomposition requested for %s", req.EntityID),
			Success: true,
		}, nil
	case "approve":
		return &model.IMActionResponse{
			Result:  fmt.Sprintf("Approval recorded for %s", req.EntityID),
			Success: true,
		}, nil
	case "request-changes":
		return &model.IMActionResponse{
			Result:  fmt.Sprintf("Change request recorded for %s", req.EntityID),
			Success: true,
		}, nil
	default:
		return &model.IMActionResponse{
			Result:  fmt.Sprintf("Unknown action: %s", req.Action),
			Success: false,
		}, nil
	}
}

// Send delivers a message to an IM channel via the IM Bridge notification receiver.
func (s *IMService) Send(ctx context.Context, req *model.IMSendRequest) error {
	if s.controlPlane != nil {
		_, err := s.controlPlane.QueueDelivery(ctx, IMQueueDeliveryRequest{
			TargetBridgeID: strings.TrimSpace(req.BridgeID),
			Platform:       req.Platform,
			ProjectID:      req.ProjectID,
			Kind:           IMDeliveryKindSend,
			Content:        req.Text,
			TargetChatID:   req.ChannelID,
			ReplyTarget:    req.ReplyTarget,
		})
		if err == nil {
			return nil
		}
		s.logger.WarnContext(ctx, "IM control-plane send failed, falling back to compatibility HTTP", "error", err)
	}

	if s.notifyURL == "" {
		s.logger.WarnContext(ctx, "IM send skipped: no notify URL configured")
		return nil
	}

	payload := map[string]string{
		"platform":   req.Platform,
		"chat_id":    req.ChannelID,
		"content":    req.Text,
		"thread_id":  req.ThreadID,
	}
	body, err := json.Marshal(payload)
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
		return fmt.Errorf("IM send failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IM send returned %d", resp.StatusCode)
	}
	return nil
}

// Notify delivers a notification event to an IM channel via the IM Bridge.
func (s *IMService) Notify(ctx context.Context, req *model.IMNotifyRequest) error {
	if s.controlPlane != nil {
		content := strings.TrimSpace(strings.TrimSpace(req.Title) + "\n" + strings.TrimSpace(req.Body))
		_, err := s.controlPlane.QueueDelivery(ctx, IMQueueDeliveryRequest{
			TargetBridgeID: strings.TrimSpace(req.BridgeID),
			Platform:       req.Platform,
			ProjectID:      req.ProjectID,
			Kind:           IMDeliveryKindNotify,
			Content:        strings.TrimSpace(content),
			TargetChatID:   req.ChannelID,
			ReplyTarget:    req.ReplyTarget,
		})
		if err == nil {
			return nil
		}
		s.logger.WarnContext(ctx, "IM control-plane notify failed, falling back to compatibility HTTP", "error", err)
	}

	if s.notifyURL == "" {
		s.logger.WarnContext(ctx, "IM notify skipped: no notify URL configured")
		return nil
	}

	payload := map[string]any{
		"type":            req.Event,
		"platform":        req.Platform,
		"target_chat_id":  req.ChannelID,
		"content":         fmt.Sprintf("%s\n%s", req.Title, req.Body),
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
		return fmt.Errorf("IM notify failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("IM notify returned %d", resp.StatusCode)
	}
	return nil
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
