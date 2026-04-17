package commands

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	log "github.com/sirupsen/logrus"
)

// RegisterAgentCommands registers /agent sub-commands on the engine.
func RegisterAgentCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/agent", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/agent"))
			return
		}
		subCmd := canonicalSubcommand("/agent", parts[0])
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch subCmd {
		case "status":
			if strings.TrimSpace(subArgs) != "" {
				run, runErr := scopedClient.GetAgentRun(ctx, strings.TrimSpace(subArgs))
				if runErr != nil {
					_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Agent 状态失败: %v", runErr))
					return
				}
				_ = p.Reply(ctx, msg.ReplyCtx, formatAgentRunSummary(run))
				return
			}
			status, err := scopedClient.GetAgentPoolStatus(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Agent 状态失败: %v", err))
				return
			}
			route := engine.ResolveCommandRoute("/agent", "status")
			available, bridgeErr := engine.BridgeCapabilityAvailable(ctx, route.Capability)
			if !available {
				log.WithFields(log.Fields{"component": "commands.agent"}).WithError(bridgeErr).Warn("Bridge pool capability unavailable during agent status")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("%s\nBridge pool unavailable: %v", formatAgentPoolStatus(status), bridgeErr))
				return
			}
			bridgePool, bridgeErr := scopedClient.GetBridgePoolStatus(ctx)
			if bridgeErr != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("%s\nBridge pool unavailable: %v", formatAgentPoolStatus(status), bridgeErr))
				return
			}
			if sm := buildAgentPoolStructuredMessage(status, bridgePool); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentPoolStatusWithBridge(status, bridgePool))
		case "runtimes":
			route := engine.ResolveCommandRoute("/agent", "runtimes")
			if available, bridgeErr := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
				log.WithFields(log.Fields{"component": "commands.agent"}).WithError(bridgeErr).Warn("Bridge runtime catalog capability unavailable")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge runtimes 失败（%s）: %v", describeBridgeFailure(bridgeErr), bridgeErr))
				return
			}
			runtimes, err := scopedClient.GetBridgeRuntimes(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge runtimes 失败（%s）: %v", describeBridgeFailure(err), err))
				return
			}
			if sm := buildRuntimeCatalogStructuredMessage(runtimes); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeRuntimeCatalog(runtimes))
		case "health":
			route := engine.ResolveCommandRoute("/agent", "health")
			if available, bridgeErr := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
				log.WithFields(log.Fields{"component": "commands.agent"}).WithError(bridgeErr).Warn("Bridge health capability unavailable")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge health 失败（%s）: %v", describeBridgeFailure(bridgeErr), bridgeErr))
				return
			}
			health, err := scopedClient.GetBridgeHealth(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge health 失败（%s）: %v", describeBridgeFailure(err), err))
				return
			}
			if sm := buildBridgeHealthStructuredMessage(health); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeHealthStatus(health))
		case "spawn":
			if subArgs == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/agent", subCmd))
				return
			}
			result, err := scopedClient.SpawnAgent(ctx, subArgs)
			if err != nil {
				replyError(ctx, p, msg.ReplyCtx, "启动 Agent 失败", fmt.Sprintf("%v", err), "请检查 task-id 是否存在，或使用 /login status 确认 runtime 就绪")
				return
			}
			log.WithFields(log.Fields{
				"component": "commands.agent",
				"taskId":    subArgs,
				"status":    result.Dispatch.Status,
			}).Info("Agent spawn command completed")
			bindAgentDispatch(ctx, scopedClient, result, msg)
			if sm := buildAgentSpawnStructuredMessage(result, subArgs); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentSpawnReplyWithBridgeTools(ctx, scopedClient, result, subArgs))
		case "run":
			runRequest, err := parseAgentRunRequest(subArgs)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			selection, err := resolveAgentRunSelection(ctx, scopedClient, runRequest)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			replyProcessing(ctx, p, msg.ReplyCtx, "正在创建任务并启动 Agent...")
			result, err := scopedClient.QuickAgentRunWithOptions(ctx, runRequest.Prompt, client.AgentSpawnOptions{
				Runtime:  selection.Runtime,
				Provider: selection.Provider,
				Model:    selection.Model,
			})
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("执行失败: %v", err))
				return
			}
			log.WithFields(log.Fields{
				"component": "commands.agent",
				"taskId":    result.Task.ID,
				"status":    result.Dispatch.Status,
			}).Info("Quick agent run command completed")
			bindAgentDispatch(ctx, scopedClient, result, msg)
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentSpawnReplyWithBridgeTools(ctx, scopedClient, result, result.Task.ID))
		case "config":
			handleAgentConfigCommand(ctx, p, msg, scopedClient, subArgs)
		case "logs":
			if subArgs == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/agent", subCmd))
				return
			}
			logs, err := scopedClient.GetAgentLogs(ctx, subArgs)
			if err != nil {
				replyError(ctx, p, msg.ReplyCtx, "获取日志失败", fmt.Sprintf("%v", err), "请检查 run-id 是否正确")
				return
			}
			if sm := buildAgentLogsStructuredMessage(logs, subArgs); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentLogs(logs, subArgs))
		case "pause":
			runAgentLifecycleAction(ctx, p, msg, scopedClient, subCmd, subArgs)
		case "resume":
			runAgentLifecycleAction(ctx, p, msg, scopedClient, subCmd, subArgs)
		case "kill":
			runAgentLifecycleAction(ctx, p, msg, scopedClient, subCmd, subArgs)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/agent"))
		}
	})
}

