package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

type imControlPlaneStub struct {
	channels              []*model.IMChannel
	status                *model.IMBridgeStatus
	deliveries            []*model.IMDelivery
	eventTypes            []string
	listDeliveryHistoryFn func(filters *model.IMDeliveryHistoryFilters) ([]*model.IMDelivery, error)
	lastFilters           *model.IMDeliveryHistoryFilters
	savedChannel          *model.IMChannel
	deletedID             string
	retriedID             string
	retriedIDs            []string
	retryErrors           map[string]error
	ackInput              *model.IMDeliveryAck
}

type imControlSenderStub struct {
	sent []*model.IMSendRequest
	err  error
}

func (s *imControlPlaneStub) RegisterBridge(context.Context, *model.IMBridgeRegisterRequest) (*model.IMBridgeInstance, error) {
	return nil, nil
}

func (s *imControlPlaneStub) RecordHeartbeat(context.Context, string, map[string]string) (*model.IMBridgeHeartbeatResponse, error) {
	return nil, nil
}

func (s *imControlPlaneStub) UnregisterBridge(context.Context, string) error { return nil }
func (s *imControlPlaneStub) BindAction(context.Context, *model.IMActionBinding) error {
	return nil
}
func (s *imControlPlaneStub) AckDelivery(_ context.Context, ack *model.IMDeliveryAck) error {
	if ack == nil {
		s.ackInput = nil
		return nil
	}
	cloned := *ack
	s.ackInput = &cloned
	return nil
}
func (s *imControlPlaneStub) ListChannels(context.Context) ([]*model.IMChannel, error) {
	return s.channels, nil
}
func (s *imControlPlaneStub) UpsertChannel(_ context.Context, channel *model.IMChannel) (*model.IMChannel, error) {
	s.savedChannel = channel
	if channel.ID == "" {
		channel.ID = "generated-channel"
	}
	return channel, nil
}
func (s *imControlPlaneStub) DeleteChannel(_ context.Context, channelID string) error {
	s.deletedID = channelID
	return nil
}
func (s *imControlPlaneStub) GetBridgeStatus(context.Context) (*model.IMBridgeStatus, error) {
	return s.status, nil
}
func (s *imControlPlaneStub) ListDeliveryHistory(_ context.Context, filters *model.IMDeliveryHistoryFilters) ([]*model.IMDelivery, error) {
	if filters != nil {
		cloned := *filters
		s.lastFilters = &cloned
	} else {
		s.lastFilters = nil
	}
	if s.listDeliveryHistoryFn != nil {
		return s.listDeliveryHistoryFn(filters)
	}
	return s.deliveries, nil
}
func (s *imControlPlaneStub) ListEventTypes(context.Context) ([]string, error) {
	return s.eventTypes, nil
}
func (s *imControlPlaneStub) RetryDelivery(_ context.Context, deliveryID string) (*model.IMDelivery, error) {
	s.retriedIDs = append(s.retriedIDs, deliveryID)
	s.retriedID = deliveryID
	if err := s.retryErrors[deliveryID]; err != nil {
		return nil, err
	}
	return &model.IMDelivery{ID: deliveryID, Status: model.IMDeliveryStatusPending}, nil
}

func (s *imControlSenderStub) Send(_ context.Context, req *model.IMSendRequest) error {
	if req != nil {
		cloned := *req
		s.sent = append(s.sent, &cloned)
	}
	return s.err
}

func newIMControlTestContext(method, target, body string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e, e.NewContext(req, rec), rec
}

