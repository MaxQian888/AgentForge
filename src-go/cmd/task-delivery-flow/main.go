package main

import (
	"fmt"

	pluginsdk "github.com/react-go-quick-starter/server/plugin-sdk-go"
)

type workflowPlugin struct{}

func (workflowPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion:  "agentforge/v1",
		Kind:        "WorkflowPlugin",
		ID:          "task-delivery-flow",
		Name:        "Task Delivery Flow",
		Version:     "0.1.0",
		Runtime:     "wasm",
		ABIVersion:  pluginsdk.ABIVersion,
		Description: "Sequential starter workflow plugin for planner to coding to review task delivery.",
		Capabilities: []pluginsdk.Capability{
			{Name: "run_workflow", Description: "Execute the task delivery starter workflow"},
		},
	}, nil
}

func (workflowPlugin) Init(ctx *pluginsdk.Context) error {
	return nil
}

func (workflowPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.Success(map[string]any{
		"status":   "ok",
		"workflow": "task-delivery-flow",
	}), nil
}

func (workflowPlugin) Invoke(ctx *pluginsdk.Context, invocation pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch invocation.Operation {
	case "run_workflow":
		return pluginsdk.Success(map[string]any{
			"status":    "accepted",
			"workflow":  "task-delivery-flow",
			"operation": invocation.Operation,
		}), nil
	default:
		return nil, pluginsdk.NewRuntimeError("unsupported_operation", fmt.Sprintf("unsupported operation %s", invocation.Operation)).
			WithDetail("operation", invocation.Operation)
	}
}

var runtime = pluginsdk.NewRuntime(workflowPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 {
	return pluginsdk.ExportABIVersion(runtime)
}

//go:wasmexport agentforge_run
func agentforgeRun() uint32 {
	return pluginsdk.ExportRun(runtime)
}

func main() {
	pluginsdk.Autorun(runtime)
}
