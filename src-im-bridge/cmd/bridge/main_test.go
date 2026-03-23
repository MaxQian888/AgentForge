package main

import "testing"

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