func TestIMControlHandlerOperatorEndpoints(t *testing.T) {
	stub := &imControlPlaneStub{
		channels: []*model.IMChannel{
			{
				ID:         "channel-1",
				Platform:   "feishu",
				Name:       "Alerts",
				ChannelID:  "chat-1",
				WebhookURL: "https://example.test/webhook",
				Events:     []string{"task.created"},
				Active:     true,
			},
		},
		status: &model.IMBridgeStatus{
			Registered: true,
			Providers:  []string{"feishu"},
			Health:     "healthy",
		},
		deliveries: []*model.IMDelivery{
			{
				ID:        "delivery-1",
				ChannelID: "chat-1",
				Platform:  "feishu",
				EventType: "task.created",
				Status:    model.IMDeliveryStatusDelivered,
				CreatedAt: "2026-03-26T08:00:00Z",
			},
		},
		eventTypes: []string{"task.created", "workflow.failed"},
	}
	h := NewIMControlHandler(stub)

	_, listCtx, listRec := newIMControlTestContext(http.MethodGet, "/api/v1/im/channels", "")
	if err := h.ListChannels(listCtx); err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("ListChannels() status = %d", listRec.Code)
	}

	_, saveCtx, saveRec := newIMControlTestContext(http.MethodPost, "/api/v1/im/channels", `{"platform":"feishu","name":"Ops","channelId":"chat-2","webhookUrl":"https://example.test/ops","events":["task.completed"],"active":true}`)
	if err := h.SaveChannel(saveCtx); err != nil {
		t.Fatalf("SaveChannel(create) error = %v", err)
	}
	if saveRec.Code != http.StatusCreated {
		t.Fatalf("SaveChannel(create) status = %d", saveRec.Code)
	}
	if stub.savedChannel == nil || stub.savedChannel.Name != "Ops" {
		t.Fatalf("saved channel = %+v", stub.savedChannel)
	}

	_, updateCtx, updateRec := newIMControlTestContext(http.MethodPut, "/api/v1/im/channels/channel-1", `{"platform":"feishu","name":"Renamed","channelId":"chat-1","webhookUrl":"https://example.test/webhook","events":["task.created"],"active":true}`)
	updateCtx.SetParamNames("id")
	updateCtx.SetParamValues("channel-1")
	if err := h.SaveChannel(updateCtx); err != nil {
		t.Fatalf("SaveChannel(update) error = %v", err)
	}
	if updateRec.Code != http.StatusOK {
		t.Fatalf("SaveChannel(update) status = %d", updateRec.Code)
	}
	if stub.savedChannel == nil || stub.savedChannel.ID != "channel-1" {
		t.Fatalf("updated channel = %+v", stub.savedChannel)
	}

	_, statusCtx, statusRec := newIMControlTestContext(http.MethodGet, "/api/v1/im/bridge/status", "")
	if err := h.GetStatus(statusCtx); err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}
	if statusRec.Code != http.StatusOK {
		t.Fatalf("GetStatus() status = %d", statusRec.Code)
	}

	_, deliveriesCtx, deliveriesRec := newIMControlTestContext(http.MethodGet, "/api/v1/im/deliveries", "")
	if err := h.ListDeliveries(deliveriesCtx); err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if deliveriesRec.Code != http.StatusOK {
		t.Fatalf("ListDeliveries() status = %d", deliveriesRec.Code)
	}

	_, eventTypesCtx, eventTypesRec := newIMControlTestContext(http.MethodGet, "/api/v1/im/event-types", "")
	if err := h.ListEventTypes(eventTypesCtx); err != nil {
		t.Fatalf("ListEventTypes() error = %v", err)
	}
	if eventTypesRec.Code != http.StatusOK {
		t.Fatalf("ListEventTypes() status = %d", eventTypesRec.Code)
	}

	_, retryCtx, retryRec := newIMControlTestContext(http.MethodPost, "/api/v1/im/deliveries/delivery-1/retry", "")
	retryCtx.SetParamNames("id")
	retryCtx.SetParamValues("delivery-1")
	if err := h.RetryDelivery(retryCtx); err != nil {
		t.Fatalf("RetryDelivery() error = %v", err)
	}
	if retryRec.Code != http.StatusOK {
		t.Fatalf("RetryDelivery() status = %d", retryRec.Code)
	}
	if stub.retriedID != "delivery-1" {
		t.Fatalf("retried id = %q, want delivery-1", stub.retriedID)
	}

	_, deleteCtx, deleteRec := newIMControlTestContext(http.MethodDelete, "/api/v1/im/channels/channel-1", "")
	deleteCtx.SetParamNames("id")
	deleteCtx.SetParamValues("channel-1")
	if err := h.DeleteChannel(deleteCtx); err != nil {
		t.Fatalf("DeleteChannel() error = %v", err)
	}
	if deleteRec.Code != http.StatusOK {
		t.Fatalf("DeleteChannel() status = %d", deleteRec.Code)
	}
	if stub.deletedID != "channel-1" {
		t.Fatalf("deleted id = %q, want channel-1", stub.deletedID)
	}
}