type agentRunRequest struct {
	Prompt   string
	Runtime  string
	Provider string
	Model    string
}

func parseAgentRunRequest(raw string) (*agentRunRequest, error) {
	tokens := strings.Fields(strings.TrimSpace(raw))
	if len(tokens) == 0 {
		return nil, fmt.Errorf("用法: /agent run [--runtime <runtime>] [--provider <provider>] [--model <model>] <描述>")
	}
	request := &agentRunRequest{}
	promptStart := 0
	for promptStart < len(tokens) {
		switch tokens[promptStart] {
		case "--runtime":
			if promptStart+1 >= len(tokens) {
				return nil, fmt.Errorf("缺少 runtime 值。用法: /agent run [--runtime <runtime>] [--provider <provider>] [--model <model>] <描述>")
			}
			request.Runtime = strings.TrimSpace(tokens[promptStart+1])
			promptStart += 2
		case "--provider":
			if promptStart+1 >= len(tokens) {
				return nil, fmt.Errorf("缺少 provider 值。用法: /agent run [--runtime <runtime>] [--provider <provider>] [--model <model>] <描述>")
			}
			request.Provider = strings.TrimSpace(tokens[promptStart+1])
			promptStart += 2
		case "--model":
			if promptStart+1 >= len(tokens) {
				return nil, fmt.Errorf("缺少 model 值。用法: /agent run [--runtime <runtime>] [--provider <provider>] [--model <model>] <描述>")
			}
			request.Model = strings.TrimSpace(tokens[promptStart+1])
			promptStart += 2
		default:
			request.Prompt = strings.TrimSpace(strings.Join(tokens[promptStart:], " "))
			promptStart = len(tokens)
		}
	}
	if request.Prompt == "" {
		return nil, fmt.Errorf("用法: /agent run [--runtime <runtime>] [--provider <provider>] [--model <model>] <描述>")
	}
	return request, nil
}

