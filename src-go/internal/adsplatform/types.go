// Package adsplatform defines the provider-neutral interface AgentForge uses
// to talk to advertising platforms (Qianchuan / 巨量千川 today; Taobao /
// JD Cloud Ads / Kuaishou / TikTok Ads in the future).
//
// Design invariants:
//   - Action surface is finite and auditable. Implementations expose ONLY
//     the methods on Provider; no raw / passthrough call exists.
//   - Tokens enter the package as plaintext via BindingRef.AccessToken;
//     the package does not import internal/secrets. Callers resolve the
//     secret and pass plaintext for the lifetime of the call.
//   - Currency is integer minor units (e.g. fen for CNY) to avoid float
//     drift; see Money.
//
// Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §8
package adsplatform

import "time"

// BindingRef is the per-call binding context passed to every Provider
// method. AccessToken is plaintext and lives only for the call frame.
type BindingRef struct {
	AdvertiserID string
	AwemeID      string
	AccessToken  string
}

// Tokens is the OAuth token tuple returned by OAuthExchange / RefreshToken.
type Tokens struct {
	AccessToken   string
	RefreshToken  string
	ExpiresAt     time.Time
	Scopes        []string
	AdvertiserIDs []string // populated by OAuthExchange; empty on RefreshToken
}

// Money holds an integer amount of currency minor units (fen for CNY).
type Money struct {
	Amount   int64  // minor units, e.g. 12345 = ¥123.45 when Currency=="CNY"
	Currency string // ISO 4217; "CNY" for Qianchuan
}

// MetricDimensions narrows a FetchMetrics call. Empty fields apply
// provider-default behaviour (e.g. "today" for Range).
type MetricDimensions struct {
	Range    string   // "today" | "yesterday" | "1h" | "5m"
	AdIDs    []string // empty → all
	AwemeIDs []string
	Granular string // "minute" | "hour" | "day"
}

// MetricSnapshot is the normalized adsplatform metric form. Provider
// implementations populate Live/Ads/Materials from their native shapes.
type MetricSnapshot struct {
	BucketAt  time.Time        `json:"bucket_at"`
	Live      map[string]any   `json:"live,omitempty"`
	Ads       []AdMetric       `json:"ads,omitempty"`
	Materials []MaterialHealth `json:"materials,omitempty"`
	Raw       map[string]any   `json:"raw,omitempty"` // diagnostic only
}

// AdMetric is one ad's per-bucket performance.
type AdMetric struct {
	AdID   string  `json:"ad_id"`
	Status string  `json:"status"`
	Spend  float64 `json:"spend"`
	ROI    float64 `json:"roi"`
	CTR    float64 `json:"ctr"`
	CPM    float64 `json:"cpm"`
	Bid    float64 `json:"bid"`
	Budget float64 `json:"budget"`
}

// LiveSession is the current state of one Douyin live room.
type LiveSession struct {
	AwemeID   string         `json:"aweme_id"`
	RoomID    string         `json:"room_id"` // string to preserve big-int precision
	Status    string         `json:"status"`  // 'warming' | 'live' | 'ended'
	StartedAt time.Time      `json:"started_at"`
	Viewers   int64          `json:"viewers"`
	GMV       float64        `json:"gmv"`
	Raw       map[string]any `json:"raw,omitempty"`
}

// MaterialHealth is the per-creative-asset health snapshot.
type MaterialHealth struct {
	MaterialID string  `json:"material_id"`
	Status     string  `json:"status"`
	Health     float64 `json:"health"` // 0..1
	Reason     string  `json:"reason,omitempty"`
}
