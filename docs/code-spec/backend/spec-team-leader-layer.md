# Team Leader 层规范（代码事实版）

状态：`保留`

## 1. 入口模型

当前 Team Leader 入口是 Chat 会话接口（V1）：
- 用户消息进入 `/api/v1/projects/{projectID}/chat`
- 系统创建或续接 `session_id`
- 异步执行 assistant 回合并写入会话

默认角色：
- `role` 为空时默认 `team_leader`

## 2. 会话执行与并发约束

会话级并发保护：
- 同一个 `session_id` 同时只允许一个运行中的回合。
- 重复触发返回冲突（`CHAT_SESSION_BUSY`）。

取消语义：
- `POST /chat/{sessionID}/cancel`
- 会触发运行取消并返回 `cancelling` 状态。

## 3. Issue 生成链路

Issue 创建依赖已有 chat session：
- 文本模式：`POST /projects/{projectID}/issues`
- 文件模式：`POST /projects/{projectID}/issues/from-files`

关键字段：
- `session_id`（必填）
- `name`（可选）
- `fail_policy`（可选，默认由服务端处理）
- `auto_merge`（可选）
- `file_paths`（仅 from-files）

## 4. 评审与动作

Issue 审批/动作接口：
- `POST /issues/{id}/review`
- `POST /issues/{id}/action`
- `POST /issues/{id}/auto-merge`

当前 `action` 语义以后端实现为准，不再沿用旧 plan/task 叙事。

## 5. 保留与剔除

### 保留
- `team_leader` 作为默认对话角色。
- 会话级事件追踪 + 可取消执行。
- issue 从 chat session 衍生的主流程。

### 观察
- 代码中仍有 `plan` 兼容命名（标题/标签/前端别名）；暂不作为新规范主名词。

### 剔除
- “仅 V2 会话接口”叙事（当前不存在 `/api/v2/sessions*`）。
