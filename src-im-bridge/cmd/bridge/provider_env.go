package main

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/agentforge/im-bridge/core"
)

// cfgProviderEnv is a ProviderEnv backed by the parsed config struct.
// Env keys that begin with one of prefixes are resolved from config
// fields (preferred, already parsed) or fallback to os.Getenv. Keys
// that do not match any prefix return the zero value.
type cfgProviderEnv struct {
	cfg      *config
	prefixes []string
}

func newCfgProviderEnv(cfg *config, prefixes []string) *cfgProviderEnv {
	return &cfgProviderEnv{cfg: cfg, prefixes: prefixes}
}

var _ core.ProviderEnv = (*cfgProviderEnv)(nil)

func (e *cfgProviderEnv) inNamespace(key string) bool {
	upper := strings.ToUpper(strings.TrimSpace(key))
	for _, p := range e.prefixes {
		if strings.HasPrefix(upper, strings.ToUpper(p)) {
			return true
		}
	}
	return false
}

func (e *cfgProviderEnv) Get(key string) string {
	if !e.inNamespace(key) {
		return ""
	}
	if v := lookupCfgField(e.cfg, key); v != "" {
		return v
	}
	return os.Getenv(key)
}

func (e *cfgProviderEnv) BoolOr(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(e.Get(key)))
	switch raw {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func (e *cfgProviderEnv) DurationOr(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(e.Get(key))
	if raw == "" {
		return fallback
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return time.Duration(n) * time.Second
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d
	}
	return fallback
}

func (e *cfgProviderEnv) TestPort() string { return e.cfg.TestPort }

// lookupCfgField maps env-var names to already-parsed config fields. This
// avoids re-reading os.Getenv for values loadConfig has already parsed,
// and crucially keeps the field-to-env mapping in one place for review.
func lookupCfgField(cfg *config, key string) string {
	upper := strings.ToUpper(strings.TrimSpace(key))
	switch upper {
	// --- Feishu ---
	case "FEISHU_APP_ID":
		return cfg.FeishuApp
	case "FEISHU_APP_SECRET":
		return cfg.FeishuSec
	case "FEISHU_VERIFICATION_TOKEN":
		return cfg.FeishuVerificationToken
	case "FEISHU_EVENT_ENCRYPT_KEY":
		return cfg.FeishuEventEncryptKey
	case "FEISHU_CALLBACK_PATH":
		return cfg.FeishuCallbackPath
	// --- Slack ---
	case "SLACK_BOT_TOKEN":
		return cfg.SlackBotToken
	case "SLACK_APP_TOKEN":
		return cfg.SlackAppToken
	// --- DingTalk ---
	case "DINGTALK_APP_KEY":
		return cfg.DingTalkAppKey
	case "DINGTALK_APP_SECRET":
		return cfg.DingTalkAppSecret
	case "DINGTALK_CARD_TEMPLATE_ID":
		return cfg.DingTalkCardTemplateID
	// --- WeCom ---
	case "WECOM_CORP_ID":
		return cfg.WeComCorpID
	case "WECOM_AGENT_ID":
		return cfg.WeComAgentID
	case "WECOM_AGENT_SECRET":
		return cfg.WeComAgentSecret
	case "WECOM_CALLBACK_TOKEN":
		return cfg.WeComCallbackToken
	case "WECOM_CALLBACK_PORT":
		return cfg.WeComCallbackPort
	case "WECOM_CALLBACK_PATH":
		return cfg.WeComCallbackPath
	// --- WeChat ---
	case "WECHAT_APP_ID":
		return cfg.WeChatAppID
	case "WECHAT_APP_SECRET":
		return cfg.WeChatAppSecret
	case "WECHAT_CALLBACK_TOKEN":
		return cfg.WeChatCallbackToken
	case "WECHAT_CALLBACK_PORT":
		return cfg.WeChatCallbackPort
	case "WECHAT_CALLBACK_PATH":
		return cfg.WeChatCallbackPath
	// --- QQ (OneBot) ---
	case "QQ_ONEBOT_WS_URL":
		return cfg.QQOneBotWSURL
	case "QQ_ACCESS_TOKEN":
		return cfg.QQAccessToken
	// --- QQ Bot ---
	case "QQBOT_APP_ID":
		return cfg.QQBotAppID
	case "QQBOT_APP_SECRET":
		return cfg.QQBotAppSecret
	case "QQBOT_CALLBACK_PORT":
		return cfg.QQBotCallbackPort
	case "QQBOT_CALLBACK_PATH":
		return cfg.QQBotCallbackPath
	case "QQBOT_API_BASE":
		return cfg.QQBotAPIBase
	case "QQBOT_TOKEN_BASE":
		return cfg.QQBotTokenBase
	// --- Telegram ---
	case "TELEGRAM_BOT_TOKEN":
		return cfg.TelegramBotToken
	case "TELEGRAM_UPDATE_MODE":
		return cfg.TelegramUpdateMode
	case "TELEGRAM_WEBHOOK_URL":
		return cfg.TelegramWebhookURL
	// --- Discord ---
	case "DISCORD_APP_ID":
		return cfg.DiscordAppID
	case "DISCORD_BOT_TOKEN":
		return cfg.DiscordBotToken
	case "DISCORD_PUBLIC_KEY":
		return cfg.DiscordPublicKey
	case "DISCORD_INTERACTIONS_PORT":
		return cfg.DiscordInteractionsPort
	case "DISCORD_COMMAND_GUILD_ID":
		return cfg.DiscordCommandGuildID
	// --- Email ---
	case "EMAIL_SMTP_HOST":
		return cfg.EmailSMTPHost
	case "EMAIL_SMTP_PORT":
		return cfg.EmailSMTPPort
	case "EMAIL_SMTP_USER":
		return cfg.EmailSMTPUser
	case "EMAIL_SMTP_PASS":
		return cfg.EmailSMTPPass
	case "EMAIL_FROM_ADDRESS":
		return cfg.EmailFromAddress
	case "EMAIL_SMTP_TLS":
		return cfg.EmailSMTPTLS
	case "EMAIL_IMAP_HOST":
		return cfg.EmailIMAPHost
	case "EMAIL_IMAP_PORT":
		return cfg.EmailIMAPPort
	case "EMAIL_IMAP_USER":
		return cfg.EmailIMAPUser
	case "EMAIL_IMAP_PASS":
		return cfg.EmailIMAPPass
	// --- Shared tunables (reachable via IM_ prefix) ---
	case "IM_BRIDGE_HEARTBEAT_INTERVAL":
		return cfg.HeartbeatInterval.String()
	}
	return ""
}
