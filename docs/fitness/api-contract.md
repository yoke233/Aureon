---
dimension: api_contract
weight: 15
threshold:
  pass: 90
  warn: 80
metrics:
  - name: thread_ws_types_declared
    command: Select-String -Path .\web\src\types\ws.ts -Pattern 'thread\.send|subscribe_thread|thread\.task\.started|thread\.task\.completed'
    hard_gate: false
  - name: thread_ws_event_handlers_present
    command: Select-String -Path .\internal\adapters\http\event.go -Pattern 'case "thread\.send"|case "subscribe_thread"|case "unsubscribe_thread"'
    hard_gate: false
  - name: audit_routes_registered
    command: Select-String -Path .\internal\adapters\http\handler.go -Pattern '/tool-calls/\{auditID\}|/runs/\{runID\}/audit-timeline'
    hard_gate: false
  - name: thread_ws_backend_contract_tests
    command: go test ./internal/adapters/http -run 'TestAPI_WebSocket_ThreadSend' -count=1
    hard_gate: true
  - name: audit_http_contract_tests
    command: go test ./internal/adapters/http -run 'TestAPI_(ToolCallAuditRoutes|RunAuditTimelineRoute)' -count=1
    hard_gate: true
---

# API Contract 证据

> 当前项目还没有像 `routa` 那样的 OpenAPI 单一事实源，因此本文件把“契约”收口为三类证据：类型定义、handler 注册、行为测试。

## 契约原则

### 1. 当前契约来源是多点收口，不是假装单源

当前项目里，关键接口与协议分布在：

- REST handler 注册
- WebSocket 消息类型
- `docs/spec` 的现行说明
- 后端/前端测试

所以本文件不强行宣称存在一个 OpenAPI 总契约，而是要求这些点保持一致。

### 2. Thread / Audit 是当前最值得硬约束的契约面

原因：

- `Thread` 是当前讨论层主线
- `thread.send` / `subscribe_thread` / `thread.task.*` 直接影响前后端联调
- audit 路由是当前管理与追溯的重要入口

## 当前证据链

### Thread WebSocket

- 类型定义：[ws.ts](D:/project/ai-workflow/web/src/types/ws.ts)
- 后端分发：[event.go](D:/project/ai-workflow/internal/adapters/http/event.go)
- 后端测试：[thread_ws_test.go](D:/project/ai-workflow/internal/adapters/http/thread_ws_test.go)
- 前端页面与测试：
  - [ThreadDetailPage.tsx](D:/project/ai-workflow/web/src/pages/ThreadDetailPage.tsx)
  - [ThreadDetailPage.test.tsx](D:/project/ai-workflow/web/src/pages/ThreadDetailPage.test.tsx)

### Audit HTTP

- 路由注册：[handler.go](D:/project/ai-workflow/internal/adapters/http/handler.go)
- 行为测试：[handler_test.go](D:/project/ai-workflow/internal/adapters/http/handler_test.go)

## 变更规则

### 新增或修改 Thread WebSocket 协议时

至少同时更新：

- `web/src/types/ws.ts`
- `internal/adapters/http/event.go`
- 至少一个后端或前端测试

### 新增或修改 audit 查询接口时

至少同时更新：

- `internal/adapters/http/handler.go`
- `internal/adapters/http/handler_test.go`
- 如影响前端消费，再同步类型或页面

## 当前局限

- 还没有 OpenAPI/JSON Schema 一致性检查
- 还没有自动比较 `spec` 与代码的工具
- 因此当前阶段先用“关键路径行为测试 + 类型存在性”替代
