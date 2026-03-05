# ChatView / RunView 交互规范（现状）

状态：`保留（交互） + 观察（数据契约）`

## 1. ChatView（建议保留）

### 会话机制
- 创建消息时可新建或续接 `session_id`。
- 切换会话会先取消旧订阅，再订阅新会话。
- WS 重连会自动补发当前会话订阅。

### 事件展示
- 仅展示当前 `session_id` 相关 run 事件。
- 可展示：
  - `run_started`
  - `run_update`
  - `run_completed`
  - `run_failed`
  - `run_cancelled`
- `run_update` 会解析 `acp.sessionUpdate` 并展示 tool_call/plan/chunk 等细节。

### issue 入口
- 支持从 chat session “从文件创建 issue”。

## 2. RunView（建议保留）

- run 列表 + 详情 + 事件流联动。
- 支持展示 GitHub issue/pr 链接与状态。
- 支持 run action（按钮按状态启用/禁用）。

## 3. 需要后续统一的契约问题

- 前端显示层的 run 状态语义，仍偏旧模型（`running/waiting_review/done/...`）。
- 后端实际模型是 `status + conclusion` 双轴。

建议：
- 保留 ChatView/RunView 的交互骨架。
- 将状态展示逻辑切换到后端真实字段，避免继续扩大语义漂移。
