package main

import (
	"bytes"
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
)

func TestLoadConfig_ReadsExplicitPlatformSettings(t *testing.T) {
	t.Setenv("IM_PLATFORM", "slack")
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
	if cfg.SlackBotToken != "xoxb-test" {
		t.Fatalf("SlackBotToken = %q, want xoxb-test", cfg.SlackBotToken)
	}
	if cfg.SlackAppToken != "xapp-test" {
		t.Fatalf("SlackAppToken = %q, want xapp-test", cfg.SlackAppToken)
	}
}

func TestSelectPlatform_RejectsMissingCredentials(t *testing.T) {
	cfg := &config{
		Platform: "dingtalk",
		TestPort: "9010",
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
		Platform: "feishu",
		TestPort: "9010",
	}

	platform, err := selectPlatform(cfg)
	if err != nil {
		t.Fatalf("selectPlatform error: %v", err)
	}
	if platform.Name() != "feishu-stub" {
		t.Fatalf("platform name = %q", platform.Name())
	}
}

func TestSelectPlatform_ReturnsStubForSlackWithCredentials(t *testing.T) {
	cfg := &config{
		Platform:      "slack",
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

func TestSelectPlatform_ReturnsStubForDingTalkWithCredentials(t *testing.T) {
	cfg := &config{
		Platform:          "dingtalk",
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

func TestSelectPlatform_RejectsUnsupportedPlatform(t *testing.T) {
	cfg := &config{
		Platform: "discord",
		TestPort: "9010",
	}

	_, err := selectPlatform(cfg)
	if err == nil {
		t.Fatal("expected unsupported platform to fail")
	}
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
