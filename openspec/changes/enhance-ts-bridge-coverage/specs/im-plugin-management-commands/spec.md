# im-plugin-management-commands Specification

## Purpose
Provide IM command surface for managing TS Bridge plugins and tools, enabling ChatOps workflows for plugin installation, uninstallation, and lifecycle management from chat platforms.

## ADDED Requirements

### Requirement: List installed Bridge plugins via IM command
The IM Bridge SHALL provide `/tools list` command to display all installed Bridge plugins with their status, capabilities, and metadata.

#### Scenario: List all plugins
- **WHEN** user sends `/tools list` via IM platform
- **THEN** IM Bridge calls `GET /api/v1/bridge/tools`
- **THEN** Go backend proxies to `GET http://localhost:7778/bridge/tools`
- **THEN** IM Bridge displays formatted list of plugins with id, name, status, and capabilities

#### Scenario: List plugins with filtering
- **WHEN** user sends `/tools list --status enabled` via IM platform
- **THEN** IM Bridge filters response to show only enabled plugins
- **THEN** IM Bridge displays filtered plugin list

#### Scenario: No plugins installed
- **WHEN** user sends `/tools list` and no plugins are installed
- **THEN** IM Bridge displays "No plugins installed" message with installation hint

### Requirement: Install Bridge plugin via IM command
The IM Bridge SHALL provide `/tools install <manifest-url>` command to install a Bridge plugin from a manifest URL with admin role requirement.

#### Scenario: Install plugin with admin role
- **WHEN** admin user sends `/tools install https://example.com/plugin-manifest.json` via IM platform
- **THEN** IM Bridge validates user has admin role
- **THEN** IM Bridge calls `POST /api/v1/bridge/tools/install` with `{ manifest_url: "https://..." }`
- **THEN** Go backend proxies to `POST http://localhost:7778/bridge/tools/install`
- **THEN** IM Bridge displays installation progress and success message with plugin id

#### Scenario: Install plugin without admin role
- **WHEN** non-admin user sends `/tools install <url>` via IM platform
- **THEN** IM Bridge rejects command with "Admin role required for plugin installation" error
- **THEN** Installation does not proceed

#### Scenario: Install plugin with invalid manifest URL
- **WHEN** admin sends `/tools install https://invalid-url.com/manifest.json`
- **THEN** Bridge returns error "Failed to fetch manifest"
- **THEN** IM Bridge displays error message with details

#### Scenario: Install plugin with manifest URL not in allowlist
- **WHEN** admin sends `/tools install https://untrusted.com/manifest.json`
- **THEN** Go backend validates URL against allowlist
- **THEN** Go backend rejects with "Manifest URL not in allowlist" error
- **THEN** IM Bridge displays security error

### Requirement: Uninstall Bridge plugin via IM command
The IM Bridge SHALL provide `/tools uninstall <plugin-id>` command to uninstall a Bridge plugin with admin role requirement and plugin validation.

#### Scenario: Uninstall installed plugin
- **WHEN** admin user sends `/tools install my-plugin` via IM platform
- **THEN** IM Bridge validates user has admin role
- **THEN** IM Bridge calls `POST /api/v1/bridge/tools/uninstall` with `{ plugin_id: "my-plugin" }`
- **THEN** Go backend validates plugin exists
- **THEN** Go backend proxies to `POST http://localhost:7778/bridge/tools/uninstall`
- **THEN** IM Bridge displays success message

#### Scenario: Uninstall non-existent plugin
- **WHEN** admin sends `/tools uninstall nonexistent-plugin`
- **THEN** Go backend returns 404 "Plugin not found"
- **THEN** IM Bridge displays error "Plugin 'nonexistent-plugin' not found"

#### Scenario: Uninstall plugin without admin role
- **WHEN** non-admin user sends `/tools uninstall my-plugin`
- **THEN** IM Bridge rejects with "Admin role required for plugin uninstallation" error

