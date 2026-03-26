package feishu

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type roundTripperFunc func(req *http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func newRewrittenClient(t *testing.T, rawBaseURL string) *http.Client {
	t.Helper()

	baseURL, err := url.Parse(rawBaseURL)
	if err != nil {
		t.Fatalf("parse base URL: %v", err)
	}

	return &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			clone := req.Clone(req.Context())
			clone.URL.Scheme = baseURL.Scheme
			clone.URL.Host = baseURL.Host
			clone.Host = baseURL.Host
			return http.DefaultTransport.RoundTrip(clone)
		}),
	}
}

func TestSDKCardUpdater_UpdateRequiresCallbackToken(t *testing.T) {
	updater := &sdkCardUpdater{}
	err := updater.Update(context.Background(), "   ", &core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:       core.FeishuCardModeTemplate,
			TemplateID: "ctp_123",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "requires callback token") {
		t.Fatalf("error = %v", err)
	}
}

func TestSDKCardUpdater_UpdateSendsTenantTokenAndCardPayload(t *testing.T) {
	var sawTokenRequest bool
	var sawUpdateRequest bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			sawTokenRequest = true
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","expire":7200,"tenant_access_token":"tenant-token"}`))
		case "/open-apis/interactive/v1/card/update":
			sawUpdateRequest = true
			if got := r.Header.Get("Authorization"); got != "Bearer tenant-token" {
				t.Fatalf("Authorization = %q", got)
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			var payload map[string]any
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Fatalf("decode update payload: %v", err)
			}
			if payload["token"] != "cb-token-1" {
				t.Fatalf("payload = %+v", payload)
			}
			card, ok := payload["card"].(map[string]any)
			if !ok || card["type"] != "template" {
				t.Fatalf("payload = %+v", payload)
			}

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	updater := &sdkCardUpdater{
		client: lark.NewClient(
			"app-id",
			"app-secret",
			lark.WithOpenBaseUrl(server.URL),
			lark.WithEnableTokenCache(false),
			lark.WithHttpClient(server.Client()),
		),
		appID:      "app-id",
		appSecret:  "app-secret",
		httpClient: newRewrittenClient(t, server.URL),
	}

	err := updater.Update(context.Background(), "cb-token-1", &core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:       core.FeishuCardModeTemplate,
			TemplateID: "ctp_native",
			TemplateVariable: map[string]any{
				"status": "done",
			},
		},
	})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if !sawTokenRequest || !sawUpdateRequest {
		t.Fatalf("token request = %v, update request = %v", sawTokenRequest, sawUpdateRequest)
	}
}

func TestSDKCardUpdater_UpdateReportsTokenFetchFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open-apis/auth/v3/tenant_access_token/internal" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":999,"msg":"denied"}`))
	}))
	defer server.Close()

	updater := &sdkCardUpdater{
		client: lark.NewClient(
			"app-id",
			"app-secret",
			lark.WithOpenBaseUrl(server.URL),
			lark.WithEnableTokenCache(false),
			lark.WithHttpClient(server.Client()),
		),
		appID:      "app-id",
		appSecret:  "app-secret",
		httpClient: newRewrittenClient(t, server.URL),
	}

	err := updater.Update(context.Background(), "cb-token-1", &core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:       core.FeishuCardModeTemplate,
			TemplateID: "ctp_native",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "code=999 msg=denied") {
		t.Fatalf("error = %v", err)
	}
}

func TestSDKCardUpdater_UpdateReportsNon2xxResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"msg":"ok","expire":7200,"tenant_access_token":"tenant-token"}`))
		case "/open-apis/interactive/v1/card/update":
			http.Error(w, "boom", http.StatusBadGateway)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	updater := &sdkCardUpdater{
		client: lark.NewClient(
			"app-id",
			"app-secret",
			lark.WithOpenBaseUrl(server.URL),
			lark.WithEnableTokenCache(false),
			lark.WithHttpClient(server.Client()),
		),
		appID:      "app-id",
		appSecret:  "app-secret",
		httpClient: newRewrittenClient(t, server.URL),
	}

	err := updater.Update(context.Background(), "cb-token-1", &core.NativeMessage{
		Platform: "feishu",
		FeishuCard: &core.FeishuCardPayload{
			Mode:       core.FeishuCardModeTemplate,
			TemplateID: "ctp_native",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "status=502") {
		t.Fatalf("error = %v", err)
	}
}
