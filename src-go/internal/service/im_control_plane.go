package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	DeliveryID     string
	TargetBridgeID string
	Platform       string
	ProjectID      string
	Kind           string
	Content        string
	Structured     *model.IMStructuredMessage
	Native         *model.IMNativeMessage
	Metadata       map[string]string
	TargetChatID   string
	ReplyTarget    *model.IMReplyTarget
}

type IMBoundProgressRequest struct {
	TaskID     string
	RunID      string
	ReviewID   string
	Kind       string
	Content    string
	Structured *model.IMStructuredMessage
	Native     *model.IMNativeMessage
	Metadata   map[string]string
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
	channels   map[string]*model.IMChannel
	history    []*model.IMDelivery
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
		channels:                  make(map[string]*model.IMChannel),
		history:                   make([]*model.IMDelivery, 0),
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

func (s *IMControlPlane) RecordHeartbeat(_ context.Context, bridgeID string, metadata map[string]string) (*model.IMBridgeHeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	instance, ok := s.instances[strings.TrimSpace(bridgeID)]
	if !ok {
		log.WithField("bridgeId", strings.TrimSpace(bridgeID)).Warn("IM control plane heartbeat failed: bridge not found")
		return nil, ErrIMBridgeNotFound
	}
	if len(metadata) > 0 {
		instance.record.Metadata = cloneStringMap(metadata)
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

func (s *IMControlPlane) ListChannels(_ context.Context) ([]*model.IMChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	channels := make([]*model.IMChannel, 0, len(s.channels))
	for _, channel := range s.channels {
		channels = append(channels, cloneChannel(channel))
	}
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})
	return channels, nil
}

func (s *IMControlPlane) UpsertChannel(_ context.Context, channel *model.IMChannel) (*model.IMChannel, error) {
	if channel == nil {
		return nil, ErrIMDeliveryRejected
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	id := strings.TrimSpace(channel.ID)
	if id == "" {
		id = "im-channel-" + uuid.NewString()
	}
	record := &model.IMChannel{
		ID:             id,
		Platform:       normalizePlatform(channel.Platform),
		Name:           strings.TrimSpace(channel.Name),
		ChannelID:      strings.TrimSpace(channel.ChannelID),
		WebhookURL:     strings.TrimSpace(channel.WebhookURL),
		PlatformConfig: cloneStringMap(channel.PlatformConfig),
		Events:         dedupeStrings(channel.Events),
		Active:         channel.Active,
	}
	s.channels[id] = record
	return cloneChannel(record), nil
}

func (s *IMControlPlane) DeleteChannel(_ context.Context, channelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.channels, strings.TrimSpace(channelID))
	return nil
}

func (s *IMControlPlane) ResolveChannelsForEvent(_ context.Context, eventType string, platform string, channelID string) ([]*model.IMChannel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEvent := strings.TrimSpace(eventType)
	normalizedPlatform := normalizePlatform(platform)
	normalizedChannelID := strings.TrimSpace(channelID)

	channels := make([]*model.IMChannel, 0, len(s.channels))
	for _, channel := range s.channels {
		if channel == nil || !channel.Active {
			continue
		}
		if normalizedPlatform != "" && normalizePlatform(channel.Platform) != normalizedPlatform {
			continue
		}
		if normalizedChannelID != "" && strings.TrimSpace(channel.ChannelID) != normalizedChannelID {
			continue
		}
		if normalizedEvent != "" && !containsFold(channel.Events, normalizedEvent) {
			continue
		}
		channels = append(channels, cloneChannel(channel))
	}

	sort.Slice(channels, func(i, j int) bool {
		if channels[i].Platform == channels[j].Platform {
			if channels[i].Name == channels[j].Name {
				return channels[i].ChannelID < channels[j].ChannelID
			}
			return channels[i].Name < channels[j].Name
		}
		return channels[i].Platform < channels[j].Platform
	})

	return channels, nil
}

func (s *IMControlPlane) GetBridgeStatus(_ context.Context) (*model.IMBridgeStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	registered := false
	health := "disconnected"
	lastHeartbeat := ""
	providers := make([]string, 0, len(s.instances))
	providerDetailsByPlatform := make(map[string]*model.IMBridgeProviderDetail, len(s.instances))
	var pendingDeliveries int
	var recentFailures int
	var recentDowngrades int
	var settledLatencyTotal int64
	var settledLatencyCount int64

	ensureProviderDetail := func(platform string) *model.IMBridgeProviderDetail {
		normalized := normalizePlatform(platform)
		if normalized == "" {
			return nil
		}
		if detail, ok := providerDetailsByPlatform[normalized]; ok {
			return detail
		}
		providers = append(providers, normalized)
		detail := &model.IMBridgeProviderDetail{Platform: normalized}
		providerDetailsByPlatform[normalized] = detail
		return detail
	}

	for _, instance := range s.instances {
		if instance == nil || instance.record == nil {
			continue
		}
		record := cloneBridgeInstance(instance.record)
		if s.isBridgeAlive(record, now) {
			registered = true
		}
		if detail := ensureProviderDetail(record.Platform); detail != nil {
			if detail.Status == "" || s.isBridgeAlive(record, now) {
				detail.Status = strings.TrimSpace(record.Status)
			}
			if detail.Transport == "" {
				detail.Transport = strings.TrimSpace(record.Transport)
			}
			if len(detail.CallbackPaths) == 0 {
				detail.CallbackPaths = append([]string(nil), record.CallbackPaths...)
			} else {
				detail.CallbackPaths = dedupeStrings(append(detail.CallbackPaths, record.CallbackPaths...))
			}
			if len(detail.CapabilityMatrix) == 0 {
				detail.CapabilityMatrix = cloneAnyMap(record.CapabilityMatrix)
			}
			if len(detail.Diagnostics) == 0 {
				detail.Diagnostics = cloneStringMap(record.Metadata)
			}
		}
		if record.LastSeenAt > lastHeartbeat {
			lastHeartbeat = record.LastSeenAt
		}
	}

	for bridgeID, queued := range s.pending {
		if len(queued) == 0 {
			continue
		}
		instance := s.instances[strings.TrimSpace(bridgeID)]
		if instance == nil || instance.record == nil {
			continue
		}
		detail := ensureProviderDetail(instance.record.Platform)
		if detail == nil {
			continue
		}
		detail.PendingDeliveries += len(queued)
		pendingDeliveries += len(queued)
	}

	for _, delivery := range s.history {
		if delivery == nil {
			continue
		}
		detail := ensureProviderDetail(delivery.Platform)
		if detail == nil {
			continue
		}
		if eventAt := deliveryEventTimestamp(delivery); eventAt != "" {
			if detail.LastDeliveryAt == nil || eventAt > strings.TrimSpace(*detail.LastDeliveryAt) {
				detail.LastDeliveryAt = &eventAt
			}
		}
		switch delivery.Status {
		case model.IMDeliveryStatusFailed, model.IMDeliveryStatusTimeout:
			detail.RecentFailures++
			recentFailures++
		}
		if strings.TrimSpace(delivery.DowngradeReason) != "" {
			detail.RecentDowngrades++
			recentDowngrades++
		}
		if delivery.Status == model.IMDeliveryStatusDelivered {
			if latency := deliveryLatencyMs(delivery); latency > 0 {
				settledLatencyTotal += latency
				settledLatencyCount++
			}
		}
	}

	switch {
	case registered:
		health = "healthy"
	case len(s.instances) > 0:
		health = "degraded"
	}

	sort.Strings(providers)
	providerDetails := make([]model.IMBridgeProviderDetail, 0, len(providers))
	for _, provider := range providers {
		if detail := providerDetailsByPlatform[provider]; detail != nil {
			providerDetails = append(providerDetails, *detail)
		}
	}
	var heartbeat *string
	if lastHeartbeat != "" {
		heartbeat = &lastHeartbeat
	}
	var averageLatencyMs int64
	if settledLatencyCount > 0 {
		averageLatencyMs = settledLatencyTotal / settledLatencyCount
	}
	return &model.IMBridgeStatus{
		Registered:        registered,
		LastHeartbeat:     heartbeat,
		Providers:         providers,
		ProviderDetails:   providerDetails,
		Health:            health,
		PendingDeliveries: pendingDeliveries,
		RecentFailures:    recentFailures,
		RecentDowngrades:  recentDowngrades,
		AverageLatencyMs:  averageLatencyMs,
	}, nil
}

func (s *IMControlPlane) RecordDeliveryResult(result model.IMDelivery) {
	s.mu.Lock()
	defer s.mu.Unlock()

	record := &model.IMDelivery{
		ID:              strings.TrimSpace(result.ID),
		BridgeID:        strings.TrimSpace(result.BridgeID),
		ProjectID:       strings.TrimSpace(result.ProjectID),
		ChannelID:       strings.TrimSpace(result.ChannelID),
		TargetChatID:    strings.TrimSpace(result.TargetChatID),
		Platform:        normalizePlatform(result.Platform),
		EventType:       strings.TrimSpace(result.EventType),
		Kind:            normalizeDeliveryKind(result.Kind),
		Status:          result.Status,
		FailureReason:   strings.TrimSpace(result.FailureReason),
		DowngradeReason: strings.TrimSpace(result.DowngradeReason),
		Content:         strings.TrimSpace(result.Content),
		Structured:      cloneStructuredMessage(result.Structured),
		Native:          cloneNativeMessage(result.Native),
		Metadata:        cloneStringMap(result.Metadata),
		ReplyTarget:     cloneReplyTarget(result.ReplyTarget),
		CreatedAt:       strings.TrimSpace(result.CreatedAt),
		ProcessedAt:     strings.TrimSpace(result.ProcessedAt),
		LatencyMs:       result.LatencyMs,
	}
	if record.ID == "" {
		record.ID = uuid.NewString()
	}
	if record.CreatedAt == "" {
		record.CreatedAt = s.now().UTC().Format(time.RFC3339)
	}
	s.history = append([]*model.IMDelivery{record}, s.history...)
	if len(s.history) > 200 {
		s.history = s.history[:200]
	}
}

func (s *IMControlPlane) ListDeliveryHistory(_ context.Context, filters *model.IMDeliveryHistoryFilters) ([]*model.IMDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	history := make([]*model.IMDelivery, 0, len(s.history))
	for _, delivery := range s.history {
		if !matchesDeliveryFilters(delivery, filters) {
			continue
		}
		history = append(history, cloneDeliveryRecord(delivery))
	}
	return history, nil
}

func (s *IMControlPlane) ListEventTypes(_ context.Context) ([]string, error) {
	return CanonicalIMEventInventory(), nil
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
		DeliveryID:     firstNonEmpty(strings.TrimSpace(req.DeliveryID), uuid.NewString()),
		TargetBridgeID: instance.record.BridgeID,
		Platform:       instance.record.Platform,
		ProjectID:      strings.TrimSpace(req.ProjectID),
		Kind:           normalizeDeliveryKind(req.Kind),
		Content:        normalizeDeliveryContent(req.Content, req.Structured),
		Structured:     cloneStructuredMessage(req.Structured),
		Native:         cloneNativeMessage(req.Native),
		Metadata:       cloneStringMap(req.Metadata),
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

func (s *IMControlPlane) AckDelivery(_ context.Context, ack *model.IMDeliveryAck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ack == nil {
		return ErrIMDeliveryRejected
	}
	bridgeID := strings.TrimSpace(ack.BridgeID)
	if _, ok := s.instances[bridgeID]; !ok {
		log.WithField("bridgeId", bridgeID).Warn("IM control plane delivery ack failed: bridge not found")
		return ErrIMBridgeNotFound
	}

	pending := s.pending[bridgeID]
	filtered := pending[:0]
	for _, delivery := range pending {
		if delivery.Cursor < ack.Cursor {
			continue
		}
		if delivery.Cursor == ack.Cursor && strings.TrimSpace(ack.DeliveryID) != "" && delivery.DeliveryID == strings.TrimSpace(ack.DeliveryID) {
			continue
		}
		filtered = append(filtered, delivery)
	}
	s.pending[bridgeID] = filtered
	s.applyDeliveryAckLocked(ack)
	log.WithFields(log.Fields{
		"bridgeId":        bridgeID,
		"cursor":          ack.Cursor,
		"deliveryId":      strings.TrimSpace(ack.DeliveryID),
		"downgradeReason": strings.TrimSpace(ack.DowngradeReason),
		"pendingCount":    len(filtered),
	}).Debug("IM control plane delivery acknowledged")
	return nil
}

func (s *IMControlPlane) RetryDelivery(ctx context.Context, deliveryID string) (*model.IMDelivery, error) {
	s.mu.Lock()
	record := s.findDeliveryLocked(strings.TrimSpace(deliveryID))
	if record == nil {
		s.mu.Unlock()
		return nil, ErrIMDeliveryRejected
	}
	if record.Status != model.IMDeliveryStatusFailed && record.Status != model.IMDeliveryStatusTimeout {
		s.mu.Unlock()
		return nil, ErrIMDeliveryRejected
	}
	retryRecord := cloneDeliveryRecord(record)
	s.mu.Unlock()

	if _, err := s.QueueDelivery(ctx, IMQueueDeliveryRequest{
		DeliveryID:     retryRecord.ID,
		TargetBridgeID: retryRecord.BridgeID,
		Platform:       retryRecord.Platform,
		ProjectID:      retryRecord.ProjectID,
		Kind:           retryRecord.Kind,
		Content:        retryRecord.Content,
		Structured:     retryRecord.Structured,
		Native:         retryRecord.Native,
		Metadata:       retryRecord.Metadata,
		TargetChatID:   firstNonEmpty(retryRecord.TargetChatID, retryRecord.ChannelID),
		ReplyTarget:    retryRecord.ReplyTarget,
	}); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	record = s.findDeliveryLocked(strings.TrimSpace(deliveryID))
	if record == nil {
		return nil, ErrIMDeliveryRejected
	}
	record.Status = model.IMDeliveryStatusPending
	record.FailureReason = ""
	record.DowngradeReason = ""
	record.CreatedAt = s.now().UTC().Format(time.RFC3339)
	return cloneDeliveryRecord(record), nil
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
	if !isBoundProgressEventEnabled(state.binding, req.Metadata) {
		s.mu.Unlock()
		log.WithFields(log.Fields{
			"bridgeId":   state.binding.BridgeID,
			"taskId":     state.binding.TaskID,
			"runId":      state.binding.RunID,
			"reviewId":   state.binding.ReviewID,
			"eventType":  strings.TrimSpace(req.Metadata["bridge_event_type"]),
			"kind":       normalizeDeliveryKind(req.Kind),
			"isTerminal": req.IsTerminal,
		}).Debug("IM control plane bound progress skipped: event type disabled")
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
	source := imDeliverySourceBoundProgress
	if req.IsTerminal {
		source = imDeliverySourceBoundTerminal
	}
	metadata := buildIMConnectivityMetadata(
		req.Metadata,
		source,
		binding.BridgeID,
		binding.Platform,
		binding.ProjectID,
		binding.TaskID,
		binding.RunID,
		binding.ReviewID,
		binding.ReplyTarget,
	)
	delivery, err := s.QueueDelivery(ctx, IMQueueDeliveryRequest{
		TargetBridgeID: binding.BridgeID,
		Platform:       binding.Platform,
		ProjectID:      binding.ProjectID,
		Kind:           kind,
		Content:        req.Content,
		Structured:     req.Structured,
		Native:         req.Native,
		Metadata:       metadata,
		ReplyTarget:    binding.ReplyTarget,
		TargetChatID:   resolveTargetChatID(binding.ReplyTarget),
	})
	if err != nil {
		s.RecordDeliveryResult(model.IMDelivery{
			BridgeID:      binding.BridgeID,
			ProjectID:     binding.ProjectID,
			ChannelID:     resolveTargetChatID(binding.ReplyTarget),
			TargetChatID:  resolveTargetChatID(binding.ReplyTarget),
			Platform:      binding.Platform,
			EventType:     firstNonEmpty(strings.TrimSpace(metadata["bridge_event_type"]), "bound.progress"),
			Kind:          kind,
			Status:        model.IMDeliveryStatusFailed,
			FailureReason: err.Error(),
			Content:       normalizeDeliveryContent(req.Content, req.Structured),
			Structured:    req.Structured,
			Native:        req.Native,
			Metadata:      metadata,
			ReplyTarget:   binding.ReplyTarget,
		})
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
	deliveryID := ""
	if delivery != nil {
		deliveryID = strings.TrimSpace(delivery.DeliveryID)
	}
	s.RecordDeliveryResult(model.IMDelivery{
		ID:           deliveryID,
		BridgeID:     binding.BridgeID,
		ProjectID:    binding.ProjectID,
		ChannelID:    resolveTargetChatID(binding.ReplyTarget),
		TargetChatID: resolveTargetChatID(binding.ReplyTarget),
		Platform:     binding.Platform,
		EventType:    firstNonEmpty(strings.TrimSpace(metadata["bridge_event_type"]), "bound.progress"),
		Kind:         kind,
		Status:       model.IMDeliveryStatusPending,
		Content:      normalizeDeliveryContent(req.Content, req.Structured),
		Structured:   req.Structured,
		Native:       req.Native,
		Metadata:     metadata,
		ReplyTarget:  binding.ReplyTarget,
	})
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

func isBoundProgressEventEnabled(binding *IMActionBinding, metadata map[string]string) bool {
	if binding == nil || binding.ReplyTarget == nil || binding.ReplyTarget.Metadata == nil {
		return true
	}
	eventType := strings.TrimSpace(metadata["bridge_event_type"])
	if eventType == "" {
		return true
	}
	raw, ok := binding.ReplyTarget.Metadata["bridge_event_enabled."+eventType]
	if !ok {
		return true
	}
	enabled, parsed := parseMetadataBool(raw)
	if !parsed {
		return true
	}
	return enabled
}

func parseMetadataBool(raw string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on", "enabled":
		return true, true
	case "0", "false", "no", "off", "disabled":
		return false, true
	default:
		return false, false
	}
}

func containsFold(values []string, target string) bool {
	trimmedTarget := strings.TrimSpace(target)
	if trimmedTarget == "" {
		return false
	}
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), trimmedTarget) {
			return true
		}
	}
	return false
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
	return s.isBridgeAlive(record, s.now().UTC())
}

func (s *IMControlPlane) isBridgeAlive(record *model.IMBridgeInstance, now time.Time) bool {
	if record == nil || record.Status == "offline" {
		return false
	}
	expiresAt, err := time.Parse(time.RFC3339, record.ExpiresAt)
	if err != nil {
		return false
	}
	return !now.After(expiresAt)
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
	payload, err := json.Marshal(struct {
		TargetBridgeID string                     `json:"targetBridgeId"`
		Cursor         int64                      `json:"cursor"`
		DeliveryID     string                     `json:"deliveryId"`
		Platform       string                     `json:"platform"`
		ProjectID      string                     `json:"projectId,omitempty"`
		Kind           string                     `json:"kind"`
		Content        string                     `json:"content,omitempty"`
		Structured     *model.IMStructuredMessage `json:"structured,omitempty"`
		Native         *model.IMNativeMessage     `json:"native,omitempty"`
		Metadata       map[string]string          `json:"metadata,omitempty"`
		TargetChatID   string                     `json:"targetChatId,omitempty"`
		ReplyTarget    *model.IMReplyTarget       `json:"replyTarget,omitempty"`
		Timestamp      string                     `json:"timestamp"`
	}{
		TargetBridgeID: strings.TrimSpace(delivery.TargetBridgeID),
		Cursor:         delivery.Cursor,
		DeliveryID:     strings.TrimSpace(delivery.DeliveryID),
		Platform:       normalizePlatform(delivery.Platform),
		ProjectID:      strings.TrimSpace(delivery.ProjectID),
		Kind:           normalizeDeliveryKind(delivery.Kind),
		Content:        strings.TrimSpace(delivery.Content),
		Structured:     cloneStructuredMessage(delivery.Structured),
		Native:         cloneNativeMessage(delivery.Native),
		Metadata:       cloneStringMap(delivery.Metadata),
		TargetChatID:   strings.TrimSpace(delivery.TargetChatID),
		ReplyTarget:    cloneReplyTarget(delivery.ReplyTarget),
		Timestamp:      strings.TrimSpace(delivery.Timestamp),
	})
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(s.deliverySecret))
	_, _ = mac.Write(payload)
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
	clone.Structured = cloneStructuredMessage(delivery.Structured)
	clone.Native = cloneNativeMessage(delivery.Native)
	clone.Metadata = cloneStringMap(delivery.Metadata)
	clone.ReplyTarget = cloneReplyTarget(delivery.ReplyTarget)
	return &clone
}

