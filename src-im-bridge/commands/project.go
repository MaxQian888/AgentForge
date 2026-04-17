package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterProjectCommands registers /project sub-commands on the engine.
func RegisterProjectCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/project", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/project"))
			return
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch canonicalSubcommand("/project", parts[0]) {
		case "list":
			projects, err := scopedClient.ListProjects(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取项目列表失败: %v", err))
				return
			}
			if sm := buildProjectListStructuredMessage(projects, scopedClient.ProjectScope()); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectList(projects, scopedClient.ProjectScope()))
		case "current":
			projectID := scopedClient.ProjectScope()
			if strings.TrimSpace(projectID) == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, clientHintProjectScopeRequired())
				return
			}
			project, err := scopedClient.GetProject(ctx, projectID)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取当前项目失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatCurrentProject(project))
		case "info":
			project, err := resolveProjectForCommand(ctx, scopedClient, parts)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectInfo(project))
		case "members":
			project, err := resolveProjectForCommand(ctx, scopedClient, parts)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			members, err := scopedClient.WithProjectScope(project.ID).ListProjectMembers(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取项目成员失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectMembers(project, members))
		case "set":
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/project", "set"))
				return
			}
			projects, err := scopedClient.ListProjects(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取项目列表失败: %v", err))
				return
			}
			project, err := resolveProjectSelection(projects, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			// Mutate the underlying factory client so the scope persists
			// across subsequent messages. In legacy single-client mode
			// factory.For(_) returns the process-wide client; in multi-
			// tenant mode it returns a per-tenant clone whose mutation is
			// correctly ephemeral (tenant projectId is authoritative).
			factory.For(msg.TenantID).SetProjectScope(project.ID)
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectScopeSet(project))
		case "create":
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/project", "create"))
				return
			}
			if err := requireProjectAdmin(ctx, scopedClient, msg, "Admin role required for project creation"); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			project, err := scopedClient.CreateProject(ctx, client.CreateProjectInput{
				Name: strings.TrimSpace(parts[1]),
				Slug: toProjectSlug(parts[1]),
			})
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("创建项目失败: %v", err))
				return
			}
			// Mutate the underlying factory client so the scope persists
			// across subsequent messages. In legacy single-client mode
			// factory.For(_) returns the process-wide client; in multi-
			// tenant mode it returns a per-tenant clone whose mutation is
			// correctly ephemeral (tenant projectId is authoritative).
			factory.For(msg.TenantID).SetProjectScope(project.ID)
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectCreated(project))
		case "rename":
			tokens := strings.Fields(strings.TrimSpace(parts[1]))
			if len(tokens) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/project", "rename"))
				return
			}
			if err := requireProjectAdmin(ctx, scopedClient, msg, "Admin role required for project rename"); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			projects, err := scopedClient.ListProjects(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取项目列表失败: %v", err))
				return
			}
			project, err := resolveProjectSelection(projects, tokens[0])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			newName := strings.TrimSpace(strings.Join(tokens[1:], " "))
			updated, err := scopedClient.UpdateProject(ctx, project.ID, client.ProjectUpdateInput{
				Name: &newName,
			})
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("重命名项目失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectRenamed(updated))
		case "delete":
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/project", "delete"))
				return
			}
			if err := requireProjectAdmin(ctx, scopedClient, msg, "Admin role required for project deletion"); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			projects, err := scopedClient.ListProjects(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取项目列表失败: %v", err))
				return
			}
			project, err := resolveProjectSelection(projects, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			if err := scopedClient.DeleteProject(ctx, project.ID); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("删除项目失败: %v", err))
				return
			}
			if factory.For(msg.TenantID).ProjectScope() == project.ID {
				factory.For(msg.TenantID).SetProjectScope("")
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatProjectDeleted(project))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/project"))
		}
	})
}

func clientHintProjectScopeRequired() string {
	return "当前未设置 project。先用 /project list 查看项目，再用 /project set <project-id|slug> 选择项目。"
}

