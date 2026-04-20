# Spec 3C — Qianchuan Strategy Authoring (YAML schema + library CRUD + FE editor)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 落地 Spec 3 声明式策略：YAML schema + 解析 + 校验 + 库 CRUD + 系统种子策略 + FE Monaco 编辑器 + 测试面板。

**Architecture:** YAML schema 严格枚举 6 个 action 类型（避免 raw JS 沙箱风险）+ 表达式复用 function 节点求值器 + 草稿/发布/归档三态 + 发布后不可改（必须新版本）+ 系统策略种子 + FE Monaco 编辑器带服务端校验标记 + dry-run test 面板。

**Tech Stack:** Go (gopkg.in/yaml.v3 + 现有表达式求值器), Postgres jsonb, Next.js + @monaco-editor/react, Zustand.

**Depends on:** none directly (touches no Spec 1/2/3-other tables)

**Parallel with:** 3A, 3B — completely independent

**Unblocks:** 3D（DAG 中的 function 节点解析策略 ParsedSpec 并 emit actions）

---

## Coordination notes (read before starting)

- **Migration number**: 067 is currently used by `secrets` (Plan 1B). If 1B and any 3A/3B migrations land first, this plan's number shifts up — pick the next free slot at implementation time. Currently treat as `0XX_create_qianchuan_strategies` — confirm number before writing the file.
- **Implementation note 2026-04-20**: per-user directive (parallel plans 1A using 067/068, 1B using 069), this plan landed migration as `070_create_qianchuan_strategies`.
- **Action allowlist is load-bearing**: the 6 action types `adjust_bid`, `adjust_budget`, `pause_ad`, `resume_ad`, `apply_material`, `notify_im`, `record_event` are the contract Plan 3D's runtime ships against. Adding/removing types is a cross-plan change.
- **Expression evaluator reuse**: rule conditions MUST be parsed by `nodetypes.EvaluateExpression` (with template-var resolution). DO NOT introduce a new expression engine — Spec §4 decision #3 explicitly chose to reuse it to avoid sandbox surface area. The strategy parser only **pre-validates** that the expression is non-empty and that any `len(path)` it contains looks well-formed; actual evaluation happens at strategy-runtime.
- **Snapshot data shape**: a "snapshot" is the JSON object exposed to expressions as `snapshot.*`. v1 fields used by seed strategies: `snapshot.metrics.cost`, `snapshot.metrics.conversions`, `snapshot.metrics.cost_per_conversion`, `snapshot.metrics.cvr`, `snapshot.window_minutes`, `snapshot.ad_id`, `snapshot.ads[].{id,cost,conversions,cvr}`. Plan 3A/3B finalize the wire shape; this plan just declares which field names rules can reference and rejects unknown roots.
- **Monaco dependency**: `@monaco-editor/react` is NOT in `package.json` today. Plan adds it; pin a known-working version (use the latest stable at implementation time and lock via pnpm).
- **Status transitions**: `draft → published → archived` only (no published → draft, no archived → anything). Edits to published create a NEW row (version=max+1, status=draft, same name).
- **System seeds are project_id=NULL**: handler list endpoint returns BOTH system + project rows; FE displays a `system` badge. System rows are read-only from any handler endpoint.
- **YAML errors carry line/col**: `yaml.v3` decoder errors surface line/col via `yaml.TypeError` and the standard `*yaml.Node`. Wrap in a structured `StrategyParseError{Line, Col, Field, Msg}` for the FE editor markers.

---

## Task 1 — Migration: qianchuan_strategies table

- [x] Step 1.1 — write the up migration
  - File: `src-go/migrations/070_create_qianchuan_strategies.up.sql`
    ```sql
    -- Declarative strategy library. project_id NULL = system seed strategy.
    -- yaml_source = the raw YAML the author saw.
    -- parsed_spec = compiled form (pre-resolved expressions, normalized actions)
    --               cached for fast strategy-runtime use (see internal/qianchuan/strategy).
    -- status: draft → published (immutable) → archived. New version row on edit-after-publish.
    CREATE TABLE IF NOT EXISTS qianchuan_strategies (
        id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        project_id    UUID REFERENCES projects(id) ON DELETE CASCADE,
        name          VARCHAR(128) NOT NULL,
        description   TEXT,
        yaml_source   TEXT NOT NULL,
        parsed_spec   JSONB NOT NULL,
        version       INT NOT NULL DEFAULT 1,
        status        VARCHAR(16) NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft','published','archived')),
        created_by    UUID NOT NULL,
        created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
        updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
        UNIQUE (project_id, name, version)
    );
    CREATE INDEX IF NOT EXISTS qianchuan_strategies_project_idx
        ON qianchuan_strategies(project_id);
    CREATE INDEX IF NOT EXISTS qianchuan_strategies_status_idx
        ON qianchuan_strategies(status);

    CREATE TRIGGER set_qianchuan_strategies_updated_at
        BEFORE UPDATE ON qianchuan_strategies
        FOR EACH ROW
        EXECUTE FUNCTION update_updated_at_column();
    ```