### Requirement: Restart Bridge plugin via IM command
The IM Bridge SHALL provide `/tools restart <plugin-id>` command to restart a Bridge plugin for troubleshooting or applying configuration changes.

#### Scenario: Restart running plugin
- **WHEN** user sends `/tools restart my-plugin` via IM platform
- **THEN** IM Bridge calls `POST /api/v1/bridge/tools/my-plugin/restart`
- **THEN** Go backend proxies to `POST http://localhost:7778/bridge/tools/my-plugin/restart`
- **THEN** Bridge disables and re-enables the plugin
- **THEN** IM Bridge displays "Plugin 'my-plugin' restarted successfully"

#### Scenario: Restart plugin with MCP reconnection
- **WHEN** user restarts a plugin with MCP servers
- **THEN** Bridge refreshes MCP connections during restart
- **THEN** IM Bridge displays "Plugin restarted, MCP servers reconnected"

#### Scenario: Restart failed plugin
- **WHEN** user sends `/tools restart failed-plugin` for a plugin in error state
- **THEN** Bridge attempts restart
- **THEN** IM Bridge displays restart attempt and final status

#### Scenario: Restart non-existent plugin
- **WHEN** user sends `/tools restart nonexistent-plugin`
- **THEN** Go backend returns 404
- **THEN** IM Bridge displays "Plugin 'nonexistent-plugin' not found"

### Requirement: Go backend proxies Bridge tools endpoints
The Go backend SHALL expose `/api/v1/bridge/tools`, `/api/v1/bridge/tools/install`, `/api/v1/bridge/tools/uninstall`, and `/api/v1/bridge/tools/:id/restart` endpoints that proxy to TS Bridge with authentication and authorization.

#### Scenario: Get tools proxies to Bridge
- **WHEN** authenticated client calls `GET /api/v1/bridge/tools`
- **THEN** Go backend forwards to `GET http://localhost:7778/bridge/tools`
- **THEN** Go backend returns plugin list from Bridge

#### Scenario: Install tool proxies to Bridge with validation
- **WHEN** admin client calls `POST /api/v1/bridge/tools/install` with `{ manifest_url: "..." }`
- **THEN** Go backend validates admin role
- **THEN** Go backend validates manifest URL against allowlist
- **THEN** Go backend forwards to `POST http://localhost:7778/bridge/tools/install`
- **THEN** Go backend returns installation result

#### Scenario: Uninstall tool proxies to Bridge with validation
- **WHEN** admin client calls `POST /api/v1/bridge/tools/uninstall` with `{ plugin_id: "..." }`
- **THEN** Go backend validates admin role
- **THEN** Go backend validates plugin exists
- **THEN** Go backend forwards to `POST http://localhost:7778/bridge/tools/uninstall`
- **THEN** Go backend returns uninstallation result

#### Scenario: Restart tool proxies to Bridge
- **WHEN** authenticated client calls `POST /api/v1/bridge/tools/:id/restart`
- **THEN** Go backend validates plugin exists
- **THEN** Go backend forwards to `POST http://localhost:7778/bridge/tools/:id/restart`
- **THEN** Go backend returns restart result

### Requirement: Plugin management audit logging
The Go backend SHALL log all plugin installation, uninstallation, and restart operations with user ID, timestamp, and plugin details for audit purposes.

#### Scenario: Log plugin installation
- **WHEN** admin installs plugin via `/tools install`
- **THEN** Go backend logs `{ event: "plugin_installed", user_id: "...", plugin_id: "...", manifest_url: "...", timestamp: "..." }`

#### Scenario: Log plugin uninstallation
- **WHEN** admin uninstalls plugin via `/tools uninstall`
- **THEN** Go backend logs `{ event: "plugin_uninstalled", user_id: "...", plugin_id: "...", timestamp: "..." }`

#### Scenario: Log plugin restart
- **WHEN** user restarts plugin via `/tools restart`
- **THEN** Go backend logs `{ event: "plugin_restarted", user_id: "...", plugin_id: "...", timestamp: "..." }`
