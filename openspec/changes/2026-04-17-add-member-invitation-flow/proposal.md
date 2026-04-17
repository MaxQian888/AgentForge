## Why

`add-project-rbac` 让 `POST /projects/:pid/members` 能显式分配 `projectRole`，但它仍是一次**直接创建**——调用方必须已经拥有被邀请者的 userID。真实团队协作场景是反过来的：**先发邀请，被邀请者接受后才成为成员**，这期间需要 pending 状态、有效期、可撤销、IM/email 投递。当前系统没有这层流程，导致实际使用中必须依赖外部工具把人先拉进来（创建账号），再在项目里建 member——流程割裂且容易错配角色。

这个 change 在 Wave 1 RBAC 的基础上补齐邀请生命周期，把"直接 create member"转成"create invitation → accept → materialize member"的标准路径。

## What Changes

- 新增 `project_invitations` 表：`id`、`project_id`、`inviter_user_id`、`invited_identity`（可为 email 或 IM identity 三元组）、`invited_user_id`（接受时填回）、`project_role`、`status`（`pending|accepted|declined|expired|revoked`）、`token_hash`、`expires_at`、`created_at`、`accepted_at`、`decline_reason`。
- 新增邀请 API：
  - `POST /projects/:pid/invitations`（`admin+`）创建邀请；请求体含 `invitedIdentity`、`projectRole`、可选 `message`、可选 `expiresAt`（默认 7 天）。
  - `GET /projects/:pid/invitations`（`admin+`）列出当前项目的所有邀请，支持按 status 过滤。
  - `POST /projects/:pid/invitations/:id/revoke`（`admin+`）撤销 pending 邀请。
  - `POST /projects/:pid/invitations/:id/resend`（`admin+`）重发未过期邀请（重新触发 IM/email 投递，不重置 token）。
  - `GET /invitations/by-token/:token`（未登录可访问）查看邀请基本信息（项目名、角色、邀请人、过期时间）。
  - `POST /invitations/accept`（已登录）接受邀请，body 为 `{token}`；服务端校验当前用户身份与邀请目标匹配，创建 member 记录并置邀请 `status=accepted`。
  - `POST /invitations/decline`（已登录或匿名带 token）拒绝邀请。
- 破坏性：`POST /projects/:pid/members` **不再**用于新增 human 成员（只保留 agent 成员创建路径）；human 成员必须通过 invitation 流落地。agent 成员仍走原路径。
- 邀请投递：**本 change 只实现存储与状态机**，实际 IM/email 投递通道通过现有 notification/IM bridge 复用——但 delivery 失败不阻塞邀请创建（邀请仍在 pending，运营可手动复制接受链接）。
- 邀请过期：scheduler 定时任务扫描 `expires_at < now()` 的 pending 邀请，置为 `expired`。
- 前端：`invite-member-dialog` 改为发起邀请而非直接建成员；roster 展示 pending 邀请条目（独立于正式 member）；新增 `/invitations/accept?token=…` 接受页。

## Capabilities

### New Capabilities
- `member-invitation-flow`：定义邀请实体、状态机、token 与过期语义、接受/拒绝/撤销/重发契约，以及与 member 创建路径的衔接。

### Modified Capabilities
- `team-management`：roster 额外展示 pending 邀请列；`POST /projects/:pid/members` 只接受 agent 类型；UI 邀请按钮语义变更。
- `project-access-control`：新增 ActionID `invitation.create|invitation.view|invitation.revoke|invitation.resend`，挂在 `admin+`。

## Impact

- DB 迁移：新表 `project_invitations` + 索引（`project_id + status + expires_at`、`token_hash` 唯一、`invited_user_id + status`）。
- 后端：`internal/model/invitation.go`（新）、`internal/repository/invitation_repo.go`（新）、`internal/service/invitation_service.go`（新）、`internal/handler/invitation_handler.go`（新）、`internal/server/routes.go`（挂接）、scheduler 新增 `invitation.expire_sweeper` 任务。
- Notification/IM 发送接入：沿用现有 `notification_handler` 的 webhook/IM bridge；delivery failure 记录 warning 但不阻塞主流程。
- 前端：`lib/stores/invitation-store.ts`（新）、`components/team/invite-member-dialog.tsx`（改为邀请流）、`components/team/team-management.tsx`（roster pending 列）、`app/(auth)/invitations/accept/page.tsx`（新接受页，无需登录访问）、国际化 `messages/**/invitations.json`。

## Depends On

- `2026-04-17-add-project-rbac`：`invitation.*` ActionID 加到 RBAC 矩阵、邀请创建必须 `admin+`、被邀请者是 human 默认以邀请中的 `projectRole` 创建 member。