- [x] Step 1.2 — write the down migration
  - File: `src-go/migrations/070_create_qianchuan_strategies.down.sql`
    ```sql
    DROP TRIGGER IF EXISTS set_qianchuan_strategies_updated_at ON qianchuan_strategies;
    DROP INDEX IF EXISTS qianchuan_strategies_status_idx;
    DROP INDEX IF EXISTS qianchuan_strategies_project_idx;
    DROP TABLE IF EXISTS qianchuan_strategies;
    ```

- [x] Step 1.3 — apply + verify (embed.go is glob-based; embed_test.go validates)
  - Run `pnpm dev:backend` (which runs migrations) or invoke the migrate tool directly
  - `psql -c "\d qianchuan_strategies"` shows the columns + check constraint
  - Verify `embed.go` picks up both files (no manual change needed; embed is glob-based — confirm via `embed_test.go`)

---

## Task 2 — Strategy YAML schema (Go structs)

- [x] Step 2.1 — write failing test for the in-memory shape
  - File: `src-go/internal/qianchuan/strategy/schema_test.go`
  - Test asserts the `Strategy` zero-value round-trips through `yaml.Marshal` / `yaml.Unmarshal` and that the action type enum matches the documented allowlist exactly.

- [x] Step 2.2 — implement schema types
  - File: `src-go/internal/qianchuan/strategy/schema.go`
    ```go
    package strategy

    // Strategy is the in-memory form of a strategy YAML manifest.
    type Strategy struct {
        Name        string           `yaml:"name"`
        Description string           `yaml:"description,omitempty"`
        Triggers    StrategyTriggers `yaml:"triggers"`
        Inputs      []StrategyInput  `yaml:"inputs"`
        Rules       []StrategyRule   `yaml:"rules"`
    }
    type StrategyTriggers struct {
        Schedule string `yaml:"schedule"` // "10s" | "1m" | "1h" — parsed via time.ParseDuration; range [10s, 1h]
    }
    type StrategyInput struct {
        Metric    string   `yaml:"metric"`     // e.g. "cost", "conversions", "cvr"
        Dimensions []string `yaml:"dimensions"` // e.g. ["ad_id"]
        Window    string   `yaml:"window"`     // e.g. "1m", "15m"
    }
    type StrategyRule struct {
        Name      string           `yaml:"name"`
        Condition string           `yaml:"condition"` // expression evaluated by nodetypes.EvaluateExpression
        Actions   []StrategyAction `yaml:"actions"`
    }
    type StrategyAction struct {
        Type   string         `yaml:"type"`   // see ActionTypes allowlist
        Target StrategyTarget `yaml:"target"`
        Params map[string]any `yaml:"params"`
    }
    type StrategyTarget struct {
        AdIDExpr string `yaml:"ad_id_expr,omitempty"` // expression resolving to a Qianchuan ad id
    }

    // ActionTypes is the load-bearing allowlist. Plan 3D's runtime ships against it.
    var ActionTypes = []string{
        "adjust_bid", "adjust_budget", "pause_ad", "resume_ad",
        "apply_material", "notify_im", "record_event",
    }
    ```

- [x] Step 2.3 — green the test
  - Run `cd src-go && go test ./internal/qianchuan/strategy/...`
  - All passes

---

## Task 3 — ParsedSpec (compiled form for runtime)

- [x] Step 3.1 — write failing test for ParsedSpec round-trip
  - File: `src-go/internal/qianchuan/strategy/parsed_spec_test.go`
  - Cases:
    - `ParsedSpec.MarshalJSON` produces stable, sorted-key JSON suitable for jsonb storage
    - A round-trip through `json.Marshal` → `json.Unmarshal` preserves all fields including `ScheduleSeconds`, `Rules[].ConditionRaw`, action params
    - `ParsedSpec.SchemaVersion` is `1` so future format bumps can be detected

