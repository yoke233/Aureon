# Actor 工作空间：动态多 Agent 协作模型

> **前置**: [02-Escalation/Directive](02-escalation-directive-pattern.zh-CN.md) 定义了纵向通信。本文将其泛化为 Actor 间任意通信。
> **相关**: [05-多用户部署](05-multi-user-deployment-model.zh-CN.md) 的单实例多 Project 模型是本设计的部署基础。

## 问题

当前系统是**固定流水线**模型：

```
Issue → Run → [setup → implement → review → merge] → Done
```

- 角色在 config 里静态定义
- ACP session 绑定 stage 生命周期，stage 结束即销毁
- Agent 之间不能直接对话，只通过 EventBus 间接协调
- 所有工作流是预定义模板（standard / quick / hotfix）

**缺什么：**

1. 不能动态创建角色 — TL 发现需要一个"数据库专家"，得改 config 重启
2. Agent 没有持久记忆 — 每次启动都是全新 session，前一个任务学到的上下文丢失
3. Agent 之间不能对话 — Coder 想问 Reviewer 一个问题，必须走完整个 stage 流转
4. 流程不能动态编排 — TL 不能说"先让 A 和 B 并行，B 做完再让 C 开始"

## 核心洞察

**Agent 不是函数，是人。**

函数调用：调用 → 等结果 → 销毁。适合确定性流水线。
人的工作方式：常驻 → 接活 → 干活 → 跟人沟通 → 汇报 → 等下一个活。

把 Agent 从"被调用的函数"变成"常驻的 Actor"，系统从"流水线调度器"变成"团队工作空间"。

## 核心概念

### Actor

一个有持久身份的 Agent 实例。

```
Actor = 角色画像 + 持久 Session + 工作目录 + 收件箱
```

| 属性 | 说明 |
|------|------|
| ID | 唯一标识，如 `actor-coder-01` |
| Role | 角色模板（画像、能力、提示词） |
| Session | ACP 持久会话（可休眠/唤醒） |
| Workspace | 独立工作目录（worktree 或自定义路径） |
| Inbox | 消息队列，按优先级排序 |
| Status | `idle` / `busy` / `sleeping` / `dead` |

### Inbox（收件箱）

每个 Actor 有一个收件箱。发消息 = 投递到收件箱，**立刻返回**，不阻塞发送方。

```
TL 给 Coder 发消息：
  TL → Gateway.Send(to=coder, msg) → Coder.Inbox.Push(msg) → 返回（TL 继续干别的）

  ...稍后...

  Coder 空闲 → Coder.Inbox.Pop() → 处理消息 → 可能回复 TL
```

**这就是 Actor 模型。** 没有同步调用，没有阻塞等待。

### Gateway（网关）

消息路由中心。所有 Actor 间通信经过 Gateway。

```
┌─────────────────────────────────────────────────┐
│                   Gateway                        │
│                                                  │
│  路由表: Actor ID → Inbox 地址                    │
│  权限: 谁能给谁发消息                              │
│  策略: 消息优先级、超时、死信队列                    │
│  日志: 全量消息审计                                │
└────┬──────────┬──────────┬──────────┬────────────┘
     │          │          │          │
 ┌───▼──┐  ┌───▼──┐  ┌───▼───┐  ┌───▼───┐
 │  TL  │  │Coder │  │Coder  │  │Review │
 │Inbox │  │#1    │  │#2     │  │er     │
 │      │  │Inbox │  │Inbox  │  │Inbox  │
 └──────┘  └──────┘  └──────┘  └───────┘
```

Gateway 不做决策，只做路由。决策是 TL（或更上层）的事。

### 消息类型

02 的 Escalation/Directive 变成消息类型的子集：

| 类型 | 方向 | 说明 | 举例 |
|------|------|------|------|
| `directive` | 上→下 | 指令 | TL 让 Coder 实现某功能 |
| `escalation` | 下→上 | 上报 | Coder 遇到冲突求助 TL |
| `query` | 对等 | 问答 | Coder 问 Reviewer "这个接口你觉得行吗" |
| `handoff` | 对等 | 交接 | Coder 完成后交给 Reviewer |
| `notify` | 单向 | 通知 | "我做完了" / "有新 Issue" |
| `broadcast` | 一对多 | 广播 | TL 宣布"所有人暂停，main 分支有问题" |

