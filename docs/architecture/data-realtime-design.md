# AgentForge — 数据架构与实时系统设计

> 版本：v1.0 | 日期：2026-03-22
> 关联文档：PRD v1.0 第五节（数据模型设计）、第六节（API 设计）

---

## 当前实现快照（2026-04-22）

本文档覆盖数据与实时系统的长线设计，当前仓库已实现以下边界：

- WebSocket 广播由 Go 侧 `internal/ws` hub 统一承接，按 `project_id` 过滤广播；任务、审查、调度、插件、文档关联等前端实时更新均走此 live hub。
- 任务、实体链接、任务评论、调度、插件事件直接走 Go hub 广播，而非早期设计中的 Redis Pub/Sub 主链路。
- 事件总线（`internal/eventbus/`）提供内部 pub/sub，含 legacy adapter 与多个 observer（auth、channel router、enrich、persist、metrics、IM forward）。
- 文档/Wiki 数据面包含 `wiki_space`、`wiki_page`、`page_version`、`page_comment`、`doc_template` 及其页面树、模板、评论、版本能力。
- 项目设置前端工作区围绕 `codingAgentCatalog`、预算治理、review policy、webhook 配置与 fallback diagnostics 落地。
- 审查实时状态引入 `pending_human`，通过 `review.pending_human` 等事件驱动前端工作区。
- 全局通知系统（`notification_handler`）通过事件总线驱动 IM + WebSocket 双通道扇出。

---

## 目录