- [x] Step 3.2 — implement ParsedSpec
  - File: `src-go/internal/qianchuan/strategy/parsed_spec.go`
    ```go
    type ParsedSpec struct {
        SchemaVersion   int             `json:"schema_version"`
        ScheduleSeconds int             `json:"schedule_seconds"`
        Inputs          []ParsedInput   `json:"inputs"`
        Rules           []ParsedRule    `json:"rules"`
    }
    type ParsedInput struct {
        Metric        string   `json:"metric"`
        Dimensions    []string `json:"dimensions"`
        WindowSeconds int      `json:"window_seconds"`
    }
    type ParsedRule struct {
        Name         string          `json:"name"`
        ConditionRaw string          `json:"condition_raw"` // raw expression; runtime calls nodetypes.EvaluateExpression
        Actions      []ParsedAction  `json:"actions"`
    }
    type ParsedAction struct {
        Type     string         `json:"type"`
        AdIDExpr string         `json:"ad_id_expr,omitempty"`
        Params   map[string]any `json:"params"`
    }
    ```
  - No expression AST is precomputed — `EvaluateExpression` is itself fast and stateless. SchemaVersion=1 is the only forward-compat hook.

- [x] Step 3.3 — green the test

---

## Task 4 — Action params validators (per-type)

- [x] Step 4.1 — write failing test table
  - File: `src-go/internal/qianchuan/strategy/action_validators_test.go`
  - Table-driven cases per action type:
    - `adjust_bid`: requires exactly one of `pct` (float -100..100, non-zero) or `to` (positive float); both → reject; neither → reject
    - `adjust_budget`: same shape as `adjust_bid` (`pct` or `to`)
    - `pause_ad` / `resume_ad`: params MUST be empty
    - `apply_material`: requires `material_id` (non-empty string)
    - `notify_im`: requires `channel` (non-empty) AND `template` (non-empty)
    - `record_event`: requires `event_name` (non-empty)
    - Unknown type → reject with clear message

- [x] Step 4.2 — implement
  - File: `src-go/internal/qianchuan/strategy/action_validators.go`
  - Export `ValidateAction(a StrategyAction) error`
  - Internally a `map[string]func(map[string]any) error` keyed by type
  - Errors include the action type for FE display

- [x] Step 4.3 — green

---

## Task 5 — Strategy parser + top-level validation

- [x] Step 5.1 — write failing tests
  - File: `src-go/internal/qianchuan/strategy/parser_test.go`
  - Cases:
    - Well-formed YAML (one rule, `notify_im` action) parses; `ParsedSpec.ScheduleSeconds == 60`
    - Empty `rules: []` rejected with `at least one rule required`
    - `triggers.schedule = "5s"` rejected (below 10s floor)
    - `triggers.schedule = "2h"` rejected (above 1h ceiling)
    - Unknown action type rejected, error carries line/col from yaml.v3
    - Missing required action params rejected; error references the action's name + type
    - Empty `condition` rejected
    - `condition` containing only whitespace rejected
    - Duplicate rule names rejected
    - YAML syntax error (unclosed quote) rejected; error carries line from yaml.v3
    - `name` longer than 128 chars rejected
    - `inputs[].window` parses durations `1m`, `15m`, `1h`; bad value rejected

- [x] Step 5.2 — implement parser
  - File: `src-go/internal/qianchuan/strategy/parser.go`
    ```go
    type StrategyParseError struct {
        Line  int    `json:"line"`
        Col   int    `json:"col"`
        Field string `json:"field"`
        Msg   string `json:"msg"`
    }
    func (e *StrategyParseError) Error() string { /* Field: Msg (line:col) */ }

    // Parse parses YAML, validates structure, and produces both the in-memory
    // Strategy form and the runtime-optimized ParsedSpec.
    func Parse(yamlSource string) (*Strategy, *ParsedSpec, error) { /* ... */ }
    ```
  - Steps inside Parse:
    1. Decode into a `*yaml.Node` first to capture line/col for any later error
    2. Decode again into `Strategy{}`; on yaml.TypeError extract line/col
    3. Validate: name length, schedule duration range, at least 1 rule, unique rule names, condition non-empty, action type in `ActionTypes`, action params via `ValidateAction`
    4. Build `ParsedSpec` (fill `ScheduleSeconds`, `WindowSeconds`, copy expressions)

