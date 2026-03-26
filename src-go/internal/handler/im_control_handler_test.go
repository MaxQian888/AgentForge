package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
)

type imControlPlaneStub struct {
	channels     []*model.IMChannel
	status       *model.IMBridgeStatus
	deliveries   []*model.IMDelivery
	savedChannel *model.IMChannel
	deletedID    string
}

func (s *imControlPlaneStub) RegisterBridge(context.Context, *model.IMBridgeRegisterRequest) (*model.IMBridgeInstance, error) {
	return nil, nil
}

func (s *imControlPlaneStub) RecordHeartbeat(context.Context, string) (*model.IMBridgeHeartbeatResponse, error) {
	return nil, nil
}

func (s *imControlPlaneStub) UnregisterBridge(context.Context, string) error { return nil }
func (s *imControlPlaneStub) BindAction(context.Context, *model.IMActionBinding) error {
	return nil
}
func (s *imControlPlaneStub) AckDelivery(context.Context, string, int64, string) error {
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
func (s *imControlPlaneStub) ListDeliveryHistory(context.Context) ([]*model.IMDelivery, error) {
	return s.deliveries, nil
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
