# OpenAgents Python SDK Integration — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a parallel Python sidecar (`openagent-runtime`, port 7780) that executes AgentForge roles tagged `execution_engine: openagent` via the OpenAgents Python SDK, coexisting with the existing TypeScript Bridge path.

**Architecture:** Dual-track — Go dispatcher routes spawns by `role.execution_engine` to either `BridgeClient` (existing) or new `PythonAgentClient`. Python sidecar wraps `openagents.Runtime` in FastAPI. Tools and sessions proxy back to Go over HTTP; events stream via WS to Go. TS Bridge is not modified (remains the MCP hub).

**Tech Stack:** Python 3.10+ / FastAPI / uvicorn / OpenAgents SDK / httpx; Go 1.21+ / Echo / Gorilla WebSocket; PyInstaller for packaging; existing `scripts/dev/dev-all.js` and `scripts/build/build-backend.js` patterns.

**Reference Spec:** `docs/superpowers/specs/2026-04-20-openagents-integration-design.md`

---

## File Structure

**New (Python sidecar)**:
- `src-openagent/pyproject.toml` — uv/pip project manifest
- `src-openagent/main.py` — FastAPI app entry
- `src-openagent/openagent_sidecar/__init__.py`
- `src-openagent/openagent_sidecar/config.py` — env + settings
- `src-openagent/openagent_sidecar/agent_registry.py` — loads agent.json from disk
- `src-openagent/openagent_sidecar/runtime_handler.py` — wraps `openagents.Runtime.run_stream`
- `src-openagent/openagent_sidecar/plugins/session_manager.py` — `AgentForgeSessionManager`
- `src-openagent/openagent_sidecar/plugins/tool_proxy.py` — `AgentForgeToolProxy`
- `src-openagent/openagent_sidecar/plugins/event_forwarder.py` — `AgentForgeEventForwarder`
- `src-openagent/openagent_sidecar/event_translator.py` — OpenAgents → AgentForge event mapping
- `src-openagent/openagent_sidecar/models.py` — pydantic models matching Go `ExecuteRequest`
- `src-openagent/openagent_sidecar/ws_client.py` — WebSocket client to Go `/ws/bridge`
- `src-openagent/openagent_sidecar/go_client.py` — HTTP client for `/api/v1/internal/*`
- `src-openagent/tests/test_*.py` — pytest tests
- `scripts/build/build-openagent.js` — PyInstaller build script
- `scripts/dev/openagent.mjs` — dev-all integration fragment (if needed)

**New (Go)**:
- `src-go/internal/openagent/client.go` — `PythonAgentClient`
- `src-go/internal/openagent/role_mapper.go` — `RoleConfig` → `AgentDefinition` dict
- `src-go/internal/openagent/agent_config_writer.go` — writes agent.json on role change
- `src-go/internal/handler/internal_handler.go` — `/api/v1/internal/*` endpoints
- `src-go/internal/openagent/*_test.go` — Go unit tests

**Modified (Go)**:
- `src-go/internal/model/role_manifest.go` — new fields
- `src-go/internal/role/registry.go` — load + validate new fields
- `src-go/internal/service/agent_service.go` — dispatcher branch
- `src-go/internal/service/dispatch_service.go` (or equivalent) — route by engine
- `src-go/internal/bridge/client.go` — expose tool invocation if missing
- `src-go/internal/ws/event_types.go` (or nearest) — new `AgentEventType` constants
- `src-go/internal/config/config.go` — add `OpenagentURL` config
- `src-go/cmd/server/main.go` — wire PythonAgentClient + AgentConfigWriter

**Modified (build/dev)**:
- `package.json` — new scripts: `dev:openagent`, `build:backend:openagent`
- `scripts/dev/dev-all.js` — register openagent service
- `scripts/build/build-backend.js` — include openagent in backend build
- `src-tauri/tauri.conf.json` — add `openagent-runtime` to `externalBin`
- `src-tauri/tauri.windows.conf.json` / `tauri.macos.conf.json` / `tauri.linux.conf.json` — per-platform sidecar binary names

**Modified (schema)**:
- `src-bridge/src/schemas.ts` — no changes (Bridge is untouched)
- Role YAML schema docs if any — reflect new fields

---

## Task 1: Extend Role Manifest Model (Go)

**Files:**
- Modify: `src-go/internal/model/role_manifest.go`
- Modify: `src-go/internal/role/registry.go`
- Test: `src-go/internal/role/registry_test.go`

- [ ] **Step 1: Write failing test for new role fields**

Append to `src-go/internal/role/registry_test.go`:

```go
func TestLoadRoleWithOpenagentEngine(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "planner", `
metadata:
  id: planner
  name: Planner
execution_engine: openagent
openagent_pattern: react
requires_worktree: false
goal: plan stuff
backstory: a planner
`)
	reg, err := role.NewFileStore(dir).Load()
	require.NoError(t, err)

	m, ok := reg.Get("planner")
	require.True(t, ok)
	require.Equal(t, "openagent", m.ExecutionEngine)
	require.Equal(t, "react", m.OpenagentPattern)
	require.False(t, m.RequiresWorktree)
}

func TestOpenagentEngineRequiresPattern(t *testing.T) {
	dir := t.TempDir()
	writeRoleFile(t, dir, "bad", `
metadata:
  id: bad
  name: Bad
execution_engine: openagent
goal: x
backstory: y
`)
	_, err := role.NewFileStore(dir).Load()
	require.ErrorContains(t, err, "openagent_pattern")
}
```

Helper if absent:

```go
func writeRoleFile(t *testing.T, dir, id, body string) {
	t.Helper()
	sub := filepath.Join(dir, id)
	require.NoError(t, os.MkdirAll(sub, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "role.yaml"), []byte(body), 0644))
}
```

- [ ] **Step 2: Run test — expect fail**

```bash
cd src-go && go test ./internal/role/... -run TestLoadRoleWithOpenagentEngine -v
```

Expected: compile error or field-not-found.

- [ ] **Step 3: Add fields to `Manifest` struct**

In `src-go/internal/model/role_manifest.go`, add to the top-level `Manifest` struct:

```go
type Manifest struct {
	// ... existing fields ...
	ExecutionEngine  string `yaml:"execution_engine" json:"execution_engine,omitempty"`
	OpenagentPattern string `yaml:"openagent_pattern" json:"openagent_pattern,omitempty"`
	RequiresWorktree bool   `yaml:"requires_worktree" json:"requires_worktree,omitempty"`
}
```

- [ ] **Step 4: Add validation in loader**

In `src-go/internal/role/registry.go` where manifests are validated post-load, add:

```go
func validateEngine(m *model.Manifest) error {
	engine := m.ExecutionEngine
	if engine == "" {
		engine = "bridge"
		m.ExecutionEngine = engine
	}
	switch engine {
	case "bridge":
		return nil
	case "openagent":
		switch m.OpenagentPattern {
		case "react", "plan_execute", "reflexion":
			return nil
		default:
			return fmt.Errorf("role %q: openagent_pattern must be one of react|plan_execute|reflexion when execution_engine=openagent", m.Metadata.ID)
		}
	default:
		return fmt.Errorf("role %q: unknown execution_engine %q", m.Metadata.ID, engine)
	}
}
```

Call `validateEngine(m)` inside the existing `Load()` loop after unmarshalling.

- [ ] **Step 5: Default `RequiresWorktree` based on engine**

In the same validation function, after engine check:

```go
if !m.RequiresWorktreeExplicit() {
	if engine == "bridge" {
		m.RequiresWorktree = true
	}
	// openagent default is false (zero value already)
}
```

This requires a helper to know whether the field was explicitly set. Simplest path: change the YAML field to `*bool`, normalize in code. Update `Manifest`:

```go
RequiresWorktree    bool  `yaml:"-" json:"requires_worktree"`
RequiresWorktreeRaw *bool `yaml:"requires_worktree" json:"-"`
```

In `validateEngine`:

```go
if m.RequiresWorktreeRaw != nil {
	m.RequiresWorktree = *m.RequiresWorktreeRaw
} else {
	m.RequiresWorktree = (engine == "bridge")
}
```

- [ ] **Step 6: Run tests — expect pass**

```bash
cd src-go && go test ./internal/role/... -v
```

Expected: both new tests PASS.

- [ ] **Step 7: Commit**

```bash
rtk git add src-go/internal/model/role_manifest.go src-go/internal/role/registry.go src-go/internal/role/registry_test.go
rtk git commit -m "feat(role): add execution_engine, openagent_pattern, requires_worktree fields"
```

---

## Task 2: Dispatcher Routing Stub (Go)

**Files:**
- Modify: `src-go/internal/service/agent_service.go`
- Test: `src-go/internal/service/agent_service_openagent_test.go`

- [ ] **Step 1: Write failing test for routing**

Create `src-go/internal/service/agent_service_openagent_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"agentforge/src-go/internal/service"
	// plus fakes
)

func TestSpawnRoutesToOpenagentEngine(t *testing.T) {
	fakeBridge := &fakeBridgeClient{}
	fakePython := &fakePythonClient{}
	svc := newTestAgentServiceWithEngines(t, fakeBridge, fakePython, "planner-openagent")

	_, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: uuid.New(), RoleID: "planner-openagent", Caller: service.Caller{UserID: uuid.New()},
	})
	require.NoError(t, err)
	require.Equal(t, 0, fakeBridge.CallCount)
	require.Equal(t, 1, fakePython.CallCount)
}

func TestSpawnRoutesToBridgeByDefault(t *testing.T) {
	fakeBridge := &fakeBridgeClient{}
	fakePython := &fakePythonClient{}
	svc := newTestAgentServiceWithEngines(t, fakeBridge, fakePython, "coder-bridge")

	_, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: uuid.New(), RoleID: "coder-bridge", Caller: service.Caller{UserID: uuid.New()},
	})
	require.NoError(t, err)
	require.Equal(t, 1, fakeBridge.CallCount)
	require.Equal(t, 0, fakePython.CallCount)
}
```

Fakes (define in same test file):