- [x] Step 5.3 — green all parser tests

---

## Task 6 — Repository + service

- [x] Step 6.1 — write failing repo tests
  - File: `src-go/internal/repository/qianchuan_strategy_repo_test.go`
  - Use the existing test DB harness; cases: insert + get-by-id, list-by-project (excludes other projects, includes system NULL-project rows), version bump on (project_id, name) collision

- [x] Step 6.2 — implement repo
  - File: `src-go/internal/repository/qianchuan_strategy_repo.go`
  - Methods: `Insert`, `GetByID`, `ListByProject(projectID, includeSystem bool)`, `UpdateDraft` (only when current status='draft'), `SetStatus`, `DeleteDraft`, `MaxVersion(projectID, name)`

- [x] Step 6.3 — write failing service tests
  - File: `src-go/internal/service/qianchuan_strategy_service_test.go`
  - Cases:
    - `Create` parses → validates → persists with version=1
    - `Create` of same (project, name) bumps version
    - `Update` rejected when status != draft
    - `Publish` flips draft → published; subsequent `Update` of same row rejected
    - Edit-after-publish creates a NEW row (status=draft, version=max+1, same name)
    - `Delete` only allowed for drafts
    - `TestRun(strategy, snapshot)` returns the actions a single eval would emit (dry-run; no persistence, no policy gate); uses `nodetypes.EvaluateExpression` for the condition

- [x] Step 6.4 — implement service
  - File: `src-go/internal/service/qianchuan_strategy_service.go`
  - Wire the repo + parser; surface structured errors (`StrategyParseError`) so the handler can return JSON-shaped 400s
  - `TestRun` walks `ParsedSpec.Rules` evaluating `ConditionRaw` against `{"snapshot": <payload>}` via `nodetypes.EvaluateExpression`; collects emitted actions per matching rule with `ad_id_expr` resolved against the same data store

- [x] Step 6.5 — green

---

## Task 7 — HTTP handler

- [x] Step 7.1 — write failing handler tests
  - File: `src-go/internal/handler/qianchuan_strategies_handler_test.go`
  - Use the existing `httptest` harness; cases:
    - `GET /api/v1/projects/:pid/qianchuan/strategies` returns paginated + filterable by `status`
    - `GET /api/v1/qianchuan/strategies/:id` returns a single row including `yaml_source` and `parsed_spec`
    - `POST /api/v1/projects/:pid/qianchuan/strategies` with valid YAML → 201; with invalid YAML → 400 + `{error:{line,col,field,msg}}`
    - `PATCH /api/v1/qianchuan/strategies/:id` on draft → 200; on published → 409 + hint to create a new version
    - `POST /api/v1/qianchuan/strategies/:id/publish` flips status; second publish call → 409
    - `POST /api/v1/qianchuan/strategies/:id/archive` flips status from published; from draft → 409
    - `DELETE /api/v1/qianchuan/strategies/:id` on draft → 204; on published → 409
    - `POST /api/v1/qianchuan/strategies/:id/test` with snapshot body → 200 + `{actions: [...]}`; with invalid JSON body → 400
    - All write endpoints reject when caller targets a system strategy (project_id IS NULL) → 403

- [x] Step 7.2 — implement handler
  - File: `src-go/internal/handler/qianchuan_strategies_handler.go`
  - Routes registered in the existing handler-wiring file (find via `Grep "RegisterRoutes" src-go/internal/handler`); add a `RegisterQianchuanStrategyRoutes(g *echo.Group, svc *service.QianchuanStrategyService)`
  - RBAC: write endpoints gate on project membership (use the existing middleware); read endpoints follow the standard project-scoped guard

- [x] Step 7.3 — green

---

## Task 8 — System seed strategies

