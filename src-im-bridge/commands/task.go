package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	log "github.com/sirupsen/logrus"
)

var taskUsage = commandUsage("/task")

// RegisterTaskCommands registers /task sub-commands on the engine.
func RegisterTaskCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/task", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, taskUsage)
			return
		}
		subCmd := canonicalSubcommand("/task", parts[0])
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch subCmd {
		case "create":
			handleTaskCreate(ctx, p, msg, scopedClient, subArgs)
		case "list":
			handleTaskList(ctx, p, msg, scopedClient, subArgs)
		case "status":
			handleTaskStatus(ctx, p, msg, scopedClient, subArgs)
		case "assign":
			handleTaskAssign(ctx, p, msg, scopedClient, subArgs)
		case "decompose":
			handleTaskDecomposeBridgeFirst(ctx, p, msg, engine, scopedClient, subArgs)
		case "ai":
			handleTaskAI(ctx, p, msg, engine, scopedClient, subArgs)
		case "move":
			handleTaskMove(ctx, p, msg, scopedClient, subArgs)
		case "delete":
			handleTaskDelete(ctx, p, msg, scopedClient, subArgs)
		case "attach":
			handleTaskAttach(ctx, p, msg, scopedClient, subArgs)
		case "upload-log":
			handleTaskUploadLog(ctx, p, msg, scopedClient, subArgs)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, taskUsage)
		}
	})
}

// handleTaskAttach attaches a previously-staged attachment to a backend task.
// Usage: /task attach <task-id> <staged-id> [kind]
// The staged id comes from a prior POST /im/attachments response. The backend
// persists the attachment metadata + reference against the task.
func handleTaskAttach(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	fields := strings.Fields(strings.TrimSpace(args))
	if len(fields) < 2 {
		_ = p.Reply(ctx, msg.ReplyCtx, "Usage: /task attach <task-id> <staged-id> [kind]")
		return
	}
	taskID := strings.TrimSpace(fields[0])
	stagedID := strings.TrimSpace(fields[1])
	kind := "file"
	if len(fields) >= 3 {
		kind = strings.TrimSpace(fields[2])
	}
	if err := c.AttachToTask(ctx, taskID, client.TaskAttachmentRef{
		StagedID: stagedID,
		Kind:     kind,
	}); err != nil {
		replyError(ctx, p, msg.ReplyCtx, "添加附件失败", fmt.Sprintf("%v", err), "请先通过 POST /im/attachments 暂存文件，再引用返回的 staged_id")
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已为任务 #%s 添加附件 (staged_id=%s, kind=%s)", shortID(taskID), stagedID, kind))
}

// handleTaskUploadLog is a shortcut that classifies the attachment as
// AttachmentKindLogs. Same underlying contract as /task attach.
func handleTaskUploadLog(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	fields := strings.Fields(strings.TrimSpace(args))
	if len(fields) < 2 {
		_ = p.Reply(ctx, msg.ReplyCtx, "Usage: /task upload-log <task-id> <staged-id>")
		return
	}
	taskID := strings.TrimSpace(fields[0])
	stagedID := strings.TrimSpace(fields[1])
	if err := c.AttachToTask(ctx, taskID, client.TaskAttachmentRef{
		StagedID: stagedID,
		Kind:     "logs",
	}); err != nil {
		replyError(ctx, p, msg.ReplyCtx, "上传日志失败", fmt.Sprintf("%v", err), "请先通过 POST /im/attachments 暂存 log 文件")
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已为任务 #%s 上传日志 (staged_id=%s)", shortID(taskID), stagedID))
}

func handleTaskCreate(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, raw string) {
	input, err := parseTaskCreateInput(raw)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
		return
	}
	task, err := c.CreateTaskWithInput(ctx, client.CreateTaskInput{
		Title:       input.Title,
		Description: input.Description,
		Priority:    input.Priority,
	})
	if err != nil {
		replyError(ctx, p, msg.ReplyCtx, "创建任务失败", fmt.Sprintf("%v", err), "请检查参数后重试，或使用 /task list 查看现有任务")
		return
	}
	if err := replyTaskSummary(ctx, p, msg.ReplyCtx, task, "已创建任务"); err == nil {
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已创建任务 #%s: %s", shortID(task.ID), task.Title))
}

type taskCreateInput struct {
	Title       string
	Description string
	Priority    string
}

func parseTaskCreateInput(raw string) (*taskCreateInput, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	if len(tokens) == 0 {
		return nil, fmt.Errorf("用法: /task create <标题> [--priority <级别>] [--description <描述>]")
	}
	input := &taskCreateInput{Priority: "medium"}
	titleTokens := make([]string, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		switch tokens[i] {
		case "--priority":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("缺少 priority 值。用法: /task create <标题> [--priority <级别>] [--description <描述>]")
			}
			input.Priority = strings.TrimSpace(tokens[i+1])
			i++
		case "--description":
			if i+1 >= len(tokens) {
				return nil, fmt.Errorf("缺少 description 值。用法: /task create <标题> [--priority <级别>] [--description <描述>]")
			}
			input.Description = strings.TrimSpace(strings.Join(tokens[i+1:], " "))
			i = len(tokens)
		default:
			titleTokens = append(titleTokens, tokens[i])
		}
	}
	input.Title = strings.TrimSpace(strings.Join(titleTokens, " "))
	if input.Title == "" {
		return nil, fmt.Errorf("用法: /task create <标题> [--priority <级别>] [--description <描述>]")
	}
	switch input.Priority {
	case "critical", "high", "medium", "low":
	default:
		return nil, fmt.Errorf("priority 只支持 critical|high|medium|low")
	}
	return input, nil
}

func handleTaskList(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, filter string) {
	tasks, err := c.ListTasks(ctx, filter)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取任务列表失败: %v", err))
		return
	}
	if len(tasks) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "暂无任务")
		return
	}

	// Try rich rendering.
	message := buildTaskListStructuredMessage(tasks)
	if err := replyStructured(ctx, p, msg.ReplyCtx, message); err == nil {
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务列表 (%d):\n", len(tasks)))
	for _, t := range tasks {
		sb.WriteString(fmt.Sprintf("  #%s [%s] %s", shortID(t.ID), t.Status, t.Title))
		if t.AssigneeName != "" {
			sb.WriteString(fmt.Sprintf(" (@%s)", t.AssigneeName))
		}
		sb.WriteString("\n")
	}
	_ = p.Reply(ctx, msg.ReplyCtx, sb.String())
}