```go
type fakeBridgeClient struct{ CallCount int }
func (f *fakeBridgeClient) Execute(_ context.Context, _ bridge.ExecuteRequest) (*bridge.ExecuteResponse, error) {
	f.CallCount++; return &bridge.ExecuteResponse{SessionID: "s"}, nil
}

type fakePythonClient struct{ CallCount int }
func (f *fakePythonClient) Execute(_ context.Context, _ openagent.ExecuteRequest) (*openagent.ExecuteResponse, error) {
	f.CallCount++; return &openagent.ExecuteResponse{SessionID: "s"}, nil
}
```

`newTestAgentServiceWithEngines` builds an `AgentService` with a minimal in-memory repo + role registry seeded with one role matching the requested engine.

- [ ] **Step 2: Run test — expect compile fail (no PythonClient yet)**

```bash
cd src-go && go test ./internal/service/... -run TestSpawnRoutes -v
```

Expected: undefined `openagent` / `fakePythonClient`.

- [ ] **Step 3: Create placeholder Python client interface**

Create `src-go/internal/openagent/client.go`:

```go
package openagent

import "context"

type ExecuteRequest struct {
	TaskID       string            `json:"task_id"`
	SessionID    string            `json:"session_id"`
	RoleID       string            `json:"role_id"`
	Prompt       string            `json:"prompt"`
	KnowledgeCtx string            `json:"knowledge_context,omitempty"`
	MaxSteps     int               `json:"max_steps,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type ExecuteResponse struct {
	SessionID string `json:"session_id"`
}

type Client interface {
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)
	Health(ctx context.Context) error
	Cancel(ctx context.Context, sessionID string) error
	ReloadAgents(ctx context.Context) error
}
```

- [ ] **Step 4: Branch in AgentService.Spawn**

In `src-go/internal/service/agent_service.go`, find the `Spawn` (or `SpawnAgent`) method. After resolving `roleConfig`, before calling `s.bridge.Execute`:

```go
manifest, _ := s.roleStore.Get(input.RoleID)
engine := manifest.ExecutionEngine
if engine == "" {
	engine = "bridge"
}

switch engine {
case "openagent":
	pyReq := openagent.ExecuteRequest{
		TaskID:    task.ID.String(),
		SessionID: sessionID,
		RoleID:    input.RoleID,
		Prompt:    buildSpawnPrompt(task),
		MaxSteps:  resolveSpawnMaxTurns(roleConfig),
	}
	pyResp, err := s.pythonAgent.Execute(ctx, pyReq)
	if err != nil {
		return dispatchResult{}, fmt.Errorf("openagent spawn: %w", err)
	}
	// persist AgentRun, publish events as bridge path does
	return s.finalizeSpawn(ctx, task, memberID, pyResp.SessionID, "openagent"), nil
case "bridge":
	// existing path unchanged
default:
	return dispatchResult{}, fmt.Errorf("unknown execution_engine %q", engine)
}
```

Add `pythonAgent openagent.Client` field to `AgentService` and its constructor.

- [ ] **Step 5: Run test — expect pass**

```bash
cd src-go && go test ./internal/service/... -run TestSpawnRoutes -v
```

Expected: both tests PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/openagent/client.go src-go/internal/service/agent_service.go src-go/internal/service/agent_service_openagent_test.go
rtk git commit -m "feat(agent): route spawn to openagent engine when role.execution_engine=openagent"
```

---

## Task 3: Scaffold Python Sidecar

**Files:**
- Create: `src-openagent/pyproject.toml`
- Create: `src-openagent/main.py`
- Create: `src-openagent/openagent_sidecar/__init__.py`
- Create: `src-openagent/openagent_sidecar/config.py`
- Create: `src-openagent/tests/test_health.py`

- [ ] **Step 1: Create pyproject.toml**

```toml
[project]
name = "openagent-sidecar"
version = "0.1.0"
requires-python = ">=3.10"
dependencies = [
  "fastapi>=0.115",
  "uvicorn[standard]>=0.30",
  "httpx[http2]>=0.28",
  "pydantic>=2.0",
  "websockets>=12.0",
  "openagents @ file:///${PROJECT_ROOT:-../openagent-python-sdk}",
]

[project.optional-dependencies]
dev = ["pytest>=8", "pytest-asyncio>=0.23", "respx>=0.21"]

[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.build_meta"

[tool.pytest.ini_options]
asyncio_mode = "auto"
```

Adjust the `openagents` dep path based on actual sibling location; if the SDK is pip-installable later, switch to a version spec.

- [ ] **Step 2: Create settings module**

`src-openagent/openagent_sidecar/config.py`:

```python
from __future__ import annotations

import os
from pathlib import Path
from pydantic import BaseModel


class Settings(BaseModel):
    port: int = 7780
    host: str = "127.0.0.1"
    go_base_url: str = "http://127.0.0.1:7777"
    go_ws_url: str = "ws://127.0.0.1:7777/ws/bridge"
    internal_token: str = ""
    agents_dir: Path = Path.home() / ".agentforge" / "openagent-agents"
    checkpoints_db: Path = Path.home() / ".agentforge" / "openagent-checkpoints.db"
    log_level: str = "INFO"

    @classmethod
    def from_env(cls) -> "Settings":
        return cls(
            port=int(os.getenv("OPENAGENT_PORT", "7780")),
            host=os.getenv("OPENAGENT_HOST", "127.0.0.1"),
            go_base_url=os.getenv("AGENTFORGE_GO_URL", "http://127.0.0.1:7777"),
            go_ws_url=os.getenv("AGENTFORGE_GO_WS", "ws://127.0.0.1:7777/ws/bridge"),
            internal_token=os.getenv("AGENTFORGE_INTERNAL_TOKEN", ""),
            agents_dir=Path(os.getenv("OPENAGENT_AGENTS_DIR", str(Path.home() / ".agentforge" / "openagent-agents"))),
            checkpoints_db=Path(os.getenv("OPENAGENT_CHECKPOINTS_DB", str(Path.home() / ".agentforge" / "openagent-checkpoints.db"))),
            log_level=os.getenv("OPENAGENT_LOG_LEVEL", "INFO"),
        )
```

- [ ] **Step 3: Create main.py with /health**

```python
from __future__ import annotations

import time
from fastapi import FastAPI
from openagent_sidecar.config import Settings


def create_app(settings: Settings | None = None) -> FastAPI:
    settings = settings or Settings.from_env()
    app = FastAPI(title="openagent-sidecar")
    started_ms = int(time.time() * 1000)

    @app.get("/health")
    async def health():
        return {
            "status": "ok",
            "uptime_ms": int(time.time() * 1000) - started_ms,
            "loaded_agents": 0,
            "active_runs": 0,
        }

    app.state.settings = settings
    return app


app = create_app()


if __name__ == "__main__":
    import uvicorn
    s = Settings.from_env()
    uvicorn.run("main:app", host=s.host, port=s.port, log_level=s.log_level.lower())
```

- [ ] **Step 4: Create __init__.py and placeholder dirs**

`src-openagent/openagent_sidecar/__init__.py` → empty
`src-openagent/openagent_sidecar/plugins/__init__.py` → empty
`src-openagent/tests/__init__.py` → empty

- [ ] **Step 5: Write health test**

`src-openagent/tests/test_health.py`:

```python
from fastapi.testclient import TestClient
from main import create_app


def test_health_returns_ok():
    app = create_app()
    client = TestClient(app)
    r = client.get("/health")
    assert r.status_code == 200
    body = r.json()
    assert body["status"] == "ok"
    assert body["uptime_ms"] >= 0
```

- [ ] **Step 6: Run tests**

```bash
cd src-openagent && uv venv && uv pip install -e ".[dev]" && uv run pytest -v
```

Expected: `test_health_returns_ok` PASS.

- [ ] **Step 7: Commit**

```bash
rtk git add src-openagent/
rtk git commit -m "feat(openagent): scaffold Python sidecar with /health endpoint"
```

---

## Task 4: Python `/run` Endpoint with Static ReAct Agent

**Files:**
- Create: `src-openagent/openagent_sidecar/agent_registry.py`
- Create: `src-openagent/openagent_sidecar/models.py`
- Create: `src-openagent/openagent_sidecar/runtime_handler.py`
- Modify: `src-openagent/main.py`
- Test: `src-openagent/tests/test_run_endpoint.py`

- [ ] **Step 1: Define request/response models**

`src-openagent/openagent_sidecar/models.py`:

```python
from __future__ import annotations
from pydantic import BaseModel, Field


class RunRequestIn(BaseModel):
    task_id: str
    session_id: str
    role_id: str
    prompt: str
    knowledge_context: str | None = None
    max_steps: int = 10
    env: dict[str, str] | None = None


class RunResponseFinal(BaseModel):
    session_id: str
    stop_reason: str
    final_output: str | None = None
    error: str | None = None
```

- [ ] **Step 2: Define agent registry stub**

`src-openagent/openagent_sidecar/agent_registry.py`:

```python
from __future__ import annotations
import json
from pathlib import Path
from typing import Any


class AgentRegistry:
    def __init__(self, agents_dir: Path):
        self._dir = agents_dir
        self._agents: dict[str, dict[str, Any]] = {}

    def load_all(self) -> int:
        self._agents.clear()
        if not self._dir.exists():
            return 0
        for f in self._dir.glob("*.json"):
            with f.open("r", encoding="utf-8") as fp:
                data = json.load(fp)
            role_id = data.get("id")
            if role_id:
                self._agents[role_id] = data
        return len(self._agents)

    def get(self, role_id: str) -> dict[str, Any] | None:
        return self._agents.get(role_id)

    def count(self) -> int:
        return len(self._agents)
```

- [ ] **Step 3: Write minimal runtime handler (hardcoded ReAct, no tools, no plugins yet)**

`src-openagent/openagent_sidecar/runtime_handler.py`:

