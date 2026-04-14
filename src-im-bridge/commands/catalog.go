package commands

import (
	"fmt"
	"regexp"
	"slices"
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

type IntentCandidate struct {
	Intent   string
	Command  string
	Summary  string
	Keywords []string
}

type directRuntimeMention struct {
	Mentions []string
	Runtime  string
	Provider string
}

var operatorCommandCatalog = []commandCatalogEntry{
	{
		Command:   "/task",
		Summary:   "任务管理",
		Usage:     "/task create|list|status|assign|decompose|ai|move|delete",
		UsageText: "用法: /task create|list|status|assign|decompose|ai|move|delete <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "create", Usage: "/task create <标题> [--priority <级别>] [--description <描述>]", Summary: "创建新任务"},
			{Name: "list", Usage: "/task list [状态]", Summary: "查看任务列表"},
			{Name: "status", Usage: "/task status <task-id>", Summary: "查看任务详情"},
			{Name: "assign", Usage: "/task assign <id> <人员>", Summary: "分配任务"},
			{Name: "decompose", Usage: "/task decompose <id>", Summary: "AI 分解现有任务"},
			{Name: "ai", Usage: "/task ai generate|classify <参数>", Summary: "Bridge AI 生成与意图分类"},
			{Name: "move", Usage: "/task move <task-id> <status>", Summary: "流转任务状态", Aliases: []string{"transition"}},
			{Name: "delete", Usage: "/task delete <task-id>", Summary: "删除任务"},
		},
	},
	{
		Command:   "/agent",
		Summary:   "Agent 运行控制",
		Usage:     "/agent status|runtimes|health|spawn|run|config|logs|pause|resume|kill",
		UsageText: "用法: /agent status|runtimes|health|spawn|run|config|logs|pause|resume|kill <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "status", Usage: "/agent status [run-id]", Summary: "查看 Agent 池或运行状态", Aliases: []string{"list"}},
			{Name: "runtimes", Usage: "/agent runtimes", Summary: "查看 Bridge runtimes"},
			{Name: "health", Usage: "/agent health", Summary: "查看 Bridge health"},
			{Name: "spawn", Usage: "/agent spawn <task-id>", Summary: "为任务启动 Agent"},
			{Name: "run", Usage: "/agent run [--runtime <runtime>] [--provider <provider>] [--model <model>] <描述>", Summary: "创建任务并按指定 runtime 启动 Agent"},
			{Name: "config", Usage: "/agent config get|set <参数>", Summary: "查看或设置当前项目的代码 Agent 默认配置"},
			{Name: "logs", Usage: "/agent logs <run-id>", Summary: "查看 Agent 执行日志"},
			{Name: "pause", Usage: "/agent pause <run-id>", Summary: "暂停 Agent 运行"},
			{Name: "resume", Usage: "/agent resume <run-id>", Summary: "恢复 Agent 运行"},
			{Name: "kill", Usage: "/agent kill <run-id>", Summary: "终止 Agent 运行"},
		},
	},
	{
		Command:   "/login",
		Summary:   "运行时登录与凭据状态",
		Usage:     "/login status|codex|claude|opencode|cursor|gemini|qoder|iflow",
		UsageText: "用法: /login status|codex|claude|opencode|cursor|gemini|qoder|iflow",
		Subcommands: []commandCatalogSubcommand{
			{Name: "status", Usage: "/login status", Summary: "查看所有 runtime 登录/凭据状态"},
			{Name: "codex", Usage: "/login codex", Summary: "查看 Codex 登录指引"},
			{Name: "claude", Usage: "/login claude", Summary: "查看 Claude Code 凭据指引"},
			{Name: "opencode", Usage: "/login opencode", Summary: "查看 OpenCode 登录/配置指引"},
			{Name: "cursor", Usage: "/login cursor", Summary: "查看 Cursor CLI 可用性"},
			{Name: "gemini", Usage: "/login gemini", Summary: "查看 Gemini CLI 可用性"},
			{Name: "qoder", Usage: "/login qoder", Summary: "查看 Qoder CLI 可用性"},
			{Name: "iflow", Usage: "/login iflow", Summary: "查看 iFlow CLI 可用性"},
		},
	},
	{
		Command:   "/project",
		Summary:   "项目作用域与管理",
		Usage:     "/project list|current|info|members|set|create|rename|delete",
		UsageText: "用法: /project list|current|info|members|set|create|rename|delete <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "list", Usage: "/project list", Summary: "查看可用项目"},
			{Name: "current", Usage: "/project current", Summary: "查看当前已选项目"},
			{Name: "info", Usage: "/project info [project-id|slug]", Summary: "查看项目详情"},
			{Name: "members", Usage: "/project members [project-id|slug]", Summary: "查看项目成员"},
			{Name: "set", Usage: "/project set <project-id|slug>", Summary: "切换当前项目作用域"},
			{Name: "create", Usage: "/project create <name>", Summary: "创建新项目并自动切换"},
			{Name: "rename", Usage: "/project rename <project-id|slug> <new-name>", Summary: "重命名项目"},
			{Name: "delete", Usage: "/project delete <project-id|slug>", Summary: "删除指定项目"},
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
		Usage:     "/team list|add|remove",
		UsageText: "用法: /team list|add|remove <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "list", Usage: "/team list", Summary: "查看项目成员摘要"},
			{Name: "add", Usage: "/team add <human|agent> <name> [role]", Summary: "添加项目成员"},
			{Name: "remove", Usage: "/team remove <member-id|name>", Summary: "移除项目成员"},
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
		Summary:   "Bridge 工具管理",
		Usage:     "/tools list|install|uninstall|restart",
		UsageText: "用法: /tools list|install|uninstall|restart <参数>",
		Subcommands: []commandCatalogSubcommand{
			{Name: "list", Usage: "/tools list", Summary: "查看 Bridge tools"},
			{Name: "install", Usage: "/tools install <manifest-url>", Summary: "安装 Bridge tool 插件"},
			{Name: "uninstall", Usage: "/tools uninstall <plugin-id>", Summary: "卸载 Bridge tool 插件"},
			{Name: "restart", Usage: "/tools restart <plugin-id>", Summary: "重启 Bridge tool 插件"},
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

var operatorIntentCatalog = []IntentCandidate{
	{Intent: "help", Command: "/help", Summary: "查看帮助和可用命令", Keywords: []string{"help", "帮助", "命令"}},
	{Intent: "create_task", Command: "/task create", Summary: "创建新任务", Keywords: []string{"create task", "创建任务", "新任务"}},
	{Intent: "decompose_task", Command: "/task decompose", Summary: "分解任务", Keywords: []string{"decompose", "分解", "拆分任务"}},
	{Intent: "task_list", Command: "/task list", Summary: "查看任务列表", Keywords: []string{"任务", "task", "列表", "list"}},
	{Intent: "agent_spawn", Command: "/agent spawn", Summary: "为任务启动 Agent", Keywords: []string{"spawn agent", "启动 agent", "派 agent", "执行任务"}},
	{Intent: "login_status", Command: "/login status", Summary: "查看 runtime 登录状态", Keywords: []string{"登录", "login", "auth", "认证"}},
	{Intent: "project_list", Command: "/project list", Summary: "查看项目列表", Keywords: []string{"项目", "project", "workspace"}},
	{Intent: "sprint_status", Command: "/sprint status", Summary: "查看当前 sprint", Keywords: []string{"sprint", "迭代", "进度", "状态"}},
	{Intent: "review", Command: "/review", Summary: "触发代码审查", Keywords: []string{"review", "审查", "pr", "pull request"}},
	{Intent: "review_followup_tasks", Command: "/review", Summary: "审查并生成后续任务建议", Keywords: []string{"review", "follow-up", "后续任务", "issues", "修复项"}},
	{Intent: "queue_list", Command: "/queue list", Summary: "查看 Agent 队列", Keywords: []string{"队列", "queue", "排队"}},
	{Intent: "team_list", Command: "/team list", Summary: "查看团队成员摘要", Keywords: []string{"团队", "成员", "team"}},
	{Intent: "memory_search", Command: "/memory search <query>", Summary: "搜索项目记忆", Keywords: []string{"记忆", "memory", "知识"}},
	{Intent: "agent_pause", Command: "/agent pause", Summary: "暂停指定 Agent", Keywords: []string{"暂停", "pause"}},
	{Intent: "agent_resume", Command: "/agent resume", Summary: "恢复指定 Agent", Keywords: []string{"恢复", "resume"}},
	{Intent: "agent_kill", Command: "/agent kill", Summary: "终止指定 Agent", Keywords: []string{"终止", "停止", "kill"}},
}

var directRuntimeMentions = []directRuntimeMention{
	{Mentions: []string{"@claude", "@claudecode", "@claude-code"}, Runtime: "claude_code", Provider: "anthropic"},
	{Mentions: []string{"@codex"}, Runtime: "codex", Provider: "openai"},
	{Mentions: []string{"@opencode"}, Runtime: "opencode", Provider: "opencode"},
	{Mentions: []string{"@cursor"}, Runtime: "cursor", Provider: "cursor"},
	{Mentions: []string{"@gemini"}, Runtime: "gemini", Provider: "google"},
	{Mentions: []string{"@qoder"}, Runtime: "qoder", Provider: "qoder"},
	{Mentions: []string{"@iflow"}, Runtime: "iflow", Provider: "iflow"},
}

func IntentCandidates() []string {
	candidates := make([]string, 0, len(operatorIntentCatalog))
	for _, item := range operatorIntentCatalog {
		candidates = append(candidates, item.Intent)
	}
	return candidates
}

func RankIntentCandidates(content string) []IntentCandidate {
	trimmed := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(content, "@AgentForge", "")))
	scored := make([]struct {
		item  IntentCandidate
		score int
	}, 0, len(operatorIntentCatalog))

	for _, item := range operatorIntentCatalog {
		score := 0
		switch item.Intent {
		case "task_list":
			score += 3
		case "sprint_status":
			score += 2
		case "help":
			score += 1
		}
		for _, keyword := range item.Keywords {
			if keyword != "" && strings.Contains(trimmed, strings.ToLower(keyword)) {
				score += 10
			}
		}
		scored = append(scored, struct {
			item  IntentCandidate
			score int
		}{item: item, score: score})
	}

	slices.SortStableFunc(scored, func(a, b struct {
		item  IntentCandidate
		score int
	}) int {
		switch {
		case a.score > b.score:
			return -1
		case a.score < b.score:
			return 1
		default:
			return strings.Compare(a.item.Intent, b.item.Intent)
		}
	})

	ranked := make([]IntentCandidate, 0, len(scored))
	for _, entry := range scored {
		ranked = append(ranked, entry.item)
	}
	return ranked
}

