// Package qianchuan implements adsplatform.Provider against the Qianchuan
// (巨量千川) OpenAPI hosted at api.oceanengine.com / ad.oceanengine.com.
//
// Authentication: every request carries `Access-Token: <accessToken>`.
// The reference TS project uses the same scheme (no body HMAC). The
// App ID / App Secret pair is required only by the OAuth code-exchange
// endpoint, not by data-plane requests.
//
// Spec: docs/superpowers/specs/2026-04-20-ecommerce-streaming-employee-design.md §8
package qianchuan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/agentforge/server/internal/adsplatform"
)

const (
	// DefaultHost is the default Qianchuan ad host.
	DefaultHost = "https://ad.oceanengine.com"
	// APIV1Prefix is the v1.0 OpenAPI prefix.
	APIV1Prefix = "/open_api/v1.0"
	// OAuth2Prefix is the OAuth2 endpoint prefix.
	OAuth2Prefix = "/open_api/oauth2"

	defaultRetries = 3
	defaultTimeout = 15 * time.Second
)

// Options configures Client.
type Options struct {
	Host       string       // e.g. https://ad.oceanengine.com (no trailing slash)
	AppID      string       // QIANCHUAN_APP_ID
	AppSecret  string       // QIANCHUAN_APP_SECRET
	HTTPClient *http.Client // optional; defaults to a 15s-timeout client
	MaxRetries int          // <0 disables retry; default 3 when zero AND HTTPClient is nil
}

// Client is the lower-level HTTP client. Higher-level Provider methods
// live in provider.go and call into Client.
type Client struct {
	host    string
	appID   string
	secret  string
	httpc   *http.Client
	retries int
}

// NewClient returns a configured Client. Missing host falls back to DefaultHost.
//
// MaxRetries semantics:
//   - opts.MaxRetries < 0 → defaultRetries (3) — caller wants the default
//   - opts.MaxRetries == 0 → 0 retries (caller explicitly disabled retry)
//   - opts.MaxRetries > 0 → use as-is
//
// To get the default with a custom HTTP client, leave MaxRetries at -1.
func NewClient(opts Options) *Client {
	host := strings.TrimRight(opts.Host, "/")
	if host == "" {
		host = DefaultHost
	}
	httpc := opts.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: defaultTimeout}
	}
	retries := opts.MaxRetries
	if retries < 0 {
		retries = defaultRetries
	}
	return &Client{host: host, appID: opts.AppID, secret: opts.AppSecret, httpc: httpc, retries: retries}
}

// GetJSON issues a GET against path with query params. Returns the
// decoded top-level object (the wrapper {code, message, request_id, data}).
func (c *Client) GetJSON(ctx context.Context, accessToken, path string, query map[string]string) (map[string]any, error) {
	u := c.host + APIV1Prefix + path
	if len(query) > 0 {
		sep := "?"
		for k, v := range query {
			u += sep + k + "=" + v
			sep = "&"
		}
	}
	return c.do(ctx, http.MethodGet, u, accessToken, nil)
}

// PostJSON issues a POST with body marshalled as JSON.
func (c *Client) PostJSON(ctx context.Context, accessToken, path string, body any) (map[string]any, error) {
	u := c.host + APIV1Prefix + path
	return c.do(ctx, http.MethodPost, u, accessToken, body)
}

// OAuthExchange swaps an authorization code for a token pair.
// Endpoint: <host>/open_api/oauth2/access_token/
func (c *Client) OAuthExchange(ctx context.Context, code, redirectURI string) (map[string]any, error) {
	u := c.host + OAuth2Prefix + "/access_token/"
	body := map[string]any{
		"app_id":       c.appID,
		"secret":       c.secret,
		"grant_type":   "auth_code",
		"auth_code":    code,
		"redirect_uri": redirectURI,
	}
	return c.do(ctx, http.MethodPost, u, "", body)
}

// OAuthRefresh refreshes an expired access token.
func (c *Client) OAuthRefresh(ctx context.Context, refreshToken string) (map[string]any, error) {
	u := c.host + OAuth2Prefix + "/refresh_token/"
	body := map[string]any{
		"app_id":        c.appID,
		"secret":        c.secret,
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
	}
	return c.do(ctx, http.MethodPost, u, "", body)
}

func (c *Client) do(ctx context.Context, method, url, accessToken string, body any) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		obj, err := c.attempt(ctx, method, url, accessToken, body)
		if err == nil {
			return obj, nil
		}
		lastErr = err
		if !isRetryable(err) {
			return nil, err
		}
		if attempt == c.retries {
			break
		}
		// exponential backoff with jitter: 1s, 2s, 4s (+50% jitter)
		base := time.Duration(1<<attempt) * time.Second
		// rand.Int63n panics on n<=0; guard for the unusual base==0 case.
		var jitter time.Duration
		if half := int64(base / 2); half > 0 {
			jitter = time.Duration(rand.Int63n(half))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(base + jitter):
		}
	}
	return nil, lastErr
}

func (c *Client) attempt(ctx context.Context, method, url, accessToken string, body any) (map[string]any, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("qianchuan: marshal: %w", err)
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return nil, fmt.Errorf("qianchuan: build req: %w", err)
	}
	if accessToken != "" {
		req.Header.Set("Access-Token", accessToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", adsplatform.ErrTransientFailure, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	// Map HTTP status first.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%w: http %d", adsplatform.ErrAuthExpired, resp.StatusCode)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("%w: http 429", adsplatform.ErrRateLimited)
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("%w: http %d", adsplatform.ErrTransientFailure, resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%w: http %d body=%s", adsplatform.ErrInvalidRequest, resp.StatusCode, truncate(raw))
	}
	// Use json.Number to avoid float64 precision loss on big-int IDs.
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("qianchuan: decode: %w body=%s", err, truncate(raw))
	}
	// Map Qianchuan-level error codes.
	if codeJSON, ok := obj["code"].(json.Number); ok {
		code, _ := codeJSON.Int64()
		switch {
		case code == 0:
			return obj, nil
		case code == 40100:
			return nil, fmt.Errorf("%w: code=%d message=%v", adsplatform.ErrRateLimited, code, obj["message"])
		case code == 40104 || code == 40105:
			return nil, fmt.Errorf("%w: code=%d", adsplatform.ErrAuthExpired, code)
		case code == 51010 || code == 51011:
			return nil, fmt.Errorf("%w: code=%d", adsplatform.ErrTransientFailure, code)
		default:
			return nil, fmt.Errorf("%w: code=%d message=%v", adsplatform.ErrUpstreamRejected, code, obj["message"])
		}
	}
	return obj, nil
}

func isRetryable(err error) bool {
	return errors.Is(err, adsplatform.ErrTransientFailure) || errors.Is(err, adsplatform.ErrRateLimited)
}

func truncate(b []byte) string {
	if len(b) <= 200 {
		return string(b)
	}
	return string(b[:200]) + "...(truncated)"
}
