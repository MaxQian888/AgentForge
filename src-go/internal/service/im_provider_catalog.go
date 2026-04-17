package service

import (
	"context"

	"github.com/react-go-quick-starter/server/internal/model"
)

var canonicalIMProviderCatalog = []model.IMProviderCatalogEntry{
	{
		ID:                    "feishu",
		Label:                 "Feishu",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
	},
	{
		ID:                    "dingtalk",
		Label:                 "DingTalk",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
	},
	{
		ID:                    "slack",
		Label:                 "Slack",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
	},
	{
		ID:                    "telegram",
		Label:                 "Telegram",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
	},
	{
		ID:                    "discord",
		Label:                 "Discord",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
	},
	{
		ID:                    "wecom",
		Label:                 "WeCom",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
		ConfigFields: []model.IMProviderConfigField{
			{Key: "corpId", Label: "Corp ID", Placeholder: "ww1234567890"},
			{Key: "agentId", Label: "Agent ID", Placeholder: "1000002"},
			{Key: "callbackToken", Label: "Callback Token", Placeholder: "wecom-callback-token", Type: "password"},
		},
	},
	{
		ID:                    "qq",
		Label:                 "QQ",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
		ConfigFields: []model.IMProviderConfigField{
			{Key: "onebotEndpoint", Label: "OneBot Endpoint", Placeholder: "ws://localhost:6700", Type: "url"},
			{Key: "accessToken", Label: "Access Token", Placeholder: "onebot-access-token", Type: "password"},
		},
	},
	{
		ID:                    "qqbot",
		Label:                 "QQ Bot",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
		ConfigFields: []model.IMProviderConfigField{
			{Key: "appId", Label: "App ID", Placeholder: "1024"},
			{Key: "appSecret", Label: "App Secret", Placeholder: "qqbot-app-secret", Type: "password"},
		},
	},
	{
		ID:                    "wechat",
		Label:                 "WeChat",
		InteractionClass:      model.IMProviderInteractionClassInteractive,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
		ConfigFields: []model.IMProviderConfigField{
			{Key: "appId", Label: "App ID", Placeholder: "wx1234567890"},
			{Key: "appSecret", Label: "App Secret", Placeholder: "wechat-app-secret", Type: "password"},
			{Key: "callbackToken", Label: "Callback Token", Placeholder: "wechat-callback-token", Type: "password"},
		},
	},
	{
		ID:                    "email",
		Label:                 "Email",
		InteractionClass:      model.IMProviderInteractionClassDeliveryOnly,
		SupportsChannelConfig: true,
		SupportsTestSend:      true,
		ConfigFields: []model.IMProviderConfigField{
			{Key: "smtpHost", Label: "SMTP Host", Placeholder: "smtp.example.com"},
			{Key: "smtpPort", Label: "SMTP Port", Placeholder: "587"},
			{Key: "fromAddress", Label: "From Address", Placeholder: "noreply@example.com"},
			{Key: "smtpTls", Label: "TLS", Placeholder: "true"},
		},
	},
}

func CanonicalIMProviderCatalog() []model.IMProviderCatalogEntry {
	result := make([]model.IMProviderCatalogEntry, 0, len(canonicalIMProviderCatalog))
	for _, entry := range canonicalIMProviderCatalog {
		cloned := entry
		if len(entry.ConfigFields) > 0 {
			cloned.ConfigFields = append([]model.IMProviderConfigField(nil), entry.ConfigFields...)
		}
		result = append(result, cloned)
	}
	return result
}

func (s *IMControlPlane) ListProviderCatalog(_ context.Context) ([]model.IMProviderCatalogEntry, error) {
	return CanonicalIMProviderCatalog(), nil
}