func ResolveIntentCommand(intent, command, args string) string {
	intent = strings.TrimSpace(intent)
	command = strings.TrimSpace(command)
	args = strings.TrimSpace(args)

	if intent != "" {
		for _, item := range operatorIntentCatalog {
			if item.Intent == intent {
				command = item.Command
				break
			}
		}
	}
	if command == "" || !strings.HasPrefix(command, "/") {
		return ""
	}
	if args == "" {
		return command
	}
	return command + " " + args
}

func FormatIntentDisambiguation(content string, preferredCommand string) string {
	lines := []string{"可能的命令:"}
	seen := map[string]struct{}{}
	appendCommand := func(command string) {
		command = strings.TrimSpace(command)
		if command == "" {
			return
		}
		if _, ok := seen[command]; ok {
			return
		}
		seen[command] = struct{}{}
		lines = append(lines, fmt.Sprintf("- %s", command))
	}
	appendCommand(preferredCommand)
	for _, item := range RankIntentCandidates(content) {
		appendCommand(item.Command)
		if len(lines) >= 4 {
			break
		}
	}
	if len(lines) == 1 {
		lines = append(lines, "- /help")
	}
	return strings.Join(lines, "\n")
}

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
		ranked := RankIntentCandidates(content)
		if len(ranked) > 0 {
			return ranked[0].Command
		}
		return "/help"
	}
}

func SuggestCommandFromCatalog(content string) string {
	return suggestCommandFromCatalog(content)
}

func ResolveDirectRuntimeMention(content string) string {
	trimmed := strings.TrimSpace(content)
	lowerTrimmed := strings.ToLower(trimmed)
	for _, entry := range directRuntimeMentions {
		for _, mention := range entry.Mentions {
			if !strings.HasPrefix(lowerTrimmed, mention) {
				continue
			}
			remainder := strings.TrimSpace(trimmed[len(mention):])
			if remainder == "" {
				return ""
			}
			return fmt.Sprintf("/agent run --runtime %s %s", entry.Runtime, remainder)
		}
	}
	return ""
}