func buildTaskListStructuredMessage(tasks []client.Task) *core.StructuredMessage {
	sections := make([]core.StructuredSection, 0, len(tasks)+2)

	// Task rows as fields.
	fields := make([]core.StructuredField, 0, len(tasks))
	for _, t := range tasks {
		label := fmt.Sprintf("#%s [%s]", shortID(t.ID), t.Status)
		value := t.Title
		if t.AssigneeName != "" {
			value += fmt.Sprintf(" (@%s)", t.AssigneeName)
		}
		fields = append(fields, core.StructuredField{Label: label, Value: value})
	}
	sections = append(sections, core.StructuredSection{
		Type:          core.StructuredSectionTypeFields,
		FieldsSection: &core.FieldsSection{Fields: fields},
	})

	// Quick action buttons for first few tasks.
	actions := make([]core.StructuredAction, 0, 3)
	limit := len(tasks)
	if limit > 3 {
		limit = 3
	}
	for _, t := range tasks[:limit] {
		if t.Status != "done" && t.Status != "cancelled" {
			actions = append(actions, core.StructuredAction{
				ID:    "act:assign-agent:" + t.ID,
				Label: fmt.Sprintf("启动 #%s", shortID(t.ID)),
				Style: core.ActionStylePrimary,
			})
		}
	}
	if len(actions) > 0 {
		sections = append(sections, core.StructuredSection{
			Type:           core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{Actions: actions, ButtonsPerRow: 3},
		})
	}

	return &core.StructuredMessage{
		Title:    fmt.Sprintf("任务列表 (%d)", len(tasks)),
		Sections: sections,
	}
}

