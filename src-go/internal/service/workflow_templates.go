package service

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// System template names — used as stable identifiers for upsert.
const (
	TemplatePlanCodeReview  = "plan-code-review"
	TemplatePipeline        = "pipeline"
	TemplateSwarm           = "swarm"
	TemplateContentCreation = "content-creation"
	TemplateCustomerService = "customer-service"
)

// buildNodes / buildEdges are helpers to avoid repetitive json.Marshal calls.
func buildNodes(nodes []model.WorkflowNode) json.RawMessage {
	b, _ := json.Marshal(nodes)
	return b
}
func buildEdges(edges []model.WorkflowEdge) json.RawMessage {
	b, _ := json.Marshal(edges)
	return b
}
func buildConfig(cfg map[string]any) json.RawMessage {
	b, _ := json.Marshal(cfg)
	return b
}

// PlanCodeReviewTemplate returns the default coding workflow:
//
//	trigger → planner (llm_agent) → fan-out function → parallel_split
//	  → coder (llm_agent, dynamic) → parallel_join → reviewer (llm_agent)
func PlanCodeReviewTemplate() *model.WorkflowDefinition {
	nodes := []model.WorkflowNode{
		{ID: "trigger", Type: model.NodeTypeTrigger, Label: "Start", Position: model.WorkflowPos{X: 0, Y: 200}},
		{ID: "planner", Type: model.NodeTypeLLMAgent, Label: "Planner", Position: model.WorkflowPos{X: 250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Analyze the task and create a structured plan with subtasks.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
				"roleId":   "planner",
			})},
		{ID: "fan_out", Type: model.NodeTypeFunction, Label: "Create subtasks", Position: model.WorkflowPos{X: 500, Y: 200},
			Config: buildConfig(map[string]any{
				"expression": "{{planner.output.subtasks}}",
			})},
		{ID: "split", Type: model.NodeTypeParallelSplit, Label: "Parallel", Position: model.WorkflowPos{X: 750, Y: 200}},
		{ID: "coder", Type: model.NodeTypeLLMAgent, Label: "Coder", Position: model.WorkflowPos{X: 1000, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Implement the assigned subtask according to the plan.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 5.0,
				"roleId":   "coder",
			})},
		{ID: "join", Type: model.NodeTypeParallelJoin, Label: "Join", Position: model.WorkflowPos{X: 1250, Y: 200}},
		{ID: "reviewer", Type: model.NodeTypeLLMAgent, Label: "Reviewer", Position: model.WorkflowPos{X: 1500, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Review all code changes and provide feedback.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
				"roleId":   "reviewer",
			})},
	}
	edges := []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "planner"},
		{ID: "e2", Source: "planner", Target: "fan_out"},
		{ID: "e3", Source: "fan_out", Target: "split"},
		{ID: "e4", Source: "split", Target: "coder"},
		{ID: "e5", Source: "coder", Target: "join"},
		{ID: "e6", Source: "join", Target: "reviewer"},
	}
	templateVars, _ := json.Marshal(map[string]any{
		"runtime":  "claude_code",
		"provider": "anthropic",
		"model":    "claude-sonnet-4-20250514",
	})
	return &model.WorkflowDefinition{
		ID:           uuid.New(),
		Name:         TemplatePlanCodeReview,
		Description:  "Plan, code in parallel, then review. The default software development workflow.",
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategorySystem,
		Nodes:        buildNodes(nodes),
		Edges:        buildEdges(edges),
		TemplateVars: templateVars,
		Version:      1,
	}
}

// PipelineTemplate returns a sequential coding workflow:
//
//	trigger → planner → coder1 → coder2 → … → reviewer
func PipelineTemplate() *model.WorkflowDefinition {
	nodes := []model.WorkflowNode{
		{ID: "trigger", Type: model.NodeTypeTrigger, Label: "Start", Position: model.WorkflowPos{X: 0, Y: 200}},
		{ID: "planner", Type: model.NodeTypeLLMAgent, Label: "Planner", Position: model.WorkflowPos{X: 250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Analyze the task and create an ordered list of implementation steps.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
				"roleId":   "planner",
			})},
		{ID: "coder", Type: model.NodeTypeLLMAgent, Label: "Coder (sequential)", Position: model.WorkflowPos{X: 500, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Implement the next step based on the plan and previous work.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 5.0,
				"roleId":   "coder",
			})},
		{ID: "reviewer", Type: model.NodeTypeLLMAgent, Label: "Reviewer", Position: model.WorkflowPos{X: 750, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Review all changes from the pipeline.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
				"roleId":   "reviewer",
			})},
	}
	edges := []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "planner"},
		{ID: "e2", Source: "planner", Target: "coder"},
		{ID: "e3", Source: "coder", Target: "reviewer"},
	}
	templateVars, _ := json.Marshal(map[string]any{
		"runtime":  "claude_code",
		"provider": "anthropic",
		"model":    "claude-sonnet-4-20250514",
	})
	return &model.WorkflowDefinition{
		ID:           uuid.New(),
		Name:         TemplatePipeline,
		Description:  "Sequential pipeline: plan, then code steps one by one, then review.",
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategorySystem,
		Nodes:        buildNodes(nodes),
		Edges:        buildEdges(edges),
		TemplateVars: templateVars,
		Version:      1,
	}
}

