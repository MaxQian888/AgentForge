package discord

import (
	"github.com/agentforge/im-bridge/core"
	"testing"
)

func TestRenderCardToDiscordMessage_ProducesCorrectEmbedAndComponents(t *testing.T) {
	card := core.NewCard().
		SetTitle("Build Summary").
		AddField("Status", "success").
		AddField("Duration", "2m 31s").
		AddPrimaryButton("View Logs", "act:view-logs:build-1").
		AddDangerButton("Cancel", "act:cancel:build-1").
		AddButton("Open", "link:https://example.test/builds/1")

	outgoing := renderCardToDiscordMessage(card)

	if outgoing.Content != "Build Summary" {
		t.Fatalf("Content = %q", outgoing.Content)
	}
	if len(outgoing.Embeds) != 1 {
		t.Fatalf("Embeds = %+v", outgoing.Embeds)
	}
	embed := outgoing.Embeds[0]
	if embed.Title != "Build Summary" {
		t.Fatalf("embed.Title = %q", embed.Title)
	}
	if embed.Color != defaultCardEmbedColor {
		t.Fatalf("embed.Color = %d", embed.Color)
	}
	if len(embed.Fields) != 2 {
		t.Fatalf("embed.Fields = %+v", embed.Fields)
	}
	if embed.Fields[0].Name != "Status" || embed.Fields[0].Value != "success" || !embed.Fields[0].Inline {
		t.Fatalf("embed.Fields[0] = %+v", embed.Fields[0])
	}
	if embed.Fields[1].Name != "Duration" || embed.Fields[1].Value != "2m 31s" {
		t.Fatalf("embed.Fields[1] = %+v", embed.Fields[1])
	}

	if len(outgoing.Components) != 1 {
		t.Fatalf("Components = %+v", outgoing.Components)
	}
	row := outgoing.Components[0]
	if row.Type != componentTypeActionRow || len(row.Components) != 3 {
		t.Fatalf("action row = %+v", row)
	}
	if row.Components[0].Label != "View Logs" || row.Components[0].Style != componentStylePrimary || row.Components[0].CustomID != "act:view-logs:build-1" {
		t.Fatalf("button[0] = %+v", row.Components[0])
	}
	if row.Components[1].Label != "Cancel" || row.Components[1].Style != componentStyleDanger || row.Components[1].CustomID != "act:cancel:build-1" {
		t.Fatalf("button[1] = %+v", row.Components[1])
	}
	if row.Components[2].Label != "Open" || row.Components[2].Style != componentStyleLink || row.Components[2].URL != "https://example.test/builds/1" {
		t.Fatalf("button[2] = %+v", row.Components[2])
	}
}

func TestRenderCardToDiscordMessage_NilCardReturnsEmptyMessage(t *testing.T) {
	outgoing := renderCardToDiscordMessage(nil)
	if outgoing.Content != "" || len(outgoing.Embeds) != 0 || len(outgoing.Components) != 0 {
		t.Fatalf("outgoing = %+v", outgoing)
	}
}

func TestRenderCardToDiscordMessage_EmptyFieldsSkipped(t *testing.T) {
	card := core.NewCard().
		SetTitle("Test").
		AddField("", "").
		AddField("Valid", "value")

	outgoing := renderCardToDiscordMessage(card)
	if len(outgoing.Embeds) != 1 || len(outgoing.Embeds[0].Fields) != 1 {
		t.Fatalf("embed fields = %+v", outgoing.Embeds[0].Fields)
	}
	if outgoing.Embeds[0].Fields[0].Name != "Valid" {
		t.Fatalf("field = %+v", outgoing.Embeds[0].Fields[0])
	}
}

func TestRenderCardToDiscordMessage_ButtonWithoutActionGetsFallbackID(t *testing.T) {
	card := core.NewCard().
		SetTitle("Test").
		AddButton("Click", "")

	outgoing := renderCardToDiscordMessage(card)
	if len(outgoing.Components) != 1 || len(outgoing.Components[0].Components) != 1 {
		t.Fatalf("components = %+v", outgoing.Components)
	}
	if outgoing.Components[0].Components[0].CustomID != "card-btn-0" {
		t.Fatalf("button custom id = %q", outgoing.Components[0].Components[0].CustomID)
	}
}

func TestDiscordRendererHelpers_DefaultStylesAndFallbackIDs(t *testing.T) {
	rows := renderDiscordActionRows([]core.DiscordActionRow{
		{
			Buttons: []core.DiscordButton{
				{Label: "Approve", Style: string(core.ActionStylePrimary)},
				{Label: "Reject", Style: string(core.ActionStyleDanger), CustomID: "act:reject:review-1"},
				{Label: "Open", URL: "https://example.test/reviews/1"},
			},
		},
	})
	if len(rows) != 1 || len(rows[0].Components) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Components[0].CustomID == "" || rows[0].Components[0].Style != componentStylePrimary {
		t.Fatalf("first component = %+v", rows[0].Components[0])
	}
	if rows[0].Components[1].CustomID != "act:reject:review-1" || rows[0].Components[1].Style != componentStyleDanger {
		t.Fatalf("second component = %+v", rows[0].Components[1])
	}
	if rows[0].Components[2].URL != "https://example.test/reviews/1" || rows[0].Components[2].Style != componentStyleLink {
		t.Fatalf("third component = %+v", rows[0].Components[2])
	}

	if got := discordButtonStyle(""); got != componentStyleSecondary {
		t.Fatalf("discordButtonStyle(empty) = %d", got)
	}
}
