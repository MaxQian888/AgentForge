## Context

Wave 1 `add-project-rbac` 让 member 创建路径带上了 `projectRole`，但创建是**直发**：调用方必须已经知道受邀者的 `userID`。真实协作里"邀请人"不等于"受邀者账号"——邀请发起时受邀者可能还没注册，或已经在系统里但在别的项目。现有流程让"邀请人"承担了"手动在系统找到 userID 或先拉人进系统"两个额外负担，并且没有撤销、过期、重发语义。

这个 change 把邀请做成独立实体：邀请是一个"将来可能 materialize 成 member 的 pending 契约"。邀请落地的三条分界：

1. **邀请 ≠ member**。pending 邀请不占 roster slot，不给任何访问能力。
2. **邀请的身份标识**先支持两类：email 字符串与 IM 身份三元组（platform + userId + displayName）；不强制要求受邀者已注册。
3. **接受邀请是一次认证+匹配**：受邀者登录后持 token 调用 accept，服务端校验 token 仍有效 + 当前登录身份与邀请身份之一匹配，才 materialize。

## Goals / Non-Goals

**Goals:**
- 为 human 成员引入显式 pending 流：create → pending → (accepted | declined | expired | revoked)。
- 让邀请与 IM/email 投递通道解耦；投递失败不阻塞创建，便于手动复制链接兜底。
- 邀请过期由 scheduler 兜底，不依赖每次请求检查。
- 维持 agent 成员的直接创建路径不变（agent 没有被邀请概念）。
- Token 仅存 hash，接受链接一次性失效（接受/拒绝/撤销任一都使 token 失效）。

**Non-Goals:**
- 不在本 change 里新增 email 投递通道（复用 notification 现有通道；若无 email 通道则仅 IM 或手动复制链接）。
- 不做批量邀请（CSV 上传）；本期一次一人。
- 不做跨项目邀请（一个邀请绑一个项目）。
- 不做"邀请转让"（邀请 A 结果 B 接受不允许）。
- 不做"角色请求"反向流（受邀者无法反提升自己角色；若要改，需要另一张邀请）。

## Decisions

### 1. 邀请身份支持两种形式，匹配策略分明

- `invited_identity` 是 JSON 列，形如 `{"kind":"email","value":"x@y.com"}` 或 `{"kind":"im","platform":"feishu","userId":"…","displayName":"…"}`。
- 接受时匹配规则：
  - `kind=email` → 当前登录用户的 primary email 必须 case-insensitive 匹配；
  - `kind=im` → 当前登录用户已绑定的 IM identity 集合中必须含同 `(platform, userId)` 条目。
- 匹配失败返回 `403 invitation_identity_mismatch`，不 materialize。

**Why this**：两条路径覆盖"已注册用户通过 email 登录"和"已通过 IM bridge 绑定"两种典型场景。允许多身份匹配避免强制用户合并账号。

**Alternative rejected – 只支持 email**：IM-native 场景（飞书/钉钉）用户可能没有 email；强制要 email 会把这类场景排除。

**Alternative rejected – 允许任何登录用户持 token 接受**：token 泄漏就等于角色泄漏；必须绑定身份验证。

### 2. Token 只存 hash；明文只在创建响应里返回一次

- 服务端生成 32-byte CSPRNG token，持久化 SHA-256 hash 到 `token_hash`。
- 明文 token 仅在 `POST /projects/:pid/invitations` 的 201 响应 body 中返回一次，由前端构造 accept 链接（`/invitations/accept?token=…`）发给受邀者（或自动走 IM/email 通道）。
- 后续任何端点（list、get、resend）都不返回明文 token；resend 只重新触发 delivery，token 不变。
- 撤销时使 `token_hash` 置空并 `status=revoked`，accept 对已撤销邀请直接 404。

**Why this**：token 是认证凭证，和 session/api-key 同级别敏感；仓库现有 `auth_service` 的 refresh token 模式就是这么做的，保持一致。

### 3. 过期用 scheduler 兜底，不依赖请求时计算

- 新增 scheduler job `invitation.expire_sweeper`，每 15 分钟扫 `status='pending' AND expires_at < now()`，批量置 `expired`。
- accept 端点同样校验 `expires_at`：即便 sweeper 晚了一拍，过期邀请也不会生效。
- 过期阈值默认 7 天，创建时可显式传（最短 1 小时，最长 30 天）。

**Why this**：sweeper 保证状态一致（UI list 不会看到"明明应该过期还显示 pending"），请求端校验保证正确性（不会被 sweeper 延迟放行）；两个一起才完整。

### 4. 投递与创建解耦；投递失败不影响创建