func resolveProjectSelection(projects []client.Project, raw string) (*client.Project, error) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return nil, fmt.Errorf("%s", clientHintProjectScopeRequired())
	}
	lowerQuery := strings.ToLower(query)
	var exactMatches []*client.Project
	var fuzzyMatches []*client.Project
	for i := range projects {
		project := &projects[i]
		switch {
		case project.ID == query:
			exactMatches = append(exactMatches, project)
		case strings.EqualFold(strings.TrimSpace(project.Slug), query):
			exactMatches = append(exactMatches, project)
		case strings.EqualFold(strings.TrimSpace(project.Name), query):
			exactMatches = append(exactMatches, project)
		case strings.Contains(strings.ToLower(project.Name), lowerQuery), strings.Contains(strings.ToLower(project.Slug), lowerQuery):
			fuzzyMatches = append(fuzzyMatches, project)
		}
	}
	switch {
	case len(exactMatches) == 1:
		return exactMatches[0], nil
	case len(exactMatches) > 1:
		return nil, fmt.Errorf("匹配到多个项目，请改用更精确的 project id 或 slug。")
	case len(fuzzyMatches) == 1:
		return fuzzyMatches[0], nil
	case len(fuzzyMatches) > 1:
		names := make([]string, 0, len(fuzzyMatches))
		for _, project := range fuzzyMatches {
			names = append(names, fmt.Sprintf("%s(%s)", project.Name, project.Slug))
		}
		return nil, fmt.Errorf("匹配到多个项目：%s", strings.Join(names, "、"))
	default:
		return nil, fmt.Errorf("找不到项目 %q。先用 /project list 查看可用项目。", query)
	}
}

func buildProjectListStructuredMessage(projects []client.Project, currentProjectID string) *core.StructuredMessage {
	if len(projects) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(projects))
	for _, project := range projects {
		label := project.Name
		if strings.TrimSpace(project.ID) != "" && project.ID == strings.TrimSpace(currentProjectID) {
			label = "* " + label
		}
		fields = append(fields, core.StructuredField{Label: label, Value: project.Slug})
	}
	sections := []core.StructuredSection{
		{
			Type:          core.StructuredSectionTypeFields,
			FieldsSection: &core.FieldsSection{Fields: fields},
		},
		{
			Type:           core.StructuredSectionTypeContext,
			ContextSection: &core.ContextSection{Elements: []string{"使用 /project set <slug> 切换项目"}},
		},
	}
	return &core.StructuredMessage{
		Title:    fmt.Sprintf("项目列表 (%d)", len(projects)),
		Sections: sections,
	}
}

func formatProjectList(projects []client.Project, currentProjectID string) string {
	if len(projects) == 0 {
		return "当前没有可用项目"
	}
	lines := []string{fmt.Sprintf("项目列表 (%d):", len(projects))}
	for _, project := range projects {
		marker := " "
		if strings.TrimSpace(project.ID) != "" && project.ID == strings.TrimSpace(currentProjectID) {
			marker = "*"
		}
		lines = append(lines, fmt.Sprintf("%s %s (%s)", marker, project.Name, project.Slug))
		lines = append(lines, fmt.Sprintf("  id: %s", project.ID))
	}
	lines = append(lines, "使用 /project set <project-id|slug> 切换当前项目")
	return strings.Join(lines, "\n")
}

func formatCurrentProject(project *client.Project) string {
	if project == nil {
		return clientHintProjectScopeRequired()
	}
	return strings.Join([]string{
		fmt.Sprintf("当前项目: %s (%s)", project.Name, project.Slug),
		fmt.Sprintf("id: %s", project.ID),
		fmt.Sprintf("默认代码 Agent: %s", formatCodingAgentSelection(project.Settings.CodingAgent)),
	}, "\n")
}

func formatProjectInfo(project *client.Project) string {
	if project == nil {
		return clientHintProjectScopeRequired()
	}
	lines := []string{
		fmt.Sprintf("项目: %s (%s)", project.Name, project.Slug),
		fmt.Sprintf("id: %s", project.ID),
		fmt.Sprintf("默认代码 Agent: %s", formatCodingAgentSelection(project.Settings.CodingAgent)),
	}
	if trimmed := strings.TrimSpace(project.Description); trimmed != "" {
		lines = append(lines, "描述: "+trimmed)
	}
	if trimmed := strings.TrimSpace(project.RepoURL); trimmed != "" {
		lines = append(lines, "仓库: "+trimmed)
	}
	if trimmed := strings.TrimSpace(project.DefaultBranch); trimmed != "" {
		lines = append(lines, "默认分支: "+trimmed)
	}
	return strings.Join(lines, "\n")
}

