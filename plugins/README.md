# AgentForge Plugins

This directory holds every plugin the platform ships with. Each plugin is an
**independent unit** — its own `manifest.yaml` and (where applicable) its own
compiled artifact — that the orchestrator can load, activate, invoke, and
tear down at runtime without a server restart.

## Directory layout

```
plugins/
├── builtin-bundle.yaml           # bundle index (which plugins ship with a release)
├── integrations/                 # IntegrationPlugin — event ingestion / outbound
│   ├── feishu-adapter/
│   │   ├── manifest.yaml
│   │   └── dist/feishu.wasm      # compiled WASM module
│   └── qianchuan-ads/
│       └── manifest.yaml         # firstparty-inproc — ships inside the Go binary
├── reviews/                      # ReviewPlugin — MCP reviewers invoked by the bridge
│   ├── architecture-check/
│   └── performance-check/
├── tools/                        # ToolPlugin — MCP tool servers callable by agents
│   ├── db-query/
│   ├── github-tool/
│   ├── review-control/
│   ├── task-control/
│   ├── web-search/
│   └── workflow-control/
└── workflows/                    # WorkflowPlugin — declarative DAG/sequential flows
```

## Plugin kinds and runtimes

| Kind                | Runtime options                            | Host            |
| ------------------- | ------------------------------------------ | --------------- |
| `ToolPlugin`        | `mcp` (stdio/http)                         | ts-bridge       |
| `ReviewPlugin`      | `mcp` (stdio/http)                         | ts-bridge       |
| `IntegrationPlugin` | `go-plugin`, `wasm`, `firstparty-inproc`   | go-orchestrator |
| `WorkflowPlugin`    | `wasm`                                     | go-orchestrator |
| `RolePlugin`        | `declarative`                              | (UI only)       |

### `firstparty-inproc` runtime

First-party integrations whose Go source ships inside the monolith
(e.g. `qianchuan-ads`). They declare a manifest so the control plane
surfaces them uniformly, but their wiring lives in a registration hook
(see `src-go/internal/server/qianchuan_plugin.go`). Gated by an env
var — operators disable without editing code. **This is a pragmatic
stopgap**; the target shape is WASM so the code can be physically
extracted into its own module.

## Lifecycle

```
  [disk manifest]
       │
       │  (1) boot-time SyncBuiltIns / POST /api/v1/plugins/rescan
       ▼
  installed ──(2) POST /plugins/:id/enable──▶ enabled
                                                │
                                                │  (3) POST /plugins/:id/activate
                                                ▼
                                            activating ─▶ active ─▶ degraded
                                                │           │
                                                │  (4) POST /plugins/:id/deactivate
                                                ▼           ▼
                                             enabled    (heartbeat)
                                                │
                                                │  (5) POST /plugins/:id/disable
                                                ▼
                                             disabled ─▶ DELETE /plugins/:id
```

1. **Discovery** — `SyncBuiltIns` walks this directory at boot and upserts
   every new manifest into the registry. Existing operator state is
   preserved. To hot-reload a newly-dropped manifest without restarting,
   `POST /api/v1/plugins/rescan`.
2. **Enable** — operator approves the plugin. No runtime yet.
3. **Activate** — runtime connects: `ts-bridge` spawns the MCP child
   process (stdio) or opens an HTTP client; `wasm` instantiates the
   module via wazero; `firstparty-inproc` is already active by boot.
4. **Deactivate** — teardown via `ToolPluginRuntimeClient.DisableToolPlugin`
   (MCP disconnect + child-process release). WASM modules are
   ephemeral per-invoke, so nothing to release.
5. **Disable / Uninstall** — same teardown path plus DB row delete.
   Manifest files on disk are **not** removed automatically.

## HTTP surface

All routes live under `/api/v1`. See `plugin_handler.go` for handlers.

```
GET    /plugins                     list with filters: kind, state, source, trust
GET    /plugins/:id                 full record
GET    /plugins/:id/status          lightweight probe for FE gates (404 = not installed)
GET    /plugins/:id/health          active health check
POST   /plugins/install             install from local path or catalog entry
POST   /plugins/rescan              rescan disk for new builtin manifests
GET    /plugins/discover            read-only: discover builtin manifests (no DB write)
POST   /plugins/:id/enable          → enabled
POST   /plugins/:id/activate        → active (runtime loads)
POST   /plugins/:id/deactivate      → enabled (runtime releases)
POST   /plugins/:id/disable         → disabled (runtime releases)
POST   /plugins/:id/restart         re-activate the runtime
POST   /plugins/:id/invoke          { operation, payload }  (WASM integration plugins)
POST   /plugins/:id/mcp/tools/call  { tool, args }          (MCP tool plugins)
POST   /plugins/:id/mcp/refresh     re-query MCP tools/resources/prompts
DELETE /plugins/:id                 uninstall (runtime torn down first)
```

## Authoring a new plugin

1. Create a directory under the appropriate kind root (`tools/`, `reviews/`,
   `integrations/`, or `workflows/`).
2. Write `manifest.yaml`. Minimum fields:
   ```yaml
   apiVersion: agentforge/v1
   kind: ToolPlugin            # or ReviewPlugin / IntegrationPlugin / etc.
   metadata:
     id: my-plugin             # unique, kebab-case
     name: My Plugin
     version: 0.1.0
   spec:
     runtime: mcp              # must be compatible with kind — see parser.go
     transport: stdio
     command: bun
     args: ["run", "src/index.ts"]
   permissions:
     network: { required: false }
   ```
3. For **MCP plugins** (Tool / Review): implement the server with the
   TS SDK at `src-bridge/src/plugin-sdk/` — `defineToolPlugin` or
   `defineReviewPlugin` + `createPluginMcpServer`.
4. For **WASM plugins** (Integration / Workflow): compile Go/Rust/AssemblyScript
   to `.wasm`, export `agentforge_abi_version` and `agentforge_run`. Place
   the module under `dist/<name>.wasm` and reference it via `spec.module`.
5. **Rebuild the server** or `POST /api/v1/plugins/rescan` to register
   the new manifest, then hit `/plugins/:id/enable` and `/plugins/:id/activate`.

## Caveats

- **Module caching**: the WASM runtime currently re-instantiates the
  module per invocation. Fine for low-frequency operations; a pooled
  handle is future work (tracked in `runtime.go` comments).
- **Health heartbeat**: there is no background ticker polling plugin
  health today. Clients should call `GET /plugins/:id/health` before
  critical invocations if the runtime may have drifted.
- **Manifest file cleanup on Uninstall**: the DB row is removed, the
  on-disk manifest is left in place — so a subsequent rescan will
  re-register the plugin. Remove the directory manually to fully
  uninstall a builtin.
- **Trust / signatures**: `source.trust` and `source.signature` are
  wired in the schema but enforcement is still permissive; treat
  them as advisory for now.

## Extending the SDK

- Go: manifest/runtime types are under `src-go/internal/model/plugin.go`
  and validation at `src-go/internal/plugin/parser.go`. Add new kinds
  or runtimes here and update `isAllowedRuntime`.
- TS: `src-bridge/src/plugins/schema.ts` (Zod) mirrors the Go schema —
  keep parity or the bridge will reject manifests that Go accepts.
  Helpers live at `src-bridge/src/plugin-sdk/`.
