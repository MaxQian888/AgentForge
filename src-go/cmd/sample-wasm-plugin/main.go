package main

import (
	"fmt"

	pluginsdk "github.com/react-go-quick-starter/server/plugin-sdk-go"
)

type samplePlugin struct{}

func (samplePlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	ctx.Log("info", "describe sample wasm plugin")
	return &pluginsdk.Descriptor{
		APIVersion:  "agentforge/v1",
		Kind:        "IntegrationPlugin",
		ID:          "feishu-adapter",
		Name:        "Feishu Adapter",
		Version:     "0.1.0",
		Runtime:     "wasm",
		ABIVersion:  pluginsdk.ABIVersion,
		Description: "Built-in Go integration plugin example for IM event ingestion and outbound delivery.",
		Capabilities: []pluginsdk.Capability{
			{Name: "health", Description: "Report plugin health and current mode"},
			{Name: "send_message", Description: "Send a message payload to a chat target"},
		},
	}, nil
}

func (samplePlugin) Init(ctx *pluginsdk.Context) error {
	ctx.Log("info", "initialize sample wasm plugin")
	return nil
}

func (samplePlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.Success(map[string]any{
		"status": "ok",
		"mode":   ctx.ConfigString("mode"),
	}), nil
}

func (samplePlugin) Invoke(ctx *pluginsdk.Context, invocation pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch invocation.Operation {
	case "send_message":
		return pluginsdk.Success(map[string]any{
			"status":  "sent",
			"chat_id": invocation.Payload["chat_id"],
			"content": invocation.Payload["content"],
			"mode":    ctx.ConfigString("mode"),
		}), nil
	default:
		return nil, pluginsdk.NewRuntimeError("unsupported_operation", fmt.Sprintf("unsupported operation %s", invocation.Operation)).
			WithDetail("operation", invocation.Operation)
	}
}

var runtime = pluginsdk.NewRuntime(samplePlugin{})

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
