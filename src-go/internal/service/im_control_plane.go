package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

const (
	IMDeliveryKindSend     = "send"
	IMDeliveryKindNotify   = "notify"
	IMDeliveryKindProgress = "progress"
	IMDeliveryKindTerminal = "terminal"
)

var (
	ErrIMBridgeNotFound     = errors.New("im bridge instance not found")
	ErrIMBridgeUnavailable  = errors.New("im bridge instance unavailable")
	ErrIMDeliveryRejected   = errors.New("im delivery rejected")
	ErrIMActionBindingEmpty = errors.New("im action binding requires at least one entity id")
)

type IMControlPlaneConfig struct {
	HeartbeatTTL              time.Duration
	ProgressHeartbeatInterval time.Duration
	DeliverySecret            string
	Now                       func() time.Time
}

type IMBridgeDeliveryListener = ws.IMBridgeListener

type IMBridgeRegisterRequest = model.IMBridgeRegisterRequest
type IMBridgeInstance = model.IMBridgeInstance
type IMControlDelivery = model.IMControlDelivery
type IMActionBinding = model.IMActionBinding

type IMQueueDeliveryRequest struct {
	TargetBridgeID string
	Platform       string
	ProjectID      string
	Kind           string
	Content        string
	TargetChatID   string
	ReplyTarget    *model.IMReplyTarget
}

type IMBoundProgressRequest struct {
	TaskID     string
	RunID      string
	ReviewID   string
	Kind       string
	Content    string
	IsTerminal bool
}

type IMControlPlane struct {
	mu sync.Mutex

	heartbeatTTL              time.Duration
	progressHeartbeatInterval time.Duration
	deliverySecret            string
	now                       func() time.Time

	instances  map[string]*bridgeInstanceState
	listeners  map[string]IMBridgeDeliveryListener
	pending    map[string][]*model.IMControlDelivery
	nextCursor int64

	actionByTask   map[string]*boundActionState
	actionByRun    map[string]*boundActionState
	actionByReview map[string]*boundActionState
}

func imBridgeFields(record *model.IMBridgeInstance) log.Fields {
	if record == nil {
		return log.Fields{}
	}
	return log.Fields{
		"bridgeId":     record.BridgeID,
		"platform":     record.Platform,
		"transport":    record.Transport,
		"status":       record.Status,
		"projectCount": len(record.ProjectIDs),
	}
}

func imDeliveryFields(delivery *model.IMControlDelivery) log.Fields {
	if delivery == nil {
		return log.Fields{}
	}
	return log.Fields{
		"bridgeId":   delivery.TargetBridgeID,
		"deliveryId": delivery.DeliveryID,
		"cursor":     delivery.Cursor,
		"platform":   delivery.Platform,
		"projectId":  delivery.ProjectID,
		"kind":       delivery.Kind,
	}
}

type bridgeInstanceState struct {
	record *model.IMBridgeInstance
}

type boundActionState struct {
	binding         *model.IMActionBinding
	lastHeartbeatAt time.Time
}

func NewIMControlPlane(cfg IMControlPlaneConfig) *IMControlPlane {
	if cfg.HeartbeatTTL <= 0 {
		cfg.HeartbeatTTL = 2 * time.Minute
	}
	if cfg.ProgressHeartbeatInterval <= 0 {
		cfg.ProgressHeartbeatInterval = 30 * time.Second
	}
	if cfg.Now == nil {
		cfg.Now = func() time.Time { return time.Now().UTC() }
	}
	return &IMControlPlane{
		heartbeatTTL:              cfg.HeartbeatTTL,
		progressHeartbeatInterval: cfg.ProgressHeartbeatInterval,
		deliverySecret:            strings.TrimSpace(cfg.DeliverySecret),
		now:                       cfg.Now,
		instances:                 make(map[string]*bridgeInstanceState),
		listeners:                 make(map[string]IMBridgeDeliveryListener),
		pending:                   make(map[string][]*model.IMControlDelivery),
		actionByTask:              make(map[string]*boundActionState),
		actionByRun:               make(map[string]*boundActionState),
		actionByReview:            make(map[string]*boundActionState),
	}
}

