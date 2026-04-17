## ADDED Requirements

### Requirement: Command plugins SHALL be declared through a YAML manifest schema

IM Bridge SHALL load command plugins from manifest files under `${IM_BRIDGE_PLUGIN_DIR}/<plugin-id>/plugin.yaml`. Each manifest MUST declare a stable plugin `id`, a semantic `version`, a human-readable `name`, and a non-empty `commands` list where each command entry specifies a slash name, optional subcommands, an `action_class`, and an `invoke` block. Manifest validation MUST reject unknown fields that claim dynamic Go code loading or native binary execution, and MUST fail loading the plugin (without failing the bridge) when any field is malformed.

#### Scenario: Valid manifest registers its commands
- **WHEN** `plugins/jira/plugin.yaml` declares plugin `@acme/jira-commands` with a `/jira create` subcommand
- **THEN** the command engine exposes `/jira create` and routes matching messages to the plugin's handler
- **AND** the operator status snapshot lists the plugin as `loaded`

#### Scenario: Malformed manifest is isolated
- **WHEN** `plugins/broken/plugin.yaml` is invalid YAML or references an unknown `invoke.kind`
- **THEN** the bridge skips that plugin, emits an audit event `direction=internal action=plugin_load_failed`, and reports the failure in the operator snapshot
- **AND** other plugins continue to load and the bridge stays ready

#### Scenario: Duplicate slash command is rejected
- **WHEN** two plugins declare the same top-level slash command with overlapping subcommand names
- **THEN** the second plugin to register the overlap is rejected with a conflict error
- **AND** the built-in registration path (if any) takes precedence over a conflicting external manifest

### Requirement: Plugin invoke modes SHALL cover http, mcp, and builtin

The manifest `invoke.kind` field SHALL accept exactly `http`, `mcp`, and `builtin`. HTTP invocations MUST send a POST request with a JSON body containing `tenant`, `user`, `chat`, `command`, `args`, and normalized reply-target metadata, honor a per-invocation timeout (default 10s), and map the response body into a delivery envelope. MCP invocations MUST reuse the existing `src-bridge` MCP client path through an in-process proxy, naming the server and tool declared in the manifest. Builtin invocations MUST reference a handler already compiled into the bridge binary via a stable registration key.

#### Scenario: HTTP invoke produces a delivery envelope
- **WHEN** `/jira create ISSUE-123` invokes `http://localhost:9090/plugins/jira/create` and receives `{ "text": "Created ISSUE-123", "mutable": false }`
- **THEN** the bridge emits a delivery envelope carrying the returned text and the original reply target
- **AND** the audit event records `metadata.plugin_id=@acme/jira-commands metadata.invoke_kind=http`

#### Scenario: HTTP invoke timeout produces a clean failure
- **WHEN** the plugin HTTP endpoint does not respond within the manifest-declared timeout
- **THEN** the invocation is cancelled, the user receives a short "plugin timed out" reply, and the failure is counted against the plugin's rate limit
- **AND** subsequent invocations are still possible once the endpoint recovers

#### Scenario: MCP invoke routes to the declared tool
- **WHEN** `invoke.kind: mcp` names server `@acme/jira-mcp` and tool `link_task`
- **THEN** the bridge forwards the request to the configured MCP proxy with the tool arguments
- **AND** the response is mapped into a delivery envelope just like the HTTP path

#### Scenario: Builtin invoke dispatches in-process
- **WHEN** a migrated built-in command such as `/task list` is declared with `invoke.kind: builtin` and key `builtin.task.list`
- **THEN** the registry dispatches to the in-process handler without an HTTP hop
- **AND** the same rate limit, audit, and tenant scoping apply as for external plugins

### Requirement: Plugin execution SHALL respect tenant scope and allowlist

Each command dispatch SHALL resolve the current tenant from the inbound message and MUST check the plugin's optional `tenants` allowlist. If the tenant is not in the allowlist the dispatch MUST fail with an explicit "plugin not available for this tenant" reply, MUST emit an audit event with `metadata.reason=plugin_tenant_scope`, and MUST NOT invoke the plugin's transport (HTTP, MCP, or builtin).

#### Scenario: Tenant allowlist blocks disallowed tenant
- **WHEN** plugin `@acme/jira-commands` declares `tenants: [acme]` and a user in tenant `beta` sends `/jira create`
- **THEN** the bridge rejects the command with an explicit notice
- **AND** no HTTP request or MCP call is made

#### Scenario: Empty allowlist allows every tenant
- **WHEN** the manifest omits `tenants` entirely
- **THEN** the plugin is available to every tenant in the runtime
- **AND** per-tenant metadata is still threaded through the invocation

#### Scenario: Tenant-scoped credential reference resolves at dispatch
- **WHEN** the manifest contains `${TENANT_META_KEY}` references in headers or arguments
- **THEN** the placeholder is expanded with the current tenant's metadata before the invocation
- **AND** expansion failures produce a clean error reply instead of a partial invocation

### Requirement: Plugin directory SHALL support hot-reload on filesystem changes

The bridge SHALL watch `IM_BRIDGE_PLUGIN_DIR` with `fsnotify` (or an equivalent platform watcher) and MUST reload manifests on create, modify, and delete events. Reloads MUST be isolated per plugin: a failure while reloading one plugin MUST NOT unload other plugins. Removed plugin directories MUST cause their registered commands to be unregistered on the next reload tick.

#### Scenario: New plugin appears without a restart
- **WHEN** an operator drops `plugins/deploy/plugin.yaml` into the plugin dir
- **THEN** the next reload tick registers the plugin's commands and marks it `loaded` in the snapshot
- **AND** no restart is required to expose the new slash command

#### Scenario: Manifest edit is picked up atomically
- **WHEN** an operator edits `plugins/jira/plugin.yaml` to change a subcommand timeout
- **THEN** the reload swaps the plugin's registration atomically and does not serve a half-updated manifest
- **AND** inflight invocations continue against the snapshot they started under

#### Scenario: Deleted plugin directory unregisters commands
- **WHEN** `plugins/legacy` is removed from disk
- **THEN** the next reload tick unregisters its commands and removes the entry from the snapshot
- **AND** subsequent invocations of those commands produce an "unknown command" reply

### Requirement: Marketplace install SHALL materialize command plugins into the plugin dir

When the AgentForge marketplace installs a plugin whose manifest declares `im_commands`, the marketplace consumer path SHALL write the plugin's files into `IM_BRIDGE_PLUGIN_DIR` so the bridge can pick them up through the hot-reload watcher. The bridge MUST NOT reach back to the marketplace to download plugins on its own; distribution remains a backend-side push or shared-volume mount.

#### Scenario: Marketplace install ends in a bridge reload
- **WHEN** an operator installs plugin `@acme/jira-commands` from the marketplace
- **THEN** the marketplace consumer writes `plugins/@acme/jira-commands/plugin.yaml` (and referenced assets) to `IM_BRIDGE_PLUGIN_DIR`
- **AND** the bridge's next reload tick registers the plugin's commands automatically

#### Scenario: Marketplace uninstall removes the plugin from the bridge
- **WHEN** an operator uninstalls a previously installed marketplace plugin
- **THEN** the consumer deletes the plugin directory from `IM_BRIDGE_PLUGIN_DIR`
- **AND** the bridge hot-reload observes the removal and unregisters the commands

#### Scenario: Bridge never fetches directly from the marketplace
- **WHEN** the plugin dir is empty and a tenant invokes a command that would be provided by a marketplace plugin
- **THEN** the bridge replies "unknown command"
- **AND** the bridge does not attempt an outbound marketplace download