消息结构：

```go
type ActorMessage struct {
    ID        string
    From      string            // 发送者 Actor ID
    To        string            // 接收者 Actor ID（broadcast 时为 channel）
    Type      MessageType       // directive / escalation / query / handoff / notify
    ReplyTo   string            // 回复哪条消息（对话串联）
    Priority  int               // 优先级（escalation 默认高优）
    Subject   string            // 主题（人可读）
    Body      string            // 自然语言内容
    Data      map[string]any    // 结构化附加数据
    CreatedAt time.Time
    ExpiresAt time.Time         // 超时未处理则进死信队列
}
```

## TL 的角色管理技能

TL 是工作空间的管理者。通过对话创建、配置、管理 Actor。

### MCP 工具集

```
角色管理：
  create_role(name, base_agent, capabilities, prompt, description)
  update_role(name, ...)
  delete_role(name)
  list_roles()

Actor 生命周期：
  spawn_actor(role, workspace_path?)     → 创建并启动一个 Actor
  kill_actor(actor_id)                   → 停止并销毁
  sleep_actor(actor_id)                  → 休眠（释放进程，保留状态）
  wake_actor(actor_id)                   → 唤醒（恢复进程和状态）
  list_actors()                          → 查看所有活跃 Actor

消息：
  send_message(to, type, subject, body)  → 发送消息到 Actor
  broadcast(channel, subject, body)      → 广播
  check_inbox()                          → 查看自己的收件箱
```

### TL 对话示例

```
Human: "我们需要一个专门处理数据库迁移的角色"

TL:  → create_role(
         name="db-specialist",
         base_agent="claude",
         capabilities={fs_read: true, fs_write: true, terminal: true},
         prompt="你是数据库迁移专家，熟悉 PostgreSQL 和 SQLite...",
       )
     → spawn_actor(role="db-specialist", workspace="/projects/backend")

     "已创建 db-specialist 角色并启动了一个实例。
      他现在在 /projects/backend 工作目录待命。要给他分配任务吗？"
```

### TL 动态编排示例

```
Human: "项目 A 需要重构认证模块，让两个 coder 并行做，一个负责后端 API，一个负责前端适配"

TL:  → spawn_actor(role="worker", workspace="/projects/A")        // coder-01
     → spawn_actor(role="worker", workspace="/projects/A")        // coder-02
     → send_message(to="coder-01", type=directive,
         subject="重构后端认证 API",
         body="重构 /src/auth/ 下的 handler，切换到 JWT...")
     → send_message(to="coder-02", type=directive,
         subject="前端认证适配",
         body="后端 API 变更后，适配 /web/src/lib/auth.ts...")

     "已分配两个 coder 并行工作。coder-01 负责后端，coder-02 负责前端。
      后端完成后我会通知前端 coder 对齐接口。"

  ...coder-01 完成后...

  TL 收到 notify("后端认证 API 重构完成")
     → send_message(to="coder-02", type=notify,
         body="后端 API 已完成，接口变更: POST /auth/login 返回格式改了...")
```

**关键区别**：TL 不是在执行预定义流水线，而是根据任务性质**即兴编排**。

## 与固定流水线的关系

### 不替代，共存

固定流水线 = 成熟的、可重复的工作流（标准开发、hotfix）。
Actor 动态编排 = 非标准、探索性、跨域协作的工作流。

```
用户说 "修一个 bug"
  → TL 判断这是标准任务
  → 走固定流水线（Issue → Run → stages → Done）
  → 流水线内部可以复用常驻 Actor 的 session（不用每次冷启动）

用户说 "重构整个认证模块，涉及三个项目"
  → TL 判断这需要自定义编排
  → 动态创建角色、分配任务、协调沟通
  → 不走预定义 stage 模板
```

### 流水线在 Actor 模型下的表达

固定流水线其实就是 TL 按照模板发的一系列 directive：

```
标准流水线 =
  TL → directive(coder, "实现")
  TL ← notify(coder, "实现完成")
  TL → directive(reviewer, "审查")
  TL ← notify(reviewer, "审查通过")
  TL → directive(coder, "merge")
```