func (s *IMControlPlane) RegisterBridge(_ context.Context, req *IMBridgeRegisterRequest) (*model.IMBridgeInstance, error) {
	if req == nil || strings.TrimSpace(req.BridgeID) == "" {
		log.Warn("IM control plane register rejected: missing bridge id")
		return nil, ErrIMBridgeNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record := &model.IMBridgeInstance{
		BridgeID:         strings.TrimSpace(req.BridgeID),
		Platform:         normalizePlatform(req.Platform),
		Transport:        strings.TrimSpace(req.Transport),
		ProjectIDs:       dedupeStrings(req.ProjectIDs),
		Capabilities:     cloneBoolMap(req.Capabilities),
		CapabilityMatrix: cloneAnyMap(req.CapabilityMatrix),
		CallbackPaths:    dedupeStrings(req.CallbackPaths),
		Metadata:         cloneStringMap(req.Metadata),
		Status:           "online",
	}
	s.applyHeartbeat(record)
	s.instances[record.BridgeID] = &bridgeInstanceState{record: record}
	log.WithFields(imBridgeFields(record)).Info("IM control plane bridge registered")
	return cloneBridgeInstance(record), nil
}

func (s *IMControlPlane) RecordHeartbeat(_ context.Context, bridgeID string) (*model.IMBridgeHeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance, ok := s.instances[strings.TrimSpace(bridgeID)]
	if !ok {
		log.WithField("bridgeId", strings.TrimSpace(bridgeID)).Warn("IM control plane heartbeat failed: bridge not found")
		return nil, ErrIMBridgeNotFound
	}
	s.applyHeartbeat(instance.record)
	log.WithFields(imBridgeFields(instance.record)).Debug("IM control plane heartbeat recorded")
	return &model.IMBridgeHeartbeatResponse{
		BridgeID:   instance.record.BridgeID,
		LastSeenAt: instance.record.LastSeenAt,
		ExpiresAt:  instance.record.ExpiresAt,
		Status:     instance.record.Status,
	}, nil
}

func (s *IMControlPlane) UnregisterBridge(_ context.Context, bridgeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bridgeID = strings.TrimSpace(bridgeID)
	instance, ok := s.instances[bridgeID]
	if !ok {
		log.WithField("bridgeId", bridgeID).Warn("IM control plane unregister failed: bridge not found")
		return ErrIMBridgeNotFound
	}
	instance.record.Status = "offline"
	if listener, exists := s.listeners[bridgeID]; exists {
		_ = listener.Close()
		delete(s.listeners, bridgeID)
	}
	log.WithFields(imBridgeFields(instance.record)).Info("IM control plane bridge unregistered")
	return nil
}

func (s *IMControlPlane) ResolveBridgeTarget(platform string, projectID string, targetBridgeID string) (*model.IMBridgeInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance, err := s.resolveBridgeLocked(normalizePlatform(platform), strings.TrimSpace(projectID), strings.TrimSpace(targetBridgeID))
	if err != nil {
		return nil, err
	}
	return cloneBridgeInstance(instance.record), nil
}

func (s *IMControlPlane) AttachBridgeListener(_ context.Context, bridgeID string, afterCursor int64, listener IMBridgeDeliveryListener) ([]*model.IMControlDelivery, error) {
	if listener == nil {
		return nil, ErrIMBridgeUnavailable
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	instance, err := s.resolveBridgeLocked("", "", strings.TrimSpace(bridgeID))
	if err != nil {
		return nil, err
	}

	s.listeners[instance.record.BridgeID] = listener
	replayed := make([]*model.IMControlDelivery, 0)
	for _, delivery := range s.pending[instance.record.BridgeID] {
		if delivery.Cursor > afterCursor {
			replayed = append(replayed, cloneDelivery(delivery))
		}
	}
	fields := imBridgeFields(instance.record)
	fields["afterCursor"] = afterCursor
	fields["replayedCount"] = len(replayed)
	log.WithFields(fields).Info("IM control plane listener attached")
	return replayed, nil
}

func (s *IMControlPlane) DetachBridgeListener(bridgeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bridgeID = strings.TrimSpace(bridgeID)
	if listener, exists := s.listeners[bridgeID]; exists {
		_ = listener.Close()
		delete(s.listeners, bridgeID)
		log.WithField("bridgeId", bridgeID).Info("IM control plane listener detached")
	}
}

func (s *IMControlPlane) QueueDelivery(ctx context.Context, req IMQueueDeliveryRequest) (*model.IMControlDelivery, error) {
	s.mu.Lock()
	instance, err := s.resolveBridgeLocked(normalizePlatform(req.Platform), strings.TrimSpace(req.ProjectID), strings.TrimSpace(req.TargetBridgeID))
	if err != nil {
		s.mu.Unlock()
		log.WithFields(log.Fields{
			"bridgeId":  strings.TrimSpace(req.TargetBridgeID),
			"platform":  normalizePlatform(req.Platform),
			"projectId": strings.TrimSpace(req.ProjectID),
			"kind":      normalizeDeliveryKind(req.Kind),
		}).WithError(err).Warn("IM control plane delivery target resolution failed")
		return nil, err
	}

	s.nextCursor++
	now := s.now().UTC()
	delivery := &model.IMControlDelivery{
		Cursor:         s.nextCursor,
		DeliveryID:     uuid.NewString(),
		TargetBridgeID: instance.record.BridgeID,
		Platform:       instance.record.Platform,
		ProjectID:      strings.TrimSpace(req.ProjectID),
		Kind:           normalizeDeliveryKind(req.Kind),
		Content:        strings.TrimSpace(req.Content),
		TargetChatID:   strings.TrimSpace(req.TargetChatID),
		ReplyTarget:    cloneReplyTarget(req.ReplyTarget),
		Timestamp:      now.Format(time.RFC3339),
	}
	delivery.Signature = s.signDelivery(delivery)
	s.pending[instance.record.BridgeID] = append(s.pending[instance.record.BridgeID], delivery)
	listener := s.listeners[instance.record.BridgeID]
	cloned := cloneDelivery(delivery)
	s.mu.Unlock()
	log.WithFields(imDeliveryFields(cloned)).Info("IM control plane delivery queued")

	if listener != nil {
		if err := listener.Send(ctx, cloned); err != nil {
			log.WithFields(imDeliveryFields(cloned)).WithError(err).Warn("IM control plane delivery push failed")
			return nil, fmt.Errorf("send control-plane delivery: %w", err)
		}
		log.WithFields(imDeliveryFields(cloned)).Debug("IM control plane delivery pushed to live listener")
	}
	return cloned, nil
}

func (s *IMControlPlane) AckDelivery(_ context.Context, bridgeID string, cursor int64, deliveryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bridgeID = strings.TrimSpace(bridgeID)
	if _, ok := s.instances[bridgeID]; !ok {
		log.WithField("bridgeId", bridgeID).Warn("IM control plane delivery ack failed: bridge not found")
		return ErrIMBridgeNotFound
	}

	pending := s.pending[bridgeID]
	filtered := pending[:0]
	for _, delivery := range pending {
		if delivery.Cursor < cursor {
			continue
		}
		if delivery.Cursor == cursor && strings.TrimSpace(deliveryID) != "" && delivery.DeliveryID == strings.TrimSpace(deliveryID) {
			continue
		}
		filtered = append(filtered, delivery)
	}
	s.pending[bridgeID] = filtered
	log.WithFields(log.Fields{
		"bridgeId":     bridgeID,
		"cursor":       cursor,
		"deliveryId":   strings.TrimSpace(deliveryID),
		"pendingCount": len(filtered),
	}).Debug("IM control plane delivery acknowledged")
	return nil
}

func (s *IMControlPlane) BindAction(_ context.Context, binding *IMActionBinding) error {
	if binding == nil {
		log.Warn("IM control plane bind action rejected: empty binding")
		return ErrIMActionBindingEmpty
	}
	if strings.TrimSpace(binding.TaskID) == "" && strings.TrimSpace(binding.RunID) == "" && strings.TrimSpace(binding.ReviewID) == "" {
		log.WithField("bridgeId", strings.TrimSpace(binding.BridgeID)).Warn("IM control plane bind action rejected: missing entity ids")
		return ErrIMActionBindingEmpty
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state := &boundActionState{
		binding: &model.IMActionBinding{
			BridgeID:    strings.TrimSpace(binding.BridgeID),
			Platform:    normalizePlatform(binding.Platform),
			ProjectID:   strings.TrimSpace(binding.ProjectID),
			TaskID:      strings.TrimSpace(binding.TaskID),
			RunID:       strings.TrimSpace(binding.RunID),
			ReviewID:    strings.TrimSpace(binding.ReviewID),
			ReplyTarget: cloneReplyTarget(binding.ReplyTarget),
		},
	}
	if state.binding.ReplyTarget == nil {
		log.WithFields(log.Fields{
			"bridgeId": strings.TrimSpace(binding.BridgeID),
			"taskId":   strings.TrimSpace(binding.TaskID),
			"runId":    strings.TrimSpace(binding.RunID),
			"reviewId": strings.TrimSpace(binding.ReviewID),
		}).Warn("IM control plane bind action rejected: missing reply target")
		return ErrIMActionBindingEmpty
	}
	if state.binding.TaskID != "" {
		s.actionByTask[state.binding.TaskID] = state
	}
	if state.binding.RunID != "" {
		s.actionByRun[state.binding.RunID] = state
	}
	if state.binding.ReviewID != "" {
		s.actionByReview[state.binding.ReviewID] = state
	}
	log.WithFields(log.Fields{
		"bridgeId":  state.binding.BridgeID,
		"platform":  state.binding.Platform,
		"projectId": state.binding.ProjectID,
		"taskId":    state.binding.TaskID,
		"runId":     state.binding.RunID,
		"reviewId":  state.binding.ReviewID,
	}).Info("IM control plane action binding stored")
	return nil
}

func (s *IMControlPlane) QueueBoundProgress(ctx context.Context, req IMBoundProgressRequest) (bool, error) {
	s.mu.Lock()
	state := s.resolveBoundActionLocked(req.RunID, req.TaskID, req.ReviewID)
	if state == nil || state.binding == nil || state.binding.ReplyTarget == nil {
		s.mu.Unlock()
		log.WithFields(log.Fields{
			"taskId":     strings.TrimSpace(req.TaskID),
			"runId":      strings.TrimSpace(req.RunID),
			"reviewId":   strings.TrimSpace(req.ReviewID),
			"kind":       normalizeDeliveryKind(req.Kind),
			"isTerminal": req.IsTerminal,
		}).Debug("IM control plane bound progress skipped: no binding")
		return false, nil
	}

	now := s.now().UTC()
	if !req.IsTerminal && !state.lastHeartbeatAt.IsZero() && now.Sub(state.lastHeartbeatAt) < s.progressHeartbeatInterval {
		s.mu.Unlock()
		log.WithFields(log.Fields{
			"bridgeId":   state.binding.BridgeID,
			"taskId":     state.binding.TaskID,
			"runId":      state.binding.RunID,
			"reviewId":   state.binding.ReviewID,
			"kind":       normalizeDeliveryKind(req.Kind),
			"isTerminal": req.IsTerminal,
		}).Debug("IM control plane bound progress throttled")
		return false, nil
	}
	if !req.IsTerminal {
		state.lastHeartbeatAt = now
	}
	binding := *state.binding
	binding.ReplyTarget = cloneReplyTarget(state.binding.ReplyTarget)
	s.mu.Unlock()

	kind := normalizeDeliveryKind(req.Kind)
	if req.IsTerminal {
		kind = IMDeliveryKindTerminal
	}
	_, err := s.QueueDelivery(ctx, IMQueueDeliveryRequest{
		TargetBridgeID: binding.BridgeID,
		Platform:       binding.Platform,
		ProjectID:      binding.ProjectID,
		Kind:           kind,
		Content:        req.Content,
		ReplyTarget:    binding.ReplyTarget,
		TargetChatID:   resolveTargetChatID(binding.ReplyTarget),
	})
	if err != nil {
		log.WithFields(log.Fields{
			"bridgeId":   binding.BridgeID,
			"platform":   binding.Platform,
			"projectId":  binding.ProjectID,
			"taskId":     binding.TaskID,
			"runId":      binding.RunID,
			"reviewId":   binding.ReviewID,
			"kind":       kind,
			"isTerminal": req.IsTerminal,
		}).WithError(err).Warn("IM control plane bound progress queue failed")
		return false, err
	}
	log.WithFields(log.Fields{
		"bridgeId":   binding.BridgeID,
		"platform":   binding.Platform,
		"projectId":  binding.ProjectID,
		"taskId":     binding.TaskID,
		"runId":      binding.RunID,
		"reviewId":   binding.ReviewID,
		"kind":       kind,
		"isTerminal": req.IsTerminal,
	}).Debug("IM control plane bound progress queued")
	return true, nil
}

func (s *IMControlPlane) SignDelivery(delivery *model.IMControlDelivery) string {
	return s.signDelivery(delivery)
}

func (s *IMControlPlane) resolveBridgeLocked(platform string, projectID string, targetBridgeID string) (*bridgeInstanceState, error) {
	if targetBridgeID != "" {
		instance, ok := s.instances[targetBridgeID]
		if !ok {
			return nil, ErrIMBridgeNotFound
		}
		if !s.isInstanceLive(instance.record) {
			return nil, ErrIMBridgeUnavailable
		}
		return instance, nil
	}

	candidates := make([]*bridgeInstanceState, 0, len(s.instances))
	for _, instance := range s.instances {
		if !s.isInstanceLive(instance.record) {
			continue
		}
		if platform != "" && normalizePlatform(instance.record.Platform) != platform {
			continue
		}
		if projectID != "" && !containsString(instance.record.ProjectIDs, projectID) {
			continue
		}
		candidates = append(candidates, instance)
	}
	if len(candidates) == 0 {
		return nil, ErrIMBridgeUnavailable
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].record.BridgeID < candidates[j].record.BridgeID
	})
	return candidates[0], nil
}

func (s *IMControlPlane) resolveBoundActionLocked(runID string, taskID string, reviewID string) *boundActionState {
	if key := strings.TrimSpace(runID); key != "" {
		if state := s.actionByRun[key]; state != nil {
			return state
		}
	}
	if key := strings.TrimSpace(taskID); key != "" {
		if state := s.actionByTask[key]; state != nil {
			return state
		}
	}
	if key := strings.TrimSpace(reviewID); key != "" {
		if state := s.actionByReview[key]; state != nil {
			return state
		}
	}
	return nil
}

func (s *IMControlPlane) isInstanceLive(record *model.IMBridgeInstance) bool {
	if record == nil || record.Status == "offline" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt)
	if err != nil {
		return false
	}
	return !s.now().UTC().After(expiresAt)
}