- [x] Step 8.1 — author seed YAML files
  - File: `qianchuan-strategies/system-monitor-only.yaml`
    ```yaml
    name: system:monitor-only
    description: Notifies IM on every tick; never adjusts ads. Safe baseline.
    triggers:
      schedule: 1m
    inputs:
      - metric: cost
        dimensions: [ad_id]
        window: 1m
      - metric: conversions
        dimensions: [ad_id]
        window: 1m
    rules:
      - name: heartbeat
        condition: "true"
        actions:
          - type: notify_im
            target: {}
            params:
              channel: default
              template: "qianchuan tick — cost={{snapshot.metrics.cost}}"
    ```
  - File: `qianchuan-strategies/system-conservative-bid-optimizer.yaml`
    ```yaml
    name: system:conservative-bid-optimizer
    description: Bumps bid by up to 5% when CVR is healthy; pauses ad if cost-per-conversion degrades.
    triggers:
      schedule: 1m
    inputs:
      - metric: cvr
        dimensions: [ad_id]
        window: 15m
      - metric: cost_per_conversion
        dimensions: [ad_id]
        window: 15m
    rules:
      - name: bump-bid-when-cvr-good
        condition: "snapshot.metrics.cvr"   # threshold check resolved by 3D runtime; v1 evaluator lacks comparators, see note below
        actions:
          - type: adjust_bid
            target: { ad_id_expr: "snapshot.ad_id" }
            params: { pct: 5 }
      - name: pause-on-bad-cpa
        condition: "snapshot.metrics.cost_per_conversion"
        actions:
          - type: pause_ad
            target: { ad_id_expr: "snapshot.ad_id" }
            params: {}
    ```
    > NOTE: `nodetypes.EvaluateExpression` is intentionally minimal today (no comparators). Plan 3D either extends the evaluator or wraps the snapshot in a derived-flag layer (e.g. `snapshot.flags.cvr_healthy`). For Plan 3C the seeds must parse cleanly and TestRun must execute end-to-end against the current evaluator semantics — adjust the seeds at implementation time to match whatever evaluator surface 3D commits to. If 3D is not yet decided, simplify the conditions to plain truthy lookups (as above) and TODO-comment the threshold logic.

- [x] Step 8.2 — write failing seeds-loader test
  - File: `src-go/internal/qianchuan/strategy/seeds_test.go`
  - Cases: loader is idempotent (running twice yields exactly 2 rows); loader rejects malformed seed YAML at startup; seed `project_id` is NULL

- [x] Step 8.3 — implement loader
  - File: `src-go/internal/qianchuan/strategy/seeds.go`
  - Embed both YAML files via `//go:embed`
  - `SeedSystemStrategies(ctx, repo) error` — for each, parse + (insert if absent by name)
  - Wire the call into server bootstrap (find via `Grep "Migrate" src-go/cmd/server`); seed runs after migrations, before HTTP listen

- [x] Step 8.4 — green

---

## Task 9 — Frontend Zustand store

- [ ] Step 9.1 — add Monaco dependency
  - `pnpm add @monaco-editor/react`
  - Confirm pnpm-lock.yaml updated; commit lockfile in same PR

- [ ] Step 9.2 — write failing store test
  - File: `lib/stores/qianchuan-strategies-store.test.ts`
  - Cases: `fetchList(projectId)` populates `strategies`; `create(projectId, payload)` appends on success and surfaces `StrategyParseError` on 400; `publish(id)` flips status; `archive(id)` flips status; `testRun(id, snapshot)` returns `{actions}` and stores last result

- [ ] Step 9.3 — implement store
  - File: `lib/stores/qianchuan-strategies-store.ts`
  - Mirror the shape of an existing Zustand store (e.g. `lib/stores/secrets-store.ts` if present, otherwise pick any thin store under `lib/stores/`)
  - State: `strategies`, `selected`, `loading`, `lastError`, `lastTestResult`
  - Actions: `fetchList`, `fetchOne`, `create`, `update`, `publish`, `archive`, `delete`, `testRun`
  - Use `lib/api/client.ts` (or whatever the existing pattern is) so backend URL resolution is reused

- [ ] Step 9.4 — green

---

## Task 10 — FE list page

- [ ] Step 10.1 — write failing component test
  - File: `app/(dashboard)/projects/[id]/qianchuan/strategies/page.test.tsx`
  - Render with mocked store; cases:
    - Empty state shows "no strategies yet"
    - List renders rows with name/version/status badge
    - System rows display a `system` badge and disable Edit/Delete buttons
    - Status filter chip narrows the list
    - `New strategy` button navigates to the editor route

