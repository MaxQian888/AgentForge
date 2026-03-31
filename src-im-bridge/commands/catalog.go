package commands

import (
	"fmt"
	"regexp"
	"strings"
)

type commandCatalogEntry struct {
	Command     string
	Summary     string
	Usage       string
	UsageText   string
	Aliases     []string
	Subcommands []commandCatalogSubcommand
}

type commandCatalogSubcommand struct {
	Name    string
	Summary string
	Usage   string
	Aliases []string
}

var operatorCommandCatalog = []commandCatalogEntry{
	{
		Command:   "/task",
		Summary:   "任务管理",
		Usage:     "/task create|list|status|assign|decompose|ai|move",
		UsageText: "用法: /task create|list|status|assign|decompose|ai|move <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "create", Usage: "/task create <标题>", Summary: "创建新任务"},
			{Name: "list", Usage: "/task list [状态]", Summary: "查看任务列表"},
			{Name: "status", Usage: "/task status <task-id>", Summary: "查看任务详情"},
			{Name: "assign", Usage: "/task assign <id> <人员>", Summary: "分配任务"},
			{Name: "decompose", Usage: "/task decompose <id>", Summary: "AI 分解现有任务"},
			{Name: "ai", Usage: "/task ai generate|classify <参数>", Summary: "Bridge AI 生成与意图分类"},
			{Name: "move", Usage: "/task move <task-id> <status>", Summary: "流转任务状态", Aliases: []string{"transition"}},
		},
	},
	{
		Command:   "/agent",
		Summary:   "Agent 运行控制",
		Usage:     "/agent status|runtimes|health|spawn|run|logs|pause|resume|kill",
		UsageText: "用法: /agent status|runtimes|health|spawn|run|logs|pause|resume|kill <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "status", Usage: "/agent status [run-id]", Summary: "查看 Agent 池或运行状态", Aliases: []string{"list"}},
			{Name: "runtimes", Usage: "/agent runtimes", Summary: "查看 Bridge runtimes"},
			{Name: "health", Usage: "/agent health", Summary: "查看 Bridge health"},
			{Name: "spawn", Usage: "/agent spawn <task-id>", Summary: "为任务启动 Agent"},
			{Name: "run", Usage: "/agent run <描述>", Summary: "创建任务并自动启动 Agent"},
			{Name: "logs", Usage: "/agent logs <run-id>", Summary: "查看 Agent 执行日志"},
			{Name: "pause", Usage: "/agent pause <run-id>", Summary: "暂停 Agent 运行"},
			{Name: "resume", Usage: "/agent resume <run-id>", Summary: "恢复 Agent 运行"},
			{Name: "kill", Usage: "/agent kill <run-id>", Summary: "终止 Agent 运行"},
		},
	},
	{
		Command:   "/review",
		Summary:   "代码审查",
		Usage:     "/review <pr-url>|status|deep|approve|request-changes",
		UsageText: "用法: /review <pr-url> | /review status <id> | /review deep <pr-url> | /review approve <id> | /review request-changes <id> [comment]",
		Subcommands: []commandCatalogSubcommand{
			{Name: "<pr-url>", Usage: "/review <pr-url>", Summary: "触发代码审查"},
			{Name: "status", Usage: "/review status <id>", Summary: "查看审查状态"},
			{Name: "deep", Usage: "/review deep <pr-url>", Summary: "触发深度审查"},
			{Name: "approve", Usage: "/review approve <id>", Summary: "批准审查"},
			{Name: "request-changes", Usage: "/review request-changes <id> [comment]", Summary: "请求修改"},
		},
	},
	{
		Command:   "/sprint",
		Summary:   "Sprint 摘要",
		Usage:     "/sprint status|burndown",
		UsageText: "用法: /sprint status 或 /sprint burndown",
		Subcommands: []commandCatalogSubcommand{
			{Name: "status", Usage: "/sprint status", Summary: "查看当前 Sprint"},
			{Name: "burndown", Usage: "/sprint burndown", Summary: "查看燃尽图"},
		},
	},
	{
		Command:   "/queue",
		Summary:   "队列观察",
		Usage:     "/queue list|cancel",
		UsageText: "用法: /queue list|cancel <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "list", Usage: "/queue list [status]", Summary: "查看 Agent 队列"},
			{Name: "cancel", Usage: "/queue cancel <entry-id>", Summary: "取消队列项"},
		},
	},
	{
		Command:   "/team",
		Summary:   "团队摘要",
		Usage:     "/team list",
		UsageText: "用法: /team list",
		Subcommands: []commandCatalogSubcommand{
			{Name: "list", Usage: "/team list", Summary: "查看项目成员摘要"},
		},
	},
	{
		Command:   "/memory",
		Summary:   "项目记忆",
		Usage:     "/memory search|note",
		UsageText: "用法: /memory search|note <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "search", Usage: "/memory search <query>", Summary: "搜索项目记忆"},
			{Name: "note", Usage: "/memory note <content>", Summary: "记录项目记忆"},
		},
	},
	{
		Command:   "/tools",
		Summary:   "Bridge 宸ュ叿绠＄悊",
		Usage:     "/tools list|install|uninstall|restart",
		UsageText: "鐢ㄦ硶: /tools list|install|uninstall|restart <鍙傛暟>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "list", Usage: "/tools list", Summary: "鏌ョ湅 Bridge tools"},
			{Name: "install", Usage: "/tools install <manifest-url>", Summary: "瀹夎 Bridge tool 鎻掍欢"},
			{Name: "uninstall", Usage: "/tools uninstall <plugin-id>", Summary: "鍗歌浇 Bridge tool 鎻掍欢"},
			{Name: "restart", Usage: "/tools restart <plugin-id>", Summary: "閲嶅惎 Bridge tool 鎻掍欢"},
		},
	},
	{
		Command:   "/cost",
		Summary:   "费用统计",
		Usage:     "/cost",
		UsageText: "用法: /cost",
	},
	{
		Command:   "/help",
		Summary:   "显示帮助",
		Usage:     "/help",
		UsageText: "用法: /help",
	},
}

