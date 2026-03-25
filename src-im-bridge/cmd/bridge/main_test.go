package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	"github.com/agentforge/im-bridge/platform/dingtalk"
	"github.com/agentforge/im-bridge/platform/discord"
	"github.com/agentforge/im-bridge/platform/feishu"
	"github.com/agentforge/im-bridge/platform/slack"
	"github.com/agentforge/im-bridge/platform/telegram"
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

func TestSelectPlatform_RejectsPlannedWecomRuntimeActivation(t *testing.T) {
	cfg := &config{
		Platform:      "wecom",
		TransportMode: "stub",
		TestPort:      "9010",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected wecom selection to fail")
	}
	if err.Error() != "selected platform wecom is planned but not yet runnable; adapter, capability matrix, and runtime wiring are still pending" {
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

func TestLookupPlatformDescriptor_ReportsPlannedWecomGap(t *testing.T) {
	descriptor, err := lookupPlatformDescriptor("wecom")
	if err != nil {
		t.Fatalf("lookupPlatformDescriptor error: %v", err)
	}
	if descriptor.Metadata.Source != "wecom" {
		t.Fatalf("source = %q, want wecom", descriptor.Metadata.Source)
	}
	if descriptor.NewStub != nil || descriptor.NewLive != nil {
		t.Fatalf("expected wecom to remain non-runnable, got NewStub=%v NewLive=%v", descriptor.NewStub, descriptor.NewLive)
	}
	if descriptor.PlannedReason == "" {
		t.Fatal("expected planned provider to include a planned reason")
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
