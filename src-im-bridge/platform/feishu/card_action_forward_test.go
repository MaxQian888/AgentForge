package feishu

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractCorrelationToken_Present(t *testing.T) {
	val := map[string]any{
		"correlation_token": "11111111-1111-1111-1111-111111111111",
		"action_id":         "approve",
	}
	tok := ExtractCorrelationToken(val)
	if tok != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("got %q", tok)
	}
}

func TestExtractCorrelationToken_Missing(t *testing.T) {
	val := map[string]any{"foo": "bar"}
	tok := ExtractCorrelationToken(val)
	if tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestExtractCorrelationToken_Nil(t *testing.T) {
	if tok := ExtractCorrelationToken(nil); tok != "" {
		t.Errorf("expected empty, got %q", tok)
	}
}

func TestCardActionForwarder_200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"outcome":"resumed"}`))
	}))
	defer srv.Close()

	f := &CardActionForwarder{BackendURL: srv.URL}
	result := f.Forward(context.Background(), ForwardInput{
		Token: "tok", ActionID: "approve", UserID: "U1", TenantID: "T1",
	})
	if !result.OK {
		t.Error("expected OK")
	}
}

func TestCardActionForwarder_410Expired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusGone)
		_, _ = w.Write([]byte(`{"code":"card_action:expired"}`))
	}))
	defer srv.Close()

	f := &CardActionForwarder{BackendURL: srv.URL}
	result := f.Forward(context.Background(), ForwardInput{Token: "tok"})
	if result.OK {
		t.Error("expected not OK")
	}
	if result.Toast != "操作已过期" {
		t.Errorf("toast = %q", result.Toast)
	}
}

func TestCardActionForwarder_409Consumed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"code":"card_action:consumed"}`))
	}))
	defer srv.Close()

	f := &CardActionForwarder{BackendURL: srv.URL}
	result := f.Forward(context.Background(), ForwardInput{Token: "tok"})
	if result.Toast != "操作已处理" {
		t.Errorf("toast = %q", result.Toast)
	}
}

func TestCardActionForwarder_NetworkError(t *testing.T) {
	f := &CardActionForwarder{BackendURL: "http://127.0.0.1:1"} // unreachable
	result := f.Forward(context.Background(), ForwardInput{Token: "tok"})
	if result.OK {
		t.Error("expected not OK")
	}
	if result.Toast == "" {
		t.Error("expected a toast message")
	}
}