func resolveAgentRunSelection(ctx context.Context, c *client.AgentForgeClient, request *agentRunRequest) (client.CodingAgentSelection, error) {
	projectID := strings.TrimSpace(c.ProjectScope())
	if projectID == "" {
		return client.CodingAgentSelection{}, fmt.Errorf("%s", clientHintProjectScopeRequired())
	}

	project, err := c.GetProject(ctx, projectID)
	if err != nil {
		return client.CodingAgentSelection{}, fmt.Errorf("获取当前项目失败: %w", err)
	}

	var args []string
	if request != nil && strings.TrimSpace(request.Runtime) != "" {
		args = append(args, strings.TrimSpace(request.Runtime))
		if strings.TrimSpace(request.Provider) != "" {
			args = append(args, strings.TrimSpace(request.Provider))
		}
		if strings.TrimSpace(request.Model) != "" {
			args = append(args, strings.TrimSpace(request.Model))
		}
	} else {
		stored := project.Settings.CodingAgent
		if strings.TrimSpace(stored.Runtime) != "" {
			args = append(args, strings.TrimSpace(stored.Runtime))
			if strings.TrimSpace(stored.Provider) != "" {
				args = append(args, strings.TrimSpace(stored.Provider))
			}
			if strings.TrimSpace(stored.Model) != "" {
				args = append(args, strings.TrimSpace(stored.Model))
			}
		} else if project.CodingAgentCatalog != nil {
			defaultSelection := project.CodingAgentCatalog.DefaultSelection
			if strings.TrimSpace(defaultSelection.Runtime) != "" {
				args = append(args, strings.TrimSpace(defaultSelection.Runtime))
				if strings.TrimSpace(defaultSelection.Provider) != "" {
					args = append(args, strings.TrimSpace(defaultSelection.Provider))
				}
				if strings.TrimSpace(defaultSelection.Model) != "" {
					args = append(args, strings.TrimSpace(defaultSelection.Model))
				}
			} else if strings.TrimSpace(project.CodingAgentCatalog.DefaultRuntime) != "" {
				args = append(args, strings.TrimSpace(project.CodingAgentCatalog.DefaultRuntime))
			}
		}
	}

	selection, err := resolveCodingAgentSelection(project, args)
	if err != nil {
		return client.CodingAgentSelection{}, err
	}
	if runtime := findProjectRuntime(project, selection.Runtime); runtime != nil && !runtime.Available {
		return client.CodingAgentSelection{}, fmt.Errorf("%s", formatRuntimeLoginGuidance(projectRuntimeToBridgeRuntime(runtime)))
	}
	return selection, nil
}

func handleAgentConfigCommand(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	parts := strings.Fields(strings.TrimSpace(args))
	if len(parts) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent config get | /agent config set <runtime> [provider] [model]")
		return
	}
	switch strings.ToLower(parts[0]) {
	case "get":
		projectID := strings.TrimSpace(c.ProjectScope())
		if projectID == "" {
			_ = p.Reply(ctx, msg.ReplyCtx, clientHintProjectScopeRequired())
			return
		}
		project, err := c.GetProject(ctx, projectID)
		if err != nil {
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Agent 配置失败: %v", err))
			return
		}
		_ = p.Reply(ctx, msg.ReplyCtx, formatAgentConfig(project))
	case "set":
		if len(parts) < 2 {
			_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent config set <runtime> [provider] [model]")
			return
		}
		projectID := strings.TrimSpace(c.ProjectScope())
		if projectID == "" {
			_ = p.Reply(ctx, msg.ReplyCtx, clientHintProjectScopeRequired())
			return
		}
		project, err := c.GetProject(ctx, projectID)
		if err != nil {
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取当前项目失败: %v", err))
			return
		}
		selection, err := resolveCodingAgentSelection(project, parts[1:])
		if err != nil {
			_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
			return
		}
		updated, err := c.UpdateProject(ctx, projectID, client.ProjectUpdateInput{
			Settings: &client.ProjectSettingsPatch{
				CodingAgent: &selection,
			},
		})
		if err != nil {
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("更新 Agent 配置失败: %v", err))
			return
		}
		_ = p.Reply(ctx, msg.ReplyCtx, "已更新当前项目代码 Agent 默认配置。\n"+formatAgentConfig(updated))
	default:
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent config get | /agent config set <runtime> [provider] [model]")
	}
}