func handleTaskStatus(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, taskID string) {
	if taskID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task status <task-id>")
		return
	}
	task, err := c.GetTask(ctx, taskID)
	if err != nil {
		replyError(ctx, p, msg.ReplyCtx, "获取任务失败", fmt.Sprintf("%v", err), "请检查 task-id 是否正确")
		return
	}
	if err := replyTaskSummary(ctx, p, msg.ReplyCtx, task, ""); err == nil {
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, formatTaskSummaryText(task, ""))
}

func handleTaskDelete(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, taskID string) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task delete <task-id>")
		return
	}
	if err := c.DeleteTask(ctx, taskID); err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("删除任务失败: %v", err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已删除任务 #%s", shortID(taskID)))
}

func handleTaskAssign(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task assign <task-id> <assignee>")
		return
	}
	member, err := resolveProjectMember(ctx, c, parts[1])
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("分配失败: %v", err))
		return
	}

	result, err := c.AssignTask(ctx, parts[0], member.ID, member.Type)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("分配失败: %v", err))
		return
	}
	if msg.ReplyTarget != nil {
		binding := client.IMActionBinding{
			Platform:    msg.Platform,
			ProjectID:   result.Task.ProjectID,
			TaskID:      result.Task.ID,
			ReplyTarget: msg.ReplyTarget,
		}
		if result.Dispatch.Run != nil {
			binding.RunID = result.Dispatch.Run.ID
		}
		_ = c.BindActionContext(ctx, binding)
	}

	_ = p.Reply(ctx, msg.ReplyCtx, formatTaskDispatchReply(result, member.Name))
}

func handleTaskDecomposeBridgeFirst(ctx context.Context, p core.Platform, msg *core.Message, engine *core.Engine, c *client.AgentForgeClient, args string) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "鐢ㄦ硶: /task decompose <task-id>")
		return
	}
	taskID := parts[0]
	provider := ""
	model := ""
	if len(parts) > 1 {
		provider = parts[1]
	}
	if len(parts) > 2 {
		model = parts[2]
	}

	_ = p.Reply(ctx, msg.ReplyCtx, "姝ｅ湪鍒嗚В浠诲姟锛岃绋嶅€?..")

	route := engine.ResolveCommandRoute("/task", "decompose")
	available, routeErr := engine.BridgeCapabilityAvailable(ctx, route.Capability)
	fallbackReason := "Bridge unavailable"
	if available {
		result, err := c.DecomposeTaskViaBridge(ctx, taskID, provider, model)
		if err == nil {
			log.WithFields(log.Fields{
				"component": "commands.task",
				"taskId":    taskID,
				"provider":  provider,
				"model":     model,
			}).Info("Bridge task decomposition succeeded")
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeTaskDecomposition(result))
			return
		}
		fallbackReason = describeBridgeFailure(err)
		log.WithFields(log.Fields{
			"component": "commands.task",
			"taskId":    taskID,
			"provider":  provider,
			"model":     model,
		}).WithError(err).Warn("Bridge task decomposition failed; attempting legacy fallback")
	} else if routeErr != nil {
		fallbackReason = describeBridgeFailure(routeErr)
	}

	legacyResult, legacyErr := c.DecomposeTask(ctx, taskID)
	if msg.ReplyTarget != nil && legacyErr == nil {
		_ = c.BindActionContext(ctx, client.IMActionBinding{
			Platform:    msg.Platform,
			ProjectID:   legacyResult.ParentTask.ProjectID,
			TaskID:      legacyResult.ParentTask.ID,
			ReplyTarget: msg.ReplyTarget,
		})
	}
	if legacyErr != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("任务分解失败（%s，fallback 也失败）: %v\n未创建任何子任务，请稍后重试。", fallbackReason, legacyErr))
		return
	}
	prefix := fmt.Sprintf("Using fallback (%s)\n", fallbackReason)
	log.WithFields(log.Fields{
		"component": "commands.task",
		"taskId":    taskID,
		"fallback":  prefix,
	}).Info("Task decomposition completed via fallback path")
	_ = p.Reply(ctx, msg.ReplyCtx, formatLegacyTaskDecompositionWithPrefix(prefix, legacyResult))
}

