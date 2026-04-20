package mock_test

import (
	"context"
	"testing"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	mockprov "github.com/react-go-quick-starter/server/internal/adsplatform/mock"
)

func TestProvider_RecordsCalls(t *testing.T) {
	p := mockprov.New("qianchuan")
	ctx := context.Background()
	ref := adsplatform.BindingRef{AdvertiserID: "A1", AccessToken: "tok"}
	if err := p.AdjustBid(ctx, ref, "AD7", adsplatform.Money{Amount: 4500, Currency: "CNY"}); err != nil {
		t.Fatal(err)
	}
	if err := p.PauseAd(ctx, ref, "AD7"); err != nil {
		t.Fatal(err)
	}
	calls := p.Calls()
	if len(calls) != 2 {
		t.Fatalf("calls=%d", len(calls))
	}
	if calls[0].Method != "AdjustBid" || calls[0].AdID != "AD7" {
		t.Errorf("call[0]=%+v", calls[0])
	}
}

func TestProvider_StubMetricsResponse(t *testing.T) {
	p := mockprov.New("qianchuan")
	p.SetMetrics(&adsplatform.MetricSnapshot{Ads: []adsplatform.AdMetric{{AdID: "AD1", ROI: 1.7}}})
	snap, err := p.FetchMetrics(context.Background(), adsplatform.BindingRef{}, adsplatform.MetricDimensions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Ads) != 1 || snap.Ads[0].ROI != 1.7 {
		t.Errorf("got %+v", snap)
	}
}
