# Backend 总览（主基线）

状态：`保留`

## 1. 当前主链路（代码事实）

当前后端是“V1 写接口 + V2 读接口”的混合形态：

- V1（`/api/v1`）承担写入与控制面：
  - project/repo/chat/issue/admin/ws
- V2（`/api/v2`）承担查询面：
  - issue/run/workflow-profile 只读查询

这不是理想分层，但它是当前可运行事实，应作为迁移起点。

## 2. 核心实体

### Project
- 核心字段：`id/name/repo_path/default_branch/github_owner/github_repo`
- `default_branch` 在创建项目时自动探测（可显式传入）。

### Issue
- 保留字段：`session_id/template/auto_merge/fail_policy/state/status`
- 当前仍保留依赖字段：`depends_on/blocks`（供 DAG 查询与统计）。
- `status` 当前实现集合：
  - `draft/reviewing/queued/ready/executing/done/failed/decomposing/decomposed/superseded/abandoned`

### Run
- 状态分离模型（应保留）：
  - `status`: `queued/in_progress/completed/action_required`
  - `conclusion`: `success/failure/timed_out/cancelled`

### ChatSession（Team Leader 入口）
- 通过 `/api/v1/projects/{projectID}/chat*` 驱动会话。
- 会话事件独立落库（`chat_run_events` 语义），并可从 WS 订阅。

## 3. 事件模型

事件总线是后端编排核心，推荐保留：
- Run 主事件：`run_done/run_failed/run_update/run_started/run_completed/run_cancelled`
- Issue 主事件：`issue_created/issue_reviewing/issue_queued/issue_ready/issue_executing/issue_done/issue_failed`
- Auto-merge 事件：`auto_merged`

注：`run_timeout` 与 `run_cancelled` 在调度恢复层会被统一映射为失败分支处理（非成功结论都按失败收敛）。

## 4. 保留与剔除

### 保留（作为重构目标的“好设计”）
- Run `status` 与 `conclusion` 分离。
- EventBus + EventStore 的可追溯架构。
- Chat 会话级事件流 + WS 按 `session_id` 订阅。
- Issue `fail_policy/auto_merge` 的显式化。

### 观察（先记录，不立刻固化）
- V1/V2 路由分裂。
- Issue 仍保留 DAG 相关字段和接口。
- `plan` 兼容命名仍在前后端存在。

### 剔除（不纳入新规范主线）
- 文档中未落地的 `/api/v2/sessions*` 运行面接口。
- 未落地的 OpenViking 深度集成主链路（仅有辅助工具与模板）。