func resolveCodingAgentSelection(project *client.Project, args []string) (client.CodingAgentSelection, error) {
	if len(args) == 0 {
		return client.CodingAgentSelection{}, fmt.Errorf("当前项目还没有默认代码 Agent 配置。先用 /agent config get 查看可用运行时，再用 /agent config set <runtime> [provider] [model] 设置。")
	}
	runtime := strings.TrimSpace(args[0])
	if runtime == "" {
		return client.CodingAgentSelection{}, fmt.Errorf("runtime 不能为空")
	}
	if project == nil || project.CodingAgentCatalog == nil {
		selection := client.CodingAgentSelection{Runtime: runtime}
		if len(args) > 1 {
			selection.Provider = strings.TrimSpace(args[1])
		}
		if len(args) > 2 {
			selection.Model = strings.TrimSpace(args[2])
		}
		return selection, nil
	}
	var selected *client.ProjectCodingAgentRuntime
	for i := range project.CodingAgentCatalog.Runtimes {
		if project.CodingAgentCatalog.Runtimes[i].Runtime == runtime {
			selected = &project.CodingAgentCatalog.Runtimes[i]
			break
		}
	}
	if selected == nil {
		return client.CodingAgentSelection{}, fmt.Errorf("找不到 runtime %q。先用 /agent config get 查看可用运行时。", runtime)
	}
	provider := selected.DefaultProvider
	if len(args) > 1 && strings.TrimSpace(args[1]) != "" {
		provider = strings.TrimSpace(args[1])
	}
	if provider != "" && len(selected.CompatibleProviders) > 0 && !slices.Contains(selected.CompatibleProviders, provider) {
		return client.CodingAgentSelection{}, fmt.Errorf("runtime %s 不支持 provider %s。可选值：%s", runtime, provider, strings.Join(selected.CompatibleProviders, ", "))
	}
	model := selected.DefaultModel
	if len(args) > 2 && strings.TrimSpace(args[2]) != "" {
		model = strings.TrimSpace(args[2])
	}
	if model != "" && len(selected.ModelOptions) > 0 && !slices.Contains(selected.ModelOptions, model) {
		return client.CodingAgentSelection{}, fmt.Errorf("runtime %s 不支持 model %s。可选值：%s", runtime, model, strings.Join(selected.ModelOptions, ", "))
	}
	return client.CodingAgentSelection{
		Runtime:  runtime,
		Provider: provider,
		Model:    model,
	}, nil
}

func findProjectRuntime(project *client.Project, runtime string) *client.ProjectCodingAgentRuntime {
	if project == nil || project.CodingAgentCatalog == nil {
		return nil
	}
	for i := range project.CodingAgentCatalog.Runtimes {
		if project.CodingAgentCatalog.Runtimes[i].Runtime == runtime {
			return &project.CodingAgentCatalog.Runtimes[i]
		}
	}
	return nil
}

func projectRuntimeToBridgeRuntime(runtime *client.ProjectCodingAgentRuntime) *client.BridgeRuntimeEntry {
	if runtime == nil {
		return nil
	}
	return &client.BridgeRuntimeEntry{
		Key:             runtime.Runtime,
		Label:           runtime.Label,
		DefaultProvider: runtime.DefaultProvider,
		DefaultModel:    runtime.DefaultModel,
		Available:       runtime.Available,
		Diagnostics:     runtime.Diagnostics,
	}
}