func handleTaskDecompose(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, taskID string) {
	if strings.TrimSpace(taskID) == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task decompose <task-id>")
		return
	}

	replyProcessing(ctx, p, msg.ReplyCtx, "正在分解任务...")

	result, err := c.DecomposeTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("任务分解失败: %v\n未创建任何子任务，请稍后重试。", err))
		return
	}
	if msg.ReplyTarget != nil {
		_ = c.BindActionContext(ctx, client.IMActionBinding{
			Platform:    msg.Platform,
			ProjectID:   result.ParentTask.ProjectID,
			TaskID:      result.ParentTask.ID,
			ReplyTarget: msg.ReplyTarget,
		})
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务分解完成: #%s %s\n", shortID(result.ParentTask.ID), result.ParentTask.Title))
	sb.WriteString(fmt.Sprintf("摘要: %s\n", result.Summary))
	sb.WriteString("子任务:\n")
	for i, subtask := range result.Subtasks {
		sb.WriteString(fmt.Sprintf("%d. #%s [%s/%s] %s\n",
			i+1, shortID(subtask.ID), subtask.Status, subtask.Priority, subtask.Title))
	}

	_ = p.Reply(ctx, msg.ReplyCtx, strings.TrimRight(sb.String(), "\n"))
}

func formatBridgeTaskDecomposition(result *client.BridgeTaskDecompositionResponse) string {
	if result == nil {
		return "浠诲姟鍒嗚В缁撴灉涓嶅彲鐢?"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("浠诲姟鍒嗚В瀹屾垚: #%s %s\n", shortID(result.ParentTask.ID), result.ParentTask.Title))
	sb.WriteString(fmt.Sprintf("鎽樿: %s\n", result.Summary))
	sb.WriteString("瀛愪换鍔?\n")
	for i, subtask := range result.Subtasks {
		sb.WriteString(fmt.Sprintf("%d. [%s/%s] %s\n",
			i+1, subtask.ExecutionMode, subtask.Priority, subtask.Title))
	}
	appendBridgeDecomposeHandoff(&sb, result)
	return strings.TrimRight(sb.String(), "\n")
}

func formatLegacyTaskDecompositionWithPrefix(prefix string, result *client.TaskDecompositionResponse) string {
	if result == nil {
		return strings.TrimSpace(prefix)
	}
	var sb strings.Builder
	sb.WriteString(prefix)
	sb.WriteString(fmt.Sprintf("浠诲姟鍒嗚В瀹屾垚: #%s %s\n", shortID(result.ParentTask.ID), result.ParentTask.Title))
	sb.WriteString(fmt.Sprintf("鎽樿: %s\n", result.Summary))
	sb.WriteString("瀛愪换鍔?\n")
	for i, subtask := range result.Subtasks {
		sb.WriteString(fmt.Sprintf("%d. #%s [%s/%s] %s\n",
			i+1, shortID(subtask.ID), subtask.Status, subtask.Priority, subtask.Title))
	}
	appendLegacyDecomposeHandoff(&sb, result)
	return strings.TrimRight(sb.String(), "\n")
}

func appendBridgeDecomposeHandoff(sb *strings.Builder, result *client.BridgeTaskDecompositionResponse) {
	if sb == nil || result == nil {
		return
	}
	commands := make([]string, 0, len(result.Subtasks))
	for _, subtask := range result.Subtasks {
		if strings.TrimSpace(subtask.ExecutionMode) != "agent" {
			continue
		}
		title := strings.TrimSpace(subtask.Title)
		if title == "" {
			continue
		}
		commands = append(commands, "/agent run "+title)
	}
	appendHandoffCommands(sb, commands)
}