func cloneDeliveryRecord(delivery *model.IMDelivery) *model.IMDelivery {
	if delivery == nil {
		return nil
	}
	clone := *delivery
	clone.Structured = cloneStructuredMessage(delivery.Structured)
	clone.Native = cloneNativeMessage(delivery.Native)
	clone.Metadata = cloneStringMap(delivery.Metadata)
	clone.ReplyTarget = cloneReplyTarget(delivery.ReplyTarget)
	return &clone
}

func cloneChannel(channel *model.IMChannel) *model.IMChannel {
	if channel == nil {
		return nil
	}
	clone := *channel
	if channel.Events != nil {
		clone.Events = append([]string(nil), channel.Events...)
	}
	clone.PlatformConfig = cloneStringMap(channel.PlatformConfig)
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

func cloneStructuredMessage(message *model.IMStructuredMessage) *model.IMStructuredMessage {
	if message == nil {
		return nil
	}
	clone := *message
	if message.Fields != nil {
		clone.Fields = append([]model.IMStructuredField(nil), message.Fields...)
	}
	if message.Actions != nil {
		clone.Actions = append([]model.IMStructuredAction(nil), message.Actions...)
	}
	return &clone
}

func cloneNativeMessage(message *model.IMNativeMessage) *model.IMNativeMessage {
	if message == nil {
		return nil
	}
	clone := *message
	if message.FeishuCard != nil {
		cardClone := *message.FeishuCard
		if message.FeishuCard.JSON != nil {
			cardClone.JSON = append([]byte(nil), message.FeishuCard.JSON...)
		}
		cardClone.TemplateVariable = cloneAnyMap(message.FeishuCard.TemplateVariable)
		clone.FeishuCard = &cardClone
	}
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

func normalizeDeliveryContent(content string, structured *model.IMStructuredMessage) string {
	trimmed := strings.TrimSpace(content)
	if trimmed != "" {
		return trimmed
	}
	if structured != nil {
		return strings.TrimSpace(structured.FallbackText())
	}
	return ""
}

func (s *IMControlPlane) findDeliveryLocked(deliveryID string) *model.IMDelivery {
	if deliveryID == "" {
		return nil
	}
	for _, delivery := range s.history {
		if delivery != nil && strings.TrimSpace(delivery.ID) == deliveryID {
			return delivery
		}
	}
	return nil
}

// QueueBoundProgressRaw is a thin adapter used by the im_forward eventbus
// observer. It wraps QueueBoundProgress with a simplified call signature so
// the mods package need not import the service package's request type.
func (s *IMControlPlane) QueueBoundProgressRaw(ctx context.Context, taskID, content string, isTerminal bool, metadata map[string]string) (bool, error) {
	return s.QueueBoundProgress(ctx, IMBoundProgressRequest{
		TaskID:     taskID,
		Content:    content,
		IsTerminal: isTerminal,
		Metadata:   metadata,
	})
}

// BoundPlatformForTask returns the IM platform of the bound action for the given
// task ID, or an empty string if no binding exists. Used by the im_forward
// observer to determine the folding mode before calling QueueBoundProgress.
func (s *IMControlPlane) BoundPlatformForTask(taskID string) string {
	if strings.TrimSpace(taskID) == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state := s.actionByTask[strings.TrimSpace(taskID)]
	if state == nil || state.binding == nil {
		return ""
	}
	return state.binding.Platform
}

func (s *IMControlPlane) applyDeliveryAckLocked(ack *model.IMDeliveryAck) {
	if ack == nil {
		return
	}
	deliveryID := strings.TrimSpace(ack.DeliveryID)
	if deliveryID == "" {
		return
	}
	delivery := s.findDeliveryLocked(deliveryID)
	if delivery == nil {
		return
	}
	status := model.IMDeliveryStatus(strings.TrimSpace(ack.Status))
	if status == "" {
		status = model.IMDeliveryStatusDelivered
	}
	if delivery.Status == model.IMDeliveryStatusPending {
		delivery.Status = status
	}
	delivery.FailureReason = strings.TrimSpace(ack.FailureReason)
	processedAt := strings.TrimSpace(ack.ProcessedAt)
	if processedAt == "" {
		processedAt = s.now().UTC().Format(time.RFC3339)
	}
	delivery.ProcessedAt = processedAt
	if queuedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(delivery.CreatedAt)); err == nil {
		if processed, err := time.Parse(time.RFC3339, processedAt); err == nil && !processed.Before(queuedAt) {
			delivery.LatencyMs = processed.Sub(queuedAt).Milliseconds()
		}
	}
	if strings.TrimSpace(ack.DowngradeReason) != "" {
		delivery.DowngradeReason = strings.TrimSpace(ack.DowngradeReason)
	}
}

func deliveryEventTimestamp(delivery *model.IMDelivery) string {
	if delivery == nil {
		return ""
	}
	if processedAt := strings.TrimSpace(delivery.ProcessedAt); processedAt != "" {
		return processedAt
	}
	return strings.TrimSpace(delivery.CreatedAt)
}

func deliveryLatencyMs(delivery *model.IMDelivery) int64 {
	if delivery == nil {
		return 0
	}
	if delivery.LatencyMs > 0 {
		return delivery.LatencyMs
	}
	queuedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(delivery.CreatedAt))
	if err != nil {
		return 0
	}
	processedAt, err := time.Parse(time.RFC3339, strings.TrimSpace(delivery.ProcessedAt))
	if err != nil || processedAt.Before(queuedAt) {
		return 0
	}
	return processedAt.Sub(queuedAt).Milliseconds()
}

