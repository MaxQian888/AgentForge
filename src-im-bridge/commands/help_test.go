package commands

import (
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestHelpCommand_RepliesWithHelpText(t *testing.T) {
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterHelpCommand(engine)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/help",
	})

	if len(platform.replies) != 1 || platform.replies[0] != helpText {
		t.Fatalf("replies = %v", platform.replies)
	}
}