func appendLegacyDecomposeHandoff(sb *strings.Builder, result *client.TaskDecompositionResponse) {
	if sb == nil || result == nil {
		return
	}
	commands := make([]string, 0, len(result.Subtasks))
	for _, subtask := range result.Subtasks {
		taskID := strings.TrimSpace(subtask.ID)
		if taskID == "" {
			continue
		}
		commands = append(commands, "/agent spawn "+taskID)
	}
	appendHandoffCommands(sb, commands)
}

func appendHandoffCommands(sb *strings.Builder, commands []string) {
	if sb == nil || len(commands) == 0 {
		return
	}
	sb.WriteString("可继续执行:\n")
	for _, command := range commands {
		sb.WriteString("- " + command + "\n")
	}
}

func handleTaskMove(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) < 2 {
		_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/task", "move"))
		return
	}

	task, err := c.TransitionTaskStatus(ctx, parts[0], parts[1])
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("流转任务失败: %v", err))
		return
	}

	if err := replyTaskSummary(ctx, p, msg.ReplyCtx, task, "任务状态已更新"); err == nil {
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, formatTaskSummaryText(task, "任务状态已更新"))
}

func handleTaskAI(ctx context.Context, p core.Platform, msg *core.Message, engine *core.Engine, c *client.AgentForgeClient, args string) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "鐢ㄦ硶: /task ai generate|classify <鍙傛暟>")
		return
	}

	switch parts[0] {
	case "generate":
		route := engine.ResolveCommandRoute("/task ai", "generate")
		if available, err := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
			log.WithFields(log.Fields{"component": "commands.task"}).WithError(err).Warn("Bridge task AI generate capability unavailable")
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("AI 鐢熸垚澶辫触: %v", err))
			return
		}
		handleTaskAIGenerate(ctx, p, msg, c, strings.TrimSpace(args[len(parts[0]):]))
	case "classify":
		route := engine.ResolveCommandRoute("/task ai", "classify")
		if available, err := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
			log.WithFields(log.Fields{"component": "commands.task"}).WithError(err).Warn("Bridge task AI classify capability unavailable")
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("AI 鎰忓浘鍒嗙被澶辫触: %v", err))
			return
		}
		handleTaskAIClassify(ctx, p, msg, c, strings.TrimSpace(args[len(parts[0]):]))
	default:
		_ = p.Reply(ctx, msg.ReplyCtx, "鐢ㄦ硶: /task ai generate|classify <鍙傛暟>")
	}
}

func handleTaskAIGenerate(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	model := ""
	trimmed := strings.TrimSpace(args)
	if strings.HasPrefix(trimmed, "--model ") {
		remainder := strings.TrimSpace(strings.TrimPrefix(trimmed, "--model "))
		parts := strings.Fields(remainder)
		if len(parts) == 0 {
			_ = p.Reply(ctx, msg.ReplyCtx, "鐢ㄦ硶: /task ai generate [--model <model>] <prompt>")
			return
		}
		model = parts[0]
		trimmed = strings.TrimSpace(strings.TrimPrefix(remainder, model))
	}
	if trimmed == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "鐢ㄦ硶: /task ai generate [--model <model>] <prompt>")
		return
	}

	result, err := c.GenerateTaskAI(ctx, trimmed, "", model)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("AI 鐢熸垚澶辫触（%s）: %v", describeBridgeFailure(err), err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, result.Text)
}

func handleTaskAIClassify(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "鐢ㄦ硶: /task ai classify <text> [candidate1,candidate2]")
		return
	}

	text := trimmed
	var candidates []string
	if idx := strings.LastIndex(trimmed, " "); idx > 0 {
		tail := strings.TrimSpace(trimmed[idx+1:])
		if strings.Contains(tail, ",") {
			text = strings.TrimSpace(trimmed[:idx])
			for _, candidate := range strings.Split(tail, ",") {
				candidate = strings.TrimSpace(candidate)
				if candidate != "" {
					candidates = append(candidates, candidate)
				}
			}
		}
	}

	result, err := c.ClassifyTaskAI(ctx, text, candidates)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("AI 鎰忓浘鍒嗙被澶辫触（%s）: %v", describeBridgeFailure(err), err))
		return
	}
	reply := fmt.Sprintf("intent=%s confidence=%.2f", result.Intent, result.Confidence)
	if strings.TrimSpace(result.Reply) != "" {
		reply += "\n" + result.Reply
	}
	_ = p.Reply(ctx, msg.ReplyCtx, reply)
}