func TestIMControlHandlerAckDeliveryCapturesDowngradeReason(t *testing.T) {
	stub := &imControlPlaneStub{}
	h := NewIMControlHandler(stub)

	_, ctx, rec := newIMControlTestContext(http.MethodPost, "/api/v1/im/bridge/ack", `{"bridgeId":"bridge-1","cursor":7,"deliveryId":"delivery-1","status":"failed","failureReason":"rate_limit","downgradeReason":"actioncard_send_failed","processedAt":"2026-03-26T08:00:00Z"}`)
	if err := h.AckDelivery(ctx); err != nil {
		t.Fatalf("AckDelivery() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("AckDelivery() status = %d", rec.Code)
	}
	if stub.ackInput == nil || stub.ackInput.DowngradeReason != "actioncard_send_failed" {
		t.Fatalf("ack input = %+v", stub.ackInput)
	}
	if stub.ackInput.Status != string(model.IMDeliveryStatusFailed) {
		t.Fatalf("ack status = %q", stub.ackInput.Status)
	}
	if stub.ackInput.FailureReason != "rate_limit" {
		t.Fatalf("ack failureReason = %q", stub.ackInput.FailureReason)
	}
	if stub.ackInput.ProcessedAt != "2026-03-26T08:00:00Z" {
		t.Fatalf("ack processedAt = %q", stub.ackInput.ProcessedAt)
	}
}

func TestIMControlHandlerListChannelsResponseShape(t *testing.T) {
	stub := &imControlPlaneStub{
		channels: []*model.IMChannel{{ID: "channel-1", Name: "Alerts", Platform: "feishu", ChannelID: "chat-1", Active: true}},
	}
	h := NewIMControlHandler(stub)

	_, ctx, rec := newIMControlTestContext(http.MethodGet, "/api/v1/im/channels", "")
	if err := h.ListChannels(ctx); err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}

	var payload []model.IMChannel
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal channels: %v", err)
	}
	if len(payload) != 1 || payload[0].ID != "channel-1" {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestIMControlHandlerGetStatusIncludesEmptyProviderDetails(t *testing.T) {
	stub := &imControlPlaneStub{
		status: &model.IMBridgeStatus{
			Registered:      false,
			LastHeartbeat:   nil,
			Providers:       []string{},
			ProviderDetails: []model.IMBridgeProviderDetail{},
			Health:          "disconnected",
		},
	}
	h := NewIMControlHandler(stub)

	_, ctx, rec := newIMControlTestContext(http.MethodGet, "/api/v1/im/bridge/status", "")
	if err := h.GetStatus(ctx); err != nil {
		t.Fatalf("GetStatus() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal status: %v", err)
	}

	rawProviderDetails, ok := payload["providerDetails"]
	if !ok {
		t.Fatalf("providerDetails missing from payload: %s", rec.Body.String())
	}
	providerDetails, ok := rawProviderDetails.([]any)
	if !ok {
		t.Fatalf("providerDetails = %#v, want array", rawProviderDetails)
	}
	if len(providerDetails) != 0 {
		t.Fatalf("providerDetails = %#v, want empty array", providerDetails)
	}
}

func TestIMControlHandlerListDeliveriesPassesFilters(t *testing.T) {
	stub := &imControlPlaneStub{
		deliveries: []*model.IMDelivery{{ID: "delivery-1", Platform: "slack", Status: model.IMDeliveryStatusFailed, CreatedAt: "2026-03-26T09:00:00Z"}},
	}
	h := NewIMControlHandler(stub)

	_, ctx, rec := newIMControlTestContext(http.MethodGet, "/api/v1/im/deliveries?status=failed&platform=slack&eventType=task.created&kind=notify&since=2026-03-26T08:30:00Z", "")
	if err := h.ListDeliveries(ctx); err != nil {
		t.Fatalf("ListDeliveries() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("ListDeliveries() status = %d", rec.Code)
	}
	if stub.lastFilters == nil {
		t.Fatal("expected filters to be forwarded")
	}
	if stub.lastFilters.Status != "failed" || stub.lastFilters.Platform != "slack" || stub.lastFilters.EventType != "task.created" || stub.lastFilters.Kind != "notify" || stub.lastFilters.Since != "2026-03-26T08:30:00Z" {
		t.Fatalf("filters = %+v", stub.lastFilters)
	}
}

func TestIMControlHandlerRetryBatchDeliveriesReturnsPerItemOutcomes(t *testing.T) {
	stub := &imControlPlaneStub{
		retryErrors: map[string]error{
			"delivery-3": errors.New("not retryable"),
		},
	}
	h := NewIMControlHandler(stub)

	_, ctx, rec := newIMControlTestContext(http.MethodPost, "/api/v1/im/deliveries/retry-batch", `{"deliveryIds":["delivery-1","delivery-2","delivery-3"]}`)
	if err := h.RetryBatchDeliveries(ctx); err != nil {
		t.Fatalf("RetryBatchDeliveries() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("RetryBatchDeliveries() status = %d", rec.Code)
	}

	if len(stub.retriedIDs) != 3 {
		t.Fatalf("retriedIDs = %+v, want all delivery ids attempted", stub.retriedIDs)
	}

	var payload model.IMRetryBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal retry batch response: %v", err)
	}
	if len(payload.Results) != 3 {
		t.Fatalf("results len = %d, want 3", len(payload.Results))
	}
	if payload.Results[0].DeliveryID != "delivery-1" || payload.Results[0].Status != model.IMDeliveryStatusPending {
		t.Fatalf("result[0] = %+v", payload.Results[0])
	}
	if payload.Results[2].DeliveryID != "delivery-3" || string(payload.Results[2].Status) != "rejected" {
		t.Fatalf("result[2] = %+v", payload.Results[2])
	}
}

func TestIMControlHandlerTestSendReturnsSettledDeliveryResult(t *testing.T) {
	stub := &imControlPlaneStub{
		deliveries: []*model.IMDelivery{
			{
				ID:            "delivery-test-1",
				Platform:      "slack",
				ChannelID:     "C123",
				Status:        model.IMDeliveryStatusDelivered,
				FailureReason: "",
				ProcessedAt:   "2026-03-26T08:00:01Z",
				LatencyMs:     320,
				CreatedAt:     "2026-03-26T08:00:00Z",
			},
		},
	}
	sender := &imControlSenderStub{}
	h := NewIMControlHandler(stub, sender)

	_, ctx, rec := newIMControlTestContext(http.MethodPost, "/api/v1/im/test-send", `{"platform":"slack","channelId":"C123","text":"ping","deliveryId":"delivery-test-1"}`)
	if err := h.TestSend(ctx); err != nil {
		t.Fatalf("TestSend() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("TestSend() status = %d", rec.Code)
	}
	if len(sender.sent) != 1 || sender.sent[0].DeliveryID != "delivery-test-1" {
		t.Fatalf("sent requests = %+v", sender.sent)
	}

	var payload model.IMTestSendResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal test send response: %v", err)
	}
	if payload.DeliveryID != "delivery-test-1" || payload.Status != model.IMDeliveryStatusDelivered || payload.LatencyMs != 320 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestIMControlHandlerTestSendGeneratesUniqueDeliveryIDsWhenOmitted(t *testing.T) {
	stub := &imControlPlaneStub{
		listDeliveryHistoryFn: func(filters *model.IMDeliveryHistoryFilters) ([]*model.IMDelivery, error) {
			deliveryID := ""
			if filters != nil {
				deliveryID = strings.TrimSpace(filters.DeliveryID)
			}
			if deliveryID == "" {
				return nil, nil
			}
			return []*model.IMDelivery{
				{
					ID:          deliveryID,
					Platform:    "slack",
					ChannelID:   "C123",
					Status:      model.IMDeliveryStatusDelivered,
					ProcessedAt: "2026-03-26T08:00:01Z",
					LatencyMs:   320,
					CreatedAt:   "2026-03-26T08:00:00Z",
				},
			}, nil
		},
	}
	sender := &imControlSenderStub{}
	h := NewIMControlHandler(stub, sender)

	_, firstCtx, firstRec := newIMControlTestContext(http.MethodPost, "/api/v1/im/test-send", `{"platform":"slack","channelId":"C123","text":"ping"}`)
	if err := h.TestSend(firstCtx); err != nil {
		t.Fatalf("first TestSend() error = %v", err)
	}
	_, secondCtx, secondRec := newIMControlTestContext(http.MethodPost, "/api/v1/im/test-send", `{"platform":"slack","channelId":"C123","text":"pong"}`)
	if err := h.TestSend(secondCtx); err != nil {
		t.Fatalf("second TestSend() error = %v", err)
	}

	if len(sender.sent) != 2 {
		t.Fatalf("sent requests = %+v", sender.sent)
	}
	if sender.sent[0].DeliveryID == "" || sender.sent[1].DeliveryID == "" {
		t.Fatalf("delivery ids = %q, %q", sender.sent[0].DeliveryID, sender.sent[1].DeliveryID)
	}
	if !strings.HasPrefix(sender.sent[0].DeliveryID, "test-send-slack-") {
		t.Fatalf("first delivery id = %q", sender.sent[0].DeliveryID)
	}
	if !strings.HasPrefix(sender.sent[1].DeliveryID, "test-send-slack-") {
		t.Fatalf("second delivery id = %q", sender.sent[1].DeliveryID)
	}
	if sender.sent[0].DeliveryID == sender.sent[1].DeliveryID {
		t.Fatalf("delivery ids must be unique, got %q", sender.sent[0].DeliveryID)
	}

	var firstPayload model.IMTestSendResponse
	if err := json.Unmarshal(firstRec.Body.Bytes(), &firstPayload); err != nil {
		t.Fatalf("unmarshal first test send response: %v", err)
	}
	var secondPayload model.IMTestSendResponse
	if err := json.Unmarshal(secondRec.Body.Bytes(), &secondPayload); err != nil {
		t.Fatalf("unmarshal second test send response: %v", err)
	}
	if firstPayload.DeliveryID != sender.sent[0].DeliveryID {
		t.Fatalf("first payload = %+v, sent = %+v", firstPayload, sender.sent[0])
	}
	if secondPayload.DeliveryID != sender.sent[1].DeliveryID {
		t.Fatalf("second payload = %+v, sent = %+v", secondPayload, sender.sent[1])
	}
}
