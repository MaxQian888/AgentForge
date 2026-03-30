package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	"github.com/agentforge/im-bridge/platform/dingtalk"
	"github.com/agentforge/im-bridge/platform/discord"
	"github.com/agentforge/im-bridge/platform/feishu"
	"github.com/agentforge/im-bridge/platform/qq"
	"github.com/agentforge/im-bridge/platform/qqbot"
	"github.com/agentforge/im-bridge/platform/slack"
	"github.com/agentforge/im-bridge/platform/telegram"
	"github.com/agentforge/im-bridge/platform/wecom"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig_ReadsExplicitPlatformSettings(t *testing.T) {
	t.Setenv("IM_PLATFORM", "slack")
	t.Setenv("IM_TRANSPORT_MODE", "stub")
	t.Setenv("AGENTFORGE_API_BASE", "http://example.test")
	t.Setenv("AGENTFORGE_PROJECT_ID", "proj-1")
	t.Setenv("AGENTFORGE_API_KEY", "secret")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test")
	t.Setenv("SLACK_APP_TOKEN", "xapp-test")
	t.Setenv("NOTIFY_PORT", "9001")
	t.Setenv("TEST_PORT", "9002")

	cfg := loadConfig()

	if cfg.Platform != "slack" {
		t.Fatalf("Platform = %q, want slack", cfg.Platform)
	}
	if cfg.TransportMode != "stub" {
		t.Fatalf("TransportMode = %q, want stub", cfg.TransportMode)
	}
	if cfg.SlackBotToken != "xoxb-test" {
		t.Fatalf("SlackBotToken = %q, want xoxb-test", cfg.SlackBotToken)
	}
	if cfg.SlackAppToken != "xapp-test" {
		t.Fatalf("SlackAppToken = %q, want xapp-test", cfg.SlackAppToken)
	}
}