func (s *IMControlPlane) applyHeartbeat(record *model.IMBridgeInstance) {
	now := s.now().UTC()
	record.Status = "online"
	record.LastSeenAt = now.Format(time.RFC3339)
	record.ExpiresAt = now.Add(s.heartbeatTTL).Format(time.RFC3339)
}

func (s *IMControlPlane) signDelivery(delivery *model.IMControlDelivery) string {
	if delivery == nil || s.deliverySecret == "" {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.deliverySecret))
	_, _ = mac.Write([]byte(strings.Join([]string{
		delivery.TargetBridgeID,
		fmt.Sprintf("%d", delivery.Cursor),
		delivery.DeliveryID,
		delivery.Kind,
		delivery.Content,
		delivery.Timestamp,
	}, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

func normalizePlatform(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func normalizeDeliveryKind(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case IMDeliveryKindNotify:
		return IMDeliveryKindNotify
	case IMDeliveryKindProgress:
		return IMDeliveryKindProgress
	case IMDeliveryKindTerminal:
		return IMDeliveryKindTerminal
	default:
		return IMDeliveryKindSend
	}
}

func cloneBridgeInstance(record *model.IMBridgeInstance) *model.IMBridgeInstance {
	if record == nil {
		return nil
	}
	clone := *record
	clone.ProjectIDs = append([]string(nil), record.ProjectIDs...)
	clone.CallbackPaths = append([]string(nil), record.CallbackPaths...)
	clone.Capabilities = cloneBoolMap(record.Capabilities)
	clone.CapabilityMatrix = cloneAnyMap(record.CapabilityMatrix)
	clone.Metadata = cloneStringMap(record.Metadata)
	return &clone
}

func cloneDelivery(delivery *model.IMControlDelivery) *model.IMControlDelivery {
	if delivery == nil {
		return nil
	}
	clone := *delivery
	clone.ReplyTarget = cloneReplyTarget(delivery.ReplyTarget)
	return &clone
}

func cloneReplyTarget(target *model.IMReplyTarget) *model.IMReplyTarget {
	if target == nil {
		return nil
	}
	clone := *target
	clone.Metadata = cloneStringMap(target.Metadata)
	return &clone
}

func cloneBoolMap(input map[string]bool) map[string]bool {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]bool, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneAnyValue(value)
	}
	return output
}

func cloneAnySlice(input []any) []any {
	if len(input) == 0 {
		return nil
	}
	output := make([]any, 0, len(input))
	for _, value := range input {
		output = append(output, cloneAnyValue(value))
	}
	return output
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		return cloneAnySlice(typed)
	case []string:
		cloned := make([]any, 0, len(typed))
		for _, item := range typed {
			cloned = append(cloned, item)
		}
		return cloned
	default:
		return typed
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func containsString(values []string, target string) bool {
	target = strings.TrimSpace(target)
	for _, value := range values {
		if strings.TrimSpace(value) == target {
			return true
		}
	}
	return false
}

func resolveTargetChatID(target *model.IMReplyTarget) string {
	if target == nil {
		return ""
	}
	for _, value := range []string{target.ChatID, target.ChannelID, target.ConversationID} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
