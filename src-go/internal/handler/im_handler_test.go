package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
)

type imHandlerValidator struct {
	validator *validator.Validate
}

func (v *imHandlerValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type imServiceStub struct {
	messageReq *model.IMMessageRequest
	commandReq *model.IMCommandRequest
	intentReq  *model.IMIntentRequest
	actionReq  *model.IMActionRequest
	sendReq    *model.IMSendRequest
	notifyReq  *model.IMNotifyRequest

	messageResp *model.IMMessageResponse
	commandResp *model.IMCommandResponse
	intentResp  *model.IMIntentResponse
	actionResp  *model.IMActionResponse

	messageErr error
	commandErr error
	intentErr  error
	actionErr  error
	sendErr    error
	notifyErr  error
}

func (s *imServiceStub) HandleIncoming(_ context.Context, req *model.IMMessageRequest) (*model.IMMessageResponse, error) {
	s.messageReq = req
	return s.messageResp, s.messageErr
}

func (s *imServiceStub) HandleCommand(_ context.Context, req *model.IMCommandRequest) (*model.IMCommandResponse, error) {
	s.commandReq = req
	return s.commandResp, s.commandErr
}

func (s *imServiceStub) HandleIntent(_ context.Context, req *model.IMIntentRequest) (*model.IMIntentResponse, error) {
	s.intentReq = req
	return s.intentResp, s.intentErr
}

func (s *imServiceStub) HandleAction(_ context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
	s.actionReq = req
	return s.actionResp, s.actionErr
}

func (s *imServiceStub) Send(_ context.Context, req *model.IMSendRequest) error {
	s.sendReq = req
	return s.sendErr
}

func (s *imServiceStub) Notify(_ context.Context, req *model.IMNotifyRequest) error {
	s.notifyReq = req
	return s.notifyErr
}

func newIMHandlerContext(method, target, body string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = &imHandlerValidator{validator: validator.New()}
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e, e.NewContext(req, rec), rec
}

func TestIMHandlerSuccessfulEndpoints(t *testing.T) {
	stub := &imServiceStub{
		messageResp: &model.IMMessageResponse{Reply: "hello"},
		commandResp: &model.IMCommandResponse{Result: "done", Success: true},
		intentResp:  &model.IMIntentResponse{Reply: "triaged", Intent: "task.query"},
		actionResp:  &model.IMActionResponse{Result: "started", Success: true, Status: model.IMActionStatusStarted},
	}
	h := handler.NewIMHandler(stub)

	_, messageCtx, messageRec := newIMHandlerContext(http.MethodPost, "/api/v1/im/message", `{"platform":"feishu","channelId":"chat-1","text":"hello"}`)
	if err := h.HandleMessage(messageCtx); err != nil {
		t.Fatalf("HandleMessage() error = %v", err)
	}
	if messageRec.Code != http.StatusOK || stub.messageReq == nil || stub.messageReq.Platform != "feishu" {
		t.Fatalf("HandleMessage() status/request = %d / %#v", messageRec.Code, stub.messageReq)
	}

	_, commandCtx, commandRec := newIMHandlerContext(http.MethodPost, "/api/v1/im/command", `{"platform":"slack","command":"/run","args":{"scope":"all"}}`)
	if err := h.HandleCommand(commandCtx); err != nil {
		t.Fatalf("HandleCommand() error = %v", err)
	}
	if commandRec.Code != http.StatusOK || stub.commandReq == nil || stub.commandReq.Command != "/run" {
		t.Fatalf("HandleCommand() status/request = %d / %#v", commandRec.Code, stub.commandReq)
	}

	_, sendCtx, sendRec := newIMHandlerContext(http.MethodPost, "/api/v1/im/send", `{"platform":"feishu","channelId":"chat-1","text":"ship it"}`)
	if err := h.Send(sendCtx); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if sendRec.Code != http.StatusOK || stub.sendReq == nil || stub.sendReq.Text != "ship it" {
		t.Fatalf("Send() status/request = %d / %#v", sendRec.Code, stub.sendReq)
	}

	_, notifyCtx, notifyRec := newIMHandlerContext(http.MethodPost, "/api/v1/im/notify", `{"platform":"feishu","channelId":"chat-1","event":"task.created","title":"Created"}`)
	if err := h.Notify(notifyCtx); err != nil {
		t.Fatalf("Notify() error = %v", err)
	}
	if notifyRec.Code != http.StatusOK || stub.notifyReq == nil || stub.notifyReq.Event != "task.created" {
		t.Fatalf("Notify() status/request = %d / %#v", notifyRec.Code, stub.notifyReq)
	}

	_, actionCtx, actionRec := newIMHandlerContext(http.MethodPost, "/api/v1/im/action", `{"platform":"feishu","action":"approve","entityId":"task-1","channelId":"chat-1"}`)
	if err := h.HandleAction(actionCtx); err != nil {
		t.Fatalf("HandleAction() error = %v", err)
	}
	if actionRec.Code != http.StatusOK || stub.actionReq == nil || stub.actionReq.Action != "approve" {
		t.Fatalf("HandleAction() status/request = %d / %#v", actionRec.Code, stub.actionReq)
	}

	_, intentCtx, intentRec := newIMHandlerContext(http.MethodPost, "/api/v1/im/intent", `{"text":"show me tasks","user_id":"user-1","project_id":"project-1"}`)
	if err := h.HandleIntent(intentCtx); err != nil {
		t.Fatalf("HandleIntent() error = %v", err)
	}
	if intentRec.Code != http.StatusOK || stub.intentReq == nil || stub.intentReq.Text != "show me tasks" {
		t.Fatalf("HandleIntent() status/request = %d / %#v", intentRec.Code, stub.intentReq)
	}
}

func TestIMHandlerValidationAndErrorBranches(t *testing.T) {
	t.Run("invalid request body returns bad request", func(t *testing.T) {
		h := handler.NewIMHandler(&imServiceStub{})
		_, ctx, rec := newIMHandlerContext(http.MethodPost, "/api/v1/im/message", `{`)
		if err := h.HandleMessage(ctx); err != nil {
			t.Fatalf("HandleMessage() error = %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", rec.Code)
		}
	})

	t.Run("validation failure returns unprocessable entity", func(t *testing.T) {
		h := handler.NewIMHandler(&imServiceStub{})
		_, ctx, rec := newIMHandlerContext(http.MethodPost, "/api/v1/im/intent", `{}`)
		if err := h.HandleIntent(ctx); err != nil {
			t.Fatalf("HandleIntent() error = %v", err)
		}
		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("status = %d, want 422", rec.Code)
		}
	})

	t.Run("service failure returns internal server error", func(t *testing.T) {
		h := handler.NewIMHandler(&imServiceStub{notifyErr: errors.New("bridge unavailable")})
		_, ctx, rec := newIMHandlerContext(http.MethodPost, "/api/v1/im/notify", `{"platform":"feishu","channelId":"chat-1","event":"task.created"}`)
		if err := h.Notify(ctx); err != nil {
			t.Fatalf("Notify() error = %v", err)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want 500", rec.Code)
		}

		var payload map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["message"] != "bridge unavailable" {
			t.Fatalf("payload = %#v", payload)
		}
	})
}
