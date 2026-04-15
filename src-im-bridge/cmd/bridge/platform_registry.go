package main

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/platform/dingtalk"
	"github.com/agentforge/im-bridge/platform/discord"
	"github.com/agentforge/im-bridge/platform/email"
	"github.com/agentforge/im-bridge/platform/feishu"
	"github.com/agentforge/im-bridge/platform/qq"
	"github.com/agentforge/im-bridge/platform/qqbot"
	"github.com/agentforge/im-bridge/platform/slack"
	"github.com/agentforge/im-bridge/platform/telegram"
	"github.com/agentforge/im-bridge/platform/wechat"
	"github.com/agentforge/im-bridge/platform/wecom"
)

const (
	transportModeStub = "stub"
	transportModeLive = "live"
)

type platformFactory func(cfg *config) (core.Platform, error)

func normalizeTransportMode(mode string) string {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return transportModeStub
	}
	return normalized
}

func providerDescriptors() map[string]providerDescriptor {
	return map[string]providerDescriptor{
		"email": {
			ID:                      "email",
			Metadata:                email.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				switch {
				case strings.TrimSpace(cfg.EmailSMTPHost) == "":
					return fmt.Errorf("selected platform email requires EMAIL_SMTP_HOST for live transport")
				case strings.TrimSpace(cfg.EmailFromAddress) == "":
					return fmt.Errorf("selected platform email requires EMAIL_FROM_ADDRESS for live transport")
				default:
					return nil
				}
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return email.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				opts := make([]email.LiveOption, 0, 1)
				if strings.EqualFold(strings.TrimSpace(cfg.EmailSMTPTLS), "false") {
					opts = append(opts, email.WithTLS(false))
				}
				return email.NewLive(
					cfg.EmailSMTPHost,
					cfg.EmailSMTPPort,
					cfg.EmailSMTPUser,
					cfg.EmailSMTPPass,
					cfg.EmailFromAddress,
					opts...,
				)
			},
		},
		"wecom": {
			ID:                      "wecom",
			Metadata:                wecom.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				switch {
				case strings.TrimSpace(cfg.WeComCorpID) == "":
					return fmt.Errorf("selected platform wecom requires WECOM_CORP_ID for live transport")
				case strings.TrimSpace(cfg.WeComAgentID) == "":
					return fmt.Errorf("selected platform wecom requires WECOM_AGENT_ID for live transport")
				case strings.TrimSpace(cfg.WeComAgentSecret) == "":
					return fmt.Errorf("selected platform wecom requires WECOM_AGENT_SECRET for live transport")
				case strings.TrimSpace(cfg.WeComCallbackToken) == "":
					return fmt.Errorf("selected platform wecom requires WECOM_CALLBACK_TOKEN for live transport")
				case strings.TrimSpace(cfg.WeComCallbackPort) == "":
					return fmt.Errorf("selected platform wecom requires WECOM_CALLBACK_PORT for live transport")
				default:
					return nil
				}
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return wecom.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return wecom.NewLive(
					cfg.WeComCorpID,
					cfg.WeComAgentID,
					cfg.WeComAgentSecret,
					cfg.WeComCallbackToken,
					cfg.WeComCallbackPort,
					cfg.WeComCallbackPath,
				)
			},
		},
		"wechat": {
			ID:                      "wechat",
			Metadata:                wechat.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				switch {
				case strings.TrimSpace(cfg.WeChatAppID) == "":
					return fmt.Errorf("selected platform wechat requires WECHAT_APP_ID for live transport")
				case strings.TrimSpace(cfg.WeChatAppSecret) == "":
					return fmt.Errorf("selected platform wechat requires WECHAT_APP_SECRET for live transport")
				default:
					return nil
				}
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return wechat.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				opts := make([]wechat.LiveOption, 0, 2)
				if strings.TrimSpace(cfg.WeChatCallbackPort) != "" {
					opts = append(opts, wechat.WithCallbackPort(cfg.WeChatCallbackPort))
				}
				if strings.TrimSpace(cfg.WeChatCallbackPath) != "" {
					opts = append(opts, wechat.WithCallbackPath(cfg.WeChatCallbackPath))
				}
				return wechat.NewLive(cfg.WeChatAppID, cfg.WeChatAppSecret, cfg.WeChatCallbackToken, opts...)
			},
		},
		"qq": {
			ID:                      "qq",
			Metadata:                qq.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				if strings.TrimSpace(cfg.QQOneBotWSURL) == "" {
					return fmt.Errorf("selected platform qq requires QQ_ONEBOT_WS_URL for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return qq.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return qq.NewLive(cfg.QQOneBotWSURL, cfg.QQAccessToken)
			},
		},
		"qqbot": {
			ID:                      "qqbot",
			Metadata:                qqbot.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				switch {
				case strings.TrimSpace(cfg.QQBotAppID) == "":
					return fmt.Errorf("selected platform qqbot requires QQBOT_APP_ID for live transport")
				case strings.TrimSpace(cfg.QQBotAppSecret) == "":
					return fmt.Errorf("selected platform qqbot requires QQBOT_APP_SECRET for live transport")
				case strings.TrimSpace(cfg.QQBotCallbackPort) == "":
					return fmt.Errorf("selected platform qqbot requires QQBOT_CALLBACK_PORT for live transport")
				default:
					return nil
				}
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return qqbot.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return qqbot.NewLive(
					cfg.QQBotAppID,
					cfg.QQBotAppSecret,
					cfg.QQBotCallbackPort,
					cfg.QQBotCallbackPath,
					qqbot.WithAPIBase(cfg.QQBotAPIBase),
					qqbot.WithTokenBase(cfg.QQBotTokenBase),
				)
			},
		},
		"feishu": {
			ID:                      "feishu",
			Metadata:                feishu.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			Features: providerFeatureSet{
				FeishuCards: &feishuProviderFeatures{
					SupportsJSONCards:      true,
					SupportsTemplateCards:  true,
					SupportsDelayedUpdates: true,
				},
			},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode == transportModeLive && (cfg.FeishuApp == "" || cfg.FeishuSec == "") {
					return fmt.Errorf("selected platform feishu requires FEISHU_APP_ID and FEISHU_APP_SECRET for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return feishu.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				opts := make([]feishu.LiveOption, 0, 1)
				if strings.TrimSpace(cfg.FeishuVerificationToken) != "" {
					opts = append(opts, feishu.WithCardCallbackWebhook(
						cfg.FeishuVerificationToken,
						cfg.FeishuEventEncryptKey,
						cfg.FeishuCallbackPath,
					))
				}
				return feishu.NewLive(cfg.FeishuApp, cfg.FeishuSec, opts...)
			},
		},
		"slack": {
			ID:                      "slack",
			Metadata:                slack.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode == transportModeLive && (cfg.SlackBotToken == "" || cfg.SlackAppToken == "") {
					return fmt.Errorf("selected platform slack requires SLACK_BOT_TOKEN and SLACK_APP_TOKEN for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return slack.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return slack.NewLive(cfg.SlackBotToken, cfg.SlackAppToken)
			},
		},
		"dingtalk": {
			ID:                      "dingtalk",
			Metadata:                dingtalk.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode == transportModeLive && (cfg.DingTalkAppKey == "" || cfg.DingTalkAppSecret == "") {
					return fmt.Errorf("selected platform dingtalk requires DINGTALK_APP_KEY and DINGTALK_APP_SECRET for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return dingtalk.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				opts := make([]dingtalk.LiveOption, 0, 1)
				if strings.TrimSpace(cfg.DingTalkCardTemplateID) != "" {
					opts = append(opts, dingtalk.WithAdvancedCardTemplate(cfg.DingTalkCardTemplateID))
				}
				return dingtalk.NewLive(cfg.DingTalkAppKey, cfg.DingTalkAppSecret, opts...)
			},
		},
		"telegram": {
			ID:                      "telegram",
			Metadata:                telegram.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				if strings.TrimSpace(cfg.TelegramBotToken) == "" {
					return fmt.Errorf("selected platform telegram requires TELEGRAM_BOT_TOKEN for live transport")
				}
				return telegramValidateConfig(cfg.TelegramUpdateMode, cfg.TelegramWebhookURL)
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return telegram.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				return telegram.NewLive(cfg.TelegramBotToken)
			},
		},
		"discord": {
			ID:                      "discord",
			Metadata:                discord.NewStub("0").Metadata(),
			SupportedTransportModes: []string{transportModeStub, transportModeLive},
			ValidateConfig: func(cfg *config, mode string) error {
				if mode != transportModeLive {
					return nil
				}
				if strings.TrimSpace(cfg.DiscordAppID) == "" || strings.TrimSpace(cfg.DiscordBotToken) == "" || strings.TrimSpace(cfg.DiscordPublicKey) == "" {
					return fmt.Errorf("selected platform discord requires DISCORD_APP_ID, DISCORD_BOT_TOKEN, and DISCORD_PUBLIC_KEY for live transport")
				}
				if strings.TrimSpace(cfg.DiscordInteractionsPort) == "" {
					return fmt.Errorf("selected platform discord requires DISCORD_INTERACTIONS_PORT for live transport")
				}
				return nil
			},
			NewStub: func(cfg *config) (core.Platform, error) {
				return discord.NewStub(cfg.TestPort), nil
			},
			NewLive: func(cfg *config) (core.Platform, error) {
				opts := make([]discord.LiveOption, 0, 1)
				if strings.TrimSpace(cfg.DiscordCommandGuildID) != "" {
					opts = append(opts, discord.WithCommandGuildID(cfg.DiscordCommandGuildID))
				}
				return discord.NewLive(
					cfg.DiscordAppID,
					cfg.DiscordBotToken,
					cfg.DiscordPublicKey,
					cfg.DiscordInteractionsPort,
					opts...,
				)
			},
		},
	}
}

func platformDescriptors() map[string]platformDescriptor {
	return providerDescriptors()
}

func telegramValidateConfig(updateMode, webhookURL string) error {
	normalized := strings.ToLower(strings.TrimSpace(updateMode))
	if normalized == "" {
		normalized = "longpoll"
	}
	if normalized != "longpoll" {
		return fmt.Errorf("telegram live transport currently supports only longpoll update mode")
	}
	if strings.TrimSpace(webhookURL) != "" {
		return fmt.Errorf("telegram long polling cannot be combined with webhook configuration")
	}
	return nil
}