- [ ] Step 10.2 — implement page
  - File: `app/(dashboard)/projects/[id]/qianchuan/strategies/page.tsx`
  - Use shadcn `Table`, `Badge`, `Input` (search), `Button`
  - Add a sidebar entry under the project nav (find existing project sidebar — likely `components/projects/sidebar.tsx` or under `app/(dashboard)/projects/[id]/layout.tsx`); add a `Strategies` link

- [ ] Step 10.3 — green

---

## Task 11 — FE Monaco editor + test panel

- [ ] Step 11.1 — write failing component test
  - File: `app/(dashboard)/projects/[id]/qianchuan/strategies/[sid]/edit/page.test.tsx`
  - Cases:
    - Editor mounts and loads `yaml_source` for an existing strategy
    - On save success the page shows a success toast and refreshes
    - On save error (mocked 400 with `{line:3, col:5, field:"rules[0].condition", msg:"..."}`) editor displays a marker at line 3
    - Test panel: pasting valid JSON into the snapshot textarea + clicking `Run` calls `testRun` and renders the returned actions list
    - Test panel: invalid JSON shows a parse-error message inline (does NOT call the backend)
    - Editor is read-only when the strategy is system (project_id is null) or status='archived'
    - Publish button visible only on draft; clicking calls `publish` then redirects to list

- [ ] Step 11.2 — implement page
  - File: `app/(dashboard)/projects/[id]/qianchuan/strategies/[sid]/edit/page.tsx`
  - Layout: split pane — Monaco YAML editor on the left, Test panel on the right
  - Editor:
    - `@monaco-editor/react` with `language="yaml"`
    - On save, post to backend; on `StrategyParseError` response, set `monaco.editor.setModelMarkers(model, "strategy", [{startLineNumber: line, ...}])`
    - On successful save, clear markers
  - Test panel: textarea + `Run` button + actions list (`Type`, `Target`, `Params`)
  - Header: status badge, version, `Publish` button (drafts only), `Archive` button (published only)

- [ ] Step 11.3 — green

---

## Task 12 — FE i18n + smoke

- [ ] Step 12.1 — i18n keys
  - Add Chinese + English copies for: page titles, status labels (draft/published/archived/system), button labels (Save / Publish / Archive / Run), error toast templates, empty state copy
  - Use the existing `next-intl` message catalogs (find via `Grep "useTranslations" app/(dashboard) | head`)
  - Run `pnpm exec ts-node scripts/i18n-audit.ts` (or whatever the existing audit script is) to confirm no missing keys

- [ ] Step 12.2 — manual smoke
  - `pnpm dev:backend` + `pnpm dev`
  - Navigate to `/projects/<id>/qianchuan/strategies`
  - Verify both system seeds appear with the `system` badge and disabled actions
  - Click `New strategy` → paste the `system-monitor-only` YAML body (rename) → save → publish → archive
  - Edit a published strategy → confirm a new draft row appears at version=2
  - Run the test panel against `{"metrics":{"cost":12.5}}` → confirm the heartbeat rule emits a `notify_im` action

---

## Task 13 — Verification + final lint/test sweep

- [ ] Step 13.1 — Go side
  - `cd src-go && go test ./internal/qianchuan/strategy/... ./internal/repository/... ./internal/service/... ./internal/handler/...`
  - `cd src-go && go vet ./...`

- [ ] Step 13.2 — FE side
  - `pnpm lint`
  - `pnpm test -- qianchuan-strategies`
  - `pnpm exec tsc --noEmit`

- [ ] Step 13.3 — confirm scope
  - No file outside `src-go/internal/qianchuan/strategy/`, `src-go/internal/repository/qianchuan_strategy_repo*`, `src-go/internal/service/qianchuan_strategy_service*`, `src-go/internal/handler/qianchuan_strategies_handler*`, `src-go/migrations/0XX_create_qianchuan_strategies.*`, `qianchuan-strategies/`, `lib/stores/qianchuan-strategies-store*`, `app/(dashboard)/projects/[id]/qianchuan/strategies/**`, project-sidebar component, i18n catalogs, and `package.json`/`pnpm-lock.yaml` was modified
  - No changes to existing migrations, no changes to `nodetypes/expr.go`, no changes to other Spec 3 plans' surfaces

- [ ] Step 13.4 — request code review
  - Use `superpowers:requesting-code-review` against the diff
  - Address any findings before declaring done
