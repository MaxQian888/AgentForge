// Package system defines canonical system workflow DAG definitions that are
// seeded at server startup.
package system

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// TemplateCodeFixer is the stable name for the code_fixer DAG template.
const TemplateCodeFixer = "code_fixer"

// CodeFixerNodes are the canonical node IDs in the code_fixer DAG.
var CodeFixerNodes = []string{
	"trigger",
	"fetch_file",
	"has_prebaked",
	"generate",
	"validate",
	"decide",
	"execute",
	"update_original_pr",
	"card",
}

func buildCfg(cfg map[string]any) json.RawMessage {
	b, _ := json.Marshal(cfg)
	return b
}

// CodeFixerDefinition returns the canonical code_fixer DAG definition.
func CodeFixerDefinition() *model.WorkflowDefinition {
	nodes := []model.WorkflowNode{
		{ID: "trigger", Type: model.NodeTypeTrigger, Label: "Start",
			Config: buildCfg(map[string]any{
				"inputs": []string{"review_id", "finding_id", "integration_id", "head_sha", "employee_id"},
			})},
		{ID: "fetch_file", Type: model.NodeTypeHTTPCall, Label: "Fetch source file",
			Config: buildCfg(map[string]any{
				"method": "GET",
				"url":    "/api/v1/internal/vcs/raw?integration_id={{integration_id}}&sha={{head_sha}}&path={{finding.file}}",
				"auth":   "internal",
			})},
		{ID: "has_prebaked", Type: model.NodeTypeCondition, Label: "Has prebaked patch?",
			Config: buildCfg(map[string]any{
				"expr": "input.suggested_patch != null && input.suggested_patch != ''",
			})},
		{ID: "generate", Type: model.NodeTypeLLMAgent, Label: "Generate patch",
			Config: buildCfg(map[string]any{
				"roleId":    "default-code-fixer",
				"model":     "claude-sonnet-4-6",
				"budgetUsd": 1.0,
			})},
		{ID: "validate", Type: model.NodeTypeFunction, Label: "Validate patch",
			Config: buildCfg(map[string]any{
				"name": "patch_validate",
				// Placeholder: replaced when Plan 2E lands fix_runner endpoints
				"url": "/api/v1/internal/fix-runs/dry-run",
			})},
		{ID: "decide", Type: model.NodeTypeCondition, Label: "Dry-run OK?",
			Config: buildCfg(map[string]any{
				"expr": "validate.output.dry_run_ok == true",
			})},
		{ID: "execute", Type: model.NodeTypeHTTPCall, Label: "Execute fix",
			Config: buildCfg(map[string]any{
				"method": "POST",
				// Placeholder: replaced when Plan 2E lands fix_runner endpoints
				"url":           "/api/v1/internal/fix-runs/execute",
				"body_template": `{"finding_id":"{{finding_id}}","patch":"{{validate.output.patch}}"}`,
			})},
		{ID: "update_original_pr", Type: model.NodeTypeHTTPCall, Label: "Post PR comment",
			Config: buildCfg(map[string]any{
				"method": "POST",
				"url":    "/api/v1/internal/vcs/post-comment",
			})},
		{ID: "card", Type: model.NodeTypeIMSend, Label: "Send result card",
			Config: buildCfg(map[string]any{
				"template": "fix_result",
			})},
	}

	edges := []model.WorkflowEdge{
		{ID: "e1", Source: "trigger", Target: "fetch_file"},
		{ID: "e2", Source: "fetch_file", Target: "has_prebaked"},
		// Prebaked path: skip generate, go directly to validate
		{ID: "e3", Source: "has_prebaked", Target: "validate", Condition: "true", Label: "has patch"},
		// Generate path: no prebaked patch
		{ID: "e4", Source: "has_prebaked", Target: "generate", Condition: "false", Label: "needs generation"},
		{ID: "e5", Source: "generate", Target: "validate"},
		// After validation
		{ID: "e6", Source: "validate", Target: "decide"},
		{ID: "e7", Source: "decide", Target: "execute", Condition: "true", Label: "dry-run OK"},
		{ID: "e8", Source: "decide", Target: "card", Condition: "false", Label: "validation failed"},
		{ID: "e9", Source: "execute", Target: "update_original_pr"},
		{ID: "e10", Source: "update_original_pr", Target: "card"},
	}

	nodesJSON, _ := json.Marshal(nodes)
	edgesJSON, _ := json.Marshal(edges)

	return &model.WorkflowDefinition{
		ID:          uuid.New(),
		Name:        TemplateCodeFixer,
		Description: "System code-fixer DAG: fetch file, optionally generate patch, validate, execute, post comment, send IM card.",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategorySystem,
		Nodes:       nodesJSON,
		Edges:       edgesJSON,
		Version:     1,
	}
}
