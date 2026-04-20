package qianchuan_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	"github.com/react-go-quick-starter/server/internal/adsplatform/qianchuan"
)

func TestClient_AccessTokenHeaderInjected(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("Access-Token")
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
	}))
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "x", AppSecret: "y"})
	_, err := c.GetJSON(context.Background(), "tok123", "/qianchuan/ad/get/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if gotToken != "tok123" {
		t.Errorf("Access-Token=%q", gotToken)
	}
}

func TestClient_MapsAuthExpired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":40104,"message":"access_token expired"}`))
	}))
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "x", AppSecret: "y"})
	_, err := c.GetJSON(context.Background(), "t", "/x", nil)
	if !errors.Is(err, adsplatform.ErrAuthExpired) {
		t.Fatalf("want ErrAuthExpired, got %v", err)
	}
}

func TestClient_MapsRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":40100,"message":"rate limit exceeded"}`))
	}))
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{
		Host: srv.URL, AppID: "x", AppSecret: "y",
		MaxRetries: 0, // do not retry; we want the surface error
	})
	_, err := c.GetJSON(context.Background(), "t", "/x", nil)
	if !errors.Is(err, adsplatform.ErrRateLimited) {
		t.Fatalf("want ErrRateLimited, got %v", err)
	}
}

func TestClient_RetriesTransientThenSucceeds(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"code":51010,"message":"系统开小差"}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{"ok": true}})
	}))
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "x", AppSecret: "y", MaxRetries: 3})
	body, err := c.GetJSON(context.Background(), "t", "/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("hits=%d, want 3", hits)
	}
	if body["data"] == nil {
		t.Error("data missing")
	}
}