流水线模板可以变成 TL 的**行为模式**，而不是硬编码的 stage 序列。TL 根据经验选择模式，也可以在执行过程中调整。

## Actor 生命周期

### 状态机

```
                spawn
                  │
                  ▼
idle ◄──────► busy
  │              │
  │ sleep        │ sleep（完成当前消息后）
  ▼              ▼
sleeping ───► idle（wake）
  │
  │ kill / 超时回收
  ▼
dead
```

### 休眠与唤醒

Agent 进程占资源。空闲 Actor 需要休眠：

```
休眠：
  1. 序列化 ACP session 状态（对话历史、工具状态）
  2. 保存到 Store（actor_sessions 表）
  3. 终止 ACP 进程
  4. Actor 状态 → sleeping

唤醒：
  1. 从 Store 加载 session 状态
  2. 启动新 ACP 进程
  3. 注入历史对话作为上下文
  4. Actor 状态 → idle
```

**注意**：ACP 协议目前不支持原生 session 序列化。唤醒后注入历史是近似恢复，不是精确恢复。对于大部分场景够用 — Agent 会"记得"之前的对话，但丢失进程内状态（如打开的文件句柄）。

### 自动回收策略

```toml
[actor_pool]
max_idle_duration = "30m"       # 空闲超过 30 分钟自动休眠
max_sleeping_duration = "24h"   # 休眠超过 24 小时自动销毁
max_concurrent_actors = 10      # 全局最多同时活跃 Actor
```

## Inbox 设计

### 消息队列语义

```
优先级排序：
  escalation (P0) > directive (P1) > query (P2) > handoff (P3) > notify (P4)

同优先级内：FIFO

Actor 消费循环：
  loop:
    msg = inbox.Pop()          // 阻塞等待
    if status == busy:
      inbox.PushFront(msg)     // 忙碌时退回队首
      wait(current_task_done)
      continue
    process(msg)
```

### 死信队列

消息超过 `ExpiresAt` 未被消费 → 进入死信队列 → Gateway 通知发送方：

```
TL 给 Coder 发了 directive，Coder 挂了
  → 消息在 Inbox 超时
  → Gateway → TL.Inbox: "你给 coder-01 的指令超时未处理"
  → TL 决策：唤醒 coder-01 / 转发给 coder-02 / 升级
```

### 对话串联

通过 `ReplyTo` 字段串联消息链：

```
msg-1: TL → Coder "实现登录功能"
msg-2: Coder → TL "数据库 schema 不确定，用 sessions 表还是 tokens 表？" (ReplyTo: msg-1)
msg-3: TL → Coder "用 sessions 表，参考 project B 的实现" (ReplyTo: msg-2)
```

每个 Actor 在处理消息时，自动加载 ReplyTo 链作为上下文。

## Gateway 设计

### 路由规则

```go
type RoutingRule struct {
    From     string      // Actor ID 或 "*"
    To       string      // Actor ID 或 role pattern
    MsgType  MessageType // 消息类型过滤
    Action   string      // allow / deny / redirect / transform
    Target   string      // redirect 目标（可选）
}
```

默认规则：
- TL 可以给所有人发任何消息
- Worker 只能给 TL 发 escalation 和 notify
- Worker 之间的 query 需要 TL 预先授权（防止 Agent 之间无限聊天）
- 外部（Human / A2A）只能给 TL 发消息

### 与外部的桥接

```
Human / A2A → Gateway → TL.Inbox     （外部消息统一进 TL）
              Gateway ← TL.Inbox     （TL 的回复统一出 Gateway）
                      → Human / A2A
```

TL 是外部世界的唯一接口。其他 Actor 对外不可见。
这与当前 A2A Bridge 的设计一致 — A2A 只跟"团队"对话，不跟个别成员对话。

## 存储

### 新增表