- 创建邀请成功后，invitation_service 异步通过现有 notification/IM bridge 发送接受链接。
- 投递结果在邀请记录上记录 `last_delivery_status` 和 `last_delivery_attempted_at`（非关键字段，用于 UI 展示）。
- 投递失败不回滚创建、不影响 token 生效、不 block admin。admin 可以从 list 页看到 delivery 状态并选择 resend 或手动复制链接。

**Why this**：邀请创建是一次 governance 决定，投递是一次 I/O——绑在一起会让邀请不可靠；解耦后 delivery 降级到 observability 层。

### 5. `POST /projects/:pid/members` 破坏性收紧：只接受 agent 类型

Wave 1 让 member 创建路径接受 `projectRole`；本 change 收紧为"只允许 agent 成员直接创建"。human 成员**必须**通过邀请流 materialize。

**Why this**：两条路径并存会让"邀请流被绕过"成为可能（调用方在系统里已有某 userID 就直接建，跳过记录邀请这一步）。破坏性期允许一步到位。

**Alternative rejected – 保留两条路径作为过渡**：内测期无用户迁移成本，不保留兼容。

### 6. Accept/Decline 路径对未登录与已登录的处理

- `GET /invitations/by-token/:token`：**不需要登录**，返回邀请最小信息（项目 display name、角色、邀请人 display name、过期时间、当前 status）。用于受邀者点击链接后看到"你被邀请加入 X"预览。
- `POST /invitations/accept`：**必须登录**，body 含 token。实现上是"登录用户确认接受某邀请"。未登录用户命中此端点返回 401 + 登录引导（前端展示登录框后再重试）。
- `POST /invitations/decline`：**允许未登录**带 token 调用，也允许登录调用。未登录场景是受邀者看到链接但根本不打算加入（直接 decline 关闭邀请即可）。

**Why this**：UX 上 "点链接→登录/注册→自动接受" 是最顺的路径；decline 允许无登录保障受邀者随时能关掉没兴趣的邀请。

## Risks / Trade-offs

- **[Risk] Token 分发渠道不安全**：IM/email 如果被截获，邀请即可被冒领（虽有身份匹配兜底）。→ **Mitigation**：身份匹配策略已把"持 token 即可接受"收紧为"持 token + 身份匹配"；投递渠道额外风险在合规层处理，不在本 change。
- **[Risk] 邀请创建后受邀者尚未注册**：未来注册时如何关联现有 pending 邀请？→ **Mitigation**：本期不实现自动关联——受邀者注册后登录再点链接即可。未来可考虑"注册完成后扫描 pending 邀请并提示"，属 follow-up。
- **[Risk] 邀请列表可能被持续增长的 expired 邀请污染**：→ **Mitigation**：list 默认只返回近 90 天；expired 30 天后走 scheduler 清理（或 soft-archive，具体清理策略留 Open）。
- **[Risk] 邀请 `projectRole` 与 Wave 1 约束冲突**：邀请创建时的 role 是否允许 `owner`？→ **决定**：允许——组织里常见"把项目转让给新人"场景，创建 owner 邀请符合预期；但受邀者接受后原发起人仍为 owner（不是"转让"，是"新增"）。

## Migration Plan

1. 建表 + 索引 + model/repo。
2. service 层实现状态机（create/accept/decline/revoke/resend/expire），每一步事件发射走 audit（复用 Wave 1 第二个 change 的 eventbus 路径）。
3. handler + 路由挂接；前端 store + 邀请 dialog 改造 + accept 页。
4. scheduler expire sweeper。
5. `POST /projects/:pid/members` 收紧为 agent-only，所有 human 成员创建的测试用例改为邀请流。
6. 邀请 delivery 接入现有 notification/IM bridge；首版若无 email 通道则仅 IM 或"复制链接"，UI 兜底给出链接文本。
7. 端到端测试：正常创建→接受；过期；撤销；身份不匹配；重发；并发接受同一 token 只能成功一次。
8. 回滚：邀请表保留即可，revert 代码时把 member API 切回双路径；schema 不回滚。

## Open Questions

- expired 邀请的永久保留 vs. N 天后清理？当前倾向 expired 后 30 天物理删除（留 audit event 在审计表即可）；待确认。
- 同一 email/IM 对同一项目的**并发 pending 邀请**是否允许？当前倾向只允许一条 pending（创建时若已有 pending 则返回 409），避免受邀者收到多条。
- 是否在邀请上可绑定多个候选身份（email OR IM）以便受邀者任一方式登录都能接受？当前仅单身份；如有需求可扩展为数组。
