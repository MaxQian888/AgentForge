package qianchuan_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/adsplatform"
)

func newJSONStub(t *testing.T, route string, payload map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, route) {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(payload)
	}))
}

func TestProvider_FetchMetrics_MapsBigIntRoomID(t *testing.T) {
	srv := newJSONStub(t, "/qianchuan/report/live/get/", map[string]any{
		"code": 0,
		"data": map[string]any{
			"list": []map[string]any{
				{"ad_id": "AD7", "stat_cost": 10.0, "roi": 1.7, "show_cnt": 100, "click_cnt": 5, "bid": 5.0, "budget": 100.0, "status": "STATUS_DELIVERY_OK"},
			},
		},
	})
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "a", AppSecret: "s"})
	p := qianchuan.NewProvider(c)
	snap, err := p.FetchMetrics(context.Background(),
		adsplatform.BindingRef{AdvertiserID: "1234567890", AccessToken: "t"},
		adsplatform.MetricDimensions{Range: "today"})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ads) != 1 || snap.Ads[0].AdID != "AD7" || snap.Ads[0].ROI != 1.7 {
		t.Fatalf("ads=%+v", snap.Ads)
	}
}

func TestProvider_AdjustBid_PostsBidUpdate(t *testing.T) {
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": map[string]any{}})
	}))
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "a", AppSecret: "s"})
	p := qianchuan.NewProvider(c)
	err := p.AdjustBid(context.Background(),
		adsplatform.BindingRef{AdvertiserID: "100", AccessToken: "t"},
		"AD7", adsplatform.Money{Amount: 4500, Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	if seen["ad_id"] != "AD7" {
		t.Errorf("ad_id=%v", seen["ad_id"])
	}
	if bid, ok := seen["bid"].(float64); !ok || bid != 45.0 {
		t.Errorf("bid=%v (want 45.0)", seen["bid"])
	}
}

func TestProvider_PauseAd_SetsStatus(t *testing.T) {
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0})
	}))
	defer srv.Close()
	c := qianchuan.NewClient(qianchuan.Options{Host: srv.URL, AppID: "a", AppSecret: "s"})
	p := qianchuan.NewProvider(c)
	if err := p.PauseAd(context.Background(),
		adsplatform.BindingRef{AdvertiserID: "100", AccessToken: "t"}, "AD7"); err != nil {
		t.Fatal(err)
	}
	if seen["opt_status"] != "disable" {
		t.Errorf("opt_status=%v", seen["opt_status"])
	}
}
