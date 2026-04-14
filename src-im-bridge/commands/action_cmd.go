package commands

import (
	"context"
	"strings"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

// CommandActionHandler wraps an existing ActionHandler to intercept "cmd"
// actions (e.g., "act:cmd:/task list") and execute them as slash commands
// through the engine, routing all other actions to the wrapped handler.
type CommandActionHandler struct {
	Engine  *core.Engine
	Inner   notify.ActionHandler
	GetPlatform func() core.Platform
}

// HandleAction processes an action request. If the action is "cmd", it
// executes the entity ID as a slash command. Otherwise, it delegates to
// the inner handler.
func (h *CommandActionHandler) HandleAction(ctx context.Context, req *notify.ActionRequest) (*notify.ActionResponse, error) {
	if req != nil && strings.TrimSpace(req.Action) == "cmd" {
		command := strings.TrimSpace(req.EntityID)
		if command != "" && strings.HasPrefix(command, "/") && h.Engine != nil {
			platform := h.getPlatform()
			if platform != nil {
				msg := &core.Message{
					Platform: req.Platform,
					UserID:   req.UserID,
					ChatID:   req.ChatID,
					Content:  command,
					ReplyTarget: req.ReplyTarget,
				}
				if req.ReplyTarget != nil {
					msg.ReplyCtx = req.ReplyTarget
				}
				if h.Engine.ExecuteCommand(platform, msg, command) {
					return &notify.ActionResponse{
						Result: "",
					}, nil
				}
			}
		}
	}
	if h.Inner != nil {
		return h.Inner.HandleAction(ctx, req)
	}
	return nil, nil
}

func (h *CommandActionHandler) getPlatform() core.Platform {
	if h.GetPlatform != nil {
		return h.GetPlatform()
	}
	return nil
}