func formatAgentConfig(project *client.Project) string {
	if project == nil {
		return clientHintProjectScopeRequired()
	}
	lines := []string{
		fmt.Sprintf("项目: %s (%s)", project.Name, project.Slug),
		fmt.Sprintf("当前默认代码 Agent: %s", formatCodingAgentSelection(project.Settings.CodingAgent)),
	}
	if project.CodingAgentCatalog != nil {
		lines = append(lines, fmt.Sprintf("默认 runtime: %s", project.CodingAgentCatalog.DefaultRuntime))
		runtimeLines := make([]string, 0, len(project.CodingAgentCatalog.Runtimes))
		for _, runtime := range project.CodingAgentCatalog.Runtimes {
			status := "available"
			if !runtime.Available {
				status = "unavailable"
			}
			line := fmt.Sprintf("- %s (%s)", runtime.Runtime, status)
			if runtime.DefaultProvider != "" {
				line += " provider=" + runtime.DefaultProvider
			}
			if runtime.DefaultModel != "" {
				line += " model=" + runtime.DefaultModel
			}
			runtimeLines = append(runtimeLines, line)
		}
		if len(runtimeLines) > 0 {
			lines = append(lines, "可用运行时:")
			lines = append(lines, runtimeLines...)
		}
	}
	return strings.Join(lines, "\n")
}

func runAgentLifecycleAction(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, action, runID string) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/agent", action))
		return
	}

	var (
		run *client.AgentRunSummary
		err error
	)
	switch action {
	case "pause":
		run, err = c.PauseAgentRun(ctx, runID)
	case "resume":
		run, err = c.ResumeAgentRun(ctx, runID)
	case "kill":
		run, err = c.KillAgentRun(ctx, runID)
	default:
		_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/agent"))
		return
	}
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("更新 Agent 状态失败: %v", err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, formatAgentRunSummary(run))
}

func bindAgentDispatch(ctx context.Context, apiClient *client.AgentForgeClient, result *client.TaskDispatchResponse, msg *core.Message) {
	if apiClient == nil || result == nil || msg == nil || msg.ReplyTarget == nil {
		return
	}
	binding := client.IMActionBinding{
		Platform:    msg.Platform,
		ProjectID:   result.Task.ProjectID,
		TaskID:      result.Task.ID,
		ReplyTarget: msg.ReplyTarget,
	}
	if result.Dispatch.Run != nil {
		binding.RunID = result.Dispatch.Run.ID
	}
	_ = apiClient.BindActionContext(ctx, binding)
}

func buildAgentLogsStructuredMessage(logs []client.AgentLogEntry, runID string) *core.StructuredMessage {
	if len(logs) == 0 {
		return nil
	}
	limit := len(logs)
	if limit > 15 {
		limit = 15
	}
	var sb strings.Builder
	for _, entry := range logs[:limit] {
		icon := logLevelIcon(entry.Type)
		sb.WriteString(fmt.Sprintf("%s `[%s]` **%s** %s\n", icon, entry.Timestamp, entry.Type, entry.Content))
	}
	if len(logs) > 15 {
		sb.WriteString(fmt.Sprintf("\n... 还有 %d 条日志", len(logs)-15))
	}

	sections := []core.StructuredSection{
		{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: strings.TrimRight(sb.String(), "\n")},
		},
	}
	actions := []core.StructuredAction{
		{ID: "act:cmd:/agent status " + runID, Label: "查看状态", Style: core.ActionStyleDefault},
	}
	sections = append(sections, core.StructuredSection{
		Type:           core.StructuredSectionTypeActions,
		ActionsSection: &core.ActionsSection{Actions: actions},
	})

	return &core.StructuredMessage{
		Title:    fmt.Sprintf("Agent #%s 日志", shortID(runID)),
		Sections: sections,
	}
}

func logLevelIcon(logType string) string {
	switch strings.ToLower(strings.TrimSpace(logType)) {
	case "error", "fatal":
		return "[ERR]"
	case "warn", "warning":
		return "[WARN]"
	case "info":
		return "[INFO]"
	case "debug":
		return "[DBG]"
	default:
		return "[LOG]"
	}
}

