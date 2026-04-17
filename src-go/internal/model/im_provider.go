package model

type IMProviderInteractionClass string

const (
	IMProviderInteractionClassInteractive  IMProviderInteractionClass = "interactive"
	IMProviderInteractionClassDeliveryOnly IMProviderInteractionClass = "delivery-only"
)

type IMProviderConfigField struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Placeholder string `json:"placeholder,omitempty"`
	Type        string `json:"type,omitempty"`
}

type IMProviderCatalogEntry struct {
	ID                    string                     `json:"id"`
	Label                 string                     `json:"label"`
	InteractionClass      IMProviderInteractionClass `json:"interactionClass"`
	SupportsChannelConfig bool                       `json:"supportsChannelConfig"`
	SupportsTestSend      bool                       `json:"supportsTestSend"`
	ConfigFields          []IMProviderConfigField    `json:"configFields,omitempty"`
}
