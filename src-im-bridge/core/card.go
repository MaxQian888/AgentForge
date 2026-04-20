package core

// Card represents a rich message card (e.g. Feishu interactive card).
type Card struct {
	Title   string
	Fields  []LegacyCardField
	Buttons []CardButton
}

// LegacyCardField is a label-value pair displayed in the legacy Card body.
// The provider-neutral schema (spec §8) uses CardField for the new shape.
type LegacyCardField struct {
	Label string
	Value string
}

// CardButton is an interactive button in the card footer.
type CardButton struct {
	Text   string
	Action string // e.g., "act:assign-agent:task-id" or "link:https://..."
	Style  string // "primary", "danger", "default"
}

func NewCard() *Card { return &Card{} }

func (c *Card) SetTitle(title string) *Card {
	c.Title = title
	return c
}

func (c *Card) AddField(label, value string) *Card {
	c.Fields = append(c.Fields, LegacyCardField{Label: label, Value: value})
	return c
}

func (c *Card) AddButton(text, action string) *Card {
	c.Buttons = append(c.Buttons, CardButton{Text: text, Action: action, Style: "default"})
	return c
}

func (c *Card) AddPrimaryButton(text, action string) *Card {
	c.Buttons = append(c.Buttons, CardButton{Text: text, Action: action, Style: "primary"})
	return c
}

func (c *Card) AddDangerButton(text, action string) *Card {
	c.Buttons = append(c.Buttons, CardButton{Text: text, Action: action, Style: "danger"})
	return c
}