1. [PostgreSQL Schema 深度设计](#1-postgresql-schema-深度设计)
2. [Redis 架构设计](#2-redis-架构设计)
3. [WebSocket 系统设计](#3-websocket-系统设计)
4. [事件驱动架构](#4-事件驱动架构)
5. [实时 Agent 输出流](#5-实时-agent-输出流)
6. [成本追踪数据流](#6-成本追踪数据流)
7. [文件存储设计](#7-文件存储设计)
8. [数据迁移策略](#8-数据迁移策略)

---

## 1. PostgreSQL Schema 深度设计

### 1.1 索引策略优化

PRD 中定义了基础索引，此处补充完整的索引策略，包含复合索引、部分索引和表达式索引。

```sql
-- ============================================================
-- 基础 B-Tree 索引（PRD 已定义，此处补充注释）
-- ============================================================

-- 按项目查任务列表（看板页面核心查询）
CREATE INDEX idx_tasks_project ON tasks(project_id);

-- 按分配人查任务（"我的任务"视图）
CREATE INDEX idx_tasks_assignee ON tasks(assignee_id);

-- 按状态筛选（状态流转查询）
CREATE INDEX idx_tasks_status ON tasks(status);

-- 按 Sprint 查任务（Sprint 看板、燃尽图）
CREATE INDEX idx_tasks_sprint ON tasks(sprint_id);

-- Agent 运行记录关联任务
CREATE INDEX idx_agent_runs_task ON agent_runs(task_id);

-- 用户通知列表（未发送优先）
CREATE INDEX idx_notifications_target ON notifications(target_id, sent);


-- ============================================================
-- 复合索引（覆盖高频查询模式）
-- ============================================================

-- 看板视图：按项目+状态查询，避免回表
-- 查询: SELECT * FROM tasks WHERE project_id = ? AND status = ? ORDER BY updated_at DESC
CREATE INDEX idx_tasks_project_status_updated
    ON tasks(project_id, status, updated_at DESC);

-- Sprint 看板：Sprint 内按状态分组
-- 查询: SELECT * FROM tasks WHERE sprint_id = ? AND status != 'cancelled' ORDER BY priority
CREATE INDEX idx_tasks_sprint_status_priority
    ON tasks(sprint_id, status, priority);

-- 成员任务列表：某人在某项目下的活跃任务
-- 查询: SELECT * FROM tasks WHERE assignee_id = ? AND project_id = ? AND status NOT IN ('done','cancelled')
CREATE INDEX idx_tasks_assignee_project_active
    ON tasks(assignee_id, project_id)
    WHERE status NOT IN ('done', 'cancelled');

-- Agent 运行历史：按时间倒序查看某 Agent 的运行记录
-- 查询: SELECT * FROM agent_runs WHERE agent_member_id = ? ORDER BY started_at DESC
CREATE INDEX idx_agent_runs_member_time
    ON agent_runs(agent_member_id, started_at DESC);

-- 通知查询：某人未读通知（按时间倒序）
-- 查询: SELECT * FROM notifications WHERE target_id = ? AND read = FALSE ORDER BY created_at DESC
CREATE INDEX idx_notifications_unread
    ON notifications(target_id, created_at DESC)
    WHERE read = FALSE;


-- ============================================================
-- 部分索引（减少索引大小、加速热点查询）
-- ============================================================

-- 只索引活跃任务（done/cancelled 占大多数但极少查询）
-- 活跃任务通常不超过总量 20%，索引体积缩小 80%
CREATE INDEX idx_tasks_active
    ON tasks(project_id, assignee_id, updated_at DESC)
    WHERE status NOT IN ('done', 'cancelled');

-- 只索引正在运行的 Agent（池管理只关心活跃实例）
CREATE INDEX idx_agent_runs_running
    ON agent_runs(agent_member_id, started_at DESC)
    WHERE status = 'running';

-- 只索引未发送的通知（发送队列消费者使用）
CREATE INDEX idx_notifications_pending
    ON notifications(created_at)
    WHERE sent = FALSE;

-- 只索引超预算风险任务（成本监控告警使用）
CREATE INDEX idx_tasks_budget_risk
    ON tasks(project_id, spent_usd, budget_usd)
    WHERE spent_usd > budget_usd * 0.8 AND status = 'in_progress';


-- ============================================================
-- GIN 索引（JSONB 和数组字段）
-- ============================================================

-- 任务 metadata JSONB 查询（详见 1.3 节 JSONB 查询模式）
CREATE INDEX idx_tasks_metadata ON tasks USING GIN (metadata jsonb_path_ops);

-- 任务标签数组查询
-- 查询: SELECT * FROM tasks WHERE labels @> ARRAY['bug']
CREATE INDEX idx_tasks_labels ON tasks USING GIN (labels);

-- 成员技能标签查询（智能分配用）
-- 查询: SELECT * FROM members WHERE skills @> ARRAY['golang', 'testing']
CREATE INDEX idx_members_skills ON members USING GIN (skills);

-- 审查 findings JSONB 查询
CREATE INDEX idx_reviews_findings ON reviews USING GIN (findings jsonb_path_ops);

-- 项目 settings JSONB 查询
CREATE INDEX idx_projects_settings ON projects USING GIN (settings jsonb_path_ops);

-- Agent 配置 JSONB 查询
CREATE INDEX idx_members_agent_config ON members USING GIN (agent_config jsonb_path_ops);
```

### 1.2 agent_runs 表时间分区

`agent_runs` 是写入最频繁的表——每个 Agent 执行步骤都会产生记录。随时间增长，需要时间分区策略。

**分区策略：按月 Range 分区**

```sql
-- ============================================================
-- agent_runs 月分区方案
-- ============================================================

-- 将 started_at 纳入主键（分区键必须在主键中）
-- 注意：分区表不支持外键引用，需在应用层保证一致性
CREATE TABLE agent_runs (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL,              -- 应用层保证引用完整性
    agent_member_id UUID NOT NULL,
    session_id      VARCHAR(255),
    status          VARCHAR(20) NOT NULL,       -- running | completed | failed | cancelled
    prompt          TEXT,
    result          TEXT,
    tokens_input    BIGINT DEFAULT 0,
    tokens_output   BIGINT DEFAULT 0,
    cost_usd        DECIMAL(8,4) DEFAULT 0,
    duration_sec    INTEGER,
    error_message   TEXT,
    metadata        JSONB DEFAULT '{}',         -- 扩展字段：model, provider, tools_used 等
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,

    PRIMARY KEY (id, started_at)                -- 分区键必须在主键中
) PARTITION BY RANGE (started_at);

-- 创建月分区（示例：2026 年 Q1-Q2）
CREATE TABLE agent_runs_2026_01 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');
CREATE TABLE agent_runs_2026_02 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');
CREATE TABLE agent_runs_2026_03 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE agent_runs_2026_04 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE agent_runs_2026_05 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE agent_runs_2026_06 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

-- 每个分区独立索引（PostgreSQL 12+ 自动继承）
CREATE INDEX idx_agent_runs_task_time
    ON agent_runs(task_id, started_at DESC);
CREATE INDEX idx_agent_runs_member_time
    ON agent_runs(agent_member_id, started_at DESC);
CREATE INDEX idx_agent_runs_status
    ON agent_runs(status)
    WHERE status = 'running';
CREATE INDEX idx_agent_runs_metadata
    ON agent_runs USING GIN (metadata jsonb_path_ops);
```

**自动分区维护（Go 定时任务）：**

```go
// internal/scheduler/partition.go
package scheduler

import (
    "context"
    "fmt"
    "time"

    "github.com/jmoiron/sqlx"
)

// EnsurePartitions 确保未来 N 个月的分区存在
// 由 gocron 每天执行一次
func EnsurePartitions(ctx context.Context, db *sqlx.DB, monthsAhead int) error {
    now := time.Now()
    for i := 0; i <= monthsAhead; i++ {
        t := now.AddDate(0, i, 0)
        partName := fmt.Sprintf("agent_runs_%s", t.Format("2006_01"))
        rangeStart := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
        rangeEnd := rangeStart.AddDate(0, 1, 0)

        query := fmt.Sprintf(`
            DO $$
            BEGIN
                IF NOT EXISTS (
                    SELECT 1 FROM pg_class WHERE relname = '%s'
                ) THEN
                    CREATE TABLE %s PARTITION OF agent_runs
                        FOR VALUES FROM ('%s') TO ('%s');
                END IF;
            END $$;
        `, partName, partName,
            rangeStart.Format("2006-01-02"),
            rangeEnd.Format("2006-01-02"))

        if _, err := db.ExecContext(ctx, query); err != nil {
            return fmt.Errorf("创建分区 %s 失败: %w", partName, err)
        }
    }
    return nil
}

// DetachOldPartitions 将超过保留期的分区分离（不删除数据）
// 分离后可按需归档或删除
func DetachOldPartitions(ctx context.Context, db *sqlx.DB, retainMonths int) error {
    cutoff := time.Now().AddDate(0, -retainMonths, 0)
    partName := fmt.Sprintf("agent_runs_%s", cutoff.Format("2006_01"))

    query := fmt.Sprintf(`
        DO $$
        BEGIN
            IF EXISTS (
                SELECT 1 FROM pg_class WHERE relname = '%s'
            ) THEN
                ALTER TABLE agent_runs DETACH PARTITION %s;
            END IF;
        END $$;
    `, partName, partName)

    _, err := db.ExecContext(ctx, query)
    return err
}
```

**分区保留策略：**

| 时间范围 | 处理方式 | 查询性能 |
|---------|---------|---------|
| 近 3 个月 | 在线分区，全索引 | 最优 |
| 3-12 个月 | 在线分区，索引可精简 | 良好 |
| 12 个月以上 | DETACH 后归档到冷存储 | 需显式查询归档表 |

### 1.3 JSONB 查询模式

PRD 中多个表使用 JSONB 字段存储灵活数据。以下定义各 JSONB 字段的结构约定和查询模式。

#### tasks.metadata — 任务扩展信息

```jsonc
// tasks.metadata 结构约定
{
    // AI 分解相关
    "decomposition": {
        "source_task_id": "uuid",           // 源任务 ID（被分解的父任务）
        "confidence": 0.85,                 // AI 分解置信度
        "model": "claude-opus-4-6"          // 使用的模型
    },

    // 来源追踪
    "source": {
        "type": "im",                       // "im" | "web" | "api" | "webhook"
        "platform": "feishu",               // IM 平台
        "message_id": "msg_xxxx",           // 原始消息 ID
        "user_input": "帮我修复 issue #42"   // 用户原始输入
    },

    // Git 关联
    "git": {
        "commit_sha": "abc123",
        "files_changed": ["auth/token.go", "auth/token_test.go"],
        "lines_added": 42,
        "lines_removed": 15
    },

    // 自定义标签（用户可扩展）
    "custom": {
        "team": "backend",
        "component": "auth"
    }
}
```

**查询示例：**

```sql
-- 查询来自飞书的所有任务
SELECT * FROM tasks
WHERE metadata @> '{"source": {"platform": "feishu"}}';
-- 使用 jsonb_path_ops GIN 索引

-- 查询 AI 分解置信度低于 0.7 的任务（需人工确认）
SELECT id, title, metadata->'decomposition'->>'confidence' AS confidence
FROM tasks
WHERE (metadata->'decomposition'->>'confidence')::float < 0.7;

-- 查询修改了特定文件的任务
SELECT * FROM tasks
WHERE metadata @> '{"git": {"files_changed": ["auth/token.go"]}}';

-- 按自定义标签聚合统计
SELECT
    metadata->'custom'->>'team' AS team,
    COUNT(*) AS task_count,
    SUM(spent_usd) AS total_cost
FROM tasks
WHERE project_id = $1
GROUP BY metadata->'custom'->>'team';
```

#### projects.settings — 项目级配置

```jsonc
// projects.settings 结构约定
{
    // Agent 默认配置
    "agent_defaults": {
        "max_budget_usd": 5.00,             // 默认单任务预算
        "max_turns": 30,                    // 默认最大循环
        "model": "claude-sonnet-4-6",       // 默认模型
        "allowed_tools": ["read", "write", "bash", "git"]
    },

    // 审查配置
    "review": {
        "auto_review_enabled": true,
        "security_review_enabled": true,
        "required_approvals": 1,
        "auto_merge_on_approve": false
    },

    // 通知配置
    "notifications": {
        "stale_task_hours": 24,             // 任务停滞告警阈值
        "daily_report_enabled": true,
        "daily_report_time": "09:00",
        "channels": ["web", "feishu"]
    },

    // Git 配置
    "git": {
        "branch_pattern": "agent/{task_id}",
        "auto_delete_branch": true,
        "protected_branches": ["main", "develop"]
    }
}
```

**查询示例：**

```sql
-- 查询启用了安全审查的项目
SELECT id, name FROM projects
WHERE settings @> '{"review": {"security_review_enabled": true}}';

-- 获取项目的 Agent 默认预算
SELECT settings->'agent_defaults'->>'max_budget_usd' AS default_budget
FROM projects
WHERE id = $1;
```

#### reviews.findings — 审查发现

```jsonc
// reviews.findings 结构约定
[
    {
        "severity": "high",                 // "critical" | "high" | "medium" | "low" | "info"
        "category": "security",             // "security" | "logic" | "performance" | "style" | "test"
        "message": "SQL 注入风险：未使用参数化查询",
        "file": "internal/repo/task.go",
        "line": 42,
        "suggestion": "使用 sqlx.NamedExec 替代字符串拼接",
        "false_positive": false,            // 假阳性标记
        "resolved": false                   // 是否已解决
    }
]
```

**查询示例：**

```sql
-- 统计某项目各严重等级的审查发现数量
SELECT
    finding->>'severity' AS severity,
    COUNT(*) AS count
FROM reviews r,
    jsonb_array_elements(r.findings) AS finding
WHERE r.task_id IN (SELECT id FROM tasks WHERE project_id = $1)
GROUP BY finding->>'severity'
ORDER BY
    CASE finding->>'severity'
        WHEN 'critical' THEN 1
        WHEN 'high' THEN 2
        WHEN 'medium' THEN 3
        WHEN 'low' THEN 4
        ELSE 5
    END;

-- 查询包含安全类高危发现的审查
SELECT r.id, r.pr_url, r.summary
FROM reviews r
WHERE r.findings @> '[{"severity": "high", "category": "security"}]';

-- 统计假阳性率
SELECT
    COUNT(*) FILTER (WHERE (finding->>'false_positive')::boolean) AS false_positives,
    COUNT(*) AS total,
    ROUND(
        COUNT(*) FILTER (WHERE (finding->>'false_positive')::boolean)::numeric
        / NULLIF(COUNT(*), 0) * 100, 2
    ) AS false_positive_rate
FROM reviews r,
    jsonb_array_elements(r.findings) AS finding
WHERE r.created_at > NOW() - INTERVAL '30 days';
```

### 1.4 任务全文搜索实现

为支持看板和任务列表的搜索功能，使用 PostgreSQL 内建的全文搜索（tsvector + GIN）。

```sql
-- ============================================================
-- 全文搜索列和索引
-- ============================================================

-- 添加全文搜索向量列
ALTER TABLE tasks
    ADD COLUMN search_vector tsvector;

-- GIN 索引加速全文搜索
CREATE INDEX idx_tasks_search ON tasks USING GIN (search_vector);

-- 触发器：自动更新 search_vector
-- 支持中英文混合搜索（使用 simple 分词器处理中文分词后的结果）
CREATE OR REPLACE FUNCTION tasks_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('simple', COALESCE(array_to_string(NEW.labels, ' '), '')), 'C') ||
        setweight(to_tsvector('simple', COALESCE(NEW.metadata::text, '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tasks_search_vector
    BEFORE INSERT OR UPDATE OF title, description, labels, metadata
    ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION tasks_search_vector_update();

-- 回填现有数据
UPDATE tasks SET search_vector =
    setweight(to_tsvector('simple', COALESCE(title, '')), 'A') ||
    setweight(to_tsvector('simple', COALESCE(description, '')), 'B') ||
    setweight(to_tsvector('simple', COALESCE(array_to_string(labels, ' '), '')), 'C') ||
    setweight(to_tsvector('simple', COALESCE(metadata::text, '')), 'D');
```

**Go 搜索实现：**

```go
// internal/repo/task_search.go
package repo

import (
    "context"
    "fmt"
    "strings"

    "github.com/jmoiron/sqlx"
)

type TaskSearchParams struct {
    ProjectID string
    Query     string   // 用户搜索输入
    Status    []string // 可选状态过滤
    Labels    []string // 可选标签过滤
    Limit     int
    Offset    int
}

type TaskSearchResult struct {
    Task
    Rank       float64 `db:"rank"`       // 相关度得分
    Headline   string  `db:"headline"`   // 高亮摘要
}

func (r *TaskRepo) Search(ctx context.Context, params TaskSearchParams) ([]TaskSearchResult, int, error) {
    // 预处理搜索词：将空格分隔的词转为 & 连接（AND 语义）
    terms := strings.Fields(params.Query)
    tsQuery := strings.Join(terms, " & ")

    // 同时使用 LIKE 前缀匹配（处理部分输入）和全文搜索（处理完整词）
    query := `
        WITH matched AS (
            SELECT
                t.*,
                ts_rank(t.search_vector, plainto_tsquery('simple', $2)) AS rank
            FROM tasks t
            WHERE t.project_id = $1
              AND (
                  t.search_vector @@ plainto_tsquery('simple', $2)
                  OR t.title ILIKE '%' || $2 || '%'
              )
    `

    args := []interface{}{params.ProjectID, params.Query}
    argIdx := 3

    if len(params.Status) > 0 {
        query += fmt.Sprintf(" AND t.status = ANY($%d)", argIdx)
        args = append(args, params.Status)
        argIdx++
    }

    if len(params.Labels) > 0 {
        query += fmt.Sprintf(" AND t.labels && $%d", argIdx)
        args = append(args, params.Labels)
        argIdx++
    }

    // 计数 + 分页
    countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) matched", query+")")
    query += fmt.Sprintf(`
        )
        SELECT
            matched.*,
            ts_headline('simple', matched.title, plainto_tsquery('simple', $2),
                'StartSel=<mark>, StopSel=</mark>, MaxWords=50'
            ) AS headline
        FROM matched
        ORDER BY rank DESC, updated_at DESC
        LIMIT $%d OFFSET $%d
    `, argIdx, argIdx+1)

    args = append(args, params.Limit, params.Offset)

    var total int
    if err := r.db.GetContext(ctx, &total, countQuery, args[:argIdx-1]...); err != nil {
        return nil, 0, err
    }

    var results []TaskSearchResult
    if err := r.db.SelectContext(ctx, &results, query, args...); err != nil {
        return nil, 0, err
    }

    return results, total, nil
}
```

**中文搜索增强方案（P1 阶段）：**

PostgreSQL `simple` 分词器对中文支持有限。P1 阶段可引入 `zhparser` 或 `pg_jieba` 扩展：

```sql
-- 安装中文分词扩展（需编译安装 zhparser）
CREATE EXTENSION IF NOT EXISTS zhparser;
CREATE TEXT SEARCH CONFIGURATION chinese (PARSER = zhparser);
ALTER TEXT SEARCH CONFIGURATION chinese
    ADD MAPPING FOR n,v,a,i,e,l WITH simple;

-- 更新触发器使用中文配置
-- to_tsvector('chinese', ...) 替代 to_tsvector('simple', ...)
```

---

## 2. Redis 架构设计

### 2.1 Key 命名规范

所有 Redis Key 采用统一的命名规范，使用冒号 `:` 分隔层级，便于监控和按前缀管理。

```
格式：{业务域}:{实体类型}:{实体ID}[:{子资源}]

示例：
  af:task:queue                          -- 任务队列 Stream
  af:agent:pool:{project_id}             -- 项目 Agent 池状态
  af:session:{session_id}                -- Agent 会话缓存
  af:ws:channel:{channel_name}           -- WebSocket 频道 Pub/Sub
  af:rate:{endpoint}:{client_id}         -- 限频计数器
  af:lock:{resource}                     -- 分布式锁
  af:cache:{entity}:{id}                 -- 通用缓存

前缀说明：
  af           = AgentForge（全局前缀，避免与其他服务冲突）
  task         = 任务域
  agent        = Agent 域
  session      = 会话域
  ws           = WebSocket 域
  rate         = 限频域
  lock         = 分布式锁
  cache        = 缓存域
```

**Key 过期策略：**

| Key 模式 | TTL | 说明 |
|---------|-----|------|
| `af:session:*` | 24h | Agent 会话缓存，超时自动清理 |
| `af:cache:*` | 5m-1h | 业务缓存，按使用频率设置 |
| `af:rate:*` | 1s-1m | 限频窗口，自动过期 |
| `af:lock:*` | 30s | 分布式锁，防止死锁 |
| `af:ws:*` | 无 TTL | WebSocket 频道，连接断开后清理 |
| `af:task:queue` | 无 TTL | 任务队列 Stream，定期 XTRIM |

### 2.2 Streams 实现任务队列

使用 Redis Streams 作为任务队列，Consumer Groups 对接 Agent 池，实现可靠的任务分发。

**Stream 结构设计：**

```
Stream Key: af:task:queue:{project_id}

消息结构（XADD fields）:
  task_id       — 任务 UUID
  type          — "coding" | "review" | "test" | "decompose"
  priority      — "critical" | "high" | "medium" | "low"
  assignee_id   — 指定 Agent（可选，空则由 Consumer Group 竞争）
  budget_usd    — 预算上限
  max_turns     — 最大循环
  payload       — JSON 序列化的完整任务信息

Consumer Group: af:agent:group:{project_id}
  Consumer 名: agent:{member_id}
```

**Go 实现：**

```go
// internal/queue/task_queue.go
package queue

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type TaskQueue struct {
    rdb       *redis.Client
    projectID string
    streamKey string
    groupName string
}

func NewTaskQueue(rdb *redis.Client, projectID string) *TaskQueue {
    return &TaskQueue{
        rdb:       rdb,
        projectID: projectID,
        streamKey: fmt.Sprintf("af:task:queue:%s", projectID),
        groupName: fmt.Sprintf("af:agent:group:%s", projectID),
    }
}

// Init 创建 Consumer Group（幂等操作）
func (q *TaskQueue) Init(ctx context.Context) error {
    err := q.rdb.XGroupCreateMkStream(ctx, q.streamKey, q.groupName, "0").Err()
    if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
        return fmt.Errorf("创建 Consumer Group 失败: %w", err)
    }
    return nil
}

type TaskMessage struct {
    TaskID     string  `json:"task_id"`
    Type       string  `json:"type"`
    Priority   string  `json:"priority"`
    AssigneeID string  `json:"assignee_id,omitempty"`
    BudgetUSD  float64 `json:"budget_usd"`
    MaxTurns   int     `json:"max_turns"`
    Payload    string  `json:"payload"` // JSON 序列化的完整任务
}

// Enqueue 将任务推入队列
func (q *TaskQueue) Enqueue(ctx context.Context, msg TaskMessage) (string, error) {
    fields := map[string]interface{}{
        "task_id":     msg.TaskID,
        "type":        msg.Type,
        "priority":    msg.Priority,
        "assignee_id": msg.AssigneeID,
        "budget_usd":  fmt.Sprintf("%.2f", msg.BudgetUSD),
        "max_turns":   msg.MaxTurns,
        "payload":     msg.Payload,
    }

    // XADD 自动生成消息 ID（时间戳-序列号）
    id, err := q.rdb.XAdd(ctx, &redis.XAddArgs{
        Stream: q.streamKey,
        Values: fields,
    }).Result()
    if err != nil {
        return "", fmt.Errorf("任务入队失败: %w", err)
    }

    return id, nil
}

// Consume 消费任务（阻塞读取，Consumer Group 模式）
// agentID 是该 Agent 的唯一标识，用作 Consumer 名
func (q *TaskQueue) Consume(ctx context.Context, agentID string, batchSize int64) ([]TaskMessage, error) {
    consumerName := fmt.Sprintf("agent:%s", agentID)

    results, err := q.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
        Group:    q.groupName,
        Consumer: consumerName,
        Streams:  []string{q.streamKey, ">"},  // ">" 表示只读新消息
        Count:    batchSize,
        Block:    5 * time.Second,              // 阻塞等待 5 秒
    }).Result()
    if err == redis.Nil {
        return nil, nil // 无新消息
    }
    if err != nil {
        return nil, fmt.Errorf("消费任务失败: %w", err)
    }

    var tasks []TaskMessage
    for _, stream := range results {
        for _, msg := range stream.Messages {
            var tm TaskMessage
            tm.TaskID = msg.Values["task_id"].(string)
            tm.Type = msg.Values["type"].(string)
            tm.Priority = msg.Values["priority"].(string)
            tm.AssigneeID, _ = msg.Values["assignee_id"].(string)
            tm.Payload = msg.Values["payload"].(string)
            tasks = append(tasks, tm)
        }
    }

    return tasks, nil
}

// Ack 确认消息已处理
func (q *TaskQueue) Ack(ctx context.Context, messageIDs ...string) error {
    return q.rdb.XAck(ctx, q.streamKey, q.groupName, messageIDs...).Err()
}

// ClaimStale 认领超时未确认的消息（故障恢复）
// 当 Agent 崩溃时，其未 ACK 的消息可被其他 Agent 认领
func (q *TaskQueue) ClaimStale(ctx context.Context, agentID string, minIdleTime time.Duration) ([]redis.XMessage, error) {
    consumerName := fmt.Sprintf("agent:%s", agentID)

    // 先查找 pending 消息
    pending, err := q.rdb.XPendingExt(ctx, &redis.XPendingExtArgs{
        Stream: q.streamKey,
        Group:  q.groupName,
        Start:  "-",
        End:    "+",
        Count:  10,
        Idle:   minIdleTime,
    }).Result()
    if err != nil {
        return nil, err
    }

    if len(pending) == 0 {
        return nil, nil
    }

    ids := make([]string, len(pending))
    for i, p := range pending {
        ids[i] = p.ID
    }

    // 认领这些消息
    return q.rdb.XClaim(ctx, &redis.XClaimArgs{
        Stream:   q.streamKey,
        Group:    q.groupName,
        Consumer: consumerName,
        MinIdle:  minIdleTime,
        Messages: ids,
    }).Result()
}

// TrimOld 清理已消费的旧消息，保持 Stream 体积可控
func (q *TaskQueue) TrimOld(ctx context.Context, maxLen int64) error {
    return q.rdb.XTrimMaxLen(ctx, q.streamKey, maxLen).Err()
}
```

**任务优先级处理：**

Redis Streams 本身不支持优先级，采用多 Stream 策略：

```
af:task:queue:{project_id}:critical    -- 最高优先级
af:task:queue:{project_id}:high
af:task:queue:{project_id}:normal      -- medium + low

Agent 消费顺序：critical → high → normal
```

```go
// ConsumeByPriority 按优先级消费
func (q *TaskQueue) ConsumeByPriority(ctx context.Context, agentID string) (*TaskMessage, error) {
    priorities := []string{"critical", "high", "normal"}

    for _, p := range priorities {
        streamKey := fmt.Sprintf("%s:%s", q.streamKey, p)
        tasks, err := q.consumeFromStream(ctx, agentID, streamKey, 1)
        if err != nil {
            return nil, err
        }
        if len(tasks) > 0 {
            return &tasks[0], nil
        }
    }

    return nil, nil // 所有队列为空
}
```

### 2.3 Pub/Sub 实现 WebSocket 扇出

Redis Pub/Sub 用于将事件从业务服务扇出到所有 WebSocket 服务器实例，实现水平扩展。

```
频道命名:
  af:ws:project:{project_id}             -- 项目级事件
  af:ws:task:{task_id}                   -- 任务级事件
  af:ws:agent:{agent_member_id}          -- Agent 级事件（实时输出）
  af:ws:user:{user_id}                   -- 用户级通知
  af:ws:broadcast                        -- 全局广播
```

**发布端（业务服务发布事件）：**

```go
// internal/event/publisher.go
package event

import (
    "context"
    "encoding/json"
    "fmt"

    "github.com/redis/go-redis/v9"
)

type Publisher struct {
    rdb *redis.Client
}

type Event struct {
    Type      string      `json:"type"`       // 事件类型，如 "task.updated"
    Timestamp int64       `json:"timestamp"`  // Unix 毫秒时间戳
    Data      interface{} `json:"data"`       // 事件负载
}

func (p *Publisher) PublishProjectEvent(ctx context.Context, projectID string, evt Event) error {
    channel := fmt.Sprintf("af:ws:project:%s", projectID)
    return p.publish(ctx, channel, evt)
}

func (p *Publisher) PublishTaskEvent(ctx context.Context, taskID string, evt Event) error {
    channel := fmt.Sprintf("af:ws:task:%s", taskID)
    return p.publish(ctx, channel, evt)
}

func (p *Publisher) PublishAgentOutput(ctx context.Context, agentMemberID string, evt Event) error {
    channel := fmt.Sprintf("af:ws:agent:%s", agentMemberID)
    return p.publish(ctx, channel, evt)
}

func (p *Publisher) PublishUserNotification(ctx context.Context, userID string, evt Event) error {
    channel := fmt.Sprintf("af:ws:user:%s", userID)
    return p.publish(ctx, channel, evt)
}

func (p *Publisher) publish(ctx context.Context, channel string, evt Event) error {
    data, err := json.Marshal(evt)
    if err != nil {
        return fmt.Errorf("序列化事件失败: %w", err)
    }
    return p.rdb.Publish(ctx, channel, data).Err()
}
```

### 2.4 会话缓存结构

Agent 会话信息缓存到 Redis，支持快速查找和断点续做。

```
Key: af:session:{session_id}
Type: Hash
TTL: 24h
Fields:
  task_id          — 关联任务 ID
  agent_member_id  — Agent 成员 ID
  status           — "running" | "paused" | "completed" | "failed"
  started_at       — 开始时间（ISO 8601）
  last_active_at   — 最后活跃时间
  tokens_input     — 累计输入 token
  tokens_output    — 累计输出 token
  cost_usd         — 累计成本
  current_turn     — 当前循环次数
  max_turns        — 最大循环次数
  worktree_path    — Git worktree 路径
  branch_name      — Git 分支名
  checkpoint       — 最后检查点（JSON，用于断点续做）

Key: af:session:index:task:{task_id}
Type: String (存 session_id)
TTL: 24h
用途: 通过 task_id 快速反查 session_id
```

```go
// internal/cache/session_cache.go
package cache

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

const sessionTTL = 24 * time.Hour

type SessionCache struct {
    rdb *redis.Client
}

type SessionData struct {
    TaskID        string  `redis:"task_id"`
    AgentMemberID string  `redis:"agent_member_id"`
    Status        string  `redis:"status"`
    StartedAt     string  `redis:"started_at"`
    LastActiveAt  string  `redis:"last_active_at"`
    TokensInput   int64   `redis:"tokens_input"`
    TokensOutput  int64   `redis:"tokens_output"`
    CostUSD       float64 `redis:"cost_usd"`
    CurrentTurn   int     `redis:"current_turn"`
    MaxTurns      int     `redis:"max_turns"`
    WorktreePath  string  `redis:"worktree_path"`
    BranchName    string  `redis:"branch_name"`
    Checkpoint    string  `redis:"checkpoint"`
}

func (c *SessionCache) Set(ctx context.Context, sessionID string, data SessionData) error {
    key := fmt.Sprintf("af:session:%s", sessionID)
    pipe := c.rdb.Pipeline()
    pipe.HSet(ctx, key, data)
    pipe.Expire(ctx, key, sessionTTL)

    // 建立 task→session 索引
    indexKey := fmt.Sprintf("af:session:index:task:%s", data.TaskID)
    pipe.Set(ctx, indexKey, sessionID, sessionTTL)

    _, err := pipe.Exec(ctx)
    return err
}

func (c *SessionCache) Get(ctx context.Context, sessionID string) (*SessionData, error) {
    key := fmt.Sprintf("af:session:%s", sessionID)
    var data SessionData
    if err := c.rdb.HGetAll(ctx, key).Scan(&data); err != nil {
        return nil, err
    }
    if data.TaskID == "" {
        return nil, nil // 不存在
    }
    return &data, nil
}

func (c *SessionCache) UpdateTokens(ctx context.Context, sessionID string, inputDelta, outputDelta int64, costDelta float64) error {
    key := fmt.Sprintf("af:session:%s", sessionID)
    pipe := c.rdb.Pipeline()
    pipe.HIncrBy(ctx, key, "tokens_input", inputDelta)
    pipe.HIncrBy(ctx, key, "tokens_output", outputDelta)
    pipe.HIncrByFloat(ctx, key, "cost_usd", costDelta)
    pipe.HSet(ctx, key, "last_active_at", time.Now().Format(time.RFC3339))
    pipe.Expire(ctx, key, sessionTTL) // 刷新 TTL
    _, err := pipe.Exec(ctx)
    return err
}

func (c *SessionCache) GetByTaskID(ctx context.Context, taskID string) (*SessionData, error) {
    indexKey := fmt.Sprintf("af:session:index:task:%s", taskID)
    sessionID, err := c.rdb.Get(ctx, indexKey).Result()
    if err == redis.Nil {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return c.Get(ctx, sessionID)
}
```

### 2.5 限频实现

使用 Redis 实现滑动窗口限频，保护 API 和 LLM 调用。

```
Key 模式:
  af:rate:api:{client_id}:{window}       -- API 限频
  af:rate:llm:{project_id}:{window}      -- LLM 调用限频
  af:rate:ws:{client_id}                  -- WebSocket 消息限频
```

```go
// internal/middleware/ratelimit.go
package middleware

import (
    "context"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

type RateLimiter struct {
    rdb *redis.Client
}

type RateLimit struct {
    Key      string
    Limit    int           // 最大请求数
    Window   time.Duration // 时间窗口
}

// Allow 使用滑动窗口日志算法判断是否允许请求
// 返回 (是否允许, 剩余次数, error)
func (rl *RateLimiter) Allow(ctx context.Context, limit RateLimit) (bool, int, error) {
    now := time.Now().UnixMilli()
    windowStart := now - limit.Window.Milliseconds()
    key := fmt.Sprintf("af:rate:%s", limit.Key)

    // Lua 脚本原子操作：移除过期条目 → 计数 → 添加新条目
    script := redis.NewScript(`
        -- 移除窗口外的旧条目
        redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[1])
        -- 当前窗口内的请求数
        local count = redis.call('ZCARD', KEYS[1])
        if count < tonumber(ARGV[3]) then
            -- 未超限，添加当前请求
            redis.call('ZADD', KEYS[1], ARGV[2], ARGV[2])
            redis.call('PEXPIRE', KEYS[1], ARGV[4])
            return {1, tonumber(ARGV[3]) - count - 1}
        else
            return {0, 0}
        end
    `)

    result, err := script.Run(ctx, rl.rdb, []string{key},
        windowStart,                    // ARGV[1]: 窗口起始
        now,                            // ARGV[2]: 当前时间
        limit.Limit,                    // ARGV[3]: 限制次数
        limit.Window.Milliseconds(),    // ARGV[4]: TTL 毫秒
    ).Int64Slice()

    if err != nil {
        return false, 0, err
    }

    allowed := result[0] == 1
    remaining := int(result[1])
    return allowed, remaining, nil
}
```

**限频策略配置：**

| 端点类别 | 限频规则 | 说明 |
|---------|---------|------|
| REST API 通用 | 100 req/min/user | 防滥用 |
| 任务创建 | 20 req/min/user | 防止批量创建 |
| Agent 启动 | 10 req/min/project | 防止 Agent 池爆炸 |
| LLM 调用 | 60 req/min/project | 成本控制 |
| WebSocket 消息 | 30 msg/sec/connection | 防止消息洪泛 |
| IM 消息发送 | 20 msg/min/platform | 遵守 IM 平台限制 |

---

## 3. WebSocket 系统设计

### 3.1 连接管理

```go
// internal/ws/manager.go
package ws

import (
    "context"
    "sync"
    "time"

    "github.com/gofiber/contrib/websocket"
    "github.com/google/uuid"
)

// ConnManager 管理所有 WebSocket 连接
type ConnManager struct {
    mu    sync.RWMutex
    conns map[string]*Client           // connID → Client
    subs  map[string]map[string]bool   // channel → set(connID)
}

// Client 表示一个 WebSocket 客户端连接
type Client struct {
    ID          string
    UserID      string
    ProjectID   string
    Conn        *websocket.Conn
    Channels    map[string]bool    // 已订阅的频道
    LastPing    time.Time
    SendCh      chan []byte        // 发送缓冲
    ctx         context.Context
    cancel      context.CancelFunc
}

func NewConnManager() *ConnManager {
    return &ConnManager{
        conns: make(map[string]*Client),
        subs:  make(map[string]map[string]bool),
    }
}

// Register 注册新连接
func (m *ConnManager) Register(conn *websocket.Conn, userID, projectID string) *Client {
    ctx, cancel := context.WithCancel(context.Background())
    client := &Client{
        ID:        uuid.New().String(),
        UserID:    userID,
        ProjectID: projectID,
        Conn:      conn,
        Channels:  make(map[string]bool),
        LastPing:  time.Now(),
        SendCh:    make(chan []byte, 256), // 缓冲 256 条消息
        ctx:       ctx,
        cancel:    cancel,
    }

    m.mu.Lock()
    m.conns[client.ID] = client
    m.mu.Unlock()

    return client
}

// Unregister 注销连接并清理订阅
func (m *ConnManager) Unregister(clientID string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    client, ok := m.conns[clientID]
    if !ok {
        return
    }

    // 从所有频道取消订阅
    for ch := range client.Channels {
        if subs, ok := m.subs[ch]; ok {
            delete(subs, clientID)
            if len(subs) == 0 {
                delete(m.subs, ch)
            }
        }
    }

    client.cancel()
    close(client.SendCh)
    delete(m.conns, clientID)
}

// Subscribe 客户端订阅频道
func (m *ConnManager) Subscribe(clientID, channel string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if client, ok := m.conns[clientID]; ok {
        client.Channels[channel] = true
        if m.subs[channel] == nil {
            m.subs[channel] = make(map[string]bool)
        }
        m.subs[channel][clientID] = true
    }
}

// Unsubscribe 客户端取消订阅
func (m *ConnManager) Unsubscribe(clientID, channel string) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if client, ok := m.conns[clientID]; ok {
        delete(client.Channels, channel)
    }
    if subs, ok := m.subs[channel]; ok {
        delete(subs, clientID)
        if len(subs) == 0 {
            delete(m.subs, channel)
        }
    }
}

// Broadcast 向频道内所有连接推送消息
func (m *ConnManager) Broadcast(channel string, data []byte) {
    m.mu.RLock()
    subs := m.subs[channel]
    clients := make([]*Client, 0, len(subs))
    for connID := range subs {
        if c, ok := m.conns[connID]; ok {
            clients = append(clients, c)
        }
    }
    m.mu.RUnlock()

    for _, c := range clients {
        select {
        case c.SendCh <- data:
            // 消息入队成功
        default:
            // 缓冲区满，丢弃消息（客户端可能卡住）
            // 记录日志，考虑断开慢客户端
        }
    }
}

// ConnCount 返回当前连接数（监控用）
func (m *ConnManager) ConnCount() int {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return len(m.conns)
}
```

### 3.2 房间/频道模型

频道采用层级订阅模型，客户端可按需订阅不同粒度的事件。

```
频道层级:

project:{project_id}                     -- 项目级：任务变更、Sprint 进度、成员变动
  ├── task:{task_id}                     -- 任务级：状态流转、评论、Agent 分配
  │     └── agent:{agent_member_id}      -- Agent 级：实时输出流、Token 消耗
  └── notifications:{user_id}            -- 用户通知：个人消息

订阅规则:
- 订阅 project 级 → 接收该项目所有事件（汇总）
- 订阅 task 级 → 仅接收该任务事件
- 订阅 agent 级 → 接收 Agent 实时输出流（高频）
- 客户端可同时订阅多个频道
```

### 3.3 消息协议

所有 WebSocket 消息使用统一 JSON 格式。

**Server → Client 事件消息：**

```jsonc
{
    "type": "event",                     // 消息类型固定为 "event"
    "event": "task.updated",             // 事件名称
    "channel": "project:uuid-1234",      // 来源频道
    "timestamp": 1711123200000,          // Unix 毫秒时间戳
    "data": {                            // 事件负载（各事件不同）
        "task_id": "uuid-5678",
        "status": "in_progress",
        "assignee_id": "uuid-agent-1",
        "updated_fields": ["status", "assignee_id"]
    }
}
```

**Client → Server 命令消息：**

```jsonc
// 订阅频道
{
    "type": "subscribe",
    "channels": [
        "project:uuid-1234",
        "task:uuid-5678",
        "agent:uuid-agent-1"
    ]
}

// 取消订阅
{
    "type": "unsubscribe",
    "channels": ["agent:uuid-agent-1"]
}

// 心跳（客户端发送）
{
    "type": "ping",
    "timestamp": 1711123200000
}
```

**Server → Client 控制消息：**

```jsonc
// 心跳响应
{
    "type": "pong",
    "timestamp": 1711123200050
}

// 订阅确认
{
    "type": "subscribed",
    "channels": ["project:uuid-1234", "task:uuid-5678"]
}

// 错误
{
    "type": "error",
    "code": "UNAUTHORIZED",
    "message": "无权订阅该频道"
}
```

### 3.4 Redis Pub/Sub 扩展（多实例支持）

当部署多个 WebSocket 服务器实例时，使用 Redis Pub/Sub 将事件扇出到所有实例。

```
事件流转:

业务服务 → Redis PUBLISH → 所有 WS 实例 → 各实例本地 Broadcast → 客户端

                      ┌──── WS 实例 1 ──── 客户端 A, B
                      │
Redis Pub/Sub ────────┼──── WS 实例 2 ──── 客户端 C, D
                      │
                      └──── WS 实例 3 ──── 客户端 E, F
```

```go
// internal/ws/redis_bridge.go
package ws

import (
    "context"
    "encoding/json"
    "log/slog"
    "strings"

    "github.com/redis/go-redis/v9"
)

// RedisBridge 订阅 Redis Pub/Sub 并转发到本地 ConnManager
type RedisBridge struct {
    rdb     *redis.Client
    manager *ConnManager
    logger  *slog.Logger
}

func NewRedisBridge(rdb *redis.Client, manager *ConnManager, logger *slog.Logger) *RedisBridge {
    return &RedisBridge{rdb: rdb, manager: manager, logger: logger}
}

// Start 启动 Redis 订阅循环
// 使用 pattern subscribe 一次性订阅所有频道
func (b *RedisBridge) Start(ctx context.Context) {
    pubsub := b.rdb.PSubscribe(ctx, "af:ws:*")
    defer pubsub.Close()

    ch := pubsub.Channel()
    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-ch:
            // Redis 频道名: af:ws:project:uuid-1234
            // 本地频道名:   project:uuid-1234
            localChannel := strings.TrimPrefix(msg.Channel, "af:ws:")
            b.manager.Broadcast(localChannel, []byte(msg.Payload))
        }
    }
}
```

### 3.5 心跳与重连

**服务端心跳检测：**

```go
// internal/ws/heartbeat.go
package ws

import (
    "time"
)

const (
    pingInterval  = 30 * time.Second  // 服务端发送 ping 间隔
    pongTimeout   = 10 * time.Second  // 等待 pong 的超时
    maxMissedPong = 2                 // 最大允许丢失 pong 次数
)

// StartHeartbeat 为单个连接启动心跳检测
func (m *ConnManager) StartHeartbeat(client *Client) {
    ticker := time.NewTicker(pingInterval)
    defer ticker.Stop()

    missedPongs := 0

    for {
        select {
        case <-client.ctx.Done():
            return
        case <-ticker.C:
            // 检查上次 pong 是否超时
            if time.Since(client.LastPing) > pingInterval+pongTimeout {
                missedPongs++
                if missedPongs >= maxMissedPong {
                    // 连接可能已断开，清理
                    m.Unregister(client.ID)
                    return
                }
            } else {
                missedPongs = 0
            }

            // 发送 ping
            pingMsg := []byte(`{"type":"ping","timestamp":` +
                formatTimestamp(time.Now()) + `}`)
            select {
            case client.SendCh <- pingMsg:
            default:
                // 发送缓冲满，断开
                m.Unregister(client.ID)
                return
            }
        }
    }
}
```

**客户端重连策略（前端 TypeScript）：**

```typescript
// lib/ws-client.ts
interface WSClientOptions {
  url: string;
  token: string;
  onEvent: (event: WSEvent) => void;
  onStatusChange: (status: 'connecting' | 'connected' | 'disconnected') => void;
}

class WSClient {
  private ws: WebSocket | null = null;
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 10;
  private subscriptions = new Set<string>();
  private pingTimer: NodeJS.Timer | null = null;
  private pongTimer: NodeJS.Timer | null = null;

  constructor(private options: WSClientOptions) {}

  connect() {
    this.options.onStatusChange('connecting');

    this.ws = new WebSocket(`${this.options.url}?token=${this.options.token}`);

    this.ws.onopen = () => {
      this.reconnectAttempts = 0;
      this.options.onStatusChange('connected');

      // 重新订阅之前的频道
      if (this.subscriptions.size > 0) {
        this.send({
          type: 'subscribe',
          channels: Array.from(this.subscriptions),
        });
      }

      this.startPingLoop();
    };

    this.ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);

      if (msg.type === 'pong') {
        this.clearPongTimeout();
        return;
      }

      if (msg.type === 'event') {
        this.options.onEvent(msg);
      }
    };

    this.ws.onclose = () => {
      this.stopPingLoop();
      this.options.onStatusChange('disconnected');
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.ws?.close();
    };
  }

  subscribe(channels: string[]) {
    channels.forEach((ch) => this.subscriptions.add(ch));
    this.send({ type: 'subscribe', channels });
  }

  unsubscribe(channels: string[]) {
    channels.forEach((ch) => this.subscriptions.delete(ch));
    this.send({ type: 'unsubscribe', channels });
  }

  // 指数退避重连：1s, 2s, 4s, 8s, ... 最大 30s
  private scheduleReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      return; // 超过最大重试，放弃
    }

    const baseDelay = 1000;
    const maxDelay = 30000;
    const delay = Math.min(
      baseDelay * Math.pow(2, this.reconnectAttempts),
      maxDelay
    );
    // 加抖动避免雷群效应
    const jitter = delay * 0.2 * Math.random();

    this.reconnectAttempts++;
    setTimeout(() => this.connect(), delay + jitter);
  }

  private startPingLoop() {
    this.pingTimer = setInterval(() => {
      this.send({ type: 'ping', timestamp: Date.now() });

      // 设置 pong 超时
      this.pongTimer = setTimeout(() => {
        // 未收到 pong，关闭连接触发重连
        this.ws?.close();
      }, 10000);
    }, 30000);
  }

  private clearPongTimeout() {
    if (this.pongTimer) {
      clearTimeout(this.pongTimer);
      this.pongTimer = null;
    }
  }

  private stopPingLoop() {
    if (this.pingTimer) {
      clearInterval(this.pingTimer);
      this.pingTimer = null;
    }
    this.clearPongTimeout();
  }

  private send(msg: object) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    }
  }

  close() {
    this.maxReconnectAttempts = 0; // 阻止重连
    this.stopPingLoop();
    this.ws?.close();
  }
}
```

---

## 4. 事件驱动架构

### 4.1 事件类型定义

所有系统事件统一定义，每个事件包含类型、来源和负载。

```go
// internal/event/types.go
package event

import "time"

// EventType 事件类型枚举
type EventType string

const (
    // 任务事件
    TaskCreated    EventType = "task.created"
    TaskUpdated    EventType = "task.updated"
    TaskAssigned   EventType = "task.assigned"
    TaskTransition EventType = "task.transition"   // 状态流转
    TaskDeleted    EventType = "task.deleted"

    // Agent 事件
    AgentSpawned   EventType = "agent.spawned"     // Agent 实例创建
    AgentStatus    EventType = "agent.status"       // 状态变更：running/paused/completed/failed
    AgentOutput    EventType = "agent.output"       // Agent 实时输出（高频）
    AgentHeartbeat EventType = "agent.heartbeat"    // Agent 心跳（包含 token 统计）

    // 审查事件
    ReviewTriggered EventType = "review.triggered"
    ReviewCompleted EventType = "review.completed"
    ReviewApproved  EventType = "review.approved"
    ReviewRejected  EventType = "review.rejected"

    // 通知事件
    NotificationNew EventType = "notification.new"

    // Sprint 事件
    SprintProgress EventType = "sprint.progress"
    SprintStarted  EventType = "sprint.started"
    SprintCompleted EventType = "sprint.completed"

    // 成本事件
    CostUpdated     EventType = "cost.updated"
    CostBudgetAlert EventType = "cost.budget_alert" // 预算告警
)

// Event 统一事件结构
type Event struct {
    ID        string      `json:"id"`         // 事件唯一 ID（用于去重和溯源）
    Type      EventType   `json:"type"`       // 事件类型
    Source    string      `json:"source"`     // 来源服务（"task-service", "agent-service" 等）
    Timestamp time.Time   `json:"timestamp"`  // 事件发生时间
    ProjectID string      `json:"project_id"` // 所属项目
    Data      interface{} `json:"data"`       // 事件负载
}

// ======== 事件负载类型定义 ========

type TaskUpdatedData struct {
    TaskID        string   `json:"task_id"`
    UpdatedFields []string `json:"updated_fields"` // 变更的字段列表
    OldValues     map[string]interface{} `json:"old_values,omitempty"`
    NewValues     map[string]interface{} `json:"new_values"`
    UpdatedBy     string   `json:"updated_by"`     // 操作者 ID
}

type TaskTransitionData struct {
    TaskID    string `json:"task_id"`
    FromState string `json:"from_state"`
    ToState   string `json:"to_state"`
    Trigger   string `json:"trigger"`   // "manual" | "auto" | "agent"
    TriggerBy string `json:"trigger_by"`
}

type AgentStatusData struct {
    AgentMemberID string `json:"agent_member_id"`
    TaskID        string `json:"task_id"`
    SessionID     string `json:"session_id"`
    OldStatus     string `json:"old_status"`
    NewStatus     string `json:"new_status"`  // "running" | "paused" | "completed" | "failed"
    Reason        string `json:"reason,omitempty"`
}

type AgentOutputData struct {
    AgentMemberID string `json:"agent_member_id"`
    TaskID        string `json:"task_id"`
    SessionID     string `json:"session_id"`
    OutputType    string `json:"output_type"`  // "text" | "tool_call" | "tool_result" | "error"
    Content       string `json:"content"`
    Sequence      int64  `json:"sequence"`     // 输出序列号（用于排序和去重）
}

type ReviewCompletedData struct {
    ReviewID  string `json:"review_id"`
    TaskID    string `json:"task_id"`
    PRURL     string `json:"pr_url"`
    Status    string `json:"status"`    // "approved" | "changes_requested" | "rejected"
    Reviewer  string `json:"reviewer"`
    Summary   string `json:"summary"`
    FindCount int    `json:"find_count"` // 发现问题数
}

type SprintProgressData struct {
    SprintID      string  `json:"sprint_id"`
    TotalTasks    int     `json:"total_tasks"`
    CompletedTasks int    `json:"completed_tasks"`
    InProgress    int     `json:"in_progress"`
    Blocked       int     `json:"blocked"`
    BurndownPoint float64 `json:"burndown_point"` // 燃尽图数据点
    SpentUSD      float64 `json:"spent_usd"`
    BudgetUSD     float64 `json:"budget_usd"`
}

type CostBudgetAlertData struct {
    TaskID     string  `json:"task_id"`
    SpentUSD   float64 `json:"spent_usd"`
    BudgetUSD  float64 `json:"budget_usd"`
    Percentage float64 `json:"percentage"` // 已花费百分比
    AlertLevel string  `json:"alert_level"` // "warning"(80%) | "critical"(95%) | "exceeded"(100%)
}
```

### 4.2 事件总线

```go
// internal/event/bus.go
package event

import (
    "context"
    "log/slog"
    "sync"
)

// Handler 事件处理器函数
type Handler func(ctx context.Context, evt Event) error

// Bus 进程内事件总线
type Bus struct {
    mu       sync.RWMutex
    handlers map[EventType][]Handler
    logger   *slog.Logger
}

func NewBus(logger *slog.Logger) *Bus {
    return &Bus{
        handlers: make(map[EventType][]Handler),
        logger:   logger,
    }
}

// On 注册事件处理器
func (b *Bus) On(eventType EventType, handler Handler) {
    b.mu.Lock()
    defer b.mu.Unlock()
    b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Emit 发布事件，异步调用所有处理器
func (b *Bus) Emit(ctx context.Context, evt Event) {
    b.mu.RLock()
    handlers := b.handlers[evt.Type]
    b.mu.RUnlock()

    for _, h := range handlers {
        go func(handler Handler) {
            if err := handler(ctx, evt); err != nil {
                b.logger.Error("事件处理失败",
                    "event_type", evt.Type,
                    "event_id", evt.ID,
                    "error", err,
                )
            }
        }(h)
    }
}
```

### 4.3 事件溯源考量

当前 MVP 阶段不实现完整的 Event Sourcing 模式，但在 `agent_runs` 和 `notifications` 表中天然保留了事件记录。为未来可能的事件溯源做如下准备：

**事件日志表（P1 阶段引入）：**

```sql
-- 事件日志表（追加写入，不可变）
-- 用于审计、调试和事件重放
CREATE TABLE event_log (
    id          BIGSERIAL PRIMARY KEY,          -- 全局递增序列
    event_id    UUID NOT NULL UNIQUE,           -- 事件唯一 ID
    event_type  VARCHAR(50) NOT NULL,
    source      VARCHAR(50) NOT NULL,           -- 来源服务
    project_id  UUID NOT NULL,
    entity_type VARCHAR(30),                    -- "task" | "agent" | "review" | "sprint"
    entity_id   UUID,                           -- 关联实体 ID
    data        JSONB NOT NULL,                 -- 完整事件负载
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- 索引
CREATE INDEX idx_event_log_type ON event_log(event_type, created_at DESC);
CREATE INDEX idx_event_log_entity ON event_log(entity_type, entity_id, created_at DESC);
CREATE INDEX idx_event_log_project ON event_log(project_id, created_at DESC);
```

### 4.4 事件重放调试

```go
// internal/event/replay.go
package event

import (
    "context"
    "time"

    "github.com/jmoiron/sqlx"
)

type ReplayFilter struct {
    ProjectID  string
    EntityType string // 可选
    EntityID   string // 可选
    EventTypes []EventType
    StartTime  time.Time
    EndTime    time.Time
    Limit      int
}

// Replay 从事件日志重放事件（调试用）
func Replay(ctx context.Context, db *sqlx.DB, filter ReplayFilter, handler Handler) error {
    query := `
        SELECT event_id, event_type, source, project_id, data, created_at
        FROM event_log
        WHERE project_id = $1
          AND created_at BETWEEN $2 AND $3
    `
    args := []interface{}{filter.ProjectID, filter.StartTime, filter.EndTime}
    argIdx := 4

    if filter.EntityType != "" {
        query += " AND entity_type = $" + itoa(argIdx)
        args = append(args, filter.EntityType)
        argIdx++
    }

    if filter.EntityID != "" {
        query += " AND entity_id = $" + itoa(argIdx)
        args = append(args, filter.EntityID)
        argIdx++
    }

    query += " ORDER BY id ASC"

    if filter.Limit > 0 {
        query += " LIMIT $" + itoa(argIdx)
        args = append(args, filter.Limit)
    }

    rows, err := db.QueryxContext(ctx, query, args...)
    if err != nil {
        return err
    }
    defer rows.Close()

    for rows.Next() {
        var evt Event
        if err := rows.StructScan(&evt); err != nil {
            return err
        }
        if err := handler(ctx, evt); err != nil {
            return err
        }
    }

    return rows.Err()
}
```

---

## 5. 实时 Agent 输出流

### 5.1 完整流转链路

```
┌─────────────────────────────────────────────────────────────────────┐
│                         完整数据流转                                 │
│                                                                     │
│  Agent SDK (TS)          Bridge (TS)         Go Backend             │
│  ┌────────────┐         ┌────────────┐      ┌──────────────────┐   │
│  │ query()    │ stdout  │ 解析输出   │  WS  │ Agent Service    │   │
│  │ claude     ├────────→│ 打 tag     ├─────→│ 聚合 + 持久化    │   │
│  │ subagent   │ stderr  │ 缓冲/批量  │ /HTTP│ 成本计算         │   │
│  │ tool calls │────────→│ 背压控制   │      │ 预算检查         │   │
│  └────────────┘         └────────────┘      └────────┬─────────┘   │
│                                                      │              │
│                         Go WS Hub                    │              │
│                         ┌────────────┐               │              │
│                         │ project-   │←──────────────┘              │
│                         │ scoped     │                              │
│                         │ broadcast  │                              │
│                         └─────┬──────┘                              │
│                               │                                     │
│                    ┌──────────┼──────────┐                          │
│                    ↓          ↓          ↓                          │
│               Dashboard   IM Bridge   Other clients                │
│               (Browser)   / sidecars  / future surfaces            │
└─────────────────────────────────────────────────────────────────────┘
```

### 5.2 Agent SDK Bridge 输出捕获

```typescript
// agent-bridge/src/output-capture.ts
import { AgentSDK } from '@anthropic-ai/agent-sdk';

interface AgentOutput {
  type: 'text' | 'tool_call' | 'tool_result' | 'error' | 'status';
  content: string;
  sequence: number;
  timestamp: number;
  metadata?: {
    tool_name?: string;
    tokens_input?: number;
    tokens_output?: number;
  };
}

class OutputCapture {
  private sequence = 0;
  private buffer: AgentOutput[] = [];
  private flushInterval: NodeJS.Timer;
  private onFlush: (outputs: AgentOutput[]) => Promise<void>;

  constructor(
    private bufferSize: number = 10,
    private flushIntervalMs: number = 200,
    onFlush: (outputs: AgentOutput[]) => Promise<void>
  ) {
    this.onFlush = onFlush;
    // 定时刷新缓冲区（即使未满）
    this.flushInterval = setInterval(() => this.flush(), flushIntervalMs);
  }

  capture(type: AgentOutput['type'], content: string, metadata?: AgentOutput['metadata']) {
    this.buffer.push({
      type,
      content,
      sequence: this.sequence++,
      timestamp: Date.now(),
      metadata,
    });

    if (this.buffer.length >= this.bufferSize) {
      this.flush();
    }
  }

  private async flush() {
    if (this.buffer.length === 0) return;

    const batch = this.buffer.splice(0);
    try {
      await this.onFlush(batch);
    } catch (err) {
      // 发送失败，放回缓冲区头部（不丢数据）
      this.buffer.unshift(...batch);
      // 如果积压过多，启用背压（见 5.3）
      if (this.buffer.length > this.bufferSize * 10) {
        this.applyBackpressure();
      }
    }
  }

  private applyBackpressure() {
    // 降低刷新频率，丢弃低优先级输出（如 text 保留最新 N 条）
    const critical = this.buffer.filter(
      (o) => o.type === 'error' || o.type === 'status' || o.type === 'tool_call'
    );
    const text = this.buffer
      .filter((o) => o.type === 'text')
      .slice(-this.bufferSize); // 只保留最新的
    this.buffer = [...critical, ...text].sort((a, b) => a.sequence - b.sequence);
  }

  destroy() {
    clearInterval(this.flushInterval);
    this.flush(); // 最后一次刷新
  }
}
```

### 5.3 背压处理策略

当下游消费速度跟不上 Agent 输出速度时，需要在各个环节实施背压控制。

```
背压层级:

1. Bridge 缓冲层
   - 缓冲区大小: 10 条消息 / 200ms
   - 满则触发: 批量发送到 Go 后端
   - 积压 > 100 条: 丢弃低优先级 text 输出，保留 tool_call/error/status

2. Go 后端处理层
   - WebSocket 事件接收 + 内存队列
   - 满则: 走 Bridge 本地缓冲与丢弃低优先级文本策略，而不是依赖 gRPC 流控
   - 持久化到 agent_runs 使用批量写入或聚合更新

3. Go WS Hub 广播层
   - 当前 live path 由 Go hub 直接向前端/消费者广播
   - 解决方案: 继续按 `project_id` 做过滤，对高频输出保持摘要化广播
   - 全量输出仍以持久化记录和按需拉取为准，实时广播不承诺无限缓存

4. WebSocket 推送层
   - 每连接发送缓冲: 256 条消息
   - 缓冲满: 丢弃旧消息，客户端可通过 REST API 补拉
   - 慢客户端检测: 连续 3 次缓冲满则断开连接
```

```go
// internal/agent/output_handler.go
package agent

import (
    "context"
    "time"
)

// OutputHandler 处理 Agent 输出流的背压控制
type OutputHandler struct {
    publisher    *event.Publisher
    repo         *repo.AgentRunRepo
    batchBuffer  []AgentOutputData
    batchSize    int
    flushTicker  *time.Ticker
    sampleRate   int  // 每 N 条 text 输出只推送 1 条到 WebSocket
    sampleCount  int
}

func NewOutputHandler(publisher *event.Publisher, repo *repo.AgentRunRepo) *OutputHandler {
    h := &OutputHandler{
        publisher:   publisher,
        repo:        repo,
        batchSize:   50,
        flushTicker: time.NewTicker(500 * time.Millisecond),
        sampleRate:  5, // 每 5 条 text 推送 1 条
    }
    go h.flushLoop()
    return h
}

func (h *OutputHandler) Handle(ctx context.Context, output AgentOutputData) {
    // 1. 所有输出都进入持久化缓冲
    h.batchBuffer = append(h.batchBuffer, output)
    if len(h.batchBuffer) >= h.batchSize {
        h.flushToDB(ctx)
    }

    // 2. WebSocket 推送采样：tool_call/error/status 全量推送，text 采样
    shouldPush := output.OutputType != "text"
    if output.OutputType == "text" {
        h.sampleCount++
        shouldPush = h.sampleCount%h.sampleRate == 0
    }

    if shouldPush {
        h.publisher.PublishAgentOutput(ctx, output.AgentMemberID, event.Event{
            Type:      string(event.AgentOutput),
            Timestamp: time.Now().UnixMilli(),
            Data:      output,
        })
    }
}

func (h *OutputHandler) flushLoop() {
    for range h.flushTicker.C {
        if len(h.batchBuffer) > 0 {
            h.flushToDB(context.Background())
        }
    }
}

func (h *OutputHandler) flushToDB(ctx context.Context) {
    batch := h.batchBuffer
    h.batchBuffer = nil
    // 批量 INSERT（异步，不阻塞主流程）
    go h.repo.BatchInsertOutputs(ctx, batch)
}
```

---

## 6. 成本追踪数据流

### 6.1 完整数据流

```
┌─────────────────────────────────────────────────────────────────────┐
│                       成本追踪数据流                                 │
│                                                                     │
│  Agent SDK                Bridge                Go Backend          │
│  ┌────────────┐         ┌────────────┐         ┌──────────────┐    │
│  │ query()    │         │ Token 计数 │         │ 成本聚合     │    │
│  │            │ usage   │            │  report │              │    │
│  │ response   ├────────→│ 单价计算   ├────────→│ 写入 DB      │    │
│  │ .usage     │ event   │ 累计统计   │ /batch  │ 更新 task    │    │
│  └────────────┘         └────────────┘         │ 更新 sprint  │    │
│                                                └──────┬───────┘    │
│                                                       │            │
│                              ┌─────────────────┬──────┘            │
│                              │                 │                   │
│                              ↓                 ↓                   │
│                      Redis Session      预算检查点                  │
│                      Token 累计         ┌──────────────┐           │
│                      ┌──────────┐       │ 80% → 告警   │           │
│                      │ af:      │       │ 95% → 警告   │           │
│                      │ session: │       │ 100% → 暂停  │           │
│                      │ xxx      │       └──────┬───────┘           │
│                      └──────────┘              │                   │
│                                                ↓                   │
│                                        WebSocket 推送              │
│                                        cost.updated                │
│                                        cost.budget_alert           │
└─────────────────────────────────────────────────────────────────────┘
```

### 6.2 Bridge Token 计数

```typescript
// agent-bridge/src/cost-tracker.ts

interface TokenUsage {
  inputTokens: number;
  outputTokens: number;
  cacheReadTokens?: number;
  cacheWriteTokens?: number;
}

interface CostReport {
  sessionId: string;
  taskId: string;
  usageDelta: TokenUsage;    // 本次增量
  usageTotal: TokenUsage;    // 累计总量
  costDeltaUsd: number;      // 本次成本（美元）
  costTotalUsd: number;      // 累计成本（美元）
  model: string;
  timestamp: number;
}

// 模型定价表（美元/百万 token）
const MODEL_PRICING: Record<string, { input: number; output: number; cacheRead?: number }> = {
  'claude-opus-4-6':   { input: 15.0,  output: 75.0, cacheRead: 1.5 },
  'claude-sonnet-4-6': { input: 3.0,   output: 15.0, cacheRead: 0.3 },
  'claude-haiku-4-5':  { input: 0.80,  output: 4.0,  cacheRead: 0.08 },
};

class CostTracker {
  private totalUsage: TokenUsage = {
    inputTokens: 0,
    outputTokens: 0,
    cacheReadTokens: 0,
    cacheWriteTokens: 0,
  };
  private totalCostUsd = 0;

  constructor(
    private sessionId: string,
    private taskId: string,
    private model: string,
    private onReport: (report: CostReport) => Promise<void>,
    private reportIntervalMs: number = 5000  // 每 5 秒上报一次
  ) {
    // 定时上报（避免高频调用）
    setInterval(() => this.report(), reportIntervalMs);
  }

  private pendingDelta: TokenUsage = {
    inputTokens: 0, outputTokens: 0,
    cacheReadTokens: 0, cacheWriteTokens: 0,
  };

  // 每次 Agent SDK 返回 usage 时调用
  trackUsage(usage: TokenUsage) {
    this.pendingDelta.inputTokens += usage.inputTokens;
    this.pendingDelta.outputTokens += usage.outputTokens;
    this.pendingDelta.cacheReadTokens! += usage.cacheReadTokens ?? 0;
    this.pendingDelta.cacheWriteTokens! += usage.cacheWriteTokens ?? 0;

    this.totalUsage.inputTokens += usage.inputTokens;
    this.totalUsage.outputTokens += usage.outputTokens;
    this.totalUsage.cacheReadTokens! += usage.cacheReadTokens ?? 0;
    this.totalUsage.cacheWriteTokens! += usage.cacheWriteTokens ?? 0;

    const cost = this.calculateCost(usage);
    this.totalCostUsd += cost;
  }

  private calculateCost(usage: TokenUsage): number {
    const pricing = MODEL_PRICING[this.model];
    if (!pricing) return 0;

    const inputCost = (usage.inputTokens / 1_000_000) * pricing.input;
    const outputCost = (usage.outputTokens / 1_000_000) * pricing.output;
    const cacheCost = ((usage.cacheReadTokens ?? 0) / 1_000_000) * (pricing.cacheRead ?? 0);

    return inputCost + outputCost + cacheCost;
  }

  private async report() {
    if (this.pendingDelta.inputTokens === 0 && this.pendingDelta.outputTokens === 0) {
      return; // 无新增
    }

    const delta = { ...this.pendingDelta };
    const costDelta = this.calculateCost(delta);
    this.pendingDelta = {
      inputTokens: 0, outputTokens: 0,
      cacheReadTokens: 0, cacheWriteTokens: 0,
    };

    await this.onReport({
      sessionId: this.sessionId,
      taskId: this.taskId,
      usageDelta: delta,
      usageTotal: { ...this.totalUsage },
      costDeltaUsd: costDelta,
      costTotalUsd: this.totalCostUsd,
      model: this.model,
      timestamp: Date.now(),
    });
  }
}
```

### 6.3 Go 后端成本聚合与预算检查

```go
// internal/cost/aggregator.go
package cost

import (
    "context"
    "fmt"
    "log/slog"

    "github.com/jmoiron/sqlx"
    "github.com/redis/go-redis/v9"
)

type Aggregator struct {
    db        *sqlx.DB
    rdb       *redis.Client
    publisher *event.Publisher
    logger    *slog.Logger
}

type CostReport struct {
    SessionID    string  `json:"session_id"`
    TaskID       string  `json:"task_id"`
    CostDeltaUSD float64 `json:"cost_delta_usd"`
    CostTotalUSD float64 `json:"cost_total_usd"`
    TokensInput  int64   `json:"tokens_input"`
    TokensOutput int64   `json:"tokens_output"`
}

// ProcessCostReport 处理来自 Bridge 的成本上报
func (a *Aggregator) ProcessCostReport(ctx context.Context, report CostReport) error {
    // 1. 更新 Redis 会话缓存（实时数据）
    sessionKey := fmt.Sprintf("af:session:%s", report.SessionID)
    pipe := a.rdb.Pipeline()
    pipe.HIncrByFloat(ctx, sessionKey, "cost_usd", report.CostDeltaUSD)
    pipe.HIncrBy(ctx, sessionKey, "tokens_input", report.TokensInput)
    pipe.HIncrBy(ctx, sessionKey, "tokens_output", report.TokensOutput)
    if _, err := pipe.Exec(ctx); err != nil {
        a.logger.Error("更新会话缓存失败", "session_id", report.SessionID, "error", err)
    }

    // 2. 更新 PostgreSQL（持久化）
    tx, err := a.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 更新 tasks.spent_usd
    var task struct {
        SpentUSD  float64 `db:"spent_usd"`
        BudgetUSD float64 `db:"budget_usd"`
        SprintID  *string `db:"sprint_id"`
        ProjectID string  `db:"project_id"`
    }
    err = tx.GetContext(ctx, &task, `
        UPDATE tasks
        SET spent_usd = spent_usd + $1, updated_at = NOW()
        WHERE id = $2
        RETURNING spent_usd, budget_usd, sprint_id, project_id
    `, report.CostDeltaUSD, report.TaskID)
    if err != nil {
        return fmt.Errorf("更新任务成本失败: %w", err)
    }

    // 更新 sprints.spent_usd（如果有 Sprint）
    if task.SprintID != nil {
        _, err = tx.ExecContext(ctx, `
            UPDATE sprints SET spent_usd = spent_usd + $1 WHERE id = $2
        `, report.CostDeltaUSD, *task.SprintID)
        if err != nil {
            return fmt.Errorf("更新 Sprint 成本失败: %w", err)
        }
    }

    if err := tx.Commit(); err != nil {
        return err
    }

    // 3. WebSocket 推送成本更新
    a.publisher.PublishProjectEvent(ctx, task.ProjectID, event.Event{
        Type: string(event.CostUpdated),
        Data: map[string]interface{}{
            "task_id":   report.TaskID,
            "spent_usd": task.SpentUSD,
            "budget_usd": task.BudgetUSD,
        },
    })

    // 4. 预算检查点
    a.checkBudget(ctx, report.TaskID, task.SpentUSD, task.BudgetUSD, task.ProjectID)

    return nil
}

// checkBudget 预算检查点
func (a *Aggregator) checkBudget(ctx context.Context, taskID string, spent, budget float64, projectID string) {
    if budget <= 0 {
        return
    }

    percentage := (spent / budget) * 100

    var alertLevel string
    switch {
    case percentage >= 100:
        alertLevel = "exceeded"
        // 超预算：暂停 Agent
        a.pauseAgent(ctx, taskID)
    case percentage >= 95:
        alertLevel = "critical"
    case percentage >= 80:
        alertLevel = "warning"
    default:
        return // 正常范围，不告警
    }

    a.publisher.PublishProjectEvent(ctx, projectID, event.Event{
        Type: string(event.CostBudgetAlert),
        Data: CostBudgetAlertData{
            TaskID:     taskID,
            SpentUSD:   spent,
            BudgetUSD:  budget,
            Percentage: percentage,
            AlertLevel: alertLevel,
        },
    })
}

func (a *Aggregator) pauseAgent(ctx context.Context, taskID string) {
    a.logger.Warn("任务超预算，暂停 Agent", "task_id", taskID)
    // 通过 Agent Service 暂停 Agent
    // agentService.PauseByTaskID(ctx, taskID)
}
```

---

## 7. 文件存储设计

### 7.1 存储分类

| 文件类型 | 大小范围 | 写入频率 | 读取频率 | 保留期 |
|---------|---------|---------|---------|--------|
| Agent 执行日志 | 10KB-10MB/次 | 高（Agent 运行时连续写入） | 中（调试时查看） | 30 天 |
| 审查报告 | 5KB-500KB | 中（每次审查生成） | 低（需要时查看） | 永久 |
| 代码快照 | 1KB-50MB | 低（任务完成时快照） | 低（回溯时查看） | 90 天 |
| Agent 工件产物 | 可变 | 低 | 低 | 30 天 |

### 7.2 存储层抽象

```go
// internal/storage/storage.go
package storage

import (
    "context"
    "io"
    "time"
)

// FileInfo 文件元信息
type FileInfo struct {
    Key          string    // 存储路径/键
    Size         int64
    ContentType  string
    CreatedAt    time.Time
    ExpiresAt    *time.Time // 过期时间（nil 表示永久）
}

// Storage 存储接口（本地/S3/MinIO 统一接口）
type Storage interface {
    // Put 上传文件
    Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) error
    // Get 下载文件
    Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error)
    // Delete 删除文件
    Delete(ctx context.Context, key string) error
    // List 列出前缀下的文件
    List(ctx context.Context, prefix string) ([]FileInfo, error)
    // Exists 检查文件是否存在
    Exists(ctx context.Context, key string) (bool, error)
}

type PutOptions struct {
    ContentType string
    ExpiresAt   *time.Time
    Metadata    map[string]string
}

// Key 命名规范:
// agent-logs/{project_id}/{task_id}/{session_id}/{timestamp}.log
// reviews/{project_id}/{task_id}/{review_id}/report.json
// snapshots/{project_id}/{task_id}/{commit_sha}.tar.gz
// artifacts/{project_id}/{task_id}/{filename}
```

### 7.3 本地存储实现（MVP 阶段）

```go
// internal/storage/local.go
package storage

import (
    "context"
    "io"
    "os"
    "path/filepath"
    "time"
)

type LocalStorage struct {
    basePath string
}

func NewLocalStorage(basePath string) *LocalStorage {
    return &LocalStorage{basePath: basePath}
}

func (s *LocalStorage) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) error {
    fullPath := filepath.Join(s.basePath, key)

    if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
        return err
    }

    f, err := os.Create(fullPath)
    if err != nil {
        return err
    }
    defer f.Close()

    _, err = io.Copy(f, reader)
    return err
}

func (s *LocalStorage) Get(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error) {
    fullPath := filepath.Join(s.basePath, key)

    f, err := os.Open(fullPath)
    if err != nil {
        return nil, nil, err
    }

    stat, err := f.Stat()
    if err != nil {
        f.Close()
        return nil, nil, err
    }

    info := &FileInfo{
        Key:       key,
        Size:      stat.Size(),
        CreatedAt: stat.ModTime(),
    }

    return f, info, nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
    return os.Remove(filepath.Join(s.basePath, key))
}

func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
    _, err := os.Stat(filepath.Join(s.basePath, key))
    if os.IsNotExist(err) {
        return false, nil
    }
    return err == nil, err
}
```

### 7.4 S3/MinIO 实现（生产阶段）

```go
// internal/storage/s3.go
package storage

import (
    "context"
    "io"
    "time"

    "github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Storage struct {
    client *s3.Client
    bucket string
}

func NewS3Storage(client *s3.Client, bucket string) *S3Storage {
    return &S3Storage{client: client, bucket: bucket}
}

func (s *S3Storage) Put(ctx context.Context, key string, reader io.Reader, opts PutOptions) error {
    input := &s3.PutObjectInput{
        Bucket:      &s.bucket,
        Key:         &key,
        Body:        reader,
        ContentType: &opts.ContentType,
    }

    if opts.ExpiresAt != nil {
        input.Expires = opts.ExpiresAt
    }

    _, err := s.client.PutObject(ctx, input)
    return err
}

// Get, Delete, List, Exists 实现类似...
```

### 7.5 保留策略（gocron 定时清理）

```go
// internal/scheduler/retention.go
package scheduler

import (
    "context"
    "log/slog"
    "time"
)

type RetentionPolicy struct {
    Prefix     string        // 文件路径前缀
    MaxAge     time.Duration // 最大保留时间
}

var DefaultPolicies = []RetentionPolicy{
    {Prefix: "agent-logs/",  MaxAge: 30 * 24 * time.Hour},   // 30 天
    {Prefix: "snapshots/",   MaxAge: 90 * 24 * time.Hour},   // 90 天
    {Prefix: "artifacts/",   MaxAge: 30 * 24 * time.Hour},   // 30 天
    // reviews/ 不设置自动清理（永久保留）
}

// CleanExpiredFiles 清理过期文件
// 由 gocron 每天凌晨执行
func CleanExpiredFiles(ctx context.Context, store Storage, policies []RetentionPolicy, logger *slog.Logger) error {
    for _, policy := range policies {
        files, err := store.List(ctx, policy.Prefix)
        if err != nil {
            logger.Error("列出文件失败", "prefix", policy.Prefix, "error", err)
            continue
        }

        cutoff := time.Now().Add(-policy.MaxAge)
        deleted := 0

        for _, f := range files {
            if f.CreatedAt.Before(cutoff) {
                if err := store.Delete(ctx, f.Key); err != nil {
                    logger.Error("删除过期文件失败", "key", f.Key, "error", err)
                } else {
                    deleted++
                }
            }
        }

        logger.Info("清理过期文件完成",
            "prefix", policy.Prefix,
            "scanned", len(files),
            "deleted", deleted,
        )
    }
    return nil
}
```

---

## 8. 数据迁移策略

### 8.1 golang-migrate 版本管理

使用 [golang-migrate](https://github.com/golang-migrate/migrate) 管理数据库 Schema 版本。

**目录结构：**

```
migrations/
├── 000001_init_schema.up.sql          -- 初始化 Schema
├── 000001_init_schema.down.sql
├── 000002_add_search_vector.up.sql    -- 添加全文搜索
├── 000002_add_search_vector.down.sql
├── 000003_partition_agent_runs.up.sql -- agent_runs 分区
├── 000003_partition_agent_runs.down.sql
├── 000004_add_event_log.up.sql        -- 事件日志表
├── 000004_add_event_log.down.sql
└── ...
```

**初始化迁移文件（000001_init_schema.up.sql）：**

```sql
-- 000001_init_schema.up.sql
-- 初始化 AgentForge 核心 Schema
-- 版本: 1 | 日期: 2026-03-22

BEGIN;

-- 启用必要扩展
CREATE EXTENSION IF NOT EXISTS "pgcrypto";     -- gen_random_uuid()

-- ============ 项目表 ============
CREATE TABLE projects (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(100) UNIQUE NOT NULL,
    description TEXT,
    repo_url    VARCHAR(500),
    repo_branch VARCHAR(100) DEFAULT 'main',
    settings    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_projects_slug ON projects(slug);
CREATE INDEX idx_projects_settings ON projects USING GIN (settings jsonb_path_ops);

-- ============ 成员表 ============
CREATE TABLE members (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name         VARCHAR(255) NOT NULL,
    type         VARCHAR(20) NOT NULL CHECK (type IN ('human', 'agent')),
    role         VARCHAR(50),
    email        VARCHAR(255),
    im_platform  VARCHAR(50),
    im_user_id   VARCHAR(255),
    agent_config JSONB,
    skills       TEXT[],
    status       VARCHAR(20) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'suspended')),
    created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_members_project ON members(project_id);
CREATE INDEX idx_members_skills ON members USING GIN (skills);
CREATE INDEX idx_members_agent_config ON members USING GIN (agent_config jsonb_path_ops);
CREATE UNIQUE INDEX idx_members_im ON members(im_platform, im_user_id) WHERE im_platform IS NOT NULL;

-- ============ Sprint 表 ============
CREATE TABLE sprints (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name            VARCHAR(255) NOT NULL,
    start_date      DATE,
    end_date        DATE,
    status          VARCHAR(20) DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'completed')),
    total_budget_usd DECIMAL(10,2),
    spent_usd       DECIMAL(10,2) DEFAULT 0.00,
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_sprints_project ON sprints(project_id);
CREATE INDEX idx_sprints_status ON sprints(status) WHERE status = 'active';

-- ============ 任务表 ============
CREATE TABLE tasks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parent_id        UUID REFERENCES tasks(id) ON DELETE SET NULL,
    title            VARCHAR(500) NOT NULL,
    description      TEXT,
    status           VARCHAR(30) NOT NULL DEFAULT 'inbox'
        CHECK (status IN ('inbox','triaged','assigned','in_progress','in_review','done','cancelled')),
    priority         VARCHAR(20) DEFAULT 'medium'
        CHECK (priority IN ('critical','high','medium','low')),
    assignee_id      UUID REFERENCES members(id) ON DELETE SET NULL,
    assignee_type    VARCHAR(20) CHECK (assignee_type IN ('human', 'agent')),
    reporter_id      UUID REFERENCES members(id) ON DELETE SET NULL,
    sprint_id        UUID REFERENCES sprints(id) ON DELETE SET NULL,
    estimated_hours  DECIMAL(5,2),
    actual_hours     DECIMAL(5,2),

    -- Agent 执行相关
    agent_session_id VARCHAR(255),
    agent_branch     VARCHAR(255),
    agent_worktree   VARCHAR(500),
    budget_usd       DECIMAL(8,2) DEFAULT 5.00,
    spent_usd        DECIMAL(8,2) DEFAULT 0.00,
    max_turns        INTEGER DEFAULT 30,
    pr_url           VARCHAR(500),
    review_required  BOOLEAN DEFAULT TRUE,

    -- 元数据
    labels       TEXT[],
    blocked_by   UUID[],
    metadata     JSONB DEFAULT '{}',
    search_vector tsvector,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

-- 基础索引
CREATE INDEX idx_tasks_project ON tasks(project_id);
CREATE INDEX idx_tasks_assignee ON tasks(assignee_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_sprint ON tasks(sprint_id);

-- 复合索引
CREATE INDEX idx_tasks_project_status_updated ON tasks(project_id, status, updated_at DESC);
CREATE INDEX idx_tasks_sprint_status_priority ON tasks(sprint_id, status, priority);
CREATE INDEX idx_tasks_assignee_project_active ON tasks(assignee_id, project_id)
    WHERE status NOT IN ('done', 'cancelled');

-- GIN 索引
CREATE INDEX idx_tasks_metadata ON tasks USING GIN (metadata jsonb_path_ops);
CREATE INDEX idx_tasks_labels ON tasks USING GIN (labels);
CREATE INDEX idx_tasks_search ON tasks USING GIN (search_vector);

-- 部分索引
CREATE INDEX idx_tasks_active ON tasks(project_id, assignee_id, updated_at DESC)
    WHERE status NOT IN ('done', 'cancelled');

-- 全文搜索触发器
CREATE OR REPLACE FUNCTION tasks_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector :=
        setweight(to_tsvector('simple', COALESCE(NEW.title, '')), 'A') ||
        setweight(to_tsvector('simple', COALESCE(NEW.description, '')), 'B') ||
        setweight(to_tsvector('simple', COALESCE(array_to_string(NEW.labels, ' '), '')), 'C') ||
        setweight(to_tsvector('simple', COALESCE(NEW.metadata::text, '')), 'D');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tasks_search_vector
    BEFORE INSERT OR UPDATE OF title, description, labels, metadata
    ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION tasks_search_vector_update();

-- updated_at 自动更新触发器
CREATE OR REPLACE FUNCTION update_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_projects_updated_at
    BEFORE UPDATE ON projects
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at();

-- ============ Agent 运行记录表（分区） ============
CREATE TABLE agent_runs (
    id              UUID NOT NULL DEFAULT gen_random_uuid(),
    task_id         UUID NOT NULL,
    agent_member_id UUID NOT NULL,
    session_id      VARCHAR(255),
    status          VARCHAR(20) NOT NULL CHECK (status IN ('running','completed','failed','cancelled')),
    prompt          TEXT,
    result          TEXT,
    tokens_input    BIGINT DEFAULT 0,
    tokens_output   BIGINT DEFAULT 0,
    cost_usd        DECIMAL(8,4) DEFAULT 0,
    duration_sec    INTEGER,
    error_message   TEXT,
    metadata        JSONB DEFAULT '{}',
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    PRIMARY KEY (id, started_at)
) PARTITION BY RANGE (started_at);

-- 创建初始分区
CREATE TABLE agent_runs_2026_03 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');
CREATE TABLE agent_runs_2026_04 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE TABLE agent_runs_2026_05 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE agent_runs_2026_06 PARTITION OF agent_runs
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX idx_agent_runs_task_time ON agent_runs(task_id, started_at DESC);
CREATE INDEX idx_agent_runs_member_time ON agent_runs(agent_member_id, started_at DESC);
CREATE INDEX idx_agent_runs_running ON agent_runs(agent_member_id, started_at DESC)
    WHERE status = 'running';
CREATE INDEX idx_agent_runs_metadata ON agent_runs USING GIN (metadata jsonb_path_ops);

-- ============ 审查记录表 ============
CREATE TABLE reviews (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    pr_url      VARCHAR(500),
    reviewer    VARCHAR(100) NOT NULL,
    status      VARCHAR(20) NOT NULL CHECK (status IN ('pending','approved','changes_requested','rejected')),
    findings    JSONB DEFAULT '[]',
    summary     TEXT,
    cost_usd    DECIMAL(8,4),
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_reviews_task ON reviews(task_id);
CREATE INDEX idx_reviews_findings ON reviews USING GIN (findings jsonb_path_ops);
CREATE INDEX idx_reviews_status ON reviews(status) WHERE status = 'pending';

-- ============ 通知表 ============
CREATE TABLE notifications (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    target_id   UUID NOT NULL REFERENCES members(id) ON DELETE CASCADE,
    type        VARCHAR(50) NOT NULL,
    title       VARCHAR(500) NOT NULL,
    content     TEXT,
    channel     VARCHAR(50) NOT NULL,
    sent        BOOLEAN DEFAULT FALSE,
    read        BOOLEAN DEFAULT FALSE,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_notifications_target ON notifications(target_id, sent);
CREATE INDEX idx_notifications_unread ON notifications(target_id, created_at DESC)
    WHERE read = FALSE;
CREATE INDEX idx_notifications_pending ON notifications(created_at)
    WHERE sent = FALSE;

COMMIT;
```

**回滚文件（000001_init_schema.down.sql）：**

```sql
-- 000001_init_schema.down.sql
BEGIN;

DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS reviews;
DROP TABLE IF EXISTS agent_runs CASCADE;   -- CASCADE 删除所有分区
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS sprints;
DROP TABLE IF EXISTS members;
DROP TABLE IF EXISTS projects;

DROP FUNCTION IF EXISTS tasks_search_vector_update();
DROP FUNCTION IF EXISTS update_updated_at();

COMMIT;
```

### 8.2 迁移执行集成

```go
// internal/database/migrate.go
package database

import (
    "database/sql"
    "fmt"
    "log/slog"

    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

type Migrator struct {
    m      *migrate.Migrate
    logger *slog.Logger
}

func NewMigrator(db *sql.DB, migrationsPath string, logger *slog.Logger) (*Migrator, error) {
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    if err != nil {
        return nil, fmt.Errorf("创建迁移驱动失败: %w", err)
    }

    m, err := migrate.NewWithDatabaseInstance(
        fmt.Sprintf("file://%s", migrationsPath),
        "postgres",
        driver,
    )
    if err != nil {
        return nil, fmt.Errorf("初始化迁移器失败: %w", err)
    }

    return &Migrator{m: m, logger: logger}, nil
}

// Up 执行所有未应用的迁移
func (mg *Migrator) Up() error {
    mg.logger.Info("开始执行数据库迁移...")
    err := mg.m.Up()
    if err == migrate.ErrNoChange {
        mg.logger.Info("数据库已是最新版本")
        return nil
    }
    if err != nil {
        return fmt.Errorf("迁移执行失败: %w", err)
    }
    mg.logger.Info("迁移执行完成")
    return nil
}

// Version 返回当前 Schema 版本
func (mg *Migrator) Version() (uint, bool, error) {
    return mg.m.Version()
}
```

### 8.3 零停机迁移规范

为确保生产环境迁移不中断服务，所有迁移必须遵循以下规范：

**规则 1：只增不删（立即生效类）**

```sql
-- 允许：新增列（带默认值）
ALTER TABLE tasks ADD COLUMN priority_score INTEGER DEFAULT 0;

-- 允许：新增索引（CONCURRENTLY 不锁表）
CREATE INDEX CONCURRENTLY idx_tasks_priority_score ON tasks(priority_score);

-- 允许：新增表
CREATE TABLE task_comments (...);
```

**规则 2：分步删除（跨版本）**

```
-- 删除列的正确流程（跨 3 个版本）：

版本 N:   应用代码停止写入该列
版本 N+1: ALTER TABLE tasks DROP COLUMN IF EXISTS old_column;
版本 N+2: 确认清理完成

-- 错误做法：直接 DROP COLUMN（正在读写的代码会报错）
```

**规则 3：重命名列使用"加新列 → 双写 → 迁移 → 删旧列"模式**

```sql
-- 步骤 1（版本 N）：添加新列
ALTER TABLE tasks ADD COLUMN assignee_member_id UUID REFERENCES members(id);

-- 步骤 2（版本 N，应用层）：双写
-- INSERT/UPDATE 同时写 assignee_id 和 assignee_member_id
-- SELECT 优先读 assignee_member_id，fallback 到 assignee_id

-- 步骤 3（版本 N+1）：数据回填
UPDATE tasks SET assignee_member_id = assignee_id WHERE assignee_member_id IS NULL;

-- 步骤 4（版本 N+2）：应用完全切换到新列，删除旧列
ALTER TABLE tasks DROP COLUMN assignee_id;
```

### 8.4 开发环境种子数据

```go
// internal/database/seed.go
package database

import (
    "context"
    "log/slog"

    "github.com/jmoiron/sqlx"
)

// Seed 填充开发环境种子数据
func Seed(ctx context.Context, db *sqlx.DB, logger *slog.Logger) error {
    logger.Info("开始填充种子数据...")

    tx, err := db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // 1. 创建示例项目
    var projectID string
    err = tx.GetContext(ctx, &projectID, `
        INSERT INTO projects (name, slug, description, repo_url, settings)
        VALUES (
            'AgentForge Demo',
            'agentforge-demo',
            'AgentForge 平台演示项目',
            'https://github.com/example/agentforge-demo',
            $1
        )
        ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name
        RETURNING id
    `, `{
        "agent_defaults": {"max_budget_usd": 5.00, "max_turns": 30, "model": "claude-sonnet-4-6"},
        "review": {"auto_review_enabled": true, "security_review_enabled": false},
        "notifications": {"stale_task_hours": 24, "channels": ["web"]}
    }`)
    if err != nil {
        return err
    }

    // 2. 创建成员（人类 + Agent）
    var humanID, agentID string
    err = tx.GetContext(ctx, &humanID, `
        INSERT INTO members (project_id, name, type, role, email, skills, status)
        VALUES ($1, '张三', 'human', 'lead', 'zhangsan@example.com', ARRAY['golang','react','architecture'], 'active')
        ON CONFLICT DO NOTHING
        RETURNING id
    `, projectID)
    if err != nil {
        return err
    }

    err = tx.GetContext(ctx, &agentID, `
        INSERT INTO members (project_id, name, type, role, skills, agent_config, status)
        VALUES ($1, 'Coder Agent #1', 'agent', 'developer', ARRAY['golang','testing','refactoring'],
            '{"model": "claude-sonnet-4-6", "max_concurrent": 1}', 'active')
        ON CONFLICT DO NOTHING
        RETURNING id
    `, projectID)
    if err != nil {
        return err
    }

    // 3. 创建 Sprint
    var sprintID string
    err = tx.GetContext(ctx, &sprintID, `
        INSERT INTO sprints (project_id, name, start_date, end_date, status, total_budget_usd)
        VALUES ($1, 'Sprint 1 - MVP', '2026-03-25', '2026-04-05', 'active', 100.00)
        RETURNING id
    `, projectID)
    if err != nil {
        return err
    }

    // 4. 创建示例任务（各种状态）
    seedTasks := []struct {
        title       string
        status      string
        priority    string
        assigneeID  *string
        assigneeType string
    }{
        {"实现用户认证 JWT 中间件", "in_progress", "high", &humanID, "human"},
        {"编写任务 CRUD API 单元测试", "assigned", "medium", &agentID, "agent"},
        {"修复 WebSocket 心跳超时问题", "inbox", "high", nil, ""},
        {"添加 Redis 会话缓存", "triaged", "medium", nil, ""},
        {"实现看板拖拽排序", "done", "medium", &humanID, "human"},
        {"Agent 池管理接口", "in_review", "high", &agentID, "agent"},
    }

    for _, t := range seedTasks {
        _, err = tx.ExecContext(ctx, `
            INSERT INTO tasks (project_id, title, status, priority, assignee_id, assignee_type, sprint_id, budget_usd)
            VALUES ($1, $2, $3, $4, $5, $6, $7, 5.00)
        `, projectID, t.title, t.status, t.priority, t.assigneeID, nilIfEmpty(t.assigneeType), sprintID)
        if err != nil {
            return err
        }
    }

    if err := tx.Commit(); err != nil {
        return err
    }

    logger.Info("种子数据填充完成",
        "project_id", projectID,
        "human_member", humanID,
        "agent_member", agentID,
        "sprint_id", sprintID,
    )
    return nil
}

func nilIfEmpty(s string) *string {
    if s == "" {
        return nil
    }
    return &s
}
```

**使用方式：**

```bash
# 执行迁移
go run cmd/migrate/main.go up

# 填充种子数据（仅开发环境）
go run cmd/seed/main.go

# 回滚最后一次迁移
go run cmd/migrate/main.go down 1

# 查看当前版本
go run cmd/migrate/main.go version
```

---

## 附录：Redis Key 速查表

| Key 模式 | 类型 | TTL | 用途 |
|---------|------|-----|------|
| `af:task:queue:{project_id}` | Stream | 无 | 任务队列 |
| `af:task:queue:{project_id}:{priority}` | Stream | 无 | 优先级任务队列 |
| `af:agent:group:{project_id}` | Consumer Group | — | Agent 消费者组 |
| `af:session:{session_id}` | Hash | 24h | Agent 会话缓存 |
| `af:session:index:task:{task_id}` | String | 24h | Task→Session 索引 |
| `af:ws:project:{project_id}` | Pub/Sub Channel | — | 项目级事件 |
| `af:ws:task:{task_id}` | Pub/Sub Channel | — | 任务级事件 |
| `af:ws:agent:{agent_member_id}` | Pub/Sub Channel | — | Agent 输出流 |
| `af:ws:user:{user_id}` | Pub/Sub Channel | — | 用户通知 |
| `af:rate:api:{client_id}:{window}` | Sorted Set | 窗口时长 | API 限频 |
| `af:rate:llm:{project_id}:{window}` | Sorted Set | 窗口时长 | LLM 限频 |
| `af:lock:{resource}` | String | 30s | 分布式锁 |
| `af:cache:task:{task_id}` | String (JSON) | 5m | 任务详情缓存 |
| `af:cache:project:{project_id}:members` | String (JSON) | 10m | 项目成员缓存 |