func TestLoadConfig_ReadsWeComSettings(t *testing.T) {
	t.Setenv("IM_PLATFORM", "wecom")
	t.Setenv("IM_TRANSPORT_MODE", "live")
	t.Setenv("WECOM_CORP_ID", "corp-id")
	t.Setenv("WECOM_AGENT_ID", "1000002")
	t.Setenv("WECOM_AGENT_SECRET", "agent-secret")
	t.Setenv("WECOM_CALLBACK_TOKEN", "callback-token")
	t.Setenv("WECOM_CALLBACK_PORT", "9080")
	t.Setenv("WECOM_CALLBACK_PATH", "/wecom/callback")

	cfg := loadConfig()

	if cfg.Platform != "wecom" {
		t.Fatalf("Platform = %q, want wecom", cfg.Platform)
	}
	if cfg.WeComCorpID != "corp-id" || cfg.WeComAgentID != "1000002" || cfg.WeComCallbackPath != "/wecom/callback" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestLoadConfig_ReadsQQAndQQBotSettings(t *testing.T) {
	t.Setenv("IM_PLATFORM", "qq")
	t.Setenv("IM_TRANSPORT_MODE", "live")
	t.Setenv("QQ_ONEBOT_WS_URL", "ws://127.0.0.1:3001/onebot/v11/ws")
	t.Setenv("QQ_ACCESS_TOKEN", "qq-token")
	t.Setenv("QQBOT_APP_ID", "1024")
	t.Setenv("QQBOT_APP_SECRET", "secret")
	t.Setenv("QQBOT_CALLBACK_PORT", "9082")
	t.Setenv("QQBOT_CALLBACK_PATH", "/qqbot/callback")
	t.Setenv("QQBOT_API_BASE", "https://api.sgroup.qq.com")
	t.Setenv("QQBOT_TOKEN_BASE", "https://bots.qq.com")

	cfg := loadConfig()

	if cfg.Platform != "qq" {
		t.Fatalf("Platform = %q, want qq", cfg.Platform)
	}
	if cfg.QQOneBotWSURL != "ws://127.0.0.1:3001/onebot/v11/ws" || cfg.QQAccessToken != "qq-token" {
		t.Fatalf("qq cfg = %+v", cfg)
	}
	if cfg.QQBotAppID != "1024" || cfg.QQBotCallbackPath != "/qqbot/callback" || cfg.QQBotAPIBase != "https://api.sgroup.qq.com" {
		t.Fatalf("qqbot cfg = %+v", cfg)
	}
}

func TestLoadConfig_ReadsDingTalkCardTemplateSettings(t *testing.T) {
	t.Setenv("IM_PLATFORM", "dingtalk")
	t.Setenv("IM_TRANSPORT_MODE", "live")
	t.Setenv("DINGTALK_APP_KEY", "ding-key")
	t.Setenv("DINGTALK_APP_SECRET", "ding-secret")
	t.Setenv("DINGTALK_CARD_TEMPLATE_ID", "template-1.schema")

	cfg := loadConfig()

	if cfg.Platform != "dingtalk" {
		t.Fatalf("Platform = %q, want dingtalk", cfg.Platform)
	}
	if cfg.DingTalkCardTemplateID != "template-1.schema" {
		t.Fatalf("DingTalkCardTemplateID = %q, want template-1.schema", cfg.DingTalkCardTemplateID)
	}
}

func TestSelectPlatform_RejectsMissingCredentials(t *testing.T) {
	cfg := &config{
		Platform:      "dingtalk",
		TransportMode: "live",
		TestPort:      "9010",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected selectPlatform to fail when required DingTalk credentials are missing")
	}
}

func TestEnvOrDefault_ReturnsFallbackForMissingValue(t *testing.T) {
	t.Setenv("AGENTFORGE_API_BASE", "")

	if got := envOrDefault("AGENTFORGE_API_BASE", "http://localhost:7777"); got != "http://localhost:7777" {
		t.Fatalf("envOrDefault = %q", got)
	}
}

func TestSelectPlatform_ReturnsStubForFeishuWithoutCredentials(t *testing.T) {
	cfg := &config{
		Platform:      "feishu",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "feishu-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveFeishuAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:      "feishu",
		TransportMode: "live",
		FeishuApp:     "app-id",
		FeishuSec:     "app-secret",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "feishu-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*feishu.Live); !ok {
		t.Fatalf("platform type = %T, want *feishu.Live", platform)
	}
}

func TestSelectPlatform_ReturnsStubForSlackWithCredentials(t *testing.T) {
	cfg := &config{
		Platform:      "slack",
		TransportMode: "stub",
		SlackBotToken: "xoxb-test",
		SlackAppToken: "xapp-test",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "slack-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveSlackAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:      "slack",
		TransportMode: "live",
		SlackBotToken: "xoxb-test",
		SlackAppToken: "xapp-test",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "slack-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*slack.Live); !ok {
		t.Fatalf("platform type = %T, want *slack.Live", platform)
	}
}

func TestSelectPlatform_ReturnsStubForDingTalkWithCredentials(t *testing.T) {
	cfg := &config{
		Platform:          "dingtalk",
		TransportMode:     "stub",
		DingTalkAppKey:    "ding-key",
		DingTalkAppSecret: "ding-secret",
		TestPort:          "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "dingtalk-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveDingTalkAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:          "dingtalk",
		TransportMode:     "live",
		DingTalkAppKey:    "ding-key",
		DingTalkAppSecret: "ding-secret",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "dingtalk-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*dingtalk.Live); !ok {
		t.Fatalf("platform type = %T, want *dingtalk.Live", platform)
	}
}

func TestSelectPlatform_ReturnsStubForTelegramWithoutLiveSettings(t *testing.T) {
	cfg := &config{
		Platform:      "telegram",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "telegram-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveTelegramAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:           "telegram",
		TransportMode:      "live",
		TelegramBotToken:   "telegram-bot-token",
		TelegramUpdateMode: "longpoll",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "telegram-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*telegram.Live); !ok {
		t.Fatalf("platform type = %T, want *telegram.Live", platform)
	}
}

func TestSelectPlatform_RejectsTelegramWebhookConfigForLongPollingMode(t *testing.T) {
	cfg := &config{
		Platform:           "telegram",
		TransportMode:      "live",
		TelegramBotToken:   "telegram-bot-token",
		TelegramUpdateMode: "longpoll",
		TelegramWebhookURL: "https://example.test/webhook",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected telegram live selection to reject webhook config when long polling is selected")
	}
}

func TestSelectPlatform_RejectsUnsupportedPlatform(t *testing.T) {
	cfg := &config{
		Platform:      "teams",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected unsupported platform to fail")
	}
}

func TestSelectPlatform_ReturnsStubForWeComWithoutLiveSettings(t *testing.T) {
	cfg := &config{
		Platform:      "wecom",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "wecom-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveWeComAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:           "wecom",
		TransportMode:      "live",
		WeComCorpID:        "corp-id",
		WeComAgentID:       "1000002",
		WeComAgentSecret:   "agent-secret",
		WeComCallbackToken: "callback-token",
		WeComCallbackPort:  "9012",
		WeComCallbackPath:  "/wecom/callback",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "wecom-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*wecom.Live); !ok {
		t.Fatalf("platform type = %T, want *wecom.Live", platform)
	}
}

func TestSelectPlatform_RejectsWeComLiveWithoutCallbackConfig(t *testing.T) {
	cfg := &config{
		Platform:         "wecom",
		TransportMode:    "live",
		WeComCorpID:      "corp-id",
		WeComAgentID:     "1000002",
		WeComAgentSecret: "agent-secret",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected wecom live selection to fail")
	}
	if err != nil && err.Error() == "" {
		t.Fatal("expected actionable wecom config error")
	}
}

func TestSelectPlatform_ReturnsStubForQQWithoutLiveSettings(t *testing.T) {
	cfg := &config{
		Platform:      "qq",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "qq-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveQQAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:      "qq",
		TransportMode: "live",
		QQOneBotWSURL: "ws://127.0.0.1:3001/onebot/v11/ws",
		QQAccessToken: "qq-token",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "qq-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*qq.Live); !ok {
		t.Fatalf("platform type = %T, want *qq.Live", platform)
	}
}

func TestSelectPlatform_RejectsQQLiveWithoutWSURL(t *testing.T) {
	cfg := &config{
		Platform:      "qq",
		TransportMode: "live",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected qq live selection to fail")
	}
	if !strings.Contains(err.Error(), "QQ_ONEBOT_WS_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectPlatform_ReturnsStubForQQBotWithoutLiveSettings(t *testing.T) {
	cfg := &config{
		Platform:      "qqbot",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "qqbot-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveQQBotAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:          "qqbot",
		TransportMode:     "live",
		QQBotAppID:        "1024",
		QQBotAppSecret:    "secret",
		QQBotCallbackPort: "9013",
		QQBotCallbackPath: "/qqbot/callback",
		QQBotAPIBase:      "https://api.sgroup.qq.com",
		QQBotTokenBase:    "https://bots.qq.com",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "qqbot-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*qqbot.Live); !ok {
		t.Fatalf("platform type = %T, want *qqbot.Live", platform)
	}
}

func TestSelectPlatform_RejectsQQBotLiveWithoutCallbackConfig(t *testing.T) {
	cfg := &config{
		Platform:       "qqbot",
		TransportMode:  "live",
		QQBotAppID:     "1024",
		QQBotAppSecret: "secret",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected qqbot live selection to fail")
	}
	if !strings.Contains(err.Error(), "QQBOT_CALLBACK_PORT") {
		t.Fatalf("err = %v", err)
	}
}

func TestSelectPlatform_AllowsStubModeWithoutProviderCredentials(t *testing.T) {
	cfg := &config{
		Platform:      "slack",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "slack-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsStubForDiscordWithoutLiveSettings(t *testing.T) {
	cfg := &config{
		Platform:      "discord",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "discord-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsLiveDiscordAdapterWhenConfigured(t *testing.T) {
	cfg := &config{
		Platform:                "discord",
		TransportMode:           "live",
		DiscordAppID:            "app-123",
		DiscordBotToken:         "bot-token",
		DiscordPublicKey:        "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		DiscordInteractionsPort: "9011",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "discord-live" {
		t.Fatalf("platform name = %q", platform.Name())
	}
	if _, ok := platform.(*discord.Live); !ok {
		t.Fatalf("platform type = %T, want *discord.Live", platform)
	}
}

func TestSelectPlatform_RejectsDiscordLiveWithoutInteractionPort(t *testing.T) {
	cfg := &config{
		Platform:         "discord",
		TransportMode:    "live",
		DiscordAppID:     "app-123",
		DiscordBotToken:  "bot-token",
		DiscordPublicKey: "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected discord live selection to reject missing interaction port")
	}
}

func TestLookupPlatformDescriptor_ReturnsCapabilities(t *testing.T) {
	descriptor, err := lookupPlatformDescriptor("feishu")
	if err != nil {
		t.Fatalf("lookupPlatformDescriptor error: %v", err)
	}
	if descriptor.ID != "feishu" {
		t.Fatalf("id = %q, want feishu", descriptor.ID)
	}
	if descriptor.Metadata.Source != "feishu" {
		t.Fatalf("source = %q, want feishu", descriptor.Metadata.Source)
	}
	if !descriptor.Metadata.Capabilities.SupportsMentions {
		t.Fatal("expected feishu descriptor to support mentions")
	}
	if !descriptor.supportsTransport(transportModeStub) || !descriptor.supportsTransport(transportModeLive) {
		t.Fatalf("supported transports = %+v", descriptor.SupportedTransportModes)
	}
	if descriptor.Features.FeishuCards == nil || !descriptor.Features.FeishuCards.SupportsTemplateCards || !descriptor.Features.FeishuCards.SupportsDelayedUpdates {
		t.Fatalf("features = %+v", descriptor.Features)
	}
}

func TestLookupPlatformDescriptor_ReportsWeComCapabilities(t *testing.T) {
	descriptor, err := lookupPlatformDescriptor("wecom")
	if err != nil {
		t.Fatalf("lookupPlatformDescriptor error: %v", err)
	}
	if descriptor.Metadata.Source != "wecom" {
		t.Fatalf("source = %q, want wecom", descriptor.Metadata.Source)
	}
	if descriptor.NewStub == nil || descriptor.NewLive == nil {
		t.Fatalf("expected wecom to be runnable, got NewStub=%v NewLive=%v", descriptor.NewStub, descriptor.NewLive)
	}
	if descriptor.Metadata.Capabilities.ActionCallbackMode != core.ActionCallbackWebhook {
		t.Fatalf("callback mode = %q", descriptor.Metadata.Capabilities.ActionCallbackMode)
	}
	if !descriptor.Metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected wecom live transport to require a public callback")
	}
}

func TestLookupPlatformDescriptor_ReportsQQCapabilities(t *testing.T) {
	descriptor, err := lookupPlatformDescriptor("qq")
	if err != nil {
		t.Fatalf("lookupPlatformDescriptor error: %v", err)
	}
	if descriptor.Metadata.Source != "qq" {
		t.Fatalf("source = %q, want qq", descriptor.Metadata.Source)
	}
	if descriptor.NewStub == nil || descriptor.NewLive == nil {
		t.Fatalf("expected qq to be runnable, got NewStub=%v NewLive=%v", descriptor.NewStub, descriptor.NewLive)
	}
	if !descriptor.Metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected qq live transport to support slash-style commands")
	}
}

func TestLookupPlatformDescriptor_ReportsQQBotCapabilities(t *testing.T) {
	descriptor, err := lookupPlatformDescriptor("qqbot")
	if err != nil {
		t.Fatalf("lookupPlatformDescriptor error: %v", err)
	}
	if descriptor.Metadata.Source != "qqbot" {
		t.Fatalf("source = %q, want qqbot", descriptor.Metadata.Source)
	}
	if descriptor.NewStub == nil || descriptor.NewLive == nil {
		t.Fatalf("expected qqbot to be runnable, got NewStub=%v NewLive=%v", descriptor.NewStub, descriptor.NewLive)
	}
	if !descriptor.Metadata.Capabilities.RequiresPublicCallback {
		t.Fatal("expected qqbot live transport to require callback exposure")
	}
}

func TestSelectProvider_ReturnsDescriptorBackedRuntime(t *testing.T) {
	provider, err := selectProvider(&config{
		Platform:      "slack",
		TransportMode: "stub",
		TestPort:      "9010",
	})
	if err != nil {
		t.Fatalf("selectProvider error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected active provider")
	}
	if provider.Descriptor.ID != "slack" {
		t.Fatalf("descriptor id = %q, want slack", provider.Descriptor.ID)
	}
	if provider.Source() != "slack" {
		t.Fatalf("source = %q, want slack", provider.Source())
	}
	if provider.TransportMode != transportModeStub {
		t.Fatalf("transport mode = %q, want %q", provider.TransportMode, transportModeStub)
	}
	if provider.Platform == nil || provider.Platform.Name() != "slack-stub" {
		t.Fatalf("platform = %#v", provider.Platform)
	}
}

func TestConfigurePlatformActionCallbacks_WiresSetterPlatforms(t *testing.T) {
	mockPlatform := &actionHandlerAwarePlatform{}
	handler := &noopActionHandler{}

	configurePlatformActionCallbacks(mockPlatform, handler)

	if mockPlatform.handler != handler {
		t.Fatalf("handler = %#v, want %#v", mockPlatform.handler, handler)
	}
}

func TestConfigurePlatformActionCallbacks_IgnoresPlainPlatforms(t *testing.T) {
	configurePlatformActionCallbacks(&plainPlatform{}, &noopActionHandler{})
}

func TestMain_StartsBridgeAndShutsDownGracefully(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows test subprocess cannot reliably deliver os.Interrupt to the helper process")
	}

	notifyPort := getFreePort(t)
	testPort := getFreePort(t)

	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcessMain")
	cmd.Env = append(os.Environ(),
		"GO_WANT_HELPER_PROCESS=1",
		"GO_HELPER_INTERRUPT_DELAY_MS=2000",
		"IM_PLATFORM=feishu",
		"NOTIFY_PORT="+notifyPort,
		"TEST_PORT="+testPort,
	)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper process: %v", err)
	}

	healthResp := waitForHTTP(t, "http://127.0.0.1:"+notifyPort+"/im/health")
	_, _ = io.Copy(io.Discard, healthResp.Body)
	healthResp.Body.Close()

	body := bytes.NewBufferString(`{"content":"@AgentForge hello","chat_id":"chat-1"}`)
	resp, err := http.Post("http://127.0.0.1:"+testPort+"/test/message", "application/json", body)
	if err != nil {
		t.Fatalf("post test message: %v", err)
	}
	resp.Body.Close()

	replyResp := waitForHTTP(t, "http://127.0.0.1:"+testPort+"/test/replies")
	defer replyResp.Body.Close()

	var replies []struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(replyResp.Body).Decode(&replies); err != nil {
		t.Fatalf("decode replies: %v", err)
	}
	if len(replies) == 0 {
		t.Fatalf("expected replies, output=%s", output.String())
	}
	if replies[0].Content == "" {
		t.Fatalf("reply = %+v", replies[0])
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("helper process error: %v\noutput:\n%s", err, output.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("helper process did not exit in time\noutput:\n%s", output.String())
	}
}

