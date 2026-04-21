package main

import (
	"testing"
	"time"
)

func TestCfgProviderEnv_NamespaceHit(t *testing.T) {
	cfg := &config{FeishuApp: "cli_abc", FeishuSec: "sec_xyz", TestPort: "7780"}
	env := newCfgProviderEnv(cfg, []string{"FEISHU_"})
	if got := env.Get("FEISHU_APP_ID"); got != "cli_abc" {
		t.Errorf("Get FEISHU_APP_ID = %q, want cli_abc", got)
	}
	if got := env.Get("FEISHU_APP_SECRET"); got != "sec_xyz" {
		t.Errorf("Get FEISHU_APP_SECRET = %q, want sec_xyz", got)
	}
	if got := env.TestPort(); got != "7780" {
		t.Errorf("TestPort = %q, want 7780", got)
	}
}

func TestCfgProviderEnv_CrossNamespaceReturnsEmpty(t *testing.T) {
	cfg := &config{FeishuApp: "cli_abc", SlackBotToken: "xoxb-xxx"}
	env := newCfgProviderEnv(cfg, []string{"FEISHU_"})
	if got := env.Get("SLACK_BOT_TOKEN"); got != "" {
		t.Errorf("cross-namespace Get SLACK_BOT_TOKEN = %q, want empty", got)
	}
}

func TestCfgProviderEnv_BoolOr(t *testing.T) {
	cfg := &config{EmailSMTPTLS: "false"}
	env := newCfgProviderEnv(cfg, []string{"EMAIL_"})
	if got := env.BoolOr("EMAIL_SMTP_TLS", true); got != false {
		t.Errorf("BoolOr EMAIL_SMTP_TLS = %v, want false", got)
	}
	if got := env.BoolOr("EMAIL_SMTP_UNKNOWN", true); got != true {
		t.Errorf("BoolOr EMAIL_SMTP_UNKNOWN fallback = %v, want true", got)
	}
}

func TestCfgProviderEnv_DurationOr(t *testing.T) {
	cfg := &config{HeartbeatInterval: 45 * time.Second}
	env := newCfgProviderEnv(cfg, []string{"IM_"})
	if got := env.DurationOr("IM_BRIDGE_HEARTBEAT_INTERVAL", 30*time.Second); got != 45*time.Second {
		t.Errorf("DurationOr IM_BRIDGE_HEARTBEAT_INTERVAL = %v, want 45s", got)
	}
	if got := env.DurationOr("IM_UNKNOWN", 10*time.Second); got != 10*time.Second {
		t.Errorf("DurationOr IM_UNKNOWN fallback = %v, want 10s", got)
	}
}