// SwarmTemplate returns an aggressive parallel coding workflow:
//
//	trigger → planner → parallel_split → all coders → parallel_join → reviewer
func SwarmTemplate() *model.WorkflowDefinition {
	// Identical structure to PlanCodeReview but described as "swarm" mode
	nodes := []model.WorkflowNode{
		{ID: "trigger", Type: model.NodeTypeTrigger, Label: "Start", Position: model.WorkflowPos{X: 0, Y: 200}},
		{ID: "planner", Type: model.NodeTypeLLMAgent, Label: "Planner", Position: model.WorkflowPos{X: 250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Break down the task into independent subtasks for maximum parallelism.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
				"roleId":   "planner",
			})},
		{ID: "split", Type: model.NodeTypeParallelSplit, Label: "Swarm", Position: model.WorkflowPos{X: 500, Y: 200}},
		{ID: "coder", Type: model.NodeTypeLLMAgent, Label: "Coder (parallel)", Position: model.WorkflowPos{X: 750, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Implement the assigned subtask independently.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 5.0,
				"roleId":   "coder",
			})},
		{ID: "join", Type: model.NodeTypeParallelJoin, Label: "Join", Position: model.WorkflowPos{X: 1000, Y: 200}},
		{ID: "reviewer", Type: model.NodeTypeLLMAgent, Label: "Reviewer", Position: model.WorkflowPos{X: 1250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Review all parallel changes for consistency and correctness.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
				"roleId":   "reviewer",
			})},
	}
	edges := []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "planner"},
		{ID: "e2", Source: "planner", Target: "split"},
		{ID: "e3", Source: "split", Target: "coder"},
		{ID: "e4", Source: "coder", Target: "join"},
		{ID: "e5", Source: "join", Target: "reviewer"},
	}
	templateVars, _ := json.Marshal(map[string]any{
		"runtime":  "claude_code",
		"provider": "anthropic",
		"model":    "claude-sonnet-4-20250514",
	})
	return &model.WorkflowDefinition{
		ID:           uuid.New(),
		Name:         TemplateSwarm,
		Description:  "Swarm mode: plan then execute all subtasks in parallel, then review.",
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategorySystem,
		Nodes:        buildNodes(nodes),
		Edges:        buildEdges(edges),
		TemplateVars: templateVars,
		Version:      1,
	}
}

// ContentCreationTemplate returns a content creation pipeline:
//
//	trigger → research → outline → writer ↔ editor (loop max 3) → SEO → notification
func ContentCreationTemplate() *model.WorkflowDefinition {
	nodes := []model.WorkflowNode{
		{ID: "trigger", Type: model.NodeTypeTrigger, Label: "Start", Position: model.WorkflowPos{X: 0, Y: 200}},
		{ID: "research", Type: model.NodeTypeLLMAgent, Label: "Topic Research", Position: model.WorkflowPos{X: 250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Research the topic {{topic}} and produce key insights, data points, and angles.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 1.0,
			})},
		{ID: "outline", Type: model.NodeTypeLLMAgent, Label: "Outline", Position: model.WorkflowPos{X: 500, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Create a detailed outline based on research: {{research.output}}",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 1.0,
			})},
		{ID: "writer", Type: model.NodeTypeLLMAgent, Label: "Writer", Position: model.WorkflowPos{X: 750, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Write the full content based on the outline: {{outline.output}}",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 2.0,
			})},
		{ID: "editor", Type: model.NodeTypeLLMAgent, Label: "Editor", Position: model.WorkflowPos{X: 1000, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Edit and improve the draft: {{writer.output}}. Provide specific feedback.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 1.0,
			})},
		{ID: "edit_loop", Type: model.NodeTypeLoop, Label: "Revision loop", Position: model.WorkflowPos{X: 1000, Y: 400},
			Config: buildConfig(map[string]any{
				"target_node":    "writer",
				"max_iterations": 3,
				"exit_condition": "{{editor.output.approved}} == true",
			})},
		{ID: "seo", Type: model.NodeTypeLLMAgent, Label: "SEO Optimization", Position: model.WorkflowPos{X: 1250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Optimize the final content for SEO: {{writer.output}}",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 1.0,
			})},
		{ID: "done", Type: model.NodeTypeNotification, Label: "Content Ready", Position: model.WorkflowPos{X: 1500, Y: 200},
			Config: buildConfig(map[string]any{
				"message": "Content creation workflow completed.",
			})},
	}
	edges := []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "research"},
		{ID: "e2", Source: "research", Target: "outline"},
		{ID: "e3", Source: "outline", Target: "writer"},
		{ID: "e4", Source: "writer", Target: "editor"},
		{ID: "e5", Source: "editor", Target: "edit_loop"},
		{ID: "e6", Source: "edit_loop", Target: "seo"},
	}
	templateVars, _ := json.Marshal(map[string]any{
		"runtime":  "claude_code",
		"provider": "anthropic",
		"model":    "claude-sonnet-4-20250514",
		"topic":    "",
	})
	return &model.WorkflowDefinition{
		ID:           uuid.New(),
		Name:         TemplateContentCreation,
		Description:  "Content creation pipeline with research, writing, editing loop, and SEO optimization.",
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategorySystem,
		Nodes:        buildNodes(nodes),
		Edges:        buildEdges(edges),
		TemplateVars: templateVars,
		Version:      1,
	}
}