```python
from __future__ import annotations
import json
from typing import AsyncIterator, Any
from openagents import Runtime
from openagents.interfaces.runtime import RunRequest, RunBudget

from openagent_sidecar.models import RunRequestIn


def _build_app_config(agent_def: dict[str, Any]) -> dict[str, Any]:
    return {
        "version": "1.0",
        "session": {"type": "in_memory"},
        "events": {"type": "async"},
        "agents": [agent_def],
    }


async def stream_run(
    runtime: Runtime,
    req_in: RunRequestIn,
) -> AsyncIterator[str]:
    request = RunRequest(
        agent_id=req_in.role_id,
        session_id=req_in.session_id,
        input_text=req_in.prompt,
        budget=RunBudget(max_steps=req_in.max_steps),
        context_hints={"knowledge_context": req_in.knowledge_context or ""},
    )
    async for chunk in runtime.run_stream(request=request):
        yield json.dumps({
            "kind": chunk.kind.value,
            "sequence": chunk.sequence,
            "timestamp_ms": chunk.timestamp_ms,
            "payload": chunk.payload,
            "result": chunk.result.model_dump(mode="json") if chunk.result else None,
        }) + "\n"


def build_runtime(agent_def: dict[str, Any]) -> Runtime:
    return Runtime.from_dict(_build_app_config(agent_def))
```

- [ ] **Step 4: Wire /run endpoint**

Append to `src-openagent/main.py` inside `create_app`:

```python
from fastapi import HTTPException
from fastapi.responses import StreamingResponse
from openagent_sidecar.agent_registry import AgentRegistry
from openagent_sidecar.models import RunRequestIn
from openagent_sidecar.runtime_handler import build_runtime, stream_run

registry = AgentRegistry(settings.agents_dir)
registry.load_all()
app.state.registry = registry

@app.post("/run")
async def run_endpoint(body: RunRequestIn):
    agent_def = registry.get(body.role_id)
    if agent_def is None:
        raise HTTPException(404, f"agent {body.role_id} not loaded")
    runtime = build_runtime(agent_def)
    return StreamingResponse(stream_run(runtime, body), media_type="application/x-ndjson")
```

Also update health:

```python
@app.get("/health")
async def health():
    return {
        "status": "ok",
        "uptime_ms": int(time.time() * 1000) - started_ms,
        "loaded_agents": registry.count(),
        "active_runs": 0,
    }
```

- [ ] **Step 5: Write test with mock LLM**

`src-openagent/tests/test_run_endpoint.py`:

```python
import json
import tempfile
from pathlib import Path

from fastapi.testclient import TestClient
from main import create_app
from openagent_sidecar.config import Settings


def _make_mock_agent(dir: Path, role_id: str) -> None:
    dir.mkdir(parents=True, exist_ok=True)
    (dir / f"{role_id}.json").write_text(json.dumps({
        "id": role_id,
        "name": role_id,
        "pattern": {"type": "react", "config": {}},
        "memory": {"type": "window_buffer", "config": {"window_size": 4}},
        "llm": {"provider": "mock", "model": "mock", "config": {
            "responses": [{"content": "hello from mock"}]
        }},
        "tools": [],
    }))


def test_run_streams_ndjson_with_mock_llm():
    tmp = Path(tempfile.mkdtemp())
    _make_mock_agent(tmp, "planner")
    settings = Settings(agents_dir=tmp)
    app = create_app(settings)
    client = TestClient(app)

    with client.stream("POST", "/run", json={
        "task_id": "t1", "session_id": "s1", "role_id": "planner",
        "prompt": "hi", "max_steps": 3,
    }) as r:
        assert r.status_code == 200
        lines = [json.loads(x) for x in r.iter_lines() if x]

    assert any(l["kind"] == "run.started" for l in lines)
    assert any(l["kind"] == "run.finished" for l in lines)
```

Note: verify the `mock` provider response format matches actual SDK (`openagents/llm/providers/mock.py`) before finalizing test — adjust response shape if needed.

- [ ] **Step 6: Run tests**

```bash
cd src-openagent && uv run pytest tests/test_run_endpoint.py -v
```

Expected: PASS. If the mock provider response schema differs from the example, fix the agent.json fixture accordingly.

- [ ] **Step 7: Commit**

```bash
rtk git add src-openagent/
rtk git commit -m "feat(openagent): /run endpoint with NDJSON streaming and hardcoded react agent"
```

---

## Task 5: Event Translator + WS Forwarder

**Files:**
- Create: `src-openagent/openagent_sidecar/event_translator.py`
- Create: `src-openagent/openagent_sidecar/ws_client.py`
- Create: `src-openagent/openagent_sidecar/plugins/event_forwarder.py`
- Test: `src-openagent/tests/test_event_translator.py`

- [ ] **Step 1: Write translator test**

`src-openagent/tests/test_event_translator.py`:

```python
from openagent_sidecar.event_translator import translate


def test_translate_tool_called():
    e = translate("tool.called", {"tool_name": "search", "params": {"q": "x"}})
    assert e is not None
    assert e["type"] == "tool_call"
    assert e["data"]["tool_name"] == "search"


def test_translate_usage_updated_token_only():
    e = translate("usage.updated", {
        "input_tokens": 100, "output_tokens": 50, "cached_read_tokens": 10,
    })
    assert e["type"] == "cost_update"
    # USD is Go's responsibility
    assert "cost_usd" not in e["data"]
    assert e["data"]["input_tokens"] == 100


def test_translate_pattern_plan_created_is_new_type():
    e = translate("pattern.plan_created", {"plan": [{"step": "s1"}]})
    assert e["type"] == "agent.plan_updated"


def test_translate_unknown_event_returns_none():
    assert translate("some.unknown", {}) is None
```

- [ ] **Step 2: Run test — expect import error**

```bash
cd src-openagent && uv run pytest tests/test_event_translator.py -v
```

- [ ] **Step 3: Implement translator**

`src-openagent/openagent_sidecar/event_translator.py`:

```python
from __future__ import annotations
from typing import Any

MAP: dict[str, str] = {
    "tool.called": "tool_call",
    "tool.succeeded": "tool_result",
    "tool.failed": "tool_result",
    "llm.succeeded": "output",
    "llm.delta": "partial_message",
    "usage.updated": "cost_update",
    "session.run.started": "status_change",
    "session.run.completed": "status_change",
    "run.checkpoint_saved": "agent.checkpoint_saved",
    "pattern.phase": "agent.pattern_phase",
    "pattern.step_started": "agent.pattern_step_started",
    "pattern.plan_created": "agent.plan_updated",
    "memory.inject.completed": "agent.memory_injected",
}


def translate(name: str, payload: dict[str, Any]) -> dict[str, Any] | None:
    out_type = MAP.get(name)
    if out_type is None:
        return None
    # cost_update — strip any cost_usd field, token counts only
    data = dict(payload)
    if out_type == "cost_update":
        data.pop("cost_usd", None)
        data.pop("cost_breakdown", None)
    return {"type": out_type, "data": data}
```

- [ ] **Step 4: Run translator tests — expect pass**

```bash
cd src-openagent && uv run pytest tests/test_event_translator.py -v
```

Expected: 4 PASS.

- [ ] **Step 5: Implement WS client**

`src-openagent/openagent_sidecar/ws_client.py`:

```python
from __future__ import annotations
import asyncio
import json
import logging
from typing import Any
import websockets

logger = logging.getLogger(__name__)


class GoWSClient:
    def __init__(self, url: str, token: str):
        self._url = url
        self._token = token
        self._ws: Any = None
        self._lock = asyncio.Lock()
        self._queue: asyncio.Queue[dict[str, Any]] = asyncio.Queue(maxsize=1000)
        self._task: asyncio.Task | None = None

    async def start(self) -> None:
        self._task = asyncio.create_task(self._run())

    async def stop(self) -> None:
        if self._task:
            self._task.cancel()
        if self._ws is not None:
            await self._ws.close()

    async def send(self, event: dict[str, Any]) -> None:
        try:
            self._queue.put_nowait(event)
        except asyncio.QueueFull:
            logger.warning("ws queue full, dropping event %s", event.get("type"))

    async def _run(self) -> None:
        backoff = 1.0
        headers = {"Authorization": f"Bearer {self._token}"} if self._token else {}
        while True:
            try:
                async with websockets.connect(self._url, additional_headers=headers) as ws:
                    self._ws = ws
                    backoff = 1.0
                    while True:
                        ev = await self._queue.get()
                        await ws.send(json.dumps(ev))
            except asyncio.CancelledError:
                raise
            except Exception as e:
                logger.warning("ws disconnected: %s, reconnecting in %.1fs", e, backoff)
                await asyncio.sleep(backoff)
                backoff = min(backoff * 2, 30.0)
```

- [ ] **Step 6: Implement EventForwarder plugin**

`src-openagent/openagent_sidecar/plugins/event_forwarder.py`:

```python
from __future__ import annotations
import time
from openagents.interfaces.events import EventBusPlugin, RuntimeEvent
from openagent_sidecar.event_translator import translate
from openagent_sidecar.ws_client import GoWSClient


class AgentForgeEventForwarder(EventBusPlugin):
    def __init__(self, ws: GoWSClient, task_id: str, session_id: str):
        self._ws = ws
        self._task_id = task_id
        self._session_id = session_id

    async def emit(self, event_name: str, **payload) -> RuntimeEvent:
        ev = RuntimeEvent(name=event_name, payload=payload)
        translated = translate(event_name, payload)
        if translated is not None:
            await self._ws.send({
                "task_id": self._task_id,
                "session_id": self._session_id,
                "timestamp_ms": int(time.time() * 1000),
                "type": translated["type"],
                "data": translated["data"],
            })
        return ev

    def subscribe(self, event_name: str, handler) -> None:
        # forwarder doesn't maintain local subscribers
        pass

    async def get_history(self, event_name=None, limit=None):
        return []

    async def clear_history(self) -> None:
        pass
```

Note: verify `EventBusPlugin` signature against actual OpenAgents SDK version before merging; adjust method signatures if SDK differs.

- [ ] **Step 7: Commit**

```bash
rtk git add src-openagent/openagent_sidecar/event_translator.py src-openagent/openagent_sidecar/ws_client.py src-openagent/openagent_sidecar/plugins/event_forwarder.py src-openagent/tests/test_event_translator.py
rtk git commit -m "feat(openagent): event translator and WS forwarder to Go"
```

---

## Task 6: Go `PythonAgentClient` Implementation + Wire in Main

**Files:**
- Modify: `src-go/internal/openagent/client.go`
- Create: `src-go/internal/openagent/client_test.go`
- Modify: `src-go/cmd/server/main.go`
- Modify: `src-go/internal/config/config.go`

