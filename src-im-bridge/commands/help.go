package commands

import (
	"context"

	"github.com/agentforge/im-bridge/core"
)

var helpText = buildHelpText()

// RegisterHelpCommand registers the /help command on the engine.
func RegisterHelpCommand(engine *core.Engine) {
	engine.RegisterCommand("/help", func(p core.Platform, msg *core.Message, args string) {
		_ = p.Reply(context.Background(), msg.ReplyCtx, helpText)
	})
}
