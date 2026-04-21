# Go WASM Plugin Runtime

这份文档说明 AgentForge 当前仓库中已经落地的 Go 侧 WASM 插件运行时、Go SDK、作者工作流和本地验证方式。

## 当前范围

- Go 宿主当前的 Go SDK / build helper 链路同时覆盖 `IntegrationPlugin + runtime: wasm` 和 `WorkflowPlugin + runtime: wasm`
- manifest 需要声明：
  - `spec.module`
  - `spec.abiVersion`
  - 可选 `spec.capabilities`
- Go 宿主通过 `wazero` + WASI 加载插件模块
- Go 宿主会在执行前校验必需导出：
  - `agentforge_abi_version`
  - `agentforge_run`
- 插件注册记录会保存：
  - `resolved_source_path`
  - `runtime_metadata.abi_version`
  - `runtime_metadata.compatible`
- 激活、调用、健康检查、降级和重启结果来自真实运行时，而不是乐观状态推断
- 当前 built-in bundle 也把 Go-hosted WASM built-ins 作为真实资产管理，而不是仅靠文档示例

## 目录位置

- SDK：`src-go/plugin-sdk-go`
- 样例插件：`src-go/cmd/sample-wasm-plugin`
- 工作流样例：`src-go/cmd/standard-dev-flow`
- 脚手架入口：`scripts/plugin/create-plugin.js`
- built-in bundle：`plugins/builtin-bundle.yaml`
- 内置示例 manifest：`plugins/integrations/sample-integration-plugin/manifest.yaml`
- 内置工作流 manifest：`plugins/workflows/standard-dev-flow/manifest.yaml`
- 构建脚本：`scripts/plugin/build-go-wasm-plugin.js`
- 调试脚本：`scripts/plugin/debug-go-wasm-plugin.js`
- 最小运行栈脚本：`scripts/plugin/run-plugin-dev-stack.js`
- smoke 验证脚本：`scripts/plugin/verify-plugin-dev-workflow.js`
- built-in bundle 校验脚本：`scripts/plugin/verify-built-in-plugin-bundle.js`
- 校验 fixture：`scripts/plugin/__fixtures__/invalid-go-wasm-plugin-manifest.yaml`

## SDK 契约

当前仓库里的 Go SDK 暴露的是 typed authoring contract：

- `Describe(ctx) (*Descriptor, error)`
- `Init(ctx) error`
- `Health(ctx) (*Result, error)`
- `Invoke(ctx, invocation) (*Result, error)`

其中高频 helper 包括：

- `Descriptor` / `Capability`：声明插件元数据、ABI、运行时和能力清单
- `Invocation`：封装当前 operation 和 payload
- `Result` / `Success(...)`：统一成功返回
- `RuntimeError` / `NewRuntimeError(...)`：统一结构化错误返回
- `Context.Operation()` / `Context.AllowedCapabilities()` / `Context.CapabilityAllowed(...)`
- `ExportABIVersion(...)` / `ExportRun(...)` / `Autorun(...)`

同时保留两个导出函数作为稳定 ABI 契约：

- `agentforge_abi_version`
- `agentforge_run`

当前执行模型使用 WASI 环境变量驱动：

- `AGENTFORGE_AUTORUN`
- `AGENTFORGE_OPERATION`
- `AGENTFORGE_CONFIG`
- `AGENTFORGE_CAPABILITIES`
- `AGENTFORGE_PAYLOAD`

插件通过 stdout 返回 JSON envelope，通过 stderr 输出结构化日志。

如果 manifest 声明了 `spec.capabilities`，Go 宿主会拒绝未声明的 operation 调用；如果插件自己通过 `Context.CapabilityAllowed(...)` 进行额外保护，返回的错误也会以结构化 runtime error 落到宿主侧。

当前推荐的 sample 写法是：

```go
type samplePlugin struct{}

func (samplePlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "sample-integration-plugin",
		Name:       "Sample Integration Plugin",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "echo"},
		},
	}, nil
}

var runtime = pluginsdk.NewRuntime(samplePlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }

func main() { pluginsdk.Autorun(runtime) }
```

## API Surface

当前 Go 后端暴露的集成插件管理入口包括：

- `POST /api/v1/plugins/install`
- `GET /api/v1/plugins/catalog`
- `POST /api/v1/plugins/catalog/install`
- `POST /api/v1/plugins/:id/update`
- `POST /api/v1/plugins/:id/deactivate`
- `POST /api/v1/plugins/:id/activate`
- `GET /api/v1/plugins/:id/health`
- `POST /api/v1/plugins/:id/restart`
- `POST /api/v1/plugins/:id/invoke`

其中 `invoke` 请求体格式为：

```json
{
  "operation": "send_message",
  "payload": {
    "chat_id": "chat-1",
    "content": "hello"
  }
}
```

外部 source 的当前控制面语义：

- `builtin` / `local` 仍可直接进入现有 install -> enable -> activate 流程
- `git` / `npm` / `catalog` source 会在 install 时记录 digest、signature、approval、release metadata
- 这些外部 source 只有在 digest + signature 或 operator approval 使 trust 状态变为 `verified` 后，才能继续 enable / activate

当前 built-in bundle 语义：

- `plugins/builtin-bundle.yaml` 是官方 built-ins 的权威清单
- 当前 bundle 至少覆盖 `sample-integration-plugin` 和 `standard-dev-flow` 两个 Go-hosted WASM 目标
- bundle 还会同时声明 ToolPlugin / ReviewPlugin / WorkflowPlugin 的最小验证要求，避免“文档里有、仓库里没有”的漂移