func TestHelperProcessMain(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	if rawDelay := os.Getenv("GO_HELPER_INTERRUPT_DELAY_MS"); rawDelay != "" {
		delayMs, err := strconv.Atoi(rawDelay)
		if err != nil {
			t.Fatalf("invalid delay: %v", err)
		}
		go func() {
			time.Sleep(time.Duration(delayMs) * time.Millisecond)
			process, err := os.FindProcess(os.Getpid())
			if err == nil {
				_ = process.Signal(os.Interrupt)
			}
		}()
	}

	main()
	os.Exit(0)
}

func getFreePort(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	return strconv.Itoa(listener.Addr().(*net.TCPAddr).Port)
}

func waitForHTTP(t *testing.T, url string) *http.Response {
	t.Helper()

	var lastErr error
	for i := 0; i < 40; i++ {
		resp, err := http.Get(url)
		if err == nil {
			return resp
		}
		lastErr = err
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("request %s failed: %v", url, lastErr)
	return nil
}

type actionHandlerAwarePlatform struct {
	handler notify.ActionHandler
}

func (p *actionHandlerAwarePlatform) Name() string                            { return "mock-platform" }
func (p *actionHandlerAwarePlatform) Start(handler core.MessageHandler) error { return nil }
func (p *actionHandlerAwarePlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	return nil
}
func (p *actionHandlerAwarePlatform) Send(ctx context.Context, chatID string, content string) error {
	return nil
}
func (p *actionHandlerAwarePlatform) Stop() error { return nil }
func (p *actionHandlerAwarePlatform) SetActionHandler(handler notify.ActionHandler) {
	p.handler = handler
}

type plainPlatform struct{}

func (p *plainPlatform) Name() string                            { return "plain-platform" }
func (p *plainPlatform) Start(handler core.MessageHandler) error { return nil }
func (p *plainPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	return nil
}
func (p *plainPlatform) Send(ctx context.Context, chatID string, content string) error {
	return nil
}
func (p *plainPlatform) Stop() error { return nil }

type noopActionHandler struct{}

func (h *noopActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	return &notify.ActionResponse{}, nil
}

func TestDurationEnvOrDefault_UsesFallbackForInvalidValue(t *testing.T) {
	t.Setenv("IM_BRIDGE_RECONNECT_DELAY", "not-a-duration")
	if got := durationEnvOrDefault("IM_BRIDGE_RECONNECT_DELAY", 5*time.Second); got != 5*time.Second {
		t.Fatalf("durationEnvOrDefault = %v", got)
	}
}

func TestBackendActionRelay_HandleAction_UsesRequestPlatformAndBridgeContext(t *testing.T) {
	var gotBody client.IMActionRequest
	var gotSource string
	var gotBridgeID string
	var gotReplyTarget string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSource = r.Header.Get("X-IM-Source")
		gotBridgeID = r.Header.Get("X-IM-Bridge-ID")
		gotReplyTarget = r.Header.Get("X-IM-Reply-Target")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.IMActionResponse{
			Result:      "Approved",
			ReplyTarget: gotBody.ReplyTarget,
			Metadata:    map[string]string{"source": "block_actions"},
		})
	}))
	defer server.Close()

	relay := &backendActionRelay{
		client:   client.NewAgentForgeClient(server.URL, "proj", "secret"),
		bridgeID: "bridge-default",
	}

	resp, err := relay.HandleAction(context.Background(), &notify.ActionRequest{
		Platform: "slack-stub",
		Action:   "approve",
		EntityID: "review-1",
		ChatID:   "C123",
		UserID:   "U123",
		ReplyTarget: &core.ReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp == nil || resp.Result != "Approved" || resp.Metadata["source"] != "block_actions" {
		t.Fatalf("response = %+v", resp)
	}
	if gotSource != "slack" {
		t.Fatalf("X-IM-Source = %q", gotSource)
	}
	if gotBridgeID != "bridge-default" {
		t.Fatalf("X-IM-Bridge-ID = %q", gotBridgeID)
	}
	if !strings.Contains(gotReplyTarget, "\"threadId\":\"thread-1\"") {
		t.Fatalf("X-IM-Reply-Target = %q", gotReplyTarget)
	}
	if gotBody.Action != "approve" || gotBody.EntityID != "review-1" || gotBody.BridgeID != "bridge-default" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestBackendActionRelay_ForwardsStructuredAndNative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.IMActionResponse{
			Result:  "Agent dispatched",
			Success: true,
			Status:  "started",
			Structured: &core.StructuredMessage{
				Title: "Agent Dispatched",
				Body:  "Run run-1 started for task-1.",
				Fields: []core.StructuredField{
					{Label: "Task", Value: "task-1"},
					{Label: "Run", Value: "run-1"},
				},
			},
			Native: &core.NativeMessage{
				Platform: "feishu",
				FeishuCard: &core.FeishuCardPayload{
					Mode: "json",
					JSON: json.RawMessage(`{"header":{"title":{"tag":"plain_text","content":"Dispatched"}}}`),
				},
			},
			Metadata: map[string]string{"action_status": "started"},
		})
	}))
	defer server.Close()

	relay := &backendActionRelay{
		client:   client.NewAgentForgeClient(server.URL, "proj", "secret"),
		bridgeID: "bridge-default",
	}

	resp, err := relay.HandleAction(context.Background(), &notify.ActionRequest{
		Action:   "assign-agent",
		EntityID: "task-1",
		ChatID:   "C123",
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp.Result != "Agent dispatched" {
		t.Fatalf("Result = %q", resp.Result)
	}
	if resp.Structured == nil || resp.Structured.Title != "Agent Dispatched" {
		t.Fatalf("Structured = %+v", resp.Structured)
	}
	if len(resp.Structured.Fields) != 2 {
		t.Fatalf("Structured.Fields = %d, want 2", len(resp.Structured.Fields))
	}
	if resp.Native == nil || resp.Native.Platform != "feishu" {
		t.Fatalf("Native = %+v", resp.Native)
	}
	if resp.Metadata["action_status"] != "started" {
		t.Fatalf("Metadata = %+v", resp.Metadata)
	}
}

func TestBackendActionRelay_HandleAction_NilInputsAreSafe(t *testing.T) {
	var relay *backendActionRelay
	resp, err := relay.HandleAction(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp != nil {
		t.Fatalf("response = %+v, want nil", resp)
	}
}