func replyTaskSummary(ctx context.Context, p core.Platform, replyCtx any, task *client.Task, lead string) error {
	message := buildTaskSummaryMessage(p, task, lead)
	if err := replyStructured(ctx, p, replyCtx, message); err == nil {
		return nil
	}
	if message == nil {
		return errNoRichSender
	}
	return p.Reply(ctx, replyCtx, message.FallbackText())
}

func buildTaskCard(task *client.Task) *core.Card {
	if message := buildTaskSummaryMessage(nil, task, ""); message != nil {
		return message.LegacyCard()
	}
	return nil
}

func buildTaskSummaryMessage(p core.Platform, task *client.Task, lead string) *core.StructuredMessage {
	if task == nil {
		return nil
	}
	sections := []core.StructuredSection{
		{
			Type: core.StructuredSectionTypeText,
			TextSection: &core.TextSection{
				Body: formatTaskHeader(task, lead),
			},
		},
		{
			Type: core.StructuredSectionTypeFields,
			FieldsSection: &core.FieldsSection{
				Fields: buildTaskSummaryFields(task),
			},
		},
	}

	actionsEnabled := taskCallbackActionsEnabled(p)
	actions := buildTaskSummaryActions(task, actionsEnabled)
	if len(actions) > 0 {
		sections = append(sections, core.StructuredSection{
			Type:           core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{Actions: actions, ButtonsPerRow: 2},
		})
	}
	if !actionsEnabled {
		if guidanceFields := buildTaskManualGuidanceFields(task); len(guidanceFields) > 0 {
			sections = append(sections, core.StructuredSection{
				Type:          core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{Fields: guidanceFields},
			})
		}
	}

	return &core.StructuredMessage{
		Title:    fmt.Sprintf("任务 #%s", shortID(task.ID)),
		Sections: sections,
	}
}

func buildTaskSummaryFields(task *client.Task) []core.StructuredField {
	fields := []core.StructuredField{
		{Label: "标题", Value: strings.TrimSpace(task.Title)},
		{Label: "状态", Value: strings.TrimSpace(task.Status)},
		{Label: "优先级", Value: strings.TrimSpace(task.Priority)},
	}
	if trimmed := strings.TrimSpace(task.AssigneeName); trimmed != "" {
		fields = append(fields, core.StructuredField{Label: "负责人", Value: trimmed})
	}
	if task.BudgetUsd > 0 {
		fields = append(fields, core.StructuredField{Label: "预算", Value: fmt.Sprintf("$%.2f / $%.2f", task.SpentUsd, task.BudgetUsd)})
	}
	return fields
}

func buildTaskSummaryActions(task *client.Task, actionsEnabled bool) []core.StructuredAction {
	if task == nil {
		return nil
	}
	actions := make([]core.StructuredAction, 0, 5)
	if actionsEnabled {
		if nextStatus, ok := suggestedNextTaskStatus(task.Status); ok {
			actions = append(actions, core.StructuredAction{
				ID: core.BuildActionReference("transition-task", task.ID, map[string]string{
					"targetStatus": nextStatus,
				}),
				Label: transitionActionLabel(nextStatus),
				Style: core.ActionStylePrimary,
			})
		}
		if canAssignAgent(task.Status) {
			actions = append(actions, core.StructuredAction{
				ID:    "act:assign-agent:" + task.ID,
				Label: "分配给 Agent",
				Style: core.ActionStyleDefault,
			})
		}
		if strings.TrimSpace(task.ProjectID) != "" {
			actions = append(actions, core.StructuredAction{
				ID: core.BuildActionReference("save-as-doc", task.ProjectID, map[string]string{
					"body":  buildTaskMessageBody(task),
					"title": task.Title,
				}),
				Label: "保存为文档",
				Style: core.ActionStyleDefault,
			})
			followupTitle := strings.TrimSpace(task.Title)
			if followupTitle == "" {
				followupTitle = "IM Task"
			} else {
				followupTitle = "Follow up: " + followupTitle
			}
			actions = append(actions, core.StructuredAction{
				ID: core.BuildActionReference("create-task", task.ProjectID, map[string]string{
					"body":     buildTaskMessageBody(task),
					"priority": defaultTaskPriority(task.Priority),
					"title":    followupTitle,
				}),
				Label: "创建跟进任务",
				Style: core.ActionStyleDefault,
			})
		}
	}
	if strings.TrimSpace(task.ID) != "" {
		actions = append(actions, core.StructuredAction{
			Label: "查看详情",
			URL:   "/tasks/" + task.ID,
			Style: core.ActionStyleDefault,
		})
	}
	return actions
}

