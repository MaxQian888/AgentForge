# Go WASM Plugin Runtime

这份文档说明 AgentForge 当前仓库中已经落地的 Go 侧 WASM 插件运行时、Go SDK 和本地验证方式。

## 当前范围

- Go 宿主当前支持 `IntegrationPlugin + runtime: wasm`
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

## 目录位置

- SDK：`src-go/plugin-sdk-go`
- 样例插件：`src-go/cmd/sample-wasm-plugin`
- 内置示例 manifest：`plugins/integrations/feishu-adapter/manifest.yaml`
- 构建脚本：`scripts/build-go-wasm-plugin.js`

## SDK 契约

首版 Go SDK 暴露的插件接口是：

- `Describe`
- `Init`
- `Health`
- `Invoke`

同时保留两个导出函数作为稳定 ABI 契约：

- `agentforge_abi_version`
- `agentforge_run`

当前执行模型使用 WASI 环境变量驱动：

- `AGENTFORGE_AUTORUN`
- `AGENTFORGE_OPERATION`
- `AGENTFORGE_CONFIG`
- `AGENTFORGE_PAYLOAD`

插件通过 stdout 返回 JSON envelope，通过 stderr 输出结构化日志。

如果 manifest 声明了 `spec.capabilities`，Go 宿主会拒绝未声明的 operation 调用。

## API Surface

当前 Go 后端暴露的集成插件管理入口包括：

- `POST /api/v1/plugins/install`
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
plugins/integrations/feishu-adapter/dist/feishu.wasm
```

## 本地验证

推荐的聚焦验证命令：

```bash
pnpm build:plugin:wasm
cd src-go
go test ./internal/plugin
go test ./internal/bridge
go test ./internal/handler
cd internal/service
go test plugin_service.go plugin_service_test.go
```

覆盖范围：

- `internal/plugin`：manifest 校验、WASM runtime 激活/健康检查/调用/重启、ABI mismatch
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

- 目前只落地了 Go 宿主的首版 WASM 原型，尚未扩展到 `WorkflowPlugin`
- SDK 目前采用 JSON envelope + WASI env；如果后续调用面增大，可以再演进为更正式的 ABI/内存桥接
- 仓库 `service` 目录还有其他并行工作线尚未收敛，因此更广的 `go test ./...` 仍可能被非本变更问题阻塞
