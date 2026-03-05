# Frontend 总览（次基线）

状态：`观察`（按你的要求与后端分开）

## 1. 定位

前端 SPA 用于承载当前可视化工作流，但更新节奏落后于后端。

因此本目录的定位是：
- 记录当前前端真实行为与接口调用。
- 标注可保留交互设计与过时契约。
- 不反向约束后端实现。

## 2. 主要视图

- `ChatView`：Team Leader 对话、会话切换、会话事件回放、从文件创建 issue。
- `RunView`：run 列表、run 事件、checkpoint 与 GitHub 链接展示。
- `BoardView`：issue 看板视图。
- `A2AChatView`：A2A 相关交互。

## 3. 可保留设计（建议保留）

- 会话维度订阅：`subscribe_chat_session` / `unsubscribe_chat_session`
- WS 重连后自动补订阅当前会话
- run 事件面板按 `session_id` 过滤
- `issue` 与 `plan` 双命名兼容（短期迁移有价值）

## 4. 仅记录不固化

- 前端类型中仍使用旧 run 状态枚举（`created/running/waiting_review/...`），与后端实际状态机不一致。
- API client 中存在未落地接口定义（尤其 `/Runs` 大写路径相关）。