func taskCallbackActionsEnabled(p core.Platform) bool {
	if p == nil {
		return true
	}
	if _, ok := p.(core.ReplyStructuredSender); !ok {
		if _, ok := p.(core.CardSender); !ok {
			return false
		}
	}
	return helpQuickActionsEnabled(p)
}

func formatTaskHeader(task *client.Task, lead string) string {
	parts := make([]string, 0, 2)
	if trimmed := strings.TrimSpace(lead); trimmed != "" {
		parts = append(parts, trimmed)
	}
	parts = append(parts, fmt.Sprintf("任务 %s [%s] %s", shortID(task.ID), strings.TrimSpace(task.Status), strings.TrimSpace(task.Title)))
	return strings.Join(parts, "\n")
}

func formatTaskSummaryText(task *client.Task, lead string) string {
	if task == nil {
		return strings.TrimSpace(lead)
	}
	lines := []string{formatTaskHeader(task, lead)}
	for _, field := range buildTaskSummaryFields(task) {
		if strings.TrimSpace(field.Value) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", field.Label, field.Value))
	}
	if guidance := formatTaskNextStepGuidance(task); guidance != "" {
		lines = append(lines, guidance)
	}
	return strings.Join(lines, "\n")
}

func formatTaskNextStepGuidance(task *client.Task) string {
	if task == nil {
		return ""
	}
	commands := make([]string, 0, 2)
	if nextStatus, ok := suggestedNextTaskStatus(task.Status); ok && strings.TrimSpace(task.ID) != "" {
		commands = append(commands, fmt.Sprintf("/task move %s %s", task.ID, nextStatus))
	}
	if canAssignAgent(task.Status) && strings.TrimSpace(task.ID) != "" {
		commands = append(commands, fmt.Sprintf("/task assign %s <assignee>", task.ID))
	}
	if len(commands) == 0 {
		return ""
	}
	return "下一步:\n- " + strings.Join(commands, "\n- ")
}

func buildTaskManualGuidanceFields(task *client.Task) []core.StructuredField {
	commands := make([]string, 0, 2)
	if task == nil {
		return nil
	}
	if nextStatus, ok := suggestedNextTaskStatus(task.Status); ok && strings.TrimSpace(task.ID) != "" {
		commands = append(commands, fmt.Sprintf("/task move %s %s", task.ID, nextStatus))
	}
	if canAssignAgent(task.Status) && strings.TrimSpace(task.ID) != "" {
		commands = append(commands, fmt.Sprintf("/task assign %s <assignee>", task.ID))
	}
	if len(commands) == 0 {
		return nil
	}
	return []core.StructuredField{{Label: "下一步", Value: strings.Join(commands, "\n")}}
}

func suggestedNextTaskStatus(status string) (string, bool) {
	switch strings.TrimSpace(status) {
	case "inbox":
		return "triaged", true
	case "triaged":
		return "assigned", true
	case "assigned":
		return "in_progress", true
	case "in_progress":
		return "in_review", true
	case "in_review":
		return "done", true
	case "changes_requested":
		return "in_progress", true
	case "blocked":
		return "in_progress", true
	case "budget_exceeded":
		return "in_progress", true
	default:
		return "", false
	}
}

func transitionActionLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "triaged":
		return "标记为已分诊"
	case "assigned":
		return "标记为已分配"
	case "in_progress":
		return "开始处理"
	case "in_review":
		return "提交审查"
	case "done":
		return "标记为完成"
	default:
		return "流转任务"
	}
}

func canAssignAgent(status string) bool {
	switch strings.TrimSpace(status) {
	case "done", "cancelled":
		return false
	default:
		return true
	}
}

func buildTaskMessageBody(task *client.Task) string {
	if task == nil {
		return ""
	}
	description := strings.TrimSpace(task.Description)
	if description != "" {
		return description
	}
	parts := []string{
		fmt.Sprintf("任务 #%s", shortID(task.ID)),
		strings.TrimSpace(task.Title),
	}
	if status := strings.TrimSpace(task.Status); status != "" {
		parts = append(parts, "状态: "+status)
	}
	if priority := strings.TrimSpace(task.Priority); priority != "" {
		parts = append(parts, "优先级: "+priority)
	}
	return strings.Join(parts, "\n")
}

func defaultTaskPriority(priority string) string {
	trimmed := strings.TrimSpace(priority)
	if trimmed == "" {
		return "medium"
	}
	return trimmed
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func resolveProjectMember(ctx context.Context, c *client.AgentForgeClient, assignee string) (*client.Member, error) {
	members, err := c.ListProjectMembers(ctx)
	if err != nil {
		return nil, err
	}

	query := strings.TrimSpace(assignee)
	lowerQuery := strings.ToLower(query)
	for i := range members {
		member := &members[i]
		if member.ID == query || strings.EqualFold(member.Name, query) || strings.ToLower(member.Name) == lowerQuery {
			return member, nil
		}
	}

	return nil, fmt.Errorf("未找到成员 %q", assignee)
}

func formatTaskDispatchReply(result *client.TaskDispatchResponse, assigneeName string) string {
	taskID := shortID(result.Task.ID)
	switch result.Dispatch.Status {
	case "started":
		if result.Dispatch.Run != nil {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，并启动 Agent #%s", taskID, assigneeName, shortID(result.Dispatch.Run.ID))
		}
		return fmt.Sprintf("已将任务 #%s 分配给 %s，并启动 Agent", taskID, assigneeName)
	case "queued":
		if result.Dispatch.Queue != nil {
			prefix := fmt.Sprintf("已将任务 #%s 分配给 %s，并加入 Agent 队列 #%s", taskID, assigneeName, shortID(result.Dispatch.Queue.EntryID))
			if result.Dispatch.Queue.RecoveryDisposition == "recoverable" {
				prefix = fmt.Sprintf("已将任务 #%s 分配给 %s，且仍在 Agent 队列 #%s 中等待恢复", taskID, assigneeName, shortID(result.Dispatch.Queue.EntryID))
			}
			if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
				return prefix + "：" + reason
			}
			return prefix
		}
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，并加入 Agent 队列：%s", taskID, assigneeName, reason)
		}
		return fmt.Sprintf("已将任务 #%s 分配给 %s，并加入 Agent 队列", taskID, assigneeName)
	case "blocked":
		if result.Dispatch.GuardrailType == "budget" && strings.TrimSpace(result.Dispatch.GuardrailScope) != "" {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，但未启动 Agent：%s budget blocked dispatch: %s", taskID, assigneeName, strings.TrimSpace(result.Dispatch.GuardrailScope), defaultDispatchReplyReason(result.Dispatch.Reason, "budget guardrail"))
		}
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，但未启动 Agent：%s", taskID, assigneeName, reason)
		}
		return fmt.Sprintf("已将任务 #%s 分配给 %s，但未启动 Agent", taskID, assigneeName)
	case "skipped":
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，但本次未启动 Agent：%s", taskID, assigneeName, reason)
		}
		return fmt.Sprintf("已将任务 #%s 分配给 %s，但本次未启动 Agent", taskID, assigneeName)
	default:
		return fmt.Sprintf("已将任务 #%s 分配给 %s", taskID, assigneeName)
	}
}