func matchesDeliveryFilters(delivery *model.IMDelivery, filters *model.IMDeliveryHistoryFilters) bool {
	if delivery == nil || filters == nil {
		return delivery != nil
	}
	if deliveryID := strings.TrimSpace(filters.DeliveryID); deliveryID != "" && strings.TrimSpace(delivery.ID) != deliveryID {
		return false
	}
	if status := strings.TrimSpace(filters.Status); status != "" && !strings.EqualFold(string(delivery.Status), status) {
		return false
	}
	if platform := normalizePlatform(filters.Platform); platform != "" && normalizePlatform(delivery.Platform) != platform {
		return false
	}
	if eventType := strings.TrimSpace(filters.EventType); eventType != "" && !strings.EqualFold(strings.TrimSpace(delivery.EventType), eventType) {
		return false
	}
	if kind := strings.TrimSpace(filters.Kind); kind != "" && !strings.EqualFold(strings.TrimSpace(delivery.Kind), kind) {
		return false
	}
	if since := strings.TrimSpace(filters.Since); since != "" {
		sinceTime, err := time.Parse(time.RFC3339, since)
		if err != nil {
			return false
		}
		eventAt, err := time.Parse(time.RFC3339, deliveryEventTimestamp(delivery))
		if err != nil || eventAt.Before(sinceTime) {
			return false
		}
	}
	return true
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
