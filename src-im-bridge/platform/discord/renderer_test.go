package discord

import (
	"github.com/agentforge/im-bridge/core"
	"testing"
)

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
