package main

import (
	"fmt"
	"os"

	pluginsdk "github.com/react-go-quick-starter/server/plugin-sdk-go"
)

type samplePlugin struct{}

func (samplePlugin) Describe(ctx *pluginsdk.Context) (map[string]any, error) {
	ctx.Log("info", "describe sample wasm plugin")
	return map[string]any{
		"id":           "feishu-adapter",
		"name":         "Feishu Adapter",
		"abiVersion":   pluginsdk.ABIVersion,
		"capabilities": []string{"health", "send_message"},
	}, nil
}

func (samplePlugin) Init(ctx *pluginsdk.Context) error {
	ctx.Log("info", "initialize sample wasm plugin")
	return nil
}

func (samplePlugin) Health(ctx *pluginsdk.Context) (map[string]any, error) {
	return map[string]any{
		"status": "ok",
		"mode":   ctx.ConfigString("mode"),
	}, nil
}

func (samplePlugin) Invoke(ctx *pluginsdk.Context, operation string, payload map[string]any) (map[string]any, error) {
	switch operation {
	case "send_message":
		return map[string]any{
			"status":  "sent",
			"chat_id": payload["chat_id"],
			"content": payload["content"],
			"mode":    ctx.ConfigString("mode"),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported operation %s", operation)
	}
}

var runtime = pluginsdk.NewRuntime(samplePlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 {
	return runtime.ABIVersion()
}

//go:wasmexport agentforge_run
func agentforgeRun() uint32 {
	return runtime.Run()
}

func main() {
	if pluginsdk.ShouldAutorun() {
		os.Exit(int(runtime.Run()))
	}
}