func formatProjectScopeSet(project *client.Project) string {
	if project == nil {
		return clientHintProjectScopeRequired()
	}
	return strings.Join([]string{
		fmt.Sprintf("已切换到项目: %s (%s)", project.Name, project.Slug),
		fmt.Sprintf("id: %s", project.ID),
		fmt.Sprintf("默认代码 Agent: %s", formatCodingAgentSelection(project.Settings.CodingAgent)),
		"现在可以直接使用 /task、/agent、/review 等 project 相关命令了。",
	}, "\n")
}

func formatProjectCreated(project *client.Project) string {
	if project == nil {
		return "项目创建成功"
	}
	return strings.Join([]string{
		fmt.Sprintf("已创建项目: %s (%s)", project.Name, project.Slug),
		fmt.Sprintf("id: %s", project.ID),
		fmt.Sprintf("默认代码 Agent: %s", formatCodingAgentSelection(project.Settings.CodingAgent)),
		"当前 project 已自动切换到新项目。",
	}, "\n")
}

func formatProjectDeleted(project *client.Project) string {
	if project == nil {
		return "项目已删除"
	}
	return fmt.Sprintf("已删除项目: %s (%s)", project.Name, project.Slug)
}

func formatProjectRenamed(project *client.Project) string {
	if project == nil {
		return "项目已重命名"
	}
	return fmt.Sprintf("已重命名项目: %s (%s)", project.Name, project.Slug)
}

func formatProjectMembers(project *client.Project, members []client.Member) string {
	header := "项目成员"
	if project != nil {
		header = fmt.Sprintf("项目成员: %s (%s)", project.Name, project.Slug)
	}
	if len(members) == 0 {
		return header + "\n当前没有成员"
	}
	return header + "\n" + formatTeamMembers(members)
}

func formatCodingAgentSelection(selection client.CodingAgentSelection) string {
	parts := make([]string, 0, 3)
	if trimmed := strings.TrimSpace(selection.Runtime); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(selection.Provider); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if trimmed := strings.TrimSpace(selection.Model); trimmed != "" {
		parts = append(parts, trimmed)
	}
	if len(parts) == 0 {
		return "未配置"
	}
	return strings.Join(parts, " / ")
}

func requireProjectAdmin(ctx context.Context, c *client.AgentForgeClient, msg *core.Message, message string) error {
	member, err := resolveToolsOperator(ctx, c, msg)
	if err != nil {
		return err
	}
	if member == nil || !isToolsAdminRole(member.Role) {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func resolveProjectForCommand(ctx context.Context, c *client.AgentForgeClient, parts []string) (*client.Project, error) {
	if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
		projectID := c.ProjectScope()
		if strings.TrimSpace(projectID) == "" {
			return nil, fmt.Errorf("%s", clientHintProjectScopeRequired())
		}
		project, err := c.GetProject(ctx, projectID)
		if err != nil {
			return nil, fmt.Errorf("获取当前项目失败: %w", err)
		}
		return project, nil
	}
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取项目列表失败: %w", err)
	}
	project, err := resolveProjectSelection(projects, parts[1])
	if err != nil {
		return nil, err
	}
	fullProject, err := c.GetProject(ctx, project.ID)
	if err != nil {
		return nil, fmt.Errorf("获取项目失败: %w", err)
	}
	return fullProject, nil
}

func toProjectSlug(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	normalized = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_' || r == '.':
			return '-'
		default:
			return -1
		}
	}, normalized)
	normalized = strings.Trim(strings.Join(strings.FieldsFunc(normalized, func(r rune) bool { return r == '-' }), "-"), "-")
	if normalized == "" {
		return "project"
	}
	return normalized
}
