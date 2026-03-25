package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/agentforge/im-bridge/core"
)

type sdkCardUpdater struct {
	client     *lark.Client
	appID      string
	appSecret  string
	httpClient *http.Client
}

func (u *sdkCardUpdater) Update(ctx context.Context, callbackToken string, message *core.NativeMessage) error {
	if strings.TrimSpace(callbackToken) == "" {
		return fmt.Errorf("feishu native update requires callback token")
	}
	cardPayload, err := renderFeishuNativePayload(message)
	if err != nil {
		return err
	}
	tokenResp, err := u.client.GetTenantAccessTokenBySelfBuiltApp(ctx, &larkcore.SelfBuiltTenantAccessTokenReq{
		AppID:     u.appID,
		AppSecret: u.appSecret,
	})
	if err != nil {
		return fmt.Errorf("fetch feishu tenant access token: %w", err)
	}
	if tokenResp == nil || !tokenResp.Success() || strings.TrimSpace(tokenResp.TenantAccessToken) == "" {
		code := -1
		msg := ""
		if tokenResp != nil {
			code = tokenResp.Code
			msg = tokenResp.Msg
		}
		return fmt.Errorf("fetch feishu tenant access token failed: code=%d msg=%s", code, msg)
	}

	body, err := json.Marshal(map[string]any{
		"token": callbackToken,
		"card":  cardPayload,
	})
	if err != nil {
		return fmt.Errorf("marshal feishu native update payload: %w", err)
	}
	httpClient := u.httpClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://open.feishu.cn/open-apis/interactive/v1/card/update", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create feishu native update request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(tokenResp.TenantAccessToken))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send feishu native update request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read feishu native update response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("feishu native update failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, &payload); err == nil && payload.Code != 0 {
			return fmt.Errorf("feishu native update failed: code=%d msg=%s", payload.Code, payload.Msg)
		}
	}
	return nil
}
