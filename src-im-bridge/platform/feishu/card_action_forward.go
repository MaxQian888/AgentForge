package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// CardActionForwarder handles routing workflow-minted card-action callbacks
// to the Go backend's /api/v1/im/card-actions endpoint.
type CardActionForwarder struct {
	BackendURL string
	HTTPClient *http.Client
}

// ForwardInput carries the structured payload extracted from a Feishu
// card_action webhook.
type ForwardInput struct {
	Token       string
	ActionID    string
	Value       map[string]any
	ReplyTarget map[string]any
	UserID      string
	TenantID    string
}

// ForwardResult carries the outcome and an optional user-facing toast.
type ForwardResult struct {
	OK       bool
	Toast    string
	HTTPCode int
}

// ExtractCorrelationToken returns the correlation_token from a card action
// value map. Returns empty string if absent — caller should fall back to
// the legacy notify handler.
func ExtractCorrelationToken(value map[string]any) string {
	if value == nil {
		return ""
	}
	tok, ok := value["correlation_token"].(string)
	if !ok || tok == "" {
		return ""
	}
	return tok
}

// Forward POSTs the card-action to the backend and maps the response to
// a user-facing toast. Errors are never propagated to the Feishu webhook
// response path — the forwarder always returns a renderable result.
func (f *CardActionForwarder) Forward(ctx context.Context, in ForwardInput) ForwardResult {
	body := map[string]any{
		"correlation_token": in.Token,
		"action_id":         in.ActionID,
		"value":             in.Value,
		"replyTarget":       in.ReplyTarget,
		"user_id":           in.UserID,
		"tenant_id":         in.TenantID,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ForwardResult{OK: false, Toast: "操作失败"}
	}

	url := fmt.Sprintf("%s/api/v1/im/card-actions", f.BackendURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return ForwardResult{OK: false, Toast: "操作失败，请稍后再试"}
	}
	req.Header.Set("Content-Type", "application/json")

	client := f.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return ForwardResult{OK: false, Toast: "操作失败，请稍后再试"}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return ForwardResult{OK: true, HTTPCode: 200}
	}

	// Map backend error codes to localized toasts.
	respBody, _ := io.ReadAll(resp.Body)
	var parsed struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(respBody, &parsed)

	var toast string
	switch parsed.Code {
	case "card_action:expired":
		toast = "操作已过期"
	case "card_action:consumed":
		toast = "操作已处理"
	case "card_action:execution_not_waiting":
		toast = "工作流已结束"
	default:
		toast = "操作失败"
	}
	return ForwardResult{OK: false, Toast: toast, HTTPCode: resp.StatusCode}
}