```sql
-- 角色模板（TL 动态创建）
CREATE TABLE roles (
    name        TEXT PRIMARY KEY,
    base_agent  TEXT NOT NULL,
    capabilities TEXT NOT NULL,  -- JSON
    prompt      TEXT,
    description TEXT,
    created_by  TEXT,            -- actor ID（通常是 TL）
    created_at  DATETIME,
    source      TEXT DEFAULT 'dynamic'  -- 'static' (config) / 'dynamic' (TL 创建)
);

-- Actor 实例
CREATE TABLE actors (
    id          TEXT PRIMARY KEY,
    role        TEXT NOT NULL REFERENCES roles(name),
    workspace   TEXT,
    status      TEXT NOT NULL DEFAULT 'idle',
    session_data TEXT,           -- 休眠时序列化的 session 状态
    last_active DATETIME,
    created_at  DATETIME
);

-- 消息（Inbox 持久化）
CREATE TABLE actor_messages (
    id          TEXT PRIMARY KEY,
    from_actor  TEXT NOT NULL,
    to_actor    TEXT NOT NULL,
    type        TEXT NOT NULL,
    reply_to    TEXT,
    priority    INTEGER DEFAULT 3,
    subject     TEXT,
    body        TEXT,
    data        TEXT,            -- JSON
    status      TEXT DEFAULT 'pending',  -- pending / delivered / processed / expired
    created_at  DATETIME,
    expires_at  DATETIME,
    processed_at DATETIME
);
```

### 与现有 Store 的关系

新增表，不改现有表。现有的 `issues`、`runs`、`checkpoints` 继续用于固定流水线。Actor 层是叠加的，不是替换的。

## 方案对比

### A: 纯 Actor（全部替换）

把流水线也用 Actor 重写。Run 不再是一等概念。

- 优点：统一模型，没有两套逻辑
- 缺点：改动巨大，现有测试全部失效，风险高

### B: Actor 层叠加（推荐）

保留流水线引擎。新增 Actor 层用于动态协作。流水线内部可选择复用常驻 Actor。

- 优点：增量实施，现有功能不受影响，两种模式按场景选用
- 缺点：两套执行模型并存，维护成本稍高

### C: 流水线渐进 Actor 化

先实现 Actor 基础设施（Inbox + Gateway）。然后逐步把流水线 stage 改为 Actor 消息。最终流水线变成 Actor 编排的一种"宏"。

- 优点：最终统一，但过程渐进
- 缺点：过渡期两套逻辑都在，统一时间不确定

**推荐 B，以 C 为北极星。** 先把 Actor 跑起来，验证价值，再决定是否统一。

## 实施路径

| 阶段 | 内容 | 依赖 |
|------|------|------|
| P0 | Gateway + Inbox + ActorMessage 存储 + 基础路由 | 无 |
| P0 | TL 常驻 Actor（第一个持久 session） | Gateway |
| P0 | TL 的 `send_message` / `check_inbox` MCP 工具 | Gateway + Inbox |
| P1 | Actor 生命周期管理（spawn / kill / sleep / wake） | P0 |
| P1 | TL 的角色管理工具（create_role / spawn_actor） | P0 |
| P1 | Worker Actor 复用（流水线 stage 可指向常驻 Actor） | P1 |
| P2 | 动态编排（TL 自由组合 Actor 完成非标任务） | P1 |
| P2 | Actor 间 query 通信（对等问答） | P1 + 权限规则 |
| P3 | 流水线 Actor 化（stage → Actor message 宏） | P2 验证后 |

## 开放问题

1. **ACP session 休眠精度** — 注入历史对话能恢复多少上下文？需要实验。可能需要 ACP 协议扩展支持 session snapshot。

2. **Actor 间聊天失控** — 两个 Agent 互相 query 可能无限循环。需要 Gateway 限制：每个 query 链最多 N 轮，超出自动 escalate 给 TL。

3. **资源预算** — 10 个常驻 Actor = 10 个 ACP 进程。API token 消耗怎么控制？idle 状态是否计费？需要根据 agent provider 的计费模型设计休眠策略。

4. **人类参与模式** — Human 是一个特殊 Actor 还是 Gateway 的外部接口？如果是 Actor，他的 Inbox 就是 Web UI 的通知面板。

5. **与 A2A 的关系** — 外部 A2A client 发消息给"团队"，Gateway 路由到 TL。TL 是否需要感知"这条消息来自外部 A2A"还是统一当作 Inbox 消息处理？

---

> **后续**: 本设计确定后，实施计划由 `plan-v3-actor-workspace` 承接。02 的 Escalation/Directive 协议作为消息类型的子集自然融入，不再需要单独实施。