## 本地构建

在仓库根目录执行：

```bash
pnpm build:plugin:wasm
```

该命令会使用：

- `GOOS=wasip1`
- `GOARCH=wasm`
- `CGO_ENABLED=0`

并生成：

```text
plugins/integrations/sample-integration-plugin/dist/sample.wasm
```

如需走更接近作者工作流的构建路径，也可以显式传入 manifest/source/output：

```bash
pnpm plugin:build -- --manifest plugins/integrations/sample-integration-plugin/manifest.yaml
pnpm plugin:build -- --manifest path/to/manifest.yaml --source ./cmd/sample-wasm-plugin
pnpm plugin:build -- --manifest path/to/manifest.yaml --source ./cmd/sample-wasm-plugin --output dist/custom.wasm
```

当前支持边界是：

- 受维护样例可以只给 manifest，因为脚本会通过 repo 内 target map 解析它的 Go build 入口。
- 非受维护目标如果没有 target map，就必须显式传 `--source`。
- `source.path` 仍然表示 manifest 来源，不会被误当成 `go build` 的包路径。
- `pnpm create-plugin -- --type workflow --name <name>` / `integration` 会生成一条与这些 build helper 对齐的 repo-local starter，而不是另一套平行模板。

`scripts/plugin/__fixtures__/invalid-go-wasm-plugin-manifest.yaml` 用来验证缺少 `spec.module` 时的提前失败语义。

## 本地调试

在仓库根目录执行：

```bash
pnpm plugin:debug -- --manifest plugins/integrations/sample-integration-plugin/manifest.yaml --operation health
pnpm plugin:debug -- --manifest plugins/integrations/sample-integration-plugin/manifest.yaml --operation echo --payload "{\"message\":\"hello\"}"
```

调试脚本会：

- 通过 `go run ./cmd/plugin-debugger` 复用当前 Go WASM runtime，而不是另起一套 dev-only 协议。
- 按真实运行时合同传入 `AGENTFORGE_AUTORUN`、`AGENTFORGE_OPERATION`、`AGENTFORGE_CONFIG`、`AGENTFORGE_CAPABILITIES`、`AGENTFORGE_PAYLOAD`。
- 返回结构化 JSON，同时保留 stdout/stderr，便于定位 envelope 错误、能力声明错误和宿主加载错误。

## 最小插件开发栈

如果你在做插件作者工作流，而不是全量桌面联调，可以先运行：

```bash
pnpm plugin:dev
```

这个命令只负责最小宿主组合：

- Go Orchestrator：`http://127.0.0.1:7777/health`
- TS Bridge：`http://127.0.0.1:7778/bridge/health`

语义说明：

- 如果服务已经健康，会直接复用，而不是重复启动。
- 如果缺少 `go` 或 `bun`，会先报缺失前置依赖。
- 如果进程能拉起但健康检查迟迟不过，会明确标记为 unhealthy，而不是静默挂起。

## 本地验证

推荐的聚焦验证命令：

```bash
pnpm exec jest --runInBand scripts/plugin/build-go-wasm-plugin.test.ts scripts/plugin/debug-go-wasm-plugin.test.ts scripts/plugin/run-plugin-dev-stack.test.ts scripts/plugin/verify-plugin-dev-workflow.test.ts scripts/plugin/verify-built-in-plugin-bundle.test.ts
pnpm plugin:verify -- --manifest plugins/integrations/sample-integration-plugin/manifest.yaml
pnpm plugin:verify:builtins
cd src-go
go test ./plugin-sdk-go -count=1
go test ./internal/plugin -count=1
go test ./internal/bridge
go test ./internal/handler
go test ./internal/service -run PluginService -count=1
```

覆盖范围：

- `scripts/plugin/*.test.ts`：manifest 驱动构建、debug envelope、最小开发栈合同、verify stage 计划，以及 built-in bundle 合同校验
- `plugin:verify`：受维护样例的 `build -> debug health` smoke 路径
- `plugin:verify:builtins`：校验内置 bundle 清单、manifest 存在性、以及 built-in readiness/verify 元数据
- `plugin-sdk-go`：typed descriptor/context/result/error helper 和 export helper
- `internal/plugin`：manifest 校验、WASM runtime 激活/健康检查/调用/重启、ABI mismatch、debug execution path
- `internal/bridge`：插件运行时状态字段桥接解析
- `internal/handler`：插件安装、列表、调用入口和 runtime state sync
- `plugin_service.go + plugin_service_test.go`：Go 插件服务层的聚焦行为验证

## Legacy 迁移说明

仓库仍接受 `IntegrationPlugin + runtime: go-plugin` 的 manifest 解析，以便旧清单可以被识别；但它们不会再被激活执行。

如果用户尝试激活旧清单，系统会返回明确迁移提示，要求迁移到：

- `runtime: wasm`
- `spec.module`
- `spec.abiVersion`

## 当前 Follow-up

- `WorkflowPlugin` 当前只支持 `process: sequential` 的执行路径；`hierarchical` 和 `event-driven` 仍会返回显式 unsupported error
- SDK 仍然建立在 JSON envelope + WASI env transport 上；如果后续调用面继续扩大，可以再演进为更正式的 ABI/内存桥接
- 仓库 `service` 目录还有其他并行工作线尚未收敛，因此更广的 `go test ./...` 仍可能被非本变更问题阻塞
- `src-go/cmd/sample-wasm-plugin` 仍是受维护的 integration sample；`src-go/cmd/standard-dev-flow` 则是受维护的 workflow sample。两者都已被 built-in bundle 和脚本 target map 纳入当前作者工作流。
