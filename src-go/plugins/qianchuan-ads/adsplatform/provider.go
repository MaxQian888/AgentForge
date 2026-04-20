package qianchuan

import (
	"context"
	"fmt"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
)

// Provider is the Qianchuan implementation of adsplatform.Provider.
// All methods funnel through Client and use mapping.go to neutralize the
// Qianchuan response shape.
type Provider struct {
	client *Client
}

// NewProvider wraps c.
func NewProvider(c *Client) *Provider { return &Provider{client: c} }

// Name returns the provider id used in registry / DB rows.
func (*Provider) Name() string { return "qianchuan" }

// OAuthAuthorizeURL builds the user-facing authorize URL.
// Reference: https://open.oceanengine.com/labels/7/docs/1696710606895111
func (p *Provider) OAuthAuthorizeURL(_ context.Context, state, redirectURI string) (string, error) {
	return fmt.Sprintf(
		"%s/openauth/index?app_id=%s&state=%s&material_auth=1&redirect_uri=%s",
		p.client.host, p.client.appID, state, redirectURI,
	), nil
}

// OAuthExchange swaps an authorization code for tokens.
func (p *Provider) OAuthExchange(ctx context.Context, code, redirectURI string) (*adsplatform.Tokens, error) {
	obj, err := p.client.OAuthExchange(ctx, code, redirectURI)
	if err != nil {
		return nil, err
	}
	return mapTokens(obj)
}

// RefreshToken exchanges a refresh token for a fresh pair.
func (p *Provider) RefreshToken(ctx context.Context, refreshToken string) (*adsplatform.Tokens, error) {
	obj, err := p.client.OAuthRefresh(ctx, refreshToken)
	if err != nil {
		return nil, err
	}
	return mapTokens(obj)
}

// FetchMetrics fetches the live-room metrics report and maps to the
// neutral shape. Spec calls out `today_live` series; we use
// /qianchuan/report/live/get/ as the canonical primary report.
func (p *Provider) FetchMetrics(ctx context.Context, b adsplatform.BindingRef, dims adsplatform.MetricDimensions) (*adsplatform.MetricSnapshot, error) {
	q := map[string]string{
		"advertiser_id":    b.AdvertiserID,
		"time_granularity": defaultStr(dims.Granular, "STAT_TIME_GRANULARITY_HOURLY"),
	}
	obj, err := p.client.GetJSON(ctx, b.AccessToken, "/qianchuan/report/live/get/", q)
	if err != nil {
		return nil, err
	}
	return mapMetrics(obj)
}

// FetchLiveSession returns the current state of a Douyin live room.
func (p *Provider) FetchLiveSession(ctx context.Context, b adsplatform.BindingRef, awemeID string) (*adsplatform.LiveSession, error) {
	q := map[string]string{
		"advertiser_id": b.AdvertiserID,
		"aweme_id":      awemeID,
	}
	obj, err := p.client.GetJSON(ctx, b.AccessToken, "/qianchuan/today_live/room/get/", q)
	if err != nil {
		return nil, err
	}
	return mapLiveSession(obj, awemeID)
}

// FetchMaterialHealth returns per-material health.
func (p *Provider) FetchMaterialHealth(ctx context.Context, b adsplatform.BindingRef, materialIDs []string) ([]adsplatform.MaterialHealth, error) {
	body := map[string]any{
		"advertiser_id": b.AdvertiserID,
		"material_ids":  materialIDs,
	}
	obj, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/material/health/get/", body)
	if err != nil {
		return nil, err
	}
	return mapMaterialHealth(obj)
}

// AdjustBid maps to /qianchuan/ad/bid/update/ on ad.oceanengine.com.
// newBid is in fen (CNY minor units); the upstream API expects yuan-as-decimal.
func (p *Provider) AdjustBid(ctx context.Context, b adsplatform.BindingRef, adID string, newBid adsplatform.Money) error {
	body := map[string]any{
		"advertiser_id": b.AdvertiserID,
		"ad_id":         adID,
		"bid":           float64(newBid.Amount) / 100.0,
	}
	_, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/bid/update/", body)
	return err
}

// AdjustBudget maps to /qianchuan/ad/budget/update/.
func (p *Provider) AdjustBudget(ctx context.Context, b adsplatform.BindingRef, adID string, newBudget adsplatform.Money) error {
	body := map[string]any{
		"advertiser_id": b.AdvertiserID,
		"ad_id":         adID,
		"budget":        float64(newBudget.Amount) / 100.0,
	}
	_, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/budget/update/", body)
	return err
}

// PauseAd maps to /qianchuan/ad/status/update/ with opt_status="disable".
func (p *Provider) PauseAd(ctx context.Context, b adsplatform.BindingRef, adID string) error {
	return p.setAdStatus(ctx, b, adID, "disable")
}

// ResumeAd maps to the same endpoint with opt_status="enable".
func (p *Provider) ResumeAd(ctx context.Context, b adsplatform.BindingRef, adID string) error {
	return p.setAdStatus(ctx, b, adID, "enable")
}

func (p *Provider) setAdStatus(ctx context.Context, b adsplatform.BindingRef, adID, status string) error {
	body := map[string]any{
		"advertiser_id": b.AdvertiserID,
		"ad_ids":        []string{adID},
		"opt_status":    status,
	}
	_, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/status/update/", body)
	return err
}

// ApplyMaterial swaps a creative on a specific ad.
func (p *Provider) ApplyMaterial(ctx context.Context, b adsplatform.BindingRef, adID, materialID string) error {
	body := map[string]any{
		"advertiser_id": b.AdvertiserID,
		"ad_id":         adID,
		"material_id":   materialID,
	}
	_, err := p.client.PostJSON(ctx, b.AccessToken, "/qianchuan/ad/creative/update/", body)
	return err
}

func defaultStr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// compile-time check.
var _ adsplatform.Provider = (*Provider)(nil)