func formatAgentLogs(logs []client.AgentLogEntry, runID string) string {
	if len(logs) == 0 {
		return fmt.Sprintf("Agent #%s 暂无日志", shortID(runID))
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent #%s 最近日志:\n", shortID(runID)))
	limit := len(logs)
	if limit > 15 {
		limit = 15
	}
	for _, entry := range logs[:limit] {
		sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", entry.Timestamp, entry.Type, entry.Content))
	}
	if len(logs) > 15 {
		sb.WriteString(fmt.Sprintf("  ... 还有 %d 条日志\n", len(logs)-15))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func buildAgentSpawnStructuredMessage(result *client.TaskDispatchResponse, requestedTaskID string) *core.StructuredMessage {
	if result == nil {
		return nil
	}
	fields := []core.StructuredField{
		{Label: "状态", Value: result.Dispatch.Status},
		{Label: "任务", Value: fmt.Sprintf("#%s %s", shortID(result.Task.ID), result.Task.Title)},
	}
	if result.Dispatch.Run != nil {
		fields = append(fields, core.StructuredField{Label: "Agent", Value: "#" + shortID(result.Dispatch.Run.ID)})
	}
	if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
		fields = append(fields, core.StructuredField{Label: "原因", Value: reason})
	}

	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{Fields: fields}},
	}

	actions := make([]core.StructuredAction, 0, 2)
	if result.Dispatch.Run != nil {
		actions = append(actions, core.StructuredAction{
			ID: "act:cmd:/agent logs " + result.Dispatch.Run.ID, Label: "查看日志", Style: core.ActionStyleDefault,
		})
		if result.Dispatch.Status == "started" {
			actions = append(actions, core.StructuredAction{
				ID: "act:cmd:/agent kill " + result.Dispatch.Run.ID, Label: "终止", Style: core.ActionStyleDanger,
			})
		}
	}
	if len(actions) > 0 {
		sections = append(sections, core.StructuredSection{
			Type:           core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{Actions: actions},
		})
	}

	title := "Agent 派发结果"
	switch result.Dispatch.Status {
	case "started":
		title = "Agent 已启动"
	case "queued":
		title = "已加入队列"
	case "blocked":
		title = "派发被阻止"
	}
	return &core.StructuredMessage{Title: title, Sections: sections}
}

func formatAgentSpawnReply(result *client.TaskDispatchResponse, requestedTaskID string) string {
	switch result.Dispatch.Status {
	case "started":
		if result.Dispatch.Run != nil {
			return fmt.Sprintf("已启动 Agent #%s 执行任务 %s",
				shortID(result.Dispatch.Run.ID), shortID(result.Dispatch.Run.TaskID))
		}
		return fmt.Sprintf("已启动 Agent 执行任务 %s", shortID(requestedTaskID))
	case "skipped":
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("任务 %s 本次未启动 Agent：%s", shortID(requestedTaskID), reason)
		}
		return fmt.Sprintf("任务 %s 本次未启动 Agent", shortID(requestedTaskID))
	case "blocked":
		if result.Dispatch.GuardrailType == "budget" && strings.TrimSpace(result.Dispatch.GuardrailScope) != "" {
			return fmt.Sprintf("未启动 Agent：%s budget blocked dispatch: %s", strings.TrimSpace(result.Dispatch.GuardrailScope), defaultDispatchReplyReason(result.Dispatch.Reason, "budget guardrail"))
		}
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("未启动 Agent：%s", reason)
		}
		return "未启动 Agent"
	default:
		if result.Dispatch.Status == "queued" {
			prefix := "已加入队列"
			if result.Dispatch.Queue != nil && result.Dispatch.Queue.RecoveryDisposition == "recoverable" {
				prefix = "仍在队列中等待恢复"
			}
			if result.Dispatch.Queue != nil {
				parts := []string{prefix, "#" + shortID(result.Dispatch.Queue.EntryID)}
				if result.Dispatch.Queue.Priority > 0 {
					parts = append(parts, fmt.Sprintf("优先级 %d", result.Dispatch.Queue.Priority))
				}
				if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
					return strings.Join(parts, " ") + "：" + reason
				}
				return strings.Join(parts, " ")
			}
			if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
				return fmt.Sprintf("已加入队列：%s", reason)
			}
		}
		return fmt.Sprintf("任务 %s 当前未启动 Agent", shortID(requestedTaskID))
	}
}

