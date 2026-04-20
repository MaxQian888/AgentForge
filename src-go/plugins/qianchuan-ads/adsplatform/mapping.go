package qianchuan

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/adsplatform"
)

// mapTokens reads {data: {access_token, refresh_token, expires_in, advertiser_ids}}.
func mapTokens(obj map[string]any) (*adsplatform.Tokens, error) {
	data, _ := obj["data"].(map[string]any)
	if data == nil {
		return nil, fmt.Errorf("qianchuan: missing data in token response")
	}
	access, _ := data["access_token"].(string)
	refresh, _ := data["refresh_token"].(string)
	expires := time.Now()
	if e, ok := data["expires_in"].(json.Number); ok {
		secs, _ := e.Int64()
		expires = time.Now().Add(time.Duration(secs) * time.Second)
	}
	// Extract advertiser_ids (present in OAuthExchange response).
	var advIDs []string
	if rawIDs, ok := data["advertiser_ids"].([]any); ok {
		for _, raw := range rawIDs {
			switch v := raw.(type) {
			case string:
				advIDs = append(advIDs, v)
			case json.Number:
				advIDs = append(advIDs, v.String())
			}
		}
	}
	return &adsplatform.Tokens{
		AccessToken:   access,
		RefreshToken:  refresh,
		ExpiresAt:     expires,
		AdvertiserIDs: advIDs,
	}, nil
}

// mapMetrics reads /qianchuan/report/live/get/ data.list rows.
func mapMetrics(obj map[string]any) (*adsplatform.MetricSnapshot, error) {
	data, _ := obj["data"].(map[string]any)
	if data == nil {
		return &adsplatform.MetricSnapshot{BucketAt: time.Now().UTC()}, nil
	}
	rows, _ := data["list"].([]any)
	ads := make([]adsplatform.AdMetric, 0, len(rows))
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			continue
		}
		ads = append(ads, adsplatform.AdMetric{
			AdID:   asString(row["ad_id"]),
			Status: asString(row["status"]),
			Spend:  asFloat(row["stat_cost"]),
			ROI:    asFloat(row["roi"]),
			CTR:    asFloat(row["ctr"]),
			CPM:    asFloat(row["cpm"]),
			Bid:    asFloat(row["bid"]),
			Budget: asFloat(row["budget"]),
		})
	}
	return &adsplatform.MetricSnapshot{
		BucketAt: time.Now().UTC().Truncate(time.Minute),
		Ads:      ads,
		Raw:      data,
	}, nil
}

// mapLiveSession reads /qianchuan/today_live/room/get/ shape.
func mapLiveSession(obj map[string]any, awemeID string) (*adsplatform.LiveSession, error) {
	data, _ := obj["data"].(map[string]any)
	if data == nil {
		return &adsplatform.LiveSession{AwemeID: awemeID, Status: "unknown"}, nil
	}
	return &adsplatform.LiveSession{
		AwemeID: awemeID,
		RoomID:  asString(data["room_id"]),
		Status:  asString(data["live_status"]),
		Viewers: asInt(data["audience_count"]),
		GMV:     asFloat(data["gmv"]),
		Raw:     data,
	}, nil
}

// mapMaterialHealth reads /qianchuan/material/health/get/ shape.
func mapMaterialHealth(obj map[string]any) ([]adsplatform.MaterialHealth, error) {
	data, _ := obj["data"].(map[string]any)
	if data == nil {
		return nil, nil
	}
	rows, _ := data["list"].([]any)
	out := make([]adsplatform.MaterialHealth, 0, len(rows))
	for _, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, adsplatform.MaterialHealth{
			MaterialID: asString(row["material_id"]),
			Status:     asString(row["status"]),
			Health:     asFloat(row["health_score"]),
			Reason:     asString(row["reason"]),
		})
	}
	return out, nil
}

// asString preserves string-valued numbers (big-int IDs) without precision loss.
func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		return fmt.Sprintf("%.0f", x)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

func asInt(v any) int64 {
	switch x := v.(type) {
	case json.Number:
		i, _ := x.Int64()
		return i
	case float64:
		return int64(x)
	}
	return 0
}
