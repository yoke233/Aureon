---
dimension: runtime_contract
weight: 10
threshold:
  pass: 90
  warn: 80
metrics:
  - name: thread_runtime_spec_present
    command: Select-String -Path .\docs\spec\README.md,.\docs\spec\thread-agent-runtime.zh-CN.md -Pattern 'thread\.send|subscribe_thread|unsubscribe_thread'
    hard_gate: false
  - name: thread_runtime_frontend_subscriptions
    command: Select-String -Path .\web\src\pages\ThreadDetailPage.tsx -Pattern 'subscribe_thread|unsubscribe_thread|thread\.send'
    hard_gate: false
  - name: thread_runtime_frontend_tests
    command: Select-String -Path .\web\src\pages\ThreadDetailPage.test.tsx -Pattern 'subscribe_thread|thread\.send'
    hard_gate: false
  - name: thread_runtime_backend_ws_tests
    command: go test ./internal/adapters/http -run 'TestAPI_WebSocket_ThreadSend|TestAPI_WebSocket_ThreadSubscribe' -count=1
    hard_gate: false
---

# Runtime Contract 证据

> 本文件借鉴 `routa` 的“行为规则必须可验证”思路，但聚焦本项目当前最核心的运行时契约：Thread + WebSocket。

## 规则目标

- 确保线程运行时协议没有只改一半
- 确保文档、前端订阅、后端事件、测试之间存在最小闭环

## 当前关注的运行时面

### 1. Thread 消息发送

- `thread.send`
- `target_agent_id`
- `subscribe_thread`
- `unsubscribe_thread`

### 2. Thread 订阅生命周期

- `subscribe_thread`
- `unsubscribe_thread`

## 证据来源

- 现行说明：
  - [spec/README.md](D:/project/ai-workflow/docs/spec/README.md)
  - [thread-agent-runtime.zh-CN.md](D:/project/ai-workflow/docs/spec/thread-agent-runtime.zh-CN.md)
- 前端类型与订阅：
  - [ws.ts](D:/project/ai-workflow/web/src/types/ws.ts)
  - [ThreadDetailPage.tsx](D:/project/ai-workflow/web/src/pages/ThreadDetailPage.tsx)
- 测试：
  - [thread_ws_test.go](D:/project/ai-workflow/internal/adapters/http/thread_ws_test.go)
  - [ThreadDetailPage.test.tsx](D:/project/ai-workflow/web/src/pages/ThreadDetailPage.test.tsx)

## 变更规则

- 修改线程运行时协议时，不允许只改 `docs/spec`
- 修改 thread WebSocket 协议时，不允许只改后端事件常量，不改前端订阅和测试
- 如果运行时语义仍处于过渡态，必须在 `docs/spec/README.md` 或对应专题文档中写清“当前实现状态”