- [ ] **Step 1: Add config field**

In `src-go/internal/config/config.go`:

```go
type Config struct {
	// ... existing ...
	OpenagentURL     string `mapstructure:"openagent_url"`
	OpenagentToken   string `mapstructure:"openagent_internal_token"`
}
```

In the loader (where defaults are set):

```go
v.SetDefault("openagent_url", "http://127.0.0.1:7780")
v.SetDefault("openagent_internal_token", "")
```

- [ ] **Step 2: Write client test (with httptest)**

`src-go/internal/openagent/client_test.go`:

```go
package openagent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"agentforge/src-go/internal/openagent"
)

func TestClientExecuteHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/run", r.URL.Path)
		w.Header().Set("Content-Type", "application/x-ndjson")
		_, _ = w.Write([]byte(`{"kind":"run.started"}` + "\n"))
		_, _ = w.Write([]byte(`{"kind":"run.finished","result":{"run_id":"r1","stop_reason":"completed"}}` + "\n"))
	}))
	defer srv.Close()

	c := openagent.NewClient(srv.URL, "", nil)
	resp, err := c.Execute(context.Background(), openagent.ExecuteRequest{
		TaskID: "t", SessionID: "s", RoleID: "r", Prompt: "hi", MaxSteps: 3,
	})
	require.NoError(t, err)
	require.Equal(t, "s", resp.SessionID)
}

func TestClientHealth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
			return
		}
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := openagent.NewClient(srv.URL, "", nil)
	require.NoError(t, c.Health(context.Background()))
}
```

- [ ] **Step 3: Implement client**

In `src-go/internal/openagent/client.go`, replace the interface-only file with:

```go
package openagent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client interface {
	Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error)
	Health(ctx context.Context) error
	Cancel(ctx context.Context, sessionID string) error
	ReloadAgents(ctx context.Context) error
}

type HTTPClient struct {
	baseURL string
	token   string
	h       *http.Client
}

func NewClient(baseURL, token string, h *http.Client) *HTTPClient {
	if h == nil {
		h = &http.Client{Timeout: 0}
	}
	return &HTTPClient{baseURL: baseURL, token: token, h: h}
}

type ExecuteRequest struct {
	TaskID       string            `json:"task_id"`
	SessionID    string            `json:"session_id"`
	RoleID       string            `json:"role_id"`
	Prompt       string            `json:"prompt"`
	KnowledgeCtx string            `json:"knowledge_context,omitempty"`
	MaxSteps     int               `json:"max_steps,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
}

type ExecuteResponse struct {
	SessionID string `json:"session_id"`
}

func (c *HTTPClient) Execute(ctx context.Context, req ExecuteRequest) (*ExecuteResponse, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/run", bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.h.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openagent execute: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("openagent execute: status %d", resp.StatusCode)
	}
	// For NDJSON streams, we only need to acknowledge the session; streaming
	// events go through WS. Drain body so connection can be reused.
	_, _ = bytes.NewBuffer(nil).ReadFrom(resp.Body)
	return &ExecuteResponse{SessionID: req.SessionID}, nil
}

func (c *HTTPClient) Health(ctx context.Context) error {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	resp, err := c.h.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("health: status %d", resp.StatusCode)
	}
	return nil
}

func (c *HTTPClient) Cancel(ctx context.Context, sessionID string) error {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/sessions/"+sessionID+"/cancel", nil)
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.h.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("cancel: status %d", resp.StatusCode)
	}
	return nil
}

func (c *HTTPClient) ReloadAgents(ctx context.Context) error {
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/agents/reload", nil)
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.h.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("reload: status %d", resp.StatusCode)
	}
	return nil
}

var _ Client = (*HTTPClient)(nil)

const startTimeout = 5 * time.Second
```

Also evaluate: since `/run` is streaming NDJSON, for the phase-1 MVP we **do not read the stream** — the events flow back via WS. We just hold the HTTP connection open until the run completes. Consider launching `Execute` as fire-and-forget in a goroutine in `AgentService.Spawn`; bookkeep the run via WS events. Update the dispatcher from Task 2 accordingly if the current implementation expected synchronous completion. For Task 6, the minimum is: acknowledge handoff (200 OK reached) and return SessionID.

- [ ] **Step 4: Wire client in cmd/server/main.go**

Import:

```go
import "agentforge/src-go/internal/openagent"
```

During service construction:

```go
pythonClient := openagent.NewClient(cfg.OpenagentURL, cfg.OpenagentToken, nil)
agentSvc := service.NewAgentService(
	// ... existing deps ...
	bridgeClient,
	pythonClient, // new
)
```

- [ ] **Step 5: Run all tests**

```bash
cd src-go && go test ./internal/openagent/... ./internal/service/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/openagent/client.go src-go/internal/openagent/client_test.go src-go/internal/config/config.go src-go/cmd/server/main.go
rtk git commit -m "feat(openagent): HTTP client + wire into AgentService"
```

---

## Task 7: Role → agent.json Mapper + Writer

**Files:**
- Create: `src-go/internal/openagent/role_mapper.go`
- Create: `src-go/internal/openagent/role_mapper_test.go`
- Create: `src-go/internal/openagent/agent_config_writer.go`
- Modify: `src-go/cmd/server/main.go`

- [ ] **Step 1: Write mapper tests**

`src-go/internal/openagent/role_mapper_test.go`:

```go
package openagent_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"agentforge/src-go/internal/model"
	"agentforge/src-go/internal/openagent"
)

func TestMapToAgentDefinitionBasic(t *testing.T) {
	m := &model.Manifest{
		Metadata: model.RoleMetadata{ID: "planner", Name: "Planner"},
		ExecutionEngine: "openagent", OpenagentPattern: "react",
		Goal: "plan", Backstory: "thinker",
	}
	def := openagent.MapToAgentDefinition(m)
	require.Equal(t, "planner", def["id"])
	require.Equal(t, "Planner", def["name"])
	pattern := def["pattern"].(map[string]any)
	require.Equal(t, "react", pattern["type"])
}

func TestMapToAgentDefinitionToolsAsProxies(t *testing.T) {
	m := &model.Manifest{
		Metadata: model.RoleMetadata{ID: "r", Name: "R"},
		ExecutionEngine: "openagent", OpenagentPattern: "react",
		Capabilities: model.Capabilities{
			ToolConfig: model.ToolConfig{External: []string{"web_search", "fs_read"}},
		},
	}
	def := openagent.MapToAgentDefinition(m)
	tools := def["tools"].([]map[string]any)
	require.Len(t, tools, 2)
	require.Equal(t, "web_search", tools[0]["id"])
	require.Equal(t, "agentforge_tool_proxy", tools[0]["impl"])
}

func TestMapToAgentDefinitionAddsAgentForgeSeams(t *testing.T) {
	m := &model.Manifest{
		Metadata: model.RoleMetadata{ID: "r", Name: "R"},
		ExecutionEngine: "openagent", OpenagentPattern: "react",
	}
	def := openagent.MapToAgentDefinition(m)
	sess := def["session"].(map[string]any)
	require.Equal(t, "agentforge", sess["type"])
	ev := def["events"].(map[string]any)
	require.Equal(t, "agentforge", ev["type"])
}
```

Adjust `model.Capabilities` field paths to match the real `role_manifest.go`.

- [ ] **Step 2: Implement mapper**

`src-go/internal/openagent/role_mapper.go`:

```go
package openagent

import (
	"strings"

	"agentforge/src-go/internal/model"
)

// MapToAgentDefinition converts a role manifest to OpenAgents agent.json structure.
// Fields not representable in OpenAgents (file_permissions, network_permissions,
// output_filters) are dropped; callers should log a warning at role-write time.
func MapToAgentDefinition(m *model.Manifest) map[string]any {
	pattern := m.OpenagentPattern
	if pattern == "" {
		pattern = "react"
	}

	tools := buildTools(m)
	sysPrompt := buildSystemPrompt(m)

	return map[string]any{
		"id":   m.Metadata.ID,
		"name": m.Metadata.Name,
		"pattern": map[string]any{
			"type": pattern,
			"config": map[string]any{
				"system_prompt": sysPrompt,
			},
		},
		"memory": map[string]any{
			"type":   "window_buffer",
			"config": map[string]any{"window_size": 12},
		},
		"llm":     buildLLM(m),
		"tools":   tools,
		"session": map[string]any{"type": "agentforge"},
		"events":  map[string]any{"type": "agentforge"},
	}
}

func buildSystemPrompt(m *model.Manifest) string {
	var parts []string
	if m.Goal != "" {
		parts = append(parts, "# Goal\n"+m.Goal)
	}
	if m.Backstory != "" {
		parts = append(parts, "# Backstory\n"+m.Backstory)
	}
	if m.SystemPrompt != "" {
		parts = append(parts, m.SystemPrompt)
	}
	return strings.Join(parts, "\n\n")
}

func buildTools(m *model.Manifest) []map[string]any {
	var out []map[string]any
	for _, toolID := range m.Capabilities.ToolConfig.External {
		out = append(out, map[string]any{
			"id":   toolID,
			"impl": "agentforge_tool_proxy",
			"config": map[string]any{
				"tool_id": toolID,
			},
		})
	}
	return out
}

func buildLLM(m *model.Manifest) map[string]any {
	provider := "anthropic"
	model := "claude-3-5-sonnet-20241022"
	if m.LLMConfig.Provider != "" {
		provider = m.LLMConfig.Provider
	}
	if m.LLMConfig.Model != "" {
		model = m.LLMConfig.Model
	}
	return map[string]any{
		"provider":    provider,
		"model":       model,
		"api_key_env": "ANTHROPIC_API_KEY",
	}
}
```

Adjust references (`m.LLMConfig`, `m.Capabilities.ToolConfig.External`) to the actual manifest field paths; rename helpers if collision.

- [ ] **Step 3: Run mapper tests — expect pass**

```bash
cd src-go && go test ./internal/openagent/... -run TestMapTo -v
```

- [ ] **Step 4: Implement config writer**

`src-go/internal/openagent/agent_config_writer.go`:

```go
package openagent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"agentforge/src-go/internal/model"
)

