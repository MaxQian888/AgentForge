package commands

import (
	"context"

	"github.com/agentforge/im-bridge/core"
)

const helpText = `AgentForge IM 助手

可用命令:
  /task create <标题>      — 创建新任务
  /task list [状态]        — 查看任务列表
  /task status <task-id>   — 查看任务详情
  /task assign <id> <人员> — 分配任务

  /agent list              — 查看 Agent 池状态
  /agent spawn <task-id>   — 为任务启动 Agent

  /cost                    — 查看费用统计

  /help                    — 显示此帮助

或者直接 @AgentForge <你的需求> 使用自然语言`

// RegisterHelpCommand registers the /help command on the engine.
func RegisterHelpCommand(engine *core.Engine) {
	engine.RegisterCommand("/help", func(p core.Platform, msg *core.Message, args string) {
		_ = p.Reply(context.Background(), msg.ReplyCtx, helpText)
	})
}
