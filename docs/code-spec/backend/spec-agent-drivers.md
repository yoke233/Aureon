# Agent Driver / Chat 事件驱动规范（代码事实版）

状态：`保留`

## 1. Chat 回合驱动

Chat handler 的一个回合流程：
1. 发布 `run_started`（会话级）
2. 调用 assistant（支持 ACP 驱动）
3. 成功写会话后发布 `run_completed`
4. 失败发布 `run_failed`，取消发布 `run_cancelled`

这套会话回合事件与 workflow run 事件并存，是当前架构事实。

## 2. WS 订阅模型

客户端消息类型：
- `subscribe_run` / `unsubscribe_run`
- `subscribe_issue` / `unsubscribe_issue`
- `subscribe_chat_session` / `unsubscribe_chat_session`

会话事件缓存：
- Hub 会按 `session_id` 缓存最近会话事件，重连后可补发。

会话事件类型白名单：
- `run_started`
- `run_update`
- `run_completed`
- `run_failed`
- `run_cancelled`

## 3. 前后端事件契约（当前可用）

前端 ChatView 以 `session_id` 过滤 WS 事件，只消费当前会话。

对于 `run_update`，前端会解析 `data.acp.sessionUpdate` 来展示：
- `agent_message_chunk`
- `tool_call`
- `plan`
- 其他自定义更新类型

这部分契约虽然历史包袱较重，但对现有交互是关键路径，建议保留。

## 4. 不纳入主规范的内容

- 旧 `secretary_*` 事件前缀（当前已非主路径）。
- 未验证落地的“统一 V2 run 流式会话接口”叙事。