type RoleLister interface {
	All() []*model.Manifest
}

type Reloader interface {
	ReloadAgents(ctx context.Context) error
}

type AgentConfigWriter struct {
	dir      string
	roles    RoleLister
	reloader Reloader
	log      *slog.Logger
}

func NewAgentConfigWriter(dir string, roles RoleLister, reloader Reloader, log *slog.Logger) *AgentConfigWriter {
	return &AgentConfigWriter{dir: dir, roles: roles, reloader: reloader, log: log}
}

func (w *AgentConfigWriter) WriteAll(ctx context.Context) error {
	if err := os.MkdirAll(w.dir, 0755); err != nil {
		return err
	}
	count := 0
	for _, m := range w.roles.All() {
		if m.ExecutionEngine != "openagent" {
			continue
		}
		if err := w.writeOne(m); err != nil {
			w.log.Warn("agent_config_writer: write failed", "role_id", m.Metadata.ID, "err", err)
			continue
		}
		count++
		w.warnUnsupportedFields(m)
	}
	w.log.Info("agent_config_writer: wrote openagent agents", "count", count)
	if err := w.reloader.ReloadAgents(ctx); err != nil {
		w.log.Warn("agent_config_writer: reload failed", "err", err)
	}
	return nil
}

func (w *AgentConfigWriter) writeOne(m *model.Manifest) error {
	def := MapToAgentDefinition(m)
	body, err := json.MarshalIndent(def, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(w.dir, m.Metadata.ID+".json")
	return os.WriteFile(path, body, 0644)
}

func (w *AgentConfigWriter) warnUnsupportedFields(m *model.Manifest) {
	if len(m.Capabilities.FilePermissions.AllowedPaths) > 0 {
		w.log.Warn("role has file_permissions but execution_engine=openagent — field dropped", "role_id", m.Metadata.ID)
	}
	// repeat for network_permissions, output_filters
	_ = fmt.Sprintf // placeholder if compiler needs
}
```

- [ ] **Step 5: Call writer on startup**

In `src-go/cmd/server/main.go`, after the role store loads:

```go
writer := openagent.NewAgentConfigWriter(
	filepath.Join(cfg.AppDataDir, "openagent-agents"),
	roleStore,
	pythonClient,
	logger,
)
if err := writer.WriteAll(ctx); err != nil {
	logger.Error("failed to write openagent agents", "err", err)
}
```

(If `cfg.AppDataDir` doesn't exist, add it — default to `$APPDATA/agentforge` on Windows, `~/.agentforge` elsewhere.)

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/openagent/role_mapper.go src-go/internal/openagent/role_mapper_test.go src-go/internal/openagent/agent_config_writer.go src-go/cmd/server/main.go
rtk git commit -m "feat(openagent): role→agent.json mapper and config writer"
```

---

## Task 8: Python `/agents/reload` + Sidecar Loads from Disk

**Files:**
- Modify: `src-openagent/main.py`
- Modify: `src-openagent/tests/test_health.py`
- Create: `src-openagent/tests/test_reload.py`

- [ ] **Step 1: Write reload test**

`src-openagent/tests/test_reload.py`:

```python
import json
import tempfile
from pathlib import Path

from fastapi.testclient import TestClient
from main import create_app
from openagent_sidecar.config import Settings


def test_reload_picks_up_new_agent():
    tmp = Path(tempfile.mkdtemp())
    settings = Settings(agents_dir=tmp)
    app = create_app(settings)
    client = TestClient(app)

    r = client.get("/health")
    assert r.json()["loaded_agents"] == 0

    (tmp / "r1.json").write_text(json.dumps({
        "id": "r1", "name": "R", "pattern": {"type": "react", "config": {}},
        "memory": {"type": "window_buffer", "config": {"window_size": 4}},
        "llm": {"provider": "mock", "model": "mock"}, "tools": [],
    }))

    r = client.post("/agents/reload")
    assert r.status_code == 200

    r = client.get("/health")
    assert r.json()["loaded_agents"] == 1
```

- [ ] **Step 2: Add /agents/reload endpoint**

In `src-openagent/main.py`, add inside `create_app`:

```python
@app.post("/agents/reload")
async def agents_reload():
    count = registry.load_all()
    return {"loaded_agents": count}

@app.get("/agents")
async def agents_list():
    return {"agents": list(registry._agents.keys()), "count": registry.count()}
```

- [ ] **Step 3: Run tests**

```bash
cd src-openagent && uv run pytest tests/test_reload.py -v
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
rtk git add src-openagent/main.py src-openagent/tests/test_reload.py
rtk git commit -m "feat(openagent): /agents/reload endpoint for hot reload"
```

---

## Task 9: Python `AgentForgeSessionManager` Plugin

**Files:**
- Create: `src-openagent/openagent_sidecar/plugins/session_manager.py`
- Create: `src-openagent/openagent_sidecar/go_client.py`
- Create: `src-openagent/tests/test_session_manager.py`

- [ ] **Step 1: Implement Go HTTP client**

`src-openagent/openagent_sidecar/go_client.py`:

```python
from __future__ import annotations
import httpx


class GoInternalClient:
    def __init__(self, base_url: str, token: str):
        self._url = base_url.rstrip("/")
        self._token = token
        self._client = httpx.AsyncClient(timeout=10.0)

    def _headers(self) -> dict[str, str]:
        h = {"Content-Type": "application/json"}
        if self._token:
            h["Authorization"] = f"Bearer {self._token}"
        return h

    async def append_transcript(self, session_id: str, message: dict) -> None:
        r = await self._client.post(
            f"{self._url}/api/v1/internal/sessions/{session_id}/transcript",
            json=message, headers=self._headers(),
        )
        r.raise_for_status()

    async def save_artifact(self, session_id: str, artifact: dict) -> None:
        r = await self._client.post(
            f"{self._url}/api/v1/internal/sessions/{session_id}/artifacts",
            json=artifact, headers=self._headers(),
        )
        r.raise_for_status()

    async def invoke_tool(self, payload: dict) -> dict:
        r = await self._client.post(
            f"{self._url}/api/v1/internal/tools/invoke",
            json=payload, headers=self._headers(),
        )
        r.raise_for_status()
        return r.json()

    async def close(self) -> None:
        await self._client.aclose()
```

- [ ] **Step 2: Write session manager test (with respx mock)**

`src-openagent/tests/test_session_manager.py`:

```python
import pytest
import respx
from httpx import Response

from openagent_sidecar.go_client import GoInternalClient
from openagent_sidecar.plugins.session_manager import AgentForgeSessionManager


@pytest.mark.asyncio
@respx.mock
async def test_append_message_posts_to_go():
    route = respx.post("http://go/api/v1/internal/sessions/sid/transcript").mock(
        return_value=Response(200, json={"ok": True})
    )
    go = GoInternalClient("http://go", "tok")
    mgr = AgentForgeSessionManager(go, use_local_checkpoints=False)

    await mgr.append_message("sid", {"role": "user", "content": "hi"})
    assert route.called


@pytest.mark.asyncio
async def test_get_state_returns_empty_dict_for_unknown():
    go = GoInternalClient("http://go", "tok")
    mgr = AgentForgeSessionManager(go, use_local_checkpoints=False)
    state = await mgr.get_state("nonexistent")
    assert state == {}
```

- [ ] **Step 3: Implement AgentForgeSessionManager**

`src-openagent/openagent_sidecar/plugins/session_manager.py`:

```python
from __future__ import annotations
from contextlib import asynccontextmanager
from typing import AsyncIterator, Any

from openagents.interfaces.session import SessionManagerPlugin
from openagent_sidecar.go_client import GoInternalClient


class AgentForgeSessionManager(SessionManagerPlugin):
    """Proxies transcript/artifact writes to Go; keeps an in-memory scratch
    for per-run state since OpenAgents runtime uses session() as an async
    context for locking + state carrying."""

    def __init__(self, go: GoInternalClient, use_local_checkpoints: bool = True):
        self._go = go
        self._states: dict[str, dict[str, Any]] = {}
        self._use_local_checkpoints = use_local_checkpoints

    @asynccontextmanager
    async def session(self, session_id: str) -> AsyncIterator[dict[str, Any]]:
        state = self._states.setdefault(session_id, {})
        try:
            yield state
        finally:
            pass  # no lock release; single-run per sidecar

    async def get_state(self, session_id: str) -> dict[str, Any]:
        return self._states.get(session_id, {})

    async def set_state(self, session_id: str, state: dict[str, Any]) -> None:
        self._states[session_id] = state

    async def append_message(self, session_id: str, message: dict) -> None:
        await self._go.append_transcript(session_id, message)

    async def load_messages(self, session_id: str) -> list[dict]:
        # Optional: GET from Go if needed; MVP returns in-memory only.
        return self._states.get(session_id, {}).get("_transcript", [])

    async def save_artifact(self, session_id: str, artifact: dict) -> None:
        await self._go.save_artifact(session_id, artifact)

    async def create_checkpoint(self, session_id: str, checkpoint_id: str):
        # Local SQLite handling deferred; MVP raises NotImplementedError if called.
        raise NotImplementedError("checkpoint deferred to phase 2")

    async def load_checkpoint(self, session_id: str, checkpoint_id: str):
        return None
```

Adjust method signatures to match OpenAgents `SessionManagerPlugin` exactly; consult `openagents/interfaces/session.py` if any abstract method missing raises at runtime.

- [ ] **Step 4: Run tests**

```bash
cd src-openagent && uv run pytest tests/test_session_manager.py -v
```

- [ ] **Step 5: Commit**

```bash
rtk git add src-openagent/openagent_sidecar/plugins/session_manager.py src-openagent/openagent_sidecar/go_client.py src-openagent/tests/test_session_manager.py
rtk git commit -m "feat(openagent): AgentForgeSessionManager proxies transcript/artifacts to Go"
```

---

## Task 10: Go `/api/v1/internal/sessions/*` Endpoints

**Files:**
- Create: `src-go/internal/handler/internal_handler.go`
- Create: `src-go/internal/handler/internal_handler_test.go`
- Modify: `src-go/internal/server/routes.go`
- Modify: `src-go/internal/middleware/auth.go` (or wherever token middleware lives)

- [ ] **Step 1: Write handler test**

`src-go/internal/handler/internal_handler_test.go`:

```go
package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"agentforge/src-go/internal/handler"
)

func TestInternalTranscriptAppendRequiresToken(t *testing.T) {
	fakeSvc := &fakeSessionService{}
	h := handler.NewInternalHandler(fakeSvc, nil, "secret-token")
	e := echo.New()
	e.POST("/api/v1/internal/sessions/:id/transcript", h.AppendTranscript, h.AuthMiddleware())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/sessions/sid/transcript",
		bytes.NewReader([]byte(`{"role":"user","content":"hi"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, 401, rec.Code)
}