func formatAgentSpawnReplyWithBridgeTools(ctx context.Context, c *client.AgentForgeClient, result *client.TaskDispatchResponse, requestedTaskID string) string {
	reply := formatAgentSpawnReply(result, requestedTaskID)
	if c == nil || result == nil || result.Dispatch.Status != "started" {
		return reply
	}
	tools, err := c.ListBridgeTools(ctx)
	if err != nil || len(tools) == 0 {
		return reply
	}
	return reply + "\n" + formatSpawnBridgeTools(tools)
}

func formatSpawnBridgeTools(tools []client.BridgeTool) string {
	if len(tools) == 0 {
		return ""
	}
	limit := len(tools)
	if limit > 3 {
		limit = 3
	}
	parts := make([]string, 0, limit)
	for _, tool := range tools[:limit] {
		if id := strings.TrimSpace(tool.PluginID); id != "" {
			parts = append(parts, id)
			continue
		}
		if name := strings.TrimSpace(tool.Name); name != "" {
			parts = append(parts, name)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "Bridge tools: " + strings.Join(parts, ", ")
}

func defaultDispatchReplyReason(reason string, fallback string) string {
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		return trimmed
	}
	return fallback
}

func formatAgentPoolStatus(status *client.PoolStatus) string {
	if status == nil {
		return "Agent 池状态不可用"
	}
	active := status.ActiveAgents
	if active == 0 {
		active = status.Active
	}
	max := status.MaxAgents
	if max == 0 {
		max = status.Max
	}
	return fmt.Sprintf(
		"Agent 池状态: %d/%d 活跃，可用 %d，排队 %d，可恢复 %d",
		active,
		max,
		status.Available,
		status.Queued,
		status.PausedResumable,
	)
}

func buildAgentPoolStructuredMessage(status *client.PoolStatus, bridgePool *client.BridgePoolStatus) *core.StructuredMessage {
	if status == nil {
		return nil
	}
	active := status.ActiveAgents
	if active == 0 {
		active = status.Active
	}
	max := status.MaxAgents
	if max == 0 {
		max = status.Max
	}

	fields := []core.StructuredField{
		{Label: "活跃", Value: fmt.Sprintf("%d / %d", active, max)},
		{Label: "可用", Value: fmt.Sprintf("%d", status.Available)},
		{Label: "排队", Value: fmt.Sprintf("%d", status.Queued)},
		{Label: "可恢复", Value: fmt.Sprintf("%d", status.PausedResumable)},
	}
	if bridgePool != nil {
		fields = append(fields,
			core.StructuredField{Label: "Bridge Active", Value: fmt.Sprintf("%d / %d", bridgePool.Active, bridgePool.Max)},
			core.StructuredField{Label: "Bridge Warm", Value: fmt.Sprintf("%d / %d", bridgePool.WarmAvailable, bridgePool.WarmTotal)},
		)
	}

	return &core.StructuredMessage{
		Title: "Agent 池状态",
		Sections: []core.StructuredSection{
			{
				Type:          core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{Fields: fields},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{
						{ID: "act:cmd:/queue list", Label: "查看队列", Style: core.ActionStyleDefault},
						{ID: "act:cmd:/agent runtimes", Label: "Runtimes", Style: core.ActionStyleDefault},
					},
				},
			},
		},
	}
}