// CustomerServiceTemplate returns a customer service triage workflow:
//
//	trigger → classify → condition(urgent?) → [urgent: human_review] / [normal: auto_reply → notification]
func CustomerServiceTemplate() *model.WorkflowDefinition {
	nodes := []model.WorkflowNode{
		{ID: "trigger", Type: model.NodeTypeTrigger, Label: "Ticket received", Position: model.WorkflowPos{X: 0, Y: 200}},
		{ID: "classify", Type: model.NodeTypeLLMAgent, Label: "Classify & Analyze", Position: model.WorkflowPos{X: 250, Y: 200},
			Config: buildConfig(map[string]any{
				"prompt":   "Classify the customer inquiry. Determine urgency (score 0-1) and category. Input: {{trigger.output}}",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 0.5,
			})},
		{ID: "urgent_check", Type: model.NodeTypeCondition, Label: "Urgent?", Position: model.WorkflowPos{X: 500, Y: 200},
			Config: buildConfig(map[string]any{
				"expression": "{{classify.output.urgency}} > 0.7",
			})},
		{ID: "human_review", Type: model.NodeTypeHumanReview, Label: "Escalate to human", Position: model.WorkflowPos{X: 750, Y: 100},
			Config: buildConfig(map[string]any{
				"prompt": "Urgent ticket requires human attention. Category: {{classify.output.category}}",
			})},
		{ID: "auto_reply", Type: model.NodeTypeLLMAgent, Label: "Draft auto-reply", Position: model.WorkflowPos{X: 750, Y: 300},
			Config: buildConfig(map[string]any{
				"prompt":   "Draft a helpful response for this {{classify.output.category}} inquiry.",
				"runtime":  "{{runtime}}",
				"provider": "{{provider}}",
				"model":    "{{model}}",
				"budgetUsd": 0.5,
			})},
		{ID: "done", Type: model.NodeTypeNotification, Label: "Resolved", Position: model.WorkflowPos{X: 1000, Y: 200},
			Config: buildConfig(map[string]any{
				"message": "Customer service ticket processed.",
			})},
	}
	edges := []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "classify"},
		{ID: "e2", Source: "classify", Target: "urgent_check"},
		{ID: "e3", Source: "urgent_check", Target: "human_review", Condition: "{{classify.output.urgency}} > 0.7", Label: "Urgent"},
		{ID: "e4", Source: "urgent_check", Target: "auto_reply", Condition: "{{classify.output.urgency}} <= 0.7", Label: "Normal"},
		{ID: "e5", Source: "human_review", Target: "done"},
		{ID: "e6", Source: "auto_reply", Target: "done"},
	}
	templateVars, _ := json.Marshal(map[string]any{
		"runtime":  "claude_code",
		"provider": "anthropic",
		"model":    "claude-sonnet-4-20250514",
	})
	return &model.WorkflowDefinition{
		ID:           uuid.New(),
		Name:         TemplateCustomerService,
		Description:  "Customer service triage: classify, route urgent to humans, auto-reply to normal.",
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategorySystem,
		Nodes:        buildNodes(nodes),
		Edges:        buildEdges(edges),
		TemplateVars: templateVars,
		Version:      1,
	}
}

// AllSystemTemplates returns all built-in system templates.
func AllSystemTemplates() []*model.WorkflowDefinition {
	return []*model.WorkflowDefinition{
		PlanCodeReviewTemplate(),
		PipelineTemplate(),
		SwarmTemplate(),
		ContentCreationTemplate(),
		CustomerServiceTemplate(),
	}
}