func TestInternalTranscriptAppendSucceeds(t *testing.T) {
	fakeSvc := &fakeSessionService{}
	h := handler.NewInternalHandler(fakeSvc, nil, "secret-token")
	e := echo.New()
	e.POST("/api/v1/internal/sessions/:id/transcript", h.AppendTranscript, h.AuthMiddleware())

	req := httptest.NewRequest(http.MethodPost, "/api/v1/internal/sessions/sid/transcript",
		bytes.NewReader([]byte(`{"role":"user","content":"hi"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret-token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, 1, fakeSvc.AppendCount)
	require.Equal(t, "sid", fakeSvc.LastSessionID)
}

type fakeSessionService struct {
	AppendCount   int
	LastSessionID string
}

func (f *fakeSessionService) AppendTranscript(sessionID string, message map[string]any) error {
	f.AppendCount++
	f.LastSessionID = sessionID
	return nil
}

func (f *fakeSessionService) SaveArtifact(sessionID string, artifact map[string]any) error {
	return nil
}
```

- [ ] **Step 2: Implement handler**

`src-go/internal/handler/internal_handler.go`:

```go
package handler

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

type InternalSessionService interface {
	AppendTranscript(sessionID string, message map[string]any) error
	SaveArtifact(sessionID string, artifact map[string]any) error
}

type InternalToolService interface {
	InvokeTool(payload map[string]any) (map[string]any, error)
}

type InternalHandler struct {
	sessions InternalSessionService
	tools    InternalToolService
	token    string
}

func NewInternalHandler(sessions InternalSessionService, tools InternalToolService, token string) *InternalHandler {
	return &InternalHandler{sessions: sessions, tools: tools, token: token}
}

func (h *InternalHandler) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth := c.Request().Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" || token != h.token {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid internal token")
			}
			// also require loopback
			host := c.Request().Host
			if !isLoopback(host) {
				return echo.NewHTTPError(http.StatusForbidden, "internal endpoints only on loopback")
			}
			return next(c)
		}
	}
}

func isLoopback(hostport string) bool {
	// strip port
	h := hostport
	if i := strings.LastIndex(h, ":"); i >= 0 {
		h = h[:i]
	}
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func (h *InternalHandler) AppendTranscript(c echo.Context) error {
	sid := c.Param("id")
	var msg map[string]any
	if err := c.Bind(&msg); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.sessions.AppendTranscript(sid, msg); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *InternalHandler) SaveArtifact(c echo.Context) error {
	sid := c.Param("id")
	var art map[string]any
	if err := c.Bind(&art); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := h.sessions.SaveArtifact(sid, art); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, map[string]any{"ok": true})
}

func (h *InternalHandler) InvokeTool(c echo.Context) error {
	var payload map[string]any
	if err := c.Bind(&payload); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	result, err := h.tools.InvokeTool(payload)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, result)
}
```

- [ ] **Step 3: Register routes**

In `src-go/internal/server/routes.go`:

```go
internal := e.Group("/api/v1/internal", internalH.AuthMiddleware())
internal.POST("/sessions/:id/transcript", internalH.AppendTranscript)
internal.POST("/sessions/:id/artifacts", internalH.SaveArtifact)
internal.POST("/tools/invoke", internalH.InvokeTool)
```

- [ ] **Step 4: Implement session service backed by repo**

In a new file or append to existing session service: implement `AppendTranscript` / `SaveArtifact` using existing repository methods that handle message persistence. Reuse whatever `messageRepo` or equivalent already exists for Bridge flow.

- [ ] **Step 5: Run tests**

```bash
cd src-go && go test ./internal/handler/... -run TestInternal -v
```

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/handler/internal_handler.go src-go/internal/handler/internal_handler_test.go src-go/internal/server/routes.go
rtk git commit -m "feat(internal-api): /api/v1/internal/sessions/* with loopback + token auth"
```

---

## Task 11: Python `AgentForgeToolProxy` Plugin

**Files:**
- Create: `src-openagent/openagent_sidecar/plugins/tool_proxy.py`
- Create: `src-openagent/tests/test_tool_proxy.py`

- [ ] **Step 1: Write tool proxy test**

`src-openagent/tests/test_tool_proxy.py`:

```python
import pytest
import respx
from httpx import Response

from openagent_sidecar.go_client import GoInternalClient
from openagent_sidecar.plugins.tool_proxy import AgentForgeToolProxy


@pytest.mark.asyncio
@respx.mock
async def test_tool_proxy_forwards_to_go():
    respx.post("http://go/api/v1/internal/tools/invoke").mock(
        return_value=Response(200, json={"ok": True, "result": {"answer": "42"}})
    )
    go = GoInternalClient("http://go", "tok")
    proxy = AgentForgeToolProxy(
        tool_id="web_search",
        go=go,
        task_id="t1",
        session_id="s1",
        role_id="r",
    )

    result = await proxy.invoke({"q": "life"}, context=None)
    assert result == {"answer": "42"}
```

- [ ] **Step 2: Implement proxy**

`src-openagent/openagent_sidecar/plugins/tool_proxy.py`:

```python
from __future__ import annotations
from typing import Any

from openagents.interfaces.tool import ToolPlugin
from openagent_sidecar.go_client import GoInternalClient


class AgentForgeToolProxy(ToolPlugin):
    def __init__(self, tool_id: str, go: GoInternalClient, task_id: str, session_id: str, role_id: str):
        self.name = tool_id
        self._tool_id = tool_id
        self._go = go
        self._task_id = task_id
        self._session_id = session_id
        self._role_id = role_id

    async def invoke(self, params: dict[str, Any], context: Any) -> Any:
        payload = {
            "tool_id": self._tool_id,
            "params": params,
            "run_context": {
                "task_id": self._task_id,
                "session_id": self._session_id,
                "role_id": self._role_id,
            },
        }
        response = await self._go.invoke_tool(payload)
        if not response.get("ok", False):
            raise RuntimeError(response.get("error", "tool invocation failed"))
        return response.get("result")

    def schema(self) -> dict:
        return {"type": "object", "additionalProperties": True}
```

- [ ] **Step 3: Register proxy with OpenAgents' plugin impl loader**

Since the mapper puts `"impl": "agentforge_tool_proxy"` in agent.json, we need that impl path resolvable. Add to the sidecar startup a registration:

In `src-openagent/main.py` (inside `create_app`):

```python
from openagents.plugins.registry import register_tool_impl

def _make_proxy_factory(go_client: GoInternalClient):
    def factory(config: dict, context: dict) -> AgentForgeToolProxy:
        return AgentForgeToolProxy(
            tool_id=config["tool_id"],
            go=go_client,
            task_id=context.get("task_id", ""),
            session_id=context.get("session_id", ""),
            role_id=context.get("role_id", ""),
        )
    return factory

# register_tool_impl("agentforge_tool_proxy", _make_proxy_factory(go_client))
```

Verify the actual registration API in OpenAgents (`openagents/plugins/loader.py` / `openagents/plugins/registry.py`) — the above is a sketch; real method name may be `register_plugin_impl` or similar. If the loader supports dotted-path import instead of a registry, make `agentforge_tool_proxy` resolvable as a dotted path and pass that as `impl` in agent.json.

- [ ] **Step 4: Run proxy test**

```bash
cd src-openagent && uv run pytest tests/test_tool_proxy.py -v
```

- [ ] **Step 5: Commit**

```bash
rtk git add src-openagent/openagent_sidecar/plugins/tool_proxy.py src-openagent/tests/test_tool_proxy.py src-openagent/main.py
rtk git commit -m "feat(openagent): AgentForgeToolProxy forwards invocations to Go"
```

---

## Task 12: Go `/api/v1/internal/tools/invoke` → Bridge Forwarding

**Files:**
- Create: `src-go/internal/service/tool_invoke_service.go`
- Create: `src-go/internal/service/tool_invoke_service_test.go`
- Modify: `src-go/internal/bridge/client.go` (add InvokeTool if missing)
- Modify: `src-go/cmd/server/main.go`

- [ ] **Step 1: Write service test**

`src-go/internal/service/tool_invoke_service_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"agentforge/src-go/internal/service"
)

type fakeBridgeTools struct{ Called int; LastToolID string }

func (f *fakeBridgeTools) InvokeTool(_ context.Context, toolID string, params map[string]any) (map[string]any, error) {
	f.Called++; f.LastToolID = toolID
	return map[string]any{"answer": "42"}, nil
}

func TestToolInvokeServiceChecksBudget(t *testing.T) {
	bridge := &fakeBridgeTools{}
	svc := service.NewToolInvokeService(bridge, &noopRBAC{}, &unlimitedBudget{})
	result, err := svc.InvokeTool(context.Background(), map[string]any{
		"tool_id": "web_search",
		"params":  map[string]any{"q": "x"},
		"run_context": map[string]any{"task_id": "t1", "session_id": "s1"},
	})
	require.NoError(t, err)
	require.Equal(t, 1, bridge.Called)
	require.Equal(t, true, result["ok"])
}

func TestToolInvokeServiceDeniesWhenRBACFails(t *testing.T) {
	bridge := &fakeBridgeTools{}
	svc := service.NewToolInvokeService(bridge, &denyRBAC{}, &unlimitedBudget{})
	_, err := svc.InvokeTool(context.Background(), map[string]any{
		"tool_id": "web_search", "run_context": map[string]any{"task_id": "t1"},
	})
	require.ErrorContains(t, err, "rbac")
}

type noopRBAC struct{}

func (noopRBAC) CanInvokeTool(_ context.Context, _ string, _ string) error { return nil }

type denyRBAC struct{}

func (denyRBAC) CanInvokeTool(_ context.Context, _ string, _ string) error {
	return errors.New("rbac denied")
}

type unlimitedBudget struct{}

func (unlimitedBudget) CheckCanProceed(_ context.Context, _ string) error { return nil }
```

- [ ] **Step 2: Implement service**

`src-go/internal/service/tool_invoke_service.go`:

```go
package service

import (
	"context"
	"errors"
	"fmt"
)

type BridgeToolInvoker interface {
	InvokeTool(ctx context.Context, toolID string, params map[string]any) (map[string]any, error)
}

type RBACChecker interface {
	CanInvokeTool(ctx context.Context, taskID string, toolID string) error
}

type BudgetChecker interface {
	CheckCanProceed(ctx context.Context, taskID string) error
}

type ToolInvokeService struct {
	bridge BridgeToolInvoker
	rbac   RBACChecker
	budget BudgetChecker
}

func NewToolInvokeService(bridge BridgeToolInvoker, rbac RBACChecker, budget BudgetChecker) *ToolInvokeService {
	return &ToolInvokeService{bridge: bridge, rbac: rbac, budget: budget}
}

func (s *ToolInvokeService) InvokeTool(ctx context.Context, payload map[string]any) (map[string]any, error) {
	toolID, _ := payload["tool_id"].(string)
	if toolID == "" {
		return nil, errors.New("tool_id required")
	}
	runCtx, _ := payload["run_context"].(map[string]any)
	taskID, _ := runCtx["task_id"].(string)
	params, _ := payload["params"].(map[string]any)

	if err := s.rbac.CanInvokeTool(ctx, taskID, toolID); err != nil {
		return map[string]any{"ok": false, "error": fmt.Sprintf("rbac: %v", err)}, fmt.Errorf("rbac: %w", err)
	}
	if err := s.budget.CheckCanProceed(ctx, taskID); err != nil {
		return map[string]any{"ok": false, "error": fmt.Sprintf("budget: %v", err)}, fmt.Errorf("budget: %w", err)
	}

	result, err := s.bridge.InvokeTool(ctx, toolID, params)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, err
	}
	return map[string]any{"ok": true, "result": result}, nil
}
```

- [ ] **Step 3: Add Bridge.InvokeTool if missing**

Check `src-go/internal/bridge/client.go` for existing tool-invocation method. If absent, add:

```go
func (c *Client) InvokeTool(ctx context.Context, toolID string, params map[string]any) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{
		"tool_id": toolID,
		"params":  params,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bridge/tools/invoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("bridge invoke tool: %d", resp.StatusCode)
	}
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}
```

Verify Bridge endpoint name (`/bridge/tools/invoke` vs `/bridge/tools/call` vs similar) by reading `src-bridge/src/handlers/tools/*.ts`.

- [ ] **Step 4: Wire in main.go**

```go
toolSvc := service.NewToolInvokeService(bridgeClient, rbacChecker, budgetChecker)
internalH := handler.NewInternalHandler(sessionSvc, toolSvc, cfg.InternalToken)
```

- [ ] **Step 5: Run tests**

```bash
cd src-go && go test ./internal/service/... -run TestToolInvoke -v
```

- [ ] **Step 6: Commit**

```bash
rtk git add src-go/internal/service/tool_invoke_service.go src-go/internal/service/tool_invoke_service_test.go src-go/internal/bridge/client.go src-go/cmd/server/main.go
rtk git commit -m "feat(internal-api): tool invocation service with RBAC+budget → bridge forward"
```

---

## Task 13: Cost Token Forwarding + Go USD Calculation

**Files:**
- Modify: `src-openagent/openagent_sidecar/event_translator.py` (already handles strip)
- Modify: `src-go/internal/cost/*` (extend to accept token-only events)
- Modify: `src-go/internal/ws/hub.go` or event processor to recognize `cost_update` events from openagent

- [ ] **Step 1: Write Go cost computation test**

Find existing cost service test file (`src-go/internal/cost/service_test.go` or similar). Add:

```go
func TestComputeUSDFromTokens(t *testing.T) {
	prices := cost.PriceTable{
		"claude-3-5-sonnet-20241022": {Input: 3.00, Output: 15.00, CachedRead: 0.30},
	}
	svc := cost.NewService(prices)
	usd := svc.ComputeUSD("claude-3-5-sonnet-20241022", cost.TokenUsage{
		InputTokens: 1_000_000, OutputTokens: 500_000, CachedReadTokens: 100_000,
	})
	require.InDelta(t, 3.00+7.50+0.03, usd, 0.0001)
}
```

- [ ] **Step 2: Implement/augment cost service**

If absent, add to `src-go/internal/cost/service.go`:

```go
type Price struct {
	Input       float64 // per 1M tokens
	Output      float64
	CachedRead  float64
	CachedWrite float64
}

type TokenUsage struct {
	InputTokens       int
	OutputTokens      int
	CachedReadTokens  int
	CachedWriteTokens int
}

type PriceTable map[string]Price

type Service struct {
	prices PriceTable
}

func NewService(p PriceTable) *Service {
	return &Service{prices: p}
}

func (s *Service) ComputeUSD(model string, u TokenUsage) float64 {
	p, ok := s.prices[model]
	if !ok {
		return 0
	}
	perMillion := func(tokens int, rate float64) float64 {
		return float64(tokens) / 1_000_000.0 * rate
	}
	return perMillion(u.InputTokens, p.Input) +
		perMillion(u.OutputTokens, p.Output) +
		perMillion(u.CachedReadTokens, p.CachedRead) +
		perMillion(u.CachedWriteTokens, p.CachedWrite)
}
```

- [ ] **Step 3: Wire into WS event handler**

In the Go WS event ingress (where Bridge events are already processed), detect `cost_update` events without `cost_usd` field, compute via `costSvc.ComputeUSD(model, tokens)`, then call the existing cost aggregation path.

Pseudocode (find the actual handler location by searching for "cost_update" in `src-go/`):

```go
case "cost_update":
    tokens := decodeTokens(ev.Data)
    if _, ok := ev.Data["cost_usd"]; !ok {
        model := resolveModelForTask(ev.TaskID)
        usd := s.cost.ComputeUSD(model, tokens)
        ev.Data["cost_usd"] = usd
    }
    s.recordCostUpdate(ev)
```

- [ ] **Step 4: Run tests**

```bash
cd src-go && go test ./internal/cost/... -v
```

- [ ] **Step 5: Commit**

```bash
rtk git add src-go/internal/cost/ src-go/internal/ws/
rtk git commit -m "feat(cost): compute USD from token-only cost_update events (openagent path)"
```

---

## Task 14: Conditional Worktree Allocation

**Files:**
- Modify: `src-go/internal/service/agent_service.go` (or wherever `workTreeManager.Prepare` is called)

- [ ] **Step 1: Write test**

In `src-go/internal/service/agent_service_openagent_test.go` add:

```go
func TestSpawnSkipsWorktreeWhenRequiresWorktreeFalse(t *testing.T) {
	fakeBridge := &fakeBridgeClient{}
	fakePython := &fakePythonClient{}
	fakeWT := &fakeWorktreeManager{}
	svc := newTestAgentServiceWithWT(t, fakeBridge, fakePython, fakeWT, roleWith{
		ID: "planner", Engine: "openagent", Pattern: "react", RequiresWT: false,
	})
	_, err := svc.Spawn(context.Background(), service.DispatchSpawnInput{
		TaskID: uuid.New(), RoleID: "planner", Caller: service.Caller{UserID: uuid.New()},
	})
	require.NoError(t, err)
	require.Equal(t, 0, fakeWT.PrepareCount)
}

func TestSpawnAllocatesWorktreeWhenRequiresWorktreeTrue(t *testing.T) {
	// same but RequiresWT: true — expect PrepareCount == 1
}
```

- [ ] **Step 2: Branch in AgentService.Spawn**

In the spawn path, before calling `WorktreeManager.Prepare`:

```go
if manifest.RequiresWorktree {
	wtInfo, err = s.workTreeManager.Prepare(ctx, task)
	if err != nil {
		return dispatchResult{}, err
	}
}
// else: pass empty worktree path
```

If openagent request includes worktree: `pyReq.KnowledgeCtx = fmt.Sprintf("worktree_path=%s", wtInfo.Path)` or set a proper context_hints field.

- [ ] **Step 3: Run tests**

```bash
cd src-go && go test ./internal/service/... -run TestSpawn -v
```

- [ ] **Step 4: Commit**

```bash
rtk git add src-go/internal/service/agent_service.go src-go/internal/service/agent_service_openagent_test.go
rtk git commit -m "feat(agent): skip worktree allocation when role.requires_worktree=false"
```

---

## Task 15: Dev Mode Scripts

**Files:**
- Modify: `package.json`
- Modify: `scripts/dev/dev-all.js`
- Create: `scripts/dev/openagent.mjs` (if service registry uses ESM fragments)

- [ ] **Step 1: Add pnpm scripts**

Edit `package.json` scripts:

```json
"dev:openagent": "cd src-openagent && uv run uvicorn main:app --reload --port 7780 --host 127.0.0.1",
"dev:openagent:install": "cd src-openagent && uv venv && uv pip install -e \".[dev]\"",
"build:backend:openagent": "node scripts/build/build-openagent.js",
"build:backend:openagent:dev": "node scripts/build/build-openagent.js --current-only"
```

- [ ] **Step 2: Register openagent as a managed service in dev-all.js**

Read the existing `scripts/dev/dev-all.js` to understand how bridge/im-bridge/go are registered. Add an analogous entry:

```js
{
  name: "openagent",
  cwd: path.join(repoRoot, "src-openagent"),
  command: "uv",
  args: ["run", "uvicorn", "main:app", "--port", "7780", "--host", "127.0.0.1"],
  env: {
    OPENAGENT_PORT: "7780",
    AGENTFORGE_GO_URL: "http://127.0.0.1:7777",
    AGENTFORGE_GO_WS: "ws://127.0.0.1:7777/ws/bridge",
    AGENTFORGE_INTERNAL_TOKEN: sharedInternalToken,
  },
  healthUrl: "http://127.0.0.1:7780/health",
  startupTimeoutMs: 20_000,
}
```

Add to the `backend` service group so `pnpm dev:backend` spins it up alongside go/bridge/im-bridge.

Share `sharedInternalToken` — generate at dev-all startup, inject into both Go (`AGENTFORGE_INTERNAL_TOKEN`) and Python (same env), so the internal endpoints auth works.

- [ ] **Step 3: Test**

```bash
pnpm dev:openagent:install
pnpm dev:openagent &
sleep 3
curl http://127.0.0.1:7780/health
kill %1
```

Expected: `{"status":"ok", ...}`.

- [ ] **Step 4: Commit**

```bash
rtk git add package.json scripts/dev/
rtk git commit -m "feat(dev): add openagent sidecar to pnpm dev scripts and dev-all"
```

---

## Task 16: PyInstaller Build + Tauri Sidecar

**Files:**
- Create: `scripts/build/build-openagent.js`
- Create: `src-openagent/build.spec` (PyInstaller spec)
- Modify: `src-tauri/tauri.conf.json`
- Modify: `scripts/build/build-backend.js`

- [ ] **Step 1: Create PyInstaller spec**

`src-openagent/build.spec`:

```python
# -*- mode: python ; coding: utf-8 -*-
block_cipher = None

a = Analysis(
    ['main.py'],
    pathex=[],
    binaries=[],
    datas=[],
    hiddenimports=[
        'openagent_sidecar',
        'openagent_sidecar.plugins.session_manager',
        'openagent_sidecar.plugins.tool_proxy',
        'openagent_sidecar.plugins.event_forwarder',
        'openagents',
        'uvicorn.logging',
        'uvicorn.protocols',
        'uvicorn.protocols.http',
        'uvicorn.protocols.http.auto',
        'uvicorn.protocols.websockets',
        'uvicorn.protocols.websockets.auto',
        'uvicorn.lifespan.on',
    ],
    hookspath=[],
    runtime_hooks=[],
    excludes=[],
    win_no_prefer_redirects=False,
    win_private_assemblies=False,
    cipher=block_cipher,
    noarchive=False,
)

pyz = PYZ(a.pure, a.zipped_data, cipher=block_cipher)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.zipfiles,
    a.datas,
    [],
    name='openagent-runtime',
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=False,
    runtime_tmpdir=None,
    console=True,
    disable_windowed_traceback=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
```

- [ ] **Step 2: Create build script**

`scripts/build/build-openagent.js`:

```js
const { execSync } = require("node:child_process");
const path = require("node:path");
const fs = require("node:fs");

const repoRoot = path.resolve(__dirname, "..", "..");
const srcDir = path.join(repoRoot, "src-openagent");
const outDir = path.join(repoRoot, "src-tauri", "binaries");

function run(cmd, cwd) {
  console.log(`[build-openagent] $ ${cmd}`);
  execSync(cmd, { cwd, stdio: "inherit" });
}

function main() {
  // Ensure venv + pyinstaller
  run("uv venv", srcDir);
  run("uv pip install -e .", srcDir);
  run("uv pip install pyinstaller>=6.0", srcDir);

  // Build
  run("uv run pyinstaller --onefile --clean build.spec", srcDir);

  // Copy to src-tauri/binaries with triple suffix Tauri expects
  const triple = process.env.TARGET_TRIPLE || detectTriple();
  const ext = process.platform === "win32" ? ".exe" : "";
  const src = path.join(srcDir, "dist", `openagent-runtime${ext}`);
  const dst = path.join(outDir, `openagent-runtime-${triple}${ext}`);
  fs.mkdirSync(outDir, { recursive: true });
  fs.copyFileSync(src, dst);
  console.log(`[build-openagent] copied → ${dst}`);
}

function detectTriple() {
  // Reuse logic from existing build-backend.js
  const os = process.platform;
  const arch = process.arch;
  const m = {
    "win32:x64": "x86_64-pc-windows-msvc",
    "darwin:x64": "x86_64-apple-darwin",
    "darwin:arm64": "aarch64-apple-darwin",
    "linux:x64": "x86_64-unknown-linux-gnu",
  }[`${os}:${arch}`];
  if (!m) throw new Error(`no triple for ${os}/${arch}`);
  return m;
}

main();
```

- [ ] **Step 3: Add to Tauri externalBin**

Edit `src-tauri/tauri.conf.json` `bundle.externalBin` array:

```json
"externalBin": [
  "binaries/go-orchestrator",
  "binaries/bridge",
  "binaries/im-bridge",
  "binaries/openagent-runtime"
]
```

- [ ] **Step 4: Add sidecar spawn to Tauri Rust code**

In `src-tauri/src/` where existing sidecars are spawned (e.g., `sidecars.rs`), add entry for `openagent-runtime`. Copy the pattern from the bridge entry (port/env/stdin handling).

- [ ] **Step 5: Extend build-backend.js to include openagent**

In `scripts/build/build-backend.js`, after go/bridge/im-bridge builds:

```js
if (!currentOnly || currentOnly) {
  execSync("node scripts/build/build-openagent.js" + (currentOnly ? " --current-only" : ""), {
    cwd: repoRoot, stdio: "inherit"
  });
}
```

- [ ] **Step 6: Smoke test the build on current platform**

```bash
pnpm build:backend:openagent:dev
ls src-tauri/binaries/ | grep openagent-runtime
./src-tauri/binaries/openagent-runtime* &
sleep 5
curl http://127.0.0.1:7780/health
kill %1
```

Expected: binary exists, starts, `/health` returns 200.

- [ ] **Step 7: Commit**

```bash
rtk git add scripts/build/build-openagent.js src-openagent/build.spec src-tauri/tauri.conf.json src-tauri/src/ scripts/build/build-backend.js
rtk git commit -m "feat(build): PyInstaller package openagent-runtime as Tauri sidecar"
```

---

## Task 17: End-to-End Verification

**Files:**
- Create: `src-openagent/tests/test_e2e_smoke.py` (optional, integration)
- Manual verification script below

- [ ] **Step 1: Author smoke test procedure**

Document in `src-openagent/README.md`:

```markdown
## E2E Smoke Test

1. Start Go + Bridge + OpenAgent in dev mode: `pnpm dev:backend`
2. Create a role:

   ```bash
   curl -XPOST http://127.0.0.1:7777/api/v1/roles -d '{
     "id": "planner-test",
     "name": "Planner Test",
     "execution_engine": "openagent",
     "openagent_pattern": "react",
     "goal": "test",
     "backstory": "smoke",
     "llm_config": {"provider": "mock", "model": "mock"}
   }'
   ```

3. Trigger reload: `curl -XPOST http://127.0.0.1:7780/agents/reload`
4. Verify loaded: `curl http://127.0.0.1:7780/agents`
5. Spawn an agent: `curl -XPOST http://127.0.0.1:7777/api/v1/agents/spawn -d '{"taskId":"...","roleId":"planner-test"}'`
6. Watch WS events flowing to Go (via `/ws/bridge` subscribers)
7. Verify transcript appears in Postgres `messages` table
```

- [ ] **Step 2: Run manual smoke**

Execute the steps above. If any step fails, fix and create a task.

- [ ] **Step 3: Commit docs**

```bash
rtk git add src-openagent/README.md
rtk git commit -m "docs(openagent): E2E smoke test procedure"
```

---

## Self-Review Notes

Issues to watch during implementation (documented here instead of left as TODO-in-plan):

1. **OpenAgents SDK API drift**: Task 4 / 5 / 9 / 11 assume specific `SessionManagerPlugin` / `EventBusPlugin` / `ToolPlugin` method signatures. Before starting each task, open the actual file in `openagent-python-sdk/openagents/interfaces/*.py` and verify. Signature diffs should be fixed inline, not added to a follow-up.

2. **Mock LLM provider response shape**: Task 4's test uses `{"provider": "mock", "model": "mock"}` with a response list. The OpenAgents mock provider may want a different schema — check `openagents/llm/providers/mock.py` and adjust the fixture.

3. **Bridge tool invoke endpoint path**: Task 12 assumes `/bridge/tools/invoke`. Confirm in `src-bridge/src/handlers/*` — it may be `/bridge/tools/call` or `/bridge/plugins/:id/tools/:name/invoke`. Align before writing the Go `InvokeTool` method.

4. **Internal token generation**: Task 10 and Task 15 both expect `AGENTFORGE_INTERNAL_TOKEN`. Generation happens once at process-group startup (dev-all.js generates, Go reads from env, Python reads from env). For Tauri production: generate at Tauri startup via `rand::thread_rng`, pass via sidecar env.

5. **Session manager abstract methods**: Task 9's session manager may fail at Runtime instantiation if it doesn't implement every abstract method. Verify against `openagents/interfaces/session.py` — add stub implementations for any missing method before running the first `/run` call.

6. **Streaming response lifetime**: Task 6's Go client drains the NDJSON body into a discard buffer. In practice Python sends events via WS; Go just needs the HTTP 200 ack to know the run started. If the `/run` response closes prematurely because Python errors before emitting the first event, the Go caller won't see the failure — consider adding a short "run_started" marker on the first NDJSON line and reading only that before discarding.

7. **Worktree path in context_hints**: Task 14 sketches `context_hints={"worktree_path": ...}`. Real OpenAgents tools would need to read this hint. For MVP, worktree-using roles won't exist on openagent engine yet — leave the field populated for future use, but don't build tools that depend on it in this plan.

---

## Task Dependency Graph

```
1 (role fields) ──┬─► 2 (dispatcher)
                  └─► 7 (mapper)
3 (scaffold) ──► 4 (/run) ──► 5 (events)
                     │           │
                     ├───────────┘
                     ▼
                     6 (Go client)
                     │
                     └──► 8 (/agents/reload)
                          └──► 9 (session mgr) ──► 10 (/internal/sessions)
                               └──► 11 (tool proxy) ──► 12 (/internal/tools)
                                    └──► 13 (cost)
                                         └──► 14 (worktree)
                                              └──► 15 (dev scripts)
                                                   └──► 16 (build) ──► 17 (E2E)
```

Tasks 1–2 and 3–5 can run in parallel (Go vs Python). Task 6 requires both 2 and 5.