func formatAgentPoolStatusWithBridge(status *client.PoolStatus, bridgePool *client.BridgePoolStatus) string {
	base := formatAgentPoolStatus(status)
	if bridgePool == nil {
		return base
	}
	return fmt.Sprintf(
		"%s\nBridge pool: %d/%d active, warm %d/%d",
		base,
		bridgePool.Active,
		bridgePool.Max,
		bridgePool.WarmAvailable,
		bridgePool.WarmTotal,
	)
}

func buildRuntimeCatalogStructuredMessage(catalog *client.BridgeRuntimeCatalog) *core.StructuredMessage {
	if catalog == nil || len(catalog.Runtimes) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(catalog.Runtimes))
	for _, runtime := range catalog.Runtimes {
		status := "available"
		if !runtime.Available {
			status = "unavailable"
		}
		label := fmt.Sprintf("%s [%s]", runtime.Key, status)
		value := fmt.Sprintf("%s / %s", runtime.DefaultProvider, runtime.DefaultModel)
		fields = append(fields, core.StructuredField{Label: label, Value: value})
	}
	return &core.StructuredMessage{
		Title: fmt.Sprintf("Bridge Runtimes (default=%s)", catalog.DefaultRuntime),
		Sections: []core.StructuredSection{
			{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{Fields: fields}},
			{
				Type:           core.StructuredSectionTypeContext,
				ContextSection: &core.ContextSection{Elements: []string{"使用 /login <runtime> 查看登录指引"}},
			},
		},
	}
}

func buildBridgeHealthStructuredMessage(health *client.BridgeHealthStatus) *core.StructuredMessage {
	if health == nil {
		return nil
	}
	return &core.StructuredMessage{
		Title: "Bridge Health",
		Sections: []core.StructuredSection{
			{
				Type: core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{
					Fields: []core.StructuredField{
						{Label: "状态", Value: health.Status},
						{Label: "Active", Value: fmt.Sprintf("%d", health.Pool.Active)},
						{Label: "Available", Value: fmt.Sprintf("%d", health.Pool.Available)},
						{Label: "Warm", Value: fmt.Sprintf("%d", health.Pool.Warm)},
					},
				},
			},
		},
	}
}

func formatBridgeRuntimeCatalog(catalog *client.BridgeRuntimeCatalog) string {
	if catalog == nil {
		return "Bridge runtimes unavailable"
	}
	lines := []string{fmt.Sprintf("Bridge runtimes (default=%s):", catalog.DefaultRuntime)}
	for _, runtime := range catalog.Runtimes {
		lines = append(lines, fmt.Sprintf("- %s [%t] provider=%s model=%s", runtime.Key, runtime.Available, runtime.DefaultProvider, runtime.DefaultModel))
	}
	return strings.Join(lines, "\n")
}

func formatBridgeHealthStatus(health *client.BridgeHealthStatus) string {
	if health == nil {
		return "Bridge health unavailable"
	}
	return fmt.Sprintf(
		"Bridge health: %s | active=%d available=%d warm=%d",
		health.Status,
		health.Pool.Active,
		health.Pool.Available,
		health.Pool.Warm,
	)
}

func formatAgentRunSummary(run *client.AgentRunSummary) string {
	if run == nil {
		return "Agent 运行状态不可用"
	}
	parts := []string{
		fmt.Sprintf("Agent #%s", shortID(run.ID)),
		fmt.Sprintf("[%s] %s", run.Status, run.TaskTitle),
	}
	if strings.TrimSpace(run.Runtime) != "" {
		parts = append(parts, fmt.Sprintf("runtime=%s", run.Runtime))
	}
	if strings.TrimSpace(run.Provider) != "" {
		parts = append(parts, fmt.Sprintf("provider=%s", run.Provider))
	}
	if strings.TrimSpace(run.Model) != "" {
		parts = append(parts, fmt.Sprintf("model=%s", run.Model))
	}
	if run.CanResume {
		parts = append(parts, "可恢复")
	}
	return strings.Join(parts, " | ")
}