func buildHelpText() string {
	var sb strings.Builder
	sb.WriteString("AgentForge IM 助手\n\n可用命令:\n")

	for _, entry := range operatorCommandCatalog {
		if len(entry.Subcommands) == 0 {
			sb.WriteString(fmt.Sprintf("  %-24s — %s\n", entry.Usage, entry.Summary))
			continue
		}
		for _, subcommand := range entry.Subcommands {
			sb.WriteString(fmt.Sprintf("  %-24s — %s\n", subcommand.Usage, subcommand.Summary))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("或者直接 @AgentForge <你的需求> 使用自然语言")
	return strings.TrimRight(sb.String(), "\n")
}

func defaultCommandGuidance() string {
	return "发送 /help 查看可用命令，或 @AgentForge <你的需求> 使用自然语言"
}

func DefaultCommandGuidance() string {
	return defaultCommandGuidance()
}

func commandUsage(command string) string {
	if entry := findCommandCatalogEntry(command); entry != nil {
		if strings.TrimSpace(entry.UsageText) != "" {
			return entry.UsageText
		}
		return fmt.Sprintf("用法: %s", entry.Usage)
	}
	return "用法: /help"
}

func canonicalSubcommand(command, raw string) string {
	entry := findCommandCatalogEntry(command)
	if entry == nil {
		return strings.TrimSpace(raw)
	}
	normalized := strings.TrimSpace(raw)
	for _, subcommand := range entry.Subcommands {
		if subcommand.Name == normalized {
			return subcommand.Name
		}
		for _, alias := range subcommand.Aliases {
			if alias == normalized {
				return subcommand.Name
			}
		}
	}
	return normalized
}

func subcommandUsage(command, subcommand string) string {
	entry := findCommandCatalogEntry(command)
	if entry == nil {
		return commandUsage(command)
	}
	canonical := canonicalSubcommand(command, subcommand)
	for _, item := range entry.Subcommands {
		if item.Name == canonical {
			return fmt.Sprintf("用法: %s", item.Usage)
		}
	}
	return commandUsage(command)
}

func findCommandCatalogEntry(command string) *commandCatalogEntry {
	for i := range operatorCommandCatalog {
		if operatorCommandCatalog[i].Command == command {
			return &operatorCommandCatalog[i]
		}
	}
	return nil
}

var runIDPattern = regexp.MustCompile(`run-[A-Za-z0-9_-]+`)

func suggestCommandFromCatalog(content string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(content, "@AgentForge", ""))
	if trimmed == "" {
		return "/help"
	}

	runID := runIDPattern.FindString(trimmed)
	switch {
	case strings.Contains(trimmed, "暂停") || strings.Contains(strings.ToLower(trimmed), "pause"):
		if runID != "" {
			return "/agent pause " + runID
		}
		return subcommandUsage("/agent", "pause")
	case strings.Contains(trimmed, "恢复") || strings.Contains(strings.ToLower(trimmed), "resume"):
		if runID != "" {
			return "/agent resume " + runID
		}
		return subcommandUsage("/agent", "resume")
	case strings.Contains(trimmed, "终止") || strings.Contains(trimmed, "停止") || strings.Contains(strings.ToLower(trimmed), "kill"):
		if runID != "" {
			return "/agent kill " + runID
		}
		return subcommandUsage("/agent", "kill")
	case strings.Contains(trimmed, "队列"):
		return "/queue list"
	case strings.Contains(trimmed, "成员") || strings.Contains(trimmed, "团队"):
		return "/team list"
	case strings.Contains(trimmed, "记忆") || strings.Contains(strings.ToLower(trimmed), "memory"):
		return "/memory search <query>"
	default:
		return "/help"
	}
}

func SuggestCommandFromCatalog(content string) string {
	return suggestCommandFromCatalog(content)
}
